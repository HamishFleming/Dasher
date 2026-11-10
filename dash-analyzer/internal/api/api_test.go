package api

import (
	"context"
	"dash-analyzer/internal/activity"
	"dash-analyzer/internal/geo"
	"dash-analyzer/internal/history"
	"dash-analyzer/internal/overpass"
	"dash-analyzer/internal/scoring"
	"dash-analyzer/internal/testhttp"
	"dash-analyzer/internal/weather"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAPIIntegration(t *testing.T) {
	dir := t.TempDir()
	a, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Store.DB.Close()
	ws := testhttp.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer ws.Close()
	a.Weather = &weather.Client{HTTP: ws.Client(), URL: ws.URL}
	a.Fetch = func(context.Context, geo.Bounds) ([]overpass.Venue, error) {
		return []overpass.Venue{{ID: 1, Name: "Test", Lat: -37.81, Lon: 144.96, Type: "fast_food"}}, nil
	}
	if err = a.Refresh(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	h := a.Handler(t.TempDir())
	request := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		return w
	}
	body := `{"started_at":"2026-11-10T01:00:00Z","ended_at":"2026-11-10T02:00:00Z","gross_aud":40,"distance_km":20,"fuel_cost_aud":4,"other_cost_aud":2}`
	if w := request("POST", "/api/deliveries", body); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	w := request("GET", "/api/analytics", "")
	var stats history.Analytics
	if err = json.Unmarshal(w.Body.Bytes(), &stats); err != nil || stats.Net != 34 || stats.PerHour != 34 || stats.PerKM != 1.7 {
		t.Fatal(stats, err)
	}
	for _, path := range []string{"/api/zones", "/api/recommendations", "/api/weather", "/api/health", "/api/deliveries"} {
		if w := request("GET", path, ""); w.Code != 200 {
			t.Fatal(path, w.Code)
		}
	}

	w = request("GET", "/api/zones?day=1&hour=12", "")
	var selection map[string]json.RawMessage
	if err = json.Unmarshal(w.Body.Bytes(), &selection); err != nil || w.Code != 200 {
		t.Fatal(w.Body.String(), err)
	}
	var selected time.Time
	if err = json.Unmarshal(selection["selected_local_datetime"], &selected); err != nil || selected.Weekday() != time.Monday || selected.Hour() != 12 {
		t.Fatal(selected, err)
	}
	var zones []scoring.Hotspot
	if err = json.Unmarshal(selection["zones"], &zones); err != nil || len(zones) != 1 || zones[0].ActivityScore <= 0 || zones[0].UnknownHours != 1 {
		t.Fatal(zones, err)
	}
	var venues []map[string]json.RawMessage
	if err = json.Unmarshal(selection["venues"], &venues); err != nil || len(venues) != 1 {
		t.Fatal(venues, err)
	}
	var estimate activity.Estimate
	if err = json.Unmarshal(venues[0]["activity"], &estimate); err != nil || estimate.Status != "unknown" || estimate.Confidence != "low" {
		t.Fatal(estimate, err)
	}
	for _, path := range []string{"/api/zones?lat=NaN", "/api/zones?parking=2", "/api/zones?day=7", "/api/zones?hour=24", "/api/recommendations?lon=500"} {
		if w := request("GET", path, ""); w.Code != 400 {
			t.Fatal(path, w.Code)
		}
	}
	for _, b := range []string{`{}`, body + ` {}`, strings.Replace(body, `"distance_km":20`, `"distance_km":-1`, 1)} {
		if w := request("POST", "/api/deliveries", b); w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	a.Venues = append(a.Venues, overpass.Venue{ID: 2, Type: "supermarket", Lat: -37.81, Lon: 144.96})
	for _, tc := range []struct {
		query string
		count int
	}{{"", 1}, {"?include_grocery=false", 1}, {"?include_grocery=true", 2}} {
		w := request("GET", "/api/zones"+tc.query, "")
		var data struct {
			Total  int               `json:"total_venues"`
			Venues []overpass.Venue  `json:"venues"`
			Zones  []scoring.Hotspot `json:"zones"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil || data.Total != tc.count || len(data.Venues) != tc.count || len(data.Zones) != 1 || data.Zones[0].Venues != tc.count {
			t.Fatal(w.Body.String(), err)
		}
	}
	for _, query := range []string{"?include_grocery=1", "?include_grocery=", "?include_grocery=true&include_grocery=false"} {
		if w := request("GET", "/api/recommendations"+query, ""); w.Code != 400 {
			t.Fatal(w.Code)
		}
	}
	a.Venues = append(a.Venues, overpass.Venue{ID: 3, Type: "restaurant", Lat: -37.9, Lon: 145.1})
	for _, path := range []string{"/api/zones", "/api/recommendations"} {
		w := request("GET", path+"?lat=-37.81&lon=144.96&radius_km=1&include_grocery=true&ignore_distance=true", "")
		var data struct {
			Total  int               `json:"total_venues"`
			Venues []overpass.Venue  `json:"venues"`
			Zones  []scoring.Hotspot `json:"zones"`
			Radius float64           `json:"radius_km"`
			Ignore bool              `json:"ignore_distance"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil || w.Code != 200 || data.Total != 2 || len(data.Venues) != 2 || len(data.Zones) != 1 || data.Zones[0].Venues != 2 || data.Radius != 1 || !data.Ignore {
			t.Fatal(w.Body.String(), err)
		}
		w = request("GET", path+"?lat=-37.6&lon=145.3&radius_km=0.1", "")
		if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil || data.Total != 0 || len(data.Zones) != 0 {
			t.Fatal(w.Body.String(), err)
		}
	}
	for _, query := range []string{"radius_km=-1", "radius_km=101", "radius_km=NaN", "radius_km=", "radius_km=1&radius_km=2", "ignore_distance=1", "ignore_distance=", "ignore_distance=true&ignore_distance=false"} {
		if w := request("GET", "/api/zones?"+query, ""); w.Code != 400 {
			t.Fatal(query, w.Code)
		}
	}
	a.Venues = a.Venues[:1]
	a.Updated = time.Now().Add(-48 * time.Hour)
	a.Fetch = func(context.Context, geo.Bounds) ([]overpass.Venue, error) { return nil, errors.New("offline") }
	if err = a.Refresh(context.Background(), true); err == nil || len(a.Venues) != 1 {
		t.Fatal("failed refresh destroyed cache")
	}
	b, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Store.DB.Close()
	ds, err := b.Store.List()
	if err != nil || len(ds) != 1 || len(b.Venues) != 1 {
		t.Fatal("persistence", ds, err)
	}
}

func TestSelectedTime(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, activity.Melbourne)
	for _, tc := range []struct {
		query        string
		valid        bool
		hour, offset int
	}{
		{"", true, 12, 10 * 3600},
		{"?day=0&hour=3", true, 3, 11 * 3600},
		{"?day=0&hour=2", false, 0, 0},
		{"?day=7", false, 0, 0}, {"?day=-1", false, 0, 0},
		{"?day=Monday", false, 0, 0}, {"?hour=24", false, 0, 0},
		{"?hour=2.5", false, 0, 0}, {"?hour=", false, 0, 0},
		{"?day=1&day=2", false, 0, 0},
	} {
		got, err := selectedTime(httptest.NewRequest("GET", "/api/zones"+tc.query, nil), now)
		if (err == nil) != tc.valid {
			t.Fatal(tc, got, err)
		}
		if err == nil {
			_, offset := got.Zone()
			if got.Hour() != tc.hour || offset != tc.offset {
				t.Fatal(tc, got)
			}
		}
	}
	// Fall-back Sunday has a repeated 02:00; it remains a valid local hour.
	fall := time.Date(2026, 4, 4, 12, 0, 0, 0, activity.Melbourne)
	got, err := selectedTime(httptest.NewRequest("GET", "/api/zones?day=0&hour=2", nil), fall)
	if err != nil || got.Hour() != 2 || got.Weekday() != time.Sunday {
		t.Fatal(got, err)
	}
}

func TestSelectedCalendarDate(t *testing.T) {
	now := time.Date(2026, 11, 10, 18, 30, 0, 0, activity.Melbourne)
	for _, tc := range []struct {
		query        string
		valid        bool
		date         string
		hour, offset int
	}{
		{"?date=2026-07-06&hour=19", true, "2026-07-06", 19, 10 * 3600},
		{"?date=2026-12-25&hour=12", true, "2026-12-25", 12, 11 * 3600},
		{"?date=2026-12-25", true, "2026-12-25", 18, 11 * 3600},
		{"?date=2026-10-04&hour=2", false, "", 0, 0},
		{"?date=2026-10-04&hour=3", true, "2026-10-04", 3, 11 * 3600},
		{"?date=2026-02-30", false, "", 0, 0},
		{"?date=", false, "", 0, 0},
		{"?date=2026-11-10&date=2026-11-11", false, "", 0, 0},
		{"?date=2026-11-10&day=2", false, "", 0, 0},
	} {
		got, err := selectedTime(httptest.NewRequest("GET", "/api/zones"+tc.query, nil), now)
		if (err == nil) != tc.valid {
			t.Fatal(tc, got, err)
		}
		if tc.valid {
			_, offset := got.Zone()
			if got.Format("2006-01-02") != tc.date || got.Hour() != tc.hour || offset != tc.offset {
				t.Fatal(tc, got)
			}
		}
	}
}
