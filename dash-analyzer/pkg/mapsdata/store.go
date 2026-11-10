package mapsdata

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	_ "modernc.org/sqlite"
	"time"
)

var errMappingChanged = errors.New("venue mapping changed; use a new key to retain source identity")

type Store struct{ DB *sql.DB }

// Open can use the same SQLite file as the delivery ledger; it owns only its
// google_maps_venues table. Close the returned store when finished.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store, err := NewStore(db)
	if err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

// NewStore initializes the Maps table using a caller-owned SQLite connection.
// The caller retains ownership; do not call Close on this wrapper.
func NewStore(db *sql.DB) (*Store, error) {
	_, err := db.Exec(`PRAGMA busy_timeout=5000; PRAGMA journal_mode=WAL;
 CREATE TABLE IF NOT EXISTS google_maps_venues (
 venue_key TEXT PRIMARY KEY, expected_name TEXT NOT NULL, maps_url TEXT NOT NULL,
 title TEXT, source_url TEXT, busy_json TEXT, hours_json TEXT,
 busy_fetched_at TEXT, hours_fetched_at TEXT,
 last_attempt TEXT NOT NULL, last_error TEXT NOT NULL);`)
	if err != nil {
		return nil, err
	}
	return &Store{DB: db}, nil
}

func (s *Store) Close() error { return s.DB.Close() }
func (s *Store) save(ctx context.Context, v Venue, o Observation, at time.Time, message string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var oldName, oldURL string
	var previousHours, previousHoursAt sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT expected_name,maps_url,hours_json,hours_fetched_at FROM google_maps_venues WHERE venue_key=?`, v.Key).Scan(&oldName, &oldURL, &previousHours, &previousHoursAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && (oldName != v.Name || oldURL != v.MapsURL) {
		return errMappingChanged
	}
	var busy, hours, busyAt, hoursAt, title, source any
	if len(o.Busy) > 0 {
		b, err := json.Marshal(o.Busy)
		if err != nil {
			return err
		}
		busy = string(b)
		busyAt = o.FetchedAt.UTC().Format(time.RFC3339Nano)
	}
	if len(o.OpeningHours) > 0 {
		var old []Hours
		if previousHours.Valid {
			if err := json.Unmarshal([]byte(previousHours.String), &old); err != nil {
				return err
			}
			if previousHoursAt.Valid {
				previousTime, err := time.Parse(time.RFC3339Nano, previousHoursAt.String)
				if err != nil {
					return err
				}
				for i := range old {
					if old[i].FetchedAt == nil {
						old[i].FetchedAt = &previousTime
					}
				}
			}
		}
		merged := mergeHours(old, o.OpeningHours, o.FetchedAt)
		b, err := json.Marshal(merged)
		if err != nil {
			return err
		}
		hours = string(b)
		hoursAt = o.FetchedAt.UTC().Format(time.RFC3339Nano)
	}
	if busy != nil || hours != nil {
		title = o.Title
		source = o.SourceURL
	}
	// Refuse to carry observations across a change of the explicit place mapping.
	// The caller must choose a new key rather than silently relabelling old data.
	_, err = tx.ExecContext(ctx, `INSERT INTO google_maps_venues
 (venue_key,expected_name,maps_url,title,source_url,busy_json,hours_json,busy_fetched_at,hours_fetched_at,last_attempt,last_error)
 VALUES(?,?,?,?,?,?,?,?,?,?,?)
 ON CONFLICT(venue_key) DO UPDATE SET
 title=COALESCE(excluded.title,title),source_url=COALESCE(excluded.source_url,source_url),
 busy_json=COALESCE(excluded.busy_json,busy_json),hours_json=COALESCE(excluded.hours_json,hours_json),
 busy_fetched_at=COALESCE(excluded.busy_fetched_at,busy_fetched_at),hours_fetched_at=COALESCE(excluded.hours_fetched_at,hours_fetched_at),
 last_attempt=excluded.last_attempt,last_error=excluded.last_error
 WHERE maps_url=excluded.maps_url AND expected_name=excluded.expected_name`, v.Key, v.Name, v.MapsURL, title, source, busy, hours, busyAt, hoursAt, at.UTC().Format(time.RFC3339Nano), message)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Cached includes separate component timestamps so partial refreshes cannot
// make older observations look fresh. A recorded error never clears data.
type Cached struct {
	Venue          Venue       `json:"venue"`
	Observation    Observation `json:"observation"`
	BusyFetchedAt  *time.Time  `json:"busy_fetched_at,omitempty"`
	HoursFetchedAt *time.Time  `json:"hours_fetched_at,omitempty"`
	LastAttempt    time.Time   `json:"last_attempt"`
	LastError      string      `json:"last_error,omitempty"`
}

func (s *Store) Get(ctx context.Context, key string) (Cached, error) {
	var c Cached
	var title, source, busy, hours, busyAt, hoursAt sql.NullString
	var attempt string
	err := s.DB.QueryRowContext(ctx, `SELECT venue_key,expected_name,maps_url,title,source_url,busy_json,hours_json,busy_fetched_at,hours_fetched_at,last_attempt,last_error FROM google_maps_venues WHERE venue_key=?`, key).Scan(&c.Venue.Key, &c.Venue.Name, &c.Venue.MapsURL, &title, &source, &busy, &hours, &busyAt, &hoursAt, &attempt, &c.LastError)
	if err != nil {
		return c, err
	}
	c.Observation.Title = title.String
	c.Observation.SourceURL = source.String
	if busy.Valid {
		if err = json.Unmarshal([]byte(busy.String), &c.Observation.Busy); err != nil {
			return c, err
		}
	}
	if hours.Valid {
		if err = json.Unmarshal([]byte(hours.String), &c.Observation.OpeningHours); err != nil {
			return c, err
		}
	}
	c.LastAttempt, err = time.Parse(time.RFC3339Nano, attempt)
	if err != nil {
		return c, err
	}
	if busyAt.Valid {
		t, e := time.Parse(time.RFC3339Nano, busyAt.String)
		if e != nil {
			return c, e
		}
		c.BusyFetchedAt = &t
	}
	if hoursAt.Valid {
		t, e := time.Parse(time.RFC3339Nano, hoursAt.String)
		if e != nil {
			return c, e
		}
		c.HoursFetchedAt = &t
	}
	return c, nil
}

var weekdays = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

func mergeHours(previous, current []Hours, at time.Time) []Hours {
	byDay := map[string]Hours{}
	for _, h := range previous {
		byDay[h.Day] = h
	}
	for _, h := range current {
		timestamp := at.UTC()
		h.FetchedAt = &timestamp
		byDay[h.Day] = h
	}
	merged := make([]Hours, 0, len(byDay))
	for _, day := range weekdays {
		if h, ok := byDay[day]; ok {
			merged = append(merged, h)
		}
	}
	return merged
}

// FreshWeeklyHours requires all seven explicitly observed days, with each day's
// own timestamp. Legacy rows use their original component timestamp.
func (c Cached) FreshWeeklyHours(now time.Time) bool {
	if len(c.Observation.OpeningHours) != 7 {
		return false
	}
	seen := map[string]bool{}
	for _, h := range c.Observation.OpeningHours {
		at := h.FetchedAt
		if at == nil {
			at = c.HoursFetchedAt
		}
		if !validDay(h.Day) || seen[h.Day] || at == nil || now.Sub(*at) < 0 || now.Sub(*at) >= 24*time.Hour {
			return false
		}
		seen[h.Day] = true
	}
	return len(seen) == 7
}

// HoursSnapshot reads source schedules in one query for a scoring request.
// Legacy per-day timestamps retain their original component collection time.
type HoursSnapshot struct {
	Hours     []Hours
	SourceURL string
}

func (s *Store) HoursSnapshot(ctx context.Context) (map[string]HoursSnapshot, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT venue_key,hours_json,hours_fetched_at,COALESCE(source_url,maps_url) FROM google_maps_venues WHERE hours_json IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]HoursSnapshot{}
	for rows.Next() {
		var key, raw, source string
		var stamp sql.NullString
		if err = rows.Scan(&key, &raw, &stamp, &source); err != nil {
			return nil, err
		}
		var hours []Hours
		if err = json.Unmarshal([]byte(raw), &hours); err != nil {
			return nil, err
		}
		if stamp.Valid {
			at, e := time.Parse(time.RFC3339Nano, stamp.String)
			if e != nil {
				return nil, e
			}
			for i := range hours {
				if hours[i].FetchedAt == nil {
					hours[i].FetchedAt = &at
				}
			}
		}
		out[key] = HoursSnapshot{Hours: hours, SourceURL: source}
	}
	return out, rows.Err()
}
