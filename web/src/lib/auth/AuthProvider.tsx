"use client";

import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from "react";

import { createApi, type Api } from "@/lib/api/client";

/**
 * ログイン状態。トークンはメモリに持ち、localStorage には置かない（docs/07_security.md 3.6）。
 *
 * いまはローカル用の固定トークン（api の cmd/local・MSW が受け付ける）だけに対応している。
 * Cognito（Authorization Code + PKCE）は AWS への構築時に追加する。
 * ローカル用の固定トークンは本物の資格情報ではないため、再読み込みでログアウトしないよう sessionStorage に置く。
 */
export const LOCAL_USERS = [
  { id: "alice", label: "alice（シナリオの地点を登録済み）", token: "local-alice" },
  { id: "bob", label: "bob", token: "local-bob" },
] as const;

const SESSION_KEY = "saigai.localToken";

type AuthState = {
  token: string | null;
  userLabel: string | null;
  api: Api;
  loginLocal: (userId: string) => void;
  logout: () => void;
};

const AuthContext = createContext<AuthState | null>(null);

function readSession(): string | null {
  try {
    return typeof window === "undefined" ? null : window.sessionStorage.getItem(SESSION_KEY);
  } catch {
    return null;
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(() => readSession());

  const logout = useCallback(() => {
    try {
      window.sessionStorage.removeItem(SESSION_KEY);
    } catch {
      /* 保存できない環境では何もしない */
    }
    setToken(null);
  }, []);

  const loginLocal = useCallback((userId: string) => {
    const u = LOCAL_USERS.find((x) => x.id === userId);
    if (!u) return;
    try {
      window.sessionStorage.setItem(SESSION_KEY, u.token);
    } catch {
      /* 保存できなくてもメモリ上ではログインできる */
    }
    setToken(u.token);
  }, []);

  const api = useMemo(() => createApi(() => token, logout), [token, logout]);
  const userLabel = LOCAL_USERS.find((u) => u.token === token)?.id ?? null;

  const value = useMemo(() => ({ token, userLabel, api, loginLocal, logout }), [token, userLabel, api, loginLocal, logout]);
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthState {
  const v = useContext(AuthContext);
  if (!v) throw new Error("useAuth は AuthProvider の中で使う");
  return v;
}
