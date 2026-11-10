package mapsdata

import "testing"

func TestPlaceMatching(t *testing.T) {
	c := Candidate{Key: "node/1", Name: "Test Market", Lat: -37.81, Lon: 144.96}
	for _, tc := range []struct {
		name, url string
		want      bool
	}{
		{"Test Market", "https://www.google.com/maps/place/Test/data=!3d-37.81!4d144.96", true},
		{" test  market ", "https://www.google.com/maps/place/Test/data=!3d-37.8101!4d144.9601", true},
		{"Test Market Northland", "https://www.google.com/maps/place/Test/data=!3d-37.8101!4d144.9601", true},
		{"Test Market Northland", "https://www.google.com/maps/place/Test/data=!3d-37.8115!4d144.96", false},
		{"Test Marketplace", "https://www.google.com/maps/place/Test/data=!3d-37.81!4d144.96", false},
		{"Other Market", "https://www.google.com/maps/place/Test/data=!3d-37.81!4d144.96", false},
		{"Test Market", "https://www.google.com/maps/place/Test/@-37.81,144.96,15z", false},
		{"Test Market", "https://www.google.com/maps/place/Test/data=!3d-37.9!4d145.1", false},
		{"Test Market", "https://evil.example/maps/place/Test/data=!3d-37.81!4d144.96", false},
		{"Test Market", "https://www.google.com/maps/place/Test/data=!3d-37.81!4d144.96!3d-37.9!4d145.1", false},
	} {
		if got := matched(c, tc.name, tc.url); got != tc.want {
			t.Errorf("%s %s: got %v", tc.name, tc.url, got)
		}
	}
}

func TestConjunctionMatching(t *testing.T) {
	c := Candidate{Key: "node/1", Name: "Burger Daddy and Kebab", Lat: -37.81, Lon: 144.96}
	near := "https://www.google.com/maps/place/Test/data=!3d-37.81!4d144.96"
	if !matched(c, "Burger Daddy & Kebab", near) {
		t.Fatal("equivalent conjunction rejected")
	}
	if matched(c, "Burger Daddy & Kebab", "https://www.google.com/maps/place/Test/data=!3d-37.9!4d145.1") {
		t.Fatal("far place accepted")
	}
	if matched(c, "Forever Sisters Nail Salon", near) {
		t.Fatal("unrelated business accepted")
	}
}
