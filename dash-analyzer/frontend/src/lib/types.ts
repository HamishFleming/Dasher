export type Settings = {
  lat: number;
  lon: number;
  radius: number;
  parking: number;
  fuel: number;
  consumption: number;
  roadFactor: number;
  grocery: boolean;
  ignoreDistance: boolean;
  markers: boolean;
  minScore: number;
  date: string;
  day: number;
  hour: number;
};
export type Activity = {
  hours_source?: string;
  hours_text?: string;
  hours_url?: string;
  reason?: string;
  status: string;
  factor: number;
  profile: string;
  confidence: string;
};
export type Venue = {
  id: number;
  osm_type: string;
  name: string;
  type: string;
  lat: number;
  lon: number;
  cuisine?: string;
  opening_hours?: string;
  activity: Activity;
};
export type Zone = {
  id: number;
  name: string;
  lat: number;
  lon: number;
  venues: number;
  score: number;
  activity_score: number;
  confirmed_open: number;
  confirmed_closed: number;
  unknown_hours: number;
  confidence: string;
  estimated_road_km: number;
  fuel_cost_aud: number;
  estimated_travel_minutes: number;
  parking_penalty: number;
};
export type ZonesResponse = {
  zones: Zone[];
  venues: Venue[];
  total_venues: number;
  selected_local_datetime: string;
  updated_at: string;
  stale: boolean;
  weather: {
    available: boolean;
    stale: boolean;
    temperature_c: number;
    precipitation_mm: number;
    wind_kmh: number;
  };
};
export type BusyDay = { day?: string; labels: string[] };
export type MapsCache = {
  observation: {
    title: string;
    busy?: BusyDay[];
    opening_hours?: { day: string; text: string; fetched_at?: string }[];
  };
  venue: { maps_url: string; name: string };
  busy_fetched_at?: string;
  hours_fetched_at?: string;
  last_error?: string;
  last_attempt: string;
};
export type MapsVenue = {
  key: string;
  name: string;
  type: string;
  distance_km: number;
  google_maps?: MapsCache;
};
export type MapsResponse = {
  venues: MapsVenue[];
  total: number;
  radius_km: number;
  origin: { lat: number; lon: number };
  osm_stale: boolean;
};
export type Outcome = {
  key: string;
  name: string;
  busy_collected: boolean;
  hours_collected: boolean;
  hours_days?: number;
  hours_complete?: boolean;
  cached: boolean;
  error?: string;
};
export type MapsJob = {
  state: string;
  total: number;
  completed: number;
  current?: string;
  radius_km: number;
  origin: { lat: number; lon: number };
  results: Outcome[];
  error?: string;
};
export type Delivery = {
  id: number;
  started_at: string;
  ended_at: string;
  gross_aud: number;
  distance_km: number;
  fuel_cost_aud: number;
  other_cost_aud: number;
  notes: string;
};
export type Analytics = {
  deliveries: number;
  gross_aud: number;
  net_aud: number;
  distance_km: number;
  logged_hours: number;
  net_per_hour: number;
  net_per_km: number;
};
