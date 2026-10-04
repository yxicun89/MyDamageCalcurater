// カードをタイプ色で染めるための CSS 変数(--card-type)を作る(F-12、ADR-0331 §2)。
// 画面は選択中の種族の最初のタイプを渡し、.ui-card--typed がふち・上端の帯にこの変数を使う。
// ID は CSS の変数名に埋め込むので、英小文字・数字・ハイフンだけの値に限る(CSS の注入を防ぐ)。

import type { CSSProperties } from "react";

export const CARD_TYPE_VARIABLE = "--card-type";

const SAFE_TYPE_ID = /^[a-z0-9]+(-[a-z0-9]+)*$/;

/** 未知のタイプ ID でも壊れないよう、既定のブランド色つきの var にする。種族が未選択・不正な ID なら何も置かない。 */
export function typeAccentStyle(typeId?: string): CSSProperties {
  if (typeId === undefined || !SAFE_TYPE_ID.test(typeId)) {
    return {};
  }
  return { [CARD_TYPE_VARIABLE]: `var(--type-${typeId}, var(--brand-primary))` } as CSSProperties;
}
