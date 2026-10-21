import type { NextConfig } from "next";

// 静的出力（S3 + CloudFront で配信する。docs/05_system_architecture.md 3.1）
const config: NextConfig = {
  output: "export",
  trailingSlash: false,
  images: { unoptimized: true },
  reactStrictMode: true,
  poweredByHeader: false,
  // next dev が AGENTS.md / CLAUDE.md を自動で生成しないようにする（リポジトリの CLAUDE.md はルートで管理する）
  agentRules: false,
  turbopack: {
    resolveAlias: {
      // msw/browser は Node.js 向けの出力を持たないため、事前描画では空の実装に差し替える
      "msw/browser": { browser: "./node_modules/msw/lib/browser/index.js", default: "./src/mocks/browser-stub.ts" },
    },
  },
};

export default config;
