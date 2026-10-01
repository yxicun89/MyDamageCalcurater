// P4-4: 逆算の WASM 結合テスト(ADR-0300 §7・§8、ADR-0010 §R、ADR-0011 §3 calcReverse)。
// Web が組み立てた逆算リクエスト(buildReverseRequest・reverseItemCandidates)を本物の engine.wasm に通し、
// 結果の形(候補数 = 2 × 持ち物候補、SP の範囲は 0〜32 の昇順・互いに素な区間、性格クラス)と、
// 観測を足すと一致候補が絞られること(requirements.md「同じ相手の観測を複数入力すると候補を絞り込める」)を確かめる。
// 観測は架空の値ではなく、既知の調整を calc に通して得た本物のダメージから作る(整数%は切り捨て。
// engine の観測は丸め規則に依存しない区間で照合する。ADR-0010 §R2)。
// 前提(web/public/engine.wasm・wasm_exec.js)が無ければスキップせず失敗する(CLAUDE.md、ADR-0300 §8)。
//
// 実行: make web-test-wasm(vitest.wasm.config.ts、node 環境)

import { beforeAll, describe, expect, test } from "vitest";
import {
  BATTLE_LEVEL,
  NEUTRAL_NATURE,
  NO_ABILITY,
  ZERO_SP,
  buildCalcRequest,
  buildIndividual,
  buildReverseRequest,
  defaultAbility,
  toEngineSpecies,
} from "../domain/requests";
import { learnsetMoves } from "../domain/moves";
import { reverseItemCandidates } from "../domain/reverseItems";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import { fileWasmLoader, requireWasmArtifacts } from "../test/fileWasmLoader";
import type {
  CalcEngine,
  CalcResult,
  Individual,
  Move,
  Observation,
  ReverseRequest,
  ReverseResult,
  ReverseSide,
  Stats,
} from "./types";
import { createWasmEngine } from "./wasmEngine";

/** 逆算で探索する性格クラス(engine/reverse.go の NatureClass。下降補正は探索しない。ADR-0010 §R1)。 */
const natureClasses = ["neutral", "plus"];
/** 1ステータスの SP の上限(CLAUDE.md ドメイン規約)。 */
const maxSp = 32;
/** 防御側の逆算が仮定する H の SP(ADR-0010 §R1)。 */
const assumedDefenderHpSp = 32;
/** 真の調整(観測を作るのに使う)の関連ステータスの SP。0 と 32 の間にして、範囲の端に張り付かないようにする。 */
const trueSp = 16;

let engine: CalcEngine;
let master: MasterData;

beforeAll(async () => {
  requireWasmArtifacts();
  engine = createWasmEngine(fileWasmLoader());
  master = await exampleMasterSource.load();
});

/** 種族が覚える、指定の分類でいちばん威力の高い技(観測の%が 1 以上になるように)。 */
function strongestMove(species: MasterSpecies, category: "physical" | "special"): Move {
  const moves = learnsetMoves(species, master.moves).filter((move) => move.category === category);
  const [best] = [...moves].sort((a, b) => b.power - a.power);
  if (best === undefined) {
    throw new Error(`${species.key} は ${category} の技を覚えない`);
  }
  return best;
}

/** 分類の技を覚える種族と、その技・別の相手の組。 */
function pairFor(category: "physical" | "special"): {
  user: MasterSpecies;
  target: MasterSpecies;
  move: Move;
} {
  for (const user of master.species) {
    const hasMove = learnsetMoves(user, master.moves).some((move) => move.category === category);
    const target = master.species.find((species) => species.key !== user.key);
    if (hasMove && target !== undefined) {
      return { user, target, move: strongestMove(user, category) };
    }
  }
  throw new Error(`例データに ${category} の技を覚える種族が無い`);
}

function individual(species: MasterSpecies, sp: Stats = ZERO_SP): Individual {
  return buildIndividual(species, {
    sp,
    nature: NEUTRAL_NATURE,
    item: null,
    ability: defaultAbility(species, master.abilities),
  });
}

/** 逆算で推定する側の「真の個体」(推定側は特性を探索しないので、特性なしで観測を作る)。 */
function unknownTruth(species: MasterSpecies, sp: Stats): Individual {
  return {
    species: toEngineSpecies(species),
    level: BATTLE_LEVEL,
    nature: NEUTRAL_NATURE,
    ability: NO_ABILITY,
    item: null,
    sp,
  };
}

async function calcOrThrow(attacker: Individual, defender: Individual, move: Move): Promise<CalcResult> {
  const result = await engine.calc(
    buildCalcRequest({ attacker, defender, move, typeChart: master.typeChart }),
  );
  if (!result.ok) {
    throw new Error(`calc が失敗: ${result.error.code} ${result.error.message}`);
  }
  return result.value;
}

function rollAt(result: CalcResult, index: number): number {
  const roll = result.rolls[index];
  if (roll === undefined) {
    throw new Error(`乱数 ${index} 番目が無い`);
  }
  return roll;
}

/** 実点数から整数%の観測(切り捨て)を作る。1 未満にはしない前提を確かめる。 */
function percentObservation(damage: number, maxHp: number): Observation {
  const percent = Math.floor((damage * 100) / maxHp);
  if (percent < 1 || percent > 100) {
    throw new Error(`観測%が 1〜100 に入らない: ${percent}(damage ${damage} / HP ${maxHp})`);
  }
  return { percent };
}

async function reverseOrThrow(request: ReverseRequest): Promise<ReverseResult> {
  const result = await engine.calcReverse(request);
  if (!result.ok) {
    throw new Error(`calcReverse が失敗: ${result.error.code} ${result.error.message}`);
  }
  return result.value;
}

function expectWellFormed(result: ReverseResult, side: ReverseSide, itemCandidateCount: number): void {
  expect(result.side).toBe(side);
  expect(result.candidates).toHaveLength(natureClasses.length * itemCandidateCount);
  expect(result.exactCount).toBe(result.candidates.filter((candidate) => candidate.exact).length);
  for (const candidate of result.candidates) {
    expect(natureClasses).toContain(candidate.natureClass);
    expect(candidate.ranges.length).toBeGreaterThan(0);
    let previousMax = -2;
    let count = 0;
    for (const range of candidate.ranges) {
      expect(range.min).toBeGreaterThanOrEqual(0);
      expect(range.max).toBeLessThanOrEqual(maxSp);
      expect(range.min).toBeLessThanOrEqual(range.max);
      // 昇順・互いに素・隣接しない極大区間(ADR-0010 §R3)
      expect(range.min).toBeGreaterThan(previousMax + 1);
      previousMax = range.max;
      count += range.max - range.min + 1;
    }
    expect(candidate.spCount).toBe(count);
    expect(candidate.minPercent).toBeLessThanOrEqual(candidate.maxPercent);
    // issue 271 / issue 270(ADR-0123 §6): 候補は常に unsupported を持つ(印なしは空配列)。
    // 例データの技には機構が無いので、ここでは空配列であること = 境界を通って DTO に届くことを見る。
    expect(candidate.unsupported).toEqual([]);
  }
}

function exactSpTotal(result: ReverseResult): number {
  return result.candidates
    .filter((candidate) => candidate.exact)
    .reduce((sum, candidate) => sum + candidate.spCount, 0);
}

function containsSp(result: ReverseResult, natureClass: string, itemId: string, sp: number): boolean {
  const candidate = result.candidates.find(
    (entry) => entry.natureClass === natureClass && entry.itemId === itemId,
  );
  return candidate?.ranges.some((range) => range.min <= sp && sp <= range.max) ?? false;
}

describe("calcReverse(与えたダメージ = side defender)", () => {
  async function setup(): Promise<{
    request: (observations: Observation[]) => ReverseRequest;
    observations: Observation[];
    itemCandidateCount: number;
  }> {
    const { user: mine, target: theirs, move } = pairFor("physical");
    const attacker = individual(mine);
    const truth = unknownTruth(theirs, { ...ZERO_SP, hp: assumedDefenderHpSp, def: trueSp });
    const calc = await calcOrThrow(attacker, truth, move);
    const observations = [
      percentObservation(rollAt(calc, 0), calc.defenderHP),
      percentObservation(rollAt(calc, calc.rolls.length - 1), calc.defenderHP),
    ];
    const { candidates: itemCandidates } = reverseItemCandidates("defender", master.items, move);
    return {
      request: (obs) =>
        buildReverseRequest({
          side: "defender",
          known: attacker,
          unknownSpecies: theirs,
          move,
          typeChart: master.typeChart,
          itemCandidates,
          observations: obs,
        }),
      observations,
      itemCandidateCount: itemCandidates.length,
    };
  }

  test("観測1件: 成功し、2 × 持ち物候補の候補、H32 前提、真の調整(補正なし・持ち物なし・B16)を含む", async () => {
    const { request, observations, itemCandidateCount } = await setup();
    const result = await reverseOrThrow(request(observations.slice(0, 1)));
    expectWellFormed(result, "defender", itemCandidateCount);
    expect(result.stat).toBe("def");
    expect(result.assumedHpSp).toBe(assumedDefenderHpSp);
    expect(result.exactCount).toBeGreaterThan(0);
    expect(containsSp(result, "neutral", "", trueSp)).toBe(true);
  });

  test("観測を2件にすると成功し、一致候補の SP の総数は増えない(絞り込み)", async () => {
    const { request, observations, itemCandidateCount } = await setup();
    const one = await reverseOrThrow(request(observations.slice(0, 1)));
    const two = await reverseOrThrow(request(observations));
    expectWellFormed(two, "defender", itemCandidateCount);
    expect(two.exactCount).toBeGreaterThan(0);
    expect(containsSp(two, "neutral", "", trueSp)).toBe(true);
    expect(exactSpTotal(two)).toBeLessThanOrEqual(exactSpTotal(one));
  });
});

describe("calcReverse(受けたダメージ = side attacker)", () => {
  test("HP の実点数の観測で成功し、攻撃側の関連ステータスを返す(H の仮定なし)", async () => {
    const { user: theirs, target: mine, move } = pairFor("special");
    const defender = individual(mine);
    const truth = unknownTruth(theirs, { ...ZERO_SP, spa: trueSp });
    const calc = await calcOrThrow(truth, defender, move);
    const { candidates: itemCandidates } = reverseItemCandidates("attacker", master.items, move);
    const result = await reverseOrThrow(
      buildReverseRequest({
        side: "attacker",
        known: defender,
        unknownSpecies: theirs,
        move,
        typeChart: master.typeChart,
        itemCandidates,
        observations: [{ damage: rollAt(calc, 7) }],
      }),
    );
    expectWellFormed(result, "attacker", itemCandidates.length);
    expect(result.stat).toBe("spa");
    expect(result.assumedHpSp).toBe(0);
    expect(result.exactCount).toBeGreaterThan(0);
    expect(containsSp(result, "neutral", "", trueSp)).toBe(true);
  });
});
