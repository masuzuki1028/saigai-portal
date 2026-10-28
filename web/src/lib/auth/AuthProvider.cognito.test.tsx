import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";

import { server } from "@/mocks/node";
import { idToken } from "@/test/jwt";

import { AuthProvider, useAuth } from "./AuthProvider";
import { redirectTo } from "./navigate";

vi.mock("./navigate", () => ({ redirectTo: vi.fn() }));

const DOMAIN = "https://login.example.com";

function Probe() {
  const { mode, token, userLabel, api, login, finishLogin, logout } = useAuth();
  return (
    <>
      <p>{mode}</p>
      <p>{token ? `ログイン中: ${userLabel}` : "未ログイン"}</p>
      <button onClick={() => void login("/app/map")}>login</button>
      <button
        onClick={() => {
          const state = (JSON.parse(window.sessionStorage.getItem("saigai.auth.pkce") ?? "{}") as { state?: string }).state ?? "";
          void finishLogin(new URLSearchParams({ code: "C1", state }));
        }}
      >
        callback
      </button>
      <button onClick={() => void api.GET("/me")}>call</button>
      <button onClick={() => logout()}>logout</button>
    </>
  );
}

let tokenRequests: URLSearchParams[];
let apiAuth: (string | null)[];

beforeEach(() => {
  vi.stubEnv("NEXT_PUBLIC_COGNITO_DOMAIN", DOMAIN);
  vi.stubEnv("NEXT_PUBLIC_COGNITO_CLIENT_ID", "client1");
  tokenRequests = [];
  apiAuth = [];
  server.use(
    http.post(`${DOMAIN}/oauth2/token`, async ({ request }) => {
      const body = new URLSearchParams(await request.text());
      tokenRequests.push(body);
      if (body.get("grant_type") === "refresh_token") return HttpResponse.json({ access_token: "AT2", expires_in: 3600 });
      // 期限が近いアクセストークンを返し、次の API の呼び出しでリフレッシュさせる
      return HttpResponse.json({ access_token: "AT1", refresh_token: "RT", id_token: idToken({ email: "a@example.com" }), expires_in: 30 });
    }),
    http.post(`${DOMAIN}/oauth2/revoke`, () => new HttpResponse(null, { status: 200 })),
    http.get("*/api/me", ({ request }) => {
      apiAuth.push(request.headers.get("Authorization"));
      return HttpResponse.json({ userId: "U1", plan: "free", locationLimit: 3, locationCount: 0 });
    }),
  );
});
afterEach(() => vi.unstubAllEnvs());

describe("AuthProvider（Cognito）", () => {
  it("ログイン → API の呼び出し（期限が近ければリフレッシュ）→ ログアウト。トークンはどのストレージにも置かない", async () => {
    const setItem = vi.spyOn(Storage.prototype, "setItem");
    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    );
    expect(screen.getByText("cognito")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "login" }));
    await waitFor(() => expect(redirectTo).toHaveBeenCalledWith(expect.stringMatching(/^https:\/\/login\.example\.com\/oauth2\/authorize\?/)));

    await userEvent.click(screen.getByRole("button", { name: "callback" }));
    expect(await screen.findByText("ログイン中: a@example.com")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "call" }));
    await waitFor(() => expect(apiAuth).toEqual(["Bearer AT2"]));
    expect(tokenRequests.map((b) => b.get("grant_type"))).toEqual(["authorization_code", "refresh_token"]);
    expect(tokenRequests[1]?.get("refresh_token")).toBe("RT");

    const stored = setItem.mock.calls.map((c) => String(c[1]));
    expect(stored.some((v) => /AT1|AT2|RT/.test(v))).toBe(false);
    // eslint-disable-next-line no-restricted-properties -- 保存されていないことの確認のために参照する
    expect(window.localStorage.length).toBe(0);

    vi.mocked(redirectTo).mockClear();
    await userEvent.click(screen.getByRole("button", { name: "logout" }));
    expect(screen.getByText("未ログイン")).toBeInTheDocument();
    await waitFor(() => expect(redirectTo).toHaveBeenCalledWith(expect.stringMatching(/^https:\/\/login\.example\.com\/logout\?/)));
    expect(window.sessionStorage.getItem("saigai.auth.active")).toBeNull();
  });

  it("リフレッシュに失敗したらログアウトする", async () => {
    server.use(http.post(`${DOMAIN}/oauth2/token`, async ({ request }) => {
      const body = new URLSearchParams(await request.text());
      if (body.get("grant_type") === "refresh_token") return HttpResponse.json({ error: "invalid_grant" }, { status: 400 });
      return HttpResponse.json({ access_token: "AT1", refresh_token: "RT", expires_in: 30 });
    }));
    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    );
    await userEvent.click(screen.getByRole("button", { name: "login" }));
    await userEvent.click(screen.getByRole("button", { name: "callback" }));
    await screen.findByText(/ログイン中/);
    await userEvent.click(screen.getByRole("button", { name: "call" }));
    expect(await screen.findByText("未ログイン")).toBeInTheDocument();
    expect(apiAuth).toEqual([null]);
  });
});
