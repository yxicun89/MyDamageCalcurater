# ADR-0335: 技の選択肢の並び(習得順・五十音順・タイプ順)— F-02 / I-web-9

- 状態: 採用(2026-10-04)
- 関連: docs/usability-round2.md F-02(原文6)、docs/plan/improvements/web.md I-web-9、ADR-0328(技はダメージ技のみ)、
  ADR-0333 §4(技を戻せなかったときの未選択の選択肢)、ADR-0308(訪問済みタブの保持)、ADR-0331・ADR-0334(F-12 の部品 .ui-chip)、
  ADR-0013(タイプ相性表はマスタから引く)、ADR-0301 §4 / app/calcMode.ts(localStorage の作法)

## 背景

利用者の要望(原文6): 技の選択肢は五十音順(またはタイプ順)にソートできるとよい。いまは種族の learnset の順のままで、
覚える技が多い種族では目当ての技を探しにくい。計算画面・逆算画面の技欄(`<select>`)に並びの切り替えを足す。
engine・API・マスタの契約は変えない(画面の並べ替えだけ)。

## 決定

### 1. 並びの種類

| 値(`MoveSortOrder`) | 表示 | 内容 |
|---|---|---|
| `learnset`(既定) | 習得順 | 従来の並び(learnset の順のまま) |
| `kana` | 五十音順 | 技名(`nameJa`)の五十音順 |
| `type` | タイプ順 | マスタのタイプ表の並びでタイプごとに群にし、群の中は五十音順 |

切り替えは技欄の近くの小さなチップ群(`role="radiogroup"`・名前「技の並び」・F-12 の `.ui-chip`。計算画面の攻撃側プリセットと同じ作り)。
文言は `web/src/i18n/moveSort.ts`(`moveSortText.groupLabel`・`moveSortText.options`)。iOS と同じ語にする。
既定の技の自動選択(`firstDamagingMove` = learnset の最初のダメージ技)は**変えない**(並びを五十音順にしても、
攻撃側を選んだときの既定の技は learnset の先頭。並びは表示の順だけ)。

### 2. 五十音順の比較(`domain/moveSort.ts` の `sortMoves(moves, "kana", types)`)

- `Intl.Collator("ja")` で `nameJa` を比べる。ひらがな/カタカナは同じ字として扱い、濁点・半濁点の違いは同じ字の中で後
  (が は か の仲間。「かまう」「がまん」は「かみなり」より前)。長音「ー」は照合器の扱いに従う(「スーパー」は「スパーク」より前)
- 照合器で同順位(ひらがな/カタカナだけの違いなど)のときは、技 ID の昇順で決める(入力の順に依らず決定的)
- 入力の配列は書き換えない(新しい配列を返す)

### 3. タイプ順と見出し

- `sortMoves(moves, "type", types)`・`moveTypeGroups(moves, types)`。`types` は `master.typeChart.types`(マスタのタイプ表の並び。ハードコードしない)
- 技のあるタイプだけ群にする。タイプ表に無いタイプの技は最後に、タイプ ID の昇順で群にまとめる(壊れない)
- 選択肢の見出しは **optgroup**(`label` = `typeNameJa[type]`、未知のタイプは ID のまま)。タイプ順のときだけ使い、習得順・五十音順は平らな option
  (a11y: optgroup は支援技術が群の名前として読む。接頭辞方式は技名の読み上げを長くするので採らない)
- 表示は従来どおり「技名・分類・威力」

### 4. 並びの記憶(計算画面と逆算画面で共有)

- localStorage の1キー `pokecalc.moveSort`(`app/moveSortStorage.ts`。`loadMoveSort` / `saveMoveSort`)。値は `MoveSortOrder` の文字列
- 読み書きは try/catch(使えなくても失敗させない。不正な値・未知の値は既定=習得順に戻す)。画面の中では保存できなくても切り替わる
- 計算画面と逆算画面は同じ並びを共有する。ADR-0308 で mount したままのタブにも反映する(どちらで変えても、もう一方のタブへ往復したときに同じ)。
  実装は共有の状態(`useSyncExternalStore` 等)でよい

### 5. 選択中の技・結果・要求

- 並びを変えても選択中の技の ID は変わらない。計算・逆算の要求と結果は変わらず、並びの切り替えだけで再計算しない
- F-09 の「技を選んでください」(disabled の未選択の選択肢)は、どの並びでも先頭に残し、optgroup の外に置く

### 6. 実装の置き場

- 純粋関数と型: `web/src/domain/moveSort.ts`(`sortMoves`・`moveTypeGroups`・`MOVE_SORT_ORDERS`・`DEFAULT_MOVE_SORT_ORDER`・`isMoveSortOrder`)
- 保存: `web/src/app/moveSortStorage.ts`
- 文言: `web/src/i18n/moveSort.ts`
- 画面: CalcScreen.tsx・ReverseScreen.tsx の `MoveSelect` が結果を描画するだけ(共通のチップ群部品にしてよい)

### 7. 既存テストへの影響(期待値更新の対象)

既定が習得順(従来)なので、既存テストの期待値は変わらない見込み。技欄に radiogroup を足すため、
`getByRole("radiogroup")` を名前なしで引く既存テストがあれば名前つきに直す必要があるかもしれない(期待値は弱めない)。

## 結果(実装)

- `web/src/domain/moveSort.ts`(純粋関数)、`web/src/app/moveSortStorage.ts`(保存)、`web/src/app/useMoveSort.ts`(`useSyncExternalStore` で
  計算・逆算・隠れたタブへ共有。保存できないときだけ購読者がいる間メモリに覚える)、`web/src/i18n/moveSort.ts`、
  `web/src/screens/MoveSortControls.tsx`(チップ群 `MoveSortChips` と選択肢 `MoveOptions`。両画面の `MoveSelect` が使う)。
- `<select>` を持つ `MoveSelect` が並びを読んで `MoveOptions` に渡す(選択肢だけが別コンポーネントで再描画されると、
  optgroup への入れ替えで選択中の値が先頭に戻るため。select 自身を再描画して React に値を再設定させる)。
- 未選択の選択肢(F-09)は計算画面だけ(逆算にはもともと無い)。
- 既存テストの期待値は変更なし。新規テストの 2 件の lint/型の指摘(`options: {}` に `critical: false`、`?.` の除去、async の除去、
  アロー関数の波括弧)だけ最小限直した。

## 付録: iOS 向け(docs/ai-shared/decisions/2026-10-04-web-move-sort.md に書いた内容)

- 並びの種類と既定: 習得順(既定)・五十音順・タイプ順。語は「習得順」「五十音順」「タイプ順」
- 五十音順: 日本語照合(`localizedStandardCompare` ではなく、ひらがな/カタカナ・濁点を同じ字として比べる照合。Foundation の `Locale(identifier: "ja")` で
  `compare(_:options:[.widthInsensitive, .caseInsensitive]:locale:)` 相当)。同順位は技 ID の昇順
- タイプ順: マスタのタイプ表の並びで群にし、群の中は五十音順。タイプ表に無いタイプは最後(ID の昇順)。見出しはタイプ名(Picker の Section)
- 保存: UserDefaults の1キー(`pokecalc.moveSort` 相当)。不正値は既定。計算と逆算で共有
- 既定の技の自動選択は learnset の最初のダメージ技のまま。選択中の技は並びを変えても保つ
