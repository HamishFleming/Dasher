package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"dash-analyzer/internal/geo"
	"dash-analyzer/internal/overpass"
	"dash-analyzer/pkg/mapsdata"
)

const mapsRadiusKM = 5.0

type mapsVenue struct {
	overpass.Venue
	Key        string           `json:"key"`
	DistanceKM float64          `json:"distance_km"`
	GoogleMaps *mapsdata.Cached `json:"google_maps,omitempty"`
}
type mapsOutcome struct {
	mapsdata.Result
	Name   string `json:"name"`
	Cached bool   `json:"cached"`
}
type mapsJob struct {
	State      string        `json:"state"`
	Origin     geo.Point     `json:"origin"`
	RadiusKM   float64       `json:"radius_km"`
	Total      int           `json:"total"`
	Completed  int           `json:"completed"`
	Current    string        `json:"current,omitempty"`
	StartedAt  time.Time     `json:"started_at"`
	FinishedAt *time.Time    `json:"finished_at,omitempty"`
	Results    []mapsOutcome `json:"results"`
	Error      string        `json:"error,omitempty"`
}

func osmKey(v overpass.Venue) string {
	kind := v.OSMType
	if kind == "" {
		kind = "legacy"
	}
	return fmt.Sprintf("%s/%d", kind, v.ID)
}
func mapsOrigin(r *http.Request) (geo.Point, error) {
	var p geo.Point
	for _, f := range []struct {
		key      string
		dest     *float64
		min, max float64
	}{{"lat", &p.Lat, Bounds.South, Bounds.North}, {"lon", &p.Lon, Bounds.West, Bounds.East}} {
		values := r.URL.Query()[f.key]
		if len(values) != 1 || values[0] == "" {
			return p, errors.New("a single " + f.key + " is required")
		}
		var err error
		*f.dest, err = number(r, f.key, 0, f.min, f.max)
		if err != nil {
			return p, err
		}
	}
	return p, nil
}
func (a *App) nearbyMaps(ctx context.Context, origin geo.Point) ([]mapsVenue, error) {
	a.mu.RLock()
	venues := append([]overpass.Venue(nil), a.Venues...)
	a.mu.RUnlock()
	out := make([]mapsVenue, 0)
	seen := map[string]bool{}
	for _, v := range venues {
		distance := geo.DistanceKM(origin, geo.Point{Lat: v.Lat, Lon: v.Lon})
		key := osmKey(v)
		if v.ID <= 0 || distance > mapsRadiusKM || seen[key] {
			continue
		}
		seen[key] = true
		item := mapsVenue{Venue: v, Key: key, DistanceKM: distance}
		cached, err := a.MapsStore.Get(ctx, key)
		if err == nil {
			item.GoogleMaps = &cached
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}
func (a *App) mapsVenues(w http.ResponseWriter, r *http.Request) {
	origin, err := mapsOrigin(r)
	if err != nil {
		fail(w, 400, err)
		return
	}
	venues, err := a.nearbyMaps(r.Context(), origin)
	if err != nil {
		fail(w, 500, err)
		return
	}
	a.mu.RLock()
	updated := a.Updated
	a.mu.RUnlock()
	write(w, 200, map[string]any{"origin": origin, "radius_km": mapsRadiusKM, "venues": venues, "total": len(venues), "osm_updated_at": updated, "osm_stale": time.Since(updated) > 24*time.Hour})
}
func (a *App) mapsStatus(w http.ResponseWriter, r *http.Request) {
	a.mapsMu.Lock()
	defer a.mapsMu.Unlock()
	write(w, 200, a.mapsJob)
}
func (a *App) mapsStart(w http.ResponseWriter, r *http.Request) {
	if browser, ok := a.MapsFetcher.(mapsdata.Chromium); ok {
		if err := browser.Available(); err != nil {
			fail(w, 503, err)
			return
		}
	}

	origin, err := mapsOrigin(r)
	if err != nil {
		fail(w, 400, err)
		return
	}
	venues, err := a.nearbyMaps(r.Context(), origin)
	if err != nil {
		fail(w, 500, err)
		return
	}
	if len(venues) == 0 {
		fail(w, 400, errors.New("no cached venues within 5 km; refresh OSM data first"))
		return
	}
	a.mapsMu.Lock()
	if a.mapsClosed {
		a.mapsMu.Unlock()
		fail(w, 503, errors.New("server is stopping"))
		return
	}
	if a.mapsJob.State == "running" {
		a.mapsMu.Unlock()
		fail(w, 409, errors.New("a Maps collection is already running"))
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.mapsCancel = cancel
	a.mapsJob = mapsJob{State: "running", Origin: origin, RadiusKM: mapsRadiusKM, Total: len(venues), StartedAt: time.Now().UTC(), Results: []mapsOutcome{}}
	// Serialize the accepted snapshot before the worker can modify it.
	write(w, 202, a.mapsJob)
	a.mapsWG.Add(1)
	go a.runMaps(ctx, venues)
	a.mapsMu.Unlock()
}
func (a *App) mapsStop(w http.ResponseWriter, r *http.Request) {
	a.mapsMu.Lock()
	defer a.mapsMu.Unlock()
	if a.mapsCancel != nil {
		slog.Info("Maps collection cancellation requested", "component", "maps_job", "completed", a.mapsJob.Completed, "total", a.mapsJob.Total)
		a.mapsCancel()
	}
	write(w, 202, map[string]string{"status": "cancellation requested"})
}
func (a *App) runMaps(ctx context.Context, venues []mapsVenue) {
	defer a.mapsWG.Done()
	started := time.Now()
	logger := slog.Default().With("component", "maps_job", "collection_started_at", started.UTC().Format(time.RFC3339Nano))
	logger.Info("Maps collection started", "venues", len(venues), "radius_km", mapsRadiusKM)
	finalState, finalError := "complete", ""
	defer func() {
		a.mapsMu.Lock()
		defer a.mapsMu.Unlock()
		now := time.Now().UTC()
		a.mapsJob.State = finalState
		a.mapsJob.Error = finalError
		a.mapsJob.Current = ""
		a.mapsJob.FinishedAt = &now
		issues, cached := 0, 0
		for _, result := range a.mapsJob.Results {
			if result.Error != "" {
				issues++
			}
			if result.Cached {
				cached++
			}
		}
		attrs := []any{"state", finalState, "processed", a.mapsJob.Completed, "total", len(venues), "cached", cached, "issues", issues, "elapsed_ms", time.Since(started).Milliseconds()}
		if finalError != "" {
			attrs = append(attrs, "error", finalError)
			logger.Error("Maps collection stopped", attrs...)
		} else {
			logger.Info("Maps collection finished", attrs...)
		}
		if a.mapsCancel != nil {
			a.mapsCancel()
			a.mapsCancel = nil
		}
	}()
	interval := a.MapsInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	for i, item := range venues {
		if i > 0 {
			timer := time.NewTimer(interval)
			select {
			case <-ctx.Done():
				timer.Stop()
				finalState = "cancelled"
				return
			case <-timer.C:
			}
		}
		if ctx.Err() != nil {
			finalState = "cancelled"
			return
		}
		a.mapsMu.Lock()
		a.mapsJob.Current = item.Name
		a.mapsMu.Unlock()
		venueStarted := time.Now()
		venueLog := logger.With("venue_key", item.Key, "venue", item.Name, "index", i+1, "total", len(venues))
		venueLog.Info("Processing nearby venue")
		outcome := mapsOutcome{Result: mapsdata.Result{Key: item.Key}, Name: item.Name}
		if cached := item.GoogleMaps; cached != nil && cached.BusyFetchedAt != nil && cached.FreshWeeklyHours(time.Now()) && time.Since(*cached.BusyFetchedAt) < 24*time.Hour && cached.LastError == "" {
			venueLog.Info("Reusing fresh Maps observations")
			outcome.Cached = true
			outcome.BusyCollected = true
			outcome.HoursCollected = true
			outcome.HoursDays = 7
			outcome.HoursComplete = true
		} else {
			var mapped mapsdata.Venue
			var err error
			if item.GoogleMaps != nil {
				venueLog.Info("Reusing stored Google place mapping")
				mapped = item.GoogleMaps.Venue
			} else {
				venueLog.Info("Matching venue to Google Maps")
				mapped, err = a.MapsResolver.Resolve(ctx, mapsdata.Candidate{Key: item.Key, Name: item.Name, Lat: item.Lat, Lon: item.Lon})
			}
			if err == nil {
				venueLog.Info("Google place ready; collecting observations")
				var results []mapsdata.Result
				results, err = (mapsdata.Collector{Fetcher: a.MapsFetcher, Store: a.MapsStore}).Collect(ctx, []mapsdata.Venue{mapped})
				if len(results) > 0 {
					outcome.Result = results[0]
				}
			}
			if err != nil {
				outcome.Error = err.Error()
			}
		}
		if ctx.Err() != nil {
			finalState = "cancelled"
			return
		}
		attrs := []any{"cached", outcome.Cached, "busy_collected", outcome.BusyCollected, "hours_collected", outcome.HoursCollected, "hours_days", outcome.HoursDays, "hours_complete", outcome.HoursComplete, "elapsed_ms", time.Since(venueStarted).Milliseconds()}
		if outcome.Error != "" {
			attrs = append(attrs, "error", outcome.Error)
			venueLog.Warn("Nearby venue collection failed or partial", attrs...)
		} else {
			venueLog.Info("Nearby venue collection complete", attrs...)
		}
		a.mapsMu.Lock()
		a.mapsJob.Results = append(a.mapsJob.Results, outcome)
		a.mapsJob.Completed++
		a.mapsMu.Unlock()
		// Stop the batch on challenges instead of repeatedly sending blocked requests.
		if strings.Contains(outcome.Error, "consent/challenge") {
			finalState = "failed"
			finalError = "Google requested consent or a challenge; collection stopped"
			return
		}
	}
}

// Close cancels collection and waits for Chromium to stop before closing SQLite.
func (a *App) Close() error {
	a.mapsMu.Lock()
	a.mapsClosed = true
	if a.mapsCancel != nil {
		slog.Info("Stopping Maps collection for server shutdown", "component", "maps_job")
		a.mapsCancel()
	}
	a.mapsMu.Unlock()
	a.mapsWG.Wait()
	return a.Store.DB.Close()
}
