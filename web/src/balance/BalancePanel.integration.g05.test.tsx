// I-web-13f(G-05、ADR-0339・ADR-0352): タイプバランス画面の見た目の作り直し
// (docs/design.md「見た目の作り直し方針(G-05)」→「画面ごとの方向」)。
// 確かめること:
//   - 入力済みの枠はポケモンカード(画像かエンブレム + 名前 + タイプバッジ)。空の枠は出さない
//   - 「メンバーを追加」「仮想敵を追加」は「+」の空きタイル(名前は不変・6 体で押せない)
//   - 結果の升目は記号(アイコン)と文字の両方(色だけに頼らない)。列・行の見出しはタイプバッジ
//   - 集計の人数・仮想敵の人数は大きい数字。仮想敵の見出しはポケモンカード(小)
//   - おすすめタイプの穴・候補・特性の表はタイプバッジ(文字の区切り「・」は残す)
//   - 説明の文を出し続けない(待機の状態に <p> が無い)
//   - CSS: 升目の色の組は popPalette.test.ts の検査済みの組だけ・44px・横スクロールの包み・常時動かない
// 読み上げ用の名前・API の呼び方・状態の流れは既存のテスト(BalancePanel.integration*.test.tsx)が守る。

import { screen, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { readFileSync } from "node:fs";
import { beforeAll, describe, expect, test } from "vitest";
import type { components } from "../api/balance.gen";
import type { BalanceClient } from "../api/balanceClient";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import { contrastRatio } from "../test/colorContrast";
import { declarationMap, parseCss, rulesInAtRules, splitSelectors, topLevelRules } from "../test/cssRules";
import { localPath } from "../test/localPath";
import { renderBalanceTestBed } from "../test/balanceTestBed";

type Schemas = components["schemas"];

let master: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
});

const TYPES: readonly Schemas["TypeId"][] = ["normal", "fire", "water", "grass"];

/** すぐ応答を返す BalanceClient。応答の値は升目の種類(弱点・耐性・無効・等倍など)が全部出るように選ぶ。 */
function createInstantClient(): BalanceClient {
  const categories = [
    ["2", "weak"],
    ["4", "quad_weak"],
    ["1/2", "resist"],
    ["0", "immune"],
  ] as const;
  return {
    analyze: (members) =>
      Promise.resolve({
        ok: true,
        value: {
          members: members.map((member) => ({
            pokemonId: member.pokemonId,
            types: ["normal"],
            defense: [
              ...TYPES.map((attackType, index) => {
                const [multiplier, category] = categories[index] ?? (["1", "neutral"] as const);
                return { attackType, multiplier, category, source: "type", effect: "none" } as const;
              }),
              { attackType: "ice", multiplier: "1", category: "neutral", source: "type", effect: "none" },
            ],
          })),
          teamSummary: [...TYPES, "ice"].map((attackType, index) => ({
            attackType: attackType as Schemas["TypeId"],
            weak: index,
            quadWeak: 0,
            resist: 1,
            immune: 0,
            neutral: 2,
          })),
        },
      }),
    coverage: (members) =>
      Promise.resolve({
        ok: true,
        value: {
          members: members.map((member) => ({
            pokemonId: member.pokemonId,
            moveIds: member.moveIds,
            attackTypes: [],
            coverage: [],
          })),
          teamCoverage: (["2", "1", "1/2", "0"] as const).map((bestMultiplier, index) => ({
            defenseType: TYPES[index] ?? "normal",
            bestMultiplier,
            effectiveMembers: 1,
            superEffectiveMembers: 1,
          })),
        },
      }),
    threats: (members, threats) =>
      Promise.resolve({
        ok: true,
        value: {
          threats: threats.map((threat) => ({
            pokemonId: threat.pokemonId,
            attackTypes: [],
            matchups: members.map((member, index) => ({
              pokemonId: member.pokemonId,
              incoming: "2",
              outgoing: "1/2",
              safe: index === 0,
              superEffective: index === 0,
            })),
            safeMembers: 1,
            superEffectiveMembers: 1,
          })),
        },
      }),
    recommendations: () =>
      Promise.resolve({
        ok: true,
        value: {
          defenseHoles: ["fire", "water"],
          offenseHoles: [],
          candidates: [
            {
              types: ["water", "steel"],
              defenseCovered: ["fire", "grass"],
              offenseCovered: [],
              weaknesses: 1,
              pokemon: [],
            },
          ],
          abilityOptions: [],
        },
      }),
  };
}

function species(index: number): MasterSpecies {
  const found = master.species[index];
  if (found === undefined) {
    throw new Error(`例データに ${String(index)} 番目の種族が無い`);
  }
  return found;
}

function renderScreen(): { user: UserEvent; container: HTMLElement } {
  const user = userEvent.setup();
  const { container } = renderBalanceTestBed({ master, client: createInstantClient() });
  return { user, container };
}

async function pick(user: UserEvent, groupName: string, found: MasterSpecies): Promise<void> {
  await user.selectOptions(
    within(screen.getByRole("group", { name: groupName })).getByRole("combobox", { name: "ポケモン" }),
    found.key,
  );
}

async function renderWithResults(): Promise<{ user: UserEvent; container: HTMLElement }> {
  const rendered = renderScreen();
  await pick(rendered.user, "メンバー1", species(0));
  await pick(rendered.user, "仮想敵1", species(1));
  await screen.findByRole("table", { name: "防御相性" });
  await screen.findByRole("table", { name: "相性" });
  await screen.findByRole("table", { name: "おすすめタイプの候補" });
  return rendered;
}

describe("ポケモンカード", () => {
  test("入力済みの枠にカード(エンブレム・名前・タイプバッジ)が出る。空の枠には出さない", async () => {
    const { user } = renderScreen();
    const member = screen.getByRole("group", { name: "メンバー1" });
    expect(member.querySelector(".ui-pokemon-card")).toBeNull();
    await pick(user, "メンバー1", species(0));
    const card = member.querySelector(".ui-pokemon-card");
    expect(card).not.toBeNull();
    expect(within(member).getByTestId("pokemon-icon-emblem")).toBeInTheDocument();
    expect(card?.querySelector(".ui-pokemon-card__name")).toHaveTextContent(species(0).nameJa);
    expect(card?.querySelectorAll(".ui-badge")).toHaveLength(species(0).types.length);
    // 仮想敵の枠も同じ
    expect(screen.getByRole("group", { name: "仮想敵1" }).querySelector(".ui-pokemon-card")).toBeNull();
    await pick(user, "仮想敵1", species(1));
    expect(screen.getByRole("group", { name: "仮想敵1" }).querySelector(".ui-pokemon-card")).not.toBeNull();
  });
});

describe("追加は「+」の空きタイル", () => {
  test("メンバー・仮想敵とも ui-tile--empty で、名前は不変。6 体で押せない", async () => {
    const { user } = renderScreen();
    for (const [name, groupPrefix] of [
      ["メンバーを追加", "メンバー"],
      ["仮想敵を追加", "仮想敵"],
    ] as const) {
      const tile = screen.getByRole("button", { name });
      expect(tile).toHaveClass("ui-tile", "ui-tile--empty");
      expect(tile.querySelector("svg")).not.toBeNull();
      for (let count = 1; count < 6; count += 1) {
        await user.click(screen.getByRole("button", { name }));
      }
      expect(screen.getAllByRole("group", { name: new RegExp(`^${groupPrefix}\\d$`) })).toHaveLength(6);
      expect(screen.getByRole("button", { name })).toBeDisabled();
    }
  });
});

describe("結果の升目は記号と文字の両方", () => {
  test("防御相性: 全てのセルがアイコンと文字を持ち、種類ごとに記号と色の組が違う", async () => {
    await renderWithResults();
    const table = screen.getByRole("table", { name: "防御相性" });
    const cells = within(table).getAllByRole("cell");
    expect(cells.length).toBeGreaterThan(0);
    const kinds = new Map<string, string>();
    for (const cell of cells) {
      const icon = cell.querySelector("svg.ui-icon");
      expect(icon, `「${cell.textContent}」に記号が無い`).not.toBeNull();
      expect(icon).toHaveAttribute("aria-hidden", "true");
      expect(cell.textContent.trim()).toMatch(/^×\S+ \S+$/);
      const word = cell.textContent.trim().split(" ")[1] ?? "";
      const signature = `${icon?.innerHTML ?? ""}|${cell.className}`;
      expect(kinds.get(word) ?? signature).toBe(signature);
      kinds.set(word, signature);
    }
    expect([...kinds.keys()].sort()).toEqual(["弱点", "無効", "等倍", "耐性"]);
    expect(new Set(kinds.values()).size).toBe(4);
  });

  test("列見出しと行見出し(集計)はタイプバッジ", async () => {
    await renderWithResults();
    const defense = screen.getByRole("table", { name: "防御相性" });
    const typeHeaders = within(defense).getAllByRole("columnheader").slice(1);
    expect(typeHeaders.length).toBeGreaterThan(0);
    for (const header of typeHeaders) {
      expect(header.querySelector(".ui-badge")).not.toBeNull();
    }
    for (const header of within(screen.getByRole("table", { name: "チームの集計" })).getAllByRole(
      "rowheader",
    )) {
      expect(header.querySelector(".ui-badge")).not.toBeNull();
    }
  });

  test("防御相性の行見出しはポケモンの画像かエンブレム + 名前(読み上げ名は名前だけ)", async () => {
    await renderWithResults();
    const header = within(screen.getByRole("table", { name: "防御相性" })).getByRole("rowheader", {
      name: species(0).nameJa,
    });
    expect(header.querySelector('[data-testid="pokemon-icon-emblem"], img')).not.toBeNull();
  });

  test("攻撃範囲の最大倍率も記号と文字(抜群・いまひとつ・無効・等倍)", async () => {
    const { user } = renderScreen();
    await pick(user, "メンバー1", species(0));
    const moveSelect = within(screen.getByRole("group", { name: "メンバー1" })).getByRole("combobox", {
      name: "技1",
    });
    const move = within(moveSelect)
      .getAllByRole("option")
      .find((option) => option.getAttribute("value") !== "");
    if (move === undefined) {
      throw new Error("技の選択肢が無い");
    }
    await user.selectOptions(moveSelect, move.getAttribute("value") ?? "");
    const table = await screen.findByRole("table", { name: "攻撃範囲" });
    const best = within(table)
      .getAllByRole("row")
      .slice(1)
      .map((row) => within(row).getAllByRole("cell")[0] as HTMLElement);
    expect(best.map((cell) => cell.textContent.trim())).toEqual([
      "×2 抜群",
      "×1 等倍",
      "×1/2 いまひとつ",
      "×0 無効",
    ]);
    for (const cell of best) {
      expect(cell.querySelector("svg.ui-icon")).not.toBeNull();
    }
    for (const header of within(table).getAllByRole("rowheader")) {
      expect(header.querySelector(".ui-badge")).not.toBeNull();
    }
  });
});

describe("人数は大きい数字・仮想敵はカード", () => {
  test("チームの集計の人数は balance-count の数字", async () => {
    await renderWithResults();
    const table = screen.getByRole("table", { name: "チームの集計" });
    const counts = within(table).getAllByRole("cell");
    expect(counts.length).toBeGreaterThan(0);
    for (const cell of counts) {
      expect(cell).toHaveClass("balance-count-cell");
    }
  });

  test("仮想敵の結果: 見出しはポケモンカード(小)、人数は大きい数字、安全・抜群は記号と文字", async () => {
    await renderWithResults();
    const region = screen.getByRole("region", { name: `仮想敵1(${species(1).nameJa})` });
    expect(region.querySelector(".ui-pokemon-card--small")).not.toBeNull();
    const numbers = [...region.querySelectorAll(".balance-count__number")];
    expect(numbers.map((node) => node.textContent)).toEqual(["1", "1"]);
    const table = within(region).getByRole("table", { name: "相性" });
    const cells = within(table).getAllByRole("cell");
    const [incoming, outgoing, safe, superEffective] = cells;
    expect(incoming?.querySelector("svg")).toBeNull();
    expect(outgoing?.querySelector("svg")).toBeNull();
    expect(safe?.querySelector("svg.ui-icon")).not.toBeNull();
    expect(superEffective?.querySelector("svg.ui-icon")).not.toBeNull();
    expect(safe).toHaveTextContent("安全");
    expect(superEffective).toHaveTextContent("抜群");
  });

  test("おすすめタイプ: 穴と候補のタイプはバッジ。穴が無いときは「なし」の文字", async () => {
    await renderWithResults();
    const region = screen.getByRole("region", { name: "おすすめタイプ" });
    const holes = [...region.querySelectorAll(".balance-holes")];
    expect(holes).toHaveLength(2);
    expect(holes[0]?.querySelectorAll(".ui-badge")).toHaveLength(2);
    expect(holes[1]?.querySelectorAll(".ui-badge")).toHaveLength(0);
    expect(holes[1]).toHaveTextContent("攻撃範囲の穴: なし");
    const candidates = within(region).getByRole("table", { name: "おすすめタイプの候補" });
    expect(within(candidates).getAllByRole("rowheader")[0]?.querySelectorAll(".ui-badge")).toHaveLength(2);
  });
});

describe("文を出し続けない", () => {
  test("待機の状態に説明の段落(<p>)が無い", () => {
    const { container } = renderScreen();
    // 構築の節が自分で出す「ポケモンを選ぶと…」の案内(.team-balance__empty)は節の文言で、パネルの説明ではない。
    expect(container.querySelectorAll("p:not(.team-balance__empty)")).toHaveLength(0);
  });
});

describe("CSS", () => {
  const css = readFileSync(localPath("../balance/BalancePanel.css", import.meta.url), "utf8");
  const tokens = parseCss(readFileSync(localPath("../styles/tokens.css", import.meta.url), "utf8"));
  const isDark = (prelude: string): boolean => /^@media\b.*prefers-color-scheme\s*:\s*dark/.test(prelude);
  const light = declarationMap(
    topLevelRules(tokens).filter((r) => splitSelectors(r.selector).includes(":root")),
  );
  const dark = declarationMap(rulesInAtRules(tokens, isDark));

  const ruleOf = (selector: string): string => {
    const start = css.indexOf(`${selector} {`);
    expect(start, `${selector} の規則が無い`).toBeGreaterThanOrEqual(0);
    return css.slice(start, css.indexOf("}", start));
  };
  const variableIn = (rule: string, property: string): string => {
    const match = new RegExp(`${property}:\\s*var\\(--([a-z-]+)\\)`).exec(rule);
    expect(match, `${property} が var() でない: ${rule}`).not.toBeNull();
    return match?.[1] ?? "";
  };
  const colorOf = (name: string, scheme: "light" | "dark"): string => {
    const value = (scheme === "light" ? light : dark).get(`--${name}`) ?? light.get(`--${name}`);
    expect(value, `--${name} が tokens.css に無い`).toBeDefined();
    return value ?? "";
  };

  // 升目の色分け(背景 = *-soft、記号 = 状態色、文字 = text.primary)。組は popPalette.test.ts で検査済み。
  const TONES = [
    ["good", "success-soft", "success"],
    ["caution", "warning-soft", "warning"],
    ["bad", "danger-soft", "danger"],
    ["info", "info-soft", "info"],
  ] as const;

  test.each(TONES)(
    "升目 %s: 背景 %s の上で記号 %s と文字が 4.5:1 以上(ライト・ダーク)",
    (tone, soft, ink) => {
      const rule = ruleOf(`.balance-cell--${tone}`);
      expect(variableIn(rule, "background-color")).toBe(soft);
      expect(variableIn(ruleOf(`.balance-cell--${tone} .ui-icon`), "color")).toBe(ink);
      expect(variableIn(ruleOf(".balance-cell__text"), "color")).toBe("text-primary");
      for (const scheme of ["light", "dark"] as const) {
        expect(contrastRatio(colorOf(ink, scheme), colorOf(soft, scheme))).toBeGreaterThanOrEqual(4.5);
        expect(contrastRatio(colorOf("text-primary", scheme), colorOf(soft, scheme))).toBeGreaterThanOrEqual(
          4.5,
        );
      }
    },
  );

  test("選択欄は 44px、表は横スクロールの包みの中、人数は結果の大きさ、アニメーションを置かない", () => {
    expect(ruleOf(".balance-member select")).toContain("min-height: 44px");
    expect(ruleOf(".balance-screen__table-scroll")).toContain("overflow-x: auto");
    expect(ruleOf(".balance-count__number")).toContain("var(--font-size-result)");
    expect(css).not.toMatch(/animation|@keyframes/);
  });
});
