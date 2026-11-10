package activity

import (
	"dash-analyzer/internal/overpass"
	"testing"
	"time"
)

func local(day, hour int) time.Time { return time.Date(2026, 7, day, hour, 0, 0, 0, Melbourne) }
func TestProfiles(t *testing.T) {
	cafe := overpass.Venue{Type: "cafe"}
	restaurant := overpass.Venue{Type: "restaurant"}
	for _, tc := range []struct {
		v       overpass.Venue
		hour    int
		profile string
	}{
		{cafe, 8, "breakfast"}, {restaurant, 12, "lunch"}, {restaurant, 19, "dinner"}, {restaurant, 23, "late_night"},
	} {
		if got := Evaluate(tc.v, local(6, tc.hour)); got.Profile != tc.profile || got.Factor <= 0 {
			t.Fatal(tc, got)
		}
	}
	if Evaluate(cafe, local(6, 8)).Factor <= Evaluate(cafe, local(6, 19)).Factor {
		t.Fatal("breakfast cafe")
	}
	if Evaluate(restaurant, local(6, 12)).Factor <= Evaluate(restaurant, local(6, 4)).Factor {
		t.Fatal("lunch")
	}
	if Evaluate(restaurant, local(6, 19)).Factor <= Evaluate(restaurant, local(6, 8)).Factor {
		t.Fatal("dinner")
	}
	for d := 6; d <= 12; d++ {
		if Evaluate(restaurant, local(d, 19)).Factor <= 0 {
			t.Fatal("day", d)
		}
	}
	if Evaluate(cafe, local(11, 8)).Factor <= Evaluate(cafe, local(6, 8)).Factor {
		t.Fatal("weekend")
	}
	pizza := restaurant
	pizza.Cuisine = "pizza"
	if Evaluate(pizza, local(6, 19)).Factor <= Evaluate(restaurant, local(6, 19)).Factor {
		t.Fatal("cuisine")
	}
}

func TestOpeningHours(t *testing.T) {
	for _, tc := range []struct {
		hours     string
		day, hour int
		status    string
	}{
		{"Mo-Fr 09:00-17:00", 6, 12, "confirmed_open"},
		{"Mo-Fr 09:00-17:00", 6, 19, "confirmed_closed"},
		{"Mo-Fr 09:00-17:00", 11, 12, "confirmed_closed"},
		{"24/7", 12, 3, "confirmed_open"}, {"off", 6, 12, "confirmed_closed"},
		{"Mo 22:00-02:00", 7, 1, "confirmed_open"},
		{"Mo 09:00-17:00; Mo 12:00-13:00 off", 6, 12, "confirmed_closed"},
		{"", 6, 12, "unknown"}, {"nonsense", 6, 12, "unknown"},
		{"Mo-Fr 09:00-17:00; PH off", 6, 12, "unknown"},
		{"sunrise-sunset", 6, 12, "unknown"},
		{"Mo-Fr 09:00-17:00; Mo 09:00-12:00", 6, 15, "unknown"},
		{"Mo off; Mo 09:00-17:00", 6, 12, "unknown"}, {"Mo 29:00-30:00", 6, 12, "unknown"},
	} {
		e := Evaluate(overpass.Venue{Type: "restaurant", OpeningHours: tc.hours}, local(tc.day, tc.hour))
		if e.Status != tc.status {
			t.Fatalf("%+v: %+v", tc, e)
		}
		if tc.status == "confirmed_closed" && e.Factor != 0 {
			t.Fatal(e)
		}
		if tc.status == "unknown" && (e.Factor <= 0 || e.Confidence != "low") {
			t.Fatal(e)
		}
	}
}
func TestDST(t *testing.T) {
	v := overpass.Venue{Type: "restaurant", OpeningHours: "Mo 17:00-18:00"}
	summer := time.Date(2026, 1, 5, 6, 0, 0, 0, time.UTC)
	winter := time.Date(2026, 7, 6, 6, 0, 0, 0, time.UTC)
	if Evaluate(v, summer).Status != "confirmed_open" || Evaluate(v, winter).Status != "confirmed_closed" {
		t.Fatal("DST ignored")
	}
}

func TestGroceryActivity(t *testing.T) {
	for _, typ := range []string{"supermarket", "convenience", "grocery"} {
		v := overpass.Venue{Type: typ, Cuisine: "pizza"}
		for _, hour := range []int{8, 12, 19, 23} {
			e := Evaluate(v, local(6, hour))
			if e.Factor != 1 || e.Profile != "grocery" || e.Confidence != "low" {
				t.Fatal(e)
			}
		}
		v.OpeningHours = "off"
		if e := Evaluate(v, local(6, 12)); e.Factor != 0 || e.Status != "confirmed_closed" {
			t.Fatal(e)
		}
	}
}
