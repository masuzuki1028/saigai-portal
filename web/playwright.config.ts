import { defineConfig, devices } from "@playwright/test";

const PORT = 3100;

/**
 * E2E（モック API）。npm run build:mock で作った静的出力（out/）を配信してテストする（docs/08 4.2）。
 * スマホのビューポートを優先し、Chromium（Android 相当）と WebKit（iPhone の Safari 相当）で確認する。
 */
export default defineConfig({
  testDir: "e2e",
  fullyParallel: true,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["list"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL: `http://127.0.0.1:${PORT}`,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
    locale: "ja-JP",
    timezoneId: "Asia/Tokyo",
  },
  projects: [
    { name: "android-chromium", use: { ...devices["Pixel 7"] } },
    { name: "iphone-webkit", use: { ...devices["iPhone 15"] } },
  ],
  webServer: {
    command: `npm run build:mock && node scripts/serve-out.mjs ${PORT}`,
    url: `http://127.0.0.1:${PORT}`,
    reuseExistingServer: !process.env.CI,
    timeout: 180_000,
  },
});
