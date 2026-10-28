"use client";

import { useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";

import { DELETED_KEY } from "@/lib/auth/AuthProvider";

function Inner() {
  const query = useSearchParams().get("deleted") === "1";
  // Cognito のログアウトを経由して戻ったときは、sessionStorage の目印で表示する（1回だけ）
  const [flag, setFlag] = useState(false);
  useEffect(() => {
    try {
      if (window.sessionStorage.getItem(DELETED_KEY) === "1") {
        window.sessionStorage.removeItem(DELETED_KEY);
        setFlag(true);
      }
    } catch {
      /* 読めない環境では表示しない */
    }
  }, []);
  if (!query && !flag) return null;
  return (
    <p role="status" className="success">
      退会しました。登録したデータはすべて削除しました。
    </p>
  );
}

/** 退会した直後にトップで表示するお知らせ */
export function DeletedNotice() {
  return (
    <Suspense fallback={null}>
      <Inner />
    </Suspense>
  );
}
