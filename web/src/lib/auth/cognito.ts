/**
 * Cognito のマネージドログイン（Authorization Code + PKCE）。docs/07 3.1
 *
 * - トークン（アクセス・リフレッシュ）はメモリにだけ置く。sessionStorage に置くのは、
 *   ログインの往復の間だけ使う PKCE の値と、再読み込みのときにログインし直すための目印（トークンではない）
 * - 設定（NEXT_PUBLIC_COGNITO_*）はビルド時に埋め込む。秘密の値ではない
 */

export type CognitoConfig = {
  /** マネージドログインのオリジン（例: https://xxx.auth.ap-northeast-1.amazoncognito.com） */
  domain: string;
  clientId: string;
};

/** Cognito の設定。未設定ならローカル用の固定トークンでログインする（MSW・cmd/local） */
export function cognitoConfig(): CognitoConfig | null {
  const domain = process.env.NEXT_PUBLIC_COGNITO_DOMAIN;
  const clientId = process.env.NEXT_PUBLIC_COGNITO_CLIENT_ID;
  return domain && clientId ? { domain: domain.replace(/\/$/, ""), clientId } : null;
}

export type Tokens = {
  accessToken: string;
  refreshToken?: string;
  /** アクセストークンの有効期限（ミリ秒） */
  expiresAt: number;
  email?: string;
};

const PKCE_KEY = "saigai.auth.pkce";

function base64url(bytes: Uint8Array): string {
  let s = "";
  for (const b of bytes) s += String.fromCharCode(b);
  return btoa(s).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

function randomString(bytes = 32): string {
  const b = new Uint8Array(bytes);
  crypto.getRandomValues(b);
  return base64url(b);
}

/** PKCE の code_challenge（S256） */
export async function codeChallenge(verifier: string): Promise<string> {
  const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(verifier));
  return base64url(new Uint8Array(digest));
}

export function redirectUri(): string {
  return `${window.location.origin}/auth/callback`;
}

/** ログイン後に戻る画面。自サイトの画面だけを許す（オープンリダイレクトを防ぐ） */
export function safeReturnPath(p: string | null | undefined): string {
  if (!p || !p.startsWith("/") || p.startsWith("//") || p.startsWith("/\\")) return "/app";
  if (p.startsWith("/auth/") || p.startsWith("/login")) return "/app";
  return p;
}

/** ログイン画面の URL を作る。PKCE の値と state は、戻ってくるまで sessionStorage に置く */
export async function authorizeUrl(cfg: CognitoConfig, returnTo: string): Promise<string> {
  const verifier = randomString();
  const state = randomString(16);
  window.sessionStorage.setItem(PKCE_KEY, JSON.stringify({ verifier, state, returnTo: safeReturnPath(returnTo) }));
  const q = new URLSearchParams({
    response_type: "code",
    client_id: cfg.clientId,
    redirect_uri: redirectUri(),
    scope: "openid email",
    state,
    code_challenge: await codeChallenge(verifier),
    code_challenge_method: "S256",
  });
  return `${cfg.domain}/oauth2/authorize?${q}`;
}

export class LoginError extends Error {}

/** ログイン画面から戻ってきたときに、認可コードをトークンに交換する。PKCE の値は1回で捨てる */
export async function completeLogin(cfg: CognitoConfig, search: URLSearchParams): Promise<{ tokens: Tokens; returnTo: string }> {
  const raw = window.sessionStorage.getItem(PKCE_KEY);
  window.sessionStorage.removeItem(PKCE_KEY);
  if (search.get("error")) throw new LoginError("ログインを完了できませんでした");
  const saved = raw ? (JSON.parse(raw) as { verifier: string; state: string; returnTo: string }) : null;
  const code = search.get("code");
  if (!saved || !code || search.get("state") !== saved.state) throw new LoginError("ログインの手続きが正しくありません。もう一度ログインしてください");
  const tokens = await tokenRequest(
    cfg,
    new URLSearchParams({ grant_type: "authorization_code", client_id: cfg.clientId, code, redirect_uri: redirectUri(), code_verifier: saved.verifier }),
  );
  return { tokens, returnTo: safeReturnPath(saved.returnTo) };
}

/** リフレッシュトークンで新しいアクセストークンを得る */
export async function refreshTokens(cfg: CognitoConfig, refreshToken: string): Promise<Tokens> {
  const t = await tokenRequest(cfg, new URLSearchParams({ grant_type: "refresh_token", client_id: cfg.clientId, refresh_token: refreshToken }));
  return { ...t, refreshToken: t.refreshToken ?? refreshToken };
}

async function tokenRequest(cfg: CognitoConfig, body: URLSearchParams): Promise<Tokens> {
  const res = await fetch(`${cfg.domain}/oauth2/token`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body,
  });
  if (!res.ok) throw new LoginError("ログインの有効期限が切れました。もう一度ログインしてください");
  const j = (await res.json()) as { access_token: string; refresh_token?: string; id_token?: string; expires_in: number };
  return {
    accessToken: j.access_token,
    refreshToken: j.refresh_token,
    expiresAt: Date.now() + j.expires_in * 1000,
    email: j.id_token ? emailFromIdToken(j.id_token) : undefined,
  };
}

/** リフレッシュトークンを無効にする（ログアウト時）。失敗しても画面の操作は止めない */
export async function revokeToken(cfg: CognitoConfig, refreshToken: string): Promise<void> {
  try {
    await fetch(`${cfg.domain}/oauth2/revoke`, {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: new URLSearchParams({ token: refreshToken, client_id: cfg.clientId }),
    });
  } catch {
    /* 無効にできなくても、トークンはメモリから消えている */
  }
}

/** Cognito のログインの状態（Cookie）も消すログアウトの URL。戻り先は Terraform の logout_urls と一致させる */
export function logoutUrl(cfg: CognitoConfig): string {
  const q = new URLSearchParams({ client_id: cfg.clientId, logout_uri: `${window.location.origin}/` });
  return `${cfg.domain}/logout?${q}`;
}

/** ID トークンからメールアドレスを取り出す（表示用。署名は検証しない。API の認可には使わない） */
export function emailFromIdToken(idToken: string): string | undefined {
  try {
    const part = idToken.split(".")[1];
    if (!part) return undefined;
    const payload = part.replace(/-/g, "+").replace(/_/g, "/");
    const json = new TextDecoder().decode(Uint8Array.from(atob(payload), (c) => c.charCodeAt(0)));
    const email = (JSON.parse(json) as { email?: unknown }).email;
    return typeof email === "string" ? email : undefined;
  } catch {
    return undefined;
  }
}
