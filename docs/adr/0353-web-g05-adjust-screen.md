# ADR-0353: 調整画面の作り直し(G-05 / I-web-13h)

- 状態: 採用(2026-10-11)
- 関連: ADR-0339(方針)・ADR-0344(共通の部品)・ADR-0319(調整画面)・ADR-0331(目標モード・メガのストーン固定)・ADR-0336(結果の強弱)・ADR-0351/0352(同じ作法)

## 決めたこと

docs/design.md「画面ごとの方向」の調整の行を、**見た目だけ**実装した(機能を持つ ec レーンと合意)。ロジック・`adjustGoals.ts`・`adjustClient.ts`・`adjustRequest.ts`・`adjustFormat.ts`・API 呼び出し・検証・文言は変えない。

1. 領域の見出し(h2)にアイコン(`ui-heading`)。
2. 自分・相手は選ぶと `PokemonCard`(名前 h3・タイプバッジ・持ち物の印)。メガ種族の持ち物の欄は従来どおりストーン固定(disabled・理由)で、カードの印にもストーン名(`megaStoneLabel`)を出す。目標の相手のカードも同じ。
3. 固定する SP・上限は − / + 付き(`ui-stepper`)。数値欄は type=number のまま文字列の state を使い、範囲外・空は従来どおり送信時に誤りとして出す(`ui/Stepper` は範囲に収めてしまい誤りの経路が消えるので使わず、素早さ・計算と同じく見た目だけ `ui-stepper`)。ボタンは範囲(0〜32)に収めた値へ直す。
4. 「目標を追加」は「+」の空きタイル(`Tile empty`)。上限では無効で、理由は aria-describedby のまま(`Tile` に任意 prop `describedBy` を足した)。外したあとのフォーカスは「目標を追加」へ戻る(包みの div から button を引く)。
5. 目標カード(fieldset)の legend に種類のアイコン(装飾)、相手を選ぶと小さな `PokemonCard`。
6. 結果: 振り方の文の下に能力ごとの横棒(`SpBars`。aria-hidden。数値は直前の文が持つ)と大きい数字(`--font-size-title`)。結論(`adjust-screen__verdict`)の先頭に成功(チェック)/失敗(×)のアイコン。色の組は既存(success/danger とその soft)のみで、新しい組は無い。横棒の幅は固定の遷移なし(常時動くものなし)。
7. 選択欄・数値欄は 44px。

## 適用しなかったもの

(追記 2026-10-11: 種類・技・振り方の部品化と説明の文の「説明」ボタン化は ADR-0354 で適用した。以下は当時の判断。)

- 目標の種類・調整・技の `SegmentedControl`/`MovePicker` 化: `AdjustScreen.goals.test.tsx` などが `selectOptions` で固定しており、機械的な付け替えでは済まない(13f の方針と同じ)。モード(6 択)は既存どおりチップ。
- 説明の文を「説明」ボタンの奥へ: 指数の注(`indexNote`)・固定 SP の補足は表示・aria-describedby を既存テストが固定している。文言の削除は語の変更になるため見送り(後続で必要なら)。
- 能力ごとの横棒は結果の振り方のみ(耐久の指数などは数字のまま)。

## 既存テストの付け替え(理由つき・assertion は弱めていない)

- `AdjustScreen.visual.test.tsx` 1 件: 「目標を追加」の見た目の class 検査を `ui-button ui-button--secondary` から `ui-tile ui-tile--empty` へ(部品が変わったため)。「外す」の検査は不変。

## iOS・ec レーンへ

decisions/2026-10-11-web-g05-adjust.md。語の変更なし。
