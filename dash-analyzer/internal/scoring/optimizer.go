package scoring

import (
	"dash-analyzer/internal/activity"
	"dash-analyzer/internal/geo"
	"dash-analyzer/internal/overpass"
	"dash-analyzer/internal/weather"
	"math"
	"sort"
	"time"
	_ "time/tzdata"
)

type Options struct {
	IgnoreDistance                                 bool
	Origin                                         geo.Point
	At                                             time.Time
	Parking, LitresPer100KM, FuelPrice, RoadFactor float64
}
type Hotspot struct {
	activity.Breakdown
	ID             int     `json:"id"`
	Name           string  `json:"name"`
	Lat            float64 `json:"lat"`
	Lon            float64 `json:"lon"`
	Venues         int     `json:"venues"`
	Score          float64 `json:"score"`
	Density        float64 `json:"density"`
	TimeFactor     float64 `json:"time_factor"`
	WeatherFactor  float64 `json:"weather_factor"`
	ParkingPenalty float64 `json:"parking_penalty"`
	DistanceKM     float64 `json:"estimated_road_km"`
	FuelCost       float64 `json:"fuel_cost_aud"`
	TravelMinutes  float64 `json:"estimated_travel_minutes"`
}

func FuelCost(km, litres, price float64) float64 { return km * litres * price / 100 }
func TimeFactor(at time.Time, kind string) float64 {
	return activity.Evaluate(overpass.Venue{Type: kind}, at).Factor
}

// Optimize uses greedy, non-overlapping, 800m radius clusters. Each venue
// belongs to exactly one cluster; centroids are recomputed from its members.
func Optimize(venues []overpass.Venue, o Options, w weather.Conditions) []Hotspot {
	v := append([]overpass.Venue(nil), venues...)
	sort.Slice(v, func(i, j int) bool {
		if v[i].Lat != v[j].Lat {
			return v[i].Lat < v[j].Lat
		}
		if v[i].Lon != v[j].Lon {
			return v[i].Lon < v[j].Lon
		}
		return v[i].ID < v[j].ID
	})
	used := make([]bool, len(v))
	out := []Hotspot{}
	for i, seed := range v {
		if used[i] {
			continue
		}
		members := []overpass.Venue{}
		p := geo.Point{Lat: seed.Lat, Lon: seed.Lon}
		for j, x := range v {
			if !used[j] && geo.DistanceKM(p, geo.Point{Lat: x.Lat, Lon: x.Lon}) <= 0.8 {
				used[j] = true
				members = append(members, x)
			}
		}
		z := Hotspot{ID: len(out) + 1, Name: seed.Name, Venues: len(members), WeatherFactor: 1}
		weighted := 0.0
		for _, x := range members {
			z.Lat += x.Lat
			z.Lon += x.Lon
			weight := 1.0
			if overpass.IsGrocery(x.Type) {
				weight = activity.GroceryWeight
			}
			if x.Type == "fast_food" {
				weight = 1.4
			}
			if x.Takeaway == "yes" || x.Takeaway == "only" {
				weight *= 1.2
			}
			z.Density += weight
			e := activity.Evaluate(x, o.At)
			z.Breakdown.Add(e, weight)
			weighted += weight * e.Factor
		}
		z.Lat /= float64(z.Venues)
		z.Lon /= float64(z.Venues)
		z.TimeFactor = weighted / z.Density
		center := geo.Point{Lat: z.Lat, Lon: z.Lon}
		z.ParkingPenalty = 0.55 * o.Parking * math.Exp(-math.Pow(geo.DistanceKM(center, CBD)/2.5, 2))
		// Weather is an access/travel penalty, never an assertion of increased orders.
		if w.Available && !w.Stale {
			z.WeatherFactor = 1 - math.Min(0.3, w.Rain*0.03+w.Wind*0.001)
		}
		z.DistanceKM = geo.DistanceKM(o.Origin, center) * o.RoadFactor
		z.FuelCost = FuelCost(z.DistanceKM, o.LitresPer100KM, o.FuelPrice)
		z.TravelMinutes = z.DistanceKM / 25 * 60
		travelPenalty := 1.0
		if !o.IgnoreDistance {
			travelPenalty += z.DistanceKM/8 + z.FuelCost/4
		}
		z.Score = 100 * (1 - math.Exp(-weighted/12)) * (1 - z.ParkingPenalty) * z.WeatherFactor / travelPenalty
		out = append(out, z)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}
