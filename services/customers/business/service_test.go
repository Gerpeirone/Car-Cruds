package business

import (
	"context"
	"testing"
)

type recorder struct{ items map[string]Customer }

func (r *recorder) Create(_ context.Context, c Customer) error         { r.items[c.ID] = c; return nil }
func (r *recorder) Get(_ context.Context, id string) (Customer, error) { return r.items[id], nil }
func (r *recorder) List(context.Context) ([]Customer, error)           { return nil, nil }
func TestCustomerValidationBeforePersistence(t *testing.T) {
	r := &recorder{items: map[string]Customer{}}
	s := &Service{Repo: r}
	for _, c := range []Customer{{ID: "bad/id", Name: "Cliente", LicenseValidUntil: "2030-12-31"}, {ID: "cli", Name: " ", LicenseValidUntil: "2030-12-31"}, {ID: "cli", Name: "Cliente", LicenseValidUntil: "2030-02-30"}} {
		if _, err := s.Create(context.Background(), c); err == nil {
			t.Fatal("invalid profile accepted")
		}
	}
	if len(r.items) != 0 {
		t.Fatal("invalid profiles reached persistence")
	}
	c, err := s.Create(context.Background(), Customer{ID: "cli", Name: " Cliente ", Active: true, LicenseValidUntil: "2030-12-31"})
	if err != nil || c.Name != "Cliente" || !r.items["cli"].Active {
		t.Fatal(c, err)
	}
}
