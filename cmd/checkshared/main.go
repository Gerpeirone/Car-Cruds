// checkshared verifies the published employee capability using read-only requests.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Gerpeirone/Car-Cruds/internal/devconfig"
)

func main() {
	address := flag.String("url", "", "origen explícito de la API; HTTPS salvo loopback")
	envFile := flag.String("env", "", "archivo de credenciales opcional; por defecto no se lee .env")
	timeout := flag.Duration("timeout", 30*time.Second, "presupuesto total de verificación")
	flag.Parse()
	if *timeout <= 0 || devconfig.Load(*envFile) != nil {
		fmt.Println("FAIL: configuración inválida")
		os.Exit(2)
	}
	base, err := validateURL(*address)
	if err != nil || len(os.Getenv("AUTH_GYM_TOKEN")) < 24 {
		fmt.Println("FAIL: indicar un origen válido y AUTH_GYM_TOKEN de al menos 24 caracteres")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if err := verify(ctx, base, os.Getenv("AUTH_GYM_TOKEN")); err != nil {
		fmt.Printf("FAIL: %s\n", err)
		os.Exit(1)
	}
	fmt.Println("PASS: autenticación, lista mínima, paginación y aislamiento administrativo; solo lecturas, sin acreditar aceptación externa")
}

func validateURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", errors.New("origen inválido")
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	loopback := host == "localhost" || (ip != nil && ip.IsLoopback())
	if u.Scheme != "https" && !(u.Scheme == "http" && loopback) {
		return "", errors.New("se requiere HTTPS fuera de loopback")
	}
	return strings.TrimRight(u.String(), "/"), nil
}

func verify(ctx context.Context, base, token string) error {
	client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("redirección rechazada")
	}}
	defer client.CloseIdleConnections()
	get := func(path, credential string, expected int) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
		if err != nil {
			return nil, errors.New("solicitud inválida")
		}
		if credential != "" {
			req.Header.Set("Authorization", "Bearer "+credential)
		}
		resp, err := client.Do(req)
		if err != nil {
			// Transport errors may contain URLs or credentials; never print them.
			return nil, errors.New("no se pudo completar la conexión HTTP/TLS")
		}
		defer resp.Body.Close()
		if resp.StatusCode != expected {
			return nil, fmt.Errorf("GET %s: HTTP %d; esperado %d", path, resp.StatusCode, expected)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
		if err != nil || len(body) > 1024*1024 {
			return nil, errors.New("respuesta ilegible o demasiado grande")
		}
		return body, nil
	}
	for _, credential := range []string{"", "invalid-checkshared-credential"} {
		if _, err := get("/v1/employees", credential, 401); err != nil {
			return err
		}
	}
	for _, query := range []struct {
		path       string
		page, size int
	}{
		{"/v1/employees?page=1&pageSize=1", 1, 1},
		{"/v1/employees?page=2&pageSize=1", 2, 1},
	} {
		body, err := get(query.path, token, 200)
		if err != nil {
			return err
		}
		if err := validatePage(body, query.page, query.size); err != nil {
			return err
		}
	}
	if _, err := get("/v1/employees?page=0", token, 400); err != nil {
		return err
	}
	for _, path := range []string{"/v1/admin/employees", "/v1/employees/integrations"} {
		if _, err := get(path, token, 403); err != nil {
			return err
		}
	}
	if _, err := get("/v1/candidates", token, 404); err != nil {
		return err
	}
	return nil
}

func validatePage(body []byte, number, size int) error {
	var page struct {
		Items    []map[string]json.RawMessage `json:"items"`
		Page     *int                         `json:"page"`
		PageSize *int                         `json:"pageSize"`
		Total    *int64                       `json:"total"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&page)
	if err != nil || decoder.Decode(new(any)) != io.EOF || page.Items == nil || page.Page == nil || page.PageSize == nil || page.Total == nil || *page.Page != number || *page.PageSize != size || *page.Total < 0 || len(page.Items) > size {
		return errors.New("contrato de paginación inválido")
	}
	// A page is one snapshot; do not infer deletions from totals across pages.
	offset := int64(number-1) * int64(size)
	expected := *page.Total - offset
	if expected < 0 {
		expected = 0
	}
	if expected > int64(size) {
		expected = int64(size)
	}
	if int64(len(page.Items)) != expected {
		return errors.New("conteo e ítems no corresponden a la misma página")
	}
	ids := make(map[string]bool)
	for _, item := range page.Items {
		var id, name string
		if len(item) != 2 || json.Unmarshal(item["employeeId"], &id) != nil || json.Unmarshal(item["displayName"], &name) != nil || strings.TrimSpace(id) == "" || strings.TrimSpace(name) == "" || ids[id] {
			return errors.New("lista con campos no permitidos, datos inválidos o identificadores duplicados")
		}
		ids[id] = true
	}
	return nil
}
