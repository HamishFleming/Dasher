// Package activity provides uncalibrated meal-time heuristics, never live demand.
package activity

import (
	"regexp"
	"strings"
	"time"
	_ "time/tzdata"

	"dash-analyzer/internal/overpass"
	"github.com/chneau/openhours/v2"
)

var Melbourne = func() *time.Location {
	loc, err := time.LoadLocation("Australia/Melbourne")
	if err != nil {
		panic(err)
	}
	return loc
}()

// GroceryWeight is a planning preference, not measured demand or platform participation.
const GroceryWeight = 2.0

type Estimate struct {
	HoursSource string `json:"hours_source"`
	HoursText   string `json:"hours_text,omitempty"`
	HoursURL    string `json:"hours_url,omitempty"`

	Status     string  `json:"status"`
	Factor     float64 `json:"factor"`
	Profile    string  `json:"profile"`
	Confidence string  `json:"confidence"`
	Reason     string  `json:"reason"`
}

// Validate a conservative weekly subset before using the permissive library.
// Holiday, seasonal, solar, fallback and comment rules need context we lack.
var weekly = regexp.MustCompile(`^(?:(?:Mo|Tu|We|Th|Fr|Sa|Su)(?:-(?:Mo|Tu|We|Th|Fr|Sa|Su))?(?:,(?:Mo|Tu|We|Th|Fr|Sa|Su)(?:-(?:Mo|Tu|We|Th|Fr|Sa|Su))?)*\s+)?(?:[0-2][0-9]:[0-5][0-9]-[0-2][0-9]:[0-5][0-9](?:,\s*[0-2][0-9]:[0-5][0-9]-[0-2][0-9]:[0-5][0-9])*(?:\s+(?:open|off|closed))?|off|closed|open)$`)
var clock = regexp.MustCompile(`([0-2][0-9]):([0-5][0-9])`)

func OpeningStatus(hours string, at time.Time) string {
	hours = strings.TrimSpace(hours)
	if hours == "" {
		return "unknown"
	}
	if hours != "24/7" {
		seenDays, seenOff := 0, false
		for _, rule := range strings.Split(hours, ";") {
			rule = strings.TrimSpace(rule)
			if !weekly.MatchString(rule) {
				return "unknown"
			}
			// OSM replacement rules differ from the parser's union semantics.
			// Reject overlapping weekdays and reopening after an off rule.
			off := strings.HasSuffix(rule, "off") || strings.HasSuffix(rule, "closed")
			if off {
				seenOff = true
			} else {
				if seenOff {
					return "unknown"
				}
				mask := 127
				first := strings.Fields(rule)[0]
				if first[0] >= 'A' && first[0] <= 'Z' {
					mask = 0
					days := openhours.Parse(first + " 00:00-24:00")
					for d := 0; d < 7; d++ {
						if days.IsOpen(time.Date(2026, 7, 6+d, 12, 0, 0, 0, Melbourne)) {
							mask |= 1 << d
						}
					}
				}
				if seenDays&mask != 0 {
					return "unknown"
				}
				seenDays |= mask
			}
			for _, c := range clock.FindAllStringSubmatch(rule, -1) {
				if c[1] > "24" || (c[1] == "24" && c[2] != "00") {
					return "unknown"
				}
			}
		}
	}
	if openhours.Parse(hours).IsOpen(at.In(Melbourne)) {
		return "confirmed_open"
	}
	return "confirmed_closed"
}

// Evaluate uses four meal windows plus an off-peak baseline. Factors are
// relative activity assumptions, not probabilities or measured popularity.
func Evaluate(v overpass.Venue, at time.Time) Estimate {
	t := at.In(Melbourne)
	h := t.Hour()
	profile := "off_peak"
	idx := 4
	switch {
	case h >= 6 && h < 11:
		profile, idx = "breakfast", 0
	case h >= 11 && h < 15:
		profile, idx = "lunch", 1
	case h >= 17 && h < 22:
		profile, idx = "dinner", 2
	case h >= 22 || h < 3:
		profile, idx = "late_night", 3
	}
	weights := [5]float64{.35, 1.15, 1.4, .3, .2}
	if v.Type == "cafe" {
		weights = [5]float64{1.3, 1.1, .25, .05, .3}
	}
	if v.Type == "fast_food" {
		weights = [5]float64{.55, 1.2, 1.3, .8, .3}
	}
	if v.Type == "food_court" {
		weights = [5]float64{.3, 1.3, 1.0, .1, .2}
	}
	for _, cuisine := range strings.Split(strings.ToLower(v.Cuisine), ";") {
		switch strings.TrimSpace(cuisine) {
		case "breakfast", "coffee_shop":
			weights[0] *= 1.25
		case "sandwich", "salad":
			weights[1] *= 1.15
		case "pizza", "indian", "thai", "chinese":
			weights[2] *= 1.15
		case "burger", "kebab":
			weights[3] *= 1.25
		}
	}
	factor := weights[idx]
	if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		factor *= 1.1
		if idx == 0 {
			factor *= 1.1
		}
	}
	if t.Weekday() == time.Friday && (idx == 2 || idx == 3) {
		factor *= 1.1
	}
	if overpass.IsGrocery(v.Type) {
		factor, profile = 1, "grocery"
	}
	status := OpeningStatus(v.OpeningHours, t)
	result := Estimate{Status: status, Factor: factor, Profile: profile, Confidence: "low", HoursSource: "unknown", Reason: "Missing or unsupported opening hours; category/cuisine heuristic only"}
	if status != "unknown" {
		result.HoursSource = "osm"
		result.HoursText = v.OpeningHours
		result.Confidence = "medium"
		result.Reason = "OSM weekly opening_hours; activity remains an uncalibrated estimate"
	}
	google, dayText := googleOpeningStatus(v, t)
	// Keep the existing invariant that a confirmed OSM closure contributes zero.
	// Otherwise fresh, explicit Google hours take precedence over OSM/heuristics.
	if status != "confirmed_closed" && google != "unknown" {
		status = google
		result.Status = status
		result.HoursSource = "google_maps"
		result.HoursText = dayText
		result.HoursURL = v.GoogleHoursURL
		result.Confidence = "medium"
		result.Reason = "Fresh Google Maps opening hours; activity remains an uncalibrated estimate"
	} else if status == "confirmed_closed" && google == "confirmed_open" {
		result.Reason = "OSM indicates closed while Google hours indicate open; conservative closure retained"
	}

	if status == "confirmed_closed" {
		result.Factor = 0
	}
	return result
}

type Breakdown struct {
	ActivityScore   float64 `json:"activity_score"`
	ConfirmedOpen   int     `json:"confirmed_open"`
	ConfirmedClosed int     `json:"confirmed_closed"`
	UnknownHours    int     `json:"unknown_hours"`
	Confidence      string  `json:"confidence"`
}

func (b *Breakdown) Add(e Estimate, weight float64) {
	b.ActivityScore += weight * e.Factor
	switch e.Status {
	case "confirmed_open":
		b.ConfirmedOpen++
	case "confirmed_closed":
		b.ConfirmedClosed++
	default:
		b.UnknownHours++
	}
	b.Confidence = "medium"
	if b.UnknownHours > 0 {
		b.Confidence = "low"
	}
}
