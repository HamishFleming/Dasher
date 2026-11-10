package overpass

import (
	"context"
	"dash-analyzer/internal/geo"
	"dash-analyzer/internal/testhttp"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetryAndCache(t *testing.T) {
	calls := 0
	s := testhttp.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(429)
			return
		}
		if err := r.ParseForm(); err != nil || r.Form.Get("data") == "" {
			t.Error("missing form query")
		}
		_, _ = w.Write([]byte(`{"elements":[{"type":"node","id":1,"lat":-37.8,"lon":145,"tags":{"amenity":"restaurant","name":"Test","opening_hours":"Mo-Fr 09:00-17:00","cuisine":"pizza","takeaway":"yes"}},{"type":"node","id":1,"lat":-37.8,"lon":145,"tags":{"amenity":"restaurant"}}]}`))
	}))
	defer s.Close()
	c := Client{HTTP: s.Client(), Endpoints: []string{s.URL}}
	v, err := c.Fetch(context.Background(), geo.Bounds{South: -38, North: -37, West: 144, East: 146})
	if err != nil || len(v) != 1 || calls != 2 {
		t.Fatalf("%v %v calls %d", v, err, calls)
	}
	p := filepath.Join(t.TempDir(), "venues.json")
	if err = Save(p, v); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil || len(got) != 1 || got[0].Name != "Test" || got[0].OpeningHours != "Mo-Fr 09:00-17:00" || got[0].Cuisine != "pizza" || got[0].Takeaway != "yes" || got[0].Type != "restaurant" || got[0].Lat != -37.8 || got[0].Lon != 145 {
		t.Fatal(got, err)
	}
}
func TestPartialRejected(t *testing.T) {
	s := testhttp.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"remark":"timeout","elements":[]}`))
	}))
	defer s.Close()
	c := Client{HTTP: s.Client(), Endpoints: []string{s.URL}}
	if _, err := c.Fetch(context.Background(), geo.Bounds{}); err == nil {
		t.Fatal("accepted partial response")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Fetch(ctx, geo.Bounds{}); err == nil {
		t.Fatal("expected cancellation")
	}
}

func TestGroceryCollection(t *testing.T) {
	s := testhttp.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if !strings.Contains(r.Form.Get("data"), `nwr["shop"~"^(supermarket|convenience|grocery)$"]`) {
			t.Error("missing grocery query")
		}
		w.Write([]byte(`{"elements":[{"type":"way","id":2,"center":{"lat":-37.8,"lon":145},"tags":{"shop":"supermarket","name":"Market","opening_hours":"off"}},{"type":"node","id":3,"lat":-37.8,"lon":145,"tags":{"shop":"convenience"}},{"type":"node","id":4,"lat":-37.8,"lon":145,"tags":{"shop":"grocery"}},{"type":"node","id":5,"lat":-37.8,"lon":145,"tags":{"shop":"clothes"}}]}`))
	}))
	defer s.Close()
	c := Client{HTTP: s.Client(), Endpoints: []string{s.URL}}
	v, err := c.Fetch(context.Background(), geo.Bounds{South: -38, North: -37, West: 144, East: 146})
	if err != nil || len(v) != 3 {
		t.Fatal(v, err)
	}
	if v[0].Type != "supermarket" || v[0].OpeningHours != "off" || v[1].Name != "Unnamed convenience" {
		t.Fatal(v)
	}
}
