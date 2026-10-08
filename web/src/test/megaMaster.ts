// テスト専用(issue #515、ADR-0320): 架空のメガ種族とメガストーンを足したマスタ。
// 名前は「メガテスト」「テスト…ナイト」、図鑑番号は実在と重ならない 9101 以降(ADR-0002)。実データは使わない。
// 画面テスト(計算・逆算)・マスタ写像のテスト・E2E の pokedex フィクスチャ(e2e/support/pokedexFixtureServer.mjs)が共有する。
//
// 構成(例データの基本種 9001-000 テストほのお・9002-000 テストみずを土台にする):
//   - 9101-001 メガテストほのお: 必要な持ち物 = ほのおナイト(マスタにある)。効果は防御・特防 1.5 倍
//     (「持ち物の候補も比較」の候補にもし出るなら防御側の候補に混ざる効果。単独の選択肢・候補から外れることを確かめる)
//   - 9102-001 メガテストみず: 必要な持ち物 = みずナイト(マスタにある。効果なし)
//   - 9103-001 メガテストくさ: 必要な持ち物の ID がマスタの持ち物に無い(固定できないときの扱いを確かめる)
//   - 基本種の key・日本語名(baseSpeciesKey・baseSpeciesNameJa。ADR-0175 §3)を持つ(固定中の表示に使う。ADR-0326)
// learnset は土台の種族と同じ(ダメージ技を覚える)ので、攻撃側にも選べる。

import type { Item } from "../engine/types";
import { exampleSpecies } from "../master/example/species";
import type { MasterData, MasterSpecies } from "../master/types";

/** マスタの持ち物の ID は /^[a-z0-9]+$/(calc-svc の共通マスタの codeIDPattern。example/ と同じ)。 */
export const MEGA_FIRE_STONE: Item = {
  id: "examplemegastonefire",
  nameJa: "テストほのおナイト",
  effect: { statMods: { def: 6144, spd: 6144 } },
};

export const MEGA_WATER_STONE: Item = {
  id: "examplemegastonewater",
  nameJa: "テストみずナイト",
  effect: null,
};

/** マスタの持ち物に存在しない ID(MEGA_ORPHAN の requiredItemId)。 */
export const MISSING_STONE_ID = "examplemegastonemissing";

function baseSpecies(key: string): MasterSpecies {
  const found = exampleSpecies.find((species) => species.key === key);
  if (found === undefined) {
    throw new Error(`例データに ${key} が無い`);
  }
  return found;
}

export const MEGA_FIRE: MasterSpecies = {
  ...baseSpecies("9001-000"),
  key: "9101-001",
  dexNo: 9101,
  form: 1,
  nameJa: "メガテストほのお",
  baseStats: { hp: 80, atk: 130, def: 90, spa: 110, spd: 90, spe: 95 },
  isMega: true,
  requiredItemId: MEGA_FIRE_STONE.id,
  baseSpeciesKey: "9001-000",
  baseSpeciesNameJa: "テストほのお",
};

export const MEGA_WATER: MasterSpecies = {
  ...baseSpecies("9002-000"),
  key: "9102-001",
  dexNo: 9102,
  form: 1,
  nameJa: "メガテストみず",
  baseStats: { hp: 90, atk: 95, def: 100, spa: 115, spd: 105, spe: 70 },
  isMega: true,
  requiredItemId: MEGA_WATER_STONE.id,
  baseSpeciesKey: "9002-000",
  baseSpeciesNameJa: "テストみず",
};

export const MEGA_ORPHAN: MasterSpecies = {
  ...baseSpecies("9001-000"),
  key: "9103-001",
  dexNo: 9103,
  form: 1,
  nameJa: "メガテストくさ",
  isMega: true,
  requiredItemId: MISSING_STONE_ID,
  baseSpeciesKey: "9001-000",
  baseSpeciesNameJa: "テストほのお",
};

/**
 * ADR-0326(ADR-0175 §4): メガ種族の固定中に持ち物欄へ出す名前の期待値(手書き)。ストーンの nameJa ではなく
 * 「{基本種名}のメガストーン」。基本種名が無いメガ種族は「メガストーン」だけ。
 */
export const MEGA_FIRE_STONE_LABEL = MEGA_FIRE_STONE.nameJa;
export const MEGA_WATER_STONE_LABEL = MEGA_WATER_STONE.nameJa;
export const UNNAMED_MEGA_STONE_LABEL = "メガストーン";

/** メガ種族(MEGA_ORPHAN を含む)。 */
export const MEGA_SPECIES: readonly MasterSpecies[] = [MEGA_FIRE, MEGA_WATER, MEGA_ORPHAN];

/** 基本の例データに、架空のメガ種族(species の末尾)とメガストーン(items の末尾)を足す。 */
export function withMegaFixture(base: MasterData): MasterData {
  return {
    ...base,
    species: [...base.species, ...MEGA_SPECIES],
    items: [...base.items, MEGA_FIRE_STONE, MEGA_WATER_STONE],
  };
}
