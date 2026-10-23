package platform

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type Principal struct{ App, Role, CustomerID string }
type principalKey struct{}
type requestIDKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}
func PrincipalFrom(ctx context.Context) Principal {
	p, _ := ctx.Value(principalKey{}).(Principal)
	return p
}
func Env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func UUID() string { return uuid.NewString() }

func TokensFromEnv() (map[string]Principal, error) {
	entries := []struct {
		key       string
		principal Principal
	}{
		{"AUTH_ADMIN_TOKEN", Principal{App: "operador", Role: "admin"}},
		{"AUTH_CLIENT_TOKEN", Principal{App: "cliente-demo", Role: "client", CustomerID: Env("AUTH_CLIENT_CUSTOMER_ID", "cli-001")}},
		{"AUTH_GYM_TOKEN", Principal{App: "gimnasio-pablo", Role: "gym"}},
		{"AUTH_SERVICE_TOKEN", Principal{App: "car-cruds", Role: "service"}},
	}
	tokens := make(map[string]Principal)
	for _, entry := range entries {
		value := os.Getenv(entry.key)
		if value == "" {
			if entry.principal.Role == "admin" || entry.principal.Role == "service" {
				return nil, errors.New("faltan credenciales de aplicación obligatorias")
			}
			continue
		}
		if len(value) < 24 {
			return nil, errors.New("las credenciales de aplicación deben tener al menos 24 caracteres")
		}
		if _, exists := tokens[value]; exists {
			return nil, errors.New("cada rol requiere una credencial diferente")
		}
		tokens[value] = entry.principal
	}
	return tokens, nil
}

func AuthMiddleware(next http.Handler, tokens map[string]Principal) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || parts[0] != "Bearer" {
			Error(w, 401, "UNAUTHORIZED", "Se requiere autenticación de aplicación")
			return
		}
		var principal Principal
		for token, p := range tokens {
			if subtle.ConstantTimeCompare([]byte(token), []byte(parts[1])) == 1 {
				principal = p
			}
		}
		if principal.App == "" {
			Error(w, 401, "UNAUTHORIZED", "Credencial inválida")
			return
		}
		*r = *r.WithContext(WithPrincipal(r.Context(), principal))
		next.ServeHTTP(w, r)
	})
}

func RequireRole(w http.ResponseWriter, r *http.Request, roles ...string) bool {
	p := PrincipalFrom(r.Context())
	for _, role := range roles {
		if p.Role == role {
			return true
		}
	}
	Error(w, 403, "FORBIDDEN", "La aplicación no tiene permiso para esta operación")
	return false
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func Error(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, map[string]string{"code": code, "message": message})
}
func Decode(w http.ResponseWriter, r *http.Request, out any) error {
	defer r.Body.Close()
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		return errors.New("cuerpo JSON inválido o demasiado grande")
	}
	check := json.NewDecoder(bytes.NewReader(raw))
	check.UseNumber()
	value, err := uniqueJSON(check, 0)
	if err != nil {
		return err
	}
	object, ok := value.(map[string]any)
	if !ok {
		return errors.New("se requiere un objeto JSON")
	}
	if _, err = check.Token(); err != io.EOF {
		return errors.New("se admite un único objeto JSON")
	}
	typ := reflect.TypeOf(out)
	if typ != nil && typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ != nil && typ.Kind() == reflect.Struct {
		allowed := map[string]bool{}
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if !f.IsExported() {
				continue
			}
			name := strings.Split(f.Tag.Get("json"), ",")[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = f.Name
			}
			allowed[name] = true
		}
		for name := range object {
			if !allowed[name] {
				return errors.New("campo JSON desconocido")
			}
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return errors.New("cuerpo JSON inválido o demasiado grande")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("se admite un único objeto JSON")
	}
	return nil
}

func uniqueJSON(decoder *json.Decoder, depth int) (any, error) {
	if depth > 32 {
		return nil, errors.New("JSON demasiado anidado")
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, errors.New("JSON inválido")
	}
	if delimiter, ok := token.(json.Delim); ok {
		switch delimiter {
		case '{':
			object := map[string]any{}
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return nil, errors.New("JSON inválido")
				}
				name, ok := key.(string)
				if !ok {
					return nil, errors.New("JSON inválido")
				}
				if _, exists := object[name]; exists {
					return nil, errors.New("campo JSON duplicado")
				}
				value, err := uniqueJSON(decoder, depth+1)
				if err != nil {
					return nil, err
				}
				object[name] = value
			}
			if closing, err := decoder.Token(); err != nil || closing != json.Delim('}') {
				return nil, errors.New("JSON inválido")
			}
			return object, nil
		case '[':
			var array []any
			for decoder.More() {
				value, err := uniqueJSON(decoder, depth+1)
				if err != nil {
					return nil, err
				}
				array = append(array, value)
			}
			if closing, err := decoder.Token(); err != nil || closing != json.Delim(']') {
				return nil, errors.New("JSON inválido")
			}
			return array, nil
		default:
			return nil, errors.New("JSON inválido")
		}
	}
	return token, nil
}
func NewHTTPClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConnsPerHost = 32
	return &http.Client{Timeout: timeout, Transport: correlationTransport{otelhttp.NewTransport(transport)}}
}

type correlationTransport struct{ base http.RoundTripper }

func (t correlationTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	request := r.Clone(r.Context())
	if id, ok := r.Context().Value(requestIDKey{}).(string); ok {
		request.Header.Set("X-Request-ID", id)
	}
	return t.base.RoundTrip(request)
}
