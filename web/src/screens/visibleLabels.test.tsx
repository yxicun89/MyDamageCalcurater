// issue #304: 計算・逆算・タイプバランスの入力に「見えるラベル」があること。
// 正は docs/design.md「入力のラベル」、WCAG 2.2 SC 3.3.2(ラベル又は説明)・SC 2.5.3(見出しどおりの名前)。
//
// 確かめること:
//   - 領域(カード・枠)に、画面の文字として読める見出しがある(計算「攻撃側」「防御側」/
//     逆算「自分」「相手」/ タイプバランス「メンバーn」「仮想敵n」)
//   - 見出しの階層は h2(領域)→ h3(領域の中の名前 = 選んだポケモンの名前)
//   - どの select・数値入力にも、label で結び付いた見えるラベルがある(aria-label だけの欄が無い)
//   - 見えるラベルの文字は、その欄の accessible name に必ず含まれる(SC 2.5.3)
//   - 未選択の select の表示が空にならない(先頭に文言つきの option がある。一覧には出さない hidden)
//   - 逆算の観測欄に、単位(%/HP)と観測した側(与えた/受けた)に応じた意味の説明が見える文字である
//
// 回帰: accessible name(既存テストが getByRole の name で引いている文字列)は変えない。
// このファイルは name を直接書いて、既存テストと同じ引き方で引けることも同時に確かめる。

import { render, screen, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { beforeAll, describe, expect, test } from "vitest";
import type { BalanceClient, BalanceResult } from "../api/balanceClient";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import { accessibleNameOf, describedByTextsOf, visibleLabelOf } from "../test/accessibleName";
import { createFakeEngine } from "../test/fakeEngine";
import { BalanceScreen } from "./BalanceScreen";
import { CalcScreen } from "./CalcScreen";
import { ReverseScreen } from "./ReverseScreen";

let master: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
});

function speciesAt(index: number): MasterSpecies {
  const species = master.species[index];
  if (species === undefined) {
    throw new Error(`例データに ${index} 番目の種族が無い`);
  }
  return species;
}

/** 呼び出しを受けるだけで応答を返さない BalanceClient(この画面の入力の見た目だけを見るため)。 */
function createSilentBalanceClient(): BalanceClient {
  const pending = <T,>(): Promise<BalanceResult<T>> => new Promise<BalanceResult<T>>(() => undefined);
  return {
    analyze: pending,
    coverage: pending,
    threats: pending,
    recommendations: pending,
  };
}

function renderCalc(): { user: UserEvent } {
  const user = userEvent.setup();
  render(<CalcScreen engine={createFakeEngine()} master={master} />);
  return { user };
}

function renderReverse(): { user: UserEvent } {
  const user = userEvent.setup();
  render(<ReverseScreen engine={createFakeEngine()} master={master} />);
  return { user };
}

function renderBalance(): { user: UserEvent } {
  const user = userEvent.setup();
  render(<BalanceScreen master={master} client={createSilentBalanceClient()} />);
  return { user };
}

/**
 * SC 2.5.3: 範囲の中のすべての select・数値入力について、
 * (1) 結び付いた見えるラベルがあること (2) その文字が accessible name に含まれること。
 */
function expectVisibleLabelsInNames(container: HTMLElement): void {
  const controls = [
    ...within(container).queryAllByRole("combobox"),
    ...within(container).queryAllByRole("textbox"),
  ];
  expect(controls.length).toBeGreaterThan(0);
  for (const control of controls) {
    const name = accessibleNameOf(control);
    const visible = visibleLabelOf(control);
    expect(visible, `accessible name「${name}」の欄に、結び付いた見えるラベルが無い`).not.toBeNull();
    expect(visible).not.toBe("");
    expect(name, `見えるラベル「${visible ?? ""}」が accessible name「${name}」に含まれていない`).toContain(
      visible ?? "",
    );
  }
}

/** 未選択(value が "")の select の表示が空でないこと = 選ばれている option に文字があること。 */
function expectUnselectedOptionHasText(select: HTMLElement): void {
  expect(select).toHaveValue("");
  const options = Array.from(select.querySelectorAll("option"));
  const selected = options.find((option) => option.selected) ?? options[0];
  expect(selected, "option が1つも無い select がある").toBeDefined();
  expect((selected?.textContent ?? "").trim()).not.toBe("");
}

describe("計算画面(issue #304)", () => {
  const attackerCard = () => screen.getByRole("region", { name: "攻撃側" });
  const defenderCard = () => screen.getByRole("region", { name: "防御側" });

  test("攻撃側・防御側のカードに、見える文字の見出し(h2)がある", () => {
    renderCalc();
    expect(within(attackerCard()).getByRole("heading", { name: "攻撃側", level: 2 })).toBeVisible();
    expect(within(defenderCard()).getByRole("heading", { name: "防御側", level: 2 })).toBeVisible();
    // 見出しは領域の accessible name と同じ文字から作る(二重管理をしない。design.md「入力のラベル」)。
    expect(accessibleNameOf(attackerCard())).toContain("攻撃側");
    expect(accessibleNameOf(defenderCard())).toContain("防御側");
  });

  test("ポケモンを選んでも、領域の見出し(h2)と種族名(h3)が別の階層で両方出る", async () => {
    const { user } = renderCalc();
    const attacker = speciesAt(0);
    await user.selectOptions(screen.getByRole("combobox", { name: "攻撃側のポケモン" }), attacker.key);
    expect(within(attackerCard()).getByRole("heading", { name: "攻撃側", level: 2 })).toBeVisible();
    expect(within(attackerCard()).getByRole("heading", { name: attacker.nameJa, level: 3 })).toBeVisible();
  });

  test("ポケモン・持ち物・技のすべての欄に、見えるラベルがあり accessible name に含まれる", () => {
    renderCalc();
    expectVisibleLabelsInNames(document.body);
    // 見えるラベルは短い語(側は見出しが担う)。accessible name は今までどおり「側+項目」。
    expect(visibleLabelOf(screen.getByRole("combobox", { name: "攻撃側のポケモン" }))).toBe("ポケモン");
    expect(visibleLabelOf(screen.getByRole("combobox", { name: "防御側のポケモン" }))).toBe("ポケモン");
    expect(visibleLabelOf(screen.getByRole("combobox", { name: "攻撃側の持ち物" }))).toBe("持ち物");
    expect(visibleLabelOf(screen.getByRole("combobox", { name: "防御側の持ち物" }))).toBe("持ち物");
    expect(visibleLabelOf(screen.getByRole("combobox", { name: "技" }))).toBe("技");
  });

  test("ポケモンの select は、未選択のとき何を選ぶか分かる文言を表示する", () => {
    renderCalc();
    for (const name of ["攻撃側のポケモン", "防御側のポケモン"]) {
      const select = screen.getByRole("combobox", { name });
      expectUnselectedOptionHasText(select);
      expect(within(select).queryByRole("option", { name: "ポケモンを選ぶ" })).toBeNull();
      expect((select.querySelector("option[value='']")?.textContent ?? "").trim()).toBe("ポケモンを選ぶ");
    }
  });
});

describe("逆算画面(issue #304)", () => {
  const myCard = () => screen.getByRole("region", { name: "自分のポケモン" });
  const theirCard = () => screen.getByRole("region", { name: "相手のポケモン" });

  test("自分側・相手側のカードに、見える文字の見出し(h2)がある", () => {
    renderReverse();
    expect(within(myCard()).getByRole("heading", { name: "自分", level: 2 })).toBeVisible();
    expect(within(theirCard()).getByRole("heading", { name: "相手", level: 2 })).toBeVisible();
    // 領域の accessible name は変えない(既存テストの引き方のまま)。見出しの語はその一部から作る。
    expect(accessibleNameOf(myCard())).toBe("自分のポケモン");
    expect(accessibleNameOf(theirCard())).toBe("相手のポケモン");
  });

  test("ポケモン・持ち物・技・観測のすべての欄に、見えるラベルがあり accessible name に含まれる", () => {
    renderReverse();
    expectVisibleLabelsInNames(document.body);
    expect(visibleLabelOf(screen.getByRole("combobox", { name: "自分のポケモン" }))).toBe("ポケモン");
    expect(visibleLabelOf(screen.getByRole("combobox", { name: "相手のポケモン" }))).toBe("ポケモン");
    expect(visibleLabelOf(screen.getByRole("combobox", { name: "自分の持ち物" }))).toBe("持ち物");
    expect(visibleLabelOf(screen.getByRole("combobox", { name: "技" }))).toBe("技");
    expect(visibleLabelOf(screen.getByRole("textbox", { name: "観測1" }))).toBe("観測1");
  });

  test("ポケモンの select は、未選択のとき何を選ぶか分かる文言を表示する", () => {
    renderReverse();
    for (const name of ["自分のポケモン", "相手のポケモン"]) {
      const select = screen.getByRole("combobox", { name });
      expectUnselectedOptionHasText(select);
      expect((select.querySelector("option[value='']")?.textContent ?? "").trim()).toBe("ポケモンを選ぶ");
    }
  });

  test("観測欄に、何を入れる数値なのかを説明する見える文字がある(単位に応じて変わる)", async () => {
    const { user } = renderReverse();
    const input = () => screen.getByRole("textbox", { name: "観測1" });

    // 既定(与えたダメージ・%): 相手の HP が減った割合。
    expect(screen.getByText("相手の HP が減った割合(%)")).toBeVisible();
    expect(describedByTextsOf(input())).toContain("相手の HP が減った割合(%)");

    // 単位を HP にすると、実数値であることが分かる文に変わる。
    await user.click(
      within(screen.getByRole("radiogroup", { name: "観測1の単位" })).getByRole("radio", { name: "HP" }),
    );
    expect(screen.getByText("相手の HP が減った実数値(HP)")).toBeVisible();
    expect(describedByTextsOf(input())).toContain("相手の HP が減った実数値(HP)");

    // 観測した側を「受けたダメージ」にすると、減るのは自分の HP になる。
    await user.click(
      within(screen.getByRole("radiogroup", { name: "観測したダメージ" })).getByRole("radio", {
        name: "受けたダメージ",
      }),
    );
    expect(describedByTextsOf(input()).join(" ")).toContain("自分の HP が減った");
  });
});

describe("タイプバランス画面(issue #304)", () => {
  const memberGroup = (n: number) => screen.getByRole("group", { name: `メンバー${String(n)}` });

  test("メンバーの枠に、見える文字の見出し(legend)がある", () => {
    renderBalance();
    expect(within(memberGroup(1)).getByText("メンバー1")).toBeVisible();
    expect(accessibleNameOf(memberGroup(1))).toBe("メンバー1");
  });

  test("ポケモン・特性・技1〜技4のすべての欄に、見えるラベルがあり accessible name に含まれる", () => {
    renderBalance();
    expectVisibleLabelsInNames(memberGroup(1));
    expect(visibleLabelOf(within(memberGroup(1)).getByRole("combobox", { name: "ポケモン" }))).toBe(
      "ポケモン",
    );
    expect(visibleLabelOf(within(memberGroup(1)).getByRole("combobox", { name: "特性" }))).toBe("特性");
    for (const slot of [1, 2, 3, 4]) {
      const label = `技${String(slot)}`;
      expect(visibleLabelOf(within(memberGroup(1)).getByRole("combobox", { name: label }))).toBe(label);
    }
  });

  test("ポケモン・特性の select は、未選択のとき空欄にならない", () => {
    renderBalance();
    const species = within(memberGroup(1)).getByRole("combobox", { name: "ポケモン" });
    expectUnselectedOptionHasText(species);
    expect((species.querySelector("option[value='']")?.textContent ?? "").trim()).toBe("ポケモンを選ぶ");

    // ポケモンを選ぶまで特性は1件も無い(= 選択肢ゼロの空の枠)。何を待てばよいか分かる文言を出す。
    const ability = within(memberGroup(1)).getByRole("combobox", { name: "特性" });
    expectUnselectedOptionHasText(ability);
  });

  test("ポケモンを選ぶと特性は既定の1つ目が入り、未選択の文言は一覧に出さない", async () => {
    const { user } = renderBalance();
    const species = speciesAt(0);
    await user.selectOptions(within(memberGroup(1)).getByRole("combobox", { name: "ポケモン" }), species.key);
    const ability = within(memberGroup(1)).getByRole("combobox", { name: "特性" });
    expect(ability).toHaveValue(species.abilities[0] ?? "");
    // 未選択の option は hidden にして一覧(role=option)には出さない(既存テストの選択肢の検証を壊さない)。
    expect(
      within(ability)
        .getAllByRole("option")
        .map((option) => option.getAttribute("value")),
    ).toEqual([...species.abilities]);
  });
});
