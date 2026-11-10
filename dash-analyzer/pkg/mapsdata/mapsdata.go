// Package mapsdata collects source-labelled Google Maps observations separately
// from OSM schedules and the dashboard's heuristic scoring.
package mapsdata

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"
)

// Venue uses a caller-owned stable key, for example "node/123". Name must match
// the place heading (ignoring case and whitespace) to guard against wrong places.
type Venue struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	MapsURL string `json:"maps_url"`
}

func (v Venue) Validate() error {
	if strings.TrimSpace(v.Key) == "" || strings.TrimSpace(v.Name) == "" {
		return errors.New("venue key and expected Google Maps name are required")
	}
	u, err := url.Parse(v.MapsURL)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || (u.Hostname() != "www.google.com" && u.Hostname() != "www.google.com.au") || !strings.HasPrefix(u.Path, "/maps/place/") {
		return errors.New("use a full https://www.google.com/maps/place/ URL, not a search or short URL")
	}
	return nil
}

// BusyDay retains the original accessible labels, not inferred percentages,
// visits, orders or earnings. Empty Day means the page did not identify a day.
type BusyDay struct {
	Day    string   `json:"day,omitempty"`
	Labels []string `json:"labels"`
}
type Hours struct {
	FetchedAt *time.Time `json:"fetched_at,omitempty"`
	Day       string     `json:"day"`
	Text      string     `json:"text"`
}

// Observation is partial: nil components mean unavailable, never zero or closed.
// Times are stored as source text; special/holiday hours are not normalized.
type Observation struct {
	AvailabilityNote string    `json:"availability_note,omitempty"`
	Title            string    `json:"title"`
	SourceURL        string    `json:"source_url"`
	FetchedAt        time.Time `json:"fetched_at"`
	Busy             []BusyDay `json:"busy,omitempty"`
	OpeningHours     []Hours   `json:"opening_hours,omitempty"`
}
type Fetcher interface {
	Fetch(context.Context, Venue) (Observation, error)
}
type Result struct {
	Key            string `json:"key"`
	BusyCollected  bool   `json:"busy_collected"`
	HoursCollected bool   `json:"hours_collected"`
	HoursDays      int    `json:"hours_days"`
	HoursComplete  bool   `json:"hours_complete"`
	Error          string `json:"error,omitempty"`
}

type Collector struct {
	Fetcher Fetcher
	Store   *Store
	// Interval is the minimum gap after each request; zero defaults to 5 seconds.
	Interval time.Duration
}

// Collect works sequentially, preserves cached components on failure, and
// returns per-venue outcomes. Database and cancellation errors stop the batch.
func (c Collector) Collect(ctx context.Context, venues []Venue) (results []Result, collectErr error) {
	if c.Fetcher == nil || c.Store == nil {
		return nil, errors.New("fetcher and store are required")
	}
	interval := c.Interval
	if interval == 0 {
		interval = 5 * time.Second
	}
	if interval < 0 {
		return nil, errors.New("interval must not be negative")
	}
	// Validate the whole input before browser or database side effects.
	seen := map[string]bool{}
	for _, v := range venues {
		if err := v.Validate(); err != nil {
			return nil, fmt.Errorf("%s: %w", v.Key, err)
		}
		if seen[v.Key] {
			return nil, fmt.Errorf("duplicate venue key %q", v.Key)
		}
		seen[v.Key] = true
	}
	started := time.Now()
	logger := slog.Default().With("component", "maps_collector")
	logger.Info("Maps batch started", "venues", len(venues), "interval", interval)
	defer func() {
		issues := 0
		for _, result := range results {
			if result.Error != "" {
				issues++
			}
		}
		attrs := []any{"processed", len(results), "total", len(venues), "issues", issues, "elapsed_ms", time.Since(started).Milliseconds()}
		if collectErr != nil {
			attrs = append(attrs, "error", collectErr)
			logger.Warn("Maps batch stopped", attrs...)
		} else {
			logger.Info("Maps batch finished", attrs...)
		}
	}()
	results = make([]Result, 0, len(venues))
	for i, v := range venues {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		if i > 0 {
			timer := time.NewTimer(interval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return results, ctx.Err()
			case <-timer.C:
			}
		}
		at := time.Now().UTC()
		venueLog := logger.With("venue_key", v.Key, "venue", v.Name, "index", i+1, "total", len(venues))
		venueLog.Info("Fetching Maps busy times and opening hours")
		observation, err := c.Fetcher.Fetch(ctx, v)
		result := Result{Key: v.Key}
		if err == nil {
			if observation.FetchedAt.IsZero() {
				observation.FetchedAt = at
			}
			if observation.SourceURL == "" {
				observation.SourceURL = v.MapsURL
			}
			if err = validateObservation(v, observation); err == nil {
				result.BusyCollected = len(observation.Busy) > 0
				result.HoursCollected = len(observation.OpeningHours) > 0
				result.HoursDays = len(observation.OpeningHours)
				result.HoursComplete = result.HoursDays == 7
				var missing []string
				if !result.BusyCollected {
					missing = append(missing, "busy times unavailable")
				}
				if !result.HoursComplete {
					missing = append(missing, fmt.Sprintf("opening hours collected for %d/7 days", result.HoursDays))
				}
				if len(missing) > 0 {
					result.Error = "partial observation: " + strings.Join(missing, "; ") + "; unobserved data retained"
					if observation.AvailabilityNote != "" {
						result.Error += "; " + observation.AvailabilityNote
					}
				}
			}
		}
		if err != nil {
			result.Error = err.Error()
			observation = Observation{}
		}
		if ctx.Err() != nil {
			venueLog.Info("Maps venue collection cancelled", "elapsed_ms", time.Since(at).Milliseconds())
			return results, ctx.Err()
		}
		if err := c.Store.save(ctx, v, observation, at, result.Error); err != nil {
			venueLog.Error("Saving Maps observation failed; previous data retained", "error", err, "elapsed_ms", time.Since(at).Milliseconds())
			return results, err
		}
		attrs := []any{"busy_collected", result.BusyCollected, "hours_collected", result.HoursCollected, "busy_days", len(observation.Busy), "hours_days", len(observation.OpeningHours), "hours_complete", result.HoursComplete, "elapsed_ms", time.Since(at).Milliseconds()}
		if result.Error != "" {
			attrs = append(attrs, "error", result.Error)
			venueLog.Warn("Maps venue attempt saved; unavailable components retained", attrs...)
		} else {
			venueLog.Info("Maps venue observations saved", attrs...)
		}
		results = append(results, result)
	}
	return results, nil
}
func normalized(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }
func validateObservation(v Venue, o Observation) error {
	if normalized(o.Title) != normalized(v.Name) {
		return fmt.Errorf("place heading %q does not match expected name %q", o.Title, v.Name)
	}
	source := v
	source.MapsURL = o.SourceURL
	if err := source.Validate(); err != nil {
		return err
	}
	if len(o.Busy) == 0 && len(o.OpeningHours) == 0 {
		return errors.New("no supported busy times or opening hours found")
	}
	seen := map[string]bool{}
	for _, day := range o.Busy {
		if day.Day != "" && !validDay(day.Day) {
			return errors.New("invalid busy day")
		}
		if len(day.Labels) == 0 || seen[day.Day] {
			return errors.New("empty or duplicate busy day")
		}
		seen[day.Day] = true
		for _, label := range day.Labels {
			if strings.TrimSpace(label) == "" {
				return errors.New("empty busy label")
			}
		}
	}
	seen = map[string]bool{}
	for _, hours := range o.OpeningHours {
		if !validDay(hours.Day) || strings.TrimSpace(hours.Text) == "" || seen[hours.Day] {
			return errors.New("invalid or duplicate opening hours day")
		}
		seen[hours.Day] = true
	}
	return nil
}
func validDay(s string) bool {
	switch s {
	case "Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday":
		return true
	}
	return false
}
