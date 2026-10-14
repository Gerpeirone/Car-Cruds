package fleet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Solr struct {
	base   string
	client *http.Client
}

func NewSolr(base string, client *http.Client) *Solr {
	return &Solr{base: strings.TrimRight(base, "/"), client: client}
}

type solrDocument struct {
	ID             string    `json:"id"`
	Brand          string    `json:"brand_s"`
	Model          string    `json:"model_s"`
	Category       string    `json:"category_s"`
	Transmission   string    `json:"transmission_s"`
	Seats          int       `json:"seats_i"`
	DailyRateMinor int64     `json:"dailyRateMinor_l"`
	Currency       string    `json:"currency_s"`
	Active         bool      `json:"active_b"`
	VehicleVersion int64     `json:"vehicleVersion_l"`
	UpdatedAt      time.Time `json:"updatedAt_dt"`
	SearchText     string    `json:"searchText_t,omitempty"`
	SolrVersion    int64     `json:"_version_"`
}

func solrVehicle(d solrDocument) Vehicle {
	return Vehicle{VehicleID: d.ID, Brand: d.Brand, Model: d.Model, Category: d.Category, Transmission: d.Transmission, Seats: d.Seats, DailyRateMinor: d.DailyRateMinor, Currency: d.Currency, Active: d.Active, Version: d.VehicleVersion, UpdatedAt: d.UpdatedAt}
}
func toSolr(v Vehicle, expected int64) solrDocument {
	return solrDocument{ID: v.VehicleID, Brand: v.Brand, Model: v.Model, Category: v.Category, Transmission: v.Transmission, Seats: v.Seats, DailyRateMinor: v.DailyRateMinor, Currency: v.Currency, Active: v.Active, VehicleVersion: v.Version, UpdatedAt: v.UpdatedAt, SearchText: v.Brand + " " + v.Model + " " + v.Category + " " + v.Transmission, SolrVersion: expected}
}
func (s *Solr) request(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, method, s.base+path, bytes.NewReader(body))
	if err != nil {
		return nil, ErrSearchUnavailable
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return nil, ErrSearchUnavailable
	}
	return response, nil
}
func (s *Solr) Ready(ctx context.Context) error {
	response, err := s.request(ctx, http.MethodGet, "/admin/ping?wt=json", nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return ErrSearchUnavailable
	}
	return nil
}
func (s *Solr) Search(ctx context.Context, q Query) (Page, error) {
	if err := q.Validate(); err != nil {
		return Page{}, err
	}
	values := url.Values{"q": {"*:*"}, "wt": {"json"}, "start": {strconv.Itoa((q.Page - 1) * q.PageSize)}, "rows": {strconv.Itoa(q.PageSize)}}
	if q.Text != "" {
		values.Set("q", escapeLucene(q.Text))
		values.Set("defType", "edismax")
		values.Set("qf", "searchText_t")
		values.Set("q.op", "AND")
	}
	values.Add("fq", "active_b:true")
	if q.Brand != "" {
		values.Add("fq", "{!term f=brand_s}"+q.Brand)
	}
	if q.Category != "" {
		values.Add("fq", "{!term f=category_s}"+q.Category)
	}
	if q.Transmission != "" {
		values.Add("fq", "{!term f=transmission_s}"+q.Transmission)
	}
	if q.Seats > 0 {
		values.Add("fq", "seats_i:"+strconv.Itoa(q.Seats))
	}
	direction := "asc"
	if q.Sort == "price_desc" {
		direction = "desc"
	}
	values.Set("sort", "dailyRateMinor_l "+direction+",id asc")
	response, err := s.request(ctx, http.MethodGet, "/select?"+values.Encode(), nil)
	if err != nil {
		return Page{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return Page{}, ErrSearchUnavailable
	}
	var decoded struct {
		Response *struct {
			NumFound *int64         `json:"numFound"`
			Docs     []solrDocument `json:"docs"`
		} `json:"response"`
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&decoded); err != nil {
		return Page{}, ErrSearchUnavailable
	}
	if decoded.Response == nil || decoded.Response.NumFound == nil || decoded.Response.Docs == nil || *decoded.Response.NumFound < 0 || len(decoded.Response.Docs) > q.PageSize {
		return Page{}, ErrSearchUnavailable
	}
	out := Page{Items: make([]Vehicle, 0, len(decoded.Response.Docs)), Page: q.Page, PageSize: q.PageSize, Total: *decoded.Response.NumFound}
	for _, doc := range decoded.Response.Docs {
		if ValidateID(doc.ID) != nil || doc.VehicleVersion < 1 || doc.Currency != "ARS" || doc.DailyRateMinor < 1 {
			return Page{}, ErrSearchUnavailable
		}
		out.Items = append(out.Items, solrVehicle(doc))
	}
	return out, nil
}
func escapeLucene(text string) string {
	var b strings.Builder
	for _, r := range text {
		if strings.ContainsRune(`+-!(){}[]^"~*?:\\/|&`, r) {
			b.WriteRune('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// RealTimeGet and Solr CAS prevent a delayed event or rebuild from overwriting a newer source version.
func (s *Solr) Upsert(ctx context.Context, v Vehicle) error {
	for attempt := 0; attempt < 4; attempt++ {
		current, exists, err := s.get(ctx, v.VehicleID)
		if err != nil {
			return err
		}
		expected := int64(-1)
		if exists {
			if current.VehicleVersion >= v.Version {
				return nil
			}
			expected = current.SolrVersion
			if expected <= 1 {
				return fmt.Errorf("%w: missing optimistic version", ErrSearchUnavailable)
			}
		}
		raw, err := json.Marshal([]solrDocument{toSolr(v, expected)})
		if err != nil {
			return err
		}
		response, err := s.request(ctx, http.MethodPost, "/update?commitWithin=1000&wt=json", raw)
		if err != nil {
			return err
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		response.Body.Close()
		if response.StatusCode == 409 {
			continue
		}
		if response.StatusCode != 200 {
			return ErrSearchUnavailable
		}
		return nil
	}
	return ErrSearchUnavailable
}
func (s *Solr) get(ctx context.Context, id string) (solrDocument, bool, error) {
	values := url.Values{"id": {id}, "wt": {"json"}}
	response, err := s.request(ctx, http.MethodGet, "/get?"+values.Encode(), nil)
	if err != nil {
		return solrDocument{}, false, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return solrDocument{}, false, ErrSearchUnavailable
	}
	var envelope map[string]json.RawMessage
	if err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&envelope); err != nil {
		return solrDocument{}, false, ErrSearchUnavailable
	}
	raw, present := envelope["doc"]
	if !present {
		return solrDocument{}, false, ErrSearchUnavailable
	}
	var doc *solrDocument
	if err = json.Unmarshal(raw, &doc); err != nil {
		return solrDocument{}, false, ErrSearchUnavailable
	}
	if doc == nil {
		return solrDocument{}, false, nil
	}
	if doc.ID != id || doc.VehicleVersion < 1 || doc.SolrVersion <= 1 {
		return solrDocument{}, false, ErrSearchUnavailable
	}
	return *doc, true, nil
}
