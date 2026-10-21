"use client";

import { useEffect, useRef } from "react";

/** 平常時は60秒、警報級の情報が出ているときは30秒ごと（docs/03_screens.md 5章） */
export const NORMAL_INTERVAL_MS = 60_000;
export const ALERT_INTERVAL_MS = 30_000;
/** アクセスが集中しないよう、間隔に最大 ±10% のばらつき（ジッター）を入れる */
export const JITTER_RATIO = 0.1;

export function nextDelay(alert: boolean, random: () => number = Math.random): number {
  const base = alert ? ALERT_INTERVAL_MS : NORMAL_INTERVAL_MS;
  return Math.round(base * (1 - JITTER_RATIO + random() * 2 * JITTER_RATIO));
}

/** fn を定期的に呼ぶ。画面が非表示のときは呼ばない */
export function usePolling(fn: () => void, alert: boolean, enabled = true) {
  const fnRef = useRef(fn);
  fnRef.current = fn;
  useEffect(() => {
    if (!enabled) return;
    let timer: ReturnType<typeof setTimeout>;
    const tick = () => {
      timer = setTimeout(() => {
        if (typeof document === "undefined" || document.visibilityState !== "hidden") fnRef.current();
        tick();
      }, nextDelay(alert));
    };
    tick();
    return () => clearTimeout(timer);
  }, [alert, enabled]);
}
