// 判定の画面の送信前の検査(ADR-0705 §7・受け入れ条件7)。契約の範囲と同じ検査を、呼ぶ前にクライアント側で行う。
// issue 309: 最初の1件で止めず、全員の誤りを欄ごとに集める(どの体かを文言に添える)。

import { MAX_SP_PER_STAT, MAX_SP_TOTAL } from "../domain/requests";
import type { StatKey } from "../engine/types";
import { judgeScreenText } from "../i18n/ja";
import { type IndividualFormState, parseRank, RANK_STATS, type RankKey, SP_STATS } from "./individualForm";

/** ランクの範囲(CLAUDE.md ドメイン規約 / 契約の RankBlock と同じ -6..+6)。 */
const MIN_RANK = -6;
const MAX_RANK = 6;

/** 検査の対象1体(key は state の置き場の識別子、who は文言に添える「どの体か」)。 */
export interface ValidationBody {
  readonly key: string;
  readonly who: string;
  readonly state: IndividualFormState;
}

/** 1体分の欄ごとの誤り(文言は「どの体か: 理由」。無ければ null)。 */
export interface BodyErrors {
  readonly species: string | null;
  readonly nature: string | null;
  readonly move: string | null;
  /** SP が範囲外の欄(その欄だけが invalid)と、その文言。 */
  readonly spRange: string | null;
  readonly spRangeStats: ReadonlySet<StatKey>;
  /** SP の合計超過(その体の6欄すべてが invalid)の文言。 */
  readonly spTotal: string | null;
  /** ランクが範囲外の欄と、その文言。 */
  readonly rankRange: string | null;
  readonly rankStats: ReadonlySet<RankKey>;
}

export interface ValidationResult {
  /** 従来の全体メッセージ(role="alert")。誤りが無ければ null。 */
  readonly firstMessage: string | null;
  /** 誤りのある体だけを持つ。 */
  readonly errors: ReadonlyMap<string, BodyErrors>;
}

/** 「詳細」の中の欄(SP・ランク)に誤りがあるか(あれば送信時に自動で開く)。 */
export function hasDetailsError(errors: BodyErrors): boolean {
  return errors.spRange !== null || errors.spTotal !== null || errors.rankRange !== null;
}

function withWho(who: string, reason: string): string {
  return `${who}: ${reason}`;
}

function validateBody(body: ValidationBody): BodyErrors | null {
  const { state, who } = body;
  const required = judgeScreenText.requiredMessage;
  const spRangeStats = new Set(
    SP_STATS.filter((stat) => state.sp[stat] < 0 || state.sp[stat] > MAX_SP_PER_STAT),
  );
  const total = SP_STATS.reduce((sum, stat) => sum + state.sp[stat], 0);
  const rankStats = new Set(
    RANK_STATS.filter((stat) => {
      const value = parseRank(state.ranks[stat]);
      return Number.isNaN(value) || value < MIN_RANK || value > MAX_RANK;
    }),
  );
  const errors: BodyErrors = {
    species: state.speciesKey.trim() === "" ? withWho(who, required) : null,
    nature: state.natureId.trim() === "" ? withWho(who, required) : null,
    move: state.moveId.trim() === "" ? withWho(who, required) : null,
    spRange: spRangeStats.size > 0 ? withWho(who, judgeScreenText.spRangeMessage(MAX_SP_PER_STAT)) : null,
    spRangeStats,
    spTotal: total > MAX_SP_TOTAL ? withWho(who, judgeScreenText.spTotalMessage(MAX_SP_TOTAL)) : null,
    rankRange: rankStats.size > 0 ? withWho(who, judgeScreenText.rankRangeMessage) : null,
    rankStats,
  };
  const hasError =
    errors.species !== null || errors.nature !== null || errors.move !== null || hasDetailsError(errors);
  return hasError ? errors : null;
}

/** 全員を検査する。全体メッセージの優先は従来と同じ(必須 → SP の範囲・合計 → ランク)。 */
export function validateBodies(bodies: readonly ValidationBody[]): ValidationResult {
  const errors = new Map<string, BodyErrors>();
  for (const body of bodies) {
    const bodyErrors = validateBody(body);
    if (bodyErrors !== null) {
      errors.set(body.key, bodyErrors);
    }
  }
  const all = [...errors.values()];
  let firstMessage: string | null = null;
  if (all.some((e) => e.species !== null || e.nature !== null || e.move !== null)) {
    firstMessage = judgeScreenText.requiredMessage;
  } else if (all.some((e) => e.spRange !== null)) {
    firstMessage = judgeScreenText.spRangeMessage(MAX_SP_PER_STAT);
  } else if (all.some((e) => e.spTotal !== null)) {
    firstMessage = judgeScreenText.spTotalMessage(MAX_SP_TOTAL);
  } else if (all.some((e) => e.rankRange !== null)) {
    firstMessage = judgeScreenText.rankRangeMessage;
  }
  return { firstMessage, errors };
}
