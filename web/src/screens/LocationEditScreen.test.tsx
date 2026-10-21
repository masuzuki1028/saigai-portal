import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { db } from "@/mocks/db";
import { IDS } from "@/mocks/fixtures";
import { renderLoggedIn } from "@/test/render";
import { router } from "../../vitest.setup";

import { LocationEditScreen } from "./LocationEditScreen";

describe("LocationEditScreen", () => {
  it("名前と地点がないとエラーを表示する", async () => {
    renderLoggedIn(<LocationEditScreen id={null} />, "empty");
    await userEvent.click(screen.getByRole("button", { name: "登録" }));
    expect(screen.getByRole("alert")).toHaveTextContent("名前を入力してください");

    await userEvent.type(screen.getByRole("textbox", { name: "名前" }), "実家");
    await userEvent.click(screen.getByRole("button", { name: "登録" }));
    expect(screen.getByRole("alert")).toHaveTextContent("地図で地点を選んでください");
  });

  it("地点を登録すると、タイムラインに移る", async () => {
    renderLoggedIn(<LocationEditScreen id={null} />, "empty");
    await userEvent.type(screen.getByRole("textbox", { name: "名前" }), "実家");
    await userEvent.click(screen.getByRole("button", { name: /地図で地点を選ぶ/ }));
    await userEvent.click(screen.getByRole("radio", { name: "5km" }));
    await userEvent.selectOptions(screen.getByRole("combobox", { name: "地震の通知" }), "4");
    await userEvent.click(screen.getByRole("button", { name: "登録" }));

    await waitFor(() => expect(router.push).toHaveBeenCalledWith(expect.stringMatching(/^\/app\/location\?id=/)));
    const saved = [...db.locations.values()][0]!;
    expect(saved).toMatchObject({ name: "実家", radiusM: 5000, coord: { lat: 37.4462, lon: 138.8512 } });
    expect(saved.hazardSettings.earthquake.notifyMinIntensity).toBe("4");
  });

  it("既存の地点を編集できる", async () => {
    renderLoggedIn(<LocationEditScreen id={IDS.HOME} />, "normal");
    const name = await screen.findByRole("textbox", { name: "名前" });
    expect(name).toHaveValue("自宅");
    await userEvent.clear(name);
    await userEvent.type(name, "自宅マンション");
    await userEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(db.locations.get(IDS.HOME)?.name).toBe("自宅マンション"));
    expect(db.locations.get(IDS.HOME)?.version).toBe(2);
  });
});

describe("LocationEditScreen（沿岸の地点）", () => {
  it("沿岸の地点では津波の設定を表示する", async () => {
    renderLoggedIn(<LocationEditScreen id={IDS.OFFICE} />, "tsunami");
    expect(await screen.findByRole("combobox", { name: "津波の通知" })).toHaveValue("3");
  });

  it("内陸の地点では津波の設定を表示しない", async () => {
    renderLoggedIn(<LocationEditScreen id={IDS.PARENTS} />, "tsunami");
    await screen.findByRole("textbox", { name: "名前" });
    expect(screen.queryByRole("combobox", { name: "津波の通知" })).not.toBeInTheDocument();
  });
});
