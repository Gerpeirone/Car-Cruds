package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/employees/adapters/httpapi"
	"github.com/Gerpeirone/Car-Cruds/services/employees/application"
	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
)

func testHandler() http.Handler {
	repo := &memoryRepository{employees: map[string]domain.Employee{}}
	next := 0
	service := application.New(repo, func() string { next++; return fmt.Sprintf("person-%03d", next) })
	return platform.AuthMiddleware(httpapi.New(service), map[string]platform.Principal{
		"test-admin":   {App: "test-operator", Role: "admin"},
		"test-gym":     {App: "test-gym", Role: "gym"},
		"test-client":  {App: "test-customer", Role: "client"},
		"test-service": {App: "test-internal", Role: "service"},
	})
}

func request(handler http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestSharedListProjectsActiveEmployees(t *testing.T) {
	handler := testHandler()
	for _, body := range []string{`{"displayName":"Ana ficticia","active":true}`, `{"displayName":"Luis ficticio","active":false}`, `{"displayName":"Rosa ficticia","active":true}`} {
		if w := request(handler, "POST", "/v1/employees", body, "test-admin"); w.Code != 201 {
			t.Fatalf("employee creation: %d %s", w.Code, w.Body)
		}
	}
	w := request(handler, "GET", "/v1/employees?page=2&pageSize=1", "", "test-gym")
	if w.Code != 200 {
		t.Fatalf("list: %d %s", w.Code, w.Body)
	}
	var page struct {
		Items          []map[string]any `json:"items"`
		Page, PageSize int
		Total          int
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Page != 2 || page.PageSize != 1 || page.Total != 2 || len(page.Items) != 1 || page.Items[0]["employeeId"] != "person-003" {
		t.Fatalf("wrong active page: %s", w.Body)
	}
	for _, item := range page.Items {
		if len(item) != 2 || item["displayName"] == nil || item["employeeId"] == nil {
			t.Fatalf("projection leaks undeclared fields: %+v", item)
		}
	}
	// A logical leave immediately affects the shared population and remains in
	// administration; repeating it cannot recreate or duplicate a record.
	for attempt := 0; attempt < 2; attempt++ {
		if w := request(handler, "DELETE", "/v1/employees/person-003", "", "test-admin"); w.Code != 200 {
			t.Fatalf("deactivation: %d", w.Code)
		}
	}
	w = request(handler, "GET", "/v1/employees", "", "test-gym")
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 {
		t.Fatalf("inactive employee still shared: %s", w.Body)
	}
	w = request(handler, "GET", "/v1/admin/employees", "", "test-admin")
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 {
		t.Fatalf("administration lost leave records: %s", w.Body)
	}
}

func TestRoleIsolationAndUnconfiguredIntegrations(t *testing.T) {
	handler := testHandler()
	for _, entry := range []struct {
		method, path, body, token string
		status                    int
	}{
		{"GET", "/v1/employees", "", "", 401},
		{"GET", "/v1/employees", "", "test-client", 403},
		{"POST", "/v1/employees", `{"displayName":"A","active":true}`, "test-gym", 403},
		{"PUT", "/v1/employees/any", `{"displayName":"A","active":true}`, "test-gym", 403},
		{"DELETE", "/v1/employees/any", "", "test-gym", 403},
		{"GET", "/v1/admin/employees", "", "test-gym", 403},
		{"GET", "/v1/candidates", "", "test-gym", 404},
		{"POST", "/v1/candidates", `{"displayName":"A"}`, "test-gym", 404},
		{"GET", "/v1/candidates/any", "", "test-service", 404},
		{"GET", "/v1/employees/integrations", "", "test-gym", 403},
		{"GET", "/v1/employees/integrations", "", "test-admin", 503},
	} {
		w := request(handler, entry.method, entry.path, entry.body, entry.token)
		if w.Code != entry.status {
			t.Errorf("%s %s (%s): %d != %d", entry.method, entry.path, entry.token, w.Code, entry.status)
		}
		if entry.path == "/v1/employees/integrations" && entry.status == 503 {
			var body map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body["code"] != "NOT_CONFIGURED" || len(body) != 2 {
				t.Errorf("integration invented an outcome: %+v", body)
			}
		}
	}
}

func TestStableRoutePatternsSurvivePlatformMuxAndAuthentication(t *testing.T) {
	// Empleados uses a native ServeMux, so it supplies stable templates itself.
	// Reproduce Run's outer '/' mux and authentication to verify that the route
	// remains visible on the request observed by logs, traces and metrics.
	outer := http.NewServeMux()
	outer.Handle("/", testHandler())
	for _, entry := range []struct{ path, pattern string }{
		{"/v1/employees/person-001", "/v1/employees/{id}"},
		{"/v1/employees/person-002", "/v1/employees/{id}"},
		{"/v1/employees/integrations", "/v1/employees/integrations"},
	} {
		r := httptest.NewRequest(http.MethodGet, entry.path, nil)
		r.Header.Set("Authorization", "Bearer test-admin")
		outer.ServeHTTP(httptest.NewRecorder(), r)
		if r.Pattern != entry.pattern {
			t.Errorf("%s: route pattern %q != %q", entry.path, r.Pattern, entry.pattern)
		}
	}
}

func TestAdministrativeEmployeeLifecycle(t *testing.T) {
	handler := testHandler()
	for _, step := range []struct {
		method, path, body string
		status             int
	}{
		{"POST", "/v1/employees", `{"displayName":"Empleado ficticio","active":false}`, 201},
		{"GET", "/v1/employees/person-001", "", 200},
		{"PUT", "/v1/employees/person-001", `{"displayName":"Nombre revisado","active":true}`, 200},
		{"PUT", "/v1/employees/missing", `{"displayName":"No registrado","active":true}`, 404},
	} {
		w := request(handler, step.method, step.path, step.body, "test-admin")
		if w.Code != step.status {
			t.Fatalf("%s %s = %d: %s", step.method, step.path, w.Code, w.Body)
		}
	}
	w := request(handler, "GET", "/v1/employees", "", "test-gym")
	if !strings.Contains(w.Body.String(), `"employeeId":"person-001"`) || !strings.Contains(w.Body.String(), `"displayName":"Nombre revisado"`) {
		t.Fatalf("employee update lost: %s", w.Body)
	}
}

func TestCandidateRoutesWereRemoved(t *testing.T) {
	handler := testHandler()
	for _, route := range []struct{ method, path string }{
		{"GET", "/v1/candidates"}, {"POST", "/v1/candidates"},
		{"GET", "/v1/candidates/legacy"}, {"PUT", "/v1/candidates/legacy"}, {"DELETE", "/v1/candidates/legacy"},
	} {
		for _, token := range []string{"test-admin", "test-gym", "test-service"} {
			if w := request(handler, route.method, route.path, `{}`, token); w.Code != 404 {
				t.Errorf("retired route %s %s = %d", route.method, route.path, w.Code)
			}
		}
	}
}

func TestStrictInputsCannotCreateIncompleteOrMedicalProfiles(t *testing.T) {
	handler := testHandler()
	for _, body := range []string{`{"displayName":"Ana"}`, `{"displayName":"Ana","active":null}`, `null`, `[]`, `{"displayName":"Ana","active":true,"diagnosis":"private"}`, `{"displayName":"Ana","active":true} {}`, `{"displayName":" ","active":true}`, `{"displayName":"Ana","active":"true"}`, `{"displayName":"` + strings.Repeat("a", 1<<20) + `","active":true}`} {
		if w := request(handler, "POST", "/v1/employees", body, "test-admin"); w.Code != 400 {
			t.Errorf("accepted invalid body (length %d): %d", len(body), w.Code)
		}
	}
	for _, query := range []string{"?page=0", "?pageSize=101", "?page=1&page=2", "?page=bad", "?active=false", "?page=%ZZ", "?page=9223372036854775807&pageSize=100"} {
		if w := request(handler, "GET", "/v1/employees"+query, "", "test-gym"); w.Code != 400 {
			t.Errorf("accepted query %s: %d", query, w.Code)
		}
	}
	w := request(handler, "GET", "/v1/employees", "", "test-gym")
	if !strings.Contains(w.Body.String(), `"items":[]`) || !strings.Contains(w.Body.String(), `"total":0`) {
		t.Fatalf("invalid mutations persisted: %s", w.Body)
	}
}

func TestSharedResponseMatchesOwnOpenAPIProjection(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "docs", "contracts", "employees-openapi.json")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var contract struct {
		OpenAPI    string
		Components struct {
			Schemas map[string]struct {
				Required             []string
				Properties           map[string]json.RawMessage
				AdditionalProperties *bool
			}
		}
	}
	if err = json.Unmarshal(contents, &contract); err != nil {
		t.Fatal(err)
	}
	schema := contract.Components.Schemas["EmployeeSummary"]
	if contract.OpenAPI != "3.0.3" || len(schema.Properties) != 2 || len(schema.Required) != 2 || schema.AdditionalProperties == nil || *schema.AdditionalProperties {
		t.Fatal("shared schema must define exactly employeeId/displayName")
	}
	handler := testHandler()
	request(handler, "POST", "/v1/employees", `{"displayName":"Persona ficticia","active":true}`, "test-admin")
	w := request(handler, "GET", "/v1/employees", "", "test-gym")
	var response struct{ Items []map[string]json.RawMessage }
	if err = json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	for _, item := range response.Items {
		for property := range item {
			if _, declared := schema.Properties[property]; !declared {
				t.Errorf("undeclared response property %s", property)
			}
		}
		for _, required := range schema.Required {
			if len(item[required]) == 0 {
				t.Errorf("missing %s", required)
			}
		}
	}
}

// In-memory output-port double for HTTP and authorization unit tests only.
// PostgreSQL behavior is verified separately against TEST_DATABASE_URL.
type memoryRepository struct {
	employees map[string]domain.Employee
}

func (m *memoryRepository) ListEmployees(_ context.Context, p domain.Page, active bool) ([]domain.Employee, int64, error) {
	items := []domain.Employee{}
	for _, item := range m.employees {
		if !active || item.Active {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	total := int64(len(items))
	start := p.Offset()
	if start >= total {
		return []domain.Employee{}, total, nil
	}
	end := min(start+int64(p.Size), total)
	return items[start:end], total, nil
}
func (m *memoryRepository) GetEmployee(_ context.Context, id string) (domain.Employee, error) {
	item, ok := m.employees[id]
	if !ok {
		return item, domain.ErrNotFound
	}
	return item, nil
}
func (m *memoryRepository) CreateEmployee(_ context.Context, item domain.Employee) error {
	m.employees[item.ID] = item
	return nil
}
func (m *memoryRepository) UpdateEmployee(ctx context.Context, item domain.Employee) (domain.Employee, error) {
	if _, err := m.GetEmployee(ctx, item.ID); err != nil {
		return item, err
	}
	m.employees[item.ID] = item
	return item, nil
}
func (m *memoryRepository) DeactivateEmployee(ctx context.Context, id string) (domain.Employee, error) {
	item, err := m.GetEmployee(ctx, id)
	if err != nil {
		return item, err
	}
	item.Active = false
	m.employees[id] = item
	return item, nil
}
