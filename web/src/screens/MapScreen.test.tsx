import { screen } from "@testing-library/react";

import { IDS } from "@/mocks/fixtures";
import { renderLoggedIn } from "@/test/render";

import { MapScreen } from "./MapScreen";

describe("MapScreen", () => {
  it("警報の発表中は、発表区域と発表中の警報を一覧に表示する", async () => {
    renderLoggedIn(<MapScreen id={IDS.PARENTS} />, "heavy-rain");
    const legend = await screen.findByRole("region", { name: "地図に表示している情報" });
    expect(await screen.findByText(/発表区域: 新潟県長岡市/)).toBeInTheDocument();
    expect(legend).toHaveTextContent("大雨警報");
    expect(screen.getByRole("region", { name: /実家と半径3kmの範囲の地図/ })).toBeInTheDocument();
  });

  it("地震の震央をピンとして一覧に表示する（同じ震央は1つにまとめる）", async () => {
    renderLoggedIn(<MapScreen id={IDS.PARENTS} />, "earthquake");
    const legend = await screen.findByRole("region", { name: "地図に表示している情報" });
    expect(await screen.findAllByText(/の震央/)).toHaveLength(1);
    expect(legend).toHaveTextContent("地震（長岡市 震度6弱）の震央");
  });

  it("平常時は、発表中の警報・注意報がないことを表示する", async () => {
    renderLoggedIn(<MapScreen id={IDS.HOME} />, "normal");
    expect(await screen.findByText(/発表中の警報・注意報はありません/)).toBeInTheDocument();
  });
});
