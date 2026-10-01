// issue #274(ADR-0312): 計算画面の「詳細」の条件(急所・やけど・天候)を本物の engine.wasm に通す結合テスト。
// 数値の正しさは Go のゴールデン側の役割。ここは「画面が組み立てた要求(domain/calcConditions → requests)が
// 境界で受理され、%が条件で変わる向きに動く」ことだけを見る(急所は上がる・やけどは物理で下がる・
// 炎技ははれで上がり あめで下がる)。条件なしは従来の要求と同じ結果。
// 前提(web/public/engine.wasm)が無ければスキップせず失敗する(CLAUDE.md、ADR-0300 §8)。
// 実行: make web-test-wasm(vitest.wasm.config.ts、node 環境)

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

const fireMove: Move = {
  id: "examplemovefire",
  nameJa: "テスト炎技",
  type: "fire",
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

/** 画面と同じ経路(conditionRequestParts → buildIndividual / buildBulkRequest)で要求を作り、先頭行の最大%を返す。 */
async function maxPercent(patch: Partial<CalcConditions>): Promise<number> {
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
    ...(parts.status === undefined ? {} : { status: parts.status }),
    ...(parts.ranks === undefined ? {} : { ranks: parts.ranks }),
  });
  const result = await engine.calcBulk(
    buildBulkRequest({
      attacker,
      defenderSpecies,
      move: fireMove,
      typeChart: master.typeChart,
      ...(parts.critical === undefined ? {} : { critical: parts.critical }),
      ...(parts.field === undefined ? {} : { field: parts.field }),
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

describe("画面の条件が本物の engine で効く", () => {
  test("急所で%が上がる", async () => {
    expect(await maxPercent({ critical: true })).toBeGreaterThan(await maxPercent({}));
  });

  test("やけど(物理)で%が下がる", async () => {
    expect(await maxPercent({ burned: true })).toBeLessThan(await maxPercent({}));
  });

  test("はれで炎技の%が上がり、あめで下がる", async () => {
    const none = await maxPercent({});
    expect(await maxPercent({ weather: "sun" })).toBeGreaterThan(none);
    expect(await maxPercent({ weather: "rain" })).toBeLessThan(none);
  });

  test("攻撃ランク +1 で%が上がる(atk と spa を両方送っても物理技は atk だけが効く)", async () => {
    const none = await maxPercent({});
    expect(await maxPercent({ ranks: { atk: 1, spa: -6 } })).toBeGreaterThan(none);
    expect(await maxPercent({ ranks: { atk: 0, spa: 6 } })).toBe(none);
  });

  test("防御側のリフレクターで物理の%が下がる(ひかりのかべは物理に効かない)", async () => {
    const none = await maxPercent({});
    const screens = { reflect: false, lightScreen: false, auroraVeil: false };
    expect(await maxPercent({ defenderScreens: { ...screens, reflect: true } })).toBeLessThan(none);
    expect(await maxPercent({ defenderScreens: { ...screens, lightScreen: true } })).toBe(none);
  });
});
