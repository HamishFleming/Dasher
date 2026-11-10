package mapsdata

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dash-analyzer/internal/testhttp"
)

type fetchFunc func(context.Context, Venue) (Observation, error)

func (f fetchFunc) Fetch(ctx context.Context, v Venue) (Observation, error) { return f(ctx, v) }
func testVenue() Venue {
	return Venue{Key: "node/123", Name: "Fixture Market", MapsURL: "https://www.google.com/maps/place/Fixture+Market/"}
}
func TestCollectionAndPartialCache(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "deliveries.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	observed := time.Date(2026, 11, 10, 1, 0, 0, 0, time.UTC)
	// An in-process fixture transport, with no public calls or listening socket.
	server := testhttp.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(Observation{Title: "Fixture Market", SourceURL: testVenue().MapsURL, FetchedAt: observed, Busy: []BusyDay{{Day: "Tuesday", Labels: []string{"50% busy at 12 pm."}}}, OpeningHours: fixtureWeek()})
	}))
	defer server.Close()
	fetch := fetchFunc(func(ctx context.Context, v Venue) (Observation, error) {
		req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
		resp, err := server.Client().Do(req)
		if err != nil {
			return Observation{}, err
		}
		defer resp.Body.Close()
		var o Observation
		err = json.NewDecoder(resp.Body).Decode(&o)
		return o, err
	})
	collector := Collector{Store: store, Fetcher: fetch}
	results, err := collector.Collect(ctx, []Venue{testVenue()})
	if err != nil || len(results) != 1 || !results[0].BusyCollected || !results[0].HoursCollected || results[0].Error != "" {
		t.Fatal(results, err)
	}
	cached, err := store.Get(ctx, testVenue().Key)
	if err != nil || len(cached.Observation.Busy) != 1 || len(cached.Observation.OpeningHours) != 7 || !cached.BusyFetchedAt.Equal(observed) {
		t.Fatal(cached, err)
	}
	// Missing busy times must not erase the previous successful observation.
	collector.Fetcher = fetchFunc(func(context.Context, Venue) (Observation, error) {
		return Observation{Title: testVenue().Name, AvailabilityNote: "Google limited view without busy chart", FetchedAt: observed.Add(time.Hour), OpeningHours: []Hours{{Day: "Tuesday", Text: "Closed"}}}, nil
	})
	results, err = collector.Collect(ctx, []Venue{testVenue()})
	if err != nil || results[0].BusyCollected || !results[0].HoursCollected || !strings.Contains(results[0].Error, "Google limited view") {
		t.Fatal(results, err)
	}
	cached, err = store.Get(ctx, testVenue().Key)
	if err != nil || hoursText(cached.Observation.OpeningHours, "Tuesday") != "Closed" || cached.Observation.Busy[0].Labels[0] != "50% busy at 12 pm." || !cached.BusyFetchedAt.Equal(observed) || !cached.HoursFetchedAt.Equal(observed.Add(time.Hour)) {
		t.Fatal(cached, err)
	}
	collector.Fetcher = fetchFunc(func(context.Context, Venue) (Observation, error) { return Observation{}, errors.New("blocked") })
	results, err = collector.Collect(ctx, []Venue{testVenue()})
	if err != nil || results[0].Error != "blocked" {
		t.Fatal(results, err)
	}
	// Reopening the file proves persistence and failure preservation.
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	cached, err = reopened.Get(ctx, testVenue().Key)
	if err != nil || cached.LastError != "blocked" || len(cached.Observation.Busy) != 1 || hoursText(cached.Observation.OpeningHours, "Tuesday") != "Closed" {
		t.Fatal(cached, err)
	}
	// Explicit mappings must never relabel previously cached places.
	changed := testVenue()
	changed.MapsURL += "other"
	if _, err = collector.Collect(ctx, []Venue{changed}); err == nil {
		t.Fatal("mapping silently changed")
	}
}
func TestValidationAndCancellation(t *testing.T) {
	for _, raw := range []string{"http://www.google.com/maps/place/Test", "https://evil.example/maps/place/Test", "https://www.google.com/maps/search/Test", "https://www.google.com:444/maps/place/Test", "https://user@www.google.com/maps/place/Test"} {
		v := testVenue()
		v.MapsURL = raw
		if v.Validate() == nil {
			t.Fatal(raw)
		}
	}
	v := testVenue()
	for _, o := range []Observation{{Title: "Wrong venue", OpeningHours: []Hours{{Day: "Monday", Text: "Closed"}}}, {Title: v.Name}, {Title: v.Name, OpeningHours: []Hours{{Day: "Mo", Text: "Closed"}}}, {Title: v.Name, Busy: []BusyDay{{Day: "Tuesday"}}}} {
		o.SourceURL = v.MapsURL
		if validateObservation(v, o) == nil {
			t.Fatal(o)
		}
	}
	store, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	calls := 0
	collector := Collector{Store: store, Fetcher: fetchFunc(func(context.Context, Venue) (Observation, error) { calls++; return Observation{}, nil })}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = collector.Collect(ctx, []Venue{v}); !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatal(err, calls)
	}
	if _, err = collector.Collect(context.Background(), []Venue{v, v}); err == nil || calls != 0 {
		t.Fatal("duplicate input fetched", err, calls)
	}
}

func fixtureWeek() []Hours {
	var hours []Hours
	for _, day := range weekdays {
		hours = append(hours, Hours{Day: day, Text: "9 am–5 pm"})
	}
	return hours
}
func hoursText(hours []Hours, day string) string {
	for _, h := range hours {
		if h.Day == day {
			return h.Text
		}
	}
	return ""
}

func TestPartialWeekMergeKeepsPerDayFreshness(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "week.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	at := time.Date(2026, 11, 10, 1, 0, 0, 0, time.UTC)
	v := testVenue()
	o := Observation{Title: v.Name, SourceURL: v.MapsURL, FetchedAt: at, OpeningHours: fixtureWeek()}
	if err = store.save(ctx, v, o, at, ""); err != nil {
		t.Fatal(err)
	}
	later := at.Add(25 * time.Hour)
	o.FetchedAt = later
	o.OpeningHours = []Hours{{Day: "Tuesday", Text: "Closed"}}
	if err = store.save(ctx, v, o, later, "partial"); err != nil {
		t.Fatal(err)
	}
	cached, err := store.Get(ctx, v.Key)
	if err != nil || len(cached.Observation.OpeningHours) != 7 || hoursText(cached.Observation.OpeningHours, "Tuesday") != "Closed" || hoursText(cached.Observation.OpeningHours, "Monday") != "9 am–5 pm" {
		t.Fatal(cached, err)
	}
	for _, h := range cached.Observation.OpeningHours {
		expected := at
		if h.Day == "Tuesday" {
			expected = later
		}
		if h.FetchedAt == nil || !h.FetchedAt.Equal(expected) {
			t.Fatal(h, expected)
		}
	}
	if cached.FreshWeeklyHours(later.Add(time.Minute)) {
		t.Fatal("retained old weekdays incorrectly refreshed")
	}
	o.OpeningHours = fixtureWeek()
	o.FetchedAt = later
	if err = store.save(ctx, v, o, later, ""); err != nil {
		t.Fatal(err)
	}
	cached, err = store.Get(ctx, v.Key)
	if err != nil || !cached.FreshWeeklyHours(later.Add(time.Minute)) {
		t.Fatal(cached, err)
	}
}
