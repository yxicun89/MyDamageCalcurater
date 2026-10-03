// AJ6: 調整の画面の表示の書式(ADR-0319 §6)。純粋関数。

import { adjustErrorText } from "../i18n/ja";

/**
 * engine の生の確率(%、double。ADR-0250 §6 で丸めずに返る)を表示の文字にする。
 * 0.1% 単位で**切り捨て**る(99.99% を「100%」と出して確定に見せない)。整数なら小数点を付けない。
 * 例: 100 → "100%"、37.5 → "37.5%"、99.99 → "99.9%"、0 → "0%"、12.34 → "12.3%"。
 */
export function formatChancePercent(percent: number): string {
  const tenths = Math.floor(percent * 10 + 1e-9) / 10;
  return `${String(tenths)}%`;
}

/**
 * エラーコードから画面に出す日本語の文言を引く(i18n/ja.ts の adjustErrorText)。未知のコードは fallback。
 * サーバーの message(英語の内部メッセージを含みうる)は使わない(ADR-0411 §3 と同じ)。
 */
export function adjustErrorMessage(code: string): string {
  return Object.hasOwn(adjustErrorText, code)
    ? adjustErrorText[code as keyof typeof adjustErrorText]
    : adjustErrorText.fallback;
}
