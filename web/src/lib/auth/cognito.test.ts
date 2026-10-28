import { http, HttpResponse } from "msw";

import { server } from "@/mocks/node";
import { idToken } from "@/test/jwt";

import { authorizeUrl, codeChallenge, completeLogin, emailFromIdToken, logoutUrl, safeReturnPath } from "./cognito";

const cfg = { domain: "https://login.example.com", clientId: "client1" };

describe("cognito", () => {
  it("PKCE（S256）のログイン画面の URL を作り、戻ってきたら認可コードをトークンに交換する", async () => {
    const url = new URL(await authorizeUrl(cfg, "/app/location?id=L1"));
    expect(url.origin + url.pathname).toBe("https://login.example.com/oauth2/authorize");
    const q = url.searchParams;
    expect(q.get("response_type")).toBe("code");
    expect(q.get("client_id")).toBe("client1");
    expect(q.get("redirect_uri")).toBe(`${window.location.origin}/auth/callback`);
    expect(q.get("code_challenge_method")).toBe("S256");

    let tokenBody: URLSearchParams | undefined;
    server.use(
      http.post("https://login.example.com/oauth2/token", async ({ request }) => {
        tokenBody = new URLSearchParams(await request.text());
        return HttpResponse.json({ access_token: "AT", refresh_token: "RT", id_token: idToken({ email: "a@example.com" }), expires_in: 3600 });
      }),
    );
    const { tokens, returnTo } = await completeLogin(cfg, new URLSearchParams({ code: "C1", state: q.get("state") ?? "" }));
    expect(tokens).toMatchObject({ accessToken: "AT", refreshToken: "RT", email: "a@example.com" });
    expect(returnTo).toBe("/app/location?id=L1");
    // 送った code_verifier から、ログイン画面に渡した code_challenge になる
    expect(await codeChallenge(tokenBody?.get("code_verifier") ?? "")).toBe(q.get("code_challenge"));
    expect(tokenBody?.get("grant_type")).toBe("authorization_code");
    // PKCE の値は1回で捨てる。トークンは sessionStorage に置かない
    expect(window.sessionStorage.length).toBe(0);
  });

  it("state が一致しない応答は受け付けない", async () => {
    await authorizeUrl(cfg, "/app");
    await expect(completeLogin(cfg, new URLSearchParams({ code: "C1", state: "other" }))).rejects.toThrow(/正しくありません/);
    // 2回目は PKCE の値がないため、正しい state でも受け付けない
    await expect(completeLogin(cfg, new URLSearchParams({ code: "C1", state: "" }))).rejects.toThrow();
  });

  it("戻り先は自サイトの画面だけ", () => {
    expect(safeReturnPath("/app/map")).toBe("/app/map");
    for (const p of ["https://evil.example.com/", "//evil.example.com", "/\\evil.example.com", "/auth/callback", "/login", "", null]) {
      expect(safeReturnPath(p)).toBe("/app");
    }
  });

  it("ログアウトの戻り先はトップ", () => {
    const u = new URL(logoutUrl(cfg));
    expect(u.pathname).toBe("/logout");
    expect(u.searchParams.get("logout_uri")).toBe(`${window.location.origin}/`);
  });

  it("ID トークンからメールアドレスを読む（壊れていれば undefined）", () => {
    expect(emailFromIdToken(idToken({ email: "日本@example.com" }))).toBe("日本@example.com");
    expect(emailFromIdToken("broken")).toBeUndefined();
  });
});
