import type { FeatureCollection, MultiPolygon } from "geojson";

export type WarnAreaCollection = FeatureCollection<MultiPolygon, { code: string; name: string }>;

/**
 * 警報・注意報の発表区域の形を読み込む（web/public/geo/warn/<都道府県コード>.json。api の genwarnmap で生成）。
 * 発表区域のコード（5桁）で始まる区域だけを返す（富山市の平地・山間部のように分かれている区域はまとめて返す）。
 */
export async function loadWarnArea(warnAreaCode: string, signal?: AbortSignal): Promise<WarnAreaCollection | null> {
  if (!/^\d{5}$/.test(warnAreaCode)) return null;
  const url = new URL(`/geo/warn/${warnAreaCode.slice(0, 2)}.json`, window.location.origin);
  const res = await fetch(url, { signal });
  if (!res.ok) return null;
  const all = (await res.json()) as WarnAreaCollection;
  const features = all.features.filter((f) => f.properties.code.startsWith(warnAreaCode));
  return features.length ? { type: "FeatureCollection", features } : null;
}
