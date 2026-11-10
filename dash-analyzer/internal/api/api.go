package api

import (
	"context"
	"dash-analyzer/internal/activity"
	"dash-analyzer/internal/geo"
	"dash-analyzer/internal/history"
	"dash-analyzer/internal/overpass"
	"dash-analyzer/internal/scoring"
	"dash-analyzer/internal/weather"
	"dash-analyzer/pkg/mapsdata"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

var Bounds = geo.Bounds{South: -38.05, West: 144.65, North: -37.55, East: 145.35}

type App struct {
	mapsMu       sync.Mutex
	mapsWG       sync.WaitGroup
	mapsJob      mapsJob
	mapsCancel   context.CancelFunc
	mapsClosed   bool
	MapsStore    *mapsdata.Store
	MapsFetcher  mapsdata.Fetcher
	MapsResolver mapsdata.Resolver
	MapsInterval time.Duration

	mu        sync.RWMutex
	refreshMu sync.Mutex
	Venues    []overpass.Venue
	Updated   time.Time
	Cache     string
	Store     *history.Store
	Weather   *weather.Client
	Fetch     func(context.Context, geo.Bounds) ([]overpass.Venue, error)
}

func New(dir string) (*App, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	s, err := history.Open(filepath.Join(dir, "deliveries.db"))
	if err != nil {
		return nil, err
	}
	mapsStore, err := mapsdata.NewStore(s.DB)
	if err != nil {
		s.DB.Close()
		return nil, err
	}
	browser := mapsdata.Chromium{Executable: os.Getenv("CHROMIUM_PATH")}
	a := &App{MapsStore: mapsStore, MapsFetcher: browser, MapsResolver: browser, mapsJob: mapsJob{State: "idle", RadiusKM: mapsRadiusKM, Results: []mapsOutcome{}}, Cache: filepath.Join(dir, "venues.json"), Store: s, Weather: weather.New(), Fetch: overpass.Fetch}
	if v, err := overpass.Load(a.Cache); err == nil {
		a.Venues = v
		if st, err := os.Stat(a.Cache); err == nil {
			a.Updated = st.ModTime()
		}
	}
	return a, nil
}
func (a *App) Refresh(ctx context.Context, force bool) error {
	a.refreshMu.Lock()
	defer a.refreshMu.Unlock()
	a.mu.RLock()
	age := time.Since(a.Updated)
	a.mu.RUnlock()
	if age < time.Minute || (!force && age < 24*time.Hour) {
		return nil
	}
	v, err := a.Fetch(ctx, Bounds)
	if err != nil {
		return err
	}
	if len(v) == 0 {
		return errors.New("empty venue response")
	}
	if err = overpass.Save(a.Cache, v); err != nil {
		return err
	}
	a.mu.Lock()
	a.Venues = v
	a.Updated = time.Now().UTC()
	a.mu.Unlock()
	return nil
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, err error) {
	write(w, status, map[string]string{"error": err.Error()})
}
func number(r *http.Request, key string, def, min, max float64) (float64, error) {
	s := r.URL.Query().Get(key)
	if s == "" {
		return def, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < min || v > max {
		return 0, errors.New("invalid " + key)
	}
	return v, nil
}
func (a *App) zones(w http.ResponseWriter, r *http.Request) {
	at, selectErr := selectedTime(r, time.Now())
	if selectErr != nil {
		fail(w, 400, selectErr)
		return
	}
	o := scoring.Options{At: at}
	var err error
	var radiusKM float64
	params := []struct {
		key           string
		dest          *float64
		def, min, max float64
	}{{"radius_km", &radiusKM, 0, 0, 100}, {"lat", &o.Origin.Lat, -37.8136, Bounds.South, Bounds.North}, {"lon", &o.Origin.Lon, 144.9631, Bounds.West, Bounds.East}, {"parking", &o.Parking, .65, 0, 1}, {"litres_per_100km", &o.LitresPer100KM, 8, 0, 40}, {"fuel_price", &o.FuelPrice, 2, 0, 10}, {"road_factor", &o.RoadFactor, 1.3, 1, 3}}
	for _, p := range params {
		*p.dest, err = number(r, p.key, p.def, p.min, p.max)
		if err != nil {
			fail(w, 400, err)
			return
		}
	}
	if values, ok := r.URL.Query()["ignore_distance"]; ok {
		if len(values) != 1 || (values[0] != "true" && values[0] != "false") {
			fail(w, 400, errors.New("invalid ignore_distance"))
			return
		}
		o.IgnoreDistance = values[0] == "true"
	}
	if values := r.URL.Query()["radius_km"]; len(values) > 1 || (len(values) == 1 && values[0] == "") {
		fail(w, 400, errors.New("invalid radius_km"))
		return
	}
	a.mu.RLock()
	v := append([]overpass.Venue{}, a.Venues...)
	updated := a.Updated
	a.mu.RUnlock()
	includeGrocery := false
	if values, ok := r.URL.Query()["include_grocery"]; ok {
		if len(values) != 1 || (values[0] != "true" && values[0] != "false") {
			fail(w, 400, errors.New("invalid include_grocery"))
			return
		}
		includeGrocery = values[0] == "true"
	}
	filtered := make([]overpass.Venue, 0, len(v))
	for _, venue := range v {
		if radiusKM > 0 && geo.DistanceKM(o.Origin, geo.Point{Lat: venue.Lat, Lon: venue.Lon}) > radiusKM {
			continue
		}
		if includeGrocery || !overpass.IsGrocery(venue.Type) {
			filtered = append(filtered, venue)
		}
	}
	v = filtered
	snapshots, err := a.MapsStore.HoursSnapshot(r.Context())
	if err != nil {
		fail(w, 500, err)
		return
	}
	checkedAt := time.Now()
	for i := range v {
		if source, ok := snapshots[osmKey(v[i])]; ok {
			v[i].HoursCheckedAt = checkedAt
			v[i].GoogleHoursURL = source.SourceURL
			for _, day := range source.Hours {
				if day.FetchedAt != nil {
					v[i].GoogleHours = append(v[i].GoogleHours, overpass.GoogleHoursDay{Day: day.Day, Text: day.Text, FetchedAt: *day.FetchedAt})
				}
			}
		}
	}
	conditions := a.Weather.Get(r.Context())
	zones := scoring.Optimize(v, o, conditions)
	if r.URL.Path == "/api/recommendations" && len(zones) > 5 {
		zones = zones[:5]
	}
	type activeVenue struct {
		overpass.Venue
		Activity activity.Estimate `json:"activity"`
	}
	active := make([]activeVenue, 0, len(v))
	for _, venue := range v {
		active = append(active, activeVenue{venue, activity.Evaluate(venue, o.At)})
	}
	write(w, 200, map[string]any{"radius_km": radiusKM, "ignore_distance": o.IgnoreDistance, "selected_local_datetime": o.At, "timezone": "Australia/Melbourne", "zones": zones, "venues": active, "total_venues": len(v), "bounds": Bounds, "updated_at": updated, "stale": time.Since(updated) > 24*time.Hour, "weather": conditions, "scored_at": o.At, "disclaimer": "Estimated venue activity, not live demand or Google Maps popularity. Heuristic venue opportunity, not DoorDash demand, participation or predicted earnings. Road distance and travel time are estimates; parking is not measured."})
}
func (a *App) Handler(webDir string) http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /api/maps/venues", a.mapsVenues)
	m.HandleFunc("POST /api/maps/collect", a.mapsStart)
	m.HandleFunc("GET /api/maps/job", a.mapsStatus)
	m.HandleFunc("DELETE /api/maps/job", a.mapsStop)
	m.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		if err := a.Store.DB.PingContext(r.Context()); err != nil {
			fail(w, 503, err)
			return
		}
		write(w, 200, map[string]string{"status": "ok"})
	})
	m.HandleFunc("GET /api/zones", a.zones)
	m.HandleFunc("GET /api/recommendations", a.zones)
	m.HandleFunc("GET /api/weather", func(w http.ResponseWriter, r *http.Request) { write(w, 200, a.Weather.Get(r.Context())) })
	m.HandleFunc("POST /api/refresh", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 130*time.Second)
		defer cancel()
		if err := a.Refresh(ctx, true); err != nil {
			fail(w, 502, err)
			return
		}
		write(w, 200, map[string]string{"status": "ok"})
	})
	m.HandleFunc("GET /api/deliveries", func(w http.ResponseWriter, r *http.Request) {
		ds, err := a.Store.List()
		if err != nil {
			fail(w, 500, err)
			return
		}
		write(w, 200, ds)
	})
	m.HandleFunc("POST /api/deliveries", func(w http.ResponseWriter, r *http.Request) {
		var d history.Delivery
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&d); err != nil {
			fail(w, 400, err)
			return
		}
		if err := dec.Decode(&struct{}{}); err != io.EOF {
			fail(w, 400, errors.New("expected one JSON object"))
			return
		}
		if err := d.Validate(); err != nil {
			fail(w, 400, err)
			return
		}
		d, err := a.Store.Add(d)
		if err != nil {
			fail(w, 500, err)
			return
		}
		write(w, 201, d)
	})
	m.HandleFunc("GET /api/analytics", func(w http.ResponseWriter, r *http.Request) {
		ds, err := a.Store.List()
		if err != nil {
			fail(w, 500, err)
			return
		}
		write(w, 200, history.Summarize(ds))
	})
	m.Handle("GET /web/", http.StripPrefix("/web/", http.FileServer(http.Dir(webDir))))
	m.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/web/", 302)
	})
	return m
}

// selectedTime selects an explicit Melbourne date or the next occurrence of day
// (0=Sunday), including today.
// Calendar arithmetic preserves local hours across DST rather than adding 24h.
func selectedTime(r *http.Request, now time.Time) (time.Time, error) {
	local := now.In(activity.Melbourne)
	day, hour := int(local.Weekday()), local.Hour()
	for _, p := range []struct {
		key  string
		dest *int
		max  int
	}{
		{"day", &day, 6}, {"hour", &hour, 23},
	} {
		values, present := r.URL.Query()[p.key]
		if !present {
			continue
		}
		if len(values) != 1 {
			return time.Time{}, errors.New("invalid " + p.key)
		}
		v, err := strconv.Atoi(values[0])
		if err != nil || v < 0 || v > p.max {
			return time.Time{}, errors.New("invalid " + p.key)
		}
		*p.dest = v
	}
	var chosenDate *time.Time
	if values, present := r.URL.Query()["date"]; present {
		if len(values) != 1 || r.URL.Query().Has("day") {
			return time.Time{}, errors.New("provide a single date (YYYY-MM-DD) or day, not both")
		}
		parsed, err := time.ParseInLocation("2006-01-02", values[0], activity.Melbourne)
		if err != nil || parsed.Format("2006-01-02") != values[0] {
			return time.Time{}, errors.New("invalid date; expected YYYY-MM-DD")
		}
		chosenDate = &parsed
	}
	if chosenDate == nil && !r.URL.Query().Has("day") && !r.URL.Query().Has("hour") {
		return local, nil
	}
	date := local.AddDate(0, 0, (day-int(local.Weekday())+7)%7)
	if chosenDate != nil {
		date = *chosenDate
	}
	selected := time.Date(date.Year(), date.Month(), date.Day(), hour, 0, 0, 0, activity.Melbourne)
	if selected.Hour() != hour || selected.Day() != date.Day() {
		return time.Time{}, errors.New("selected hour does not exist in Melbourne due to daylight saving")
	}
	return selected, nil
}
