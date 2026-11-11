package platform

import (
	"net/http"
	"regexp"
)

const AccountCookie = "__Host-carcruds-account"

func AccountRoute(r *http.Request) bool {
	switch r.Method + " " + r.URL.EscapedPath() {
	case "POST /v1/auth/register", "POST /v1/auth/login", "POST /v1/auth/logout", "GET /v1/auth/me":
		return true
	default:
		return false
	}
}

var catalogRoute = regexp.MustCompile(`^/v1/vehicles(?:/[a-zA-Z0-9][a-zA-Z0-9_-]{0,63})?$`)

// Personal sessions are accepted exclusively on the catalog read surface.
func AccountCatalogRoute(r *http.Request) bool {
	if r.Method != http.MethodGet || r.Header.Get("Authorization") != "" || !catalogRoute.MatchString(r.URL.EscapedPath()) {
		return false
	}
	cookie, err := r.Cookie(AccountCookie)
	return err == nil && len(cookie.Value) == 64
}
