// 画面 ID → タブのアイコン名の対応表(F-12、ADR-0334 §2)。画面の登録ファイル(*.screen.tsx)は変えない。
// 対応表に無い画面(後から足された画面)は既定のアイコンになり、タブ列は壊れない。

import type { IconName } from "../ui/Icon";

const SCREEN_ICONS: Readonly<Record<string, IconName>> = {
  calc: "calc",
  reverse: "reverse",
  balance: "balance",
  speed: "speed",
  judge: "judge",
  team: "team",
  favorites: "favorites",
  adjust: "adjust",
};

export function screenIconName(screenId: string): IconName {
  return SCREEN_ICONS[screenId] ?? "screen";
}
