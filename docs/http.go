// Package docs publica únicamente la documentación de las API propias vigentes.
package docs

import (
	"embed"
	"net/http"
)

//go:embed api/index.html api/swagger-initializer.js api/catalog.json api/vendor/* contracts/*-openapi.json
var content embed.FS

// Handler ofrece un visor de lectura independiente del gateway y sus credenciales.
func Handler() http.Handler {
	allowed := map[string]bool{
		"/api/":                                       true,
		"/api/index.html":                             true,
		"/api/swagger-initializer.js":                 true,
		"/api/catalog.json":                           true,
		"/api/vendor/swagger-ui.css":                  true,
		"/api/vendor/swagger-ui-bundle.js":            true,
		"/api/vendor/swagger-ui-standalone-preset.js": true,
	}
	for _, name := range []string{"customers", "fleet", "rentals", "employees", "clinic-integration", "operations"} {
		allowed["/contracts/"+name+"-openapi.json"] = true
	}
	files := http.FileServer(http.FS(content))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "El visor admite únicamente lectura", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/api/", http.StatusTemporaryRedirect)
			return
		}
		if !allowed[r.URL.Path] {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-cache")
		files.ServeHTTP(w, r)
	})
}
