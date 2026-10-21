"use client";

import { useState } from "react";

import type { TimelineItem } from "@/lib/api/client";
import { formatDateTime } from "@/lib/labels";

import { LevelBadge } from "./LevelBadge";

const tabs = [
  { id: "all", label: "すべて" },
  { id: "official", label: "公式" },
  { id: "sns", label: "SNS" },
] as const;

const reliabilityLabel: Record<TimelineItem["reliability"], string> = {
  official: "公式",
  corroborated: "複数投稿で裏付け",
  unverified: "未確認",
  reported: "報告",
};

const statusLabel: Record<TimelineItem["status"], string> = { issued: "発表", continued: "切り替え", cleared: "解除" };

/** 地点のタイムライン（新しい順）。公式情報と SNS 投稿は色とラベルで区別する（デマ対策） */
export function Timeline({ items }: { items: TimelineItem[] }) {
  const [tab, setTab] = useState<(typeof tabs)[number]["id"]>("all");
  const shown = items.filter((it) => tab === "all" || it.kind === tab);
  return (
    <section aria-label="タイムライン">
      <div role="tablist" aria-label="表示する情報" className="tabs">
        {tabs.map((t) => (
          <button key={t.id} role="tab" type="button" aria-selected={tab === t.id} onClick={() => setTab(t.id)}>
            {t.label}
          </button>
        ))}
      </div>
      {tab === "sns" && <p className="muted">SNS の投稿の表示は Phase 2 で対応します。</p>}
      {shown.length === 0 ? (
        tab !== "sns" && <p className="empty">この地点の周辺で、表示の対象になる情報はまだありません。</p>
      ) : (
        <ol className="timeline">
          {shown.map((it) => (
            <li key={it.id} className={`timeline-item kind-${it.kind}`}>
              <div className="timeline-meta">
                <time dateTime={it.issuedAt}>{formatDateTime(it.issuedAt)}</time>
                <span className={`reliability rel-${it.reliability}`}>【{reliabilityLabel[it.reliability]}】</span>
                <LevelBadge severity={it.status === "cleared" ? 0 : it.severity} hazard={it.hazard} />
                <span className="status">{statusLabel[it.status]}</span>
              </div>
              <h3 className="timeline-title">{it.title}</h3>
              <p>{it.summary}</p>
              <p className="source">
                出典: {it.source}
                {it.sourceUrl && (
                  <>
                    {" "}
                    <a href={it.sourceUrl} target="_blank" rel="noopener noreferrer">
                      公式の情報を見る
                    </a>
                  </>
                )}
                {it.notified && <span className="muted">（通知済み）</span>}
              </p>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}
