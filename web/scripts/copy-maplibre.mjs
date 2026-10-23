// MapLibre GL JS の Web Worker（maplibre-gl-worker.mjs と、それが読み込む maplibre-gl-shared.mjs）を public/maplibre/ にコピーする。
// MapLibre 6 は自分の場所（import.meta.url）から Worker を探すが、Next.js でまとめると場所が分からなくなるため、
// コピーした場所を setWorkerUrl で指定する（src/components/map/maplibre.ts）。node_modules のバージョンと必ず一致させるため、
// ビルド・開発サーバーの起動のたびにコピーする（ignore-scripts のため pre/post のフックは使えない）。
import { copyFileSync, mkdirSync } from "node:fs";
import { join } from "node:path";

const src = join("node_modules", "maplibre-gl", "dist");
const dest = join("public", "maplibre");
mkdirSync(dest, { recursive: true });
for (const f of ["maplibre-gl-worker.mjs", "maplibre-gl-shared.mjs"]) {
  copyFileSync(join(src, f), join(dest, f));
}
