package wire

import (
	"encoding/json"
	"github.com/Gerpeirone/Car-Cruds/services/rentals/application"
	"github.com/Gerpeirone/Car-Cruds/services/rentals/domain"
)

type Reservation struct {
	ID             string        `json:"id"`
	Status         domain.Status `json:"status"`
	VehicleID      string        `json:"vehicleId"`
	CustomerID     string        `json:"customerId"`
	StartsOn       string        `json:"startsOn"`
	EndsOn         string        `json:"endsOn"`
	Days           int           `json:"days"`
	DailyRateMinor int64         `json:"dailyRateMinor"`
	TotalMinor     int64         `json:"totalMinor"`
	Currency       string        `json:"currency"`
}
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func FromDomain(r domain.Reservation) Reservation {
	return Reservation{r.ID, r.Status, r.VehicleID, r.CustomerID, r.Period.StartsOn, r.Period.EndsOn, r.Period.Days, r.DailyRateMinor, r.TotalMinor, r.Currency}
}
func (r Reservation) Domain(app string) domain.Reservation {
	return domain.Reservation{ID: r.ID, Status: r.Status, VehicleID: r.VehicleID, CustomerID: r.CustomerID, OwnerApp: app, Period: domain.Period{StartsOn: r.StartsOn, EndsOn: r.EndsOn, Days: r.Days}, DailyRateMinor: r.DailyRateMinor, TotalMinor: r.TotalMinor, Currency: r.Currency}
}
func Status(code string) int {
	switch code {
	case "INVALID_INPUT":
		return 400
	case "FORBIDDEN":
		return 403
	case "NOT_FOUND":
		return 404
	case "PRICE_CHANGED", "VEHICLE_UNAVAILABLE", "IDEMPOTENCY_KEY_REUSED", "INVALID_STATE":
		return 409
	case "CUSTOMER_NOT_ELIGIBLE":
		return 422
	default:
		return 503
	}
}
func Encode(result application.Result) (int, []byte, error) {
	if result.Problem != nil {
		b, e := json.Marshal(Error{result.Problem.Code, result.Problem.Message})
		return Status(result.Problem.Code), b, e
	}
	b, e := json.Marshal(FromDomain(*result.Reservation))
	return 201, b, e
}
func Decode(status int, b []byte, app string) (application.Result, error) {
	if status == 201 {
		var r Reservation
		if e := json.Unmarshal(b, &r); e != nil {
			return application.Result{}, e
		}
		v := r.Domain(app)
		return application.Result{Reservation: &v}, nil
	}
	var p Error
	if e := json.Unmarshal(b, &p); e != nil {
		return application.Result{}, e
	}
	return application.Result{Problem: domain.Fail(p.Code, p.Message)}, nil
}
