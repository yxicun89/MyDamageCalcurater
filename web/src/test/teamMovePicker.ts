// テスト専用(I-web-13d2、ADR-0350): 構築の技ピッカー(技1〜技4)を、枠(group)の中で操作・読み取る道具。
// test/movePicker.ts は画面に 1 つだけの技ピッカー用(screen 全体から引く)。構築は枠ごとに「技1」〜「技4」があり
// 複数の枠が同時に出るため、枠の要素の中から引く。旧: select(group, 技N) の value / selectOptions / option。

import { fireEvent, within } from "@testing-library/react";
import type userEvent from "@testing-library/user-event";
import { teamMemberText } from "../i18n/team";

type User = ReturnType<typeof userEvent.setup>;

/** 枠の中の技 N のトリガー(role=combobox・名前「技N」)。 */
export function teamMoveTrigger(group: HTMLElement, slot: number): HTMLElement {
  return within(group).getByRole("combobox", { name: teamMemberText.moveLabel(slot) });
}

/** 選択中の技 ID(空き枠は "")。旧 `select(group, 技N).value` に相当。 */
export function teamMoveValue(group: HTMLElement, slot: number): string {
  return teamMoveTrigger(group, slot).getAttribute("data-value") ?? "";
}

/** 開いていなければ開いて、listbox を返す。 */
export async function openTeamMove(user: User, group: HTMLElement, slot: number): Promise<HTMLElement> {
  const trigger = teamMoveTrigger(group, slot);
  if (trigger.getAttribute("aria-expanded") !== "true") {
    await user.click(trigger);
  }
  return within(group).getByRole("listbox", { name: teamMemberText.moveLabel(slot) });
}

/** 開いて、行(option)を並び順に返す。「(なし)」の行は先頭(data-move-id="")。旧 `within(select).getAllByRole("option")` に相当。 */
export async function teamMoveRows(user: User, group: HTMLElement, slot: number): Promise<HTMLElement[]> {
  const listbox = await openTeamMove(user, group, slot);
  return within(listbox).getAllByRole("option");
}

/** 行の表示名(技名。「(なし)」の行はその文言)。旧 option の表示文字に相当。 */
export function teamMoveRowName(row: HTMLElement): string {
  return row.querySelector(".move-picker__name")?.textContent ?? "";
}

/** 開いて、技 ID(空にするなら "")の行を押す。旧 `user.selectOptions(select, id)` に相当。 */
export async function chooseTeamMove(
  user: User,
  group: HTMLElement,
  slot: number,
  id: string,
): Promise<void> {
  const listbox = await openTeamMove(user, group, slot);
  const row = listbox.querySelector(`[role="option"][data-move-id="${id}"]`);
  if (row === null) {
    throw new Error(`技 ${id} の行が無い`);
  }
  await user.click(row);
}

/** 同期版(waitFor の中でも使える)。閉じていれば開き、行の技名を並び順に返す。 */
export function teamMoveRowNamesSync(group: HTMLElement, slot: number): string[] {
  const trigger = teamMoveTrigger(group, slot);
  if (trigger.getAttribute("aria-expanded") !== "true") {
    fireEvent.click(trigger);
  }
  const listbox = within(group).getByRole("listbox", { name: teamMemberText.moveLabel(slot) });
  return within(listbox).getAllByRole("option").map(teamMoveRowName);
}
