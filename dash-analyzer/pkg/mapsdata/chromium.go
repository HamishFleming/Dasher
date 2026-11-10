package mapsdata

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

//go:embed extract.js
var extractJS string

// Chromium opens a fresh browser profile per venue, without your login/cookies.
// It stops on consent or challenge pages; it does not bypass them.
type Chromium struct {
	Executable string
	Timeout    time.Duration
}

func (c Chromium) Fetch(ctx context.Context, v Venue) (Observation, error) {
	if err := v.Validate(); err != nil {
		return Observation{}, err
	}
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 45 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	browser, closeBrowser := c.browser(ctx)
	defer closeBrowser()
	u, _ := url.Parse(v.MapsURL)
	query := u.Query()
	query.Set("hl", "en")
	u.RawQuery = query.Encode()
	var snapshot pageSnapshot
	if err := chromedp.Do(browser, desktopPage(), chromedp.Navigate(u.String()),
		waitForPage(`!!document.querySelector('h1') || /consent.google|\/sorry\//.test(location.href)`, chromedp.WithPollingTimeout(25*time.Second)),
		evaluateInto(extractJS, &snapshot)); err != nil {
		return Observation{}, fmt.Errorf("loading Google place: %w", err)
	}
	if snapshot.Blocked {
		return Observation{}, errors.New("Google consent/challenge page; collection stopped")
	}
	if normalized(snapshot.Title) != normalized(v.Name) {
		return Observation{}, errors.New("Google place heading does not match expected venue name")
	}
	// The place heading arrives before its details. Wait for the hours control
	// before deciding it is absent, especially on limited-view pages.
	controlsReady := `(() => {const r=` + extractJS + `;return r.blocked || r.busy.length || r.hours.length || !!document.querySelector('[aria-label="Show open hours for the week"],[data-item-id="oh"]');})()`
	if err := chromedp.Do(browser, waitForPage(controlsReady, chromedp.WithPollingTimeout(10*time.Second))); err != nil {
		if ctx.Err() != nil {
			return Observation{}, fmt.Errorf("waiting for Google place details: %w", err)
		}
		slog.Warn("Google place details did not become ready; inspecting available page", "component", "maps_browser", "venue_key", v.Key, "error", err)
	}
	// Capture available charts and the verified identity before opening a panel
	// that may replace both the heading and the visible chart.
	if err := chromedp.Do(browser, evaluateInto(extractJS, &snapshot)); err != nil {
		return Observation{}, err
	}
	if normalized(snapshot.Title) != normalized(v.Name) {
		return Observation{}, fmt.Errorf("Google place changed before opening hours: %q", snapshot.Title)
	}
	before := snapshot
	expanded, err := expandWeeklyHours(browser)
	if err != nil {
		return Observation{}, err
	}
	// A current-day row can render before the rest of the week. Never treat
	// the first row (or a busy chart) as proof that weekly hours are ready.
	ready := `(() => {const r=` + extractJS + `;return r.blocked || r.hours.length === 7;})()`
	if err := chromedp.Do(browser, waitForPage(ready, chromedp.WithPollingTimeout(10*time.Second))); err != nil {
		if ctx.Err() != nil {
			return Observation{}, fmt.Errorf("waiting for weekly Google hours: %w", err)
		}
		slog.Warn("Full weekly opening hours not exposed; retaining observed days", "component", "maps_browser", "venue_key", v.Key)
	}
	if err := chromedp.Do(browser, evaluateInto(extractJS, &snapshot)); err != nil {
		return Observation{}, err
	}

	slog.Info("Google place page inspected", "component", "maps_browser", "venue_key", v.Key, "limited_view", before.LimitedView || snapshot.LimitedView, "hours_expanded", expanded, "busy_days", len(snapshot.Busy), "hours_days", len(snapshot.Hours), "hours_complete", len(snapshot.Hours) == 7)
	return observationFromPages(v, before, snapshot, expanded)

}

type pageSnapshot struct {
	LimitedView bool      `json:"limited_view"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Blocked     bool      `json:"blocked"`
	Busy        []BusyDay `json:"busy"`
	Hours       []Hours   `json:"hours"`
}

func (c Chromium) browser(ctx context.Context) (context.Context, func()) {
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	opts = append(opts, chromedp.Flag("lang", "en-AU"))
	if c.Executable != "" {
		opts = append(opts, chromedp.ExecPath(c.Executable))
	}
	alloc, closeAlloc := chromedp.NewExecAllocator(ctx, opts...)
	browser, closeBrowser := chromedp.NewContext(alloc)
	return browser, func() { closeBrowser(); closeAlloc() }
}

// Available checks browser installation without launching a browser.
func (c Chromium) Available() error {
	if c.Executable != "" {
		_, err := exec.LookPath(c.Executable)
		return err
	}
	for _, name := range []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable", "chrome"} {
		if _, err := exec.LookPath(name); err == nil {
			return nil
		}
	}
	return errors.New("Chromium is not installed; install it or set CHROMIUM_PATH and restart the server")
}

// evaluateInto captures a typed evaluation within a sequence of void actions.
func evaluateInto[T any](expression string, dest *T) chromedp.Action[chromedp.Void] {
	return func(ctx context.Context, target *chromedp.Target) (chromedp.Void, error) {
		err := retryPageRead(ctx, func() error { _, err := evaluateOnceInto(expression, dest)(ctx, target); return err })
		return chromedp.Void{}, err
	}
}

// Click expressions run once: replaying a click can close the panel again.
func evaluateOnceInto[T any](expression string, dest *T) chromedp.Action[chromedp.Void] {
	return func(ctx context.Context, target *chromedp.Target) (chromedp.Void, error) {
		value, err := chromedp.Evaluate[T](expression)(ctx, target)
		if err == nil {
			*dest = value
		}
		return chromedp.Void{}, err
	}
}
func waitForPage(expression string, opts ...chromedp.PollOption) chromedp.Action[chromedp.Void] {
	return func(ctx context.Context, target *chromedp.Target) (chromedp.Void, error) {
		err := retryPageRead(ctx, func() error { _, err := chromedp.Poll[chromedp.Void](expression, opts...)(ctx, target); return err })
		return chromedp.Void{}, err
	}
}
func retryPageRead(ctx context.Context, read func() error) error {
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := read()
		if err == nil || attempt >= 3 || !transientPageContext(err) {
			return err
		}
		slog.Info("Google page context changed; retrying read", "component", "maps_browser", "attempt", attempt+1, "error", err)
		timer := time.NewTimer(time.Duration(150*(1<<attempt)) * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
func transientPageContext(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "cannot find context with specified id") || strings.Contains(message, "execution context was destroyed") || strings.Contains(message, "cannot find default execution context")
}
func observationFromPages(v Venue, before, after pageSnapshot, expanded bool) (Observation, error) {
	if before.Blocked || after.Blocked {
		return Observation{}, errors.New("Google consent/challenge page; collection stopped")
	}
	if normalized(before.Title) != normalized(v.Name) {
		return Observation{}, errors.New("Google place heading does not match expected venue name")
	}
	if !samePlaceURL(before.URL, after.URL) {
		return Observation{}, errors.New("Google place URL changed identity while opening hours")
	}
	title := after.Title
	if expanded && normalized(title) == "hours" {
		title = before.Title
	}
	o := Observation{Title: title, SourceURL: before.URL, FetchedAt: time.Now().UTC(), Busy: after.Busy, OpeningHours: after.Hours}
	if len(o.Busy) == 0 {
		o.Busy = before.Busy
	}
	if len(o.OpeningHours) == 0 {
		o.OpeningHours = before.Hours
	}
	if (before.LimitedView || after.LimitedView) && len(o.Busy) == 0 {
		o.AvailabilityNote = "Google Maps returned a limited view without a busy-time chart; only explicitly displayed hours can be collected"
		if len(o.OpeningHours) == 0 {
			return Observation{}, errors.New(o.AvailabilityNote + "; no readable hours table found")
		}
	}
	if err := validateObservation(v, o); err != nil {
		return Observation{}, err
	}
	return o, nil
}

// Prefer the stable Google place ID; viewport coordinates alone cannot prove
// identity. Without an ID, require identical decoded place paths.
func samePlaceURL(a, b string) bool {
	first, e1 := url.Parse(a)
	second, e2 := url.Parse(b)
	if e1 != nil || e2 != nil || first.Host != second.Host {
		return false
	}
	if (Venue{Key: "check", Name: "check", MapsURL: a}).Validate() != nil || (Venue{Key: "check", Name: "check", MapsURL: b}).Validate() != nil {
		return false
	}
	id1 := googlePlaceID.FindStringSubmatch(first.Path)
	id2 := googlePlaceID.FindStringSubmatch(second.Path)
	if len(id1) > 0 || len(id2) > 0 {
		return len(id1) > 0 && len(id2) > 0 && id1[1] == id2[1]
	}
	return first.Path == second.Path
}

var googlePlaceID = regexp.MustCompile(`!1s([^!/?]+)`)

// Open the place's full hours panel rather than a current-day summary. The
// selected action is performed once so a retry cannot toggle the panel closed.
func expandWeeklyHours(browser context.Context) (bool, error) {
	var expanded bool
	err := chromedp.Do(browser, evaluateOnceInto(`(() => {
 const weekly=document.querySelector('button[data-item-id="oh"],[data-item-id="oh"] [role="button"],[data-item-id="oh"][role="button"]');
 const inline=document.querySelector('[aria-label="Show open hours for the week"]');
 const button=weekly||inline||[...document.querySelectorAll('button,[role="button"]')].find(e=>/See more hours|Show open hours for the week/i.test(e.getAttribute('aria-label')||''));
 if(!button)return false;
 if(button.getAttribute('aria-expanded')==='true')return true;
 button.click();return true;
 })()`, &expanded))
	return expanded, err
}

// Request the normal desktop layout with the installed Chromium version. This
// avoids Google's reduced headless layout, which may contain only today's hours.
// Consent/challenge handling and isolated, logged-out profiles are unchanged.
func desktopPage() chromedp.Action[chromedp.Void] {
	return chromedp.Func(func(ctx context.Context, target *chromedp.Target) error {
		agent, err := chromedp.Evaluate[string](`navigator.userAgent`)(ctx, target)
		if err != nil {
			return err
		}
		agent = strings.ReplaceAll(agent, "HeadlessChrome/", "Chrome/")
		_, err = cdp.Call(ctx, target, emulation.SetUserAgentOverride, emulation.SetUserAgentOverrideParams{UserAgent: agent, AcceptLanguage: "en-AU,en;q=0.9"})
		return err
	})
}
