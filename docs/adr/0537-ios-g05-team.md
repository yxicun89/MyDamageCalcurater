# ADR-0537: iOS の見た目の作り直し 第 2 弾(構築の一覧・編集)

- 状態: 採用(2026-10-12。仕様とテスト先行。実装は plan `I-ios-23`)
- 関連: ADR-0536(第 1 弾。部品 `PopHelpButton`・`PopIconButton`・`PopEmptyLabel`・`PopSymbol` の追加分・`CommonPartsText`)、ADR-0339(Web の方針 G-05)、
  ADR-0344(共通の部品)、ADR-0347(Web の構築画面)、ADR-0350(Web の技の欄)、ADR-0522(F-08 構築の作り直し)、ADR-0527(技の行 G-01)、
  ADR-0532(構築編集の中のタイプバランス)、ADR-0533(ランクのメーター)、ADR-0501 追記章「G-05 構築の作り直し」、
  docs/design.md「見た目の作り直し方針(G-05)」「iOS の追い方(G-05)」、
  docs/ai-shared/decisions/2026-10-11-web-g05-team.md・web-g05-team-move-picker.md・web-visual-policy-g05.md・web-g05-components.md
- 担当: iOS(`ios/PokeCalc` の構築 7 ファイルと `MasterSearchSheet.swift` の技シート、`PokeCalcCore` の文言)
- 前提: ADR-0536 の実装(PR #774。`PopIconButton` などの部品)が main に入っていること。

## 背景

第 1 弾(ADR-0536)に続き、構築の一覧・編集を方針(文で説明しない)に直す。Web は ADR-0347(構造)と ADR-0350(技の欄)で、
一覧のカードの 6 枠・編集の 6 枠のタイル・アイコンだけの操作・SP の増減ボタンと数値の合計にした。iOS はこの構造と語に追う。
**変えないもの**: 構築編集の中のタイプバランスの節(`TeamBalanceSection`。ADR-0532)、ランクのメーター(ADR-0533)、
保存の明示・未保存で戻るときの 2 段階・「構築 N」の名前・識別子と読み上げ名(新規だけ追加)。

## 決定

1. **一覧**: 空は `ContentUnavailableView`(構築のアイコン + 「まだ構築がありません」。次の行動は上の[新しい構築]が示すので、ボタンは置かない)。
   読み込み中は `PopNoticeView(.loading)`(静止したアイコン + 「読み込み中」)。カードは名前の下に 6 つの枠(埋まった枠は画像かエンブレム、
   空きは破線の枠で読み上げない)・n/6体・最終更新。[開く]は 編集アイコン + 名前のまま、[削除]は**ごみ箱のアイコンだけ**(`PopIconButton`。
   読み上げ名は従来の「「名前」を削除」)。削除の確認(文)は取り消せない操作なので文のまま、「削除する」にごみ箱、確定に `.sensoryFeedback(.warning)`。
2. **編集のタイルの格子**(新規 `TeamSlotGridView.swift`): 画面の先頭の戻る行の次に、6 枠のタイルを `LazyVGrid`(通常 3 列、AX では 1〜2 列)で置く。
   埋まった枠はポケモンの小さなカード(画像かエンブレム・名前・タイプバッジ・持ち物の印。メガは固定のストーン名)、空きは破線の「+ N体目を追加」。
   押すとその枠の見出しまでスクロールする(`ScrollViewReader`。Web の「欄へ移る近道」。全枠の欄は従来どおり常に出す)。
   格子の読み上げ名は「メンバー」。タイルは 36pt 以上、押下の縮みは `accessibilityReduceMotion` で 0 秒。
3. **枠の操作**: 上へ・下へ・外すは**アイコンだけ**(`PopIconButton` + `PopSymbol.up/down/delete`)。読み上げ名と識別子は従来のまま。
   空の枠は「ポケモン」の欄だけ(案内の文 `emptySlotHint` は廃止。タイルの「+」が示す)。
4. **メンバーカード**: 「わざ」→「技」、ニックネーム欄の「(任意)」を外す、空の技の枠は「(なし)」(旧「技を選択」)。
   SP は `Stepper` をやめ、6 欄とも − / +(`PopIconButton`。`PopSymbol.minus/add`。0〜32、合計 66 で止まる)+ 数値欄(`TextField`、数字キーボードに「完了」)。
   見出しの文(旧「能力ポイント(合計 x/66)」)は数値の行「合計 66/66(残り 0)」+ 装飾のバーにする。読み上げ名は用語集の「増やす/減らす」。
   メンバーのエラー(`memberError`)は文のまま(エラー)で、アイコン + 文(`PopNoticeView(.error)`)。
5. **技ピッカー**(G-01 済みの `MoveSearchSheet` との差だけ。`MasterSearchSheet.swift` の技シート): 計算・逆算と同じ「タイプ順の群」(`groupsByType: true`)にする。
   先頭の「外す」を「(なし)」(識別子 `moveSearchRemoveButton` は不変。選ぶと枠が空く。空の枠でも出す)。他の枠で選んだ技は一覧から消さず、
   **薄く押せない行**にする(`disabledIDs`。自分の枠の技は選べる)。候補が learnset(変化技を含む)、マスタに無い ID は技名を ID のまま出す、
   読み上げ名(技名 + 値にタイプ・分類・威力)は既に Web と同じ。**採らない**: シートの閉じるをアイコンだけにする・検索中の `ProgressView`・案内の文の整理は
   計算・逆算と共有のため、計算・逆算の弾で扱う(この弾の対象外)。
6. **未保存・保存済み**: caption の文字だけをやめ、アイコン + 一言(未保存 = 注意、保存済み = 完了)にする。語は不変。
   読み込み中(マスタ)も静止した表示。
7. **「構築から選ぶ」の入口**(`TeamSourceMenuRow`): `lineLimit(1)` と `minimumScaleFactor` を外し、全文を折り返して読めるようにする。
   空の案内は caption の文字から `PopNoticeView(.empty)`(アイコン + 一言)に。SF Symbols の直書きを `PopSymbol` 経由に。`Menu` と識別子は不変。
8. **部品**: `PopSymbol` に `back`(chevron.left)・`picker`(chevron.up.chevron.down)を足す(View の直書きをなくすため)。`PopEmptyLabel` は空の状態のアイコンを
   渡せる(既定は従来の情報)。`TeamTextControls.buttonLabel` は確認のボタンにアイコンを付けられる(既定なし)。
9. **語**(Web の `web/src/i18n/team.ts` と同じ。`PokeCalcCore/G05TeamText.swift`): 一覧の空 =「まだ構築がありません」、読み込み中 =「読み込み中」、
   格子 =「メンバー」、空きのタイル =「N体目を追加」、技 =「技」、空の技 =「(なし)」、ニックネーム、SP の合計 =「合計 66/66(残り 0)」、
   SP のボタン =「<欄>の能力ポイントを増やす/減らす」、数値欄 =「<欄>の能力ポイント」。
10. 守る決まり(ソース検査 `G05TeamSourceRulesTests` が 7 ファイルで見る): `ProgressView(`・`repeatForever`・`lineLimit`・`minimumScaleFactor`・色のリテラル/システム色・
    SF Symbols 名の直書きを使わない。パレット K のトークンだけ。動き(縮み)は視差効果を減らすで 0 秒。常時アニメ無し。

## 採らなかった案

- 空の一覧に「新しい構築」のボタンを `ContentUnavailableView` の中に置く: 上に同じボタンがあり、同じ識別子・同じ名前のボタンが 2 つ並ぶ。Web も置かない。
- タイルを押すとその枠の「ポケモン」欄を自動で開く(シートを出す): 空きのタイルで意図せずシートが開く。Web も欄へ移るだけ(開かない)。
- SP を `Stepper` のまま、見た目だけ変える: Web の「数値欄を残す」とそろわず、`Stepper` の読み上げ名は用語集の語にできない。
- 技ピッカーのシートの閉じる・検索中の表示をこの弾で直す: 計算・逆算と共有で、既存テスト(`moveSearchCancelButton` ほか)の更新が広がる。後続の弾。
- 持ち物のメガ固定の理由・保存データを直した通知などの文: 状態の通知(文を置いてよい)なので変えない。
