// issue #274(ADR-0312): 「詳細」の文言は i18n/ja.ts に置く(ハードコード禁止)。文言と並びは
// docs/ai-shared/DECISIONS.md 2026-09-25「計算条件の入力 UI」(iOS)と一字一句同じ。

import { describe, expect, test } from "vitest";
import { calcConditionsText } from "./ja";

describe("calcConditionsText", () => {
  test("見出し・トグル・小見出し", () => {
    expect(calcConditionsText.toggleLabel).toBe("詳細");
    expect(calcConditionsText.criticalLabel).toBe("急所");
    expect(calcConditionsText.burnLabel).toBe("やけど");
    expect(calcConditionsText.weatherLabel).toBe("天候");
    expect(calcConditionsText.terrainLabel).toBe("フィールド");
    expect(calcConditionsText.screensLabel).toBe("防御側の壁");
    expect(calcConditionsText.ranksLabel).toBe("攻撃側のランク");
  });

  test("天候・フィールド・壁の名前", () => {
    expect(calcConditionsText.weather).toEqual({
      none: "なし",
      sun: "はれ",
      rain: "あめ",
      sand: "すなあらし",
      snow: "ゆき",
    });
    expect(calcConditionsText.terrain).toEqual({
      none: "なし",
      electric: "エレキフィールド",
      grassy: "グラスフィールド",
      psychic: "サイコフィールド",
      misty: "ミストフィールド",
    });
    expect(calcConditionsText.screens).toEqual({
      reflect: "リフレクター",
      lightScreen: "ひかりのかべ",
      auroraVeil: "オーロラベール",
    });
  });

  test("ランクのボタンの名前", () => {
    expect(calcConditionsText.rankUpLabel).toBe("攻撃側のランクを上げる");
    expect(calcConditionsText.rankDownLabel).toBe("攻撃側のランクを下げる");
  });
});
