"use client";

import type { HazardSettings, Intensity } from "@/lib/api/client";
import { intensityOptions, thresholdOptions } from "@/lib/labels";

type LevelKey = "rain" | "flood" | "landslide" | "tsunami";

const rows: { key: LevelKey; label: string; options: "weather" | "tsunami" }[] = [
  { key: "rain", label: "大雨", options: "weather" },
  { key: "flood", label: "洪水", options: "weather" },
  { key: "landslide", label: "土砂災害", options: "weather" },
];

/** 災害種別 × レベルの設定（表示 / 通知）。docs/01_concept_overview.md 4.2 */
export function HazardSettingsFields({ value, onChange, isCoastal }: { value: HazardSettings; onChange: (v: HazardSettings) => void; isCoastal: boolean }) {
  const setLevel = (key: LevelKey, field: "display" | "notify", v: number) => onChange({ ...value, [key]: { ...value[key], [field]: v } });
  const setQuake = (field: "displayMinIntensity" | "notifyMinIntensity", v: Intensity) =>
    onChange({ ...value, earthquake: { ...value.earthquake, [field]: v } });

  const levelRows = isCoastal ? [...rows, { key: "tsunami" as const, label: "津波", options: "tsunami" as const }] : rows;
  return (
    <fieldset className="settings">
      <legend>見る災害とレベル</legend>
      <table>
        <thead>
          <tr>
            <th scope="col">種別</th>
            <th scope="col">表示</th>
            <th scope="col">通知</th>
          </tr>
        </thead>
        <tbody>
          {levelRows.map((r) => (
            <tr key={r.key}>
              <th scope="row">{r.label}</th>
              {(["display", "notify"] as const).map((f) => (
                <td key={f}>
                  <select aria-label={`${r.label}の${f === "display" ? "表示" : "通知"}`} value={value[r.key][f]} onChange={(e) => setLevel(r.key, f, Number(e.target.value))}>
                    {thresholdOptions[r.options].map((o) => (
                      <option key={o.value} value={o.value}>
                        {o.label}
                      </option>
                    ))}
                  </select>
                </td>
              ))}
            </tr>
          ))}
          <tr>
            <th scope="row">地震</th>
            {(["displayMinIntensity", "notifyMinIntensity"] as const).map((f) => (
              <td key={f}>
                <select aria-label={`地震の${f === "displayMinIntensity" ? "表示" : "通知"}`} value={value.earthquake[f]} onChange={(e) => setQuake(f, e.target.value as Intensity)}>
                  {intensityOptions.map((o) => (
                    <option key={o.value} value={o.value}>
                      {o.label}
                    </option>
                  ))}
                </select>
              </td>
            ))}
          </tr>
        </tbody>
      </table>
      {!isCoastal && <p className="muted">津波の設定は、海岸から約5km以内の地点にだけ表示します（登録後に判定します）。</p>}
    </fieldset>
  );
}
