"use client";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Banknote, Clock3, Route, ReceiptText, Plus, X } from "lucide-react";
import { localDateTime } from "@/lib/time";
import { api, date, money } from "@/lib/api";
import type { Analytics, Delivery } from "@/lib/types";
import { PageHeading, ErrorNotice, Stat, Empty } from "@/components/ui";

export default function History() {
  const [deliveries, setDeliveries] = useState<Delivery[]>([]);
  const [stats, setStats] = useState<Analytics | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState("");
  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [rows, totals] = await Promise.all([
        api<Delivery[] | null>("deliveries"),
        api<Analytics>("analytics"),
      ]);
      setDeliveries(rows || []);
      setStats(totals);
      setError("");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => {
    void load();
  }, [load]);
  async function save(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = e.currentTarget;
    setSaving(true);
    setError("");
    try {
      const input = Object.fromEntries(new FormData(form));
      const body = {
        started_at: localDateTime(String(input.started_at)),
        ended_at: localDateTime(String(input.ended_at)),
        gross_aud: Number(input.gross_aud),
        distance_km: Number(input.distance_km),
        fuel_cost_aud: Number(input.fuel_cost_aud),
        other_cost_aud: Number(input.other_cost_aud),
        notes: String(input.notes || ""),
      };
      await api("deliveries", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      form.reset();
      setShowForm(false);
      setSaved("Delivery saved to your local ledger.");
      await load();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setSaving(false);
    }
  }
  return (
    <>
      <PageHeading
        eyebrow="YOUR ACTUAL NUMBERS"
        title="Delivery ledger"
        description="Track completed deliveries and work blocks. Keep the whole shift in view."
      >
        <button
          className="button"
          onClick={() => {
            setShowForm((v) => !v);
            setSaved("");
          }}
        >
          <Plus size={16} />
          Log delivery / work block
        </button>
      </PageHeading>
      <ErrorNotice message={error} />
      {saved && (
        <p className="success-notice" role="status">
          {saved}
        </p>
      )}
      <div className="stats-grid">
        <Stat
          label="Recorded net"
          value={stats ? money(stats.net_aud) : "—"}
          detail="Gross less entered fuel and other costs"
          icon={Banknote}
        />
        <Stat
          label="Net per logged hour"
          value={stats ? money(stats.net_per_hour) : "—"}
          detail={
            stats
              ? `${stats.logged_hours.toFixed(1)} total logged hours`
              : "Waiting for ledger"
          }
          icon={Clock3}
        />
        <Stat
          label="Net per kilometre"
          value={stats ? money(stats.net_per_km) : "—"}
          detail={
            stats
              ? `${stats.distance_km.toFixed(1)} logged km`
              : "Waiting for ledger"
          }
          icon={Route}
        />
        <Stat
          label="Logged entries"
          value={stats ? String(stats.deliveries) : "—"}
          detail="Completed deliveries and work blocks"
          icon={ReceiptText}
        />
      </div>
      {showForm && (
        <section className="panel entry-panel">
          <div className="panel-heading">
            <div>
              <span className="eyebrow">ADD TO YOUR PRIVATE LEDGER</span>
              <h2>A completed work block</h2>
            </div>
            <button
              aria-label="Close entry form"
              className="icon-button"
              onClick={() => setShowForm(false)}
            >
              <X size={19} />
            </button>
          </div>
          <form onSubmit={save}>
            <div className="form-grid three">
              <label className="field">
                Start · Melbourne time
                <input type="datetime-local" name="started_at" required />
              </label>
              <label className="field">
                End · Melbourne time
                <input type="datetime-local" name="ended_at" required />
              </label>
              <label className="field">
                Gross earnings · AUD
                <input
                  name="gross_aud"
                  type="number"
                  min="0"
                  max="1000000"
                  step=".01"
                  required
                />
              </label>
              <label className="field">
                Total distance · km
                <input
                  name="distance_km"
                  type="number"
                  min="0"
                  max="1000000"
                  step=".1"
                  required
                />
              </label>
              <label className="field">
                Fuel cost · AUD
                <input
                  name="fuel_cost_aud"
                  type="number"
                  min="0"
                  max="1000000"
                  step=".01"
                  required
                />
              </label>
              <label className="field">
                Other costs · AUD
                <input
                  name="other_cost_aud"
                  type="number"
                  min="0"
                  max="1000000"
                  step=".01"
                  defaultValue="0"
                  required
                />
              </label>
            </div>
            <label className="field">
              Notes{" "}
              <textarea
                name="notes"
                maxLength={2000}
                placeholder="Optional context for this block"
                rows={2}
              />
            </label>
            <div className="form-actions">
              <small>
                Include waiting, repositioning and return travel. Avoid
                overlapping blocks.
              </small>
              <button className="button" disabled={saving}>
                {saving ? "Saving…" : "Save to local ledger"}
              </button>
            </div>
          </form>
        </section>
      )}
      <section className="panel table-panel">
        <div className="panel-heading">
          <div>
            <span className="eyebrow">MEASURED, NOT PREDICTED</span>
            <h2>Your recent entries</h2>
          </div>
          <span className="tag">Newest first · Melbourne time</span>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Work block</th>
                <th>Gross</th>
                <th>Costs</th>
                <th>Net</th>
                <th>Distance</th>
                <th>Notes</th>
              </tr>
            </thead>
            <tbody>
              {deliveries.map((d) => (
                <tr key={d.id}>
                  <td>
                    <strong>{date(d.started_at)}</strong>
                    <span className="subtle">to {date(d.ended_at)}</span>
                  </td>
                  <td>{money(d.gross_aud)}</td>
                  <td>{money(d.fuel_cost_aud + d.other_cost_aud)}</td>
                  <td className="accent-text">
                    {money(d.gross_aud - d.fuel_cost_aud - d.other_cost_aud)}
                  </td>
                  <td className="nowrap">{d.distance_km.toFixed(1)} km</td>
                  <td className="notes-cell">{d.notes || "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {!loading && deliveries.length === 0 && (
          <Empty title="Start with your first shift">
            Log a completed delivery or work block to see your actual costs and
            returns.
          </Empty>
        )}
        {loading && (
          <p className="loading-text" role="status">
            Loading your local ledger…
          </p>
        )}
      </section>
      <div className="note-panel">
        <strong>Good records make useful numbers.</strong>
        <p>
          Totals only describe entries you provide. Missing waiting time
          inflates hourly figures, overlapping blocks double-count time, and
          unlogged costs are excluded. Include maintenance, parking and tolls
          where relevant. No tax model is applied.
        </p>
      </div>
    </>
  );
}
