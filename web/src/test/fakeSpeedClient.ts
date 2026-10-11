// 素早さ画面のテスト用 fake の SpeedClient と架空データ(services/speed/testdata と同じ 9xxx-xxx の形。実マスタは使わない。ADR-0002)。
// 呼び出しを記録し、テストが応答を返す(BalanceScreen.test.tsx と同じ形)。I-speed-4b。

import type { components } from "../speed/speed.gen";
import type { SpeedClient, SpeedResult } from "../speed/speedClient";

type Schemas = components["schemas"];

export interface PendingCall<A, T> {
  readonly args: A;
  resolve(result: SpeedResult<T>): void;
}

export interface FakeSpeedClient extends SpeedClient {
  readonly pokemonCalls: PendingCall<null, Schemas["PokemonListResponse"]>[];
  readonly matchupCalls: PendingCall<Schemas["MatchupRequest"], Schemas["MatchupResponse"]>[];
  readonly reachCalls: PendingCall<Schemas["PositionRequest"], Schemas["ReachResponse"]>[];
  /** 表・位置は画面から外れた。呼ばれたらここに溜まる(テストが「0 回」を確かめる)。 */
  readonly tableCalls: unknown[];
  readonly positionCalls: unknown[];
}

export function createFakeSpeedClient(): FakeSpeedClient {
  const pokemonCalls: FakeSpeedClient["pokemonCalls"] = [];
  const matchupCalls: FakeSpeedClient["matchupCalls"] = [];
  const reachCalls: FakeSpeedClient["reachCalls"] = [];
  const tableCalls: unknown[] = [];
  const positionCalls: unknown[] = [];
  return {
    pokemonCalls,
    matchupCalls,
    reachCalls,
    tableCalls,
    positionCalls,
    pokemon() {
      return new Promise((resolve) => {
        pokemonCalls.push({ args: null, resolve });
      });
    },
    matchup(request) {
      return new Promise((resolve) => {
        matchupCalls.push({ args: structuredClone(request), resolve });
      });
    },
    reach(request) {
      return new Promise((resolve) => {
        reachCalls.push({ args: structuredClone(request), resolve });
      });
    },
    table() {
      tableCalls.push(true);
      return new Promise(() => undefined);
    },
    position(request) {
      positionCalls.push(request);
      return new Promise(() => undefined);
    },
  };
}

export function lastOf<T>(calls: readonly T[], name: string): T {
  const call = calls.at(-1);
  if (call === undefined) {
    throw new Error(`${name} が呼ばれていない`);
  }
  return call;
}

export const BIRD: Schemas["SpeedPokemon"] = {
  pokemonId: "9001-000",
  nameJa: "テストカソウドリ",
  types: ["fire", "flying"],
  baseSpeed: 100,
};
export const FISH: Schemas["SpeedPokemon"] = {
  pokemonId: "9002-000",
  nameJa: "テストカソウギョ",
  types: ["water"],
  baseSpeed: 80,
};
export const GRASS: Schemas["SpeedPokemon"] = {
  pokemonId: "9003-000",
  nameJa: "テストカソウソウ",
  types: ["grass"],
  baseSpeed: 50,
};

export const pokemonListResponse: Schemas["PokemonListResponse"] = {
  regulationId: "example",
  pokemon: [BIRD, FISH, GRASS],
};

/** 性格 3 件の区間から NatureLine を作る(minus・neutral・plus の順。契約どおり常に 3 件)。 */
export function natures(
  lines: Partial<Record<Schemas["NatureId"], { movesFirst?: [number, number]; tie?: [number, number] }>>,
): Schemas["NatureLine"][] {
  return (["minus", "neutral", "plus"] as const).map((nature) => {
    const line = lines[nature] ?? {};
    return {
      nature,
      ...(line.movesFirst === undefined
        ? {}
        : { movesFirst: { min: line.movesFirst[0], max: line.movesFirst[1] } }),
      ...(line.tie === undefined ? {} : { tie: { min: line.tie[0], max: line.tie[1] } }),
    };
  });
}

/**
 * 通常の場の応答(相手 = FISH〈80 族〉)。
 *  - 無振り(素早さ 100): 補正なしは S0 以上で抜ける(= 無振りで抜ける)
 *  - 準速(132): 補正なしは S20 以上、上昇は S0 以上
 *  - 最速(145): 補正なしは届かない(同速も無し)、上昇は S12 以上で抜け S11 で同速
 *  - 最速スカーフ(217): どの性格でも届かない。上昇の S32 で同速(= 上を取れないが同速だけある)
 */
export const matchupResponse: Schemas["MatchupResponse"] = {
  self: BIRD,
  opponent: FISH,
  trickRoom: false,
  lines: [
    {
      assumption: { preset: "uninvested" },
      opponentSpeed: 100,
      natures: natures({ neutral: { movesFirst: [0, 32] }, plus: { movesFirst: [0, 32] } }),
    },
    {
      assumption: { preset: "neutral-max" },
      opponentSpeed: 132,
      natures: natures({ neutral: { movesFirst: [20, 32] }, plus: { movesFirst: [0, 32] } }),
    },
    {
      assumption: { preset: "max" },
      opponentSpeed: 145,
      natures: natures({ plus: { movesFirst: [12, 32], tie: [11, 11] } }),
    },
    {
      assumption: { preset: "max-scarf" },
      opponentSpeed: 217,
      natures: natures({ plus: { tie: [32, 32] } }),
    },
  ],
};

/** どの性格でも上を取れず、同速もない応答(最速スカーフ 1 件)。 */
export const unreachableMatchupResponse: Schemas["MatchupResponse"] = {
  ...matchupResponse,
  lines: [{ assumption: { preset: "max-scarf" }, opponentSpeed: 400, natures: natures({}) }],
};

/** トリックルーム中の応答(movesFirst は [0, 上限])。 */
export const trickRoomMatchupResponse: Schemas["MatchupResponse"] = {
  ...matchupResponse,
  trickRoom: true,
  lines: [
    {
      assumption: { preset: "max" },
      opponentSpeed: 145,
      natures: natures({
        minus: { movesFirst: [0, 32] },
        neutral: { movesFirst: [0, 28], tie: [29, 29] },
        plus: { movesFirst: [0, 20] },
      }),
    },
  ],
};

/** 「これ以上」: 準速だけ境界の例つき、最速スカーフは「抜かれる族なし」(faster 無し)の形も含む。 */
export const reachResponse: Schemas["ReachResponse"] = {
  speed: 170,
  pokemon: BIRD,
  lines: [
    {
      preset: "uninvested",
      slower: { min: 1, max: 149 },
      tie: { min: 150, max: 150 },
      faster: { min: 151, max: 255 },
      slowerCount: 30,
      tieCount: 1,
      fasterCount: 12,
      fastestSlower: GRASS,
      slowestFaster: FISH,
    },
    {
      preset: "neutral-max",
      slower: { min: 1, max: 117 },
      faster: { min: 118, max: 255 },
      slowerCount: 20,
      tieCount: 0,
      fasterCount: 22,
      fastestSlower: FISH,
      slowestFaster: BIRD,
    },
    {
      preset: "max",
      slower: { min: 1, max: 102 },
      tie: { min: 103, max: 103 },
      faster: { min: 104, max: 255 },
      slowerCount: 40,
      tieCount: 1,
      fasterCount: 2,
      fastestSlower: BIRD,
      slowestFaster: FISH,
    },
    {
      preset: "max-scarf",
      slower: { min: 1, max: 255 },
      slowerCount: 42,
      tieCount: 0,
      fasterCount: 0,
      fastestSlower: BIRD,
    },
    {
      preset: "max-plus1",
      faster: { min: 1, max: 255 },
      slowerCount: 0,
      tieCount: 0,
      fasterCount: 42,
      slowestFaster: GRASS,
    },
    {
      preset: "max-plus2",
      slower: { min: 1, max: 25 },
      tie: { min: 26, max: 26 },
      faster: { min: 27, max: 255 },
      slowerCount: 3,
      tieCount: 1,
      fasterCount: 38,
      fastestSlower: GRASS,
      slowestFaster: FISH,
    },
  ],
};
