"use client";

import "maplibre-gl/dist/maplibre-gl.css";

import { useEffect, useRef } from "react";

import { gsiStyle } from "./style";

type LatLon = { lat: number; lon: number };
type Props = { value: LatLon | null; onChange: (c: LatLon) => void };

const JAPAN_CENTER: LatLon = { lat: 36.2, lon: 138.25 };

/** 地図をタップ（クリック）して地点を選ぶ。住所の入力なしで登録できるようにする（個人情報の最小化） */
export function MapPicker({ value, onChange }: Props) {
  const ref = useRef<HTMLDivElement>(null);
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;
  const markerRef = useRef<import("maplibre-gl").Marker | null>(null);
  const mapRef = useRef<import("maplibre-gl").Map | null>(null);
  const initial = useRef(value);

  useEffect(() => {
    if (!ref.current) return;
    let map: import("maplibre-gl").Map | undefined;
    let cancelled = false;
    void import("maplibre-gl").then(({ Map, Marker, NavigationControl }) => {
      if (cancelled || !ref.current) return;
      const c = initial.current ?? JAPAN_CENTER;
      map = new Map({ container: ref.current, style: gsiStyle, center: [c.lon, c.lat], zoom: initial.current ? 13 : 4.5, attributionControl: { compact: false } });
      mapRef.current = map;
      map.addControl(new NavigationControl({ showCompass: false }));
      markerRef.current = new Marker({ color: "#1d4ed8" });
      if (initial.current) markerRef.current.setLngLat([c.lon, c.lat]).addTo(map);
      map.on("click", (e) => {
        markerRef.current?.setLngLat(e.lngLat).addTo(map!);
        onChangeRef.current({ lat: Number(e.lngLat.lat.toFixed(6)), lon: Number(e.lngLat.lng.toFixed(6)) });
      });
    });
    return () => {
      cancelled = true;
      mapRef.current = null;
      map?.remove();
    };
  }, []);

  // 住所検索などで外から地点が変わったら、ピンと地図の表示を合わせる
  useEffect(() => {
    const map = mapRef.current;
    const marker = markerRef.current;
    if (!map || !marker || !value) return;
    const cur = marker.getLngLat?.();
    if (cur && Math.abs(cur.lat - value.lat) < 1e-6 && Math.abs(cur.lng - value.lon) < 1e-6) return;
    marker.setLngLat([value.lon, value.lat]).addTo(map);
    map.flyTo({ center: [value.lon, value.lat], zoom: Math.max(map.getZoom(), 14) });
  }, [value]);

  return <div ref={ref} className="map map-picker" role="application" aria-label="地図をタップして地点を選ぶ" />;
}
