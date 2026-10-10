// ADR-0144 §3(I-web-13): 本物の engine.wasm で、一括の行の結果と、battleState なしの同じ入力の calc の結果が一致すること。
// (状態を指定したときに行を calc で計算し直しても、状態が無ければ一括と同じ数値になる = 差し替えの前提。)
// 前提(web/public/engine.wasm)が無ければスキップせず失敗する。実行: npm run test:wasm

import { beforeAll, expect, test } from "vitest";
import {
  NEUTRAL_NATURE,
  ZERO_SP,
  buildBulkRequest,
  buildIndividual,
  toEngineSpecies,
} from "../domain/requests";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData } from "../master/types";
import { fileWasmLoader, requireWasmArtifacts } from "../test/fileWasmLoader";
import type { CalcEngine, Move } from "./types";
import { createWasmEngine } from "./wasmEngine";

const move: Move = {
  id: "examplemovetackle",
  nameJa: "テスト物理",
  type: "normal",
  category: "physical",
  power: 80,
  priority: 0,
};

let engine: CalcEngine;
let master: MasterData;

beforeAll(async () => {
  requireWasmArtifacts();
  engine = createWasmEngine(fileWasmLoader());
  master = await exampleMasterSource.load();
});

test("一括の各行の結果 = 同じ入力の battleState なしの calc の結果", async () => {
  const [attackerSpecies, defenderSpecies] = master.species;
  if (attackerSpecies === undefined || defenderSpecies === undefined) {
    throw new Error("例データが足りない");
  }
  const ability = { id: "", nameJa: "", effect: null };
  const attacker = buildIndividual(attackerSpecies, {
    sp: ZERO_SP,
    nature: NEUTRAL_NATURE,
    item: null,
    ability,
  });
  const bulk = await engine.calcBulk(
    buildBulkRequest({ attacker, defenderSpecies, move, typeChart: master.typeChart }),
  );
  if (!bulk.ok) {
    throw new Error(bulk.error.message);
  }
  for (const row of bulk.value.rows) {
    const defender = {
      ...buildIndividual(defenderSpecies, {
        sp: row.defender.sp,
        nature: row.defender.nature,
        item: null,
        ability,
      }),
      species: toEngineSpecies(defenderSpecies),
    };
    const single = await engine.calc({
      format: "single",
      attacker,
      defender,
      move,
      typeChart: master.typeChart,
    });
    expect(single.ok && single.value).toEqual(row.result);
  }
});
