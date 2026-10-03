// レーンをまたいで使う文言の語(ADR-0323)。i18n/ja.ts とレーン別の文言ファイル(i18n/<レーン>.ts)の両方がここから読む
// (レーン別のファイルが ja.ts を import すると、ja.ts の再エクスポートと循環して未初期化の値を読むため)。

import type { StatKey } from "../engine/types";

/** 入力欄の見えるラベルの語(短くする。どちら側かは領域の見出しが担う。issue 304)。 */
export const pokemonFieldLabel = "ポケモン";
export const itemFieldLabel = "持ち物";
export const abilityFieldLabel = "特性";

/** 種族 select が未選択のとき、hidden の先頭 option に出す文言(空文字にしない。issue 304)。 */
export const speciesPlaceholderOption = "ポケモンを選ぶ";

/** ステータスの1文字表記(H・A・B・C・D・S)。逆算の SP 範囲・目安の名前の表示に使う(P4-4)。 */
export const statLetterJa: Record<StatKey, string> = {
  hp: "H",
  atk: "A",
  def: "B",
  spa: "C",
  spd: "D",
  spe: "S",
};
