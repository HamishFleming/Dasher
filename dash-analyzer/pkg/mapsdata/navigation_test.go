package mapsdata

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestHoursPanelPreservesVerifiedIdentity(t *testing.T) {
	v := Venue{Key: "node/1", Name: "Fixture Market", MapsURL: "https://www.google.com/maps/place/Fixture/data=!1sfixture-id!3d-37.81!4d144.96"}
	before := pageSnapshot{Title: v.Name, URL: v.MapsURL, LimitedView: true, Busy: []BusyDay{{Day: "Tuesday", Labels: []string{"50% busy at 12 pm."}}}}
	panel := pageSnapshot{Title: "Hours", URL: v.MapsURL + "?hl=en", Hours: []Hours{{Day: "Tuesday", Text: "Open 24 hours"}}}
	observation, err := observationFromPages(v, before, panel, true)
	if err != nil || observation.Title != v.Name || len(observation.Busy) != 1 || len(observation.OpeningHours) != 1 {
		t.Fatal(observation, err)
	}
	before.Busy = nil
	observation, err = observationFromPages(v, before, panel, true)
	if err != nil || !strings.Contains(observation.AvailabilityNote, "limited view") {
		t.Fatal(observation, err)
	}
	if _, err = observationFromPages(v, before, panel, false); err == nil {
		t.Fatal("unopened Hours page accepted")
	}
	panel.Title = "Another business"
	if _, err = observationFromPages(v, before, panel, true); err == nil {
		t.Fatal("different business accepted")
	}
	panel.Title = "Hours"
	panel.URL = strings.Replace(v.MapsURL, "fixture-id", "wrong-id", 1)
	if _, err = observationFromPages(v, before, panel, true); err == nil {
		t.Fatal("changed place ID accepted")
	}
	panel.URL = "https://evil.example/maps/place/Fixture/data=!1sfixture-id"
	if _, err = observationFromPages(v, before, panel, true); err == nil {
		t.Fatal("changed source accepted")
	}
}
func TestRetryPageContext(t *testing.T) {
	attempts := 0
	err := retryPageRead(context.Background(), func() error {
		attempts++
		if attempts < 2 {
			return errors.New("Cannot find context with specified id (-32000)")
		}
		return nil
	})
	if err != nil || attempts != 2 {
		t.Fatal(attempts, err)
	}
	attempts = 0
	expected := errors.New("place heading mismatch")
	err = retryPageRead(context.Background(), func() error { attempts++; return expected })
	if !errors.Is(err, expected) || attempts != 1 {
		t.Fatal(attempts, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	attempts = 0
	err = retryPageRead(ctx, func() error { attempts++; cancel(); return errors.New("Execution context was destroyed") })
	if !errors.Is(err, context.Canceled) || attempts != 1 {
		t.Fatal(attempts, err)
	}
}
