"use client";
import { useEffect } from "react";
import {
  MapContainer,
  TileLayer,
  Circle,
  CircleMarker,
  Popup,
  Tooltip,
  useMap,
  useMapEvents,
} from "react-leaflet";
import type { Settings, Venue, Zone } from "@/lib/types";
export type MapProps = {
  settings: Settings;
  venues: Venue[];
  zones: Zone[];
  selecting: boolean;
  onSelect: (lat: number, lon: number) => void;
  focus: Zone | null;
};
function Controls({ settings, selecting, onSelect, focus }: MapProps) {
  const map = useMap();
  useEffect(() => {
    map.panTo([settings.lat, settings.lon]);
  }, [map, settings.lat, settings.lon]);
  useEffect(() => {
    if (focus) map.flyTo([focus.lat, focus.lon], 15, { duration: 0.6 });
  }, [map, focus]);
  useEffect(() => {
    map.getContainer().style.cursor = selecting ? "crosshair" : "";
  }, [map, selecting]);
  useMapEvents({
    click: (e) => {
      if (selecting) onSelect(e.latlng.lat, e.latlng.lng);
    },
  });
  return null;
}
function scoreColor(score: number) {
  return score >= 60 ? "#c6f17b" : score >= 30 ? "#e4c975" : "#a9b5b0";
}
export default function VenueMap(props: MapProps) {
  const { settings, venues, zones } = props;
  return (
    <MapContainer
      center={[settings.lat, settings.lon]}
      zoom={12}
      scrollWheelZoom
      className="venue-map"
      aria-label="Melbourne venue map"
    >
      <TileLayer
        attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
        url="https://tile.openstreetmap.org/{z}/{x}/{y}.png"
      />
      <Controls {...props} />
      {settings.radius > 0 && (
        <Circle
          center={[settings.lat, settings.lon]}
          radius={settings.radius * 1000}
          pathOptions={{
            color: "#c6f17b",
            fillOpacity: 0.025,
            weight: 1.5,
            dashArray: "6 7",
            interactive: false,
          }}
        />
      )}
      {zones.map((z) => (
        <Circle
          key={z.id}
          center={[z.lat, z.lon]}
          radius={450}
          bubblingMouseEvents={false}
          pathOptions={{
            color: scoreColor(z.score),
            fillColor: scoreColor(z.score),
            fillOpacity: 0.17,
            weight: 1,
          }}
        >
          <Popup>
            <strong>{z.name}</strong>
            <p>
              Opportunity score {z.score.toFixed(1)}/100 · {z.venues} venues
            </p>
            <p>
              Estimated activity {z.activity_score.toFixed(2)} · {z.confidence}{" "}
              confidence
            </p>
            <p>
              Open / closed / unknown: {z.confirmed_open} / {z.confirmed_closed}{" "}
              / {z.unknown_hours}
            </p>
          </Popup>
          <Tooltip>
            {z.score.toFixed(0)} · {z.name}
          </Tooltip>
        </Circle>
      ))}
      {settings.markers &&
        venues.map((v) => (
          <CircleMarker
            key={`${v.osm_type}/${v.id}`}
            center={[v.lat, v.lon]}
            radius={3.5}
            bubblingMouseEvents={false}
            pathOptions={{
              color: ["supermarket", "convenience", "grocery"].includes(v.type)
                ? "#ba9ded"
                : "#87b7d1",
              fillOpacity: 0.9,
              weight: 1,
            }}
          >
            <Popup>
              <strong>{v.name}</strong>
              <p>
                {v.type.replaceAll("_", " ")} {v.cuisine && "· " + v.cuisine}
              </p>
              <p>
                {v.activity.status.replaceAll("_", " ")} ·{" "}
                {v.activity.confidence} confidence
              </p>
              <p>
                Estimated activity {v.activity.factor.toFixed(2)} (
                {v.activity.profile})
              </p>
              <p>
                Opening status source:{" "}
                {v.activity.hours_source === "google_maps"
                  ? "Google Maps"
                  : v.activity.hours_source === "osm"
                    ? "OpenStreetMap"
                    : "Unknown"}
              </p>
              {v.activity.hours_text && <p>{v.activity.hours_text}</p>}
              <p>{v.activity.reason}</p>
              <p>OSM hours: {v.opening_hours || "Unknown"}</p>
            </Popup>
          </CircleMarker>
        ))}
      <CircleMarker
        center={[settings.lat, settings.lon]}
        radius={8}
        bubblingMouseEvents={false}
        pathOptions={{
          color: "#17251b",
          fillColor: "#d8ffa1",
          fillOpacity: 1,
          weight: 4,
        }}
      >
        <Tooltip>Your selected location</Tooltip>
      </CircleMarker>
    </MapContainer>
  );
}
