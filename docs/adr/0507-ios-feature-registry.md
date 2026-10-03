# ADR-0507: iOS の画面を「機能レジストリ」で登録し、RootView・AppEnvironment を画面ごとに編集しない

- 状態: 採用(2026-10-03。ユーザー決定「画面レジストリ化」。iOS レーン)
- 日付: 2026-10-03
- 関連: ADR-0500 §5(AppEnvironment)、ADR-0501(各画面の起動時環境変数)、ADR-0502・0503・0415(`.ready` に値を足した経緯)、
  docs/ai-shared/COORDINATION.md「ADR の帯」(iOS は 0500〜。依頼文の仮番号 0174 ではなく帯の空きを使った)

## 背景

画面を1つ足すたびに、全レーンが次の2ファイルを編集していた。

- `ios/PokeCalc/RootView.swift`: 入口のボタン・`navigationDestination`・起動時に開く環境変数の else-if・Route 型・`#Preview`
- `ios/PokeCalc/AppEnvironment.swift`: `.ready` の連想値(`service, deviceData, backendDescription, adjust, balance, frequentOpponents, speed`)

`.ready` は位置引数なので、値を1つ足すと全パターン(`if case .ready(_, _, let x, _, _, _, _)`)と `#Preview` を書き換える必要があり、
並行するブランチ(例: 判定画面 #511)と毎回衝突した。

## 決定

### 1. 機能の抽象 `AppFeature` と `FeatureRegistry`(`ios/PokeCalc/Features/`)

- 1画面 = 1ファイルの `AppFeature` 実装(`CalcFeature.swift` など)。持つもの:
  - `id`: 画面の識別子。`ios/scripts/sim-run.sh` の `IOS_SCREEN` の画面名と同じ値(`calc`・`reverse`・`team`・`adjust`・`speed`。
    名前を持たなかった画面は `balance`・`about`)
  - `order`: 入口の並び順(小さい順。100 刻み。間に入れるときは間の値を使う)
  - `entry`: 入口の形。`.rootButton(title:accessibilityIdentifier:)`(ルートのピル)または
    `.toolbarIcon(systemImage:accessibilityLabel:accessibilityIdentifier:)`(右上のアイコン。「このアプリについて」)
  - `openAtLaunchEnvironmentKey`: 起動時にその画面を開く環境変数(`POKECALC_OPEN_*_SCREEN_AT_LAUNCH`。無い画面は `nil`)
  - `registerServices(for:into:)`: 自分の画面が使うサービスの生成(モック/API)を宣言する
  - `destination(in:)`: 遷移先の View。設定エラー時でも開ける画面(「このアプリについて」)だけ `destinationWithoutServices()` を持つ
- 遷移は1つの値型 `FeatureRoute(featureID:)` に統一し、`RootView` は `navigationDestination(for: FeatureRoute.self)` 1つで
  レジストリから View を引く。入口・起動時の画面・遷移先はすべて `FeatureRegistry.features`(`order` 順)から作る。
- 起動時に開く画面は、従来の else-if と同じく「並び順で最初に `=1` だった1つ」。

### 2. サービスは型で引くコンテナ `FeatureServices`(`PokeCalcCore`)

- `AppEnvironment` を `.ready(core: CoreServices, features: FeatureServices)` / `.configurationError(String)` の安定した形にする。
  機能を足しても `.ready` のパターンは変わらない。
- `CoreServices`: 複数の画面が**同じインスタンスを共有する**もの(`pokeCalc`・`frequentOpponents`・`backendDescription`)。
  よく使う相手は計算・逆算が同じ actor(モック)を共有していたので、どちらかの機能に持たせず core に置く。
- `FeatureServices`: 1つの画面だけが使うサービスを型で引く(`services.resolve((any AdjustService).self)`)。登録は
  `register(_:_:)`。**同じ型を2回登録したら `FeatureServicesError.duplicateRegistration` を投げ**、起動時の設定エラーとして
  画面に出す(2つの機能が同じサービスを別々に作る取り違えを黙って上書きしない。クラッシュもしない。coding-rules §2)。
- 各機能の `registerServices` は `FeatureBackend`(`.mock(environment:)` / `.api(baseURL:identity:pokeCalc:)`)を受け取る。
  API の `ClientIdentity` は全機能で1つ(セッション ID を起動ごとに1つに保つ。ADR-0500 §5)。
- 既存のサービス型(`AdjustService`・`BalanceService`・`SpeedService`・`DeviceDataService` など)の定義は変えない。
- 機能 ID が重複していても起動時の設定エラーにする(後から登録した画面に到達できなくなるのを黙らせない)。

### 3. 共有ファイルに残る1行(正直な記録)

Swift にはファイルの glob が無く、登録を自動で集める仕組みも使わない(リフレクションやコード生成を持ち込まない)ため、
**`ios/PokeCalc/Features/FeatureRegistry.swift` の配列への1行の追記**だけは共有ファイルの編集として残る。

- 1行1要素・末尾カンマ。並びは各機能の `order` で決まるので、追記は配列の**末尾**でよい(行の順は意味を持たない)。
- 2つのブランチが同時に末尾へ1行ずつ足すと、隣接行の追加として Git は競合を出す。解決は「両方の行を残す」だけで、
  他の行の書き換えは起きない(`.gitattributes` の `merge=union` は GitHub のマージで効く保証が無いので採らない)。
- 起動時に開く画面を `make ios-sim-run IOS_SCREEN=<名前>` でも開きたいときは、`ios/scripts/sim-run.sh` の `case` に1行足す
  (任意。XCUITest は環境変数を直接渡すので不要)。

### 4. 既存の挙動は変えない

入口の並び(計算する → 逆算する → 構築 → 調整 → タイプバランス → 素早さ)・文言・`accessibilityIdentifier`・右上の
「このアプリについて」(設定エラー時も表示・遷移できる)・起動時の環境変数と優先順・モック強制・設定エラー/状態バッジの表示は
従来どおり。既存画面の View 本体・ViewModel は変えない(登録の層だけを足した)。右上のアイコンは `ToolbarItem` 1つから
`ToolbarItemGroup` の `ForEach` に変えた(登録された数だけ並べるため。1つのときの見た目・識別子は同じ)。

## 他のブランチの移行手順(未マージで RootView・AppEnvironment を編集しているブランチ向け)

1. main を自分のブランチに merge する。`RootView.swift`・`AppEnvironment.swift` が競合したら **main 側を採る**(自分の追加は捨てる)。
2. 自分の画面の `AppFeature` を `ios/PokeCalc/Features/<名前>Feature.swift` に新規で書く(既存の `SpeedFeature.swift` などが見本)。
   - 入口の文言・識別子・環境変数のキーは、捨てた `RootView` の差分からそのまま移す。`order` は既存の並びの間か後ろの値。
   - サービスは `registerServices` で `services.register((any XxxService).self, ...)`。モック/API の生成は捨てた `AppEnvironment` の差分から移す。
   - 遷移先は `destination(in:)` で `context.services.resolve(...)`・`context.core.pokeCalc`・`context.teamStore`・`context.path` を使う。
3. `FeatureRegistry.swift` の配列の末尾に `XxxFeature(),` を1行足す。
4. `#Preview` や `.ready(...)` を直接作っていた箇所は `AppEnvironment.makeAtLaunch(environment: [AppConfiguration.useMockEnvironmentKey: "1"])` にする。

例(判定画面 #511): `JudgeFeature`(`id: "judge"`・`order: 700`・`.rootButton(title: JudgeLabels.openButton, accessibilityIdentifier: "openJudgeScreen")`・
`POKECALC_OPEN_JUDGE_SCREEN_AT_LAUNCH`・`JudgeService` をモック/API で登録・`JudgeScreenView(service:master:teamStore:)`)。

## 検討した代替

- `.ready` をラベル付きの構造体1つにする: パターンの書き換えは減るが、構造体のフィールドと生成箇所(モック/API 両方)は共有ファイルのまま。
- 各機能が `static` 初期化子で自己登録する: Swift は未参照の型の初期化を保証しないため、登録漏れが起きる。
- コード生成でファイルを集める: Xcode のビルドフェーズとスクリプトが増え、手順が重くなる。1行の追記の方が単純。

## 影響

- 画面を足すときの共有ファイルの編集は `FeatureRegistry.swift` の1行(と任意で `sim-run.sh` の1行)になる。
- `RootView` の `static let open*AtLaunchEnvironmentKey` は各 `AppFeature` に移った(参照していたのは RootView 自身だけ)。
