package api

import (
	"context"
	"dash-analyzer/internal/activity"
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

	"dash-analyzer/internal/overpass"
	"dash-analyzer/pkg/mapsdata"
)

type resolverFunc func(context.Context, mapsdata.Candidate) (mapsdata.Venue, error)

func (f resolverFunc) Resolve(ctx context.Context, c mapsdata.Candidate) (mapsdata.Venue, error) {
	return f(ctx, c)
}

type mapsFetchFunc func(context.Context, mapsdata.Venue) (mapsdata.Observation, error)

func (f mapsFetchFunc) Fetch(ctx context.Context, v mapsdata.Venue) (mapsdata.Observation, error) {
	return f(ctx, v)
}
func mapsRequest(a *App, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	a.Handler("").ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w
}
func waitMaps(t *testing.T, a *App) mapsJob {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		a.mapsMu.Lock()
		job := a.mapsJob
		a.mapsMu.Unlock()
		if job.State != "running" {
			return job
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("Maps job did not finish")
	return mapsJob{}
}
func TestMapsNearbyCollectionAndCache(t *testing.T) {
	a, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.MapsInterval = time.Nanosecond
	a.Venues = []overpass.Venue{
		{ID: 1, OSMType: "node", Name: "Food", Lat: -37.81, Lon: 144.96, Type: "restaurant"},
		{ID: 2, OSMType: "way", Name: "Market", Lat: -37.82, Lon: 144.96, Type: "supermarket"},
		{ID: 4, OSMType: "node", Name: "Between five and ten km", Lat: -37.87, Lon: 144.96, Type: "restaurant"},
		{ID: 3, OSMType: "node", Name: "Far", Lat: -37.96, Lon: 144.96, Type: "cafe"},
	}
	calls, resolved := 0, 0
	a.MapsResolver = resolverFunc(func(ctx context.Context, c mapsdata.Candidate) (mapsdata.Venue, error) {
		resolved++
		return mapsdata.Venue{Key: c.Key, Name: c.Name, MapsURL: "https://www.google.com/maps/place/" + c.Name}, nil
	})
	a.MapsFetcher = mapsFetchFunc(func(ctx context.Context, v mapsdata.Venue) (mapsdata.Observation, error) {
		calls++
		return mapsdata.Observation{Title: v.Name, Busy: []mapsdata.BusyDay{{Day: "Tuesday", Labels: []string{"50% busy at 12 pm."}}}, OpeningHours: apiFixtureWeek()}, nil
	})
	query := "?lat=-37.81&lon=144.96"
	w := mapsRequest(a, "GET", "/api/maps/venues"+query)
	var data struct {
		Total  int         `json:"total"`
		Radius float64     `json:"radius_km"`
		Venues []mapsVenue `json:"venues"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &data); err != nil || w.Code != 200 || data.Total != 2 || data.Radius != 5 {
		t.Fatal(w.Body.String(), err)
	}
	// A changed selected origin must move the collection circle.
	shifted := mapsRequest(a, "GET", "/api/maps/venues?lat=-37.96&lon=144.96")
	var nearby struct {
		Venues []mapsVenue `json:"venues"`
	}
	if err = json.Unmarshal(shifted.Body.Bytes(), &nearby); err != nil || len(nearby.Venues) != 1 || nearby.Venues[0].ID != 3 {
		t.Fatal(shifted.Body.String(), err)
	}
	w = mapsRequest(a, "POST", "/api/maps/collect"+query)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	job := waitMaps(t, a)
	if job.State != "complete" || job.Completed != 2 || len(job.Results) != 2 || calls != 2 || resolved != 2 {
		t.Fatal(job, calls, resolved)
	}
	w = mapsRequest(a, "GET", "/api/maps/venues"+query)
	if err = json.Unmarshal(w.Body.Bytes(), &data); err != nil || data.Venues[0].GoogleMaps == nil || len(data.Venues[0].GoogleMaps.Observation.OpeningHours) != 7 {
		t.Fatal(w.Body.String(), err)
	}
	w = mapsRequest(a, "POST", "/api/maps/collect"+query)
	if w.Code != 202 {
		t.Fatal(w.Code)
	}
	job = waitMaps(t, a)
	if !job.Results[0].Cached || calls != 2 || resolved != 2 {
		t.Fatal(job, calls, resolved)
	}
	for _, q := range []string{"", "?lat=NaN&lon=144.96", "?lat=-37.81&lon=500", "?lat=-37.81&lat=-37.8&lon=144.96"} {
		if w := mapsRequest(a, "GET", "/api/maps/venues"+q); w.Code != 400 {
			t.Fatal(q, w.Code)
		}
	}
}
func TestMapsCancellationAndAmbiguity(t *testing.T) {
	a, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.MapsInterval = time.Nanosecond
	a.Venues = []overpass.Venue{{ID: 1, Name: "A", Lat: -37.81, Lon: 144.96}}
	entered := make(chan struct{})
	a.MapsFetcher = mapsFetchFunc(func(context.Context, mapsdata.Venue) (mapsdata.Observation, error) {
		t.Error("fetch should not run for unresolved venue")
		return mapsdata.Observation{}, nil
	})
	a.MapsResolver = resolverFunc(func(ctx context.Context, c mapsdata.Candidate) (mapsdata.Venue, error) {
		close(entered)
		<-ctx.Done()
		return mapsdata.Venue{}, ctx.Err()
	})
	query := "?lat=-37.81&lon=144.96"
	if w := mapsRequest(a, "POST", "/api/maps/collect"+query); w.Code != 202 {
		t.Fatal(w.Code)
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("resolver did not start")
	}
	if w := mapsRequest(a, "POST", "/api/maps/collect"+query); w.Code != 409 {
		t.Fatal(w.Code)
	}
	if w := mapsRequest(a, "DELETE", "/api/maps/job"); w.Code != 202 {
		t.Fatal(w.Code)
	}
	if job := waitMaps(t, a); job.State != "cancelled" {
		t.Fatal(job)
	}
	a.MapsResolver = resolverFunc(func(context.Context, mapsdata.Candidate) (mapsdata.Venue, error) {
		return mapsdata.Venue{}, errors.New("ambiguous match")
	})
	mapsRequest(a, "POST", "/api/maps/collect"+query)
	job := waitMaps(t, a)
	if job.Completed != 1 || !strings.Contains(job.Results[0].Error, "ambiguous") {
		t.Fatal(job)
	}
}

func apiFixtureWeek() []mapsdata.Hours {
	var hours []mapsdata.Hours
	for _, day := range []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"} {
		hours = append(hours, mapsdata.Hours{Day: day, Text: "9 am–5 pm"})
	}
	return hours
}

func TestCollectedHoursAffectAPIActivityAndRanking(t *testing.T) {
	a, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.Venues = []overpass.Venue{{ID: 7, OSMType: "node", Name: "Pelligra", Type: "cafe", Lat: -37.81, Lon: 144.96, OpeningHours: "24/7"}}
	ws := testhttp.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer ws.Close()
	a.Weather = &weather.Client{HTTP: ws.Client(), URL: ws.URL}
	mapped := mapsdata.Venue{Key: "node/7", Name: "Pelligra Cakes", MapsURL: "https://www.google.com/maps/place/Pelligra+Cakes"}
	fetch := mapsFetchFunc(func(context.Context, mapsdata.Venue) (mapsdata.Observation, error) {
		var hours []mapsdata.Hours
		for day := time.Sunday; day <= time.Saturday; day++ {
			text := "7 am–4 pm"
			if day == time.Sunday {
				text = "Closed"
			}
			hours = append(hours, mapsdata.Hours{Day: day.String(), Text: text})
		}
		return mapsdata.Observation{Title: mapped.Name, OpeningHours: hours}, nil
	})
	results, err := (mapsdata.Collector{Store: a.MapsStore, Fetcher: fetch}).Collect(context.Background(), []mapsdata.Venue{mapped})
	if err != nil || len(results) != 1 || !results[0].HoursComplete || results[0].BusyCollected {
		t.Fatal(results, err)
	}
	for _, tc := range []struct {
		date, hour, status string
		open               bool
	}{{"2026-11-10", "12", "confirmed_open", true}, {"2026-11-10", "16", "confirmed_closed", false}, {"2026-11-15", "12", "confirmed_closed", false}} {
		for _, endpoint := range []string{"zones", "recommendations"} {
			response := mapsRequest(a, "GET", "/api/"+endpoint+"?date="+tc.date+"&hour="+tc.hour)
			var data struct {
				Venues []struct {
					Activity activity.Estimate `json:"activity"`
				} `json:"venues"`
				Zones []scoring.Hotspot `json:"zones"`
			}
			if err = json.Unmarshal(response.Body.Bytes(), &data); err != nil || response.Code != 200 || len(data.Venues) != 1 || len(data.Zones) != 1 {
				t.Fatal(response.Body.String(), err)
			}
			estimate := data.Venues[0].Activity
			if estimate.Status != tc.status || estimate.HoursSource != "google_maps" || estimate.Confidence != "medium" {
				t.Fatal(tc, estimate)
			}
			if tc.open && data.Zones[0].Score <= 0 || !tc.open && (data.Zones[0].Score != 0 || estimate.Factor != 0) {
				t.Fatal(tc, data)
			}
		}
	}
	if a.Venues[0].GoogleHours != nil || a.Venues[0].OpeningHours != "24/7" {
		t.Fatal("source cache mutated")
	}
}
