import AxeBuilder from "@axe-core/playwright";
import { test as base, expect, type Page } from "@playwright/test";

// 1x1 の透明な PNG。地図タイルの代わりに返す
const PNG = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=", "base64");

const isLocal = (url: string) => ["127.0.0.1", "localhost"].includes(new URL(url).hostname);

export const test = base.extend<{ page: Page }>({
  page: async ({ page }, use) => {
    // CI から外部のサーバー（地理院タイルなど）を呼ばない（docs/08 4.2）。
    // - ページからの通信: 地理院タイルはダミーの画像を返し、それ以外の外部への通信は止めて記録する
    // - Service Worker（MSW）からの通信: page.route では止められないため、MSW 側で地理院タイルを止めている（src/mocks/browser.ts）。
    //   それでも外部に出ていった通信を記録する
    const external: string[] = [];
    await page.route(
      (url) => !isLocal(url.toString()),
      (route) => {
        const host = new URL(route.request().url()).hostname;
        if (host === "cyberjapandata.gsi.go.jp") return route.fulfill({ status: 200, contentType: "image/png", body: PNG });
        external.push(host);
        return route.abort();
      },
    );
    page.context().on("request", (req) => {
      if (req.serviceWorker() && !isLocal(req.url())) external.push(`${new URL(req.url()).hostname}（Service Worker 経由）`);
    });
    await use(page);
    expect(external, "外部のサーバーへの通信").toEqual([]);
  },
});

export { expect };

/** シナリオを選んでから、ローカル用のユーザーでログインする */
export async function login(page: Page, scenario = "normal") {
  await page.goto(`/login?scenario=${scenario}`);
  await page.getByRole("button", { name: /alice/ }).click();
  await expect(page.getByRole("heading", { name: "ホーム" })).toBeVisible();
}

/** 重大なアクセシビリティの違反がないこと */
export async function expectNoSeriousA11yViolations(page: Page) {
  const result = await new AxeBuilder({ page }).exclude(".maplibregl-canvas").analyze();
  const serious = result.violations.filter((v) => v.impact === "serious" || v.impact === "critical");
  expect(serious.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
}
