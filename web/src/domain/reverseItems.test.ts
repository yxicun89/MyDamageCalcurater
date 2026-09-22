// P4-4: 逆算の持ち物候補(ADR-0300 §7、requirements.md「逆算の持ち物候補」)。ID・名前では選ばず、効果データから選ぶ。
//   防御側(side defender)= なし / 技の分類の防御側ステータス(物理→def、特殊→spd)を上げる / 技のタイプの半減きのみ
//   攻撃側(side attacker)= なし / ダメージ倍率(damageMod)/ 分類の威力(powerMod。powerCategory が技の分類か全分類)
//                          / 技のタイプの強化(boostType が技のタイプ)
// 先頭は必ず null(持ち物なし)、続きはマスタの順のまま(並べ替えない)。

import { describe, expect, test } from "vitest";
import type { Item, Move } from "../engine/types";
import { reverseItemCandidates } from "./reverseItems";

function move(category: Move["category"], type = "fire"): Move {
  return { id: `m-${category}`, nameJa: "わざ", type, category, power: 80, priority: 0 };
}

const defUp: Item = { id: "def-up", nameJa: "ぼうぎょ", effect: { statMods: { def: 6144 } } };
const spdUp: Item = { id: "spd-up", nameJa: "とくぼう", effect: { statMods: { spd: 6144 } } };
const defNeutral: Item = { id: "def-neutral", nameJa: "ぼうぎょ等倍", effect: { statMods: { def: 4096 } } };
const fireBerry: Item = { id: "fire-berry", nameJa: "ほのおきのみ", effect: { resistBerryType: "fire" } };
const waterBerry: Item = { id: "water-berry", nameJa: "みずきのみ", effect: { resistBerryType: "water" } };
const damageUp: Item = { id: "damage-up", nameJa: "ダメージ", effect: { damageMod: 5324 } };
const physicalPower: Item = {
  id: "physical-power",
  nameJa: "物理威力",
  effect: { powerMod: 4505, powerCategory: "physical" },
};
const specialPower: Item = {
  id: "special-power",
  nameJa: "特殊威力",
  effect: { powerMod: 4505, powerCategory: "special" },
};
const anyPower: Item = { id: "any-power", nameJa: "全分類威力", effect: { powerMod: 4505 } };
const fireBoost: Item = {
  id: "fire-boost",
  nameJa: "ほのお強化",
  effect: { boostType: "fire", boostTypeMod: 4915 },
};
const waterBoost: Item = {
  id: "water-boost",
  nameJa: "みず強化",
  effect: { boostType: "water", boostTypeMod: 4915 },
};
const noEffect: Item = { id: "no-effect", nameJa: "効果なし", effect: null };

const all: readonly Item[] = [
  noEffect,
  damageUp,
  defUp,
  physicalPower,
  spdUp,
  fireBerry,
  specialPower,
  waterBerry,
  anyPower,
  fireBoost,
  defNeutral,
  waterBoost,
];

function ids(candidates: ReadonlyArray<Item | null>): Array<string | null> {
  return candidates.map((item) => item?.id ?? null);
}

describe("reverseItemCandidates(防御側)", () => {
  test("物理技: なし・防御を上げる・技のタイプの半減きのみ(マスタの順)", () => {
    expect(ids(reverseItemCandidates("defender", all, move("physical", "fire")))).toEqual([
      null,
      "def-up",
      "fire-berry",
    ]);
  });

  test("特殊技: なし・特防を上げる・技のタイプの半減きのみ", () => {
    expect(ids(reverseItemCandidates("defender", all, move("special", "water")))).toEqual([
      null,
      "spd-up",
      "water-berry",
    ]);
  });

  test("該当が無ければ持ち物なしの1通り", () => {
    expect(ids(reverseItemCandidates("defender", [noEffect, damageUp], move("physical")))).toEqual([null]);
  });

  test("変化技には持ち物なしの1通り(defensiveItemCandidates と判定を共有する)", () => {
    expect(ids(reverseItemCandidates("defender", all, move("status")))).toEqual([null]);
  });
});

describe("reverseItemCandidates(攻撃側)", () => {
  test("物理技: なし・ダメージ倍率・物理/全分類の威力・技のタイプの強化(マスタの順)", () => {
    expect(ids(reverseItemCandidates("attacker", all, move("physical", "fire")))).toEqual([
      null,
      "damage-up",
      "physical-power",
      "any-power",
      "fire-boost",
    ]);
  });

  test("特殊技: 特殊/全分類の威力と、技のタイプ(みず)の強化だけ", () => {
    expect(ids(reverseItemCandidates("attacker", all, move("special", "water")))).toEqual([
      null,
      "damage-up",
      "special-power",
      "any-power",
      "water-boost",
    ]);
  });

  test("防御側の持ち物(防御を上げる・半減きのみ)は攻撃側の候補に入らない", () => {
    expect(ids(reverseItemCandidates("attacker", [defUp, fireBerry, spdUp], move("physical")))).toEqual([
      null,
    ]);
  });

  test("技の分類の攻撃側ステータス(物理→atk、特殊→spa)を上げる持ち物(こだわり系のような statMods)も候補になる: マスタに戻れば自動で復活する(requirements.md「逆算の持ち物候補」)", () => {
    const atkUp: Item = { id: "atk-up", nameJa: "こうげき上昇", effect: { statMods: { atk: 6144 } } };
    const spaUp: Item = { id: "spa-up", nameJa: "とくこう上昇", effect: { statMods: { spa: 6144 } } };
    const items = [atkUp, spaUp];
    expect(ids(reverseItemCandidates("attacker", items, move("physical")))).toEqual([null, "atk-up"]);
    expect(ids(reverseItemCandidates("attacker", items, move("special")))).toEqual([null, "spa-up"]);
  });
});

test("渡した Item の実体をそのまま返す(engine に解決済みの効果を渡すため。コピーしない)", () => {
  const [, first] = reverseItemCandidates("defender", all, move("physical", "fire"));
  expect(first).toBe(defUp);
});
