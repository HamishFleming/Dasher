// venue-enrich runs the optional Google Maps collector independently of the server.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dash-analyzer/pkg/mapsdata"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	input := flag.String("venues", "", "JSON array of key, name, maps_url objects")
	dbPath := flag.String("db", "data/deliveries.db", "SQLite file (parent directory must exist)")
	executable := flag.String("chromium", "", "Chromium executable (default: auto-detect)")
	timeout := flag.Duration("timeout", 45*time.Second, "per-venue browser timeout")
	interval := flag.Duration("interval", 5*time.Second, "minimum interval between venues")
	flag.Parse()
	if *input == "" {
		return errors.New("-venues is required")
	}
	if *timeout <= 0 || *interval < time.Second {
		return errors.New("timeout must be positive; interval must be at least 1s")
	}
	file, err := os.Open(*input)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 10<<20))
	decoder.DisallowUnknownFields()
	var venues []mapsdata.Venue
	if err = decoder.Decode(&venues); err != nil {
		return err
	}
	if len(venues) == 0 {
		return errors.New("venue list is empty")
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return errors.New("expected a single JSON array")
	}
	// Reject bad input before opening a database or launching Chromium.
	for _, v := range venues {
		if err = v.Validate(); err != nil {
			return fmt.Errorf("%s: %w", v.Key, err)
		}
	}
	store, err := mapsdata.Open(*dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	results, collectErr := (mapsdata.Collector{Store: store, Fetcher: mapsdata.Chromium{Executable: *executable, Timeout: *timeout}, Interval: *interval}).Collect(ctx, venues)
	if err = json.NewEncoder(os.Stdout).Encode(results); err != nil {
		return err
	}
	if collectErr != nil {
		return collectErr
	}
	for _, r := range results {
		if r.Error != "" {
			return errors.New("one or more venues failed or returned partial data; see JSON results and cached last_error")
		}
	}
	return nil
}
