// テスト専用(G-01、ADR-0341): 技ピッカー(MovePicker)を操作・読み取る道具。
// 旧: <select> を getByRole("combobox", {name:"技"}) で引き、selectOptions / toHaveValue / option で読んでいた。
// 新: トリガー(role=combobox・名前「技」・data-value=選択中の技 ID)を押すと listbox(名前「技」)が開き、
//     各行は role=option で data-move-id を持つ。閉じている間は listbox / option は DOM に無い。

import { fireEvent, screen, within } from "@testing-library/react";
import type userEvent from "@testing-library/user-event";

type User = ReturnType<typeof userEvent.setup>;

/** 技ピッカーのトリガー(閉じていても開いていても同じ)。 */
export const moveTrigger = (): HTMLElement => screen.getByRole("combobox", { name: "技" });

/** 選択中の技 ID(未選択は空文字)。旧 `expect(select).toHaveValue(id)` に相当。 */
export function selectedMoveId(): string {
  return moveTrigger().getAttribute("data-value") ?? "";
}

/** 開いていなければ開く。開いた listbox を返す。 */
export async function openMovePicker(user: User): Promise<HTMLElement> {
  if (moveTrigger().getAttribute("aria-expanded") !== "true") {
    await user.click(moveTrigger());
  }
  return screen.getByRole("listbox", { name: "技" });
}

/** 行(option)の技 ID。未選択の行は ""。 */
export function rowIds(listbox: HTMLElement): string[] {
  return within(listbox)
    .getAllByRole("option")
    .map((row) => row.getAttribute("data-move-id") ?? "?");
}

/** 開いて、選べる技の ID を並び順に返す(未選択の行 "" は除く)。 */
export async function listedMoveIds(user: User): Promise<string[]> {
  const listbox = await openMovePicker(user);
  return rowIds(listbox).filter((id) => id !== "");
}

/** 開いて、その技の行を押す(旧 `user.selectOptions(select, id)` に相当)。 */
export async function chooseMove(user: User, id: string): Promise<void> {
  const listbox = await openMovePicker(user);
  const row = listbox.querySelector(`[role="option"][data-move-id="${id}"]`);
  if (row === null) {
    throw new Error(`技 ${id} の行が無い`);
  }
  await user.click(row);
}

/**
 * 同期版: 閉じていて押せるなら開き(RTL の fireEvent は act に包まれる)、並んでいる技の行(option)を返す。
 * 押せない(disabled)・技が無いときは []。未選択の行("")は含めない。waitFor の中でも使える。
 * 旧 `within(select).queryAllByRole("option")` に相当。
 */
export function listedOptions(): HTMLElement[] {
  const trigger = moveTrigger();
  if (trigger.getAttribute("aria-expanded") !== "true" && !(trigger as HTMLButtonElement).disabled) {
    fireEvent.click(trigger);
  }
  const listbox = screen.queryByRole("listbox", { name: "技" });
  if (listbox === null) {
    return [];
  }
  return within(listbox)
    .queryAllByRole("option")
    .filter((row) => row.getAttribute("data-move-id") !== "");
}

/** 行の技名(表示している技名の文字)。 */
export function optionName(row: HTMLElement): string {
  return row.querySelector(".move-picker__name")?.textContent ?? "";
}

/** 技名で行を探す(文字列は完全一致、正規表現は部分一致。無ければ null)。 */
export function listedOption(name: string | RegExp): HTMLElement | null {
  return (
    listedOptions().find((row) =>
      typeof name === "string" ? optionName(row) === name : name.test(optionName(row)),
    ) ?? null
  );
}
