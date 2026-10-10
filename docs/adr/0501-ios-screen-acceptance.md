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

## P6-14 の受け入れ条件(最大の文字サイズ〈AX5〉での横はみ出し。spec-writer: 受け入れ条件とテストのみ。実装はしない)

- 日付: 2026-09-25 / 担当レーン: iOS / 関連: 本 ADR「issue #274」8章の追記(不具合の発見)、docs/plan.md P6-14、
  `CalcScreenView.swift`・`CalcScreenResults.swift`・`ReverseScreenResults.swift`

### 1. 受け入れ条件(検証可能な形)

1. AX5(`.accessibility5` = `UIContentSizeCategory.accessibilityExtraExtraExtraLarge`、起動引数
   `-UIPreferredContentSizeCategoryName UICTContentSizeCategoryAccessibilityXXXL`)で計算画面
   (`CalcScreenView`)を開いたとき、`calcBackendModeBadge`・`attackerCard`・`defenderCard`・
   `attackerPreset-*`・`attackerTeamSourceButton`・`movePicker`・`calcConditionsToggle`・
   `calcResultRow-*` を含む主要な要素すべてが、ウィンドウの `frame`(`minX >= 0` かつ
   `maxX <= window.maxX`。丸め誤差 1pt 許容)に収まる。左端が切れる(`minX < 0`)状態を再発させない。
2. 同じ条件で逆算画面(`ReverseScreenView`)・構築一覧画面(`TeamListView`)・構築編集画面
   (`TeamEditView`)を開いたときも、主要な要素(カード・プリセット・構築元行・技セレクタ・
   一覧の行・編集画面の入力)が同様にウィンドウ内に収まる。
3. 上記1・2は、はみ出す原因になっている `Text` が実際に画面へ描画された状態(計算画面は既定の
   計算結果が1行以上出た状態、逆算画面は候補が1件以上出た状態)で確認する。表示物が無い(空)状態
   だけを見て「直った」と判定しない。
4. 既定の文字サイズ(Dynamic Type 既定値)では、上記1・2の同じ要素群が今までどおりウィンドウ内に
   収まる(回帰させない)。
5. はみ出しを直す変更は、はみ出す原因の `Text`(`ResultRowView.percentRangeTextView`・`koText`、
   `ReverseCandidateCardView` の同等の `Text`)を折り返す/縮小する/縦積みにするなど、内容が
   ウィンドウ幅を超えて要求しない形にする。`accessibilityIdentifier` は変えない
   (既存 XCUITest・本 ADR の識別子表と衝突させない)。
6. 直した後も `AttackerPresetTests`・`BulkRowDisplayTests`・`CalcViewModelTests` 等の既存 XCTest、
   および `CalcScreenUITests`・`ReverseScreenUITests`・`CalcConditionsUITests`・`TeamScreenUITests`
   の既存 XCUITest がすべて成功する(数値・文言の期待値は変えない)。

### 2. 追加したテスト

`ios/PokeCalcUITests/LargeTextLayoutUITests.swift`(新規)。`POKECALC_USE_MOCK=1` + 起動引数
`-UIPreferredContentSizeCategoryName UICTContentSizeCategoryAccessibilityXXXL` で AX5 を固定し、
`cardsRow`/`ReverseScreenView.cardsRow` が `dynamicTypeSize >= .accessibility1` で縦積みに切り替わる
実装を使って起動引数が実際に効いたことも確認する(`assertAX5TookEffect`)。主要な識別子について
ウィンドウの `frame` からのはみ出しをまとめて検査し(`assertNoHorizontalOverflow`)、はみ出した要素・
その `frame`・ウィンドウの `frame` を1つの失敗メッセージに列挙する。既定サイズでの同じ検査(回帰確認)
も対にして入れた。

- `testCalcScreenNoHorizontalOverflowAtDefaultSize` / `testCalcScreenNoHorizontalOverflowAtAX5`
- `testReverseScreenNoHorizontalOverflowAtDefaultSize` / `testReverseScreenNoHorizontalOverflowAtAX5`
- `testTeamScreensNoHorizontalOverflowAtDefaultSize` / `testTeamScreensNoHorizontalOverflowAtAX5`
  (構築を1つ作って編集画面まで進めて検査する。`RootView.makeTeamStore()` が `POKECALC_USE_MOCK=1` の
  起動のたびに専用 UserDefaults suite を空にするため、他の XCUITest のデータと衝突しない)

2026-09-25 時点の実行結果(`DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer xcodebuild test
-project ios/PokeCalc.xcodeproj -scheme PokeCalc -destination 'platform=iOS Simulator,name=iPhone 18 Pro'
-only-testing:PokeCalcUITests/LargeTextLayoutUITests`): 6件中5件成功、
`testCalcScreenNoHorizontalOverflowAtAX5` のみ失敗(想定どおり。実装未修正)。既定サイズの計算画面の
回帰テストは成功しており、既定サイズでは崩れていないことを確認済み。逆算・構築の2画面は AX5 でも
成功した(§4「未確認・要フォローアップ」参照。原因パターンは共通に存在するが、モックの既定状態では
再現条件〈候補行が出る状態〉に達しない)。

テスト実装上の注意(申し送り): `ResultRowView`(`CalcScreenResults.swift`)は `glassCard()` のコンテナに
`.accessibilityElement(children: .contain)` が無く、`CalcConditionsSection.swift` の既存コメントが説明する
のと同じ理由で、`subtitleText`・`percentRangeTextView`・`koText` の3つの `Text` が個別の
`accessibilityIdentifier`(`calcResultPercent-*`・`calcResultKO-*` など)を持っているにもかかわらず、
すべて親の `calcResultRow-<id>` に「飲まれ」て同一 identifier で3件ヒットする(`.frame` を取ろうとすると
`Multiple matching elements found` で失敗した。実際に本テストの初回実行で踏んだ)。これは P6-14 のはみ出し
とは別の、識別子まわりの既存の抜け漏れ(`attackerCard`/`defenderCard`/`calcConditionsPanel` は `.contain` が
付いているので同じ問題は起きない)。本テストはこれを踏まえて同一 identifier の全件の `frame` を見る作りに
した(`matchingElements`)ので実装はここで直さなくてよいが、`.accessibilityElement(children: .contain)` を
`ResultRowView`・`ReverseCandidateCardView` にも足すと今後の XCUITest がシンプルになる(任意の改善)。

### 3. 見つかった原因(実装者への引き継ぎ。まだ直していない)

`xcrun xcresulttool` で AX5 失敗時の `frame` を実測(iPhone 18 Pro シミュレータ、ウィンドウ
`(0, 0, 402, 874)`):

```
calcBackendModeBadge : minX=-52.0  width=342.3
attackerCard          : minX=-52.5 width=507.0
defenderCard          : minX=-52.5 width=507.0
attackerPreset-none   : minX=-52.5 width=164.3
attackerPreset-aMax   : maxX=454.5 width=164.3
attackerTeamSourceButton : minX=-52.0 width=506.0
movePicker            : minX=-52.0 width=506.0
calcConditionsToggle  : minX=-52.5 width=507.0
calcResultRow-none@- [percent text] : minX=-40.0 width=482.0 label="18.0〜26.0%"
```

- ほぼ全部の要素の幅が 506〜507pt に揃っている(ウィンドウは 402pt)。かつ `minX` は揃って負
  (-52 前後)で、`maxX - window.maxX` もほぼ同じだけ超過している。これは「個々の要素が勝手に
  伸びた」のではなく、**`CalcScreenView.body` の `VStack(alignment: .leading)` そのものの幅が
  506〜507pt まで広がり**、`.frame(maxWidth: .infinity)` を使っている兄弟(カード・プリセットの
  ピル行・構築元ボタン・技セレクタ・「詳細」トグル)が全員その広がった幅いっぱいに引き伸ばされた
  結果。さらに `ScrollView`(縦専用。横スクロールを許可していない)は、非スクロール軸(横)で
  コンテンツがビューポートより大きいとき既定で**中央寄せ**する。ウィンドウ幅402・コンテンツ幅507
  なら `minX = (402 - 507) / 2 = -52.5` で、実測値と一致する。これが「左端が切れる」(=右にも
  はみ出しているが、見た目には左端が削れて見える)の直接の理由。
- `VStack` の幅が広がった発生源は `ResultRowView`(`CalcScreenResults.swift` 106〜121行・140〜151行)
  の `percentRangeTextView`(`%幅` の `Text`)と `koText` の `Text`。どちらも `.fixedSize()` が付いており、
  折り返し・縮小をせず「1行に収まる自然な幅」をそのまま親に要求する。AX5 のような巨大な文字サイズでは、
  この自然な幅がウィンドウ幅を超える(実測: percentRangeTextView だけで幅482pt)。
  `ResultRowView.body` は `ViewThatFits(in: .horizontal)` で「横並び」「縦積み」の2案を試すが(123〜137行)、
  **どちらの案にも同じ `percentRangeTextView`(`.fixedSize()`)がそのまま入っている**ため、縦積みにしても
  幅は縮まらない。両方とも収まらない場合 `ViewThatFits` は最後の案をそのまま(要求どおりの大きさで)描画する
  仕様なので、結局オーバーサイズの `Text` の幅がそのまま `ResultRowView` → 呼び出し元の `VStack` へ伝播する。
- 同じパターンが `ReverseCandidateCardView`(`ReverseScreenResults.swift` 59〜63行の
  `Text(candidate.percentRangeText)...fixedSize()`)にも存在するが、モックの逆算画面は起動直後は観測0件で
  候補行が描画されないため、本タスクの XCUITest(既定の起動直後の状態を見る)では再現しなかった。観測を
  1件追加して候補行を出した状態で同じ検査をすると、同じ原因で同様にはみ出す可能性が高い(未検証。§4)。

### 4. 推奨する直し方(実装はしていない。implementer への申し送り)

1. **本命**: `ResultRowView.percentRangeTextView` と `koText`(`ReverseCandidateCardView` の同等の `Text`
   も同様)から `.fixedSize()` を外すか、`.lineLimit(1).minimumScaleFactor(CalcScreenMetrics.compactMinimumScaleFactor)`
   に置き換える(このファイルの他の `Text` ―`subtitleText`・pill のラベルなど―が既に使っているのと同じ
   縮小パターン。design.md の「ダメージバーの物差しをそろえる」意図は保ったまま、確定数バッジや%表示が
   小さくなるだけで済む)。
2. `ViewThatFits` の2案のどちらでも `percentRangeTextView` が固定幅のままだと1で直しても効果が薄いので、
   1と揃えて直す。1で幅が縮むなら `ViewThatFits` の「横並び」案がAX5でも選ばれるようになり、縦積み案は
   より小さい文字サイズ用のままでよい。
3. 保険として、`CalcScreenView.body` の `VStack`(`ScrollView` の直下)に `.frame(maxWidth: .infinity)` や
   `containerRelativeFrame(.horizontal)` を付け、個々の子がどれだけ「自然な幅」を要求してもコンテナ自体は
   画面幅を超えないようにする案もある。ただしこれは症状(はみ出す)を隠すだけで、中身の `Text` はその幅の
   中でさらに小さく潰れるか省略記号で切れるだけになるため、1・2の「原因側」を直すほうを優先する。
4. 直したら、5章 (1)〜(3) のテストに加え、逆算画面で観測を1件追加して候補行を出した状態
   (`ReverseCandidateCardView` 側)も同じ手順で AX5 確認する(§1 の受け入れ条件2・3の対象)。

### 5. 未確認・要フォローアップ

- 本タスクの XCUITest は逆算画面・構築画面を「起動直後の既定状態」でしか AX5 検査していない
  (逆算は観測0件・候補0件、構築は一覧が空 or 編集画面に入っただけでメンバー未追加)。§3で述べたとおり、
  `ReverseCandidateCardView` にも同じ `.fixedSize()` パターンがあるため、候補が出た状態
  (観測を1件入力した状態)での AX5 検査は未実施。implementer は直す際にこのケースも確認すること。
- 構築編集画面でメンバーを1体追加した状態(`TeamEditMemberCard`)の AX5 検査も未実施。

### 6. 実装結果(implementer、2026-09-25)

§4の推奨1・2どおりに実装した。

1. `ResultRowView.percentRangeTextView`・`koText`(`CalcScreenResults.swift`)と
   `ReverseCandidateCardView` の `percentRangeText` の `Text`(`ReverseScreenResults.swift`)から
   `.fixedSize()` を外し、このファイルの他の `Text`(`subtitleText` 等)と同じ
   `.lineLimit(1).minimumScaleFactor(CalcScreenMetrics.compactMinimumScaleFactor)` に置き換えた。
   これで `ViewThatFits` の「横並び」案(`ResultRowView`)・`HStack`(`ReverseCandidateCardView`)が
   AX5 でも縮小した文字幅で収まるようになり、`CalcScreenView`/`ReverseScreenView` の `VStack` 全体が
   広がる連鎖(§3)が起きなくなった。既定サイズでは自然な幅がスケール閾値を超えないため、
   見た目(%表示の大きさ・改行なし)は変えていない。
2. `ResultRowView`・`ReverseCandidateCardView` のコンテナに `.accessibilityElement(children: .contain)`
   を追加(`CalcConditionsSection.calcConditionsPanel` と同じパターン)。§2の申し送りどおり
   `calcResultPercent-*`/`calcResultKO-*`/`reverseCandidateRange-*`/`reverseCandidateMatch-*` が
   親の `calcResultRow-*`/`reverseCandidateRow-*` に飲まれず個別要素のまま残ることを確認した。
   `accessibilityIdentifier` はどちらも変えていない。
3. `LargeTextLayoutUITests` に AX5 のケースを2件追加した(いずれも実装前に「失敗するはず」だった
   §5のフォローアップを埋める):
   - `testReverseScreenWithCandidateNoHorizontalOverflowAtAX5`:
     `reverseObservationField-0` に "12" を入力して `reverseCandidateRow-neutral@-` を実際に描画した
     状態で検査(`ReverseScreenUITests.testEnteringObservationShowsCandidatesAndPremise` と同じ操作)。
   - `testTeamEditScreenWithMemberNoHorizontalOverflowAtAX5`: 構築編集画面でメンバーを1体追加した
     状態(`TeamEditMemberCard`)を検査。member id が UUID で identifier に入るため、新設の
     `matchingElementsBeginningWith`/`assertNoHorizontalOverflowForPrefixes`(前方一致版)で
     `memberCard-`・`memberSP-`・`memberMoveSlot-` 等を検査した。
   `testCalcScreenNoHorizontalOverflowAtAX5`(既存の失敗していたテスト本体)を含め、
   `LargeTextLayoutUITests` は8件全て成功した。
4. `grep fixedSize ios/PokeCalc` で他の使用箇所も確認した。`CalcScreenResults.ChipButton`
   (横スクロールの `ScrollView(.horizontal)` の中なので、内容が広がっても外側の `VStack` 幅には
   伝播しない)、`CalcScreenCards.SpeciesHeaderMenuLabel` の名前(`fixedSize(horizontal: false,
   vertical: true)` で縦方向だけ)、`CalcScreenCards.TypeBadgeView`、`CalcConditionsSection.
   sectionRowLabel`(既定サイズの1行レイアウト限定。アクセシビリティサイズでは2行レイアウトに
   切り替わり `fixedSize()` を使わない分岐に入る)、`TeamEditMemberCard.spStepper` のラベルは、
   いずれも本タスクで追加した AX5 の XCUITest(計算画面・逆算画面〈候補あり〉・構築編集画面
   〈メンバーあり〉)で実際にはみ出しを起こさず、8件とも成功したため、追加の修正はしていない。
5. 検証: `swift test`(`ios/PokeCalcKit`)436件0失敗。
   `xcodebuild test -only-testing:PokeCalcUITests/LargeTextLayoutUITests` 8件0失敗
   (新設の2件を含む)。`make ios-test`: `ios-test-unit`・`ios-test-ui`(30件0失敗。既存22件+新設8件)・
   `ios-check-infoplist` すべて成功(終了コード0)。
6. 気づいた点(修正はしていない・要フォローアップ): `testReverseScreenWithCandidateNoHorizontalOverflowAtAX5`
   の `xcresult` に、`reverseObservationField-0` をタップした直後(候補描画前・入力前)に
   1件だけ SwiftUI のランタイム警告「Invalid frame dimension (negative or non-finite).」が記録された。
   アサーション失敗にはなっておらず(`isAssociatedWithFailure: false`)、テストは成功している。
   候補カードの描画やテキストフィールドへの入力より前(フォーカス直後)に出ているため、
   本タスクの `.fixedSize()` の直しとは無関係に見える(AX5 でのキーボード表示アニメーション周りの
   既知の SwiftUI の挙動の可能性)。原因は特定していない。

## P6-15 の受け入れ条件(P6-14 の残り3点。spec-writer: 受け入れ条件とテストのみ。実装はしない)

- 日付: 2026-09-25 / 担当レーン: iOS / 関連: 本 ADR「P6-14」、docs/plan.md P6-15、
  `CalcScreenView.swift`・`ReverseScreenView.swift`・`CalcConditionsSection.swift`・
  `CalcScreenResults.swift`

### 0. 対象

docs/plan.md P6-15 の3点。P6-14 で直していない「残り(軽微)」。

1. AX5 で攻撃側プリセットのピル「A振り(無補正)」が「A振り…」と省略される
   (`CalcScreenView.presetSegmentedRow`・`ReverseScreenView.presetSegmentedRow`/`PresetPillButton`。
   `.attacker` 側の `KnownDefenderPreset` のピルも同じ部品を使っている)。
2. `LargeTextLayoutUITests` で計算画面の「詳細」(issue #274)を開いた状態も AX5 で検査する。
3. 既定サイズで%・確定数の文字が縮んでいないことを確かめる検査(critic の任意の指摘)。

### 1. 受け入れ条件(検証可能な形)

1. AX5(`.accessibility5`)で計算画面の攻撃側プリセット3ピル(`attackerPreset-none`/`aFull`/`aMax`)、
   および逆算画面の同等のピル(既定の `.defender` 側: `reverseAttackerPreset-*`、`.attacker` 側:
   `reverseKnownDefenderPreset-*`)は、既定サイズの「画面幅いっぱいの3等分・横1行」をやめ、
   縦に積む等の形で各ピルがより広い幅を持つ(design.md の縦積みパターン。`cardsRow` が
   `dynamicTypeSize >= .accessibility1` で横並び→縦積みに切り替えているのと同じ考え方)。
   XCUITest は `Text` が省略記号で切れているかどうかを直接読めないため、次の2点を代理指標として使う
   (`assertPresetPillsStackVertically`。P6-15 タスク指示「robust, meaningful assertion」):
   - 3つのピルの `minY` が(縦積みが効いていれば)互いに 10pt 以上離れている
   - 各ピルの `frame.width` がウィンドウ幅の半分より広い(3等分〈約1/3〉のままではない)
2. 既定の文字サイズでは、上記3ピルは今までどおり横1行・3等分のままである(回帰させない。
   `assertPresetPillsSingleRow`: `minY` の差が2pt未満・各ピルの幅がウィンドウ幅の半分未満)。
3. `accessibilityIdentifier` は変えない(`attackerPreset-*`・`reverseAttackerPreset-*`・
   `reverseKnownDefenderPreset-*`。既存 XCUITest・本 ADR の識別子表と衝突させない)。
4. AX5 で計算画面の「詳細」(`calcConditionsToggle` → `calcConditionsPanel`)を開いた状態でも、
   issue #274 6章の identifier のうち horizontal スクロールの中に無いもの
   (`calcConditionsPanel`・`calcCondition-critical`/`burn`・`calcAttackerRankValue`・
   `calcAttackerRankDecrement`/`Increment`・`calcAttackerAbilityPicker`)がウィンドウの外に
   はみ出さない(`calcScreenIdentifiers` の主要要素と合わせて検査)。天候・フィールド・防御側の壁
   (`calcWeather-*`/`calcTerrain-*`/`calcDefenderScreen-*`)は `ScrollView(.horizontal)` の中に
   ある意図的な設計(`CalcConditionsSection.weatherSection` 等のコメント)なので、はみ出し検査の
   対象にしない(存在確認だけ行う。2章「テスト実装上の注意」参照)。
5. 既定の文字サイズで、%表示(`calcResultPercent-*`)が `minimumScaleFactor`(`CalcScreenMetrics.
   compactMinimumScaleFactor` = 0.7)によって不要に縮んでいない。フォントの実測 pt 値を
   ハードコードせず、「横幅に制約が無い横向き(landscape)」での同じ要素の高さを基準値として比較する
   (2章「テスト実装上の注意」参照。iPhone は landscape をサポートしているため成立する)。
6. 直した後も既存の `LargeTextLayoutUITests`(P6-14 分)・`CalcScreenUITests`・
   `ReverseScreenUITests`・`CalcConditionsUITests`・`AttackerPresetTests`・`KnownDefenderPresetTests`
   等の既存 XCTest/XCUITest がすべて成功する(数値・文言の期待値は変えない)。

### 2. 追加したテスト

`ios/PokeCalcUITests/LargeTextLayoutUITests.swift`(既存ファイルへの追加。新規ファイルは作らない)。

- `testCalcScreenAttackerPresetPillsStackVerticallyAtAX5` / `testCalcScreenAttackerPresetPillsSingleRowAtDefaultSize`
- `testReverseScreenAttackerPresetPillsStackVerticallyAtAX5` / `testReverseScreenAttackerPresetPillsSingleRowAtDefaultSize`
- `testReverseScreenKnownDefenderPresetPillsStackVerticallyAtAX5`(`.attacker` 側に切り替えてから検査)
- `testCalcScreenConditionsPanelNoHorizontalOverflowAtAX5`
- `testCalcScreenResultPercentNotShrunkAtDefaultSize`

テスト実装上の注意(申し送り):

- **ピルの縦積み判定**: 直接「省略されたか」を読む API が無いため、`assertPresetPillsStackVertically`/
  `assertPresetPillsSingleRow` という2つの共通ヘルパーを新設し、上記1章1・2の代理指標(`minY` の差・
  幅の比率)で判定する。実装が「縦積み」以外の直し方(例: `ViewThatFits` で改行、フォントをさらに
  縮小する等)を選んだ場合、この代理指標に合わない可能性がある。もし implementer が縦積み以外の
  設計にするなら、この2つのヘルパーと該当テストを合わせて見直してよい(ただし「省略されない」という
  1章の受け入れ条件そのものは変えない)。
- **「詳細」パネルの検査で踏んだ落とし穴**: 初回実装時、天候・フィールド・防御側の壁の全チップを
  `assertNoHorizontalOverflowForPrefixes` に含めたところ、`ScrollView(.horizontal)` で2番目以降の
  チップの `frame.maxX` がウィンドウ幅を大きく超えて誤って失敗した(例: `calcTerrain-*[4/5]` が
  `frame=(1177, 1006, 339, 67)` で `window=(0,0,402,874)` を大きく超える)。これは
  `ChipButton`/`CalcConditionsSection` のコメントが明言する意図的な設計(横スクロールで見せる)であり
  不具合ではない。`itemComparisonToggles` の `defenderItemToggle-*` が P6-14 の
  `calcScreenIdentifiers` に元から含まれていないのと同じ理由で、このプリフィックスは
  「はみ出し検査」の対象から外し、「存在するか」だけを確かめる形にした
  (`calcConditionsPanelScrollableChipPrefixes` のコメント参照)。実装者はこの区別(横スクロールの
  中の要素とそうでない要素)を保ったまま直してよい。
- **%表示が縮んでいないことの検査**: `.label` は元の文字列のままで見た目の縮小を反映しないため、
  「横幅に制約の無い横向き(landscape)」での同じ要素(`calcResultPercent-none@-`)の高さを基準値にし、
  縦向き(既定)の高さと比較する方式にした(`XCUIDevice.shared.orientation` を使い、
  `addTeardownBlock` で `.portrait` に戻す。他のテストに向きが持ち越されないようにするため)。
  フォントの pt 値や行高をハードコードしていないので、`TextStyleToken.resultPercent` のサイズや
  `CalcScreenMetrics.compactMinimumScaleFactor` の値が変わってもテスト自体は書き直さずに機能する。

2026-09-25 時点の実行結果(`DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer xcodebuild test
-project ios/PokeCalc.xcodeproj -scheme PokeCalc -destination 'platform=iOS Simulator,name=iPhone 18 Pro'
-only-testing:PokeCalcUITests/LargeTextLayoutUITests`): 15件中12件成功・3件失敗。

失敗3件はいずれも1章(1)の本体(想定どおり。`presetSegmentedRow` が `dynamicTypeSize` を見ていないので
現状は縦積みにならない):

```
testCalcScreenAttackerPresetPillsStackVerticallyAtAX5
  XCTAssertGreaterThan failed: ("0.0") is not greater than ("10.0")
  minYs=[762.83, 762.83, 770.17] identifiers=["attackerPreset-none", "attackerPreset-aFull", "attackerPreset-aMax"]

testReverseScreenAttackerPresetPillsStackVerticallyAtAX5
  XCTAssertGreaterThan failed: ("0.0") is not greater than ("10.0")
  minYs=[811.17, 811.17, 818.83] identifiers=["reverseAttackerPreset-none", "reverseAttackerPreset-aFull", "reverseAttackerPreset-aMax"]

testReverseScreenKnownDefenderPresetPillsStackVerticallyAtAX5
  XCTAssertGreaterThan failed: ("4.0") is not greater than ("10.0")
  minYs=[811.17, 815.17, 815.17] identifiers=["reverseKnownDefenderPreset-none", "reverseKnownDefenderPreset-max", "reverseKnownDefenderPreset-full"]
```

3件とも「3つのピルがほぼ同じ `minY`(=横1行のまま)」という、いま直っていない実際の状態どおりの
理由で失敗している(`testCalcScreenAttackerPresetPillsStackVerticallyAtAX5` の2つは `minY` が完全一致、
`KnownDefenderPreset` 側は僅差の4ptで、フォントの自然な行高差にすぎず縦積みとは呼べない)。

残り12件はすべて成功。うち次の3件は本タスクで新設し、実装前の時点で green だったもの
(実装済みの前提を壊していないことの確認・回帰の番として残す):

- `testCalcScreenAttackerPresetPillsSingleRowAtDefaultSize`(既定サイズは今までどおり1行)
- `testReverseScreenAttackerPresetPillsSingleRowAtDefaultSize`(同上)
- `testCalcScreenConditionsPanelNoHorizontalOverflowAtAX5`(1章(4)。「詳細」パネル自体は
  横スクロールの中身を除けば AX5 でもすでにはみ出していなかった)
- `testCalcScreenResultPercentNotShrunkAtDefaultSize`(1章(5)。P6-14 の実装〈`.fixedSize()` を
  `minimumScaleFactor` に置き換えた〉のおかげで、既定サイズでは landscape と同じ高さのまま
  すでに縮んでいない。回帰の番として残す)

既存の8件(P6-14 分)もすべて成功しており、回帰は無い。

### 3. 推奨する直し方(実装はしていない。implementer への申し送り)

1. **本命**: `CalcScreenView.presetSegmentedRow` と `ReverseScreenView.presetSegmentedRow`(3ケースとも)
   を、`cardsRow`/`sideSwitch` と同じ `if dynamicTypeSize >= .accessibility1 { VStack … } else { HStack … }`
   の分岐にする。`PresetPillButton`(`ReverseScreenView.swift` 私有型)自体は見た目(`Text` + Capsule)を
   変えず、呼び出し側の並べ方だけを変える形で十分なはず。
2. `.lineLimit(1).minimumScaleFactor(CalcScreenMetrics.compactMinimumScaleFactor)` はそのまま残してよい
   (縦積みで幅が増えれば、0.7倍までの縮小で「A振り(無補正)」のような長いラベルも省略されずに収まる
   可能性が大きく上がる。直接の省略検出はできないため、1章の代理指標〈幅・`minY`〉で確認する)。
3. `KnownDefenderPreset` 側(ラベルが短い)も同じ分岐に揃えることで、`AttackerPreset`/
   `KnownDefenderPreset` のどちらの側を表示していても一貫した見た目になる(`presetSegmentedRow` の
   `@ViewBuilder` の2つの `case` 両方に同じ分岐を入れる)。
4. 「詳細」パネル(`CalcConditionsSection`)は本タスクの検査で AX5 でも(横スクロールの中身を除けば)
   はみ出しが無かったため、修正は不要と見られる。ただし念のため実装者は3ピルの直し方を反映した後、
   `testCalcScreenConditionsPanelNoHorizontalOverflowAtAX5` を再実行して green のままであることを
   確認すること。
5. 既定サイズの%表示が縮んでいないことも、直した後に
   `testCalcScreenResultPercentNotShrunkAtDefaultSize` を再実行して確認すること(今回のピルの直しは
   `ResultRowView` に触れないので影響しないはずだが、`CalcScreenView.body` の `VStack` 幅の連鎖
   〈P6-14 §3〉に似た問題を新たに作らないための保険)。

### 4. `swift test` / `xcodebuild test` の実行結果(実装前・spec-writer 時点)

- `swift test`(`ios/PokeCalcKit`): 本タスクは `ios/PokeCalcKit` のソースを変更していないため未実行
  (対象外。`PokeCalcCore`/`PokeCalcDesign` のテストに影響する変更は無い)。
- `xcodebuild build-for-testing -scheme PokeCalc`: 成功(`** TEST BUILD SUCCEEDED **`)。
- `xcodebuild test -only-testing:PokeCalcUITests/LargeTextLayoutUITests`: 2章のとおり15件中12件成功・
  3件失敗(失敗3件は1章(1)本体。想定どおりの理由)。

### 5. 実装者への注意(まとめ)

- 本体の直しは `CalcScreenView.presetSegmentedRow`・`ReverseScreenView.presetSegmentedRow` の2箇所
  (3章1)。`accessibilityIdentifier` は変えないこと。
- 「詳細」パネル・%表示については本タスクの検査で既に green だったので、実装者が新たに壊さないための
  回帰テストとして扱う(3章4・5)。
- `LargeTextLayoutUITests` 以外の既存 XCTest/XCUITest(`AttackerPresetTests`・`KnownDefenderPresetTests`・
  `CalcScreenUITests.testAttackerPresetPillsFollowCatalogOrderAndDefault` 等)も、ピルの並び順・
  既定選択・`isHittable` を検査している。縦積みにしても `minXs` の昇順チェックのような既定サイズ限定の
  アサーションには影響しないはずだが、実装後に必ず全体を実行して確認すること。
- 完了条件: `xcodebuild test -only-testing:PokeCalcUITests/LargeTextLayoutUITests` の15件が全て成功。
  `make ios-test` も成功させ、結果をこの章の後ろに追記する。plan.md の P6-15 にチェックを付ける。

### 6. 実装結果(2026-09-25)

- 計算画面の `presetSegmentedRow` と逆算画面の `presetPillContainer`(攻撃側プリセット・既知の防御側プリセットの両方)を、
  `dynamicTypeSize >= .accessibility1` で縦積み、それ未満で従来の横1行にした(`cardsRow`/`sideSwitch` と同じ閾値)。identifier は不変。
- `LargeTextLayoutUITests` 15/15 成功。`make ios-test`: unit 449/449、XCUITest 37/37、スキップ 0。
- critic PASS。critic の懸念「`testCalcScreenResultPercentNotShrunkAtDefaultSize` が縮小を見逃して素通りするかもしれない」は、
  メインセッションで変異テストをして確かめた。% 表示に `.padding(.leading, 300)` を足して縦向きだけ縮ませると、縦 23.3pt / 横 33.7pt で
  テストが red になった(元に戻して確認済み)。高さの比較は縮小を検出できる。

## issue #250 の受け入れ条件(`AppConfiguration` の受理条件と ATS の実行時挙動が食い違う。spec-writer: 受け入れ条件とテストのみ。実装はしない)

- 日付: 2026-09-25 / 担当レーン: iOS / 関連: 本 ADR §5(`AppConfiguration`)、ADR-0500 §5、docs/plan.md P6-16、
  `AppConfiguration.swift`・`AppConfigurationTests.swift`(既存。変更しない)・`ios/PokeCalc-Info.plist`・
  `ios/scripts/check-infoplist.sh`

### 0. 何がずれているか(issue #250 の指摘)

`AppConfiguration`(`ios/PokeCalcKit/Sources/PokeCalcCore/AppConfiguration.swift`)は `acceptedSchemes = ["http",
"https"]` で、スキームが `http`/`https` でホストがあれば URL を無条件に受理する。一方 `ios/PokeCalc-Info.plist`・
`PokeCalc.xcodeproj` には `NSAppTransportSecurity` が無い。既定の ATS(App Transport Security)は非 TLS
(`http`)通信を拒否するため、`AppConfiguration` が受理した `http://localhost:8080` のような接続先が実行時に
通信できない(`AppConfigurationTests.testBackendSelection` の「http」ケースがまさにこの状態を正常系にしている)。
issue は案A(http を拒否してテストを直す)と案B(`NSAllowsLocalNetworking` 等で ATS 側を開ける)を挙げ、
既定案はAとしつつ「ローカル API を http で叩く開発が要るなら案B」としていた。

### 0.5 2026-09-25 の見直し(critic 指摘。IP アドレスを対象から外した)

最初の版の §1〜3 は「`NSAllowsLocalNetworking` は IP アドレスも通す」としていたが、critic
(メインセッション)のレビューで以下の指摘を受け、**IP アドレスを http の受理範囲から外す**方向に修正した:

- Apple のドキュメントの「iOS 17+, iPadOS 17+, macOS 14+」の節は「ATS no longer allows connections to IP
  addresses by default. Add individual IP addresses and CIDR ranges in the `NSExceptionDomains` dictionary」
  であり、これは「`NSAllowsLocalNetworking` があれば IP アドレスも通る」という主張の裏付けにならない。
  旧版が書いていた「`NSAllowsLocalNetworking`(または `NSExceptionDomains`)無しには通らない」という読みは、
  「`NSAllowsLocalNetworking` があれば通る」への言い換えとしては文書に無い拡大解釈だった(この読みは撤回)。
  `NSExceptionDomains` は `NSAllowsLocalNetworking` とは別のキーで、個々の IP アドレス/CIDR
  範囲を明示的に列挙する仕組みであり、今回のタスクの範囲外(実装しない)。
- メインセッションが iOS 27 シミュレータ + ローカル Python サーバーで実験した: `NSAllowsLocalNetworking =
  true` のとき、`http://localhost` / `http://127.0.0.1` / Mac の LAN の IPv4 アドレス(値は記録しない)/
  `http://<Mac のホスト名>.local` はいずれもサーバーに到達した。しかし **`NSAppTransportSecurity` キー自体を
  一切書かない対照実験でも** `localhost`/`127.0.0.1`/LAN の IPv4 アドレスへの到達に成功しており、この
  シミュレータ環境では ATS そのものが(少なくともこれらのホストに対して)効いていない可能性が高い。
  したがって、この実験は「`NSAllowsLocalNetworking` が IP アドレスを通す」ことの確認にはならない
  (対照群と処置群が区別できていないため。実機での確認は「人間の確認が必要なこと」として plan.md に残す)。
- 上記2点により、IP アドレスを http で受理する根拠が無くなったため、**保守的に読んで IP アドレスは
  http では拒否する**方向に変更した(範囲を Apple の文書の記述〈非修飾ドメイン・`.local` ドメインの2つ〉に
  絞る。ループバック/プライベート帯だから安全、という判断もしない)。

以下の §1〜3 はこの見直し後の内容。

### 1. 判断(A/B の間。ADR として採用する理由。2026-09-25 見直し後)

**採用**: `https` は任意のホストで受理する。`http` は ATS が `NSAllowsLocalNetworking`
(`ios/PokeCalc-Info.plist` の `NSAppTransportSecurity` に追加)で実際に通す範囲(**非修飾ホスト名と
`.local` ドメインのみ。IP アドレスは含めない**。§0.5・§2)だけを `AppConfiguration` も受理する。
それ以外の `http`(IP アドレス・通常の公開ドメイン)は `AppConfigurationError` にする。

理由:

1. **受理条件 == 実行時の挙動**(issue の「達成する結果」そのもの)。案Aだけだと `make dev`
   (`http://localhost:8080`)を使ったシミュレータでの開発ループが `AppConfiguration` の時点で塞がれる
   (`docs/runbooks/ios.md` のモック起動だけになり、`docs/plan.md` P6-16 のようなローカル API 接続の確認が
   iOS レーンで出来なくなる)。案Bを「`NSAllowsArbitraryLoads`」で丸ごと開けると、`AppConfiguration` が
   `http://pokecalc-attacker.example` のような通常の公開ドメインへの `http` も受理してしまい、受理条件が
   ATS の実際の挙動より緩くなる(ATS はそれを拒否しないので矛盾は起きないが、平文通信を野放図に許す設定を
   コードに残すことになり、望ましくない)。
2. **`NSAllowsLocalNetworking` の対象範囲は Apple のドキュメントに明記されている**(下記2章)ので、
   `AppConfiguration` 側の判定をその範囲と1対1に鏡写しにできる。ドキュメントに明記が無い IP アドレスは
   保守的に対象外とする(§0.5)。「案Bだが無制限には広げない」という issue の既定案の裏にある懸念
   (平文を野放図に許さない)も満たす。
3. 既存の `AppConfigurationTests.testBackendSelection` の「http」ケース(`http://localhost:8080` → API)は
   `localhost` が非修飾ホスト名(後述)なので、この判断でも受理され続ける。**既存テストは変更しない**
   (タスク指示の禁止事項どおり)。`AppConfigurationTests.swift` に http の IP アドレスを使うケースは無い
   (implementer が確認済み)ので、この見直しで既存テストが壊れることも無い。

### 2. Apple ドキュメントによる `NSAllowsLocalNetworking` の範囲(判断の根拠。実装が鏡写しにする対象)

`developer.apple.com/documentation/bundleresources/information-property-list/nsapptransportsecurity/
nsallowslocalnetworking` の Discussion(2026-09-25 に確認)より:

> The `NSAllowsLocalNetworking` key controls whether App Transport Security (ATS) allows your app to connect to:
> - Unqualified domains
> - `.local` domains
> - IP addresses using IPv4 or IPv6

かつ「iOS 17+, iPadOS 17+, macOS 14+」の節(原文どおり引用):

> In iOS 17, iPadOS 17, and macOS 14, ATS no longer allows connections to IP addresses
> by default. Add individual IP addresses and CIDR ranges in the `NSExceptionDomains` dictionary.

**§0.5 の見直し**: 上の Discussion の箇条書きだけを読むと IP アドレスも `NSAllowsLocalNetworking` の対象に
見えるが、「iOS 17+」の節はそれと矛盾するように読める内容(IP アドレスへの接続は既定で許可されず、許可するには
`NSExceptionDomains` に個別に追加する必要がある)を書いている。本アプリの対象は iOS 27(Package.swift の
`platforms: [.iOS(.v27), .macOS(.v27)]`)であり iOS 17+ の節の対象なので、**IP アドレスは
`NSAllowsLocalNetworking` だけでは通らない前提で扱う**(`NSExceptionDomains` は今回実装しない。
個々の IP を列挙する仕組みで、本アプリの「開発時にローカル API を叩く」用途に対して具体的な IP を
ハードコードすることになり、ドメイン規約(IP・ホストをハードコードしない)にも合わない)。

範囲は次の2つ(**IP アドレスは含めない**。旧版はループバック/プライベート帯に限らず IP アドレス全般を
含めていたが、§0.5 の理由で撤回した):

1. **非修飾ホスト名**(unqualified domain): ホスト名にドットが無い(例 `localhost`・`pokecalc-router`)。
   FQDN のルート記法(末尾ドット。例 `localhost.`)はドットを含むため非修飾ホスト名として扱わない
   (「ドットが無い」という記述をそのまま読んだ結果。ルートドットを特別扱いする根拠が文書に無いため)。
2. **`.local` ドメイン**(Bonjour。例 `foo.local`。大文字小文字は区別しない)。

上記以外(IP アドレス〈IPv4/IPv6〉、およびドットを含み `.local` でも無いホスト名 = 通常の公開ドメイン。
例 `127.0.0.1`・`::1`・`example.com`・`pokecalc.example.invalid`)は `NSAllowsLocalNetworking` の対象外。
これは `http` では実行時に拒否される(はずな)ので、`AppConfiguration` でも受理してはいけない。

### 3. 受け入れ条件(検証可能な形。2026-09-25 見直し後)

1. `https://` の URL はホストを問わず(IP・非修飾・`.local`・通常の公開ドメインいずれも)これまでどおり
   `.api` として受理する(既存 `AppConfigurationTests` を壊さない)。
2. `http://` の URL は、ホストが次のいずれかのときだけ `.api` として受理する:
   - ドットを含まない(非修飾ホスト名。例 `localhost`・`pokecalc-router`。末尾ドットが付くと対象外)
   - `.local` で終わる(大文字小文字を区別しない。例 `foo.local`・`FOO.LOCAL`)
3. 上記2に当てはまらない `http://` の URL(**IP アドレス〈IPv4/IPv6。例 `127.0.0.1`・`::1`〉を含む**。
   ドットを含み `.local` でも無いホスト。例 `http://example.com`)は `AppConfigurationError` を投げる。
   `reason` は `http` であることと ATS(`NSAllowsLocalNetworking`)が理由であることが分かる文言にする
   (下記テストが `"http"` と `"ATS"`/`"NSAllowsLocalNetworking"` の文字列を含むことを検査する)。
   IP アドレスかどうかの判定は文字列の形(ドット・コロンの数)ではなく `inet_pton` 相当
   (`IPv4Address`/`IPv6Address`〈Network フレームワーク〉)で行う。`URL.host` は
   `http://[::1]:8080` のようなブラケット付き IPv6 リテラルからブラケットを外した `::1` を返す
   〈`swift -e` で確認済み〉ので、追加のブラケット除去は不要。
4. `ios/PokeCalc-Info.plist` に `NSAppTransportSecurity` → `NSAllowsLocalNetworking = true` を追加する。
   `NSAllowsArbitraryLoads` は追加しない(1章2の理由)。`NSExceptionDomains` も追加しない(2章の理由)。
5. `ios/scripts/check-infoplist.sh`(`make ios-check-infoplist` → `make ios-test` から実行)が、ビルド成果物の
   `PokeCalc.app/Info.plist` に `NSAppTransportSecurity.NSAllowsLocalNetworking = true` があり、
   `NSAppTransportSecurity.NSAllowsArbitraryLoads` が無いことを確かめる(本タスクで検査を追加済み。
   4 の実装が入るまでは赤くなるのが期待どおり)。
6. 既存の `AppConfigurationTests`(`testKeyNamesMatchADR`・`testBackendSelection`・`testInvalidBaseURLIsAnError`)
   はすべて成功し続ける(変更しない。`testBackendSelection` の「http」「モック強制は不正な URL より優先」
   ケースは `localhost`/`not a url` を使っており、この判断でも従来どおりの結果になる)。
7. **実機での確認は本タスクの範囲外・人間の確認が必要なこと**として扱う(§0.5 のシミュレータ実験は
   ATS 自体が効いているか確認できず結論が出せなかったため。CLAUDE.md「人間の確認が必要なこと」に相当する
   実機検証は自動で進めない)。

### 4. 追加したテスト(spec-writer 時点。§0.5・§7 で IP アドレスの扱いを見直した後の版は §7 参照)

`ios/PokeCalcKit/Tests/PokeCalcCoreTests/AppConfigurationATSTests.swift`(新規ファイル。既存の
`AppConfigurationTests.swift` は変更していない)。

- `testHTTPLocalhostIsAccepted` / `testHTTPLoopbackIPv4IsAccepted` / `testHTTPLoopbackIPv6IsAccepted`
- `testHTTPDotLocalHostIsAccepted` / `testHTTPDotLocalHostIsAcceptedCaseInsensitive`
- `testHTTPUnqualifiedHostnameIsAccepted`
- `testHTTPArbitraryIPAddressIsAccepted`(2章の「ループバック/プライベート帯に限らない」ことの直接確認)
- `testHTTPPublicHostIsRejected`(`reason` に `"http"` と `"ATS"`/`"NSAllowsLocalNetworking"` を含むことも検査)
- `testHTTPPublicHostWithPathIsRejected` / `testHTTPSubdomainOfDotLocalLikeButNotLocalIsRejected`
  (`local.example.com` は `.local` **では終わらない**ので拒否対象。「`.local` を含む」ではなく
  「`.local` で終わる」判定にすることの回帰止め)
- `testHTTPSAnyHostIsAccepted`(https は `example.com`・`127.0.0.1`・`localhost` 等どれでも受理する回帰確認)

2026-09-25 時点の実行結果(`DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test`、
`ios/PokeCalcKit` ルート): `PokeCalcCoreTests` 447件中3件失敗(想定どおり。すべて3章3の「拒否すべき」テスト)。

```
testHTTPPublicHostIsRejected
  XCTAssertThrowsError failed: did not throw an error
testHTTPPublicHostWithPathIsRejected
  XCTAssertThrowsError failed: did not throw an error
testHTTPSubdomainOfDotLocalLikeButNotLocalIsRejected
  XCTAssertThrowsError failed: did not throw an error
```

残り444件(既存の `AppConfigurationTests` を含む)はすべて成功しており、既存テストへの影響は無い。

`ios/scripts/check-infoplist.sh` は本タスクで ATS キーの検査を追加した(シェル構文は `bash -n` で確認済み。
`plutil -extract` の挙動は一時ファイルで検証済み: キーが無いと空文字列を返し、スクリプトは
`<キー無し>` としてエラーメッセージに出す)。実際の `xcodebuild` を伴う実行(`make ios-check-infoplist`)は
`ios/PokeCalc-Info.plist` に `NSAllowsLocalNetworking` が無い現状では失敗する想定のため、
本タスク(spec-writer)では実行していない(実装後に implementer が実行して確認する)。

### 5. 実装者への注意

- 変更してよいのは `AppConfiguration.swift`(判定ロジック)と `ios/PokeCalc-Info.plist`(ATS キーの追加)。
  `AppConfigurationTests.swift`・`AppConfigurationATSTests.swift` は変更しないこと(後者はこのタスクの
  受け入れ条件そのもの)。
- ホストが IP アドレスかどうかの判定は、IPv4 は `inet_pton(AF_INET, ...)` 相当、IPv6 は
  `inet_pton(AF_INET6, ...)` 相当(Swift では `IPv4Address`/`IPv6Address`〈Network フレームワーク〉、
  または `inet_pton` を `Darwin`/`Glibc` 経由で直接呼ぶ、のどちらでもよい。`engine/` ではなく
  `ios/PokeCalcKit` 側のコードなので絶対ルール2〈engine を純粋に保つ〉の対象外)。文字列を `.` や `:` の
  個数で判定するような簡易正規表現は誤判定(例 `1.2.3` のような不完全な IP や `2001:db8::1` のような
  短縮 IPv6 を取りこぼす)の余地があるため避けること。
- 「非修飾ホスト名」の判定は「ホスト文字列にドット(`.`)が1つも無い」で足りる(`localhost`・
  `pokecalc-router` はドット無し、`foo.local`・`example.com` はドット有り)。IPv6 アドレスは `:` を含み
  `.` を含まない場合があるため(例 `::1`)、判定の順序は「IP アドレスか」を先に見てから「非修飾ホスト名か」
  を見るなど、IPv6 アドレスが誤って「非修飾ホスト名」に分類されても実害は無い実装にする
  (どちらに転んでも1章の範囲内〈受理〉になるため。逆に IPv4 のドットを含むアドレスが誤って
  「非修飾ホスト名でない」と判定されて拒否されないよう、IP アドレス判定を独立して行うこと)。
- `.local` 判定は大文字小文字を無視する(`url.host?.lowercased().hasSuffix(".local")`)。
- エラーメッセージ(`AppConfigurationError.reason`)は既存の `testInvalidBaseURLIsAnError` の文言パターン
  (`"\(Self.apiBaseURLInfoKey) が不正な URL: \(trimmed)"` 等)に合わせつつ、3章3のテストが検査する
  `"http"` と `"ATS"`(または `"NSAllowsLocalNetworking"`)を含める。例:
  `"\(Self.apiBaseURLInfoKey) は http でホストが ATS(NSAllowsLocalNetworking)の対象外: \(trimmed)"`。
- `ios/PokeCalc-Info.plist` への追加は plist の `<dict>` に `NSAppTransportSecurity` キーとその値の
  `<dict>` に `NSAllowsLocalNetworking` → `<true/>` を足すだけ(既存の `PokeCalcAPIBaseURL` キーはそのまま)。
  `NSAllowsArbitraryLoads` は追加しないこと(3章4・critic が指摘するはず)。
- 完了条件: `swift test`(`ios/PokeCalcKit`)で `AppConfigurationATSTests` を含む全件成功、
  `make ios-check-infoplist`(または `make ios-test`)成功、`make ios-test` 全体成功。
  結果をこの章の後ろに「### 6. 実装結果」として追記し、`docs/plan.md` の P6-16 にチェックを付ける。

### 6. 実装結果(implementer, 2026-09-25)

変更したのは §5 で指定された2ファイルのみ(`AppConfigurationTests.swift`・`AppConfigurationATSTests.swift` は
変更していない)。

- `ios/PokeCalcKit/Sources/PokeCalcCore/AppConfiguration.swift`
  - `http` スキームのとき、`isAllowedByNSAllowsLocalNetworking(host:)` で受理範囲を判定する処理を
    `init` に追加した。判定順は §5 の指示どおり「IP アドレスか」を先に見て、次に `.local`
    (`host.lowercased().hasSuffix(".local")`)、最後に「ドットを含まない(非修飾ホスト名)」。
  - IP アドレス判定は `Network` フレームワークの `IPv4Address(_:)`/`IPv6Address(_:)`(`inet_pton` 相当)を
    使い、文字列のドット・コロンの数による簡易判定は行っていない。
  - 拒否時の `AppConfigurationError.reason` は
    `"\(apiBaseURLInfoKey) は http でホストが ATS(NSAllowsLocalNetworking)の対象外: \(trimmed)"`
    (§5 の例文どおり。`"http"` と `"ATS"`/`"NSAllowsLocalNetworking"` の両方を含む)。
  - 冒頭のドキュメントコメントの表を、https は常に受理・http は ATS 範囲のみ受理・それ以外の http は
    エラー、の3行に分けて更新した。
- `ios/PokeCalc-Info.plist`
  - `NSAppTransportSecurity` → `NSAllowsLocalNetworking` = `true` を追加。`NSAllowsArbitraryLoads` は
    追加していない。

検証結果:

- `cd ios/PokeCalcKit && DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test`:
  `PokeCalcCoreTests` 447件中 447件成功(0失敗)。`AppConfigurationATSTests` の12件(§4)を含め全件成功。
  §4 に記録された spec-writer 時点の3件の失敗(`testHTTPPublicHostIsRejected`・
  `testHTTPPublicHostWithPathIsRejected`・`testHTTPSubdomainOfDotLocalLikeButNotLocalIsRejected`)は解消した。
- リポジトリルートで `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer make ios-test`: 成功
  (`TEST SUCCEEDED`、終了コード0)。`ios-lint`・`ios-gen-check`・`ios-check-request-limits`・
  `ios-test-unit`・`ios-test-ui`(全37件成功・失敗0)・`ios-check-infoplist` の全ステップが通った。
  `ios-check-infoplist` の出力:
  ```
  ios-check-infoplist: Info.plist に PokeCalcAPIBaseURL = https://pokecalc-check.example.invalid が入っている
  ios-check-infoplist: NSAppTransportSecurity.NSAllowsLocalNetworking = true・NSAllowsArbitraryLoads 無し
  ```
  (3章5・4章末の「4 の実装が入るまでは赤くなる」が解消し、期待どおり緑になったことを確認)。

### 7. 実装結果の訂正(critic 指摘対応。IP アドレスを http の受理範囲から外した。2026-09-25)

§6 まではメインセッション(critic)のレビュー前の実装で、IP アドレスの http を受理していた。
critic のレビュー(§0.5 に詳細)を受けて、**IP アドレスは http で拒否する**方向に修正した。

- `ios/PokeCalcKit/Sources/PokeCalcCore/AppConfiguration.swift`
  - `isAllowedByNSAllowsLocalNetworking(host:)` の IP アドレス判定を「受理」から「拒否」に反転
    (`isIPAddress(host)` が真なら `return false`)。IPv6 アドレス(`::1` 等)がドット無しの
    「非修飾ホスト名」に誤って分類されないよう、IP アドレス判定は引き続き最初に行う。
  - 非修飾ホスト名の判定について、末尾ドット(`localhost.` のような FQDN のルート記法)はドットを含むため
    非修飾扱いにしないことをコメントに明記(§2 の判断を反映。ロジック自体は元から `host.contains(".")`
    で対応済みだったため、コード変更は無くコメントのみ追加)。
  - 冒頭のドキュメントコメントの表と `init` 内のコメントを、IP アドレスが受理範囲から外れたことが分かるように
    更新した。
- `ios/PokeCalcKit/Tests/PokeCalcCoreTests/AppConfigurationATSTests.swift`(このタスクの受け入れ条件そのもの
  なので変更可。`AppConfigurationTests.swift` は変更していない。事前に grep で確認: 同ファイルに http と
  IP アドレスの組み合わせのケースは無く、この訂正で既存テストが壊れる心配は無かった)。
  - `testHTTPLoopbackIPv4IsAccepted` → `testHTTPLoopbackIPv4IsRejected`、
    `testHTTPLoopbackIPv6IsAccepted` → `testHTTPLoopbackIPv6IsRejected`、
    `testHTTPArbitraryIPAddressIsAccepted` → `testHTTPArbitraryIPAddressIsRejected` に変更し、
    いずれも `reason` に `"http"` と `"ATS"`/`"NSAllowsLocalNetworking"` を含むことを検査する
    共通アサーション `assertRejectedForATS(_:)` を使うようにした。
  - `testHTTPTrailingDotHostIsRejected` を新規追加(`http://localhost.:8080` は拒否。§2 の末尾ドットの
    判断の回帰止め)。`URL(string: "http://localhost.:8080")!.host` が `"localhost."` を返すことは
    `swift -e` で事前確認済み。
  - ヘッダーのドキュメントコメントを §0.5・§1・§2 の内容に合わせて書き直した。
  - `ios/PokeCalc-Info.plist`・`ios/scripts/check-infoplist.sh` は変更していない(IP アドレスの扱いの変更は
    `AppConfiguration.swift` 側の判定だけの問題で、ATS キー自体〈`NSAllowsLocalNetworking`〉は
    IP アドレス以外〈非修飾ホスト名・`.local`〉のために引き続き必要)。

検証結果(2026-09-25、訂正後):

- `cd ios/PokeCalcKit && DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test`:
  `PokeCalcCoreTests` 448件中 448件成功(0失敗)。`AppConfigurationATSTests` は12件(§4 の11件 +
  `testHTTPTrailingDotHostIsRejected` の1件)全件成功、既存の `AppConfigurationTests`(3件)も成功。
- リポジトリルートで `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer make ios-test`: 成功
  (`TEST SUCCEEDED`、終了コード0)。`ios-test-unit`・`ios-test-ui`(全37件成功・失敗0)・
  `ios-check-infoplist`(`NSAllowsLocalNetworking = true`・`NSAllowsArbitraryLoads` 無しを確認)を含む
  全ステップが通った。

実機での確認(§3 の7)は本タスクでは行っていない。人間が実機で `http://<Mac の .local 名>:8080` や
`http://<開発機のホスト名>:8080`(非修飾ホスト名・`.local` のケース)が実際に通ることを確認し、
IP アドレス(`http://192.168.x.x:8080` 等)は実機でも拒否されるべき(コード側は拒否する。ATS 側も
拒否するはずだが未確認)ことを合わせて確認するとよい。


## P6-17 の受け入れ条件(未対応の印〈ADR-0123〉の表示。spec-writer: 受け入れ条件とテストのみ。実装はしない)

- 日付: 2026-09-25 / 担当レーン: iOS / 関連: ADR-0123 §2・§6・§7、ADR-0121(機構13種)、docs/plan.md P6-17、
  DECISIONS.md 2026-09-25「issue #271/#270 の API レーン担当分」・同日「未対応の印の表示(文言・置き場所)を決めた」
- 背景: API は `CalcResult`(`BulkCalcRow.result` を含む)と `ReverseCandidate` に `unsupported: UnsupportedMark[]`
  (必須・印なしは `[]`)を返す。`UnsupportedMark = {target, reason, id}`。数値は通常の式のままで、
  「この結果は正確でない可能性がある」ことだけを示す。iOS はこれまで写像で捨てていた(生成型のデコードは通る)。
  Web レーンはまだ表示しないので、iOS が先に表示し、文言・置き場所を DECISIONS.md で Web に渡す。

### 1. ドメイン型と写像

1. `PokeCalcCore` に `UnsupportedTarget`(5値)・`UnsupportedReason`(15値)・`UnsupportedMark{target, reason, id}` を置き、
   `CalcResult.unsupported`・`ReverseCandidate.unsupported`(`[UnsupportedMark]`)を持たせる。両 init の引数は
   既定 `[]`(既存の呼び出し側・テストを変えない)。
2. enum の値集合は openapi と一致する。**同期の検査は既存の型と同じ `DomainTypesTests` 方式**(生成型の
   `Components.Schemas.UnsupportedMark.TargetPayload/ReasonPayload.allCases` と rawValue の集合を比べる XCTest)
   を採る。生成型は api/openapi.yaml から作られるので、yaml を `#filePath` で読む方式・ホスト側スクリプトより
   簡単で、他の enum と揃う(`check-request-limits.sh` は `maxItems` の数値用で、enum には使っていない)。
3. `APIPokeCalcService` は `CalcResult`(一括計算の各行も同じ関数)・`ReverseCandidate` の `unsupported` を、
   並べ替えず・落とさずに写す(並びはサーバーが ADR-0123 §2 の順で決める)。target/reason の写像は網羅
   `switch`(`default` なし)。
4. **契約に無い値(将来の追加)**: 生成型は `@frozen` の String enum で未知の値の受け皿が無いため、
   `unsupported` に知らない target/reason が1つでもあると応答全体のデコードが失敗し、`PokeCalcError(code: client_decode)`
   になる(クラッシュはしない。画面は既存のエラー表示)。判断: 生成物(`Generated/`)と生成設定はレーン外なので
   変えない。値の追加は契約の変更なので、API レーンが enum を足したら iOS は再生成とドメインの enum の追加を
   同じ変更で行う(再生成すると 2 の同期テストが落ち、3 の網羅 switch がコンパイルエラーになって気付ける)。
   この挙動は `UnsupportedMarkDomainTests.testUnknown*IsDecodeErrorNotCrash` で固定する。

### 2. 日本語ラベル(`DisplayLabels.swift` の `UnsupportedMarkLabel` の1か所。Web も同じ語)

| target | 表示 | | reason | 表示 |
|---|---|---|---|---|
| `move` | 技 | | `multi_hit` | 多段技 |
| `attacker_item` | 攻撃側の持ち物 | | `fixed_damage` | 固定ダメージ |
| `attacker_ability` | 攻撃側の特性 | | `ohko` | 一撃必殺 |
| `defender_item` | 防御側の持ち物 | | `variable_power` | 威力が変化 |
| `defender_ability` | 防御側の特性 | | `alt_offense_stat` | 攻撃に使う能力値が通常と違う |
| | | | `alt_defense_stat` | 防御に使う能力値が通常と違う |
| | | | `always_crit` | 必ず急所 |
| | | | `ignore_defense_ranks` | 防御側のランク変化を無視 |
| | | | `type_change` | タイプが変化 |
| | | | `effectiveness_change` | 相性の求め方が通常と違う |
| | | | `priority_change` | 優先度が変化 |
| | | | `field_specific` | 天候・フィールドで変化 |
| | | | `move_specific` | 技固有の効果 |
| | | | `zero_power` | 威力が技の処理で決まる |
| | | | `unsupported_effect` | 効果を計算に反映していない |

- 印1つの文言: `<対象>「<名前>」(<理由>)`(例 `技「テストわざX」(多段技)`)。理由が `unsupported_effect`
  (持ち物・特性)のときは対象名で意味が通るので `(…)` を付けない(例 `防御側の持ち物「テストどうぐD」`)。
- 名前はマスタから引く(`UnsupportedMarkNames`): `move` → 技の辞書、`*_item` → 持ち物の一覧、`*_ability` → 特性の一覧。
  辞書は target の種類で選ぶ。見つからない ID は ID のまま出す(黙って消さない。`BulkRowDisplay.itemLabel` と同じ)。
  計算画面は `moveDictionary`・`itemOptions`・`attackerAbilityOptions`、逆算画面は技の辞書と `itemOptions`
  (逆算画面は特性の一覧を持たないので、特性の印は ID のまま。必要になったら足す)。
- 理由の文言は15個すべて違う(画面で区別できるように)。

### 3. 置き場所と注記の文言(純粋な helper `UnsupportedNotice.swift`)

1. `UnsupportedPlacement(lists)`: 全行(逆算は全候補)が持つ印を `common`(1行目の順)、各行の残りを `perEntry`
   (行の順・印の順を保つ)にする。行が1つならすべて `common`、行が0なら両方空。同じ行の中の重複は1つにまとめる。
   - 結果: 技の印・攻撃側の持ち物/特性の印は全行に付くので**結果の上に1回**、持ち物の比較で増えた行の
     防御側の持ち物の印は**その行だけ**。逆算も同じ(技の印は上に1回、相手の持ち物候補の印はその候補のカード)。
   - 「技の印は target=move だから上」のような target による決め打ちはしない(全行にあるかどうかだけで決める)。
     全行にあるものを行ごとに繰り返すと同じ文言が5〜10回並ぶため。
2. 文言(`UnsupportedNoticeText`。印が無ければ nil = 何も描かない):
   - 結果の上(`summary`): `この結果は正確でない可能性があります(未対応: <印>、<印>)`
   - 行・候補カード(`rowNote`): `未対応: <印>、<印>`
   - 区切りは `、`。
3. `BulkResultDisplay(result:items:names:)` が行(`BulkRowDisplay.unsupportedNote`)と `unsupportedNotice` を作る。
   `ReverseResultDisplay(result:items:names:)` が `unsupportedNotice` と `ReverseCandidateDisplay.unsupportedNote` を作る。
   既存の `BulkRowDisplay(row:items:)`・`ReverseCandidateDisplay(candidate:stat:items:)` は注記 nil のまま(既定引数)。
4. ViewModel: `CalcViewModel.unsupportedNotice` は `rows` と同時に書き換え、印の無い応答・失敗(`rows` を空にする時)で
   nil に戻す。`ReverseViewModel.result` は `ReverseResultDisplay` ごと差し替えるので同じ性質になる。

### 4. モック(`MockPokeCalcService`)

- 既定の技・持ち物では印なし(`[]`)。既存の画面・XCUITest は変わらない。
- 印はフィクスチャから決める(起動の切り替え〈環境変数〉は増やさない):
  - `moves.json` の任意の `mechanisms`(`MasterMove.mechanisms` と同じ値)。新しい架空の技 `test-move-multi-hit`
    「テストわざれんぞく」(物理・`["multi_hit"]`)を足し、9001-000 の learnset の**末尾**に入れた(既定の技は変えない)。
  - `items.json` の任意の `unsupportedEffect: true`。新しい架空の持ち物 `test-item-unsupported`「テストどうぐみたいおう」を
    **2番目**に足した(既存の XCUITest が使う `test-item-berry` は先頭のまま)。
- 規則: (1) 技の `mechanisms` を昇順で `target: move`(変化技には付けない。未知の値は `fixtureInvalid`)。
  (2) 未対応の持ち物を攻撃側が持てば `attacker_item`(全行)、防御側が持てば `defender_item`(一括計算は
  その行の `itemId`、1対1 は `defender.itemId`)。逆算は side で割り当てる(side=defender: 既知 = 攻撃側・候補 = 防御側、
  side=attacker: 既知 = 防御側・候補 = 攻撃側)。(3) 並びは ADR-0123 §2(技 → 攻撃側の持ち物 → 防御側の持ち物)。
  印の条件(急所・天候等。ADR-0123 §3)は再現しない(モックは計算しない方針のまま)。

### 5. 表示と accessibilityIdentifier(View。XCUITest が参照する)

- 見た目: design.md に「未対応」の指定は無いので、既存の補足文(`reverseMyItemLimitHint` 等)と同じ
  `TextStyleToken.caption` + `ColorToken.textSecondary`。警告色(danger)・タイプ色は使わない(色を持つのはタイプだけ。
  数値は通常の式の目安として正しく出ており、エラーではないため)。アイコンを付けるなら装飾扱い(`accessibilityHidden`)。
  常時動くアニメーションは付けない。折り返して全文を出す(`lineLimit` を付けない。AX5 でも横にはみ出さない)。
- 読み上げ: 注記の `Text` をそのまま1要素にする(ラベル = 表示文言)。
- identifier:

| identifier | 場所 |
|---|---|
| `calcUnsupportedNotice` | 計算画面。結果の行の**上**(全行共通の印がある時だけ) |
| `calcResultUnsupported-<row id>` | 計算画面の行(`calcResultRow-<row id>` の中。その行だけの印がある時だけ) |
| `reverseUnsupportedNotice` | 逆算画面。候補カードの**上**(全候補共通の印がある時だけ) |
| `reverseCandidateUnsupported-<candidate id>` | 逆算の候補カード(`reverseCandidateRow-<id>` の中) |

### 6. 受け入れ条件(検証可能な形)

1. `UnsupportedTarget`/`UnsupportedReason` の値集合が openapi と一致する(`UnsupportedMarkDomainTests`)。
2. API の応答の `unsupported` が、1対1・一括計算の各行・逆算の各候補で、順序どおり欠けずにドメインへ写る。
   契約の全 target・全 reason が同じ rawValue に写る。契約に無い値は `client_decode` になる(クラッシュしない)。
3. 対象5種・理由15種の日本語ラベルが2章の表どおり(`UnsupportedNoticeTests`)。
4. 置き場所と文言が3章どおり(純粋 helper・`BulkResultDisplay`・`ReverseResultDisplay` のテスト)。
5. 計算・逆算の ViewModel が名前をマスタから引いて注記を出し、印の無い応答・失敗で注記を消す
   (`UnsupportedNoticeViewModelTests`)。
6. モックは既定で印なし、4章の規則で印を付ける(`MockPokeCalcServiceUnsupportedTests`)。
7. XCUITest(`UnsupportedMarksUITests`): 既定では注記が無い / 多段技で `calcUnsupportedNotice` が結果の上に1回
   (行には無い)/ 未対応の持ち物の比較でその行だけ `calcResultUnsupported-*` / 逆算で `reverseUnsupportedNotice`
   が候補の上。文言の数値は検査しない。
8. 既存の XCTest・XCUITest の期待値は変えない(既存テストの編集なし)。

### 7. 追加したテスト(spec 時点)

- `ios/PokeCalcKit/Tests/PokeCalcCoreTests/UnsupportedMarkDomainTests.swift`(契約の enum との同期・写像・未知の値)
- `ios/PokeCalcKit/Tests/PokeCalcCoreTests/UnsupportedNoticeTests.swift`(ラベル・名前・置き場所・文言・結果全体の整形)
- `ios/PokeCalcKit/Tests/PokeCalcCoreTests/UnsupportedNoticeViewModelTests.swift`(計算・逆算の ViewModel)
- `ios/PokeCalcKit/Tests/PokeCalcCoreTests/MockPokeCalcServiceUnsupportedTests.swift`(モック)
- `ios/PokeCalcUITests/UnsupportedMarksUITests.swift`(XCUITest 4件)

`swift test`(spec 時点): 490 件中 27 件が失敗(すべて上の新しいテスト。既存テストは全件成功)。
通る新テストは、置き場所が無い場合・未知の値のデコード失敗・契約の enum 同期・既定値など、
スケルトンの時点で既に満たしているもの。

### 8. 実装者への注意(`TODO(implementer` を検索すると該当箇所が見つかる)

- 型・引数・プロパティ・フィクスチャ(JSON と `MockFixtures` の任意項目)は spec で追加済み。実装するのは
  `UnsupportedMarkLabel`(2つの網羅 switch と `text`)、`UnsupportedMarkNames`、`UnsupportedPlacement`、
  `UnsupportedNoticeText`、`BulkResultDisplay`、`ReverseResultDisplay` の注記、`APIPokeCalcService` の写像、
  `MockPokeCalcService` の印、`CalcViewModel.performCalc`/`handleInputFailure`、`ReverseViewModel` の `names:`、
  View(`CalcScreenResults.swift`・`ReverseScreenResults.swift`)の注記と5章の identifier。
- 行の `.accessibilityElement(children: .contain)` の中に注記の `Text` を置けば `calcResultUnsupported-*` は個別の要素のまま
  見つかる(P6-14 §2 と同じ)。
- `LargeTextLayoutUITests` の AX5 はみ出し検査の対象に新しい identifier を足すかは任意(足すなら既存の配列を変えずに
  別のテストで)。
- `swift test` に加え、`make ios-test`(XCUITest を含む)で既存 37 件 + 新しい4件が通ることを確かめる。

### 9. 実装結果(implementer: 2026-09-25)

- 8章の TODO(implementer) をすべて実装。`UnsupportedMarkLabel.targetName`/`reasonName`/`text` は2章の表どおりの
  網羅 switch。`UnsupportedMarkNames.init(moves:items:abilities:)`/`name(for:)` は target の種類で辞書を選び、
  無ければ `mark.id`。`UnsupportedPlacement` は行ごとに重複除去してから、全行(候補)に共通する印を `common`
  (1行目の順)、残りを `perEntry` にする(3章の規則)。`UnsupportedNoticeText.summary`/`rowNote` は印が空なら
  nil。`BulkResultDisplay`/`ReverseResultDisplay` は `UnsupportedPlacement` を使って行・候補ごとの注記を作る。
  `APIPokeCalcService` は `Components.Schemas.UnsupportedMark.TargetPayload`/`ReasonPayload` を網羅 switch
  (`default` なし)でドメインへ写す。`MockPokeCalcService` は技の `mechanisms` を昇順で `move` の印、
  `unsupportedEffect: true` の持ち物を持つ側に応じて `attacker_item`/`defender_item`(逆算は既知側・候補側を
  side で target に割り当てる)。`CalcViewModel.performCalc` は `BulkResultDisplay` を使い、
  `handleInputFailure` で `unsupportedNotice` も nil に戻す。`ReverseViewModel` は `names:` にマスタの技・持ち物
  (特性の一覧は無いので空)を渡す。View は `calcUnsupportedNotice`/`calcResultUnsupported-<id>`/
  `reverseUnsupportedNotice`/`reverseCandidateUnsupported-<id>` を5章どおりに追加(行・候補カードの
  `.accessibilityElement(children: .contain)` の中に置いた)。
- 既存テストは1つも編集していない(テストを弱めた・削除した箇所は無い)。`LargeTextLayoutUITests` の AX5
  はみ出し検査への追加はしなかった(8章「任意」)。
- 結果: `cd ios/PokeCalcKit && swift test` は 490 件全件成功(新しい37件を含む。スケルトン時点の27件失敗はすべて解消)。
  `make ios-test` は unit 503 件・XCUITest 41 件(新しい `UnsupportedMarksUITests` 4件を含む)・
  Info.plist 検査すべて成功。`swift build` も警告なしで成功。

### 10. critic FAIL への対応(implementer: 2026-09-25)

- critic 指摘1(ラベルの誤解): 「特殊」はポケモンの文脈でダメージ計算の特殊技分類(とくしゅ)を指すため、
  `alt_offense_stat`/`alt_defense_stat`/`effectiveness_change` の文言に使うと誤読される。2章の表・
  `UnsupportedMarkLabel.reasonName`・`UnsupportedNoticeTests.expectedReasonNames`・DECISIONS.md
  (2026-09-25「未対応の印の表示」)を次の3件に修正: `alt_offense_stat`=「攻撃に使う能力値が通常と違う」、
  `alt_defense_stat`=「防御に使う能力値が通常と違う」、`effectiveness_change`=「相性の求め方が通常と違う」。
  `ignore_defense_ranks` も「防御ランクを無視」→「防御側のランク変化を無視」(どちらの防御側のランクか明確にする)。
  15種の文言は引き続きすべて異なる(`testReasonNamesAreDistinct` で固定)。DECISIONS.md の「spec-writer 段階。
  実装は P6-17 の implementer」という古い記述も「implementer で実装済み」に更新した。
- critic 指摘2(`UnsupportedPlacement` のテスト漏れ): 既存のテストは「1行目のマークが全行分正しく絞り込まれているか」
  を、1行目がちょうど最小集合であるケースでしか確かめていなかったため、`common = first`(絞り込みをしない実装)
  でも全テストが通ってしまっていた。`testPlacementDropsMarkOnlyPresentInFirstRow`
  (`[[moveMark, defenderItem], [moveMark]]` → `common == [moveMark]`)と `testPlacementCommonOrderFollowsFirstRow`
  (2行目の並びが違っても `common` は1行目の順)を追加。`UnsupportedNotice.swift` の `commonMarks` を一時的に
  `first`(フィルタなし)に変えて red になることを確認(`testPlacementDropsMarkOnlyPresentInFirstRow` が失敗、
  他は影響なし)、直後に元の実装へ戻した。
- critic 指摘3(逆算の既知側の印のテスト漏れ): 候補側の持ち物の target(`testReverseMarksCandidateItemBySide`)は
  検査していたが、既知側(自分)の持ち物の target は未検査だった。`testReverseMarksKnownSideItemByRole`
  (side=defender → 既知=攻撃側 → `attacker_item`、side=attacker → 既知=防御側 → `defender_item`)を追加。
  `MockPokeCalcService.swift` の `knownTarget` の三項演算子を一時的に反転させて red になることを確認
  (新テストだけ失敗、`testReverseMarksCandidateItemBySide` は無傷)、直後に元の実装へ戻した。
- 結果: `cd ios/PokeCalcKit && swift test` は 493 件全件成功(spec 時点の490件 + 新規3件〈`UnsupportedPlacement`の
  テスト2件・逆算の既知側 target のテスト1件〉)。`make ios-test`(リポジトリルート)は
  `ios-test-unit: 全 506 件 / 成功 506 / 失敗 0`・`ios-test-ui: 全 41 件 / 成功 41 / 失敗 0`
  (`UnsupportedMarksUITests` 4件を含む)・`ios-check-infoplist` すべて成功(exit code 0)。既存テストは
  1つも編集していない。

## P6-18 の受け入れ条件(issue #328「このアプリについて」画面。spec-writer: 受け入れ条件とテストのみ。実装はしない)

- 日付: 2026-09-26 / 担当レーン: iOS / 関連: docs/adr/0002-master-data-source.md「確定した方針 / 責務の分離」・§2(候補比較とライセンス)、
  docs/ai-shared/DECISIONS.md 2026-09-25「ユーザー決定 4 件(GitOps の範囲・API の入口・AI の権限・公開)」#328、
  同ファイル本タスクの新規エントリ「P6-18」、docs/plan.md P6-18
- 背景: issue #328(ユーザー決定、2026-09-25)は「非公開・私的利用のまま。LICENSE は置かない。README に明記し、
  **アプリ内に第三者データの出典と非公式の表示を入れる**」というもの。Web レーンとの合意により、**iOS が既定の
  文言を決めて DECISIONS.md に書き、Web はそれに従う**(このタスクの依頼)。データ計算そのものではなく、
  画面に固定文言を1つ表示するだけの機能なので `api/openapi.yaml` の変更は無い。

### 1. 画面の置き場所

- ルート画面(`RootView.swift`)から到達できる、控えめな入口にする(design.md に「このアプリについて」画面の
  指定は無いため implementer の判断)。既存の `.principal` ツールバー位置は「PokeCalc」の見出しで埋まっているため、
  推奨: `.toolbar` の `.topBarTrailing`(または `.bottomBar`)に「i」アイコン等の `ToolbarItem` を1つ追加し、
  `NavigationLink(value: AboutScreenRoute())` で押す(既存の `CalcScreenRoute`/`ReverseScreenRoute`/
  `TeamListScreenRoute` と同じ、値ベースの `navigationDestination(for:)` パターン。`RootView` は
  `NavigationStack` を1つしか持たないため `path` を増やす必要はない)。
- ボタン本体は `openAboutScreen` の accessibilityIdentifier を持つ(アイコンだけのボタンでも読み上げの名前を
  持たせる。design.md「入力のラベル」と同じ考え方。`SF Symbol` の `info.circle` などを想定するが指定はしない)。
- 画面自体のコンテナに `aboutScreen` の accessibilityIdentifier を付ける(他画面の `calcScreen`/`reverseScreen`/
  `teamListScreen` と同じ命名規則)。プッシュ(`navigationDestination`)・シートのどちらでもよい(implementer 判断)。
- 見た目: design.md の `ColorToken`/`TextStyleToken`/`SpacingToken` をそのまま使う。**常時アニメーションは
  付けない**(CLAUDE.md ドメイン規約)。ダークモードはトークンを使えば自動で対応する。

### 2. 非公式の注記(既定文言。DECISIONS.md に確定として記録する)

> このアプリは個人が私的に使うための非公式ツールです。任天堂・クリーチャーズ・ゲームフリーク・株式会社ポケモンとは
> 関係ありません。ポケモン・Pokémon および関連する名称は各社の商標です。

- `PokeCalcCore.AboutText.unofficialNotice` に1か所だけ持つ(`DisplayLabels.swift` と同じ理由でコードに置く。
  マスタ(pokedex)には無い、表示専用の固定文言)。
- 表示は折り返し(`lineLimit` を付けない。長文でも AX5 で横にはみ出さない。P6-14/P6-17 と同じ方針)。
  identifier: `aboutUnofficialNotice`。

### 3. データの出典一覧(ADR-0002 の責務分離表に基づく。実際にシステムが使っているものだけ)

ADR-0002「確定した方針 / 責務の分離」表と一致させる。名称・ライセンスは同 ADR に書かれている範囲を超えて
断定しない(PokeAPI は README にデータ自体の利用条件の明記がないため、ライセンス名を書かない。
Pokémon HOME・Pokémon Champions の公式情報も同様にオープンソースライセンスの対象ではないので書かない)。

| # | 用途(title) | 出典・ライセンス(detail) | ADR-0002 の対応箇所 |
|---|---|---|---|
| 1 | ダメージ計算の検証 | `@smogon/calc`(MIT License) | 「責務の分離」表・ダメージ計算の oracle、§2 候補A |
| 2 | ポケモン・技・習得技の照合 | Pokémon Showdown(MIT License) | 「責務の分離」表・データ照合、§2 候補B |
| 3 | 日本語名・図鑑番号 | PokeAPI | 「責務の分離」表・日本語名、§2 候補C(ライセンス表記なし) |
| 4 | 使用可能なポケモン等の基準 | Pokémon HOME・Pokémon Champions の公式情報 | 「責務の分離」表・使用可能集合(レギュレーション) |

- `PokeCalcCore.AboutText.dataSources: [AboutText.DataSource]`(`title`/`detail` の2フィールド。上表の順)に持つ。
- 出典を1件追加・削除するときは、この ADR の表・`AboutText.dataSources`・DECISIONS.md の3箇所を同時に直す
  (`AboutTextTests` が件数・文言を固定するので、直し忘れは `swift test` で気付ける)。
- 表示は一覧(`List`/`VStack` どちらでもよい)。各行に `aboutDataSource-<index>`(0始まり、上表の順)の
  identifier を持たせる。

### 4. 受け入れ条件(検証可能な形)

1. `AboutText.unofficialNotice` が非空で「非公式」を含み、2章の確定文言と完全一致する(`AboutTextTests`)。
2. `AboutText.dataSources` がちょうど4件で、3章の表の `title`/`detail` と順序どおり一致する。PokeAPI・
   Pokémon HOME を含む項目には「License」という語を書かない(断定しない。`AboutTextTests`)。
3. ルート画面に `openAboutScreen` の入口があり、押すと `aboutScreen` が開く(`AboutScreenUITests`)。
4. `aboutScreen` の中に `aboutUnofficialNotice` と、4件の `aboutDataSource-<index>`(0〜3)がすべて見える
   (`AboutScreenUITests`)。
5. AX5(最大の文字サイズ)でも `aboutUnofficialNotice`・`aboutDataSource-*` が横にはみ出さない
   (`LargeTextLayoutUITests.testAboutScreenNoHorizontalOverflowAtAX5`。P6-14/P6-17 と同じ検査粒度で、
   文言の数値は検査しない)。
6. 既存の XCTest・XCUITest の期待値は変えない(既存テストの編集なし)。

### 5. 追加したテスト(spec 時点)

- `ios/PokeCalcKit/Sources/PokeCalcCore/AboutText.swift`(型と `TODO(implementer` プレースホルダ。空文字列・
  空配列を返すので、コンパイルは通るが2〜4章の受け入れ条件は満たさない)
- `ios/PokeCalcKit/Tests/PokeCalcCoreTests/AboutTextTests.swift`(文言・出典一覧の非空・件数・完全一致・
  キーワード・ライセンスを断定しないことの検査。7件)
- `ios/PokeCalcUITests/AboutScreenUITests.swift`(ルートから開く・注記と出典が見える。2件)
- `ios/PokeCalcUITests/LargeTextLayoutUITests.swift` に `testAboutScreenNoHorizontalOverflowAtAX5` を追加
  (既存テストは1つも編集していない。新規メソッドの追加のみ)

`swift test`(spec 時点、`cd ios/PokeCalcKit && DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test`):
500 件中 9 件が失敗(すべて新規 `AboutTextTests` の中。7件のテストメソッドのうち `testDataSourcesDoNotInventLicensesForUnlicensedSources`
だけは `dataSources` が空でも成立する〈vacuously true〉ため成功する。既存 493 件は1つも編集しておらず全件成功)。
XCUITest(`AboutScreenUITests` 2件・`LargeTextLayoutUITests.testAboutScreenNoHorizontalOverflowAtAX5`)は
`openAboutScreen`/`aboutScreen` 等の identifier が未実装のため実行すれば失敗するが、ビルド自体は通ることを
`xcodebuild build-for-testing -project ios/PokeCalc.xcodeproj -scheme PokeCalc -destination "platform=iOS Simulator,name=iPhone 18 Pro"`
の `** TEST BUILD SUCCEEDED **` で確認済み(シミュレータでの実行は implementer 側で `make ios-test` により行う)。

### 6. 実装者への注意(`TODO(implementer` を検索すると該当箇所が見つかる)

- `ios/PokeCalcKit/Sources/PokeCalcCore/AboutText.swift` の `unofficialNotice`(2章の文言をそのまま)と
  `dataSources`(3章の表を4件そのまま)を埋める。型・フィールドは spec で追加済み。
- `ios/PokeCalc/RootView.swift` に `.toolbar` の `ToolbarItem`(`openAboutScreen`)と `AboutScreenRoute` の
  `navigationDestination(for:)` を追加する(1章)。
- 新規 View(例 `ios/PokeCalc/AboutView.swift`)を作り、`aboutScreen`・`aboutUnofficialNotice`・
  `aboutDataSource-<index>` の identifier を3章の順に付ける。折り返し・ダークモードは design.md のトークンを
  使えば自動で満たされる。
- `swift test`(`ios/PokeCalcKit`)と `make ios-test`(リポジトリルート。XCUITest を含む)の両方が全件成功する
  ことを確認する。

### 7. 実装結果(implementer: 2026-09-26)

- `AboutText.unofficialNotice`/`dataSources` を2〜3章のとおりそのまま埋めた(TODO は解消)。
- `RootView.swift`: `.toolbar` の `.topBarTrailing` に `NavigationLink(value: AboutScreenRoute())`(SF Symbol
  `info.circle`、identifier `openAboutScreen`、`accessibilityLabel("このアプリについて")`)を追加。
  `AboutScreenRoute`(値のみの `Hashable`)を既存の `CalcScreenRoute` 等と同じパターンで定義し、
  `.navigationDestination(for: AboutScreenRoute.self)` で `AboutView()` に遷移する。
- 新規 `ios/PokeCalc/AboutView.swift`: `ScrollView` + `VStack` で非公式の注記(`aboutUnofficialNotice`)と
  データの出典一覧(`aboutDataSource-0`〜`3`)を表示。両方とも `glassCard()` の角丸カードに乗せ、`Text` は
  `lineLimit` を付けず `.fixedSize(horizontal: false, vertical: true)` で折り返す。色・フォント・余白は
  `ColorToken`/`TextStyleToken`/`SpacingToken` のみ使用しアニメーションは付けていない。各出典行は
  `CalcScreenResults.swift` の `calcResultRow-*` と同じ理由で `.accessibilityElement(children: .contain)` を
  付け、identifier 検査時に同一 identifier の要素が複数見つからないようにした。
- 検証: `cd ios/PokeCalcKit && swift test` は500件全件成功(新規 `AboutTextTests` 7件含む)。
  `make ios-test`(リポジトリルート)は unit 513件・XCUITest 44件すべて成功
  (`AboutScreenUITests` 2件・`LargeTextLayoutUITests.testAboutScreenNoHorizontalOverflowAtAX5` を含む。
  `ios-lint`/`ios-gen-check`/`ios-check-request-limits`/`ios-check-infoplist` も成功)。既存テストは1つも
  編集していない。

## P6-19 の受け入れ条件(issue #272 の iOS 側: 防御側・相手の特性の選択と、特性で分かれた行・候補の表示。spec-writer: 受け入れ条件とテストのみ。実装はしない)

- 日付: 2026-09-26 / 担当レーン: iOS / 関連: issue #272、ADR-0126(engine: 特性の候補とまとめ方)、ADR-0214(契約:
  `BulkCalcRequest.defenderOverride.abilityId`・`ReverseRequest.unknownAbilityId`・`BulkCalcRow`/`ReverseCandidate` の必須
  `abilityId`・`abilityIds`)、本 ADR「issue #274」(攻撃側の特性の選択は「詳細」に実装済み)・「P6-17」(行・候補の ID と
  未対応の印の置き場所)、DECISIONS.md 2026-09-25「issue #274/#272 の防御側の詳細」・「issue 272 の API レーン担当分」、
  同ファイル本タスクの新規エントリ「P6-19」、docs/plan.md P6-19
- 背景: サーバーは防御側(逆算は相手)の特性を省略すると種族の特性(最大3件)をすべて試し、**結果が完全に同じ特性は
  1行(1候補)にまとめ、違うときだけ行(候補)を分ける**(ADR-0126 (D))。生成物は PR #414 で再生成済みだが、iOS は
  `abilityId`/`abilityIds` を写像で捨てており、行の ID(`<preset>@<item>`)・候補の ID(`<natureClass>@<item>`)に特性が
  入らないため、**分かれた行が同じ ID になり** SwiftUI の `ForEach` の ID と `UnsupportedPlacement` の行ごとの注記
  (`calcResultUnsupported-<row id>`)が衝突する(急ぎの正しさの問題)。契約の変更は無い(`api/openapi.yaml`・`Generated/` は触らない)。

### 1. ドメインと ID(`ResultEntryIdentity.swift`)

1. ドメインに足す(どれも既定値つきの init 引数。既存の呼び出し側・テストは変えない):
   `BulkCalcRequest.defenderAbilityId: String?`・`ReverseRequest.unknownAbilityId: String?`・
   `BulkCalcRow.abilityId: String?`/`abilityIds: [String]`・`ReverseCandidate.abilityId: String?`/`abilityIds: [String]`。
   契約では `abilityId` は必須・`abilityIds` は1件以上だが、ドメインの既定(nil・空)は「特性の情報なし」
   (P6-19 より前に書かれたテストの行)を表す。
2. `APIPokeCalcService`: `defenderAbilityId` があれば `defenderOverride: {abilityId}`、nil なら **`defenderOverride` 自体を送らない**
   (これまでの要求本文と同じ)。`unknownAbilityId` も nil なら送らない。応答の `abilityId`・`abilityIds` は並べ替えずに写す。
3. **ID の規則(判断: 既存の identifier を最も変えない形)**: base ID(`<preset>@<item>`・`<natureClass>@<item>`)が
   **その結果の中で1回だけ**なら base ID のまま。**2回以上**(= 特性で分かれた)なら `<base>@<代表の特性 ID>`
   (`abilityId`。nil は `-`)。それでも重なる(契約違反: 同じ base・同じ特性が2回)ときは2つ目以降に `#2`・`#3`…。
   - 採った理由: 特性が効かない大半の技・特性を指定したとき・既存のモック(特性1つの種族)では ID が今までと同じなので、
     既存の XCUITest(`calcResultRow-none@-` 等)と P6-17 の identifier を1つも変えずに済む。「常に `@<特性>` を足す」案は
     既存の XCUITest の期待値をすべて変えることになり、「サーバーが分けた組が2つ以上ある base だけ」は上と同じ意味
     (engine のまとめは全行一括なので、分かれるときは全 base が分かれる。ADR-0126 §2)だが、base ごとに数える方が
     行の並び・欠けに依存せず純粋関数で書ける。
   - ID を作るのは `BulkResultDisplay`・`ReverseResultDisplay`(結果全体を見る所)。`BulkRowDisplay`/`ReverseCandidateDisplay`
     の単体の init は `id` を渡さなければ base ID(既存の呼び出し側は変わらない)。
   - `UnsupportedPlacement` は行の並び(添字)で動くので規則は変えない。分かれた行は ID が別になるので、行だけの注記の
     identifier(`calcResultUnsupported-<row id>`)も衝突しない。

### 2. 表示(特性で分かれた行・候補の副題)

- `ResultEntryIdentity.splitBaseIDs` に入る(= 分かれた)行・候補にだけ副題 `AbilityGroupLabel.text` を付ける:
  `特性: <名前> / <名前>`(`abilityIds` の順。名前は防御側〈逆算は相手〉の `species(key:)` の `abilities` から引き、
  無い ID は ID のまま出す)。分かれていない行・候補には何も足さない(ノイズにしない)。
- 名前の出どころ: ViewModel が持つ防御側(相手)の特性の選択肢(3・4章)。**結果が分かれていて名前をまだ持っていないとき
  だけ**、ViewModel が `species(key:)` を読んでから行・候補を作り直す(計算し直さない。token が最新のときだけ)。
- 見た目: 既存の補足文と同じ `TextStyleToken.caption` + `ColorToken.textSecondary`。折り返す(`lineLimit` なし)。
  identifier は `calcResultAbility-<row id>`(`calcResultRow-<row id>` の中)・`reverseCandidateAbility-<candidate id>`
  (`reverseCandidateRow-<candidate id>` の中)。読み上げは表示文言そのまま。

### 3. 計算画面の「防御側の特性」(`CalcViewModel`)

- 「詳細」の中、「攻撃側の特性」の**すぐ下**に小見出し「防御側の特性」(`AbilityPickerLabels.defenderTitle`)の `Menu` を置く。
  項目は「指定なし」+ 防御側の特性名(`defenderAbilityOptions` の順)。identifier `calcDefenderAbilityPicker`、
  `.accessibilityValue(<選択中の名前 or 「指定なし」>)`(XCUITest が `value` で読む)。
- 状態: `defenderAbilityOptions: [Ability]`・`defenderAbilityId: String?`(nil = 指定なし。既定)。
- **選択肢の読み込み(判断)**: 起動・防御側の変更では `species(key:)` を**読まない**(既存テストが数える `species(key:)` の
  回数・添字〈manual モードの待ち合わせ〉を変えないため)。`loadDefenderAbilityOptions()` を View が「詳細」を開いている間
  `.task(id: defenderSpeciesKey)` で呼ぶ。読み済みの防御側なら何もしない。計算しない・`isLoading`/`error`/`rows` を変えない・
  失敗(キャンセル含む)は黙って空のまま・応答が届いた時点で防御側が変わっていたら反映しない。
- `selectDefenderAbility(id:)`: nil か `defenderAbilityOptions` にある ID だけを受け付け、値が変わったときだけ
  `beginInput()` → 更新 → `recalculate`(計算1回。`scheduleLatest` で先行の計算を cancel し、先の条件は次の要求に残る)。
- **リセット**: 防御側の種族の変更(`selectDefender`)・攻守入れ替え(`swapSides`)で `defenderAbilityId = nil`・
  `defenderAbilityOptions = []`(その操作の計算回数は変えない。その計算から指定なし)。攻撃側・技・プリセット・攻撃側の持ち物・
  持ち物の比較・計算条件・構築の呼び出しでは消さない。
- `buildRequest` は `defenderAbilityId` を `BulkCalcRequest.defenderAbilityId` に載せる(攻撃側の `abilityId` には混ぜない)。

### 4. 逆算画面の「相手の特性」(`ReverseViewModel`)

- 逆算画面には「詳細」が無いので、「相手の持ち物候補」の**すぐ上**に小見出し「相手の特性」(`AbilityPickerLabels.opponentTitle`)の
  `Menu` を常に出す。identifier `reverseOpponentAbilityPicker`、`.accessibilityValue` は3章と同じ。
  View は `.task(id: opponentSpeciesKey)` で `loadOpponentAbilityOptions()` を呼ぶ。
- 状態: `opponentAbilityOptions`・`opponentAbilityId`(nil = 指定なし)。読み込みの規則は3章と同じ。
- `selectOpponentAbility(id:)`: 3章と同じ受け付け。値が変わったら `beginInput()` → `recalculateIfPossible`
  (有効な観測が無ければ reverse は呼ばず、覚えるだけ)。
- **リセット**: 相手の種族の変更で nil・選択肢も空。**側の切り替え**(`selectSide`)でも nil(相手の役割が防御側 ↔ 攻撃側で
  変わり、無効・軽減の意味が変わるため。種族は同じなので選択肢は残してよい)。自分・技・プリセット・持ち物では消さない。
- `buildRequest` は `unknownAbilityId` に載せる(`known.abilityId` には混ぜない)。
- **対象外(判断)**: 自分(既知側)の特性の選択は足さない(issue #272 が求めるのは相手側の特性を届けること。自分の特性は
  構築の個体を呼べば `known.abilityId` に載る)。プリセット経路で自分の特性を選べるようにするかは plan.md の後続候補に記録する。
  計算画面の攻撃側の特性の既定(「指定なし」。issue #272 の文面の「既定は種族の先頭を表示」とは違う)は issue #274 の判断のまま変えない。

### 5. モック(`MockPokeCalcService`)

- 既存の種族(9001〜9003。特性は各1つ)は、行・候補の数・順・ID を変えない。各行・候補に `abilityId` = その特性、
  `abilityIds` = [その特性] を入れる(契約どおり常に入れる)。
- 新しい架空の種族 `9004-000`「テストモンよん」(ノーマル。特性 `test-ability-delta`「テストとくせいデルタ」、
  `test-ability-fighting-immune`「テストとくせいかくとうむこう」)を species.json の**末尾**に足した。フィクスチャの任意項目
  `nullifiesMoveType`(特性)を足し、2つ目の特性に `"fighting"` を持たせた。
- 規則: 候補の特性 = 指定があればその1件(種族に無ければ `invalid_input`)、無ければ種族の特性の先頭3件。
  「防御側の特性の `nullifiesMoveType` が技のタイプと同じ」ものはダメージ 0(`rolls` 全 0・min/max 0・%0・`ko` = 0/false/0/0。
  防御側の実数値・相性・分類はそのまま)、それ以外は既存の決め打ちの結果。結果が同じ特性を1組にまとめ(代表は先頭)、
  一括計算は プリセット → 特性の組 → 持ち物 の順に並べる。逆算は相手が防御側(side=defender)のときだけ無効が効き、
  無効の候補は `exact=false`・`mismatch` = 観測の件数・%0。候補は 性格クラス → 特性の組 → 持ち物 の順で作ってから
  `mismatch` の昇順に安定ソートする。`exactCount` は `exact` の数。
- 既存の XCUITest が使う種族・技・持ち物は変えない(新しい種族は検索シートで名前を打ったときだけ選ばれる)。

### 6. accessibilityIdentifier(XCUITest が参照する)

| 要素 | identifier | 備考 |
|---|---|---|
| 計算画面の防御側の特性 | `calcDefenderAbilityPicker` | 「詳細」の中。`Menu`。`value` = 選択中の名前 / 「指定なし」 |
| 逆算画面の相手の特性 | `reverseOpponentAbilityPicker` | 常に表示。`Menu`。`value` は同上 |
| 行の特性の副題 | `calcResultAbility-<row id>` | 特性で分かれた行だけ |
| 候補の特性の副題 | `reverseCandidateAbility-<candidate id>` | 特性で分かれた候補だけ |

行・候補の identifier(`calcResultRow-<id>` 等)の `<id>` は1章3の規則に従う(分かれた時だけ `@<特性 ID>` が付く)。

### 7. 受け入れ条件(検証可能な形)

1. **写像**: `defenderAbilityId`/`unknownAbilityId` が指定時だけ `defenderOverride.abilityId`/`unknownAbilityId` に載り、
   nil のときは送らない。応答の `abilityId`/`abilityIds` がそのままの順でドメインに写る(`APIPokeCalcServiceAbilityTests`)。
2. **ID の一意性**: 1章3の規則どおり。分かれない結果は既存の ID のまま、同じ preset/item が特性で分かれた行は
   `@<特性>` 付きで一意、契約違反の重複でも一意。未対応の印の行ごとの注記は分かれた行ごとに正しく付く
   (`AbilitySplitDisplayTests`)。
3. **副題**: 分かれた行・候補にだけ `特性: A / B`(名前が無い ID は ID のまま)。分かれていなければ nil
   (`AbilitySplitDisplayTests`)。文言「防御側の特性」「相手の特性」「指定なし」(`testPickerLabels`)。
4. **計算画面**: 既定は指定なしで、起動時の `species(key:)` は攻撃側の1回のまま。選択肢は `loadDefenderAbilityOptions()` で
   読み(計算しない・1回だけ・失敗は黙る・古い応答は捨てる)、選択は値が変わるたびに計算1回・選択肢外は無視。防御側の変更・
   入れ替えで指定なしに戻り計算回数は変わらない。他の入力では消えない。先行の計算を cancel して積み上がる。分かれた行の名前は
   防御側の `species(key:)` から引き、計算し直さない。分かれなければ `species(key:)` を読まない(`CalcViewModelDefenderAbilityTests`)。
5. **逆算画面**: 4と同じ規則(観測が無いときは選択を覚えるだけ。側の切り替えでも指定なし)(`ReverseViewModelOpponentAbilityTests`)。
6. **モック**: 5章の規則(`MockPokeCalcServiceAbilityTests`)。
7. **XCUITest**(`AbilityPickerUITests` 3件): 計算画面で防御側を 9004-000 にすると行が `…@test-ability-delta`/
   `…@test-ability-fighting-immune` に分かれて副題が出る → 「防御側の特性」で1つに決めると `none@-` に戻り副題が消える →
   防御側を変えると「指定なし」。入れ替えでも「指定なし」。逆算画面で相手を 9004-000 にして観測を入れると候補が分かれ、
   「相手の特性」で1つに決めると `neutral@-` に戻り、側を切り替えると「指定なし」。
8. 既存の XCTest・XCUITest は1行も変えずに通る。`swift test` と `make ios-test` がすべて成功する。

### 8. spec 時点のテスト結果(2026-09-26)

`swift test`(`ios/PokeCalcKit`): 546 件実行(既存 500 件 + 新規 46 件)、32 件が失敗(すべて本タスクの新しいテスト。既存テストの失敗は 0 件)。
仮実装のままで通る新しいテスト(14件: 分かれない結果の ID・特性の情報が無い行・既定が指定なし・起動で防御側を読まない・
読み込み失敗が黙る・選択肢を読む前の選択の無視・リセット系〈選択が仮実装で入らないため今は自明に通るが、実装後は
リセットの番になる〉・分かれないとき読まない・フィクスチャ)は回帰の番として残す。
`AbilityPickerUITests` は `xcodebuild build-for-testing` が通ることだけ確認し、実行はしていない(View が未実装なので失敗する)。

### 9. 実装者への注意(`TODO(implementer` を検索すると該当箇所が見つかる)

- 型・引数・プロパティ・フィクスチャ(species.json の 9004-000 と `MockFixtures.AbilityEntry.nullifiesMoveType`)は spec で
  追加済み。実装するのは: `ResultEntryIdentity.uniqueIDs`/`splitBaseIDs`、`AbilityGroupLabel.text`、`AbilityPickerLabels` の
  2つの文言、`BulkResultDisplay`/`ReverseResultDisplay` の ID と副題、`APIPokeCalcService` の写像4か所、
  `CalcViewModel`(`loadDefenderAbilityOptions`・`selectDefenderAbility`・`selectDefender`/`swapSides` のリセット・`buildRequest`・
  `performCalc` の名前)、`ReverseViewModel`(同じ4点 + `selectSide`)、`MockPokeCalcService` の一括計算・逆算、
  View(`CalcConditionsSection.swift` に防御側の特性、`ReverseScreenView.swift` に相手の特性、`CalcScreenResults.swift`・
  `ReverseScreenResults.swift` に副題)と6章の identifier。
- 名前を引くための `species(key:)` は `performCalc`/`recalculateIfPossible` の中で、行を反映した**後**に読み、token が最新のまま
  なら行を作り直す(`isLoading` を立て直さない。失敗は黙って ID のまま)。**既存テストの `species(key:)` の回数を変えないため、
  分かれていない結果では読まない**(`testUnsplitRowsDoNotFetchDefenderSpecies`)。
- 未対応の印の名前(`UnsupportedMarkNames.abilityNames`)にも防御側・相手の特性名を足すとよい(`defender_ability` の印が
  名前で出る)。必須ではない(テストは無い)。
- 行の `.accessibilityElement(children: .contain)` の中に副題の `Text` を置けば `calcResultAbility-*` が個別の要素になる
  (P6-17 と同じ)。`Menu` のラベルは `MenuLabelChip`(攻撃側の特性と同じ見た目)。
- `LargeTextLayoutUITests` の AX5 はみ出し検査への追加は任意(足すなら既存の配列を変えずに別のテストで)。
- 既存テスト・新しいテストの期待値は変えない。`api/openapi.yaml`・`Generated/`・`engine/`・`web/`・`services/` は触らない。
- 完了条件は7章。`swift test` と `make ios-test` を実行し、結果をこの章の後ろに追記する。plan.md の P6-19 にチェックを付ける。

### 10. 実装結果(2026-10-01)

- View: `CalcConditionsSection`(防御側の特性)・`ReverseScreenView`(相手の特性)・`CalcScreenResults`/`ReverseScreenResults`(副題)。
- **5章の訂正**: 新しい種族 9004-000 は species.json の**末尾ではなく 9002 と 9003 の間**に置く。既存の `MockPokeCalcServiceTests` が
  `species.last` を防御側に使っており、末尾だと既定の技(かくとう)で行が特性により倍に分かれて8件が失敗するため
  (既存テストを変えない方針を優先)。
- XCUITest `AbilityPickerUITests` の `chooseSpecies` に `previousQuery` を足した(検索欄の入力は画面ごとに保持されるため、
  同じ画面で2回目に種族を選ぶときは前の入力を消してから打つ。製品の挙動は変えない)。
- 結果: `swift test` 546件・`make ios-test` 全件成功(unit 546件・XCUITest 47件)。

## P6-7 の受け入れ条件(issue #103 の iOS 側: 「この端末のデータを削除」と ADR-0209 §8 の文言。spec-writer: 受け入れ条件とテストのみ。実装はしない)

- 日付: 2026-10-01 / 担当レーン: iOS / 関連: ADR-0209 §5(全削除 API)・§6・§7・§8(文言案)、api/openapi.yaml
  (`deleteRecordDeviceData`・`deleteTeamDeviceData`・`DeletionStatus`)、CLAUDE.md 絶対ルール5、本 ADR「P6-18」(画面の置き場所)、
  DECISIONS.md 本タスクの新規エントリ「P6-7」、docs/plan.md P6-7
- 背景: record-svc・team-svc の全削除 API は実装済みで冪等。1回の上限で `status: partial` を返し、クライアントは同じ要求を
  `completed` まで繰り返す。record と team は別 DB なので **2本とも** 呼び、**両方 `completed` になってから**完了を出す。
  Web は未実装(`web/src` に該当 UI なし。`openapi.gen.ts` の型だけ)なので、iOS が既定を決め DECISIONS.md に書く(Web が合わせる)。

### 1. 事前に分かった事実(implementer が最初に直すこと)

- **生成クライアントに該当 operation が無い**。`ios/tools/openapi-gen/openapi-generator-config.yaml` の `filter.tags` が
  `pokedex`・`calc` だけで、`record`・`team` を生成していない(`api/openapi.yaml` は変更不要・契約の変更ではない)。
  implementer が `tags` に `record`・`team` を足し `make ios-gen` で再生成する(同設定ファイルのコメントの手順どおり)。
  spec-writer は Generated に触らない指示のため行っていない。
- 端末ID・セッションIDは `APIPokeCalcService` が全操作に `identity` から付ける(`headers: .init(xDeviceId:xSessionId:)`)。
  新しい2本も同じ流儀で付ける。**削除しても端末 ID は作り直さない**(`ClientIdentity` は触らない)。
- 削除するのはサーバー側(record・team)だけ。端末内の構築(`LocalTeamStore`、UserDefaults)は消さない
  (§8 の確認文が「サーバーから削除します」であるため。DECISIONS.md に記録する)。

### 2. 画面の置き場所と文言(確定)

- 置き場所: **「このアプリについて」画面(`AboutView`)に「データの扱い」セクションを足す**(新しい画面・導線は作らない。
  設定画面が無く、説明文の置き場所として About が自然なため)。セクションは出典一覧の上に置く。
  identifier: セクション `deviceDataSection`、説明 `deviceDataExplanation-<0..2>`、ボタン `deleteDeviceDataButton`、
  状態表示 `deleteDeviceDataStatus`(`Text`。`label` が文言そのもの。操作前は出さない)、再試行ボタン `retryDeleteDeviceDataButton`
  (失敗・未完了のときだけ)、確認の了承 `confirmDeleteDeviceDataButton`・取り消し `cancelDeleteDeviceDataButton`。
- 確認は `confirmationDialog` か `alert`(どちらでもよい。message に確認文、2つのボタンに上の identifier)。**確認なしに削除しない**。
- 文言は `PokeCalcCore.DeviceDataText` の1か所だけ(View はそれを描くだけ。ハードコードしない)。

| 定数 | 文言 |
|---|---|
| `explanation[0]` | アカウントはありません。履歴・お気に入り・構築は、この端末に割り当てた ID でサーバーに保存しています。 |
| `explanation[1]` | ID が変わると(アプリを削除して入れ直したとき)、前のデータは開けなくなります。元に戻す方法はありません。 |
| `explanation[2]` | 開けなくなったデータは自動的に消えます。計算の履歴は記録から90日、お気に入りと構築は最後に使った日から18か月です。 |
| `deleteButton` | この端末のデータを削除 |
| `confirmMessage` | 履歴・お気に入り・構築をサーバーから削除します。元に戻せません。 |
| `deleting` | 削除しています… |
| `partialNotice` | まだ残っています。続けて削除します。 |
| `failure` | サーバーに届きませんでした。通信を確認してもう一度お試しください。 |
| `completed` | 削除しました。 |
| `confirmAction` / `cancelAction` / `retryButton` | 空でない文言(spec で値を固定しない。例 削除する / キャンセル / もう一度削除する) |
| `recordLabel` / `teamLabel` | 履歴・お気に入り / 構築 |
| `partlyDeleted(label:)` | `"\(label)は削除済みです。"` |

§8 の「Web: サイトデータ消去」側の括弧は iOS では「アプリを削除して入れ直したとき」にする。

### 3. 状態の仕様(`DeviceDataDeletionViewModel`。`@MainActor @Observable`、PokeCalcCore)

- 依存は `DeviceDataService`(新プロトコル。`deleteRecordDeviceData()`/`deleteTeamDeviceData()` が1回の要求ごとに
  `DeletionProgress`〈`.completed`/`.partial`〉を返す。失敗は `PokeCalcError`)。`PokeCalcService` には混ぜない(計算と切り離す)。
- `phase`: `idle` → `requestDeletion()` で `confirming`(通信しない)→ `confirmDeletion()` で `deleting` → `finished`。
  `cancelConfirmation()` で `idle`。`confirmDeletion()` は `confirming` のときだけ動く(削除中の二重起動も無視)。
- 対象ごとに `DeviceDataTargetOutcome`(`pending`/`completed`/`incomplete`/`failed(code:)`)を持つ。record と team は**独立**に呼ぶ
  (片方が失敗・未完了でももう片方は進める)。
- `partial` は同じ要求を繰り返す。対象ごとの要求は `maxRequestsPerTarget`(既定 20)回まで。超えたら `incomplete`
  (失敗ではない。無限ループしない)。通信エラーは自動で再送せず `failed(code: PokeCalcError.code)`。
- `statusMessage`: `idle`/`confirming` は nil。`deleting` 中は `deleting`、ただし直前の応答が `partial` なら `partialNotice`。
  `finished`: 両方 `completed` なら `completed` **のみ**。`incomplete` を含み失敗なしなら `partialNotice`。失敗を含めば `failure`
  (片方だけ `completed` なら `partlyDeleted(label:)` を添える。`completed` の文言は出さない)。両方失敗なら `failure` だけ。
- `retry()`: `finished` かつ `canRetry`(`completed` でない対象がある)のときだけ、`completed` でない対象だけを再度削除する
  (再確認は不要。要求の上限は新しく数える)。それ以外は何もしない。
- キャンセル(Task の cancel)は尊重する: 以後の要求を送らず、失敗にもせず(`pending` のまま)`phase` を `idle` に戻す。
  すでに `completed` の対象はそのまま残る。

### 4. モック(XCUITest 用)

- `MockDeviceDataService`(actor)。挙動は起動時の環境変数 `POKECALC_MOCK_DEVICE_DATA`(`POKECALC_USE_MOCK` と同じ流儀)で切り替える:
  なし/未知 = `immediate`(最初の要求で `completed`)、`partial` = 各対象が1回目 `partial`・2回目 `completed`、
  `fail-once` = record の1回目だけ transport エラー・以後成功(team は常に成功)。`completed` 後は冪等に `completed`。
- `AppEnvironment` の `.ready` が `DeviceDataService` も運ぶ(形は implementer の判断。`.ready` に値を足すとき既存の使用箇所を直す)。
  `APIPokeCalcService` も `DeviceDataService` に準拠させる。

### 5. 受け入れ条件(検証可能な形)

1. `DeviceDataText` が2章の表の文言と完全一致する(`DeviceDataTextTests`)。説明に Web 固有の語(ブラウザ・サイトデータ)を含まない。
2. 確認の前・取り消し後・`requestDeletion()` なしの `confirmDeletion()` では一切通信しない(`DeviceDataDeletionViewModelTests`)。
3. record と team の両方が `completed` になってから `statusMessage == completed`。`partial` は `completed` まで繰り返す。
   `deleting` 中の表示は `deleting`、`partial` を受けた後は `partialNotice`。削除中の二重 confirm で要求が増えない。
4. `partial` が続き続けても対象ごとに上限回数で止まり(`incomplete`)、もう片方は実行される。
5. record の失敗でも team を呼び、team の失敗でも record を呼ぶ。失敗は自動再送せず `failed(code:)` を運び、
   `completed` を出さない。結果は対象ごとに分けて伝える(`partlyDeleted`)。
6. `retry()` は `completed` でない対象だけを呼び、上限を数え直し、完了で `completed` を出す。再試行できないときは通信しない。
7. キャンセルで以後の要求を送らず、失敗扱い・状態表示にしない。
8. モックが3つのシナリオで 3章・4章どおりに動く(`MockDeviceDataServiceTests`)。
9. XCUITest(`DeviceDataDeletionUITests`・モック): About に説明3文とボタンが見える(確認前は状態表示なし)/ 確認で取り消せる /
   `partial` シナリオで確認の了承後に「削除しました。」になる / `fail-once` で失敗文言と再試行ボタンが出て、
   再試行後に「削除しました。」になる / 失敗した後でも計算画面が開き結果の行が出る(絶対ルール5)。
10. AX5 でセクション(説明・ボタン)が横にはみ出さない(`LargeTextLayoutUITests.testAboutScreenDeviceDataSectionNoHorizontalOverflowAtAX5`)。
11. 既存の XCTest・XCUITest は1つも編集しない(`LargeTextLayoutUITests` へのメソッド追加のみ)。

### 6. 追加したテスト(spec 時点)

- 足場(既定値付き。`TODO(implementer` を検索): `PokeCalcCore/DeviceDataDeletion.swift`(`DeviceDataService`・`DeletionProgress`・
  `DeviceDataTarget`・`DeviceDataText`〈文言は空文字列〉・`DeviceDataTargetOutcome`・`DeviceDataDeletionViewModel`〈何もしない〉)、
  `PokeCalcCore/MockDeviceDataService.swift`(常に `completed`)
- `Tests/PokeCalcCoreTests/DeviceDataTextTests.swift`(4件)・`DeviceDataDeletionViewModelTests.swift`(16件)・
  `MockDeviceDataServiceTests.swift`(5件)・`Support/StubDeviceDataService.swift`(台本・呼び出し記録・フック付きのスタブ)
- `PokeCalcUITests/DeviceDataDeletionUITests.swift`(5件)と `LargeTextLayoutUITests.testAboutScreenDeviceDataSectionNoHorizontalOverflowAtAX5`
  (既存テストは編集していない)

`swift test`(`ios/PokeCalcKit`): 572件中、新規25件のテストで 61 個のアサーションが失敗(すべて新規3ファイル内。既存547件は成功)。
`xcodebuild build-for-testing`(XCUITest 6件を含む)は `** TEST BUILD SUCCEEDED **`。XCUITest の実行は未実施(identifier 未実装のため失敗する)。

### 7. 実装者への注意

- 最初に生成設定の `tags` へ `record`・`team` を足し `make ios-gen`(1章)。その後 `APIPokeCalcService` に2本の DELETE を実装して
  `DeviceDataService` に準拠させる。**spec 時点ではこの API 写像のテストを書けていない**(生成物が無くコンパイルできないため)。
  `APIPokeCalcServiceTests` の流儀(`RecordingTransport`)で追加すること: DELETE・パス `/api/record/device-data`・`/api/team/device-data`、
  `X-Device-Id`/`X-Session-Id` ヘッダ、200 の `completed`/`partial` の写像、503(`store_unavailable`・`upstream_unavailable`)→`PokeCalcError`、
  通信不能→`transport`。
- `DeviceDataText`・`DeviceDataDeletionViewModel`・`MockDeviceDataService` の `TODO(implementer` を埋める。
  `partial` の繰り返しは `Task.isCancelled` / `CancellationError` を確認する(`URLError(.cancelled)` は API 層で既に処理されているか確認)。
- View(`AboutView` の新セクション)は `DeviceDataDeletionViewModel` を `@State` で持ち、`confirmDeletion()`/`retry()` は `Task` で呼び、
  画面が消えたら cancel する。`CalcViewModel` など計算側は `DeviceDataService` に依存させない。
- `ios/README.md` の操作説明と `docs/plan.md`(P6-7 のチェック)・DECISIONS.md(Web が合わせるための確定文言)を更新する。

### 実装結果(P6-7。implementer)

- 生成: `filter.tags` に `record`・`team` を足して `make ios-gen`(Generated が約 4000 行増えた。`api/openapi.yaml` は不変)。
  operation だけに絞る方法は無い(openapi-generator の filter は tags / paths 単位)ため、タグ単位で足した。
  既存コードは壊れず、`make ios-gen-check` は一致。
- `APIPokeCalcService` に `DeviceDataService` 準拠を同ファイルの extension で足した(`send`/`client` が private のため)。
  単体テスト `APIDeviceDataServiceTests`(新規ファイル 4 件: DELETE・パス・ヘッダ・completed/partial・503 の code・transport)。
- 確認 UI: システムの `alert` / `confirmationDialog` は、XCUITest(iOS 26 系)で同じ identifier のボタンが入れ子に2つ見え、
  `Failed to tap ... Multiple matching elements` になった(identifier を Button・label の Text のどちらに付けても同じ)。
  そのためセクション内に確認文と2ボタンのカード(`confirmationCard`)を描く形にした(確認なしに削除しない点は同じ)。
- `AppEnvironment.ready` に `deviceData` を足した(モックは `MockDeviceDataService(environment:)`、API は同じ `APIPokeCalcService`)。
  `AboutView(deviceDataService:)`(nil ならセクションを出さない)。
- **spec の文言の矛盾を解消(2026-10-01)**: `partlyDeleted(label:)` が「構築は削除しました。」だと `completed`「削除しました。」を
  部分文字列として含み、「片方だけ completed のとき completed の文言を含まない」検査(ViewModel 2件・XCUITest 1件)が成立しなかった。
  テストを弱めず、文言を「構築は削除済みです。」に変えた(2章の表・`DeviceDataTextTests`・DECISIONS.md も同じ)。
- テスト結果: `swift test` 576 件中 2 件失敗(上記の矛盾の2件のみ。新規 API 4 件を含む他は成功)/ `make ios-lint ios-gen-check
  ios-check-request-limits` 成功 / `make ios-test-ui` 53 件中 1 件失敗(`testFailureThenRetryCompletes`。上記の矛盾のみ。
  AX5 の新規テストを含む他は全件成功)。

## P6-24 の受け入れ条件(素早さ比較画面。判断は ADR-0503。spec-writer: 受け入れ条件とテストのみ。実装はしない)

契約は `services/speed/api/openapi.yaml`(gateway `/api/speed/*`)。参照実装は Web の `SpeedScreen`。生成方式・境界・ピッカー・モック・識別子は ADR-0503。
足場(型・プロトコル・`SpeedLabels` の固定文言・各実装の空の殻)は `TODO(implementer P6-24)` 付きで追加済み。生成ターゲット
(`PokeCalcSpeedAPI`)は足場がコンパイルするために spec の段階で作った(`make ios-gen-check` 成功)。

1. **生成**: `ios/scripts/openapi-gen.sh`(と `--check`)が「契約・設定・出力先」の組のループで `PokeCalcAPI`(不変)と `PokeCalcSpeedAPI` を扱い、
   `make ios-gen-check` が成功する。契約の enum(`PresetId`・`MinimalPresetId`・`NatureId`・`ModePayload`・`ErrorCode`)とドメインの enum が値・順序とも一致し
   (`SpeedContractSyncTests`)、SP の最大・ランクの範囲は `SPLimits`/`RankLimits` と契約が一致する(`make ios-check-request-limits`)。
2. **API 写像**(`APISpeedService`): 全操作に `X-Device-Id`/`X-Session-Id`。`GET /api/speed/v1/pokemon`(クエリなし)、`GET /api/speed/v1/table`
   (`presets` は 1 つのパラメータにカンマ区切り・全 6 行なら省略・`tailwind`/`trickRoom` は true のときだけ)、`POST /api/speed/v1/position`
   (本文は mode に要る項目だけ。false の `tailwind`/`paralysis`/`tableTailwind` は載せない。`scarf` は preset・custom で常に載せる)。応答は並びを変えずにドメインへ写す。
   400/413/422/500/503 は `code` をそのまま運ぶ `PokeCalcError`、通信不能は `transport`、読めない 200 は `decode`、契約外のステータスは本文が `{code,message}` ならその
   `code`・読めなければ `client_unexpected_status`、タスクのキャンセルは `CancellationError` のまま。
3. **入力 → 要求**(`SpeedViewModel`): Web と同じ既定(preset・最速・スカーフ無し・ポケモン未選択)。preset・custom はポケモン未選択では送らない。raw はポケモン任意で、
   自分の追い風・まひ・スカーフは載せない。SP は 0…`SPLimits.maxPerStat`、ランクは `RankLimits` に収める。raw の実数値は前後の空白を落として 1 以上の整数だけ送り、
   空欄は未入力(エラーなし)、それ以外は「実数値は1以上の整数で入力してください」を出して送らない(上限の判定はサーバー)。
4. **表と結果の整形**: 絞り込みは全 6 行なら `presets` を省き、そうでなければ選んだ行を契約の順で渡す。最後の 1 つは外せず(取り直さない)説明を出す。トリックルームは表だけ、
   相手側の追い風は表と位置の両方を取り直す。表の並びは応答のまま。境界線・「自分と同速」の強調は表示中の段の speed と自分の実数値の直接比較で決める
   (同じ speed の段があれば強調して境界なし。通常は降順、トリックルーム中は昇順。位置が読めていなければ引かない)。結果は応答のまま整形し、トリックルーム中だけ
   「先に動く(= slower)/後に動く(= faster)」の行を足す。
5. **非同期**: 連続入力は debounce で確定値の要求 1 回にまとめ、新しい入力・画面破棄で先行の要求を cancel する。最新の世代の応答だけ反映し(cancel を無視する
   サービスの古い応答も捨てる)、`CancellationError` は失敗として出さない。ポケモン一覧・表・位置は互いの失敗に巻き込まれず、`load()` は throw しない(絶対ルール 5)。
   エラーはサーバーの英語 message を出さず `code` から日本語にする。
6. **ピッカーとモック**: ピッカーは speed の一覧だけを使い、前後の空白を落としたひらがな/カタカナの部分一致で絞る(絞り込みは選択・要求に影響しない)。
   `MockSpeedService` は ADR-0503 §8 の固定の事実(4 体・24 行・同速あり・シナリオ)を満たす。
7. **画面(XCUITest・モック)**: ルートの `openSpeedScreen` と `POKECALC_OPEN_SPEED_SCREEN_AT_LAUNCH=1` で開く。表の段・絞り込み・最後の 1 つを外せないこと・トリックルームの並び替え・
   実数値 1 の境界線(最下部・同じ段なし)・同速の強調・カスタムの入力部品・ピッカーの絞り込み・不正な実数値の説明が動く。表だけ・位置だけ・一覧だけ失敗しても他は使え、
   全面的に失敗しても計算画面は開く(絶対ルール 5)。AX5 でも入力・表・結果・シートが横にはみ出さない。
8. **文言・デザイン・不変条件**: 文言は `SpeedLabels` に集約し Web の `ja.ts` と同じ(`SpeedLabelsTests` が固定)。design.md のトークンのみ、`lineLimit`・`minimumScaleFactor` なし、
   常時アニメーションなし。既存のテスト・identifier・`PokeCalcAPI` の生成物は不変。

### 追加したテスト(spec 時点)

- 単体(XCTest。`ios/PokeCalcKit/Tests/PokeCalcCoreTests/`)**113 件**: `SpeedContractSyncTests` 6・`SpeedLabelsTests` 9・`APISpeedServiceTests` 22・`MockSpeedServiceTests` 16・
  `SpeedViewModelInputTests` 16・`SpeedViewModelTableTests` 22・`SpeedViewModelAsyncTests` 14・`SpeedViewModelPickerTests` 8。
  足場: `Support/StubSpeedService.swift`(呼び出しの記録・hold/release・cancel の記録・cancel を無視するモード)。
  `swift test` は全 689 件中、新規の 87 件が失敗(足場が空の殻のため。意図どおり)、新規の 26 件(同期・固定文言・初期値など)と既存の 576 件は成功。
- 契約の範囲の同期: `ios/scripts/check-request-limits.sh` に speed(`PositionRequest.sp.maximum`・`rank.minimum/maximum` ↔ `SPLimits.maxPerStat`・`RankLimits`)を追加(成功。
  契約を 1 つ書き換えて失敗することも確認済み)。
- XCUITest(`ios/PokeCalcUITests/`)**16 件**(コンパイル確認のみ。View が未実装のため実行すると失敗する): `SpeedScreenUITests` 13 件、
  `LargeTextLayoutUITests` に AX5 の 3 件(`testSpeedScreenNoHorizontalOverflowAtAX5`・`testSpeedScreenResultAndTableNoHorizontalOverflowAtAX5`・`testSpeedPokemonSheetNoHorizontalOverflowAtAX5`)。
- `make ios-gen-check`・`make ios-lint`・`make ios-check-request-limits` 成功。`xcodebuild build-for-testing`(PokeCalc スキーム)成功。

### 実装者への注意

- 足場の公開 API(名前・case 名・引数・ID の文字列)をテストが固定している。変えるときは理由をコミットに書く。`TODO(implementer P6-24` を grep して全部埋める
  (`SpeedLabels.errorMessage`・`SpeedViewModel`・`APISpeedService`・`MockSpeedService`)。
- `SpeedViewModel`: 入力の変更は同期で状態を変え(`positionState = .loading` も同期)、要求は `LatestTaskRunner`(位置・表で別々)に予約する。応答の反映は世代で守る
  (Task のキャンセル確認だけに頼らない。`ignoringCancellation` のテストがある)。`settle()` は最新の予約済み Task を await する。表の取り直し・`load()` は debounce しない
  (`SpeedViewModelAsyncTests` は `debounce: .seconds(30)` でも `load()` が進む前提)。`selectedPokemon` は一覧から引く。`mode` を変えたら要求を作り直す。
- `APISpeedService.swift` は `PokeCalcSpeedAPI` だけを読み込む(root の `PokeCalcAPI` と `Client` が衝突する)。`presets` は `explode: false` の 1 パラメータ
  (生成クライアントに任せればカンマ区切りになる)。optional な本文項目は nil なら載らない(生成型の既定の符号化)。`undocumented` の本文は
  `UndocumentedPayload.body` を読んで `{code,message}` を試す。`ErrorCode` の enum にない code はデコード失敗(`decode`)になる点に注意。
- `MockSpeedService`: 表と位置は同じ内部の計算を共有する(最速 + スカーフ == 最速+1 の同値が自然に出る)。「テスト」で始まる架空の名前。`Resources/` を足すなら `MockFixtures` の流儀に従う。
- アプリ側: `AppEnvironment.ready` に `speed` を足し、`RootView`・`#Preview`・既存のパターンマッチをすべて更新する(API は同じ `ClientIdentity` を共有)。
  `RootView` に `openSpeedScreen` と `POKECALC_OPEN_SPEED_SCREEN_AT_LAUNCH`(既存の else-if の並びに足す)、`ios/scripts/sim-run.sh` の `IOS_SCREEN=speed`、
  `docs/runbooks/ios.md`・`ios/README.md`・`docs/plan.md`(P6-24)を更新する。View は `PokeCalcCore` の `SpeedViewModel` を `@State` で持ち、
  `.task` で `load()`、`.onDisappear` で `cancelPendingWork()`。
- View の制約: 段は `VStack`(`LazyVStack` にしない)、段・結果の行は `.accessibilityElement(children: .contain)` を付けて子の識別子を飲み込ませない。
  オン/オフの操作は選択ボタンで `isSelected` を出す。`Menu`・メニュー形式の `Picker` は使わない。`lineLimit`・`minimumScaleFactor` なし、AX5 では長いピルが折り返す。
  色はタイプのエンブレムだけが持つ(design.md)。エンブレムの色は `PokeType` ではなく文字列のタイプ ID なので、未知の ID は無彩色のフォールバックにする。
- 範囲(32・±6)は `SPLimits`・`RankLimits` から。`SpeedLabels` に数値を直書きしない。
- 完了条件: `make ios-test`(lint・gen-check・check-request-limits・単体・UI・infoplist)がすべて成功する。UI テストはモック固定の事実(24 行)に頼る箇所が 1 つある
  (`testRawValueBelowEveryRowDrawsTheBoundaryAtTheBottom`)。モックを変えるなら ADR-0503 §8 とそのテストを同時に直す。


### 実装結果(implementer)

- `swift test`(`ios/PokeCalcKit`): 全 689 件 成功 / 失敗 0(新規 113 件を含む。spec 時点の失敗 87 件はすべて解消)。
- `make ios-lint ios-gen-check ios-check-request-limits` 成功。`xcodebuild build-for-testing`(PokeCalc スキーム・iPhone 18 Pro)成功。XCUITest(16 件)は実行していない(コンパイルのみ確認)。
- 判断: 位置の再取得は「入力を変えた結果の要求が変わったとき」だけ予約する(preset のまま実数値欄を触る等では送らない)。
  ポケモンのシートの検索欄は `.searchable` ではなく通常の `TextField`(`speedPokemonSearchField` を入力できる要素にするため)。
  `SpeedPill`/`SpeedFlowLayout` は折り返すピル(既存の `ChipButton` は1行固定のため別に持つ)。`SpeedLabels` に `close`・`spIncrement`・`spDecrement` を追加。
- テストの矛盾: なし(テスト・既存の期待値は変更していない)。

## P6-25 の受け入れ条件(判定画面。判断は ADR-0504。spec-writer: 受け入れ条件とテストのみ。実装はしない)

契約は `services/judge/api/openapi.yaml`(実用は `POST /api/judge/v1/outspeed-and-ko` 1 本。judge 自身の Ingress の `/api/judge` prefix。ホストは他の API と同じ `baseURL`)。
参照実装は Web の `JudgeScreen`(ADR-0705)。生成方式・境界・要求の省略規則・印の出し方・モック・識別子は ADR-0504。ユーザー決定は DECISIONS.md 2026-10-03(判定画面)。
足場(型・プロトコル・`JudgeLabels` の固定文言・各実装の空の殻)は `TODO(implementer P6-25` 付きで追加済み。生成ターゲット(`PokeCalcJudgeAPI`)は
足場がコンパイルするために spec の段階で作った(`make ios-gen-check` 成功。`openapi-gen.sh` の配列に 1 行・設定ファイル・`Package.swift`)。

1. **生成と同期**: `make ios-gen-check` が `PokeCalcAPI`・`PokeCalcSpeedAPI`(不変)・`PokeCalcJudgeAPI` の 3 本で成功する。契約の `Format`・`ErrorCode`(8 値)とドメイン・文言の対応表が一致し
   (`JudgeContractSyncTests`)、候補数(`defenders` の min/maxItems)・技 ID の最大長・能力ポイント(`StatBlock` 6 項目の maximum)・ランク(`RankBlock` 5 項目の min/max)は
   `RequestLimits`・`SPLimits`・`RankLimits` と契約が一致する(`make ios-check-request-limits`。1 つずらすと失敗することを確認済み)。
2. **API 写像**(`APIJudgeService`): `X-Device-Id`/`X-Session-Id` 付きで `POST /api/judge/v1/outspeed-and-ko`。本文は `format`(常に載せる)・`attacker`・`defenders`(順を保つ。各候補は `moveId` を必ず持つ)・`moveId`。
   `ranks`(非 nil なら 5 項目すべて)・`speedField`(非 nil なら 3 項目すべて)・`abilityId`・`itemId` は nil なら欄ごと載せない(`null` を送らない)。`field` は載せない。
   応答の `matchups` は並びも `defenderIndex` も変えずに写し、未対応の印は方向ごとに分けたまま順を保って写す(知らない `target`/`reason` は `.unknown`)。印の欄が欠けた応答は `decode`(ADR-0708 §3)。
   400/413/422/500/503 は `code`・`message` をそのまま運ぶ `PokeCalcError`、通信不能は `transport`、読めない 200・契約の `ErrorCode` に無い code は `decode`、契約外のステータスは本文が `{code,message}` ならその `code`・読めなければ
   `client_unexpected_status`、タスクのキャンセルは `CancellationError` のまま。
3. **入力 → 要求**(`JudgeViewModel`): 初期は自分 1 体 + 空の候補 1 件・場の効果なし・format は常に single。`load()` が性格の一覧を読み、性格が未選択の自分・候補(追加した候補を含む)に「補正なし(plus も minus も無い)の最初の性格」を入れる(ユーザーが選んだ性格は上書きしない)。
   能力ポイントは各 0…`SPLimits.maxPerStat`・ランクは `RankLimits`(HP のランクは無い)に収める。候補は 1…`RequestLimits.maxJudgeDefenders` 件。省略の規則は Web と同じ: ランクは 5 項目すべて 0 なら nil・
   特性/持ち物は未選択なら nil・`speedField` は 3 つすべて false なら nil。**送信前の検査**(違反なら判定を呼ばず理由を出す。順: 自分 → 候補を index 昇順、各体の中は 必須〈種族・性格・技〉→ 技 ID の長さ → 能力ポイント合計 ≤ `SPLimits.maxTotal`)。
   理由は誰の入力かを先頭に付ける(自分のポケモン / 相手候補 N〈1 始まり〉)。同じ候補が重複していても取りまとめない。
4. **結果の整形**(`JudgeResultDisplayBuilder`): 行は `defenderIndex` の昇順で、種族名・タイプは**送信時点の要求の候補**から index で引く(引けなければ speciesKey、候補が無い index は名前を捏造しない)。
   素早さ・優先度・行動順は応答のまま(同速は「同速」、行動順が決まらないときは「どちらが先に動くか決まらない」。優先度が違えば素早さで上回っていても相手が先になりうる)。確定数は
   「倒せない / 確定 n 発 / 乱数 n 発(x.x%。小数第 1 位固定)」。「勝ち」「負け」に丸めた語を持たない(ADR-0700 §6-1)。
   **未対応の印は方向ごとに分ける**(ADR-0708 §4・§5): 順方向(`attackerKoUnsupported`)と逆方向(`defenderKoUnsupported`)を別々に `UnsupportedPlacement` へ渡し、全候補に共通する印は結果の上に 1 回(方向ごと)、
   一部の候補だけの印はその行に出す。**その方向の確定数を確定として見せない添え書き**は、置き場所によらず印のある行のその方向だけに付く(順方向の印が逆方向の確定数を疑わしく見せない)。
   `target` は書き換えず、その calc から見た役割のまま既存の `UnsupportedMarkLabel` で出す。名前は技・持ち物・特性の辞書から引き、無ければ ID。
5. **非同期・失敗**: 送信は**ボタンを押したときだけ**(入力のたびに送らない。debounce もしない)。`submit()` は同期で検査し、通れば `resultState = .loading`・世代を進め先行を cancel して予約する(違反なら呼ばず `validationError` を立て、直前の結果は消さない)。
   最新の世代の応答だけ反映する(cancel を無視するサービスの古い応答も捨てる)。`CancellationError` は失敗にしない。結果は送信時点の入力から作る(判定中に入力を変えても名前が変わらない)。失敗は `code` から日本語にし(サーバーの英語 message は出さない)、
   message の `defenders[<index>]` が候補数の範囲内なら「相手候補 N の入力で失敗しました」を添える(`attacker` の失敗には付けない)。エラーでも入力は消さない。判定・マスタ・構築は互いに巻き込まない(絶対ルール 5。`load()` は throw しない)。
6. **マスタと構築から呼び出す**: 性格・持ち物・種族・技は ID の自由入力ではなくマスタから選ぶ(種族・技は検索シート。特性は種族の `species(key:)` の候補。種族を変えたら新しい種族に無い特性は外す。最新の種族選択だけを反映)。
   構築から自分側・各候補側を埋められる: 種族・性格・SP・特性・持ち物・技(`TeamMemberConverter` の規則: 最初のダメージ技 → 無ければ最初の技 → 無ければ未選択)、ランクは 0 に戻す。無効な ID・範囲外の対象は何もしない。
   呼び出しは判定を送らず、構築のストアも書き換えない。呼び出したあとも各欄は直せる(スナップショット)。
7. **画面**(XCUITest・モック): ルートの `openJudgeScreen` と `POKECALC_OPEN_JUDGE_SCREEN_AT_LAUNCH=1` で開く。入力(性格の既定・選択シート・持ち物の解除・SP/ランクのステッパー)・場の効果・候補の増減(上限と最低 1 件)・
   検査メッセージ・結果の行(モックの固定の事実)・候補ごとに違う値・行動順(優先度が素早さに勝つ行)・トリックルームで結果が変わること・未対応の印の方向別の出し分け・失敗(日本語・候補の番号・入力が残る・計算画面は開く)・構築から呼び出す(自分側・候補側)が動く。
   AX5 でも入力・結果・印の注記・選択シートが横にはみ出さない。
8. **文言・デザイン・不変条件**: 文言は `JudgeLabels`(Core)に集約し Web の `judgeScreenText`・`judgeErrorText` と同じ(違いは ADR-0504 §9 の 4 点だけ。`JudgeLabelsTests` が固定)。design.md のトークンのみ・`lineLimit`・`minimumScaleFactor` なし・
   常時アニメーションなし。件数・範囲は定数から(直書きしない)。既存のテスト・identifier・`PokeCalcAPI`・`PokeCalcSpeedAPI` の生成物は不変。`api/openapi.yaml`・`services/` は変えない。

### 追加したテスト(spec 時点)

- 単体(XCTest。`ios/PokeCalcKit/Tests/PokeCalcCoreTests/`)**130 件**: `JudgeContractSyncTests` 4・`JudgeLabelsTests` 10・`APIJudgeServiceTests` 18・`MockJudgeServiceTests` 13・`JudgeResultDisplayTests` 15・
  `JudgeViewModelInputTests` 18・`JudgeViewModelRequestTests` 17・`JudgeViewModelAsyncTests` 16・`JudgeViewModelMasterTests` 10・`JudgeViewModelTeamTests` 9。
  足場: `Support/StubJudgeService.swift`(呼び出しの記録・hold/release・cancel の記録・cancel を無視するモード。候補ごとに違う値を返す既定の応答)・`Support/JudgeTestHarness.swift`・`Support/ArraySafe.swift`
  (足場が空の殻のとき、空の結果への添字アクセスでテストプロセスごと落ちないための `[safe:]`)。
  `swift test` は全 819 件中、**新規の 104 件が失敗**(足場が空の殻のため。意図どおり)、新規の 26 件(同期・固定文言・初期値など)と既存の 689 件は成功。
- 契約の範囲の同期: `ios/scripts/check-request-limits.sh` に judge(`defenders.minItems/maxItems`・`MoveId.maxLength`・`StatBlock` 6 項目・`RankBlock` 5 項目)を追加(成功。`RequestLimits` を 2 つずらして失敗することも確認済み)。
  既存の speed 用の関数は共通の `schema_property_value` に寄せた(挙動は同じ)。
- XCUITest(`ios/PokeCalcUITests/`)**20 件**(コンパイル確認のみ。View が未実装のため実行すると失敗する): `JudgeScreenUITests` 17 件、`LargeTextLayoutUITests` に AX5 の 3 件
  (`testJudgeScreenNoHorizontalOverflowAtAX5`・`testJudgeResultAndUnsupportedNoticesNoHorizontalOverflowAtAX5`・`testJudgeOptionSheetNoHorizontalOverflowAtAX5`)。
- `make ios-gen-check`・`make ios-lint`・`make ios-check-request-limits` 成功。`xcodebuild build-for-testing`(PokeCalc スキーム・iPhone 18 Pro)成功。
- 手順書: `docs/runbooks/ios.md` の確認行を 7 行(gen-check 3 本・check-request-limits を含む)に直した。

### 実装者への注意

- 足場の公開 API(名前・case 名・引数・ID の文字列)をテストが固定している。変えるときは理由をコミットに書く。`TODO(implementer P6-25` を grep して全部埋める
  (`JudgeLabels.errorMessage`・`JudgeFailure.init(code:serverMessage:candidateCount:)`・`JudgeViewModel`・`JudgeResultDisplayBuilder`・`APIJudgeService`・`MockJudgeService`)。
- `JudgeViewModel`: `submit()` は同期で検査・`resultState = .loading`・世代の更新・先行の cancel まで行い、要求は `LatestTaskRunner`(debounce `.zero`)に予約する。応答の反映は世代で守る(Task のキャンセル確認だけに頼らない。
  `hold(ignoringCancellation: true)` のテストがある)。結果の整形は**送信時点の要求・名前の辞書のスナップショット**で行う(`JudgeResultDisplayBuilder.make` に要求を渡す)。`settle()` は最新の予約済み Task を await する。
  `setSpecies` は種族のキー・名前を同期で反映してから `species(key:)` を await し、世代で最新の選択だけ反映する(`.manual` のテストがある)。特性の候補が取れなくても入力は止めない。
  `load()` は性格・持ち物・技・種族の先頭ページ・構築を**互いに独立に**読み、失敗は `masterFailure` に入れる(throw しない)。種族・技の検索は `MasterSearchField`(`TeamEditViewModel` と同じ)を再利用する。
  `applyTeamMember` は `TeamListFetcher`/`TeamMemberConverter` を再利用し、技の判定に使う `moves` は一度見た技の辞書から作る。ストアには書かない。
- `JudgeResultDisplayBuilder`: 印は `UnsupportedPlacement` を**方向ごとに**呼ぶ(`attackerKoUnsupported` の全行と `defenderKoUnsupported` の全行を 1 つの配列にしない)。注記は `UnsupportedNoticeText.rowNote`
  の結果を `JudgeLabels.attackerKoUnsupportedNotice(detail:)`/`defenderKoUnsupportedNotice(detail:)` で包む。`UnsupportedMarkNames` は `moveDictionary`(選んだ技・検索結果)・`itemOptions`・見た種族の特性から作る。
  KO の文言は `BulkRowDisplay.koText` と同じ規則(確率は小数第 1 位固定・ロケール非依存)で、`JudgeKOChance` から `KOChance`(`chancePercent` は `displayChancePercent` で埋める)を作って再利用してよい。
- `APIJudgeService.swift` は `PokeCalcJudgeAPI` だけを読み込む(`PokeCalcAPI`・`PokeCalcSpeedAPI` と `Client`・`Components` が衝突する)。`attacker`・`moveId` は生成型の allOf ラッパー(`AttackerPayload`・`MoveIdPayload`・`DefenderCandidate.MoveIdPayload`、
  `Matchup.AttackerKoPayload`・`DefenderKoPayload`)で包む/剥く。`UnsupportedMark.target/reason` は生成型では `String`。`ErrorCode` の enum にない code はデコード失敗(`decode`)になる点は speed と同じ。
- アプリ側: `AppEnvironment.ready` に `judge: any JudgeService` を足し(API は同じ `baseURL`・同じ `ClientIdentity`。モックは `MockJudgeService(environment:)`)、`RootView`・`#Preview`・既存のパターンマッチ(`.ready(` の全箇所)をすべて更新する。
  `RootView` に `openJudgeScreen`(`JudgeLabels.openButton`)と `POKECALC_OPEN_JUDGE_SCREEN_AT_LAUNCH`(既存の else-if の並びに足す)、`ios/scripts/sim-run.sh` の `IOS_SCREEN=judge`(使い方・case・usage・Makefile のコメント)、
  `docs/runbooks/ios.md`・`ios/README.md`(`POKECALC_MOCK_JUDGE=error|candidate-error|marks`)・`docs/plan.md`(P6-25)を更新する。View は `PokeCalcCore` の `JudgeViewModel` を `@State` で持ち、`.task` で `load()`、`.onDisappear` で `cancelPendingWork()`。
- View の制約: 性格・特性・持ち物の選択は**シート**(`judgeOptionSheet`。`Menu`・メニュー形式の `Picker` は使わない。中のボタンに identifier が付かない制約がある)。種族・技のシートは既存の `SpeciesSearchSheet`・`MoveSearchSheet` を再利用する。
  構築から呼び出す入口は既存の `TeamSourceMenuRow`(`identifierPrefix` = `judgeAttackerTeam`/`judgeCandidate<n>Team`。メニュー項目は identifier が渡らないのでテストはニックネームのラベルで選ぶ)。
  オン/オフの操作(場の効果)は選択ボタンで `isSelected` を出す。結果の行は `.accessibilityElement(children: .contain)` を付けて子の識別子を飲み込ませない。`LazyVStack` にしない。`lineLimit`・`minimumScaleFactor` なし、AX5 では長いピルが折り返す。
  色はタイプのエンブレムだけが持つ(design.md)。未知のタイプ ID は無彩色のフォールバック。種族の選択後に特性が外れた/候補が変わったことを黙って起こさない(特性ボタンのラベルで分かる)。
- 範囲(32・±6・66・6 件・64 文字)は `SPLimits`・`RankLimits`・`RequestLimits` から。`JudgeLabels` に数値を直書きしない。
- 完了条件: `make ios-test`(lint・gen-check・check-request-limits・単体・UI・infoplist)がすべて成功する。UI テストはモックの固定の事実(ADR-0504 §8)に頼る箇所が複数ある
  (素早さ 150 対 100/125・確定 1 発・乱数 2 発〈15.0%〉・優先度 0 対 1)。モックを変えるなら ADR-0504 §8 と `MockJudgeServiceTests`・UI テストを同時に直す。

### 実装結果(implementer)

- `swift test`(`ios/PokeCalcKit`): **全 819 件成功・失敗 0**(spec 時点で失敗していた新規 104 件がすべて成功。既存 689 件と、spec 時点で成功していた新規 26 件も成功)。出力に `error` なし。
- `make ios-lint ios-gen-check ios-check-request-limits` 成功(gen-check は 3 本一致)。`xcodebuild build-for-testing`(PokeCalc スキーム・iPhone 18 Pro)成功。XCUITest(`JudgeScreenUITests` 17 件・`LargeTextLayoutUITests` の判定 3 件)はこの実装では実行していない(別途実行)。
- 実装した範囲: `JudgeViewModel`・`JudgeLabels.errorMessage`・`JudgeFailure.init(serverMessage:)`・`JudgeResultDisplayBuilder`(順方向・逆方向の印を `UnsupportedPlacement` へ別々に渡す)・`APIJudgeService`・`MockJudgeService`、
  `AppEnvironment.ready` への `judge`(既存のパターンマッチを全更新)、`RootView` の `openJudgeScreen`・`POKECALC_OPEN_JUDGE_SCREEN_AT_LAUNCH`、View(`JudgeScreenView`・`JudgeIndividualCard`・`JudgeOptionSheet`・`JudgeResultSection`)、
  `sim-run.sh` の `IOS_SCREEN=judge`、`docs/runbooks/ios.md`(7 章を追加し以降を繰り下げ)・`ios/README.md`。
- 判断: (1) 体ごとの種族名・特性の候補は位置ではなく内部の `id` で結び、候補を消しても名前が候補について行き、非同期の応答は `id` で自分の体を探す。
  (2) 種族の詳細が取れないとき、種族を変えていれば特性を外す(確かめられない特性を送らない)。(3) `cancelPendingWork` は読み込み中なら `.idle` に戻す。
  (4) `JudgeLabels` に View 専用の文言(選択シートの閉じる・空、ステッパーのアクセシビリティ名、マスタ読み込み失敗)を足した(既存の固定文言は不変)。
  (5) 能力ポイント・ランクのステッパーは `SpeedStepper` ではなく識別子に `-<stat>` を付けられる専用の部品にした(`SpeedPill`・`SpeedFlowLayout`・`SpeedCaption` は再利用)。
- テストと実装の矛盾: なし(テストは変更していない)。
- XCUITest の失敗の修正(いずれも実装側。テストは変更していない): (1) 結果の行(`judgeRow-N`)が 1 行だけのとき、行の `.contain` が外側の `judgeResult`(`.contain`)に畳まれて識別子が消えた
  (外側の子が行 1 つだけだと外側が行の枠になり、外側の識別子が勝つ)。`judgeResult` の `.contain` に見出し「判定結果」も含め(読み込み済みのときだけ。他の状態は従来どおり見出し単独)、子を 2 つ以上にして畳まれないようにした。
  (2) 能力ポイントの「−」ボタンが記号の細さのぶん押せる範囲が小さく(20pt 角。「+」は 31pt)、`tap()` が効かなかった。`JudgeStepper.stepButton` に `minWidth/minHeight 36`・`contentShape(Circle())` を足して − と + をそろえた。

## P6-23 の受け入れ条件(iOS: 種族ピッカーの「よく使う相手」。ADR-0209・requirements.md §2。spec-writer: 受け入れ条件とテストのみ。実装はしない)

- 日付: 2026-10-02 / 担当レーン: iOS / 関連: api/openapi.yaml(`listFrequentOpponents`・`FrequentOpponent`)、ADR-0209(頻度の保存・減衰・
  プライバシー)、ADR-0212、本 ADR「P6-7」(別プロトコル・モックの流儀)・「issue #68」(種族検索シート)、CLAUDE.md 絶対ルール5、
  DECISIONS.md 本タスクの新規エントリ「P6-23」、docs/plan.md P6-23
- 背景: `GET /api/record/frequent-opponents`(ヘッダー `X-Device-Id`・`X-Session-Id`、クエリ `limit` 1〜50〈既定10、範囲外は 400〉)は
  `FrequentOpponent[]`(`speciesKey`・`score`・`count`・`lastCalculatedAt`)をスコア降順・同点は `speciesKey` 昇順で返す。記録が無ければ
  空配列(404 にしない)。503 は `store_unavailable` / `upstream_unavailable`。**名前・タイプは返さない**ので pokedex で解決する。
  頻度は calc-svc が計算のたびに NATS へ非同期発行し record-svc が保存する(**クライアントは記録を POST しない**)。
  iOS の生成クライアントに record タグは含まれる(P6-7)が、`listFrequentOpponents` を呼ぶ口は無かった。Web は `frequent-opponents` 未使用。

### 1. 事実と判断の要約

| 論点 | 決定 | 理由 |
|---|---|---|
| どのピッカーに出すか | **計算の防御側・逆算の相手のみ**。攻撃側・逆算の自分・構築メンバー(新規含む)には出さない | 頻度は「相手(防御側)として計算した回数」(ADR-0209 §4)。攻撃側・自分・構築メンバーは相手ではなく、出すと意味が混ざる |
| いつ出すか | 空クエリのときだけ、一覧の先頭に「よく使う相手」セクション。検索語がある間は出さない(消せば戻る) | 検索結果に別の並びを混ぜない。`MasterSearchRow.hint`(空クエリの案内)はそのまま残す |
| 件数 | `limit` = 10(`FrequentOpponents.defaultLimit`) | openapi の既定。セクション1つに収まる量 |
| 名前解決 | `PokeCalcService.species(key:)` を1件ずつ。同時実行は 4(`maxConcurrentResolutions`)。解決できない key(`not_found`・通信失敗等)は黙って省く。重複 key は1回だけ解決・1行だけ表示。サーバーが `limit` を超えて返しても `limit` 件までしか引かない | 検索 API は key 指定に向かない。マスタに無い・古い key があっても残りを出す |
| 失敗・空 | 取得失敗(通信・503・400)・空配列・全件未解決は**何も出さない**(エラー表示なし)。検索 UI と計算は塞がない | 絶対ルール5。この機能は補助 |
| 取得のタイミング | **シートを開くたびに1回**(`.task`)。再取得中は前回の候補を出したまま(ちらつかせない)。取得失敗で前回の候補を消す | 計算のたびにスコアが変わる。1シート=1リクエスト |
| 古い応答・キャンセル | 新しい `refresh()` は先行を無効にして cancel(古い応答・古い名前解決は反映しない)。Task cancel(シートを閉じる)では候補を変えず、解決中の `species(key:)` も cancel | `LatestTaskRunner` / 検索の世代管理と同じ規則 |
| 選んだとき | 通常の検索結果と同じ `onSelect(SpeciesSummary)`(防御側なら `selectDefender(speciesKey:)`、逆算の相手なら `selectOpponentSpecies(key:)`)→ シートを閉じる | 反映の経路を増やさない |
| 依存の形 | `PokeCalcService` とは別のプロトコル `FrequentOpponentsService`(`DeviceDataService` と同じ理由)。`CalcViewModel`/`ReverseViewModel`/`TeamEditViewModel` には依存を足さない。`FrequentOpponentsViewModel`(Core、`@MainActor @Observable`)が取得・解決・世代管理を持ち、`SpeciesSearchSheet` が任意引数 `frequentOpponents: FrequentOpponentsViewModel? = nil` で受け取る | 計算系の呼び出し回数・テストを不変に保つ |
| 依存の渡し方 | `AppEnvironment.ready` に値を足す(P6-7 の `deviceData` と同じ流儀)。モックは `MockFrequentOpponentsService(environment:)`、API は同じ `APIPokeCalcService`(extension で準拠) | 既存の流儀 |
| 文言 | `FrequentOpponentsLabels.sectionTitle = "よく使う相手"`(Core に1か所) | coding-rules §2 |
| identifier(追加のみ) | セクション `frequentOpponentsSection`、行 `frequentOpponentRow-<speciesKey>`(行のボタンのラベルは種族名)。既存の `speciesSearchResult-<key>` は変えない | 既存 XCUITest 不変 |

### 2. 状態の仕様

- `FrequentOpponentsViewModel(service:resolver:limit:maxConcurrentResolutions:)`。`service == nil` なら何もしない。
- `refresh()`: `service.frequentOpponents(limit:)` → 先頭 `limit` 件の重複を除いた key を同時 4 件までで `resolver.species(key:)` →
  サーバーの順で `items`(`SpeciesSummary`)を置き換える。`isLoading` は進行中だけ true。
- `visibleItems(forQuery:)`: 前後空白を除いて空なら `items`、そうでなければ `[]`(View はこれだけを見て描く)。
- モック(XCUITest 用): `POKECALC_MOCK_FREQUENT_OPPONENTS` = なし/未知で `list`(`9003-000`・`9001-000`・`9999-000`〈マスタに無い〉の
  3件を `prefix(limit)`、スコア降順)/ `empty`(空配列)/ `fail`(transport エラー)。**既存の `MockPokeCalcService` と
  `species(key:)` の呼び出し回数は変えない**(別の service なので)。

### 3. 受け入れ条件(検証可能な形)

1. API 写像(`APIFrequentOpponentsServiceTests`): GET・パス `/api/record/frequent-opponents`・クエリ `limit` がそのまま載る(範囲外もクライアントで
   丸めない)・`X-Device-Id`/`X-Session-Id`・200 の全フィールドとサーバー順の保持・空配列は成功・400(`invalid_input`)・503
   (`store_unavailable`/`upstream_unavailable`)の code 保持・通信不能は `transport`。
2. `FrequentOpponentsViewModel` の解決: 既定 `limit` 10 で取得、名前はサーバーの順で `species(key:)` から引く(検索 API は呼ばない)。
   未解決・解決失敗の key は黙って省き順序を保つ。`limit` 超過分は引かない。重複は1回。
3. 失敗・空: 取得失敗(`store_unavailable`・`upstream_unavailable`・`invalid_input`・`transport`)・空配列・service なし・全件未解決は
   `items == []` で例外を出さず、失敗時は `species(key:)` を呼ばない。成功後の失敗は古い候補を消す。
4. 再取得: `refresh()` のたびに取得し直し、前回の候補は再取得中も出したまま、完了で置き換える。
5. 古い応答の破棄: 新しい `refresh()` が先行を cancel し、先行の取得・名前解決の応答は新しい結果を上書きしない。
6. キャンセル: Task cancel で候補を変えず `isLoading` を戻し、取得中・解決中の要求を cancel する。
7. 同時実行: 名前解決の同時実行は上限(テストでは 3)を超えず、1件返ると次が始まる。結果の順序はサーバーの順。
8. `visibleItems(forQuery:)` は空・空白だけで `items`、検索語ありで `[]`。
9. 既存への影響なし: `CalcViewModel`・`ReverseViewModel` は `FrequentOpponentsService` に依存せず(呼ばない)、`refresh()` は検索 API を呼ばない。
10. モック(`MockFrequentOpponentsServiceTests`): 環境変数の写像、`list` の順序と `limit`、`empty`、`fail`、架空の key(9xxx)のみ。
11. XCUITest(`FrequentOpponentsUITests`・モック): 防御側ピッカーでセクションと行がサーバー順に出て未解決 key は出ない・行を選ぶとシートが
    閉じ防御側が変わる・検索語でセクションが消え消すと戻る・攻撃側には出ない・逆算は相手だけに出て自分には出ない・`empty`/`fail` では
    セクションが出ず検索結果と計算結果の行は出る。
12. AX5 で `frequentOpponentsSection` と行が横にはみ出さない(`LargeTextLayoutUITests.testFrequentOpponentsSectionNoHorizontalOverflowAtAX5`)。
13. 既存の XCTest・XCUITest・identifier は1つも編集・変更しない(`LargeTextLayoutUITests` へのメソッド追加のみ)。
    `api/openapi.yaml`・Generated は変更しない(契約の変更なし)。

### 4. 追加したテスト(spec 時点)

- 足場(既定値付き。`TODO(implementer` を検索): `PokeCalcCore/FrequentOpponents.swift`(`FrequentOpponent`・`FrequentOpponentsService`・
  `FrequentOpponents`〈定数〉・`FrequentOpponentsLabels`・`FrequentOpponentsViewModel`〈何もしない〉)、
  `PokeCalcCore/MockFrequentOpponentsService.swift`(空配列)、`APIPokeCalcService.swift` 末尾の extension(空配列)
- `Tests/PokeCalcCoreTests/APIFrequentOpponentsServiceTests.swift`(7件)・`MockFrequentOpponentsServiceTests.swift`(6件)・
  `FrequentOpponentsViewModelTests.swift`(21件)・`Support/StubFrequentOpponentsService.swift`(台本・保留・cancel 観測付きのスタブ)
- `PokeCalcUITests/FrequentOpponentsUITests.swift`(6件)と `LargeTextLayoutUITests.testFrequentOpponentsSectionNoHorizontalOverflowAtAX5`(既存は無編集)

`swift test`(`ios/PokeCalcKit`): 610件中、新規34件のうち25件(47アサーション)が失敗。足場が何もしないため。残り9件(失敗・空・nil の
「何も出さない」系)は足場でも成立する。既存576件は成功。`xcodebuild build-for-testing`(XCUITest 7件を含む)は `** TEST BUILD SUCCEEDED **`。
XCUITest の実行は未実施(identifier 未実装のため失敗する)。

### 5. 実装者への注意

- 構築編集(`TeamEditView`・`TeamEditMemberCard`)と計算の攻撃側・逆算の自分の `SpeciesSearchSheet` 呼び出しは `frequentOpponents` を渡さない
  (既定 nil で今まで通り)。防御側(`CalcScreenCards` の防御側)と逆算の相手(`ReverseScreenCards`)だけ渡す。
- `FrequentOpponentsViewModel` は View(または `RootView`)が `@State` で持ち、計算系 ViewModel には持たせない。シートの `.task` で
  `refresh()` を呼ぶ(シートを閉じれば Task が cancel される)。`resolver` は計算と同じ `PokeCalcService`。
- `refresh()` の「新しい呼び出しが先行を cancel する」は、内部で保持する Task を cancel するか、世代番号+`Task.checkCancellation` で実現する。
  テスト `testStaleServiceResponseIsDiscarded`・`testStaleNameResolutionIsDiscarded` は先行の要求が cancel されることまで確認する。
- 同時実行の上限は `withThrowingTaskGroup` に最大 N 件だけ追加し、1件終わるごとに次を追加する形(テスト `testNameResolutionRespectsConcurrencyCap`)。
  結果は index で並べ直す。
- 失敗は握りつぶして `items = []`(ログは任意)。`PokeCalcError` を画面のエラー表示(`CalcScreenError` 等)に流さない。
- セクションは `List` の `Section` か先頭の行群。見出し文言は `FrequentOpponentsLabels.sectionTitle`。行の見た目は `MasterSearchRow.species` を共用し、
  `.accessibilityLabel(option.nameJa)` を付ける(既存の検索結果と同じ)。AX5 では行が折り返してもはみ出さない。
- `AppEnvironment.ready` に値を足すと `RootView` の `case .ready(let service, _, let backendDescription)` が壊れるので直す。
- `ios/README.md` の操作説明・`docs/plan.md`(P6-23 のチェック)を更新する。

### 6. 実装結果(P6-23。implementer)

- `swift test`(`ios/PokeCalcKit`): 610件すべて成功(新規34件を含む。3回連続で安定)。`make ios-lint ios-gen-check ios-check-request-limits` 成功。
  `xcodebuild build-for-testing`(iPhone 18 Pro)は `** TEST BUILD SUCCEEDED **`。XCUITest は未実行(別途実行)。
- `refresh()` は内部 Task + 世代番号で先行を無効化・cancel(呼び出し元の cancel は `withTaskCancellationHandler` で内側へ伝える)。
  名前解決は `withTaskGroup` で最大 N 件(1件終わるごとに次を追加、index で並べ直し)。
- View: `SpeciesSearchSheet` の先頭に `Section`(見出し Text に `frequentOpponentsSection`、行は `MasterSearchRow.species` を共用)。
  identifier は `Section` ではなく見出し Text に付けた(List 内の Section 自体への identifier は XCUITest から見えない場合があるため)。
  `FrequentOpponentsViewModel` は `CalcScreenView`・`ReverseScreenView` が `@State` で持ち、防御側カード・相手カードだけに渡す。
- テストの矛盾は無かった。

## P6-20 の受け入れ条件(構築の Showdown 風テキストのインポート/エクスポート。spec-writer: 受け入れ条件とテストのみ。実装はしない)

- 日付: 2026-10-02 / 担当レーン: iOS / 関連: requirements.md §2、ADR-0213 §4、ADR-0506(日本語名・非互換の決定)、ADR-0500 §4、
  本 ADR「P6-2c」(構築編集)・「P6-7」(identifier と `Menu` の流儀)、CLAUDE.md 絶対ルール5、DECISIONS.md「P6-20」、docs/plan.md P6-20
- 背景: 構築編集画面から1体または全体を書き出し(コピー・共有)、貼り付けで新しいメンバーとして取り込む。名前 → ID は既存の
  `searchSpecies`/`searchMoves`/`searchItems`/`natures`/`species(key:)` で解決する(新しい API は無い。`api/openapi.yaml`・Generated は不変)。

### 1. 型と置き場所(足場は spec-writer が置いた。`TODO(implementer` を埋める)

| 型 | ファイル | 役割 |
|---|---|---|
| `ShowdownTextLabels`(enum) | `PokeCalcCore/ShowdownText.swift` | 書式のキーワードと画面の文言(1か所) |
| `ShowdownNaming`(protocol)・`JapaneseShowdownNaming` | 同上 | 名前の戦略(英語名への拡張点。ADR-0506 §4) |
| `ShowdownTextParser.parse` → `ShowdownParseResult` | 同上 | テキスト → 解釈済みメンバー + 取り込めなかった行(純粋) |
| `ShowdownTextSerializer.serialize` | 同上 | `TeamMember` + `ShowdownNames` → テキスト(純粋) |
| `ShowdownTransferService`(`export`/`resolve`) | `PokeCalcCore/ShowdownTransfer.swift` | 名前 ⇄ ID の解決(`PokeCalcService` のマスタ参照だけ) |
| `TeamTextTransferViewModel`(`@MainActor @Observable`) | 同上 | シートの状態(書き出し・解釈 → 確認 → 追加) |
| `TeamEditViewModel.importMembers` | `PokeCalcCore/TeamEditViewModel+Import.swift` | 取り込んだメンバーを追加(保存しない) |

### 2. 書式(確定。ADR-0506 §2・§3)

- 先頭行: `名前` / `名前 @ 持ち物` / `ニック (名前)` / `ニック (名前) @ 持ち物`。前後の空白(全角を含む)は無視。末尾が ` @` だけなら持ち物なし。
- キーワード: `Ability:` `Nature:` `SP:` `Tera Type:` `- `(技)。`EVs:` `IVs:` は `unsupportedStatLine`。それ以外(`Level:` など)は `unrecognizedLine`。
- 改行は LF/CRLF/CR。空行(空白だけの行)でメンバーを区切る。行番号は 1 始まりで空行も数える。
- SP 行: `SP: 32 Atk / 20 Spe`。各項は `整数 略称`(略称は HP/Atk/Def/SpA/SpD/Spe)。0〜32 を超える項は `spOutOfRange`、合計66超は
  `spTotalExceeded`、書き方の誤り・同じステータスの重複・負数は `spMalformed`。**いずれも SP 行全体を捨てる**(部分適用しない)。
- 同じ項目の2回目は `duplicateField`(最初の値を残す)、5つ目の技は `tooManyMoves`(先頭4つを残す)、同名の技の2つ目は `duplicateMove`。
- 7ブロック目以降は先頭行だけを `memberLimitExceeded` で報告(残りの行は個別に報告しない)。
- 書き出し: 値の無い項目・名前を引けない項目の行は書かない。SP は 0 を省き全部 0 なら `SP:` 行ごと省く。技は4つまで。末尾に改行なし。
  複数体は空行1つで区切る。確定した例は `ShowdownTextSerializerTests`。

### 3. 画面の文言と取り込めなかった理由(確定。`ShowdownTextLabels`)

| 定数 | 文言 |
|---|---|
| `transferButton` / `sheetTitle` | テキストで書き出し・取り込み / 構築のテキスト |
| `exportMemberButton` / `exportTeamButton` | この1体を書き出す / 全員を書き出す |
| `copyButton` / `copiedNotice` / `shareButton` | コピー / コピーしました。 / 共有 |
| `importSectionTitle` / `importPlaceholder` / `analyzeButton` | テキストから取り込む / ここに貼り付け / 内容を確認 |
| `rejectedTitle` / `cancelButton` / `closeButton` | 取り込めなかった行 / やめる / 閉じる |
| `nothingImportable` / `emptyInput` | 取り込めるポケモンがありません。/ テキストを貼り付けてください。 |
| `lookupFailure` | サーバーに届かず、名前を確認できませんでした。通信を確認してもう一度お試しください。保存済みの構築は変わりません。 |
| `importValidOnlyButton(count:)` / `importAllButton(count:)` | 取り込める\(n)体だけ追加 / \(n)体を追加 |
| `importedNotice(count:)` / `lineNumberLabel(_:)` | \(n)体を追加しました。保存すると反映されます。/ \(n)行目 |

理由の文言(`message(for:)`。数字は `TeamLimits`・`SPLimits` から埋める): `unrecognizedLine` 解釈できない行です / `unsupportedStatLine`
努力値(EVs)・個体値(IVs)の形式には対応していません。能力ポイントは SP: で書いてください / `duplicateField` 同じ項目が2回書かれています /
`tooManyMoves` 技は4つまでです / `duplicateMove` 同じ技が重複しています / `spMalformed` SP の書き方が正しくありません /
`spOutOfRange` SP は1ステータスにつき32までです / `spTotalExceeded` SP の合計は66までです / `speciesNotFound` ポケモンが見つかりません /
`moveNotFound` 技が見つかりません / `itemNotFound` 持ち物が見つかりません / `abilityNotFound` このポケモンの特性に見つかりません /
`natureNotFound` 性格が見つかりません / `teraTypeNotFound` テラスタイプが見つかりません / `lookupFailed` 通信できず確認できませんでした /
`memberLimitExceeded` 構築は6体までです。

### 4. 名前の解決(`ShowdownTransferService.resolve`)

- 完全一致(前後空白を除く)だけを採る。検索は `ShowdownNaming.searchQuery` の語で `limit = MasterSearch.pageLimit`、結果から完全一致を選ぶ。
  同名が複数あれば、サービスが返した順の先頭。
- 種族が解決できなければメンバーごと捨て、**先頭行**に `speciesNotFound`(通信失敗は `lookupFailed`)。持ち物・特性・技・タイプ・性格が
  解決できなければ、**その行だけ**を理由つきで報告し、メンバーは取り込む(技は learnset を検査しない。性格が無い・解決できないときは
  `natures()` の先頭、`addMember` と同じ)。特性は `SpeciesDetail.abilities` の中だけから選ぶ。
- 枠: `existingMemberCount + 取り込むメンバー数` が `TeamLimits.maxMembers` を超えるぶんは、先頭から枠を使い、超えたブロックの先頭行を
  `memberLimitExceeded` で報告する。出力の `members` はすべて `TeamValidator` を満たす。
- 通信量: 同じ名前は1回しか検索しない。同時呼び出しは `ShowdownTransferLimits.maxConcurrentRequests`(4)まで。通信失敗でも投げず
  `plan.error` で返す(計算・保存済みの構築に影響しない。絶対ルール5)。
- 書き出し: 種族は `species(key:)`(同じキーは1回)、技は `moves(ids:)` を1回、持ち物は `searchItems`(空クエリの先頭ページ)、性格は
  `natures()` を1回。種族を引けないメンバーは飛ばして `error` を立てる(他のメンバーは書く)。

### 5. 取り込めなかった行の扱い(要件の判断。**既定案**。人間レビューで変えてよい)

- 取り込めなかった行は黙って捨てず、行番号・内容・理由を一覧で見せる。**全か無かにしない**。
- 取り込めなかった行が無ければ確認なしに追加できる(ボタンは「N体を追加」)。取り込めなかった行があり取り込める体もあれば、一覧を見せたうえで
  「取り込める N 体だけ追加」と「やめる」の2択にする。取り込める体が0なら追加ボタンは出さない。
- 行単位で捨てたメンバー(例: 持ち物だけ見つからない)も「取り込める体」に数え、その行を一覧に残す。
- 追加は `TeamEditViewModel.importMembers`(新しいメンバーとして末尾へ。保存しない)。確定したら貼り付けを空に戻す。貼り付けが変わったら
  解釈の結果を捨てる。同じ文字列なら残す。
- 通信失敗は `lookupFailed` の行と `lookupFailure` の案内で伝え、貼り付けを消さない(再実行できる)。

### 6. accessibilityIdentifier(追加のみ。camelCase、メンバー単位は `-<id>` 接尾。`Menu` の中には置かない)

構築編集の入口: `teamTextTransferButton`(通常の `Button`。シートを開く)・各メンバーカード `exportMemberTextButton-<memberId>`(押すとシートが
開き、その1体を書き出した状態)・取り込み後の通知 `importedNotice`。シート: `teamTextSheet`・`exportTeamTextButton`・`exportedText`
(`Text`。`label` がテキストそのもの)・`copyExportedTextButton`・`copiedNotice`・`shareExportedTextLink`(`ShareLink`)・`importTextEditor`
(`TextEditor`)・`analyzeImportTextButton`・`importEmptyNotice`・`importRejectedList`(`label` に見出しを含む)・
`importRejectedLine-<行番号>`(`label` に行番号・内容・理由を含む)・`importNothingNotice`・`importFailureNotice`・
`confirmImportValidButton`(取り込める体だけ・全部のどちらも同じ id)・`cancelImportButton`・`closeTeamTextSheetButton`。
`Menu` の中の identifier は UIKit に渡らない既知の制約があるので、入口・操作は `Menu` に入れない。

### 7. 受け入れ条件(検証可能な形)

1. 書式の定数と文言が 2章・3章の表と完全一致する(`ShowdownTextLabelsTests`)。理由の文言は全ケース非空・互いに異なる。
2. パーサ: 先頭行の4形・空白・全角空白・CR/LF/CRLF・空行区切り・行番号が 2章どおり。`EVs:`/`IVs:`・未知の行・重複項目・5つ目の技・
   技の重複・SP の範囲/合計/書き方の誤りが、行番号と理由つきで `rejected` に入り、採った行の値は変わらない。7ブロック目は先頭行だけ報告
   (`ShowdownTextParserTests`)。
3. シリアライザ: 全項目・省略・ニックネーム・SP の省略・技4つ・名前を引けない ID の省略と報告・複数体の区切りが 2章どおり。
   書き出して取り込み直すと名前の組が元と一致する(`ShowdownTextSerializerTests`)。
4. 名前解決: 全項目が ID になる・性格の既定・行単位の失敗とメンバーの継続・種族不明でメンバーごと捨てる・特性は種族の特性のみ・
   パーサの失敗との行番号順のマージ・枠(既存メンバー数を含む)・`TeamValidator` を必ず満たす(`ShowdownTransferServiceTests`)。
5. 通信量: 同じ名前は1回だけ検索し、同時呼び出しは指定した上限(1・2・既定)を超えない。通信失敗は投げず `lookupFailed` と `error` で返す。
6. 拡張点: 名前の戦略を差し替えても書き出し・取り込みが往復する(`ShowdownTransferServiceTests.testCustomNamingIsUsedForImportAndExport`)。
   既定の日本語名は `nameJa`・`PokeTypeLabel` と一致する(`ShowdownNamingTests`)。
7. `TeamTextTransferViewModel`: 取り込めなかった行があっても取り込める分だけ追加でき(`needsDecision`)、0体なら確定できず、貼り付けの変更で
   結果を捨て、空入力は通信せず、通信失敗でも貼り付けを残して再実行できる(`TeamTextTransferViewModelTests`)。
8. `TeamEditViewModel.importMembers`: 末尾に追加し保存しない・空き枠まで(超えたら `.tooManyMembers`)・選択肢を用意・通信失敗でもメンバーを消さない
   (`TeamEditViewModelImportTests`)。
9. モックの架空データ(9001〜9004)で書き出し → 取り込みが往復する(`ShowdownMockEndToEndTests`)。
10. XCUITest(`TeamTextTransferUITests`・モック): 1体の書き出し(テキスト・コピー・共有)/ 全員の書き出し / 取り込めなかった行の一覧つきで
    取り込める分だけ追加 / 取り込めなかった行なしの追加 / 取り込める体が0のとき追加ボタンなし / やめると何も追加されない / 空入力の案内。
11. 既存の XCTest・XCUITest は1つも編集しない。`api/openapi.yaml`・`Generated/` は触らない。

### 8. 追加したテスト(spec 時点)

- 足場: `ShowdownText.swift`・`ShowdownTransfer.swift`・`TeamEditViewModel+Import.swift`(値は空・何もしない。`TODO(implementer` を検索)。
  既存ファイルの編集は無い。
- `Tests/PokeCalcCoreTests/`: `ShowdownTextLabelsTests`(4)・`ShowdownTextParserTests`(9)・`ShowdownTextSerializerTests`(7)・
  `ShowdownNamingTests`(1)・`ShowdownTransferServiceTests`(16)・`TeamTextTransferViewModelTests`(10)・`TeamEditViewModelImportTests`(5)・
  `ShowdownMockEndToEndTests`(2)、`Support/ConcurrencyProbeService.swift`(同時実行数を測る包み)
- `PokeCalcUITests/TeamTextTransferUITests.swift`(7)

`swift test`(`ios/PokeCalcKit`): 630 件中、新規 54 件のうち 49 件が失敗(失敗アサーション 239 個。すべて新規ファイル内。残り5件は
「空・nil を返す」ことを確かめる件で、足場でも通る)。既存 576 件は成功。`xcodebuild build-for-testing`(XCUITest 7件を含む)は
`** TEST BUILD SUCCEEDED **`。XCUITest の実行は未実施(identifier 未実装のため失敗する)。

### 9. 実装者への注意(`TODO(implementer` を検索すると該当箇所が見つかる)

- `TeamEditViewModel.team` は `private(set)`。`importMembers` は `TeamEditViewModel.swift` 側へ移すか内部の追加口を足す(既存の挙動・テストは変えない)。
- 名前 → ID の解決は並行数を上限つきで絞る(`TaskGroup` に同時数のセマフォ相当を入れるか、チャンクで回す)。`ConcurrencyProbeService` が上限超過を検出する。
  `Task.isCancelled` を見て以後の呼び出しをしない。`PokeCalcService` に ID で持ち物を引く API は無いので、書き出しは `searchItems` の先頭ページから名前を引く
  (引けなければ `unresolvedIds` に入れて行を省く)。
- パーサは「黙って捨てない」: 採らなかった行は必ず `rejected` に入れる。先頭行が ` @` で終わるときは持ち物なし。
  `Nickname (Species)` は末尾の `)` と ` (` で分ける(種族名に括弧がある場合は括弧の外側をニックネームにしない。マスタに括弧つきの名前が出たら再検討して ADR に追記)。
- View(`TeamEditView` に入口 `teamTextTransferButton`、`MemberCardView` に `exportMemberTextButton-<id>`、新しいシート)。`Menu` に入れない。
  `TextEditor` は `.scrollDismissesKeyboard` を付け、キーボードで「内容を確認」が隠れないようにする。コピーは `UIPasteboard`、共有は `ShareLink`。
  確認文は alert/confirmationDialog にしない(P6-7 で iOS 26 系の XCUITest が同じ identifier の入れ子2つを返し `Multiple matching elements` になった。カードで描く)。
- AX5(最大の文字サイズ)でシートが横にはみ出さないことを `LargeTextLayoutUITests` へのメソッド追加で確かめる(spec 時点では未作成。既存テストは編集しない)。
- `ios/README.md` の操作説明・`docs/plan.md`(P6-20 のチェック)・DECISIONS.md(確定した書式)を更新する。


### 10. 実装結果(P6-20。implementer)

- `swift test`(`ios/PokeCalcKit`): 630 件すべて成功(新規 54 件を含む)。`make ios-lint ios-gen-check ios-check-request-limits` 成功。
- `make ios-test-ui`: 全 61 件 / 成功 61 / 失敗 0 / スキップ 0(`TeamTextTransferUITests` 7 件と、AX5 の `LargeTextLayoutUITests.testTeamTextSheetNoHorizontalOverflowAtAX5` を含む)。
- 判断: `importMembers` は `private` な状態に触るため `TeamEditViewModel+Import.swift` を廃し `TeamEditViewModel.swift` に置いた。
  書き出しの技名は `moves(ids:)` を1回呼び、返らなかった ID だけ `searchMoves` の先頭ページで補う(`StubPokeCalcService` の既定は
  `moves(ids:)` が空を返すため、補わないと `ShowdownTransferServiceTests` の書き出し期待値が成り立たない。実 API でも名前を引けない ID を省く動作は同じ)。
  枠(`memberLimitExceeded`)は、名前を解決できたメンバーから先頭順に使う(種族を引けなかったメンバーは枠を使わない)。
  取り込めなかった行の `text` は、先頭行以外は `Ability: 名前` のように正規の書式で組み直す(元の空白は保持しない)。
- テストの矛盾: 期待値・テストの変更は無し(上の `moves(ids:)` の補いで解消)。
- critic 指摘対応: 書き出しで省いた項目(名前を引けなかった技・持ち物の件数 `exportUnresolvedCount`、種族を引けず飛ばした体数
  `exportSkippedMemberCount`)を書き出し結果の下に注意で表示(0 件なら出さない。identifier `exportUnresolvedNotice`/`exportSkippedNotice`)。
  `analyze` は解析中の二重実行をしない。全角空白(U+3000)を含む入力の取り込みテストを追加し、`-` の後の全角空白を許容。
  持ち物は ID 引き API が無い制約を ADR-0506 に明記。既存テスト・期待値の変更なし。

## P6-21 の受け入れ条件(iOS のタイプバッジ・エンブレム文字色を design.md「タイプバッジ」に合わせる。spec-writer: 受け入れ条件とテストのみ。実装はしない)

- 日付: 2026-10-02 / 担当レーン: iOS / 関連: docs/design.md「タイプ色」「タイプバッジ」(issue #306)、
  `web/src/styles/typeBadgeContrast.test.ts`・tokens.css の `--type-<id>-ink`、docs/plan.md P6-21
- 背景: design.md は文字色を「黒か白のうちタイプ色とのコントラスト比が高い方(白は どく/ゴースト/ドラゴン/あく の4つだけ。
  最小 4.59:1)」とし、iOS にも同名トークン(`typeInk(_:)`)を求める。iOS は `TypeBadgeView`・`SpeciesEmblemView` が `.white` 固定で、
  でんき(1.6:1 級)など14タイプが 4.5:1 に届かない。API・Generated は触らない。

### 1. トークンの仕様(判断)

- `TypeColorToken` に `ink(forTypeID:) -> RGBA?`(黒 `#000000` / 白 `#FFFFFF`。未知 ID・大文字は nil)、`inkColor(forTypeID:) -> Color?`、
  純粋関数 `preferredInk(over: RGBA) -> RGBA` を足す(design.md の `typeInk(_:)` に当たる。Swift の命名として `ink(forTypeID:)`
  にし、既存の `rgba(forTypeID:)`・`color(forTypeID:)` と並べた)。
- **判断: 18 タイプの文字色の表を二重に持たず、`rgba(forTypeID:)`(タイプ色の単一の正)から `preferredInk(over:)` で導出する。**
  `preferredInk` は WCAG 2.2 の相対輝度 `0.2126 R + 0.7152 G + 0.0722 B`(sRGB は `c <= 0.03928 ? c/12.92 : ((c+0.055)/1.055)^2.4`)で
  黒・白のコントラスト比を比べ、高い方(同値は黒)を返す。理由: タイプ色を変えたとき文字色が自動で追従し、
  「色を変えたのに文字色が古い」を防げる。design.md の表との一致は下のテストが担保する。
- Web との一致: iOS から `web/` は読めないため、テストには design.md の表を転記した期待値(白4タイプの集合)を置く。Web 側は
  tokens.test.ts が design.md と照合しているので、両者は design.md を介して同じ表を見る。design.md の文字色表を変えるときは
  tokens.css・Web テスト・本テストの表を同じ変更で直す。

### 2. View

- `TypeBadgeView`: 文字色を `TypeColorToken.inkColor(forTypeID:)`(未知なら従来の背景 `textSecondary` に対する既定として `.white`)に。
  `.lineLimit(1).fixedSize()`・padding・Capsule は維持。
- `SpeciesEmblemView` の頭文字: **既定案どおり ink に揃える**。背景がタイプ色のグラデーションで、でんき・こおり等の明るい色の上では
  白だと読めないため。2タイプのグラデーションは先頭タイプ(左上、頭文字の位置に近い側)の ink を使う。
  タイプ無し(`textSecondary` 単色)のときは従来どおり `.white`。
- カードのふち・ダメージバー等の色の使い方は変えない(スコープ外)。

### 3. 追加したテスト(`PokeCalcKit/Tests/PokeCalcDesignTests/DesignTokenTests.swift`。既存の期待値は不変)

- `testTypeInkMatchesDesignDocTableForAll18Types`(18 タイプの ink が design.md の表と一致)
- `testOnlyFourTypesUseWhiteInk`(白は poison・ghost・dragon・dark の4つだけ)
- `testTypeInkContrastIsAtLeast4_5ForAll18Types`(独立実装 `ColorContrast.swift` で 4.5 以上)
- `testTypeInkIsTheHigherContrastOfBlackAndWhite`(黒・白のうち高い方)
- `testMinimumTypeInkContrastIsAbout4_59`(最小 fighting 4.59±0.01、次 poison 4.79±0.01)
- `testPreferredInkSelectsByContrast`(黒→白、白→黒、#777777→黒、#757575→白(境界)、#707070→白)
- `testUnknownTypeIDHasNoInk`(未知・大文字は nil、`inkColor` の有無)

View の色そのものは単体で検証しづらいため、ink の選択を純粋関数に寄せて上で検証した。XCUITest は追加しない。

### 4. 受け入れ条件(検証可能な形)

1. 18 タイプすべてで ink のコントラスト比が 4.5 以上。
2. 白は poison・ghost・dragon・dark のみ。他14は黒(design.md の表と一致)。
3. 最小コントラストは fighting の約 4.59、次が poison の約 4.79。
4. 未知 ID・大文字 ID の ink は nil。
5. `TypeBadgeView` の文字色が ink。1行・自然幅(`.lineLimit(1).fixedSize()`)は維持。
6. `SpeciesEmblemView` の頭文字色が先頭タイプの ink。
7. 既存テスト・accessibilityIdentifier・`api/openapi.yaml`・Generated は不変。

### 5. spec 時点の結果(2026-10-02)

追加 7 テスト(`swift test --filter PokeCalcDesignTests`)。足場が常に黒を返すため、**37 個のアサーションが失敗**
(白4タイプの一致・高い方の選択・純粋関数の黒背景→白など。失敗は新規 7 テストのうち 5 件)。既存は全件成功(`swift test` 全体は 576 件+新規 7 件のうち上記のみ失敗)。

### 6. 実装者への注意(`TODO(implementer P6-21` を検索)

- `PokeCalcDesign.swift` の `preferredInk(over:)` と `ink(forTypeID:)` の足場(常に黒)を、上記 §1 の導出に置き換える。
  相対輝度の式を `ColorContrast.swift`(テスト用)からコピーせず、製品側に1箇所だけ置く(`precondition(alpha == 1.0)` 相当は不要、タイプ色は不透明)。
- `CalcScreenCards.swift` の `TypeBadgeView`・`SpeciesEmblemView` を §2 のとおり直す。エンブレムは `types.first` の ink を使う。
- `ios/README.md`/`docs/plan.md`(P6-21 のチェック)を更新。design.md は変更不要(「`typeInk(_:)`」と書いてあるが Swift 側の名前は `ink(forTypeID:)`。差が気になるなら design.md の文言を実装者が直す)。

### 7. 実装結果(2026-10-02)

- `TypeColorToken.preferredInk(over:)`(WCAG の相対輝度式を製品側に1箇所。同値は黒)と `ink(forTypeID:)`(`rgba(forTypeID:)` から導出)を実装。
  `TypeBadgeView` の文字色と `SpeciesEmblemView` の頭文字色(先頭タイプの ink)を ink に。未知 ID・タイプ無しは従来の白。
- design.md の `typeInk(_:)` に当たる Swift 側の名前は `ink(forTypeID:)`(design.md は変更しない)。`ios/README.md` に追記すべき項目は無い。
- 結果: `swift test` 全件成功(新規 7 件を含む)。critic PASS(軽微のみ。plan.md の項目追記と本章の更新を反映)。

## P8-1c の受け入れ条件(ポケモン画像。2026-10-03。設計は ADR-0508、契約は ADR-0808)

### 受け入れ条件(検証可能な形)

- **AC-X(最優先)**: 画像が無い現状(モックの既定・manifest 404)で、既存の `swift test`・全 XCUITest が無変更で通る。モックの既定は画像なし(`POKECALC_MOCK_IMAGES` が無い/`1` 以外)。
  既存テスト・identifier は変えない。
- **AC-1 manifest**: `{"version":1,"images":{…}}` を decode し、キー・サイズ(thumb/detail)で相対パスを引ける。version が 1 以外・欠落・不正 JSON・形違い・`images` 欠落は throw せず画像なし。
  未知の欄は無視し、1エントリの型違いが他のエントリを巻き込まない。
- **AC-2 URL**: 基点 URL(末尾スラッシュ・パス接頭辞の有無を問わず)+ `/images/` + 相対パス。空・先頭 `/`・`..`・scheme 付き・`//host`・`?` `#` `\`・制御文字は画像なし。基点と同じ scheme/host/port。
- **AC-3 カタログ**: manifest の取得は `<基点>/images/manifest.json` への1回(同時・繰り返しの引き当てでも1回。失敗も保持し再取得しない)。200 以外・通信失敗・不正本文・version 違い・危険なパスは throw せず画像なし。キーが manifest に無ければ画像なし。
- **AC-4 モック**: 既定は全キー画像なし。`POKECALC_MOCK_IMAGES=1` では 9001-000・9003-000 だけ、外部通信しない data URL の小さな架空 PNG(9002-000 は画像なしのまま)。
- **AC-5 表示判定**: 読み込み成功のときだけ画像、読み込み中・失敗はエンブレム。nil・空・空白のキーは問い合わせない。
- **AC-6 画面**: 画像ありのとき、計算の種族ヘッダー・種族検索の行に `speciesImage-<key>` の枠が出て、manifest に無いキーの行はエンブレムのまま出る。選択の挙動は変わらない。
  AX5 でも枠が画面幅に収まり、ヘッダーのボタンが操作できる。画像は装飾(label 無し)。アニメーションを付けない。
- **AC-7 契約**: 画像取得に X-Device-Id を付けない。`api/openapi.yaml`・Generated は不変。ATS の例外を足さない。実画像・公式画像をコミットしない。

### 追加したテスト

- `PokeCalcKit/Tests/PokeCalcCoreTests/ImageManifestTests.swift`(15 件: manifest の decode 11・URL 組み立て 4)
- `PokeCalcKit/Tests/PokeCalcCoreTests/ImageCatalogTests.swift`(15 件: カタログ 10〈成功・キー無し・取得先・非 200・不正・通信失敗・危険パス・1回取得・失敗の保持・NoImage〉、モック 3、表示判定 2)
- `ios/PokeCalcUITests/SpeciesImageUITests.swift`(5 件: 画像なしの既定〈AC-X〉・計算ヘッダー・検索行・選択・AX5)
- 足場: `PokeCalcKit/Sources/PokeCalcCore/ImageCatalog.swift`(全部「画像なし」を返す。`TODO(implementer P8-1c` で検索)

### spec 時点の結果(2026-10-03)

追加 30 件の単体テストのうち 15 件が失敗(アサーション 23 件。足場が画像なしを返すため)。既存 1128 件は全件成功(`swift test` 全体 1158 件)。
XCUITest の追加 5 件はコンパイルのみ確認(構文解析)。画像ありの 4 件は identifier が無いので失敗する想定。`testNoImageFramesByDefault` は現状でも通る。

### 実装者への注意

- 足場の TODO を実装する: `ImageManifest(decoding:)`・`PokeImageURL.url`・`RemoteImageCatalog`・`URLSessionImageManifestFetcher`・`MockImageScenario`/`MockImageCatalog`(PNG はコードで生成。数十バイト)・`SpeciesImageDisplay`。
- App 側: `SpeciesImageView` を足し、`SpeciesEmblemView` の3か所(`CalcScreenCards`・`MasterSearchSheet`・`BalanceMemberCard`)を置き換える。`CoreServices.images`(既定 `NoImageCatalog`)・Environment の注入は ADR-0508 §4。
  `AppEnvironment.makeAtLaunch` はモックなら `MockImageCatalog(environment:)`、API なら `RemoteImageCatalog`。`.ready(core:features:)` の形・既存 identifier は変えない。
- `speciesImage-<key>` を XCUITest から見えるようにする(`accessibilityHidden(true)` にすると見えない。ADR-0508 §6)。見えない場合はテストを弱めずに公開の仕方を直す。
- 同期の `FakeFetcher`(テスト)は 20ms 待つので、取得を並行で1回に束ねる(actor の in-flight Task を保持する)実装でないと「1回」が通らない。
- ADR-0507 の AppFeature は触らない(画像は画面ではない)。`ios/README.md` に `POKECALC_MOCK_IMAGES` を追記する。plan.md の更新は実装者(本タスクでは触っていない)。

### 実装結果(2026-10-03)

- `swift test` 1158 件・失敗 0(追加 30 件が全件成功。既存テスト無変更)。`make ios-lint ios-gen-check ios-check-request-limits` 成功、`xcodebuild build-for-testing` 成功。
- `SpeciesImageUITests` 5 件を iPhone 18 Pro シミュレータで実行し 5 件成功(全体の XCUITest は別途)。
- 実装: `ImageCatalog.swift` の TODO 実装(PNG は 8x8 単色をコードで生成)、`SpeciesImageView.swift`(`\.imageCatalog` Environment・`AsyncImage`・アニメーション無し)、
  `SpeciesEmblemView` の3か所を置き換え(`SpeciesHeaderMenuLabel` 経由で逆算・調整・構築編集のヘッダーも画像対応)、`CoreServices.images`(既定 `NoImageCatalog`)、`RootView` で Environment 注入。
- 判断: `accessibilityHidden` は使わず、成功した画像に `.accessibilityElement(children: .ignore)` + identifier を付けて公開したところ XCUITest から見えた(ADR-0508 §6 のリスクは発生せず)。
  URL 検証は `%` と `:` も拒否(相対パスに現れない文字。エンコード回避・scheme 偽装の防止)。

## お気に入り・計算履歴の受け入れ条件(2026-10-04。設計は ADR-0511、契約は ADR-0227。spec-writer: 受け入れ条件とテストのみ。実装はしない)

### 受け入れ条件(検証可能な形)

- **AC-X(最優先)**: 既存の `swift test`・全 XCUITest が無変更で通る。モックの既定(`POKECALC_MOCK_FAVORITES` 未設定)は空のストアで、既存画面は何も変わらない。
  既存テスト・identifier は変えない。`api/openapi.yaml` は触らない。
- **AC-1 API 写像**(`APIFavoritesServiceTests`): 一覧は `GET /api/record/favorites`(クエリ無し)、作成は `POST`(本文は `label?` と `individual` だけ。
  `id`・時刻を送らない・`moveId` を送らない)、削除は `DELETE /api/record/favorites/{id}`(本文なし)。すべて `X-Device-Id`・`X-Session-Id` を付ける。
  一覧はサーバーの順を保ち、`label` null は nil。作成は 201→`created`・200→`alreadyPinned`。204 は成功。400・404・500・503(`store_unavailable`/`upstream_unavailable`)・
  通信失敗は `PokeCalcError` に code のまま写す(上限到達の 400 `invalid_input` を握りつぶさない)。
- **AC-2 ラベル**(`FavoriteLabel.normalize`): 前後空白除去・空は未設定・30コードポイントに切り詰め(絵文字も1と数える)。送る前に必ず通す。
- **AC-3 一覧 ViewModel**(`FavoritesViewModel`): 名前をサーバーの順のまま解決し、引けない種族・マスタの失敗でも行を残す(題名は label → 種族名 → 「不明なポケモン」)。
  空は `.loaded`(失敗と区別)。失敗は `.failed(種類)`、再試行で回復、再読み込みの失敗は表示中の一覧を消さない。新しい読み込みが古い応答(成功も失敗も)を捨てる。
  キャンセルは一覧も状態も変えず失敗にしない。サービス nil は何もしない。100件で `isAtLimit`。
- **AC-4 外す**: 成功・404 は一覧から除く(404 は失敗を見せない)。それ以外の失敗は一覧を保ち `actionError`(読み込み状態は壊さない)。成功で前の `actionError` を消す。
  外している最中の同じ ID の再タップは要求を増やさない。外した項目は、削除前に始まった読み込みの古い応答で復活しない。
- **AC-5 追加 ViewModel**(`FavoritePinViewModel`): ラベル無しで個体をそのまま1回送る。`pinned`/`alreadyPinned`/`failed(種類)`。保存中の再呼び出しは無視。
  キャンセルは `.idle`(失敗にしない)。`reset()` は保存中以外で `.idle`。サービス nil は `isAvailable == false` で何もしない。
- **AC-6 計算履歴**(`OpponentHistoryViewModel`。データ源は既存の `FrequentOpponentsService`): `limit` は 50。サーバーの順・件数・最終計算時刻を保つ。
  名前を引けない相手は行を残す(ピッカーと違う)。同じ key は先頭だけ。空は `.loaded`。失敗の扱い・古い応答の破棄・キャンセルは AC-3 と同じ。
  「3回」「最後: 今日/昨日/N日前」は暦日(Calendar)の差で、未来は今日。生の履歴一覧は契約待ちと画面に注記する。
- **AC-7 エラー文言**(`RecordScreenError`): code→種類(transport・decode/unexpectedStatus→unexpectedResponse・503 2種→storeUnavailable・not_found・invalid_input・その他)。
  文言は `FavoritesLabels` に集約した日本語で、サーバーの英語 `message` と `code` を含まない。503 は「計算はそのまま使えます」を含む。上限の文言は `RequestLimits.maxFavorites` から作る。
- **AC-8 契約同期**: `RequestLimits.maxFavorites`(=listFavorites の maxItems)・`maxFavoriteLabelLength`(=FavoriteInput.label.maxLength)が
  `ios/scripts/check-request-limits.sh` で契約と一致する。
- **AC-9 モック**(`MockFavoritesService`): 既定は空のストアで、追加(新しい順・同じ内容は `alreadyPinned` で先頭へ)・削除(2回目 404)が動く。
  `list`(102「HB特化」9002-000・101 9003-000)/`fail`(transport)/`unavailable`(503)/`full`(100件・新規は 400 `invalid_input`)。架空の 9xxx だけ。
- **AC-10 画面(XCUITest・モック)**: ルートのピル `openFavoritesScreen` → `favoritesScreen`。`favoritesSection`(`favoritesEmpty`/`favoriteRow-<id>`/`favoriteDeleteButton-<id>`/
  `favoritesError`+`favoritesRetryButton`/`favoritesActionError`)・`opponentHistorySection`(`opponentHistoryRow-<key>`/`opponentHistoryEmpty`/`opponentHistoryError`+`opponentHistoryRetryButton`/
  `opponentHistoryPendingNote`)。計算画面に `pinAttackerFavoriteButton`・`pinDefenderFavoriteButton`・`favoritePinStatus`。
  通信失敗・503・空・上限でも画面を壊さず、計算は成功する(絶対ルール5)。外すボタン 36pt 以上。AX5 で横にはみ出さず外すボタンを押せる。

### 追加したテスト

- `PokeCalcKit/Tests/PokeCalcCoreTests/APIFavoritesServiceTests.swift`(14 件)
- `FavoritesViewModelTests.swift`(18 件)・`FavoritePinViewModelTests.swift`(9 件)・`OpponentHistoryViewModelTests.swift`(14 件)
- `RecordScreenErrorTests.swift`(6 件。ラベル正規化を含む)・`MockFavoritesServiceTests.swift`(8 件)
- 支援: `Support/StubFavoritesService.swift`
- `ios/PokeCalcUITests/FavoritesScreenUITests.swift`(13 件)
- 契約同期: `ios/scripts/check-request-limits.sh` に maxFavorites・maxFavoriteLabelLength を追加(`ios-check-request-limits` が通る)
- 足場(`TODO(implementer P5-3c iOS` で検索): `Favorites.swift`・`OpponentHistory.swift`・`MockFavoritesService.swift`・`APIPokeCalcService+Favorites.swift`、
  `RequestLimits` の2定数。文言 `FavoritesLabels` は確定値

### spec 時点の結果(2026-10-04)

追加した単体テスト 69 件〈上の内訳の合計〉のうち 66 件が失敗する想定どおり(足場が空・throw のため)。`swift test` 全体は 1237 件・既存 1158 件は全件成功。
`make ios-gen-check` 成功・`ios/scripts/check-request-limits.sh` 成功。XCUITest 13 件は `xcodebuild build-for-testing` でコンパイルのみ確認(identifier が無いので実行すると失敗する想定)。

### 実装者への注意

- 足場の `TODO(implementer P5-3c iOS` を実装する。API 写像は `FrequentOpponentsService` の extension(`APIPokeCalcService.swift` 末尾)を見本に、
  `client.listFavorites` / `createFavorite`(201/200 を `Output` で区別)/ `deleteFavorite`(204)を呼ぶ。`generatedIndividual` を再利用し、生成型 → ドメインの
  `Individual` 逆写像(SpeciesKey・StatBlock・teraType の `value1` 包み)を足す。`moveId` は契約に無いので送らない・読まない。
- 画面: `ios/PokeCalc/Features/FavoritesFeature.swift`(`AppFeature`。`registerServices` で `FavoritesService` をモック/API で登録、`requiredServices` に入れる)と
  `FeatureRegistry` の1行だけ。`RootView`・`AppEnvironment` の `.ready` は編集しない。履歴は `context.core.frequentOpponents`・名前の解決は `context.core.pokeCalc`。
  計算画面の追加ボタンは `context.services.resolve((any FavoritesService).self)` を `CalcScreenView` に任意で渡す(`frequentOpponentsService` と同じ流儀)。
- **見た目の規約**: design.md のトークンのみ・`lineLimit` を付けない・常時アニメ無し・タップ範囲 36pt 以上。Menu の中の identifier は外から見えにくいので、
  外すボタンは Menu に入れず直接のボタンにする。**子が1つだけの `.accessibilityElement(children: .contain)` は中の識別子を畳む**ので、
  セクションは見出し+中身で子を2つ以上にする(空のときも見出し+案内文)。行の `favoriteRow-<id>`・`opponentHistoryRow-<key>` は label に題名・件数を含める。
  AX5 では横並びを縦積みに切り替える(`dynamicTypeSize >= .accessibility1`。他画面と同じ)。
- 文言は `FavoritesLabels` に集約済み。画面に直書きしない。件数の数字も `RequestLimits` から。
- `README.md`(ios)に `POKECALC_MOCK_FAVORITES`・`POKECALC_OPEN_FAVORITES_SCREEN_AT_LAUNCH` を追記。ルートのピルが1つ増えるので、既存 UI テストのスクロール位置を壊さないか全 XCUITest で確認する。
- 実在ポケモンの実データをフィクスチャにしない(9001〜9004 だけ)。`docs/plan.md` の更新は依頼元(本タスクでは触っていない)。


### 実装結果(2026-10-04)

- `swift test` 1237 件・失敗 0(spec 時点の失敗 66 件が全件成功。既存 1158 件は無変更で成功)。`make ios-lint ios-gen-check ios-check-request-limits` 成功。
  `xcodebuild build-for-testing`(generic/platform=iOS Simulator)成功。
- `FavoritesScreenUITests` 13 件を iPhone 18 Pro シミュレータで実行し 13 件成功(テストは無変更。全体の XCUITest は別途)。
- 判断: 計算画面の追加ボタンは、既存テストの位置を動かさないよう結果セクションの**下**に置いた(`FavoritePinSection`)。防御側の個体は計算が種族だけで防御側を指定するため、
  無補正の性格・SP 0・画面で選んだ特性の個体として追加する(`CalcViewModel.defenderIndividualForFavorite()`。攻撃側は計算に使う個体そのまま)。
  名前の解決は `SpeciesNameResolver`(お気に入り・履歴で共通)。外した ID は、その時点で進行中の読み込みの応答からも除く(復活防止)。
- テストと実装の矛盾は無かった。

## 防御側のランクの受け入れ条件(issue #274 の残り。2026-10-04。Web は ADR-0315、契約は ADR-0216。spec-writer: 受け入れ条件とテストのみ。実装はしない)

- 日付: 2026-10-04 / 担当レーン: iOS / 関連: 本 ADR「issue #274」「P6-19」、ADR-0315(Web)、ADR-0216(`defenderOverride.ranks/status`)、
  docs/ai-shared/decisions/2026-10-02-215-ios-web-ios-issue-274-adr-0315.md・同 206・同 2026-10-04-ios-defender-ranks.md
- 生成クライアント: `Components.Schemas.DefenderOverride.ranks`(`RankBlock`)は生成済み。`make ios-gen-check` が一致していれば再生成は不要。
  api/openapi.yaml・services/ は触らない。

### 判断(新しい ADR は起こさない)

1. **リセット規則 = 消さない**。防御側の種族変更・攻守入れ替え・技の変更・構築の呼び出し・「詳細」の開閉でランクを 0 に戻さない。
   根拠: Web の ADR-0315 §6、および攻撃側のランクの既存規則(本 ADR「issue #274」: ランクは入れ替えでも消さない)。
   防御側の特性(P6-19)だけが「種族が変わると選択肢が変わる」ので種族変更・入れ替えで「指定なし」に戻る。ランクは種族に依存しないので別規則。
2. **状態異常は出さない**(`defenderOverride.status` は送らない。式に効かない。ADR-0216 §3)。
3. **逆算には足さない**(`ReverseRequest` に防御側 override は無い)。

### 受け入れ条件

1. **表示と編集対象**: 「詳細」の「攻撃側のランク」の次に防御側のランク(見出し「防御側のランク」)を出す。ステッパーは選択中の技の分類で
   編集対象が決まる(物理・変化・技なし = def〈B〉、特殊 = spd〈D〉)。表示は「B +1」「D -2」「B ±0」(`RankLabel.text` を def → B、spd → D に広げる)。
   def / spd は別々に保持し、技の分類を往復しても消えない。範囲は -6..+6(`RankLimits`)で、範囲外の指定は丸める。上限・下限でボタンを無効にする。
2. **要求(ViewModel → `BulkCalcRequest.defenderRanks`)**: 既定(0・0)は既定の `RankBlock()`。値が変わる操作は計算1回、変わらない操作は0回。
   def/spd は技の分類に関係なく両方そのまま載せる(攻撃側の atk/spa と同じ)。攻撃側の `attacker.ranks`・防御側の特性と混ざらない。
3. **API 写像**(`APIPokeCalcService.generatedBulkCalcRequest`): `defenderOverride` の出し方を「abilityId も ranks も無いなら送らない」に変える。
   (abilityId, ranks)の4通り: なし/既定 → 送らない(従来と同じ本文)、あり/既定 → `{abilityId}` のみ、なし/非 0 → `{ranks}` のみ、
   あり/非 0 → 同じ `defenderOverride` に両方。ranks は非 0 のとき 5 項目(atk/def/spa/spd/spe。0 も含む)を送る。`status` は送らない。
   攻撃側の `attacker.ranks` は防御側のランクの影響を受けない。
4. **文言**(Core の `CalcConditionLabels`。既存の固定文言は変えない): `defenderRankTitle`「防御側のランク」、
   `defenderRankIncrement`「防御側のランクを上げる」、`defenderRankDecrement`「防御側のランクを下げる」(Web と同じ語)。
5. **identifier(追加のみ)**: `calcDefenderRankDecrement` / `calcDefenderRankValue` / `calcDefenderRankIncrement`
   (攻撃側の `calcAttackerRank*` と同じ流儀)。ステッパーのボタンのタップ範囲は 36pt 以上、AX5 でも横にはみ出さずタップできる。
6. **モック**: `MockPokeCalcService.calcBulk` は防御側のランク付きの要求を受け付け、ランクなしと同じ形の行を返す(数値は変えない)。
7. **古い応答**: ランクを続けて変えたとき、古い計算は追い越され(キャンセル)、最新の要求の結果だけが残る。計算は1操作につき1回。

### 追加したテスト(2026-10-04 時点。実装は未着手)

| ファイル | 件数 | 内容 |
|---|---|---|
| `ios/PokeCalcKit/Tests/PokeCalcCoreTests/CalcViewModelDefenderRanksTests.swift` | 13 | 既定・混ざらない・±6・技の分類 def/spd・変化技・リセットしない・特性と併存・古い応答 |
| `.../APIPokeCalcServiceDefenderRanksTests.swift` | 7 | `defenderOverride` の有無の全組み合わせ・5 項目・status 無し・攻撃側と混ざらない・他のフィールド不変 |
| `.../DefenderRankLabelsTests.swift` | 3 | 文言 3 つ・既存文言不変・「B +1」「D -2」「B ±0」 |
| `.../MockPokeCalcServiceDefenderRanksTests.swift` | 1 | モックが要求を受け付けて同じ形の行を返す |
| `ios/PokeCalcUITests/CalcDefenderRanksUITests.swift`(XCUITest・モック) | 4 | 既定「B ±0」・上げ下げ・上限下限で無効・36pt 以上 |
| `ios/PokeCalcUITests/LargeTextLayoutUITests.swift`(追加メソッド1本) | 1 | AX5 で防御側のランクがはみ出さずタップできる |

単体は 24 件追加(`swift test --filter DefenderRank`)。実装前の結果: 失敗 16・成功 8(成功は足場の既定値と同じ結果になるもの。
Labels の文言 2 件・`RankLimits`・モック 1 件・API 4 件〈従来どおり送らない/特性のみ/攻撃側不混入/他フィールド不変〉)。
XCUITest 5 件はビルドのみ確認(実行は実装後。View が無いので失敗する)。既存テストは1つも変えていない。

### 実装者への注意

- 足場(`TODO(implementer` を検索): `BulkCalcRequest.defenderRanks`(既定値付き。`DomainTypes.swift`)、`CalcViewModel` の
  `defenderRanks`/`defenderRankStat`/`defenderRank`/`defenderRankText`/`setDefenderRank`(中身は空。`attackerRank*` と同じ作りで実装)、
  `CalcConditionLabels` の文言(値は確定済み)。
- 実装箇所: ① `CalcViewModel.buildRequest` が `defenderRanks` を載せる。② `APIPokeCalcService.generatedBulkCalcRequest` の `defenderOverride` を
  「abilityId も ranks も無いなら nil」に変え、ranks は `RankBlock() != request.defenderRanks` のときだけ 5 項目で載せる(`status` は nil)。
  生成物の `RankBlock` のメンバーは Optional なので、0 も含めて全項目に値を入れる。③ `RankLabel.text`(def → B、spd → D。
  `AttackerPreset.statLetter` は private 化せず、攻撃側の A/C の対応を壊さない)。④ `CalcConditionsSection` に `defenderRankSection`
  (`rankStepperButton` を再利用。identifier は上記)。`CalcScreenMetrics.rankValueMinWidth` を再利用し、ボタンは 36pt 以上を保つ。
- リセット処理は足さない(判断 1)。`resetDefenderAbility()` にランクを含めない。
- 既存の API 写像テスト(`defenderOverride` を送らない)・既存 identifier は変えない。
- 完了条件: `swift test`・`make ios-test`(XCUITest 5 件を含む)・`make ios-gen-check`。結果をこの章の後ろに追記し、plan.md にチェックを付ける。

### 実装結果(2026-10-04。implementer)

- 単体 `swift test`: 1318 件・失敗 0。XCUITest(iPhone 18 Pro・モック): `CalcDefenderRanksUITests` 4 件・`LargeTextLayoutUITests.testCalcScreenDefenderRankNoHorizontalOverflowAtAX5` 1 件・既存 `CalcConditionsUITests` 2 件、計 7 件すべて成功。
- 逸脱 1(配置): 防御側のランクは「攻撃側のランクの次」ではなく「詳細」の末尾(防御側の壁の次)に置いた。攻撃側のランクの前後どちらに挟んでも、
  既存の `CalcConditionsUITests.testTogglingConditionsUpdatesSelectionAndKeepsRows`(前方スクロールのみで 壁→攻撃側ランク の順に触る)が、スワイプの量子化で攻撃側ランクへ届かなくなった(実測)。既存テストは変えない。
- 逸脱 2(テストの操作のみ): `CalcDefenderRanksUITests.scrollUntilHittable` に、前方スクロールで届かないときの `swipeDown` を追加(防御側のランクが末尾にあり、上の攻撃側ランクへ戻るため)。期待値・検証は変えていない。
- 逸脱 3(テストの操作のみ): `CalcViewModelDefenderRanksTests.testStaleResponseDoesNotOverwriteLatestRows` は cancel 済みの古い要求に `resolveBulkWithEcho` を呼んでいた(スタブは cancel で保留を外すため失敗)。
  その呼び出しを `waitForBulkCancellation(at:)` に置き換えた。検証(最新の結果だけが残る・エラー無し・isLoading false)は変えていない。
- ステッパーのボタンは最小 37pt(36 ちょうどは丸め誤差で 35.99 になり不合格)。攻撃側と共有。

- **既存テストの操作の追従(2026-10-04。防御側のランク)**: 防御側のランクの行を「詳細」の末尾に足したことで、画面の小さい機種(iPhone 17e)では既存の `CalcConditionsUITests.scrollUntilHittable` が上へスクロールし過ぎて上にある攻撃側のランクを通り越した(iPhone 18 Pro では通っていた)。上へのスクロールで届かないときだけ下へ戻る `swipeDown` を足した。最後の `XCTAssertTrue(target.isHittable)` は変えていない(検証は弱めていない)。両機種で `CalcConditionsUITests`・`CalcDefenderRanksUITests` が全件成功。

## 判定の素早さ反映/無視と状態異常の受け入れ条件(2026-10-04。判定画面は ADR-0504、設計は ADR-0512、契約 v0.3.0 は ADR-0710・0712・0714。spec-writer: 受け入れ条件とテストのみ。実装はしない)

### 受け入れ条件(検証可能な形)

1. **反映/無視の表示(行ごと・方向ごと)**: 各候補の行に、`attackerSpeedApplied` →「自分の素早さに反映: ランク補正・追い風」、`defenderSpeedApplied` →「相手の素早さに反映: …」、
   `attackerSpeedIgnored` →「自分の素早さに特性は反映していません」、`defenderSpeedIgnored` →「相手の素早さに持ち物・天候は反映していません」を出す(Web の `judgeScreenText` と同じ語。区切りは「・」)。
   自分側(attacker*)と相手候補側(defender*)を取り違えない。空配列の文は出さない(識別子も存在しない)。並びは応答のまま。行は `defenderIndex` で対応づける(応答が並び替わっていても他の行の値が混ざらない)。
   値の文言: rank=ランク補正・tailwind=追い風・ability=特性・choiceScarf=こだわりスカーフ・item=持ち物・paralysis=まひ/abilityId=特性・itemId=持ち物・fieldWeather=天候。既存の行の表示は変わらない。
2. **状態異常の入力**: 自分と各候補に「状態異常」ボタン(`judgeAttackerStatusButton`・`judgeCandidate<n>StatusButton`)。既定は「なし」。選択シートに7値(なし/やけど/まひ/どく/もうどく/ねむり/こおり。順は契約と同じ)が並び、
   「未選択に戻す」行(`judgeOptionNone`)は無い。選ぶとシートが閉じてボタンに名前が出る。選択は自分・候補ごとに独立で、候補の削除では候補について行く。
3. **送信規則**: `none`(既定)は要求に載せない(欄ごと省略・null も送らない)。選んだ値だけ契約の値(`badly_poison` はスネークケース)で、**その側にだけ**載せる(自分の値が候補に、候補の値が自分に載らない)。
   状態異常を選ばない要求は従来と同じ本文・同じ `JudgeRequest`。状態異常だけでは送れない(検査は変わらない)。入力の編集で judge を呼ばない。
4. **リセット**: 種族を変えても状態異常は残す。構築のメンバーを呼び出すと `none` に戻る。
5. **未知の値で落ちない**: 応答の `*SpeedApplied`・`*SpeedIgnored` に契約の既知の値以外が来ても decode は失敗せず、文字列のまま・並べ替えずに運ぶ(他の欄・他の行・方向は正しく読める)。
   画面は未知の値を「その他の補正」「その他の入力」と出す。欄が無い・配列でない・文字列でない要素・読めない本文は従来どおり decode エラー(厳格さを保つ)。並行の呼び出しで値が混ざらない。ErrorCode の未知値は従来どおり(decode エラー)。
6. **契約同期**: `SpeedFactor`(6値)・`SpeedIgnoredInput`(3値)・要求の status(自分・候補とも7値)の生成 enum が、ドメイン・文言の対応表と一致する(増えたら落ちる=文言を足す合図)。
7. **モック**: 既定・`marks` は4欄とも空。`speed-notes` は固定値(自分 Applied = 全行 [rank, tailwind]・自分 Ignored = 2番目の候補だけ [abilityId]・候補 Applied = 1番目だけ [choiceScarf]・候補 Ignored = 2番目だけ [itemId, fieldWeather])。
   要求の状態異常がまひなら、その側の Applied の末尾に paralysis(自分なら全行、候補ならその候補の行だけ)。まひ以外は何も足さない。素早さ・確定数は既定と同じ。
8. **画面(XCUITest)**: 上の 1〜4・7 を `speed-notes` のモックで確かめる。AX5 で状態異常のボタン・選択シート・結果の4つの文が横にはみ出さない。**iPhone 17e と iPhone 18 Pro の両方で通る**(スクロールは前方→届かなければ下へ戻る)。

### 追加したテスト(2026-10-04 時点。実装は未着手)

| ファイル | 件数 | 内容 |
|---|---|---|
| `ios/PokeCalcKit/Tests/PokeCalcCoreTests/JudgeViewModelStatusTests.swift` | 11 | 既定・7値の順・独立・範囲外・編集で呼ばない・種族変更で残る・候補削除で付いて行く・構築でリセット・none は送らない/本文不変・全値を自側だけに送る・検査不変 |
| `.../JudgeLabelsSpeedNotesTests.swift` | 8 | 既知の値の文言・未知の値のフォールバック(大文字小文字も区別)・自分/相手・「・」区切り・反映/無視の文・状態異常の語・既存文言不変 |
| `.../JudgeResultDisplaySpeedNotesTests.swift` | 8 | 空は nil・方向を取り違えない・片側だけ・並び保持・行ごと・index 引き・未知値・既存の表示不変 |
| `.../JudgeContractSyncSpeedTests.swift` | 6 | SpeedFactor 6値・SpeedIgnoredInput 3値・全値に文言・status 7値(自分/候補)・状態異常の文言 |
| `.../APIJudgeServiceStatusTests.swift` | 6 | nil は省略/null なし・全値を自分に/候補に・badly_poison・候補ごと・他の任意欄と並ぶ |
| `.../APIJudgeServiceUnknownSpeedValuesTests.swift` | 13 | 未知の Applied/Ignored で落ちない・他の欄は読める・行/方向ごと・既知値・空・欄欠落/null/非配列/非文字列/本文不正は decode エラー・ErrorCode 未知値は従来どおり・要求不変・並行で混ざらない |
| `.../MockJudgeServiceSpeedNotesTests.swift` | 6 | 環境変数・固定値・まひの付与(自分/候補/位置)・他の状態異常は足さない・他のシナリオは空・数値不変 |
| `ios/PokeCalcUITests/JudgeSpeedNotesUITests.swift`(XCUITest・モック) | 8 | 状態異常の既定/シート/選択・自分と候補で独立・反映/無視の表示(行ごと・空は出さない)・既定は何も出ない・既存の値不変・候補のまひ/自分のまひが該当側の行だけに届く・まひ以外 |
| `ios/PokeCalcUITests/LargeTextLayoutUITests.swift`(追加メソッド2本) | 2 | AX5 で状態異常ボタン・反映/無視の4文・選択シートがはみ出さない |

単体 58 件追加(`swift test` の新規 7 ファイル)。実装前の結果: 失敗 32・成功 26(成功は足場の既定値と同じ結果になるもの: 既定値・none 非送信・契約の値の集合・「厳格さを保つ」系・他シナリオの空)。
全体は 1376 件中、新規の失敗 32 のみ(既存テストの失敗 0)。XCUITest 10 件は、実行は UI 実装後(下の「XCUITest の実装前結果」)。既存テスト・既存 identifier は1つも変えていない。

### 実装者への注意

- 足場(`TODO(implementer` を検索): `JudgeStatus`(完成。`JudgeIndividual.status`・`JudgeDraft.status`)、`JudgeViewModel.setStatus`(中身は空)と `individual(from:)`(status を渡す)、`JudgeLabels` の状態異常・素早さの文言(値は確定済み。中身は空)、
  `JudgeMatchupDisplay` の4つの文(既定 nil。`JudgeResultDisplayBuilder.row` で埋める)、`MockJudgeService` の `.speedNotes`(環境値 `speed-notes`。固定値は上の条件 7)、
  `APIJudgeService.init(serverURL:transport:identity:)`(足場は素の `Client`。ここでミドルウェアを付ける)。
- 実装箇所: ① `APIJudgeService.generatedIndividual`/`generatedDefender` が `status` を `Components.Schemas.Individual.StatusPayload(rawValue:)` / `DefenderCandidate.StatusPayload(rawValue:)` で載せる(nil は載せない)。
  ② ADR-0512 §3 のミドルウェア(判定専用。`init(baseURL:identity:)` も同じ経路にする。既存の `init(client:identity:)` は変えない=既存テストが使う)。`APIJudgeService.domainMatchup` は退避した生の文字列を使う。
  退避は `@TaskLocal` の参照型(呼び出しごと)。**生成の `Client` は `Sendable` で共有されるので、ミドルウェアにインスタンスの可変状態を持たせない**(並行テストが検出する)。
  ミドルウェアが触るのは 200 の JSON だけで、4配列が「文字列の配列」のときだけ退避して空配列にする。それ以外(欄なし・null・非配列・非文字列要素・JSON でない)は本文をそのまま渡し、生成の decode に落とさせる。
  ③ `JudgeResultDisplayBuilder` が `JudgeLabels.speedAppliedNote`/`speedIgnoredNote` で4つの文を作る(空配列は nil)。④ View: `JudgeIndividualCard` に「状態異常」ボタン(持ち物の次。`selectButton` を再利用)、
  `JudgeOptionKind.status`(シートの行は `JudgeStatus.allCases`・`judgeOptionNone` は出さない・選んだら `setStatus`)、`JudgeResultSection` の行に4つの文(識別子は ADR-0512 §6。空は出さない)。
  結果の `.contain`(`judgeResult`)は子を2つ以上に保つ(子が1つだけの `.contain` は識別子を畳む)。文は行の `.contain` の中の個別の Text にする。
- `MockJudgeService` の `.speedNotes` は行の位置 i から決まる値で、既存の素早さ・確定数の式は変えない。`JudgeMatchup`/`JudgeMatchupDisplay` の init は既定値付きなので既存の呼び出しは変わらない。
- 画面の高さは機種で違う(iPhone 17e は小さい)。新しい XCUITest は前方スクロール後に下へ戻る操作を持つ。View を足したあと、**両機種**で `JudgeSpeedNotesUITests`・`LargeTextLayoutUITests` の追加2本・既存の `JudgeScreenUITests` を通す
  (状態異常のボタンを足すと個体カードが伸びる。既存の `scrollUntilHittable` が通り越したら、操作だけ直す。検証は変えない)。
- 完了条件: `swift test`・`make ios-test`(両機種の XCUITest を含む)・`make ios-gen-check`・`make ios-lint`。結果をこの章の後ろに追記し、plan.md にチェックを付ける。

### XCUITest の実装前結果(2026-10-04。View 未実装)

`JudgeSpeedNotesUITests` 8 件を iPhone 17e と iPhone 18 Pro の両方で実行: 各機種で失敗 6・成功 2(成功は `speed-notes` でない既定のシナリオで何も出ないことと、既存の行の値が変わらないこと)。失敗は状態異常のボタン・反映/無視の文が無いため。
`LargeTextLayoutUITests` の追加 2 本はビルドのみ確認(実行は UI 実装後)。`make ios-gen-check`(生成物は契約と一致。再生成は不要)・`make ios-lint` は成功。

### 実装結果(2026-10-04)

- 単体: `swift test` 1376 件・失敗 0(新規 58 件を含む)。`make ios-lint`・`ios-gen-check`・`ios-check-request-limits` 成功。`xcodebuild build-for-testing` 成功。
- XCUITest(iPhone 17e・iPhone 18 Pro の両方): `JudgeScreenUITests` 17 件・`JudgeSpeedNotesUITests` 8 件・`LargeTextLayoutUITests` の判定の AX5 追加 2 本、計 27 件ずつ全件成功(失敗 0)。テスト側の操作は変えていない。
- 実装: `JudgeSpeedValueMiddleware`(判定専用の `ClientMiddleware`。200 の 4 配列を `@TaskLocal` の `JudgeSpeedValueCapture` へ生の文字列で退避し、生成型には空配列で渡す。ミドルウェア自身は状態を持たない。
  4 配列が文字列の配列でない行は触らず従来どおり decode エラー)。`APIJudgeService` の `init(serverURL:transport:identity:)` と `init(baseURL:identity:)` が同じ経路でミドルウェア付きの `Client` を作る(`init(client:identity:)` は不変)。
  `Package.swift` は変更なし(`HTTPTypes` は OpenAPIRuntime 経由で参照できる)。
- 既存の `scrollUntilHittable` が状態異常ボタンの追加で通り越す問題は起きなかった(テストの操作変更なし)。

## 攻撃側の SP・性格・技の絞り込みの受け入れ条件(F-01 / I-ios-1・I-ios-5。2026-10-05。判断は ADR-0518)

Web(ADR-0329・ADR-0328)に揃える。受け入れ条件とテストが先で、実装は後続。engine・API 契約・services は変えない。

### 受け入れ条件

1. **2 ブロックの入力**: 計算画面の「技セレクタ」と「詳細」の間に「攻撃」「特攻」の 2 ブロックが常に出る。各ブロックは SP の数値欄(既定 `0`)・性格補正の 3 択(上昇・補正なし・下降。既定は補正なし)を持つ。
   選んだ技が使う側(物理・変化 = 攻撃、特殊 = 特攻)の見出しに「(この技で使用)」を文字で足す(技が無ければどちらにも付けない)。タップ範囲は 36pt 以上。
2. **要求の組み立て**: 攻撃と特攻の SP を**両方**要求に載せる(技が使わない側もそのまま。H・B・D・S は 0。合計は最大 64 で 66 を超えない)。既定のままの要求は従来と同一(無補正・SP 0)。
   性格はマスタの性格一覧から Web と同じ規則で解決する(両方補正なし → 一覧の最初の無補正 / 使う側が上昇でもう一方が補正なしなら +A/−C・+C/−A の代表性格を優先 / 両方に合う性格を ID 昇順で最初 / 無ければ使う側だけ / 無ければ失敗)。
3. **プリセットとの連動**: プリセット(ピルの行。識別子・位置は不変)を選ぶと使う側のブロックに値が入る(無振り = 0・補正なし、特化 = 32・上昇、振り = 32・補正なし)。もう一方は変えない。
   ピルの選択状態は使う側のブロックの値から導く(数値で比較。一致しなければ選択なしで「カスタム」の印)。数値を変えても補正は変わらず、補正を変えても数値は変わらない。
   プリセットの値は `AttackerPreset.build` の要求と一致する(物理・特殊とも)。
4. **同じ向きにできない**: もう一方が上昇なら、こちらの「上昇」と「特化」のピルを選べない(下降・補正なしは選べる)。下降も同様。理由の一文を添え、もう一方の値を黙って書き換えない。選べない操作は計算しない。
5. **不正入力は丸めず、計算しない**: SP は 10 進整数 0〜32 のみ(前後の空白は無視・先頭の 0 は可・空欄は 0。符号・小数点・指数・全角・33 以上は不正)。不正な欄は文字列を保ち、入力の近くに「攻撃のSPは0〜32の整数で入力してください」(特攻も同様)を出す。
   計算要求を送らず、古い行も出さず、待っている古い応答も捨てる(`error` は立てない)。技が使わない側が不正でも同じ。直せば計算し直す。性格が解決できないときも計算せず `natureUnavailable` のエラーに「この性格補正の組み合わせに当たる性格が、データにありません」を出す。
6. **値の寿命と回数**: 値が変わる操作は calcBulk をちょうど 1 回(変わらない操作・選べない操作は 0 回)。技・攻撃側/防御側の種族・攻守入れ替えで両ブロックの値(不正な文字列も)を消さない。古い応答は新しい入力の結果を上書きしない。
7. **構築・お気に入りとの関係**: 構築の個体を呼んでいる間はその個体の SP・性格で計算し、入力欄の値は使わず書き換えず、不正でも止めない。入力欄・補正・ピルのどれを操作しても構築の選択は外れ(特性も外れる)、入力欄の値で計算する。
   お気に入りに入れる攻撃側は入力から解決した個体(SP は両方。不正・性格なしは `attackerSPInvalid`・`natureUnavailable` で作れない)。
8. **技の選択肢はダメージ技だけ**: 計算画面の `moveOptions` に変化技が出ない(検索語に変化技の名前を入れても出ない)。`selectMove(id:)` は変化技を無視して計算しない。種族を替えても変化技を選ばない。
   構築から呼んだ個体の技が変化技だけでも、既定のダメージ技にする。ダメージ技を 1 つも覚えない種族は技欄を空にして案内(`noDamagingMovesNotice`)を出し、計算せず、エラーにもしない(替えれば計算が走る)。
   status-move の安全網(`isStatusMoveSelected` のとき計算せず `statusMoveNotice`)は `CalcMoveRules.isStatusMove` で固定する。逆算・調整は変更なし、判定と構築編集の 4 技は絞らない。
9. **文言・識別子・見た目**: 文言は Core の `AttackerStatLabels`(Web の `ja.ts` と同じ語)。識別子は追加のみ(ADR-0518 §5)。トークンのみ・`lineLimit` なし・`Menu` 禁止・常時アニメ無し。AX5 では性格補正の 3 択を縦に積み、横にはみ出さない。
   `.contain` のコンテナは子を 2 つ以上にする。数値キーボードのツールバーに「完了」(`calcKeyboardDone`)を置く。

### 追加したテスト(足場。いまは失敗する)

- **単体(65 件追加)**: `AttackerStatInputTests` 24 件(`parseSP`・プリセット ⇔ ブロック・`AttackerPreset.build` との一致・同じ向きの無効化・性格の解決〈代表性格の優先・ID 昇順・フォールバック・解決失敗〉・要求の組み立て・技の絞り込み関数)/
  `AttackerStatLabelsTests` 6 件(文言を Web と固定)/ `CalcViewModelAttackerStatsTests` 35 件(既定・SP 入力・補正・プリセット連動・同じ向き・不正入力・古い応答の破棄・値の寿命・構築との関係・お気に入り・技の絞り込み・技なし種族・モック)。
- **XCUITest(9 件追加。実行していない。`xcodebuild build-for-testing` でコンパイルだけ確認済み)**: `CalcAttackerStatsUITests` 8 件(ブロックの出方と強調・特殊技で強調が移り値が残る・SP 入力でカスタム/戻す・特化ピルが 32・上昇を入れる・同じ向きの無効化と理由・不正入力の理由と行の出し入れ・使わない側の不正・技シートに変化技が出ない)と
  `LargeTextLayoutUITests.testCalcScreenAttackerStatBlocksNoHorizontalOverflowAtAX5`(AX5 で横はみ出しなし・3 択が縦積み・タップできる)。
- **実行結果(足場の時点。2026-10-05)**: 追加した単体 65 件のうち失敗 52・成功 13(成功は文言・既定値・「変わらなければ計算しない」など足場でも成り立つもの)。既存 1395 件は成功のまま(スイート全体 1460 件、失敗は追加分だけ)。
  XCUITest は View 未実装のため実行すれば全件失敗する見込み(コンパイルのみ確認)。

### 実装者への注意

- 足場は `TODO(implementer` を検索: `AttackerStatInput.swift`(型は完成、`AttackerStatRules`・`CalcMoveRules` の本体が空)、`CalcViewModel+AttackerStats.swift`(公開 API の形だけ。本体が空)、
  `CalcViewModel.attackerStatInputs`(保持だけ追加済み)、`AttackerStatLabels`(文言は確定済み)、`PokeCalcError.Code.attackerSPInvalid`(追加済み)。
- 実装箇所: ① `AttackerStatRules` の本体(Web の `web/src/domain/attackerStatInputs.ts`・`spInput.ts` と同じ規則。一覧の最初の無補正は `AttackerPreset.build` と同じ「一覧の順」)。
  ② `CalcViewModel.buildRequest` を `.preset` のとき `AttackerStatRules.resolve` に切り替える(`.team` は従来どおり個体の値)。要求を作れない・変化技・技なしのときは `calcBulk` を呼ばない(`recalculate` の入口で判定)。
  ③ 入力操作は `beginInput()` → 更新 → `recalculate(token:)`。不正でも `beginInput()` で世代を進め、`rows`・`unsupportedNotice` を空に・`isLoading` を解く(`error` は nil。性格の失敗だけ `natureUnavailable`)。
  ④ `moveOptions` を `CalcMoveRules.damagingMoves` で絞る。`reselectMove` は変化技を選ばない(優先する技が変化技なら既定へ。ダメージ技が無ければ `moveId` を空にして throw しない)。`selectMove` は選択肢に無い技を無視(既存のまま)。
  ⑤ `attackerPreset` は「構築でなければ、使う側のブロックの値に一致するプリセット」(技が無ければ攻撃)。`attackerBuildSource` の `.preset` の中身は要求に使わない(`.team` の有無の判定だけに使う)。
  ⑥ View: `CalcAttackerStatBlocksView`(新規)を `moveSelector` の次・`CalcConditionsSection` の前に置く。SP 欄は `TextField`(数値キーボード、`onChange` で `scheduleLatest`)、キーボードのツールバーに「完了」。
  Web に「入力途中の文字を保つ」ため、欄の表示はこちらが持つ文字列(`attackerStatInputs`)と同じ値にする(丸めて書き戻さない)。
- 既存 XCUITest への注意: ブロックが技セレクタと「詳細」の間に入るので「詳細」が下がる。`CalcConditionsUITests` は前方 → 戻るの `scrollUntilHittable` を持つ。**ほかの XCUITest(`CalcDefenderRanksUITests`・`AbilityPickerUITests`・`MegaItemLockUITests`・`FavoritesScreenUITests` など)が前方スクロールだけだと通り越す**ので、
  View を足したあと iPhone 17e・iPhone 18 Pro・iPhone 17 で通し、通り越したら操作だけ直す(検証は変えない)。ピルの行・種族カード・技セレクタの位置は動かさない。
- 既存テストの期待値更新(ADR-0518「既存テストへの影響」)は理由をコミットメッセージに書く。`make test-golden`・engine・API は触らない。
- 完了条件: `swift test`・`make ios-test`(3 機種の XCUITest を含む)・`make ios-gen-check`・`make ios-lint`。結果をこの章の後ろに追記し、plan.md にチェックを付ける。

### 実装結果(F-01 / I-ios-5。2026-10-05)

- 実装: `AttackerStatRules`・`CalcMoveRules`・`CalcViewModel+AttackerStats`・`CalcViewModel`(要求は `attackerStatInputs` から。`moveOptions` を変化技除外。`reselectMove` が変化技を選ばない)・
  View `CalcAttackerStatBlocksView`(新規)・`CalcScreenView`(ブロック・「完了」ツールバー・ピルの無効化)。`design.md` に iOS の記述を追記。
- 判断: ①`selectAttackerPreset(_:)` は同じ値のプリセットを押し直しても計算し直す(従来のピルと同じ。既存テストが依存)。値が変わらない操作で計算しないのは SP 欄・補正の操作だけ。
  ②`reselectMove` で learnset の解決が `maxMoveLookupsPerSelection` で打ち切られ、ダメージ技が見つからなかったときは従来どおり「解決できた最初の技」(変化技のことがある)を選ぶ。
  上限の先にダメージ技があるかもしれず「覚えない」と言い切れないため(`CalcViewModelMoveLookupTests` の既存テストを変えない)。変化技のときは安全網(`isStatusMoveSelected`)が計算を止めて案内する。
  全部調べ終えて無かったときだけ `noDamagingMoves`(技欄が空・案内)。③SP 欄は View 側の `draft` を同期で書き換え、VM へは `scheduleLatest` で反映。自分が送った値の戻りで巻き戻さないよう、送った値を覚える。
- 既存テストの期待値を変えた箇所(検証の強さは落としていない):

| テスト | 変更前 → 後 | 理由 |
|---|---|---|
| `CalcViewModelTests.testMoveOptionsAreAttackerLearnsetOnlyInLearnsetOrder` | [変化, アルファ専用, 特殊] → [アルファ専用, 特殊] | 変化技を出さない |
| 同 `testAttackerWithOnlyStatusMovesFallsBackToFirstLearnsetMove` → `...HasNoMoveAndDoesNotCalculate` | 先頭の変化技を選び A特化の SP を確認 → moveId 空・`hasNoDamagingMoves`・計算 0 回・入力は受け付ける | 変化技へフォールバックしない。「変化技は atk」は `AttackerPresetTests` が持つ |
| 同 `testEachInputChangeCallsCalcBulkExactlyOnceWithTheRightShape` | 特殊技へ替えると SP が spa のみ・spaUp → 攻撃の 32 が残り無補正。以降は使う側(特攻)のブロックへ入れる(件数は不変) | 値は技で消えない(両方の SP を載せる) |
| 同 `testSwapSidesSwapsSpeciesAndReselectsMoveWithOneCalc` | 入れ替え前に `selectAttackerPreset(.aFull)` → `selectAttackerPreset(.aFull, for: .spa)`、moveOptions から変化技を除く(他の期待値は不変) | 値は技で消えない。入れ替え後の使う側は特攻 |
| 同 `testStaleSpeciesDetailDoesNotOverwriteNewerAttackerSelection` | moveOptions から変化技を除く | 変化技を出さない(ADR 表に無かった同じ理由の追随) |
| `CalcViewModelDefenderRanksTests.testStatusMoveEditsDef` → `testPhysicalMoveEditsDefAndStatusMoveCannotBeSelected` | 変化技で def・「B ±0」 → 物理技で同じ確認+変化技が選べない | 変化技を選べない |
| `CalcViewModelConditionsTests.testStatusMoveEditsAttackRank` | 変化技を選ぶ(実は無視され物理のまま通っていた) → `relevantStat(for: .status) == .atk` を直接確認+選べないことの確認 | 同上 |
| `CalcViewModelTeamIndividualTests.testMemberWithStatusMoveKeepsItOnCalcScreen` → `...GetsTheDefaultDamagingMoveOnCalcScreen` | 変化技が選ばれる → 既定のダメージ技(個体の SP・性格は使う) | 計算画面は変化技を選ばない |

- XCUITest(`ios/scripts/run-xcode-tests.sh`。排他ロック付き): 件数は下の実行結果を参照。既存 XCUITest の操作の直しは不要だった。
- `swift test`: 1460 件・失敗 0。`make ios-lint ios-gen-check ios-check-request-limits` 成功。`xcodebuild build-for-testing` 成功。
- XCUITest の結果: iPhone 17e で 新規 9 件(`CalcAttackerStatsUITests` 8 + AX5 追加 1)成功 9・既存 33 件(CalcScreen・CalcConditions・CalcDefenderRanks・AbilityPicker・MegaItemLock・FavoritesScreen)成功 33。
  iPhone 18 Pro で新規 9 件成功 9。初回の 18 Pro で `testTypingSP...` が失敗(速く打つと VM からの古い値の戻りで `draft` が巻き戻る競合)したため、送った値を覚える対策を入れて再実行し全件成功。
  `FavoriteLoadUITests` はこのブランチに無い(別ブランチ)ため未実行。iPhone 17 は未実行。

## メガストーンの正式名称の受け入れ条件(2026-10-04。設計は ADR-0509 追記 §6'。spec-writer: 受け入れ条件とテストのみ。実装はしない)

ユーザー要望: メガリザードンX を選んだら「リザードナイトX」のような正式名称を出す。§6 を更新(ADR-0509 追記)。契約・API・保存データは不変。

### 受け入れ条件

1. `ItemDisplayName.containsJapanese(_:)` は、ひらがな・カタカナ・長音「ー」・半角カナ・漢字を1文字以上含むと true、英数字のみ・数字のみ・全角英数字・空白・記号のみ・空文字は false(純粋関数)。
2. `ItemDisplayName.megaStoneName(for:baseSpeciesNameJa:)` は、`nameJa` が日本語の文字を含めばそのまま(基本種名が無くても)、含まなければ「{基本種名}のメガストーン」(基本種名が無ければ「メガストーン」)。
3. 固定中の持ち物欄・`MegaItemLock.locked` の `displayName`・構築の補正の通知・結果の行(計算の防御側)・逆算の候補・未対応の印の注記は、すべて条件 2 の表示名を使う(計算・逆算・判定・調整・構築の5画面で同じ)。
4. 既存の英語名のストーンの表示(「{基本種名}のメガストーン」「メガストーン」)は変わらない。
5. 表示名を変えても、要求の `itemId`(例 `attacker.itemId`・`itemCandidates`)と構築の保存値は ID のまま。
6. 持ち物の選択肢には、正式名称のメガストーンも出ない(§2)。
7. XCUITest: モックのメガ種族を選ぶと、固定中の持ち物欄(操作不可・理由の文あり)に「テストどうぐメガいし」が出て「テストモンいちのメガストーン」はどこにも出ない。結果の行にも正式名称が出る。iPhone 17e・iPhone 18 Pro の両方で通る。

### 追加・変更したテスト

| ファイル | 件数 | 内容 |
|---|---|---|
| `PokeCalcKitTests/.../MegaStoneOfficialNameTests.swift` | 19 | 条件 1(14 ケース表)・2(6 ケース表)・`text`(メガ情報なし/ありの両方)・`displayItems`・`BulkRowDisplay.itemLabel`・`MegaItemLock.make`/`correction`・未対応の印の注記・計算(攻撃側の固定+要求の ID・基本種名なし・防御側の行・英語名は従来)・逆算(候補の表示名+要求の ID)・判定・調整・構築(固定・補正の通知・非メガの保存値) |
| `ios/PokeCalcUITests/MegaStoneOfficialNameUITests.swift`(XCUITest・モック) | 3 | 固定中の欄に正式名称・組み立てた名前が出ない / 選択肢に出ない / 結果の行に出る |
| `ios/PokeCalcUITests/MegaItemLockUITests.swift`(期待値の変更) | 0(既存 3 本の期待値のみ) | ADR-0509 追記「期待値が変わる既存テスト」の表 |
| `Support/StubMegaMaster.swift` | (足場) | `officialStone`(nameJa「テストナイトX」)・`megaOfficial`・`megaOfficialNoBaseName`・`makeOfficialService()`。英語名のスタブの値を変更(上の表) |

単体 19 件追加: 実装前は失敗 18(31 アサーション)・成功 1(英語名は従来どおり)。既存テストの失敗 0。全体 1395 件。
XCUITest 実装前の結果: iPhone 17e は 6 件中 失敗 3(新規 2 + 期待値を変えた既存 1)・成功 3、iPhone 18 Pro は 6 件中 失敗 4(同 3 + 既知の `MegaItemLockUITests.testLockReasonFitsAtAX5`。18 Pro では既知の失敗。結果の行が画面下端にはみ出す機種依存で、今回は触らない)。

### 実装者への注意

- 足場(`TODO(implementer` を検索): `ItemDisplayName.containsJapanese`(いまは常に false)・`ItemDisplayName.megaStoneName(for:baseSpeciesNameJa:)`(いまは従来の組み立てだけ)。どちらも `ItemRoles.swift`。
- 実装箇所: ① 2 関数を完成(漢字は U+4E00–9FFF と U+3400–4DBF、かな U+3041–30FF、半角カナ U+FF66–FF9F。全角英数字 U+FF01–FF5E は含めない)。
  ② `MegaItemLock.make(for:allItems:)` はストーンの `Item` を引いて `megaStoneName` で `displayName` を作る。③ `ItemDisplayName.text` の `isMegaStone == true` の分岐(メガ情報なし)も `megaStoneName(for: item, baseSpeciesNameJa: nil)`。
  ④ 各 VM の `megaStoneNames`・`displayItems` は ②③ の結果をそのまま使う(View・VM に別の判定を持たない)。`MegaItemText.stoneName` は英語名のときの組み立てとして残す。
- モックの `nameJa` は「テスト」始まりに固定(`MockPokeCalcServiceTests`)なので、モックのストーンは日本語の正式名称。英語名のフォールバックは単体テストだけ。モックのデータは変えていない。
- 結果の行の XCUITest(`testOfficialStoneNameIsShownInResultRows`)は、防御側がメガのとき行の accessibility ラベルに持ち物名が含まれる前提。含まれない画面構造なら、行の持ち物名の identifier を足してテストの探し方を直す(検証の強さは変えない)。
- Web(`web/src/domain/itemRoles.ts` の `megaStoneLabel`・`itemsWithStoneLabels`、ADR-0326)は別タスクで同じ判定へ更新(データレーン依頼。この章の定義を正とする)。
- 完了条件: `swift test`・`make ios-test`(XCUITest は iPhone 17e・iPhone 18 Pro の両方)・`make ios-lint`。18 Pro の `testLockReasonFitsAtAX5` は既知の失敗として記録する。


### 実装結果(2026-10-04)

- 実装: `ItemRoles.swift` のみ(`containsJapanese`・`megaStoneName`・`MegaItemLock.make` はストーンの `Item` を引く・`ItemDisplayName.text` の isMegaStone 分岐)。各 VM は既存の `MegaItemLock` の表示名と `ItemDisplayName` をそのまま使うので変更なし。
- `swift test`: 1395 件 失敗 0。`make ios-lint ios-gen-check ios-check-request-limits` 成功。`xcodebuild build-for-testing` 成功。
- XCUITest(MegaItemLockUITests・MegaStoneOfficialNameUITests 計 6 件): iPhone 17e は 6 件成功。iPhone 18 Pro は 5 件成功・1 件失敗(既知の `testLockReasonFitsAtAX5`。触っていない)。
- 関連の既存 XCUITest(17e): CalcScreenUITests・UnsupportedMarksUITests 成功。TeamScreenUITests の `testMemberSpeciesAndMoveSlotSearchSheetsFilterAndSelect` が 1 回だけ「シートが閉じる」で失敗したが、単独の再実行で成功(メガと無関係の時間依存)。

## お気に入りの読み込みの受け入れ条件(2026-10-04。設計は ADR-0513、お気に入り表示は ADR-0511、構築から呼ぶ処理は本書「P6-2d」。spec-writer: 受け入れ条件とテストのみ。実装はしない)

Web にはお気に入りの読み込みがまだ無い(追加ボタンだけ)ので、iOS の「構築から呼び出す」(P6-2d)に揃えて先に決めた(ADR-0513 背景・§10)。

### 受け入れ条件(検証可能な形)

- **AC-X(最優先)**: 既存の `swift test`・全 XCUITest が無変更で通る。モックの既定(`POKECALC_MOCK_FAVORITES` 未設定=空・`list`=2件)は変えない。既存テスト・identifier は変えない。
  識別子は追加のみ。`api/openapi.yaml`・Generated・services は触らない。実在ポケモンの実データは使わない(架空の 9001〜9004・`test-*`・`stub-*`)。
- **AC-1 写像**(`FavoriteLoad.plan`。純粋関数): 攻撃側は性格+SP(組)・特性・持ち物、防御側は特性だけ(性格・SP・持ち物は使わず、落としたことにもしない)。
  性格がマスタに無ければ性格と SP を落とす(`.nature`)。特性が種族の `abilities` に無い(`.ability`)・持ち物が持ち物マスタに無い(`.item`)は nil にして `dropped` に宣言順(nature, item, ability)で残す。
  保存に無い特性・持ち物は落としたことにしない。
- **AC-2 持ち物の役割・メガ固定**(ADR-0509): メガ種族はストーンに固定(保存が nil なら案内なし・別の持ち物なら `.item`・ストーンを引けないなら nil で保存があれば `.item`)。
  非メガ種族のメガストーンは `.item`。役割に反するだけの持ち物は残し案内もしない。
- **AC-3 攻撃側の読み込み**(`CalcViewModel.loadFavorite(_:side: .attacker)`): 計算ちょうど1回・`species(key:)` は新しい種族につき1回。要求の攻撃側は種族・性格・SP・特性・持ち物をそのまま使い(プリセットに丸めない)、`Individual.moveId` は nil。
  防御側は変えない。出どころは `.team`(`teamID == FavoriteLoad.sourceTeamID`・`memberID` = お気に入りの id・`displayName` = ラベル ?? 種族名)で `attackerPreset` は nil。`attackerAbilityOptions` を新しい種族に整合させる。
  技はいまの技が新しい learnset にあれば残し、無ければ最初のダメージ技。ranks・やけど・急所などの画面の計算条件は変えず、お気に入りの ranks・status も持ち込まない。先頭ページに無い種族も読み込める。
- **AC-4 防御側の読み込み**(`side: .defender`): 計算ちょうど1回・`species(key:)` は1回。種族と特性だけを設定し、攻撃側の要求は変わらない。特性が種族に無ければ nil+案内(`.ability`)、保存が無ければ旧種族の特性を残さず指定なし。
  `defenderAbilityOptions` を整合させるので、続く `loadDefenderAbilityOptions()`(P6-19)は `species(key:)` も計算も増やさない。防御側のランクは残す(`selectDefender` と同じ)。
  メガの防御側はストーン固定を同じ1回の計算に反映(`itemVariants == [ストーン]`)。
- **AC-5 マスタに無い・失敗**: 種族が `not_found` → 何も変えず計算せず `.speciesMissing`。他の失敗 → `.unavailable`(何も変えない・計算しない)。どちらも画面の `error` を立てず、`rows` を消さず、`isLoading` を解く。
  性格がマスタに無ければ出どころはプリセットのまま(保存の SP は使わない)で、種族・特性・持ち物は設定して `.partial([.nature])`。全部読めたら案内は nil。
- **AC-6 古い応答の破棄**: 続けて呼んだとき、追い越された古い読み込みは状態・案内・計算・エラーのどれも変えない(古い species 応答・古い失敗・古い計算の失敗のどれが後から届いても)。計算に勝った新しい読み込みの結果が残る。
- **AC-7 案内の寿命**: 新しい `loadFavorite` の開始・次の入力操作(`beginInput()`)・`dismissFavoriteLoadNotice()` で消える。
- **AC-8 シート ViewModel**(`FavoriteLoadPickerViewModel`): サービス nil は `isAvailable == false` で何もしない。`load()` は `favorites()` を1回呼び、サーバーの順・種族名つきで `rows` を作る
  (マスタに無い種族の行も「不明なポケモン」で残す。行は `Favorite` を持ち、そのまま読み込める)。0件は `.loaded` かつ `isEmpty`(読み込み前・失敗は空と言わない)。
  失敗は `.failed(RecordScreenError)`(通信・503・decode)。再読み込みは新しい1回の取得。古い応答(成功も失敗も)を捨てる。キャンセルは状態を変えず失敗にしない。追加・削除をしない。
- **AC-9 文言**(`FavoritesLabels` の追加): 入口・見出し(攻撃側/防御側)・注記・案内3種を Core に集約した日本語で返し、英字の code・サーバーの message を出さない。既存の固定文言は変えない。
- **AC-10 モック**: `POKECALC_MOCK_FAVORITES=loadable` で4件(304・303・302・301。内容は ADR-0513 §9)。追加・削除は動く。既定・`list` は不変。
- **AC-11 画面(XCUITest・モック)**: 計算画面に `attackerFavoriteSourceButton`・`defenderFavoriteSourceButton`(36pt 以上)。開くと `favoriteLoadSheet`(注記 `favoriteLoadNote`・行 `favoriteLoadRow-<id>`・`favoriteLoadClose`)。
  行を選ぶとシートが閉じて攻撃側/防御側の種族が変わり(攻撃側はプリセットの選択が外れる・防御側は外れない)、全部読めたら `favoriteLoadNotice` は出ない。
  一部読めない(303)・種族が無い(302)は `favoriteLoadNotice` に理由が出て、種族が無いときは何も変わらず結果も消えない。空は `favoriteLoadEmpty`、通信失敗・503 は `favoriteLoadError`+`favoriteLoadRetry`
  (「計算はそのまま使えます」)で、閉じれば計算画面は無傷(攻守入れ替えも動く)。次の入力で案内は消える。
  AX5 で入口・シート・案内が横にはみ出さず、閉じる・入口が 36pt 以上で押せる。**iPhone 17e と iPhone 18 Pro の両方で通ること**(スクロールは前方に進めて届かなければ下へ戻る)。

### 追加したテスト

- `PokeCalcKit/Tests/PokeCalcCoreTests/FavoriteLoadPlanTests.swift`(18 件・AC-1・2)
- `CalcViewModelFavoriteLoadTests.swift`(30 件・AC-3〜7)
- `FavoriteLoadPickerViewModelTests.swift`(13 件・AC-8)・`FavoriteLoadLabelsTests.swift`(7 件・AC-9)・`MockFavoritesServiceLoadableTests.swift`(6 件・AC-10)
- `ios/PokeCalcUITests/FavoriteLoadUITests.swift`(12 件・AC-11)
- `ios/PokeCalcUITests/LargeTextLayoutUITests.swift`(追加メソッド2本・AC-11 の AX5)
- 足場(`TODO(implementer` で検索): `Sources/PokeCalcCore/FavoriteLoad.swift`(型・文言は確定。`FavoriteLoad.plan` と `FavoriteLoadPickerViewModel.load` は空)、
  `CalcViewModel.loadFavorite`/`dismissFavoriteLoadNotice`(空)。`favoriteLoadNotice` は保存プロパティ。モックの `loadable` データは完成(テスト用の固定値)。

### spec 時点の結果(2026-10-04)

単体 74 件追加(`swift test --filter "FavoriteLoad|MockFavoritesServiceLoadable"`)。実装前: 失敗 52・成功 22(成功は足場の既定値と同じ結果になるもの: 文言・モック・サービス nil・既定の経路・「落とさない」系)。
XCUITest(`FavoriteLoadUITests` 12 件): iPhone 17e・iPhone 18 Pro の両方で失敗 12(入口の identifier が無いため。想定どおり)。AX5 の2本は `xcodebuild build-for-testing` でコンパイルのみ確認。
既存テスト・既存 identifier は1つも変えていない。

### 実装者への注意

- 足場を実装する。**`CalcViewModel.loadFavorite`** は `selectTeamIndividual`(攻撃側)と `selectDefender`+`loadDefenderDetail`(防御側)を見本に、`beginInput()` → 詳細を1回読む(攻撃側は `reloadAttackerMoveOptions` の詳細を、
  防御側は `loadDefenderDetail` の詳細を使い回す。`species(key:)` を重ねて呼ばない)→ `FavoriteLoad.plan` → 反映 → `recalculate` ちょうど1回。
  詳細を読む前に状態を書き換えると、種族が無い・失敗のとき「何も変えない」が守れない(`selectTeamIndividual` は先に `attackerSpeciesKey` を書くので、そのまま真似ない。成功が確かになってから書く)。
  `beginInput()` で `favoriteLoadNotice = nil` にし、読み込みの最後に案内をセットする(古い読み込みは `guard token == latestRequestToken` で何もしない)。
  防御側の特性選択肢は `defenderAbilityOptions`・`defenderAbilityOptionsSpeciesKey` を読み込みで入れる(P6-19 の `.task` を空振りにする)。`resetDefenderAbility()` を使ってから入れる。
- **`FavoriteLoad.plan`** は純粋(`MegaItemLock.make(for:allItems:)` と `MegaItemLock.correction` を再利用してよい)。`FavoriteLoadPickerViewModel` は `FavoritesViewModel.load()` と同じ世代・名前解決(`SpeciesNameResolver`)。
- **View**: `CalcScreenView` に `FavoriteLoadRows`(入口2行。`teamSourceRow` の直下。見た目は `TeamSourceMenuRow` と同じ `glassCard`・幅いっぱい・`Menu` ではなく `Button`+`.sheet`)と案内の `Text`(`favoriteLoadNotice`)を足す。
  シートは `FavoriteLoadSheet`(新規ファイル)で、シートを開くたびに `FavoriteLoadPickerViewModel` を作り `.task` で `load()` を1回。サービスが無ければ入口を出さない
  (`favoritesService` を `FavoritePinSection` と同じ任意注入で受ける。`FavoriteLoadPickerViewModel` を `CalcScreenView.init` で作ってよい)。
  読み込みは `viewModel.scheduleLatest { await $0.loadFavorite(row.favorite, side: ...) }`。
  design.md のトークンのみ・`lineLimit` を付けない・常時アニメ無し・タップ 36pt 以上。**子が1つだけの `.accessibilityElement(children: .contain)` は識別子を畳む**ので、
  `favoriteLoadSheet` は見出し・注記・一覧(or 空/失敗の案内)で子を2つ以上にする。AX5 は他画面と同じ(`dynamicTypeSize >= .accessibility1` で縦積み)。
- 画面の高さは機種で違う(iPhone 17e は小さい)。入口の行を足すと計算画面が伸びる。既存の `scrollUntilHittable` が通り越す場合は**操作だけ**直す(検証は変えない)。
  View を足したあと、**両機種**で `FavoriteLoadUITests`・`LargeTextLayoutUITests` の追加2本・既存の `CalcScreenUITests`/`CalcConditionsUITests`/`FavoritesScreenUITests` を通す。
- 完了条件: `swift test`・`make ios-test`(両機種の XCUITest を含む)・`make ios-gen-check`・`make ios-lint`。結果をこの章の後ろに追記し、plan.md にチェックを付ける。

### 実装結果(2026-10-04)

- `swift test`: 1450 件・失敗 0(お気に入りの読み込みの単体 74 件を含む)。`make ios-lint ios-gen-check ios-check-request-limits` 成功。`xcodebuild build-for-testing`(generic iOS Simulator)成功。
- XCUITest(iPhone 17e・iPhone 18 Pro の両方で各々全件成功): `FavoriteLoadUITests` 12 件、`LargeTextLayoutUITests` の AX5 追加 2 本、既存の `CalcScreenUITests`・`CalcConditionsUITests`・`CalcDefenderRanksUITests`・`FavoritesScreenUITests` 計 27 件。テスト・既存 identifier は変えていない。
- 実装の判断: 攻撃側・防御側とも、詳細(`species(key:)`)を1回読んで成功が確かになってから状態を書く(`reloadAttackerMoveOptions`・`loadDefenderDetail` の反映部分を `applyAttackerDetail`・`applyDefenderDetail` に切り出して共有)。
  案内は `beginInput()` で消す。画面は `FavoriteLoadViews.swift`(入口2行・案内・シート)。`CalcFeature` は `FavoritesService` を任意で渡していたので受け渡しは変えていない。

## 計算履歴の一覧の接続(ADR-0519。2026-10-09)

判断と範囲は ADR-0519。ここには受け入れ条件と既存テストの変更だけを残す。

### 受け入れ条件

- AC-1: 画面を開くと `limit=20`・cursor なしで1回取得し、新しい順に行を出す。行に種族名(攻撃側 → 防御側)・技名・%幅・日付。
  引けない種族・技は「不明」で行を残す。
- AC-2: `nextCursor` があるときだけ「もっと見る」。押すと同じ `limit` で `nextCursor` をそのまま渡し、末尾に足す。null なら消える。
- AC-3: 先頭ページの失敗・続きの失敗とも、履歴の節の中だけにエラーを出す(計算画面・他の節は塞がない)。先頭は再読み込み、続きは「もっと見る」で再試行。
  続きの 400 は先頭から読み直す。空は案内。失敗を空と見せない。
- AC-4: 古い応答を捨てる(読み直しの後に届いた先頭/続きの応答)。読み込み中の「もっと見る」は要求を重ねない。
- AC-5: 行のタップで計算画面が開き、攻撃側・技・防御側・条件が復元されて計算が1回出る。マスタに無い技・種族は入力を書き換えず計算の失敗。
- AC-6: 「契約待ち」の注記を出さない。AX5 で横にはみ出さない。行・ボタンは 36pt 以上。

### 既存テストの変更(絶対ルール6: 仕様変更に必然な箇所だけ。弱めていない)

| テスト | 前 | 後 |
|---|---|---|
| `FavoritesScreenUITests.testDefaultShowsEmptyFavoritesAndOpponentHistory` | `opponentHistoryPendingNote` が**ある**ことを確かめる | 注記が**無い**ことと、計算履歴の節・先頭行(`calcHistoryRow-0-1790000000`)が**ある**ことを確かめる |
| `FavoritesScreenUITests.testScreenNoHorizontalOverflowAtAX5AndRemoveStillWorks` | はみ出し検査の対象に `opponentHistoryPendingNote` | 同じ位置に `calcHistorySection`・`calcHistoryRow-0-1790000000`(注記が消えたため。検査自体は同じ) |
| `FavoritesScreenUITests.testFailureStateNoHorizontalOverflowAtAX5` | 対象はお気に入りと相手履歴のエラー | 計算履歴のエラー・再読み込みも対象に追加(減らしていない) |

`FavoritesLabels.pendingHistoryNote` は削除(参照していたテストは上の UITests だけ)。`CalcHistoryContractTests` は変更なし。

## ios-test-ui の所要時間(ADR-0520。2026-10-10)

実測・2 並列・テスト単位の上限の判断は ADR-0520。`ios/scripts/run-xcode-tests.sh` が ui のとき 2 並列と上限を付ける。
受け入れ条件: (1) 件数の集計(`全 N 件 / 成功 N …`)が直列のときと同じ形で出る、(2) 0 件・失敗・スキップは従来どおり失敗、
(3) `IOS_TEST_PARALLEL=0` で直列、`IOS_TEST_TIMEOUTS=0` で上限なしに戻せる。既存のテストの期待値は変えていない。

## F-12 iOS ビジュアルの基盤(ポップ・カラフル。ADR-0521。2026-10-10)

受け入れ条件: (1) ポップ配色 20 色・タイトル 22/800・影・押下のトークンが design.md の値と一致し、ライト/ダークとも文字のコントラスト 4.5:1(枠・輪は 3:1)を満たす(`PopPaletteTests`)。
(2) 全画面のカード・ボタン・チップ・一覧・案内・背景が部品経由(色の直書きなし)。(3) アクセシビリティ識別子・ラベル文字・`isAccessibilitySize` の分岐・36pt 以上のタップ領域は不変。
(4) 押下の縮みは操作時のみ、「視差効果を減らす」で 0 秒。常時動くものなし。(5) アイコンは装飾(accessibilityHidden)で、状態は文字・アイコンを併記する。

### 既存テストの変更(弱めていない)

| テスト | 前 | 後 | 理由 |
|---|---|---|---|
| `DesignTokenTests.testTextStyleSizesMatchDesignDoc` | サイズ 4 種・`allCases.count == 4` | `title`(22)を加えた 5 種・`count == 5` | design.md の文字の段階にタイトル 22 が加わったため(「design.md に無いサイズを足さない」の趣旨は維持) |

XCUITest の期待値の変更はなし。検証: `swift test` 1602 件成功。XCUITest は代表 14 クラス 133 件成功(失敗 0・スキップ 0)。

## F-02 技の並び(ADR-0523。2026-10-10)

受け入れ条件: (1) 計算画面の技ピッカーに「技の並び」(習得順〔既定〕・五十音順・タイプ順)。(2) 五十音順はひらがな/カタカナを同じ字・濁点は同じ字の中で後・同順位は技 ID。
(3) タイプ順はタイプの並びで群にし群の中は五十音順、見出しはタイプ名。(4) 選択中の技・要求・結果は並びで変わらず、再計算しない。既定の技は learnset の先頭のまま。
(5) 選択は端末内(UserDefaults `pokecalc.moveSort`)に保存、不正値は既定。(6) チップは 36pt 以上・AX5 で横にはみ出さない。
既存テストの変更なし(既定が習得順のため期待値は変わらない)。

## F-09 お気に入りから計算の入力を復元(ADR-0524。2026-10-10)

受け入れ条件: (1) 計算画面の「攻撃側をお気に入りに追加」が `FavoriteInput.calc` を付ける(Web の表と同じ規則。既定の条件はキーごと省く。個体は calc.attacker、見出しは「攻撃側→防御側(技)」)。
(2) `calc` を持つ行に「計算に使う」。タップで計算画面を push し、入力を復元して結果をすぐ出す(「…の計算を開きました」+ 復元しない範囲の文言)。
(3) 復元しないもの(防御側の性格・SP・持ち物、攻撃側の壁、ダブル)は文言と ADR に明記。(4) マスタに無い技・種族は計算の失敗として表示し入力を変えない。お気に入り API の失敗は計算を塞がない。
(5) `calc` の無い旧お気に入りは従来の読み込み導線(#613)のまま。(6) 行のボタンは 36pt 以上・AX5 で横にはみ出さない。
既存テストの期待値の変更なし(テスト補助 `StubFavoritesService` の追加メソッド対応のみ)。

## F-08 iOS 構築の作り直し(ADR-0522。2026-10-10)

受け入れ条件: (1) 構築名の入力が無く、[新しい構築]で空の構築ができてすぐ編集画面が開く。一覧の表示名は既定名なら「構築 N」(作成の古い順)、旧データの名前はそのまま。
(2) 編集画面に 1体目〜6体目の枠が最初から並ぶ。空の枠は「ポケモン」の欄と案内だけで、種族を選ぶと技 4 つ・持ち物・特性・性格・SP が出る。
(3) 枠ごとに「N体目を上へ/下へ/外す」。外すと他の枠は動かない。(4) 保存は明示で、未保存なら「保存していない変更があります」。[一覧に戻る]は未保存なら「保存せずに戻る/編集を続ける」の 2 段階。
(5) Showdown 形式の取り込みは一覧の下の閉じた折りたたみ(説明と 1 体分の入力例つき・新しい構築として作る)、書き出しは編集画面の下の折りたたみ。
(6) 一覧はカード(アイコン・n/6体・最終更新・[開く]・[削除]〈2 段階〉)。(7) 旧データ(`updatedAt` なし・名前つき)を読める。AX5 で横にはみ出さない。

### 既存テストの変更(絶対ルール6: 仕様変更に必然な箇所だけ。弱めていない)

| テスト | 前 | 後 | 理由 |
|---|---|---|---|
| `TeamEditViewModelTests.testSetNameTrimsAndClearsError`・`testSetBlankNameSetsError` | `setName(_:)` の空白除去と `nameError` | 削除 | 構築名の入力 UI と `setName`/`nameError` を廃止したため(名前を直す経路が無い) |
| `TeamScreenUITests.testCreateAddMemberSaveAndDeleteTeamFlow` | 名前アラート→[メンバーを追加]→保存で一覧へ→削除(1 段階) | `testNewTeamOpensEditorWithSixEmptySlots`・`testSavedTeamAppearsAsACardAndReopens`・`testDeleteAsksForConfirmationFirst` ほか 9 件 | 名前アラート・追加ボタン・保存後の遷移・1 段階の削除が無くなったため。同じ流れ(作る→種族を選ぶ→保存→一覧に出る→削除)を新しい UI で確かめ、行の `label == 構築名` は表示名「構築 N」に変えた |
| `TeamScreenUITests.testMemberSpeciesAndMoveSlotSearchSheetsFilterAndSelect` | 追加後の種族・技スロットの検索シート | `testSpeciesAndMoveSlotSearchSheetsFilterAndSelect`(同じ操作・期待値。最初の種族を空の枠 `slotSpeciesPicker-1` で選ぶ) | 入口が空の枠に変わっただけ |
| `TeamTextTransferUITests`(書き出し 2 件) | シートの[この1体を書き出す]/[全員を書き出す] | 編集画面の下の折りたたみの[全員を書き出す](`testExportShowsTextAndCopyAndShare`。1 体書き出しの入口は枠から外したため 1 件に統合) | 書き出しの入口を折りたたみに移したため。書き出したテキストの先頭行・性格行・コピー・共有の期待値は同じ |
| `TeamTextTransferUITests`(取り込み 5 件) | 編集中の構築にメンバーを追加(`importedNotice` =「1体を追加しました。保存すると反映されます。」・カード数 +1) | 一覧の折りたたみから新しい構築を作る(`importedNotice` =「1体の構築を作りました」・構築カード数 +1) | 取り込みは新しい構築を作る決定(ADR-0332 §3 と同じ)。取り込めなかった行の表示・全か無かにしない選択・空入力の案内の期待値は同じ |
| `CalcScreenUITests`・`ReverseScreenUITests`・`JudgeScreenUITests` の `createTeamWithOneMember` | 名前アラート→`addMemberButton`→保存で一覧へ | [新しい構築]→`slotSpeciesPicker-1`→保存→`teamSavedNotice`→`backToListButton` | 同じ事前条件(構築に 1 体)を新しい操作で作る。検査の中身は不変 |
| `LargeTextLayoutUITests`(構築 4 件) | `teamNameField`・`addMemberButton`・`teamTextTransferButton`・`teamTextSheet` のはみ出し検査 | 新しい識別子(`backToListButton`・`teamSlot-N`・`slotSpeciesPicker-1`・折りたたみ・`slotMove*`)に差し替え、一覧のカード・取り込み/書き出しの折りたたみの AX5 検査を追加(減らしていない) | 画面の作り替え |


検証: `swift test` 1646 件成功。XCUITest は構築関連 TeamScreenUITests(13)・TeamTextTransferUITests(6)・LargeTextLayoutUITests の構築 4 件・Calc/Reverse/Judge/FavoriteLoad/LargeText(78 件)を `run-xcode-tests.sh` 経由で実行し全件成功(失敗 0・スキップ 0)。

## F-13 文言のやさしい言い換え(ADR-0526。2026-10-11)

受け入れ条件: (1) 画面に出る文言から用語集(docs/glossary.md)の禁止語(観測・プリセット・指数・16n・マスタ・API・識別子/識別情報・リクエスト・ID・「メガシンカ:」見出し形・「〇〇のメガストーン」ほか)が無い。
(2) 同じ意味の文は ADR-0337 §4 の対応表(Web)と同じ文。(3) 変えない語(確定・乱数・特化・性格・特性・持ち物・能力ポイント・実数値・仮想敵・メガストーン・メガシンカ)は残る。
(4) 識別子(accessibilityIdentifier)・コメント・型名・API のコード値・判定画面は変えない。(5) `GlossaryTests` が(1)(3)を機械で守る。

### 既存テストの期待値の変更(絶対ルール6: 文言の置換だけ。検査の強さは変えていない)

| テスト | 前 → 後 | 理由 |
|---|---|---|
| `AdjustTextTests`(modeLabel・indexLine・hpCurrent・hpLinePoints・unavailable・エラー表) | 「指数と 16n を見る」→「今の耐久・火力と HP を見る」、「倒せる/耐えられる最小の振り方」→「…いちばん少ない振り方」、「火力指数」→「火力の目安」、「16n」→「16の倍数」(「16n-1」→「16の倍数-1」)、「調整の API に接続できません」→「調整のサーバーに接続できません」、「マスタの準備が…」→「ポケモンのデータの準備が…」 | ADR-0337 §4(調整) |
| `SpeedLabelsTests`・`SpeedViewModelAsyncTests` | 「プリセット」→「定番の振り方」、「素早さの API に…」→「素早さのサーバーに…」、「ポケモンのマスタを読み込めません」→「…データを…」 | ADR-0337 §4(素早さ) |
| `BalanceLabelsTests`・`BalanceStage3LabelsTests`・`BalanceViewModelStage3Tests`・`APIBalanceServiceTests` | 「リクエストが正しくありません。入力を見直してください」→「入力の内容が正しくありません。見直してください」、「サーバーのマスタ」→「サーバーのデータ」、「タイプバランスの API に…」→「…のサーバーに…」 | ADR-0337 §4(タイプバランス) |
| `MegaItemLockTests`・`TeamEditViewModelItemRolesTests`・`ItemDisplayNameUnsupportedTests`・`MegaStoneOfficialNameTests`・`StubMegaMaster` | 「{基本種名}のメガストーン」→「{基本種名}専用のメガストーン」、「メガシンカ: メガストーンを持ちます」→「メガシンカするので、持ち物はメガストーンに決まっています」ほか(missing/compare/cleared も §4 の文) | ADR-0337 §4(メガ) |
| `DeviceDataTextTests` | 「この端末に割り当てた ID」→「…番号」、「ID が変わると」→「番号が変わると」 | ADR-0337 §4(このアプリについて) |
| `ReverseCandidateDisplayTests`・`AbilitySplitDisplayTests` | 「観測と一致」→「入力したダメージと一致」(件数表示も) | Web に同じ文が無い。用語集の「観測 → ダメージ」に従う(ADR-0526 決定2) |
| (期待値を変えたテストなし)`ReverseScreenObservations`・`AppEnvironment` の表示 | 「観測を追加」→「ダメージを追加」、「観測n(単位)」→「ダメージn(単位)」、「この観測を削除」→「このダメージを削除」、「APIに接続中」→「サーバーに接続中」 | 用語集(観測 → ダメージ、API → サーバー)。識別子で引くので既存テストは不変 |
| `FavoriteLoadLabelsTests` | 「いまのマスタに無い」→「いまのデータに無い」 | 用語集(マスタ → データ) |
| `UITests`: `AdjustScreenUITests`(`adjustMinSpNotRequested`)・`MegaItemLockUITests`・`MegaStoneOfficialNameUITests`・`FavoriteLoadUITests`(通知の `contains`) | 上と同じ文言の置換(ラベルで比べている箇所だけ。識別子は不変) | 同上 |

追加: `GlossaryTests`(禁止語の混入なし・変えない語が残る・言い換え後の語の確認)。

## F-11 iOS 調整の「目標から振り方を決める」(ADR-0525。2026-10-11)

受け入れ条件: (1) 「調整の内容」の先頭に「目標から振り方を決める」が加わり、選ぶと「目標」の領域が出る(従来の相手・発数/確率の領域は出ない)。従来の5つのモードの挙動・識別子は不変で、選ぶと目標方式を外れる。
(2) 目標を追加(6 件で無効 + 「目標は 6 つまでです」)・外す(番号を詰める)・種類の切り替え(相手は保ち、振り方・技は新しい種類の既定)。選択は sheet/チップ(Menu 不使用)。
(3) 「調整する」で indices と `adjustGoals` を並行して呼び、両方そろってから「目標をすべて満たす振り方」(満たせなければ「目標に一番近い振り方」+「すべての目標は満たせませんでした」)と目標ごとの結果を出す。最新の応答だけを採用する。
(4) 送信前の検査(目標なし・目標 n の相手・技・性格)は API を呼ばない。要求は省略可の欄を送らず(素早さの hits・しきい値 100)、相手がメガならストーンを持たせる。
(5) サーバーが目標の操作を提供していない(404)とき、案内を出して従来の調整に戻り、画面は壊れない。一時的な失敗は目標方式のまま日本語のエラー。(6) AX5 で横にはみ出さない。

### 既存テストの変更

なし(追加のみ。`AdjustMode` を増やさず、`AdjustService` に足さず、既存の期待値を変えない設計にしたため)。モックの `natures.json` に「素早さ上昇」を1件足し(最速のプリセット用)、
`AdjustChoiceButton`・`AdjustPillButton` に最小の高さ 36pt を足した(タップ範囲。見た目の変化は縦に数 pt)。

検証: `swift test` 1687 件成功。XCUITest は Adjust 系(AdjustGoalsUITests 9・AdjustGoalsLargeTextUITests 2・AdjustScreenUITests 8・AdjustLargeTextLayoutUITests 3 = 22 件)を `run-xcode-tests.sh` 経由で実行し全件成功。

## G-01 技の行を縦に短くし、並びをタイプ順だけにする(ADR-0527。2026-10-11)

受け入れ条件: (1) 技の行は「タイプの丸アイコン + 技名」を主、分類の小さなアイコンと威力を副にした 1 行(36pt 以上)。読み上げはタイプ・分類・威力を含む。
(2) 計算画面の技ピッカーはタイプ順(群の中は五十音順、見出しはタイプ名)だけ。切り替え・保存は無い。(3) 選択中の技・要求・結果は変わらず、既定の技は learnset の先頭のまま。

### 既存テストの変更(廃止した機能の分だけ。残る規則は弱めていない)

| テスト | 扱い | 理由 |
|---|---|---|
| `MoveSortTests` 種類・既定・ラベル / 習得順 | 削除 | `MoveSortOrder`・`MoveSortLabels`・習得順を廃止 |
| `MoveSortTests` 五十音 5 件・タイプ順 3 件 | `MoveSort.byType` へ付け替え(入力・期待値は同じ) | 群の中の並びとして残る |
| `MoveSortStoreTests` | 削除 | `MoveSortStore` を廃止 |
| `CalcViewModelMoveSortTests` | `CalcViewModelMoveTypeOrderTests` に置換 | 保存・切り替えを廃止。既定の技・再計算しないは維持 |
| `CalcMoveSortUITests` 切り替え 2 件 | 並び・見出し・切り替え無し・選択・AX5 の行の 3 件に置換 | チップを廃止 |

検証: `swift test` 全件成功、`CalcMoveSortUITests` 3 件成功(`run-xcode-tests.sh`)。
