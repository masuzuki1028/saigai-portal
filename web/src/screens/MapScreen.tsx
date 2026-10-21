"use client";

import { useEffect, useState } from "react";

import { AppShell } from "@/components/AppShell";
import { MapView } from "@/components/map/MapView";
import { errorMessage, type Location } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/AuthProvider";

/** 地図ビュー。docs/03 C-03（警報エリアの塗りは今後対応） */
export function MapScreen({ id }: { id: string }) {
  const { api, token } = useAuth();
  const [loc, setLoc] = useState<Location | null>(null);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    if (!token) return;
    void api.GET("/locations/{locationId}", { params: { path: { locationId: id } } }).then(({ data, error }) => {
      if (error || !data) setError(errorMessage(error));
      else setLoc(data);
    });
  }, [api, id, token]);
  return (
    <AppShell title={loc ? `${loc.name}の地図` : "地図"} back={`/app/location?id=${id}`}>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {loc && <MapView lat={loc.coord.lat} lon={loc.coord.lon} radiusM={loc.radiusM} label={`${loc.name}と半径${loc.radiusM / 1000}kmの範囲`} />}
    </AppShell>
  );
}
