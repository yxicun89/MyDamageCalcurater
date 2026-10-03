import type { OfficialState, OfficialStatus } from "../api/types";
import { formatJstDate } from "./price";

// フェーズ4-3 公式サイトの販売状況の表示用の純粋関数(docs/phase4-spec.md AC-OFF-*)。
// 日付は price.ts の formatJstDate(JST の M/D)を使う。推測の文言を足さない(判定できないときは「判定できません」)。

/** 状態ごとの文言(詳細シート・iOS と同じ)。 */
export const OFFICIAL_LABELS: Record<OfficialState, string> = {
  available: "販売中",
  preorder: "予約受付中",
  soldout: "在庫切れ",
  ended: "販売終了",
  unknown: "判定できません",
  ambiguous: "判定できません(複数の表示)",
  blocked: "取得しません(robots.txt)",
  failed: "取得できませんでした",
};

/** 変化を添える期間(changed_at から 7 日以内)。 */
export const OFFICIAL_CHANGE_DAYS = 7;

const isJudged = (s: OfficialState): boolean => s !== "failed" && s !== "blocked";

/**
 * 1 行目。`公式: 予約受付中(10/4 確認)`。日付は checked_at(JST)。
 * status が failed・blocked(まだ一度も判定できていない)なら「確認」を付けない:`公式: 取得できませんでした(10/4)`。
 * status が null(まだ確かめていない)なら `公式: まだ確認していません`。
 */
export function officialSummary(status: OfficialStatus | null | undefined): string {
  if (!status) return "公式: まだ確認していません";
  const date = formatJstDate(status.checked_at);
  const confirmed = isJudged(status.status) ? " 確認" : "";
  return `公式: ${OFFICIAL_LABELS[status.status]}(${date}${confirmed})`;
}

/** 根拠の行。`根拠: 予約受付中・予約する`。根拠が無ければ null。 */
export function officialEvidence(status: OfficialStatus | null | undefined): string | null {
  if (!status || status.evidence.length === 0) return null;
  return `根拠: ${status.evidence.join("・")}`;
}

/**
 * 変化の行。changed_at と previous_status があり、now - changed_at が OFFICIAL_CHANGE_DAYS 日(ミリ秒で 7×24 時間)以内なら
 * `10/3 に 販売中 → 販売終了`(日付は changed_at の JST)。それ以外は null。changed_at が now より後(時計のずれ)も出す。
 */
export function officialChange(status: OfficialStatus | null | undefined, now: Date): string | null {
  if (!status?.changed_at || !status.previous_status) return null;
  const age = now.getTime() - new Date(status.changed_at).getTime();
  if (age > OFFICIAL_CHANGE_DAYS * 24 * 60 * 60 * 1000) return null;
  const date = formatJstDate(status.changed_at);
  return `${date} に ${OFFICIAL_LABELS[status.previous_status]} → ${OFFICIAL_LABELS[status.status]}`;
}

/**
 * 最後の試行が status と違う(判定済みの status を残したまま、そのあと failed・blocked だった)ときの注記。
 * `最新の確認(10/5): 取得できませんでした`(日付は last_attempt_at の JST)。それ以外は null。
 */
export function officialLastAttempt(status: OfficialStatus | null | undefined): string | null {
  if (!status || !isJudged(status.status) || isJudged(status.last_result)) return null;
  return `最新の確認(${formatJstDate(status.last_attempt_at)}): ${OFFICIAL_LABELS[status.last_result]}`;
}
