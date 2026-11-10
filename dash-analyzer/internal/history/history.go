package history

import (
	"database/sql"
	"errors"
	"math"
	_ "modernc.org/sqlite"
	"time"
)

type Delivery struct {
	ID      int64     `json:"id"`
	Started time.Time `json:"started_at"`
	Ended   time.Time `json:"ended_at"`
	Gross   float64   `json:"gross_aud"`
	KM      float64   `json:"distance_km"`
	Fuel    float64   `json:"fuel_cost_aud"`
	Other   float64   `json:"other_cost_aud"`
	Notes   string    `json:"notes"`
}

func (d Delivery) Validate() error {
	for _, n := range []float64{d.Gross, d.KM, d.Fuel, d.Other} {
		if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > 1e6 {
			return errors.New("amounts and distance must be finite, between 0 and 1000000")
		}
	}
	if d.Started.IsZero() || !d.Ended.After(d.Started) || d.Ended.Sub(d.Started) > 24*time.Hour || len(d.Notes) > 2000 {
		return errors.New("valid start/end required (maximum 24 hours); notes maximum 2000 characters")
	}
	return nil
}

type Store struct{ DB *sql.DB }

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA busy_timeout=5000; PRAGMA journal_mode=WAL; CREATE TABLE IF NOT EXISTS deliveries(id INTEGER PRIMARY KEY, started TEXT NOT NULL, ended TEXT NOT NULL, gross REAL NOT NULL, km REAL NOT NULL, fuel REAL NOT NULL, other REAL NOT NULL, notes TEXT NOT NULL);`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db}, nil
}
func (s *Store) Add(d Delivery) (Delivery, error) {
	if err := d.Validate(); err != nil {
		return d, err
	}
	r, err := s.DB.Exec(`INSERT INTO deliveries(started,ended,gross,km,fuel,other,notes) VALUES(?,?,?,?,?,?,?)`, d.Started.UTC().Format(time.RFC3339Nano), d.Ended.UTC().Format(time.RFC3339Nano), d.Gross, d.KM, d.Fuel, d.Other, d.Notes)
	if err == nil {
		d.ID, err = r.LastInsertId()
	}
	return d, err
}
func (s *Store) List() ([]Delivery, error) {
	rows, err := s.DB.Query(`SELECT id,started,ended,gross,km,fuel,other,notes FROM deliveries ORDER BY julianday(started) DESC,id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Delivery{}
	for rows.Next() {
		var d Delivery
		var a, b string
		if err = rows.Scan(&d.ID, &a, &b, &d.Gross, &d.KM, &d.Fuel, &d.Other, &d.Notes); err != nil {
			return nil, err
		}
		d.Started, err = time.Parse(time.RFC3339Nano, a)
		if err != nil {
			return nil, err
		}
		d.Ended, err = time.Parse(time.RFC3339Nano, b)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

type Analytics struct {
	Count   int     `json:"deliveries"`
	Gross   float64 `json:"gross_aud"`
	Net     float64 `json:"net_aud"`
	KM      float64 `json:"distance_km"`
	Hours   float64 `json:"logged_hours"`
	PerHour float64 `json:"net_per_hour"`
	PerKM   float64 `json:"net_per_km"`
}

func Summarize(ds []Delivery) Analytics {
	a := Analytics{Count: len(ds)}
	for _, d := range ds {
		a.Gross += d.Gross
		a.Net += d.Gross - d.Fuel - d.Other
		a.KM += d.KM
		a.Hours += d.Ended.Sub(d.Started).Hours()
	}
	if a.Hours > 0 {
		a.PerHour = a.Net / a.Hours
	}
	if a.KM > 0 {
		a.PerKM = a.Net / a.KM
	}
	return a
}
