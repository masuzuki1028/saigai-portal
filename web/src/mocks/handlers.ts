import { http, HttpResponse } from "msw";

import type { ApiError, LocationInput, LocationUpdate } from "@/lib/api/client";
import { ADDRESS_SEARCH_URL } from "@/lib/geo/addressSearch";

import { db, newId } from "./db";
import { defaultSettings } from "./fixtures";

const api = (path: string) => `*/api${path}`;
const err = (status: number, code: string, message: string) => HttpResponse.json<ApiError>({ code, message }, { status });

function authed(request: Request) {
  return /^Bearer local-/.test(request.headers.get("Authorization") ?? "");
}

const LIMIT = 3;

/** 国土地理院の住所検索のモック（テストから外部サービスを呼ばない） */
const addresses = [
  { title: "新潟県長岡市城内町", lon: 138.851746, lat: 37.445873 },
  { title: "新潟県新潟市中央区西堀通", lon: 139.0364, lat: 37.9161 },
  { title: "東京都千代田区丸の内", lon: 139.7671, lat: 35.6812 },
];

/** 本物の API（api/openapi.yaml）と同じ形で応答するモック */
export const handlers = [
  http.get(ADDRESS_SEARCH_URL, ({ request }) => {
    const q = new URL(request.url).searchParams.get("q") ?? "";
    const hits = addresses.filter((a) => q && a.title.includes(q.replace(/\s/g, "")));
    return HttpResponse.json(
      hits.map((a) => ({ type: "Feature", geometry: { type: "Point", coordinates: [a.lon, a.lat] }, properties: { addressCode: "", title: a.title } })),
    );
  }),

  http.get(api("/health"), () => HttpResponse.json({ status: "ok" })),

  http.get(api("/me"), ({ request }) => {
    if (!authed(request)) return err(401, "unauthorized", "ログインが必要です");
    return HttpResponse.json({ userId: "01JMOCKUSER000000000000000", plan: "free", locationLimit: LIMIT });
  }),

  http.get(api("/me/notification-settings"), ({ request }) => {
    if (!authed(request)) return err(401, "unauthorized", "ログインが必要です");
    return HttpResponse.json({ emailEnabled: db.emailEnabled });
  }),

  http.put(api("/me/notification-settings"), async ({ request }) => {
    if (!authed(request)) return err(401, "unauthorized", "ログインが必要です");
    const body = (await request.json()) as { emailEnabled: boolean };
    db.emailEnabled = body.emailEnabled;
    return HttpResponse.json({ emailEnabled: db.emailEnabled });
  }),

  http.post(api("/me/notification-settings/test"), ({ request }) => {
    if (!authed(request)) return err(401, "unauthorized", "ログインが必要です");
    if (!db.emailEnabled) return err(409, "email_disabled", "メール通知がオフになっています");
    db.testNotifications += 1;
    return new HttpResponse(null, { status: 202 });
  }),

  http.post(api("/me/delete"), async ({ request }) => {
    if (!authed(request)) return err(401, "unauthorized", "ログインが必要です");
    const body = (await request.json()) as { confirm?: string };
    if (body.confirm !== "delete") return err(400, "bad_request", "入力が正しくありません");
    db.locations.clear();
    db.timelines.clear();
    db.deleted = true;
    return new HttpResponse(null, { status: 204 });
  }),

  http.get(api("/locations"), ({ request }) => {
    if (!authed(request)) return err(401, "unauthorized", "ログインが必要です");
    return HttpResponse.json({ items: [...db.locations.values()] });
  }),

  http.post(api("/locations"), async ({ request }) => {
    if (!authed(request)) return err(401, "unauthorized", "ログインが必要です");
    if (db.locations.size >= LIMIT) return err(409, "limit_exceeded", `登録できる地点は ${LIMIT} 件までです`);
    const body = (await request.json()) as LocationInput;
    if (!body.name?.trim()) return err(400, "bad_request", "名前は1〜40文字で入力してください");
    const now = new Date().toISOString();
    const loc = {
      id: newId(),
      name: body.name.trim(),
      coord: body.coord,
      address: body.address,
      radiusM: body.radiusM,
      geohash6: "xn76ur",
      areaName: "長岡市",
      isCoastal: false,
      hazardSettings: body.hazardSettings ?? defaultSettings,
      currentLevels: [],
      version: 1,
      createdAt: now,
      updatedAt: now,
    };
    db.locations.set(loc.id, loc);
    db.timelines.set(loc.id, []);
    return HttpResponse.json(loc, { status: 201 });
  }),

  http.get(api("/locations/:id"), ({ request, params }) => {
    if (!authed(request)) return err(401, "unauthorized", "ログインが必要です");
    const loc = db.locations.get(String(params.id));
    return loc ? HttpResponse.json(loc) : err(404, "not_found", "見つかりません");
  }),

  http.put(api("/locations/:id"), async ({ request, params }) => {
    if (!authed(request)) return err(401, "unauthorized", "ログインが必要です");
    const loc = db.locations.get(String(params.id));
    if (!loc) return err(404, "not_found", "見つかりません");
    const body = (await request.json()) as LocationUpdate;
    if (body.version !== loc.version) return err(409, "conflict", "ほかの画面で更新されています。読み込み直してください");
    const updated = {
      ...loc,
      name: body.name.trim(),
      coord: body.coord,
      address: body.address,
      radiusM: body.radiusM,
      hazardSettings: body.hazardSettings ?? loc.hazardSettings,
      version: loc.version + 1,
      updatedAt: new Date().toISOString(),
    };
    db.locations.set(loc.id, updated);
    return HttpResponse.json(updated);
  }),

  http.delete(api("/locations/:id"), ({ request, params }) => {
    if (!authed(request)) return err(401, "unauthorized", "ログインが必要です");
    if (!db.locations.delete(String(params.id))) return err(404, "not_found", "見つかりません");
    return new HttpResponse(null, { status: 204 });
  }),

  http.get(api("/locations/:id/timeline"), ({ request, params }) => {
    if (!authed(request)) return err(401, "unauthorized", "ログインが必要です");
    const id = String(params.id);
    if (!db.locations.has(id)) return err(404, "not_found", "見つかりません");
    return HttpResponse.json({ items: db.timelines.get(id) ?? [] });
  }),
];
