// ADR-0144 §3(I-web-14): 一括計算(calcBulk)には battleState が無いので、対戦の状態を指定したときだけ、
// 一括計算の各行を1対1の計算(calc。battleState つき)で計算し直し、行の結果を差し替える。
// 一括の要求は従来と同じまま(状態を指定しないときはこの関数を通さない)。

import type {
  Ability,
  BulkResult,
  BulkRow,
  CalcResult,
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
import { battleStateOfRow, type ScreenBattleState } from "./battleState";
import { buildIndividual } from "./requests";

/** 1対1の計算を同時に走らせる数の上限(行が多いとき WASM/サーバーへ一度に投げすぎない)。 */
export const RECALC_CONCURRENCY = 4;

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
  /** 画面の状態。防御側は割合なので、行ごとに(行の最大 HP = defenderHP で)実数値に換算する。 */
  readonly battleState: ScreenBattleState;
  readonly field?: Field;
  readonly critical?: boolean;
  readonly defenderRanks?: Ranks;
  readonly signal?: AbortSignal;
}

/** 各行を battleState つきの calc で計算し直す。どれかが失敗したら最初の失敗を返す。 */
export async function recalcRowsWithBattleState(input: RecalcRowsInput): Promise<EngineResult<BulkResult>> {
  const { bulk } = input;
  const results: Array<EngineResult<CalcResult> | null> = [];
  for (let start = 0; start < bulk.rows.length; start += RECALC_CONCURRENCY) {
    const chunk = bulk.rows.slice(start, start + RECALC_CONCURRENCY);
    results.push(...(await Promise.all(chunk.map((row) => recalcRow(input, row)))));
  }
  const rows = [];
  for (const [index, result] of results.entries()) {
    const row = bulk.rows[index];
    if (row === undefined) {
      continue;
    }
    if (result === null) {
      rows.push(row);
      continue;
    }
    if (!result.ok) {
      return result;
    }
    rows.push({ ...row, result: result.value });
  }
  return { ok: true, value: { ...bulk, rows } };
}

/** 1行を計算し直す。その行に付ける状態が無ければ(満タンに換算された等)null = 一括の結果のまま。 */
function recalcRow(input: RecalcRowsInput, row: BulkRow): Promise<EngineResult<CalcResult> | null> {
  const battleState = battleStateOfRow(input.battleState, row.result.defenderHP);
  if (battleState === undefined) {
    return Promise.resolve(null);
  }
  const item = input.itemVariants.find((candidate) => candidate?.id === row.itemId) ?? null;
  const ability = input.defenderAbilities.find((candidate) => candidate.id === row.abilityId) ??
    input.defenderAbilities[0] ?? { id: "", nameJa: "", effect: null };
  const defender = buildIndividual(input.defenderSpecies, {
    sp: row.defender.sp,
    nature: row.defender.nature,
    item,
    ability,
    defending: true,
    ...(input.defenderRanks === undefined ? {} : { ranks: input.defenderRanks }),
  });
  return input.engine.calc(
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
}
