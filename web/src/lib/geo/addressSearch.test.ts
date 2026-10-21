import { http, HttpResponse } from "msw";

import { server } from "@/mocks/node";

import { ADDRESS_SEARCH_URL, searchAddress } from "./addressSearch";

describe("searchAddress", () => {
  it("候補を座標つきで返す", async () => {
    expect(await searchAddress("長岡市")).toEqual([{ title: "新潟県長岡市城内町", lat: 37.445873, lon: 138.851746 }]);
  });

  it("空の入力では検索しない", async () => {
    expect(await searchAddress("  ")).toEqual([]);
  });

  it("形の合わない候補や日本の範囲外の候補は捨てる", async () => {
    server.use(
      http.get(ADDRESS_SEARCH_URL, () =>
        HttpResponse.json([
          { geometry: { coordinates: ["x", 1] }, properties: { title: "壊れたデータ" } },
          { geometry: { coordinates: [0, 0] }, properties: { title: "範囲外" } },
          { geometry: { coordinates: [139.7, 35.6] }, properties: { title: "東京都" } },
        ]),
      ),
    );
    expect(await searchAddress("東京")).toEqual([{ title: "東京都", lat: 35.6, lon: 139.7 }]);
  });

  it("エラーの応答では例外にする", async () => {
    server.use(http.get(ADDRESS_SEARCH_URL, () => new HttpResponse(null, { status: 500 })));
    await expect(searchAddress("長岡")).rejects.toThrow();
  });
});
