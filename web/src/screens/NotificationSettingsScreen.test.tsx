import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { db } from "@/mocks/db";
import { renderLoggedIn } from "@/test/render";

import { NotificationSettingsScreen } from "./NotificationSettingsScreen";

describe("NotificationSettingsScreen", () => {
  it("テスト通知を送れる。メール通知をオフにすると送れない", async () => {
    renderLoggedIn(<NotificationSettingsScreen />);
    await userEvent.click(await screen.findByRole("button", { name: "テスト通知を送る" }));
    expect(await screen.findByRole("status")).toHaveTextContent("テスト通知を送信しました");
    expect(db.testNotifications).toBe(1);

    await userEvent.click(screen.getByRole("checkbox", { name: /メールで通知する/ }));
    expect(await screen.findByRole("status")).toHaveTextContent("オフにしました");
    await userEvent.click(screen.getByRole("button", { name: "テスト通知を送る" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("メール通知がオフになっています");
  });
});
