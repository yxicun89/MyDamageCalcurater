// I-web-1(ADR-0329): 計算画面の「攻撃」「特攻」の入力(SP の数値・性格補正)から domain/attackerStatInputs.ts が
// 組み立てた攻撃側を本物の engine.wasm に通し、受理されることと「強くなる向き」を確かめる。
// 数値の正しさ(4096 基準・五捨五超入の丸め)はゴールデンと Go/WASM 一致テストの役割なので見ない。Web は値を渡すだけ。
// 前提(web/public/engine.wasm・wasm_exec.js)が無ければスキップせず失敗する(CLAUDE.md、ADR-0300 §8)。
//
// 実行: make web-test-wasm(vitest.wasm.config.ts、node 環境)

import { beforeAll, describe, expect, test } from "vitest";
import {
  DEFAULT_ATTACKER_STAT_INPUTS,
  resolveAttackerStats,
  type AttackStatInput,
  type AttackerStatInputs,
} from "../domain/attackerStatInputs";
import { buildBulkRequest, buildIndividual, defaultAbility } from "../domain/requests";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import { fileWasmLoader, requireWasmArtifacts } from "../test/fileWasmLoader";
import type { CalcEngine, Move } from "./types";
import { createWasmEngine } from "./wasmEngine";

let engine: CalcEngine;
let master: MasterData;

beforeAll(async () => {
  requireWasmArtifacts();
  engine = createWasmEngine(fileWasmLoader());
  master = await exampleMasterSource.load();
});

function matchup(category: "physical" | "special"): {
  attacker: MasterSpecies;
  defender: MasterSpecies;
  move: Move;
} {
  for (const attacker of master.species) {
    const move = attacker.learnset
      .flatMap((id) => master.moves.filter((candidate) => candidate.id === id))
      .find((candidate) => candidate.category === category && candidate.power > 0);
    const defender = master.species.find((species) => species.key !== attacker.key);
    if (move !== undefined && defender !== undefined) {
      return { attacker, defender, move };
    }
  }
  throw new Error(`例データに ${category} の技を覚える種族が無い`);
}

const STAT_OF = { physical: "atk", special: "spa" } as const;

/** 技が使う側のブロックだけを上書きした入力。 */
function inputsFor(category: "physical" | "special", block: Partial<AttackStatInput>): AttackerStatInputs {
  const stat = STAT_OF[category];
  return { ...DEFAULT_ATTACKER_STAT_INPUTS, [stat]: { ...DEFAULT_ATTACKER_STAT_INPUTS[stat], ...block } };
}

/** 入力から組み立てた要求を engine.wasm に通し、1行目(防御側無振り)の最大ダメージを返す。 */
async function firstRowMaxDamage(
  category: "physical" | "special",
  inputs: AttackerStatInputs,
): Promise<number> {
  const { attacker, defender, move } = matchup(category);
  const stats = resolveAttackerStats(inputs, master.natures, move.category);
  if (!stats.ok) {
    throw new Error(`入力が解決できない: ${JSON.stringify(stats.issues)}`);
  }
  const result = await engine.calcBulk(
    buildBulkRequest({
      attacker: buildIndividual(attacker, {
        sp: stats.sp,
        nature: stats.nature,
        item: null,
        ability: defaultAbility(attacker, master.abilities),
      }),
      defenderSpecies: defender,
      move,
      typeChart: master.typeChart,
    }),
  );
  if (!result.ok) {
    throw new Error(`engine が失敗: ${result.error.code} ${result.error.message}`);
  }
  const first = result.value.rows[0];
  if (first === undefined) {
    throw new Error("行が無い");
  }
  return first.result.maxDamage;
}

describe.each(["physical", "special"] as const)("%s: 攻撃側の SP・性格補正の入力(I-web-1)", (category) => {
  test("SP の数値入力は受理され、増やすとダメージが増える向き(0 < 16 < 32)", async () => {
    const [zero, half, full] = await Promise.all(
      ["0", "16", "32"].map((spText) => firstRowMaxDamage(category, inputsFor(category, { spText }))),
    );
    if (zero === undefined || half === undefined || full === undefined) {
      throw new Error("結果が足りない");
    }
    expect(half).toBeGreaterThanOrEqual(zero);
    expect(full).toBeGreaterThanOrEqual(half);
    expect(full).toBeGreaterThan(zero);
  });

  test("性格補正: 上昇で増え、下降で減る(SP 32 で比べる)", async () => {
    const [down, neutral, up] = await Promise.all(
      (["down", "neutral", "up"] as const).map((modifier) =>
        firstRowMaxDamage(category, inputsFor(category, { spText: "32", modifier })),
      ),
    );
    if (down === undefined || neutral === undefined || up === undefined) {
      throw new Error("結果が足りない");
    }
    expect(up).toBeGreaterThan(neutral);
    expect(down).toBeLessThan(neutral);
  });

  test("技が使わない側の SP・補正を変えてもダメージは変わらない", async () => {
    const other = category === "physical" ? "spa" : "atk";
    const base = await firstRowMaxDamage(category, DEFAULT_ATTACKER_STAT_INPUTS);
    const changed = await firstRowMaxDamage(category, {
      ...DEFAULT_ATTACKER_STAT_INPUTS,
      [other]: { spText: "32", modifier: "down" },
    });
    expect(changed).toBe(base);
  });
});
