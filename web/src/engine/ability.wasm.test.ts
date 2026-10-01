// issue 272(ADR-0126・ADR-0311): 防御側・相手側の特性の候補を本物の engine.wasm に通す結合テスト。
// 確かめること(数値の正しさは Go のゴールデン側の役割。ここは境界を通って届くかだけ):
//   - 無効にする特性だけを渡すと全行のダメージが 0(画面が従来「特性を渡さず」常に誤ってダメージを出していた)
//   - 特性を渡さなければ従来どおり(応答に abilityId / abilityIds が出ない)
//   - 効く特性と効かない特性を両方渡すと、結果が分かれて行・候補が分かれる
// 前提(web/public/engine.wasm)が無ければスキップせず失敗する(CLAUDE.md、ADR-0300 §8)。
//
// 実行: make web-test-wasm(vitest.wasm.config.ts、node 環境)

import { beforeAll, describe, expect, test } from "vitest";
import {
  NEUTRAL_NATURE,
  ZERO_SP,
  buildBulkRequest,
  buildIndividual,
  buildReverseRequest,
} from "../domain/requests";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import { fileWasmLoader, requireWasmArtifacts } from "../test/fileWasmLoader";
import type { Ability, CalcEngine, Move } from "./types";
import { createWasmEngine } from "./wasmEngine";

/** 地面技を無効にする架空の特性(ふゆう相当。効果の形は engine の AbilityEffect と同じ)。 */
const immune: Ability = {
  id: "exampleabilityimmune",
  nameJa: "テスト無効",
  effect: { defImmuneTypes: ["ground"] },
};
const plain: Ability = { id: "exampleabilityplain", nameJa: "テスト通常", effect: null };

const groundMove: Move = {
  id: "examplemoveground",
  nameJa: "テスト地面技",
  type: "ground",
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

function species(index: number, abilityIds: readonly string[]): MasterSpecies {
  const base = master.species[index];
  if (base === undefined) {
    throw new Error(`例データに ${String(index)} 番目の種族が無い`);
  }
  // 地面技が等倍で通るよう、防御側のタイプは無効・半減を持たない normal にそろえる。
  return { ...base, types: ["normal"], abilities: abilityIds };
}

function attackerOf() {
  return buildIndividual(species(0, [plain.id]), {
    sp: ZERO_SP,
    nature: NEUTRAL_NATURE,
    item: null,
    ability: plain,
  });
}

function bulk(defenderAbilities?: readonly Ability[]) {
  return engine.calcBulk(
    buildBulkRequest({
      attacker: attackerOf(),
      defenderSpecies: species(1, [plain.id, immune.id]),
      move: groundMove,
      typeChart: master.typeChart,
      ...(defenderAbilities === undefined ? {} : { defenderAbilities }),
    }),
  );
}

describe("calcBulk の defenderAbilities", () => {
  test("無効にする特性だけを渡すと、全行がダメージ 0(従来の「特性なし」の計算とは結果が変わる)", async () => {
    const result = await bulk([immune]);
    expect(result.ok).toBe(true);
    if (!result.ok) {
      return;
    }
    expect(result.value.rows.length).toBeGreaterThan(0);
    for (const row of result.value.rows) {
      expect(row.result.maxDamage).toBe(0);
      expect(row.abilityId).toBe(immune.id);
      expect(row.abilityIds).toEqual([immune.id]);
    }
  });

  test("特性を渡さなければ従来どおり: ダメージが出て、応答に abilityId / abilityIds が無い", async () => {
    const result = await bulk();
    expect(result.ok).toBe(true);
    if (!result.ok) {
      return;
    }
    for (const row of result.value.rows) {
      expect(row.result.maxDamage).toBeGreaterThan(0);
      expect(row).not.toHaveProperty("abilityId");
      expect(row).not.toHaveProperty("abilityIds");
    }
  });

  test("効く特性と効かない特性を両方渡すと、行が特性ごとに分かれる(おまかせの結果)", async () => {
    const result = await bulk([plain, immune]);
    expect(result.ok).toBe(true);
    if (!result.ok) {
      return;
    }
    const byAbility = new Map<string, number>();
    for (const row of result.value.rows) {
      byAbility.set(row.abilityId ?? "", (byAbility.get(row.abilityId ?? "") ?? 0) + 1);
    }
    expect([...byAbility.keys()].sort()).toEqual([immune.id, plain.id].sort());
    // 各特性ともプリセットの数だけ行がある(まとめられていない)
    expect(new Set(byAbility.values()).size).toBe(1);
  });
});

describe("calcReverse の unknownAbilities", () => {
  test("効く特性と効かない特性を両方渡すと、候補に abilityIds が付き特性ごとに分かれる", async () => {
    const result = await engine.calcReverse(
      buildReverseRequest({
        side: "defender",
        known: attackerOf(),
        unknownSpecies: species(1, [plain.id, immune.id]),
        move: groundMove,
        typeChart: master.typeChart,
        itemCandidates: [null],
        observations: [{ percent: 45 }],
        unknownAbilities: [plain, immune],
      }),
    );
    expect(result.ok).toBe(true);
    if (!result.ok) {
      return;
    }
    const abilityIds = new Set(result.value.candidates.map((candidate) => candidate.abilityId));
    expect(abilityIds).toEqual(new Set([plain.id, immune.id]));
    for (const candidate of result.value.candidates) {
      expect(candidate.abilityIds?.[0]).toBe(candidate.abilityId);
    }
  });
});
