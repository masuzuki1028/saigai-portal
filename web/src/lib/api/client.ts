import createClient, { type Middleware } from "openapi-fetch";

import type { components, paths } from "./schema";

export type Location = components["schemas"]["Location"];
export type LocationInput = components["schemas"]["LocationInput"];
export type LocationUpdate = components["schemas"]["LocationUpdate"];
export type TimelineItem = components["schemas"]["TimelineItem"];
export type HazardSettings = components["schemas"]["HazardSettings"];
export type Hazard = components["schemas"]["Hazard"];
export type Intensity = components["schemas"]["Intensity"];
export type RadiusM = components["schemas"]["RadiusM"];
export type CurrentLevel = components["schemas"]["CurrentLevel"];
export type ApiError = components["schemas"]["Error"];

/** API のベース URL。本番は CloudFront で同一オリジンの /api。ローカルは NEXT_PUBLIC_API_BASE で指定する */
export function apiBase(): string {
  const base = process.env.NEXT_PUBLIC_API_BASE ?? "/api";
  const origin = typeof window === "undefined" ? "http://localhost" : window.location.origin;
  return new URL(base, origin).toString().replace(/\/$/, "");
}

export type Api = ReturnType<typeof createApi>;

/** トークンを付けて API を呼ぶクライアント。401 が返ったら onUnauthorized を呼ぶ */
export function createApi(getToken: () => Promise<string | null>, onUnauthorized: () => void) {
  const client = createClient<paths>({ baseUrl: apiBase() });
  const auth: Middleware = {
    async onRequest({ request }) {
      const token = await getToken();
      if (token) request.headers.set("Authorization", `Bearer ${token}`);
      return request;
    },
    onResponse({ response }) {
      if (response.status === 401) onUnauthorized();
      return response;
    },
  };
  client.use(auth);
  return client;
}

/** API のエラーを画面に出す文言にする */
export function errorMessage(error: unknown): string {
  if (error && typeof error === "object" && "message" in error && typeof error.message === "string") return error.message;
  return "通信に失敗しました。時間をおいてもう一度お試しください";
}
