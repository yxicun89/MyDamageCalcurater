// ADR-0350: 技ピッカーの「空にする」行(noneLabel)と選べない技(disabledIds)。構築の技 1〜4 枠で使う。
//   N-1 noneLabel を渡すと先頭に選べる行(data-move-id="")が出て、選ぶと onChange("") で閉じる。空のときトリガーにも同じ文言
//   N-2 検索で絞っている間は空にする行を出さない
//   N-3 ArrowUp/Down・Home でこの行にも辿れ、Enter で空にできる
//   N-4 disabledIds の技の行は aria-disabled で、押しても・Enter でも選ばれない(他の行は選べる)
//   N-5 タイプが空の技(マスタに無い現在値の代役)はタイプバッジを出さず、タイプ名も読まない
//   N-6 noneLabel を渡さなければ従来どおり(計算・逆算の挙動は変わらない)
// 架空のデータだけ(test/moveSortMaster.ts)。

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, test, vi } from "vitest";
import { moveTrigger, openMovePicker, rowIds, selectedMoveId } from "../test/movePicker";
import { SORT_MOVES } from "../test/moveSortMaster";
import { MovePicker } from "./MovePicker";

const TYPES = ["electric", "fire", "normal"];
const NONE = "(なし)";

function Harness({
  initial = "",
  onChange,
  noneLabel = NONE,
  disabledIds,
  moves = SORT_MOVES,
}: {
  initial?: string;
  onChange?: (id: string) => void;
  noneLabel?: string;
  disabledIds?: ReadonlySet<string>;
  moves?: typeof SORT_MOVES;
}) {
  const [value, setValue] = useState(initial);
  return (
    <MovePicker
      label="技"
      moves={moves}
      types={TYPES}
      value={value}
      onChange={(id) => {
        setValue(id);
        onChange?.(id);
      }}
      noneLabel={noneLabel}
      disabledIds={disabledIds}
    />
  );
}

describe("N-1 空にする行", () => {
  test("先頭に選べる行。空のときトリガーにも文言。選ぶと空になって閉じる", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness initial="examplesortspark" onChange={onChange} />);
    expect(moveTrigger()).toHaveTextContent("スパーク");
    const listbox = await openMovePicker(user);
    const ids = rowIds(listbox);
    expect(ids[0]).toBe("");
    expect(ids).toHaveLength(SORT_MOVES.length + 1);
    const none = within(listbox).getByRole("option", { name: NONE });
    expect(none).not.toHaveAttribute("aria-disabled");
    await user.click(none);
    expect(onChange).toHaveBeenCalledWith("");
    expect(selectedMoveId()).toBe("");
    expect(moveTrigger()).toHaveTextContent(NONE);
    expect(screen.queryByRole("listbox")).toBeNull();
  });

  test("空のときは空にする行が選択中(aria-selected)", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const listbox = await openMovePicker(user);
    expect(within(listbox).getByRole("option", { name: NONE })).toHaveAttribute("aria-selected", "true");
  });

  test("技が 1 つも無くても、空にする行だけは出る", async () => {
    const user = userEvent.setup();
    render(<Harness moves={[]} />);
    const listbox = await openMovePicker(user);
    expect(rowIds(listbox)).toEqual([""]);
  });
});

describe("N-2 検索中", () => {
  test("絞り込んでいる間は空にする行を出さない", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const listbox = await openMovePicker(user);
    await user.type(screen.getByRole("searchbox"), "スパ");
    expect(rowIds(listbox)).toEqual(["examplesortspark"]);
  });
});

describe("N-3 キーボード", () => {
  test("Home で空にする行へ。Enter で空にできる", async () => {
    const user = userEvent.setup();
    render(<Harness initial="examplesortspark" />);
    await openMovePicker(user);
    await user.keyboard("{Home}");
    expect(screen.getByRole("searchbox")).toHaveAttribute(
      "aria-activedescendant",
      expect.stringMatching(/-none$/),
    );
    await user.keyboard("{Enter}");
    expect(selectedMoveId()).toBe("");
  });

  test("空で開くと空にする行が活性。ArrowDown で先頭の技へ", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await openMovePicker(user);
    expect(screen.getByRole("searchbox")).toHaveAttribute(
      "aria-activedescendant",
      expect.stringMatching(/-none$/),
    );
    await user.keyboard("{ArrowDown}");
    await user.keyboard("{Enter}");
    expect(selectedMoveId()).toBe("examplesortthunder");
  });
});

describe("N-4 選べない技", () => {
  test("aria-disabled の行は押しても Enter でも選ばれない。他の行は選べる", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness onChange={onChange} disabledIds={new Set(["examplesortspark"])} />);
    const listbox = await openMovePicker(user);
    const spark = listbox.querySelector('[data-move-id="examplesortspark"]');
    expect(spark).toHaveAttribute("aria-disabled", "true");
    await user.click(spark as HTMLElement);
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByRole("listbox")).toBeInTheDocument();
    expect(listbox.querySelector('[data-move-id="examplesorttackle"]')).not.toHaveAttribute("aria-disabled");
    await user.type(screen.getByRole("searchbox"), "スパーク{Enter}");
    expect(onChange).not.toHaveBeenCalled();
    await user.clear(screen.getByRole("searchbox"));
    await user.click(listbox.querySelector('[data-move-id="examplesorttackle"]') as HTMLElement);
    expect(onChange).toHaveBeenCalledWith("examplesorttackle");
  });
});

describe("N-5 タイプが空の技", () => {
  const ORPHAN = [
    { id: "ghost", nameJa: "ghost", type: "", category: "status" as const, power: 0, priority: 0 },
  ];

  test("バッジを出さず、行の読み上げ名にタイプ名が入らない", async () => {
    const user = userEvent.setup();
    render(<Harness initial="ghost" moves={ORPHAN} />);
    expect(moveTrigger().querySelector(".ui-badge")).toBeNull();
    const listbox = await openMovePicker(user);
    const row = within(listbox).getByRole("option", { name: "ghost、へんか" });
    expect(row.querySelector(".ui-badge")).toBeNull();
  });
});

describe("N-6 noneLabel なし", () => {
  test("空にする行は出ない", async () => {
    const user = userEvent.setup();
    render(<MovePicker label="技" moves={SORT_MOVES} types={TYPES} value="" onChange={() => undefined} />);
    const listbox = await openMovePicker(user);
    expect(rowIds(listbox)).not.toContain("");
  });
});
