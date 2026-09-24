// JD5: judge API のクライアント createJudgeClient(ADR-0705 §3・受け入れ条件1)。
// speed/speedClient.test.ts と同じ形。services/judge/api/openapi.yaml を正とし、リクエスト本文・応答は
// openapi-typescript の生成型(judge.gen.ts)で書く(契約とのずれを typecheck で検出する)。fetch は fake。
// 確かめること:
//   - `${baseUrl}api/judge/v1/outspeed-and-ko` に POST する
//   - ヘッダーは X-Device-Id・X-Session-Id(契約上必須。無ければ judge は 400)と Content-Type: application/json
//   - 本文は OutspeedAndKoRequest のまま送る(省略された欄をクライアントが勝手に補わない。ADR-0705 §6)
//   - 成功は {ok: true, value}(応答をそのまま運ぶ。Web で判定し直さない。ADR-0705 §3)
//   - HTTP エラーで本文が {code, message} なら、その code・message をそのまま運ぶ(422 unknown_move 等)
//   - 通信できない・応答が JSON でない・エラー本文の形が不正なら judge_unavailable(Web 側のコード)
//   - 例外を投げない(失敗はすべて戻り値で返す)
// 架空データだけを使う(実マスタ・実データは使わない。CLAUDE.md ドメイン規約・ADR-0002)。

import { describe, expect, test, vi } from "vitest";
import type { ClientIds } from "../api/clientIds";
import type { components } from "./judge.gen";
import { JUDGE_PATHS, JUDGE_UNAVAILABLE_CODE, createJudgeClient } from "./judgeClient";

type Schemas = components["schemas"];

const ids: ClientIds = {
  deviceId: "11111111-1111-4111-8111-111111111111",
  sessionId: "22222222-2222-4222-a222-222222222222",
};

const BASE_URL = "http://judge.test/";

/** 架空の能力ポイント(各 0..32・合計 <= 66)。 */
const sp: Schemas["StatBlock"] = { hp: 4, atk: 32, def: 0, spa: 0, spd: 0, spe: 30 };

/** 架空の自分の個体(省略可の欄を持たない最小の形)。 */
const attacker: Schemas["Individual"] = {
  speciesKey: "9001-000",
  natureId: "test-nature-plus-spe",
  sp,
};

/** 架空の相手候補(候補は自分の技 moveId を必ず持つ。ADR-0704 §1)。 */
const defender: Schemas["DefenderCandidate"] = {
  speciesKey: "9002-000",
  natureId: "test-nature-neutral",
  sp: { hp: 32, atk: 0, def: 32, spa: 0, spd: 0, spe: 2 },
  moveId: "test-defender-move-0",
};

/** 最小の request(ranks・abilityId・itemId・field・speedField は送らない。ADR-0705 §6)。 */
const minimalRequest: Schemas["OutspeedAndKoRequest"] = {
  format: "single",
  attacker,
  defenders: [defender],
  moveId: "test-move",
};

function ko(hits: number, guaranteed: boolean, displayChancePercent: number): Schemas["KOChance"] {
  return { hits, guaranteed, displayChancePercent };
}

const response: Schemas["OutspeedAndKoResponse"] = {
  matchups: [
    {
      defenderIndex: 0,
      outspeeds: true,
      speedTie: false,
      attackerSpeed: 180,
      defenderSpeed: 120,
      attackerMovePriority: 0,
      defenderMovePriority: 1,
      attackerMovesFirst: false,
      turnOrderTie: false,
      attackerKo: ko(2, true, 100),
      defenderKo: ko(3, false, 42.5),
    },
  ],
};

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function fakeFetch(result: Response | Error) {
  return vi.fn<typeof fetch>(() =>
    result instanceof Error ? Promise.reject(result) : Promise.resolve(result),
  );
}

/** fetch の n 回目の呼び出しの URL・RequestInit を取り出す。 */
function callAt(fetchMock: ReturnType<typeof fakeFetch>, index = 0): { url: string; init: RequestInit } {
  const call = fetchMock.mock.calls[index];
  if (call === undefined) {
    throw new Error(`fetch の ${String(index)} 回目の呼び出しが無い`);
  }
  const [url, init] = call;
  if (typeof url !== "string" || init === undefined) {
    throw new Error("fetch は (文字列の URL, RequestInit) で呼ぶ");
  }
  return { url, init };
}

function headersOf(init: RequestInit): Record<string, string> {
  return Object.fromEntries(new Headers(init.headers).entries());
}

function bodyOf(init: RequestInit): unknown {
  if (typeof init.body !== "string") {
    throw new Error("本文は JSON 文字列");
  }
  return JSON.parse(init.body);
}

describe("パス(services/judge/api/openapi.yaml のまま)", () => {
  test("判定の endpoint は1本だけで、パスは api/judge/v1/outspeed-and-ko", () => {
    // judge は gateway を経由せず自分の Ingress を持つ(ADR-0700 §6-3)。
    expect(JUDGE_PATHS.outspeedAndKo).toBe("api/judge/v1/outspeed-and-ko");
  });
});

describe("outspeedAndKo(判定)", () => {
  test("判定のパスに、端末 ID・セッション ID 付きで OutspeedAndKoRequest を POST する", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, response));
    const client = createJudgeClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });

    const result = await client.outspeedAndKo(minimalRequest);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const { url, init } = callAt(fetchMock);
    expect(url).toBe("http://judge.test/api/judge/v1/outspeed-and-ko");
    expect(init.method).toBe("POST");
    expect(headersOf(init)).toMatchObject({
      "content-type": "application/json",
      "x-device-id": ids.deviceId,
      "x-session-id": ids.sessionId,
    });
    expect(bodyOf(init)).toEqual(minimalRequest);
    // 応答はそのまま運ぶ(Web で判定し直さない・丸めない。ADR-0705 §3・§8)。
    expect(result).toEqual({ ok: true, value: response });
  });

  test("基点 URL が同じオリジン(/)なら /api/judge/... を呼ぶ", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, response));
    const client = createJudgeClient({ baseUrl: "/", fetch: fetchMock, ids });
    await client.outspeedAndKo(minimalRequest);
    expect(callAt(fetchMock).url).toBe("/api/judge/v1/outspeed-and-ko");
  });

  test("省略された欄を勝手に補わない(ranks・abilityId・itemId・field・speedField を送らない)", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, response));
    const client = createJudgeClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });

    await client.outspeedAndKo(minimalRequest);

    const body = bodyOf(callAt(fetchMock).init);
    expect(body).toEqual(minimalRequest);
    expect(Object.keys(body as Record<string, unknown>).sort()).toEqual([
      "attacker",
      "defenders",
      "format",
      "moveId",
    ]);
  });

  test("省略可の欄を入れた request も、そのままの形で送る(6件の候補・場の効果つき)", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, response));
    const client = createJudgeClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });
    const full: Schemas["OutspeedAndKoRequest"] = {
      format: "double",
      attacker: {
        ...attacker,
        ranks: { atk: 1, def: 0, spa: 0, spd: 0, spe: 2 },
        abilityId: "test-ability",
        itemId: "test-item",
      },
      defenders: [
        { ...defender, moveId: "test-defender-move-0" },
        { ...defender, speciesKey: "9003-000", moveId: "test-defender-move-1" },
      ],
      moveId: "test-move",
      speedField: { trickRoom: true, attackerTailwind: false, defenderTailwind: true },
    };

    await client.outspeedAndKo(full);

    expect(bodyOf(callAt(fetchMock).init)).toEqual(full);
  });

  test.each([
    [400, "invalid_request", "defenders[1]: moveId is required"],
    [422, "unknown_species", "defenders[0]: unknown speciesKey"],
    [422, "unknown_move", "attacker: unknown moveId"],
    [422, "unknown_nature", "attacker: unknown natureId"],
    [413, "request_too_large", "request body exceeds 8 KiB"],
    [503, "upstream_unavailable", "upstream is unavailable"],
    [500, "internal_error", "internal error"],
  ] as const)("HTTP %i の {code: %s} をそのまま運ぶ", async (status, code, message) => {
    const client = createJudgeClient({
      baseUrl: BASE_URL,
      fetch: fakeFetch(jsonResponse(status, { code, message } satisfies Schemas["Error"])),
      ids,
    });

    const result = await client.outspeedAndKo(minimalRequest);

    // どの候補で失敗したかは message に入る(ADR-0703 §3)。畳んだり書き換えたりしない。
    expect(result).toEqual({ ok: false, error: { code, message } });
  });
});

describe("通信・応答の失敗は judge_unavailable(自動の切り替えはしない)", () => {
  const cases: ReadonlyArray<[string, () => Response | Error]> = [
    ["fetch が reject する(通信できない)", () => new TypeError("Failed to fetch")],
    [
      "200 だが本文が JSON でない",
      () => new Response("<html>gateway</html>", { status: 200, headers: { "Content-Type": "text/html" } }),
    ],
    [
      "502 で本文が JSON でない",
      () => new Response("Bad Gateway", { status: 502, headers: { "Content-Type": "text/plain" } }),
    ],
    ["500 で本文が {code, message} の形でない", () => jsonResponse(500, { error: "boom" })],
  ];

  test.each(cases)("outspeedAndKo: %s", async (_name, make) => {
    const client = createJudgeClient({ baseUrl: BASE_URL, fetch: fakeFetch(make()), ids });

    const result = await client.outspeedAndKo(minimalRequest);

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.code).toBe(JUDGE_UNAVAILABLE_CODE);
      expect(result.error.message).not.toBe("");
    }
  });

  test("例外を投げない(失敗はすべて戻り値で返す)", async () => {
    const client = createJudgeClient({
      baseUrl: BASE_URL,
      fetch: fakeFetch(new TypeError("Failed to fetch")),
      ids,
    });
    await expect(client.outspeedAndKo(minimalRequest)).resolves.toMatchObject({ ok: false });
  });
});
