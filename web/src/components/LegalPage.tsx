import Link from "next/link";
import type { ReactNode } from "react";

/** 利用規約・プライバシーポリシーなどの文書の枠 */
export function LegalPage({ title, updated, children }: { title: string; updated: string; children: ReactNode }) {
  return (
    <main className="main narrow legal">
      <h1>{title}</h1>
      <p className="notice">
        この文書はたたき台です。正式な公開の前に内容を確認し、改めて掲載します。（最終更新: {updated}）
      </p>
      {children}
      <p>
        <Link href="/">トップへ</Link> · <Link href="/terms">利用規約</Link> · <Link href="/privacy">プライバシーポリシー</Link> ·{" "}
        <Link href="/disclaimer">免責・出典</Link>
      </p>
    </main>
  );
}
