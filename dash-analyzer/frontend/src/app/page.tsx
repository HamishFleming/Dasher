"use client";
import dynamic from "next/dynamic";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import {
  ArrowUpRight,
  MapPin,
  RefreshCw,
  Navigation,
  Layers3,
  CloudSun,
  Store,
  SlidersHorizontal,
  Target,
  Clock3,
} from "lucide-react";
import { NumberField } from "@/components/number-field";
import { useSettings } from "@/components/providers";
import { PageHeading, ErrorNotice, Stat, Empty } from "@/components/ui";
import { api, date, money, validLocation, zonesQuery } from "@/lib/api";
import {
  melbourneTime,
  nextMelbourneDate,
  validCalendarDate,
} from "@/lib/time";
import type { ZonesResponse, Zone } from "@/lib/types";
const VenueMap = dynamic(() => import("@/components/venue-map"), {
  ssr: false,
  loading: () => <div className="map-loading">Loading your map…</div>,
});
export default function Explore() {
  const { settings, ready, update } = useSettings();
  const [data, setData] = useState<ZonesResponse | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [selecting, setSelecting] = useState(false);
  const [focus, setFocus] = useState<Zone | null>(null);
  const [revision, setRevision] = useState(0);
  const query = zonesQuery(settings).toString();
  useEffect(() => {
    if (!ready) return;
    const controller = new AbortController();
    setLoading(true);
    setData(null);
    setError("");
    api<ZonesResponse>("zones?" + query, { signal: controller.signal })
      .then(setData)
      .catch((e) => {
        if (!controller.signal.aborted) setError(e.message);
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [ready, query, revision]);
  useEffect(() => {
    const escape = (e: KeyboardEvent) => {
      if (e.key === "Escape") setSelecting(false);
    };
    window.addEventListener("keydown", escape);
    return () => window.removeEventListener("keydown", escape);
  }, []);
  const selectLocation = useCallback(
    (lat: number, lon: number) => {
      if (!validLocation(lat, lon)) {
        setError(
          "Choose a point within Melbourne coverage: latitude −38.05 to −37.55, longitude 144.65 to 145.35.",
        );
        return;
      }
      update({ lat: Number(lat.toFixed(6)), lon: Number(lon.toFixed(6)) });
      setSelecting(false);
      setError("");
    },
    [update],
  );
  function locate() {
    if (!navigator.geolocation) {
      setError("Geolocation is unavailable. Enter coordinates in Settings.");
      return;
    }
    navigator.geolocation.getCurrentPosition(
      (p) => selectLocation(p.coords.latitude, p.coords.longitude),
      (e) => setError(e.message),
    );
  }
  async function refresh() {
    setRefreshing(true);
    setError("");
    try {
      await api("refresh", { method: "POST" });
      setRevision((v) => v + 1);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setRefreshing(false);
    }
  }
  const zones = (data?.zones || []).filter((z) => z.score >= settings.minScore);
  const unknown = data?.venues.filter(
    (v) => v.activity.status === "unknown",
  ).length;
  return (
    <>
      <PageHeading
        eyebrow="THE MELBOURNE WORKSPACE"
        title="Find your next starting point"
        description="Explore nearby venue clusters before you decide where to drive."
      >
        <button
          className="button secondary"
          onClick={refresh}
          disabled={refreshing}
        >
          <RefreshCw size={15} className={refreshing ? "spinning" : ""} />
          {refreshing ? "Refreshing OSM…" : "Refresh OSM"}
        </button>
        <Link className="button" href="/settings">
          <SlidersHorizontal size={15} />
          Adjust scoring
        </Link>
      </PageHeading>
      <ErrorNotice message={error} />
      <div className="stats-grid">
        <Stat
          label="Nearby venues"
          value={data ? String(data.total_venues) : "—"}
          detail={`${settings.radius ? settings.radius + " km radius" : "All cached coverage"} · ${settings.grocery ? "food + grocery" : "food venues"}`}
          icon={Store}
        />
        <Stat
          label="Ranked clusters"
          value={data ? String(zones.length) : "—"}
          detail="Based on estimated venue activity"
          icon={Layers3}
        />
        <Stat
          label="Hours unknown"
          value={unknown === undefined ? "—" : String(unknown)}
          detail="Missing hours retain low confidence"
          icon={MapPin}
        />
        <Stat
          label="Melbourne weather"
          value={
            data?.weather.available
              ? `${data.weather.temperature_c.toFixed(0)}°C`
              : "—"
          }
          detail={
            data?.weather.available
              ? `${data.weather.precipitation_mm} mm rain · ${data.weather.stale ? "stale; excluded from score" : data.weather.wind_kmh + " km/h wind"}`
              : "Unavailable · neutral scoring"
          }
          icon={CloudSun}
        />
      </div>
      <div className="explore-grid">
        <section className="panel map-panel">
          <div className="panel-heading">
            <div>
              <span className="eyebrow">YOUR SURROUNDINGS</span>
              <h2>
                Opportunity map <span className="tag">Estimated</span>
              </h2>
            </div>
            <button
              className="icon-button"
              aria-label="Use my location"
              title="Use my location"
              onClick={locate}
            >
              <Navigation size={18} />
            </button>
          </div>
          <div
            className="map-time-controls"
            role="group"
            aria-label="Map scoring controls"
          >
            <label htmlFor="map-date">
              Date in Melbourne
              <input
                id="map-date"
                aria-label="Date in Melbourne"
                type="date"
                disabled={!ready}
                value={
                  ready ? settings.date || nextMelbourneDate(settings.day) : ""
                }
                onChange={(e) => {
                  if (validCalendarDate(e.target.value))
                    update({ date: e.target.value });
                }}
              />
            </label>
            <label htmlFor="map-hour">
              Time in Melbourne
              <select
                id="map-hour"
                aria-label="Time in Melbourne"
                disabled={!ready}
                value={settings.hour}
                onChange={(e) => update({ hour: Number(e.target.value) })}
              >
                {Array.from({ length: 24 }, (_, hour) => (
                  <option key={hour} value={hour}>
                    {String(hour).padStart(2, "0")}:00
                  </option>
                ))}
              </select>
            </label>
            <NumberField
              label="Range (km)"
              value={settings.radius}
              min={0}
              max={100}
              step={0.1}
              disabled={!ready}
              onCommit={(radius) => update({ radius })}
            />
            <button
              className="button secondary small"
              disabled={!ready}
              onClick={() => update(melbourneTime())}
            >
              <Clock3 size={14} />
              Current hour
            </button>
            <small>
              Australia/Melbourne · 0 km includes all cached coverage · changes
              update rankings automatically
            </small>
          </div>
          <div className="map-toolbar">
            <button
              className={"button small " + (selecting ? "" : "secondary")}
              aria-pressed={selecting}
              onClick={() => setSelecting((v) => !v)}
            >
              <Target size={14} />
              {selecting ? "Cancel selection" : "Select location on map"}
            </button>
            <span>
              {selecting
                ? "Click a point · Escape to cancel"
                : `${settings.lat.toFixed(4)}, ${settings.lon.toFixed(4)}`}
            </span>
          </div>
          <div className="map-wrap">
            <VenueMap
              settings={settings}
              venues={data?.venues || []}
              zones={zones}
              selecting={selecting}
              onSelect={selectLocation}
              focus={focus}
            />
            {loading && (
              <div className="map-status" role="status">
                Updating estimates…
              </div>
            )}
          </div>
          <div className="map-bottom">
            <span>
              <i className="legend-dot lime" />
              Cluster <i className="legend-dot blue" />
              Food <i className="legend-dot purple" />
              Grocery
            </span>
            <span>© OpenStreetMap</span>
          </div>
        </section>
        <section className="panel ranking-panel">
          <div className="panel-heading">
            <div>
              <span className="eyebrow">A PLACE TO START</span>
              <h2>Nearby clusters</h2>
            </div>
            <span className="count-badge">{zones.length}</span>
          </div>
          <div className="rank-filter">
            <label htmlFor="minimum">Minimum score</label>
            <select
              id="minimum"
              value={settings.minScore}
              onChange={(e) => update({ minScore: Number(e.target.value) })}
            >
              {[0, 20, 40, 60, 80].map((v) => (
                <option key={v} value={v}>
                  {v === 0 ? "Show all" : v + "+"}
                </option>
              ))}
            </select>
          </div>
          <div className="rank-list">
            {zones.slice(0, 20).map((z, i) => (
              <button
                className={
                  "rank-item " + (focus?.id === z.id ? "selected" : "")
                }
                key={z.id}
                onClick={() => setFocus(z)}
              >
                <span className="rank-index">
                  {String(i + 1).padStart(2, "0")}
                </span>
                <span className="rank-content">
                  <strong>{z.name || "Venue cluster"}</strong>
                  <span>
                    {z.venues} venues · {z.estimated_road_km.toFixed(1)}{" "}
                    estimated km
                  </span>
                  <span>
                    {money(z.fuel_cost_aud)} fuel · ~
                    {Math.round(z.estimated_travel_minutes)} min
                  </span>
                </span>
                <span className="rank-score">
                  {z.score.toFixed(0)}
                  <small>/100</small>
                </span>
              </button>
            ))}
            {!loading && zones.length === 0 && (
              <Empty title="No clusters to show">
                {data?.total_venues
                  ? "Lower the minimum score or widen your range."
                  : "Refresh OSM or adjust your location and venue options."}
              </Empty>
            )}
          </div>
          <Link href="/collection" className="panel-link">
            Inspect source observations <ArrowUpRight size={15} />
          </Link>
        </section>
      </div>
      <div className="context-strip">
        <span className={"status-dot " + (data?.stale ? "muted" : "")} />
        <p>
          {data
            ? `OSM ${data.stale ? "stale cache" : "updated"} · ${date(data.updated_at)} · Selected ${date(data.selected_local_datetime)} (Melbourne)`
            : "Waiting for the local Go backend."}
        </p>
      </div>
      <div className="note-panel">
        <strong>Use this as a planning aid.</strong>
        <p>
          Scores combine venue categories, schedules and documented heuristics.
          They do not measure DoorDash demand, participation or competition.
          Travel distance, fuel and time are estimates; parking availability is
          not measured.
        </p>
      </div>
    </>
  );
}
