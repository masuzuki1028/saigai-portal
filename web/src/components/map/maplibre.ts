/**
 * MapLibre GL JS を読み込み、Web Worker の場所を設定する。地図の部品はこの関数から MapLibre を使う。
 * Worker のファイルは scripts/copy-maplibre.mjs で public/maplibre/ にコピーしている。
 */
let configured = false;

export async function loadMaplibre() {
  const mod = await import("maplibre-gl");
  if (!configured) {
    mod.setWorkerUrl(new URL("/maplibre/maplibre-gl-worker.mjs", window.location.origin).href);
    configured = true;
  }
  return mod;
}
