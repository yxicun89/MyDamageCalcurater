// CB-6: createApiEngine.calcCombo は POST /api/calc/combo に、技・持ち物・特性を ID に写して送る(ADR-0253)。
// 確かめること: 本文の形(steps は moveId・critical 必須・hits は指定時のみ)、省略項目はキーごと送らない、
// 応答はそのまま返す、エラーは EngineResult で運ぶ(例外を投げない)、性格不明は fetch しない、signal。

import { describe, expect, test, vi } from "vitest";
import type { ComboRequest, ComboResult, Individual, Move, Species, TypeChart } from "../engine/types";
import { REQUEST_ABORTED_CODE } from "../engine/types";
import type { MasterNature } from "../master/types";
import { createApiEngine } from "./apiEngine";

const ids = {
  deviceId: "11111111-1111-4111-8111-111111111111",
  sessionId: "22222222-2222-4222-a222-222222222222",
};
const natures: readonly MasterNature[] = [
  { id: "examplenatureneutral", nameJa: "テストむほせい", plus: null, minus: null },
];
const typeChart: TypeChart = { types: ["normal"], effectiveness: {} };
const species: Species = {
  key: "9002-000",
  dexNo: 9002,
  form: 0,
  nameJa: "テストみず",
  types: ["normal"],
  baseStats: { hp: 90, atk: 75, def: 80, spa: 95, spd: 85, spe: 70 },
  abilities: ["exampleabilityone"],
};
const attack: Move = {
  id: "examplemovetackle",
  nameJa: "テストたいあたり",
  type: "normal",
  category: "physical",
  power: 40,
  priority: 0,
};
const dance: Move = {
  id: "exampledance",
  nameJa: "テストまい",
  type: "normal",
  category: "status",
  power: 0,
  priority: 0,
  effect: { chance: 100, target: "self", stages: { atk: 1, spe: 1 } },
};
const individual: Individual = {
  species,
  level: 50,
  nature: { plus: "", minus: "" },
  ability: { id: "exampleabilityone", nameJa: "テスト一", effect: null },
  item: { id: "exampleitem", nameJa: "テスト持ち物", effect: null },
  sp: { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 },
};
const request: ComboRequest = {
  format: "single",
  attacker: individual,
  defender: { ...individual, item: null },
  steps: [{ move: dance }, { move: attack, critical: true, hits: 2 }],
  typeChart,
};

const ranks = { atk: 0, def: 0, spa: 0, spd: 0, spe: 0 };
const calcResult = {
  rolls: [1],
  minDamage: 1,
  maxDamage: 1,
  minPercent: 1,
  maxPercent: 1,
  defenderHP: 100,
  effectiveness: 1,
  stab: false,
  category: "physical",
  ko: { hits: 2, guaranteed: false, chancePercent: 0, displayChancePercent: 0 },
  unsupported: [],
};
const koChance = { chancePercent: 0, guaranteed: false, displayChancePercent: 0 };
const comboResult: ComboResult = {
  steps: [
    {
      moveId: "exampledance",
      selfStartRanks: ranks,
      selfEndRanks: { ...ranks, atk: 1, spe: 1 },
      opponentStartRanks: ranks,
      opponentEndRanks: ranks,
      damage: null,
      selfSpeed: 100,
      opponentSpeed: 0,
      selfPriority: 0,
      opponentPriority: 0,
      order: "depends_on_opponent_speed",
      line: {
        selfSpeed: 100,
        selfFirst: { min: 0, max: 99 },
        opponentFirst: { min: 101, max: 999 },
      },
    },
    {
      moveId: "examplemovetackle",
      selfStartRanks: { ...ranks, atk: 1, spe: 1 },
      selfEndRanks: { ...ranks, atk: 1, spe: 1 },
      opponentStartRanks: ranks,
      opponentEndRanks: ranks,
      damage: calcResult,
      selfSpeed: 150,
      opponentSpeed: 0,
      selfPriority: 0,
      opponentPriority: 0,
      order: "depends_on_opponent_speed",
      line: {
        selfSpeed: 150,
        selfFirst: { min: 0, max: 149 },
        opponentFirst: { min: 151, max: 999 },
      },
    },
  ],
  total: {
    minDamage: 0,
    maxDamage: 0,
    minPercent: 0,
    maxPercent: 0,
    defenderHP: 100,
    ko: koChance,
    koByStep: [koChance, koChance],
  },
  speed: { firstSelfSpeed: 100, lastSelfSpeed: 150, opponentSpeed: 0, opponentSpeedKnown: false },
  opponentAttack: null,
  unsupported: [],
} as unknown as ComboResult;

function engineWith(respond: () => Promise<Response>) {
  const fetchMock = vi.fn<typeof fetch>(respond);
  const engine = createApiEngine({ baseUrl: "http://api.test/", fetch: fetchMock, master: { natures }, ids });
  return { engine, fetchMock };
}

function ok(body: unknown): () => Promise<Response> {
  return () => Promise.resolve(new Response(JSON.stringify(body), { status: 200 }));
}

function sentBody(fetchMock: ReturnType<typeof engineWith>["fetchMock"]): Record<string, unknown> {
  const text = fetchMock.mock.calls[0]?.[1]?.body;
  if (typeof text !== "string") {
    throw new Error("本文は JSON 文字列で送る");
  }
  return JSON.parse(text) as Record<string, unknown>;
}

describe("calcCombo: リクエスト", () => {
  test("POST /api/calc/combo に、技を moveId にして端末 ID・セッション ID を付けて送る", async () => {
    const { engine, fetchMock } = engineWith(ok(comboResult));
    await engine.calcCombo(request);
    const [url, init] = fetchMock.mock.calls[0] ?? [];
    expect(url).toBe("http://api.test/api/calc/combo");
    expect(init?.method).toBe("POST");
    expect(init?.headers).toMatchObject({ "X-Device-Id": ids.deviceId, "X-Session-Id": ids.sessionId });
    const body = sentBody(fetchMock);
    expect(body).toMatchObject({
      format: "single",
      attacker: { speciesKey: "9002-000", natureId: "examplenatureneutral", itemId: "exampleitem" },
      defender: { speciesKey: "9002-000", itemId: null },
      steps: [
        { moveId: "exampledance", critical: false },
        { moveId: "examplemovetackle", critical: true, hits: 2 },
      ],
    });
    // 解決済みの実体(技・種族・相性表)は送らない。
    expect(body).not.toHaveProperty("typeChart");
    expect(JSON.stringify(body)).not.toContain("effect");
  });

  test("省略した項目(field・相手の技・相手の素早さ・素早さの場)はキーごと送らない", async () => {
    const { engine, fetchMock } = engineWith(ok(comboResult));
    await engine.calcCombo(request);
    const body = sentBody(fetchMock);
    for (const key of ["field", "opponentMoveId", "opponentSpeed", "speedField"]) {
      expect(body).not.toHaveProperty(key);
    }
  });

  test("相手の技は opponentMoveId、素早さの場は 3 項目を false で埋めて送る", async () => {
    const { engine, fetchMock } = engineWith(ok(comboResult));
    await engine.calcCombo({
      ...request,
      field: { weather: "sun" },
      opponentMove: attack,
      opponentSpeed: "from_defender",
      speedField: { selfTailwind: true },
    });
    expect(sentBody(fetchMock)).toMatchObject({
      field: { weather: "sun" },
      opponentMoveId: "examplemovetackle",
      opponentSpeed: "from_defender",
      speedField: { trickRoom: false, selfTailwind: true, opponentTailwind: false },
    });
  });

  test("性格が解決できなければ fetch せず unknown_nature", async () => {
    const { engine, fetchMock } = engineWith(ok(comboResult));
    const result = await engine.calcCombo({
      ...request,
      attacker: { ...individual, nature: { plus: "atk", minus: "def" } },
    });
    expect(result).toMatchObject({ ok: false, error: { code: "unknown_nature" } });
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

describe("calcCombo: 応答", () => {
  test("応答はそのまま ok の value になる", async () => {
    const { engine } = engineWith(ok(comboResult));
    expect(await engine.calcCombo(request)).toEqual({ ok: true, value: comboResult });
  });

  test("サーバーのエラー本文は code・message のまま not ok(例外を投げない)", async () => {
    const { engine } = engineWith(() =>
      Promise.resolve(
        new Response(JSON.stringify({ code: "invalid_input", message: "steps が範囲外" }), { status: 400 }),
      ),
    );
    expect(await engine.calcCombo(request)).toEqual({
      ok: false,
      error: { code: "invalid_input", message: "steps が範囲外" },
    });
  });

  test("通信失敗・契約外の本文は engine_unavailable", async () => {
    const down = engineWith(() => Promise.reject(new Error("network")));
    expect(await down.engine.calcCombo(request)).toMatchObject({
      ok: false,
      error: { code: "engine_unavailable" },
    });
    const broken = engineWith(ok({ steps: "x" }));
    expect(await broken.engine.calcCombo(request)).toMatchObject({
      ok: false,
      error: { code: "engine_unavailable" },
    });
  });

  test("abort 済みの signal は fetch せず request_aborted", async () => {
    const { engine, fetchMock } = engineWith(ok(comboResult));
    const controller = new AbortController();
    controller.abort();
    expect(await engine.calcCombo(request, controller.signal)).toMatchObject({
      ok: false,
      error: { code: REQUEST_ABORTED_CODE },
    });
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
