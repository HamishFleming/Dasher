package scoring

import (
	"dash-analyzer/internal/weather"
	"math"
	"testing"
	"time"

	"dash-analyzer/internal/geo"
	"dash-analyzer/internal/overpass"
)

func TestAnalyze(t *testing.T) {
	b := geo.Bounds{
		South: -37.83, West: 144.95,
		North: -37.80, East: 144.99,
	}
	v := []overpass.Venue{
		{ID: 1, Lat: -37.8136, Lon: 144.9631, Type: "fast_food"},
	}
	zones := Analyze(v, b, 0.5, time.Now())
	if len(zones) == 0 {
		t.Fatal("expected scored zones")
	}
	if zones[0].Score != 100 {
		t.Fatalf("expected top score 100, got %v", zones[0].Score)
	}
}

func TestClosedAndUnknownActivity(t *testing.T) {
	b := geo.Bounds{South: -37.83, West: 144.95, North: -37.80, East: 144.99}
	at := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	v := []overpass.Venue{{ID: 1, Lat: CBD.Lat, Lon: CBD.Lon, Type: "restaurant", OpeningHours: "off"}}
	if z := Analyze(v, b, 0, at); len(z) != 0 {
		t.Fatal("closed venue contributed", z)
	}
	z := Optimize(v, Options{At: at}, weather.Conditions{})
	if z[0].Score != 0 || z[0].ConfirmedClosed != 1 {
		t.Fatal(z)
	}
	v[0].OpeningHours = ""
	z = Optimize(v, Options{At: at}, weather.Conditions{})
	if z[0].Score <= 0 || z[0].UnknownHours != 1 || z[0].Confidence != "low" {
		t.Fatal(z)
	}
	a := Analyze(v, b, 0, at)
	penalized := Analyze(v, b, 1, at)
	if a[0].RawScore <= penalized[0].RawScore || a[0].ActivityScore != a[0].RawScore {
		t.Fatal("breakdown or parking")
	}
}

func TestGroceryWeighting(t *testing.T) {
	b := geo.Bounds{South: -37.83, West: 144.95, North: -37.80, East: 144.99}
	at := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	for _, kind := range []string{"supermarket", "convenience", "grocery"} {
		v := []overpass.Venue{{ID: 1, Lat: CBD.Lat, Lon: CBD.Lon, Type: kind}}
		z := Optimize(v, Options{At: at, Origin: CBD, IgnoreDistance: true}, weather.Conditions{})
		if len(z) != 1 || z[0].Density != 2 || z[0].ActivityScore != 2 {
			t.Fatal(kind, z)
		}
		// The grid's distance decay is unchanged; compare with a unit baseline
		// at the same coordinates and a known activity factor.
		grid := Analyze(v, b, 0, at)
		v[0].Type = "restaurant"
		baseline := Analyze(v, b, 0, at)
		factor := TimeFactor(at, "restaurant")
		if len(grid) == 0 || len(baseline) == 0 {
			t.Fatal("missing grid")
		}
		if math.Abs(grid[0].RawScore-baseline[0].RawScore*2/factor) > 1e-9 {
			t.Fatal(kind, grid[0], baseline[0])
		}
		v[0].Type = kind
		v[0].OpeningHours = "off"
		if closed := Optimize(v, Options{At: at}, weather.Conditions{}); closed[0].ActivityScore != 0 || closed[0].Score != 0 {
			t.Fatal(closed)
		}
	}
}

func TestCollectedHoursSharedByGridAndClusters(t *testing.T) {
	now := time.Now()
	v := []overpass.Venue{{ID: 1, Name: "Pelligra", Lat: CBD.Lat, Lon: CBD.Lon, Type: "cafe", HoursCheckedAt: now}}
	for day := time.Sunday; day <= time.Saturday; day++ {
		v[0].GoogleHours = append(v[0].GoogleHours, overpass.GoogleHoursDay{Day: day.String(), Text: "7 am–4 pm", FetchedAt: now})
	}
	loc, err := time.LoadLocation("Australia/Melbourne")
	if err != nil {
		t.Fatal(err)
	}
	b := geo.Bounds{South: -37.83, West: 144.95, North: -37.80, East: 144.99}
	closed := time.Date(2026, 11, 10, 17, 0, 0, 0, loc)
	cluster := Optimize(v, Options{At: closed}, weather.Conditions{})
	if len(cluster) != 1 || cluster[0].Score != 0 || cluster[0].ConfirmedClosed != 1 {
		t.Fatal(cluster)
	}
	if grid := Analyze(v, b, 0, closed); len(grid) != 0 {
		t.Fatal(grid)
	}
	open := time.Date(2026, 11, 10, 12, 0, 0, 0, loc)
	if cluster = Optimize(v, Options{At: open}, weather.Conditions{}); cluster[0].Score <= 0 {
		t.Fatal(cluster)
	}
	if grid := Analyze(v, b, 0, open); len(grid) == 0 {
		t.Fatal("open venue excluded")
	}
}
