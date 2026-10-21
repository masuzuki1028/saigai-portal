// サーバー側の事前描画（静的出力）では MSW のブラウザ用モジュールを読み込めないため、この空の実装に差し替える
// （next.config.ts の turbopack.resolveAlias）。ブラウザでは本物の msw/browser が使われる。
export function setupWorker(): never {
  throw new Error("msw/browser はブラウザでのみ使える");
}
