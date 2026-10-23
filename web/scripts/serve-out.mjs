// out/（静的出力）を配信する。E2E とビルド後の確認用。依存パッケージを増やさないため node:http だけで書く。
// 使い方: node scripts/serve-out.mjs [port]
import { createReadStream, existsSync, statSync } from "node:fs";
import { createServer } from "node:http";
import { extname, join, normalize, resolve } from "node:path";

const root = resolve("out");
const port = Number(process.argv[2] ?? 3000);
const types = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  // MapLibre の Worker（.mjs）は JavaScript の Content-Type でないと読み込めない
  ".mjs": "text/javascript; charset=utf-8",
  ".map": "application/json",
  ".css": "text/css; charset=utf-8",
  ".json": "application/json",
  ".svg": "image/svg+xml",
  ".png": "image/png",
  ".ico": "image/x-icon",
  ".txt": "text/plain; charset=utf-8",
  ".woff2": "font/woff2",
};

createServer((req, res) => {
  const url = new URL(req.url ?? "/", "http://localhost");
  // パストラバーサルを防ぐ: out/ の外は返さない
  let path = normalize(join(root, decodeURIComponent(url.pathname)));
  if (!path.startsWith(root)) {
    res.writeHead(403).end();
    return;
  }
  // Next.js の静的出力は /login → login.html。同じ名前のディレクトリ（login/）もあるため .html を先に探す
  if (existsSync(path + ".html")) path += ".html";
  else if (existsSync(path) && statSync(path).isDirectory()) path = join(path, "index.html");
  if (!existsSync(path)) {
    res.writeHead(404, { "content-type": "text/html; charset=utf-8" });
    createReadStream(join(root, "404.html")).pipe(res);
    return;
  }
  res.writeHead(200, { "content-type": types[extname(path)] ?? "application/octet-stream" });
  createReadStream(path).pipe(res);
}).listen(port, "127.0.0.1", () => console.log(`http://127.0.0.1:${port}`));
