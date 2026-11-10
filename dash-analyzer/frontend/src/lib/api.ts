import type { Settings } from "./types";
export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch("/api/" + path, { ...init, cache: "no-store" });
  const body = await response
    .json()
    .catch(() => ({ error: "The backend returned an unreadable response." }));
  if (!response.ok)
    throw new Error(body.error || `Request failed (${response.status})`);
  return body as T;
}
export function zonesQuery(s: Settings) {
  return new URLSearchParams({
    lat: String(s.lat),
    lon: String(s.lon),
    radius_km: String(s.radius),
    parking: String(s.parking),
    fuel_price: String(s.fuel),
    litres_per_100km: String(s.consumption),
    road_factor: String(s.roadFactor),
    include_grocery: String(s.grocery),
    ignore_distance: String(s.ignoreDistance),
    ...(s.date ? { date: s.date } : { day: String(s.day) }),
    hour: String(s.hour),
  });
}
export function originQuery(s: Settings) {
  return new URLSearchParams({ lat: String(s.lat), lon: String(s.lon) });
}
export const money = (value: number) =>
  new Intl.NumberFormat("en-AU", {
    style: "currency",
    currency: "AUD",
    maximumFractionDigits: 2,
  }).format(value);
export const date = (value?: string) =>
  value
    ? new Date(value).toLocaleString("en-AU", {
        timeZone: "Australia/Melbourne",
        dateStyle: "medium",
        timeStyle: "short",
      })
    : "Not collected";
export function validLocation(lat: number, lon: number) {
  return (
    Number.isFinite(lat) &&
    Number.isFinite(lon) &&
    lat >= -38.05 &&
    lat <= -37.55 &&
    lon >= 144.65 &&
    lon <= 145.35
  );
}
