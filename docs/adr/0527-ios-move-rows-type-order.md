# ADR-0527: iOS の技の選択肢を縦に短い 1 行にし、並びをタイプ順だけにする — G-01

- 状態: 採用(2026-10-11)
- 関連: docs/usability-round3.md G-01、ADR-0523(F-02。本 ADR で更新)、ADR-0335(Web。Web 側は別途)、ADR-0501「issue #68」(技は検索シート)、
  docs/ai-shared/decisions/2026-10-11-ios-move-rows-type-order.md

## 決定

1. **1 行の見た目**: 技の行(`MasterSearchRow.move`)を「タイプの丸いアイコン + 技名」を主に、右に分類の小さなアイコン(SF Symbols。ぶつり=`figure.boxing`・
   とくしゅ=`sparkles`・へんか=`circle.dashed`)と威力の数字を副の情報として 1 行に並べる。タイプのアイコンは `TypeColorToken` のタイプ色の円 + タイプ名の頭文字
   (画像を使わない。ADR-0808)。最小の高さは 36pt。色・余白・フォントはデザイントークンのみ、`lineLimit` は使わない。
2. **読み上げ**: 行の `accessibilityLabel` は従来どおり技名(識別子 `moveSearchResult-<id>` も不変)。`accessibilityValue` に「タイプ、分類、威力N」を足し、全情報を保つ。
   アイコン単体にも `accessibilityLabel`(分類名・威力N)を付け、タイプの丸は装飾として隠す。
3. **並びはタイプ順だけ**: ADR-0523 の切り替え(習得順・五十音順・タイプ順の `MoveSortChips`)と選択の保存(`MoveSortStore`・UserDefaults `pokecalc.moveSort`)・
   `MoveSortOrder`・`MoveSortLabels`・`CalcViewModel.moveSortOrder` を廃止。計算画面の技ピッカーは `MoveSort.byType`(旧 `.type`。タイプ表の並び〔`PokeType` の宣言順〕で群にし、
   群の中は五十音順)で並べ、群の見出しはタイプ名(従来のタイプ順と同じ)。習得順は使わない。既定の技の自動選択は learnset の先頭のダメージ技のまま(表示順と無関係)。
4. **範囲**: 行の見た目は技の検索シートを使う全画面(計算・逆算・判定・構築・タイプバランス)で変わる。並び(タイプ順の群)は従来どおり計算画面のみ
   (`MoveSearchSheet.groupsByType`)。他画面の並びは今回は変えない。

## ADR-0523 の更新

ADR-0523 の 1(種類と既定)・4(保存)・6(画面のチップ)は G-01 で廃止。2(五十音の比較規則)・3(タイプ順の群)・5(不変)は「タイプ順の群の中の並び」として残る。

## 既存テストの扱い(絶対ルール6)

廃止した機能のテストだけを削除した。残る規則の検査は弱めず、呼び出しを `MoveSort.byType` に付け替えて維持した。

| テスト | 扱い | 理由 |
|---|---|---|
| `MoveSortTests.testOrdersAndDefault`(種類・既定・ラベル) | 削除 | `MoveSortOrder`/`MoveSortLabels` 自体を廃止 |
| `MoveSortTests.testLearnsetKeepsInputOrder` | 削除 | 習得順を廃止 |
| `MoveSortTests` の五十音(`testKana*` 5 件) | 維持(`byType` へ付け替え。同じ入力・同じ期待値) | 群の中の並びとして残る |
| `MoveSortTests` のタイプ順 3 件 | 維持(`byType` へ付け替え) | 同上 |
| `MoveSortStoreTests`(往復・不正値は既定) | 削除 | `MoveSortStore` を廃止 |
| `CalcViewModelMoveSortTests`(保存と復元・五十音で既定の技が変わらない) | 削除し `CalcViewModelMoveTypeOrderTests` に置換 | 保存・切り替えを廃止。「表示は moveOptions と同じ集合のタイプ順」「既定の技は learnset 先頭」「表示順の取得で再計算しない」は新テストで維持 |
| `CalcMoveSortUITests` の切り替え 2 件(順の切り替え・チップの AX5) | 置換 | チップを廃止。タイプ順の並びと見出し・切り替えが無いこと・選択・行の AX5 の収まりとタップ 36pt を新テストで固定 |

## 受け入れ条件(テストで固定)

`MoveSortTests`(五十音の規則・タイプ順と群)、`CalcViewModelMoveTypeOrderTests`、`CalcMoveSortUITests`
(タイプ順だけで並ぶ・見出し・切り替え UI が無い・選択・AX5 で行が収まりタップ 36pt 以上)。
