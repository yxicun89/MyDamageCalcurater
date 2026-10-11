# ADR-0354: 調整の目標カードを「見て分かる」形に(I-web-18)

- 状態: 採用(2026-10-11)
- 関連: ADR-0353(調整画面の作り直し。「適用しなかったもの」を本 ADR で適用)・ADR-0331・ADR-0339・ADR-0341(技ピッカー)・ADR-0344

## 決めたこと

機能を持つ ec レーンが「文で説明せず見て分かるように」を求めたため、ADR-0353 で見送った項目を `AdjustScreen.tsx`・`.css` だけで適用した。ロジック・`adjustGoals.ts`・`adjustClient.ts`・`adjustRequest.ts`・`adjustFormat.ts`・API 呼び出し・検証・語(`goalRecentOpponentsLabel` の 1 語を追加)は不変。

1. 目標の種類は 3 択の `SegmentedControl`(radiogroup。アイコン + 短い名前。選択中はチェックの印)。追加直後のフォーカスは選択中の radio へ。
2. 目標の相手は、これまでに選んだ相手(相手の領域・各目標の相手。重複なし)を `PokemonIcon` つきの `Tile`(aria-pressed + チェック)で並べ、押すと相手になる。種族の select / 検索欄(`SpeciesField`)は残す(一覧が使えない既定のオンラインでは検索欄)。専用の画像ピッカーは作らない(データの流れが無いものは足さない)。
3. 目標の技は `MovePicker`(ダメージ技の絞り込みは従来どおり `choice.moves`)。「先に使う」は `noneLabel`(使わない)、ほかは未選択の行。`MovePicker` に任意 prop `visibleLabel`(見えるラベルを短く、読み上げ名は「目標 n の…」のまま)と `describedBy` を足した(計算・逆算・構築は不変)。振り方は選択肢が 4 以下なら `SegmentedControl`、5 以上(倒す)は select のまま。
4. モード(6 択)のチップは変えない。
5. 説明の文は「説明」ボタン(`HelpButton`)の奥へ: 指数の注(`indexNote`)・「目標を指定すると…」(`minSpNotRequested`)は説明ボタンのみ。固定 SP の補足・最低素早さの補足・先に使う技の補足は、説明ボタン + 欄の aria-describedby の先として画面外(視覚的に隠すだけ。display:none にしない)に同じ文を置き、支援技術からは今までどおり読める。

## 既存テストの付け替え(理由つき。期待の意味は変えていない)

- `AdjustScreen.goals.test.tsx`: 部品が select から radio / 技ピッカーに変わったため、`addGoal` と読み取りの helper(`kindRadio`・`moveValue`・`chooseMove`・`moveRowIds`・`checkedPresetLabel`)へ。種類の既定・選択肢・振り方の選択肢と既定・技の選択肢(技ピッカーはタイプ順なので順序でなく集合で比較)・フォーカス・種類変更時の保持と既定への戻し・自分の技の無効化は同じ内容を検査。G7 は、種類・振り方(radiogroup)は名前が見えるラベルを含み同じ文字が見えること、select・技ピッカーは従来どおり `<label>` が短いラベルであることを検査。未使用になった `optionTexts` を削除(検査ではない)。
- `AdjustScreen.visual.test.tsx`: 目標の種類が radiogroup(`ui-segmented`)になった検査、振り方の選択(select → radio)、指数の注は説明ボタンを開いてから `adjust-screen__detail` の中にあることを検査。
- `AdjustScreen.test.tsx`: 指数の注・最小の振り方の注は説明ボタンを開いてから同じ文を検査。aria-describedby の検査は不変。
- 新規 `AdjustScreen.goalsUi.test.tsx`(6 件)。

## 確認

375/1280 × ライト/ダークで目標 3 種(メガ固定の自分を含む)を目視(使い捨ての Playwright・コミットなし)。色は既存トークンのみ(新しい色の組なし)。ボタン・ピッカーは 44px 以上。常時動くものなし。CSS は gzip 9.1KB(予算 30KB)。

## iOS・ec レーンへ

decisions/2026-10-11-web-g05-adjust.md の追記。
