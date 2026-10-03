// 判定の画面の1体分の入力の状態と、その更新(種族・技の選択、調整プリセット)。純粋関数だけで、React を使わない。
// issue 309(ADR-0711): 技は種族の learnset から選び、調整はプリセット(SP と性格がまとめて入る)で入れる。

import { attackerPresetLabel, resolveAttackerPreset } from "../domain/attackerPresets";
import { defenderPresetLabel, resolveDefenderPreset } from "../domain/defenderPresets";
import { firstDamagingMove, learnsetMoves } from "../domain/moves";
import { MAX_SP_PER_STAT, ZERO_SP } from "../domain/requests";
import type { Move, MoveCategory, Nature, Ranks, StatKey, Stats } from "../engine/types";
import { judgeScreenText } from "../i18n/ja";
import type { MasterNature, MasterSpecies } from "../master/types";

/** SP の6欄(表示順)。 */
export const SP_STATS: readonly StatKey[] = ["hp", "atk", "def", "spa", "spd", "spe"];

/** ランクのキー(HP を持たない)。 */
export type RankKey = keyof Ranks;

/** ランクの5欄(HP を持たない。RankBlock と同じ順)。 */
export const RANK_STATS: readonly RankKey[] = ["atk", "def", "spa", "spd", "spe"];

/** 調整プリセットの選択肢(表示順)。none は無振り、attack は攻撃特化(A/C は技の分類で決まる)。 */
export type JudgePresetKey = "none" | "fastest" | "attack" | "hb" | "hd";

export const JUDGE_PRESET_KEYS: readonly JudgePresetKey[] = ["none", "fastest", "attack", "hb", "hd"];

/** 状態異常の選択肢(表示順。契約の StatusCondition と同じ。none が既定で先頭。issue 235)。 */
export const JUDGE_STATUS_KEYS = [
  "none",
  "burn",
  "paralysis",
  "poison",
  "badly_poison",
  "sleep",
  "freeze",
] as const;

export type JudgeStatusKey = (typeof JUDGE_STATUS_KEYS)[number];

/** 1体分の入力の状態(自分・候補で共通の形。ADR-0705 §4)。 */
export interface IndividualFormState {
  readonly speciesKey: string;
  /** 選んだ種族の表示名(結果の行に出す。オンライン検索では master.species が空なのでここに持つ)。 */
  readonly speciesName: string;
  readonly natureId: string;
  readonly sp: Stats;
  /**
   * ランク(-6..+6)は文字列で持つ(type="text" の生の入力そのまま)。type="number" だと、負数を
   * 1文字ずつ打つ途中の "-" 単独をブラウザ(jsdom)が無効値として即座に "" へ戻してしまう。
   */
  readonly ranks: Record<RankKey, string>;
  readonly abilityId: string;
  readonly itemId: string;
  /** 状態異常。none は送らない(省略と同じ)。 */
  readonly status: JudgeStatusKey;
  /** 選んだ種族が覚える技(learnset の順)。技の select の選択肢。 */
  readonly moves: readonly Move[];
  /** 選んだ技の ID(Move.id)。 */
  readonly moveId: string;
  /** 選んでいる調整プリセット。null は SP・性格を手で変えた状態(どのラジオも選ばない)。 */
  readonly presetKey: JudgePresetKey | null;
}

/** 未入力の1体(SP とランクは0、調整は無振り。ADR-0705 §6)。 */
export function emptyIndividual(): IndividualFormState {
  return {
    speciesKey: "",
    speciesName: "",
    natureId: "",
    sp: { ...ZERO_SP },
    ranks: { atk: "0", def: "0", spa: "0", spd: "0", spe: "0" },
    abilityId: "",
    itemId: "",
    status: "none",
    moves: [],
    moveId: "",
    presetKey: "none",
  };
}

/** ランクの1欄を数へ変換する(空白のみ・数でなければ NaN)。 */
export function parseRank(raw: string): number {
  const trimmed = raw.trim();
  if (trimmed === "") {
    return Number.NaN;
  }
  return Number.parseInt(trimmed, 10);
}

/** 選んでいる技の分類。技が未選択のときは物理として扱う(A/C・ようき/おくびょうの決め方)。 */
export function moveCategoryOf(state: IndividualFormState): MoveCategory {
  return state.moves.find((move) => move.id === state.moveId)?.category ?? "physical";
}

/** プリセットの表示名(文言は i18n/ja.ts。無振り・A/C特化・HB/HD特化は既存プリセットと同じ語)。 */
export function judgePresetLabel(key: JudgePresetKey, category: MoveCategory): string {
  switch (key) {
    case "none":
      return attackerPresetLabel("none", category);
    case "fastest":
      return judgeScreenText.fastestPresetLabel;
    case "attack":
      return attackerPresetLabel("x_full", category);
    case "hb":
      return defenderPresetLabel("hb_full");
    case "hd":
      return defenderPresetLabel("hd_full");
  }
}

/** 技の分類で値が変わるプリセットか(技を替えたとき SP・性格を再解決する)。 */
function dependsOnCategory(key: JudgePresetKey): boolean {
  return key === "attack" || key === "fastest";
}

/**
 * プリセットの SP・性格。無振り・攻撃特化・HB/HD特化は既存のプリセット(domain/attackerPresets.ts・
 * domain/defenderPresets.ts)を再利用する。最速は判定画面固有で、素早さに全振り + 素早さ上昇の性格
 * (下降は物理技なら特攻=ようき、特殊技なら攻撃=おくびょう。ADR-0711)。
 */
function presetValues(key: JudgePresetKey, category: MoveCategory): { sp: Stats; nature: Nature } {
  switch (key) {
    case "none":
      return resolveAttackerPreset("none", category);
    case "attack":
      return resolveAttackerPreset("x_full", category);
    case "hb":
      return resolveDefenderPreset("hb_full");
    case "hd":
      return resolveDefenderPreset("hd_full");
    case "fastest":
      return {
        sp: { ...ZERO_SP, spe: MAX_SP_PER_STAT },
        nature: { plus: "spe", minus: category === "special" ? "atk" : "spa" },
      };
  }
}

/** 上昇・下降が一致する性格をマスタから引く(性格のリストをハードコードしない)。無ければ空文字。 */
function natureIdFor(natures: readonly MasterNature[], nature: Nature): string {
  return (
    natures.find((entry) => (entry.plus ?? "") === nature.plus && (entry.minus ?? "") === nature.minus)?.id ??
    ""
  );
}

function sameSp(a: Stats, b: Stats): boolean {
  return SP_STATS.every((stat) => a[stat] === b[stat]);
}

/** プリセットを選ぶ(SP と性格を上書きする)。 */
export function applyPreset(
  state: IndividualFormState,
  key: JudgePresetKey,
  natures: readonly MasterNature[],
): IndividualFormState {
  const { sp, nature } = presetValues(key, moveCategoryOf(state));
  return { ...state, presetKey: key, sp, natureId: natureIdFor(natures, nature) };
}

/** 技の分類が変わったとき、分類で値が変わるプリセットを選んでいれば SP・性格を再解決する(選びは保つ)。 */
function followCategory(
  prev: IndividualFormState,
  next: IndividualFormState,
  natures: readonly MasterNature[],
): IndividualFormState {
  if (
    next.presetKey !== null &&
    dependsOnCategory(next.presetKey) &&
    moveCategoryOf(prev) !== moveCategoryOf(next)
  ) {
    return applyPreset(next, next.presetKey, natures);
  }
  return next;
}

/** SP・性格を手で変えたあと、選んでいるプリセットの値と食い違えばどのプリセットも選ばない状態にする。 */
function reconcilePreset(state: IndividualFormState, natures: readonly MasterNature[]): IndividualFormState {
  if (state.presetKey === null) {
    return state;
  }
  const { sp, nature } = presetValues(state.presetKey, moveCategoryOf(state));
  const matches = sameSp(sp, state.sp) && natureIdFor(natures, nature) === state.natureId;
  return matches ? state : { ...state, presetKey: null };
}

/**
 * 種族を選ぶ。技の候補は種族の learnset(`pool` は技の実体の引き元)で、既定は最初のダメージ技。
 * 種族を替えると前の種族の技は残さない。
 */
export function chooseSpecies(
  state: IndividualFormState,
  species: MasterSpecies,
  pool: readonly Move[],
  natures: readonly MasterNature[],
): IndividualFormState {
  const moves = learnsetMoves(species, pool);
  const defaultMove = firstDamagingMove(species, pool) ?? moves[0];
  const next: IndividualFormState = {
    ...state,
    speciesKey: species.key,
    speciesName: species.nameJa,
    moves,
    moveId: defaultMove?.id ?? "",
  };
  const followed = followCategory(state, next, natures);
  // 性格が未選択のまま(プリセットの値がまだ入っていない)なら、選んでいるプリセットを入れる。手で選んだ性格は上書きしない。
  if (followed.natureId === "" && followed.presetKey !== null) {
    return applyPreset(followed, followed.presetKey, natures);
  }
  return followed;
}

/** 技を選ぶ。 */
export function chooseMove(
  state: IndividualFormState,
  moveId: string,
  natures: readonly MasterNature[],
): IndividualFormState {
  return followCategory(state, { ...state, moveId }, natures);
}

/** SP を手で変える。 */
export function editSp(
  state: IndividualFormState,
  sp: Stats,
  natures: readonly MasterNature[],
): IndividualFormState {
  return reconcilePreset({ ...state, sp }, natures);
}

/** 性格を手で変える。 */
export function editNature(
  state: IndividualFormState,
  natureId: string,
  natures: readonly MasterNature[],
): IndividualFormState {
  return reconcilePreset({ ...state, natureId }, natures);
}
