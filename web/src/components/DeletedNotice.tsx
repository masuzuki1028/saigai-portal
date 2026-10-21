"use client";

import { useSearchParams } from "next/navigation";
import { Suspense } from "react";

function Inner() {
  if (useSearchParams().get("deleted") !== "1") return null;
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
