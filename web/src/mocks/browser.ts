import { http, HttpResponse } from "msw";
import { setupWorker } from "msw/browser";

import { loadScenario } from "./db";
import { scenarios, type ScenarioName } from "./fixtures";
import { handlers } from "./handlers";

const KEY = "saigai.mockScenario";

/**
 * ブラウザでモック API を動かす（npm run dev:mock と E2E）。
 * URL の ?scenario=heavy-rain などでシナリオを切り替えられる。選んだシナリオはタブを閉じるまで保持する。
 */
export async function startMockWorker() {
  const params = new URLSearchParams(window.location.search);
  let name = params.get("scenario") as ScenarioName | null;
  try {
    if (name && name in scenarios) window.sessionStorage.setItem(KEY, name);
    name = (window.sessionStorage.getItem(KEY) as ScenarioName | null) ?? null;
  } catch {
    /* 保存できない環境では URL の指定だけを使う */
  }
  loadScenario(name && name in scenarios ? name : "normal");
  // 自動テスト（Playwright）のブラウザでは、地理院タイルをダミーの画像に差し替える。
  // Service Worker を経由する通信は Playwright の page.route で止められないため、ここで止める（CI から外部のタイルサーバーを呼ばない）
  const extra = navigator.webdriver ? [blankTiles] : [];
  const worker = setupWorker(...extra, ...handlers);
  await worker.start({ onUnhandledFrame: "bypass", quiet: true });
}

// 1x1 の透明な PNG
const PNG = Uint8Array.from(atob("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="), (c) => c.charCodeAt(0));
const blankTiles = http.get("https://cyberjapandata.gsi.go.jp/*", () => new HttpResponse(PNG, { headers: { "Content-Type": "image/png" } }));
