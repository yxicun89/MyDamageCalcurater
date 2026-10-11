// ナビの区分(ADR-0356)。画面の登録ファイルは NAV_GROUPS の区分を `group` に指定する。
// 指定の無い画面は、その画面だけの区分になる(app/routes.ts)。区分が 1 画面だけなら下位のタブは出さない。

import { appText } from "../i18n/ja";
import type { IconName } from "../ui/Icon";

export interface NavGroup {
  readonly id: string;
  /** 上位のタブの表示名。 */
  readonly label: string;
  /** 上位のタブの並び(昇順)。 */
  readonly order: number;
  readonly icon: IconName;
}

export const NAV_GROUPS = {
  damage: { id: "damage", label: appText.navDamageLabel, order: 100, icon: "movePhysical" },
  team: { id: "team", label: appText.teamTabLabel, order: 200, icon: "team" },
  speed: { id: "speed", label: appText.navSpeedLabel, order: 300, icon: "speed" },
  records: { id: "records", label: appText.navRecordsLabel, order: 400, icon: "favorites" },
} as const satisfies Record<string, NavGroup>;

/** 上位のタブ 1 つ分(区分と、その中の画面 ID。画面の順は routes の並びのまま)。 */
export interface NavTab {
  readonly group: NavGroup;
  readonly screenIds: readonly string[];
}

/** 画面(routes の並び)を区分ごとにまとめ、区分の order の昇順に並べる(同じ order は先に出た順)。 */
export function buildNavTabs(
  routes: readonly { readonly id: string; readonly group: NavGroup }[],
): readonly NavTab[] {
  const tabs: { group: NavGroup; screenIds: string[] }[] = [];
  for (const route of routes) {
    const existing = tabs.find((tab) => tab.group.id === route.group.id);
    if (existing === undefined) {
      tabs.push({ group: route.group, screenIds: [route.id] });
    } else {
      existing.screenIds.push(route.id);
    }
  }
  return tabs.sort((a, b) => a.group.order - b.group.order);
}
