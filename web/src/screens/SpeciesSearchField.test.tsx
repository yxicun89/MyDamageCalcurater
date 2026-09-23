// P4-16c: 種族の検索欄(SpeciesSearchField)のキーボード操作と ARIA(ADR-0304 A-10 の具体化)。
// P4-16b では候補が onClick でしか選べず、WAI-ARIA Authoring Practices の Combobox パターン
// (List Autocomplete with Automatic Selection)のキーボード操作が無かった。ここで受け入れ条件を固定する。
//
// 決めた仕様(受け入れ条件 AC-1〜AC-5。他に合わせるべき既存 UI が無いので、ここで決めてテストで固定する):
//   - 候補が出た直後・候補が入れ替わった直後は、必ず**先頭**の候補がハイライトされる(自動選択)
//   - ArrowDown / ArrowUp は1件ずつ動き、**端で止まる**(ループしない)
//   - Enter はハイライト中の候補を確定する。候補が出ていなければ何もしない
//   - Escape は候補を閉じる。**入力の文字はそのまま残す**(消さない)。閉じたら候補は捨て、
//     ArrowDown では戻らない(入力を変えれば再検索される。古い候補と入力文字がずれた状態を作らないため)
//   - ハイライトは aria-activedescendant(入力欄)・aria-selected(候補)・クラス(見た目)の3つで示す
//   - マウスのクリックは今までどおり効く(キーボードと排他ではない)
//
// 検索そのもの(デバウンス・取り消し・失敗の表示)は CalcScreen.online.test.tsx / ReverseScreen.online.test.tsx
// が画面ごしに確かめているので、ここでは重複させず、キーボードと ARIA だけを見る。

import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { afterEach, describe, expect, test, vi } from "vitest";
import type { Ability } from "../engine/types";
import { masterOnlineText } from "../i18n/ja";
import { SPECIES_SEARCH_DEBOUNCE_MS } from "../master/onlineSource";
import type { MasterSpecies, MasterSpeciesResolution, MasterSpeciesSearch } from "../master/types";
import { createFakeSpeciesSearch, type FakeSpeciesSearch } from "../test/onlineMaster";
import { speciesSearchClass } from "../test/speciesSearchClasses";
import { SpeciesSearchField } from "./SpeciesSearchField";

/** 既存のスロットのラベル(accessible name は検索欄でも変えない。ADR-0304 A-10)。 */
const LABEL = "攻撃側のポケモン";
/** 候補が出る検索語(架空の種族の共通の接頭辞)。 */
const QUERY = "テスト";

const ABILITY: Ability = { id: "example-ability-blaze", nameJa: "テストもうか", effect: null };

/** 架空の種族(index 番目)。名前は QUERY で前方一致する。 */
function fixtureSpecies(index: number): MasterSpecies {
  return {
    key: `900${String(index)}-000`,
    dexNo: 9000 + index,
    form: 0,
    nameJa: `${QUERY}ポケモン${String(index)}`,
    types: ["fire"],
    baseStats: { hp: 100, atk: 100, def: 100, spa: 100, spd: 100, spe: 100 },
    abilities: [ABILITY.id],
    learnset: [],
  };
}

function fixtureSpeciesList(count: number): MasterSpecies[] {
  return Array.from({ length: count }, (_unused, index) => fixtureSpecies(index));
}

function searchOf(count: number): FakeSpeciesSearch {
  return createFakeSpeciesSearch({ species: fixtureSpeciesList(count), abilities: [ABILITY] });
}

interface RenderResult {
  readonly user: UserEvent;
  readonly onResolved: OnResolvedMock;
  /** 検索語を入れ、デバウンスを越えて候補を出す。 */
  open(query?: string): Promise<void>;
  /** fake タイマーを進める(検索のデバウンス)。 */
  advance(ms: number): void;
}

interface RenderWithSearchResult extends RenderResult {
  readonly search: FakeSpeciesSearch;
}

type OnResolvedMock = ReturnType<typeof createOnResolved>;

function createOnResolved() {
  return vi.fn<(resolution: MasterSpeciesResolution) => void>();
}

function renderField(search: MasterSpeciesSearch | undefined): RenderResult {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
  const onResolved = createOnResolved();
  render(<SpeciesSearchField label={LABEL} masterSearch={search} onResolved={onResolved} />);
  const advance = (ms: number): void => {
    act(() => {
      vi.advanceTimersByTime(ms);
    });
  };
  return {
    user,
    onResolved,
    advance,
    async open(query = QUERY) {
      await user.type(comboboxInput(), query);
      advance(SPECIES_SEARCH_DEBOUNCE_MS);
      await screen.findAllByRole("option");
    },
  };
}

/** fake の検索口付きで描画する(ほとんどのテストはこちら)。 */
function renderWithCandidates(count: number): RenderWithSearchResult {
  const search = searchOf(count);
  return { ...renderField(search), search };
}

afterEach(() => {
  vi.useRealTimers();
});

function comboboxInput(): HTMLElement {
  return screen.getByRole("combobox", { name: LABEL });
}

function optionElements(): HTMLElement[] {
  return screen.getAllByRole("option");
}

/** aria-activedescendant が指す候補(見つからなければ失敗させる)。 */
function highlightedOption(): HTMLElement {
  const id = comboboxInput().getAttribute("aria-activedescendant");
  if (id === null || id === "") {
    throw new Error("入力欄に aria-activedescendant が無い(ハイライト中の候補が分からない)");
  }
  const element = document.getElementById(id);
  if (element === null) {
    throw new Error(`aria-activedescendant の参照先(${id})が DOM に無い`);
  }
  return element;
}

/** 候補ごとの aria-selected(順番どおり)。 */
function selectedFlags(): (string | null)[] {
  return optionElements().map((option) => option.getAttribute("aria-selected"));
}

describe("AC-1 候補が出たら先頭がハイライトされる(自動選択)", () => {
  test("aria-activedescendant が先頭の候補を指し、その候補だけ aria-selected が true", async () => {
    const view = renderWithCandidates(3);
    await view.open();

    expect(optionElements()).toHaveLength(3);
    expect(highlightedOption()).toHaveTextContent(fixtureSpecies(0).nameJa);
    expect(selectedFlags()).toEqual(["true", "false", "false"]);
  });

  test("ハイライト中の候補だけが見た目のハイライトのクラスを持つ", async () => {
    const view = renderWithCandidates(3);
    await view.open();

    const [first, second, third] = optionElements();
    expect(first).toHaveClass(speciesSearchClass.activeOption);
    expect(second).not.toHaveClass(speciesSearchClass.activeOption);
    expect(third).not.toHaveClass(speciesSearchClass.activeOption);
  });

  test("候補が入れ替わったらハイライトは先頭に戻る", async () => {
    const view = renderWithCandidates(3);
    await view.open();
    await view.user.keyboard("{ArrowDown}");
    expect(highlightedOption()).toHaveTextContent(fixtureSpecies(1).nameJa);

    // 文字を足して検索し直す(母集団は同じなので候補は同じ3件に戻る)。
    await view.user.type(comboboxInput(), "ポ");
    view.advance(SPECIES_SEARCH_DEBOUNCE_MS);
    await screen.findAllByRole("option");

    expect(highlightedOption()).toHaveTextContent(fixtureSpecies(0).nameJa);
    expect(selectedFlags()).toEqual(["true", "false", "false"]);
  });
});

describe("AC-2 ArrowDown / ArrowUp は1件ずつ動き、端で止まる(ループしない)", () => {
  test("ArrowDown で次の候補へ移る", async () => {
    const view = renderWithCandidates(3);
    await view.open();

    await view.user.keyboard("{ArrowDown}");
    expect(highlightedOption()).toHaveTextContent(fixtureSpecies(1).nameJa);
    expect(selectedFlags()).toEqual(["false", "true", "false"]);

    await view.user.keyboard("{ArrowDown}");
    expect(highlightedOption()).toHaveTextContent(fixtureSpecies(2).nameJa);
  });

  test("末尾で ArrowDown を押しても末尾のまま(先頭に戻らない)", async () => {
    const view = renderWithCandidates(3);
    await view.open();

    await view.user.keyboard("{ArrowDown}{ArrowDown}{ArrowDown}{ArrowDown}");

    expect(highlightedOption()).toHaveTextContent(fixtureSpecies(2).nameJa);
    expect(selectedFlags()).toEqual(["false", "false", "true"]);
  });

  test("ArrowUp で前の候補へ戻る", async () => {
    const view = renderWithCandidates(3);
    await view.open();

    await view.user.keyboard("{ArrowDown}{ArrowDown}{ArrowUp}");

    expect(highlightedOption()).toHaveTextContent(fixtureSpecies(1).nameJa);
  });

  test("先頭で ArrowUp を押しても先頭のまま(末尾に回らない)", async () => {
    const view = renderWithCandidates(3);
    await view.open();

    await view.user.keyboard("{ArrowUp}{ArrowUp}");

    expect(highlightedOption()).toHaveTextContent(fixtureSpecies(0).nameJa);
    expect(selectedFlags()).toEqual(["true", "false", "false"]);
  });

  test("候補が1件のときは ArrowDown / ArrowUp でハイライトが動かない", async () => {
    const view = renderWithCandidates(1);
    await view.open();

    await view.user.keyboard("{ArrowDown}{ArrowUp}{ArrowDown}");

    expect(highlightedOption()).toHaveTextContent(fixtureSpecies(0).nameJa);
    expect(selectedFlags()).toEqual(["true"]);
  });
});

describe("AC-3 Enter はハイライト中の候補を確定する", () => {
  test("そのまま Enter を押すと先頭の候補を解決し、名前を入力欄に入れて候補を閉じる", async () => {
    const view = renderWithCandidates(3);
    await view.open();

    await view.user.keyboard("{Enter}");

    const chosen = fixtureSpecies(0);
    expect(view.search.resolvedKeys).toEqual([chosen.key]);
    await waitFor(() => {
      expect(view.onResolved).toHaveBeenCalledTimes(1);
    });
    expect(view.onResolved.mock.calls[0]?.[0]).toMatchObject({ species: { key: chosen.key } });
    expect(comboboxInput()).toHaveValue(chosen.nameJa);
    expect(screen.queryAllByRole("option")).toHaveLength(0);
    expect(comboboxInput()).toHaveAttribute("aria-expanded", "false");
  });

  test("ArrowDown で移してから Enter を押すと、その候補を解決する", async () => {
    const view = renderWithCandidates(3);
    await view.open();

    await view.user.keyboard("{ArrowDown}{Enter}");

    expect(view.search.resolvedKeys).toEqual([fixtureSpecies(1).key]);
    expect(comboboxInput()).toHaveValue(fixtureSpecies(1).nameJa);
  });

  test("候補が1件も無いとき(0件の検索結果)に Enter を押しても何も起きない", async () => {
    const view = renderWithCandidates(3);
    await view.user.type(comboboxInput(), "一致しない名前");
    view.advance(SPECIES_SEARCH_DEBOUNCE_MS);
    expect(await screen.findByText(masterOnlineText.speciesSearchNoResult)).toBeInTheDocument();

    await view.user.keyboard("{Enter}");

    expect(view.search.resolvedKeys).toEqual([]);
    expect(view.onResolved).not.toHaveBeenCalled();
    expect(comboboxInput()).toHaveValue("一致しない名前");
  });

  test("入力前(候補を出していない)に Enter を押しても何も起きない", async () => {
    const view = renderWithCandidates(3);

    await view.user.click(comboboxInput());
    await view.user.keyboard("{Enter}");

    expect(view.search.resolvedKeys).toEqual([]);
    expect(view.onResolved).not.toHaveBeenCalled();
  });

  test("IME変換中のEnterは候補選択に使わない(変換確定と候補選択を混同しない。critic指摘)", async () => {
    const view = renderWithCandidates(3);
    await view.open();

    fireEvent.keyDown(comboboxInput(), { key: "Enter", isComposing: true });

    expect(view.search.resolvedKeys).toEqual([]);
    expect(view.onResolved).not.toHaveBeenCalled();
    expect(screen.getAllByRole("option")).toHaveLength(3);
  });

  test("IME変換中のArrowDownはハイライトを動かさない", async () => {
    const view = renderWithCandidates(3);
    await view.open();

    fireEvent.keyDown(comboboxInput(), { key: "ArrowDown", isComposing: true });

    expect(highlightedOption()).toHaveTextContent(fixtureSpecies(0).nameJa);
  });
});

describe("AC-4 Escape は候補を閉じ、入力の文字は残す", () => {
  test("候補が消え、aria-expanded が false になり、入力の文字はそのまま", async () => {
    const view = renderWithCandidates(3);
    await view.open();

    await view.user.keyboard("{Escape}");

    expect(screen.queryAllByRole("option")).toHaveLength(0);
    expect(comboboxInput()).toHaveAttribute("aria-expanded", "false");
    expect(comboboxInput()).toHaveValue(QUERY);
    expect(view.search.resolvedKeys).toEqual([]);
  });

  test("閉じただけなので「名前を入力してください」は出さない(入力は空ではない)", async () => {
    const view = renderWithCandidates(3);
    await view.open();

    await view.user.keyboard("{Escape}");

    expect(screen.queryByText(masterOnlineText.speciesSearchEmpty)).toBeNull();
    expect(screen.queryByText(masterOnlineText.speciesSearchNoResult)).toBeNull();
    expect(screen.queryByText(masterOnlineText.speciesSearchFailed)).toBeNull();
  });

  test("閉じたあと ArrowDown を押しても候補は戻らない(閉じたら候補は捨てる)", async () => {
    const view = renderWithCandidates(3);
    await view.open();
    await view.user.keyboard("{Escape}");

    await view.user.keyboard("{ArrowDown}");

    expect(screen.queryAllByRole("option")).toHaveLength(0);
    expect(comboboxInput()).not.toHaveAttribute("aria-activedescendant");
  });

  test("閉じたあと文字を足すと、また検索して候補を出す", async () => {
    const view = renderWithCandidates(3);
    await view.open();
    await view.user.keyboard("{Escape}");

    await view.user.type(comboboxInput(), "ポ");
    view.advance(SPECIES_SEARCH_DEBOUNCE_MS);

    expect(await screen.findAllByRole("option")).toHaveLength(3);
    expect(highlightedOption()).toHaveTextContent(fixtureSpecies(0).nameJa);
  });
});

describe("AC-5 ARIA の参照は宙に浮かせない(critic 指摘(5))", () => {
  test("候補が出ていないときは aria-controls も aria-activedescendant も付けない", () => {
    renderWithCandidates(3);

    // 入力前。参照先の listbox は DOM に無いので、IDREF を残してはいけない。
    expect(comboboxInput()).not.toHaveAttribute("aria-controls");
    expect(comboboxInput()).not.toHaveAttribute("aria-activedescendant");
    expect(comboboxInput()).toHaveAttribute("aria-expanded", "false");
  });

  test("候補が出ているときの aria-controls は、その listbox の id を指す", async () => {
    const view = renderWithCandidates(3);
    await view.open();

    const listbox = screen.getByRole("listbox", { name: LABEL });
    expect(listbox.id).not.toBe("");
    expect(comboboxInput()).toHaveAttribute("aria-controls", listbox.id);
    expect(comboboxInput()).toHaveAttribute("aria-expanded", "true");
  });

  test("候補ごとの id は一意で、aria-activedescendant はそのうち1つだけを指す", async () => {
    const view = renderWithCandidates(3);
    await view.open();

    const ids = optionElements().map((option) => option.id);
    expect(ids.filter((id) => id !== "")).toHaveLength(ids.length);
    expect(new Set(ids).size).toBe(ids.length);
    expect(ids).toContain(comboboxInput().getAttribute("aria-activedescendant"));
    expect(selectedFlags().filter((flag) => flag === "true")).toHaveLength(1);
  });

  test("検索口が無い(disabled)ときも IDREF を残さない", () => {
    renderField(undefined);

    expect(comboboxInput()).toBeDisabled();
    expect(comboboxInput()).not.toHaveAttribute("aria-controls");
    expect(comboboxInput()).not.toHaveAttribute("aria-activedescendant");
  });

  test("補足(hint)は今までどおり aria-describedby で結ぶ(既存の挙動の回帰ガード)", () => {
    renderWithCandidates(3);

    const describedBy = comboboxInput().getAttribute("aria-describedby");
    expect(describedBy).not.toBeNull();
    expect(document.getElementById(describedBy ?? "")).toHaveTextContent(masterOnlineText.speciesSearchHint);
  });
});

describe("AC-6 マウスの操作は今までどおり(キーボードと排他ではない)", () => {
  test("候補をクリックすると、その候補を解決する", async () => {
    const view = renderWithCandidates(3);
    await view.open();

    const [, second] = optionElements();
    if (second === undefined) {
      throw new Error("候補が2件目まで出ていない");
    }
    await view.user.click(second);

    expect(view.search.resolvedKeys).toEqual([fixtureSpecies(1).key]);
    expect(comboboxInput()).toHaveValue(fixtureSpecies(1).nameJa);
  });

  test("ArrowDown でハイライトを動かしたあとでも、別の候補をクリックできる", async () => {
    const view = renderWithCandidates(3);
    await view.open();
    await view.user.keyboard("{ArrowDown}{ArrowDown}");

    const [first] = optionElements();
    if (first === undefined) {
      throw new Error("候補が出ていない");
    }
    await view.user.click(first);

    expect(view.search.resolvedKeys).toEqual([fixtureSpecies(0).key]);
  });
});
