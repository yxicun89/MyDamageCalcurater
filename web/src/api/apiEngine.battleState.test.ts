// ADR-0144 §3(I-web-14): calc の battleState は POST /api/calc の CalcRequest.battleState にそのまま写す。
// 指定が無ければキーごと送らない(従来と同じ本文)。一括(calcBulk)には付けない。

import { describe, expect, test, vi } from "vitest";
import type { CalcRequest, Individual, Move, Species, TypeChart } from "../engine/types";
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
const move: Move = {
  id: "examplemovetackle",
  nameJa: "テストたいあたり",
  type: "normal",
  category: "physical",
  power: 40,
  priority: 0,
};
const individual: Individual = {
  species,
  level: 50,
  nature: { plus: "", minus: "" },
  ability: { id: "exampleabilityone", nameJa: "テスト一", effect: null },
  item: null,
  sp: { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 },
};
const request: CalcRequest = {
  format: "single",
  attacker: individual,
  defender: individual,
  move,
  typeChart,
};

function engineWith() {
  const fetchMock = vi.fn<typeof fetch>(() => Promise.resolve(new Response("{}", { status: 200 })));
  const engine = createApiEngine({ baseUrl: "http://api.test/", fetch: fetchMock, master: { natures }, ids });
  return { engine, fetchMock };
}

function sentBody(fetchMock: ReturnType<typeof engineWith>["fetchMock"]): Record<string, unknown> {
  const text = fetchMock.mock.calls[0]?.[1]?.body;
  if (typeof text !== "string") {
    throw new Error("本文は JSON 文字列で送る");
  }
  return JSON.parse(text) as Record<string, unknown>;
}

describe("calc: 対戦の状態の送り方", () => {
  test("battleState をそのまま battleState に写す", async () => {
    const { engine, fetchMock } = engineWith();
    await engine.calc({ ...request, battleState: { attackerCurrentHp: 10, defenderCurrentHp: 20, hits: 3 } });
    expect(sentBody(fetchMock).battleState).toEqual({
      attackerCurrentHp: 10,
      defenderCurrentHp: 20,
      hits: 3,
    });
  });

  test("指定が無ければ battleState を送らない(従来と同じ本文)", async () => {
    const { engine, fetchMock } = engineWith();
    await engine.calc(request);
    expect(sentBody(fetchMock)).not.toHaveProperty("battleState");
  });
});
