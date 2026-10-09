// ADR-0338 §3: 履歴の行を、お気に入りと同じ「計算に使う」の経路(App.openFavoriteInCalc → CalcScreen の restoreRequest。ADR-0333)
// に渡すための純粋な変換 historyEntryAsFavorite(entry, label)。復元の本体 restoreFavoriteCalc は CalcRequest を直接受けるので、
// この変換は Favorite の形に包むだけ(サーバーには送らない・保存しない)。
// 確かめること(AC-6): calc は行の calc そのまま(同じ値)/ individual は calc.attacker / label は渡した文字列 /
//   id は空でない文字列で履歴由来と分かるもの / createdAt・updatedAt は occurredAt / 入力を書き換えない。

import { describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import { historyEntryAsFavorite } from "./calcHistoryFavorite";

type Entry = components["schemas"]["CalcHistoryEntry"];

const SP = { hp: 0, atk: 32, def: 0, spa: 0, spd: 0, spe: 0 } as const;
const entry: Entry = {
  occurredAt: "2026-10-09T03:00:00Z",
  calc: {
    format: "single",
    attacker: { speciesKey: "9001-000", level: 50, natureId: "fake-nature", sp: SP, itemId: "fake-item" },
    defender: { speciesKey: "9002-000", level: 50, natureId: "fake-nature", sp: SP },
    moveId: "fake-move",
    field: { weather: "sun" },
  },
  result: { minPercent: 41.2, maxPercent: 48.9 },
};

describe("historyEntryAsFavorite", () => {
  test("calc は行の calc と同じ値、individual は攻撃側、label は渡した文字列", () => {
    const favorite = historyEntryAsFavorite(entry, "9001-000 → 9002-000");
    expect(favorite.calc).toEqual(entry.calc);
    expect(favorite.individual).toEqual(entry.calc.attacker);
    expect(favorite.label).toBe("9001-000 → 9002-000");
  });

  test("id は空でない文字列、日時は occurredAt", () => {
    const favorite = historyEntryAsFavorite(entry, "x");
    expect(typeof favorite.id).toBe("string");
    expect(favorite.id.length).toBeGreaterThan(0);
    expect(favorite.createdAt).toBe(entry.occurredAt);
    expect(favorite.updatedAt).toBe(entry.occurredAt);
  });

  test("入力の行を書き換えない", () => {
    const copy = structuredClone(entry);
    historyEntryAsFavorite(entry, "x");
    expect(entry).toEqual(copy);
  });
});
