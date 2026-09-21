// P4-2: WASM 結合テスト(ADR-0016 §8)。Web が組み立てたリクエストを本物の engine.wasm(`make wasm` の成果物)に
// 通し、成功の封筒と結果の形を確かめる。計算の数値の正しさはゴールデンと Go/WASM 一致テストの役割なので見ない。
// 前提(web/public/engine.wasm・wasm_exec.js)が無ければスキップせず失敗する(CLAUDE.md、ADR-0016 §8)。
//
// 実行: make web-test-wasm(vitest.wasm.config.ts、node 環境)

import { existsSync, readFileSync } from "node:fs";
import vm from "node:vm";
import { beforeAll, describe, expect, test } from "vitest";
import {
  NEUTRAL_NATURE,
  ZERO_SP,
  buildBulkRequest,
  buildCalcRequest,
  buildIndividual,
  defaultAbility,
  defensiveItemCandidates,
} from "../domain/requests";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import { localPath } from "../test/localPath";
import type { BulkRequest, CalcEngine, Item, Move } from "./types";
import { createWasmEngine, type WasmLoader } from "./wasmEngine";

const wasmExecPath = localPath("../../public/wasm_exec.js", import.meta.url);
const wasmPath = localPath("../../public/engine.wasm", import.meta.url);

/** ADR-0009 §1 の既定カタログ(presetKeys 省略時、技の分類で選ばれる5行)の Key と順序。 */
const physicalPresetKeys = ["none", "hp", "hb_boost", "hb", "hb_full"];
const specialPresetKeys = ["none", "hp", "hd_boost", "hd", "hd_full"];
/** engine の1回の計算が返す乱数の数(ダメージ乱数 85〜100 の16段階)。 */
const rollCount = 16;

/** Node 用の読み込み口: ブラウザの browserWasmLoader と同じ2段階を、ファイルから行う。 */
function fileWasmLoader(): WasmLoader {
  return {
    loadRuntime() {
      new vm.Script(readFileSync(wasmExecPath, "utf8"), { filename: wasmExecPath }).runInThisContext();
      return Promise.resolve();
    },
    async instantiate(importObject) {
      const { instance } = await WebAssembly.instantiate(readFileSync(wasmPath), importObject);
      return instance;
    },
  };
}

let engine: CalcEngine;
let master: MasterData;

beforeAll(async () => {
  for (const path of [wasmExecPath, wasmPath]) {
    if (!existsSync(path)) {
      throw new Error(`${path} が無い。先に \`make wasm\` を実行すること(このテストはスキップしない)`);
    }
  }
  engine = createWasmEngine(fileWasmLoader());
  master = await exampleMasterSource.load();
});

function movesOf(species: MasterSpecies): Move[] {
  return species.learnset.flatMap((id) => master.moves.filter((move) => move.id === id));
}

/** 指定の分類の技を覚える攻撃側と、その技・別の防御側の組。 */
function matchup(category: "physical" | "special"): {
  attacker: MasterSpecies;
  defender: MasterSpecies;
  move: Move;
} {
  for (const attacker of master.species) {
    const move = movesOf(attacker).find((candidate) => candidate.category === category);
    const defender = master.species.find((species) => species.key !== attacker.key);
    if (move !== undefined && defender !== undefined) {
      return { attacker, defender, move };
    }
  }
  throw new Error(`例データに ${category} の技を覚える種族が無い`);
}

function attackerOf(species: MasterSpecies, item: Item | null = null) {
  return buildIndividual(species, {
    sp: ZERO_SP,
    nature: NEUTRAL_NATURE,
    item,
    ability: defaultAbility(species, master.abilities),
  });
}

function bulkRequestFor(category: "physical" | "special", itemVariants?: Array<Item | null>): BulkRequest {
  const { attacker, defender, move } = matchup(category);
  return buildBulkRequest({
    attacker: attackerOf(attacker),
    defenderSpecies: defender,
    move,
    typeChart: master.typeChart,
    ...(itemVariants === undefined ? {} : { itemVariants }),
  });
}

describe("calcBulk(presetKeys 省略 = engine の既定5行)", () => {
  test.each([
    ["physical", physicalPresetKeys],
    ["special", specialPresetKeys],
  ] as const)("%s の技は既定の5行を返す", async (category, presetKeys) => {
    const request = bulkRequestFor(category);
    const result = await engine.calcBulk(request);
    expect(result).toMatchObject({ ok: true });
    if (!result.ok) {
      return;
    }
    expect(result.value.defenderSpeciesKey).toBe(request.defenderSpecies.key);
    expect(result.value.rows.map((row) => row.preset)).toEqual(presetKeys);
    for (const row of result.value.rows) {
      expect(row.presetLabel.length).toBeGreaterThan(0);
      expect(row.itemId).toBe("");
      expect(row.result.rolls).toHaveLength(rollCount);
      expect(row.result.minPercent).toBeLessThanOrEqual(row.result.maxPercent);
      expect(row.result.category).toBe(category);
    }
  });

  test("持ち物の差し替え候補を渡すと 5 × バリアント数の行になり、null のバリアントは itemId が空", async () => {
    const { move } = matchup("physical");
    const candidates = defensiveItemCandidates(master.items, move);
    expect(candidates.length).toBeGreaterThan(0);
    const variants = [null, ...candidates];
    const result = await engine.calcBulk(bulkRequestFor("physical", variants));
    expect(result).toMatchObject({ ok: true });
    if (!result.ok) {
      return;
    }
    expect(result.value.rows).toHaveLength(physicalPresetKeys.length * variants.length);
    expect(new Set(result.value.rows.map((row) => row.itemId))).toEqual(
      new Set(["", ...candidates.map((item) => item.id)]),
    );
  });

  test("例データのすべての種族・技・持ち物・特性が engine に受理される(DTO の形が契約どおり)", async () => {
    const [first] = master.species;
    if (first === undefined) {
      throw new Error("例データに種族が無い");
    }
    const damagingMoves = master.moves.filter((move) => move.category !== "status");
    const requests: BulkRequest[] = [
      // 各種族を攻撃側(既定の特性つき)と防御側に
      ...master.species.flatMap((species) => {
        const [move] = movesOf(species).filter((candidate) => candidate.category !== "status");
        if (move === undefined) {
          throw new Error(`${species.key} がダメージ技を覚えない`);
        }
        return [
          buildBulkRequest({
            attacker: attackerOf(species),
            defenderSpecies: first,
            move,
            typeChart: master.typeChart,
          }),
          buildBulkRequest({
            attacker: attackerOf(first),
            defenderSpecies: species,
            move,
            typeChart: master.typeChart,
          }),
        ];
      }),
      // すべてのダメージ技
      ...damagingMoves.map((move) =>
        buildBulkRequest({
          attacker: attackerOf(first),
          defenderSpecies: first,
          move,
          typeChart: master.typeChart,
        }),
      ),
      // すべての持ち物を攻撃側に持たせ、防御側のバリアントにも入れる
      ...damagingMoves.slice(0, 1).flatMap((move) =>
        master.items.map((item) =>
          buildBulkRequest({
            attacker: attackerOf(first, item),
            defenderSpecies: first,
            move,
            typeChart: master.typeChart,
            itemVariants: [null, item],
          }),
        ),
      ),
    ];
    const failures: string[] = [];
    for (const request of requests) {
      const result = await engine.calcBulk(request);
      if (!result.ok) {
        failures.push(
          `${request.attacker.species.key} → ${request.defenderSpecies.key} / ${request.move.id}: ${result.error.code} ${result.error.message}`,
        );
      }
    }
    expect(failures).toEqual([]);
  });

  test("typeChart を省くと type_chart_missing(ADR-0011 §13。境界は既定の表を補わない)", async () => {
    // typeChart は readonly なので delete できない。分割代入で typeChart だけ省いた
    // 新しいオブジェクトを作り、境界に typeChart 無しのリクエストとして渡す
    // (捨てる側の変数は使わない。値そのものは要らず、キーを取り除くためだけに分割代入している)。
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const { typeChart: _omittedTypeChart, ...withoutTypeChart } = bulkRequestFor("physical");
    const result = await engine.calcBulk(withoutTypeChart as BulkRequest);
    expect(result).toMatchObject({ ok: false, error: { code: "type_chart_missing" } });
  });
});

describe("calc(1対1)", () => {
  test("buildCalcRequest の組み立てたリクエストが成功し、乱数16個の結果を返す", async () => {
    const { attacker, defender, move } = matchup("special");
    const result = await engine.calc(
      buildCalcRequest({
        attacker: attackerOf(attacker),
        defender: attackerOf(defender),
        move,
        typeChart: master.typeChart,
      }),
    );
    expect(result).toMatchObject({ ok: true });
    if (result.ok) {
      expect(result.value.rolls).toHaveLength(rollCount);
      expect(result.value.minDamage).toBeLessThanOrEqual(result.value.maxDamage);
      expect(result.value.minPercent).toBeLessThanOrEqual(result.value.maxPercent);
      expect(result.value.category).toBe("special");
    }
  });
});
