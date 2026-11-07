package managed

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
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

// authorized accepts a session cookie. Without an Email (the embedded catalyst
// surface), HTTP basic auth with the password stays; otherwise every client
// logs in with email and password.
func (h *HTTP) authorized(q *http.Request) bool {
	if h.Password == "" {
		return true
	}
	if c, err := q.Cookie(sessionCookie); err == nil && h.sessions.valid(c.Value, time.Now()) {
		return true
	}
	if h.Email == "" {
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

// login serves the login form and checks email plus password.
func (h *HTTP) login(w http.ResponseWriter, q *http.Request) {
	if h.Email == "" {
		http.NotFound(w, q)
		return
	}
	now := time.Now()
	if q.Method == http.MethodGet {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = Login(q.URL.Query().Get("error")).Render(q.Context(), w)
		return
	}
	if q.Method != http.MethodPost || !sameOrigin(q) {
		http.Error(w, "Method tidak diizinkan", http.StatusMethodNotAllowed)
		return
	}
	q.Body = http.MaxBytesReader(w, q.Body, 4096)
	if q.ParseForm() != nil {
		http.Error(w, "Form tidak valid", http.StatusBadRequest)
		return
	}
	fail := func(msg string) { http.Redirect(w, q, "/login?error="+url.QueryEscape(msg), http.StatusSeeOther) }
	if h.sessions.locked(now) {
		fail("Terlalu banyak percobaan. Tunggu 5 menit.")
		return
	}
	email := strings.ToLower(strings.TrimSpace(q.FormValue("email")))
	okEmail := subtle.ConstantTimeCompare([]byte(email), []byte(strings.ToLower(h.Email))) == 1
	okPass := subtle.ConstantTimeCompare([]byte(q.FormValue("password")), []byte(h.Password)) == 1
	if !okEmail || !okPass {
		h.sessions.fail(now)
		fail("Email atau password salah.")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: h.sessions.start(now), Path: "/", MaxAge: int(sessionTTL.Seconds()), HttpOnly: true, Secure: secure(q), SameSite: http.SameSiteStrictMode})
	http.Redirect(w, q, "/trade", http.StatusSeeOther)
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

// liveConfirmed requires the password again for actions on a real-money wallet.
// Wrong guesses count toward the login lockout.
func (h *HTTP) liveConfirmed(mode, password string) bool {
	if mode != "live" || h.Password == "" {
		return true
	}
	now := time.Now()
	if h.sessions.locked(now) {
		return false
	}
	if subtle.ConstantTimeCompare([]byte(password), []byte(h.Password)) == 1 {
		return true
	}
	h.sessions.fail(now)
	return false
}
