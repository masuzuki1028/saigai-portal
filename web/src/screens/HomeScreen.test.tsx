import { screen } from "@testing-library/react";

import { renderLoggedIn } from "@/test/render";

import { HomeScreen } from "./HomeScreen";

describe("HomeScreen", () => {
  it("平常時は地点を一覧し、警報のお知らせは出さない", async () => {
    renderLoggedIn(<HomeScreen />, "normal");
    expect(await screen.findByRole("link", { name: /自宅/ })).toHaveAttribute("href", expect.stringContaining("/app/location?id="));
    expect(screen.getByRole("link", { name: /実家/ })).toBeInTheDocument();
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "＋ 地点を追加" })).toBeInTheDocument();
  });

  it("大雨警報の発表中は、警報が出ている地点の数と発表中の情報を表示する", async () => {
    renderLoggedIn(<HomeScreen />, "heavy-rain");
    expect(await screen.findByRole("status")).toHaveTextContent("警報以上の情報が出ている地点: 1");
    const levels = screen.getByRole("list", { name: "発表中の情報" });
    expect(levels).toHaveTextContent("大雨警報");
    expect(levels).toHaveTextContent("洪水警報");
  });

  it("地点がないときは登録を促す", async () => {
    renderLoggedIn(<HomeScreen />, "empty");
    expect(await screen.findByText(/まだ地点が登録されていません/)).toBeInTheDocument();
  });
});
