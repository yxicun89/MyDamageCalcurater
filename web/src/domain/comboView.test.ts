// CB-6 段階 B: 連続技の表示用の純粋関数(ランク変化チップ・倒す確率・素早さの線・耐える確率・相手の振りの選択肢)。
import { describe, expect, test } from "vitest";
import type { Move } from "../engine/types";
import { comboKoText, formatSurviveChance, rankChipsOf, speedLineText } from "./comboView";

const move = (effect: Move["effect"]): Move => ({
  id: "m",
  nameJa: "m",
  type: "normal",
  category: "status",
  power: 0,
  priority: 0,
  ...(effect === undefined ? {} : { effect }),
});

describe("rankChipsOf", () => {
  test("自分への変化は『攻撃 +1』『素早さ +1』(能力の並びは攻撃・防御・特攻・特防・素早さ)、上昇は up", () => {
    const chips = rankChipsOf(move({ chance: 100, target: "self", stages: { spe: 1, atk: 1 } }));
    expect(chips).toEqual([
      { text: "攻撃 +1", direction: "up" },
      { text: "素早さ +1", direction: "up" },
    ]);
  });

  test("下降は down、相手への変化は『相手の…』", () => {
    expect(rankChipsOf(move({ chance: 100, target: "self", stages: { def: -1, spd: -2 } }))).toEqual([
      { text: "防御 -1", direction: "down" },
      { text: "特防 -2", direction: "down" },
    ]);
    expect(rankChipsOf(move({ chance: 100, target: "target", stages: { spe: -1 } }))).toEqual([
      { text: "相手の素早さ -1", direction: "down" },
    ]);
  });

  test("効果なし・null・段階 0 だけなら空", () => {
    expect(rankChipsOf(move(undefined))).toEqual([]);
    expect(rankChipsOf(move(null))).toEqual([]);
    expect(rankChipsOf(move({ chance: 100, target: "self", stages: { atk: 0 } }))).toEqual([]);
  });
});

describe("comboKoText", () => {
  const ko = (displayChancePercent: number, guaranteed: boolean) => ({
    chancePercent: displayChancePercent,
    guaranteed,
    displayChancePercent,
  });
  test("確率は『N 手で倒す 51.6%』", () => {
    expect(comboKoText(2, ko(51.6, false))).toBe("2 手で倒す 51.6%");
  });
  test("確定は『N 手で倒す(確定)』", () => {
    expect(comboKoText(2, ko(100, true))).toBe("2 手で倒す(確定)");
  });
  test("不可能(表示 0)は『倒せない』", () => {
    expect(comboKoText(3, ko(0, false))).toBe("倒せない");
  });
});

describe("formatSurviveChance", () => {
  test.each([
    [100, "100%"],
    [0, "0%"],
    [51.64, "51.6%"],
    [99.97, "99.9%"],
    [0.01, "0.1%"],
  ])("%s → %s", (raw, text) => {
    expect(formatSurviveChance(raw)).toBe(text);
  });
});

describe("speedLineText", () => {
  test("通常: 『197 以下を抜く / 198 は同速 / 199 以上は相手が先』", () => {
    expect(
      speedLineText({
        selfSpeed: 198,
        selfFirst: { min: 0, max: 197 },
        opponentFirst: { min: 199, max: 999 },
      }),
    ).toBe("197 以下を抜く / 198 は同速 / 199 以上は相手が先");
  });
  test("トリックルーム中は向きが逆: 『199 以上を抜く / 198 は同速 / 197 以下は相手が先』", () => {
    expect(
      speedLineText({
        selfSpeed: 198,
        selfFirst: { min: 199, max: 999 },
        opponentFirst: { min: 0, max: 197 },
      }),
    ).toBe("199 以上を抜く / 198 は同速 / 197 以下は相手が先");
  });
  test("空の範囲(素早さ 0 など)は省く", () => {
    expect(
      speedLineText({
        selfSpeed: 0,
        selfFirst: { min: 0, max: -1 },
        opponentFirst: { min: 1, max: 999 },
      }),
    ).toBe("0 は同速 / 1 以上は相手が先");
  });
});
