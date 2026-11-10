package geo

import "math"

type Point struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type Bounds struct {
	South float64 `json:"south"`
	West  float64 `json:"west"`
	North float64 `json:"north"`
	East  float64 `json:"east"`
}

func DistanceKM(a, b Point) float64 {
	const earthRadius = 6371.0

	lat1 := a.Lat * math.Pi / 180
	lat2 := b.Lat * math.Pi / 180
	dLat := (b.Lat - a.Lat) * math.Pi / 180
	dLon := (b.Lon - a.Lon) * math.Pi / 180

	h := math.Pow(math.Sin(dLat/2), 2) +
		math.Cos(lat1)*math.Cos(lat2)*
			math.Pow(math.Sin(dLon/2), 2)

	return 2 * earthRadius * math.Asin(math.Sqrt(h))
}
