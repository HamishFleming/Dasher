package activity

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"dash-analyzer/internal/overpass"
)

type clockRange struct{ start, end int }

var googleClock = regexp.MustCompile(`^(\d{1,2})(?::(\d{2}))?\s*(am|pm)?$`)

// parseGoogleDay accepts only explicit English schedule text, not relative
// status labels or holiday qualifiers. Boundaries are open-inclusive/close-exclusive.
func parseGoogleDay(text string) ([]clockRange, bool) {
	text = strings.ToLower(strings.Join(strings.Fields(text), " "))
	if text == "closed" {
		return nil, true
	}
	if text == "open 24 hours" {
		return []clockRange{{0, 1440}}, true
	}
	text = strings.NewReplacer("–", "-", "—", "-", "−", "-").Replace(text)
	var ranges []clockRange
	for _, part := range strings.Split(text, ",") {
		ends := strings.Split(part, "-")
		if len(ends) != 2 {
			return nil, false
		}
		start, ok1 := googleMinute(ends[0], false)
		end, ok2 := googleMinute(ends[1], true)
		if !ok1 || !ok2 || start == end {
			return nil, false
		}
		ranges = append(ranges, clockRange{start, end})
	}
	return ranges, len(ranges) > 0
}
func googleMinute(text string, end bool) (int, bool) {
	text = strings.TrimSpace(text)
	if text == "noon" {
		return 720, true
	}
	if text == "midnight" {
		return 0, true
	}
	m := googleClock.FindStringSubmatch(text)
	if m == nil {
		return 0, false
	}
	hour, _ := strconv.Atoi(m[1])
	minute := 0
	if m[2] != "" {
		minute, _ = strconv.Atoi(m[2])
	}
	if minute > 59 {
		return 0, false
	}
	if m[3] != "" {
		if hour < 1 || hour > 12 {
			return 0, false
		}
		hour %= 12
		if m[3] == "pm" {
			hour += 12
		}
	} else {
		// Bare numbers have no AM/PM evidence. Explicit 24-hour HH:MM is supported.
		if m[2] == "" || hour > 24 || hour == 24 && (!end || minute != 0) {
			return 0, false
		}
	}
	return hour*60 + minute, true
}
func googleDay(v overpass.Venue, day time.Weekday, now time.Time) ([]clockRange, string, bool) {
	var found *overpass.GoogleHoursDay
	for i := range v.GoogleHours {
		if v.GoogleHours[i].Day == day.String() {
			if found != nil {
				return nil, "", false
			}
			found = &v.GoogleHours[i]
		}
	}
	if found == nil || found.FetchedAt.IsZero() || now.Sub(found.FetchedAt) < 0 || now.Sub(found.FetchedAt) >= 24*time.Hour {
		return nil, "", false
	}
	ranges, ok := parseGoogleDay(found.Text)
	return ranges, day.String() + ": " + found.Text, ok
}
func googleOpeningStatus(v overpass.Venue, at time.Time) (string, string) {
	if len(v.GoogleHours) == 0 {
		return "unknown", ""
	}
	now := v.HoursCheckedAt
	if now.IsZero() {
		now = time.Now()
	}
	t := at.In(Melbourne)
	minute := t.Hour()*60 + t.Minute()
	current, text, known := googleDay(v, t.Weekday(), now)
	for _, r := range current {
		if r.start < r.end && minute >= r.start && minute < r.end || r.start > r.end && minute >= r.start {
			return "confirmed_open", text
		}
	}
	previous, previousText, previousKnown := googleDay(v, t.AddDate(0, 0, -1).Weekday(), now)
	for _, r := range previous {
		if r.start > r.end && minute < r.end {
			return "confirmed_open", previousText + " (overnight)"
		}
	}
	// A missing previous day could have an overnight interval extending into
	// today; a missing current day may have its own interval. Neither implies closed.
	if known && previousKnown {
		return "confirmed_closed", text
	}
	return "unknown", ""
}
