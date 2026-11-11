package fleet

import (
	"net/http"
	"time"

	"github.com/Gerpeirone/Car-Cruds/internal/platform"
)

func WithAccountCatalog(next http.Handler, customersURL string) http.Handler {
	client := platform.NewHTTPClient(2 * time.Second)
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !platform.AccountCatalogRoute(r) {
			next.ServeHTTP(w, r)
			return
		}
		cookie, _ := r.Cookie(platform.AccountCookie)
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, customersURL+"/v1/auth/me", nil)
		if err != nil {
			platform.Error(w, 503, "SERVICE_UNAVAILABLE", "No se pudo verificar el acceso.")
			return
		}
		req.AddCookie(cookie)
		res, err := client.Do(req)
		if err != nil {
			platform.Error(w, 503, "SERVICE_UNAVAILABLE", "No se pudo verificar el acceso.")
			return
		}
		defer res.Body.Close()
		switch res.StatusCode {
		case http.StatusOK:
			*r = *r.WithContext(platform.WithPrincipal(r.Context(), platform.Principal{Role: "catalog"}))
			next.ServeHTTP(w, r)
		case http.StatusUnauthorized:
			platform.Error(w, 401, "UNAUTHORIZED", "La sesión venció o no es válida.")
		default:
			platform.Error(w, 503, "SERVICE_UNAVAILABLE", "No se pudo verificar el acceso.")
		}
	})
}
