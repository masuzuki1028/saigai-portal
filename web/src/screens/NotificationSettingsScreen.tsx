"use client";

import { useEffect, useState } from "react";

import { AppShell } from "@/components/AppShell";
import { errorMessage } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/AuthProvider";

/** 通知設定。MVP はアカウントのメールアドレスへの通知のみ。docs/03 C-05 */
export function NotificationSettingsScreen() {
  const { api, token } = useAuth();
  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [message, setMessage] = useState<{ kind: "status" | "alert"; text: string } | null>(null);

  useEffect(() => {
    if (!token) return;
    void api.GET("/me/notification-settings").then(({ data }) => data && setEnabled(data.emailEnabled));
  }, [api, token]);

  const toggle = async (v: boolean) => {
    // 押したらすぐに切り替え、保存に失敗したら元に戻す
    setEnabled(v);
    const { data, error } = await api.PUT("/me/notification-settings", { body: { emailEnabled: v } });
    if (error || !data) {
      setEnabled(!v);
      return setMessage({ kind: "alert", text: errorMessage(error) });
    }
    setEnabled(data.emailEnabled);
    setMessage({ kind: "status", text: data.emailEnabled ? "メール通知をオンにしました" : "メール通知をオフにしました" });
  };

  const sendTest = async () => {
    const { error } = await api.POST("/me/notification-settings/test");
    setMessage(error ? { kind: "alert", text: errorMessage(error) } : { kind: "status", text: "テスト通知を送信しました。メールを確認してください" });
  };

  return (
    <AppShell title="通知設定" back="/app">
      {message && (
        <p role={message.kind} className={message.kind === "alert" ? "error" : "success"}>
          {message.text}
        </p>
      )}
      {enabled !== null && (
        <div className="form">
          <label className="checkbox">
            <input type="checkbox" checked={enabled} onChange={(e) => void toggle(e.target.checked)} />
            メールで通知する（アカウントのメールアドレスに届きます）
          </label>
          <p className="muted">どのレベルから通知するかは、地点ごとの設定で選べます。Web Push は Phase 2 で対応します。</p>
          <button type="button" className="secondary" onClick={() => void sendTest()}>
            テスト通知を送る
          </button>
        </div>
      )}
    </AppShell>
  );
}
