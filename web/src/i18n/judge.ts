// 判定(JD5・ADR-0705)の文言。ADR-0173 で i18n/ja.ts から移した(ja.ts が再エクスポートする)。
// このレーンの文言はこのファイルにだけ足す(ja.ts は触らない)。

import type { StatKey } from "../engine/types";
import { statLetterJa } from "./common";

/**
 * JD5: judge API のクライアント(judge/judgeClient.ts、ADR-0705 §3)の文言。
 * 通信できない・応答が読めない・エラー本文の形が不正なとき(自動の切り替え先は持たない)。
 */
export const judgeClientText = {
  unavailable: "判定の API に接続できません",
} as const;

/**
 * JD5: 判定のエラーの見出し(ADR-0705 §8)。services/judge/api/openapi.yaml の ErrorCode と、
 * Web 側の judge_unavailable(judgeClient.ts)に 1 対 1 で対応する。
 * サーバーの message(どの候補で失敗したかが `defenders[<index>]` の形で入る。ADR-0703 §3)は
 * この見出しとは別に、補助の行として画面が出す。未知のコードは message だけを出す。
 */
export const judgeErrorText = {
  missing_header: "端末の情報を送れませんでした。ページを開き直してください",
  invalid_header: "端末の情報が正しくありません。ページを開き直してください",
  invalid_request: "入力の形が正しくありません",
  unknown_species: "このポケモンはマスタにありません",
  unknown_move: "この技の ID はマスタにありません",
  unknown_nature: "この性格はマスタにありません",
  request_too_large: "入力が大きすぎます",
  upstream_unavailable: "判定に必要なサービスに接続できません",
  internal_error: "判定に失敗しました",
  /** Web 側のコード(judgeClient.ts。通信できない・応答が読めない)。 */
  judge_unavailable: "判定の API に接続できません",
} as const;

/**
 * JD5: 判定の画面(judge/JudgeScreen.tsx、ADR-0705 §4・§6・§8)の文言。
 * 判定そのもの(素早さ・行動順・確定数)は judge-svc が返した値をそのまま出す。
 * 画面は「勝ち」「負け」に丸めた語を持たない(ADR-0700 §6-1・ADR-0704 §3 の立場を画面でも保つ)。
 */
export const judgeScreenText = {
  // ---- 領域(ADR-0705 §4) ----
  attackerRegionLabel: "自分のポケモン",
  defendersRegionLabel: "相手の候補",
  resultRegionLabel: "判定結果",
  // ---- 個体の入力。自分側と候補で同じ語を使う(候補は候補の group で絞り込む。ADR-0705 §4) ----
  speciesLabel: "ポケモン",
  natureLabel: "性格",
  abilityLabel: "特性",
  itemLabel: "持ち物",
  /** 状態異常の select の名前。選択肢は契約の StatusCondition(none が先頭で既定。issue 235)。 */
  statusLabel: "状態異常",
  statusOptionLabel: {
    none: "なし",
    burn: "やけど",
    paralysis: "まひ",
    poison: "どく",
    badly_poison: "もうどく",
    sleep: "ねむり",
    freeze: "こおり",
  } as const,
  unselectedOption: "未選択",
  // ---- issue 309: 技はポケモンの覚える技から選ぶ。調整はプリセット。数値欄は「詳細」に畳む ----
  /** 技の select の名前(自分側・候補で共通。計算画面の calcScreenText.moveLabel と同じ語)。 */
  moveLabel: "技",
  /** 覚える技を1件も引けないとき(learnset が空・技の実体を解決できない)。技の select は disabled のまま。 */
  moveUnavailableNotice: "この種族の技を読み込めません",
  /** 数値の直接入力(SP6欄・ランク5欄)を畳む <details> の summary。 */
  detailsSummaryLabel: "詳細",
  /** 調整プリセットの radiogroup の名前(自分側・候補で共通。候補の group で絞り込む)。 */
  presetGroupLabel: "調整",
  /** 最速プリセット(S 全振り + 素早さ上昇の性格)の表示名。無振り・A特化は attackerPresetText、HB/HD特化は defenderPresetText から。 */
  fastestPresetLabel: "最速",
  /** 検証エラーの「どの体か」(自分側。候補は candidateGroupLabel(n) を使う)。 */
  attackerWhoLabel: "自分",
  spLabel: (stat: StatKey): string => `${statLetterJa[stat]} のポイント`,
  rankLabel: (stat: StatKey): string => `${statLetterJa[stat]} のランク`,
  formatLabel: "対戦形式",
  formatOption: { single: "シングル", double: "ダブル" } as const,
  // ---- 場の効果(speedField。ADR-0705 §6) ----
  speedFieldGroupLabel: "場の効果",
  trickRoomLabel: "トリックルーム",
  attackerTailwindLabel: "自分の側の追い風",
  defenderTailwindLabel: "相手の側の追い風",
  /** 相手側の追い風が候補ごとではない理由(ADR-0703 §5・ADR-0705 §6)。 */
  defenderTailwindNotice: "相手の側の追い風は、すべての相手候補に同じように適用されます",
  // ---- 相手候補の増減(ADR-0705 §4) ----
  candidateGroupLabel: (n: number): string => `相手候補${n}`,
  addCandidateLabel: "相手候補を追加",
  removeCandidateLabel: (n: number): string => `相手候補${n}を削除`,
  maxCandidatesNotice: (max: number): string => `相手候補は${max}件までです`,
  // ---- 送信(ADR-0705 §7) ----
  submitLabel: "判定する",
  loadingNotice: "判定中",
  emptyResultNotice: "「判定する」を押すと結果が出ます",
  // 送信前の検査(契約の範囲と同じ。違反していれば judge を呼ばずに理由を出す)。
  spRangeMessage: (max: number): string => `能力ポイントは0〜${max}の整数で入力してください`,
  spTotalMessage: (max: number): string => `能力ポイントの合計は${max}までです`,
  rankRangeMessage: "ランクは-6〜+6の整数で入力してください",
  requiredMessage: "ポケモン・性格・技をすべて選んでください",
  // ---- 結果(ADR-0705 §8)。judge の値をそのまま出す ----
  speedLabel: (attacker: number, defender: number): string => `素早さ ${attacker} 対 ${defender}`,
  /**
   * 素早さに反映した補正・反映していない入力(ADR-0710。issue 235)。judge が返した欄をそのまま文にする。
   * 反映していない入力は「指定されたが素早さには掛けていない」の意味で、効果が無い特性・持ち物でも出る。
   */
  speedAppliedNote: (side: string, names: readonly string[]): string =>
    `${side}の素早さに反映: ${names.join("・")}`,
  speedIgnoredNote: (side: string, names: readonly string[]): string =>
    `${side}の素早さに${names.join("・")}は反映していません`,
  speedSideSelf: "自分",
  speedSideOpponent: "相手",
  speedFactorLabel: {
    rank: "ランク補正",
    tailwind: "追い風",
    choiceScarf: "こだわりスカーフ",
    paralysis: "まひ",
  } as const,
  speedIgnoredLabel: { abilityId: "特性", itemId: "持ち物", fieldWeather: "天候" } as const,
  priorityLabel: (attacker: number, defender: number): string => `優先度 ${attacker} 対 ${defender}`,
  outspeedsTrueLabel: "素早さで上回る",
  outspeedsFalseLabel: "素早さで下回る",
  /** 同速(outspeeds と同時に true にならない。真偽値1つに丸めない。ADR-0700 §6-1)。 */
  speedTieLabel: "同速",
  attackerMovesFirstLabel: "自分が先に動く",
  defenderMovesFirstLabel: "相手が先に動く",
  /** 優先度も素早さも同じで行動順が決まらないとき(ADR-0704 §2)。 */
  turnOrderTieLabel: "どちらが先に動くか決まらない",
  attackerKoLabel: "自分の技で相手を",
  defenderKoLabel: "相手の技で自分が",
  koGuaranteed: (hits: number): string => `確定${hits}発`,
  koRandom: (hits: number, percent: number): string => `乱数${hits}発(${percent}%)`,
  koNone: "倒せない",
} as const;
