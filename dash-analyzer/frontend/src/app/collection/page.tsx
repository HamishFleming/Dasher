"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import {
  ArrowUpRight,
  Database,
  Download,
  Layers3,
  RefreshCw,
  Square,
  Store,
} from "lucide-react";
import { useSettings } from "@/components/providers";
import { ErrorNotice, PageHeading, Stat, Empty } from "@/components/ui";
import { api, date, originQuery } from "@/lib/api";
import type { MapsJob, MapsResponse } from "@/lib/types";
export default function Collection() {
  const { settings, ready } = useSettings();
  const [data, setData] = useState<MapsResponse | null>(null);
  const [job, setJob] = useState<MapsJob | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [filter, setFilter] = useState("");
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const requestVersion = useRef(0);
  const query = originQuery(settings).toString();
  const load = useCallback(async () => {
    const version = ++requestVersion.current;
    setLoading(true);
    try {
      const next = await api<MapsResponse>("maps/venues?" + query);
      if (version === requestVersion.current) {
        setData(next);
        setError("");
      }
    } catch (e) {
      if (version === requestVersion.current) setError((e as Error).message);
    } finally {
      if (version === requestVersion.current) setLoading(false);
    }
  }, [query]);
  useEffect(() => {
    if (ready) {
      setData(null);
      void load();
    }
    return () => {
      requestVersion.current++;
    };
  }, [load, ready]);
  useEffect(() => {
    if (!ready) return;
    let stopped = false;
    let timer: ReturnType<typeof setTimeout>;
    async function poll() {
      let shouldPoll = true;
      try {
        const current = await api<MapsJob>("maps/job");
        if (stopped) return;
        setJob(current);
        shouldPoll = current.state === "running";
        if (!shouldPoll) void load();
      } catch (e) {
        if (!stopped) setError((e as Error).message);
      } finally {
        if (!stopped && shouldPoll) timer = setTimeout(poll, 3000);
      }
    }
    void poll();
    return () => {
      stopped = true;
      clearTimeout(timer);
    };
  }, [ready, load, job?.state]);
  async function collect() {
    setBusy(true);
    setError("");
    try {
      setJob(await api<MapsJob>("maps/collect?" + query, { method: "POST" }));
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function cancel() {
    setBusy(true);
    try {
      await api("maps/job", { method: "DELETE" });
      setError("");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function refresh() {
    setRefreshing(true);
    try {
      await api("refresh", { method: "POST" });
      await load();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setRefreshing(false);
    }
  }
  const active = job?.state === "running";
  const cached = data?.venues.filter(
    (v) => v.google_maps?.hours_fetched_at || v.google_maps?.busy_fetched_at,
  ).length;
  const busyCharts = data?.venues.filter(
    (v) => v.google_maps?.busy_fetched_at,
  ).length;
  const outcomes = new Map(job?.results.map((r) => [r.key, r]) || []);
  const visible =
    data?.venues.filter((v) =>
      `${v.name} ${v.type}`.toLowerCase().includes(filter.toLowerCase()),
    ) || [];
  return (
    <>
      <PageHeading
        eyebrow="OBSERVE, THEN DECIDE"
        title="Venue intelligence"
        description="Collect source observations within 5 km of your selected location."
      >
        <button
          className="button secondary"
          onClick={() => void load()}
          disabled={loading}
        >
          <RefreshCw size={15} />
          Reload cache
        </button>
        {active ? (
          <button className="button secondary" onClick={cancel} disabled={busy}>
            <Square size={14} />
            Cancel collection
          </button>
        ) : (
          <button
            className="button"
            onClick={collect}
            disabled={busy || !ready || !data?.total}
          >
            <Download size={15} />
            {busy ? "Starting…" : "Collect within 5 km"}
          </button>
        )}
      </PageHeading>
      <ErrorNotice message={error} />
      <div className="stats-grid">
        <Stat
          label="In collection range"
          value={data ? String(data.total) : "—"}
          detail="All cached food + grocery venues · 5 km"
          icon={Store}
        />
        <Stat
          label="With saved observations"
          value={cached === undefined ? "—" : String(cached)}
          detail="Stored locally in SQLite"
          icon={Database}
        />
        <Stat
          label="Busy charts collected"
          value={busyCharts === undefined ? "—" : String(busyCharts)}
          detail="Only source data; missing is not zero"
          icon={Layers3}
        />
        <Stat
          label="Current collection"
          value={
            job && job.state !== "idle" ? `${job.completed}/${job.total}` : "—"
          }
          detail={job?.state || "Checking job status…"}
          icon={Download}
        />
      </div>
      {job && job.state !== "idle" && (
        <section className="panel job-panel" aria-live="polite">
          <div className="job-heading">
            <div className="job-title">
              <span className={"status-dot " + (active ? "pulse" : "muted")} />
              <h2>
                {active ? "Collection in progress" : `Collection ${job.state}`}
              </h2>
            </div>
            <span className="tag">
              {job.completed} of {job.total}
            </span>
          </div>
          <div className="progress-track">
            <div
              style={{
                width: `${job.total ? Math.min(100, (job.completed / job.total) * 100) : 0}%`,
              }}
            />
          </div>
          <p>
            {job.current ? `Now processing ${job.current}. ` : ""}Job origin:{" "}
            {job.origin.lat.toFixed(4)}, {job.origin.lon.toFixed(4)} ·{" "}
            {job.radius_km} km. Changing your location does not move a running
            job.
          </p>
          {job.error && <ErrorNotice message={job.error} />}
        </section>
      )}
      <div className="collection-layout">
        <section className="panel table-panel">
          <div className="panel-heading">
            <div>
              <span className="eyebrow">YOUR LOCAL SOURCE CACHE</span>
              <h2>Nearby venues</h2>
            </div>
            <input
              className="search"
              aria-label="Search venues"
              placeholder="Search venues…"
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
            />
          </div>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Venue / source details</th>
                  <th>Distance</th>
                  <th>Observations</th>
                </tr>
              </thead>
              <tbody>
                {visible.map((v) => {
                  const cache = v.google_maps;
                  const result = outcomes.get(v.key);
                  return (
                    <tr key={v.key}>
                      <td>
                        <details className="venue-details">
                          <summary>
                            <strong>{v.name}</strong>
                            <span className="subtle">
                              {v.type.replaceAll("_", " ")}
                            </span>
                          </summary>
                          <div className="venue-source">
                            {cache ? (
                              <>
                                <span className="eyebrow">
                                  GOOGLE MAPS ·{" "}
                                  {cache.observation.title || cache.venue.name}
                                </span>
                                <h4>
                                  Busy times{" "}
                                  <small>{date(cache.busy_fetched_at)}</small>
                                </h4>
                                {cache.observation.busy?.length ? (
                                  cache.observation.busy.map((d, i) => (
                                    <p key={i}>
                                      <strong>
                                        {d.day || "Day unidentified"}:
                                      </strong>{" "}
                                      {d.labels.join(" · ")}
                                    </p>
                                  ))
                                ) : (
                                  <p>
                                    Unavailable. This does not mean no activity.
                                  </p>
                                )}
                                <h4>
                                  Opening hours{" "}
                                  <small>{date(cache.hours_fetched_at)}</small>
                                </h4>
                                <p>
                                  {cache.observation.opening_hours?.length || 0}
                                  /7 days observed. Dates below show each day's
                                  source freshness.
                                </p>
                                {[
                                  "Monday",
                                  "Tuesday",
                                  "Wednesday",
                                  "Thursday",
                                  "Friday",
                                  "Saturday",
                                  "Sunday",
                                ].map((day) => {
                                  const hours =
                                    cache.observation.opening_hours?.find(
                                      (h) => h.day === day,
                                    );
                                  return (
                                    <p key={day}>
                                      <strong>{day}:</strong>{" "}
                                      {hours?.text || "Not collected"}
                                      {hours && (
                                        <small>
                                          {" "}
                                          ·{" "}
                                          {date(
                                            hours.fetched_at ||
                                              cache.hours_fetched_at,
                                          )}
                                        </small>
                                      )}
                                    </p>
                                  );
                                })}
                                {cache.last_error && (
                                  <p className="source-warning">
                                    {cache.last_error}
                                  </p>
                                )}
                                <a
                                  href={cache.venue.maps_url}
                                  target="_blank"
                                  rel="noreferrer"
                                >
                                  Open Google Maps source{" "}
                                  <ArrowUpRight size={13} />
                                </a>
                              </>
                            ) : (
                              <p>No Maps observations stored for this venue.</p>
                            )}
                            {result?.error && (
                              <p className="source-warning">
                                Latest job: {result.error}
                              </p>
                            )}
                          </div>
                        </details>
                      </td>
                      <td className="nowrap">{v.distance_km.toFixed(1)} km</td>
                      <td>
                        <span
                          className={
                            "tag " +
                            (cache?.busy_fetched_at ? "positive" : "neutral")
                          }
                        >
                          {cache?.busy_fetched_at
                            ? "Busy chart"
                            : cache?.hours_fetched_at
                              ? `Hours ${cache.observation.opening_hours?.length || 0}/7`
                              : result?.error
                                ? "Not matched"
                                : "Not collected"}
                        </span>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          {!loading && visible.length === 0 && (
            <Empty title={filter ? "No matches" : "No cached venues nearby"}>
              {filter
                ? "Try another venue name."
                : "Refresh OSM data or choose another point within Melbourne."}
            </Empty>
          )}
        </section>
        <aside className="panel source-guide">
          <div className="section-icon">
            <Database size={20} />
          </div>
          <h2>Know the source</h2>
          <p>
            This collection includes food and grocery venues, independently of
            your scoring filters.
          </p>
          <div className="guide-rule" />
          <h3>Missing charts stay missing</h3>
          <p>
            Google may provide a limited view with opening hours but no busy
            chart. Old observations stay saved when collection fails.
          </p>
          <h3>A careful match</h3>
          <p>
            Names and place coordinates must match a unique nearby result.
            Ambiguous businesses stay unavailable.
          </p>
          <h3>One job at a time</h3>
          <p>
            Requests run sequentially. A large batch takes time; you can keep
            using the app or cancel it.
          </p>
          <button
            className="button secondary full"
            onClick={refresh}
            disabled={refreshing}
          >
            <RefreshCw size={14} />
            {refreshing ? "Refreshing…" : "Refresh OSM coverage"}
          </button>
          <Link href="/settings" className="text-link">
            Change selected location <ArrowUpRight size={13} />
          </Link>
          {data?.osm_stale && (
            <p className="source-warning">OSM coverage is stale or empty.</p>
          )}
        </aside>
      </div>
    </>
  );
}
