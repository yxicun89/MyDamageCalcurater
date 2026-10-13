# ADR-0539: iOS の見た目の作り直し 第 3 弾(逆算)

- 状態: 採用(2026-10-13。仕様とテスト先行。実装は plan `I-ios-26`)
- 関連: ADR-0536(第 1 弾。部品 `PopHelpButton`・`PopIconButton`・`PopEmptyLabel`・`PopSymbol`・`CommonPartsText`)、ADR-0537(第 2 弾。構築。技シートの宿題を引き継ぐ)、
  ADR-0339(Web の方針 G-05)、ADR-0344(共通の部品)、ADR-0346(Web の逆算画面)、ADR-0533(ランクのメーター)、ADR-0534(API 不可時の表示)、
  ADR-0501 追記章「G-05 逆算の作り直し」、docs/design.md「見た目の作り直し方針(G-05)」「iOS の追い方(G-05)」、
  docs/ai-shared/decisions/2026-10-11-web-g05-reverse.md・web-visual-policy-g05.md・web-g05-components.md
- 担当: iOS(`ios/PokeCalc` の逆算 4 ファイルと `MasterSearchSheet.swift` の検索シート、`PokeCalcCore` の文言)
- 前提: ADR-0536(PR #774)・ADR-0537(PR #780)の実装が main に入っていること。

## 背景

第 1・2 弾に続き、逆算画面を方針(文で説明しない)に直す。Web は ADR-0346 で、自分・相手を見出し + ポケモンカード、単位を区切りボタン、
候補をカード(型の名前・目安のバッジ・「ほぼ合う候補」のバッジ・予測%の大きい数字・上限のバー・SP の範囲・持ち物・特性)にし、
種族検索欄の補足の文を「説明」ボタンの奥へ移した。iOS はこの構造と語に追う。iOS が既に方針どおりの点(下の「現状」の「変えないもの」)は変えない。
**変えないもの**: ランクのメーター(ADR-0533)、API 不可時の表示(ADR-0534。`PopNoticeView`・「もう一度」)、ダメージの側の切り替え
(識別子 `reverseSide-*`・AX では縦積み)、既存の識別子と読み上げ名(新規だけ追加。下の表の変更を除く)。

## 決定

1. **自分・相手**: 各カードの上に見出し(`PopHeading`。「自分」「相手」。識別子 `reverseMyHeading`・`reverseOpponentHeading`)。見出しはカード(`reverseMyCard`・
   `reverseOpponentCard`)の外・上に置き、AX5 の縦積み(カードどうし)の判定を変えない。カードの中身(画像かエンブレム・名前・タイプバッジ・持ち物の欄)は既に Web と同じ構造。
   持ち物の上限・メガ固定の注記は状態の通知なので、上限(`reverseMyItemLimitHint`)はアイコン + 一言(`PopNoticeView(.info)`)、メガ固定の理由は文のまま(Web も同じ)。
2. **観測欄(ダメージ)**: 単位は**ダメージの側で決まる**(与えた = %、受けた = HP。ADR-0501「判断した点」。実機の HP バーの表示に合わせた 2026-09-21 のユーザー回答)ので、
   Web の「行ごとの % / HP の区切りボタン」は**採らない**(切り替えられない区切りボタンは誤解を生む)。代わりに:
   - 行の頭に見える名前「ダメージ1」(`reverseObservationLabel-N`。読み上げは隠さない〈UITest が識別子で引くため。二重読み上げは短く許容〉。欄の読み上げ名「ダメージ1(%)」は従来のまま。単位が `ダメージ` → `HP` になる分だけ「ダメージ1(HP)」)
   - 単位は「%」「HP」(`reverseObservationUnit-N`。Web の語。旧: HP 側が「ダメージ」)
   - 単位の 1 行を**観測欄の一覧の下に 1 つ**(`reverseObservationHint`。Web の `observationHintLabel` と同じ文。単位が分からない欄の例外。1 カード 1 行まで)
   - 削除はアイコンだけ(`PopIconButton` + `PopSymbol.minus`。読み上げ名「このダメージを削除」・識別子は不変)、追加はアイコン + 名前(`PopSymbol.add` + 「ダメージを追加」。識別子は不変)で、どちらも 36pt 以上
   - 入力エラー(`reverseObservationError-N`)と上限の注記(`reverseObservationLimitHint`)はアイコン + 文(`PopNoticeView(.error)` / `.info`。識別子は文に付けて不変)
3. **候補のカード**(Web の ADR-0346 と同じ順):
   1 行目 = 型の名前(`reverseCandidateNature-<id>`。「補正なし」「B上昇」。旧: 「補正なし / 持ち物なし」)+ 目安の名前のバッジ(複数)+ 一致しない候補だけ「ほぼ合う候補」のバッジ(警告色の枠。識別子 `reverseCandidateMatch-<id>` をバッジに付け替える。
   一致した候補には出さない。旧: 一致・不一致とも caption の文を毎枚出していた)。右に予測%の大きい数字(`reverseCandidatePercent-<id>`)と短いラベル「予測」。
   次に予測%の上限のバー(装飾。`BulkRowDisplay.barFraction(percent:)` の幅・読み上げない)。下に SP の範囲(`reverseCandidateRange-<id>`)・持ち物(`reverseCandidateItem-<id>`)・特性(`reverseCandidateAbility-<id>`)を小さく。
   厳密に合う候補が 1 件も無く候補が 1 件以上あるとき(`ReverseLabels.hasNoExactCandidate`。判定は engine の `exactCount` のまま)は、結果の先頭に案内(`reverseNoExactNotice`。`PopNoticeView(.info)`。Web の `noExactCandidateNotice`)と、各候補の SP の範囲に「参考」のバッジ。
   未対応の印(`reverseUnsupportedNotice`・`reverseCandidateUnsupported-<id>`)・特性で分かれた候補の副題は従来どおり。候補が絞られる切り替えは操作時だけ・視差効果を減らすでなし(従来どおり)。
4. **前提・件数**: 前提(`reversePremise`)は Web の `assumedHpNote` と同じ「H32 を仮定した結果です」(旧: 「相手の HP の SP を 32(H32)と仮定した結果です」)。
   一致件数(`reverseExactCount`)は数値の行「一致 2 件 / 候補 3 件」(旧: 「入力したダメージと一致: 2 件 / 候補 3 件」)。識別子は不変。
5. **読み込み中**: `ProgressView` をやめ、`PopNoticeView(.loading)`(静止したアイコン + 「計算中」。Web の語)。識別子 `reverseLoadingIndicator` は文に付けて不変。
   場所は従来の固定の枠(レイアウトが跳ねない)に収める。
6. **アイコン**: 技の欄の `chevron.up.chevron.down` の直書きを `PopSymbol.picker`、`minus.circle`・`plus.circle` の直書きを `PopSymbol.minus`・`PopSymbol.add` にする。
7. **文字を縮めない**: 4 ファイルの `lineLimit`・`minimumScaleFactor`(側の切り替え・定番の振り方のピル・技の欄・候補の見出し)を外し、折り返して全文を見せる。AX5 でも欠けない。
8. **検索シート**(`MasterSearchSheet.swift` の `SpeciesSearchSheet`・`MoveSearchSheet`。計算・構築・調整と共用。ADR-0537 §5 が逆算の弾に送った宿題):
   - 空クエリの補足(`MasterSearchLabels.prompt`「名前の先頭で検索」)は常時出さず「説明」ボタン(`PopHelpButton`。`speciesSearchHelp`・`moveSearchHelp`。中の文は `speciesSearchNote`・`moveSearchNote`)の奥。
     「一致する候補がありません」「候補が多いので…」は状態なので従来どおり出す(`speciesSearchHint`・`moveSearchHint`)
   - 閉じるはアイコンだけ(`PopIconButton` + `PopSymbol.close`、読み上げ名 `CommonPartsText.close` = 「閉じる」。識別子 `speciesSearchCancelButton`・`moveSearchCancelButton` は不変)
   - 検索中の `ProgressView`(`speciesSearchLoadingIndicator`・`moveSearchLoadingIndicator`)を、静止したアイコンだけの表示にする(識別子は不変)
   - 検索欄の placeholder(`.searchable` の prompt)は入力欄の名前なので変えない
9. **語**(Web の `reverseScreenText`・`reverseResultText` と同じ。`PokeCalcCore/G05ReverseText.swift` の `ReverseLabels`): 自分・相手・%・HP・ダメージN・単位の 1 行・予測・ほぼ合う候補・参考・計算中・H32 を仮定した結果です・案内。
10. 守る決まり(ソース検査 `G05ReverseSourceRulesTests` が 4 ファイルで見る): `ProgressView(`・`repeatForever`・`lineLimit`・`minimumScaleFactor`・色のリテラル/システム色・SF Symbols 名の直書きを使わない。
    パレット K のトークンだけ。動きは視差効果を減らすで 0。常時アニメ無し。

## 採らなかった案

- **単位の区切りボタン(% / HP)を行ごとに置く**(Web): iOS は単位が側で決まり、切り替えられない(決定 2)。選べないものを選択肢の形に見せない。
  代わりに単位を「%」「HP」の短い語 + 1 行の補足にして Web と語・文をそろえる。
- ダメージの側を `Picker(.segmented)` に作り替える: 今の自前の区切り(識別子・選択中の特性・AX での縦積み)が既に方針どおり。ADR-0531 のとおり、`Picker(.segmented)` は大きい文字で文字を縮めて詰める。
- 一致件数の行を消す(Web には無い): 識別子 `reverseExactCount` を使う既存テストがあり、件数は「数値・結果」として残してよい。短い語にするだけにした。
- 候補の `matchLabel`(「入力したダメージと一致」)を廃止する: 読み上げの値として残す(API 不変・既存テストは弱めない)。見える文字だけバッジに置き換える。
- 共有の種族ピッカーの見出し(`SpeciesHeaderMenuLabel`。計算・調整と共用)の矢印の直書き・`lineLimit(2)` をこの弾で直す: 計算・調整の弾で扱う(逆算の 4 ファイルだけを検査対象にする)。
- 持ち物の欄に見える「持ち物」ラベルを足す・自分のカードに持ち物の印を足す: 持ち物の欄(`MenuLabelChip`)が選んだ持ち物をそのまま見せており、構造が Web と同じ。
- 入力エラーの文(「1〜100 の整数%を入力してください」など)を Web の文に合わせる: エラーは文を置いてよく、既存テスト・読み上げ名を動かさない。
