import type { Feature, Polygon } from "geojson";
import type { StyleSpecification } from "maplibre-gl";

/** 国土地理院の淡色地図（地理院タイル）。出典の表示が必要 */
export const gsiStyle: StyleSpecification = {
  version: 8,
  sources: {
    gsi: {
      type: "raster",
      tiles: ["https://cyberjapandata.gsi.go.jp/xyz/pale/{z}/{x}/{y}.png"],
      tileSize: 256,
      minzoom: 5,
      maxzoom: 18,
      attribution: '<a href="https://maps.gsi.go.jp/development/ichiran.html" target="_blank" rel="noopener noreferrer">地理院タイル</a>',
    },
  },
  layers: [{ id: "gsi", type: "raster", source: "gsi" }],
};

/** 中心と半径（m）から円の多角形（GeoJSON）を作る */
export function circlePolygon(lat: number, lon: number, radiusM: number, steps = 64): Feature<Polygon> {
  const coords: [number, number][] = [];
  const dLat = radiusM / 111_320;
  const dLon = radiusM / (111_320 * Math.cos((lat * Math.PI) / 180));
  for (let i = 0; i <= steps; i++) {
    const a = (i / steps) * 2 * Math.PI;
    coords.push([lon + dLon * Math.cos(a), lat + dLat * Math.sin(a)]);
  }
  return { type: "Feature", properties: {}, geometry: { type: "Polygon", coordinates: [coords] } };
}

/** 半径に合わせた初期のズーム */
export function zoomForRadius(radiusM: number): number {
  return radiusM <= 1000 ? 14 : radiusM <= 3000 ? 12.5 : radiusM <= 5000 ? 11.8 : 10.8;
}
