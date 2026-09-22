// P4-5(ADR-0301 §5): Web の例データを calc-svc のマスタのスナップショット(services/calc/README.md の
// 暫定スキーマ schemaVersion 1)に書き出す。calc-svc をその出力で起動すれば、Web の例データの ID が
// そのまま API に通る(オンラインの動作確認・P4-6 の E2E)。
// calc-svc のローダーは未知のフィールドをエラーにするので、画面だけの追加フィールド(species.learnset)と
// 別に渡すフィールド(typeChart。CALC_TYPECHART_PATH)は書かない(services/calc/README.md)。

import type { Ability, Item, Move, Stats } from "../engine/types";
import type { MasterData, MasterNature } from "./types";

/** calc-svc のスナップショットの schemaVersion(services/calc/README.md)。 */
const CALC_SNAPSHOT_SCHEMA_VERSION = 1;

/** スナップショットの種族(engine の Species と同じ形。learnset は含まない)。 */
export interface CalcSnapshotSpecies {
  readonly key: string;
  readonly dexNo: number;
  readonly form: number;
  readonly nameJa: string;
  readonly types: readonly string[];
  readonly baseStats: Stats;
  readonly abilities: readonly string[];
}

/** calc-svc のマスタのスナップショット(services/calc/README.md、暫定スキーマ schemaVersion 1)。 */
export interface CalcSnapshot {
  readonly schemaVersion: 1;
  readonly species: readonly CalcSnapshotSpecies[];
  readonly moves: readonly Move[];
  readonly items: readonly Item[];
  readonly abilities: readonly Ability[];
  readonly natures: readonly MasterNature[];
}

/**
 * MasterData を calc-svc のスナップショットの形にする。入力を書き換えない(新しい配列・オブジェクトを作る)。
 * moves・items・abilities・natures は engine の DTO と同じ形なのでそのまま使う。
 */
export function toCalcSnapshot(master: MasterData): CalcSnapshot {
  return {
    schemaVersion: CALC_SNAPSHOT_SCHEMA_VERSION,
    species: master.species.map((species) => ({
      key: species.key,
      dexNo: species.dexNo,
      form: species.form,
      nameJa: species.nameJa,
      types: species.types,
      baseStats: species.baseStats,
      abilities: species.abilities,
    })),
    moves: master.moves,
    items: master.items,
    abilities: master.abilities,
    natures: master.natures,
  };
}
