# ADR-0017: iOS アプリの構成(パッケージ分割・API 生成・モック・テスト)

- 状態: 採用(2026-09-21。iOS レーンの既定案。§1 の配布対象・§3 の逆算・§4 の構築と Showdown 形式は同日ユーザー確認済み)
- 日付: 2026-09-21
- 関連: docs/plan.md P6-1〜P6-3、docs/requirements.md、docs/design.md、docs/test-strategy.md L6、ADR-0009(一括計算のプリセット)、
  ADR-0010 §R(逆算の新しい形)、ADR-0012(サービス境界)、CLAUDE.md 技術規約(SwiftUI・swift-openapi-generator・XCTest)、
  docs/ai-shared/COORDINATION.md(iOS レーンは `api/openapi.yaml` に追従するだけ)

## 背景

M3 は「iPhone で使える」。サーバー(P3)と構築 API(P5-4)はまだ無く、`api/openapi.yaml` の逆算の契約は
P3-1 で ADR-0010 §R の形へ変わることが決まっている。iOS レーンは契約を変更できないので、契約が変わっても
画面を作り直さずに済む境界が要る。Xcode 27 と iOS 27 シミュレータは導入済みだが、XcodeGen などのプロジェクト生成ツールは無い。

## 決定

### 1. 構成

```
ios/
  PokeCalcKit/                 Swift Package(ロジック。macOS の swift test とシミュレータの両方で動く)
    Sources/PokeCalcAPI/       swift-openapi-generator の生成物(コミットする。手で編集しない)
    Sources/PokeCalcCore/      ドメインの型・PokeCalcService(境界)・API 実装・モック・画面の状態(ViewModel)
    Sources/PokeCalcDesign/    デザイントークン(docs/design.md と同じ名前・値)
    Tests/…                    XCTest
  PokeCalc.xcodeproj           アプリ(SwiftUI の View だけ)と XCUITest。フォルダ同期(Xcode 16 以降の synchronized group)で手書き
  PokeCalc/                    アプリのソース
  PokeCalcUITests/             XCUITest
  tools/openapi-gen/           生成用の Swift Package(generator の版を固定)
```

- **View 以外はパッケージに置く**。画面のロジック(入力 → 要求の組み立て、結果の並び、エラー表示)は ViewModel として XCTest で検証し、
  View は ViewModel の状態を描くだけにする。XCUITest は主要な操作が画面でつながることだけを見る。
- 配布対象は iOS 27 以上(ユーザー回答 2026-09-21: 実機は iOS 27。Liquid Glass の `glassEffect` は iOS 26 から。requirements.md のビジュアル B)。パッケージは macOS 27 も対象にし、
  Xcode を使わない `swift test` でも同じテストが走る。
- `.xcodeproj` はフォルダ同期にし、ファイルを足しても pbxproj を編集しない。生成ツール(XcodeGen 等)への依存を増やさない。

### 2. API クライアントの生成
- `make ios-gen` が `ios/tools/openapi-gen` の swift-openapi-generator(版は `Package.resolved` で完全固定)を実行し、
  `api/openapi.yaml` から `PokeCalcAPI/Generated/{Types,Client}.swift` を作る。**生成物はコミットする**(Go の `openapi.gen.go` と同じ扱い)。
  ビルドプラグインは使わない(Xcode のプラグイン信頼の確認と、パッケージ外のファイル参照を避けるため)。
- `make ios-gen-check` は再生成して差分が無いことを確かめる(`make ios-test` から呼ぶ)。
- ルートの `make gen` には入れない(Swift/Xcode を持たない他のレーンの `make gen` を壊さないため)。

### 3. 境界: `PokeCalcService`
- 画面は生成型を直接使わず、ドメインの型(`PokeCalcCore`)と `PokeCalcService` プロトコルだけを使う。
  実装は2つ: `APIPokeCalcService`(生成クライアント。全要求に `X-Device-Id` / `X-Session-Id` を付ける)と `MockPokeCalcService`。
- 生成型 ↔ ドメインの写像は `APIPokeCalcService` の中の1か所に置く。契約が変わったら写像だけを直す。
- **逆算**のドメインの型は ADR-0010 §R の形(性格クラス × 持ち物ごとの SP 範囲、`exact`・`assumedHpSp`)にする。
  いまの `api/openapi.yaml` の `ReverseCandidate`(`matchScore` 等)は P3-1 で廃止が決まっているので写像しない。
  `APIPokeCalcService` の逆算は、契約が更新されるまで「API が未対応」のエラーを返す(画面はその旨を表示する)。
- エラーは `PokeCalcError`(`code` は OpenAPI の `Error.code` をそのまま運ぶ)。

### 4. モック(サーバーができるまで)
- `MockPokeCalcService` は**架空データ**(名前は「テスト」で始める。実在のポケモン・技・持ち物の名前や数値を使わない。ADR-0002)を
  パッケージのリソース(JSON)から読む。
- **モックはダメージを計算しない**(engine の式を Swift に写すと単一の正が崩れる。絶対ルール2・coding-rules §2)。
  一括計算・逆算・1対1は、フィクスチャの結果を要求の形(プリセット・持ち物の組合せ)に合わせて返すだけにする。
  画面にはモックで動いていることを表示する。
- 構築(team)は API の契約が無い(P5-4)。`TeamStore` プロトコルと端末内の実装(UserDefaults に JSON)で作り、
  team-svc の契約ができたら API 実装を足す。Showdown 形式の入出力は team-svc の契約に合わせるため後回しにする。

### 5. 設定
- API の接続先はビルド設定 `POKECALC_API_BASE_URL`(xcconfig)→ Info.plist の `PokeCalcAPIBaseURL` で渡し、起動時に1か所で読む。
  独自キーは `INFOPLIST_KEY_*`(生成 Info.plist)では反映されないので、同期フォルダの外の `ios/PokeCalc-Info.plist` を
  `INFOPLIST_FILE` で指定し、`GENERATE_INFOPLIST_FILE` の生成分とマージする。
  空ならモックを使う。起動時の環境変数 `POKECALC_USE_MOCK=1` はモックを強制する(XCUITest 用)。
- 端末 ID は初回起動で UUID を作り UserDefaults に保存する(秘密ではない)。セッション ID は起動ごとに作る。

### 6. 自分側のプリセット
- 「A特化」= 攻撃(特殊技なら特攻)SP 32 + 上昇性格、「A振り」= SP 32 + 無補正、「無振り」= SP 0 + 無補正。
  上昇性格は性格一覧(マスタ)から `plus` が関連ステータスで `minus` が攻撃(関連が攻撃なら特攻)のものを選ぶ(ADR-0010 §R1 の代表性格と同じ規則)。
  無補正は `plus == nil` の最初のもの。**性格 ID を直書きしない**。

### 7. テスト(make ios-test)
- (1) `PokeCalcKit` の XCTest を `xcodebuild test -scheme PokeCalcKit-Package` でシミュレータ実行
- (2) アプリの XCUITest を `xcodebuild test -scheme PokeCalc` でシミュレータ実行(モック強制)
- (3) `make ios-gen-check`
- (4) `make ios-check-infoplist`(接続先を渡してビルドし、成果物の Info.plist にキーが入ることを確かめる。§5)
- シミュレータ名は `IOS_SIMULATOR`(既定 `iPhone 18 Pro`)。`xcode-select` が CommandLineTools のときは `DEVELOPER_DIR` を Xcode に向ける。
- 失敗・スキップを成功と数えない(`xcodebuild` の終了コードと、テスト件数 0 を失敗にする)。
- 画面のスクリーンショット(ライト/ダーク)は手元で撮って確認し、Git には入れない。

## 影響
- `api/openapi.yaml` は変更しない。P3-1 で逆算・一括計算の契約が変わったら `make ios-gen` と写像の更新だけで追従する。
- 構築 API・Showdown 形式・逆算の API 接続は、契約ができた後の iOS レーンのタスクとして残る。
