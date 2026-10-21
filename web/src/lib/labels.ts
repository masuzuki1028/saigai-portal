import type { Hazard, Intensity } from "@/lib/api/client";

export const HAZARDS: Hazard[] = ["rain", "flood", "landslide", "earthquake", "tsunami"];

export const hazardLabel: Record<Hazard, string> = {
  rain: "大雨",
  flood: "洪水",
  landslide: "土砂災害",
  earthquake: "地震",
  tsunami: "津波",
};

/** severity（1=情報, 2=注意報, 3=警報, 4=特別警報）の表示。色だけに頼らず、アイコンと文字でも区別する */
export const severityStyle: Record<number, { label: string; icon: string; className: string }> = {
  0: { label: "平常", icon: "●", className: "level-0" },
  1: { label: "情報", icon: "ⓘ", className: "level-1" },
  2: { label: "注意報", icon: "▲", className: "level-2" },
  3: { label: "警報", icon: "◆", className: "level-3" },
  4: { label: "特別警報", icon: "■", className: "level-4" },
};

/** 地震の severity の表示（震度の目安） */
export const quakeSeverityLabel: Record<number, string> = { 1: "震度1〜2", 2: "震度3〜4", 3: "震度5弱〜5強", 4: "震度6弱以上" };

export const thresholdOptions: Record<"weather" | "tsunami", { value: number; label: string }[]> = {
  weather: [
    { value: 2, label: "注意報以上" },
    { value: 3, label: "警報以上" },
    { value: 4, label: "特別警報のみ" },
    { value: 0, label: "OFF" },
  ],
  tsunami: [
    { value: 2, label: "津波注意報以上" },
    { value: 3, label: "津波警報以上" },
    { value: 4, label: "大津波警報のみ" },
    { value: 0, label: "OFF" },
  ],
};

export const intensityOptions: { value: Intensity; label: string }[] = [
  { value: "3", label: "震度3以上" },
  { value: "4", label: "震度4以上" },
  { value: "5-", label: "震度5弱以上" },
  { value: "6-", label: "震度6弱以上" },
  { value: "off", label: "OFF" },
];

export const radiusOptions = [1000, 3000, 5000, 10000] as const;

export function formatDateTime(iso: string): string {
  return new Intl.DateTimeFormat("ja-JP", { timeZone: "Asia/Tokyo", month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit" }).format(new Date(iso));
}
