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
