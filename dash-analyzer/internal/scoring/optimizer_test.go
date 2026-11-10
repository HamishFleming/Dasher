package scoring

import (
	"dash-analyzer/internal/geo"
	"dash-analyzer/internal/overpass"
	"dash-analyzer/internal/weather"
	"math"
	"testing"
	"time"
)

func TestOptimizerFactors(t *testing.T) {
	at := time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC)
	o := Options{At: at, Origin: CBD, Parking: 0, LitresPer100KM: 8, FuelPrice: 2, RoadFactor: 1.3}
	v := []overpass.Venue{{ID: 1, Name: "A", Lat: CBD.Lat, Lon: CBD.Lon, Type: "restaurant"}, {ID: 2, Lat: CBD.Lat + .001, Lon: CBD.Lon, Type: "cafe"}, {ID: 3, Lat: -37.9, Lon: 145.1, Type: "restaurant"}}
	z := Optimize(v, o, weather.Conditions{})
	if len(z) != 2 || z[0].Venues != 2 {
		t.Fatal(z)
	}
	base := z[0].Score
	o.Parking = 1
	z = Optimize(v, o, weather.Conditions{})
	if z[0].Score >= base {
		t.Fatal("parking not penalized")
	}
	o.Parking = 0
	wet := Optimize(v, o, weather.Conditions{Available: true, Rain: 5})
	if wet[0].Score >= base {
		t.Fatal("weather not penalized")
	}
	stale := Optimize(v, o, weather.Conditions{Available: true, Stale: true, Rain: 5})
	if stale[0].Score != base {
		t.Fatal("stale weather applied")
	}
	if math.Abs(FuelCost(100, 8, 2)-16) > 1e-9 {
		t.Fatal("fuel")
	}
	o.Origin = geo.Point{Lat: -37.7, Lon: 145.2}
	if Optimize(v, o, weather.Conditions{})[0].DistanceKM == 0 {
		t.Fatal("distance")
	}
}
func TestMelbourneTime(t *testing.T) {
	dinner := time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC)
	night := dinner.Add(8 * time.Hour)
	if TimeFactor(dinner, "restaurant") <= TimeFactor(night, "restaurant") {
		t.Fatal("local dinner weighting")
	}
	if TimeFactor(dinner.AddDate(0, 0, 5), "restaurant") <= TimeFactor(dinner, "restaurant") {
		t.Fatal("weekend weighting")
	}
}

func TestDSTAndFuelSensitivity(t *testing.T) {
	// 06:00 UTC is dinner in January (17:00 AEDT), but 16:00 in July.
	summer := time.Date(2026, 1, 5, 6, 0, 0, 0, time.UTC)
	winter := time.Date(2026, 7, 6, 6, 0, 0, 0, time.UTC)
	if TimeFactor(summer, "restaurant") <= TimeFactor(winter, "restaurant") {
		t.Fatal("Melbourne daylight saving ignored")
	}
	v := []overpass.Venue{{ID: 1, Lat: -37.82, Lon: 144.97, Type: "restaurant"}}
	o := Options{At: summer, Origin: CBD, LitresPer100KM: 8, FuelPrice: 1, RoadFactor: 1.3}
	cheap := Optimize(v, o, weather.Conditions{})[0]
	o.FuelPrice = 3
	dear := Optimize(v, o, weather.Conditions{})[0]
	if dear.Score >= cheap.Score || dear.FuelCost <= cheap.FuelCost {
		t.Fatal("fuel cost should reduce relocation suitability")
	}
}

func TestIgnoreDistance(t *testing.T) {
	v := []overpass.Venue{{ID: 1, Lat: CBD.Lat, Lon: CBD.Lon, Type: "restaurant"}}
	o := Options{At: time.Date(2026, 7, 6, 19, 0, 0, 0, time.UTC), Origin: CBD, RoadFactor: 1.3, LitresPer100KM: 8, FuelPrice: 2, IgnoreDistance: true}
	near := Optimize(v, o, weather.Conditions{})[0]
	o.Origin = geo.Point{Lat: -37.9, Lon: 145.1}
	o.FuelPrice = 4
	far := Optimize(v, o, weather.Conditions{})[0]
	if near.Score != far.Score || far.DistanceKM <= near.DistanceKM || far.FuelCost <= near.FuelCost || far.TravelMinutes <= near.TravelMinutes {
		t.Fatalf("near %+v far %+v", near, far)
	}
	o.IgnoreDistance = false
	if penalized := Optimize(v, o, weather.Conditions{})[0]; penalized.Score >= far.Score {
		t.Fatal("distance penalty missing", penalized, far)
	}
}
