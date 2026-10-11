// テスト専用(ADR-0356): ナビは上位 4 区分のタブ(ダメージ・構築・素早さ・記録)+ ダメージの下位タブ(計算・逆算・調整)。
// 画面名で「その画面のタブ」を引く・開く道具。区分が違っても同じ呼び方で、テストの意図(その画面を開く)を保つ。

import { screen } from "@testing-library/react";
import type userEvent from "@testing-library/user-event";

type User = ReturnType<typeof userEvent.setup>;

/** 画面名 → 上位のタブの名前(下位のタブがある画面は区分の名前)。 */
const GROUP_LABEL: Readonly<Record<string, string>> = {
  計算: "ダメージ",
  逆算: "ダメージ",
  調整: "ダメージ",
  構築: "構築",
  素早さ: "素早さ",
  お気に入り: "記録",
};

/** 下位のタブがある画面(ダメージの中)。 */
const SUB_TAB_SCREENS: readonly string[] = ["計算", "逆算", "調整"];

/** 画面名のタブ(下位のタブがある画面はその下位のタブ、他は上位のタブ)を返す。 */
export function screenTab(screenName: string): HTMLElement {
  if (SUB_TAB_SCREENS.includes(screenName)) {
    return screen.getByRole("tab", { name: screenName });
  }
  return screen.getByRole("tab", { name: groupLabelOf(screenName) });
}

export function groupLabelOf(screenName: string): string {
  return GROUP_LABEL[screenName] ?? screenName;
}

/** 画面を開く(上位のタブ → 必要なら下位のタブ)。 */
export async function openTab(user: User, screenName: string): Promise<void> {
  const group = screen.getByRole("tab", { name: groupLabelOf(screenName) });
  if (group.getAttribute("aria-selected") !== "true") {
    await user.click(group);
  }
  if (SUB_TAB_SCREENS.includes(screenName)) {
    const sub = screen.getByRole("tab", { name: screenName });
    if (sub.getAttribute("aria-selected") !== "true") {
      await user.click(sub);
    }
  }
}
