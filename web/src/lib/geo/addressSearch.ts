/**
 * 国土地理院の住所検索 API で、住所から座標を求める。
 * ブラウザから直接呼ぶ（CORS 許可あり）。入力した住所はこちらのサーバーを通らない。
 * 公開時は CSP の connect-src に https://msearch.gsi.go.jp を許可する（docs/07_security.md 3.6）。
 */
export const ADDRESS_SEARCH_URL = "https://msearch.gsi.go.jp/address-search/AddressSearch";

export type AddressCandidate = { title: string; lat: number; lon: number };

const MAX_RESULTS = 5;

// 日本の範囲（API の LatLon の範囲と合わせる）
const inJapan = (lat: number, lon: number) => lat >= 20 && lat <= 46 && lon >= 122 && lon <= 154;

export async function searchAddress(query: string, signal?: AbortSignal): Promise<AddressCandidate[]> {
  const q = query.trim();
  if (!q) return [];
  const res = await fetch(`${ADDRESS_SEARCH_URL}?q=${encodeURIComponent(q)}`, { signal });
  if (!res.ok) throw new Error(`address search: ${res.status}`);
  const body: unknown = await res.json();
  if (!Array.isArray(body)) return [];
  const out: AddressCandidate[] = [];
  for (const f of body) {
    // 外部の応答なので、形を確かめてから使う
    const coords = f?.geometry?.coordinates;
    const title = f?.properties?.title;
    if (!Array.isArray(coords) || typeof coords[0] !== "number" || typeof coords[1] !== "number" || typeof title !== "string") continue;
    const [lon, lat] = coords as [number, number];
    if (!inJapan(lat, lon)) continue;
    out.push({ title, lat: Number(lat.toFixed(6)), lon: Number(lon.toFixed(6)) });
    if (out.length >= MAX_RESULTS) break;
  }
  return out;
}
