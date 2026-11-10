package geo

import "testing"

func TestDistance(t *testing.T) {
	a := Point{Lat: -37.8136, Lon: 144.9631}
	if DistanceKM(a, a) != 0 {
		t.Fatal("same point should have zero distance")
	}
	b := Point{Lat: -37.8, Lon: 144.98}
	if DistanceKM(a, b) <= 0 {
		t.Fatal("different points should have positive distance")
	}
}
