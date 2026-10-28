"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";

import { createApi, type Api } from "@/lib/api/client";

import { authorizeUrl, cognitoConfig, completeLogin, logoutUrl, refreshTokens, revokeToken, type Tokens } from "./cognito";
import { redirectTo } from "./navigate";

/**
 * ログイン状態。トークンはメモリに持ち、localStorage には置かない（docs/07_security.md 3.6）。
 *
 * - 本番（NEXT_PUBLIC_COGNITO_* あり）: Cognito のマネージドログイン（Authorization Code + PKCE）。
 *   再読み込みでメモリのトークンは消えるため、ログイン中だった目印（トークンではない）だけを sessionStorage に置き、
 *   Cognito のログイン画面を経由してログインし直す（Cognito のログインが続いていれば、入力なしで戻ってくる）
 * - ローカル・モック: api の cmd/local・MSW が受け付ける固定トークン。本物の資格情報ではないため、
 *   再読み込みでログアウトしないよう sessionStorage に置く
 */
export const LOCAL_USERS = [
  { id: "alice", label: "alice（シナリオの地点を登録済み）", token: "local-alice" },
  { id: "bob", label: "bob", token: "local-bob" },
] as const;

const LOCAL_TOKEN_KEY = "saigai.localToken";
const ACTIVE_KEY = "saigai.auth.active";
/** 退会した直後の目印。Cognito のログアウトを経由するとクエリを付けて戻れないため使う（DeletedNotice が読む） */
export const DELETED_KEY = "saigai.deleted";

/** アクセストークンの期限のこの時間前になったら、リフレッシュする */
const REFRESH_BEFORE_MS = 60_000;

/** ログアウトの理由。退会のときはログイン画面ではなくトップに戻す */
export type LogoutReason = "user" | "expired" | "deleted";

type AuthState = {
  mode: "cognito" | "local";
  token: string | null;
  userLabel: string | null;
  api: Api;
  lastLogout: LogoutReason | null;
  /** 再読み込みのあと、Cognito でログインし直している途中 */
  restoring: boolean;
  /** Cognito のログイン画面に移る */
  login: (returnTo?: string) => Promise<void>;
  /** Cognito のログイン画面から戻ってきたときに呼ぶ。戻る画面のパスを返す */
  finishLogin: (search: URLSearchParams) => Promise<string>;
  loginLocal: (userId: string) => void;
  logout: (reason?: LogoutReason) => void;
};

const AuthContext = createContext<AuthState | null>(null);

function storage(): Storage | null {
  try {
    return typeof window === "undefined" ? null : window.sessionStorage;
  } catch {
    return null;
  }
}

function setItem(k: string, v: string) {
  try {
    storage()?.setItem(k, v);
  } catch {
    /* 保存できなくてもメモリ上ではログインできる */
  }
}

function removeItem(k: string) {
  try {
    storage()?.removeItem(k);
  } catch {
    /* 保存できない環境では何もしない */
  }
}

/** 再読み込みのあとにログインし直すか。ログインが必要な画面にいて、ログイン中だった目印があるときだけ */
function shouldRestore(): boolean {
  return typeof window !== "undefined" && window.location.pathname.startsWith("/app") && storage()?.getItem(ACTIVE_KEY) === "1";
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const cfg = useMemo(() => cognitoConfig(), []);
  const [session, setSession] = useState<Tokens | null>(() => {
    if (cfg) return null;
    const t = storage()?.getItem(LOCAL_TOKEN_KEY);
    return t ? { accessToken: t, expiresAt: Number.POSITIVE_INFINITY } : null;
  });
  const [lastLogout, setLastLogout] = useState<LogoutReason | null>(null);
  const [restoring, setRestoring] = useState(() => !!cfg && shouldRestore());
  const sessionRef = useRef(session);
  const refreshing = useRef<Promise<string | null> | null>(null);

  const update = useCallback((s: Tokens | null) => {
    sessionRef.current = s;
    setSession(s);
  }, []);

  const logout = useCallback(
    (reason: LogoutReason = "user") => {
      const s = sessionRef.current;
      setLastLogout(reason);
      removeItem(LOCAL_TOKEN_KEY);
      removeItem(ACTIVE_KEY);
      update(null);
      // 期限切れのときは Cognito のログアウトを経由しない（ログイン画面からログインし直せる）
      if (!cfg || reason === "expired") return;
      if (reason === "deleted") setItem(DELETED_KEY, "1");
      const revoke = reason === "user" && s?.refreshToken ? revokeToken(cfg, s.refreshToken) : Promise.resolve();
      void revoke.finally(() => redirectTo(logoutUrl(cfg)));
    },
    [cfg, update],
  );

  const login = useCallback(
    async (returnTo = "/app") => {
      if (!cfg) return;
      redirectTo(await authorizeUrl(cfg, returnTo));
    },
    [cfg],
  );

  const finishLogin = useCallback(
    async (search: URLSearchParams) => {
      if (!cfg) return "/app";
      const { tokens, returnTo } = await completeLogin(cfg, search);
      setItem(ACTIVE_KEY, "1");
      setLastLogout(null);
      update(tokens);
      return returnTo;
    },
    [cfg, update],
  );

  useEffect(() => {
    if (restoring) void login(window.location.pathname + window.location.search);
  }, [restoring, login]);
  // ログイン画面に移らずにこの画面に戻ってきたとき（ブラウザの「戻る」など）は、ログインし直す途中の表示をやめる
  useEffect(() => {
    const onShow = (e: PageTransitionEvent) => e.persisted && setRestoring(false);
    window.addEventListener("pageshow", onShow);
    return () => window.removeEventListener("pageshow", onShow);
  }, []);

  const loginLocal = useCallback(
    (userId: string) => {
      const u = LOCAL_USERS.find((x) => x.id === userId);
      if (!u) return;
      setItem(LOCAL_TOKEN_KEY, u.token);
      update({ accessToken: u.token, expiresAt: Number.POSITIVE_INFINITY });
    },
    [update],
  );

  /** API に付けるトークン。期限が近ければリフレッシュする（同時に呼ばれても1回だけ） */
  const getToken = useCallback(async (): Promise<string | null> => {
    const s = sessionRef.current;
    if (!s) return null;
    if (!cfg || !s.refreshToken || s.expiresAt - Date.now() > REFRESH_BEFORE_MS) return s.accessToken;
    refreshing.current ??= refreshTokens(cfg, s.refreshToken)
      .then((t) => {
        update({ ...t, email: t.email ?? s.email });
        return t.accessToken;
      })
      .catch(() => {
        logout("expired");
        return null;
      })
      .finally(() => {
        refreshing.current = null;
      });
    return refreshing.current;
  }, [cfg, update, logout]);

  const api = useMemo(() => createApi(getToken, () => logout("expired")), [getToken, logout]);
  const token = session?.accessToken ?? null;
  const userLabel = cfg ? (session?.email ?? null) : (LOCAL_USERS.find((u) => u.token === token)?.id ?? null);

  const value = useMemo<AuthState>(
    () => ({ mode: cfg ? "cognito" : "local", token, userLabel, api, lastLogout, restoring, login, finishLogin, loginLocal, logout }),
    [cfg, token, userLabel, api, lastLogout, restoring, login, finishLogin, loginLocal, logout],
  );
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthState {
  const v = useContext(AuthContext);
  if (!v) throw new Error("useAuth は AuthProvider の中で使う");
  return v;
}
