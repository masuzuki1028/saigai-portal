"use client";

import { useSearchParams } from "next/navigation";
import { Suspense, type ReactNode } from "react";

/** 静的出力では動的ルートが使えないため、ID はクエリパラメータ（?id=）で受け取る（docs/03 1章） */
function Inner({ render, optional }: { render: (id: string | null) => ReactNode; optional?: boolean }) {
  const id = useSearchParams().get("id");
  if (!id && !optional) return <p className="error">地点が指定されていません</p>;
  return <>{render(id)}</>;
}

export function WithId(props: { render: (id: string | null) => ReactNode; optional?: boolean }) {
  return (
    <Suspense fallback={null}>
      <Inner {...props} />
    </Suspense>
  );
}
