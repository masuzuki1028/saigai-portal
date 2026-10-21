"use client";

import "maplibre-gl/dist/maplibre-gl.css";

import { useEffect, useRef } from "react";

import { circlePolygon, gsiStyle, zoomForRadius } from "./style";

type Props = { lat: number; lon: number; radiusM: number; label?: string };

/** 地点と半径の円を表示する地図。MapLibre は WebGL が必要なため、テストではモックに差し替える */
export function MapView({ lat, lon, radiusM, label }: Props) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!ref.current) return;
    let map: import("maplibre-gl").Map | undefined;
    let cancelled = false;
    void import("maplibre-gl").then(({ Map, Marker, NavigationControl }) => {
      if (cancelled || !ref.current) return;
      map = new Map({ container: ref.current, style: gsiStyle, center: [lon, lat], zoom: zoomForRadius(radiusM), attributionControl: { compact: false } });
      map.addControl(new NavigationControl({ showCompass: false }));
      new Marker({ color: "#1d4ed8" }).setLngLat([lon, lat]).addTo(map);
      map.on("load", () => {
        map?.addSource("radius", { type: "geojson", data: circlePolygon(lat, lon, radiusM) });
        map?.addLayer({ id: "radius-fill", type: "fill", source: "radius", paint: { "fill-color": "#2563eb", "fill-opacity": 0.12 } });
        map?.addLayer({ id: "radius-line", type: "line", source: "radius", paint: { "line-color": "#2563eb", "line-width": 2 } });
      });
    });
    return () => {
      cancelled = true;
      map?.remove();
    };
  }, [lat, lon, radiusM]);
  return <div ref={ref} className="map" role="img" aria-label={label ?? "地図"} />;
}
