# ADR-0538: iOS の素早さ画面を「結論 1 行 + 想定カード + 折りたたみ」にする

- 状態: 採用(2026-10-13。Web の I-web-34 / ADR-0610 §9 に iOS を揃える)
- 関連: ADR-0610 §9(Web の決定)、`docs/ai-shared/decisions/2026-10-13-web-speed-screen-simplify.md`、ADR-0501 追記章「素早さ画面の結論 1 行と折りたたみ」、
  ADR-0533(ランクのメーター。不変)、ADR-0534(API 不可時の表示。不変)、plan `I-ios-25`
- 担当: iOS(`PokeCalcCore` の `SpeedMatchupDisplayBuilder`・`SpeedMatchupLabels`、`SpeedMatchupOutput.swift`)

## 決定

1. 値・API・既存の文言の語は変えない。変えるのは出し方だけ(文言の正は `web/src/i18n/speed.ts`)。
2. **出力 1(最低ライン)**: 先頭に結論の 1 行(`SpeedMatchupDisplayBuilder.matchupConclusion`)。上を取れる想定 = どれかの性格に `movesFirst` がある想定
   (同速だけは数えない)。「N つの想定のうち M つで上を取れます」/ 全部「N つの想定すべてで上を取れます」/ 0「どの想定でも上を取れません」。
   トリックルーム中は「先に動けます」「どの想定でも先に動けません」。カードの見出しは「最速 ・ 素早さ 145」(`heading`)。
   一言見出し・行動順のバッジ・既存の `speedCardOpponentSpeed-*` は残す。
3. **性格の行はカードごとの折りたたみ(実装は自前の `SpeedDisclosure`。補足参照)**(題「性格補正ごとの最小 SP」。トリックルーム中は「…上限 SP」)。初期は閉じる。開くと従来の行(同速の併記を含む)。
4. **出力 2(これ以上)**: 結論の 1 行(`reachConclusion` = 最速の行の `slowerText`。例「最速 60 族まで抜ける」)+ 折りたたみ(`SpeedDisclosure`)
   「6 つの振り方ごとの一覧を見る」(初期は閉じる)。
5. **色**: 想定カードの端の帯は選んだ相手の先頭タイプ色、自分・相手の入力カードはそれぞれのポケモンの先頭タイプ色
   (`accentTypeID`)。未選択は `brandPrimary`。色は意味を担わない飾り(文字で言う)。
6. 守ること: パレット K のトークンのみ・タップ 36pt 以上・`lineLimit` 無し・常時アニメ無し・AX5 で横にはみ出さない・禁止語なし(G-05)。
   折りたたみの開閉アニメは操作時のみ。

## 結果

- 識別子は新規のみ追加(`speedConclusion`・`speedCardHeading-<id>`・`speedCardNaturesToggle-<id>`・`speedReachConclusion`・`speedReachListToggle`)。
  既存の識別子・読み上げ名は付け替えない。性格の行・同速・「これ以上」の各行は閉じている間は画面に無い。

## 実装での補足(2026-10-13)

- 折りたたみは `DisclosureGroup` ではなく、自前の `SpeedDisclosure`(題の `Button` + 開いたときだけ中身を描く)にした。`DisclosureGroup` は題の識別子を
  ボタンと内側の三角(`DisclosureTriangle`)の両方に付け、XCUITest の `descendants(matching: .any)[id]` が 2 件に見えて使えなかったため。
  題は 40pt の最小高(36pt の指定は丸め誤差で 35.99… になりうる)。読み上げの値は「開いています」「閉じています」。
- `SpeedMatchupLargeTextUITests` は検査する識別子が増え、AX5 のスクロールで 300 秒(run-xcode-tests.sh の既定)を超えた(実測 約 316 秒)ため、
  `executionTimeAllowance = 600`(ADR-0520 の上限)を setUp に足した。
