// タイプバッジ(docs/design.md「タイプバッジ」)。タイプ色のピル + タイプ名。色は var(--type-<id>, 既定値)。
// 技ピッカー・ポケモンカードで共有する。

import type { ReactElement } from "react";
import { isTypeId, typeNameJa } from "../i18n/ja";

// タイプ ID は CSS 変数名に埋め込むので、英小文字・数字・ハイフンだけに限る(typeAccent.ts と同じ)。
const SAFE_TYPE_ID = /^[a-z0-9]+(-[a-z0-9]+)*$/;

export function typeDisplayName(type: string): string {
  return isTypeId(type) ? typeNameJa[type] : type;
}

export function TypeBadge({
  type,
  className,
}: {
  readonly type: string;
  readonly className?: string;
}): ReactElement {
  const safe = SAFE_TYPE_ID.test(type);
  return (
    <span
      className={className === undefined ? "ui-badge" : `ui-badge ${className}`}
      style={{
        backgroundColor: safe ? `var(--type-${type}, var(--border-hairline))` : "var(--border-hairline)",
        color: safe ? `var(--type-${type}-ink, var(--text-primary))` : "var(--text-primary)",
      }}
    >
      {typeDisplayName(type)}
    </span>
  );
}
