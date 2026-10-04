// SP(能力ポイント)の1欄の文字列の読み方。構築のメンバー編集(team/teamMember.ts の checkSpDraft)と
// 計算画面の攻撃側の SP 欄(domain/attackerStatInputs.ts)で同じ規則を使う(ADR-0316 §4、ADR-0329 §5)。

import { MAX_SP_PER_STAT } from "./requests";

/** SP の入力として読める形(符号・小数点・指数・全角を含まない 10 進整数)。 */
export const SP_DIGITS = /^\d+$/;

/** parseSpText の結果。 */
export type SpTextResult = { readonly ok: true; readonly value: number } | { readonly ok: false };

/**
 * SP の1欄を読む。前後の空白は無視し、空欄は 0(1文字ずつ消す途中を弾かない)。
 * 10 進の整数で 0〜MAX_SP_PER_STAT のときだけ ok。
 */
export function parseSpText(text: string): SpTextResult {
  const trimmed = text.trim();
  if (trimmed === "") {
    return { ok: true, value: 0 };
  }
  if (!SP_DIGITS.test(trimmed)) {
    return { ok: false };
  }
  const value = Number(trimmed);
  return value > MAX_SP_PER_STAT ? { ok: false } : { ok: true, value };
}
