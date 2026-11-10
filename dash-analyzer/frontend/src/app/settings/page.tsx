"use client";
import { useState } from "react";
import {
  Navigation,
  RotateCcw,
  MapPin,
  SlidersHorizontal,
  Fuel,
  Store,
} from "lucide-react";
import Link from "next/link";
import { NumberField } from "@/components/number-field";
import { useSettings } from "@/components/providers";
import { ErrorNotice, PageHeading } from "@/components/ui";
import { validLocation } from "@/lib/api";
import type { Settings } from "@/lib/types";
export default function SettingsPage() {
  const { settings, update, reset } = useSettings();
  const [error, setError] = useState("");
  const numeric = (
    key: keyof Settings,
    label: string,
    min: number,
    max: number,
    step: number,
    help?: string,
  ) => (
    <NumberField
      label={label}
      value={settings[key] as number}
      min={min}
      max={max}
      step={step}
      help={help}
      onCommit={(value) => {
        update({ [key]: value });
        setError("");
      }}
    />
  );
  const toggle = (
    key: "grocery" | "ignoreDistance" | "markers",
    title: string,
    help: string,
  ) => (
    <label className="toggle-row">
      <span>
        <strong>{title}</strong>
        <small>{help}</small>
      </span>
      <input
        type="checkbox"
        checked={settings[key]}
        onChange={(e) => update({ [key]: e.target.checked })}
      />
    </label>
  );
  function locate() {
    if (!navigator.geolocation) {
      setError("Geolocation unavailable. Enter coordinates instead.");
      return;
    }
    navigator.geolocation.getCurrentPosition(
      (p) => {
        if (!validLocation(p.coords.latitude, p.coords.longitude)) {
          setError(
            "Your location is outside the supported Melbourne coverage.",
          );
          return;
        }
        update({
          lat: Number(p.coords.latitude.toFixed(6)),
          lon: Number(p.coords.longitude.toFixed(6)),
        });
        setError("");
      },
      (e) => setError(e.message),
    );
  }
  return (
    <>
      <PageHeading
        eyebrow="MAKE IT YOURS"
        title="Scoring settings"
        description="Set your starting point and the assumptions behind your rankings."
      >
        <button className="button secondary" onClick={reset}>
          <RotateCcw size={15} />
          Reset defaults
        </button>
        <Link href="/" className="button">
          Explore with these settings
        </Link>
      </PageHeading>
      <ErrorNotice message={error} />
      <div className="settings-grid">
        <section className="panel settings-panel">
          <div className="section-icon">
            <MapPin size={20} />
          </div>
          <h2>Your starting point</h2>
          <p>
            Choose a location within the Melbourne coverage area. Changes apply
            across every page.
          </p>
          <div className="form-grid">
            {numeric("lat", "Latitude", -38.05, -37.55, 0.000001)}
            {numeric("lon", "Longitude", 144.65, 145.35, 0.000001)}
          </div>
          <button className="button secondary" onClick={locate}>
            <Navigation size={15} />
            Use my location
          </button>
          {numeric(
            "radius",
            "Range from location (km)",
            0,
            100,
            0.1,
            "0 includes all cached coverage. The range uses straight-line distance to each venue.",
          )}
          <div className="mini-note">
            Maps collection always uses its own 5 km radius around this
            location.
          </div>
        </section>
        <section className="panel settings-panel">
          <div className="section-icon">
            <SlidersHorizontal size={20} />
          </div>
          <h2>Ranking preferences</h2>
          <p>
            Adjust relocation and parking assumptions without changing your
            recorded trips.
          </p>
          {toggle(
            "ignoreDistance",
            "Ignore distance in scoring",
            "Remove road-distance and relocation fuel penalties. Travel estimates remain visible.",
          )}
          <label className="field slider-label">
            CBD parking sensitivity{" "}
            <strong>{Math.round(settings.parking * 100)}%</strong>
            <input
              type="range"
              min="0"
              max="100"
              value={Math.round(settings.parking * 100)}
              onChange={(e) =>
                update({ parking: Number(e.target.value) / 100 })
              }
            />
            <small>
              A CBD-based parking heuristic, not measured availability.
            </small>
          </label>
          <div className="form-grid">
            <label className="field">
              Day in Melbourne
              {settings.date && (
                <small>
                  Calendar date {settings.date} is selected. Choosing a weekday
                  clears it.
                </small>
              )}
              <select
                value={settings.day}
                onChange={(e) =>
                  update({ day: Number(e.target.value), date: "" })
                }
              >
                {[
                  "Sunday",
                  "Monday",
                  "Tuesday",
                  "Wednesday",
                  "Thursday",
                  "Friday",
                  "Saturday",
                ].map((v, i) => (
                  <option key={v} value={i}>
                    {v}
                  </option>
                ))}
              </select>
            </label>
            <label className="field">
              Hour in Melbourne
              <select
                value={settings.hour}
                onChange={(e) => update({ hour: Number(e.target.value) })}
              >
                {Array.from({ length: 24 }, (_, i) => (
                  <option key={i} value={i}>
                    {String(i).padStart(2, "0")}:00
                  </option>
                ))}
              </select>
            </label>
          </div>
          <small>
            Selections use Australia/Melbourne calendar time, including daylight
            saving.
          </small>
        </section>
        <section className="panel settings-panel">
          <div className="section-icon">
            <Fuel size={20} />
          </div>
          <h2>Vehicle & travel</h2>
          <p>
            These values affect estimated relocation costs. Your ledger retains
            the costs you entered.
          </p>
          <div className="form-grid">
            {numeric("fuel", "Fuel price · AUD/litre", 0, 10, 0.01)}
            {numeric("consumption", "Consumption · L/100km", 0, 40, 0.1)}
          </div>
          {numeric(
            "roadFactor",
            "Road distance multiplier",
            1,
            3,
            0.1,
            "Straight-line distance × this factor. Travel time assumes 25 km/h; no live routing or traffic.",
          )}
        </section>
        <section className="panel settings-panel">
          <div className="section-icon">
            <Store size={20} />
          </div>
          <h2>Venues & map</h2>
          <p>Decide what goes into your planning view.</p>
          {toggle(
            "grocery",
            "Include grocery shops",
            "Supermarkets, convenience and grocery shops. Base weight 2× is a planning preference, not measured demand.",
          )}
          {toggle(
            "markers",
            "Show individual venue markers",
            "Keep venue schedules and activity estimates available on the map.",
          )}
          <p className="mini-note">
            Preferences are saved in this browser. The delivery ledger stays in
            the Go service’s local SQLite database.
          </p>
        </section>
      </div>
    </>
  );
}
