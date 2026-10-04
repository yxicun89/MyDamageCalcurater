// ADR-0229(usability-round2 F-08「構築名はいらない」): 構築名は入力で省略でき、サーバーが既定名を補う。
// 応答の Team.name は従来どおり必ずある文字列(画面が名前の入力欄を廃止しても、既存の表示を壊さない)。
// ここが型エラーになったら、契約の変更に画面側を追従させる(make gen-ts の後)。架空の key だけを使う(ADR-0002)。

import { describe, expect, expectTypeOf, test } from "vitest";
import type { components } from "../api/openapi.gen";

type Schemas = components["schemas"];

describe("構築名の省略(ADR-0229)", () => {
  test("TeamInput.name は省略・null を取れる", () => {
    expectTypeOf<Schemas["TeamInput"]["name"]>().toEqualTypeOf<string | null | undefined>();
    const noName: Schemas["TeamInput"] = { members: [] };
    expect(noName.name).toBeUndefined();
  });

  test("Team.name は必ずある string のまま(応答の型は変えない)", () => {
    expectTypeOf<Schemas["Team"]["name"]>().toEqualTypeOf<string>();
  });
});
