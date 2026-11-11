package presentation

import (
	"crypto/sha256"
	"errors"
	"mime"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/customers/business"
)

type rateWindow struct {
	until time.Time
	count int
}

type AccountHandler struct {
	service    business.Accounts
	hashSlots  chan struct{}
	mu         sync.Mutex
	global     rateWindow
	accounts   map[[32]byte]rateWindow
	pruneAfter time.Time
}

func NewAccountHandler(service business.Accounts) *AccountHandler {
	return &AccountHandler{service: service, hashSlots: make(chan struct{}, 2), accounts: make(map[[32]byte]rateWindow)}
}

func (h *AccountHandler) retryAfter(email string) time.Duration {
	now := time.Now()
	h.mu.Lock()
	defer h.mu.Unlock()
	if !now.Before(h.global.until) {
		h.global = rateWindow{until: now.Add(time.Minute)}
	}
	if h.global.count >= 60 {
		return h.global.until.Sub(now)
	}
	h.global.count++
	if !now.Before(h.pruneAfter) {
		for key, window := range h.accounts {
			if !now.Before(window.until) {
				delete(h.accounts, key)
			}
		}
		h.pruneAfter = now.Add(time.Minute)
	}
	normalized, _ := business.NormalizeEmail(email)
	key := sha256.Sum256([]byte(normalized))
	window := h.accounts[key]
	if !now.Before(window.until) {
		window = rateWindow{until: now.Add(10 * time.Minute)}
	}
	if window.count >= 8 {
		return window.until.Sub(now)
	}
	window.count++
	h.accounts[key] = window
	return 0
}

type accountInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerInput struct {
	FirstName            string `json:"firstName"`
	LastName             string `json:"lastName"`
	Email                string `json:"email"`
	Password             string `json:"password"`
	PasswordConfirmation string `json:"passwordConfirmation"`
}

func (h *AccountHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !platform.AccountRoute(r) {
		platform.Error(w, 404, "NOT_FOUND", "Ruta inexistente.")
		return
	}
	r.Pattern = r.Method + " " + r.URL.Path
	if r.Method == http.MethodPost {
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			platform.Error(w, 415, "INVALID_INPUT", "Se requiere una solicitud JSON del sitio.")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16384)
	}
	var token string
	if cookie, err := r.Cookie(platform.AccountCookie); err == nil {
		token = cookie.Value
	}
	switch r.URL.Path {
	case "/v1/auth/register":
		var input registerInput
		if platform.Decode(w, r, &input) != nil {
			accountError(w, business.ErrAccountInput)
			return
		}
		if !h.acquire(w, input.Email) {
			return
		}
		defer func() { <-h.hashSlots }()
		if err := h.service.Register(r.Context(), input.FirstName, input.LastName, input.Email, input.Password, input.PasswordConfirmation); err != nil {
			accountError(w, err)
			return
		}
		platform.WriteJSON(w, 202, map[string]string{"status": "accepted"})
	case "/v1/auth/login":
		var input accountInput
		if platform.Decode(w, r, &input) != nil {
			accountError(w, business.ErrAccountInput)
			return
		}
		if !h.acquire(w, input.Email) {
			return
		}
		defer func() { <-h.hashSlots }()
		account, nextToken, expires, err := h.service.Login(r.Context(), input.Email, input.Password)
		if err != nil {
			accountError(w, err)
			return
		}
		// Replace the prior browser session after authenticating.
		if err := h.service.Logout(r.Context(), token); err != nil {
			_ = h.service.Logout(r.Context(), nextToken)
			accountError(w, err)
			return
		}
		setAccountCookie(w, nextToken, expires, 0)
		platform.WriteJSON(w, 200, account)
	case "/v1/auth/logout":
		if err := h.service.Logout(r.Context(), token); err != nil {
			accountError(w, err)
			return
		}
		setAccountCookie(w, "", time.Unix(0, 0), -1)
		w.WriteHeader(204)
	case "/v1/auth/me":
		account, err := h.service.Profile(r.Context(), token)
		if err != nil {
			accountError(w, err)
			return
		}
		platform.WriteJSON(w, 200, account)
	}
}

func (h *AccountHandler) acquire(w http.ResponseWriter, email string) bool {
	delay := time.Second
	select {
	case h.hashSlots <- struct{}{}:
		delay = h.retryAfter(email)
		if delay == 0 {
			return true
		}
		<-h.hashSlots
	default:
	}
	w.Header().Set("Retry-After", strconv.FormatInt(int64((delay+time.Second-1)/time.Second), 10))
	platform.Error(w, 429, "AUTH_RATE_LIMITED", "Demasiados intentos. Volvé a intentar más tarde.")
	return false
}

func setAccountCookie(w http.ResponseWriter, value string, expires time.Time, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: platform.AccountCookie, Value: value, Path: "/", Expires: expires,
		MaxAge: maxAge, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
}

func accountError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, business.ErrAccountInput):
		platform.Error(w, 400, "INVALID_ACCOUNT_INPUT", "Revisá el correo, los campos y la confirmación de contraseña.")
	case errors.Is(err, business.ErrAccountAuth):
		platform.Error(w, 401, "UNAUTHORIZED", "Credenciales inválidas o sesión vencida.")
	default:
		platform.Error(w, 503, "SERVICE_UNAVAILABLE", "El acceso no está disponible en este momento.")
	}
}
