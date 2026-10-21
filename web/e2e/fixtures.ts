import AxeBuilder from "@axe-core/playwright";
import { test as base, expect, type Page } from "@playwright/test";

// 1x1 の透明な PNG。地図タイルの代わりに返す
const PNG = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=", "base64");

export const test = base.extend<{ page: Page }>({
  page: async ({ page }, use) => {
    // CI から国土地理院のタイルサーバーに負荷をかけない（docs/08 4.2）
    await page.route("https://cyberjapandata.gsi.go.jp/**", (route) => route.fulfill({ status: 200, contentType: "image/png", body: PNG }));
    await use(page);
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
