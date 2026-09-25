// issue #304: 見える見出し・見えるラベルと accessible name を、同じ語から組み立てること。
// 正は docs/design.md「入力のラベル」、WCAG 2.2 SC 2.5.3(見出しどおりの名前)。
//
// 画面の JSX に同じ日本語を2回書くと(見える見出しと aria-label)、片方だけ直したときに
// 黙ってズレる。文言資源の側で「<領域の見出しの語>の<ラベルの語>」を組み立て、
// 画面は組み立て済みの語だけを使う。ここはその組み立てと、既存の accessible name が
// 変わっていないこと(回帰)を確かめる。

import { describe, expect, test } from "vitest";
import { balanceScreenText, calcScreenText, reverseScreenText } from "./ja";

describe("欄の accessible name は「<領域の見出しの語>の<ラベルの語>」", () => {
  test.each([
    ["攻撃側のポケモン", calcScreenText.attackerRegionLabel, calcScreenText.pokemonFieldLabel],
    ["防御側のポケモン", calcScreenText.defenderRegionLabel, calcScreenText.pokemonFieldLabel],
    ["攻撃側の持ち物", calcScreenText.attackerRegionLabel, calcScreenText.itemFieldLabel],
    ["防御側の持ち物", calcScreenText.defenderRegionLabel, calcScreenText.itemFieldLabel],
    ["自分のポケモン", reverseScreenText.myRegionLabel, calcScreenText.pokemonFieldLabel],
    ["相手のポケモン", reverseScreenText.theirRegionLabel, calcScreenText.pokemonFieldLabel],
    ["自分の持ち物", reverseScreenText.myRegionLabel, calcScreenText.itemFieldLabel],
  ])("%s = %s + の + %s", (composed, region, field) => {
    expect(composed).toBe(`${region}の${field}`);
  });

  test("計算・逆算の欄の accessible name は今までと同じ文字列(既存テストの引き方を変えない)", () => {
    expect(calcScreenText.attackerPokemonLabel).toBe("攻撃側のポケモン");
    expect(calcScreenText.defenderPokemonLabel).toBe("防御側のポケモン");
    expect(calcScreenText.attackerItemLabel).toBe("攻撃側の持ち物");
    expect(calcScreenText.defenderItemLabel).toBe("防御側の持ち物");
    expect(reverseScreenText.mySpeciesLabel).toBe("自分のポケモン");
    expect(reverseScreenText.theirSpeciesLabel).toBe("相手のポケモン");
    expect(reverseScreenText.myItemLabel).toBe("自分の持ち物");
    expect(calcScreenText.moveLabel).toBe("技");
  });

  test("組み立て済みの語は、見える見出しの語とラベルの語の両方を含む(SC 2.5.3)", () => {
    for (const composed of [
      calcScreenText.attackerPokemonLabel,
      calcScreenText.defenderPokemonLabel,
      calcScreenText.attackerItemLabel,
      calcScreenText.defenderItemLabel,
      reverseScreenText.mySpeciesLabel,
      reverseScreenText.theirSpeciesLabel,
      reverseScreenText.myItemLabel,
    ]) {
      expect(composed.length).toBeGreaterThan(0);
    }
    expect(calcScreenText.attackerPokemonLabel).toContain(calcScreenText.attackerRegionLabel);
    expect(calcScreenText.attackerPokemonLabel).toContain(calcScreenText.pokemonFieldLabel);
    expect(reverseScreenText.myItemLabel).toContain(reverseScreenText.myRegionLabel);
    expect(reverseScreenText.myItemLabel).toContain(calcScreenText.itemFieldLabel);
  });

  test("同じ物を指すラベルの語は画面をまたいで同じ(タイプバランスの「ポケモン」)", () => {
    expect(balanceScreenText.speciesLabel).toBe(calcScreenText.pokemonFieldLabel);
  });
});

describe("未選択の select に出す文言", () => {
  test("ポケモンの未選択は、何をすればよいか分かる文言(空文字にしない)", () => {
    expect(calcScreenText.speciesPlaceholderOption).toBe("ポケモンを選ぶ");
  });

  test("タイプバランスの特性は、ポケモンを選ぶまで選べないことが分かる文言", () => {
    expect(balanceScreenText.abilityPlaceholderOption.trim()).not.toBe("");
    expect(balanceScreenText.abilityPlaceholderOption).toContain(calcScreenText.pokemonFieldLabel);
  });
});

describe("逆算の観測欄の説明(単位と観測した側で変わる)", () => {
  const hints = [
    { side: "defender", unit: "percent" } as const,
    { side: "defender", unit: "damage" } as const,
    { side: "attacker", unit: "percent" } as const,
    { side: "attacker", unit: "damage" } as const,
  ];

  test("4通りすべてが空でなく、互いに違う文になる", () => {
    const texts = hints.map(({ side, unit }) => reverseScreenText.observationHintLabel(side, unit));
    for (const text of texts) {
      expect(text.trim()).not.toBe("");
    }
    expect(new Set(texts).size).toBe(texts.length);
  });

  test("与えたダメージ(side=defender)は相手の HP、受けたダメージ(side=attacker)は自分の HP が減る", () => {
    expect(reverseScreenText.observationHintLabel("defender", "percent")).toContain("相手");
    expect(reverseScreenText.observationHintLabel("defender", "damage")).toContain("相手");
    expect(reverseScreenText.observationHintLabel("attacker", "percent")).toContain("自分");
    expect(reverseScreenText.observationHintLabel("attacker", "damage")).toContain("自分");
  });

  test("% は割合、HP は実数値であることが文から分かる", () => {
    expect(reverseScreenText.observationHintLabel("defender", "percent")).toContain("割合");
    expect(reverseScreenText.observationHintLabel("defender", "percent")).toContain(
      reverseScreenText.percentUnitLabel,
    );
    expect(reverseScreenText.observationHintLabel("defender", "damage")).toContain("実数値");
    expect(reverseScreenText.observationHintLabel("defender", "damage")).toContain(
      reverseScreenText.damageUnitLabel,
    );
  });
});
