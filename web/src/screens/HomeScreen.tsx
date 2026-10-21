"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

import { AppShell } from "@/components/AppShell";
import { LocationCard } from "@/components/LocationCard";
import { errorMessage, type Location } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/AuthProvider";
import { usePolling } from "@/lib/hooks/usePolling";

/** ホーム（地点一覧）。docs/03 C-01 */
export function HomeScreen() {
  const { api, token } = useAuth();
  const [items, setItems] = useState<Location[] | null>(null);
  const [limit, setLimit] = useState(3);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    const [locs, me] = await Promise.all([api.GET("/locations"), api.GET("/me")]);
    if (locs.error || !locs.data) return setError(errorMessage(locs.error));
    setError(null);
    setItems(locs.data.items);
    if (me.data) setLimit(me.data.locationLimit);
  }, [api]);

  useEffect(() => {
    if (token) void load();
  }, [load, token]);
  const alerting = (items ?? []).filter((l) => l.currentLevels.some((c) => c.severity >= 3));
  usePolling(() => void load(), alerting.length > 0, !!token);

  return (
    <AppShell title="ホーム">
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {alerting.length > 0 && (
        <p className="banner" role="status">
          警報以上の情報が出ている地点: {alerting.length}
        </p>
      )}
      {items === null ? (
        <p className="muted">読み込んでいます…</p>
      ) : (
        <>
          {items.length === 0 && <p className="empty">まだ地点が登録されていません。気になる場所を登録しましょう。</p>}
          <ul className="cards" aria-label="登録した地点">
            {items.map((l) => (
              <LocationCard key={l.id} loc={l} />
            ))}
          </ul>
          {items.length < limit ? (
            <Link href="/app/location/edit" className="button">
              ＋ 地点を追加
            </Link>
          ) : (
            <p className="muted">無料プランで登録できる地点は {limit} 件までです。</p>
          )}
        </>
      )}
    </AppShell>
  );
}
