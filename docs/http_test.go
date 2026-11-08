package docs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestViewerPublishesCatalogAndOnlyDocumentation(t *testing.T) {
	handler := Handler()
	var catalog []struct{ File string }
	data, err := content.ReadFile("api/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	paths := []string{"/api/", "/api/swagger-initializer.js", "/api/vendor/swagger-ui.css", "/api/vendor/swagger-ui-bundle.js", "/api/vendor/swagger-ui-standalone-preset.js"}
	for _, entry := range catalog {
		paths = append(paths, "/contracts/"+entry.File)
	}
	for _, path := range paths {
		result := httptest.NewRecorder()
		handler.ServeHTTP(result, httptest.NewRequest(http.MethodGet, path, nil))
		if result.Code != http.StatusOK || result.Body.Len() == 0 {
			t.Errorf("%s: status %d, bytes %d", path, result.Code, result.Body.Len())
		}
	}
	for _, path := range []string{"/.env", "/contracts/", "/contracts/employees-openapi-draft.json", "/contracts/providers/octi-turnos-partner-v1.openapi.json", "/http.go", "/v1/employees", "/api/vendor/"} {
		result := httptest.NewRecorder()
		handler.ServeHTTP(result, httptest.NewRequest(http.MethodGet, path, nil))
		if result.Code != http.StatusNotFound {
			t.Errorf("%s: se esperaba 404, recibido %d", path, result.Code)
		}
	}
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, httptest.NewRequest(http.MethodPost, "/api/", nil))
	if result.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST: se esperaba 405, recibido %d", result.Code)
	}
}
