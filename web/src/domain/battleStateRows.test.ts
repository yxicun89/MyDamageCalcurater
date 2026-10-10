// ADR-0144 §3(I-web-13): 一括の行を battleState つきの calc で計算し直す。並列度の上限と、状態が無い行の扱い。
import { describe, expect, test } from "vitest";
import type { CalcEngine, CalcRequest, CalcResult, EngineResult, Individual, Move } from "../engine/types";
import type { MasterSpecies } from "../master/types";
import { exampleMasterSource } from "../master/exampleSource";
import { bulkRow, calcResult, ok } from "../test/fakeEngine";
import { RECALC_CONCURRENCY, recalcRowsWithBattleState } from "./battleStateRows";
import { NEUTRAL_NATURE, ZERO_SP, buildIndividual } from "./requests";

const move: Move = {
  id: "m",
  nameJa: "技",
  type: "normal",
  category: "physical",
  power: 40,
  priority: 0,
};

async function setup(): Promise<{ species: MasterSpecies; attacker: Individual }> {
  const master = await exampleMasterSource.load();
  const species = master.species[0];
  if (species === undefined) {
    throw new Error("例データが足りない");
  }
  const attacker = buildIndividual(species, {
    sp: ZERO_SP,
    nature: NEUTRAL_NATURE,
    item: null,
    ability: { id: "", nameJa: "", effect: null },
  });
  return { species, attacker };
}

describe("recalcRowsWithBattleState", () => {
  test("同時に走らせる1対1の計算は RECALC_CONCURRENCY 件まで。結果は行の順に差し替わる", async () => {
    const { species, attacker } = await setup();
    let inFlight = 0;
    let peak = 0;
    const engine: CalcEngine = {
      calc: async (request: CalcRequest): Promise<EngineResult<CalcResult>> => {
        inFlight += 1;
        peak = Math.max(peak, inFlight);
        await Promise.resolve();
        inFlight -= 1;
        return ok({ ...calcResult(), minDamage: request.battleState?.defenderCurrentHp ?? -1 });
      },
      calcBulk: () => Promise.reject(new Error("使わない")),
      calcReverse: () => Promise.reject(new Error("使わない")),
    };
    const rows = Array.from({ length: 10 }, (_, index) => ({
      ...bulkRow({ preset: `p${index}` }),
      result: { ...calcResult(), defenderHP: 100 + index },
    }));
    const result = await recalcRowsWithBattleState({
      engine,
      bulk: { defenderSpeciesKey: species.key, rows },
      attacker,
      defenderSpecies: species,
      itemVariants: [null],
      defenderAbilities: [],
      move,
      typeChart: { types: [], effectiveness: {} },
      battleState: { defenderPercent: 50 },
    });
    expect(peak).toBeLessThanOrEqual(RECALC_CONCURRENCY);
    expect(peak).toBeGreaterThan(1);
    expect(result.ok && result.value.rows.map((row) => row.result.minDamage)).toEqual(
      rows.map((_, index) => Math.floor((100 + index) / 2)),
    );
    expect(result.ok && result.value.rows.map((row) => row.preset)).toEqual(rows.map((row) => row.preset));
  });
});
