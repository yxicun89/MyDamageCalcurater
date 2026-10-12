// CB-6(段階 A): 本物の engine.wasm の calcCombo を通す結合テスト(combo-calc.md AC-1 / UC-3)。
// ミミッキュ じゃれつく → かげうち vs ガブリアス(いずれも H2 A32 S32、特性・持ち物なし、場なし)で、
// API の期待値(ゴールデン testdata/golden/combo.jsonl.gz の combo-mimikyu-playrough-shadowsneak-vs-garchomp-jolly)と
// 各手の 16 段階・合計・倒せる確率(132/256)が一致することを確かめる。種族値は公開されている値。
// 加えて、変化技(りゅうのまい相当)の effect を camelCase で渡すとランクが累積すること。
// 前提(web/public/engine.wasm)が無ければスキップせず失敗する(CLAUDE.md、ADR-0300 §8)。
//
// 実行: npm run test:wasm(vitest.wasm.config.ts、node 環境)

import { beforeAll, describe, expect, test } from "vitest";
import { buildComboRequest } from "../domain/combo";
import { buildIndividual, defaultAbility } from "../domain/requests";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import { fileWasmLoader, requireWasmArtifacts } from "../test/fileWasmLoader";
import type { CalcEngine, ComboEngine, Move, Stats } from "./types";
import { createWasmEngine } from "./wasmEngine";

function species(key: string, types: string[], baseStats: Stats): MasterSpecies {
  return { key, dexNo: 9000, form: 0, nameJa: key, types, baseStats, abilities: [], learnset: [] };
}

const mimikyu = species("test-mimikyu", ["ghost", "fairy"], {
  hp: 55,
  atk: 90,
  def: 80,
  spa: 50,
  spd: 105,
  spe: 96,
});
const garchomp = species("test-garchomp", ["dragon", "ground"], {
  hp: 108,
  atk: 130,
  def: 95,
  spa: 80,
  spd: 85,
  spe: 102,
});

const playRough: Move = {
  id: "test-playrough",
  nameJa: "テストじゃれつく",
  type: "fairy",
  category: "physical",
  power: 90,
  priority: 0,
};
const shadowSneak: Move = {
  id: "test-shadowsneak",
  nameJa: "テストかげうち",
  type: "ghost",
  category: "physical",
  power: 40,
  priority: 1,
};
const swordsDance: Move = {
  id: "test-swordsdance",
  nameJa: "テストつるぎのまい",
  type: "normal",
  category: "status",
  power: 0,
  priority: 0,
  effect: { chance: 100, target: "self", stages: { atk: 2 } },
};

const sp = (hp: number, atk: number, spe: number): Stats => ({ hp, atk, def: 0, spa: 0, spd: 0, spe });

let engine: CalcEngine & ComboEngine;
let master: MasterData;

beforeAll(async () => {
  requireWasmArtifacts();
  engine = createWasmEngine(fileWasmLoader());
  master = await exampleMasterSource.load();
});

function request(steps: Parameters<typeof buildComboRequest>[0]["steps"]) {
  const ability = defaultAbility(mimikyu, master.abilities);
  return buildComboRequest({
    attacker: buildIndividual(mimikyu, {
      sp: sp(2, 32, 32),
      nature: { plus: "atk", minus: "spa" },
      item: null,
      ability,
    }),
    defender: buildIndividual(garchomp, {
      sp: sp(2, 32, 32),
      nature: { plus: "spe", minus: "spa" },
      item: null,
      ability,
      defending: true,
    }),
    steps,
    typeChart: master.typeChart,
    opponentSpeed: "from_defender",
  });
}

describe("calcCombo(本物の engine.wasm)", () => {
  test("AC-1: じゃれつく → かげうち vs ガブリアス は 16 段階・合計・倒せる確率が期待値と一致", async () => {
    const result = await engine.calcCombo(request([{ move: playRough }, { move: shadowSneak }]));
    expect(result.ok).toBe(true);
    if (!result.ok) {
      return;
    }
    const [first, second] = result.value.steps;
    expect(first?.damage?.rolls).toEqual([
      138, 140, 140, 144, 144, 146, 150, 150, 152, 152, 156, 156, 158, 158, 162, 164,
    ]);
    expect(second?.damage?.rolls).toEqual([31, 31, 31, 33, 33, 33, 33, 34, 34, 34, 34, 36, 36, 36, 36, 37]);
    expect(result.value.total).toMatchObject({ minDamage: 169, maxDamage: 201, defenderHP: 185 });
    expect(result.value.total.ko.chancePercent).toBeCloseTo((132 / 256) * 100, 6);
    expect(result.value.total.ko.displayChancePercent).toBe(51.6);
    expect(result.value.total.koByStep).toHaveLength(2);
    expect(result.value.opponentAttack).toBeNull();
    // 優先度が違う手は素早さに関係なく決まる(かげうち +1 は自分が先)。じゃれつく(優先度 0)は相手が速い。
    expect(first?.order).toBe("opponent_first");
    expect(second?.order).toBe("self_first");
  });

  test("変化技の effect(camelCase)を渡すと、その手はダメージ null でランクが累積する", async () => {
    const result = await engine.calcCombo(request([{ move: swordsDance }, { move: playRough }]));
    expect(result.ok).toBe(true);
    if (!result.ok) {
      return;
    }
    const [dance, attack] = result.value.steps;
    expect(dance?.damage).toBeNull();
    expect(dance?.selfEndRanks.atk).toBe(2);
    expect(attack?.selfStartRanks.atk).toBe(2);
  });

  test("ランク変化の無い変化技の手は invalid_input(例外にしない)", async () => {
    const noop: Move = { ...playRough, id: "test-noop", category: "status", power: 0 };
    const result = await engine.calcCombo(request([{ move: noop }]));
    expect(result).toMatchObject({ ok: false, error: { code: "invalid_input" } });
  });
});
