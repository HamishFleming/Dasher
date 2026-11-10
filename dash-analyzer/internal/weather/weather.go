// Package weather caches a Melbourne-wide Open-Meteo observation.
package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type Conditions struct {
	Temperature float64   `json:"temperature_c"`
	Rain        float64   `json:"precipitation_mm"`
	Wind        float64   `json:"wind_kmh"`
	Observed    string    `json:"observed_at"`
	Fetched     time.Time `json:"fetched_at"`
	Available   bool      `json:"available"`
	Stale       bool      `json:"stale"`
}
type Client struct {
	Mu         sync.Mutex
	HTTP       *http.Client
	URL        string
	cached     Conditions
	retryAfter time.Time
}

func New() *Client {
	return &Client{HTTP: &http.Client{Timeout: 8 * time.Second}, URL: "https://api.open-meteo.com/v1/forecast?latitude=-37.8136&longitude=144.9631&current=temperature_2m,precipitation,wind_speed_10m&timezone=Australia%2FMelbourne"}
}
func (c *Client) Get(ctx context.Context) Conditions {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	if time.Since(c.cached.Fetched) < 15*time.Minute || time.Now().Before(c.retryAfter) {
		return c.cached
	}
	c.retryAfter = time.Now().Add(time.Minute)
	req, err := http.NewRequestWithContext(ctx, "GET", c.URL, nil)
	if err == nil {
		var res *http.Response
		res, err = c.HTTP.Do(req)
		if err == nil {
			defer res.Body.Close()
			if res.StatusCode != 200 {
				err = fmt.Errorf("weather HTTP %d", res.StatusCode)
			} else {
				var data struct {
					Current struct {
						Temp *float64 `json:"temperature_2m"`
						Rain *float64 `json:"precipitation"`
						Wind *float64 `json:"wind_speed_10m"`
						Time string   `json:"time"`
					} `json:"current"`
				}
				err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&data)
				if err == nil && data.Current.Temp != nil && data.Current.Rain != nil && data.Current.Wind != nil && data.Current.Time != "" {
					c.cached = Conditions{Temperature: *data.Current.Temp, Rain: *data.Current.Rain, Wind: *data.Current.Wind, Observed: data.Current.Time, Fetched: time.Now().UTC(), Available: true}
					return c.cached
				}
			}
		}
	}
	c.cached.Stale = true
	return c.cached
}
