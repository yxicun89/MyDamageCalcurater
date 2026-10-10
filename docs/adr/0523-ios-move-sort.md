# ADR-0523: iOS の技の選択肢の並び(習得順・五十音順・タイプ順)— F-02

- 状態: 採用(2026-10-10)
- 関連: docs/usability-round2.md F-02(原文6)、ADR-0335(Web)、docs/ai-shared/decisions/2026-10-04-web-move-sort.md、
  ADR-0518(技はダメージ技のみ)、ADR-0521(F-12 のポップ部品)、ADR-0501「issue #68」(技は Menu でなく検索シート)

## 決定

Web の ADR-0335 と同じ語・同じ規則を、計算画面の技ピッカー(検索シート `MoveSearchSheet`)に入れる。`Menu` は使わない。

1. **種類と既定**: `MoveSortOrder` = 習得順(`learnset`。**既定**。従来の並び)・五十音順(`kana`)・タイプ順(`type`)。UI 名「技の並び」。
   依頼文は「五十音順(既定)」だったが、Web の決定(ADR-0335・decisions/2026-10-04-web-move-sort.md)が既定を習得順としているため Web に合わせた
   (既存の画面・テストの並びも変わらない)。
2. **五十音順**(`MoveSort.sorted(_, by: .kana)`): カタカナをひらがなへ寄せてから日本語ロケール(`Locale("ja")`)の照合で比べる。
   ひらがな/カタカナは同じ字、濁点・半濁点の違いは同じ字の中で後、長音は照合器の扱い。同順位は技 ID の昇順(入力の順に依らず決定的)。入力は変えない。
3. **タイプ順**(`typeGroups`): タイプの並びで群にし、群の中は五十音順。技のあるタイプだけ。
   Web は `master.typeChart.types` の並びを使うが、iOS にはタイプ表のマスタが無く、タイプは契約の列挙 `PokeType`(docs/design.md のタイプ色パレットと同じ並び)なので、
   その宣言順を使う(リストのハードコードではなく型の定義)。見出しは `PokeTypeLabel.japaneseName`(List の Section)。習得順・五十音順は見出しなしの平らな並び。
4. **保存**: UserDefaults の1キー `pokecalc.moveSort`(`MoveSortStore`。保存先は注入可能)。不正値・未保存は既定。
   `POKECALC_USE_MOCK=1` のときは専用 suite を起動ごとに空にする(`RootView` の構築の保存先と同じ流儀)ので、UI テストの間で並びが残らない。
   計算画面のみ(逆算・判定・構築・タイプバランスの技ピッカーは今回触らない。`MoveSearchSheet` の `sortOrder` を渡せば同じ切り替えが付く)。
5. **不変**: 既定の技の自動選択は learnset の先頭のダメージ技のまま。選択中の技は並びを変えても保つ。並びの切り替えだけでは再計算しない。
   ダメージ技のみ(ADR-0518)の絞り込みは維持(並べ替えは `moveOptions` の後段 `displayedMoveOptions`)。
6. **画面**: シート先頭の「技の並び」チップ3つ(`moveSortPicker` / `moveSort-learnset|kana|type`)。タップ 36pt 以上・AX 文字サイズでは縦積み・
   lineLimit 不使用・色はトークンと `PopChipStyle` のみ。群の見出しは Section。

## 既存テストの影響

既定が習得順のため変更なし。

## 受け入れ条件(テストで固定)

`MoveSortTests`(習得順の保持・五十音の比較規則〈ひらがな/カタカナ・濁点・長音〉・同順位は ID・非破壊・タイプ順と群)、`MoveSortStoreTests`(往復・不正値は既定)、
`CalcViewModelMoveSortTests`(既定・表示順だけ変わる・選択保持・再計算しない・保存と復元・既定の技は learnset 先頭)、
`CalcMoveSortUITests`(切り替えの並びと見出し・選択・AX5 の横あふれとタップ 36pt)。
