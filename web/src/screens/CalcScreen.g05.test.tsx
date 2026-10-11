// I-web-13b(G-05、ADR-0339): 計算画面の見た目の作り直し(docs/design.md「見た目の作り直し方針(G-05)」→「画面ごとの方向」)。
// 確かめること:
//   - 攻撃側・防御側はポケモンカード(画像かエンブレム + 名前 + タイプバッジ + 持ち物の印)。名前は h3 のまま
//   - 結果は確定数のバッジとダメージ幅の数字が主で、補足(調整・持ち物・特性)は小さい
//   - 性格補正は区切りボタン(radiogroup)、SP は増減ボタン付き、ランクは増減ボタン、条件はチップ
//   - 説明の文を出し続けない(待機の状態に <p> が無い。長い説明は「説明」ボタンの奥)
// 読み上げ用の名前・構造は既存のテスト(CalcScreen.*.test.tsx)が守る。ここは新しい部品のクラス・役割だけを見る。

import { render, screen, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { readFileSync } from "node:fs";
import { beforeAll, describe, expect, test } from "vitest";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import { createFakeEngine } from "../test/fakeEngine";
import { localPath } from "../test/localPath";
import { MEGA_FIRE, MEGA_FIRE_STONE_LABEL, withMegaFixture } from "../test/megaMaster";
import { CalcScreen } from "./CalcScreen";

let master: MasterData;
let megaMaster: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
  megaMaster = withMegaFixture(master);
});

const attackerCard = () => screen.getByRole("region", { name: "攻撃側" });
const defenderCard = () => screen.getByRole("region", { name: "防御側" });

function species(index: number): MasterSpecies {
  const found = master.species[index];
  if (found === undefined) {
    throw new Error(`例データに ${String(index)} 番目の種族が無い`);
  }
  return found;
}

function renderScreen(data: MasterData = master): { user: UserEvent; container: HTMLElement } {
  const user = userEvent.setup();
  const { container } = render(<CalcScreen engine={createFakeEngine()} master={data} />);
  return { user, container };
}

async function choosePair(user: UserEvent): Promise<void> {
  await user.selectOptions(screen.getByRole("combobox", { name: "攻撃側のポケモン" }), species(0).key);
  await user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), species(1).key);
}

describe("ポケモンカード", () => {
  test("種族を選ぶと、カードに画像かエンブレム・名前(h3)・タイプバッジが出る", async () => {
    const { user } = renderScreen();
    await choosePair(user);
    for (const [card, found] of [
      [attackerCard(), species(0)],
      [defenderCard(), species(1)],
    ] as const) {
      const pokemonCard = card.querySelector(".ui-pokemon-card");
      expect(pokemonCard).not.toBeNull();
      expect(within(card).getByTestId("pokemon-icon-emblem")).toBeInTheDocument();
      expect(within(card).getByRole("heading", { level: 3, name: found.nameJa })).toBeInTheDocument();
      expect(pokemonCard?.querySelectorAll(".ui-badge")).toHaveLength(found.types.length);
    }
  });

  test("種族が未選択の間はカードを出さない(選ぶ欄だけ)", () => {
    renderScreen();
    expect(attackerCard().querySelector(".ui-pokemon-card")).toBeNull();
    expect(defenderCard().querySelector(".ui-pokemon-card")).toBeNull();
  });

  test("持ち物を選ぶと、カードに持ち物の印が出る。「なし」なら出さない", async () => {
    const { user } = renderScreen();
    await choosePair(user);
    expect(attackerCard().querySelector(".ui-pokemon-card__item")).toBeNull();
    const select = screen.getByRole("combobox", { name: "攻撃側の持ち物" });
    const option = within(select)
      .getAllByRole("option")
      .find((candidate) => candidate.getAttribute("value") !== "");
    if (option === undefined) {
      throw new Error("持ち物の選択肢が無い");
    }
    const item = { id: option.getAttribute("value") ?? "", nameJa: option.textContent };
    await user.selectOptions(select, item.id);
    expect(attackerCard().querySelector(".ui-pokemon-card__item")).toHaveTextContent(item.nameJa);
  });

  test("メガ種族ではメガストーンが固定の持ち物としてカードの印に出る", async () => {
    const { user } = renderScreen(megaMaster);
    await user.selectOptions(screen.getByRole("combobox", { name: "攻撃側のポケモン" }), MEGA_FIRE.key);
    expect(attackerCard().querySelector(".ui-pokemon-card__item")).toHaveTextContent(MEGA_FIRE_STONE_LABEL);
  });
});

describe("結果: 確定数とダメージ幅が主", () => {
  test("行ごとに、確定数のバッジ(ui-badge)とダメージ幅の数字が出て、効果は ui-badge", async () => {
    const { user } = renderScreen();
    await choosePair(user);
    const list = await screen.findByRole("list", { name: "計算結果" });
    for (const row of within(list).getAllByRole("listitem")) {
      expect(row.querySelector(".calc-results__ko")).toHaveClass("ui-badge");
      expect(row.querySelector(".calc-results__percent")?.textContent).toMatch(/%/);
    }
    expect(document.querySelector(".calc-results__effectiveness .ui-badge")).not.toBeNull();
  });

  test("CSS: 確定数のバッジは brand.primary の塗り + on.primary の文字、バッジは見出しの大きさ・数字は結果の大きさ、補足は caption", () => {
    const css = readFileSync(localPath("./CalcScreen.css", import.meta.url), "utf8");
    const ruleOf = (selector: string): string => {
      const start = css.indexOf(`${selector} {`);
      expect(start, `${selector} の規則が無い`).toBeGreaterThanOrEqual(0);
      return css.slice(start, css.indexOf("}", start));
    };
    const ko = ruleOf(".calc-results__ko");
    expect(ko).toContain("var(--brand-primary)");
    expect(ko).toContain("var(--on-primary)");
    expect(ko).toContain("var(--font-size-heading)");
    expect(ruleOf(".calc-results__percent")).toContain("var(--font-size-result)");
    expect(ruleOf(".calc-results__preset")).toContain("var(--font-size-caption)");
  });
});

describe("区切りボタン・増減ボタン・チップ", () => {
  test("性格補正は区切りボタン(radiogroup + radio)。矢印キーで選択が動く", async () => {
    const { user } = renderScreen();
    await choosePair(user);
    const group = screen.getAllByRole("radiogroup", { name: "攻撃の性格補正" })[0];
    expect(group).toHaveClass("ui-segmented");
    const neutral = within(group as HTMLElement).getByRole("radio", { name: "補正なし" });
    expect(neutral).toBeChecked();
    neutral.focus();
    await user.keyboard("{ArrowLeft}");
    expect(within(group as HTMLElement).getByRole("radio", { name: "上昇" })).toBeChecked();
  });

  test("SP は増減ボタンで 1 ずつ変わり、0 と 32 で止まる。数値欄(文字)は残る", async () => {
    const { user } = renderScreen();
    await choosePair(user);
    const box = screen.getByRole("textbox", { name: "攻撃のSP" });
    expect(box).toHaveValue("0");
    const down = screen.getByRole("button", { name: "攻撃のSPを減らす" });
    const up = screen.getByRole("button", { name: "攻撃のSPを増やす" });
    expect(up).toHaveClass("ui-stepper__button");
    await user.click(down);
    expect(box).toHaveValue("0");
    await user.click(up);
    await user.click(up);
    expect(box).toHaveValue("2");
    await user.clear(box);
    await user.click(box);
    await user.paste("32");
    await user.click(up);
    expect(box).toHaveValue("32");
    await user.click(down);
    expect(box).toHaveValue("31");
  });

  test("詳細: 天候・フィールド・壁・急所・やけどはチップ、ランクは増減ボタン", async () => {
    const { user } = renderScreen();
    await choosePair(user);
    await user.click(screen.getByRole("button", { name: "詳細" }));
    for (const radio of screen.getAllByRole("radio", { name: "はれ" })) {
      expect(radio.closest("label")).toHaveClass("ui-chip");
    }
    expect(screen.getByRole("checkbox", { name: "急所" }).closest("label")).toHaveClass("ui-chip");
    expect(screen.getByRole("checkbox", { name: "リフレクター" }).closest("label")).toHaveClass("ui-chip");
    await user.click(screen.getByRole("radio", { name: "はれ" }));
    expect(screen.getByRole("radio", { name: "はれ" }).closest("label")).toHaveClass("ui-chip--selected");
    expect(screen.getByRole("button", { name: "攻撃側のランクを上げる" })).toHaveClass("ui-stepper__button");
    expect(screen.getByRole("button", { name: "防御側のランクを下げる" })).toHaveClass("ui-stepper__button");
  });
});

describe("文で説明しない", () => {
  test("待機の状態(何も選んでいない)に説明の段落(<p>)が無い", () => {
    const { container } = renderScreen();
    expect(container.querySelectorAll("p")).toHaveLength(0);
  });

  test("詳細・対戦の状態を開いても、条件の説明の段落は増えない(単位の一行だけ)", async () => {
    const { user, container } = renderScreen();
    await choosePair(user);
    await user.click(screen.getByRole("button", { name: "詳細" }));
    await user.click(screen.getByRole("button", { name: /^対戦の状態/ }));
    expect(container.querySelectorAll(".calc-screen__notice")).toHaveLength(0);
  });

  test("防御側の残りHPの長い説明は「説明」ボタンの奥(開くまで出ない)", async () => {
    const { user } = renderScreen();
    await choosePair(user);
    await user.click(screen.getByRole("button", { name: /^対戦の状態/ }));
    const long = "割合(%)で入力します。確定数は残りHPから数えます(%表示は最大HPに対する値のまま)";
    expect(screen.queryByText(long)).toBeNull();
    await user.click(screen.getByRole("button", { name: "説明" }));
    expect(screen.getByText(long)).toBeInTheDocument();
  });
});
