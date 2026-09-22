# iOS(SwiftUI)

iPhone 向けのダメージ計算アプリ。設計は [ADR-0500](../docs/adr/0500-ios-app-architecture.md)、見た目は [docs/design.md](../docs/design.md)。
iOS レーンは `api/openapi.yaml` に追従するだけで、契約は変更しない。

## 構成

```
ios/
  PokeCalcKit/                 Swift Package(View 以外のすべて。macOS の swift test でも動く)
    Sources/PokeCalcAPI/       swift-openapi-generator の生成物(コミットする。手で編集しない)
    Sources/PokeCalcCore/      ドメインの型・PokeCalcService・APIPokeCalcService・MockPokeCalcService・設定・端末ID
      Resources/               モックの架空データ(JSON。名前はすべて「テスト」で始める)
    Sources/PokeCalcDesign/    デザイントークン(docs/design.md と同じ名前・値)
    Tests/                     XCTest(PokeCalcDesignTests / PokeCalcCoreTests)
  PokeCalc.xcodeproj           アプリ(View だけ)と XCUITest。フォルダ同期(PBXFileSystemSynchronizedRootGroup)で手書き
  PokeCalc/                    アプリのソース(RootView・AppEnvironment・Config/PokeCalc.xcconfig・Assets.xcassets)
  PokeCalcUITests/             XCUITest(ルート画面の骨組みだけを確認。P6-1)
  tools/openapi-gen/           生成器の版を固定する生成専用パッケージ
```

依存は完全固定: swift-openapi-runtime 1.12.1 / swift-openapi-urlsession 1.3.1 / swift-http-types 1.8.0
(いずれも Apache-2.0)。生成器は swift-openapi-generator 1.13.1。
デプロイターゲットは iOS 27 / macOS 27(ユーザー決定)。ツール版は Swift 6.4(swift-tools-version)、
言語モードは Swift 6(`swiftLanguageModes: [.v6]` / Xcode ターゲットの `SWIFT_VERSION = 6.0`)。

## Xcode で開く

`ios/PokeCalc.xcodeproj` を Xcode 27 で開く(XcodeGen 等は使わない。フォルダにファイルを足せば
次回ビルド時に Xcode が自動で拾う)。ローカルパッケージ参照は `ios/PokeCalcKit`(相対パス
`PokeCalcKit`。xcodeproj のあるディレクトリからの相対)を指すので、初回はパッケージの解決
(Resolve Package Graph)が走る。スキームは `PokeCalc`(アプリ + XCUITest)。シミュレータでの
実行・テストはそのままのシステム署名で動く。実機は `DEVELOPMENT_TEAM` が空なので、
人間が署名チームを設定する(CLAUDE.md「人間の確認が必要なこと」)。

## コマンド

Xcode 27 が必要。`xcode-select` が CommandLineTools を指しているときは `DEVELOPER_DIR` を付ける。

```sh
export DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer

make ios-gen              # api/openapi.yaml → PokeCalcAPI/Generated/(ルートの make gen には含めない)
make ios-gen-check        # 再生成して差分が無いこと
make ios-check-infoplist  # POKECALC_API_BASE_URL を渡してビルドし、成果物の Info.plist に反映されるか確認
make ios-test             # PokeCalcKit の XCTest + アプリの XCUITest(シミュレータ)+ 上の2つ

# パッケージだけを手早く(macOS)
cd ios/PokeCalcKit && swift test
# シミュレータで(既定 IOS_SIMULATOR="iPhone 18 Pro")
xcodebuild test -scheme PokeCalcKit-Package -destination 'platform=iOS Simulator,name=iPhone 18 Pro'
# アプリ本体だけをビルド/実行したいとき
xcodebuild build -project ios/PokeCalc.xcodeproj -scheme PokeCalc -destination 'platform=iOS Simulator,name=iPhone 18 Pro'
```

失敗・スキップ・テスト 0 件を成功と数えない(ADR-0500 §7)。

## モックと API の切り替え(ADR-0500 §5)

| 設定 | 結果 |
|---|---|
| Info.plist `PokeCalcAPIBaseURL`(xcconfig の `POKECALC_API_BASE_URL`)が空・無し | モック |
| 有効な `http(s)://` の URL | その API(全要求に `X-Device-Id` / `X-Session-Id`) |
| 起動時の環境変数 `POKECALC_USE_MOCK=1` | URL があってもモック(XCUITest 用) |
| URL が不正(スキーム無し・http(s) 以外・ホスト無し) | 起動時にエラー(黙ってモックに落とさない) |

モックはダメージを**計算しない**。架空データの結果を要求の形(プリセット・持ち物の組合せ)に合わせて返すだけで、
画面にはモックで動いていることを表示する。

接続先は `ios/PokeCalc/Config/PokeCalc.xcconfig` の `POKECALC_API_BASE_URL` で設定する。
**`/api` を含めない**(生成クライアントの各操作は `/api/calc` `/api/pokedex/species` のように
`/api/...` から始まるパスをすでに持っている。ベース URL に `/api` を付けると `/api/api/...` に
二重になる)。**xcconfig は `//` から行末までをコメント扱いにする**ため、URL をそのまま書くと
スキームの `//` から後ろが消える。空の変数参照 `$()` で `//` を分断して書く。

```
POKECALC_API_BASE_URL = https:/$()/pokecalc.example.invalid
```

## P6-1 の受け入れ条件(実装済み)

1. `swift test`(PokeCalcKit)で `PokeCalcDesignTests` と `PokeCalcCoreTests` がすべて成功する。
2. デザイントークンが docs/design.md と一致する: ベース6色のライト/ダークの RGBA、18 タイプの色(openapi の `PokeType` ID で引ける。欠け・余り無し)、
   文字サイズ 28/17/15/12、角丸 20/999/12、余白 4/8/12/16/24。SwiftUI の `Color` / `Font` を返すアクセサがある。
3. `APIPokeCalcService` は生成クライアント経由で、全操作に `X-Device-Id` / `X-Session-Id` を付け、パス・メソッド・クエリ(`q` / `limit`)・
   リクエスト JSON(`presets` / `itemVariants` / `sp` など)をドメインから正しく写し、200 の JSON をドメインへ、エラー body を
   `PokeCalcError`(`code` 保持)へ写す。通信失敗も `PokeCalcError`。逆算も同じ写像で API を呼ぶ(P6-2 契約追従)。
4. `ClientIdentity`: 端末 ID は初回だけ作って UserDefaults に保存(以後同じ・UUID 形式・壊れた値は作り直す)、セッション ID はインスタンスごとに新しい UUID。
5. `AppConfiguration` が上の表どおりにモック/API/エラーを決める。
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
