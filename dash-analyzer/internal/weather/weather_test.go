package weather

import (
	"context"
	"dash-analyzer/internal/testhttp"
	"net/http"
	"testing"
	"time"
)

func TestWeatherCacheAndFallback(t *testing.T) {
	calls := 0
	s := testhttp.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls > 1 {
			w.WriteHeader(503)
			return
		}
		_, _ = w.Write([]byte(`{"current":{"temperature_2m":14,"precipitation":2,"wind_speed_10m":15,"time":"2026-11-10T12:00"}}`))
	}))
	defer s.Close()
	c := &Client{HTTP: s.Client(), URL: s.URL}
	a := c.Get(context.Background())
	if !a.Available || a.Rain != 2 {
		t.Fatal(a)
	}
	c.Get(context.Background())
	if calls != 1 {
		t.Fatal("cache miss")
	}
	c.cached.Fetched = time.Now().Add(-time.Hour)
	c.retryAfter = time.Time{}
	a = c.Get(context.Background())
	if !a.Stale || !a.Available || calls != 2 {
		t.Fatal(a, calls)
	}
}
