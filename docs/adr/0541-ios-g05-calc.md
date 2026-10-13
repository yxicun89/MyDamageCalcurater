# ADR-0541: iOS の見た目の作り直し 第 5 弾(計算)

- 状態: 採用(2026-10-13。仕様とテスト先行。実装は plan `I-ios-28`)
- 関連: ADR-0536(第 1 弾。部品 `PopHelpButton`・`PopIconButton`・`PopEmptyLabel`・`PopSymbol`・`CommonPartsText`)、ADR-0537(第 2 弾。構築)、
  ADR-0539(第 3 弾=逆算。`MasterSearchSheet` の共用部の変更と、`SpeciesHeaderMenuLabel` の宿題の送り先)、ADR-0540(第 4 弾=調整。`PopPokemonCard`・`PopSymbol.judge`)、
  ADR-0339(Web の方針 G-05)、ADR-0344(共通の部品)、ADR-0345(Web の計算画面)、ADR-0329(攻撃・特攻のブロック)、ADR-0535(連続技)、
  ADR-0533(ランクのメーター)、ADR-0534(API 不可時の表示)、ADR-0501 追記章「G-05 計算の作り直し」、
  docs/design.md「見た目の作り直し方針(G-05)」「iOS の追い方(G-05)」、
  docs/ai-shared/decisions/2026-10-11-web-g05-calc.md・web-visual-policy-g05.md・web-g05-components.md
- 担当: iOS(`ios/PokeCalc` の Calc 系 6 ファイルと連続技の節 3 ファイル、`PopComponents.swift` の `PopSymbol`、`PokeCalcCore` の計算の文言・判定)
- 前提: ADR-0536・0537(main に入っている)に加えて、ADR-0539(逆算)・ADR-0540(調整)の実装が main に入っていること。
  計算が使うのは、逆算が変える `MasterSearchSheet` の共用部(補足 = 「説明」ボタンの奥・閉じる = アイコンだけ・検索中 = 静止した表示)と、
  調整が新設する `PopPokemonCard`・`PopSymbol.judge`。**計算はこれらに従い、重複して変更しない**(`MasterSearchSheet.swift` は触らない)。

## 背景

第 3・4 弾に続き、計算画面を方針(文で説明しない)に直す。Web は ADR-0345 で、攻撃側・防御側を「見出し + ポケモンカード + 選ぶ欄(ポケモン・持ち物・特性)」、
SP を − / + 付き、性格補正を区切りボタン、条件をチップ、結果の行を「大きいダメージ幅 + 確定数のバッジ + バー + 小さい補足」、技の相性を枠だけのバッジにした。
iOS はこの構造と語に追う。iOS が既に方針どおりの点は変えない。
**変えないもの**: ランクのメーター(ADR-0533。`RankMeterView.swift` は触らない)、API 不可時の表示(ADR-0534。`ErrorBannerView`/`PopNoticeView`・`LoadRetryButton`「もう一度」)、
連続技の節の機能・文言・識別子(ADR-0535。見た目の部品の置き換えと規約の確認だけ)、既存の識別子と読み上げ名(新規だけ追加)、
ロジック・API・計算結果、攻守入れ替えの読み上げ名「攻守を入れ替える」(Web は「攻守入れ替え」。既存の読み上げ名は付け替えない)、
防御側の特性の「指定なし」(Web の「おまかせ(…)」とは機能の語が別。`AbilityPickerLabels` は不変)。

## 決定

1. **見出しとカード**: `CalcScreenView.cardsRow` で、各カードの上に見出し(`PopHeading`。「攻撃側」= `PopSymbol.damage`、「防御側」= `PopSymbol.judge`。
   識別子 `calcAttackerHeading`・`calcDefenderHeading`)。見出しはカード(`attackerCard`・`defenderCard`)の外・上に置き、入れ替えの動き(`.id(speciesKey)`)に巻き込まない。
   カードの顔(種族のボタンの見た目)は `PopPokemonCard`(画像かエンブレム + 名前 + タイプバッジ + 持ち物の印)。種族が無い間(読み込み中)は従来の `MenuLabelChip`(`-`)。
   押す操作・識別子 `attackerSpeciesPicker`・`defenderSpeciesPicker`・読み上げ名(種族名)・ヒント「ポケモンを変える」は不変。
   持ち物の印は新しい Core の `CalcViewModel.attackerItemMark`(選んだ持ち物の名前。メガ種族は固定のストーン名。なしは nil)・`defenderItemMark`
   (防御側はメガ種族の固定のストーン名だけ。比較は結果の下のチップのため)。
2. **特性の欄はカードの中へ**: 攻撃側・防御側の特性(`calcAttackerAbilityPicker`・`calcDefenderAbilityPicker`)を「詳細」からカードの持ち物の欄の下へ移す(Web のカードは ポケモン・持ち物・特性 の順)。
   識別子・値(`accessibilityValue`)・選択肢は不変。防御側の選択肢の読み込み(`loadDefenderAbilityOptions`)は従来どおりカードの `.task`。
   「詳細」には 急所・やけど・天候・フィールド・防御側の壁・攻撃側/防御側のランクだけが残る(Web の `CalcConditionsPanel` と同じ)。
3. **持ち物の注記**: 上限の注記(`attackerItemLimitHint`)は `PopNoticeView(.info)`(アイコン + 一言)。メガ固定の理由(`attackerItemLockReason`・`defenderItemLock`)は状態の通知なので文のまま(Web も同じ)。
4. **攻守入れ替え**: アイコンだけのボタン(`PopIconButton` + `PopSymbol.swap`。36pt 以上)。識別子 `swapSidesButton`・読み上げ名は不変。入れ替えの動きは操作時だけ・視差効果を減らすで 0 秒(従来どおり)。
5. **技の欄**: トリガーを Web の並び(タイプバッジ + 技名 + 分類のアイコン + 威力の数字)にする。分類のアイコンは新規 `PopSymbol.moveCategory(_:)`
   (物理 = `figure.boxing`・特殊 = `sparkles`・変化 = `circle.dashed`。検索シートの行と同じ割り当てを `PopSymbol` に出す)。右端の矢印は `PopSymbol.picker`。
   要約の文(`moveSummaryText`「威力37 / 物理 / ばつぐん(×2)」)は**見える文字にせず**、ボタンの読み上げの値(`accessibilityValue`)に入れる(Core の `moveSummaryText` は不変)。
   読み上げ名は技名を含む(従来どおり)。識別子 `movePicker` は不変。`lineLimit`・`minimumScaleFactor` は外す。
6. **プリセットのピル**(`attackerPreset-*`): `lineLimit(1)`・`minimumScaleFactor` を外して折り返す(AX では縦に積む従来の分岐は維持)。選択中は塗り + `PopSymbol.done` のチェック(色だけに頼らない)。
   選べないピルの減光・識別子・選択の特性は不変。
7. **攻撃・特攻のブロック**: SP の数値欄の左右に − / +(`PopIconButton`、`PopSymbol.minus`/`PopSymbol.add`。36pt 以上)。
   読み上げ名は用語集の語「<欄の名前>を減らす」「<欄の名前>を増やす」(`AttackerStatLabels.spDecrease/spIncrease`。例「攻撃のSPを増やす」)、識別子は `attackerSPDecrease-<stat>`・`attackerSPIncrease-<stat>`。
   押せるかと押したあとの文字は Core の `AttackerSPStep`(Web の `AttackerSPField` と同じ規則: 空欄は 0・整数でない文字は 0 から数える・範囲外の整数は端へ丸める・0 で −、32 で + が止まる)。
   数値欄は従来のまま(`attackerSPField-<stat>`・数字キーボードの「完了」)。欄の不正な値(33・abc)を誤りとして出して計算しない仕様(ADR-0518)は不変。
   SP の誤り(`attackerSPError-<stat>`)は `PopNoticeView(.error)`、構築の個体で計算中の注記(`attackerStatTeamNotice`)・変化技の注記(`calcStatusMoveNotice`)・
   ダメージ技なしの注記(`calcNoDamagingMovesNotice`)は `PopNoticeView(.info)`(アイコン + 一言。識別子は文に付けて不変)。
   性格補正の 3 択(上昇・補正なし・下降)は**今の自前の区切りボタンを維持**(識別子 `attackerNature-<stat>-<choice>`・選択の特性・AX の縦積み・選べない選択肢の無効表示が既に方針どおり。
   `Picker(.segmented)` は 1 択だけ無効にできず、大きい文字で文字を縮める。逆算の側と同じ判断)。選択中に塗り + `PopSymbol.done` のチェックを足す。
   「同じ向きにできない理由」の 1 行(`attackerSameDirectionReason-<stat>`)は理由の 1 行なので残す。
8. **結果の行**(Web の `calc-results__row` の順): 1 行目 = ダメージ幅の大きい数字(`TextStyleToken.resultPercent`、識別子 `calcResultPercent-<id>`)+ 確定数のバッジ
   (brandPrimary の塗り + onPrimary の文字・見出しの大きさ、識別子 `calcResultKO-<id>`。段階が変わったときの弾みと触覚は従来どおり、視差効果を減らすで無し)、
   2 行目 = ダメージバー(従来のまま)、最後に 調整名(+ 持ち物名)を caption で(新規 `calcResultPreset-<id>`)・特性(`calcResultAbility-<id>`)・未対応の印(`calcResultUnsupported-<id>`)。
   `lineLimit`・`minimumScaleFactor` は外す(AX では数字とバッジが縦積みに落ちる従来の `ViewThatFits` を維持)。
   **技の相性**(「ばつぐん(×2)」「等倍」ほか。`EffectivenessLabel.text`)は、結果の先頭に**枠だけのバッジを 1 つだけ**(新規 `calcEffectivenessBadge`。`viewModel.moveEffectiveness` が nil なら出さない)。
   結果の一覧のコンテナに読み上げ名「計算結果」(新規 `calcResults`)。全行共通の未対応の印(`calcUnsupportedNotice`)は `PopNoticeView(.info)` に。
9. **状態の一言**: 計算中は `ProgressView` をやめ `PopNoticeView(.loading)`(静止したアイコン + 「計算中」。識別子 `calcLoadingIndicator` は文に付けて不変。場所は従来の固定の高さの枠)。
   `ErrorBannerView`(`calcErrorMessage`)・`LoadRetryButton`(`calcLoadRetry`)は ADR-0534 のまま。
10. **「詳細」**: 開閉ボタンは「下向きの山のアイコン(`PopSymbol.open`)+ 名前」。矢印の直書き(`chevron.up/down`)をやめ、開閉の状態は読み上げの値(「開いています」「閉じています」。素早さ画面の折りたたみと同じ語)で伝える。
    急所・やけど・天候・フィールド・壁は従来どおりチップ(`ChipButton`)。チップは**折り返す並び**(横スクロールの `ScrollView(.horizontal)` をやめ、連続技の節の `ComboWrapLayout` を使う)で、全部が見える。
    見出し(小見出し)の `lineLimit(1)` は外す。ランクのメーターは不変。
    「持ち物で比較」のチップ(`defenderItemToggle-<id>`・読み上げ名「持ち物で比較: <名前>」)も同じ折り返す並び。上限の注記(`calcItemVariantLimitHint`)は `PopNoticeView(.info)`、
    メガ固定の理由(`defenderItemCompareLockReason`)は文のまま。
11. **チップの部品**(`ChipButton`): `lineLimit(1)` を外す(横スクロールをやめるので 1 字ずつ縦に折り返らない形は不要。長い名前は折り返す)。最小の高さ 36pt。選択中は塗り + `PopSymbol.done`。
    選べないチップの減光・読み上げ名・識別子・`isSelected` の特性は不変。
12. **共有部品の宿題**(逆算 ADR-0539 §「採らなかった案」から計算へ送られたもの): `SpeciesHeaderMenuLabel` の矢印の直書きを `PopSymbol.picker` に、`lineLimit(2)`・`minimumScaleFactor` を外す。
    `MenuLabelChip` の矢印の直書きを `PopSymbol.picker` に、`lineLimit(1)`・`minimumScaleFactor` を外す。`TypeBadgeView` の `lineLimit(1)` を外す(`fixedSize()` で自然な幅の 1 行は保つ)。
    `SpeciesEmblemView`・`TypeBadgeView` の `?? .white`(トークンの代わりのシステム色)を `ColorToken.onPrimary.color` に。`SpeciesHeaderMenuLabel` 自体は逆算・構築が使うので残す(計算は使わなくなる)。
13. **連続技の節**(機能・文言・識別子は不変): `ProgressView`(`comboLoading`)→ `PopNoticeView(.loading)`(識別子・文言「連続技を計算中…」は不変)。
    SF Symbols の直書きを `PopSymbol` 経由に(`plus` → `add`、`xmark` → `close`、`arrow.up/down` → `up/down`、`chevron.*` → `picker`/`open`、`checkmark.circle.fill` → `done`)。
    `ComboIconButton` は `PopIconButton` に置き換えて廃止(識別子 `comboStepUp/Down/Remove-<n>`・読み上げ名は不変。36pt 以上)。オフのトグルの丸(`circle`)は出さない(オンはチェック + 塗り)。
    `lineLimit` 等は既に無い(確認のみ)。注記(`comboNote` ほか)の文は状態・条件の説明として残す(文言は不変)。
14. **語**(Web の `calcScreenText`・`attackerStatText` と同じ。Core の `CalcScreenText`・`AttackerStatLabels` の追加分): 「攻撃側」「防御側」「計算中」「計算結果」「開いています」「閉じています」、
    「<欄>のSPを増やす」「<欄>のSPを減らす」。禁止語なし。常に画面に出る語に「。」を置かない。
15. 守る決まり(ソース検査 `G05CalcSourceRulesTests` が Calc 系 6 ファイル + 連続技 3 ファイルで見る。`RankMeterView.swift`・`MasterSearchSheet.swift` は対象外):
    `ProgressView(`・`repeatForever`・`TimelineView`・`lineLimit`・`minimumScaleFactor`・色のリテラル/システム色(`?? .white` を含む)・SF Symbols 名の直書きを使わない。
    パレット K のトークンだけ。動き(縮み・弾み・入れ替え・開閉)は視差効果を減らすで 0 秒。常時アニメ無し。タップは 36pt 以上。

## 採らなかった案

- 性格補正を `Picker(.segmented)` に作り替える: 1 択だけ無効にできない(「同じ向きにできない」選択肢の表現が壊れる)・大きい文字で文字を縮める・既存の識別子/選択の特性/UITest が広く壊れる。
  今の自前の 3 等分ボタン(AX で縦積み)が既に区切りボタンと同じ見た目・操作。チェックの印だけ足す。
- 持ち物の比較を Web の「持ち物の候補も比較」1 つのチップにする: iOS は比較する持ち物を選べる(issue #110・ADR-0509)機能で、機能の違い。チップの並びと折り返しだけ Web に揃える。
- 防御側のカードに持ち物の選択欄を足す: 防御側の持ち物は比較のチップで選ぶ(機能)。カードの印はメガの固定だけ。
- `defenderItemLock`(「固定: ○○のメガストーン」の 1 行)を印と重複するので消す: 識別子を使う既存テストがあり、固定の理由は状態の通知(1 カード 1 行以内)。残す。
- 技の要約の見える文を「威力」「分類」の別バッジ 2 つにする: 数字と分類アイコンで足りる(Web も数字だけ)。読み上げの値に全部入れる。
- `MasterSearchSheet.swift` の技の行の分類アイコンの割り当てを `PopSymbol.moveCategory` に差し替える: 逆算の弾が同じファイルを変えるため触らない。
  同じ割り当てが 2 か所に残る(重複)。逆算の実装が main に入った後の後続の整理で、`MoveRowLabels.categorySymbol` を `PopSymbol.moveCategory` に委譲する。
- 横スクロールのチップ行を残す: AX5 で選択肢が画面外に隠れる(方針: 36pt 以上・隠さない)。Web も折り返す。
- ランクのメーター(ADR-0533)の `lineLimit(1)`・`minus`/`plus` の直書きを直す: この弾では変えない。
- 結果の行ごとに相性のバッジを出す: 相性は防御側の種族と技のタイプだけで決まり全行同じ。Web も先頭に 1 つ。
