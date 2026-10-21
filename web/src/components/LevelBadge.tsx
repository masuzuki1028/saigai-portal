import type { Hazard } from "@/lib/api/client";
import { hazardLabel, quakeSeverityLabel, severityStyle } from "@/lib/labels";

/** レベルの表示。色に加えてアイコンと文字でもレベルが分かるようにする（docs/03 5章・アクセシビリティ） */
export function LevelBadge({ severity, hazard }: { severity: number; hazard?: Hazard }) {
  const s = severityStyle[severity] ?? severityStyle[0]!;
  const text = hazard === "earthquake" && severity > 0 ? quakeSeverityLabel[severity] : s.label;
  return (
    <span className={`badge ${s.className}`}>
      <span aria-hidden="true">{s.icon}</span> {hazard ? `${hazardLabel[hazard]} ` : ""}
      {text}
    </span>
  );
}
