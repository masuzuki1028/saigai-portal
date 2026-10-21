"use client";

import { useEffect, useState, type ReactNode } from "react";

import { AuthProvider } from "@/lib/auth/AuthProvider";

/** NEXT_PUBLIC_API_MOCK=1 のときは、MSW のモック API を起動してから画面を表示する */
export function Providers({ children }: { children: ReactNode }) {
  const mock = process.env.NEXT_PUBLIC_API_MOCK === "1";
  const [ready, setReady] = useState(!mock);
  useEffect(() => {
    if (!mock) return;
    void import("@/mocks/browser").then(({ startMockWorker }) => startMockWorker()).then(() => setReady(true));
  }, [mock]);
  if (!ready) return null;
  return <AuthProvider>{children}</AuthProvider>;
}
