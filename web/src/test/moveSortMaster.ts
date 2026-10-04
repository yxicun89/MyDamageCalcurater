// テスト専用(I-web-9 = F-02、ADR-0335): 技の並び替えを確かめるための架空のマスタ。
// 例データに、技を6つ覚える架空の種族(9200-000)を足す。名前は実在の技ではない(ADR-0002)。
// learnset の順・五十音順・タイプ順のそれぞれで並びが違い、「既定で選ばれる技(最初のダメージ技)」が
// 五十音順の先頭と違うように作ってある(並びを変えても既定の自動選択が変わらないことを確かめるため)。
//
//   learnset 順 : たいあたり(normal) / かみなり(electric) / ガンマ(fire) / がまん(normal) / スパーク(electric) / スーパー(electric)
//   五十音順    : がまん / かみなり / ガンマ / スーパー / スパーク / たいあたり
//                 (濁点・ひらがな/カタカナの違いは同じ字として比べ、「ー」は Intl.Collator("ja") の扱いに従う)
//   タイプ順    : マスタの相性表の並び(electric < fire < normal)で群にし、群の中は五十音順
//                 electric[かみなり / スーパー / スパーク] fire[ガンマ] normal[がまん / たいあたり]

import type { Move } from "../engine/types";
import { exampleSpecies } from "../master/example/species";
import type { MasterData, MasterSpecies } from "../master/types";

function move(id: string, nameJa: string, type: string, category: Move["category"], power: number): Move {
  return { id, nameJa, type, category, power, priority: 0 };
}

export const SORT_MOVES: readonly Move[] = [
  move("examplesorttackle", "たいあたり", "normal", "physical", 40),
  move("examplesortthunder", "かみなり", "electric", "special", 90),
  move("examplesortgamma", "ガンマ", "fire", "special", 80),
  move("examplesortpatience", "がまん", "normal", "physical", 30),
  move("examplesortspark", "スパーク", "electric", "physical", 65),
  move("examplesortsuper", "スーパー", "electric", "special", 70),
];

/** learnset の順(既定の並び)。 */
export const LEARNSET_IDS: readonly string[] = SORT_MOVES.map((m) => m.id);
export const KANA_IDS: readonly string[] = [
  "examplesortpatience",
  "examplesortthunder",
  "examplesortgamma",
  "examplesortsuper",
  "examplesortspark",
  "examplesorttackle",
];
export const TYPE_IDS: readonly string[] = [
  "examplesortthunder",
  "examplesortsuper",
  "examplesortspark",
  "examplesortgamma",
  "examplesortpatience",
  "examplesorttackle",
];
/** タイプ順の見出し(optgroup の label。相性表の並び)と、その群の技 ID。 */
export const TYPE_GROUPS: readonly { readonly label: string; readonly ids: readonly string[] }[] = [
  { label: "でんき", ids: ["examplesortthunder", "examplesortsuper", "examplesortspark"] },
  { label: "ほのお", ids: ["examplesortgamma"] },
  { label: "ノーマル", ids: ["examplesortpatience", "examplesorttackle"] },
];

export const SORT_SPECIES_KEY = "9200-000";

export function sortSpecies(): MasterSpecies {
  const base = exampleSpecies[0];
  if (base === undefined) {
    throw new Error("例データに種族が無い");
  }
  return {
    ...base,
    key: SORT_SPECIES_KEY,
    dexNo: 9200,
    nameJa: "テストならびかえ",
    learnset: [...LEARNSET_IDS],
  };
}

/** 例データのマスタに、並び替え用の種族と技を足す。 */
export function withSortFixture(master: MasterData): MasterData {
  return { ...master, species: [...master.species, sortSpecies()], moves: [...master.moves, ...SORT_MOVES] };
}
