import { renderHook } from "@testing-library/react";

import { ALERT_INTERVAL_MS, nextDelay, NORMAL_INTERVAL_MS, usePolling } from "./usePolling";

describe("usePolling", () => {
  afterEach(() => vi.useRealTimers());

  it("平常時は60秒、警報級は30秒を中心に ±10% のばらつきを入れる", () => {
    expect(nextDelay(false, () => 0)).toBe(NORMAL_INTERVAL_MS * 0.9);
    expect(nextDelay(false, () => 1)).toBe(NORMAL_INTERVAL_MS * 1.1);
    expect(nextDelay(true, () => 0.5)).toBe(ALERT_INTERVAL_MS);
  });

  it("間隔ごとに呼び、画面が非表示のときは呼ばない", () => {
    vi.useFakeTimers();
    const fn = vi.fn();
    renderHook(() => usePolling(fn, true));
    vi.advanceTimersByTime(ALERT_INTERVAL_MS * 1.1);
    expect(fn).toHaveBeenCalledTimes(1);

    Object.defineProperty(document, "visibilityState", { value: "hidden", configurable: true });
    vi.advanceTimersByTime(ALERT_INTERVAL_MS * 1.1);
    expect(fn).toHaveBeenCalledTimes(1);
    Object.defineProperty(document, "visibilityState", { value: "visible", configurable: true });
  });
});
