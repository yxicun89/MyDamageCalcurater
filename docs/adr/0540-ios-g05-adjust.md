# ADR-0540: iOS の見た目の作り直し 第 4 弾(調整)

- 状態: 採用(2026-10-13。仕様とテスト先行。実装は plan `I-ios-27`)
- 関連: ADR-0536(第 1 弾。部品 `PopHelpButton`・`PopIconButton`・`PopEmptyLabel`・`PopSymbol`・`CommonPartsText`)、ADR-0537(第 2 弾。`TeamSPControl` の作法)、
  ADR-0539(第 3 弾=逆算。`MasterSearchSheet` の共用部の変更)、ADR-0339(Web の方針 G-05)、ADR-0344(共通の部品)、ADR-0353・ADR-0354(Web の調整画面)、
  ADR-0502(AJ7 調整画面)、ADR-0525(F-11 目標方式)、ADR-0533(ランクのメーター)、ADR-0534(API 不可時の表示)、ADR-0501 追記章「G-05 調整の作り直し」、
  docs/design.md「見た目の作り直し方針(G-05)」「iOS の追い方(G-05)」、
  docs/ai-shared/decisions/2026-10-11-web-g05-adjust.md・web-visual-policy-g05.md・web-g05-components.md
- 担当: iOS(`ios/PokeCalc` の `AdjustScreen*.swift` 5 ファイル、新規 `PopPokemonCard.swift`、`PokeCalcCore` の調整の文言・判定、`PokeCalcDesign` の `PopSymbolNames`)
- 前提: ADR-0536・0537 の実装(main に入っている)。ADR-0539(逆算。PR 未マージ)が変える `MasterSearchSheet` の共用部
  (補足を「説明」ボタンの奥へ・閉じるをアイコンだけ・検索中の静止表示)には**従い、調整では重複して変更しない**。
  種族の検索シート(`SpeciesSearchSheet`)は共用のまま使うので、逆算の弾が入ると自動で調整にも及ぶ。

## 背景

第 3 弾(逆算)に続き、調整画面を方針(文で説明しない)に直す。Web は ADR-0353(構造)と ADR-0354(目標カード)で、自分・相手をポケモンカード、
固定する能力ポイントを − / + 付き、目標の追加を「+」のタイル、目標の種類を区切りボタン、相手を「選んだ相手から」のタイル、補足を「説明」ボタンの奥、
結果を能力ごとの横棒 + 成功/失敗のアイコンにした。iOS はこの構造と語に追う。
**変えないもの**: ランクのメーター(ADR-0533。調整画面には出ない)、API 不可時の表示(ADR-0534。`ErrorBannerView`・`LoadRetryButton`)、
既存の識別子と読み上げ名(新規だけ追加)、ロジック・API・語(下の 1 件の言い換え以外)。

## 決定

1. **ポケモンカード**(新規 `PopPokemonCard.swift`。画像かエンブレム + 名前 + タイプバッジ + 持ち物の印。大小の 2 種): 自分・相手・目標の相手の種族の欄は、
   選んだら種族のボタンの見た目をこのカードにする(押すと従来どおり `SpeciesSearchSheet`。ボタンの識別子と読み上げ名は不変)。未選択は従来の「ポケモンを選ぶ」。
   持ち物の印は `AdjustViewModel.ownItemMark`(選んだ持ち物。メガ種族は固定のストーン名)・`opponentItemMark`・`goalOpponentItemMark(_:)`(メガ種族だけストーン名)。
   計算・逆算と共有の `SpeciesHeaderMenuLabel`(`lineLimit`・縮小つき)は使わない(共有側は計算の弾で扱う)。
2. **見出し**: カードごとに Web と同じアイコン(自分 = 構築、調整の内容 = 調整、相手 = 入れ替え、目標 = 判定〈盾〉、結果 = 計算、覚えるポケモン = 検索)。
   `AdjustCard` は見出しのアイコンを受け取る。`PopSymbolNames.judge`(盾 = `shield`)と `PopSymbol.judge` を足す。
3. **増減ボタン**: 固定する能力ポイント 6 欄と上限(耐久 H・B・D / 攻撃 A か C と S)は − / +(`PopIconButton`、`PopSymbol.minus/add`)+ 数値欄(数字キーボードに「完了」)。
   上限は Menu の Picker(0〜32 の 33 項目)をやめる。押せるかと丸めは `AdjustNumberStep`(Core。欄の文字は ASCII の数字だけ、空・読めない文字は 0、範囲外は端に丸めてから ±1)。
   読み上げ名は用語集の語「<欄の名前>を増やす/減らす」(`AdjustText.stepperIncrease/Decrease`)。範囲外の文字を打ったときのエラーは従来どおり「調整する」で出す(送らない)。
   目標の発数の − / + も直書きのアイコンをやめて `PopIconButton`(読み上げ名・識別子は不変)。
4. **補足は「説明」ボタンの奥**(`PopHelpButton`。名前 `CommonPartsText.help`、アイコン `PopSymbol.help`。開く中は文面を変えない): 固定する能力ポイント・最低素早さ(今は
   `accessibilityHint` だけで見えない)・先に使う技(今は常時の caption)・指数の注(今は常時の caption)・「目標を指定すると…」(今は常時の caption)。
   欄から説明の文への読み上げの結びつき(`accessibilityHint`)は残す。
5. **状態の一言**: 空の結果・目標が無い・未対応の印・「覚えるポケモン」の空 = `PopNoticeView`(`.empty`/`.info`)、計算中・覚えるポケモンの読み込み中 =
   `PopNoticeView(.loading)`(静止したアイコン + 一言。`ProgressView` をやめる)、覚えるポケモンのエラー = `PopNoticeView(.error)`。語は不変。
   送信のエラーと「もう一度」は ADR-0534 のまま(`ErrorBannerView`・`LoadRetryButton`)。
6. **選択肢**: モード 6 つ・耐久の基準・攻撃の分類は、縦の丸(`largecircle`)の列をやめて、全幅のチップ(`PopChipStyle`。選択中は塗り + `PopSymbol.done` のチェック。
   読み上げは選択の特性)。AX5 でも縦に積む。5 択以上・文字だけの短い選択肢(性格・特性・持ち物・技・発数・確率)は Menu のまま(方針)。
7. **目標カード**: 見出しに種類のアイコン(素早さ = 速度、耐える = 判定〈盾〉、倒す = 計算)、選んだ相手の小さなポケモンカード、種類は 3 択の**区切りボタン**
   (アイコン + 短い名前。選択中はチェック。`adjustGoal-n-kind-<kind>` と選択の特性は不変。AX5 は縦に積む)。「選んだ相手から」(`AdjustText.goalRecentOpponents`):
   `AdjustViewModel.recentOpponents`(相手の領域の相手 → 各目標の相手の順・同じ種族は 1 つ)を絵つきのタイルで並べ、押すとその目標の相手になる(従来の選択・検索も残す)。
   「目標を追加」は「+」のタイル(識別子 `adjustAddGoalButton` は不変。上限で無効 + 理由の文は従来どおり)。確率のピルは `minimumScaleFactor` をやめて折り返す。
   目標の技のシートは、閉じるをアイコンだけ(`PopIconButton`・読み上げ「閉じる」・識別子 `adjustGoalMoveSheetClose` 不変)、行をタイプバッジ + 技名 + 威力にする
   (識別子 `adjustGoalMoveOption-*`・「未選択」の行・選ぶと閉じる、は不変)。**共用の `MoveSearchSheet` には載せない**(`AdjustViewModel` に技の検索を足す広がりになるため。後続)。
8. **結果**: 結論(倒せる/耐えられる/目標を満たす/素早さの目標・目標ごとの結果)は 成功 = `PopSymbol.done` / 失敗 = `PopSymbol.close` のアイコン(装飾。文が名前を持つ)+ 従来の文。
   振り方(能力ポイント)の文の下に、能力ごとの横棒(装飾・読み上げない。幅 = `AdjustSPBar.fraction(sp:)` = 能力ポイント / 32)。振り方・指数・HP の 16 の倍数は小見出し付きのカード。
   「覚えるポケモン」の閉じるはアイコンだけ(`adjustLearnersClose`・読み上げ「閉じる」)、続きを読み込むは文字のボタン(36pt 以上)。
9. **語**: Web と同じ。足すのは「選んだ相手から」の 1 つ(用語集に追加)。**1 件の言い換え**: `goalNatureNotFoundMessage` の「マスタ」→「データ」
   (用語集の禁止語。Web は「データ」。`GlossaryTests` の対象に目標の文言を足して検出)。
10. 守る決まり(ソース検査 `G05AdjustSourceRulesTests` が 5 ファイルで見る): `ProgressView(`・`repeatForever`・`lineLimit`・`minimumScaleFactor`・色のリテラル/システム色・
    SF Symbols 名の直書きを使わない。パレット K のトークンだけ。動き(縮み)は視差効果を減らすで 0 秒。常時アニメ無し。

## 採らなかった案

- 目標の技のシートを共用の `MoveSearchSheet` に載せ替える: `AdjustViewModel` が `MasterMoveSearchProviding`(検索語・検索中の状態)を持つ必要があり、ロジックの広がりが大きい。
  Web も learnset 内の選択(`MovePicker`)で足りている。シートの閉じるのアイコン化など共用部の見た目は、逆算の弾(ADR-0539)の `MasterSearchSheet` の変更に従う。
- 性格・特性・持ち物・技を区切りボタンやタイルにする: 方針と Web は「5 択以上・文字だけの短い選択肢は select のまま」。
- 結果の横棒に読み上げの名前を付ける: 同じ数値は直前の文が持つ(Web も装飾)。重複して読ませない。
- 「覚えるポケモン」の行に画像を付ける: 行は種族名だけの一覧(ADR-0502 §7)。調整の弾の対象外にして、広がりを抑える。
- メガシンカの固定の理由(`lockReason`)を説明ボタンに入れる: 状態の通知(文を置いてよい)で、1 カード 1 行以内。

## 実装時の補足

- 目標の相手のカードは「小さなポケモンカード」ではなく regular(幅いっぱいの横並び)にした。読みやすさと AX5 での折り返しのため。compact(縦のタイル)は「選んだ相手から」のタイルだけに使う。ADR-0501 の「目標カードの見出し」の記述も同じ。
