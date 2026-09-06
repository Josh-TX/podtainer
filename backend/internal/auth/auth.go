// Package auth guards podtainer with a single admin password stored as a
// bcrypt hash in an htpasswd-formatted file, independent of OS user accounts.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	htpasswdUser = "admin"
	cookieName   = "podtainer_session"
	sessionTTL   = 7 * 24 * time.Hour
)

type Auth struct {
	file string

	mu       sync.Mutex
	sessions map[string]time.Time
}

func New(file string) *Auth {
	return &Auth{
		file:     file,
		sessions: map[string]time.Time{},
	}
}

func (a *Auth) IsConfigured() bool {
	_, err := os.Stat(a.file)
	return err == nil
}

// SetPassword writes the htpasswd file. Callers must ensure this only runs
// during first-time setup; it overwrites any existing file.
func (a *Auth) SetPassword(password string) error {
	if password == "" {
		return errors.New("password must not be empty")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	line := fmt.Sprintf("%s:%s\n", htpasswdUser, hash)
	return os.WriteFile(a.file, []byte(line), 0o600)
}

func (a *Auth) checkPassword(password string) bool {
	data, err := os.ReadFile(a.file)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		user, hash, ok := strings.Cut(line, ":")
		if !ok || user != htpasswdUser {
			continue
		}
		return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
	}
	return false
}

func (a *Auth) newSession() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	token := hex.EncodeToString(buf)

	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessions[token] = time.Now().Add(sessionTTL)
	return token
}

func (a *Auth) validSession(token string) bool {
	if token == "" {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	expiry, ok := a.sessions[token]
	if !ok {
		return false
	}
	if time.Now().After(expiry) {
		delete(a.sessions, token)
		return false
	}
	return true
}

func (a *Auth) revoke(token string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.sessions, token)
}

func (a *Auth) setCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Expires:  expires,
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// StatusHandler tells the SPA whether a password has been set yet and
// whether the current cookie (if any) is a valid session.
func (a *Auth) StatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authenticated := false
		if cookie, err := r.Cookie(cookieName); err == nil {
			authenticated = a.validSession(cookie.Value)
		}
		writeJSON(w, map[string]bool{
			"needsSetup":    !a.IsConfigured(),
			"authenticated": authenticated,
		})
	}
}

// SetupHandler creates the admin password on first run only; once a
// password file exists it always refuses so it can't be used to reset one.
func (a *Auth) SetupHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.IsConfigured() {
			writeErr(w, http.StatusConflict, "already configured")
			return
		}
		var body struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := a.SetPassword(body.Password); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		token := a.newSession()
		a.setCookie(w, token, time.Now().Add(sessionTTL))
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func (a *Auth) LoginHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if !a.checkPassword(body.Password) {
			writeErr(w, http.StatusUnauthorized, "incorrect password")
			return
		}
		token := a.newSession()
		a.setCookie(w, token, time.Now().Add(sessionTTL))
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func (a *Auth) LogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie(cookieName); err == nil {
			a.revoke(cookie.Value)
		}
		a.setCookie(w, "", time.Unix(0, 0))
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

// Middleware rejects any request without a valid session cookie, letting
// the SPA's own routing decide what to show for the login/setup screens.
func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(cookieName)
		if err != nil || !a.validSession(cookie.Value) {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
