// テスト専用(P4-16c): 種族の検索欄(screens/SpeciesSearchField.tsx)のクラス名。
// 見た目(CSS)と振る舞い(ハイライトのクラス)の契約を、DOM のテストと CSS の静的検査の両方が
// 同じ1か所から引く(コーディング規約 §2「単一の正を決める」)。
// 値は受け入れ条件が定めるもので、実装の写しではない(実装がこの名前から外れたらテストが落ちる)。

export const speciesSearchClass = {
  /** 検索欄全体の入れ物。 */
  root: "species-search",
  /** 入力欄の補足(aria-describedby で結ぶ)。 */
  hint: "species-search__hint",
  /** 状態の案内(入力前・0件・上限・失敗)。 */
  status: "species-search__status",
  /** 候補一覧(role="listbox")。 */
  list: "species-search__list",
  /** 候補1件(role="option")。 */
  option: "species-search__option",
  /** キーボードでハイライト中の候補1件(aria-activedescendant が指すもの)。 */
  activeOption: "species-search__option--active",
} as const;
