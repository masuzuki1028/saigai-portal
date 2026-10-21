import js from "@eslint/js";
import nextPlugin from "@next/eslint-plugin-next";
import security from "eslint-plugin-security";
import tseslint from "typescript-eslint";

export default tseslint.config(
  { ignores: [".next/**", "out/**", "public/mockServiceWorker.js", "src/lib/api/schema.d.ts", "playwright-report/**", "test-results/**"] },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  security.configs.recommended,
  {
    plugins: { "@next/next": nextPlugin },
    rules: {
      ...nextPlugin.configs.recommended.rules,
      // dangerouslySetInnerHTML は使わない（docs/07_security.md 3.6）
      "no-restricted-syntax": [
        "error",
        { selector: "JSXAttribute[name.name='dangerouslySetInnerHTML']", message: "dangerouslySetInnerHTML は使わない（XSS 対策）" },
      ],
      // 長期のトークンを localStorage に置かない
      "no-restricted-globals": ["error", { name: "localStorage", message: "トークンを localStorage に置かない（docs/07 3.6）" }],
      "no-restricted-properties": ["error", { object: "window", property: "localStorage", message: "トークンを localStorage に置かない（docs/07 3.6）" }],
      // オブジェクトのキーアクセスを一律に禁止すると誤検知が多いため無効にする
      "security/detect-object-injection": "off",
    },
  },
  {
    files: ["scripts/**/*.mjs"],
    languageOptions: { globals: { console: "readonly", process: "readonly", URL: "readonly" } },
    rules: { "security/detect-non-literal-fs-filename": "off" },
  },
);
