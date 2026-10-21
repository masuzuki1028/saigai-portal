"use client";

import { useState, type FormEvent } from "react";

import { AppShell } from "@/components/AppShell";
import { errorMessage } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/AuthProvider";

/**
 * アカウント設定（docs/03 C-07）。いまは退会（全データの削除）だけ。
 * メールアドレスの変更・MFA・パスキー・ログイン履歴は Cognito の導入時に追加する。
 */
export function AccountScreen() {
  const { api, logout } = useAuth();
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (confirm !== "削除") return setError("確認のため「削除」と入力してください");
    setBusy(true);
    const { error } = await api.POST("/me/delete", { body: { confirm: "delete" } });
    setBusy(false);
    if (error) return setError(errorMessage(error));
    logout("deleted"); // AppShell がトップ（/?deleted=1）に移す
  };

  return (
    <AppShell title="アカウント" back="/app">
      <section aria-labelledby="delete-heading" className="form">
        <h2 id="delete-heading">退会</h2>
        <p>
          登録した地点・タイムライン・通知の設定をすべて削除します。<strong>元に戻すことはできません。</strong>
        </p>
        <form onSubmit={submit} className="form" noValidate>
          {error && (
            <p role="alert" className="error">
              {error}
            </p>
          )}
          <label>
            確認のため「削除」と入力してください
            <input value={confirm} onChange={(e) => setConfirm(e.target.value)} autoComplete="off" />
          </label>
          <button type="submit" className="danger" disabled={busy}>
            {busy ? "削除しています…" : "退会してデータを削除する"}
          </button>
        </form>
      </section>
    </AppShell>
  );
}
