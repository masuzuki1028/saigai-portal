"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

import { AppShell } from "@/components/AppShell";
import { LevelBadge } from "@/components/LevelBadge";
import { Timeline } from "@/components/Timeline";
import { errorMessage, type Location, type TimelineItem } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/AuthProvider";
import { usePolling } from "@/lib/hooks/usePolling";

/** 地点タイムライン。docs/03 C-02 */
export function TimelineScreen({ id }: { id: string }) {
  const { api, token } = useAuth();
  const [loc, setLoc] = useState<Location | null>(null);
  const [items, setItems] = useState<TimelineItem[]>([]);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    const params = { params: { path: { locationId: id } } };
    const [l, t] = await Promise.all([api.GET("/locations/{locationId}", params), api.GET("/locations/{locationId}/timeline", params)]);
    if (l.response?.status === 404) return setError("地点が見つかりません");
    if (l.error || !l.data || t.error || !t.data) return setError(errorMessage(l.error ?? t.error));
    setError(null);
    setLoc(l.data);
    setItems(t.data.items);
  }, [api, id]);

  useEffect(() => {
    if (token) void load();
  }, [load, token]);
  usePolling(() => void load(), (loc?.currentLevels ?? []).some((c) => c.severity >= 3), !!token && !!loc);

  return (
    <AppShell title={loc ? `${loc.name}（半径${loc.radiusM / 1000}km）` : "タイムライン"} back="/app">
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {loc && (
        <>
          <div className="toolbar">
            <span className="muted">{loc.areaName}</span>
            <Link href={`/app/map?id=${loc.id}`}>地図</Link>
            <Link href={`/app/location/edit?id=${loc.id}`}>編集</Link>
          </div>
          {loc.currentLevels.length > 0 && (
            <section aria-label="発表中の情報" className="current">
              {loc.currentLevels.map((c) => (
                <p key={c.hazard}>
                  <LevelBadge severity={c.severity} hazard={c.hazard} /> {c.title}
                </p>
              ))}
            </section>
          )}
          <Timeline items={items} />
        </>
      )}
    </AppShell>
  );
}
