"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, type ReactNode } from "react";

import { useAuth } from "@/lib/auth/AuthProvider";

import { Disclaimer } from "./Disclaimer";

/** ログインが必要な画面の枠。未ログインならログイン画面へ移る */
export function AppShell({ title, back, children }: { title: string; back?: string; children: ReactNode }) {
  const { token, userLabel, logout } = useAuth();
  const router = useRouter();
  useEffect(() => {
    if (!token) router.replace("/login");
  }, [token, router]);
  if (!token) return null;
  return (
    <>
      <header className="header">
        {back ? (
          <Link href={back} className="back" aria-label="戻る">
            ←
          </Link>
        ) : (
          <Link href="/app" className="brand">
            saigai-timeline
          </Link>
        )}
        <h1>{title}</h1>
        <nav aria-label="アカウント" className="header-nav">
          <Link href="/app/settings/notifications">通知</Link>
          <button type="button" className="link" onClick={logout}>
            ログアウト{userLabel ? `（${userLabel}）` : ""}
          </button>
        </nav>
      </header>
      <main className="main">{children}</main>
      <footer className="footer">
        <Disclaimer />
        <p>
          出典: 気象庁 / 地図: 国土地理院 · <Link href="/disclaimer">免責・出典</Link>
        </p>
      </footer>
    </>
  );
}
