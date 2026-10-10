// G-01(ADR-0341、docs/design.md「見た目の作り直し方針(G-05)」): 技ピッカー MovePicker(screens/MovePicker.tsx)。
// <select> の代わりに、トリガー(role=combobox・名前「技」)+ 検索つきの listbox パネルで出す。
// 受け入れ条件(この単体で確かめるもの):
//   P-1 トリガー: role=combobox・aria-haspopup=listbox・aria-expanded・名前は「技」・data-value=選択中の技 ID。
//       閉じている間は listbox / option が DOM に無い。選択中の技名と分類アイコンを見せる
//   P-2 行(role=option)は 1 行: 技名 + タイプ(バッジ。.ui-badge)+ 分類アイコン + 威力。
//       accessible name は「技名・タイプ名・分類(ぶつり/とくしゅ)・威力 N」をこの順で含む。
//       分類アイコンの読み上げ名は ぶつり/とくしゅ(用語集の語。物理/特殊の漢字にしない)
//   P-3 並びはタイプ順だけ(orderMoves)。並びの切り替え(radiogroup)は無い
//   P-4 キーボード: トリガーで ArrowDown/Enter/Space で開く。開くと検索欄(searchbox・名前「技を検索」)へフォーカス。
//       ArrowDown/Up で aria-activedescendant が動き、Home/End、Enter で選んで閉じる。Esc は選ばず閉じてトリガーへ戻す。
//       Tab で閉じる。選んだらトリガーへフォーカスが戻る
//   P-5 検索: 打った文字を含む技名だけに絞る(ひらがな/カタカナ・大小の違いは同じ字)。0 件は「見つかりません」の 1 行(role=status)
//   P-6 disabled なら開かない。未選択(value="")のとき、先頭に disabled の「技を選んでください」行(タイプ順の外・選べない)
//   P-7 押せる行・トリガーの CSS: min-height が 24px 以上、行は white-space: nowrap(1 行)。色はトークン(var(--…))だけ
//   P-8 外側を押すと閉じる(選ばない)。常時動くアニメーションを置かない(animation-iteration-count: infinite を使わない)
// 架空のデータだけ(test/moveSortMaster.ts)。

import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { readFileSync } from "node:fs";
import { useState } from "react";
import { describe, expect, test, vi } from "vitest";
import { favoritesRestoreText } from "../i18n/favorites";
import {
  chooseMove,
  listedMoveIds,
  moveTrigger,
  openMovePicker,
  rowIds,
  selectedMoveId,
} from "../test/movePicker";
import { localPath } from "../test/localPath";
import { SORT_MOVES, TYPE_IDS } from "../test/moveSortMaster";
import { MovePicker } from "./MovePicker";

const TYPES = ["electric", "fire", "normal"];

function Harness({
  initial = "",
  onChange,
  disabled = false,
  moves = SORT_MOVES,
}: {
  initial?: string;
  onChange?: (id: string) => void;
  disabled?: boolean;
  moves?: typeof SORT_MOVES;
}) {
  const [value, setValue] = useState(initial);
  return (
    <>
      <MovePicker
        label="技"
        moves={moves}
        types={TYPES}
        value={value}
        onChange={(id) => {
          setValue(id);
          onChange?.(id);
        }}
        disabled={disabled}
        showUnselected
      />
      <button type="button">外側</button>
    </>
  );
}

describe("P-1 トリガー", () => {
  test("combobox・名前「技」・閉じている間は listbox / option が無い・data-value が選択中の技", () => {
    render(<Harness initial="examplesortspark" />);
    const trigger = moveTrigger();
    expect(trigger).toHaveAttribute("aria-haspopup", "listbox");
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(selectedMoveId()).toBe("examplesortspark");
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(screen.queryAllByRole("option")).toHaveLength(0);
    expect(trigger).toHaveTextContent("スパーク");
    expect(within(trigger).getByRole("img", { name: "ぶつり" })).toBeInTheDocument();
  });

  test("開くと aria-expanded=true・aria-controls が listbox を指す", async () => {
    const user = userEvent.setup();
    render(<Harness initial="examplesortspark" />);
    const listbox = await openMovePicker(user);
    expect(moveTrigger()).toHaveAttribute("aria-expanded", "true");
    expect(moveTrigger().getAttribute("aria-controls")).toBe(listbox.id);
  });
});

describe("P-2 行は 1 行で、名前に 技名・タイプ・分類・威力 を含む", () => {
  test("行の accessible name が 技名 → タイプ名 → 分類 → 威力 の順", async () => {
    const user = userEvent.setup();
    render(<Harness initial="examplesortspark" />);
    const listbox = await openMovePicker(user);
    const row = within(listbox).getByRole("option", { name: /^スパーク/ });
    expect(row).toHaveAccessibleName(/スパーク.*でんき.*ぶつり.*威力\s*65/);
    const special = within(listbox).getByRole("option", { name: /^かみなり/ });
    expect(special).toHaveAccessibleName(/かみなり.*でんき.*とくしゅ.*威力\s*90/);
  });

  test("行の中身: タイプのバッジ(.ui-badge)と分類アイコン(ぶつり/とくしゅ)があり、行は子の block を持たない 1 行", async () => {
    const user = userEvent.setup();
    render(<Harness initial="examplesortspark" />);
    const listbox = await openMovePicker(user);
    const row = within(listbox).getByRole("option", { name: /^スパーク/ });
    expect(row.querySelector(".ui-badge")).not.toBeNull();
    expect(row.querySelector(".ui-badge")).toHaveTextContent("でんき");
    expect(within(row).getByRole("img", { name: "ぶつり" })).toBeInTheDocument();
    expect(row).toHaveClass("move-picker__row");
    expect(row.querySelector("p, ul, ol, div")).toBeNull();
  });

  test("変化技は渡されない前提でも、status の行なら威力を出さず分類は へんか", async () => {
    const user = userEvent.setup();
    const status = {
      id: "examplestatus",
      nameJa: "テスト変化",
      type: "normal",
      category: "status",
      power: 0,
      priority: 0,
    } as const;
    render(<Harness moves={[status]} initial="examplestatus" />);
    const listbox = await openMovePicker(user);
    const row = within(listbox).getByRole("option", { name: /^テスト変化/ });
    expect(row).toHaveAccessibleName(/へんか/);
    expect(row).not.toHaveAccessibleName(/威力/);
  });
});

describe("P-3 並びはタイプ順だけ", () => {
  test("行の並びがタイプ順(タイプ表の並び→五十音順)で、並びの切り替え(radiogroup)は無い", async () => {
    const user = userEvent.setup();
    render(<Harness initial="examplesortspark" />);
    expect(await listedMoveIds(user)).toEqual(TYPE_IDS);
    expect(screen.queryByRole("radiogroup")).toBeNull();
    expect(screen.queryByText(/習得順|五十音順|タイプ順/)).toBeNull();
  });
});

describe("P-4 キーボード", () => {
  test("トリガーで ArrowDown / Enter / Space のどれでも開き、検索欄へフォーカスが移る", async () => {
    for (const key of ["{ArrowDown}", "{Enter}", " "]) {
      const user = userEvent.setup();
      const { unmount } = render(<Harness initial="examplesortspark" />);
      moveTrigger().focus();
      await user.keyboard(key);
      expect(moveTrigger()).toHaveAttribute("aria-expanded", "true");
      expect(screen.getByRole("searchbox", { name: "技を検索" })).toHaveFocus();
      unmount();
    }
  });

  test("ArrowDown / ArrowUp / Home / End で aria-activedescendant が動き、Enter で選んで閉じ、トリガーへ戻る", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness initial="examplesortspark" onChange={onChange} />);
    moveTrigger().focus();
    await user.keyboard("{Enter}");
    const search = screen.getByRole("searchbox", { name: "技を検索" });
    const listbox = screen.getByRole("listbox", { name: "技" });
    const active = () => document.getElementById(search.getAttribute("aria-activedescendant") ?? "");
    // 開いた直後は選択中の技が活性
    expect(active()).toHaveAttribute("data-move-id", "examplesortspark");
    await user.keyboard("{Home}");
    expect(active()).toHaveAttribute("data-move-id", TYPE_IDS[0]);
    await user.keyboard("{End}");
    expect(active()).toHaveAttribute("data-move-id", TYPE_IDS[TYPE_IDS.length - 1]);
    await user.keyboard("{ArrowUp}");
    expect(active()).toHaveAttribute("data-move-id", TYPE_IDS[TYPE_IDS.length - 2]);
    await user.keyboard("{ArrowDown}");
    await user.keyboard("{Enter}");
    expect(onChange).toHaveBeenCalledWith(TYPE_IDS[TYPE_IDS.length - 1]);
    expect(listbox).not.toBeInTheDocument();
    expect(moveTrigger()).toHaveFocus();
    expect(selectedMoveId()).toBe(TYPE_IDS[TYPE_IDS.length - 1]);
  });

  test("Esc は選ばず閉じてトリガーへ戻す。Tab でも閉じる", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness initial="examplesortspark" onChange={onChange} />);
    moveTrigger().focus();
    await user.keyboard("{Enter}{ArrowDown}{Escape}");
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(moveTrigger()).toHaveFocus();
    expect(onChange).not.toHaveBeenCalled();
    await user.keyboard("{Enter}{Tab}");
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(onChange).not.toHaveBeenCalled();
  });

  test("日本語入力の変換中は Enter・矢印で選ばず、閉じず、活性の行も動かさない", async () => {
    const onChange = vi.fn();
    render(<Harness initial="examplesortspark" onChange={onChange} />);
    moveTrigger().focus();
    await userEvent.setup().keyboard("{Enter}");
    const search = screen.getByRole("searchbox");
    const before = search.getAttribute("aria-activedescendant");
    fireEvent.compositionStart(search);
    fireEvent.keyDown(search, { key: "ArrowDown", isComposing: true });
    fireEvent.keyDown(search, { key: "Enter", isComposing: true });
    expect(search.getAttribute("aria-activedescendant")).toBe(before);
    expect(screen.getByRole("listbox")).toBeInTheDocument();
    expect(onChange).not.toHaveBeenCalled();
    fireEvent.compositionEnd(search);
  });

  test("マウスで行を押すと選んで閉じ、トリガーへフォーカスが戻る", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness initial="examplesortspark" onChange={onChange} />);
    await chooseMove(user, "examplesortgamma");
    expect(onChange).toHaveBeenCalledExactlyOnceWith("examplesortgamma");
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(moveTrigger()).toHaveFocus();
  });

  test("選択中の行は aria-selected=true、他は false", async () => {
    const user = userEvent.setup();
    render(<Harness initial="examplesortspark" />);
    const listbox = await openMovePicker(user);
    for (const row of within(listbox).getAllByRole("option")) {
      const isSpark = row.getAttribute("data-move-id") === "examplesortspark";
      expect(row).toHaveAttribute("aria-selected", isSpark ? "true" : "false");
    }
  });
});

describe("P-5 検索", () => {
  test("文字を打つと技名に含むものだけに絞る(ひらがな/カタカナは同じ字)", async () => {
    const user = userEvent.setup();
    render(<Harness initial="examplesortspark" />);
    await openMovePicker(user);
    await user.keyboard("すーぱー");
    expect(rowIds(screen.getByRole("listbox", { name: "技" })).filter((id) => id !== "")).toEqual([
      "examplesortsuper",
    ]);
    await user.clear(screen.getByRole("searchbox", { name: "技を検索" }));
    await user.keyboard("が");
    const ids = rowIds(screen.getByRole("listbox", { name: "技" })).filter((id) => id !== "");
    expect(ids).toEqual(["examplesortgamma", "examplesortpatience"]);
  });

  test("0 件は 1 行の状態表示(role=status)で、option は無い", async () => {
    const user = userEvent.setup();
    render(<Harness initial="examplesortspark" />);
    await openMovePicker(user);
    await user.keyboard("zzzz存在しない");
    expect(screen.queryAllByRole("option")).toHaveLength(0);
    expect(screen.getByRole("status")).toHaveTextContent("見つかりません");
  });

  test("絞り込み中も Enter は活性の行(先頭に自動で活性)を選ぶ", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness initial="examplesortspark" onChange={onChange} />);
    await openMovePicker(user);
    await user.keyboard("ガンマ{Enter}");
    expect(onChange).toHaveBeenCalledWith("examplesortgamma");
  });
});

describe("P-6 disabled・未選択", () => {
  test("disabled のときは押しても開かず、トリガーは disabled", async () => {
    const user = userEvent.setup();
    render(<Harness disabled />);
    expect(moveTrigger()).toBeDisabled();
    await user.click(moveTrigger());
    expect(screen.queryByRole("listbox")).toBeNull();
  });

  test('未選択(value="")は先頭に disabled の「技を選んでください」行があり、押しても選ばれない。タイプ順の外', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness initial="" onChange={onChange} />);
    expect(selectedMoveId()).toBe("");
    const listbox = await openMovePicker(user);
    const rows = within(listbox).getAllByRole("option");
    expect(rows[0]).toHaveAccessibleName(favoritesRestoreText.moveUnselectedOption);
    expect(rows[0]).toHaveAttribute("aria-disabled", "true");
    expect(rowIds(listbox).slice(1)).toEqual(TYPE_IDS);
    await user.click(rows[0] as HTMLElement);
    expect(onChange).not.toHaveBeenCalled();
  });

  test("選択済みなら未選択の行は出ない", async () => {
    const user = userEvent.setup();
    render(<Harness initial="examplesortspark" />);
    const listbox = await openMovePicker(user);
    expect(rowIds(listbox)).not.toContain("");
  });
});

describe("P-8 外側を押すと閉じる", () => {
  test("外側を押すと選ばずに閉じる", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness initial="examplesortspark" onChange={onChange} />);
    await openMovePicker(user);
    await user.click(screen.getByRole("button", { name: "外側" }));
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(onChange).not.toHaveBeenCalled();
  });
});

describe("P-7 CSS(見た目の決まり)", () => {
  const css = readFileSync(localPath("./MovePicker.css", import.meta.url), "utf8");

  test("行・トリガーは min-height が 24px 以上、行は 1 行(white-space: nowrap)", () => {
    const block = (selector: string): string => {
      const match = new RegExp(`${selector.replace(".", "\\.")}\\s*\\{([^}]*)\\}`).exec(css);
      if (match === null) {
        throw new Error(`${selector} の規則が無い`);
      }
      return match[1] ?? "";
    };
    for (const selector of [".move-picker__row", ".move-picker__trigger"]) {
      const minHeight = /min-height:\s*([^;]+);/.exec(block(selector))?.[1] ?? "";
      // px 直書きなら 24 以上、トークン(var(--…))でもよい
      const px = /^(\d+(?:\.\d+)?)px$/.exec(minHeight.trim());
      expect(px === null ? minHeight.trim().startsWith("var(") : Number(px[1]) >= 24).toBe(true);
    }
    expect(block(".move-picker__row")).toMatch(/white-space:\s*nowrap/);
  });

  test("色の直書きがなく(トークンだけ)、無限に繰り返すアニメーションを持たない", () => {
    expect(css).not.toMatch(/#[0-9a-fA-F]{3,8}\b|rgba?\(|hsla?\(/);
    expect(css).not.toMatch(/animation-iteration-count:\s*infinite|\binfinite\b/);
  });
});
