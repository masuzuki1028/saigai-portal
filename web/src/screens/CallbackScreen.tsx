"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";

import { useAuth } from "@/lib/auth/AuthProvider";
import { LoginError } from "@/lib/auth/cognito";

/** Cognito のログイン画面から戻ってくる画面（/auth/callback）。認可コードをトークンに交換する */
export function CallbackScreen() {
  const { finishLogin } = useAuth();
  const router = useRouter();
  const [error, setError] = useState<string | null>(null);
  const started = useRef(false);

  useEffect(() => {
    // 認可コードは1回しか使えないため、開発時の StrictMode で2回呼ばれても交換は1回にする
    if (started.current) return;
    started.current = true;
    finishLogin(new URLSearchParams(window.location.search))
      .then((to) => router.replace(to))
      .catch((e: unknown) => setError(e instanceof LoginError ? e.message : "ログインを完了できませんでした。時間をおいてもう一度お試しください"));
  }, [finishLogin, router]);

  return (
    <main className="main narrow">
      <h1>ログイン</h1>
      {error ? (
        <>
          <p role="alert" className="error">
            {error}
          </p>
          <p>
            <Link href="/login">ログイン画面に戻る</Link>
          </p>
        </>
      ) : (
        <p role="status">ログインしています…</p>
      )}
    </main>
  );
}
