"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { LOCAL_USERS, useAuth } from "@/lib/auth/AuthProvider";

/**
 * ログイン。本番は Cognito のマネージドログイン（メール＋パスワード、MFA）に移る。
 * ローカル・モックでは、開発用の固定ユーザーを選ぶ。
 */
export function LoginScreen() {
  const { mode, token, lastLogout, login, loginLocal } = useAuth();
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    if (token) router.replace("/app");
  }, [token, router]);

  if (mode === "cognito") {
    return (
      <main className="main narrow">
        <h1>ログイン</h1>
        {lastLogout === "expired" && (
          <p role="status" className="notice">
            ログインの有効期限が切れました。もう一度ログインしてください。
          </p>
        )}
        <p>メールアドレスとパスワードでログインします。はじめての方は、ログイン画面から新規登録できます。</p>
        <button
          type="button"
          disabled={busy}
          onClick={() => {
            setBusy(true);
            void login("/app");
          }}
        >
          {busy ? "ログイン画面に移動しています…" : "ログイン・新規登録"}
        </button>
      </main>
    );
  }
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
