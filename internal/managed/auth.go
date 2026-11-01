package managed

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"fmt"
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

// totpCode is the RFC 6238 code (HMAC-SHA1, 30 s, 6 digits) of a base32 secret.
func totpCode(secret string, t time.Time) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.TrimRight(strings.ReplaceAll(secret, " ", ""), "=")))
	if err != nil || len(key) < 10 {
		return "", fmt.Errorf("TOTP secret must be base32 with at least 80 bits")
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(t.Unix()/30))
	m := hmac.New(sha1.New, key)
	m.Write(msg[:])
	sum := m.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	n := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", n%1_000_000), nil
}

// ValidTOTPSecret reports whether secret can produce codes.
func ValidTOTPSecret(secret string) bool { _, err := totpCode(secret, time.Now()); return err == nil }

// checkTOTP accepts the current code and one step either side (clock drift).
func checkTOTP(secret, code string, now time.Time) bool {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false
	}
	ok := false
	for _, step := range []time.Duration{-30 * time.Second, 0, 30 * time.Second} {
		want, err := totpCode(secret, now.Add(step))
		if err == nil && subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 {
			ok = true
		}
	}
	return ok
}

type sessions struct {
	mu       sync.Mutex
	active   map[string]time.Time
	failures []time.Time
	// used blocks replaying a TOTP code inside its validity window.
	used map[string]time.Time
}

func (s *sessions) init() {
	if s.active == nil {
		s.active, s.used = map[string]time.Time{}, map[string]time.Time{}
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

// fresh records code as used; a replay inside the window is refused.
func (s *sessions) fresh(code string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.init()
	for c, at := range s.used {
		if now.Sub(at) > 2*time.Minute {
			delete(s.used, c)
		}
	}
	if _, seen := s.used[code]; seen {
		return false
	}
	s.used[code] = now
	return true
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

// authorized accepts a session cookie. Without TOTP, HTTP basic auth stays
// available for scripts; with TOTP every client must log in with a code.
func (h *HTTP) authorized(q *http.Request) bool {
	if h.Password == "" {
		return true
	}
	if c, err := q.Cookie(sessionCookie); err == nil && h.sessions.valid(c.Value, time.Now()) {
		return true
	}
	if h.TOTP == "" {
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

// login serves the login form and checks password plus TOTP code.
func (h *HTTP) login(w http.ResponseWriter, q *http.Request) {
	now := time.Now()
	if q.Method == http.MethodGet {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = Login(h.TOTP != "", q.URL.Query().Get("error")).Render(q.Context(), w)
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
	ok := subtle.ConstantTimeCompare([]byte(q.FormValue("password")), []byte(h.Password)) == 1
	if h.TOTP != "" {
		code := strings.TrimSpace(q.FormValue("code"))
		ok = ok && checkTOTP(h.TOTP, code, now) && h.sessions.fresh(code, now)
	}
	if !ok {
		h.sessions.fail(now)
		fail("Password atau kode salah.")
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

// liveConfirmed requires a fresh TOTP code for actions on a real-money wallet.
func (h *HTTP) liveConfirmed(mode, code string) bool {
	if mode != "live" || h.TOTP == "" {
		return true
	}
	now := time.Now()
	return checkTOTP(h.TOTP, code, now) && h.sessions.fresh(code, now)
}
