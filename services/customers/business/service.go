package business

import (
	"context"
	"regexp"
	"strings"
	"time"
)

type Customer struct {
	ID, Name          string
	Active            bool
	LicenseValidUntil string
}
type Problem struct{ Code, Message string }

func (p *Problem) Error() string { return p.Code + ": " + p.Message }

type Repository interface {
	Get(context.Context, string) (Customer, error)
	List(context.Context) ([]Customer, error)
	Create(context.Context, Customer) error
}
type Service struct{ Repo Repository }

var Identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

func (s *Service) Create(ctx context.Context, c Customer) (Customer, error) {
	c.Name = strings.TrimSpace(c.Name)
	date, err := time.Parse("2006-01-02", c.LicenseValidUntil)
	if !Identifier.MatchString(c.ID) || len(c.Name) < 1 || len(c.Name) > 200 || err != nil || date.Format("2006-01-02") != c.LicenseValidUntil {
		return Customer{}, &Problem{"INVALID_INPUT", "Identificador, nombre y fecha de licencia válidos requeridos."}
	}
	if err = s.Repo.Create(ctx, c); err != nil {
		return Customer{}, err
	}
	return c, nil
}
func (s *Service) Get(ctx context.Context, id string) (Customer, error) {
	if !Identifier.MatchString(id) {
		return Customer{}, &Problem{"INVALID_INPUT", "Identificador inválido."}
	}
	return s.Repo.Get(ctx, id)
}
func (s *Service) List(ctx context.Context) ([]Customer, error) { return s.Repo.List(ctx) }
