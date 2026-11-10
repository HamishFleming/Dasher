package mapsdata

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// Candidate is a known venue with measured OSM coordinates, not a guessed place.
type Candidate struct {
	Key      string
	Name     string
	Lat, Lon float64
}
type Resolver interface {
	Resolve(context.Context, Candidate) (Venue, error)
}

// Resolve accepts one exact-name result within 250m, or a name with a branch
// suffix within 100m. Map viewport coordinates are never place evidence.
func (c Chromium) Resolve(ctx context.Context, candidate Candidate) (Venue, error) {
	if candidate.Key == "" || strings.TrimSpace(candidate.Name) == "" || strings.HasPrefix(candidate.Name, "Unnamed ") || math.IsNaN(candidate.Lat) || math.IsNaN(candidate.Lon) || math.IsInf(candidate.Lat, 0) || math.IsInf(candidate.Lon, 0) || math.Abs(candidate.Lat) > 90 || math.Abs(candidate.Lon) > 180 {
		return Venue{}, errors.New("named venue and valid coordinates required for matching")
	}
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 45 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	browser, closeBrowser := c.browser(ctx)
	defer closeBrowser()
	searchURL := fmt.Sprintf("https://www.google.com/maps/search/%s/@%.6f,%.6f,17z?hl=en", url.PathEscape(candidate.Name), candidate.Lat, candidate.Lon)
	var state struct {
		Title, URL string
		Links      []struct{ Name, URL string }
		Blocked    bool
	}
	script := `(() => ({title:document.querySelector('h1')?.innerText||'',url:location.href,blocked:/consent\.google|\/sorry\//.test(location.href)||!!document.querySelector('form[action*="consent"],iframe[src*="recaptcha"],#captcha-form'),links:[...document.querySelectorAll('a[href*="/maps/place/"]')].map(e=>({name:e.getAttribute('aria-label')||e.innerText,url:e.href}))}))()`
	if err := chromedp.Do(browser, desktopPage(), chromedp.Navigate(searchURL), waitForPage(`(!!document.querySelector('h1') && location.pathname.startsWith("/maps/place/")) || !!document.querySelector('a[href*="/maps/place/"],form[action*="consent"],#captcha-form') || /consent.google|\/sorry\//.test(location.href)`, chromedp.WithPollingTimeout(25*time.Second)), evaluateInto(script, &state)); err != nil {
		return Venue{}, err
	}
	if state.Blocked {
		return Venue{}, errors.New("Google consent/challenge page; collection stopped")
	}
	// Direct place landing can be validated without selecting a search result.
	if matched(candidate, state.Title, state.URL) {
		return Venue{Key: candidate.Key, Name: state.Title, MapsURL: state.URL}, nil
	}
	urls := map[string]bool{}
	for _, link := range state.Links {
		if matched(candidate, link.Name, link.URL) {
			urls[link.URL] = true
		}
	}
	slog.Info("Google place search inspected", "component", "maps_browser", "venue_key", candidate.Key, "page_title", state.Title, "result_links", len(state.Links), "verified_matches", len(urls))
	if len(urls) != 1 {
		return Venue{}, fmt.Errorf("no unique Google place matching name and coordinates (exact name within 250m or branch suffix within 100m): %d verified matches from %d result links", len(urls), len(state.Links))
	}
	var selected string
	for u := range urls {
		selected = u
	}
	if err := chromedp.Do(browser, chromedp.Navigate(selected), waitForPage(`!!document.querySelector('h1') || /consent.google|\/sorry\//.test(location.href)`, chromedp.WithPollingTimeout(15*time.Second)), evaluateInto(script, &state)); err != nil {
		return Venue{}, err
	}
	if state.Blocked {
		return Venue{}, errors.New("Google consent/challenge page; collection stopped")
	}
	if !matched(candidate, state.Title, state.URL) {
		return Venue{}, errors.New("Google result failed place identity check")
	}
	return Venue{Key: candidate.Key, Name: state.Title, MapsURL: state.URL}, nil
}

var placeCoordinates = regexp.MustCompile(`!3d(-?[0-9]+(?:\.[0-9]+)?)!4d(-?[0-9]+(?:\.[0-9]+)?)`)

func matched(c Candidate, name, rawURL string) bool {
	base := matchingName(c.Name)
	actual := matchingName(name)
	maxDistance := .25
	if actual != base {
		if len(base) < 5 || !strings.HasPrefix(actual, base+" ") {
			return false
		}
		maxDistance = .1
	}
	if err := (Venue{Key: c.Key, Name: name, MapsURL: rawURL}).Validate(); err != nil {
		return false
	}
	matches := placeCoordinates.FindAllStringSubmatch(rawURL, -1)
	if len(matches) == 0 {
		return false
	}
	// Reject conflicting coordinate pairs rather than selecting a convenient one.
	for _, m := range matches {
		lat, e1 := strconv.ParseFloat(m[1], 64)
		lon, e2 := strconv.ParseFloat(m[2], 64)
		if e1 != nil || e2 != nil || math.Abs(lat) > 90 || math.Abs(lon) > 180 || distanceKM(c.Lat, c.Lon, lat, lon) > maxDistance {
			return false
		}
	}
	return true
}
func distanceKM(a, b, c, d float64) float64 {
	r := math.Pi / 180
	dl := (c - a) * r
	dn := (d - b) * r
	h := math.Sin(dl/2)*math.Sin(dl/2) + math.Cos(a*r)*math.Cos(c*r)*math.Sin(dn/2)*math.Sin(dn/2)
	return 6371 * 2 * math.Asin(math.Sqrt(math.Min(1, h)))
}

// Conjunction spelling does not change place identity; coordinates and unique
// results are still mandatory. This deliberately avoids fuzzy business names.
func matchingName(name string) string {
	name = strings.ReplaceAll(name, "’", "'")
	name = strings.ReplaceAll(name, "&", " and ")
	return normalized(name)
}
