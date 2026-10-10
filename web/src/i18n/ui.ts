// G-05(ADR-0339): 共通の部品(web/src/ui/)の固定の文言。語は docs/glossary.md「共通の部品の語」。iOS と同じ語。

export const uiText = {
  /** 増減ボタン: 「<欄の名前>を増やす」「<欄の名前>を減らす」。 */
  stepperIncrease: "を増やす",
  stepperDecrease: "を減らす",
  /** シートの閉じるボタン(アイコンだけ)の名前。 */
  close: "閉じる",
  /** 説明ボタンの名前(見えるラベルと同じ)。 */
  help: "説明",
  /** タイル(空き枠)の「+」の名前に添える語。 */
  add: "追加",
} as const;
