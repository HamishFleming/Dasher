package history

import (
	"math"
	"testing"
	"time"
)

func TestValidationAndEmptyAnalytics(t *testing.T) {
	if Summarize(nil) != (Analytics{}) {
		t.Fatal("empty")
	}
	d := Delivery{Started: time.Now(), Ended: time.Now().Add(time.Hour), Gross: 10}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	d.Gross = math.NaN()
	if d.Validate() == nil {
		t.Fatal("NaN accepted")
	}
	d.Gross = 0
	d.Ended = d.Started
	if d.Validate() == nil {
		t.Fatal("zero duration")
	}
}
