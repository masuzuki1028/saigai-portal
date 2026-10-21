import type { HazardSettings, Location, TimelineItem } from "@/lib/api/client";

/**
 * シナリオ単位のモックデータ（docs/08_test_strategy.md 3章）。
 * 画面の開発（npm run dev:mock）・Vitest・Playwright で共通に使う。
 */
export type ScenarioName = "normal" | "heavy-rain" | "earthquake" | "tsunami" | "empty";

export type ScenarioData = {
  label: string;
  locations: Location[];
  timelines: Record<string, TimelineItem[]>;
};

export const defaultSettings: HazardSettings = {
  rain: { display: 2, notify: 3 },
  flood: { display: 2, notify: 3 },
  landslide: { display: 3, notify: 3 },
  earthquake: { displayMinIntensity: "3", notifyMinIntensity: "5-" },
  tsunami: { display: 2, notify: 3 },
};

const t = (iso: string) => new Date(iso).toISOString();

function location(id: string, name: string, areaName: string, lat: number, lon: number, extra: Partial<Location> = {}): Location {
  return {
    id,
    name,
    coord: { lat, lon },
    radiusM: 3000,
    geohash6: "xn76ur",
    areaName,
    isCoastal: false,
    hazardSettings: defaultSettings,
    currentLevels: [],
    version: 1,
    createdAt: t("2026-10-01T00:00:00Z"),
    updatedAt: t("2026-10-01T00:00:00Z"),
    ...extra,
  };
}

const HOME = "01JHOME0000000000000000000";
const OFFICE = "01JOFFICE00000000000000000";
const PARENTS = "01JPARENTS0000000000000000";

function item(id: string, issuedAt: string, extra: Partial<TimelineItem>): TimelineItem {
  return {
    id,
    kind: "official",
    hazard: "rain",
    severity: 2,
    reliability: "official",
    title: "大雨注意報",
    summary: "",
    status: "issued",
    source: "気象庁",
    sourceUrl: "https://www.jma.go.jp/bosai/warning/",
    issuedAt: t(issuedAt),
    notified: false,
    ...extra,
  };
}

export const scenarios: Record<ScenarioName, ScenarioData> = {
  empty: { label: "地点なし", locations: [], timelines: {} },
  normal: {
    label: "平常時",
    locations: [location(HOME, "自宅", "千代田区", 35.6812, 139.7671), location(PARENTS, "実家", "長岡市", 37.4462, 138.8512)],
    timelines: {
      [HOME]: [],
      [PARENTS]: [
        item("p1", "2026-10-18T10:00:00+09:00", { title: "大雨注意報", summary: "大雨注意報（長岡市）。中越では、大雨に注意してください。" }),
        item("p2", "2026-10-18T18:00:00+09:00", { title: "大雨の警報・注意報の解除", status: "cleared", summary: "大雨の警報・注意報の解除（長岡市）。" }),
      ].reverse(),
    },
  },
  "heavy-rain": {
    label: "大雨警報の発表中",
    locations: [
      location(PARENTS, "実家", "長岡市", 37.4462, 138.8512, {
        currentLevels: [
          { hazard: "rain", severity: 3, title: "大雨警報" },
          { hazard: "flood", severity: 3, title: "洪水警報" },
          { hazard: "landslide", severity: 3, title: "大雨警報（土砂災害）" },
        ],
      }),
      location(HOME, "自宅", "千代田区", 35.6812, 139.7671),
    ],
    timelines: {
      [PARENTS]: [
        item("r3", "2026-07-10T12:00:00+09:00", { hazard: "landslide", severity: 3, title: "大雨警報（土砂災害）", summary: "大雨警報（土砂災害）（長岡市）。中越では、土砂災害や低い土地の浸水、河川の増水に警戒してください。", notified: true }),
        item("r2", "2026-07-10T12:00:00+09:00", { hazard: "flood", severity: 3, title: "洪水警報", summary: "洪水警報（長岡市）。中越では、河川の増水に警戒してください。", notified: true }),
        item("r1", "2026-07-10T12:00:00+09:00", { severity: 3, title: "大雨警報", summary: "大雨警報（長岡市）。中越では、土砂災害や低い土地の浸水に警戒してください。", notified: true }),
        item("r0", "2026-07-10T10:00:00+09:00", { title: "大雨注意報", summary: "大雨注意報（長岡市）。中越では、大雨や洪水に注意してください。" }),
      ],
      [HOME]: [],
    },
  },
  tsunami: {
    label: "津波警報の発表中",
    locations: [
      location(OFFICE, "新潟の職場", "新潟市中央区", 37.9161, 139.0364, {
        isCoastal: true,
        tsunamiAreaName: "新潟県上中下越",
        currentLevels: [{ hazard: "tsunami", severity: 3, title: "津波警報" }],
      }),
      location(PARENTS, "実家", "長岡市", 37.4462, 138.8512),
    ],
    timelines: {
      [OFFICE]: [
        item("t2", "2026-09-01T10:35:00+09:00", {
          hazard: "tsunami", severity: 3, title: "津波警報", notified: true,
          summary: "津波警報（新潟県上中下越）。10時40分ころ到達予想。予想される津波の高さ: 3m。新潟県上中下越に津波警報を発表しました。ただちに避難してください。",
          sourceUrl: "https://www.jma.go.jp/bosai/map.html#contents=tsunami",
        }),
        item("t1", "2026-09-01T10:13:00+09:00", {
          hazard: "tsunami", severity: 2, title: "津波注意報",
          summary: "津波注意報（新潟県上中下越）。10時40分ころ到達予想。予想される津波の高さ: 1m。日本海沿岸に津波注意報を発表しました。",
          sourceUrl: "https://www.jma.go.jp/bosai/map.html#contents=tsunami",
        }),
      ],
      [PARENTS]: [],
    },
  },
  earthquake: {
    label: "震度6弱の地震",
    locations: [
      location(PARENTS, "実家", "長岡市", 37.4462, 138.8512, {
        currentLevels: [{ hazard: "earthquake", severity: 4, title: "地震（長岡市 震度6弱）" }],
      }),
    ],
    timelines: {
      [PARENTS]: [
        item("q2", "2026-08-01T03:25:00+09:00", {
          hazard: "earthquake", severity: 4, title: "地震（長岡市 震度6弱）", notified: true,
          summary: "8月1日 03時12分 ころ、新潟県中越地方 を震源とする地震がありました（Ｍ６．５）。長岡市 で震度6弱を観測しました。",
          sourceUrl: "https://www.jma.go.jp/bosai/map.html#contents=earthquake_map",
        }),
        item("q1", "2026-08-01T03:15:00+09:00", {
          hazard: "earthquake", severity: 3, title: "地震（長岡市 震度5強）", notified: true,
          summary: "8月1日 03時12分 ころ、新潟県中越地方 を震源とする地震がありました（Ｍ６．５）。長岡市 で震度5強を観測しました。",
          sourceUrl: "https://www.jma.go.jp/bosai/map.html#contents=earthquake_map",
        }),
      ],
    },
  },
};

export const IDS = { HOME, PARENTS, OFFICE };
