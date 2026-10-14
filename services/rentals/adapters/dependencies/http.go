package dependencies

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Gerpeirone/Car-Cruds/services/rentals/domain"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HTTP struct {
	Client                        *http.Client
	CustomersURL, FleetURL, Token string
}

func (h *HTTP) read(ctx context.Context, base, path string, out any) error {
	callCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, http.MethodGet, strings.TrimRight(base, "/")+path, nil)
	if err != nil {
		return domain.Fail("DEPENDENCY_UNAVAILABLE", "Configuración de dependencia inválida.")
	}
	req.Header.Set("Authorization", "Bearer "+h.Token)
	response, err := h.Client.Do(req)
	if err != nil {
		return domain.Fail("DEPENDENCY_UNAVAILABLE", "Dependencia fuera de servicio o sin respuesta.")
	}
	defer response.Body.Close()
	if response.StatusCode == 404 {
		return domain.Fail("NOT_FOUND", "Cliente o vehículo inexistente.")
	}
	if response.StatusCode != 200 {
		return domain.Fail("DEPENDENCY_UNAVAILABLE", "La dependencia no completó la validación.")
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1024*1024))
	if err = decoder.Decode(out); err != nil {
		return domain.Fail("DEPENDENCY_UNAVAILABLE", "Respuesta de dependencia inválida.")
	}
	return nil
}
func (h *HTTP) Get(ctx context.Context, id string) (domain.Customer, error) {
	var value struct {
		ID      string `json:"id"`
		Active  *bool  `json:"active"`
		License string `json:"licenseValidUntil"`
	}
	err := h.read(ctx, h.CustomersURL, "/v1/customers/"+url.PathEscape(id), &value)
	if err != nil {
		return domain.Customer{}, err
	}
	if value.ID != id || value.Active == nil || value.License == "" {
		return domain.Customer{}, domain.Fail("DEPENDENCY_UNAVAILABLE", "Perfil de cliente incompleto.")
	}
	if _, err = time.Parse("2006-01-02", value.License); err != nil {
		return domain.Customer{}, domain.Fail("DEPENDENCY_UNAVAILABLE", "Licencia de dependencia inválida.")
	}
	return domain.Customer{ID: value.ID, Active: *value.Active, LicenseValidUntil: value.License}, nil
}
func (h *HTTP) Quote(ctx context.Context, id string) (domain.Quote, error) {
	var value struct {
		VehicleID      string `json:"vehicleId"`
		Active         *bool  `json:"active"`
		DailyRateMinor int64  `json:"dailyRateMinor"`
		Currency       string `json:"currency"`
	}
	err := h.read(ctx, h.FleetURL, fmt.Sprintf("/v1/vehicles/%s/quote", url.PathEscape(id)), &value)
	if err != nil {
		return domain.Quote{}, err
	}
	if value.VehicleID != id || value.Active == nil {
		return domain.Quote{}, domain.Fail("DEPENDENCY_UNAVAILABLE", "Cotización incompleta.")
	}
	return domain.Quote{VehicleID: value.VehicleID, Active: *value.Active, DailyRateMinor: value.DailyRateMinor, Currency: value.Currency}, nil
}
