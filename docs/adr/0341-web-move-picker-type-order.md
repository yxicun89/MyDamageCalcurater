# ADR-0341: 技の選択肢を技ピッカー(1 行・タイプ順だけ)にする — G-01

- 状態: 採用(2026-10-11。実装 PR で採用にした)
- 関連: docs/usability-round3.md G-01、ADR-0335(F-02。本 ADR で一部置き換え)、ADR-0339(G-05 見た目の作り直し方針。docs/design.md)、ADR-0328(技はダメージ技のみ)、
  ADR-0333 §4(未選択の行)、ADR-0304 A-12(コンボボックスのキーボード操作。`SpeciesSearchField`)、ADR-0013(タイプ表はマスタから引く)、
  ADR-0527(iOS 側の同じ決定)、docs/ai-shared/decisions/2026-10-11-web-g01-move-picker.md

## 背景

計算・逆算画面の技欄は `<select>` の option に「技名・ぶつり/とくしゅ・威力」を横一列の文字で出しており、全部読まないと選べない。
ユーザーは「技名とタイプを主に、分類(ぶつり/とくしゅ)は小さなアイコン、威力は副の情報にして 1 行で一目に分かる」ことと、
並びを**タイプ順だけ**にすること(切り替えと習得順の廃止)を求めた。`<select>` の option には絵(バッジ・アイコン)を置けないため、部品を作り直す。

## 決定

1. **技ピッカー `MovePicker`**(`web/src/screens/MovePicker.tsx` + `MovePicker.css`。計算・逆算が共有): G-05 の「ピッカー」の Web 版(広い・狭い幅とも
   トリガーの下に開く listbox パネル。下から出すシートは今回は作らない)。`<select>` は使わない(1 行の option を作れないため)。
2. **a11y**: APG の select-only combobox + 検索つき listbox。トリガーは `role="combobox"`・`aria-haspopup="listbox"`・`aria-expanded`・`aria-controls`・名前「技」
   (見えるラベルと同じ)・`data-value`=選択中の技 ID。パネルに検索欄(`role="searchbox"`・名前「技を検索」)と `role="listbox"`(名前「技」)。
   フォーカスは検索欄に置き、`aria-activedescendant` で行を指す(ArrowDown/Up・Home/End・Enter で選択・Esc で選ばず閉じてトリガーへ戻す・Tab と外側の押下で閉じる)。
   開いた直後の活性は選択中の行(無ければ先頭)。選んだらトリガーへフォーカスを戻す。検索は技名の部分一致(ひらがな/カタカナ・大小は同じ字)。0 件は `role="status"` の 1 行。
3. **行(`role="option"`)は 1 行**: `[タイプバッジ(.ui-badge、タイプ色のピル + タイプ名)] 技名 … [分類アイコン] 威力`。主=技名とタイプ、副=分類(小アイコン)と威力(`text.secondary`)。
   `white-space: nowrap`・長い技名は省略(`text-overflow`)。`min-height` 24px 以上(目安 44px)。accessible name は「技名、タイプ名、分類(ぶつり/とくしゅ)、威力 N」
   (変化技の行が渡された場合は「へんか」で威力なし)。色・余白はトークンだけ。動きは操作時のみ(パネルの出入りは `duration.*`、reduced-motion で 0)。
4. **分類アイコン**: `web/src/ui/Icon.tsx` に `movePhysical`・`moveSpecial`・`moveStatus` を足す(G-05 の既定案: ぶつり=放射状の衝撃、とくしゅ=同心の波、へんか=半分の円。
   線画・24×24・`currentColor`・分類で色を変えない)。読み上げ用の名前は「ぶつり」「とくしゅ」「へんか」を `label` で渡す(用語集に足す。漢字の物理/特殊にしない=見える語と合わせる)。
   行・トリガーの分類アイコンは名前つき(`role="img"`)。iOS の `PopSymbol` と意味をそろえる(ADR-0527)。
5. **並びはタイプ順だけ**(`web/src/domain/moveOrder.ts` の `orderMoves(moves, types)`): `types`(`master.typeChart.types`)の並びで群にし、**群の中は五十音順**
   (`Intl.Collator("ja")`、同順位は技 ID の昇順)、表に無いタイプは最後にタイプ ID の昇順。ADR-0335 の type モードと同じ結果。習得順を同順位の決めに使わない
   (入力の順に依らず決定的にするため)。**既定の技の自動選択は learnset の最初のダメージ技のまま**(表示順と無関係。`firstDamagingMove`)。
6. **未選択の行**(ADR-0333 §4。計算画面): `value=""` のとき、先頭に `aria-disabled="true"` の「技を選んでください」行をタイプ順の外に置く(押しても選べない)。
7. **廃止**: `MoveSortControls`・`useMoveSort`・`app/moveSortStorage`・`i18n/moveSort.ts`・`domain/moveSort.ts`(`orderMoves` に置換)・localStorage `pokecalc.moveSort`
   (読まない・書かない・消さない。残っていても無視)。ADR-0335 に「G-01 により一部置き換え」を追記した。
8. **範囲**: Web の計算画面・逆算画面の技欄。調整(`AdjustScreen`)・判定(`JudgeScreen`)・構築・タイプバランスの技の `<select>` は今回は変えない(別タスクで同じ `MovePicker` を使える)。
   engine・API・マスタの契約は変えない。

## 既存テストの扱い(絶対ルール 6)

廃止する機能の検査だけを削除する。ユーザーが機能の廃止を明示的に決めたため(テストを弱めて通すのではない)。残る規則は同じ入力・同じ期待値で付け替える。

| テスト | 扱い | 理由 |
|---|---|---|
| `domain/moveSort.test.ts` の learnset・kana・isMoveSortOrder・種類の検査 | 削除 | 習得順・五十音順の並び・種類を廃止 |
| `domain/moveSort.test.ts` の type・五十音の比較(群の中)・未知のタイプ・入力を書き換えない | `domain/moveOrder.test.ts` に付け替え(同じ入力・期待値。追加済み) | 規則は残る |
| `app/moveSortStorage.test.ts` | 削除 | 保存を廃止。「残っていても無視」は `*.movePicker.test.tsx` の C-5・R-3 が確かめる |
| `App.moveSort.test.tsx`・`CalcScreen.moveSort.test.tsx`・`ReverseScreen.moveSort.test.tsx` | 削除し `*.movePicker.test.tsx` に置換 | チップ・記憶・習得順・五十音順を廃止。「タイプ順」「既定の技が変わらない」「選択が保たれる」「F-09 の未選択の行」「オンラインの learnset でも同じ」は新テストと下の付け替えで維持 |
| `CalcScreen.*.test.tsx`・`ReverseScreen.*.test.tsx`(damagingMoves・online・conditions・attackerStats・defenderRanks・favoriteRestore・本体)の `moveSelect()` | 付け替え(意図は同じ) | `<select>` → ピッカー。`test/movePicker.ts` の `moveTrigger`/`chooseMove`/`selectedMoveId`/`listedMoveIds` に置換。`toHaveValue`→`selectedMoveId`、`selectOptions`→`chooseMove`、`getAllByRole("option")`→開いて数える。「変化技が出ない」「0 件」「disabled」「オンラインの件数」の期待値は変えない |

## 受け入れ条件(テストで固定)

`MovePicker.test.tsx`(P-1〜P-8)、`CalcScreen.movePicker.test.tsx`(C-1〜C-6)、`ReverseScreen.movePicker.test.tsx`(R-1〜R-3)、
`domain/moveOrder.test.ts`、`ui/Icon.moveCategory.test.tsx`、`moveSortRemoved.test.ts`。

## 補足(critic 指摘)

- 技が 0 件(攻撃側の種族を選んだ後)のときトリガーを disabled にする。開いても空のパネルが出るだけで、選べるものが無いため
  (旧 select は空でも有効だったが、操作できない部品を有効に見せない)。種族が未選択のときは従来どおり有効のまま。
- 古い localStorage の `pokecalc.moveSort` は読まず、消しもしない(無害。残っていてもタイプ順)。
- 検索欄は日本語入力の変換中(`isComposing`)の Enter・矢印を奪わない。
- 結果パネル側の分類名の表記(`resultText.moveCategory`。判定画面が使う「物理/特殊/変化」)は今回の対象外。判定画面は非表示(F-07)で、
  作り直し時に用語集の語(ぶつり/とくしゅ/へんか)へ揃える。
