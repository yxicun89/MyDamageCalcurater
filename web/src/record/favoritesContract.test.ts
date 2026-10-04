// P5-3c(ADR-0227): お気に入り API の生成型(api/openapi.gen.ts)の形を固定する契約の網。
// API レーンが契約を先に出し、Web レーンが画面(お気に入りの一覧・ピン留め・外す)を作るときの前提を、
// 型として確かめる。ここが型エラーになったら、契約の変更に画面側を追従させる(make gen-ts の後)。
// 架空の key だけを使う(実データは使わない。ADR-0002)。

import { describe, expect, expectTypeOf, test } from "vitest";
import type { components, paths } from "../api/openapi.gen";

type Schemas = components["schemas"];

describe("お気に入り API の生成型(ADR-0227)", () => {
  test("一覧・作成は /api/record/favorites、削除は /api/record/favorites/{favoriteId}。更新(PUT/PATCH)は無い", () => {
    expectTypeOf<paths["/api/record/favorites"]["get"]>().not.toBeNever();
    expectTypeOf<paths["/api/record/favorites"]["post"]>().not.toBeNever();
    expectTypeOf<paths["/api/record/favorites/{favoriteId}"]["delete"]>().not.toBeNever();
    expectTypeOf<paths["/api/record/favorites/{favoriteId}"]["put"]>().toEqualTypeOf<undefined>();
    expectTypeOf<paths["/api/record/favorites/{favoriteId}"]["patch"]>().toEqualTypeOf<undefined>();
  });

  test("作成の本文は label(任意・null 可)と individual(計算 API の Individual と同じ型)", () => {
    expectTypeOf<Schemas["FavoriteInput"]["individual"]>().toEqualTypeOf<Schemas["Individual"]>();
    expectTypeOf<Schemas["FavoriteInput"]["label"]>().toEqualTypeOf<string | null | undefined>();

    const input: Schemas["FavoriteInput"] = {
      label: "HB特化",
      individual: {
        speciesKey: "9002-000",
        level: 50,
        natureId: "fake-nature",
        sp: { hp: 32, atk: 0, def: 32, spa: 0, spd: 2, spe: 0 },
      },
    };
    expect(input.individual.speciesKey).toBe("9002-000");
  });

  test("保存済みのお気に入りは id(10進の文字列)・label(null 可)・individual・createdAt・updatedAt を必ず持つ", () => {
    expectTypeOf<Schemas["Favorite"]["id"]>().toEqualTypeOf<string>();
    expectTypeOf<Schemas["Favorite"]["label"]>().toEqualTypeOf<string | null>();
    expectTypeOf<Schemas["Favorite"]["individual"]>().toEqualTypeOf<Schemas["Individual"]>();
    expectTypeOf<Schemas["Favorite"]["createdAt"]>().toEqualTypeOf<string>();
    expectTypeOf<Schemas["Favorite"]["updatedAt"]>().toEqualTypeOf<string>();
  });

  test("一覧の 200 は Favorite の配列、作成は 201(新規)と 200(同じ内容の再ピン留め)、削除は 204", () => {
    type List = paths["/api/record/favorites"]["get"]["responses"][200]["content"]["application/json"];
    expectTypeOf<List>().toEqualTypeOf<Schemas["Favorite"][]>();
    type Post = paths["/api/record/favorites"]["post"]["responses"];
    expectTypeOf<Post[201]["content"]["application/json"]>().toEqualTypeOf<Schemas["Favorite"]>();
    expectTypeOf<Post[200]["content"]["application/json"]>().toEqualTypeOf<Schemas["Favorite"]>();
    type Del = paths["/api/record/favorites/{favoriteId}"]["delete"]["responses"];
    expectTypeOf<Del>().toHaveProperty(204);
    expectTypeOf<Del>().toHaveProperty(404);
  });
});

// ADR-0228(usability-round2 F-09): お気に入りに計算の入力全体(CalcRequest)を持たせる。
// 一覧から選んだら calc をそのまま POST /api/calc の本文にする。calc の無い(旧い)お気に入りもあるので省略可。
describe("お気に入りの calc(ADR-0228)", () => {
  test("FavoriteInput.calc と Favorite.calc は CalcRequest そのもの(包む型を挟まない)で省略可", () => {
    expectTypeOf<Schemas["FavoriteInput"]["calc"]>().toEqualTypeOf<Schemas["CalcRequest"] | undefined>();
    expectTypeOf<Schemas["Favorite"]["calc"]>().toEqualTypeOf<Schemas["CalcRequest"] | undefined>();
  });

  test("individual は従来どおり必須(旧クライアントと一覧の表示のため)", () => {
    expectTypeOf<Schemas["FavoriteInput"]["individual"]>().toEqualTypeOf<Schemas["Individual"]>();
    expectTypeOf<Schemas["Favorite"]["individual"]>().toEqualTypeOf<Schemas["Individual"]>();
  });

  test("保存済みの calc はそのまま計算 API の本文として渡せる", () => {
    type CalcBody = NonNullable<paths["/api/calc"]["post"]["requestBody"]>["content"]["application/json"];
    expectTypeOf<NonNullable<Schemas["Favorite"]["calc"]>>().toEqualTypeOf<CalcBody>();

    const attacker: Schemas["Individual"] = {
      speciesKey: "9002-000",
      level: 50,
      natureId: "fake-nature",
      sp: { hp: 0, atk: 32, def: 0, spa: 0, spd: 2, spe: 32 },
    };
    const input: Schemas["FavoriteInput"] = {
      individual: attacker,
      calc: {
        format: "single",
        attacker,
        defender: {
          speciesKey: "9003-000",
          level: 50,
          natureId: "fake-nature",
          sp: { hp: 32, atk: 0, def: 32, spa: 0, spd: 2, spe: 0 },
        },
        moveId: "fake-move",
      },
    };
    expect(input.calc?.moveId).toBe("fake-move");
  });
});
