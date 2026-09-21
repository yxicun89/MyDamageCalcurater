# iOS(SwiftUI)

iPhone 向けのダメージ計算アプリ。設計は [ADR-0017](../docs/adr/0017-ios-app-architecture.md)、見た目は [docs/design.md](../docs/design.md)。
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

失敗・スキップ・テスト 0 件を成功と数えない(ADR-0017 §7)。

## モックと API の切り替え(ADR-0017 §5)

| 設定 | 結果 |
|---|---|
| Info.plist `PokeCalcAPIBaseURL`(xcconfig の `POKECALC_API_BASE_URL`)が空・無し | モック |
| 有効な `http(s)://` の URL | その API(全要求に `X-Device-Id` / `X-Session-Id`) |
| 起動時の環境変数 `POKECALC_USE_MOCK=1` | URL があってもモック(XCUITest 用) |
| URL が不正(スキーム無し・http(s) 以外・ホスト無し) | 起動時にエラー(黙ってモックに落とさない) |

モックはダメージを**計算しない**。架空データの結果を要求の形(プリセット・持ち物の組合せ)に合わせて返すだけで、
画面にはモックで動いていることを表示する。逆算は API の契約更新(P3-1)まで API 実装では「API 未対応」エラーになる。

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
   `PokeCalcError`(`code` 保持)へ写す。通信失敗も `PokeCalcError`。逆算は HTTP を送らずに「API 未対応」の `PokeCalcError`。
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
