package managed

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	sessionCookie = "tb_session"
	sessionTTL    = 12 * time.Hour
	// loginFailures within loginWindow lock logins for loginWindow. Global,
	// not per IP: behind the proxy every client shares one address, and the
	// site has a single operator.
	loginFailures = 5
	loginWindow   = 5 * time.Minute
)

type sessions struct {
	mu       sync.Mutex
	active   map[string]time.Time
	failures []time.Time
}

func (s *sessions) init() {
	if s.active == nil {
		s.active = map[string]time.Time{}
	}
}

func (s *sessions) valid(token string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.init()
	exp, ok := s.active[token]
	if ok && now.After(exp) {
		delete(s.active, token)
		return false
	}
	return ok
}

func (s *sessions) locked(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	recent := s.failures[:0]
	for _, f := range s.failures {
		if now.Sub(f) < loginWindow {
			recent = append(recent, f)
		}
	}
	s.failures = recent
	return len(recent) >= loginFailures
}

func (s *sessions) fail(now time.Time) {
	s.mu.Lock()
	s.failures = append(s.failures, now)
	s.mu.Unlock()
}

func (s *sessions) start(now time.Time) string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.init()
	s.active[token] = now.Add(sessionTTL)
	return token
}

func (s *sessions) end(token string) {
	s.mu.Lock()
	delete(s.active, token)
	s.mu.Unlock()
}

// reset logs every browser out, e.g. after a password change.
func (s *sessions) reset() {
	s.mu.Lock()
	s.active = map[string]time.Time{}
	s.mu.Unlock()
}

// Setup prepares registration: while no account exists it returns a one-time
// code the first visitor must enter on /register. The caller logs it, so only
// someone with access to the server logs can claim the dashboard.
func (h *HTTP) Setup(ctx context.Context) (string, error) {
	if h.Owners == nil {
		return "", nil
	}
	if _, ok, err := h.Owners.Owner(ctx); err != nil || ok {
		return "", err
	}
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	h.setupMu.Lock()
	defer h.setupMu.Unlock()
	h.setupCode = hex.EncodeToString(b)
	return h.setupCode, nil
}

// authorized accepts a session cookie. Without Owners (the embedded catalyst
// surface) HTTP basic auth with Password stays; otherwise every client logs in.
func (h *HTTP) authorized(q *http.Request) bool {
	if h.Owners == nil && h.Password == "" {
		return true
	}
	if c, err := q.Cookie(sessionCookie); err == nil && h.sessions.valid(c.Value, time.Now()) {
		return true
	}
	if h.Owners == nil {
		_, pass, ok := q.BasicAuth()
		return ok && subtle.ConstantTimeCompare([]byte(pass), []byte(h.Password)) == 1
	}
	return false
}

// secure reports whether the browser reached us over HTTPS (directly or via the proxy).
func secure(q *http.Request) bool {
	return q.TLS != nil || q.Header.Get("X-Forwarded-Proto") == "https"
}

func sameOrigin(q *http.Request) bool {
	origin := q.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host == q.Host
}

func htmlPage(w http.ResponseWriter) { w.Header().Set("Content-Type", "text/html; charset=utf-8") }

// postForm accepts a same-origin form POST of at most 4 KiB.
func postForm(w http.ResponseWriter, q *http.Request) bool {
	if q.Method != http.MethodPost || !sameOrigin(q) {
		http.Error(w, "Method tidak diizinkan", http.StatusMethodNotAllowed)
		return false
	}
	q.Body = http.MaxBytesReader(w, q.Body, 4096)
	if q.ParseForm() != nil {
		http.Error(w, "Form tidak valid", http.StatusBadRequest)
		return false
	}
	return true
}

// owner loads the account; ok is false after an error response was written.
func (h *HTTP) owner(w http.ResponseWriter, q *http.Request) (o Owner, exists, ok bool) {
	o, exists, err := h.Owners.Owner(q.Context())
	if err != nil {
		http.Error(w, "Akun tidak tersedia", http.StatusServiceUnavailable)
		return Owner{}, false, false
	}
	return o, exists, true
}

func (h *HTTP) startSession(w http.ResponseWriter, q *http.Request, now time.Time) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: h.sessions.start(now), Path: "/", MaxAge: int(sessionTTL.Seconds()), HttpOnly: true, Secure: secure(q), SameSite: http.SameSiteStrictMode})
}

func redirectError(w http.ResponseWriter, q *http.Request, path, msg string) {
	http.Redirect(w, q, path+"?error="+url.QueryEscape(msg), http.StatusSeeOther)
}

// login serves the login form and checks email plus password.
func (h *HTTP) login(w http.ResponseWriter, q *http.Request) {
	if h.Owners == nil {
		http.NotFound(w, q)
		return
	}
	o, exists, ok := h.owner(w, q)
	if !ok {
		return
	}
	if !exists {
		http.Redirect(w, q, "/register", http.StatusSeeOther)
		return
	}
	if q.Method == http.MethodGet {
		htmlPage(w)
		_ = Login(q.URL.Query().Get("error")).Render(q.Context(), w)
		return
	}
	if !postForm(w, q) {
		return
	}
	now := time.Now()
	if h.sessions.locked(now) {
		redirectError(w, q, "/login", "Terlalu banyak percobaan. Tunggu 5 menit.")
		return
	}
	email := strings.ToLower(strings.TrimSpace(q.FormValue("email")))
	okEmail := subtle.ConstantTimeCompare([]byte(email), []byte(o.Email)) == 1
	okPass := checkPassword(o.Hash, q.FormValue("password"))
	if !okEmail || !okPass {
		h.sessions.fail(now)
		redirectError(w, q, "/login", "Email atau password salah.")
		return
	}
	h.startSession(w, q, now)
	http.Redirect(w, q, "/trade", http.StatusSeeOther)
}

// register creates the only account, once, with the setup code from the log.
func (h *HTTP) register(w http.ResponseWriter, q *http.Request) {
	if h.Owners == nil {
		http.NotFound(w, q)
		return
	}
	_, exists, ok := h.owner(w, q)
	if !ok {
		return
	}
	if exists {
		http.Redirect(w, q, "/login", http.StatusSeeOther)
		return
	}
	if q.Method == http.MethodGet {
		htmlPage(w)
		_ = Register(q.URL.Query().Get("error")).Render(q.Context(), w)
		return
	}
	if !postForm(w, q) {
		return
	}
	now := time.Now()
	if h.sessions.locked(now) {
		redirectError(w, q, "/register", "Terlalu banyak percobaan. Tunggu 5 menit.")
		return
	}
	h.setupMu.Lock()
	code := h.setupCode
	h.setupMu.Unlock()
	if code == "" || subtle.ConstantTimeCompare([]byte(strings.TrimSpace(q.FormValue("setup_code"))), []byte(code)) != 1 {
		h.sessions.fail(now)
		redirectError(w, q, "/register", "Kode setup salah. Lihat log bot di server.")
		return
	}
	email := strings.ToLower(strings.TrimSpace(q.FormValue("email")))
	if a, err := mail.ParseAddress(email); err != nil || a.Address != email || len(email) > 254 {
		redirectError(w, q, "/register", "Email tidak valid.")
		return
	}
	password := q.FormValue("password")
	if msg := passwordProblem(password, q.FormValue("password_confirm")); msg != "" {
		redirectError(w, q, "/register", msg)
		return
	}
	hash, err := hashPassword(password)
	if err != nil {
		http.Error(w, "Gagal menyimpan akun", http.StatusInternalServerError)
		return
	}
	created, err := h.Owners.Register(q.Context(), Owner{Email: email, Hash: hash})
	if err != nil {
		http.Error(w, "Gagal menyimpan akun", http.StatusServiceUnavailable)
		return
	}
	if !created {
		http.Redirect(w, q, "/login", http.StatusSeeOther)
		return
	}
	h.setupMu.Lock()
	h.setupCode = ""
	h.setupMu.Unlock()
	h.startSession(w, q, now)
	http.Redirect(w, q, "/trade", http.StatusSeeOther)
}

func passwordProblem(password, confirm string) string {
	switch {
	case len([]rune(password)) < minPassword:
		return "Password minimal 8 karakter."
	case len(password) > 1024:
		return "Password terlalu panjang."
	case password != confirm:
		return "Konfirmasi password tidak sama."
	}
	return ""
}

// account changes the password; every other session is logged out.
func (h *HTTP) account(w http.ResponseWriter, q *http.Request) {
	if h.Owners == nil {
		http.NotFound(w, q)
		return
	}
	o, exists, ok := h.owner(w, q)
	if !ok {
		return
	}
	if !exists {
		http.Redirect(w, q, "/register", http.StatusSeeOther)
		return
	}
	if q.Method == http.MethodGet {
		htmlPage(w)
		_ = Account(o.Email, h.token, q.URL.Query().Get("error"), q.URL.Query().Has("done")).Render(q.Context(), w)
		return
	}
	if !postForm(w, q) {
		return
	}
	if subtle.ConstantTimeCompare([]byte(q.FormValue("csrf")), []byte(h.token)) != 1 {
		http.Error(w, "Token aksi tidak valid", http.StatusForbidden)
		return
	}
	now := time.Now()
	if h.sessions.locked(now) {
		redirectError(w, q, "/account", "Terlalu banyak percobaan. Tunggu 5 menit.")
		return
	}
	if !checkPassword(o.Hash, q.FormValue("current_password")) {
		h.sessions.fail(now)
		redirectError(w, q, "/account", "Password lama salah.")
		return
	}
	password := q.FormValue("password")
	if msg := passwordProblem(password, q.FormValue("password_confirm")); msg != "" {
		redirectError(w, q, "/account", msg)
		return
	}
	hash, err := hashPassword(password)
	if err == nil {
		err = h.Owners.SetHash(q.Context(), hash)
	}
	if err != nil {
		http.Error(w, "Gagal menyimpan password", http.StatusServiceUnavailable)
		return
	}
	h.sessions.reset()
	h.startSession(w, q, now)
	http.Redirect(w, q, "/account?done=1", http.StatusSeeOther)
}

func (h *HTTP) logout(w http.ResponseWriter, q *http.Request) {
	if q.Method != http.MethodPost || !sameOrigin(q) {
		http.Error(w, "Method tidak diizinkan", http.StatusMethodNotAllowed)
		return
	}
	if c, err := q.Cookie(sessionCookie); err == nil {
		h.sessions.end(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: secure(q), SameSite: http.SameSiteStrictMode})
	http.Redirect(w, q, "/login", http.StatusSeeOther)
}

// liveConfirmed requires the account password again for actions on a
// real-money wallet. Wrong guesses count toward the login lockout.
func (h *HTTP) liveConfirmed(ctx context.Context, mode, password string) bool {
	if mode != "live" {
		return true
	}
	if h.Owners == nil && h.Password == "" {
		return true
	}
	now := time.Now()
	if h.sessions.locked(now) {
		return false
	}
	var ok bool
	if h.Owners != nil {
		o, exists, err := h.Owners.Owner(ctx)
		ok = err == nil && exists && checkPassword(o.Hash, password)
	} else {
		ok = subtle.ConstantTimeCompare([]byte(password), []byte(h.Password)) == 1
	}
	if !ok {
		h.sessions.fail(now)
	}
	return ok
}
