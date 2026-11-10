package scoring

import (
	"dash-analyzer/internal/activity"
	"math"
	"sort"
	"time"

	"dash-analyzer/internal/geo"
	"dash-analyzer/internal/overpass"
)

type Zone struct {
	activity.Breakdown
	ParkingPenalty float64 `json:"parking_penalty"`
	ID             int     `json:"id"`
	Lat            float64 `json:"lat"`
	Lon            float64 `json:"lon"`
	Score          float64 `json:"score"`
	Restaurants    int     `json:"restaurants"`
	FastFood       int     `json:"fast_food"`
	Cafes          int     `json:"cafes"`
	RawScore       float64 `json:"raw_score"`
}

var CBD = geo.Point{Lat: -37.8136, Lon: 144.9631}

func Analyze(
	venues []overpass.Venue,
	b geo.Bounds,
	parkingPenalty float64,
	at time.Time,
) []Zone {
	const step = 0.012
	const radiusKM = 1.2

	var zones []Zone
	id := 0
	maxRaw := 0.0

	for lat := b.South + step/2; lat < b.North; lat += step {
		for lon := b.West + step/2; lon < b.East; lon += step {
			center := geo.Point{Lat: lat, Lon: lon}
			var restaurants, fastfood, cafes int
			raw := 0.0
			breakdown := activity.Breakdown{}

			for _, v := range venues {
				d := geo.DistanceKM(center,
					geo.Point{Lat: v.Lat, Lon: v.Lon})
				if d > radiusKM {
					continue
				}

				weight := 1.0
				if overpass.IsGrocery(v.Type) {
					weight = activity.GroceryWeight
				}
				switch v.Type {
				case "fast_food":
					weight = 1.4
					fastfood++
				case "restaurant":
					weight = 1.0
					restaurants++
				case "cafe":
					weight = 0.55
					cafes++
				case "food_court":
					weight = 1.2
				}

				if v.Takeaway == "yes" || v.Takeaway == "only" {
					weight *= 1.2
				}

				// Nearby venues contribute more than distant ones.
				e := activity.Evaluate(v, at)
				contribution := weight * math.Exp(-d*d/(2*0.6*0.6))
				breakdown.Add(e, contribution)
				raw += contribution * e.Factor
			}

			if raw <= 0 {
				continue
			}

			// Heuristic: central Melbourne is harder to park in.
			// This is not measured parking availability.
			cbdDistance := geo.DistanceKM(center, CBD)
			cbdFactor := math.Exp(-math.Pow(cbdDistance/2.5, 2))
			penalty := math.Max(0, math.Min(1, parkingPenalty))
			raw *= 1 - 0.55*penalty*cbdFactor

			id++
			zones = append(zones, Zone{
				ID: id, Lat: lat, Lon: lon,
				Restaurants: restaurants,
				FastFood:    fastfood, Cafes: cafes,
				RawScore: raw, Breakdown: breakdown, ParkingPenalty: 0.55 * penalty * cbdFactor,
			})
			if raw > maxRaw {
				maxRaw = raw
			}
		}
	}

	for i := range zones {
		if maxRaw > 0 {
			zones[i].Score = math.Round(
				1000*zones[i].RawScore/maxRaw) / 10
		}
	}

	sort.Slice(zones, func(i, j int) bool {
		return zones[i].Score > zones[j].Score
	})
	return zones
}
