import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterAll, afterEach, beforeAll, vi } from "vitest";

import { server } from "@/mocks/node";

// Next.js のルーターはテストでは動かないため、呼び出しを記録するモックにする
export const router = { push: vi.fn(), replace: vi.fn(), back: vi.fn() };
vi.mock("next/navigation", () => ({
  useRouter: () => router,
  useSearchParams: () => new URLSearchParams(window.location.search),
  usePathname: () => window.location.pathname,
}));

// MapLibre GL JS は WebGL が必要なため、テストではモックに差し替える（docs/08 4.1）
vi.mock("@/components/map/MapView", () => ({
  MapView: ({ label }: { label?: string }) => <div role="img" aria-label={label ?? "地図"} />,
}));
vi.mock("@/components/map/MapPicker", () => ({
  MapPicker: ({ onChange }: { onChange: (c: { lat: number; lon: number }) => void }) => (
    <button type="button" onClick={() => onChange({ lat: 37.4462, lon: 138.8512 })}>
      地図で地点を選ぶ（テスト）
    </button>
  ),
}));

beforeAll(() => server.listen({ onUnhandledFrame: "error" }));
afterEach(() => {
  server.resetHandlers();
  cleanup();
  vi.clearAllMocks();
  window.sessionStorage.clear();
});
afterAll(() => server.close());
