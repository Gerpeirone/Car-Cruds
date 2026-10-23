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
	repo := &memoryRepository{employees: map[string]domain.Employee{}, candidates: map[string]domain.Candidate{}}
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

func TestSharedListProjectsActiveEmployeesAndNeverCandidates(t *testing.T) {
	handler := testHandler()
	for _, body := range []string{`{"displayName":"Ana ficticia","active":true}`, `{"displayName":"Luis ficticio","active":false}`, `{"displayName":"Rosa ficticia","active":true}`} {
		if w := request(handler, "POST", "/v1/employees", body, "test-admin"); w.Code != 201 {
			t.Fatalf("employee creation: %d %s", w.Code, w.Body)
		}
	}
	if w := request(handler, "POST", "/v1/candidates", `{"displayName":"Candidato ficticio"}`, "test-admin"); w.Code != 201 {
		t.Fatalf("candidate: %d", w.Code)
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

func TestGymEnrollmentRoutesRemainAdministrativeAndFailClosed(t *testing.T) {
	handler := testHandler()
	for _, method := range []string{"GET", "POST"} {
		for _, path := range []string{"/v1/employees/person/gym-enrollments", "/v1/employees/person/gym-enrollments/request"} {
			for _, token := range []string{"test-gym", "test-client"} {
				w := request(handler, method, path, `{"dni":"99900001","email":"person@example.invalid"}`, token)
				if w.Code != 403 {
					t.Fatalf("method=%s path=%s status=%d", method, path, w.Code)
				}
			}
		}
	}
	r := httptest.NewRequest("POST", "/v1/employees/person/gym-enrollments", strings.NewReader(`{"dni":"99900001","email":"person@example.invalid"}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer test-admin")
	r.Header.Set("Idempotency-Key", "fixture-key")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 503 || !strings.Contains(w.Body.String(), "NOT_CONFIGURED") {
		t.Fatal("unconfigured provider did not fail closed")
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
		{"GET", "/v1/candidates", "", "test-gym", 403},
		{"POST", "/v1/candidates", `{"displayName":"A"}`, "test-gym", 403},
		{"GET", "/v1/candidates/any", "", "test-service", 403},
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
		{"/v1/candidates/person-003", "/v1/candidates/{id}"},
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

func TestAdministrativeLifecycleDoesNotPromoteCandidatesImplicitly(t *testing.T) {
	handler := testHandler()
	steps := []struct {
		method, path, body string
		status             int
	}{
		{"POST", "/v1/candidates", `{"displayName":"Posible incorporación ficticia"}`, 201},
		{"GET", "/v1/candidates/person-001", "", 200},
		{"PUT", "/v1/candidates/person-001", `{"displayName":"Nombre revisado"}`, 200},
		{"POST", "/v1/employees", `{"displayName":"Empleado ficticio","active":false}`, 201},
		{"GET", "/v1/employees/person-002", "", 200},
		{"PUT", "/v1/employees/person-002", `{"displayName":"Empleado ficticio","active":true}`, 200},
		{"DELETE", "/v1/candidates/person-001", "", 204},
		{"GET", "/v1/candidates/person-001", "", 404},
		{"PUT", "/v1/employees/missing", `{"displayName":"No registrado","active":true}`, 404},
	}
	for _, step := range steps {
		w := request(handler, step.method, step.path, step.body, "test-admin")
		if w.Code != step.status {
			t.Fatalf("%s %s = %d: %s", step.method, step.path, w.Code, w.Body)
		}
	}
	w := request(handler, "GET", "/v1/employees", "", "test-gym")
	var response struct {
		Items []struct {
			ID string `json:"employeeId"`
		}
		Total int
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Total != 1 || len(response.Items) != 1 || response.Items[0].ID != "person-002" {
		t.Fatalf("candidate was promoted or employee update lost: %s", w.Body)
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
	employees  map[string]domain.Employee
	candidates map[string]domain.Candidate
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
func (m *memoryRepository) ListCandidates(_ context.Context, p domain.Page) ([]domain.Candidate, int64, error) {
	items := []domain.Candidate{}
	for _, item := range m.candidates {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	total := int64(len(items))
	start := p.Offset()
	if start >= total {
		return []domain.Candidate{}, total, nil
	}
	return items[start:min(start+int64(p.Size), total)], total, nil
}
func (m *memoryRepository) GetCandidate(_ context.Context, id string) (domain.Candidate, error) {
	item, ok := m.candidates[id]
	if !ok {
		return item, domain.ErrNotFound
	}
	return item, nil
}
func (m *memoryRepository) CreateCandidate(_ context.Context, item domain.Candidate) error {
	m.candidates[item.ID] = item
	return nil
}
func (m *memoryRepository) UpdateCandidate(ctx context.Context, item domain.Candidate) (domain.Candidate, error) {
	if _, err := m.GetCandidate(ctx, item.ID); err != nil {
		return item, err
	}
	m.candidates[item.ID] = item
	return item, nil
}
func (m *memoryRepository) DeleteCandidate(_ context.Context, id string) error {
	if _, ok := m.candidates[id]; !ok {
		return domain.ErrNotFound
	}
	delete(m.candidates, id)
	return nil
}
