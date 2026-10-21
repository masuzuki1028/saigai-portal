"use client";

import { useRef, useState } from "react";

import { searchAddress, type AddressCandidate } from "@/lib/geo/addressSearch";

/** 住所で地点を探す。選んだ候補の座標を地点にする（検索に使った住所は保存しない） */
export function AddressSearch({ onSelect }: { onSelect: (c: AddressCandidate) => void }) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<AddressCandidate[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const abort = useRef<AbortController | null>(null);

  const search = async () => {
    abort.current?.abort();
    const ctrl = new AbortController();
    abort.current = ctrl;
    setBusy(true);
    setError(null);
    try {
      setResults(await searchAddress(query, ctrl.signal));
    } catch (e) {
      if ((e as Error).name !== "AbortError") setError("住所を検索できませんでした。地図で選んでください");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="address-search">
      <div className="inline">
        <label className="grow">
          住所で探す
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => {
              // フォーム全体の送信（地点の登録）にならないようにする
              if (e.key === "Enter") {
                e.preventDefault();
                void search();
              }
            }}
            placeholder="例: 新潟県長岡市城内町"
            autoComplete="off"
          />
        </label>
        <button type="button" className="secondary" onClick={() => void search()} disabled={busy || !query.trim()}>
          検索
        </button>
      </div>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {results && (
        <div aria-live="polite">
          {results.length === 0 ? (
            <p className="muted">見つかりませんでした。市区町村名から入力するか、地図で選んでください。</p>
          ) : (
            <ul className="candidates" aria-label="住所の候補">
              {results.map((r) => (
                <li key={`${r.title}-${r.lat}-${r.lon}`}>
                  <button
                    type="button"
                    className="link"
                    onClick={() => {
                      onSelect(r);
                      setResults(null);
                    }}
                  >
                    {r.title}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
      <p className="muted">検索に使った住所は保存しません。出典: 国土地理院 住所検索</p>
    </div>
  );
}
