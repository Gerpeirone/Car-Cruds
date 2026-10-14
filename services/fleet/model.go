package fleet

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	ErrNotFound          = errors.New("vehicle not found")
	ErrConflict          = errors.New("vehicle version conflict")
	ErrOutboxFull        = errors.New("vehicle outbox full")
	ErrBusy              = errors.New("source capacity exhausted")
	ErrInvalid           = errors.New("invalid vehicle")
	ErrSearchUnavailable = errors.New("search unavailable")
)

const MaxPendingEvents = 128

type Vehicle struct {
	VehicleID      string    `json:"vehicleId" bson:"_id"`
	Brand          string    `json:"brand" bson:"brand"`
	Model          string    `json:"model" bson:"model"`
	Category       string    `json:"category" bson:"category"`
	Transmission   string    `json:"transmission" bson:"transmission"`
	Seats          int       `json:"seats" bson:"seats"`
	DailyRateMinor int64     `json:"dailyRateMinor" bson:"dailyRateMinor"`
	Currency       string    `json:"currency" bson:"currency"`
	Active         bool      `json:"active" bson:"active"`
	Version        int64     `json:"version" bson:"version"`
	UpdatedAt      time.Time `json:"updatedAt" bson:"updatedAt"`
}

type Input struct {
	VehicleID      string `json:"vehicleId"`
	Brand          string `json:"brand"`
	Model          string `json:"model"`
	Category       string `json:"category"`
	Transmission   string `json:"transmission"`
	Seats          int    `json:"seats"`
	DailyRateMinor int64  `json:"dailyRateMinor"`
	Currency       string `json:"currency"`
	Active         *bool  `json:"active"`
	Version        int64  `json:"version"`
}

type Query struct {
	Page            int
	PageSize        int
	Brand           string
	Category        string
	Transmission    string
	Seats           int
	Sort            string
	Text            string
	IncludeInactive bool
}

type Page struct {
	Items    []Vehicle `json:"items"`
	Page     int       `json:"page"`
	PageSize int       `json:"pageSize"`
	Total    int64     `json:"total"`
}

type Quote struct {
	VehicleID      string `json:"vehicleId"`
	Active         bool   `json:"active"`
	DailyRateMinor int64  `json:"dailyRateMinor"`
	Currency       string `json:"currency"`
	Version        int64  `json:"version"`
}

type Event struct {
	EventID    string    `json:"eventId" bson:"eventId"`
	Type       string    `json:"type" bson:"type"`
	VehicleID  string    `json:"vehicleId" bson:"vehicleId"`
	Version    int64     `json:"version" bson:"version"`
	OccurredAt time.Time `json:"occurredAt" bson:"occurredAt"`
}

type Repository interface {
	Get(context.Context, string) (Vehicle, error)
	List(context.Context, Query) (Page, error)
	Create(context.Context, Vehicle, Event) (Vehicle, error)
	Update(context.Context, Vehicle, int64, Event) (Vehicle, error)
}
type OutboxStore interface {
	Pending(context.Context, int) ([]Event, error)
	MarkPublished(context.Context, Event) error
	EnqueueRebuild(context.Context, string, Event) error
	Scan(context.Context, func(Vehicle) error) error
}
type Cache interface {
	Get(context.Context, string) (Vehicle, bool, error)
	Put(context.Context, Vehicle) error
	Invalidate(context.Context, string, int64) error
}
type SearchEngine interface {
	Search(context.Context, Query) (Page, error)
	Upsert(context.Context, Vehicle) error
	Ready(context.Context) error
}

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

func ValidateID(id string) error {
	if !idPattern.MatchString(id) {
		return fmt.Errorf("%w: vehicleId inválido", ErrInvalid)
	}
	return nil
}
func (in Input) vehicle(id string, version int64, active bool) (Vehicle, error) {
	v := Vehicle{VehicleID: id, Brand: strings.TrimSpace(in.Brand), Model: strings.TrimSpace(in.Model), Category: strings.TrimSpace(in.Category), Transmission: in.Transmission, Seats: in.Seats, DailyRateMinor: in.DailyRateMinor, Currency: in.Currency, Active: active, Version: version, UpdatedAt: time.Now().UTC()}
	if in.Active != nil {
		v.Active = *in.Active
	}
	if err := ValidateID(id); err != nil {
		return Vehicle{}, err
	}
	if in.VehicleID != "" && in.VehicleID != id {
		return Vehicle{}, fmt.Errorf("%w: vehicleId es inmutable", ErrInvalid)
	}
	if len(v.Brand) == 0 || len(v.Brand) > 128 || len(v.Model) == 0 || len(v.Model) > 128 || len(v.Category) == 0 || len(v.Category) > 64 {
		return Vehicle{}, fmt.Errorf("%w: marca, modelo y categoría requeridos", ErrInvalid)
	}
	if v.Transmission != "manual" && v.Transmission != "automatic" {
		return Vehicle{}, fmt.Errorf("%w: transmisión manual o automatic", ErrInvalid)
	}
	if v.Seats < 1 || v.Seats > 9 || v.DailyRateMinor < 1 || v.DailyRateMinor > 1000000000 || v.Currency != "ARS" {
		return Vehicle{}, fmt.Errorf("%w: plazas, tarifa o moneda inválidos", ErrInvalid)
	}
	return v, nil
}
func (q Query) Validate() error {
	if q.Page < 1 || q.Page > 100000 || q.PageSize < 1 || q.PageSize > 100 || q.Seats < 0 || q.Seats > 9 || len(q.Text) > 256 || len(q.Brand) > 128 || len(q.Category) > 64 {
		return ErrInvalid
	}
	if q.Sort != "" && q.Sort != "price_asc" && q.Sort != "price_desc" {
		return ErrInvalid
	}
	if q.Transmission != "" && q.Transmission != "manual" && q.Transmission != "automatic" {
		return ErrInvalid
	}
	return nil
}
