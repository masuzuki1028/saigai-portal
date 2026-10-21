"use client";

import { useState, type FormEvent } from "react";

import type { HazardSettings, Location, RadiusM } from "@/lib/api/client";
import { radiusOptions } from "@/lib/labels";

import { HazardSettingsFields } from "./HazardSettingsFields";
import { MapPicker } from "./map/MapPicker";

export type LocationFormValue = {
  name: string;
  coord: { lat: number; lon: number } | null;
  address: string;
  radiusM: RadiusM;
  hazardSettings: HazardSettings;
};

export function initialValue(defaults: HazardSettings, loc?: Location): LocationFormValue {
  return {
    name: loc?.name ?? "",
    coord: loc?.coord ?? null,
    address: loc?.address ?? "",
    radiusM: loc?.radiusM ?? 3000,
    hazardSettings: loc?.hazardSettings ?? defaults,
  };
}

type Props = {
  initial: LocationFormValue;
  isCoastal: boolean;
  submitLabel: string;
  onSubmit: (v: LocationFormValue & { coord: { lat: number; lon: number } }) => Promise<string | null>;
};

/** 地点の登録・編集フォーム（docs/03 C-04）。住所は任意で、地図のピンだけで登録できる */
export function LocationForm({ initial, isCoastal, submitLabel, onSubmit }: Props) {
  const [v, setV] = useState(initial);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    if (!v.name.trim()) return setError("名前を入力してください");
    if (v.name.trim().length > 40) return setError("名前は40文字以内で入力してください");
    if (!v.coord) return setError("地図で地点を選んでください");
    setBusy(true);
    const msg = await onSubmit({ ...v, coord: v.coord });
    setBusy(false);
    if (msg) setError(msg);
  };

  const useCurrentPosition = () => {
    navigator.geolocation?.getCurrentPosition(
      (p) => setV((x) => ({ ...x, coord: { lat: Number(p.coords.latitude.toFixed(6)), lon: Number(p.coords.longitude.toFixed(6)) } })),
      () => setError("現在地を取得できませんでした。地図で選んでください"),
    );
  };

  return (
    <form onSubmit={submit} noValidate className="form">
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      <label>
        名前
        <input value={v.name} maxLength={40} onChange={(e) => setV({ ...v, name: e.target.value })} placeholder="例: 自宅、実家、学校" />
      </label>

      <fieldset>
        <legend>場所</legend>
        <MapPicker value={v.coord} onChange={(coord) => setV((x) => ({ ...x, coord }))} />
        <p className="muted" aria-live="polite">
          {v.coord ? `選択中: 緯度 ${v.coord.lat} / 経度 ${v.coord.lon}` : "地図をタップして地点を選んでください"}
        </p>
        <button type="button" className="secondary" onClick={useCurrentPosition}>
          現在地を使う
        </button>
      </fieldset>

      <fieldset>
        <legend>半径</legend>
        <div className="radios">
          {radiusOptions.map((r) => (
            <label key={r} className="radio">
              <input type="radio" name="radius" value={r} checked={v.radiusM === r} onChange={() => setV({ ...v, radiusM: r })} />
              {r / 1000}km
            </label>
          ))}
        </div>
      </fieldset>

      <label>
        住所（任意）
        <input value={v.address} maxLength={200} onChange={(e) => setV({ ...v, address: e.target.value })} autoComplete="off" />
        <span className="muted">住所は暗号化して保存します。入力しなくても登録できます。</span>
      </label>

      <HazardSettingsFields value={v.hazardSettings} isCoastal={isCoastal} onChange={(hazardSettings) => setV({ ...v, hazardSettings })} />

      <button type="submit" disabled={busy}>
        {busy ? "保存しています…" : submitLabel}
      </button>
    </form>
  );
}
