// ADR-0144 §3(I-web-13): 一括計算(calcBulk)には battleState が無いので、対戦の状態を指定したときだけ、
// 一括計算の各行を1対1の計算(calc。battleState つき)で計算し直し、行の結果を差し替える。
// 一括の要求は従来と同じまま(状態を指定しないときはこの関数を通さない)。

import type {
  Ability,
  BattleState,
  BulkResult,
  CalcEngine,
  EngineResult,
  Field,
  Individual,
  Item,
  Move,
  Ranks,
  TypeChart,
} from "../engine/types";
import type { MasterSpecies } from "../master/types";
import { buildIndividual } from "./requests";

export interface RecalcRowsInput {
  readonly engine: CalcEngine;
  readonly bulk: BulkResult;
  readonly attacker: Individual;
  readonly defenderSpecies: MasterSpecies;
  /** 行の itemId を引く先(一括計算に渡した持ち物の通り)。 */
  readonly itemVariants: ReadonlyArray<Item | null>;
  /** 行の abilityId を引く先(一括計算に渡した防御側の特性の候補)。 */
  readonly defenderAbilities: readonly Ability[];
  readonly move: Move;
  readonly typeChart: TypeChart;
  readonly battleState: BattleState;
  readonly field?: Field;
  readonly critical?: boolean;
  readonly defenderRanks?: Ranks;
  readonly signal?: AbortSignal;
}

/** 各行を battleState つきの calc で計算し直す。どれかが失敗したら最初の失敗を返す。 */
export async function recalcRowsWithBattleState(input: RecalcRowsInput): Promise<EngineResult<BulkResult>> {
  const { engine, bulk, defenderSpecies, battleState } = input;
  const results = await Promise.all(
    bulk.rows.map((row) => {
      const item = input.itemVariants.find((candidate) => candidate?.id === row.itemId) ?? null;
      const ability = input.defenderAbilities.find((candidate) => candidate.id === row.abilityId) ??
        input.defenderAbilities[0] ?? { id: "", nameJa: "", effect: null };
      const defender = buildIndividual(defenderSpecies, {
        sp: row.defender.sp,
        nature: row.defender.nature,
        item,
        ability,
        defending: true,
        ...(input.defenderRanks === undefined ? {} : { ranks: input.defenderRanks }),
      });
      return engine.calc(
        {
          format: "single",
          attacker: input.attacker,
          defender,
          move: input.move,
          typeChart: input.typeChart,
          battleState,
          ...(input.field === undefined ? {} : { field: input.field }),
          ...(input.critical === undefined ? {} : { critical: input.critical }),
        },
        input.signal,
      );
    }),
  );
  const rows = [];
  for (const [index, result] of results.entries()) {
    const row = bulk.rows[index];
    if (row === undefined) {
      continue;
    }
    if (!result.ok) {
      return result;
    }
    rows.push({ ...row, result: result.value });
  }
  return { ok: true, value: { ...bulk, rows } };
}
