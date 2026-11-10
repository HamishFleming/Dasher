package overpass

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dash-analyzer/internal/geo"
)

const endpoint = "https://overpass-api.de/api/interpreter"

// GoogleHoursDay is an in-memory source observation, never written to the OSM cache.
type GoogleHoursDay struct {
	Day       string
	Text      string
	FetchedAt time.Time
}

type Venue struct {
	GoogleHours    []GoogleHoursDay `json:"-"`
	GoogleHoursURL string           `json:"-"`
	HoursCheckedAt time.Time        `json:"-"`

	OpeningHours string  `json:"opening_hours,omitempty"`
	Cuisine      string  `json:"cuisine,omitempty"`
	OSMType      string  `json:"osm_type"`
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Lat          float64 `json:"lat"`
	Lon          float64 `json:"lon"`
	Takeaway     string  `json:"takeaway,omitempty"`
}

type element struct {
	Type   string            `json:"type"`
	ID     int64             `json:"id"`
	Lat    float64           `json:"lat"`
	Lon    float64           `json:"lon"`
	Center *geo.Point        `json:"center"`
	Tags   map[string]string `json:"tags"`
}

type response struct {
	Elements []element `json:"elements"`
	Remark   string    `json:"remark"`
}

func Fetch(ctx context.Context, b geo.Bounds) ([]Venue, error) {
	return (&Client{HTTP: &http.Client{Timeout: 40 * time.Second}, Endpoints: []string{endpoint, "https://overpass.kumi.systems/api/interpreter"}, Backoff: time.Second}).Fetch(ctx, b)
}

type Client struct {
	HTTP      *http.Client
	Endpoints []string
	Backoff   time.Duration
}

func (c *Client) Fetch(ctx context.Context, b geo.Bounds) ([]Venue, error) {
	if len(c.Endpoints) == 0 || c.HTTP == nil {
		return nil, fmt.Errorf("overpass client requires HTTP client and endpoints")
	}
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			timer := time.NewTimer(c.Backoff * time.Duration(1<<uint(attempt-1)))
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
		v, err := c.fetch(ctx, b, c.Endpoints[attempt%len(c.Endpoints)])
		if err == nil {
			return v, nil
		}
		last = err
	}
	return nil, last
}
func (c *Client) fetch(ctx context.Context, b geo.Bounds, endpoint string) ([]Venue, error) {
	query := fmt.Sprintf(`[out:json][timeout:30];
(
  nwr["amenity"~"^(restaurant|fast_food|cafe|food_court)$"]
    (%.6f,%.6f,%.6f,%.6f);
  nwr["shop"~"^(supermarket|convenience|grocery)$"]
    (%.6f,%.6f,%.6f,%.6f);
);
out center;`, b.South, b.West, b.North, b.East, b.South, b.West, b.North, b.East)

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, endpoint,
		strings.NewReader(url.Values{"data": {query}}.Encode()),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "MelbourneDashAnalyzer/1.0 (local research project)")

	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 500))
		return nil, fmt.Errorf("overpass HTTP %d: %s",
			res.StatusCode, string(body))
	}

	var data response
	if err := json.NewDecoder(io.LimitReader(res.Body, 40<<20)).
		Decode(&data); err != nil {
		return nil, err
	}

	if data.Remark != "" {
		return nil, fmt.Errorf("incomplete Overpass response: %s", data.Remark)
	}
	venues := make([]Venue, 0, len(data.Elements))
	seen := map[string]bool{}
	for _, e := range data.Elements {
		lat, lon := e.Lat, e.Lon
		if e.Center != nil {
			lat, lon = e.Center.Lat, e.Center.Lon
		}
		if lat < b.South || lat > b.North || lon < b.West || lon > b.East {
			continue
		}

		venueType := e.Tags["amenity"]
		switch venueType {
		case "restaurant", "fast_food", "cafe", "food_court":
		default:
			venueType = e.Tags["shop"]
			if !IsGrocery(venueType) {
				continue
			}
		}
		if e.ID <= 0 {
			continue
		}
		key := fmt.Sprintf("%s/%d", e.Type, e.ID)
		if seen[key] {
			continue
		}
		seen[key] = true
		name := e.Tags["name"]
		if name == "" {
			name = "Unnamed " + venueType
		}

		venues = append(venues, Venue{
			ID:           e.ID,
			OSMType:      e.Type,
			Name:         name,
			Type:         venueType,
			Lat:          lat,
			Lon:          lon,
			Takeaway:     e.Tags["takeaway"],
			OpeningHours: e.Tags["opening_hours"],
			Cuisine:      e.Tags["cuisine"],
		})
	}

	if len(venues) == 0 {
		return nil, fmt.Errorf("no venues returned")
	}
	return venues, nil
}

func Save(path string, venues []Venue) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(venues, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "venues-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func Load(path string) ([]Venue, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var venues []Venue
	err = json.Unmarshal(data, &venues)
	return venues, err
}

// IsGrocery identifies the supported OSM shop categories.
func IsGrocery(venueType string) bool {
	return venueType == "supermarket" || venueType == "convenience" || venueType == "grocery"
}
