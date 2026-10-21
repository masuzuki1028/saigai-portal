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
  const worker = setupWorker(...handlers);
  await worker.start({ onUnhandledFrame: "bypass", quiet: true });
}
