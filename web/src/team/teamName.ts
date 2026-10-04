// 構築名の検査(api/openapi.yaml の TeamInput.name は省略可で、省略・空は既定名「名称未設定」になる〈ADR-0229〉。送る名前は前後の空白を除いて50文字まで)。
// 新規作成・名前変更・Showdown 取り込みが同じ検査を使う(ADR-0309 §4・ADR-0321 §2)。

import { teamScreenText } from "../i18n/team";

/** 構築名の長さの上限(api/openapi.yaml の TeamInput.name の maxLength)。 */
export const MAX_TEAM_NAME_LENGTH = 50;

/** 前後の空白を除いた文字数(Unicode コードポイントで数える。契約の TeamInput.name と同じ数え方)。 */
export function codePointLength(value: string): number {
  return Array.from(value).length;
}

/** 送信前の検査。問題があればその理由(画面に出す文言)、無ければ null。 */
export function teamNameNotice(name: string): string | null {
  const trimmed = name.trim();
  if (trimmed === "") {
    return teamScreenText.nameRequiredNotice;
  }
  if (codePointLength(trimmed) > MAX_TEAM_NAME_LENGTH) {
    return teamScreenText.nameTooLongNotice(MAX_TEAM_NAME_LENGTH);
  }
  return null;
}
