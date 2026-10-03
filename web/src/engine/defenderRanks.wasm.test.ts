// issue #274 残り(ADR-0315 案): 防御側のランクを本物の engine.wasm に通す結合テスト。
// 数値の正しさは Go 側(engine/wasmapi/defender_override_test.go)の役割。ここは「画面が組み立てた要求
// (domain/calcConditions → requests)が境界で受理され、向きが正しい」ことだけを見る:
//   物理技は防御側 B(def)+1 でダメージが下がり、-1 で上がる。D(spd)は物理に無関係。特殊技は逆。
// 前提(web/public/engine.wasm)が無ければスキップせず失敗する(CLAUDE.md、ADR-0300 §8)。
// 実行: web の WASM テスト(vitest.wasm.config.ts、node 環境。conditions.wasm.test.ts と同じ)

import { beforeAll, describe, expect, test } from "vitest";
import {
  DEFAULT_CALC_CONDITIONS,
  conditionRequestParts,
  type CalcConditions,
} from "../domain/calcConditions";
import { NEUTRAL_NATURE, ZERO_SP, buildBulkRequest, buildIndividual } from "../domain/requests";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData } from "../master/types";
import { fileWasmLoader, requireWasmArtifacts } from "../test/fileWasmLoader";
import type { CalcEngine, Move } from "./types";
import { createWasmEngine } from "./wasmEngine";

const physical: Move = {
  id: "examplemovetackle",
  nameJa: "テスト物理",
  type: "normal",
  category: "physical",
  power: 80,
  priority: 0,
};
const special: Move = { ...physical, id: "examplemovewaterblast", nameJa: "テスト特殊", category: "special" };

let engine: CalcEngine;
let master: MasterData;

beforeAll(async () => {
  requireWasmArtifacts();
  engine = createWasmEngine(fileWasmLoader());
  master = await exampleMasterSource.load();
});

/** 画面と同じ経路で要求を作り、先頭行の最大%を返す。 */
async function maxPercent(move: Move, patch: Partial<CalcConditions>): Promise<number> {
  const attackerSpecies = master.species[0];
  const defenderSpecies = master.species[1];
  if (attackerSpecies === undefined || defenderSpecies === undefined) {
    throw new Error("例データが足りない");
  }
  const parts = conditionRequestParts({ ...DEFAULT_CALC_CONDITIONS, ...patch });
  const attacker = buildIndividual(attackerSpecies, {
    sp: ZERO_SP,
    nature: NEUTRAL_NATURE,
    item: null,
    ability: { id: "", nameJa: "", effect: null },
  });
  const result = await engine.calcBulk(
    buildBulkRequest({
      attacker,
      defenderSpecies,
      move,
      typeChart: master.typeChart,
      ...(parts.defenderOverride === undefined ? {} : { defenderOverride: parts.defenderOverride }),
    }),
  );
  if (!result.ok) {
    throw new Error(`calcBulk が失敗: ${result.error.code} ${result.error.message}`);
  }
  const first = result.value.rows[0];
  if (first === undefined) {
    throw new Error("行が無い");
  }
  return first.result.maxPercent;
}

describe("防御側のランクが本物の engine で効く", () => {
  test("物理技は防御側 B +1 で%が下がり、B -1 で上がる", async () => {
    const none = await maxPercent(physical, {});
    expect(await maxPercent(physical, { defenderRanks: { def: 1, spd: 0 } })).toBeLessThan(none);
    expect(await maxPercent(physical, { defenderRanks: { def: -1, spd: 0 } })).toBeGreaterThan(none);
  });

  test("物理技に D(spd)は無関係(def/spd を両方送っても関連する方だけが効く)", async () => {
    const none = await maxPercent(physical, {});
    expect(await maxPercent(physical, { defenderRanks: { def: 0, spd: 6 } })).toBe(none);
  });

  test("特殊技は防御側 D +1 で%が下がり、B は無関係", async () => {
    const none = await maxPercent(special, {});
    expect(await maxPercent(special, { defenderRanks: { def: 0, spd: 1 } })).toBeLessThan(none);
    expect(await maxPercent(special, { defenderRanks: { def: 6, spd: 0 } })).toBe(none);
  });

  test("既定(0・0)は従来と同じ結果", async () => {
    expect(await maxPercent(physical, { defenderRanks: { def: 0, spd: 0 } })).toBe(
      await maxPercent(physical, {}),
    );
  });
});
