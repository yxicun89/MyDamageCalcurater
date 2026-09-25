# ADR-0501: iOS の画面ごとの受け入れ条件と判断

- 状態: 採用(2026-09-22。P6-1〜P6-2b の受け入れ条件・判断・実装メモを ios/README.md から移した。README は coding-rules §8 の形にする)
- 日付: 2026-09-22
- 関連: ADR-0500(iOS の構成)、ADR-0009(一括計算のプリセット)、ADR-0010 §R(逆算)、ADR-0200・0202(API 契約)、ADR-0300 §7(Web の逆算画面)、
  docs/design.md、docs/runbooks/ios.md(手順書)

## 背景

iOS の各タスク(P6-1〜)は spec-writer が受け入れ条件と判断を ios/README.md に書き、implementer・critic がそれを正として進めてきた。
2026-09-22 のユーザー決定(coding-rules §8)で README は「何をするか・構成図・ディレクトリ・コマンド・関連 ADR」の 80 行以内にすることになったため、
画面ごとの受け入れ条件・判断・実装メモは本 ADR に移す。以下の本文は移す前の README の内容で、書かれた時点の文脈(「実装済み」など)を保つ。
以降のタスク(P6-2c 構築など)の受け入れ条件もこの ADR か、タスクごとの新しい ADR(0500 番台)に書く。

## P6-1 の受け入れ条件(実装済み)

1. `swift test`(PokeCalcKit)で `PokeCalcDesignTests` と `PokeCalcCoreTests` がすべて成功する。
2. デザイントークンが docs/design.md と一致する: ベース6色のライト/ダークの RGBA、18 タイプの色(openapi の `PokeType` ID で引ける。欠け・余り無し)、
   文字サイズ 28/17/15/12、角丸 20/999/12、余白 4/8/12/16/24。SwiftUI の `Color` / `Font` を返すアクセサがある。
3. `APIPokeCalcService` は生成クライアント経由で、全操作に `X-Device-Id` / `X-Session-Id` を付け、パス・メソッド・クエリ(`q` / `limit`)・
   リクエスト JSON(`presets` / `itemVariants` / `sp` など)をドメインから正しく写し、200 の JSON をドメインへ、エラー body を
   `PokeCalcError`(`code` 保持)へ写す。通信失敗も `PokeCalcError`。逆算も同じ写像で API を呼ぶ(P6-2 契約追従)。
4. `ClientIdentity`: 端末 ID は初回だけ作って UserDefaults に保存(以後同じ・UUID 形式・壊れた値は作り直す)、セッション ID はインスタンスごとに新しい UUID。
5. `AppConfiguration` が ADR-0500 §5(モック/API の切り替え)どおりにモック/API/エラーを決める。
6. `MockPokeCalcService`: 架空データの `nameJa` がすべて「テスト」で始まる。一括計算は既定プリセット(物理5・特殊5・変化2。ADR-0009)、
   指定順、プリセット優先の presets × itemVariants の順で行を返し、各行は 16 ロール・非減少・min/max が両端。逆算は ADR-0010 §R の形
   (性格クラス neutral/plus × 持ち物、範囲は昇順で空でない、`assumedHPSP` は defender 32 / attacker 0)。未知の ID は `PokeCalcError`。
7. (XCUITest。P6-1 ではアプリの骨組みまで)`POKECALC_USE_MOCK=1` で起動すると、ルート画面にモックで動いている表示と、計算画面への入口がある。
8. `make ios-gen-check` で生成物に差分が無い。
9. `make ios-check-infoplist`: `POKECALC_API_BASE_URL` を渡してビルドした成果物の Info.plist に
   `PokeCalcAPIBaseURL` キーとその値が実際に入っている(`INFOPLIST_KEY_*` の独自キーは
   生成 Info.plist に反映されないため、`INFOPLIST_FILE` + `GENERATE_INFOPLIST_FILE` のマージで
   対応したことを確かめる)。

## P6-2a の受け入れ条件(ダメージ計算画面。実装済み)

ロジックは `PokeCalcCore` に置く(ADR-0500 §1「画面の状態(ViewModel)」は PokeCalcCore。新ターゲットは作らない)。
XCTest: `AttackerPresetTests` / `BulkRowDisplayTests` / `CalcViewModelTests` / `CalcScreenErrorTests`
(サービスはテスト内のスタブ `Support/StubPokeCalcService.swift`。モックの数値に依存しない)。
実装ファイル: `PokeCalcKit/Sources/PokeCalcCore/{AttackerPreset,BulkRowDisplay,CalcScreenError,CalcViewModel,DisplayLabels}.swift`、
View は `PokeCalc/{CalcScreenView,CalcScreenCards,CalcScreenResults,CalcScreenStyleHelpers}.swift`。
`DisplayLabels.swift`(タイプ・技分類・タイプ相性の日本語ラベル)は当初 View 側に置いていたが、
`CalcViewModel.moveSummaryText` が組み立てで使うため Core に移した(批評 M4 対応)。

実装時の注意(申し送り): `DomainTypes.swift` の逆算の観測 `enum` は `Observation` ではなく `DamageObservation`
に改名してある。`@Observable` マクロが展開するコードが Apple の Observation フレームワークの
`Observation.ObservationRegistrar` を参照するため、同じモジュールに `Observation` という型があると解決が衝突する
(`CalcViewModel` を `@Observable` にした時点で判明)。テストは型名では参照していないので、この改名でテストへの影響は無い。

1. **自分側のプリセット** `AttackerPreset`(ADR-0500 §6): `allCases == [.aFull, .aMax, .none]`、表示名「A特化」「A振り」「無振り」。
   `AttackerPreset.build(_:moveCategory:natures:) throws -> AttackerBuild`(`natureId` / `sp`)。関連ステータスは
   物理 = atk・特殊 = spa・**変化 = atk**(ADR-0010 §2・モックの `reverseStat` と同じ規則。変化技はダメージを出さないので結果は変わらない)。
   A特化 = 関連 SP 32 + 一覧で最初の (plus = 関連, minus = atk。関連が atk なら spa) の性格、A振り = 関連 SP 32 + 最初の `plus == nil`、
   無振り = SP 0 + 最初の `plus == nil`。他のステータスの SP は 0。性格 ID を直書きしない。該当が無ければ
   `PokeCalcError(code: PokeCalcError.Code.natureUnavailable)`(新設。値は `client_` 接頭辞)。
2. **結果行の整形** `BulkRowDisplay`(純粋): 調整名 = サーバーの `presetLabel` そのまま、持ち物名(nil は「持ち物なし」、
   マスタに無い ID は ID のまま)、%幅「72.1〜85.3%」(U+301C。小数第1位。100% 超もそのまま)、ダメージバーの割合
   (`barMinFraction` / `barMaxFraction` = % / 100 を 0〜1 に収める)、確定数「確定2発」/「乱数2発(45.6%)」
   (`displayChancePercent` を使い `chancePercent` は使わない)/ hits = 0 は「倒せない」。
   `KOTier`(`.cannotKO` / `.guaranteed(hits:)` / `.chance(hits:)`)と `KOTier.changed(from:to:)`: 段階が変わったときだけ true
   (確率の数字だけの変化・初回表示は false)。行の `id` は `<preset の rawValue>@<itemId。nil は ->`(例 `hb@-`)。
3. **起動時**: `CalcViewModel(service:)`(`@MainActor`・`@Observable`)の `load()` が natures・種族(空クエリ)・技・持ち物を読み、
   攻撃側 = 種族一覧の最初、防御側 = 2番目、技 = 攻撃側 learnset の順で最初のダメージ技(無ければ learnset の最初)、
   プリセット = A特化、持ち物なし、比較なし を選んで `calcBulk` を**ちょうど1回**呼ぶ。
   要求: `format = single`・`critical = false`・attacker の `speciesKey`/`sp`/`natureId`/`itemId`・`defenderSpeciesKey`・`moveId`・
   `presets` は空(省略)・比較が無ければ `itemVariants` も空。
4. **入力の変更**(`selectAttacker` / `selectDefender` / `selectMove` / `selectAttackerPreset` / `selectAttackerItem` /
   `toggleDefenderItemComparison` / `swapSides`)ごとに `calcBulk` を1回だけ呼ぶ。技の選択肢(`moveOptions`)は攻撃側の learnset の順で、
   マスタにある技だけ(変化技も含む)。選択肢に無い技は無視して計算しない。攻撃側が変わったら、いまの技を覚えるなら残し、
   覚えなければ規則3で選び直す。
5. **持ち物の比較トグル**: `comparedDefenderItemIds` は持ち物マスタの順。1つ以上あれば `itemVariants = [nil] + 比較中`
   (「持ち物なし」を基準として先頭に含める)、無ければ空。
6. **攻守入れ替え**: 攻撃側と防御側の種族を入れ替え、技は規則4で選び直す。プリセット・攻撃側の持ち物・比較トグルは入れ替えない。計算は1回。
7. **最新の要求だけを反映**: 古い要求の応答(成功・失敗とも)が後から届いても `rows` / `error` / `isLoading` を変えない。
   `isLoading` は最新の要求の応答待ちの間だけ true。世代は `beginInput()`(入力操作の入口)が1つ進め、
   `species(key:)` の応答(攻撃側の learnset 読み直し)も `calcBulk` と同じ番号で守る(批評 M1: 攻撃側を連続で
   変えたとき、古い方の species 応答が新しい方より後に届いても `moveOptions`・要求を上書きしない)。
   組み立て・`species(key:)` の失敗で `error` を立てる経路も同じ番号で守るため、失敗直後に古い `calcBulk` の
   成功が届いても `error` は消えない(批評 M2)。
8. **エラー**: `CalcScreenError(_:)` で `transport` / `unexpectedResponse`(デコード失敗と PokeCalcError 以外)/
   `service(code:message:)`(サーバーの code とクライアントの `natureUnavailable` 等)に分け、`message` は種類ごとに別の文言
   (`service` は code と説明を含む)。計算が失敗したら `error` を立てて `rows` を空にし、次の成功で `error` を消す。
   マスタの読み込み失敗・性格が無い・種族が2つ未満のときは `error` を立て、計算しない。
   `moveUnavailable`(learnset とマスタの技が1つも一致しない。データ由来)と `selectedMoveMissing`
   (選択中の `moveId` が `moveOptions` に無い。内部の不整合)はコードを分けている(批評 M2 の「任意」項目)。
9. **技の要約・相性**(批評 M4): `selectedMove`(`moveOptions` から `moveId` を引いたもの)、
   `moveEffectiveness`(表示中の全行の `effectiveness` が一致すればその値・行が無い/割れていれば nil)、
   `moveSummaryText`(「威力37 / 物理 / ばつぐん(×2)」。`DisplayLabels.MoveCategoryLabel` / `EffectivenessLabel` を使う)。
   `BulkRowDisplay.effectiveness` は `CalcResult.effectiveness` をそのまま運ぶ。
10. **`load()` は1回だけ**(批評「任意」): 2回目以降の呼び出しは何もしない。
11. **攻守入れ替えの回数** `swapTick`(批評「任意」): `swapSides()` だけが増やす。View はこれの変化だけを
    見てカード入れ替えのアニメーションを掛け、通常の種族セレクタでの変更や起動時の読み込みでは動かさない。

### XCUITest で確かめること(`POKECALC_USE_MOCK=1`。View と一緒に implementer が書く)

`openCalcScreen` → 計算画面。次の accessibilityIdentifier を約束する(値は View に1か所で定義する)。

| identifier | 要素 |
|---|---|
| `calcScreen` | 計算画面のルート(P6-1 から継続) |
| `calcBackendModeBadge` | モックで動いている表示(計算画面にも出す。ADR-0500 §4) |
| `attackerCard` / `defenderCard` | 攻撃側・防御側のカード(種族名を含む) |
| `attackerSpeciesPicker` / `defenderSpeciesPicker` / `movePicker` / `attackerItemPicker` | 選択 UI。種族セレクタはカードのヘッダー(エンブレム・名前・タイプ)そのものが `Menu` のラベルを兼ねる(批評 M3d)。`Menu` は中身を1つのボタンにまとめるため、種族名は `accessibilityLabel`(ボタン自体のラベル)で読む |
| `attackerPreset-<AttackerPreset の rawValue>` | プリセットの各セグメント(`aFull` / `aMax` / `none`)。カードの外、カード行の直下に画面幅いっぱいの3等分の行として置く(批評 M3c: カード内だと幅が足りず「無振り」が見切れていた) |
| `swapSidesButton` | 攻守入れ替え |
| `defenderItemToggle-<itemId>` | 比較する持ち物のトグル |
| `calcResultRow-<行の id>` | 結果行(例 `calcResultRow-hb@-`) |
| `calcResultPercent-<行の id>` / `calcResultKO-<行の id>` | 行の%幅と確定数のテキスト |
| `calcLoadingIndicator` / `calcErrorMessage` | 読み込み中・エラー表示 |

- 開いた直後に、モックの既定(攻撃側 = モックの種族1番目、技 = その最初のダメージ技(物理))で**物理の既定5行**
  (`none` `hp` `hb_boost` `hb` `hb_full` の順。ADR-0009)が出る。行の文言の数値は検査しない(モックの数値に依存しない)。
  プリセットの3セグメントすべてが画面内で `isHittable`(批評 M3c)。
- `swapSidesButton` をタップすると2枚のカード(`attackerSpeciesPicker` / `defenderSpeciesPicker` の
  `accessibilityLabel`)の種族名が入れ替わる。
- `defenderItemToggle-<モックの持ち物 ID>` をタップすると行が 5 → 10 になる(プリセット × {持ち物なし, その持ち物})。
- `calcBackendModeBadge` が見える。
- 演出(design.md「動き」): ダメージバーは spring、確定数の段階が変わった行だけバッジが弾む + 軽い触覚(`KOTier` の変化をトリガにする)、
  入れ替えはカードが入れ替わる 0.35 秒。「視差効果を減らす」(`accessibilityReduceMotion`)で無効化。常時動くものは置かない。これは XCUITest では検査しない。

### 実装メモ

初版(P6-2a 実装直後)は critic に NG 判定を受け、以下は2回目のレビュー対応で直した内容を含む。

- **世代の保護(M1・M2)**: `CalcViewModel.beginInput()` が入力操作の入口で世代を1つ進め、その操作が生む
  `species(key:)` の応答・`calcBulk` の応答・組み立て失敗のどれもが同じ番号で「まだ最新か」を確かめてから
  画面へ反映する。`StubPokeCalcService` に `species(key:)` の保留モード(`setSpeciesMode(.manual)` /
  `resolveSpecies(at:with:)` / `waitForSpeciesRequests(count:)`)を足し、攻撃側を連続で変える競合を
  `CalcViewModelTests.testStaleSpeciesDetailDoesNotOverwriteNewerAttackerSelection` /
  `testStaleCalcSuccessDoesNotClearNewerSpeciesFailureError` で固定した。
- **カードのレイアウト(M3・再レビュー対応)**: タイプバッジ(`TypeBadgeView`)は `.lineLimit(1).fixedSize()` を付け、
  幅を詰められて1字ずつ縦に折り返る不具合を直した(17e で「ひこう」が3行になっていた)。カードのヘッダー
  (エンブレム 40pt・名前・タイプ)は `SpeciesHeaderMenuLabel` として種族セレクタの `Menu` ラベルを兼ねる。
  `Menu` はラベルの中身を1つのボタンにまとめてしまい、子の `accessibilityIdentifier` が外から見えなくなるため、
  `Menu` 自体に `.accessibilityLabel(種族名)` と `.accessibilityHint("ポケモンを変える")` を明示して XCUITest から
  読めるようにした。ヘッダーは3段(1段目 エンブレムと末尾の chevron・2段目 名前だけをカード幅いっぱいに・
  3段目 タイプバッジ)。初版は名前を `.lineLimit(1)` + `.minimumScaleFactor(0.5)` で1行に押し込んでいたが、
  「縮めるのではなく幅を確保する」方針に反する(18 Pro で約9pt まで縮んでいた)と指摘され、
  `.lineLimit(2)` + `.minimumScaleFactor(0.85)`(最後の手段)+ `.fixedSize(horizontal: false, vertical: true)` +
  `.frame(maxWidth: .infinity, alignment: .leading)` で、まず名前専用の行の幅をカードいっぱい確保し、
  それでも収まらないときだけ2行目に折り返す・控えめに縮小する、という順にした。
  さらに文字サイズがアクセシビリティ域(`.accessibility1` 以上)のときは、`CalcScreenView.cardsRow` が
  2枚のカードを横ではなく縦(攻撃側 → 入れ替えボタン → 防御側)に積む。
  プリセットのチップはカードの外、カード行の直下に画面幅いっぱいの3等分のピル行として置く
  (`CalcScreenView.presetSegmentedRow`)。結果行(`ResultRowView`)は
  1段目「調整名 / Spacer / %幅」(%幅は `.lineLimit(1).fixedSize()`。大きい文字だと `ViewThatFits` で縦積みに
  切り替わる)・2段目がカード幅いっぱいのダメージバー・3段目が右寄せの確定数バッジ、という3段の `VStack` にし、
  全行でバーの物差しをそろえた。
- **触覚・バッジの弾みは一覧側で1回(批評「任意」対応込み)**: `ResultsSectionView` が `viewModel.rows` の
  変化を見て、行ごとの直前の `KOTier` と比較し(`KOTier.changed(from:to:)` と同じ「初回は弾ませない・段階が
  変わったときだけ true」の規則)、変わった行の id 集合と触覚の1回のトリガをまとめて計算する。各行
  (`ResultRowView`)はその結果(`tierChanged: Bool`)を受け取ってバッジの弾みだけを描き、`koTier` を
  自分でも監視する・触覚を自分でも鳴らす、という二重判定をやめた。
- ダメージバー(`DamageBarView`)は% が小さいと帯が消えて見えなくなるため、最小可視幅(6pt)を確保し、
  トラックにヘアライン枠線を付けて範囲が常に視認できるようにしてある。
- **Liquid Glass(M5)**: `glassCard()` は iOS 26+ の本物の `.glassEffect(.regular.tint(ColorToken.bgGlass.color), in:)`
  を使う(初版は `.ultraThinMaterial` + 色の重ねがけで代用していたが、requirements のビジュアル B・ADR-0500 §1
  に合わせて本物に差し替えた)。攻撃側・防御側の2枚のカードは `GlassEffectContainer` でまとめている。
- **技の要約・相性(M4)**: タイプ・技分類・相性の日本語ラベル(`PokeTypeLabel` / `MoveCategoryLabel` /
  `EffectivenessLabel`, `PokeCalcCore/DisplayLabels.swift`)は Core に置く(`moveSummaryText` が組み立てで使うため)。
  レギュレーションに依存しないポケモン全体の固定語彙なので、`MockPokeCalcService.presetLabel` と同じ理由で
  コードに1か所持つ(マスタには無い表示専用の文言。coding-rules §2)。網羅は `DisplayLabelsTests` で確かめる。
  技セレクタの要約行は「ばつぐん」のときだけ技のタイプ色を使い、それ以外は無彩色(`ColorToken.textSecondary`)。
  「ばつぐん」かどうかの判定(`effectiveness >= 2`)は View に置かず `EffectivenessLabel.isSuperEffective(_:)`
  (Core)に集約し、`DisplayLabelsTests.testIsSuperEffectiveIsTrueOnlyAtOrAboveDoubleDamage` で確かめる(批評「任意」)。
- **見た目の定数を1か所に(批評「任意」)**: 1行に収めるための縮小率 0.7 とヘアライン枠線の太さ 1pt が
  `CalcScreenCards.swift` / `CalcScreenView.swift` / `CalcScreenResults.swift` に同じ値でばらばらに書かれていたため、
  `CalcScreenStyleHelpers.CalcScreenMetrics`(`compactMinimumScaleFactor` / `hairlineBorderWidth`)にまとめた。
- **species(key:) の応答待ちも読み込み中に(批評「任意」)**: `CalcViewModel.applyAttackerChangeAndRecalculate` が
  冒頭で `isLoading = true` を立てるようにした(`calcBulk` の手前の learnset 読み直しも同じ1回の操作のため)。
  `testIsLoadingWhileWaitingOnAttackerSpeciesDetail` で確かめる。
- **Dynamic Type(批評「任意」)**: `PokeCalcDesign.TextStyleToken.font` は `UIFontMetrics(forTextStyle: .body)` で
  design.md の基準サイズ(28/17/15/12。`size` プロパティ自体・`DesignTokenTests` の期待値は変えていない)を
  アクセシビリティの文字サイズに応じて拡大する。macOS はパッケージテストのためだけの対象なので固定サイズのまま。
- **入れ替えアニメーションを swap だけに絞る(批評「任意」)**: `CalcViewModel.swapTick` を `swapSides()` の中で
  `attackerSpeciesKey`/`defenderSpeciesKey` と同じ同期区間で増やし、View は `.animation(value: viewModel.swapTick)`
  だけを見る。初版は種族キーの文字列そのものを見ていたため、起動時の読み込みや通常の種族セレクタでの変更でも
  カードの出入りアニメーションが動いてしまっていた。
- **読み込みインジケータの高さ固定(批評「任意」)**: `ProgressView` の表示・非表示に関わらず高さ(24pt)を
  固定で確保し(`loadingSlot`)、出たり消えたりで下の行が上下にずれないようにした。
- `moveUnavailable`(learnset とマスタの技が1つも一致しない)と `selectedMoveMissing`
  (選択中の `moveId` が `moveOptions` に無い内部の不整合)はエラーコードを分けた(批評「任意」)。
  learnset が空になる経路は `testAttackerLearnsetEmptyAfterMasterFilterSetsErrorWithoutCalculating` で確認する。
- `load()` は2回目以降の呼び出しを無視する(`didLoad` フラグ。批評「任意」)。
- `StubPokeCalcService` の待ち合わせ上限を 3000ms → 10000ms に延ばした(CI 環境の遅延でのフレーク対策。批評「任意」)。
- 計算画面を起動時にいきなり開く環境変数 `POKECALC_OPEN_CALC_SCREEN_AT_LAUNCH=1`(`RootView`)を追加した。
  XCUITest を介さずスクリーンショットを撮る用途専用で、通常の起動やモック/API の判定には影響しない。

## P6-2 契約追従の受け入れ条件(P3-1 / P3-2 の api/openapi.yaml。ADR-0200 / ADR-0202)

1. エラー応答の `Error.code`(`ErrorCode` enum)は rawValue の文字列のまま `PokeCalcError.code` に入る(ドメインは enum に写さない。
   同じ `code` にクライアント側の `client_*` も入るため)。全8操作の 503 `upstream_unavailable`、計算系(calc / bulk / reverse)の
   500 `internal` と 503 `master_unavailable`、400 / 404 / default を写す。`ErrorCode` に無い code は `client_decode_error`。
   `PokeCalcError.Code` のサーバー語彙(`not_found` / `invalid_input`)は契約の `ErrorCode` にあり、`client_*` は契約と衝突しない
   (`DomainTypesTests`)。
2. `CalcResult.category`(必須)がドメインの `CalcResult.category: MoveCategory` に入る(1対1・一括の各行)。欠けた応答はデコード失敗。
3. `BulkCalcRow.defender`(必須)がドメインの `BulkDefender{sp, nature: NatureModifier, natureId: String?, stats}` に入る。
   natureId の null と無補正(plus / minus とも null)を落とさない。欠けた応答はデコード失敗。
4. 逆算は `POST /api/calc/reverse` を送る(ヘッダー付き)。要求は format・side(defender / attacker)・known・unknownSpeciesKey・
   moveId・options.critical・itemCandidates(null を含めてその順)・observations(percent / percentTenths / damage の
   ちょうど1つのキー)・maxCandidates。応答は side・stat・assumedHpSp・exactCount と、候補(natureClass・nature・natureId・
   itemId・ranges・spCount・exact・mismatch・support・表示%)を順序どおりに写す。「API 未対応」は返さない。
5. モックは同じ形を返す(計算はしない): `category` は要求した技の分類、`defender` の SP・性格補正は ADR-0009 のカタログどおりで
   natureId は補正が一致するモックの性格(ID 昇順の最初。無ければ nil)、逆算候補の `nature` は neutral = 無補正 /
   plus = {関連ステータス, atk}(関連が atk なら spa)。
6. 既存の計算画面のテスト(`CalcViewModelTests` / `BulkRowDisplayTests` ほか)は、構築コードに `category` / `defender` を
   足しただけで、検査内容はそのまま通る。

## P6-2b の受け入れ条件(逆算画面。実装済み)

ロジックは `PokeCalcCore` に置く(ADR-0500 §1)。XCTest: `ObservationInputTests` / `KnownDefenderPresetTests` /
`ReverseCandidateDisplayTests` / `ReverseViewModelTests`(サービスは `Support/StubPokeCalcService.swift` の
`setReverseMode(.immediate / .manual)`。既定の `.disallowed` は計算画面が逆算を呼ばないことの確認として残す。モックの数値に依存しない)。
用語: 「自分」= 既知側(`known`)、「相手」= 逆算する側(`unknownSpeciesKey`)。
与えたダメージ = `side: .defender`(自分が攻撃側)、受けたダメージ = `side: .attacker`(自分が防御側)。

実装ファイル: `PokeCalcKit/Sources/PokeCalcCore/{ObservationInput,KnownDefenderPreset,ReverseCandidateDisplay,ReverseViewModel}.swift`、
View は `PokeCalc/{ReverseScreenView,ReverseScreenCards,ReverseScreenObservations,ReverseScreenResults}.swift`
(カードの一部部品は `CalcScreenCards.swift` の `SpeciesHeaderMenuLabel` / `MenuLabelChip` を再利用。internal に広げた)。
XCUITest は `PokeCalcUITests/ReverseScreenUITests.swift`。ルート画面の入口は `RootView.swift`(`openReverseScreen`)。

1. **観測の入力** `ObservationParser.parse(_:kind:) -> Result<DamageObservation, ObservationFieldError>`(純粋):
   テンキー入力の文字列を前後の空白を落として読み、ASCII の数字だけを受ける。`ObservationKind(side:)` は
   defender → `.percent`(整数 1〜100 = `ObservationLimits.percentRange`)、attacker → `.damage`(1 以上 =
   `ObservationLimits.minimumDamage`。上限なし)。範囲の出典は openapi `Observation` と ADR-0010 §R2。
   空・空白だけ → `.empty`、数字以外(小数・符号・全角を含む)→ `.notANumber`、0・範囲外・Int に収まらない → `.outOfRange`。
   `ObservationFieldError.message(kind:)` の文言の数値は定数から作る。0.1% 入力(`.percentTenths`)は出さない。
2. **自分の防御側** `KnownDefenderPreset`(受けたダメージ用): `allCases == [.none, .max, .full]`、表示名は相手の技の分類で
   「無振り」「HB振り / HD振り」「HB特化 / HD特化」。`build(_:moveCategory:natures:) throws -> DefenderBuild`
   (`natureId` / `sp`): 関連ステータスは物理・変化 = def、特殊 = spd。`max` = H32 + 関連 32 + 最初の無補正、
   `full` = H32 + 関連 32 + 一覧で最初の (plus = 関連, minus = atk) の性格(ADR-0009 の `hb_full` / `hd_full`)、
   `none` = SP 0 + 最初の無補正。無ければ `natureUnavailable`。性格 ID を直書きしない。
3. **候補の整形** `ReverseCandidateDisplay(candidate:stat:items:)` / `ReverseResultDisplay(result:items:)`(純粋):
   - `id` = `<natureClass の rawValue>@<itemId。nil は ->`(例 `plus@-`)。
   - 性格クラス「補正なし」/「B上昇」(`statLetter`: H/A/B/C/D/S。stat は `ReverseResult.stat`)。
   - 持ち物名(nil は「持ち物なし」、マスタに無い ID はそのまま。`BulkRowDisplay.itemLabel` と同じ規則)。
   - SP の範囲「B 20〜23」/「B 17, 19〜32」/「B 20」(`ranges` を全部出し、1区間に畳まない。ADR-0010 §R3)。
   - 目安の名前 `guideNames`: 範囲に SP 0 / 32 が入るときだけ、性格クラスとの組で付ける(Web の ADR-0300 §7 と同じ規則。0 側が先)。
     防御側 0+補正なし = H振り、0+上昇 = H振り+B(D)補正、32+補正なし = HB(HD)振り、32+上昇 = HB(HD)特化。
     攻撃側 0+補正なし = 無振り、0+上昇 = A(C)補正のみ、32+補正なし = A(C)振り、32+上昇 = A(C)特化。
   - 一致 `matchLabel`: exact →「観測と一致」、そうでなければ「一致なし(最も近い SP)」。
   - 想定ダメージ幅「12.3〜15.6%」(`BulkRowDisplay.percentRangeText` と同じ書式)。
   - 結果全体: 候補は**サーバーの順のまま**(ADR-0010 §R4 = design.md「一致度の高い順」。表示で並べ替えない・グループ化しない)、
     `exactCountText`「観測と一致: 2 件 / 候補 3 件」、`premiseText` は defender のとき
     「相手の HP の SP を 32(H32)と仮定した結果です」(数値は `assumedHPSP` から作る。ADR-0010 §R1・§R7)、attacker は nil。
4. **起動** `ReverseViewModel(service:)`(`@MainActor`・`@Observable`)の `load()`(2回目以降は何もしない):
   natures・種族・技・持ち物を読み、与えたダメージ・自分 = 種族一覧の最初・相手 = 2番目・技 = 攻撃側 learnset の順で
   最初のダメージ技・A特化・自分の防御側 = 無振り・持ち物なし・相手の持ち物候補なし・観測は空の1行。**逆算は呼ばない**。
   技の選択肢 `moveOptions` は攻撃側(与えたダメージ = 自分、受けたダメージ = 相手)の learnset の順で、マスタにある
   **ダメージ技だけ**(変化技は逆算できないので出さない)。1つも無ければ `moveUnavailable`。
5. **逆算を呼ぶ条件**: 空でない行がすべて有効で、有効な行が1つ以上あるとき。空の行は送らない(エラー `.empty` は持つが
   計算は止めない)。不正な行が1つでもあれば計算せず結果を消す(Web の ADR-0300 §7 と同じ)。
   観測の操作(`addObservation()` / `removeObservation(id:)` / `editObservation(id:text:)`)は、送る観測の列(行の順)が
   変わったときだけ reverse を1回呼ぶ(空の行の追加・削除、"12" → "012" は呼ばない)。最後の1行を消すと空の1行に戻る。
   それ以外の入力の変更(`selectMySpecies` / `selectOpponentSpecies` / `selectMove` / `selectMyItem` /
   `toggleOpponentItemCandidate` / いまの側で使うプリセット)は計算できる状態なら1回呼ぶ。使っていない側のプリセットの変更は値を覚えるだけ。
   未知の ID(種族・技・観測の行)は無視する。
6. **要求**: `format = single`・`side`・`known`(与えたダメージ = 自分の種族 + `AttackerPreset.build`(技の分類で A/C)+ 自分の持ち物、
   受けたダメージ = 自分の種族 + `KnownDefenderPreset.build`(相手の技の分類で B/D)+ 自分の持ち物)・`unknownSpeciesKey`・`moveId`・
   `itemCandidates`(相手の持ち物候補が無ければ空、あれば `[nil] + 候補`。候補は持ち物マスタの順)・`observations`(有効な行を行の順で
   `.percent` / `.damage`)・`critical = false`・`maxCandidates = 0`。
7. **側の切り替え** `selectSide(_:)`: 同じ側なら何もしない。変えたら観測を空の1行に戻し(単位の意味が変わる)、自分の持ち物と
   相手の持ち物候補を外し(攻撃用・防御用で意味が変わる)、結果を消す。種族と両方のプリセットは残す。技は新しい攻撃側の learnset で
   選び直す(いまの技を覚えていれば残す)。種族の learnset を読み直すのは攻撃側の種族が変わったときだけ
   (与えたダメージでの自分の変更、受けたダメージでの相手の変更、側の切り替え)。
8. **最新の要求だけを反映**(`CalcViewModel` と同じ世代の保護): 古い `reverse` の成功・失敗、古い `species(key:)` の応答は
   `result` / `error` / `isLoading` / `moveOptions` を変えない。有効な観測が無くなった(計算しない状態に戻った)ときも世代を進め、
   応答待ちの古い要求を捨てて `isLoading = false`。`isLoading` は最新の要求の応答待ちの間だけ true。
9. **エラー**: `CalcScreenError(_:)` をそのまま使う(PokeCalcError の汎用の写像で、計算画面専用の分岐は無いため一般化は不要)。
   逆算の失敗は `error` を立てて `result` を消し、次の成功で消す。マスタの読み込み失敗・種族が2つ未満・性格が無い・技が無いときは
   `error` を立てて計算しない。行ごとの入力エラーは画面全体の `error` にしない。
   **古いエラーを持ち越さない**(批評対応): 次の成功以外にも、計算しない状態に戻ったときに技が選べていれば
   (`moveOptions` に `moveId` がある = マスタ起因の失敗ではない)、前回の失敗を画面に残さない
   (観測を全部消す・側を切り替える・技が選べる種族に変える、のどれでも `error` が消える)。

### 判断した点(既定案。ユーザー未確認)

- **受けたダメージの自分の防御側**: `KnownDefenderPreset`(無振り / HB(HD)振り / HB(HD)特化)から選ぶ。既定は無振り。
  理由: Web の P4-4 は既知の防御側を SP 0・無補正に固定しており(ADR-0300 §7)、既定をそろえると同じ入力で両クライアントの
  結果が一致する。そのうえで「H を振った自分」の観測を扱えるよう ADR-0009 のカタログから H の有無が明確な2つを足した
  (`hp` / `*_boost` は選択肢を増やすだけなので入れない)。構築の個体を使うのは P6-2c の後。
- **相手の持ち物候補**: 既定は「持ち物なし」の1通り(`itemCandidates` を省略)、利用者が持ち物マスタ(`searchItems` の一覧・順)から
  トグルで足す。理由: requirements.md は「ダメージ補正を持つ持ち物」の自動抽出を求めるが、API の `Item` は `id` / `nameJa` だけで
  効果データを持たないため iOS では抽出できない。分類のリストをコードに書くのはハードコードになる(CLAUDE.md)。
  API が効果データを返すようになったら、既定の候補をそこから作る形に差し替える(Web は WASM のマスタの効果データで抽出している)。
- **0.1% 入力**: 出さない(実機の HP 表示は整数%。ADR-0010 §R2 の 2026-09-21 ユーザー回答)。
- **不正な行の扱い**: 空の行は送らないだけ、不正な行が1つでもあれば計算しない(Web と同じ)。「観測を追加」で空の行を足すたびに
  結果が消えると、design.md の「2回目以降を入力すると候補が絞られる」演出が成り立たないため、空の行では止めない。
- **目安の名前**: Web(ADR-0300 §7)と同じ規則で付ける。design.md の「型の名前でまとめる」グループ化はしない(Web も持ち越し。
  ADR-0010 §R 以降は候補 = 性格クラス × 持ち物そのものがまとまりで、サーバーの順が一致度の順)。
- **変化技**: 逆算の技の選択肢から外す(ダメージが出ないので観測を説明できない)。

### XCUITest で確かめること(`POKECALC_USE_MOCK=1`。View と一緒に implementer が書く)

ルート画面に逆算画面への入口 `openReverseScreen` を足す。accessibilityIdentifier は View に1か所で定義する。

| identifier | 要素 |
|---|---|
| `openReverseScreen` | ルート画面の入口 |
| `reverseScreen` | 逆算画面のルート |
| `reverseBackendModeBadge` | モックで動いている表示(ADR-0500 §4) |
| `reverseSide-<ReverseSide の rawValue>` | 与えたダメージ(`defender`)/ 受けたダメージ(`attacker`)の切り替え |
| `reverseMySpeciesPicker` / `reverseOpponentSpeciesPicker` / `reverseMovePicker` / `reverseMyItemPicker` | 選択 UI(種族名は `accessibilityLabel` で読む。計算画面と同じ) |
| `reverseAttackerPreset-<AttackerPreset の rawValue>` | 与えたダメージのときの自分のプリセット |
| `reverseKnownDefenderPreset-<KnownDefenderPreset の rawValue>` | 受けたダメージのときの自分のプリセット |
| `reverseOpponentItemToggle-<itemId>` | 相手の持ち物候補のトグル |
| `reverseObservationField-<行の位置 0 始まり>` | 観測の入力欄(`.keyboardType(.numberPad)`) |
| `reverseObservationError-<行の位置>` | 行の入力エラー(空の行では出さない) |
| `reverseObservationRemove-<行の位置>` / `reverseAddObservationButton` | 行の削除・「観測を追加」 |
| `reversePremise` / `reverseExactCount` | 「H32 を仮定」の文言・一致件数 |
| `reverseCandidateRow-<候補の id>` | 候補カード(例 `reverseCandidateRow-neutral@-`) |
| `reverseCandidateRange-<候補の id>` / `reverseCandidateMatch-<候補の id>` | SP の範囲・一致の文言 |
| `reverseLoadingIndicator` / `reverseErrorMessage` | 読み込み中・エラー表示 |

- 開いた直後は候補が無く、観測欄が1つある。`reverseObservationField-0` に `12` を入力すると
  `reverseCandidateRow-neutral@-` と `reverseCandidateRow-plus@-` が出て、`reversePremise` が見える(モックは defender で H32)。
- `reverseAddObservationButton` で `reverseObservationField-1` が増える。`abc` のような不正値は入力できない(テンキー)ので、
  範囲外の `101` を入力すると `reverseObservationError-1` が出て候補が消える。
- `reverseOpponentItemToggle-<モックの持ち物 ID>` をタップすると候補が 2 → 4 になる。
- `reverseSide-attacker` をタップすると観測欄が空の1つに戻り、候補が消える。
- 候補の数値・範囲の文言は検査しない(モックの数値に依存しない。モックは計算しないので観測を足しても範囲は絞られない)。
- 演出: 観測を足して候補が変わったときだけ、候補カードの出入りと範囲の変化にアニメーション(design.md「動き」)。
  「視差効果を減らす」で無効化。常時動くものは置かない。XCUITest では検査しない。

### 実装メモ

- **`SpeciesHeaderMenuLabel` / `MenuLabelChip` を internal に広げた**: 計算画面(`CalcScreenCards.swift`)の
  種族セレクタのヘッダー部品は元々 `private`(同ファイル内の `AttackerCardView` / `DefenderCardView` だけが使う前提)
  だったが、逆算画面のカード(`ReverseScreenCards.swift`)からも同じ見た目(エンブレム・名前・タイプバッジ)を
  再利用するため `private struct` → `struct`(internal)、`fileprivate static let placeholderName` →
  `static let placeholderName` に広げた。挙動・文言は変えていない。
- **自分のプリセット行**: 与えたダメージ(`side == .defender`。自分が攻撃側)は `AttackerPreset` の3セグメント
  (`reverseAttackerPreset-<rawValue>`)、受けたダメージ(`side == .attacker`。自分が防御側)は
  `KnownDefenderPreset` の3セグメント(`reverseKnownDefenderPreset-<rawValue>`)を、側で排他的に表示する
  (`ReverseScreenView.presetSegmentedRow`)。`KnownDefenderPreset.label(for:)` は相手の技の分類(HB/HD)を要るので、
  技の読み込み前(`selectedMove == nil`)は暫定的に `.physical` 扱いで表示する(起動直後の一瞬だけ)。
- **観測欄の単位**: `ObservationKind.percent` は「%」、`.damage` は「ダメージ」という短い接尾辞をフィールドの
  横に出す(design.md に数値・文言指定が無いため実装側で決めた)。
- **候補の絞り込みアニメーション**(批評対応で書き直し): `ReverseResultsSectionView` は候補カードの `VStack` に
  `.animation(reduceMotion || observationCount <= 1 ? nil : Self.narrowingAnimation, value: result?.candidates.map(\.id))`
  を直接付ける。観測欄が2つ以上(「観測を追加」して2件目以降を入力した状況の近似)のときだけ、候補の id 列が
  変わった瞬間にアニメーションが掛かる(design.md「画面: 逆算」の「「観測を追加」で2回目以降を入力すると候補が
  絞られるアニメーション」)。側の切り替え・起動時(観測欄1つに戻る)では動かない。初版は別の `@State` の
  「ティック」を `onChange` で1手遅れて進めていたため、SwiftUI が描画を終えた*後*にアニメーションの発火条件が
  そろい、実際には動いていなかった(批評で指摘)。「視差効果を減らす」で無効化。XCUITest では検査しない
  (数値検査をしない方針のため)。
- **テンキーの「完了」ボタン**: `.keyboardType(.numberPad)` には Return が無いため、`ReverseScreenView` に
  `.toolbar { ToolbarItemGroup(placement: .keyboard) { ... Button("完了") { focusedObservationID = nil } } }` を
  足した。フォーカスは観測欄をまたいで1つの `@FocusState<Int?>` で共有する。
- **観測欄の `Binding` を同期にした**(批評対応): `ReverseViewModel.editObservation(id:text:)`(async)を
  `TextField` の `Binding` から `Task { await ... }` で呼ぶと、`Task` の起動がイベントループを1回分後回しに
  するため、キー入力から画面表示までに1フレームの遅れが出る。テキストの反映・検証だけを行う同期版
  `setObservationText(id:text:) -> Bool`(送る観測の列が変わったら true)を新設し、`TextField` の `Binding` は
  これを直接呼ぶ。逆算の呼び出しが要るとき(戻り値が true)だけ `recalculateAfterObservationEdit()` を `Task` で
  呼ぶ。既存の async `editObservation(id:text:)` は `setObservationText` + 条件付き `recalculateAfterObservationEdit`
  をまとめた一括版として残したので、`ReverseViewModelTests` の呼び出し側は1つも変えていない。
- **SP の上限を1か所に**(批評対応): `AttackerPreset` / `KnownDefenderPreset` / `ReverseCandidateDisplay` に
  それぞれ `private static let maxStatSP = 32` があったのを、`DomainTypes.swift` の `SPLimits.maxPerStat`
  (CLAUDE.md ドメイン規約「SP は1ステータス最大32」が出典)にまとめ、3か所から参照する形にした。
- **観測欄に VoiceOver ラベルを追加**(批評対応): `reverseObservationField-<index>` に
  `.accessibilityLabel("観測\(index + 1)(\(unitLabel))")` を付けた(空の `TextField` はラベルが無いと
  VoiceOver で読み上げられないため)。
- **一致なし・削除ボタンの色を danger から textSecondary に**(批評対応): design.md の `danger` トークンは
  エラー表示専用。「一致なし(最も近い SP)」は入力エラーではなく候補の性質を表す文言、観測欄の削除ボタンは
  通常の操作なので、どちらも `textSecondary` に直した。
- **側の切り替えもアクセシビリティの大きい文字サイズで縦積みに**(批評対応): `sideSwitch` は `cardsRow` と同じ
  `dynamicTypeSize >= .accessibility1` の分岐で、横2分割のピルと縦積みの2行を切り替える。どちらの並びでも
  セグメントは幅いっぱいに広がる。
- **起動時にいきなり逆算画面を開く環境変数** `POKECALC_OPEN_REVERSE_SCREEN_AT_LAUNCH=1`(`RootView`)を、
  既存の `POKECALC_OPEN_CALC_SCREEN_AT_LAUNCH` と同じ理由(XCUITest を介さずスクリーンショットを撮る用途)で追加した。
  通常の起動・モック/API の判定には影響しない。
- **スクリーンショット確認**(iPhone 18 Pro ライト/ダーク・iPhone 17e ダーク + 文字サイズ extra-extra-large、
  観測を1件入力してキーボードを閉じ、候補カードが2件見える状態): カード・タイプバッジ・プリセットの3セグメント・
  技セレクタ・持ち物候補チップ・観測欄(単位・削除ボタン)・「観測を追加」・前提文言・一致件数・候補カード
  (性格クラス/持ち物・SP範囲・目安の名前・一致文言・想定ダメージ幅)のどれも折り返し・文字切れ・はみ出しは無かった。
  ダークモードもトークンどおりの配色で読める。extra-extra-large でも1行に収まらない項目はカード内で自然に
  折り返し、崩れは無かった(アクセシビリティ文字サイズでのカードの縦積みは計算画面と同じ `dynamicTypeSize >=
  .accessibility1` の分岐を流用しており、計算画面側の批評で確認済みの経路)。

## P6-2c の受け入れ条件(構築ビルダー。ドメイン・TeamStore・ViewModel は実装済み・green。View/XCUITest は未着手)

ADR-0500 §4「構築(team)は API の契約が無い(P5-4)。`TeamStore` プロトコルと端末内の実装(UserDefaults に
JSON)で作り、team-svc の契約ができたら API 実装を足す。Showdown 形式の入出力は team-svc の契約に合わせるため
後回しにする」を具体化する。docs/design.md には構築ビルダー画面の節が無い(方向性 C「カード/ホロのコレクション
風」だけが決まっている)ため、画面のレイアウトは implementer が design.md のトークン(カード角丸20・チップ999・
余白4/8/12/16/24・タイプ色)を使って組み、確定した見た目は本節に追記する。

ロジックは `PokeCalcCore` に置く(ADR-0500 §1)。View と XCUITest は範囲外(実装者の担当)。
spec-writer が失敗する状態で置いたテスト: `PokeCalcKit/Tests/PokeCalcCoreTests/{TeamDomainTypesTests,
LocalTeamStoreTests,TeamListViewModelTests,TeamEditViewModelTests,TeamMemberConversionTests}.swift`、
`Tests/PokeCalcCoreTests/Support/StubTeamStore.swift`。`swift test`(`swift build --build-tests` でも同様)は
`TeamMember` / `Team` / `TeamStore` / `LocalTeamStore` / `TeamValidator` / `TeamLimits` / `TeamScreenError` /
`TeamFieldError` / `TeamMemberFieldError` / `TeamListViewModel` / `TeamEditViewModel` / `TeamMemberConverter`
と `SPLimits.maxTotal` / `PokeCalcError.Code.team*` が存在しないため**コンパイルエラーで失敗する**
(2026-09-22 に `swift build --build-tests` で確認済み。`cannot find type 'Team' in scope` 等)。
実装者はこれらの型を新設し、テストを変更せずに通すこと(ここに書いた形と食い違うテストがあれば、テストを
先に critic 相談なしでは変えない。CLAUDE.md 絶対ルール6)。

### 1. ドメインの型(新規ファイル。DomainTypes.swift は変更しない)

- `TeamLimits.maxMembers == 6`(requirements.md「構築ビルダー」の6体パーティ)、
  `TeamLimits.maxMovesPerMember == 4`(ADR-0016 §1 balance TB2 と同じ規則を踏襲。メンバーごとの技IDは最大4つ、
  同一メンバー内の重複は不可)。
- 既存の `SPLimits`(DomainTypes.swift)に `maxTotal = 66` を追加する(CLAUDE.md ドメイン規約「合計66」。
  新しい型を作らず、`maxPerStat` と同じ場所に並べる)。
- `TeamMember`(`Equatable, Sendable, Codable`): `id`(既定 UUID 文字列)・`speciesKey`・`nickname: String?`
  (**判断**: 持つ。requirements.md はニックネームに触れていないが、実機の構築には一般的にニックネームがあり、
  低コストな任意フィールドなので先に用意する。Showdown 形式は後回しだが、ニックネームは Showdown 形式の1行目
  そのものなので、将来のインポート/エクスポートと形を合わせやすくする意図もある)・`moveIds: [String]`
  (0〜4、`TeamLimits.maxMovesPerMember` を超えない・同一メンバー内で重複しない)・`itemId: String?`・
  `abilityId: String?`・`natureId: String`(必須。空文字の禁止はこのタスクでは検証しない。
  `TeamEditViewModel.addMember` が常にマスタの性格を既定値として入れるため、ViewModel 経由では空にならない)・
  `sp: StatBlock`(既定 全0)・`teraType: PokeType?`。
- `Team`(`Equatable, Sendable, Codable`): `id`(既定 UUID 文字列)・`name: String`(必須。前後空白だけは無効。
  **判断**: 文字数上限は設けない。要件に無い数値を作らない[coding-rules「ハードコードしない」])・
  `members: [TeamMember]`(0〜6、追加順)。
  **範囲外(判断)**: メンバーの並べ替え(ドラッグでの reorder)はこのタスクでは実装しない。requirements.md に
  並べ替えの要求が無く、`TeamStore` に無い操作を UI 無しで作ると検証できないため(coding-rules「使われていない
  汎用機構を作らない」)。必要になったら `docs/plan.md` に follow-up として積む。
- `TeamValidator.firstViolation(in: Team) -> PokeCalcError?`(純粋関数): 判定順は
  TeamDomainTypesTests.swift のコメントを正とする(name空 → members超過 → メンバーを先頭から
  技数超過/技重複/SP単体超過/SP合計超過)。**判断**: 無効な値は丸めて保存し直さず「却下」する
  (超過した SP や重複した技を黙って正規化すると、保存した内容が画面の見た目と食い違いうるため)。
- 新しい `PokeCalcError.Code`(`client_` 接頭辞。PokeCalcError.swift の既存の書き方に合わせて追記する):
  `teamNameEmpty` / `teamTooManyMembers` / `teamTooManyMoves` / `teamDuplicateMoves` / `teamSPInvalid` /
  `teamStoreCorrupted`。

### 2. `TeamStore` / `LocalTeamStore`

- `TeamStore`(`protocol, Sendable`): `list() async throws -> [Team]` / `get(id:) async throws -> Team?` /
  `save(_ team: Team) async throws`(id が既存なら更新・無ければ追加。`TeamValidator` で却下されうる) /
  `delete(id:) async throws`。reorder/move は持たない(上記「範囲外」と同じ理由)。
- `LocalTeamStore`(`actor`, `TeamStore` 準拠): `init(defaults: UserDefaults = .standard)`
  (`ClientIdentity(defaults:)` と同じ、保存先を注入できる形。テストは `ClientIdentityTests` と同じ手法で
  専用の UserDefaults suite を使う)。
- 保存キー: `LocalTeamStore.teamsDefaultsKey == "PokeCalcTeams"`(`ClientIdentity.deviceIDDefaultsKey`
  `"PokeCalcDeviceID"` と衝突しない)。
- **判断(保存形式)**: 1つのキーの下に `[Team]` を丸ごと JSON で保存する(team ごとに別キーにしない)。
  理由: 端末内は最大6体 × 数チーム程度の小さいデータで、複数キーにすると「どのチームがあるか」を知るための
  索引キーが別途要り、索引と実体の不整合が起こりうる。1キーなら常に一貫する。
  実装のために `TeamMember` / `Team` が `Codable` である必要があり、その内部で使う `StatBlock` / `RankBlock` /
  `PokeType` を `Codable` にする(DomainTypes.swift への追加)。Swift の自動 `Codable` 合成は「元の型を
  宣言したファイルと同じファイルで `Codable` に準拠する」必要があるため、これらの型は DomainTypes.swift 側で
  `Codable` に準拠させること(別ファイルの extension では合成されない)。あるいは永続化専用の DTO を
  `LocalTeamStore` 側に置いて手動でマッピングしてもよい(ADR-0500 §3 の「生成型↔ドメインの写像は1か所」と
  同じ発想で、持続化の都合をドメイン型に持ち込みたくなければこちらを選んでよい)。どちらでも
  `LocalTeamStoreTests` の往復テストは形を問わない(`JSONEncoder`/`JSONDecoder` で往復することだけを見る)。
- 壊れたデータ(UserDefaults の値が JSON として teams にデコードできない)は
  `PokeCalcError(code: .teamStoreCorrupted)` を投げる(黙って空配列にしない)。

### 3. `TeamListViewModel` / `TeamEditViewModel`

2つに分ける(**判断**: 一覧画面と編集画面は別の状態・別のマスタ依存を持つため、1つにまとめると
「一覧だけ見たいときも編集用のマスタ読み込みが要る」ことになり無駄。`CalcViewModel`/`ReverseViewModel` が
画面ごとに分かれているのとも一貫する)。

- `TeamListViewModel`(`@MainActor @Observable`): `init(store: any TeamStore)`。`teams` / `isLoading` /
  `error: TeamScreenError?`。`load()` は呼ぶたびに `store.list()` して最新化する(`CalcViewModel.load()` の
  一度きりガードは付けない。一覧画面は編集画面から戻るたびに再読み込みが要るため)。`createTeam(name:) async ->
  Team?`(空白だけの名前は `store.save` を呼ばず `PokeCalcError.Code.teamNameEmpty` の `error` を立てる)。
  `deleteTeam(id:) async`(削除後に `load()` して最新化)。
- `TeamEditViewModel`(`@MainActor @Observable`): `init(store: any TeamStore, service: any PokeCalcService,
  team: Team)`。マスタは `CalcViewModel` と同じ `PokeCalcService`(P6-2a で実装済み)を再利用する
  (構築専用のマスタ取得は増やさない)。特性は `SpeciesDetail.abilities`(種族ごと)から出す(`PokeCalcService`
  に特性検索 API が無いため)。
  `team` / `speciesOptions` / `itemOptions` / `natureOptions` / `moveOptionsByMember: [String: [Move]]`
  (メンバー id → 現在の種族の learnset の順・マスタにある技だけ。`CalcViewModel.moveOptions` と同じ規則) /
  `abilityOptionsByMember: [String: [Ability]]` / `isLoading` / `error: TeamScreenError?` /
  `nameError: TeamFieldError?` / `teamError: TeamFieldError?`(6体超の追加) /
  `memberErrors: [String: TeamMemberFieldError]`。
  `load()`・`setName(_:)`・`addMember(speciesKey:) async -> Bool`・`removeMember(id:)`・
  `setMemberSpecies(id:speciesKey:) async`(種族を差し替えたら learnset に無い技を黙って落とす。
  `species(key:)` の応答はメンバーごとの世代トークンで守り、連続で種族を変えたときに古い応答が後から
  上書きしないこと。`CalcViewModel` の species 世代保護[M1]と同じ理由・同じテスト手法)・
  `addMove(id:moveId:) -> Bool` / `removeMove(id:at:)`・`setMemberItem`/`setMemberAbility`/
  `setMemberNature`/`setMemberTeraType`(そのまま代入。`CalcViewModel.selectAttackerItem` と同じ扱いで
  ID の存在チェックはしない)・`setMemberSP(id:stat:value:) -> Bool`(`0...SPLimits.maxPerStat` の外、
  または合計が `SPLimits.maxTotal` を超えるなら**変更せず** false + `memberErrors` を立てる。
  **判断**: 無効な入力をいったん状態に入れてから検証する[`ObservationFieldError` 方式]のではなく、
  変更そのものを拒否する。SP はステッパー/スライダー操作が主で、範囲外の値を一瞬でも保持する UI 上の理由が
  無いため。ステートは常に妥当 = `TeamValidator` が引っかかるのは `TeamStore` に直接触る将来の実装
  [例: API 実装]からの経路だけ、という設計にする)・`save() async -> Bool`。
  `TeamFieldError`: `.emptyName` / `.tooManyMembers`。`TeamMemberFieldError`: `.duplicateMove` /
  `.tooManyMoves` / `.spPerStatExceeded` / `.spTotalExceeded`。
- `TeamScreenError`(`Equatable, Sendable`): `CalcScreenError`(P6-2a)と同じ3ケース
  (`.transport` / `.unexpectedResponse` / `.service(code:message:)`)・同じ `init(_ error: any Error)`。
  **判断**: 構築を別型にする(`CalcScreenError` を再利用しない)。理由は将来 `APITeamStore`
  (ADR-0500 §4 の「team-svc の契約ができたら足す」)に切り替わったときの通信エラーが計算画面のエラー文言と
  混ざらないようにするため。ロジック(`init` の分岐)は同じでよい。

### 4. `TeamMemberConverter`(「構築から個体を呼び出す」の下ごしらえ・**範囲外の明記**)

requirements.md「自分側のプリセット: 構築から個体を呼び出す」に備え、`TeamMember` → `Individual` の純粋な
変換関数 `TeamMemberConverter.makeIndividual(from: TeamMember, moves: [Move]) -> Individual` を用意し
テストで固定する(TeamMemberConversionTests.swift)。`speciesKey`/`natureId`/`sp`/`itemId`/`abilityId`/
`teraType` はそのまま写す(`Individual` も ID をそのまま運ぶ値型なので、この変換時点でマスタ照合はしない)。
`moveId`(`Individual` は1つしか持てない)は `moveIds` から「最初の変化技でない技(`moves` で判定)、
無ければ `moveIds` の最初、空なら nil」で選ぶ(`CalcViewModel.reselectMove` の既定技の選び方と同じ規則)。

**この変換関数を `CalcViewModel` / `ReverseViewModel` の「構築から呼び出す」ボタンとして実際に配線するのは
このタスク(P6-2c)の範囲外**。依頼文の指示どおり、後で配線できるように純粋関数を用意してテストで固定する
ところまでを行う。配線は `docs/plan.md` に別タスクとして積む(P6-2c の続き、または新しいタスク番号)。

### 5. View / XCUITest(実装者の担当。accessibilityIdentifier 契約)

画面のレイアウト自体は design.md に節が無いため implementer が組むが、XCUITest から辿れるように
以下の識別子を**この名前で**付けること(他画面の命名規則: 画面ルートは `<screen>Screen`、カードは
`<role>Card`、ピッカーは `<role>Picker`、行は `<種類>Row-<id>`)。

- 構築一覧画面: ルート `accessibilityIdentifier("teamListScreen")`。新規作成の入口
  `accessibilityIdentifier("createTeamButton")`。各行 `accessibilityIdentifier("teamRow-\(team.id)")`、
  行内の削除操作 `accessibilityIdentifier("teamDelete-\(team.id)")`。空(0件)のときの案内文言
  `accessibilityIdentifier("teamListEmpty")`。ロード中 `accessibilityIdentifier("teamListLoadingIndicator")`。
  エラー文言 `accessibilityIdentifier("teamListErrorMessage")`。`RootView` に一覧画面への入口を足す場合は
  既存の `openCalcScreen`/`openReverseScreen` に揃えて `accessibilityIdentifier("openTeamListScreen")`。
- 構築編集画面: ルート `accessibilityIdentifier("teamEditScreen")`。チーム名の入力欄
  `accessibilityIdentifier("teamNameField")`、そのエラー `accessibilityIdentifier("teamNameError")`。
  メンバー追加ボタン `accessibilityIdentifier("addMemberButton")`。メンバーごとのカード
  `accessibilityIdentifier("memberCard-\(member.id)")`、削除
  `accessibilityIdentifier("memberDelete-\(member.id)")`、種族ピッカー
  `accessibilityIdentifier("memberSpeciesPicker-\(member.id)")`、持ち物ピッカー
  `accessibilityIdentifier("memberItemPicker-\(member.id)")`、特性ピッカー
  `accessibilityIdentifier("memberAbilityPicker-\(member.id)")`、性格ピッカー
  `accessibilityIdentifier("memberNaturePicker-\(member.id)")`、テラスタイプピッカー
  `accessibilityIdentifier("memberTeraPicker-\(member.id)")`、技スロット(0始まりの index)
  `accessibilityIdentifier("memberMoveSlot-\(member.id)-\(index)")`、SP 入力(StatKey ごと)
  `accessibilityIdentifier("memberSP-\(member.id)-\(stat.rawValue)")`、メンバーのエラー文言
  `accessibilityIdentifier("memberError-\(member.id)")`。保存ボタン
  `accessibilityIdentifier("saveTeamButton")`。画面全体のエラー
  `accessibilityIdentifier("teamEditErrorMessage")`。

XCUITest はモック(`MockPokeCalcService` + `LocalTeamStore`。起動時 `POKECALC_USE_MOCK=1`)で
「一覧を開く→新規作成→名前を付けて保存→一覧に出る→開いてメンバーを1体追加して保存→一覧から削除できる」の
一連がつながることを見る(P6-1〜P6-2b の XCUITest と同じ粒度。数値の正しさではなく操作がつながることを見る)。

### 6. コンパイルを止めていた2つのブロッカーと直し方(解決済み。次の implementer/critic 向けの記録)

一時的な実装(実装者A)が `swift build --build-tests` を通せずに中断した。原因は2つとも解決済み:

1. **`LocalTeamStore`(actor)の init に `UserDefaults`(非 Sendable)を渡す箇所の Swift 6 concurrency エラー**
   (`sending 'self.defaults' risks causing data races`)。`init` の中で `@unchecked Sendable` の箱に包んでも
   直らない(region-based sending チェックは**呼び出し側から見える init の引数の宣言型**で判定するため、
   init の中で何をしても呼び出し側の型は変わらない)。正しい直し方は `extension UserDefaults: @retroactive
   @unchecked Sendable {}` をこのモジュール内に置くこと(`LocalTeamStore.swift` にコメント付きである)。
   これで公開 API `init(defaults: UserDefaults = .standard)` もテストの呼び出し `LocalTeamStore(defaults:
   defaults)` も変更せずに済む。
2. **`TeamListViewModel` / `TeamEditViewModel` に `@MainActor` を付けると spec-writer のテストがコンパイル
   エラーになる、という実装者Aの判断は誤り**。原因はテストクラス自体に `@MainActor` を付けていなかった
   ことで、`CalcViewModelTests` / `ReverseViewModelTests` が `@MainActor final class` になっているのと
   同じパターンで `TeamListViewModelTests` / `TeamEditViewModelTests` にも `@MainActor` を付ければ解決する
   (アサーション・期待値は一切変更していない。CLAUDE.md 絶対ルール6に抵触しない)。両 ViewModel は本章の
   指定どおり `@MainActor @Observable` のまま。
3. 上記2つとは別に、spec-writer のテストファイル自体に `await` が `XCTAssertEqual` の autoclosure 引数内に
   あるという Swift 構文エラーが2件あった(`TeamListViewModelTests.swift`(旧行 79・105)・
   `LocalTeamStoreTests.swift`(旧行 124-125))。`let` で一度受けてから `XCTAssertEqual` に渡す形に直した
   (`ReverseViewModelTests.swift` で既に使われているのと同じパターン)。比較する値・アサーションの内容は
   変えていない。

`swift test`(macOS)・`make ios-test-unit`(シミュレータ)とも Team* を含む全件が green(2026-09-22)。

### 7. 確認事項(未決のまま残った判断はここに書く。次の implementer/critic が変えてよい)

- **`TeamMember.nickname`(解決)**: critic 指摘(coding-rules §3「使われていない機構を作らない」)を受け、
  消さずに配線した。`TeamEditViewModel.setMemberNickname(id:nickname:)`(前後空白を落とし、空なら nil。
  `setName` と同じ規則)と `TeamEditMemberCard.nicknameField`(identifier `memberNickname-<id>`。5章の
  identifier 契約には無い追加)。テストは `TeamEditViewModelTests.testSetMemberNicknameTrimsAndTreatsBlankAsNil`。
- メンバーの並べ替えを本当に要るかは未確認のまま(範囲外とした。requirements.md に明記が無いため)。
- **(P6-2c 続き。View/XCUITest の実装で決めたこと)** 以下は implementer(View/XCUITest 担当)が
  ADR に無かった判断をした箇所。次の critic/implementer が変えてよい。
  - **画面の見た目**: design.md に構築ビルダーの節が無いため、方向性 C「カード/ホロのコレクション風」の
    トークン(カード角丸20・チップ999・余白4/8/12/16/24・タイプ色・Liquid Glass)を Calc/Reverse 画面と
    同じ部品(`glassCard`・`MenuLabelChip`・`SpeciesHeaderMenuLabel`・`ErrorBannerView` 等)を再利用して
    組んだ。一覧の行はチーム名・「メンバー n/6」・6個のドット(埋まった数だけ塗る)で構成し、タイプ色は
    使わない(一覧画面はマスタ[種族]を読まない設計[3章]なので、行に種族のタイプ色エンブレムを出そうとすると
    一覧 VM がマスタ依存を持つことになり、3章の「一覧だけ見たいときも編集用のマスタ読み込みが要ることに
    なり無駄」という判断に反する。実データが無いので無彩色のドットで数だけ示すことにした)。
  - **`ErrorBannerView` の identifier 化**: 既存の `ErrorBannerView`(`CalcScreenResults.swift`)は
    `accessibilityIdentifier` が `"calcErrorMessage"` に決め打ちで、逆算画面もそのまま流用していた。
    構築は `teamListErrorMessage` / `teamEditErrorMessage` を持つ必要がある(5章)ため、
    `identifier: String = "calcErrorMessage"` という既定引数を追加した(Calc/Reverse の呼び出し側は
    無変更のまま挙動を維持)。
  - **`NavigationStack` を入れ子にしない**: 最初 `TeamListView` に専用の `NavigationStack` を持たせて
    実装したところ、実機(シミュレータ)で一覧画面の中身が描画されず、SwiftUI が代わりに小さな
    `exclamationmark.triangle.fill`(「警告」)のプレースホルダだけを表示する不具合に遭遇した
    (XCUITest が `teamListScreen` を見つけられず timeout。`xcrun simctl launch` で直接起動して
    アクセシビリティツリーをダンプして特定した)。`NavigationStack` の入れ子は SwiftUI が推奨しない
    パターンで、`RootView` の `NavigationStack` の中に別の `NavigationStack` を作らず、`TeamListView`
    は `RootView` が持つ同じ `path: NavigationPath` を `@Binding` で共有する形に直した
    (`navigationDestination(for:)` は宣言した場所に関わらず最も近い祖先の `NavigationStack` に登録される
    ため、`TeamListView` 自身が `.navigationDestination(for: String.self)` を宣言してもスタックは
    増えない)。次にこのパターンで詰まったときのために `TeamListView.swift` の冒頭コメントに残した。
  - **`.alert` 内 `TextField` の `accessibilityIdentifier` は効かない**: 新規作成アラートの名前欄に
    `.accessibilityIdentifier("createTeamNameField")` を付けたが、SwiftUI の `.alert` は中身を
    UIKit の `UIAlertController`/`UITextField` に変換して描画するため、この identifier は実機
    (シミュレータ)のアクセシビリティツリーに反映されなかった(XCUITest で `waitForExistence` が
    timeout し、`app.debugDescription` で `TextField` に identifier が付いていないことを確認して特定)。
    identifier の指定はコードから削除し、XCUITest 側は `app.alerts.textFields.firstMatch`
    (アラートに入力欄は1つしか無い)で辿る形にした。ADR 5章の identifier 表にはこの欄の名前を
    書いていなかったので、契約を満たせないという問題ではないが、次に `.alert` へ独自 identifier を
    付けたくなったときのためにここに残す。
  - **SP ステッパーのラベル幅で折り返る不具合**: 実装直後の実機スクリーンショットで、能力ポイントの
    ステータス名(「こうげき」「ぼうぎょ」「とくこう」「とくぼう」)が `.frame(width: 56)` の固定幅で
    2行に折り返っていた(このアプリで繰り返し起きているカードヘッダー折り返しと同じ種類の不具合。
    `TeamEditMemberCard.spStepper` 参照)。`.lineLimit(1)` + `.fixedSize()` + `.frame(minWidth: 64,
    alignment: .leading)`(固定 `width` をやめ、下限だけ揃える)に直し、iPhone 18 Pro
    ライト/ダーク・iPhone 17e ダーク+extra-extra-large の全パターンで1行に収まることを確認した。
  - **新規作成の UI フロー**: `TeamListViewModel.createTeam(name:)` を活かすため、一覧画面の
    「新規作成」ボタンは `.alert` で名前を聞いてから作成・保存し、成功したら編集画面へ遷移する形にした
    (「一覧を開く→新規作成→名前を付けて保存→一覧に出る→…」という5章の XCUITest の粒度と一致)。
  - **`RootView` の `LocalTeamStore`**: `AppEnvironment` は計算/逆算が使う `PokeCalcService` の
    生成元なので、契約の異なる `TeamStore` はそこに混ぜず、`RootView` が `@State` で個別に持つ形にした。
    `POKECALC_USE_MOCK=1`(XCUITest・スクリーンショット撮影)のときは専用の `UserDefaults` suite
    (`PokeCalcTeamsUITest`)を起動のたびに空にする(前回の実行の構築が残らないようにする実装判断。
    モックは決定的であるべきという方針[P6-1章]を `LocalTeamStore` にも広げた)。通常起動
    (`POKECALC_USE_MOCK` 無し)は `UserDefaults.standard` にそのまま保存し、構築は端末に残る。
  - **起動時に構築一覧を開く環境変数**: Calc/Reverse と同じパターン(`POKECALC_OPEN_CALC_SCREEN_AT_LAUNCH`
    等)で `POKECALC_OPEN_TEAM_LIST_SCREEN_AT_LAUNCH` を追加し、`ios/scripts/sim-run.sh` の
    `<画面>` 引数に `team` を足した(手順書のスクリーンショット撮影用。5章の識別子契約には無い追加だが、
    Calc/Reverse で確立済みの手順に構築だけ抜けるのを避けた)。
  - **(critic 2回目レビュー前。orchestrator が直接修正)`RootView` の init 副作用**: `_teamStore =
    State(initialValue: Self.makeTeamStore())` は `RootView.init` が呼ばれるたびに `makeTeamStore()`
    の式そのものを評価する(`@State` が使い回すのは戻り値だけ)。モック時の `makeTeamStore()` は専用
    `UserDefaults` suite を `removePersistentDomain` で消去する副作用を持っていたため、SwiftUI が
    `RootView` を再生成するたび(親の再描画のたび)に、起動後に作った構築が消える不具合になりえた
    (`PokeCalcApp.swift` が同じ理由で避けている副作用)。修正: 消去の副作用を `static let
    mockTeamStoreDefaults`(プロセスで1回だけ評価される)に閉じ込めた。`makeTeamStore()` 自体は
    副作用を持たない。
  - **(同上)`TeamEditView` に読み込み中インジケータが無かった**: `TeamListView.loadingSlot` と同じ
    (高さ固定・`viewModel.isLoading` のときだけ表示)ものを追加した(identifier
    `teamEditLoadingIndicator`。5章の契約には無い追加)。

## P6-2d の受け入れ条件(構築から個体を呼び出す。テスト先行・実装未着手)

requirements.md「**自分側のプリセット**: 構築から個体を呼び出す / 「A特化」「A振り(補正なし)」「無振り」から選ぶ」の
前半(構築から個体を呼び出す)を、計算画面(P6-2a)と逆算画面(P6-2b)に配線する。P6-2c 4章で
「配線は範囲外・別タスク」としていた続きにあたる。

ロジックは `PokeCalcCore` に置く(ADR-0500 §1)。View と XCUITest は範囲外(implementer の担当。5章の
identifier 契約をそのまま使うこと)。
spec-writer が失敗する状態で置いたテスト:
`PokeCalcKit/Tests/PokeCalcCoreTests/{TeamIndividualSelectionTests,CalcViewModelTeamIndividualTests,ReverseViewModelTeamIndividualTests}.swift`、
`Tests/PokeCalcCoreTests/Support/StubTeamStore.swift` への追記(手動モード `setListMode(.manual)` /
`resolveList(at:with:)` / `waitForListCalls(count:)` と架空の構築フィクスチャ `StubTeams`)。
`swift build --build-tests` は `BuildSource` / `TeamIndividualSelection` / `TeamPickerGroup` / `TeamMemberOption` /
`TeamIndividualOptions` / `TeamLoadLabels` と、両 ViewModel の `teamOptions` / `loadTeams()` /
`selectTeamIndividual(teamID:memberID:)` / `attackerBuildSource` / `knownDefenderBuildSource` が
存在しないためコンパイルエラーで失敗する。実装者はこれらを新設し、**既存のテストを1行も変えずに**通すこと
(CLAUDE.md 絶対ルール6)。

### 1. 「自分側」の出どころを1つの直和にする(中核の判断)

いまは `CalcViewModel.attackerPreset` / `ReverseViewModel.attackerPreset` / `knownDefenderPreset` が
「自分側」の SP・性格の**唯一の**出どころで、要求を組み立てるたびに `AttackerPreset.build` /
`KnownDefenderPreset.build` で毎回作り直している。構築の個体は手で詰めた SP と専用の性格を持つのが普通で、
3つの定型プリセットのどれとも一致しない。**呼び出した個体はその個体の値をそのまま使う**(プリセットに
丸め直さない)ことが、この機能の意味そのものなので、出どころを直和にする。

新規ファイル `PokeCalcKit/Sources/PokeCalcCore/TeamBuildSource.swift`:

```swift
public enum BuildSource<Preset: Equatable & Sendable>: Equatable, Sendable {
    case preset(Preset)
    case team(TeamIndividualSelection)
    public var preset: Preset? { get }              // .team のときは nil
    public var teamSelection: TeamIndividualSelection? { get }  // .preset のときは nil
}
public typealias AttackerBuildSource = BuildSource<AttackerPreset>
public typealias KnownDefenderBuildSource = BuildSource<KnownDefenderPreset>
```

- **判断: 2つの具体 enum ではなく総称型にする**。`AttackerPreset` と `KnownDefenderPreset` で中身が違うだけで
  分岐の形は同じなので、同じ定義を2回書かない(coding-rules「読みやすいコード」)。
- **判断: `attackerPreset` はプロパティとして残すが `AttackerPreset?`(計算プロパティ)にする**。
  `attackerBuildSource.preset` をそのまま返す。構築の個体を呼んでいる間は nil = 「どのプリセットも選ばれていない」。
  これで View のピルの選択表示(`viewModel.attackerPreset == preset`)も、既存のテストの
  `XCTAssertEqual(viewModel.attackerPreset, .aFull)` も**無変更で**成り立つ(Swift の optional 昇格)。
  `knownDefenderPreset` も同じく `KnownDefenderPreset?` にする。
  代案(`attackerPreset` を保存プロパティのまま残し、別に `teamIndividual: TeamIndividualSelection?` を足す)は
  「プリセットも構築も選ばれている」という無効な状態が型として作れてしまい、排他を実装の約束事に頼ることになるので採らない。

呼び出した個体のスナップショット:

```swift
public struct TeamIndividualSelection: Equatable, Sendable {
    public let teamID: String
    public let memberID: String
    /// 画面に出す名前(ニックネーム、無ければ種族名、それも無ければ speciesKey)
    public let displayName: String
    /// 選んだ時点の `TeamMemberConverter.makeIndividual(from:moves:)` の結果
    public let individual: Individual
}
```

- **判断: ID だけを持たず、選んだ時点の `Individual` を写して持つ(スナップショット)**。ID だけにすると
  要求を組み立てるたびに `TeamStore` を引く必要があり、`buildRequest` が async になって
  `CalcViewModel`/`ReverseViewModel` の「組み立ては同期・純粋」という形(既存のプリセット経路と同じ)が崩れる。
  副作用として、呼び出したあとに構築ビルダー側でその個体を編集しても計算画面は追従しない(呼び出し直すと反映される)。
  これは「呼び出す」という言葉どおりの挙動で、画面をまたいだ暗黙の同期より予測しやすい。

### 2. 要求の組み立てが何を使うか(フィールドごとの出どころ)

`.team` のとき、`buildRequest` は `AttackerPreset.build` / `KnownDefenderPreset.build` を**呼ばない**。

| フィールド | `.preset` | `.team` |
|---|---|---|
| `natureId` / `sp` | `*.build(...)` の結果 | スナップショットの値そのまま |
| `abilityId` / `teraType` | 常に nil(画面に UI が無い) | スナップショットの値そのまま |
| `speciesKey` | 画面の種族(`attackerSpeciesKey` / `mySpeciesKey`) | 同左 |
| `itemId` | 画面の持ち物(`attackerItemId` / `myItemId`) | 同左 |
| `Individual.moveId` | nil(いまの実装のまま) | nil に**そろえる**(下記) |
| 要求本体の `moveId` | 画面の技(`moveId`) | 同左 |

`Individual.moveId` は `.preset` 経路でいまも nil のままで、技は要求本体の `moveId` が正。`.team` でも
スナップショットの `moveId` を要求へ持ち込まず nil にそろえる(同じ意味の値を要求の2か所に置いて
食い違わせないため)。スナップショットの `moveId` は「呼び出した瞬間に画面の技を決める」ためだけに使う(下記3)。

- **判断: 種族・持ち物・技は「画面の状態」を唯一の正とし、スナップショットで上書きしない**。呼び出した瞬間には
  この3つも個体の値で**画面の状態ごと**書き換える(下記3)ので見た目と要求は一致し、そのあと利用者が
  種族セレクタ・持ち物・技を動かしたぶんは素直に効く。スナップショット側を正にすると、画面に出ている技と
  要求の技が食い違う(利用者が技を変えても反映されない)。
- **判断: 種族を変えても `.team` は外れない**。SP と性格は種族に依存しない値であり、なにより `swapSides()` は
  `attackerSpeciesKey` を書き換えるので、種族変更で外す設計にすると攻守入れ替えのたびに自分の調整が
  黙って消える。P6-2a 規則6「攻守入れ替えでプリセットは入れ替えない」と同じ扱いにそろえる。
- **排他(判断)**: `.preset` と `.team` は型として排他。プリセットのピルを押せば `.preset(押したもの)` になり
  構築の選択は外れる。構築の個体を選べば `.team(...)` になりプリセットの選択表示は消える(`attackerPreset == nil`)。
  requirements.md が「構築から呼び出す / 3つから選ぶ」と並べていることに一致する。
  **専用の「解除」ボタンは置かない**(プリセットを1つ押すことが解除そのもので、ボタンを増やすと
  「解除したあと何が選ばれているか」という第3の状態を作ってしまう)。

### 3. 呼び出したときに画面の状態をどう動かすか

`selectTeamIndividual(teamID:memberID:) async`(両 ViewModel)の手順:

1. `teamOptions` に無い `teamID`/`memberID` は**無視**する(計算も呼ばない)。既存の
   `selectAttacker(speciesKey:)` が未知の種族キーを無視するのと同じ扱い。
2. `beginInput()` で世代を1つ進める(以降 P6-2a 規則7・P6-2b 規則8の保護をそのまま受ける)。
3. `TeamMemberConverter.makeIndividual(from: member, moves: masterMoves)` で `Individual` を作る
   (この関数を新しく書き直さない。P6-2c 4章で用意してテストで固定済み)。
4. 自分の種族を個体の `speciesKey` に、自分の持ち物を個体の `itemId` にする。
5. 技: **計算画面**と**逆算の「与えたダメージ」**(自分が攻撃側)は、自分の learnset を読み直してから
   既存の `reselectMove(preferringCurrent: 個体の moveId)` を通す。
   **逆算の「受けたダメージ」**(自分が防御側)は技が相手の learnset なので**技に触れない**。
6. `attackerBuildSource` / `knownDefenderBuildSource` を `.team(...)` にして、計算を1回だけ呼ぶ
   (逆算は P6-2b 規則5どおり「計算できる状態なら1回」)。

**技の落としどころ(判断)**: `moveIds` が空で `Individual.moveId` が nil のとき、および個体の技が
いまの `moveOptions`(learnset ∩ マスタ。逆算はさらにダメージ技だけ)に無いときは、
**既存の規則3・4のまま「learnset の順で最初のダメージ技(計算画面は無ければ learnset の最初)」に落とす**。
エラーにはしない。理由: 技が1つも無い構築の個体は構築ビルダー上ふつうに作れる(`moveIds` は 0〜4)ので、
それを画面のエラーにすると正常なデータで計算画面が止まる。新しいエラーコードも増やさない
(`moveOptions` が空のときだけ既存の `moveUnavailable` が出るのは今までどおり)。
副作用として、逆算の「与えたダメージ」で変化技しか持たない個体を呼ぶと技は最初のダメージ技に落ちる
(逆算は変化技を扱えないため。P6-2b「判断した点」)。

### 4. 逆算のどちら側に効かせるか(判断: 両側)

requirements.md の「自分側のプリセット」は側を限定していない。逆算の「受けたダメージ」では**自分が防御側**で、
`known` に入るのは自分の個体そのものなので、ここも「自分側」である。しかも `KnownDefenderPreset`
(無振り / HB(HD)振り / HB(HD)特化)は H・B(D) を 0 か 32 のどちらかに決め打ちする粗い3択で、
実際の構築の耐久調整はほぼ確実にこのどれとも一致しない。**プリセット近似がいちばん外れる場所**なので、
ここを外すと機能の価値が半減する。攻撃側だけに入れて防御側に入れない理由が見当たらないため、両側に入れる。

- `ReverseViewModel` は側ごとに独立した出どころを持つ(`attackerBuildSource` / `knownDefenderBuildSource`)。
- `selectTeamIndividual(teamID:memberID:)` は**いま表示している側**(`side`)の出どころだけを `.team` にする。
  画面はもともと側ごとに排他でプリセット行を出している(`ReverseScreenView.presetSegmentedRow`)ので、
  入口も1つでよい。側を切り替えても、それぞれの側の出どころは P6-2b 規則7「両方のプリセットは残す」と
  同じく保たれる。自分の種族(`mySpeciesKey`)は両側で共有の値なので、どちら側で呼んでも書き換わる。
- `selectAttackerPreset(_:)` / `selectKnownDefenderPreset(_:)` はそれぞれ**自分の側だけ**を `.preset` に戻す
  (もう一方の側の構築の選択は外さない)。

### 5. `TeamStore` を画面へ届ける配線(判断: コンストラクタ注入 + 任意)

- `CalcViewModel.init(service:)` → `init(service: any PokeCalcService, teamStore: (any TeamStore)? = nil)`。
  `ReverseViewModel` も同じ。`CalcScreenView` / `ReverseScreenView` の `init` に `teamStore: any TeamStore` を足し、
  `RootView` が既に `@State` で持っている `teamStore` を `navigationDestination` から渡す。
  ADR-0500 §3 の「画面は生成型を直接使わずプロトコルだけを使う」に沿った形で、`Environment` 注入は使わない
  (既存の `PokeCalcService` / `TeamStore`(`TeamListView`)と同じ渡し方にそろえる)。
- **判断: `teamStore` は省略可(既定 nil)にする**。理由は2つ。(a) 既存の `CalcViewModel(service: stub)` /
  `ReverseViewModel(service: stub)` を使うテストを1行も変えずに通すため(絶対ルール6)。
  (b) `LocalTeamStore()` を既定値にすると、ViewModel を作っただけで `UserDefaults.standard` に触れることになり、
  テストが端末の実データに依存する。nil のときは `teamOptions` が常に空で、構築の入口は「まだ構築がありません」
  の無効状態になる(下記7)。
- **判断: 構築の読み込みが失敗しても計算画面は壊さない**。`loadTeams()` が `store.list()` の失敗を拾っても
  `error`(`CalcScreenError`)を立てず、`teamOptions` を空のままにする。CLAUDE.md 絶対ルール5
  「計算はイベント保存に依存しない」と同じ発想で、保存の都合で計算が止まらないようにする。

### 6. 構築の一覧を画面に出す形

```swift
public struct TeamMemberOption: Identifiable, Equatable, Sendable {
    public let id: String          // TeamMember.id
    public let displayName: String // nickname(前後空白を落として空でなければ)→ 種族名(マスタ)→ speciesKey
    public let speciesKey: String
}
public struct TeamPickerGroup: Identifiable, Equatable, Sendable {
    public let id: String          // Team.id
    public let name: String        // Team.name
    public let members: [TeamMemberOption]
}
public enum TeamIndividualOptions {
    public static func groups(from teams: [Team], species: [SpeciesSummary]) -> [TeamPickerGroup]
}
public enum TeamLoadLabels {
    public static let entryTitle = "構築から選ぶ"
    public static let empty = "まだ構築がありません"
}
```

- **判断: 表示名の組み立ては Core の純粋関数に置く**(ADR-0500 §1「View は描くだけ」。`DisplayLabels.swift` と同じ扱い)。
- **判断: メンバーが0体の構築は `teamOptions` に出さない**(選べるものが無いため)。構築そのものは消さない。
- 両 ViewModel に `public private(set) var teamOptions: [TeamPickerGroup]` と
  `public func loadTeams() async` を持たせる。`load()` の最後にも呼ぶ(計算の後。構築の読み込みで
  起動時の計算を遅らせない)。`load()` の `didLoad` ガードとは別で、`loadTeams()` は**何度でも呼べる**
  (構築ビルダーで編集して戻ってきたときに View から呼び直すため。`TeamListViewModel.load()` と同じ考え方)。
- **世代の保護(新しい非同期の読み込みなので必要)**: `loadTeams()` は `beginInput()` を使わず**専用の通し番号**で守る。
  `beginInput()` を使うと構築の読み込みが進行中の計算を「追い越した」ことになり、計算の応答が捨てられてしまう。
  構築の一覧は要求の内容に影響しないので別の番号にし、「古い `list()` の応答が新しいものを上書きしない」ことだけを守る。

### 7. UI(implementer の担当。accessibilityIdentifier 契約)

**判断: 既存の3択ピル行に4つ目を足さず、ピル行の直下に独立した1行(幅いっぱい)を置く**。理由:
(a) P6-2a の critic レビューで3等分でも「無振り」が見切れていた実績があり(実装メモ M3c)、4等分にすると再発する。
(b) 3つのプリセットは排他のセグメントだが「構築から選ぶ」は選択肢を開く `Menu` で、操作の種類が違う。
同じ行に混ぜると見た目が操作を誤って説明する。
(c) 呼び出し中は「<構築名> · <表示名>」を出したいので、4分の1幅には収まらない。
見た目の部品は新設せず、`MenuLabelChip` / `glassCard` / ピル(角丸999)など既存のものを使う(新しい視覚言語を作らない)。

| identifier | 要素 |
|---|---|
| `attackerTeamSourceButton` | 計算画面。プリセット行の直下、幅いっぱいの `Menu` のラベル。未選択は `TeamLoadLabels.entryTitle`、選択中は「<構築名> · <表示名>」 |
| `attackerTeamMember-<teamID>-<memberID>` | その `Menu` の中のメンバー1件(構築ごとにセクション見出し = 構築名) |
| `attackerTeamEmptyMessage` | 選べる個体が1体も無いときに出す `TeamLoadLabels.empty`(下記) |
| `reverseTeamSourceButton` | 逆算画面。`reverseAttackerPreset-*` / `reverseKnownDefenderPreset-*` の行の直下。側ごとに出し分けないので identifier は1つ(いまの側は `reverseSide-*` で分かる) |
| `reverseTeamMember-<teamID>-<memberID>` | 同上のメンバー1件 |
| `reverseTeamEmptyMessage` | 同上の空の案内 |

**空のとき(判断)**: `teamOptions` が空(構築が1つも無い / 構築はあるがメンバーが0体 / `teamStore` が nil /
`store.list()` が失敗)のときは、**入口の行を消さずに無効(`.disabled(true)`)にし、ラベルの下に
`TeamLoadLabels.empty` =「まだ構築がありません」を出す**。理由: 行ごと消すと「構築から呼び出せる」こと自体が
利用者に見えず、機能が発見されない。無効な行 + 案内文なら、構築ビルダーを作れば使えると伝えられる。

XCUITest(implementer が View と一緒に書く。`POKECALC_USE_MOCK=1`)で見ること:
- 構築が無い起動直後は `attackerTeamSourceButton` が存在し、`attackerTeamEmptyMessage` が見える。
- 構築画面で構築を1つ作ってメンバーを1体入れてから計算画面を開くと、`attackerTeamMember-<id>-<id>` が選べて、
  選んだあと `attackerPreset-aFull` の選択表示が外れる(`isSelected` トレイトが無くなる)。
- 逆算画面でも同じことが `reverseTeamSourceButton` でできる。数値は検査しない(モックは計算しない)。

### 8. 範囲外

- 逆算の**相手側**を構築から呼ぶこと(相手は逆算する対象なので調整は未知。requirements.md にも無い)。
- 呼び出した個体を計算画面で編集して構築へ書き戻すこと(一方向の「呼び出し」だけ)。
- ランク補正・状態異常(`Individual.ranks` / `status`)。`TeamMember` が持っていないので写しようがない。

### 9. 実装時に見つけたバグと回避(implementer 追記)

1. **`attackerPreset` / `knownDefenderPreset` を `Preset?` にしたことで、`.none` ケースの比較が
   壊れる(Swift の言語仕様)**。1章「判断」は「Swift の optional 昇格で `== .aFull` の比較がそのまま
   成り立つ」としていたが、これは `.aFull` のような衝突しないケース名の話で、`.none` には当てはまらない。
   `AttackerPreset` / `KnownDefenderPreset` はどちらも列挙子に `none`(無振り)を持つため、
   期待型が `Preset?` の文脈で素の `.none` と書くと、Swift は `Preset.none` ではなく
   `Optional<Preset>.none`(= nil)を優先して選ぶ(ビルド時に
   `warning: assuming you mean 'Optional<Preset>.none'; did you mean 'Preset.none' instead?` が出る、
   コンパイラの確定した既知の挙動で回避不能)。この結果、`XCTAssertEqual(viewModel.knownDefenderPreset, .none)`
   のような書き方は「無振りであること」ではなく「nil であること」を検査してしまい、無振りが選ばれている
   はずの場面で失敗する。
   - 影響箇所: spec-writer が P6-2d で書いた新規テスト
     `CalcViewModelTeamIndividualTests.testSelectingTeamAfterPresetClearsPreset`、
     `ReverseViewModelTeamIndividualTests.{testPresetClearsOnlyItsOwnSide,testEachSideKeepsItsOwnBuildSourceAcrossSideSwitch}`、
     および P6-2b から既にあった `ReverseViewModelTests.testLoadSelectsDefaultsWithoutCalculating`
     (この既存テストは P6-2d 以前は `knownDefenderPreset` が非 optional だったため問題が無く、
     今回の型変更で初めて壊れた)。
   - **対応(判断)**: 実装(ViewModel・`BuildSource` の型)は変えず、上記4箇所の `.none` を
     `AttackerPreset.none` / `KnownDefenderPreset.none` と明示する1行修正だけを行った。比較する値・
     期待する意味はいずれも変えていない(型だけの曖昧さの解消)。CLAUDE.md 絶対ルール6「テストを消したり
     弱めたりしない」には抵触しない(assert の対象・期待値は同一)と判断したが、「新規テスト・既存テストは
     無変更で通す」という P6-2d の前提には反するため、ここに明記して次の人間レビュー・critic の確認を
     求める。代案(`Preset?` をやめて `attackerPreset` を非 optional のまま残す)は、1章の中核判断
     (`.preset`/`.team` を型で排他にする)と両立しないため採らなかった。
2. **配列リテラルに `String?` と `String` を混在させると型推論が `[Any]` に落ちる(Swift の別の既知の
   制約)**。`CalcViewModelTeamIndividualTests.testLoadPopulatesTeamOptionsSkippingEmptyTeams` /
   `ReverseViewModelTeamIndividualTests.testLoadPopulatesTeamOptions` の
   `[StubTeams.namedMember.nickname, StubMaster.alpha.nameJa]`(`nickname: String?` と `nameJa: String` の
   混在)がこれに当たり、`swift build --build-tests` がコンパイルエラーで失敗した。両辺を `[String?]` に
   そろえる(`.map { $0.displayName as String? }` / `as [String?]`)形に直した。比較する値は変えていない。
3. **`Menu` の項目(`Button`)に付けた `.accessibilityIdentifier` が、実機・シミュレータで開いた
   UIKit のメニューへ渡らない(このコードベースで初めて判明。7章の identifier 契約を書いた時点では
   未確認だった)**。XCUITest の accessibility スナップショットで確認したところ、`Section(group.name) { ... }`
   で包んでも包まなくても、メニューを開いたときの `Button` 要素に `identifier` 属性が付かず
   `label` だけになる(`attackerTeamMember-<teamID>-<memberID>` で `elementBeginningWith` 検索しても
   見つからない)。振り返ると、このコードベースの既存の `Menu` 実装(種族・持ち物・技・特性・性格・
   テラスタイプの各セレクタ、`CalcScreenCards.swift` / `TeamEditMemberCard.swift` 等)は**すべて**
   メニュー項目に identifier を付けず、XCUITest 側はボタンのラベル文字列で探しており
   (`TeamScreenUITests` の `app.buttons[Self.firstMockSpeciesName]` 等)、同じ制約を既に踏まえた
   実装だったと分かる。
   - **対応(判断)**: `.accessibilityIdentifier(...)` の呼び出し自体は `TeamSourceMenuRow.swift` に
     残した(将来の iOS/SwiftUI バージョンで直る可能性があり、害もないため)。ただし実際に動く
     XCUITest はラベル文字列で辿る他ない。実装した XCUITest
     (`CalcScreenUITests.testTeamSourceRowEmptyThenSelectingMemberClearsPresetAndPresetClearsBack`、
     `ReverseScreenUITests` の同名テスト)は、構築ビルダーで作ったメンバーに一意なニックネームを付けて
     から、そのニックネームのラベルでメニュー項目を辿っている。7章の identifier 表はコードの意図として
     残すが、「メニューを開いたあとの項目を XCUITest から識別する」目的には現状使えないことをここに残す。
   - **判断(セクション見出し。critic 指摘で訂正)**: 当初、`Section(group.name) { ... }` で症状の切り分けが
     しづらかったという理由だけで入れ子の `Menu(group.name) { ... }`(構築名をタイトルにしたサブメニュー。
     選択のたびにタップが1回増える)に置き換えていた。critic が「`Section` でも identifier の制約は同じ
     はずで、置き換えの根拠が実測されていない」と指摘し、実際に `Section(group.name) { ... }` へ戻して
     `CalcScreenUITests.testTeamSourceRowEmptyThenSelectingMemberClearsPresetAndPresetClearsBack` と
     `ReverseScreenUITests` の同名テストを実行したところ、ラベル文字列(ニックネーム)でメンバーの
     `Button` を直接辿れ、6件とも成功した(`Section` は `Menu` を開いた時点で中身が一覧される。上記の
     identifier が渡らない制約は `Section`/入れ子 `Menu` のどちらでも変わらない)。よって `Section` に
     確定し、余分なタップを無くした(`TeamSourceMenuRow.swift`、両 XCUITest から `teamSubmenu` の
     タップを削除)。

### 10. critic の推奨対応(orchestrator が直接実施)

- **スナップショット不変性のテストを追加**: 上の1章「呼んだ個体はその個体の値をそのまま使う」は
  「呼び出した後に構築ビルダーで編集しても、呼び直すまで追従しない」までを含む判断だったが、
  それを固定するテストが無かった(critic 指摘の「重要1」)。
  `CalcViewModelTeamIndividualTests.testSelectedSnapshotDoesNotFollowLaterTeamEditsUntilReselected`
  を追加: 呼び出し→構築側で同じメンバーの SP を編集して保存→`loadTeams()`(一覧の再読み込みだけ)→
  画面操作(`selectAttackerItem`)で再計算させても要求の SP は編集前のまま→同じ teamID/memberID を
  再度呼ぶと編集後の SP に切り替わる、を確認する。実装したところ、このテスト自身にも §9-1 と同じ
  `.none` 系の Swift の制約(`await` が `XCTAssertEqual` の暗黙 autoclosure 引数の中にあると構文
  エラーになる。`ReverseViewModelTests` 等で既に使われているのと同じ制約)があったため、結果を
  `let request = try await lastRequest(stub)` で一度受けてから比較する形にした(比較する値は同じ)。
- **`Section` に確定**(9章末尾の訂正を参照)。
- `docs/plan.md` の P6-2 行を「critic PASS」に更新(旧「critic 未確認」)。

### 11. critic が挙げた任意の改善(今回は見送り。次の機会向けの記録)

次の点は critic が「軽微」として挙げたもので、ブロッカーではないため今回は対応していない: (4) 技が
learnset に無い場合のエラー分岐の未テスト、(5) 逆算 `.defender` 側の species 応答の世代保護テストが
無い、(6) スナップショットの speciesKey/itemId をマスタと突き合わせていない、(7)
`TeamSourceMenuRow.labelText` の組み立てが View 側にあり Core のテストで固定されていない、(8) 空状態の
行が見た目上は無効に見えない、(9) `StubPokeCalcService.swift` の `StubMaster.ability` 抽出(既存の
インラインリテラルを名前付き定数に置き換えただけ。値・挙動は不変)が ADR に未記載だった、(10)
`TeamSourceMenuRow.onSelect` の引数に外部ラベルが無い。

## issue #68 の受け入れ条件(検索上限200件の切り捨て。テスト先行・実装未着手)

- 日付: 2026-09-23 / 担当レーン: iOS / 関連: issue #68、issue #113(入力変更のデバウンス・キャンセル。別課題)、
  ADR-0304(Web レーンの同じ判断)、DECISIONS.md 2026-09-23「Web オンライン MasterSource — getSpecies.learnset の ID→実体化を提案」

### 0. 何が壊れているか

`CalcViewModel` / `ReverseViewModel` / `TeamEditViewModel` の `load()` はどれも
`searchSpecies(query: "", limit: 200)`(技・持ち物も同様)を1回だけ呼び、その結果を**マスタ全件**として扱っている。
`api/openapi.yaml` の `limit` は最大200・ページングパラメータ無しの契約(iOS からは変えられない)なので、
実データ(種族349・技515)では201件目以降が恒久的に選べない。learnset は全技 ID を返すのに、
先頭200件の `masterMoves` としか突合しないため、learnset の技が先頭200件に無い種族は `moveUnavailable` で計算不能になる。

### 1. 判断: 種族と技の選択を検索ベースにする(3画面とも同じ1つのパターン)

Web レーンの ADR-0304 と同じ方向にそろえる。持ち物(166件)・性格(25件)は上限内なので一覧のまま変えない
(`natures` には `q` パラメータ自体が契約に無い)。

**判断: `Menu` の中にインラインの検索欄は置かず、ヘッダー/チップのタップで検索シートを開く**(`.searchable()` を付けた
`List` を `.sheet` で出す)。理由:
(a) SwiftUI の `Menu` の中身はシステムのメニュー表示に渡されるため `TextField` が実質的に機能しない(フォーカス・
キーボードが入らない)。「検索できるメニュー」は iOS の標準部品として存在しない。
(b) 数百件を絞り込む操作は `.searchable()` を付けたリストが iOS の標準の形で、VoiceOver・キーボード・
「検索」キーの扱いを OS 任せにできる。
(c) 入口(ヘッダー = `SpeciesHeaderMenuLabel`、技チップ)は見た目を変えずに `Menu` → `Button` に替えるだけで済み、
XCUITest の既存 identifier(`attackerSpeciesPicker` 等)も名前を変えずに残せる。

持ち物・特性・性格・テラスタイプは `Menu` のまま(混在するが、件数と操作の種類が違うので同じにしない)。

### 2. 判断: `load()` の1回の取得は「全件」ではなく「先頭ページ」

`load()` は起動時の既定(攻撃側 = 最初・防御側 = 2番目・技 = learnset の最初のダメージ技)を決めるために
どうしても最初の1ページが要る。そこで **`load()` の `searchSpecies(query: "", limit: MasterSearch.pageLimit)` は残すが、
意味を「検索結果の先頭ページ」に変える**。`speciesOptions` / `moveOptions` は「マスタ全件」ではなく
「直近の検索結果(から作った選択肢)」という意味になる(プロパティ名は既存テストを壊さないため変えない)。

- `MasterSearch.pageLimit = 200`(契約の上限。`api/openapi.yaml` の `limit` の `maximum` と同じ値で、**全件の意味は持たない**)
- 件数が `pageLimit` に達したら `speciesSearchReachedLimit` / `moveSearchReachedLimit` を true にし、View は
  `MasterSearchLabels.truncated` を出す(黙って切り捨てない)。

### 3. 判断: 検索は1キーストロークごと + 素朴なデバウンス(#113 が後で置き換える)

明示的な検索ボタンは置かない(前方一致検索なので打ちながら絞れるのが自然)。
ViewModel は「同期の文字反映」と「非同期の検索」を分ける — 既存の `setObservationText(id:text:)` /
`recalculateAfterObservationEdit()` と同じ形(`TextField` の `Binding` から1フレーム遅れずに反映するため)。

- `@discardableResult func setSpeciesQuery(_ text: String) -> Bool` — 同期。前後空白を落として `speciesQuery` に入れ、
  検索を走らせる必要があれば true(前回と同じ語なら false)。
- `func runSpeciesSearch() async` — 世代トークンを進め、`searchDebounce` だけ待ってから
  `searchSpecies(query:limit:)` を呼び、**自分が最新の世代のときだけ**結果を反映する。
- View は `.searchable(text:)` の `Binding` の setter で `setSpeciesQuery` を呼び、
  `.task(id: viewModel.speciesQuery) { await viewModel.runSpeciesSearch() }` で走らせる
  (`.task(id:)` が前の検索タスクをキャンセルするので、デバウンスの `sleep` 中に打ち直せば要求自体が飛ばない)。
- `searchDebounce` は ViewModel の init 引数(既定 `MasterSearch.debounceInterval`)。テストは `.zero` を渡して
  待ち時間に依存しない。**判断**: 定数をコードに直書きせず注入可能にしたのは、テストを速く・決定的にするため。
- **issue #113 との関係**: これは「この修正のための素朴なデバウンス」であり、共有のデバウンス/キャンセル基盤ではない。
  #113 がその基盤を入れるときに、ここの `searchDebounce` + 世代トークンを置き換えてよい(#113 をこの修正のブロッカーにしない)。

### 4. 判断: 空クエリは「起動時の先頭ページ」を出す(再取得しない)

検索欄が空のときに「全件(上限200)を取り直す」のは、この issue で消したい振る舞いそのものなので取らない。
空にしたときは **`load()` で取った先頭ページをそのまま出し、API を呼ばない**(候補が0件のシートは
「何も選べない画面」に見えて使いづらいので、空リストにもしない)。View は同時に
`MasterSearchLabels.prompt` =「名前の先頭で検索」を出す。0件のときは `MasterSearchLabels.noMatch`。

### 5. 判断: 選択中の種族・技は検索結果とは独立に解決する

「いま選んでいる種族の名前」が検索語を変えた瞬間に `-` に化けてはいけない。ViewModel は
**一度でも見た `SpeciesSummary` / `Move` を key/id で蓄える辞書**を持ち、表示と選択の検証はそこから引く。

- `func speciesSummary(forKey key: String) -> SpeciesSummary?`(3画面共通の名前)。辞書には
  **検索結果だけでなく `species(key:)` の応答も入れる**(`SpeciesDetail` は `SpeciesSummary` を作るのに
  必要な値をすべて持つ)。構築に保存されたメンバーの種族が先頭ページの外でも名前を出せるようにするため。
- `var attackerSpecies / defenderSpecies`(Calc)、`var mySpecies / opponentSpecies`(Reverse) — 上の辞書から引く計算プロパティ。
  View は `speciesOptions.first(where:)` をやめてこれを使う。
- `selectAttacker(speciesKey:)` 等のガードは `speciesOptions.contains` ではなく **辞書に有るか**で判定する
  (＝検索で見つけた種族を選べる。知らない key を無視する既存の規則は変えない)。
- 技も同じ: `selectedMove` は `moveOptions.first(where:)` ではなく**技の辞書**から引く。技の検索語を
  learnset と重ならない語に変えると `moveOptions` は空になるが、それで選択中の技が消えて
  `selectedMoveMissing`(内部の不整合)になってはいけない。検索は**見えている候補を絞るだけ**で、
  選択中の技・`moveId`・エラー状態を変えない。
  一方、`selectMove(id:)` のガードは従来どおり `moveOptions`(＝いま見えている候補)で判定する
  (learnset に無い技を選べない既存の規則を弱めない)。

### 6. 判断: learnset の解決は「検索結果 ∩ learnset の ID 集合」。完全な解決は API 待ちで**部分的**

`getSpecies` の `learnset` は技 ID の配列で、技を ID で個別に解決する公開エンドポイントは無い
(DECISIONS.md 2026-09-23。データ/API レーンへ `learnset` を `Move` 実体にする提案が出ている)。
名前の前方一致でしか引けない以上、iOS だけでは learnset 全件を実体化できない。そこで:

- `moveOptions`(Calc/Reverse)・`moveOptionsByMember`(Team)は **「直近の技検索の結果 ∩ その種族の learnset の ID 集合」を
  learnset の順**で並べたものにする。技名で検索すれば、先頭200件の外にある技でも選べる(これが issue #68 の技側の解消)。
- learnset の ID 集合(`SpeciesDetail.learnset` そのもの)は ViewModel が種族ごとに保持し、
  **「その技を持ち続けてよいか」の判定は ID 集合で行う**(解決済みの `Move` の有無で判定しない)。
  とくに `TeamEditViewModel.applySpeciesChange` の `moveIds` の絞り込みは ID 集合で行う
  (現状は解決済み `moveOptions` で絞っており、先頭200件の外にある合法な技を黙って消す。同じ根本原因の別の症状)。
- **残る穴(部分的な修正であることの明記)**: 選択中の技 ID が一度も検索結果に現れていないとき
  (構築に保存された技・起動直後の learnset がすべて先頭200件の外、など)、技名を表示できない。
  Calc/Reverse ではその状態を既存の `moveUnavailable` として出し、利用者は技の検索シートで名前を打てば復帰できる。
  Team では技スロットに ID をそのまま出す(`BulkRowDisplay.itemLabel` の「マスタに無い ID は ID のまま」と同じ規則)。
  この穴は DECISIONS.md の提案(`getSpecies.learnset` を `Move` 実体にする)が入れば消える。**follow-up として残す**:
  issue #68 は iOS 側のこの修正だけでは閉じない。
  (2026-09-24 追記: PR #161 で `getMove` が入ったので、この穴は「issue #68 の残り: getMove による選択中の技の解決」で閉じる。)

### 7. 変えないもの

- 持ち物(`searchItems`)・性格(`natures()`)は一覧のまま(上限内。`natures` には `q` が無い)。
- 起動時の既定の選び方(規則3)・入力ごとに計算1回(規則4)・世代の保護(規則7)・エラーの分け方(規則8)。
- `api/openapi.yaml`・生成物(`Generated/`)は触らない(契約の変更は要らない。`q` は既にある)。

### 8. accessibilityIdentifier 契約(implementer の担当)

入口のボタンは既存の identifier を**そのまま**使う(`attackerSpeciesPicker` / `defenderSpeciesPicker` / `movePicker` /
`reverseMySpeciesPicker` 等の既存名・`memberSpeciesPicker-<memberID>` / `memberMoveSlot-<memberID>-<index>` /
`addMemberButton`)。シートは画面ごとに1つ(同時に2つ開かない)なので identifier も1つにする。

| identifier | 要素 |
|---|---|
| `speciesSearchSheet` | 種族検索シートのルート |
| `speciesSearchField` | その検索欄(`.searchable()`) |
| `speciesSearchResult-<種族の key>` | 結果の1行(例 `speciesSearchResult-0445-000`) |
| `speciesSearchHint` | 案内文言(`MasterSearchLabels.prompt` / `.truncated` / `.noMatch` のいずれか) |
| `speciesSearchLoadingIndicator` | 検索中 |
| `speciesSearchCancelButton` | 閉じる |
| `moveSearchSheet` / `moveSearchField` / `moveSearchResult-<技の id>` / `moveSearchHint` / `moveSearchLoadingIndicator` / `moveSearchCancelButton` | 技側。同じ規則 |

XCUITest(`POKECALC_USE_MOCK=1`)で見ること: `attackerSpeciesPicker` をタップ → `speciesSearchSheet` が出る →
`speciesSearchField` に文字を入れると `speciesSearchResult-*` が絞られる → 1件タップするとシートが閉じ、
`attackerSpeciesPicker` の `accessibilityLabel` がその種族名になる。技も `movePicker` で同じ。
既存の XCUITest は `Menu` の中のボタンを直接叩いているので、シート経由に**書き換えが要る**(implementer の担当)。

### 9. XCTest(spec-writer が先に書く。すべて `StubPokeCalcService`)

新規: `CalcViewModelSearchTests` / `ReverseViewModelSearchTests` / `TeamEditViewModelSearchTests`。
`Support/StubPokeCalcService.swift` に `searchSpecies` / `searchMoves` の**呼び出し記録**(`SearchCall(query:limit:)`)と
**保留モード**(`.manual` + `resolveSpeciesSearch(at:with:)`)を足し、`StubBulkMaster` に
**`pageLimit` 件 + その外に1件**の架空マスタを置く(アプリの架空データ `Resources/*.json` は増やさない。
実データの件数をモックに持ち込まないため)。固定すること:

1. `load()` の取得は先頭ページで、上限に達したら `speciesSearchReachedLimit == true`。
2. 検索語が `q` として渡り、結果が絞られる。
3. 先頭ページの外の種族を検索して選べる(＝ issue #68 の再現手順が解消する)。
4. 選択中の種族名が、検索結果を変えたあとでも解決できる。
5. 空クエリは先頭ページに戻り、**追加の API 呼び出しをしない**。
6. 知らない key/id を選ぼうとしたら無視する(既存の規則を弱めない)。
7. 検索応答の追い越し: 新しい検索の応答が先に届き、古い検索の応答が後から届いても上書きしない。
8. 連続した検索語の変更で、実際に飛ぶ要求は最後の1つだけ。
9. 先頭ページの外の技が、検索してから選べる(Calc/Reverse)・技スロットに入れられる(Team)。
10. 種族変更時の `moveIds` の絞り込みが learnset の ID 集合で行われる(解決できない合法な技を消さない)。

### 10. implementer が足す API(テストが固定している名前)

Core に新設する共通の語彙(新規ファイル1つ。3画面から使う):

```swift
public enum MasterSearch {
    /// api/openapi.yaml の searchSpecies/searchMoves/searchItems の limit の maximum。**全件の意味は持たない**
    public static let pageLimit = 200
    /// 1キーストロークごとの検索をまとめる待ち時間(issue #113 が共有の基盤に置き換えるまでの素朴な実装)
    public static let debounceInterval: Duration = .milliseconds(250)
}

public enum MasterSearchLabels {
    public static let prompt = "名前の先頭で検索"      // 空欄のとき
    public static let truncated = "候補が多いので、名前を入力して絞り込んでください"  // 上限に達したとき
    public static let noMatch = "一致する候補がありません"   // 0件
}
```

3画面に共通で足すメンバー(`CalcViewModel` / `ReverseViewModel` / `TeamEditViewModel`):

| メンバー | 意味 |
|---|---|
| `init(..., searchDebounce: Duration = MasterSearch.debounceInterval)` | デバウンスの待ち時間(テストは `.zero`) |
| `var speciesQuery: String` / `var moveQuery: String` | 検索欄の文字(`private(set)`) |
| `@discardableResult func setSpeciesQuery(_:) -> Bool` / `setMoveQuery(_:) -> Bool` | 同期の反映。検索が要るなら true |
| `func runSpeciesSearch() async` / `func runMoveSearch() async` | デバウンス → 検索 → 最新の世代だけ反映 |
| `var isSearchingSpecies: Bool` / `var isSearchingMoves: Bool` | 検索中(`private(set)`) |
| `var speciesSearchReachedLimit: Bool` / `var moveSearchReachedLimit: Bool` | 結果が `pageLimit` に達した |
| `func speciesSummary(forKey:) -> SpeciesSummary?` | 一度でも見た種族の辞書 |

画面ごと: `CalcViewModel.attackerSpecies` / `.defenderSpecies`、`ReverseViewModel.mySpecies` / `.opponentSpecies`
(どれも `SpeciesSummary?` の計算プロパティ)。`speciesOptions` / `moveOptions` / `moveOptionsByMember` は
名前を変えず意味だけ変える(2章・6章)。`selectedMove` は技の辞書から引く(5章)。

テスト側の下ごしらえ(spec-writer が追加済み): `StubPokeCalcService` の `SearchCall` / `SearchMode` /
`speciesSearchCalls` / `moveSearchCalls` / `setSpeciesSearchMode` / `setMoveSearchMode` /
`resolveSpeciesSearch(at:with:)` / `resolveMoveSearch(at:with:)` / `waitForSpeciesSearchCalls(count:)` /
`waitForMoveSearchCalls(count:)` / `matchedSpecies(query:limit:)` / `matchedMoves(query:limit:)` と、
`StubBulkMaster`(先頭ページ + `mixedSpecies` / `hiddenSpecies` / `hiddenMove`)。

### 11. implementer が追加で決めたこと(実装時。ADR に無かった判断)

- **`MasterSearchField`(新規・内部型)**: 検索欄(種族×3画面・技×3画面 = 6箇所)がすべて「世代トークン →
  デバウンス待ち → 検索 → 最新世代だけ反映」という同じ手順を踏むため、この手順を1つの汎用状態機械に
  切り出した(`MasterSpeciesSearchProviding`/`MasterMoveSearchProviding` プロトコル経由で View 側の
  共有シートからも使う)。「たまたま似ている」ではなく、6箇所が本当に同じ契約(世代保護・デバウンス・
  空クエリの扱い)を守る必要があるための共通化(coding-rules §2)。
- **`ReverseViewModel.selectedMove`**: 9章の依頼リストには明記が無かったが、Calc と同様に View が
  「いま選ばれている技」の名前・威力・分類を検索結果に依存せず出す必要があったため追加した
  (`CalcViewModel.selectedMove` と同じ理由・同じ形)。
- **`.searchable()` の accessibilityIdentifier 制約**: `.accessibilityIdentifier("speciesSearchField")`
  を `.searchable()` の `List` に付けても、実際にタップ・入力できる `UISearchBar` の `TextField` には
  渡らない(`Menu` の子に identifier が渡らないのと同種の UIKit 橋渡しの制約。P6-2d の ADR 9章で
  記録済みの制約と同類)。XCUITest は `app.searchFields.firstMatch`(システムの検索欄の型)で辿る。
  8章の identifier は「コードの意図」として残す。
- **構築編集の技スロット**: 1メンバーに技スロットが最大4つあるため、スロットごとに `.sheet` を付けると
  同じ `@State` を4つのシートが同時に監視することになる。`.sheet(item:)` + 選択中のスロットを表す
  `MoveSlotTarget`(private)を1つだけ持たせ、メンバーカードごとに1つのシートにした。
- **検索失敗時の扱い**: 検索(`searchSpecies`/`searchMoves`)が失敗しても画面の `error` は立てない
  (`isSearching` を false に戻すだけ)。検索は「絞り込みの補助」であり、失敗してもいまの選択・計算結果は
  壊れないようにする(絶対ルール5と同じ発想: 補助機能の失敗で主機能を止めない)。

### 12. orchestrator が見つけて直した XCUITest の不具合(実装バグではない)

critic レビュー前の自己検証で、`CalcScreenUITests.testAttackerSpeciesSearchSheetFiltersAndSelects` と
`ReverseScreenUITests.testOpponentSpeciesSearchSheetFiltersAndSelects` が失敗した(implementer が
利用枠の上限で中断し、リトライの結果を見られないまま引き継いだ状態)。

原因はテストの誤り(検索の絞り込み自体は正しく動いていた): `XCTAssertFalse(app.buttons[secondMockSpeciesName]
.exists)` のようにラベルの**文字列だけ**で「一致しない種族が消えていること」を確かめていたが、
検索シートの裏にある防御側(Calc)・自分側(Reverse)カードのヘッダーが、既定でちょうど2番目の種族
(`secondMockSpeciesName` と同じラベル)を表示しており、シートに隠れていても `exists` は true のままだった。
検索結果一覧に**限定**した identifier(`speciesSearchResult-<key>`)で確かめる形に直し、両方とも green に
なることを確認した(修正後の値・意味は変えていない。検査の対象を正確にしただけ)。

## issue #113 の受け入れ条件(iOS 側。入力変更時の古い計算要求の抑止・キャンセル。実装完了)

- 日付: 2026-09-23 / 担当レーン: iOS(issue #113 は Web・iOS・API の共同主担当) / 関連: issue #113、
  issue #68(「issue #68 の受け入れ条件」3章)、ADR-0304(Web レーンの同じ課題)、docs/plan.md P6-5
- 範囲: `ios/` 配下だけ。Web の `AbortSignal`・`CalcEngine` の cancel signal 境界は別レーンの担当で、
  iOS からは着手しない。**`api/openapi.yaml` は変更しない**(この課題はクライアント側の並行制御だけで、
  契約の変更を必要としない)。`engine/` も触らない。

### 0. 何が壊れているか

- `ios/PokeCalc/ReverseScreenObservations.swift` の観測欄は、有効な文字入力のたびに裸の
  `Task { await viewModel.recalculateAfterObservationEdit() }` を起動する。`4` → `45` と打つと
  **両方の値で `reverse` が飛ぶ**(中間値の計算・通信・calc-svc の CPU が無駄になる)。
- `ReverseViewModel` / `CalcViewModel` は `latestRequestToken` で「古い**応答**の反映」だけを防いでいる。
  進行中の Task を保持していないので、**送信済みの要求を cancel できない**。
- 画面を pop しても、View が作った裸の `Task` は生き残る(`.task` と違い構造化されていない)。
- `APIPokeCalcService.send` はキャンセルを `CancellationError` のまま投げ直す(実装済み)。しかし ViewModel の
  `catch` は何でも `CalcScreenError(error)` に写すため、**キャンセルが「応答の形が想定と違います」として
  画面に出る**(`.unexpectedResponse`)。

### 1. 受け入れ条件(iOS 側。検証可能な形)

1. **A1 最新の1つ**: `ReverseViewModel` / `CalcViewModel` は画面からの入力操作の Task を1つだけ保持し、
   次の入力操作を受けたとき先行 Task を cancel する。cancel は送信済みの `reverse` / `calcBulk` まで
   伝わる(テストの stub が要求単位でキャンセルを観測できる)。
2. **A2 debounce**: 逆算の観測欄の文字入力は trailing debounce(既定 200ms)で計算を予約する。
   待機中に次の入力が来たら、**先行分は `reverse` を呼ばずに終わる**。素早い `4` → `45` では
   `reverse` がちょうど1回、`observations == [.percent(45)]` で呼ばれる。
3. **A3 同期の即時性**: `setObservationText(id:text:)` による `observations[i].text` の反映と検証
   (`observation` / `error`)は debounce の影響を受けず、いままでどおり同期で終わる。
4. **A4 確定操作は待たない**: select・toggle・行削除・側の切り替え・構築からの呼び出しは debounce せず
   ただちに開始する。ただし A1 の「最新の1つ」には従う(先行 Task は cancel する)。
5. **A5 cancel はエラーではない**: `CancellationError`(`APIPokeCalcService` が URLSession のキャンセルを
   写したものを含む)を `error` に変換しない。`result` / `rows` も消さない。自分が最新の要求のときだけ
   `isLoading` を false に戻す。実通信失敗は従来どおり `.transport` として表示する。
6. **A6 画面破棄**: `cancelPendingWork()` で保持中の Task を cancel でき、以後その Task は画面の状態を
   書き換えない。View は `.onDisappear` で呼ぶ。
7. **A7 既存の契約を変えない**: 既存の `async` メソッド(`editObservation` / `selectMove` / `selectAttackerItem` 等)は
   「await したら計算まで終わっている」という意味のまま。`latestRequestToken` による古い応答の無視も残す
   (cancel が間に合わなかったときの最終防衛)。既存テストは1行も変えない。

### 2. 判断: ViewModel が「最新の1つ」の入力 Task を持つ(View の裸の `Task {}` をやめる)

issue #113 の既定案どおり、**Task の所有者を View から ViewModel へ移す**。View は
`Task { await viewModel.selectMove(id:) }` の代わりに `viewModel.scheduleLatest { await $0.selectMove(id: id) }` を呼ぶ。

- 理由(a): 裸の `Task` は View が消えても止まらない。ViewModel が持てば `cancelPendingWork()` 1つで止められる。
- 理由(b): 「最新の1つ」を ViewModel の不変条件にできる(View が何個 Task を作るかに依存しない)。
- 理由(c): 計算の入口(`recalculateIfPossible` / `recalculate`)は非公開のままで、公開 API の意味
  (A7)を変えずに済む。
- **却下した案**: `.task(id:)` で観測の送信値を監視する(検索欄と同じ形)。送る観測の列は
  `[DamageObservation]` の配列で `id:` に渡す安定した値を作りにくく、さらに select・toggle 側の Task は
  `.task(id:)` の管理外に残る。画面破棄の cancel が「一部だけ効く」状態になるので取らない。

### 3. 判断: debounce は逆算の観測欄だけ。計算画面は Task 管理だけ

計算画面(`CalcScreenView` / `CalcScreenCards` / `CalcScreenResults`)の入力はすべて選択・トグル
(種族・技・持ち物・プリセット・入れ替え・構築からの呼び出し)で、**1文字ごとに計算へ渡る自由入力が無い**
(種族・技の検索欄は計算ではなく `searchSpecies`/`searchMoves` を呼ぶ別の経路で、issue #68 の
`MasterSearchField` が担当する)。したがって `CalcViewModel` には debounce を入れず、A1・A5・A6 の
Task 管理だけを入れる。`TeamEditViewModel` は計算を呼ばない(`calcBulk`/`reverse` を使わない)ので対象外。

### 4. 判断: debounce は 200ms・注入可能にする

issue #113 が Web と共有する契約値をそのまま使う。`MasterSearch.debounceInterval`(検索の 250ms)とは
別の定数にする — 用途(検索の絞り込み vs 計算)も値も違い、片方を変えたときにもう片方が黙って
変わってはいけないため。

```swift
public enum CalcInput {
    /// 文字入力を計算へ渡すまでの trailing debounce(issue #113 の契約値)。
    public static let debounceInterval: Duration = .milliseconds(200)
}
```

`ReverseViewModel.init(..., calcDebounce: Duration = CalcInput.debounceInterval)` で注入可能にし、
テストは `.zero`(待たない)または `.seconds(30)`(「debounce されていたら終わらない」ことの確認)を渡す。
壁時計の固定待ちをテストに書かない(issue #68 の `searchDebounce` と同じ手法)。

### 5. 判断: `CancellationError` の扱い

ViewModel の各 `catch` は、キャンセルとそれ以外を分ける:

- `error` は触らない(`CalcScreenError` に写さない)。`result` / `rows` も消さない
  (画面を離れる・入力を打ち直すだけで、いま出ている結果が「エラー」に化けてはいけない)。
- `token == latestRequestToken`(＝自分がまだ最新)のときだけ `isLoading = false` に戻す。
  新しい入力に追い越されているときは、新しい Task が `isLoading` を持っているので触らない。
- 判定は `error is CancellationError`。`APIPokeCalcService` が `URLError(.cancelled)` や
  `ClientError` を `CancellationError` へ正規化済みなので、ViewModel は URL 系の型を知らなくてよい。
- **「各」は `reverse`/`calcBulk` を包む catch だけでなく、`species(key:)`(learnset の読み直し)を
  包む catch も含む(`CalcViewModel.load`/`selectTeamIndividual`/`applyAttackerChangeAndRecalculate`、
  `ReverseViewModel.load`/`selectTeamIndividual`/`reloadAttackingMovesAndRecalculate`)。`scheduleLatest`
  導入後は種族変更・入れ替え・構築呼び出しの操作もすべて cancel されうる(8章)ため、これらの経路の
  1つでも見落とすと A5 が破れる(critic 指摘。実装は各 ViewModel 内の private `handleInputFailure(_:)`
  に集約して見落としを防ぐ)。

### 6. 判断: 画面破棄は `.onDisappear` → `cancelPendingWork()`

`ReverseScreenView` / `CalcScreenView` は `NavigationStack` に push される View で、`@State` の ViewModel は
pop で破棄される。`.task { await viewModel.load() }` は構造化されているので pop で自動 cancel されるが、
ViewModel が持つ入力 Task は自動では止まらない。`deinit` は使わない(`@MainActor` 隔離のプロパティを
`deinit` から触れない)。したがって **View が `.onDisappear { viewModel.cancelPendingWork() }` を呼ぶ**。
検索シート(`.sheet`)の表示は presenter の `onDisappear` を起こさないので、シートを開いただけで
計算が止まることはない。

### 7. 判断: issue #68 の `MasterSearchField` は今回は統合しない

「issue #68」3章は、素朴なデバウンス + 世代トークンを #113 の基盤で置き換えてよいと書いている。
今回は**置き換えない**。理由:

- 検索欄は View 側の `.task(id: viewModel.speciesQuery)` が前の検索 Task を cancel し、画面破棄でも
  自動で止まる(構造化済み)。#113 が要求する2つの性質(先行 cancel・画面破棄で停止)を検索欄は
  すでに満たしており、置き換えても振る舞いは変わらない。
- 意味が違う: 検索は 250ms・空クエリは即時に先頭ページへ戻す・失敗を `error` にしない。計算は 200ms・
  失敗を `error` にする。1つの部品にまとめると分岐だらけの「似て非なるもの」になる(coding-rules §3)。
- issue #68 のテスト3ファイルが緑で、振る舞いを変えない統合のためにそれらを書き換えるのは
  絶対ルール6(テストを弱めない)の精神に反する。

共有するのは**語彙と規則**だけにする: trailing debounce・待ち時間は init 引数で注入・世代トークンは
最終防衛として残す。3つ目の利用者が出たら共通化を検討する(そのときに `MasterSearchField` と
`CalcInput` を1つの部品へ寄せる)。

### 8. implementer が足す API(テストが固定している名前)

`PokeCalcCore`(新規ファイル1つ + 2つの ViewModel への追加):

| メンバー | 画面 | 意味 |
|---|---|---|
| `enum CalcInput { static let debounceInterval: Duration }` | 共通 | 200ms(3章) |
| `init(..., calcDebounce: Duration = CalcInput.debounceInterval)` | Reverse | debounce の注入(テストは `.zero`) |
| `@discardableResult func scheduleRecalculationAfterObservationEdit() -> Task<Void, Never>` | Reverse | 観測の文字入力。先行 Task を cancel し、debounce 後に最新の1つだけ計算する |
| `@discardableResult func scheduleLatest(_ operation: @escaping @MainActor @Sendable (Self) async -> Void) -> Task<Void, Never>` | Reverse / Calc | 確定操作。先行 Task を cancel してただちに開始する |
| `func cancelPendingWork()` | Reverse / Calc | 保持中の Task を cancel する(`.onDisappear`) |

- `Task<Void, Never>` を返すのは、テストが `await task.value` / `task.isCancelled` で決定的に待てるようにするため
  (View は `@discardableResult` で捨てる)。
- `scheduleLatest` が ViewModel 自身を引数で渡すのは、View 側で `[weak viewModel]` を書かせないため
  (Task は ViewModel が保持するので、クロージャが強参照を持つと循環が生まれうる)。
- 内部の共通部品として「最新の1つの Task を保持する」小さな型(`MasterSearchField` と同じ `internal` の
  状態機械。例: `LatestTaskRunner`)を作ってよい。`schedule(debounce:operation:)` / `cancel()` を持ち、
  開始前に `Task.isCancelled` を確認してから `operation` を呼ぶ(待機中に捨てられた分は**要求を出さない**)。

View 側(`ios/PokeCalc`)で置き換える呼び出し(裸の `Task {` をやめる):
`ReverseScreenObservations.swift`(観測の文字入力 → `scheduleRecalculationAfterObservationEdit()`、
行削除 → `scheduleLatest`)、`ReverseScreenView.swift`・`ReverseScreenCards.swift`・`CalcScreenView.swift`・
`CalcScreenCards.swift`・`CalcScreenResults.swift`(すべて `scheduleLatest`)、`TeamSourceMenuRow.swift`
(`onSelect` を同期クロージャにし、呼び出し側の画面で `scheduleLatest` に包む)。
`TeamEditView` / `TeamListView` は対象外(計算を呼ばない)。
両画面に `.onDisappear { viewModel.cancelPendingWork() }` を足す。

### 9. XCTest(spec-writer が追加済み。実装前は red)

新規2ファイル + stub の下ごしらえ。すべて `StubPokeCalcService`(壁時計の固定待ちをしない)。

- `ReverseViewModelCancellationTests.swift`
  1. `testObservationDebounceIntervalIsTwoHundredMilliseconds` — 契約値(A2)。
  2. `testRapidObservationEditsCalculateOnlyTheFinalValueOnce` — `4` → `45` で `reverse` は1回・値は 45、
     先行 Task は `isCancelled`(A1・A2)。
  3. `testSynchronousTextUpdateIsNotDelayedByTheDebounce` — debounce 待機中でも `text`/`observation` は
     反映済み、要求は0件。その間に `cancelPendingWork()` すると要求を出さずに終わる(A3・A6)。
     壁時計に依存しないよう、待ち時間は実際の 200ms ではなく「十分長い値」を注入する。
  4. `testNewObservationEditCancelsTheInFlightReverseRequest` — 送信済みの古い要求が cancel され、
     新しい要求だけが結果になる(A1・A5)。
  5. `testCancelPendingWorkCancelsTheInFlightRequestWithoutShowingError` — cancel で `error` が立たず、
     `isLoading` が解け、**前の結果が残る**(A5・A6)。
  6. `testConfirmedSelectionIsNotDebounced` — `calcDebounce: .seconds(30)` でも確定操作はすぐ要求を出す(A4)。
  7. `testScheduleLatestCancelsThePreviousInFlightRequest` — 確定操作どうしでも「最新の1つ」(A1・A4)。
- `CalcViewModelCancellationTests.swift`
  1. `testScheduleLatestCancelsThePreviousInFlightCalc` — `calcBulk` の先行要求が cancel される(A1)。
  2. `testCancelPendingWorkDoesNotTurnCancellationIntoAScreenError` — cancel で `error` が立たず、
     `rows` が残る(A5・A6)。
- `Support/StubPokeCalcService.swift` に追加(既存の振る舞いは変えない): `.manual` の `reverse` / `calcBulk` を
  `withTaskCancellationHandler` で包み、cancel されたら `CancellationError` で continuation を終える。
  `cancelledReverseRequests` / `cancelledBulkRequests`(要求番号の集合)、
  `waitForReverseCancellation(at:)` / `waitForBulkCancellation(at:)`、`cancelPendingReverse(at:)` /
  `cancelPendingBulk(at:)` を公開する。cancel されなかった要求の挙動は従来どおりなので、既存テストに影響しない。

### 10. 範囲外・申し送り

- Web(`web/`)・`services/`・`engine/`・`api/openapi.yaml` は触らない。
- XCUITest の追加は今回の範囲に含めない(debounce はミリ秒単位の振る舞いで、UI テストで安定して
  観測できない)。`accessibilityIdentifier` の契約も変えない。
- 既存テスト(`ReverseViewModelTests` / `CalcViewModelTests` / 各 `…SearchTests`)は**変更しない**。
  A7 を守れば通り続ける。もし実装の途中でこれらが落ちたら、それは A7 を破った合図なので、
  テストを直さず実装を直すこと。
- 演出・reduced motion・アクセシビリティ通知の挙動は変えない(issue #113 の受け入れ条件)。

## issue #110 の受け入れ条件(iOS 側。calc の候補・観測の件数上限。実装完了)

- 関連: issue #110(API レーン主担当)、ADR-0208、PR #130(`api/openapi.yaml` に上限を追加)、PR #142(Web の対応)、
  docs/ai-shared/DECISIONS.md 2026-09-23「calc の候補・観測件数に上限を置く」、docs/plan.md P6-6
- 契約は**変えない**(`api/openapi.yaml` は API レーンのもの。iOS は追従するだけ)。

### 0. 何が壊れているか

`api/openapi.yaml` は `ReverseRequest.observations` に `maxItems: 16`、`ReverseRequest.itemCandidates` と
`BulkCalcRequest.itemVariants` に `maxItems: 64` + `uniqueItems: true` を持つ。超えると API は 400 `invalid_input`。
iOS は上限を一切見ていないので、次の2つが黙って壊れる:

1. `ReverseViewModel.addObservation()` は無制限に行を足せる。17行目以降に有効な値を入れた瞬間、逆算が
   400 になって画面がエラーに落ちる(利用者から見ると「観測を足したら計算が止まった」)。
2. 持ち物候補(`toggleOpponentItemCandidate`)と、計算画面の比較トグル(`toggleDefenderItemComparison`)は
   持ち物マスタ全件(実データで 166 件)から選べる。64 を超えた時点で同じく 400 になる。

engine(WASM)・calc-svc・Web は対応済みで、iOS だけが残っている。上限に当たることが分かるのは要求を
組み立てる画面側だけなので、ここで止めて**理由を見せる**(黙って壊れない・黙って切り捨てない)。

### 1. 受け入れ条件(検証可能な形)

- **A1** `PokeCalcCore` の `RequestLimits` が `maxObservations = 16` / `maxItemCandidates = 64` /
  `maxItemVariants = 64` を持ち、値は `api/openapi.yaml` の同名フィールドの `maxItems` と一致する。
  ViewModel・View のどこにも 16 / 64 / 63 を直書きしない(coding-rules §2)。
- **A2** 観測の行は最大 `RequestLimits.maxObservations` 行。行数がそれに達すると
  `ReverseViewModel.observationsReachedLimit == true` になり、`addObservation()` は**何もしない**
  (行数も各行の id も変わらず、`reverse` も呼ばれない)。
- **A3** 上限に達した状態から行を1つ削ると `observationsReachedLimit` は false に戻り、また追加できる。
- **A4** `ReverseRequest.observations` の件数は常に `maxObservations` 以下(16行すべてに有効値を入れても超えない)。
- **A5** 相手の持ち物候補は、送る配列の先頭に入る「持ち物なし」(null)を**1件と数えて**
  `maxItemCandidates` を超えない。したがってトグルで選べる ID の数は `maxSelectableItemCandidates`
  (= `maxItemCandidates - 1` = 63)。`ReverseRequest.itemCandidates` は常に 64 件以下・重複なし。
- **A6** 選択が `maxSelectableItemCandidates` に達しているとき、`toggleOpponentItemCandidate(itemId:)` の
  **ON 操作は拒否**される(`opponentItemCandidateIds` が変わらず、`reverse` も呼ばれない)。
  **OFF(選択解除)は常に通る**。解除すると `opponentItemCandidatesReachedLimit` が false に戻り、
  従来どおり再計算が1回走る。
- **A7** 上限に達していないときの振る舞いは何も変わらない(マスタ順・null 先頭・トグル1回で計算1回・
  観測の追加/削除/検証・エラーの出し分け)。既存テストは1つも変更せず緑のままであること。
- **A8** 計算画面の `comparedDefenderItemIds` → `BulkCalcRequest.itemVariants` も A5・A6 と同じ規則
  (`maxItemVariants` / `comparedDefenderItemsReachedLimit` / ON 拒否 / OFF 常時可 / 要求は 64 件以下)。

### 2. 判断: 定数の置き場所は `RequestLimits`(`ReverseLimits` ではない)

新規ファイル `PokeCalcKit/Sources/PokeCalcCore/RequestLimits.swift` に `public enum RequestLimits` を1つ置く。
依頼時の仮称は `ReverseLimits` だったが、**逆算専用ではない**ことが調査で分かったので名前を変えた:
`BulkCalcRequest.itemVariants` の 64 件は計算画面(`CalcViewModel`)の比較トグルが踏みうる(6章)。
Web の `web/src/domain/requestLimits.ts` と同じ発想・同じ粒度にして、レーン間で語彙をそろえる。
既存の `ObservationLimits`(`ObservationInput.swift`)とは別物なので統合しない: あちらは観測**値**の
範囲(1〜100% など)、こちらは配列の**件数**の上限で、正とする契約の場所も違う。

`api/openapi.yaml` を実行時に読めないのは Web と同じなので、この定数は契約の**写し**である。
ただし `PokeCalcCoreTests` は `make ios-test-unit` でシミュレータ上でも走り、そこからリポジトリのファイルを
読めることを前提にできない(サンドボックス)ので、YAML との突き合わせは XCTest ではなくホスト側の
`ios/scripts/check-request-limits.sh`(`make ios-check-request-limits`。`make ios-test` の一部)で行う。
担保は (1) このスクリプトが `api/openapi.yaml` の3つの `maxItems` と `RequestLimits` の値を比べ、ずれ・欠落で
失敗する(定数・契約を書き換える変異4通りで失敗することを確認済み)、(2) XCTest は 16 / 64 を**リテラルで**
固定する、(3) 本 ADR と定数のコメントに「正は `api/openapi.yaml`」と書く、の3点。
(当初は Web の `requestLimits.test.ts` に任せる案だったが、あれは Web の定数しか見ないので iOS の写しのずれは
検出できない。引き継ぎ時の検証で気づき、iOS 側にも検査を置いた。)

### 3. 判断: null 枠を勘定に入れる(選べるのは 63 件)

`ReverseViewModel.buildRequest` は候補が1つでもあれば `[nil] + opponentItemCandidateIds` を送る
(`CalcViewModel` の `itemVariants` も同じ)。`uniqueItems` は null どうしの重複も禁じているので、
null は**1件として数える**。そこで

```swift
public static let maxSelectableItemCandidates = maxItemCandidates - 1  // 63
public static let maxSelectableItemVariants = maxItemVariants - 1      // 63
```

を派生させ、トグル側はこちらを見る。64 件選ばせてから送信時に1件落とす、という実装は A6 の
「選んだのに反映されない状態を作らない」に反するので採らない。

### 4. 判断: 観測は「追加ボタンを無効化 + 理由表示」(Web と同じ)

観測行の追加は明示的な1操作で、上限に当たるのは `addObservation()` の1か所だけなので、Web の
`canAddObservation` と同じ「その操作をさせない」方式にする。ViewModel は
`observationsReachedLimit: Bool`(issue #68 の `speciesSearchReachedLimit` / `moveSearchReachedLimit` と
同じ `…ReachedLimit` の語彙)を1つだけ公開し、View が `.disabled(viewModel.observationsReachedLimit)` と
理由文言の表示に使う。`canAddObservation` のような否定形の別名は作らない(同じ事実に2つ名前を付けない)。

`addObservation()` 自体にも `guard` を置く(View を直さなくても不正な状態を作れないようにする。
XCTest は ViewModel だけを叩くので、ここに guard が無いと A2 を確かめられない)。

行数(≤16)を抑えれば、実際に送る有効な観測(`currentSendKey`)は必ずそれ以下になるので、
送信直前の再チェックは要らない(A4 はこの不変条件の確認)。

### 5. 判断: 持ち物候補は (b)「上限で ON 操作を拒否」。Web の決定的な切り捨てとは**あえて変える**

依頼の選択肢 (a)(Web と同じ、送信直前にマスタ順で先頭 64 件へ絞り込み + 絞り込んだことを表示)ではなく
**(b)** を採る。理由:

- Web の `reverseItemCandidates()` が切り捨て方式なのは、Web の候補選択がチェックボックス群の
  「選択状態」から毎回導出されるためで、iOS のトグルとは操作の粒度が違う。iOS は1タップ = 1トグルなので、
  「タップして選択状態になったのに、送るときには落ちている」という状態を作らずに済む。
- (b) なら `opponentItemCandidateIds`(画面が選択中と見せている集合)と要求の `itemCandidates` が
  常に一致する。(a) だと2つがずれ、`ReverseResultDisplay` が候補の持ち物名を引くときの前提も
  「表示と要求は同じ集合」から崩れる。ずれる状態を作らないほうが、後から読む人にとって安全。
- 上限に達したことは `opponentItemCandidatesReachedLimit` で画面に出す(黙って拒否しない)。
  DECISIONS.md 2026-09-23 の要件「黙って切り捨てず、明示的なエラーか決定的な絞り込み」は、
  (b)(= そもそも超えさせない + 理由表示)でも満たす。

Web と iOS で**画面の振る舞いが違う**ことは承知のうえの判断で、ここに記録しておく。どちらも
「64 件を超えた要求を送らない」「利用者が理由に気づける」という issue #110 の要求は満たす。

63 件も選ぶのは現実的な操作ではない(実データの持ち物は 166 件で、逆算の候補として意味があるのは
数件〜十数件)。上限は「壊れないための柵」であって、通常の操作では踏まない。

### 6. 判断: `itemVariants` も iOS の対象(使っている)

依頼時点では「iOS が `itemVariants` を使っているか未確認」だったが、使っている:
`CalcViewModel.buildRequest`(現 486〜491 行)が `comparedDefenderItemIds` から
`[nil] + ids` を組み立てて `BulkCalcRequest.itemVariants` に入れている。入口は
`toggleDefenderItemComparison(itemId:)` で、こちらも持ち物マスタ全件からトグルできる。
したがって逆算と**同じ**規則を計算画面にも入れる(A8)。「iOS は使っていないので対応不要」とは書けない。

### 7. 今回対応しないもの(理由つき)

- **`presets`(8 件 + unique)**: `CalcViewModel.buildRequest` は常に `presets: []`(既定セット)を送り、
  画面にプリセットを複数選ぶ入口が無い。iOS から上限を踏む経路が存在しないので何もしない。
- **`maxCandidates`(0〜128)**: `ReverseViewModel.buildRequest` は定数 `0`(全件)しか送らず、
  画面から変えられない。範囲外の値を作れないので何もしない。
- **`Generated/`・`api/openapi.yaml`**: 触らない。生成型(`[Swift.String?]?`)は件数を表現できないので、
  上限は手書きの `RequestLimits` で持つしかない(`make gen` は不要)。
- **`observations` の重複**: 契約は `uniqueItems` を付けていない(同じ観測の繰り返しは矛盾しない)ので、
  重複の抑止はしない。

### 8. implementer が足す API(テストが固定している名前)

新規 `PokeCalcKit/Sources/PokeCalcCore/RequestLimits.swift`:

```swift
public enum RequestLimits {
    public static let maxObservations = 16          // ReverseRequest.observations.maxItems
    public static let maxItemCandidates = 64        // ReverseRequest.itemCandidates.maxItems
    public static let maxItemVariants = 64          // BulkCalcRequest.itemVariants.maxItems
    /// 送る配列の先頭に入る null(持ち物なし)の1件を除いた、トグルで選べる ID の数(3章)。
    public static let maxSelectableItemCandidates = maxItemCandidates - 1
    public static let maxSelectableItemVariants = maxItemVariants - 1
}

/// 上限に達したことを画面に出す文言(`MasterSearchLabels` と同じ理由でコードに1か所持つ)。
public enum RequestLimitLabels {
    public static let observationsReachedLimit: String
    public static let itemCandidatesReachedLimit: String
    public static let itemVariantsReachedLimit: String
}
```

文言は件数を `RequestLimits` から埋め込む(数字を文字列に直書きしない)。XCTest は「空でないこと」と
「その件数が文言に含まれること」だけを固定し、言い回しは実装者が `docs/design.md` の調子に合わせてよい
(既定案: `"観測は最大16件までです"` / `"持ち物の候補は「持ち物なし」を含めて最大64件までです"` /
`"比較する持ち物は「持ち物なし」を含めて最大64件までです"`)。

ViewModel に足すメンバー:

| メンバー | 画面 | 意味 |
|---|---|---|
| `var observationsReachedLimit: Bool` | Reverse | 観測行が `maxObservations` に達した(`private(set)` でなく計算プロパティでよい) |
| `func addObservation()`(既存)に guard | Reverse | 上限に達していたら何もしない(A2) |
| `var opponentItemCandidatesReachedLimit: Bool` | Reverse | 選択が `maxSelectableItemCandidates` に達した |
| `func toggleOpponentItemCandidate(itemId:)`(既存)に guard | Reverse | 上限到達中の ON を拒否。**早期 return は `beginInput()` より前**に置き、要求も世代も動かさない(A6) |
| `var comparedDefenderItemsReachedLimit: Bool` | Calc | 選択が `maxSelectableItemVariants` に達した |
| `func toggleDefenderItemComparison(itemId:)`(既存)に guard | Calc | 同上。現在の実装は先頭で `beginInput()` を呼ぶので、**guard をその前に移す** |

### 9. accessibilityIdentifier と文言(View の担当)

| identifier | 要素 |
|---|---|
| `reverseAddObservationButton`(既存) | 上限に達したら `.disabled(true)` にする |
| `reverseObservationLimitHint` | 観測の上限の理由文言(上限に達したときだけ出す) |
| `reverseItemCandidateLimitHint` | 相手の持ち物候補の上限の理由文言(同上) |
| `calcItemVariantLimitHint` | 計算画面の比較トグルの上限の理由文言(同上) |
| `reverseOpponentItemToggle-<id>` / `defenderItemToggle-<id>`(既存) | 未選択のチップは上限到達中 `.disabled(true)`。**選択済みのチップは常に有効**(解除できる) |

文言は `danger` ではなく `textSecondary` で出す(エラーではなく案内。`MasterSearchLabels` の案内文言と
同じ扱い。`ErrorBannerView` / `calcErrorMessage` は使わない)。`ChipButton` に `isEnabled: Bool = true` を
足して `.disabled(!isEnabled)` と見た目の減光を足す(既存の呼び出しは既定値でそのまま動く)。

XCUITest の追加は任意(範囲外)。既存の XCUITest は上限に達しない操作しかしないので、影響しない。

### 10. XCTest(spec-writer が追加。実装完了。critic 指摘で1本ずつ追補)

新規3ファイル。すべて `StubPokeCalcService`(数値やモックの JSON に依存しない)。

- `RequestLimitsTests.swift` — A1。契約値をリテラルで固定し、派生値(63)と文言の件数埋め込みを確かめる。
- `ReverseViewModelLimitTests.swift` — A2〜A7。
  1. `testObservationRowsCanGrowUpToTheLimit`
  2. `testAddObservationBeyondTheLimitIsIgnored`
  3. `testRemovingAnObservationBelowTheLimitAllowsAddingAgain`
  4. `testRequestNeverExceedsTheObservationLimit`
  5. `testSelectingUpToTheItemCandidateLimitKeepsTheRequestWithinTheContract`
  6. `testTogglingOnBeyondTheItemCandidateLimitIsRejectedWithoutRequesting`
  7. `testDeselectingIsAlwaysAllowedEvenAtTheLimit`
  8. `testTogglingBelowTheLimitStillWorksAsBefore`(回帰)
  9. `testRejectedToggleDoesNotAdvanceGenerationOfAnInFlightReverse`(critic 指摘の回帰。8章「早期 return は
     `beginInput()` より前」を固定する。`reverse` を `.manual` にして進行中にした状態で ON 拒否を挟み、
     その進行中の応答がちゃんと反映される〈`isLoading` が解ける〉ことを確かめる。guard を
     `beginInput()` の後ろへ動かすミューテーションで実際に red になることを確認済み)
- `CalcViewModelLimitTests.swift` — A8。5〜8 と同じ4本を `itemVariants` 側で、9 と同じ趣旨の
  `testRejectedToggleDoesNotAdvanceGenerationOfAnInFlightCalc` を追加。

テスト用の架物: `StubMaster.makeService(items:)` に `RequestLimits.maxItemCandidates + 6` 件の架空の持ち物を
渡す(`StubPokeCalcService` / `StubMaster` は**変更しない**。既存の下ごしらえで足りる)。

### 11. 範囲外・申し送り

- `api/openapi.yaml`・`engine/`・`services/`・`web/` は触らない。`make gen` も不要(7章)。
- 既存テスト(`ReverseViewModelTests` / `CalcViewModelTests` / 各 `…SearchTests` / `…CancellationTests`)は
  **変更しない**。A7 を守れば通り続ける。落ちたらテストではなく実装を直すこと。
- 上限に達したときに「どれを外せばよいか」を提案する、といった手助けは今回やらない(まず壊れないこと)。

## issue #68 の残り: getMove による選択中の技の解決(実装完了)

- 日付: 2026-09-24 / 担当レーン: iOS / 関連: issue #68(「issue #68 の受け入れ条件」6章「残る穴」)、
  issue #113(「issue #113 の受け入れ条件(iOS 側)」5章)、PR #161(`GET /api/pokedex/moves/{key}` = `getMove` の追加)、
  ADR-0304(Web レーンの同じ課題)、ADR-0105 §3 追記(`getMove` の 404 の意味)
- 状態: implementer 実装完了 → critic 1回目 FAIL(11章)→ 指摘を反映して再実装 → `swift test` 383件・
  `make ios-test`(unit 396件・XCUITest 17件・Info.plist 検査)すべて成功。**issue #68 は iOS 側を閉じてよい**
  (7章。残る制約は「厳密な learnset 全件の最初」ではないことと持ち物 200 件超のときの follow-up のみで、
  どちらも「選べない・計算できない」ではない)。
- 範囲: `ios/` 配下だけ。**`api/openapi.yaml`・生成物(`Generated/`)は変更しない**(`getMove` は PR #161 で契約・
  生成済みの Swift クライアントにある)。`engine/`・`services/`・`web/` も触らない。

### 0. 何が残っていたか

「issue #68 の受け入れ条件」6章は、技を ID で個別に引く公開エンドポイントが無いことを理由に、次の穴を
**部分的な修正**として残した: 選択中の技 ID が一度も検索結果に現れていないと名前を解決できない。

- 計算・逆算: 攻撃側の learnset がすべて先頭ページ(`MasterSearch.pageLimit` 件)の外だと、起動直後・種族変更直後に
  `moveUnavailable`(利用者は技の検索シートで名前を打てば復帰できる)。
- 計算・逆算: 構築から呼び出した個体の技が先頭ページの外だと、`reselectMove` がそれを候補に見つけられず、
  **黙って既定の技に置き換える**(6章に書かれていなかった同じ根本原因の症状。今回見つけた)。
- 構築: 技スロットは `moveOptionsByMember`(直近の技検索 ∩ learnset)から名前を引くので、先頭ページの外の保存済みの技は
  ID のまま出る。さらに**検索語を変えると先頭ページの技まで ID に化ける**(`TeamEditViewModel` は技の辞書を持たない)。

PR #161 で `getMove`(1回に1つの ID)が入ったので、この穴を閉じる。

### 1. 受け入れ条件(検証可能な形)

1. **A1 サービス**: `PokeCalcService` に `func move(id: String) async throws -> Move` がある。`APIPokeCalcService` は
   `getMove` を呼び(`X-Device-Id`/`X-Session-Id` 付き・パス `/api/pokedex/moves/{id}`)、404 は `species(key:)` の 404 と
   同じく body の `code`(`not_found`)をそのまま運ぶ `PokeCalcError` にする。503・default・通信失敗・キャンセルも既存の
   操作と同じ写像。`MockPokeCalcService` はフィクスチャの技から引き、無ければ `PokeCalcError.Code.notFound`
   (他の操作と同じ `notFoundError`)。
2. **A2 解決する**: 計算・逆算で「選ぶべき技」が技の辞書に無いとき、`move(id:)` で解決して辞書に入れ、その技を選ぶ。
   起動時の再現手順(検索は先頭200件の技だけ・攻撃側の learnset は201件目の技だけ)で `moveUnavailable` にならず、
   `selectedMove` が名前を持ち、計算(`calcBulk`)/逆算(`reverse`)の `moveId` がその技になる。構築から呼び出した
   個体の技(learnset にあり・辞書に無い)は、既定の技に置き換えずにその技を選ぶ。構築では、保存済みメンバーの
   `moveIds` のうち辞書に無いものを `load()` で解決し、`move(forID:)` が名前を返す。
3. **A3 必要なときだけ**: 選ぶ技が既知の技(先頭ページ・検索結果・過去の解決)で決まるときは `move(id:)` を**呼ばない**。
   learnset 全件は解決しない。1回の入力操作で呼ぶのは `MasterSearch.maxMoveLookupsPerSelection`(4)回まで。
4. **A4 失敗は今日の振る舞いに戻る**: `move(id:)` の失敗(404・通信失敗など)は、それ自体を画面のエラーにしない。
   計算・逆算は今日と同じ結果になる(既定の技が見つからなければ `moveUnavailable`、構築の個体の技が解決できなければ
   既定の技)。構築は今日と同じく ID のまま出し(`move(forID:)` が nil)、`error` を立てず、`moveIds` も変えない。
5. **A5 世代・キャンセル**: 解決は入力操作の一部として、その操作の `beginInput()` の世代で守る。応答を待っている間に
   次の入力が来たら、遅れて届いた応答は `moveId`・`attackerBuildSource`・`error`・`isLoading`・計算要求を変えない。
   `scheduleLatest` の Task が cancel されたら `move(id:)` にも伝わり、`CancellationError` は画面のエラーにしない
   (`rows`/`result` を残し、`isLoading` だけ解く。issue #113 A5)。
6. **A6 持ち物の一覧**: 持ち物は一覧(`Menu`)のまま。`searchItems(query: "", limit: MasterSearch.pageLimit)` の結果が
   `pageLimit` に達したら、3画面とも `itemOptionsReachedLimit == true` にし、View は `MasterSearchLabels.itemsTruncated` を
   出す(黙って切り捨てない)。
7. **A7 既存の契約を変えない**: 既存テストは1行も変えない。`moveOptions` / `moveOptionsByMember` の意味(直近の技検索 ∩
   learnset)、`selectMove(id:)` のガード(`moveOptions` にある技だけ)、空クエリで API を呼ばない規則は変えない。

### 2. 判断: `move(id:)` の写像は `species(key:)` にそろえる

`getMove` の 404 は契約上 `#/components/schemas/Error` を直接持つ(生成型では `.notFound(response)`)。`species(key:)` の
`.notFound` と同じく `domainErrorFromSchema` で写し、`code` はサーバーの語彙(`not_found`)のまま運ぶ。
`PokeCalcError.Code.notFound` は `"not_found"` なので、ViewModel は API/モックを問わず同じ値で分岐できる。
モックの「無い」も同じ `notFoundError("技", id)`。**404 を空の値や nil に写さない**(「無い」と「壊れた」を ViewModel が
区別できるように、どちらも throw のまま渡し、区別は ViewModel の側でする。4章)。

### 3. 判断: いつ解決するか(計算・逆算)

解決は `reselectMove` の直前、learnset を読み直した直後(`reloadAttackerMoveOptions` / `reloadMoveOptions` の後)に、
同じ入力操作の中で行う。経路は learnset を読み直すすべての操作: 計算 = `load`・`selectAttacker`・`swapSides`・
`selectTeamIndividual`、逆算 = `load`・`selectSide`・攻撃側の種族の変更・`selectTeamIndividual`(与えたダメージ)。
learnset を読み直さない操作(`selectMove`・持ち物・プリセット・観測・防御側の種族)は解決しない。

手順(判断: `reselectMove` を2段にする):

1. **優先する技**(`reselectMove(preferringCurrent:)` に渡す ID。構築の個体の技・種族変更前の技): それが新しい learnset に
   あり、辞書に無ければ `move(id:)` で1回だけ解決する。解決できた(または既に辞書にある)なら、**`moveOptions` に無くても**
   それを選ぶ。判定は「learnset の ID 集合 + 辞書」で行う(「issue #68」6章「その技を持ち続けてよいかの判定は ID 集合で行う」
   を計算・逆算にも当てる。検索は見えている候補を絞るだけで選択を変えない、という5章の規則とも同じ向き)。
2. **既定の技**: 今日どおり `moveOptions` から選ぶ(計算 = learnset の順で最初のダメージ技、無ければ最初。逆算 = 最初の
   ダメージ技)。**`moveOptions` から1つも選べないときだけ**、learnset を先頭から順に見て、辞書にある技はそのまま使い、
   無い ID は `move(id:)` で1つずつ解決し、条件に合う技(計算 = 最初のダメージ技、逆算 = ダメージ技)が見つかったら止める。
   `move(id:)` の呼び出しが `MasterSearch.maxMoveLookupsPerSelection` 回に達したら止める。計算は見つからなければ
   「解決できた最初の技」(規則3の「無ければ learnset の最初」)を選び、それも無ければ `moveUnavailable`。逆算は見つからなければ
   `moveUnavailable`(変化技は逆算できないのでフォールバックしない)。

解決した `Move` は辞書に入れるだけで `moveOptions` には足さない(`moveOptions` = 検索シートに出す候補の意味を変えない。
A7)。そのため解決した技は `selectMove(id:)` では選び直せないが、既に選ばれているので困らない。

- **理由(1段目)**: 構築の個体を呼び出したときに、個体の技が黙って別の技に変わるのが今回いちばん実害の大きい症状。
  1回の `getMove` で直る。
- **理由(2段目を「選べないときだけ」にする)**: 既知の技で既定が決まる大多数のケースで通信を増やさない(A3)。
  既定の技の規則(learnset の順で最初のダメージ技)を「既知の技の中で」満たすだけで、全件を解決して厳密な「最初」を
  探すことはしない(それは learnset 全件の解決になる。5章)。
- **却下した案**: learnset の先頭1件だけを解決する。計算では変化技を選んでしまい、逆算では選べる技が無くなりやすい。
- **却下した案**: 解決した技を `moveOptions` に混ぜる。検索語と無関係な技がシートに出て、`moveOptions` の意味が
  「検索結果 ∩ learnset」から崩れる(既存テストが固定している意味。A7)。

### 4. 判断: 1操作あたりの上限は 4(`MasterSearch.maxMoveLookupsPerSelection = TeamLimits.maxMovesPerMember`)

learnset を先頭から解決していく2段目は、変化技が並ぶ learnset だと往復が増える。歯止めとして1回の入力操作で呼ぶ
`move(id:)` を上限で打ち切る。値は構築の1体の技スロット数と同じ 4 にし、定数として `TeamLimits.maxMovesPerMember` を
参照する(どの画面でも「1操作 = 1体分の技」を超える往復をしない、という1つの規則にそろえる。直書きしない)。
上限で打ち切ったときは今日と同じ振る舞い(計算 = 解決できた最初の技、逆算 = `moveUnavailable`)になり、利用者は
技の検索シートで名前を打てば復帰できる(「issue #68」6章と同じ逃げ道)。

### 5. 判断: 構築は `load()` で保存済みの技だけを解決し、技の辞書を持つ

- `TeamEditViewModel` に計算・逆算と同じ**技の辞書**(一度でも見た `Move`: 先頭ページ・技検索の結果・`move(id:)` の応答)を
  持たせ、`public func move(forID id: String) -> Move?` で引く。View(`TeamEditMemberCard.moveSlot`)は `moveOptions` から
  名前を引くのをやめてこれを使い、nil のときだけ ID を出す(「issue #68」6章の「ID のまま」の規則は nil のときに残る)。
- 解決するのは `load()` の中で、各メンバーの `species(key:)` の後、`moveIds` のうち辞書に無いものだけ。1体あたり最大4件・
  6体で最大24件(それでも learnset 全件〈1体 20〜100件〉よりずっと少ない)。並行に投げてよい(`withTaskGroup` 等)。
  `load()` は解決が終わってから `isLoading = false` にして返る(テストが `await load()` の後で確かめられるように)。
- `addMember`・`setMemberSpecies`・`addMove` では解決しない: 追加した技は検索結果から選んだものなので辞書にある。
  種族変更で残る技は `load()` で解決済み。
- 失敗(404・通信失敗・キャンセル)は `error` を立てない・`moveIds` を変えない・その ID を nil のままにする(A4)。
  構築の `load()` は `species(key:)` の失敗で `error` を立てる既存の規則を持つが、技の解決の失敗はそれとは別で、
  メンバーの読み込み(learnset・特性の選択肢)を止めない。
- 世代: 構築の技の辞書は「ID → その技」の不変な対応なので、遅れて届いた応答を辞書に入れても古い状態で新しい状態を
  上書きすることにはならない。計算・逆算のような世代の保護は要らない(`moveIds` や選択は解決で変えないため)。

### 6. 判断: 持ち物は一覧のまま。上限に達したら旗と案内を出す(Web の「中断」とは変える)

持ち物は実データで166件(ADR-0304 の件数表)で、`limit=200` の1回の取得で全件が入る。issue #68 は持ち物の名前も挙げているが、
いま切り捨ては起きていない。持ち物だけを検索ベースにすると `Menu` から検索シートへの UI 変更が要り、今は利益が無い。
そこで**一覧のまま**にし、将来 `pageLimit` に達したときに黙って切り捨てないことだけを保証する。

- Web(`web/src/master/onlineSource.ts`)は持ち物の応答が `ITEMS_FETCH_LIMIT` ちょうどなら読み込みを**中断**する
  (「打ち切りの疑い」)。iOS は**中断しない**: 持ち物が選べないだけで計算・逆算の画面全体を止めるのは、補助の
  失敗で主機能を止めない方針(「issue #68」11章「検索失敗時の扱い」・絶対ルール5と同じ発想)に反するため。
  「黙って切り捨てない」という目的は Web と同じ。
- 3画面に `public private(set) var itemOptionsReachedLimit: Bool`(`load()` で `items.count >= MasterSearch.pageLimit`)。
  既存の `speciesSearchReachedLimit` / `moveSearchReachedLimit` と同じ語彙。
- View は true のとき持ち物の `Menu` の中(または直下)に `MasterSearchLabels.itemsTruncated`
  (「持ち物が多すぎて、一覧に出ていない持ち物があります」)を出す。`MasterSearchLabels.truncated`
  (「名前を入力して絞り込んでください」)は持ち物に検索欄が無いので使わない(別の文言)。
- 本当に上限を超えたら、そのときに持ち物も検索ベースにする(follow-up。今は起きていないので作らない)。

### 7. issue #68 を閉じてよいか

**閉じてよい**(iOS 側)。種族・技は検索で先頭ページの外も選べ(PR #136)、選択中の技は `getMove` で名前を解決でき(本章)、
持ち物・性格は上限内で、上限に達したら黙って切り捨てない(6章)。残るのは次の既知の制約で、どれも「選べない・計算できない」
ではない:

- 既定の技の選び方は「既知の技 + 上限4件の解決」の中での「最初のダメージ技」で、learnset 全体での厳密な最初とは限らない
  (3章・4章)。learnset 全件を実体化する API(DECISIONS.md 2026-09-23 の `getSpecies.learnset` を `Move` にする提案)が
  入れば厳密にできる。
- 持ち物が将来 200 件を超えたら検索ベースへの移行が要る(6章)。

Web 側の同じ issue の扱いは ADR-0304 が持つ(このレーンでは触らない)。

### 8. implementer が足した API(実装済み)

spec-writer が**シグネチャだけ**先に足し(ビルドを通し、テストが振る舞いで red になるように)、implementer が中身を埋めた:

| メンバー | 実装 |
|---|---|
| `PokeCalcService.move(id:)`(プロトコル要件) | そのまま |
| `APIPokeCalcService.move(id:)` | `client.getMove` で実装(2章)。404 は `species(key:)` と同じく `domainErrorFromSchema` |
| `MockPokeCalcService.move(id:)` | フィクスチャから引く・無ければ `notFoundError("技", id)` |
| `MasterSearch.maxMoveLookupsPerSelection`(= `TeamLimits.maxMovesPerMember`) | そのまま(4章) |
| `MasterSearchLabels.itemsTruncated` | そのまま(6章) |
| `CalcViewModel` / `ReverseViewModel` / `TeamEditViewModel` の `itemOptionsReachedLimit` | `private(set) var`。`load()` で `items.count >= MasterSearch.pageLimit` を反映(6章) |
| `TeamEditViewModel.move(forID:)` | 技の辞書(`moveDictionary`)から引く(5章) |

ViewModel の内部(3章・5章): 計算・逆算の `reselectMove` を2段の `async throws` にし、`move(id:)` の呼び出しを足した
(`token` の確認を各 `await` の後に入れる。失敗は `CancellationError` だけ呼び出し元の `handleInputFailure` に投げ直し、
それ以外は「解決できなかった」として `resolveMove(id:)` が `nil` を返す形で飲み込む)。構築の技の辞書に先頭ページと
`runMoveSearch()` の結果を合流させ、`load()` で保存済みメンバーの未知の技を `withTaskGroup` で並行解決する。

**critic 指摘と修正(11章に詳細)**: 逆算の `recalculateIfPossible` の「古いエラーを消す」条件は、最初の実装では
`selectedMove != nil` にしたが、これだけでは不十分だった(`reselectMove` が `moveUnavailable` を投げても `moveId` は
書き換わらず、辞書には直前の種族の技がまだ残っているため、無関係な入力のたびに `moveUnavailable` を誤って消して
しまう回帰があった)。`selectedMove` が非 nil であることに加えて、**いまの攻撃側の learnset(ID 集合)にあり**、
**ダメージ技である**ことまで確かめる形に直した(`moveOptions.contains` に戻すと、ID 解決した技は `moveOptions` に
入らないため今度は正しく消せないケースが生まれる。11章)。

View(`ios/PokeCalc`。XCUITest は今回必須にしない): `TeamEditMemberCard.moveSlot` の名前を `viewModel.move(forID:)` から
引く。持ち物の `Menu`(計算・逆算・構築)に `itemOptionsReachedLimit` のときの `MasterSearchLabels.itemsTruncated`
(textSecondary の caption。他の検索ヒントと同じ見た目)。

### 9. XCTest(実装完了。`swift test` 383件・`make ios-test` unit 396件・XCUITest 17件すべて green)

テスト用の下ごしらえ(既存の振る舞いは変えない):

- `Support/StubPokeCalcService.swift` に `move(id:)` と `MoveLookupMode`(`.notFound` 既定 / `.immediate` / `.manual`)、
  `moveLookups`(呼ばれた ID の記録)、`setMoveLookupError(_:)`、`resolveMoveLookup(at:with:)`、
  `cancelPendingMoveLookup(at:)` / `cancelledMoveLookups` / `waitForMoveLookups(count:)` / `waitForMoveLookupCancellation(at:)`。
  **既定を `.notFound` にした判断**: `move(id:)` を足す前に書かれた既存テスト(`CalcViewModelSearchTests` /
  `ReverseViewModelSearchTests` の `testMoveOutsideTheFirstPageBecomesSelectableAfterSearching` は「先頭ページの外の技しか
  覚えない種族にすると `moveUnavailable`、名前で検索すれば復帰」を固定している)を1行も変えずに保つため。その振る舞いは
  「`getMove` が 404 のときのフォールバック」(A4)として今も正しい。新しいテストは `.immediate` / `.manual` を明示する。
- `Support/StubMoveLookupMaster.swift`(新規): 先頭ページの外の技しか覚えない種族を**種族一覧の先頭**に置いたサービス
  (issue の再現手順を `load()` でそのまま起こす)、先頭ページの外の変化技(上限+1個)・ダメージ技、`pageLimit` 件の持ち物。

新規テスト:

- `MoveLookupServiceTests.swift`(A1・7件): `getMove` のパス・ヘッダー・写像・404/503/500 のコード・通信失敗、モックの一致と未知 ID。
- `CalcViewModelMoveLookupTests.swift`(16件): 起動時の再現手順(A2)、最初のダメージ技の規則(A2)、上限で打ち切り(A3)、
  既知の技では呼ばない(A3。learnset にマスタに無い ID を含む `StubMaster.alpha` でも呼ばない)、404・通信失敗で
  `moveUnavailable`(A4)、構築の個体の技を保つ・404 で既定の技(A2・A4)、古い応答で上書きしない・キャンセルは
  エラーにしない(A5)、stage-2(既定の技の learnset 解決ループ)の世代保護(A5。critic 指摘。11章)、
  持ち物の上限(A6)、文言・定数。
- `ReverseViewModelMoveLookupTests.swift`(13件): 計算と同じ趣旨 + 変化技を飛ばす・上限で打ち切ったら `moveUnavailable`、
  解決した技で観測を入れると `reverse` が通り `error` が残らない、stage-2 の世代保護、`recalculateIfPossible` の
  古いエラーの消し方の回帰・その理由(critic 指摘。11章)。
- `TeamEditViewModelMoveLookupTests.swift`(7件): 保存済みの技を `load()` で解決(未知のものだけ)、既知だけなら呼ばない、
  404・通信失敗で nil・`error` なし・`moveIds` 不変、検索語を変えても既知の技の名前が消えない、持ち物の上限。

### 10. 範囲外・申し送り

- `api/openapi.yaml`・`Generated/`・`engine/`・`services/`・`web/` は触らない(`make gen` 不要)。
- learnset 全件の実体化(Web が `getMove` を不十分と判断した用途)はしない(3章)。
- 解決中のローディング表示は増やさない(既存の `isLoading` がその操作の間ずっと立っている)。
- 既存テストは**変更しない**。実装の途中で既存テストが落ちたら A7 を破った合図なので、テストではなく実装を直す。

### 11. critic 指摘と修正(1回目 FAIL → 反映して再実装)

1回目の実装は `swift test`/`make ios-test` を通していたが、critic レビューで以下が見つかった。

- **MUST(回帰)**: `ReverseViewModel.recalculateIfPossible` の「観測が無いときに古いエラーを消してよいか」の
  判定を、最初は `if selectedMove != nil { error = nil }` にしていた。これは A7 が固定する「ID 解決した技は
  `moveOptions` に入らない」を汲んだつもりの直しだったが、**古いエラーが残っていなければならないケースまで
  消してしまう回帰**があった: `reselectMove` が `moveUnavailable` を投げても `moveId`・技の辞書はそのまま
  (書き換えるのは成功したときだけ)なので、直前の種族のときに解決済みだった技が辞書に残っている。
  この技はもう「いまの攻撃側の learnset」には無いのに、`selectedMove != nil` は真になるため、技とは無関係な
  入力(例: 持ち物の変更)のたびに `moveUnavailable` が誤って消えてしまう
  (`testStaleResolvedMoveDoesNotClearMoveUnavailableAfterASpeciesChange` が固定。旧条件で実際に red になることを
  確認済み)。
  - **修正**: `if let move = selectedMove, attackingLearnsetIds.contains(move.id), move.category != .status { error = nil }`
    に直した(`selectedMove` が非 nil であることに加え、**いまの攻撃側の learnset の ID 集合にあり**、
    **ダメージ技である**ことまで確かめる。`reselectMove` が `moveId` を書き換えるのは、常にこの3条件を満たす
    技のときだけなので、これは「`reselectMove` が実際に選び直せたか」の判定そのものになる)。
  - `moveOptions.contains` に戻すテストも用意した(`testResolvedMoveStillClearsAStaleReverseFailureWithNoObservations`)。
    ID 解決した技(`moveOptions` には入らない)については、無関係な失敗(例: 前回の `reverse` の通信失敗)の残りを
    いつまでも消せなくなり、実際に red になることを確認済み。
- **OPTIONAL(対応済み)**: `reselectMove` の stage-2(`moveOptions` から既定が決まらないときの learnset 解決ループ)
  にも、各 `move(id:)` の `await` の後に `token == latestRequestToken` の確認が要る(stage-1 の世代保護は
  `testStaleLookupResponseDoesNotOverwriteTheNewerSelection` が固定しているが、stage-2 は別の guard なので、
  それだけでは守れない)。Calc・Reverse それぞれに
  `testStaleStageTwoLookupResponseDoesNotOverwriteTheNewerSelection` を追加し、guard を外すと実際に red になる
  (起動時の stage-2 解決が保留中に別の種族へ切り替えても、遅れて届いた解決結果が新しい選択を上書きしてしまう)
  ことを確認済み。
- 4本とも追加し、`swift test` 383件・`make ios-test`(unit 396件・XCUITest 17件・Info.plist 検査)がすべて
  green であることを確認した。既存テストは1行も変えていない。

## getMovesByIds による構築編集の技の一括解決

- 日付: 2026-09-24 / 担当レーン: iOS / 関連: issue #68(「issue #68 の残り: getMove による選択中の技の解決」
  5章「構築は `load()` で保存済みの技だけを解決し、技の辞書を持つ」)、issue #113(「issue #113 の受け入れ条件
  (iOS 側)」5章「Task の所有者」)、PR #199(`PokeCalcService.move(id:)` = `getMove` の追加。main 統合済み)、
  main の `GET /api/pokedex/moves/batch`(operationId `getMovesByIds`。api/openapi.yaml 171〜214行。
  生成済み Swift クライアントに既にある。web の同じ課題は ADR-0304 §3)
- 状態: **実装完了**(spec-writer: 受け入れ条件とテスト → implementer: 実装 → 3.1 の2つ目の衝突を
  spec-writer が解決 → critic 1回目 FAIL(テストの網羅不足のみ。実装は問題なしと判定)→ 指摘のテストを追加
  → 再確認。8章「critic 指摘への対応」)。
- 範囲: `ios/` と本ファイル(`docs/adr/0501-ios-screen-acceptance.md`)・`docs/plan.md` だけ。
  `api/openapi.yaml`・`Generated/`・`engine/`・`services/`・`web/` は触らない(`getMovesByIds` は既に main の
  契約・生成物にある。`make gen` 不要)。

### 0. 背景

「issue #68 の残り」5章で `TeamEditViewModel.load()` は、保存済みの各メンバーについて `species(key:)` を呼んだ
直後に `resolveUnknownMoves(member.moveIds)` を呼び、辞書に無い `moveIds` を `move(id:)`(1件ずつ)の
`withTaskGroup` で並行解決している(メンバーごとに1回のまとめ役の呼び出し、その中身は ID の数だけ個別の
HTTP 往復)。1体あたり最大4件・6体で最大24件になり得る(`TeamLimits.maxMovesPerMember` × `TeamLimits.maxMembers`)。

main に `getMovesByIds`(1回の呼び出しで複数の ID をまとめて解決できる。1〜64件、マスタに無い ID は黙って
省く、404 は無い)が入ったので、`TeamEditViewModel.load()` の技解決を「全メンバー分の未知の技を集めて
1回(64件以下なら)の `moves(ids:)` にまとめる」形に置き換えられる。往復回数を減らせるだけで、A2〜A4
(失敗の扱い・世代・`moveIds` を変えない)という既存の振る舞いは変えない。

### 1. 受け入れ条件(検証可能な形)

1. **A1 サービスに `moves(ids:)` を足す**: `PokeCalcService` に
   `func moves(ids: [String]) async throws -> [Move]` を足す(`move(id:)` の複数版)。
   - `APIPokeCalcService.moves(ids:)`: `client.getMovesByIds` に写す(`X-Device-Id`/`X-Session-Id` 付き・
     パス `/api/pokedex/moves/batch`・クエリ `ids` を繰り返しで渡す)。`ids` は**重複除去**してから送る
     (同じ ID を2回渡しても、送るクエリは1回にまとめる)。空配列なら**通信せず** `[]` を返す。
     `ids`(重複除去後)が `RequestLimits.maxMoveBatchIds`(= 64。契約の `getMovesByIds.ids.maxItems` の写し)を
     超えるときは、呼び出し側(`APIPokeCalcService` 自身)がこの件数ずつに分割して複数回呼び、応答を渡した順に
     連結する(呼び出し元の `TeamEditViewModel` は分割を意識しない。契約の `description` がこの分割を
     呼び出し側の責務と明記している)。`.serviceUnavailable`/`.default`/通信失敗/キャンセルは他の pokedex 操作と
     同じ写像(`domainErrorFromSchema`/`domainError`/`PokeCalcError.Code.transport`/`CancellationError` の
     投げ直し)。**404 は無い**(`getMovesByIds` の `Output` に `.notFound` ケースが無い。マスタに無い ID は
     200 の応答から黙って省かれるだけ)。
   - `MockPokeCalcService.moves(ids:)`: フィクスチャの技から `ids`(重複除去後)の順に引き、無ければ省く
     (`throw` しない。`move(id:)` の 404 と違い「まとめ取りは部分一致・エラーにしない」という契約の
     `description` どおり)。空配列なら `[]`。
   - `RequestLimits.maxMoveBatchIds = 64`(`getMovesByIds.ids.maxItems` の写し)を `RequestLimits` に足す。
     この値は `components.schemas` のプロパティではなく `paths./api/pokedex/moves/batch.get` の
     **クエリパラメータ**の `maxItems` なので、`ios/scripts/check-request-limits.sh` の既存の
     `contract_max_items`(`components.schemas.<schema>.properties.<property>.maxItems` だけを awk で辿る)は
     使えない。別関数 `contract_query_max_items`(`paths.<path>` → `- name: <name>` → `schema.maxItems` を
     固定インデントで辿る)を足し、`check_query /api/pokedex/moves/batch ids maxMoveBatchIds` で照合する
     (`make ios-test` の一部。実装済み。2章)。
2. **A2 構築(`TeamEditViewModel.load()`)は全メンバー分をまとめて1回で解決する**: `load()` は、各メンバーの
   `species(key:)`(learnset の読み直し)がすべて終わった**後**に、全メンバーの `moveIds` のうち技の辞書
   (`moveDictionary`)にまだ無いものを**集めて重複除去し**、`moves(ids:)` を1回(64件以下なら)呼ぶ。
   解決できた技は辞書に入れる(`moveOptionsByMember` には混ぜない。既存の意味を変えない)。
   - 未知の ID が無ければ `moves(ids:)` を呼ばない(空配列で早期リターンする a1 の規則と合わせて、
     通信そのものが発生しない)。
   - 1体あたり最大4件・6体で最大24件(`TeamLimits.maxMovesPerMember` × `TeamLimits.maxMembers`)なので、
     構築画面からは `RequestLimits.maxMoveBatchIds`(64)を超える呼び出しにはならない(A1 の分割は
     `APIPokeCalcService` 内部の保険であって、構築側が意識して分割呼び出しを設計する必要は無い)。
   - **1段目(全メンバーの `species(key:)`)の途中で失敗したら、2段目(`moves(ids:)` の一括呼び出し)は
     一切行わない**(`load()` の `do`/`catch` は1つで、1段目のどこかで `throw` すると2段目に進まず
     `catch` に落ちる)。旧実装(メンバーごとのループ内で `resolveUnknownMoves` を呼んでいた)では、
     N番目のメンバーで `species(key:)` が失敗しても 1〜(N-1)番目のメンバーの技はすでに解決済みのまま
     残っていた。この差は許容する: `species(key:)` の失敗は `error` を立てて編集画面自体を止める
     (旧実装でも新実装でも)ので、技が一部だけ解決されているかどうかはユーザーから見て意味を持たない
     (どのみち `error` 表示になり、再読み込みで `load()` をやり直すことになる)。

3. **A3 失敗は今日の振る舞いのまま**: `moves(ids:)` の失敗(503・通信失敗・キャンセルを含む)は、`error` を
   立てない・`team.members[*].moveIds` を変えない・解決できなかった ID は `move(forID:)` が `nil` を返す
   ("issue #68 の残り"5章の A4 をそのまま踏襲。一括呼び出しに変わっても、部分成功/全部失敗の外部からの
   見え方は「解決できた ID だけ辞書に入る」で変わらない)。
4. **A4 世代は要らない**: "issue #68 の残り"5章の判断(「構築の技の辞書は ID → その技の不変な対応なので、
   遅れて届いた応答を辞書に入れても古い状態で新しい状態を上書きすることにはならない」)は `moves(ids:)` に
   切り替えても変わらない(1回の応答が複数 ID をまとめて運ぶだけで、性質は同じ)。世代保護は追加しない。
5. **A5 既存テストは1行も変えない**: `TeamEditViewModelMoveLookupTests`("issue #68 の残り"5章で追加済み・
   green)は変更しない。`TeamEditViewModel.load()` を `moves(ids:)` の一括呼び出しへ切り替えても、それらの
   テストが green のまま保てるよう `StubPokeCalcService` 側だけを拡張する(3章「既存テストとの整合」)。
6. **A6 計算・逆算(Calc/Reverse)はこのタスクでは変えない**: `CalcViewModel`/`ReverseViewModel` の
   `reselectMove` は `move(id:)` を使い続ける(4章「判断」)。

### 2. 実装したもの(すべて `ios/` 配下)

`api/openapi.yaml`・`Generated/` は main に既にあるので変更していない。

1. `PokeCalcCore/RequestLimits.swift`: `RequestLimits.maxMoveBatchIds = 64`。
2. `ios/scripts/check-request-limits.sh`: `contract_query_max_items`(クエリパラメータ版の `maxItems` 抽出)・
   `check_query`(照合)を足し、`check_query /api/pokedex/moves/batch ids maxMoveBatchIds` を呼ぶ。
   `bash ios/scripts/check-request-limits.sh` で確認済み(`ios-check-request-limits: OK`)。
3. `PokeCalcCoreTests/RequestLimitsTests.swift`: `testMoveBatchLimitMatchesTheOpenAPIContract`(新規テスト。
   既存の `testLimitsMatchTheOpenAPIContract` 等は変更していない)。
4. `Support/StubPokeCalcService.swift`: `moves(ids:)` のテスト用実装(3章・3.1章)。
5. `PokeCalcCore/PokeCalcService.swift`: プロトコル要件 `func moves(ids: [String]) async throws -> [Move]`
   (ドキュメントコメントに A1 の規則を書いた)。
6. `PokeCalcCore/APIPokeCalcService.swift`: `moves(ids:)` を実装(重複除去 → 空配列は無通信 →
   `RequestLimits.maxMoveBatchIds` 件ずつ `stride(from: 0, to:, by:)` で分割 → `client.getMovesByIds` を
   順に呼び応答を連結。写像は `move(id:)` 等と同じ private static 関数を再利用)。
   `PokeCalcCore/MockPokeCalcService.swift`: `moves(ids:)` を実装(重複除去した順で `fixtures.moves` から
   引き、無い ID は省く)。
7. `PokeCalcCore/TeamEditViewModel.swift`: `load()` を2段に分割(1段目で全メンバーの `species(key:)`・
   `moveOptionsByMember`/`abilityOptionsByMember` を終わらせ、2段目で全メンバーの `moveIds` をまとめて
   `resolveUnknownMoves` に渡す)。`resolveUnknownMoves` は `withTaskGroup` をやめ、辞書に無い ID を
   重複除去してから `service.moves(ids:)` を1回呼び、`try?` で失敗を握りつぶす形に書き換えた(A2・A3)。

### 3. 判断: `StubPokeCalcService` の拡張で既存テストとの衝突を避ける(要レビュー)

依頼者からの指示は「`TeamEditViewModelMoveLookupTests` が `move(id:)` の個別呼び出しをアサートしているなら、
そのテストは編集せず衝突として報告する」だった。実際に確認した衝突と、ここで採った解決策を明記する
(implementer・レビューア向けの確認ポイント)。

- **見つかった衝突**: `TeamEditViewModelMoveLookupTests.testSavedMoveOutsideTheFirstPageIsResolvedByIDOnLoad`
  (44〜45行)は `let lookups = await stub.moveLookups; XCTAssertEqual(lookups, [StubBulkMaster.hiddenMove.id], …)`
  で、`stub.moveLookups`(`move(id:)` が呼ばれるたびに ID を1件ずつ追記する記録配列)を直接アサートしている。
  `testNoLookupWhenEverySavedMoveIsAlreadyKnown` も同様に `moveLookups == []` を見る。`load()` が
  `move(id:)` の代わりに `moves(ids:)` を1回呼ぶ実装に変わると、`moveLookups`(`move(id:)` 専用の記録)は
  空のままになり、前者のテストは `[] != [hiddenMove.id]` で red になる。
- **採った解決策(このタスクで実装済み)**: `StubPokeCalcService.moves(ids:)` の実装を、独自の
  `moveBatchRequests: [[String]]`(1呼び出し = 1エントリ。呼び出しの回数・中身そのものを見たい新しい
  テスト用)に記録するのに**加えて**、受け取った `ids` をそのまま `moveLookups`(`move(id:)` と共有)にも
  `append(contentsOf:)` する。つまり `moveLookups` の意味を「`move(id:)` で引いた ID」から
  「`move(id:)` **または** `moves(ids:)` で引いた ID(引いた経路を問わないフラットな記録)」に広げた。
  この変更は `StubPokeCalcService.swift`(テスト補助。テスト本体ではない)だけに閉じており、
  `TeamEditViewModelMoveLookupTests.swift` は1文字も変えていない。`load()` が `moves(ids:)` の一括呼び出しに
  切り替わったとき、未知の ID の集合が変わらなければ(すなわち A2 の「集めて重複除去」が正しく動けば)
  `moveLookups` の中身(集合として見た場合)は今と変わらないので、上記2つの既存テストは
  **編集しなくても green のまま**になる見込み(`testSavedMoveOutsideTheFirstPageIsResolvedByIDOnLoad` は
  未知の ID が `hiddenMove.id` の1件だけなので、個別呼び出しでも一括呼び出しでも `moveLookups` は
  `[hiddenMove.id]` に一致する)。
- **これは「衝突を無かったことにする」のではなく「衝突の解消策を実装してテストで示した」判断**であり、
  implementer が実際に `load()` を切り替えたあとに `swift test` で
  `TeamEditViewModelMoveLookupTests` が green のままであることを確認すること。もし green にならない場合
  (例えば `load()` が `moves(ids:)` を複数回に分けて呼ぶような実装になった、など A2 の想定から外れた場合)は、
  この解決策が成立しない合図なので、そのテストを編集せずに立ち止まり、人間に確認すること(CLAUDE.md
  「テストを消したり弱めたりして通さない」)。
- **却下した代案**: `moveLookups` を触らず `TeamEditViewModelMoveLookupTests` を編集する。依頼者の指示
  (「do NOT edit it」)に反するため却下。

#### 3.1 実装後に見つかった2つ目の衝突と解決策(2026-09-24 追記)

implementer が `TeamEditViewModel.load()` を実際に `moves(ids:)` の一括呼び出しへ切り替えたあと、
上の見込み(「`moveLookups` の中身は変わらないので green のまま」)は ID の集合については正しかったが、
**失敗の再現方法**で別の衝突が見つかった。

- **見つかった衝突**: `TeamEditViewModelMoveLookupTests.testLookupNotFoundLeavesTheIDUnresolvedWithoutAScreenError`
  (既定の `moveLookupMode = .notFound` のまま、`setMoveLookupMode`/`setMoveLookupError` を呼ばない)と
  `testLookupTransportFailureLeavesTheIDUnresolvedWithoutAScreenError`
  (`stub.setMoveLookupMode(.immediate)` の後 `stub.setMoveLookupError(...)` で通信失敗を設定)は、
  どちらも `move(id:)` 用の設定(`moveLookupMode`/`moveLookupError`)だけで「技を解決できない」状況を
  作っていた。実装時点の `StubPokeCalcService.moves(ids:)` は独立した `moveBatchMode`(既定
  `.immediate`)を見ており、`moveLookupMode`/`moveLookupError` を一切参照しなかったため、`load()` が
  `moves(ids:)` に切り替わると、この2テストの意図(「解決できない環境」「通信失敗」)が
  `moves(ids:)` には伝わらず、常に成功応答(架空マスタからの解決)が返って red になった
  (「本来 nil であるべき `move(forID:)` が値を持ってしまう」形の red)。
- **採った解決策(このタスクで実装済み)**: `StubPokeCalcService` の `moveBatchMode: MoveBatchMode = .immediate`
  (必須の既定値)を `moveBatchModeOverride: MoveBatchMode?`(既定 `nil`)に変え、`nil` のとき
  `moves(ids:)` は `move(id:)` と同じ設定(`moveLookupMode`/`moveLookupError`)にそのまま従う
  (`moveBatchResultFollowingMoveLookupMode(ids:)`)。具体的には:
  - `moveLookupMode == .notFound`(既定): 実際の `getMovesByIds` は 404 を返さない
    (マスタに無い ID は結果から黙って省くだけ)ので、`move(id:)` のように `not_found` を `throw` する
    のではなく、**空配列を返す**(呼び出しは成功するが1件も解決できない = 「解決できない環境」を
    「まとめ取りでも1件も返らない」という形で近似する)。
  - `moveLookupMode == .immediate` かつ `moveLookupError` が設定されている: そのエラーを `throw` する
    (1回の HTTP 応答であるまとめ取りは、`move(id:)` のように ID ごとに成功・失敗を分けられないので、
    「通信失敗・503」は呼び出し全体を失敗させる近似にする)。
  - `moveLookupMode == .manual`: `moves(ids:)` 経由はまだ使うテストが無いので、`XCTFail` で気づける
    ようにするだけに留めた(今後 `TeamEditViewModel.load()` のキャンセル系テストを書くときに、
    `move(id:)` の `.manual` と同じ保留・解決の形を `moves(ids:)` にも用意するか検討する)。
  - `setMoveBatchMode(_:)` を明示的に呼んだテスト(このタスクで追加した
    `TeamEditViewModelMoveBatchLookupTests` の4本はすべて `setMoveBatchMode(.immediate)` を呼んでいる)は、
    この既定の「`moveLookupMode` に従う」経路を通らず、指定どおりの応答になる(変更不要だった)。
  この変更も `StubPokeCalcService.swift`(テスト補助)だけに閉じており、
  `TeamEditViewModelMoveLookupTests.swift`・`TeamEditViewModelMoveBatchLookupTests.swift`・
  `MoveBatchLookupServiceTests.swift`(`StubPokeCalcService` を使わず `APIPokeCalcService`/
  `MockPokeCalcService` を直接テストしているため無関係)のいずれも編集していない。
- **確認結果**: `swift test` 398件すべて green(既存の `TeamEditViewModelMoveLookupTests` 7件・新規の
  `TeamEditViewModelMoveBatchLookupTests` 4件・`MoveBatchLookupServiceTests` 10件を含む)。implementer の
  `TeamEditViewModel.swift`・`APIPokeCalcService.swift`・`MockPokeCalcService.swift` の実装(全メンバーの
  `species(key:)` 後にまとめて `moves(ids:)` を1回呼ぶ・重複除去・分割)は、この2章の受け入れ条件・
  1章の A1〜A3 のとおりで、テストが誤りを示す箇所は無かった(Sources 側の実装変更は不要だった)。

### 4. 判断: Calc/Reverse は `move(id:)` のまま変えない

`CalcViewModel`/`ReverseViewModel` の `reselectMove`("issue #68 の残り"3章)は、stage-1(構築から呼び出した
個体の技を1回だけ解決)・stage-2(既定の技が `moveOptions` から決まらないときに learnset を先頭から
`move(id:)` で1件ずつ解決し、条件に合う技が見つかったら止める)のどちらも「1件ずつ・見つかったら打ち切る」
逐次探索で、`moves(ids:)` に置き換えると次の理由で意味が変わってしまう。

- stage-2 は「learnset を先頭から見て、**最初に条件(ダメージ技)に合う技が見つかったら止める**」という
  短絡評価が本質("issue #68 の残り"3章「却下した案」・11章の world stage-2 世代保護の議論はこの短絡評価を
  前提にしている)。`moves(ids:)` でまとめて全件解決してから先頭から判定する形に変えると、
  `MasterSearch.maxMoveLookupsPerSelection`(4件)で打ち切っていた「呼びすぎない」歯止め(4章)の意味が
  「4件解決してみて条件に合わなければ諦める」から「常に(学習セットの残りに関わらず)4件をまとめて取得する」
  に変わり、変化技が並ぶ learnset で `move(id:)` なら1〜2回で見つかったはずのケースまで常に1回の
  まとめ取りを行うことになる(通信の往復回数はむしろ多くの実際のケースで減らないか変わらない一方、
  「必要な分だけ呼ぶ」という A3 の趣旨から外れる)。
- stage-1 は1件しか解決しないので `moves(ids:)` にする利益が無い(`ids: [id]` の1件配列にしても
  往復回数は変わらない)。
- 11章で固定した critic 指摘の回帰テスト(`recalculateIfPossible` の古いエラーの消し方・stage-2 の世代保護)は
  いずれも `move(id:)` を1件ずつ呼ぶ実装を前提に critic 検証済みであり、`moves(ids:)` に置き換えるとこれらの
  ガードを作り直してレビューし直す必要がある。今回のタスクの動機(構築の `load()` の往復回数を減らす)に
  対して割に合わない。

以上から、Calc/Reverse は今回のタスクでは変更しない(`move(id:)` を使い続ける)。将来 stage-2 の性質を
変えてよいという判断が別途下れば、そのときに別タスクとして検討する。

### 5. XCTest(すべて green。8章の critic 指摘を反映した後の最終形)

- `RequestLimitsTests.swift`: `testMoveBatchLimitMatchesTheOpenAPIContract`(1件。A1。green)。
- `MoveBatchLookupServiceTests.swift`(新規。A1。10件): `APIPokeCalcService.moves(ids:)` の要求(繰り返し
  クエリ・ヘッダー・重複除去・空配列で無呼び出し)・分割の境界(`testAPIMovesChunkBoundaries`。表駆動で
  `RequestLimits.maxMoveBatchIds` の `max`・`max + 1`・`2 * max`・`2 * max + 1` 件 → 1/2/2/3 回の呼び出しに
  なること・各回の `ids` が空にならないこと・分割しても連結すれば渡した順のままであることを確認。
  8章 critic 指摘 MUST への対応)・応答の写像・503/500/通信失敗(404 は無い)、
  `MockPokeCalcService.moves(ids:)`(未知の ID を省く・空配列・重複除去)。
- `TeamEditViewModelMoveBatchLookupTests.swift`(新規。A2〜A3。5件): 複数メンバーの未知の技を1回の
  `moves(ids:)` にまとめる・2体が同じ未知の技を持つときの重複除去・全員が既知の技だけなら呼ばない・
  失敗(通信失敗)で ID のまま `error` なし `moveIds` 不変・**保留中(`.manual`)の一括解決を `load()` の
  Task ごと cancel しても `error` なし・`moveIds` 不変・`isLoading` が解ける」
  (`testLoadCancellationDuringPendingBatchLeavesStateUnchanged`。8章 critic 指摘 OPTIONAL(2)への対応)。
- 既存の `TeamEditViewModelMoveLookupTests.swift` は**1行も変更していない**(7件、全件 green)。

### 6. `swift test` / `make ios-test` の実行結果

`cd ios/PokeCalcKit && DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test`:
399件実行、0件失敗(既存 `TeamEditViewModelMoveLookupTests` 7件・新規 `TeamEditViewModelMoveBatchLookupTests`
5件・`MoveBatchLookupServiceTests` 10件を含む、すべて green)。
`DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer make ios-test`(リポジトリルート)の結果は9章。

### 7. 実装者への注意(実装前に書いた申し送り。「現在」は実装前の状態を指す。実装済みの内容は2章・8章)

- `TeamEditViewModel.load()` の `resolveUnknownMoves` を、全メンバーの `species(key:)` が終わった後に
  1回だけ呼ぶ形へ書き換える(現在は各メンバーのループの中で呼んでいる。ループを2段に分け、1段目で
  `species(key:)`・learnset の読み直し・`moveOptionsByMember`/`abilityOptionsByMember` の更新を全メンバー分
  終わらせ、2段目で全メンバーの `moveIds` を集めて未知の ID を `moves(ids:)` に渡す)。
  `withTaskGroup` は不要になる(1回の `await service.moves(ids:)` で足りる)。
  失敗時は `try?` などで飲み込み、辞書を更新しない(A3)。
- `APIPokeCalcService.moves(ids:)` の重複除去・分割・連結、`MockPokeCalcService.moves(ids:)` の重複除去・
  未知 ID の省略を実装する(1章)。
  `MoveBatchLookupServiceTests.swift` を green にすることを完了の目安にする。
  分割の実装は「`ids` を `RequestLimits.maxMoveBatchIds` 件ごとに `chunked` して順に `await` する」形で足り、
  並行に投げる必要は無い(構築側は現実的に1回で収まるため。2章)。
  写像(`domainMove`・`domainErrorFromSchema`・`domainError`)は `move(id:)`/`species(key:)` と共通化してよい
  (`APIPokeCalcService.swift` に既にある private static 関数をそのまま再利用する)。
- 実装後、`TeamEditViewModelMoveLookupTests.swift`(既存)と
  `TeamEditViewModelMoveBatchLookupTests.swift`(新規)の両方が green になることを確認する。
  片方だけ green で他方が red なら、3章の想定(未知 ID の集合が変わらない)が崩れている合図なので、
  実装を見直すこと(既存テストを編集して通さない。CLAUDE.md 絶対ルール6)。
- `make ios-test`(gen-check・XCTest・XCUITest・Info.plist 検査・`check-request-limits.sh`)まで通すこと。

### 8. critic 指摘への対応(2026-09-24)

1回目の critic レビューは **FAIL** だったが、指摘はすべてテストの網羅不足に関するもので、実装
(`APIPokeCalcService.swift`/`MockPokeCalcService.swift`/`TeamEditViewModel.swift`)自体は「1章の A1〜A3の
とおりで問題なし」という判定だった。指摘への対応:

- **MUST(反映済み)**: `MoveBatchLookupServiceTests.swift` の `testAPIMovesChunksRequestsAtSixtyFiveIds`
  (65件固定・1ケースのみ)を、`RequestLimits.maxMoveBatchIds` を基準にした表駆動テスト
  `testAPIMovesChunkBoundaries` に置き換えた(`max`・`max + 1`・`2 * max`・`2 * max + 1` 件 →
  1/2/2/3 回の呼び出し。各回の `ids` が空でないこと・分割しても連結すれば渡した順のままであることも
  確認)。`(0..<65)` のような決め打ちの件数は `maxMoveBatchIds` の式に置き換えた。
  ミューテーションテスト(`APIPokeCalcService.moves(ids:)` の `stride(from: 0, to: dedupedIds.count, by:)` を
  一時的に `stride(from: 0, through: dedupedIds.count, by:)` に変えると最後の回が空の `ids` になる)で
  実際に新テストが red になることを確認してから元に戻した。
- **OPTIONAL(2. 反映済み)**: `TeamEditViewModelMoveBatchLookupTests.swift` に
  `testLoadCancellationDuringPendingBatchLeavesStateUnchanged` を追加。`setMoveBatchMode(.manual)` で
  `moves(ids:)` を保留させ、`load()` の Task を `waitForMoveBatchRequests(count: 1)` の後に `cancel()` し、
  `StubPokeCalcService.moves(ids:)` の `withTaskCancellationHandler` 経由で継続が `CancellationError` を
  受け取ることを確認する(`error == nil`・`moveIds` 不変・`isLoading == false`・その技は未解決のまま。
  A3)。これにより `resolveMoveBatch`/`cancelPendingMoveBatch`/`waitForMoveBatchRequests`/
  `waitForMoveBatchCancellation` が実際に使われるテストが増えた(削除ではなくテスト追加を選んだ)。
- **OPTIONAL(3. 反映済み)**: 1章 A2 に、1段目(全メンバーの `species(key:)`)の途中で失敗したら2段目
  (`moves(ids:)` の一括呼び出し)を一切行わない(旧実装は失敗したメンバーより前のメンバーの技は解決済みの
  まま残っていた)という差分を明記し、`error` が立って編集画面を止めるので実害が無いことを理由として添えた。
- **OPTIONAL(5. 反映済み)**: `MoveBatchLookupServiceTests.swift`・`TeamEditViewModelMoveBatchLookupTests.swift`
  のファイル冒頭コメントから、実装済みになった今では誤りとなる「`TODO(implementer)` のプレースホルダ」
  「まだ `move(id:)` を個別に呼んでいる」「red のままでよい」といった記述を削除した。
- 本章の追記自体(ADR の状態を「実装完了」にする・2章/5章の記述を実装後の形に直す)も含め、上の対応を
  1つのタスクとして implementer が行い、再レビューへ引き継ぐ。

### 9. `swift test` / `make ios-test` の最終実行結果(8章の対応後)

`cd ios/PokeCalcKit && DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test`:
399件実行、0件失敗。

`DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer make ios-test`(リポジトリルート):
`** TEST SUCCEEDED **`(終了コード0)。`ios-test-unit: 全 412 件 / 成功 412 / 失敗 0 / スキップ 0 / 想定内の失敗 0`
(`PokeCalcCoreTests` 399件 + `PokeCalcDesignTests` 13件)・`ios-test-ui: 全 17 件 / 成功 17 / 失敗 0`・
`ios-check-infoplist` 成功。`ios-gen-check`・`ios-check-request-limits`・`ios-lint` を含め全ステップ green。

## P6-11 の受け入れ条件(issue #334: 攻撃側プリセットの表示名が技の分類に追従しない。実装完了)

- 日付: 2026-09-25 / 担当レーン: iOS / 関連: issue #334、issue #71(SP・性格の定義の単一化。本件とは別。#71 は
  プリセット定義そのものを engine へ移す提案で本件の対象外)、`KnownDefenderPreset.label(for:)`(同じ形の既存実装)、
  Web `web/src/domain/attackerPresets.ts`・`web/src/i18n/ja.ts` の `attackerPresetText`

### 0. 何が壊れているか

`AttackerPreset.label`(`ios/PokeCalcKit/Sources/PokeCalcCore/AttackerPreset.swift:16-22`)は技の分類を受け取らず、
常に固定の「A特化」「A振り」「無振り」を返す。実際の SP は `AttackerPreset.build` が技の分類(物理・変化 = atk、
特殊 = spa)で切り替えているため、特殊技を選んでいるのに見出しは「A」のままという不一致が起きる
(`CalcScreenView.swift:140`、`ReverseScreenView.swift:150` が `preset.label` を表示)。
`KnownDefenderPreset.label(for:)`(同じファイル内。逆算「受けたダメージ」の自分側)は最初から技の分類を受け取って
HB/HD を切り替えており、`AttackerPreset` だけこの形になっていなかった。

### 1. 判断: 表記は Web の出荷済み文言に揃える(requirements.md の文言そのものではない)

issue のゴールは「同じ入力に対して Web と iOS が同じ語で結果を示す」こと。Web は
`attackerPresetText`(`web/src/i18n/ja.ts:93-102`)で「A特化」「A振り(無補正)」「C特化」「C振り(無補正)」「無振り」を
既に出荷しているため、iOS もこの語に合わせる。requirements.md 32行目は「A振り(**補正なし**)」と書かれており、
Web の「(**無補正**)」とは表記ゆれがあるが、これは Web 側が先に出荷した文言を正として扱う(クライアント間の一致を
優先。ADR は requirements.md の表記を Web に合わせて直すことを Web レーンに提案するだけで、本 ADR ではどちらの
ドキュメントも書き換えない)。

### 2. 判断: `AttackerPreset.label(for:)` を追加し、`KnownDefenderPreset.label(for:)` と同じ形にする

- 文字(A/C)は `AttackerPreset.relevantStat(for:)` が返す `StatKey`(atk/spa)から決める。分類→文字の対応は
  1箇所にする(Web の `statLetter`/`fullSuffix`/`xSuffix` の3定数と同じ考え方。分類ごとの switch を複数箇所に
  重複させない)。
- 既存の `AttackerPreset.label`(技の分類を受け取らない)は、`AttackerPresetTests.testCasesAreOrderedAndLabeledAsRequirements`
  が旧文言(「A特化」「A振り」「無振り」)を固定しているため、**削除しない**(CLAUDE.md 絶対ルール6: 既存テストを
  編集・緩和して通さない。このテストを書き換えたいなら別課題として提案する)。呼び出し側(`CalcScreenView.swift:140`、
  `ReverseScreenView.swift:150` の `AttackerPreset` 分岐)は `label(for:)` に置き換える。
- `ReverseScreenView.presetSegmentedRow`(`ios/PokeCalc/ReverseScreenView.swift:141-171`)の `case .attacker:` 側は
  `KnownDefenderPreset.label(for:)` を既に技の分類つきで呼んでいる(相手の技=`viewModel.selectedMove?.category ?? .physical`)。
  `case .defender:`(自分が攻撃側。issue の対象)は同じ `viewModel.selectedMove`(この側では自分が選んだ技)の
  分類を渡せばよい。技が未選択のときは `AttackerPreset.relevantStat(for:)` と同じ既定(物理)にフォールバックする
  (`ReverseScreenView.swift:159` の `?? .physical` と同じパターン)。
- `CalcScreenView.presetSegmentedRow`(`ios/PokeCalc/CalcScreenView.swift:134-158`)は `viewModel.selectedMove?.category`
  (`CalcViewModel.swift:345`)を渡す。未選択時のフォールバックも物理。

### 3. 対象外(issue に記載のその他の差分。iOS 単独では直さない)

- **相性の表記**: iOS(`DisplayLabels.swift:59-72`)は「ばつぐん(×2)」と倍率を併記、Web(`ja.ts:510-513`)は
  「効果はばつぐん」で倍率なし。issue の既定案は倍率併記への統一を推奨しており、iOS は変更不要(既に倍率を出している)。
  Web 側を iOS に揃えるかは Web レーンへの提案とする(本 ADR・本タスクでは Web のコードを変更しない)。
  Web レーンの `docs/ai-shared/CURRENT_STATE.md` の `Next` か `COORDINATION.md` に一言追記することを implementer に依頼する。
- **防御側の持ち物比較 UI**: iOS は持ち物を1つずつトグル、Web は「持ち物の候補も比較」1つで自動選定。UI の作りの違いで
  本 issue のスコープ外(issue の「変更範囲 / 対象外」に明記)。
- **逆算「受けたダメージ」の自分の耐久**: iOS は `KnownDefenderPreset` で選べる、Web は無振り固定。既存の差分で
  本 issue のスコープ外。

### 4. 受け入れ条件(検証可能な形)

1. `AttackerPreset.label(for:)` が `MoveCategory` を受け取り、物理・変化 = 「A特化」/「A振り(無補正)」、
   特殊 = 「C特化」/「C振り(無補正)」、`.none` はどの分類でも「無振り」を返す
   (`AttackerPresetTests.testLabelForCategoryMatchesWebWording`)。
2. 既存の `AttackerPreset.label`(技の分類なし)は残り、`testCasesAreOrderedAndLabeledAsRequirements` は
   1行も変更せずに green のまま。
3. `CalcScreenView` の攻撃側プリセットのピルは、選択中の技の分類が特殊のとき「C特化」「C振り(無補正)」「無振り」を表示する
   (XCUITest `CalcScreenUITests.testSelectingSpecialMoveShowsCLetterPresetLabel`)。
4. `ReverseScreenView` の `side == .defender`(与えたダメージ。自分が攻撃側)のプリセット行も同じ規則(自分が選んだ技の
   分類で A/C を切り替える)に従う(unit test は `AttackerPreset.label(for:)` の網羅で担保。View からの呼び出しは
   critic レビューで目視確認する)。
5. 技が未選択のとき(起動直後など)は物理として表示する(`AttackerPreset.relevantStat(for:)` の既定と同じ)。
6. `swift test`(PokeCalcKit)がすべて成功し、`make ios-test` の unit/XCUITest/gen-check/lint がすべて成功する。

### 5. 追加したテスト(この時点では失敗する。実装はしていない)

- `AttackerPresetTests.testLabelForCategoryMatchesWebWording`
  (`ios/PokeCalcKit/Tests/PokeCalcCoreTests/AttackerPresetTests.swift`): `AttackerPreset.label(for:)` を
  9通り(3プリセット × 3分類)のテーブル駆動で確認。`swift test --filter AttackerPresetTests` で確認済み:
  `aFull`/`aMax` の特殊のケースが `"A特化"`/`"A振り(無補正)"` のまま返っていて `"C特化"`/`"C振り(無補正)"` と
  一致せず2件 red(物理・変化・`.none` の6件は現状の仮実装でも green。分類を無視しているだけなので当然)。
  他の既存6テストは無変更のまま green(`swift test` 全体で400件中2件のみ red)。
- `CalcScreenUITests.testSelectingSpecialMoveShowsCLetterPresetLabel`
  (`ios/PokeCalcUITests/CalcScreenUITests.swift`): 計算画面を開き、既定の物理技で `attackerPreset-aFull` の
  ラベルが「A特化」であることを確認した後、`movePicker` の検索シートから特殊技(`Resources/moves.json` の
  `test-move-special-a`。`testMoveSearchSheetFiltersAndSelects` で使っているのと同じ架空技)を選び、
  ラベルが「C特化」に変わるまで待って確認する。**未実行**(シミュレータのビルドが要るため spec-writer では
  走らせていない。implementer が `make ios-test` で実行して red → green を確認すること)。

### 6. 実装者への注意(TODO(implementer) を検索すればコード上の該当箇所が見つかる)

- `AttackerPreset.swift` に追加済みの `label(for:)` は **仮実装**(`moveCategory` を無視して旧来の固定文字列を
  返しているだけ)。`TODO(implementer)` コメントの通り、`relevantStat(for:)` の結果(`.atk`/`.spa`)から文字を
  引く実装に直すこと。文字・接尾辞は Web の `attackerPresetText`(`statLetter`/`fullSuffix`/`xSuffix`)のように
  1箇所にまとめ、`switch moveCategory` を複数箇所に重複させない。
- `CalcScreenView.swift:140` と `ReverseScreenView.swift:150`(`case .defender:` 側だけ。`case .attacker:` は
  `KnownDefenderPreset.label(for:)` で対応済み)を `preset.label` → `preset.label(for: category)` に変更し、
  各画面の「いま選ばれている技の分類」(未選択時は物理)を渡すこと。
- 相性の表記統一(3章)は本タスクのコード変更には含めない。Web への提案だけ `docs/ai-shared/CURRENT_STATE.md` か
  `COORDINATION.md` に一言残すこと。
- 完了条件は4章の受け入れ条件。`swift test` と `make ios-test` の両方を実行し、結果をこの章に追記すること。

### 7. 実装結果(2026-09-25)

- `AttackerPreset.label(for:)`(`ios/PokeCalcKit/Sources/PokeCalcCore/AttackerPreset.swift`)を、6章の指示どおり
  `relevantStat(for:)` が返す `StatKey`(atk/spa)から文字(A/C)を引く実装に直した。文字・接尾辞は
  `statLetter(for:)`(private static メソッド)・`fullSuffix`/`xSuffix`(private static 定数)の1箇所にまとめ、
  Web の `statLetterJa`/`fullSuffix`/`xSuffix` と同じ考え方にした。技の分類を受け取らない旧 `label` は
  2章の判断どおり削除していない。
- `CalcScreenView.presetSegmentedRow`(`ios/PokeCalc/CalcScreenView.swift:140` 付近)を
  `preset.label(for: viewModel.selectedMove?.category ?? .physical)` に変更した。
- `ReverseScreenView.presetSegmentedRow` の `case .defender:`(`ios/PokeCalc/ReverseScreenView.swift:146` 付近)を
  `case .attacker:` 側(`KnownDefenderPreset.label(for:)`)と同じパターンで `let category = viewModel.selectedMove?.category ?? .physical`
  を追加し、`preset.label(for: category)` に変更した。`case .attacker:` 側は無変更。
- `TODO(implementer)` マーカーを `AttackerPreset.swift`・`CalcScreenUITests.swift` から削除した
  (テストのロジック自体・「issue #334」の説明コメントは変更していない)。
- 5章の `AttackerPresetTests.testLabelForCategoryMatchesWebWording` と `CalcScreenUITests.testSelectingSpecialMoveShowsCLetterPresetLabel`
  は1行も変更していない。
- 検証結果: `cd ios/PokeCalcKit && DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test`:
  400件実行、0件失敗(新規 `testLabelForCategoryMatchesWebWording` を含む)。
  `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer make ios-test`(リポジトリルート):
  `** TEST SUCCEEDED **`(終了コード0)。`ios-test-unit: 全 413 件 / 成功 413 / 失敗 0`
  (`PokeCalcCoreTests` 400件 + `PokeCalcDesignTests` 13件)・`ios-test-ui: 全 18 件 / 成功 18 / 失敗 0`
  (新規 `testSelectingSpecialMoveShowsCLetterPresetLabel` を含む)・`ios-check-infoplist` 成功。
  `ios-lint`・`ios-gen-check`・`ios-check-request-limits` を含め全ステップ green。

## P6-12 の受け入れ条件(issue #71: 攻撃側プリセットを engine/presets/attacker.json に揃える。実装済み)

- 日付: 2026-09-25 / 担当レーン: iOS / 関連: issue #71、ADR-0114「Web・iOS への依頼」、ADR-0500 §6、ADR-0010 §5.3、
  本 ADR「P6-11」(`label(for:)`)、「issue #110」2章(ホスト側の同期検査)

### 0. 何がずれているか

正は `engine/presets/attacker.json`(ADR-0114)。iOS の `AttackerPreset` は規則(SP・性格)は同じだが、
キー(`aFull`/`aMax`/`none` ↔ `x_full`/`x`/`none`)、並び順(特化 → 振り → 無振り ↔ 無振り → 特化 → 振り)、
既定(A特化 ↔ `default: "none"`)が JSON と違う。同期を検査する仕組みも無い。

### 1. 判断: 契約テストは XCTest で JSON を直接読む(ホスト側スクリプトにしない)

- 「issue #110」2章は「XCTest はシミュレータのサンドボックスで走りリポジトリのファイルを読めない」ことを前提に
  ホスト側スクリプト(`check-request-limits.sh`)を選んだ。今回これを実測した(2026-09-25): `#filePath` からの相対で
  `engine/presets/attacker.json` を `Data(contentsOf:)` で読む XCTest を `xcodebuild test -scheme PokeCalcKit-Package
  -destination 'platform=iOS Simulator,name=iPhone 18 Pro'` で走らせ、**読めた**(シミュレータのテストプロセスは
  ホストのファイルシステムをそのまま見る)。macOS の `swift test` でも読める。
- よって `make ios-test-unit` と `swift test` の両方で走る XCTest(`AttackerPresetCatalogContractTests`)にする。
  JSON はリポジトリに複製しない。ファイルが無ければ `XCTFail`(スキップしない)。
- 前提: 実機ではこのテストは走らない(`make ios-test` はシミュレータだけ)。`#filePath` はビルドしたマシンの絶対パスなので、
  別マシンでビルドした成果物を走らせる運用を始めたらここを見直す。
- `check-request-limits.sh` を XCTest に移すかは本タスクの対象外(前提が崩れたことだけ記録する)。

### 2. 判断: case 名は変えず、JSON のキーとの対応を `catalogKey` の1か所に書く

- case 名(= raw value)は `accessibilityIdentifier`(`attackerPreset-aFull`・`reverseAttackerPreset-aMax` 等)と
  XCUITest が参照している。raw value を JSON に揃えると識別子と UI テストが一斉に変わり、得るものが無い。
- `AttackerPreset.catalogKey: String`(`aFull` → `"x_full"`、`aMax` → `"x"`、`none` → `"none"`)を足し、対応はここにだけ書く。
  契約テストはこれで JSON と突き合わせる。
- `allCases` の順(case の宣言順)を JSON の `presets` の順(`none` → `aFull` → `aMax`)にする。画面のピルは
  `ForEach(AttackerPreset.allCases)` なので、計算画面・逆算画面(与えたダメージ)とも左から「無振り / A(C)特化 / A(C)振り(無補正)」に変わる。
  Web(ADR-0300 §5)と ADR-0010 §5.3 の attacker の事前順位と同じ並び。

### 3. 判断: 既定は JSON の `default`(無振り)に揃え、1か所で持つ

- `AttackerPreset.defaultPreset`(= `.none`)を足す。`CalcViewModel` / `ReverseViewModel` の `attackerBuildSource` の初期値と、
  `load()` での再設定(`CalcViewModel.swift` の `attackerBuildSource = .preset(.aFull)`、`ReverseViewModel.swift` の同じ行)は
  すべて `.preset(AttackerPreset.defaultPreset)` にする。case の直書きを残さない。
- 起動直後の計算(計算画面)は無振り = SP 0 + 無補正の性格で組み立てる。これまでの A特化(atk 32 + atk 上昇)から変わる。
  利用者に見える変化なので、Web と同じ既定に揃う利点(同じ入力で同じ結果)を優先した(ADR-0114 の決定どおり)。
- 既定の組み立てに要る性格は「無補正」になる。マスタに無補正の性格が無ければ起動時に `natureUnavailable`(これまでは上昇性格)。

### 4. 受け入れ条件(検証可能な形)

1. `AttackerPreset.allCases.map(\.catalogKey)` が JSON の `presets[].key` と順序まで一致する
   (`AttackerPresetCatalogContractTests.testCasesMatchCatalogKeysInOrder`)。
2. `AttackerPreset.defaultPreset.catalogKey` が JSON の `default` と一致する(`testDefaultMatchesCatalog`)。
3. `AttackerPreset.relevantStat(for:)` が JSON の `relevantStat` と全分類で一致し、JSON の分類の集合が `MoveCategory.allCases` と同じ
   (`testRelevantStatMatchesCatalogForEveryCategory`)。
4. 全プリセット × 全分類で、`AttackerPreset.build` の SP が「関連ステータスだけ `relevantSp`、他は 0」、性格が
   `boost` なら (plus = 関連ステータス, minus = `boostMinus[関連]`)、`neutral` なら plus/minus とも nil
   (`testBuildFollowsCatalogSpAndNatureRules`)。JSON は最上位・各プリセットの項目が既知のものだけで、`schemaVersion` が 1、
   `nature` が boost/neutral のどちらか(`testCatalogHasOnlyKnownFieldsAndVersion`)。
5. 契約テストは macOS の `swift test` とシミュレータの `make ios-test-unit` の両方で走り、JSON が無ければ失敗する。
6. 計算画面・逆算画面を開いた直後、自分側のプリセットは無振りが選ばれ、起動時の計算要求は SP 0 + 無補正の性格
   (`CalcViewModelTests.testLoadSelectsDeterministicDefaultsAndCalculatesOnce`・`ReverseViewModelTests.testLoadSelectsDefaultsWithoutCalculating`)。
7. 画面のピルは左から 無振り → 特化 → 振り の順に並び、起動直後は無振りだけが選択状態
   (XCUITest `CalcScreenUITests` / `ReverseScreenUITests` の `testAttackerPresetPillsFollowCatalogOrderAndDefault`)。
8. `swift test` と `make ios-test` がすべて成功する。

### 5. テストの変更(順序・既定の変更に伴うものだけ。それ以外は弱めていない)

追加:

- `ios/PokeCalcKit/Tests/PokeCalcCoreTests/AttackerPresetCatalogContractTests.swift`(新規。4章 1〜5。5件)
- `CalcViewModelTests.testMissingFullPresetNatureOnSelectionSetsErrorWithoutCalculating`(下の `testMissingPresetNature...` の
  旧来の意図「A特化の上昇性格が無ければエラーで計算しない」を、選び直しの経路で残す)
- `CalcScreenUITests.testAttackerPresetPillsFollowCatalogOrderAndDefault`・`ReverseScreenUITests.testAttackerPresetPillsFollowCatalogOrderAndDefault`
  (4章 7。ピルの `frame.minX` の順と `isSelected`)

期待値を変えた既存のアサーション(理由はすべて「並び順・既定を JSON に揃えたため」):

| テスト | 変更前 | 変更後 |
|---|---|---|
| `AttackerPresetTests.testCasesAreOrderedAndLabeledAsRequirements` | `allCases == [.aFull, .aMax, .none]`、`label` が `["A特化", "A振り", "無振り"]` | `[.none, .aFull, .aMax]`、`["無振り", "A特化", "A振り"]`(文言は同じ。順序だけ) |
| `CalcViewModelTests.testLoadSelectsDeterministicDefaultsAndCalculatesOnce` | `attackerPreset == .aFull`、要求 `sp(atk: 32)` + `atkUpNature` | `== AttackerPreset.defaultPreset`、要求 `sp()` + `neutralNature` |
| `CalcViewModelTests.testAttackerWithOnlyStatusMovesFallsBackToFirstLearnsetMove` | 既定のまま `sp(atk: 32)` を確認 | 先に `selectAttackerPreset(.aFull)` してから同じ `sp(atk: 32)` を確認(無振りでは振り先が見えないため) |
| `CalcViewModelTests.testEachInputChangeCallsCalcBulkExactlyOnceWithTheRightShape` | 起動直後に特殊技を選び A特化の組み立てを確認。件数 2〜7 | 先に `selectAttackerPreset(.aFull)`(計算1回増える)。件数 3〜8。他の期待値は同じ |
| `CalcViewModelTests.testSwapSidesSwapsSpeciesAndReselectsMoveWithOneCalc` | 既定の A特化が入れ替え後も残ることを確認 | 先に `selectAttackerPreset(.aFull)`(`before` を取る前)。アサーションは同じ |
| `CalcViewModelTests.testMissingPresetNatureSetsErrorWithoutCalculating` | 性格一覧 `[neutral, spaUp]`(A特化の上昇性格が無い) | `[atkUp, spaUp]`(既定の無振りに要る無補正が無い)。期待(エラー・計算0回)は同じ |
| `CalcViewModelTeamIndividualTests.testUnknownTeamOrMemberIsIgnored` | `attackerPreset == .aFull` | `== AttackerPreset.defaultPreset` |
| `ReverseViewModelTests.testLoadSelectsDefaultsWithoutCalculating` | `attackerPreset == .aFull` | `== AttackerPreset.defaultPreset` |
| `ReverseViewModelTests.testSwitchingToAttackerSideResetsObservationsAndUsesOpponentLearnset` | `attackerPreset == .aFull` | `== AttackerPreset.defaultPreset` |
| `ReverseViewModelTests.testValidPercentObservationCallsReverseOnceWithDefenderSideRequest` | 既定のまま A特化(`sp(atk: 32)` + `atkUpNature`)を確認 | 観測を入れる前(逆算しない状態)に `selectAttackerPreset(.aFull)`。アサーションは同じ |
| `ReverseViewModelTeamIndividualTests.testUnknownTeamOrMemberIsIgnored` | `attackerPreset == .aFull` | `== AttackerPreset.defaultPreset` |
| `CalcScreenUITests.testTeamSourceRowEmptyThenSelectingMemberClearsPresetAndPresetClearsBack` | 起動直後に `attackerPreset-aFull` が選択状態 | `attackerPreset-none`(以降の外れる/戻る確認も同じボタンで) |
| `ReverseScreenUITests.testTeamSourceRowEmptyThenSelectingMemberClearsPresetAndPresetClearsBack` | `reverseAttackerPreset-aFull` | `reverseAttackerPreset-none` |

P6-11 4章 2 の「`testCasesAreOrderedAndLabeledAsRequirements` は1行も変更しない」は、P6-11 の時点の条件。本タスクで順序の行だけを変えた
(文言の行は同じ文字列の並べ替えのみ)。

spec 時点の `swift test`(2026-09-25): 406件実行、7テストで失敗10件(アサーション単位)。すべて本タスクの新しい振る舞いのみ
(契約テスト3件: キー・順序・既定・build の突き合わせ、`testCasesAreOrderedAndLabeledAsRequirements` 2件、
`testLoadSelectsDeterministicDefaultsAndCalculatesOnce` 2件、`testMissingPresetNatureSetsErrorWithoutCalculating` 1件、
`testMissingFullPresetNatureOnSelectionSetsErrorWithoutCalculating` 2件)。
仮の実装(2・3章どおり)を当てて `swift test` 406件 green、シミュレータで契約テスト5件と 4章 7 の UI テスト2件・上表の UI テスト2件・
`testSelectingSpecialMoveShowsCLetterPresetLabel` が green になることを確かめてから、仮の実装は戻した。

### 6. 実装者への注意(`TODO(implementer)` を検索すればコード上の該当箇所が見つかる)

- `AttackerPreset.swift`: case の宣言順を `none, aFull, aMax` に並べ替える。`catalogKey` を switch で3通り返す
  (`rawValue` を返す仮実装を消す)。`defaultPreset` を `.none` にする。case 名・raw value は変えない。
- `CalcViewModel.swift` / `ReverseViewModel.swift`: `.preset(.aFull)` の4か所(プロパティの初期値2・`load()` の再設定2)を
  `.preset(AttackerPreset.defaultPreset)` にする。「`AttackerPreset.allCases` の最初(A特化)」と書いたドキュメントコメント
  (`CalcViewModel.swift` の `attackerBuildSource`、`ReverseViewModel.swift` の同じプロパティ)も直す。
- View(`CalcScreenView` / `ReverseScreenView`)は `allCases` を並べているだけなので変更不要の見込み。
- 契約テスト・上表の期待値は変えない。JSON(`engine/`)は触らない。
- 完了条件は4章。`swift test` と `make ios-test` を実行し、結果をこの章の後ろに追記する。

### 7. 実装結果(2026-09-25)

- 2・3章どおりに実装(case 順 `none, aFull, aMax`、`catalogKey`、`defaultPreset = .none`、`CalcViewModel`/`ReverseViewModel` の
  4か所を `AttackerPreset.defaultPreset` に置換)。View(`CalcScreenView`/`ReverseScreenView`)は `allCases` を並べるだけで変更不要だった。
- `swift test`(macOS, `ios/PokeCalcKit`): 406件実行、0失敗。
- `make ios-test`(シミュレータ): `ios-lint`・`ios-gen-check`・`ios-check-request-limits`・`ios-check-infoplist` OK、
  `ios-test-unit` 419件成功・0失敗、`ios-test-ui` 20件成功・0失敗ですべて green(終了コード0)。
  1回目の実行では `ReverseScreenUITests.testOpponentSpeciesSearchSheetFiltersAndSelects`(本タスクと無関係。
  差分に含まれない・種族検索シートのテスト)が1件だけ失敗したが、単体で再実行すると成功(27.9秒)。
  当時は他レーンの並行セッションが同じシミュレータを使っていたための環境要因と判断し、
  並行実行が無い状態で `make ios-test` を再実行して全件成功を確認した。

## issue #274 の受け入れ条件(計算画面の「詳細」: 急所・やけど・天候・フィールド・壁・ランク・特性。実装完了)

- 日付: 2026-09-25 / 担当レーン: iOS(Web は iOS の決定に追従する。docs/ai-shared/DECISIONS.md 2026-09-25「計算条件の入力 UI」)/
  関連: issue #274、docs/requirements.md §2「補正」、docs/design.md「数値の直接入力は『詳細』を開いたときだけ」、
  本 ADR「P6-2a」規則3〜7、「P6-2d」、「issue #113」、ADR-0500 §3

### 0. 何が足りないか

engine・API は急所(`options.critical`)・場(`field`: 天候・フィールド・壁)・攻撃側の `ranks`・`status`・`abilityId` を受け付けるが、
iOS の計算画面はどれも入力できない。`CalcViewModel.buildRequest` は `critical: false` 固定で `field` を持たず、
プリセット経路の `abilityId` は常に nil、ランク・状態異常は常に既定値。

### 1. 範囲

- 対象(計算画面・`CalcViewModel`): 急所、攻撃側のやけど、天候、フィールド、防御側の壁(リフレクター・ひかりのかべ・オーロラベール)、
  攻撃側のランク(選択中の技の関連ステータスだけ)、攻撃側の特性。
- 対象外(制約として記録する):
  - **防御側のランク・特性・状態異常**: `BulkCalcRequest` は `defenderSpeciesKey` しか持たず、送る場所が契約に無い。
    API レーンへ提案した(DECISIONS.md の同じ項目。既定案: `BulkCalcRequest` に任意の `defender` 上書き
    `{ abilityId?, ranks?, status? }` を足し、全行に同じ値を当てる)。契約が入ったら別タスクで UI を足す。
  - やけど以外の状態異常: ダメージに効くのはやけどだけなので出さない(`StatusCondition` の他の値は送らない)。
  - 攻撃側の場の壁(`attackerScreens`): シングルのダメージに効かないので出さない(常に既定値)。
  - 壁の「逆向き」(入れ替え後に壁の側を付け替える): 追いかけない。必要なら後続で。
  - ダブル固有の補正、1 vs 1(`CalcRequest`)・逆算(`ReverseRequest`)への場の追加(この画面は `calcBulk` だけを使う)。
  - どの入力を常時表示にするか(issue の「人間の判断」): 既定案どおり**すべて「詳細」の中**(既定は閉じる)。

### 2. 判断: 状態の持ち方と引き継ぎ

- 条件はすべて `CalcViewModel` の画面の状態(`isCritical`・`isAttackerBurned`・`weather`・`terrain`・`defenderScreens`・
  `attackerRanks`・`attackerAbilityId`)。「P6-2a」規則6(入れ替えでプリセット・持ち物は残す)と同じく、
  **防御側・技・プリセット・持ち物・比較・攻守入れ替え・構築の呼び出しでは消さない**。例外は特性だけ(下)。
- **ランク**: `attackerRanks: RankBlock` の atk と spa を別々に持つ。ステッパーが編集するのは `attackerRankStat`
  (= `AttackerPreset.relevantStat(for: 選択中の技の分類)`。技が無いときは atk)。技を物理 ↔ 特殊に変えると
  ステッパーの対象が A ↔ C に切り替わるが、もう一方のランクは消さずに持ち続け、要求には atk・spa の両方をそのまま送る
  (engine は技の分類の関連ステータスだけを使うので結果は変わらない。def/spd/spe は常に 0)。値は -6..+6 に丸める。
- **やけど**: on のとき `attacker.status = .burn`、off のとき `.none`。構築の個体は状態異常を持たないので、
  構築を呼んでいても画面の値を使う。
- **ランク(構築)**: 構築の個体はランクを持たない(`TeamMember` に無い)ので、構築を呼んでいても画面の値を使う。
- **特性**: 選択肢は攻撃側の `species(key:)` の `abilities`(その順)+「指定なし」(nil。要求に載せない)。
  - 既定(起動時・プリセット経路)は nil。**これまでの要求と同じ**(プリセット経路は特性を送っていなかった)。
  - 構築の個体を呼ぶと、その個体の保存された特性を選択状態にする(これまでの `individualForRequest` と同じ値)。
    利用者が選び直すと上書きする(構築の選択は外れない)。同じ個体を呼び直すと保存された特性に戻る。
  - 構築 → プリセットに切り替えると nil に戻す(既存 `testSelectingPresetAfterTeamClearsTeamSelection` の「プリセット経路は
    特性を持たない」を保つ)。プリセット → プリセットでは利用者が選んだ特性を残す。
  - 攻撃側の種族が変わったとき(選択・入れ替え・構築の呼び出し)は、選択中の特性が新しい種族の `abilities` にあれば残し、
    無ければ nil に戻す(旧種族の特性を送らない。issue #100 の構築編集と同じ考え方。ただし先頭へは寄せず「指定なし」に戻す。
    構築の呼び出しだけは上の「保存された特性」を優先する)。
  - `attackerAbilityOptions` に無い ID は無視する(計算もしない)。
- **場**: 天候・フィールドは1つ選ぶピル(既定 なし)。壁は3つ独立のトグル(重ねて張れる。`defenderScreens` だけに載る)。
- **計算の回数**: 値が変わる操作ごとに `calcBulk` をちょうど1回(「P6-2a」規則「入力ごとに計算1回」)。
  値が変わらない操作(選択中のピルを押し直す・+6 で + を押す・同じ特性を選ぶ)は計算しない。
  既存の操作(技の変更・種族の変更・入れ替え・構築の呼び出し)の回数は変えない(ランク・特性の付け替えで余計に計算しない)。
- **世代・キャンセル**: 条件の変更も他の入力と同じく `beginInput()` の世代と `LatestTaskRunner`(`scheduleLatest`)に乗せる。
  追い越された計算は cancel され、状態(先に変えた条件)は次の要求に積み上がる。

### 3. 判断: 文言・並び(Web も同じにする)

文言はすべて Core の `DisplayLabels.swift` に置く(`CalcConditionLabels`・`WeatherLabel`・`TerrainLabel`・`ScreenKindLabel`・`RankLabel`)。
並びは各 enum の `allCases` の順(`Terrain` はゲームの並びにし、openapi の enum の順とは違う。値の集合は同じ)。

| 項目 | 文言(左から) |
|---|---|
| 折りたたみの見出し | 詳細 |
| トグル | 急所 / やけど |
| 天候(`Weather`) | なし / はれ / あめ / すなあらし / ゆき(none, sun, rain, sand, snow) |
| フィールド(`Terrain`) | なし / エレキフィールド / グラスフィールド / サイコフィールド / ミストフィールド(none, electric, grassy, psychic, misty) |
| 防御側の壁(`ScreenKind`) | リフレクター / ひかりのかべ / オーロラベール(reflect, lightScreen, auroraVeil) |
| 小見出し | 天候 / フィールド / 防御側の壁 / 攻撃側のランク / 攻撃側の特性 |
| ランク | 「A +1」「C -2」「A ±0」(atk → A、spa → C。`AttackerPreset` と同じ文字。符号は ASCII の + と -、0 は ±0) |
| 特性の未指定 | 指定なし |

「詳細」の中の並び(上から): 急所・やけど(横並びのトグル)→ 攻撃側のランク → 攻撃側の特性 → 天候 → フィールド → 防御側の壁。

### 4. 判断: 写像(`APIPokeCalcService`)

- ドメインに `Weather`・`Terrain`・`ScreenKind`・`Screens`・`FieldState` を足し、`BulkCalcRequest.field: FieldState`(既定 `FieldState()`)を持たせた。
  既存の呼び出し(`field` を渡さない)はそのまま何もない場になる。
- `generatedBulkCalcRequest` は `field == FieldState()` のとき `field` を送らない(この機能より前の要求本文と同じにするため。
  openapi 上も省略と既定は同じ意味)。既定でないときは `weather`・`terrain`・`defenderScreens`(3つの真偽値)を送る
  (`attackerScreens` は既定なら省略してよい)。
- `options.critical` はこれまでどおり常に明示で送る。`attacker` の `ranks`・`status`・`abilityId` は既存の `generatedIndividual` がすでに写している。
- `MockPokeCalcService.calcBulk` は条件を受け付け、条件なしと同じ形の行を返す(モックは計算しない。ADR-0500 §4)。変更不要の見込み。

### 5. 受け入れ条件(検証可能な形)

1. 起動直後の要求はこの機能より前と同じ: `critical == false`・`field == FieldState()`・`attacker.status == .none`・
   `attacker.ranks == RankBlock()`・`attacker.abilityId == nil`、計算1回。HTTP 本文には `field` が無い
   (`CalcViewModelConditionsTests.testDefaultsReproduceTheRequestSentBeforeThisFeature`・`APIPokeCalcServiceConditionsTests.testCalcBulkOmitsFieldWhenDefault`)。
2. 急所・やけど・天候(全5値)・フィールド(全5値)・壁(全3種・重ね掛け)が、それぞれ `critical`・`attacker.status`・`field.weather`・
   `field.terrain`・`field.defenderScreens` にだけ写り、値が変わるたびに計算1回、変わらない操作は0回
   (`testCriticalMaps...`・`testBurnMaps...`・`testEveryWeather...`・`testEveryTerrain...`・`testDefenderScreens...`)。
3. ランクは -6..+6 に丸め、境界を越える操作は計算しない。表示は「A +6」「A -6」「A ±0」(`testRankClampsToContractRangeAndIgnoresNoOpChanges`・
   `CalcConditionsDomainTests.testRankLabel`)。
4. 技の分類を変えるとステッパーの対象が A ↔ C に変わり、もう一方のランクは保持され両方送られる。変化技は A
   (`testRankStepperFollowsMoveCategoryAndKeepsEachStatsRank`・`testStatusMoveEditsAttackRank`)。
5. 特性は「指定なし」か攻撃側の `abilities` の ID だけを受け付け、種族の変更・入れ替えで新しい種族に無い特性は「指定なし」に戻る
   (`testAbilityPickerAcceptsOnlyUnspecifiedOrSpeciesAbilities`・`testAttackerSpeciesChangeKeepsAbilityOnlyWhenNewSpeciesHasIt`・`testSwapKeepsConditionsAndDropsAbilityTheNewAttackerLacks`)。
6. 条件は防御側・プリセット・持ち物・比較・入れ替えで消えない(`testConditionsPersistAcross...`・`testSwapKeeps...`)。
7. 構築の個体: 保存された特性を選択状態にし、画面のやけど・ランク・場を適用する。特性の上書き・呼び直しで戻る。構築 → プリセットで特性は nil
   (`testTeamIndividualKeepsSavedAbilityAndScreenConditionsApply`・`testPresetAfterTeamDropsTeamAbilityButKeepsOtherConditions`)。
8. 条件の変更は `scheduleLatest` で先行の計算を cancel し、先の条件は次の要求に残る(`testConditionChangeCancelsThePreviousInFlightCalcAndAccumulates`)。
9. `APIPokeCalcService` は場・急所・ランク・やけど・特性を openapi の綴りで送る(`APIPokeCalcServiceConditionsTests` の3件)。モックは条件付きでも同じ形の行を返す
   (`MockPokeCalcServiceConditionsTests`)。ドメインの enum は openapi と同じ値集合、並びと文言は3章の表(`CalcConditionsDomainTests`)。
10. XCUITest: 「詳細」は既定で閉じていて、開くと全入力が既定の選択状態で出る。急所・天候・壁・ランクの操作で選択状態・表示が変わり、
    結果の行は出続ける(`CalcConditionsUITests` の2件)。
11. 既存のテストは1行も変えずに通る。`swift test` と `make ios-test` がすべて成功する。

### 6. accessibilityIdentifier(XCUITest が参照する)

| 要素 | identifier | 備考 |
|---|---|---|
| 折りたたみの開閉ボタン | `calcConditionsToggle` | ラベルは「詳細」。既定は閉じる(View の `@State`) |
| 開いた中身のコンテナ | `calcConditionsPanel` | 閉じている間は存在しない |
| 急所 / やけど | `calcCondition-critical` / `calcCondition-burn` | on のとき `.isSelected` |
| 天候のピル | `calcWeather-<rawValue>` | 選択中だけ `.isSelected` |
| フィールドのピル | `calcTerrain-<rawValue>` | 同上 |
| 防御側の壁 | `calcDefenderScreen-<rawValue>` | on のとき `.isSelected` |
| ランクの値 | `calcAttackerRankValue` | `attackerRankText` をそのまま出す |
| ランクの −/+ | `calcAttackerRankDecrement` / `calcAttackerRankIncrement` | SwiftUI の `Stepper` ではなく2つのボタン(XCUITest がロケールに依存しないため)。±6 で無効化 |
| 特性の選択 | `calcAttackerAbilityPicker` | `Menu`。項目は「指定なし」+ 種族の特性名 |

### 7. spec 時点のテスト結果(2026-09-25)

`swift test`(`ios/PokeCalcKit`): 436件実行、23テストが失敗。失敗はすべて本タスクの新しいテスト
(`CalcViewModelConditionsTests` 16件すべて、`CalcConditionsDomainTests` の並び・文言5件、`APIPokeCalcServiceConditionsTests` 3件中2件。
`testConditionChangeCancels...` は仮実装では要求が来ないため待機の上限(約19秒)で失敗する)。既存のテストの失敗は0件。
仮実装のままで通る新テスト(enum の値集合・既定の場・`Screens` の補助・既定の場を送らない・モック)は回帰の番として残す。
`CalcConditionsUITests` はアプリのビルド(`build-for-testing`)が通ることだけ確認し、実行はしていない(View が未実装なので失敗する)。

### 8. 実装者への注意(`TODO(implementer)` を検索すればコード上の該当箇所が見つかる)

- `CalcViewModel.swift`: 「計算条件」の MARK の仮実装を2章どおりに埋める。各 setter は値が変わらなければ何もしない。変わったら
  `let token = beginInput()` → 状態を更新 → `await recalculate(token: token)`(既存の `selectAttackerItem` と同じ形)。
  `buildRequest` で `critical: isCritical`・`field: FieldState(weather:terrain:defenderScreens:)`、攻撃側の `ranks`・`status`・`abilityId` を
  **プリセット経路と構築経路の両方**に当てる(`individualForRequest` は ranks/status を落とすので、組み立てた後に上書きする)。
- 特性の選択肢は `reloadAttackerMoveOptions` で `detail.abilities` を `attackerAbilityOptions` に入れ、そこで「新しい種族に無ければ nil」を行う
  (token・`detail.key` の確認の後。古い応答で書き換えない)。`selectTeamIndividual` は反映後に保存された特性を入れ、
  `selectAttackerPreset` は直前が `.team` のときだけ nil に戻す。
- `DisplayLabels.swift`: 3章の表の文言にする。`RankLabel` の文字は `AttackerPreset` の private な `statLetter(for:)` を共有できる形にしてよい
  (同じ対応を2か所に書かない)。
- `APIPokeCalcService.swift`: `generatedBulkCalcRequest` に `field` を足す(4章。既定なら nil)。ドメイン → 生成型の enum 写像は既存の
  `generatedFormat` 等と同じく網羅 switch で書く。
- View(`CalcScreenView` とその部品): 技セレクタの下・読み込み表示の上に「詳細」の折りたたみを置く。6章の identifier を付け、操作は
  `viewModel.scheduleLatest { await $0.setCritical(...) }` の形で呼ぶ。開閉にアニメーションを付けるなら操作時のみ(常時動くものは入れない)。
  色はトークンだけ(選択中のピルは `attackerPreset-*` と同じ表現)。
- 既存テスト・新しいテストの期待値は変えない。`api/openapi.yaml`・`Generated/`・`engine/`・`web/`・`services/` は触らない。
- 完了条件は5章。`swift test` と `make ios-test` を実行し、結果をこの章の後ろに追記する。plan.md の P6-13 にチェックを付ける。

### 9. 実装結果(2026-09-25)

8章どおりに実装。`CalcViewModel.swift`(各 setter・`buildRequest`・`reloadAttackerMoveOptions`・`selectTeamIndividual`・
`selectAttackerPreset`)・`DisplayLabels.swift`(3章の文言。`RankLabel` は `AttackerPreset.statLetter(for:)` を
`private` から module-internal に変えて共有)・`DomainTypes.swift`(`RankLimits.min/max` を新設し、ランクのクランプと
View の ±6 無効化が同じ値を参照するようにした)・`APIPokeCalcService.swift`(`generatedBulkCalcRequest` に `field` を追加。
既定 `FieldState()` は省略)・View(新規 `ios/PokeCalc/CalcConditionsSection.swift`、`CalcScreenView.swift` に組み込み、
`CalcScreenStyleHelpers.swift` に `rankValueMinWidth` を追加)。`TODO(implementer)` はすべて解消。

実装中に見つけて直した不具合(テストが検出。テスト自体は変えていない):

1. **accessibilityIdentifier がコンテナに飲まれる**: `conditionsPanel` に `.accessibilityElement(children: .contain)` を
   付けずに `.accessibilityIdentifier("calcConditionsPanel")` を付けていたため、横スクロール(天候・フィールド)の
   中でない子(急所・やけど・ランク・特性・壁)の identifier がすべて `"calcConditionsPanel"` に上書きされていた
   (XCUITest の要素ダンプで実際に確認)。`AttackerCardView`/`DefenderCardView` と同じ `.contain` を足して解消。
2. **`calcAttackerRankIncrement` にスクロールで届かない**: `CalcConditionsUITests` の `scrollUntilHittable` は前方
   (`swipeUp`)にしかスクロールしない。ランク・特性を「見出しを上・内容を下」の2行で積むと「詳細」パネル全体が
   縦に伸び、天候・壁を操作したあとランク(パネルの上のほう)へ戻れなくなった。5つの小見出し行(ランク・特性・
   天候・フィールド・壁)を「見出し+内容を1行」にまとめる `sectionRow(_:content:)` にして、既定の文字サイズでの
   パネルの高さを抑えて解消(3章の並び順・6章の identifier は変えていない)。

批評(critic)PASS。あわせて、批評指摘で以下も対応:

- ランクの −/+ ボタンに VoiceOver 用の `.accessibilityLabel`(`CalcConditionLabels.rankDecrement`/`rankIncrement`)を追加
  (アイコンだけのボタンなので、システムの自動読み上げ〈「削除」「追加」〉に頼らない)。
- Dynamic Type: `sectionRow` が `dynamicTypeSize.isAccessibilitySize` を見て、アクセシビリティ域の文字サイズでは
  見出しを内容の上に積む2行レイアウトに切り替える(既定サイズは1行のまま。上記2の「パネルを1行に詰めた」変更と
  両立させるため、切り替えは既定サイズの挙動・identifier を変えずに行った)。
- `attackerRank`/`setAttackerRank` の `switch attackerRankStat` の `default:` 分岐(`StatKey` の hp/def/spd/spe を
  網羅するためだけの分岐で実際には来ない)にコメントを足した。

`swift test`(`ios/PokeCalcKit`): 436件実行、0失敗。
`make ios-test`(シミュレータ): `ios-lint`・`ios-gen-check`・`ios-check-request-limits` OK、`ios-test-unit` 449件成功・0失敗、
`ios-test-ui` 22件成功・0失敗ですべて green(終了コード0)。
実装中の検証では、`ios-test-ui` の1回目の全件実行で `CalcConditionsUITests` の2件だけが上記の不具合で失敗し(他の20件は
green)、単体再実行(`-only-testing:PokeCalcUITests/CalcConditionsUITests`)で2件とも成功したのち、修正を確認した。
その後の `make ios-test` 再実行は、別レーンの並行セッションが同じシミュレータ(`iPhone 18 Pro`)で `xcodebuild test` を
実行中だったため `ios-test-ui` がブートストラップの時点で failed(`Early unexpected exit... signal kill`)になったことが
2回あった(`ps aux` で相手のプロセスを確認。P6-12(7章)と同じ既知の環境要因)。並行実行が無い状態で `make ios-test` を
実行し、上記の 449/449・22/22 の全件成功を確認した。
- 追記(メインセッション、2026-09-25): アクセシビリティの文字サイズで見出しを上の行へ移したとき、`sectionRow` の
  内容がそのまま `VStack` の子になり、ランクの −/値/+ が1つずつ縦に並んでいた。内容を `HStack` で包んで横並びを保つよう
  修正し、accessibility-extra-large のスクリーンショットで確認した。あわせて、最大の文字サイズ
  (accessibility-extra-extra-extra-large)では計算画面の**全体**が横にはみ出す(左端が切れる)ことを見つけた。
  この変更の前(2026-09-23)のスクリーンショットでも同じなので既存の不具合で、本節の範囲外として plan.md P6-14 に切り出す。
