import "./globals.css";

import type { Metadata, Viewport } from "next";
import type { ReactNode } from "react";

import { Providers } from "@/components/Providers";

export const metadata: Metadata = {
  title: "saigai-timeline",
  description: "登録した場所の周りで起きている災害の情報を、1本のタイムラインで確認できるサービス",
};

export const viewport: Viewport = { width: "device-width", initialScale: 1, themeColor: "#1d4ed8" };

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="ja">
      <body>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
