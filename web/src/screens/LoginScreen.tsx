"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";

import { LOCAL_USERS, useAuth } from "@/lib/auth/AuthProvider";

/**
 * ログイン。いまはローカル開発用の固定ユーザーだけを選べる。
 * Cognito（メール＋パスワード / パスキー、MFA）のログインは AWS への構築時に追加する。
 */
export function LoginScreen() {
  const { token, loginLocal } = useAuth();
  const router = useRouter();
  useEffect(() => {
    if (token) router.replace("/app");
  }, [token, router]);
  return (
    <main className="main narrow">
      <h1>ログイン</h1>
      <p className="notice">ローカル開発用のログインです。利用者を選んでください。</p>
      <ul className="login-users">
        {LOCAL_USERS.map((u) => (
          <li key={u.id}>
            <button type="button" onClick={() => loginLocal(u.id)}>
              {u.label} としてログイン
            </button>
          </li>
        ))}
      </ul>
    </main>
  );
}
