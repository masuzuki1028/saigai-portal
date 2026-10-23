import { expect, expectNoSeriousA11yViolations, login, test } from "./fixtures";

// MVP の主要シナリオ（docs/08 4.2）

test("1. ログイン → 地点登録（地図にピンを置く）→ ホームに表示される", async ({ page }) => {
  await login(page, "empty");
  await expect(page.getByText("まだ地点が登録されていません")).toBeVisible();
  await page.getByRole("link", { name: "＋ 地点を追加" }).click();

  await page.getByRole("textbox", { name: "名前" }).fill("実家");
  const map = page.getByRole("application", { name: "地図をタップして地点を選ぶ" });
  await expect(map.locator("canvas")).toBeVisible();
  await map.click({ position: { x: 150, y: 150 } });
  await expect(page.getByText(/選択中: 緯度/)).toBeVisible();
  await page.getByRole("radio", { name: "5km" }).check();
  await expectNoSeriousA11yViolations(page);
  await page.getByRole("button", { name: "登録" }).click();

  await expect(page.getByRole("heading", { name: "実家（半径5km）" })).toBeVisible();
  await page.getByRole("link", { name: "戻る" }).click();
  await expect(page.getByRole("link", { name: /実家/ })).toBeVisible();
});

test("2. 警報発表中のシナリオで、タイムラインにレベル・出典・免責が表示される", async ({ page }) => {
  await login(page, "heavy-rain");
  await expect(page.getByRole("status")).toHaveText("警報以上の情報が出ている地点: 1");
  await expectNoSeriousA11yViolations(page);
  await page.getByRole("link", { name: /実家/ }).click();

  const timeline = page.getByRole("region", { name: "タイムライン" });
  const first = timeline.getByRole("listitem").first();
  await expect(first).toContainText("【公式】");
  await expect(first).toContainText(/土砂災害\s*警報/);
  await expect(first).toContainText("出典: 気象庁");
  await expect(page.getByRole("note")).toContainText("避難の判断は、自治体・気象庁の公式情報に従ってください");
  await expectNoSeriousA11yViolations(page);
});

test("3. 通知設定を変更してテスト送信する", async ({ page }) => {
  await login(page);
  await page.getByRole("link", { name: "通知" }).click();
  await page.getByRole("button", { name: "テスト通知を送る" }).click();
  await expect(page.getByRole("status")).toHaveText(/テスト通知を送信しました/);
  await page.getByRole("checkbox", { name: /メールで通知する/ }).uncheck();
  await expect(page.getByRole("status")).toHaveText("メール通知をオフにしました");
  await expectNoSeriousA11yViolations(page);
});

test("4. ログアウトすると、保護された画面にアクセスできない", async ({ page }) => {
  await login(page);
  await page.getByRole("button", { name: /ログアウト/ }).click();
  await expect(page).toHaveURL(/\/login/);
  await page.goto("/app/settings/notifications");
  await expect(page).toHaveURL(/\/login/);
  await expect(page.getByRole("heading", { name: "ログイン" })).toBeVisible();
});

test("地震のシナリオで、震度とレベルが表示される", async ({ page }) => {
  await login(page, "earthquake");
  await page.getByRole("link", { name: /実家/ }).click();
  await expect(page.getByRole("region", { name: "発表中の情報" })).toContainText("地震（長岡市 震度6弱）");
  await expect(page.getByRole("region", { name: "タイムライン" }).getByRole("listitem").first()).toContainText("震度6弱以上");
});

test("津波警報の発表中は、沿岸の地点に津波予報区とレベルが表示される", async ({ page }) => {
  await login(page, "tsunami");
  await page.getByRole("link", { name: /新潟の職場/ }).click();
  await expect(page.getByText("津波予報区: 新潟県上中下越")).toBeVisible();
  const first = page.getByRole("region", { name: "タイムライン" }).getByRole("listitem").first();
  await expect(first).toContainText(/津波\s*警報/);
  await expect(first).toContainText("到達予想");
  await expectNoSeriousA11yViolations(page);
});

test("退会すると、トップに戻りログアウトする", async ({ page }) => {
  await login(page);
  await page.getByRole("link", { name: "アカウント" }).click();
  await page.getByRole("textbox", { name: /「削除」と入力/ }).fill("削除");
  await expectNoSeriousA11yViolations(page);
  await page.getByRole("button", { name: "退会してデータを削除する" }).click();
  await expect(page.getByRole("status")).toHaveText(/退会しました/);
  await page.goto("/app");
  await expect(page).toHaveURL(/\/login/);
});

test("住所で探して地点を登録できる（地図を操作しなくてよい）", async ({ page }) => {
  await login(page, "empty");
  await page.getByRole("link", { name: "＋ 地点を追加" }).click();
  await page.getByRole("textbox", { name: "名前" }).fill("実家");
  await page.getByRole("textbox", { name: "住所で探す" }).fill("長岡市");
  await page.getByRole("button", { name: "検索", exact: true }).click();
  await page.getByRole("button", { name: "新潟県長岡市城内町" }).click();
  await expect(page.getByText("選択中: 緯度 37.445873 / 経度 138.851746")).toBeVisible();
  await expectNoSeriousA11yViolations(page);
  await page.getByRole("button", { name: "登録" }).click();
  await expect(page.getByRole("heading", { name: "実家（半径3km）" })).toBeVisible();
});

test("利用規約・プライバシーポリシー・security.txt を表示できる", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("link", { name: "プライバシーポリシー" }).click();
  await expect(page.getByRole("heading", { name: "プライバシーポリシー", level: 1 })).toBeVisible();
  await expect(page.getByRole("cell", { name: "国土地理院 住所検索" })).toBeVisible();
  await expectNoSeriousA11yViolations(page);
  await page.getByRole("link", { name: "利用規約" }).click();
  await expect(page.getByRole("heading", { name: "利用規約", level: 1 })).toBeVisible();
  await expectNoSeriousA11yViolations(page);

  const res = await page.request.get("/.well-known/security.txt");
  expect(res.ok()).toBe(true);
  expect(await res.text()).toContain("Contact: https://github.com/masuzuki1028/saigai-portal/security/advisories/new");
});

test("地図に、警報の発表区域を発表中のレベルで塗る", async ({ page }) => {
  // MapLibre の Worker が読み込めないと、地図の図形（区域の塗り・半径の円）が描画されない
  const errors: string[] = [];
  page.on("console", (m) => m.type() === "error" && errors.push(m.text()));
  await login(page, "heavy-rain");
  await page.getByRole("link", { name: /実家/ }).click();
  await page.getByRole("link", { name: "地図" }).click();
  const legend = page.getByRole("region", { name: "地図に表示している情報" });
  await expect(legend).toContainText("発表区域: 新潟県長岡市");
  await expect(legend).toContainText("大雨警報");
  await expect(page.locator(".maplibregl-canvas")).toBeVisible();
  await expect(page.locator('[data-loaded="true"]')).toBeVisible();
  await expectNoSeriousA11yViolations(page);
  await page.waitForTimeout(500); // 塗りの描画を待ってから撮影する（目視の確認用）
  await page.screenshot({ path: "test-results/map-heavy-rain.png" });
  expect(errors).toEqual([]);
});

test("地図に、地震の震央のピンを表示する", async ({ page }) => {
  await login(page, "earthquake");
  await page.getByRole("link", { name: /実家/ }).click();
  await page.getByRole("link", { name: "地図" }).click();
  await expect(page.getByRole("region", { name: "地図に表示している情報" })).toContainText("地震（長岡市 震度6弱）の震央");
  await expect(page.locator(".maplibregl-marker")).toHaveCount(2); // 地点と震央
});
