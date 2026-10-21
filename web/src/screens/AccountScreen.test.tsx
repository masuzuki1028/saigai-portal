import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { db } from "@/mocks/db";
import { renderLoggedIn } from "@/test/render";
import { router } from "../../vitest.setup";

import { AccountScreen } from "./AccountScreen";

describe("AccountScreen", () => {
  it("確認の文字列を入力しないと退会できない", async () => {
    renderLoggedIn(<AccountScreen />);
    await userEvent.click(screen.getByRole("button", { name: "退会してデータを削除する" }));
    expect(screen.getByRole("alert")).toHaveTextContent("「削除」と入力してください");
    expect(db.deleted).toBe(false);
  });

  it("退会すると、データが消えてログアウトする", async () => {
    renderLoggedIn(<AccountScreen />);
    await userEvent.type(screen.getByRole("textbox", { name: /「削除」と入力/ }), "削除");
    await userEvent.click(screen.getByRole("button", { name: "退会してデータを削除する" }));
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/?deleted=1"));
    expect(router.replace).not.toHaveBeenCalledWith("/login");
    expect(db.deleted).toBe(true);
    expect(db.locations.size).toBe(0);
    expect(window.sessionStorage.getItem("saigai.localToken")).toBeNull();
  });
});
