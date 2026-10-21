import Link from "next/link";

import type { Location } from "@/lib/api/client";

import { LevelBadge } from "./LevelBadge";

/** ホームの地点カード。地点の現在の最大レベルを表示する（docs/03 C-01） */
export function LocationCard({ loc }: { loc: Location }) {
  const max = Math.max(0, ...loc.currentLevels.map((l) => l.severity));
  return (
    <li className={`card level-border-${max}`}>
      <Link href={`/app/location?id=${loc.id}`} className="card-link">
        <span className="card-title">
          {loc.name}
          <span className="muted">
            （{loc.areaName}・半径{loc.radiusM / 1000}km）
          </span>
        </span>
        {loc.currentLevels.length === 0 ? (
          <span className="muted">
            <LevelBadge severity={0} /> 発表中の情報はありません
          </span>
        ) : (
          <ul className="levels" aria-label="発表中の情報">
            {loc.currentLevels.map((l) => (
              <li key={l.hazard}>
                <LevelBadge severity={l.severity} hazard={l.hazard} /> {l.title}
              </li>
            ))}
          </ul>
        )}
      </Link>
    </li>
  );
}
