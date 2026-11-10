"use client";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { validCalendarDate } from "@/lib/time";
import type { Settings } from "@/lib/types";
export const defaults: Settings = {
  lat: -37.8136,
  lon: 144.9631,
  radius: 5,
  parking: 0.65,
  fuel: 2,
  consumption: 8,
  roadFactor: 1.3,
  grocery: false,
  ignoreDistance: false,
  markers: true,
  minScore: 0,
  date: "",
  day: 0,
  hour: 12,
};
const limits: Partial<Record<keyof Settings, [number, number]>> = {
  lat: [-38.05, -37.55],
  lon: [144.65, 145.35],
  radius: [0, 100],
  parking: [0, 1],
  fuel: [0, 10],
  consumption: [0, 40],
  roadFactor: [1, 3],
  minScore: [0, 100],
  day: [0, 6],
  hour: [0, 23],
};
function restore(raw: unknown): Partial<Settings> {
  if (!raw || typeof raw !== "object") return {};
  const out: Partial<Settings> = {};
  for (const [key, value] of Object.entries(raw)) {
    const k = key as keyof Settings;
    if (!(k in defaults)) continue;
    if (
      k === "date" &&
      typeof value === "string" &&
      (value === "" || validCalendarDate(value))
    )
      out.date = value;
    if (typeof defaults[k] === "boolean" && typeof value === "boolean")
      Object.assign(out, { [k]: value });
    const bound = limits[k];
    if (
      bound &&
      typeof value === "number" &&
      Number.isFinite(value) &&
      value >= bound[0] &&
      value <= bound[1] &&
      (!["day", "hour"].includes(k) || Number.isInteger(value))
    )
      Object.assign(out, { [k]: value });
  }
  return out;
}
const Context = createContext<{
  settings: Settings;
  ready: boolean;
  update: (value: Partial<Settings>) => void;
  reset: () => void;
} | null>(null);
function localTime() {
  const parts = Object.fromEntries(
    new Intl.DateTimeFormat("en-AU", {
      timeZone: "Australia/Melbourne",
      weekday: "short",
      hour: "numeric",
      hourCycle: "h23",
    })
      .formatToParts(new Date())
      .map((p) => [p.type, p.value]),
  );
  return {
    day: ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"].indexOf(
      parts.weekday,
    ),
    hour: Number(parts.hour),
  };
}
export function Providers({ children }: { children: ReactNode }) {
  const [settings, setSettings] = useState(defaults);
  const [ready, setReady] = useState(false);
  useEffect(() => {
    let saved: Partial<Settings> = {};
    try {
      saved = restore(
        JSON.parse(localStorage.getItem("dash-settings-v1") || "null"),
      );
    } catch {}
    setSettings({ ...defaults, ...localTime(), ...saved });
    setReady(true);
  }, []);
  useEffect(() => {
    if (ready)
      try {
        localStorage.setItem("dash-settings-v1", JSON.stringify(settings));
      } catch {}
  }, [settings, ready]);
  const update = useCallback(
    (value: Partial<Settings>) =>
      setSettings((previous) => ({ ...previous, ...value })),
    [],
  );
  const reset = useCallback(
    () => setSettings({ ...defaults, ...localTime() }),
    [],
  );
  return (
    <Context.Provider value={{ settings, ready, update, reset }}>
      {children}
    </Context.Provider>
  );
}
export function useSettings() {
  const value = useContext(Context);
  if (!value) throw Error("Settings provider missing");
  return value;
}
