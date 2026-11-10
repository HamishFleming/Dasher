package activity

import (
	"dash-analyzer/internal/overpass"
	"testing"
	"time"
)

func pelligraFixture() overpass.Venue {
	now := time.Date(2026, 11, 10, 21, 34, 0, 0, Melbourne)
	v := overpass.Venue{Name: "Pelligra", Type: "cafe", HoursCheckedAt: now, GoogleHoursURL: "https://www.google.com/maps/place/Pelligra+Cakes"}
	for d := time.Sunday; d <= time.Saturday; d++ {
		text := "7 am–4 pm"
		if d == time.Sunday {
			text = "Closed"
		}
		v.GoogleHours = append(v.GoogleHours, overpass.GoogleHoursDay{Day: d.String(), Text: text, FetchedAt: now.Add(-time.Minute)})
	}
	return v
}
func TestGoogleHoursPelligra(t *testing.T) {
	v := pelligraFixture()
	for _, tc := range []struct {
		day, hour, minute int
		status            string
	}{{10, 6, 59, "confirmed_closed"}, {10, 7, 0, "confirmed_open"}, {10, 15, 59, "confirmed_open"}, {10, 16, 0, "confirmed_closed"}, {10, 21, 33, "confirmed_closed"}, {15, 12, 0, "confirmed_closed"}} {
		at := time.Date(2026, 11, tc.day, tc.hour, tc.minute, 0, 0, Melbourne)
		e := Evaluate(v, at)
		if e.Status != tc.status || e.HoursSource != "google_maps" || e.Confidence != "medium" {
			t.Fatal(tc, e)
		}
		if tc.status == "confirmed_closed" && e.Factor != 0 {
			t.Fatal("closed venue contributed", e)
		}
		if tc.status == "confirmed_open" && e.Factor <= 0 {
			t.Fatal("open venue missing activity", e)
		}
	}
	// Missing Google popular-times data is not a dependency of schedule scoring.
	v.OpeningHours = "24/7"
	if e := Evaluate(v, time.Date(2026, 11, 15, 12, 0, 0, 0, Melbourne)); e.Factor != 0 || e.HoursSource != "google_maps" {
		t.Fatal(e)
	}
	// Existing OSM closures remain zero even when the sources disagree.
	v.OpeningHours = "off"
	if e := Evaluate(v, time.Date(2026, 11, 10, 12, 0, 0, 0, Melbourne)); e.Factor != 0 || e.HoursSource != "osm" {
		t.Fatal(e)
	}
}
func TestGoogleHoursFallbackAndOvernight(t *testing.T) {
	v := pelligraFixture()
	v.OpeningHours = "24/7"
	for i := range v.GoogleHours {
		v.GoogleHours[i].FetchedAt = v.HoursCheckedAt.Add(-25 * time.Hour)
	}
	at := time.Date(2026, 11, 10, 20, 0, 0, 0, Melbourne)
	if e := Evaluate(v, at); e.Status != "confirmed_open" || e.HoursSource != "osm" {
		t.Fatal(e)
	}
	v = pelligraFixture()
	v.OpeningHours = "24/7"
	v.GoogleHours[int(time.Tuesday)].Text = "7 am–4 pm (holiday hours)"
	if e := Evaluate(v, at); e.HoursSource != "osm" {
		t.Fatal(e)
	}
	v = pelligraFixture()
	v.GoogleHours[int(time.Friday)].Text = "10 pm–2 am"
	v.GoogleHours[int(time.Saturday)].Text = "Closed"
	if e := Evaluate(v, time.Date(2026, 11, 14, 1, 59, 0, 0, Melbourne)); e.Status != "confirmed_open" {
		t.Fatal(e)
	}
	if e := Evaluate(v, time.Date(2026, 11, 14, 2, 0, 0, 0, Melbourne)); e.Status != "confirmed_closed" {
		t.Fatal(e)
	}
	// An omitted previous day cannot rule out its overnight interval.
	v = pelligraFixture()
	v.GoogleHours = v.GoogleHours[2:]
	if e := Evaluate(v, at); e.Status != "unknown" || e.Confidence != "low" {
		t.Fatal(e)
	}
}
func TestGoogleHoursSplitAndDST(t *testing.T) {
	for _, tc := range []struct {
		text  string
		valid bool
	}{{"7 am–4 pm", true}, {"Open 24 hours", true}, {"Closed", true}, {"9 am–12 pm, 1 pm–5 pm", true}, {"07:00–16:00", true}, {"midnight–noon", true}, {"7–4", false}, {"9 am–9 am", false}, {"7 am–4 pm; hours may differ", false}, {"25:00–26:00", false}, {"13 pm–2 pm", false}} {
		if _, ok := parseGoogleDay(tc.text); ok != tc.valid {
			t.Fatal(tc)
		}
	}
	v := pelligraFixture()
	for i := range v.GoogleHours {
		v.GoogleHours[i].Text = "9 am–12 pm, 1 pm–5 pm"
	}
	if e := Evaluate(v, time.Date(2026, 11, 10, 12, 30, 0, 0, Melbourne)); e.Factor != 0 {
		t.Fatal(e)
	}
	if e := Evaluate(v, time.Date(2026, 11, 10, 13, 0, 0, 0, Melbourne)); e.Factor <= 0 {
		t.Fatal(e)
	}
	v.GoogleHours[int(time.Saturday)].Text = "10 pm–3 am"
	v.GoogleHours[int(time.Sunday)].Text = "Closed"
	// Both the spring transition and repeated autumn hour use local calendar days.
	for _, at := range []time.Time{time.Date(2026, 10, 4, 1, 30, 0, 0, Melbourne), time.Date(2026, 4, 5, 2, 30, 0, 0, Melbourne)} {
		if e := Evaluate(v, at); e.Status != "confirmed_open" {
			t.Fatal(at, e)
		}
	}
}
