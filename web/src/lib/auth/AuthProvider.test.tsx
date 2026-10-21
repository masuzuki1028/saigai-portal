import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { AuthProvider, useAuth } from "./AuthProvider";

function Probe() {
  const { token, loginLocal, logout } = useAuth();
  return (
    <>
      <p>{token ? "ログイン中" : "未ログイン"}</p>
      <button onClick={() => loginLocal("alice")}>login</button>
      <button onClick={logout}>logout</button>
    </>
  );
}

describe("AuthProvider", () => {
  it("トークンを localStorage に保存しない", async () => {
    const setItem = vi.spyOn(Storage.prototype, "setItem");
    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    );
    await userEvent.click(screen.getByRole("button", { name: "login" }));
    expect(screen.getByText("ログイン中")).toBeInTheDocument();
    // eslint-disable-next-line no-restricted-properties -- 保存されていないことの確認のために参照する
    expect(window.localStorage.length).toBe(0);
    expect(setItem.mock.contexts.some((c) => c === window.sessionStorage)).toBe(true);

    await userEvent.click(screen.getByRole("button", { name: "logout" }));
    expect(screen.getByText("未ログイン")).toBeInTheDocument();
    expect(window.sessionStorage.length).toBe(0);
  });
});
