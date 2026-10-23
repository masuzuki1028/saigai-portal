"use client";

import "maplibre-gl/dist/maplibre-gl.css";

import type { GeoJSONSource, Map as MlMap, Marker as MlMarker } from "maplibre-gl";
import { useEffect, useRef, useState } from "react";

import type { WarnAreaCollection } from "@/lib/geo/warnArea";

import { loadMaplibre } from "./maplibre";
import { circleBounds, circlePolygon, gsiStyle, levelFill } from "./style";

export type MapPin = { lat: number; lon: number; title: string };

type Props = {
  lat: number;
  lon: number;
  radiusM: number;
  label?: string;
  /** 塗る区域（警報・注意報の発表区域） */
  area?: WarnAreaCollection | null;
  /** 区域の現在のレベル（0 は平常） */
  areaLevel?: number;
  /** 震央などのピン */
  pins?: MapPin[];
};

const EMPTY: WarnAreaCollection = { type: "FeatureCollection", features: [] };

/**
 * 地点と半径の円、発表区域の塗り、震央のピンを表示する地図。地図は地点ごとに一度だけ作り、
 * 区域とピンはデータが届いたあとで更新する。MapLibre は WebGL が必要なため、テストではモックに差し替える。
 */
export function MapView({ lat, lon, radiusM, label, area, areaLevel = 0, pins = [] }: Props) {
  const ref = useRef<HTMLDivElement>(null);
  const mapRef = useRef<MlMap | null>(null);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    if (!ref.current) return;
    let map: MlMap | undefined;
    let cancelled = false;
    void loadMaplibre().then(({ Map, Marker, NavigationControl }) => {
      if (cancelled || !ref.current) return;
      // 半径の円全体が見えるように表示範囲を合わせる
      map = new Map({
        container: ref.current,
        style: gsiStyle,
        bounds: circleBounds(lat, lon, radiusM),
        fitBoundsOptions: { padding: 24 },
        attributionControl: { compact: false },
      });
      mapRef.current = map;
      map.addControl(new NavigationControl({ showCompass: false }));
      new Marker({ color: "#1d4ed8" }).setLngLat([lon, lat]).addTo(map);
      map.on("load", () => {
        if (!map) return;
        map.addSource("warn-area", { type: "geojson", data: EMPTY });
        map.addLayer({ id: "warn-area-fill", type: "fill", source: "warn-area", paint: { "fill-color": levelFill[0]!, "fill-opacity": 0.08 } });
        map.addLayer({ id: "warn-area-line", type: "line", source: "warn-area", paint: { "line-color": levelFill[0]!, "line-width": 1.5 } });
        map.addSource("radius", { type: "geojson", data: circlePolygon(lat, lon, radiusM) });
        map.addLayer({ id: "radius-fill", type: "fill", source: "radius", paint: { "fill-color": "#2563eb", "fill-opacity": 0.12 } });
        map.addLayer({ id: "radius-line", type: "line", source: "radius", paint: { "line-color": "#2563eb", "line-width": 2 } });
        setLoaded(true);
      });
    });
    return () => {
      cancelled = true;
      setLoaded(false);
      mapRef.current = null;
      map?.remove();
    };
  }, [lat, lon, radiusM]);

  // 発表区域の塗り（レベルの色）
  useEffect(() => {
    const map = mapRef.current;
    if (!loaded || !map) return;
    const color = levelFill[areaLevel] ?? levelFill[0]!;
    (map.getSource("warn-area") as GeoJSONSource | undefined)?.setData(area ?? EMPTY);
    map.setPaintProperty("warn-area-fill", "fill-color", color);
    map.setPaintProperty("warn-area-fill", "fill-opacity", areaLevel > 0 ? 0.35 : 0.08);
    map.setPaintProperty("warn-area-line", "line-color", color);
  }, [loaded, area, areaLevel]);

  // 震央のピン。地点から離れているときは、両方が見えるように表示範囲を広げる
  useEffect(() => {
    const map = mapRef.current;
    if (!loaded || !map || pins.length === 0) return;
    let markers: MlMarker[] = [];
    let cancelled = false;
    void loadMaplibre().then(({ Marker, Popup, LngLatBounds }) => {
      if (cancelled) return;
      markers = pins.map((p) =>
        new Marker({ color: "#b91c1c" })
          .setLngLat([p.lon, p.lat])
          .setPopup(new Popup({ offset: 24 }).setText(p.title))
          .addTo(map),
      );
      const b = new LngLatBounds(...circleBounds(lat, lon, radiusM));
      for (const p of pins) b.extend([p.lon, p.lat]);
      map.fitBounds(b, { padding: 48, duration: 0 });
    });
    return () => {
      cancelled = true;
      markers.forEach((m) => m.remove());
    };
  }, [loaded, pins, lat, lon, radiusM]);

  // ピンは操作できる要素（説明を表示するボタン）を含むため、画像（img）ではなく領域（region）にする
  return <div ref={ref} className="map" role="region" aria-label={label ?? "地図"} data-loaded={loaded ? "true" : "false"} />;
}
