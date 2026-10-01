// テスト専用(issue 272、ADR-0126・ADR-0311): 特性の数が違う種族を持つマスタの fixture。
// 例データの種族は特性が1〜2件だけなので、3件・4件の種族(境界: 候補は先頭3件まで)を足して使う。
// 実在の種族・特性には依存しない(架空の ID と名前)。

import type { Ability } from "../engine/types";
import type { MasterData, MasterSpecies } from "../master/types";

/** 4件目まである架空の特性(ID は calc-svc の codeIDPattern に合わせ英小文字と数字のみ)。 */
export const quadAbilities: readonly Ability[] = [
  { id: "exampleabilityq1", nameJa: "テスト特性イチ", effect: null },
  { id: "exampleabilityq2", nameJa: "テスト特性ニ", effect: null },
  { id: "exampleabilityq3", nameJa: "テスト特性サン", effect: null },
  { id: "exampleabilityq4", nameJa: "テスト特性ヨン", effect: null },
];

export interface AbilityMaster {
  readonly master: MasterData;
  /** 特性が1つの種族。 */
  readonly single: MasterSpecies;
  /** 特性が2つの種族(スロット順に [先頭, 後ろ])。 */
  readonly dual: MasterSpecies;
  /** 特性が4つの種族(quadAbilities の順)。 */
  readonly quad: MasterSpecies;
}

function withAbilities(base: MasterSpecies, key: string, abilityIds: readonly string[]): MasterSpecies {
  return { ...base, key, abilities: abilityIds };
}

/**
 * 例データのマスタに、特性が 1 / 2 / 4 つの種族を足したマスタ。
 * 3 つの種族とも技は同じ種族(例データの先頭)のものを覚える(ダメージ技が選べる)。
 */
export function abilityMasterFrom(master: MasterData): AbilityMaster {
  const base = master.species[0];
  const none = master.abilities.find((ability) => ability.effect === null);
  const adapt = master.abilities.find((ability) => ability.effect !== null);
  if (base === undefined || none === undefined || adapt === undefined) {
    throw new Error("例データに種族 / 特性が足りない");
  }
  const single = withAbilities(base, "9101-000", [none.id]);
  const dual = withAbilities(base, "9102-000", [adapt.id, none.id]);
  const quad = withAbilities(
    base,
    "9104-000",
    quadAbilities.map((ability) => ability.id),
  );
  return {
    master: {
      ...master,
      species: [...master.species, single, dual, quad],
      abilities: [...master.abilities, ...quadAbilities],
    },
    single,
    dual,
    quad,
  };
}

/** 特性が1つも無いマスタ(種族の abilities も空)。セレクトを出さず従来どおり NO_ABILITY になる。 */
export function noAbilityMasterFrom(master: MasterData): MasterData {
  return {
    ...master,
    species: master.species.map((species) => ({ ...species, abilities: [] })),
    abilities: [],
  };
}
