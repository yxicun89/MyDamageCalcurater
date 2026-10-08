// ADR-0230: 計算履歴 API(GET /api/record/calc-history)の生成型(api/openapi.gen.ts)の形を固定する契約の網。
// API レーンが契約を先に出し、Web レーンが履歴の画面(一覧・続きを読む・行から計算を出し直す)を作るときの前提を、
// 型として確かめる。ここが型エラーになったら、契約の変更に画面側を追従させる(make gen-ts の後)。
// 架空の key だけを使う(実データは使わない。ADR-0002)。

import { describe, expect, expectTypeOf, test } from "vitest";
import type { components, paths } from "../api/openapi.gen";

type Schemas = components["schemas"];
type HistoryGet = paths["/api/record/calc-history"]["get"];

describe("計算履歴 API の生成型(ADR-0230)", () => {
  test("経路は GET /api/record/calc-history だけ(個別の削除・更新・作成は無い)", () => {
    expectTypeOf<HistoryGet>().not.toBeNever();
    expectTypeOf<paths["/api/record/calc-history"]["delete"]>().toEqualTypeOf<undefined>();
    expectTypeOf<paths["/api/record/calc-history"]["post"]>().toEqualTypeOf<undefined>();
    expectTypeOf<paths["/api/record/calc-history"]["put"]>().toEqualTypeOf<undefined>();
  });

  test("クエリは limit(任意の数値)と cursor(任意の文字列)だけ。端末 ID はヘッダで送る", () => {
    type Query = NonNullable<HistoryGet["parameters"]["query"]>;
    expectTypeOf<Query["limit"]>().toEqualTypeOf<number | undefined>();
    expectTypeOf<Query["cursor"]>().toEqualTypeOf<Schemas["CalcHistoryCursor"] | undefined>();
    expectTypeOf<keyof Query>().toEqualTypeOf<"limit" | "cursor">();
    expectTypeOf<HistoryGet["parameters"]["header"]>().toHaveProperty("X-Device-Id");
    expectTypeOf<HistoryGet["parameters"]["header"]>().toHaveProperty("X-Session-Id");
  });

  test("200 は CalcHistoryPage(items と nextCursor〈null 可〉を必ず持つ)。400・503 は Error", () => {
    type Ok = HistoryGet["responses"][200]["content"]["application/json"];
    expectTypeOf<Ok>().toEqualTypeOf<Schemas["CalcHistoryPage"]>();
    expectTypeOf<Schemas["CalcHistoryPage"]["items"]>().toEqualTypeOf<Schemas["CalcHistoryEntry"][]>();
    expectTypeOf<Schemas["CalcHistoryPage"]["nextCursor"]>().toEqualTypeOf<string | null>();
    expectTypeOf<HistoryGet["responses"][400]["content"]["application/json"]>().toEqualTypeOf<
      Schemas["Error"]
    >();
    expectTypeOf<HistoryGet["responses"][503]["content"]["application/json"]>().toEqualTypeOf<
      Schemas["Error"]
    >();
  });

  test("1行は occurredAt・calc・result の3つだけ(端末 ID・セッション ID・行の ID は無い)", () => {
    expectTypeOf<keyof Schemas["CalcHistoryEntry"]>().toEqualTypeOf<"occurredAt" | "calc" | "result">();
    expectTypeOf<Schemas["CalcHistoryEntry"]["occurredAt"]>().toEqualTypeOf<string>();
    expectTypeOf<keyof Schemas["CalcHistoryResult"]>().toEqualTypeOf<"minPercent" | "maxPercent">();
    expectTypeOf<Schemas["CalcHistoryResult"]["minPercent"]>().toEqualTypeOf<number>();
    expectTypeOf<Schemas["CalcHistoryResult"]["maxPercent"]>().toEqualTypeOf<number>();
  });

  test("行の calc は CalcRequest そのもので、そのまま POST /api/calc の本文にできる(お気に入りの calc と同じ型)", () => {
    expectTypeOf<Schemas["CalcHistoryEntry"]["calc"]>().toEqualTypeOf<Schemas["CalcRequest"]>();
    type CalcBody = NonNullable<paths["/api/calc"]["post"]["requestBody"]>["content"]["application/json"];
    expectTypeOf<Schemas["CalcHistoryEntry"]["calc"]>().toEqualTypeOf<CalcBody>();
    expectTypeOf<Schemas["CalcHistoryEntry"]["calc"]>().toEqualTypeOf<
      NonNullable<Schemas["Favorite"]["calc"]>
    >();

    const page: Schemas["CalcHistoryPage"] = {
      items: [
        {
          occurredAt: "2026-10-09T00:00:00Z",
          calc: {
            format: "single",
            attacker: {
              speciesKey: "9002-000",
              level: 50,
              natureId: "fake-nature",
              sp: { hp: 0, atk: 32, def: 0, spa: 0, spd: 2, spe: 32 },
            },
            defender: {
              speciesKey: "9003-000",
              level: 50,
              natureId: "fake-nature",
              sp: { hp: 32, atk: 0, def: 32, spa: 0, spd: 2, spe: 0 },
            },
            moveId: "fake-move",
          },
          result: { minPercent: 41.2, maxPercent: 48.9 },
        },
      ],
      nextCursor: null,
    };
    const body: CalcBody | undefined = page.items[0]?.calc;
    expect(body?.moveId).toBe("fake-move");
    expect(page.nextCursor).toBeNull();
  });
});
