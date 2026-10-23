"use client";

import { useEffect, useMemo, useState } from "react";

import { AppShell } from "@/components/AppShell";
import { LevelBadge } from "@/components/LevelBadge";
import { MapView, type MapPin } from "@/components/map/MapView";
import { errorMessage, type Location, type TimelineItem } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/AuthProvider";
import { loadWarnArea, type WarnAreaCollection } from "@/lib/geo/warnArea";
import { formatDateTime } from "@/lib/labels";

const WARNING_HAZARDS = new Set(["rain", "flood", "landslide"]);
const MAX_PINS = 5;

/** 地図ビュー（docs/03 C-03）。地点・半径、警報・注意報の発表区域の塗り、地震の震央のピンを表示する */
export function MapScreen({ id }: { id: string }) {
  const { api, token } = useAuth();
  const [loc, setLoc] = useState<Location | null>(null);
  const [items, setItems] = useState<TimelineItem[]>([]);
  const [area, setArea] = useState<WarnAreaCollection | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!token) return;
    const params = { params: { path: { locationId: id } } };
    void Promise.all([api.GET("/locations/{locationId}", params), api.GET("/locations/{locationId}/timeline", params)]).then(([l, t]) => {
      if (l.error || !l.data) return setError(errorMessage(l.error));
      setLoc(l.data);
      setItems(t.data?.items ?? []);
    });
  }, [api, id, token]);

  useEffect(() => {
    if (!loc?.warnAreaCode) return;
    const ctrl = new AbortController();
    loadWarnArea(loc.warnAreaCode, ctrl.signal)
      .then(setArea)
      .catch(() => setArea(null)); // 区域の形が読めなくても、地点と半径は表示する
    return () => ctrl.abort();
  }, [loc?.warnAreaCode]);

  // 発表中の警報・注意報（大雨・洪水・土砂災害）の最大レベルで区域を塗る
  const warnings = (loc?.currentLevels ?? []).filter((c) => WARNING_HAZARDS.has(c.hazard));
  const areaLevel = Math.max(0, ...warnings.map((c) => c.severity));
  const pins = useMemo<MapPin[]>(() => {
    const seen = new Set<string>();
    const out: MapPin[] = [];
    for (const it of items) {
      if (!it.point || it.status === "cancelled") continue;
      const k = `${it.point.lat},${it.point.lon}`;
      if (seen.has(k)) continue;
      seen.add(k);
      out.push({ lat: it.point.lat, lon: it.point.lon, title: `${it.title}の震央（${formatDateTime(it.issuedAt)}）` });
      if (out.length >= MAX_PINS) break;
    }
    return out;
  }, [items]);

  const areaName = area?.features[0]?.properties.name;
  return (
    <AppShell title={loc ? `${loc.name}の地図` : "地図"} back={`/app/location?id=${id}`}>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {loc && (
        <>
          <MapView
            lat={loc.coord.lat}
            lon={loc.coord.lon}
            radiusM={loc.radiusM}
            area={area}
            areaLevel={areaLevel}
            pins={pins}
            label={`${loc.name}と半径${loc.radiusM / 1000}kmの範囲の地図`}
          />
          <section aria-label="地図に表示している情報" className="map-legend">
            <ul>
              <li>
                <span className="legend-swatch legend-location" aria-hidden="true" /> {loc.name}（半径{loc.radiusM / 1000}km）
              </li>
              {areaName && (
                <li>
                  <span className={`legend-swatch level-fill-${areaLevel}`} aria-hidden="true" /> 発表区域: {areaName}
                  {areas(area)}・{" "}
                  {warnings.length === 0 ? (
                    "発表中の警報・注意報はありません"
                  ) : (
                    warnings.map((w) => (
                      <span key={w.hazard}>
                        <LevelBadge severity={w.severity} hazard={w.hazard} /> {w.title}{" "}
                      </span>
                    ))
                  )}
                </li>
              )}
              {pins.map((p) => (
                <li key={`${p.lat},${p.lon}`}>
                  <span className="legend-swatch legend-pin" aria-hidden="true" /> {p.title}
                </li>
              ))}
            </ul>
          </section>
        </>
      )}
    </AppShell>
  );
}

/** 市町村の中で区域が分かれている場合（富山市の平地・山間部など）の補足 */
function areas(a: WarnAreaCollection | null): string {
  const n = a?.features.length ?? 0;
  return n > 1 ? `ほか（${n}区域）` : "";
}
