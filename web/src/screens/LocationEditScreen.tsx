"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { AppShell } from "@/components/AppShell";
import { initialValue, LocationForm, type LocationFormValue } from "@/components/LocationForm";
import { errorMessage, type HazardSettings, type Location } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/AuthProvider";

/** 新しい地点の初期値（docs/01_concept_overview.md 4.2） */
export const DEFAULT_SETTINGS: HazardSettings = {
  rain: { display: 2, notify: 3 },
  flood: { display: 2, notify: 3 },
  landslide: { display: 3, notify: 3 },
  earthquake: { displayMinIntensity: "3", notifyMinIntensity: "5-" },
  tsunami: { display: 2, notify: 3 },
};

/** 地点の登録・編集。id がなければ新規登録。docs/03 C-04 */
export function LocationEditScreen({ id }: { id: string | null }) {
  const { api, token } = useAuth();
  const router = useRouter();
  const [loc, setLoc] = useState<Location | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);

  useEffect(() => {
    if (!id || !token) return;
    void api.GET("/locations/{locationId}", { params: { path: { locationId: id } } }).then(({ data, error }) => {
      if (error || !data) setLoadError(errorMessage(error));
      else setLoc(data);
    });
  }, [api, id, token]);

  const submit = async (v: LocationFormValue & { coord: { lat: number; lon: number } }): Promise<string | null> => {
    const body = { name: v.name.trim(), coord: v.coord, radiusM: v.radiusM, hazardSettings: v.hazardSettings, ...(v.address.trim() ? { address: v.address.trim() } : {}) };
    const res = id && loc
      ? await api.PUT("/locations/{locationId}", { params: { path: { locationId: id } }, body: { ...body, version: loc.version } })
      : await api.POST("/locations", { body });
    if (res.error || !res.data) return errorMessage(res.error);
    router.push(`/app/location?id=${res.data.id}`);
    return null;
  };

  const remove = async () => {
    if (!id || !window.confirm("この地点を削除しますか？タイムラインも見られなくなります")) return;
    const { error } = await api.DELETE("/locations/{locationId}", { params: { path: { locationId: id } } });
    if (error) return setLoadError(errorMessage(error));
    router.push("/app");
  };

  const editing = !!id;
  return (
    <AppShell title={editing ? "地点を編集" : "地点を登録"} back={editing ? `/app/location?id=${id}` : "/app"}>
      {loadError && (
        <p role="alert" className="error">
          {loadError}
        </p>
      )}
      {(!editing || loc) && (
        <LocationForm
          key={loc?.id ?? "new"}
          initial={initialValue(DEFAULT_SETTINGS, loc ?? undefined)}
          isCoastal={loc?.isCoastal ?? false}
          submitLabel={editing ? "保存" : "登録"}
          onSubmit={submit}
        />
      )}
      {editing && loc && (
        <button type="button" className="danger" onClick={remove}>
          この地点を削除
        </button>
      )}
    </AppShell>
  );
}
