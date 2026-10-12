// CB-6 段階 B(ADR-0359): 連続技の表示用の純粋関数。数値は engine / API の値をそのまま書式にするだけ(ADR-0300 §8)。

import type { Ability, BulkRow, ComboResult, Individual, Item, Move, Ranks, StatKey } from "../engine/types";
import { comboScreenText } from "../i18n/combo";
import { rankStatNameJa } from "../i18n/common";
import type { MasterSpecies } from "../master/types";
import { abilityNamesLabel } from "./abilityLabels";
import { rankEffectOf } from "./combo";
import { formatPercent } from "./format";
import { buildIndividual } from "./requests";

type RankStatKey = Exclude<StatKey, "hp">;

/** チップの能力の並び(ランクメーターと同じ。攻撃・防御・特攻・特防・素早さ)。 */
const RANK_STAT_ORDER: readonly RankStatKey[] = ["atk", "def", "spa", "spd", "spe"];

/** ランク変化のチップ 1 つ。direction は色(上昇 = brand.primary、下降 = warning。ADR-0357 と同じ意味)。文字でも符号を出す。 */
export interface RankChip {
  readonly text: string;
  readonly direction: "up" | "down";
}

function signed(stage: number): string {
  return stage > 0 ? `+${String(stage)}` : String(stage);
}

/** 技のランク変化をチップにする(変えない技は空)。 */
export function rankChipsOf(move: Move): RankChip[] {
  const effect = rankEffectOf(move);
  if (effect === null) {
    return [];
  }
  return RANK_STAT_ORDER.flatMap((stat) => {
    const stage = effect.stages[stat];
    if (stage === undefined || stage === 0) {
      return [];
    }
    const name =
      effect.target === "self" ? rankStatNameJa[stat] : comboScreenText.opponentRank(rankStatNameJa[stat]);
    return [{ text: `${name} ${signed(stage)}`, direction: stage > 0 ? ("up" as const) : ("down" as const) }];
  });
}

/**
 * 合計で倒す確率の文言。ko.displayChancePercent が 0 は「倒せない」(0・100.0 は不可能・確定に予約。API の説明)。
 * 確定は ko.guaranteed。
 */
export function comboKoText(steps: number, ko: ComboResult["total"]["ko"]): string {
  if (ko.guaranteed) {
    return comboScreenText.koGuaranteed(steps);
  }
  if (ko.displayChancePercent <= 0) {
    return comboScreenText.koCannot;
  }
  return comboScreenText.koChance(steps, `${formatPercent(ko.displayChancePercent)}%`);
}

/**
 * 耐える確率(engine の生値 %)の表記。0 と 100 は不可能・確定に予約し、それ以外は小数第 1 位に丸めて 0.1〜99.9 に収める
 * (KOChance.displayChancePercent と同じ規則。丸めで 100.0 / 0.0 に見えないように)。
 */
export function formatSurviveChance(rawPercent: number): string {
  if (rawPercent >= 100) {
    return "100%";
  }
  if (rawPercent <= 0) {
    return "0%";
  }
  const rounded = Math.round(rawPercent * 10) / 10;
  return `${formatPercent(Math.min(99.9, Math.max(0.1, rounded)))}%`;
}

type SpeedRange = ComboResult["steps"][number]["line"]["selfFirst"];

function isEmptyRange(range: SpeedRange): boolean {
  return range.min > range.max;
}

/** 範囲の言い方。下端が 0(以下)なら「N 以下」、そうでなければ「N 以上」(engine の線は片側が 0 始まり、反対側が上限まで)。 */
function rangeWord(range: SpeedRange): string {
  return range.min <= 0 ? comboScreenText.lineAtMost(range.max) : comboScreenText.lineAtLeast(range.min);
}

/**
 * 相手の素早さが分からないときの線(「197 以下を抜く / 198 は同速 / 199 以上は相手が先」)。
 * トリックルーム中は engine が向きを逆にして返すので、範囲の向きのまま言葉にする。空の範囲は省く。
 */
export function speedLineText(line: ComboResult["steps"][number]["line"]): string {
  const parts = [
    ...(isEmptyRange(line.selfFirst) ? [] : [`${rangeWord(line.selfFirst)}${comboScreenText.lineSelfFirst}`]),
    comboScreenText.lineTie(line.selfSpeed),
    ...(isEmptyRange(line.opponentFirst)
      ? []
      : [`${rangeWord(line.opponentFirst)}${comboScreenText.lineOpponentFirst}`]),
  ];
  return parts.join(comboScreenText.lineSeparator);
}

/** 防御側として選べる「相手の振り」1 つ(計算画面の一括計算の行)。 */
export interface ComboDefenderOption {
  readonly key: string;
  readonly label: string;
  readonly individual: Individual;
}

export interface BuildComboDefendersInput {
  readonly species: MasterSpecies;
  readonly rows: readonly BulkRow[];
  /** 行の itemId を引く先。 */
  readonly items: readonly Item[];
  /** 行の abilityId を引く先(一括計算に渡した防御側の特性の候補)。 */
  readonly candidateAbilities: readonly Ability[];
  /** 名前の解決用の特性一覧。 */
  readonly abilityNames: readonly Ability[];
  readonly hasAbilityChoice: boolean;
  readonly ranks?: Ranks;
}

/** 一括計算の行の識別(同じ preset でも持ち物・特性が違えば別の行。画面の行のキーと同じ考え方)。 */
function rowKey(row: BulkRow, index: number): string {
  return `${row.preset}-${row.itemId}-${row.abilityId ?? ""}-${String(index)}`;
}

/**
 * 計算画面の防御側の行(振り × 持ち物 × 特性)を、連続技の防御側(Individual)の選択肢にする。
 * 連続技のために別の入力欄を作らない(UC-5)。
 */
export function buildComboDefenders(input: BuildComboDefendersInput): ComboDefenderOption[] {
  return input.rows.map((row, index) => {
    const item = input.items.find((candidate) => candidate.id === row.itemId) ?? null;
    const ability = input.candidateAbilities.find((candidate) => candidate.id === row.abilityId) ??
      input.candidateAbilities[0] ?? { id: "", nameJa: "", effect: null };
    const abilityName = abilityNamesLabel(row.abilityIds, input.abilityNames, input.hasAbilityChoice);
    const label = [row.presetLabel, item?.nameJa, abilityName]
      .filter((part) => part !== undefined && part !== null && part !== "")
      .join(" / ");
    return {
      key: rowKey(row, index),
      label,
      individual: buildIndividual(input.species, {
        sp: row.defender.sp,
        nature: row.defender.nature,
        item,
        ability,
        defending: true,
        ...(input.ranks === undefined ? {} : { ranks: input.ranks }),
      }),
    };
  });
}
