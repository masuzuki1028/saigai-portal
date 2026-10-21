import type { Location, TimelineItem } from "@/lib/api/client";

import { scenarios, type ScenarioName } from "./fixtures";

/** モック API のデータ。シナリオを読み込み、登録・更新・削除はメモリ上で反映する */
export const db = {
  scenario: "normal" as ScenarioName,
  locations: new Map<string, Location>(),
  timelines: new Map<string, TimelineItem[]>(),
  emailEnabled: true,
  testNotifications: 0,
  deleted: false,
};

export function loadScenario(name: ScenarioName) {
  const sc = scenarios[name];
  db.scenario = name;
  db.locations = new Map(sc.locations.map((l) => [l.id, structuredClone(l)]));
  db.timelines = new Map(Object.entries(sc.timelines).map(([k, v]) => [k, structuredClone(v)]));
  db.emailEnabled = true;
  db.testNotifications = 0;
  db.deleted = false;
}

let seq = 0;
/** ULID の形式（26文字）のダミー ID */
export function newId(): string {
  seq += 1;
  return ("01JNEW" + String(seq).padStart(20, "0")).slice(0, 26);
}

loadScenario("normal");
