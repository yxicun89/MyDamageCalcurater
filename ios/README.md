# iOS(SwiftUI)

iPhone 向けのダメージ計算アプリ(計算画面・逆算画面・構築ビルダー)。gateway 経由で calc / pokedex の API を呼び、接続先が無いときは架空データのモックで動く。
API クライアントは `api/openapi.yaml` から swift-openapi-generator で生成し、手で書かない(契約は変更しない)。
シミュレータでの確認は [docs/runbooks/ios.md](../docs/runbooks/ios.md)、実機インストールは [docs/runbooks/ios-device-install.md](../docs/runbooks/ios-device-install.md)(人間の作業)。設計は [ADR-0500](../docs/adr/0500-ios-app-architecture.md)、見た目は [docs/design.md](../docs/design.md)。

```mermaid
flowchart LR
  subgraph App["PokeCalc(アプリ。View だけ)"]
    Views["計算画面 / 逆算画面 / 構築画面"]
  end
  subgraph Kit["PokeCalcKit(Swift Package)"]
    VM["ViewModel(Calc / Reverse / TeamList / TeamEdit)"] --> Service["PokeCalcService(プロトコル)"]
    VM --> TeamStore["TeamStore(プロトコル。構築は端末内保存)"]
    APIImpl["APIPokeCalcService"] -.->|実装| Service
    Mock["MockPokeCalcService<br/>(架空データ。計算しない)"] -.->|実装| Service
    LocalStore["LocalTeamStore<br/>(UserDefaults)"] -.->|実装| TeamStore
    APIImpl --> Gen["PokeCalcAPI<br/>(生成物)"]
    Design["PokeCalcDesign<br/>(デザイントークン)"]
  end
  Views --> VM
  Views --> Design
  Gen -->|"HTTPS + X-Device-Id / X-Session-Id"| Gateway["gateway<br/>/api/calc・/api/pokedex"]
  Spec["api/openapi.yaml"] -.->|"make ios-gen"| Gen
```

## ディレクトリ

| パス | 役割 |
|---|---|
| `PokeCalcKit/Sources/PokeCalcAPI/Generated` | `api/openapi.yaml` の生成物(`pokedex`・`calc` タグだけ。コミットする・手で編集しない) |
| `PokeCalcKit/Sources/PokeCalcCore` | ドメインの型・`PokeCalcService`(API 実装とモック)・ViewModel・表示の整形・設定・端末 ID |
| `PokeCalcKit/Sources/PokeCalcCore/Resources` | モックの架空データ(JSON。名前はすべて「テスト」で始める) |
| `PokeCalcKit/Sources/PokeCalcDesign` | デザイントークン(docs/design.md と同じ名前・値) |
| `PokeCalcKit/Tests` | XCTest(ロジック) |
| `PokeCalc.xcodeproj` / `PokeCalc/` | アプリ(SwiftUI の View だけ)。フォルダ同期で手書きのプロジェクト |
| `PokeCalc/Config/PokeCalc.xcconfig` / `PokeCalc-Info.plist` | API の接続先(`POKECALC_API_BASE_URL`。空ならモック) |
| `PokeCalcUITests/` | XCUITest(主要な画面操作。モック強制) |
| `tools/openapi-gen/` | 生成器の版を固定する生成専用パッケージと生成設定 |
| `scripts/` | テストの合否判定・生成・Info.plist の検査・シミュレータ起動 |

## よく使うコマンド

```sh
cd "$(git rev-parse --show-toplevel)"
make ios-test                        # 生成物の一致・XCTest・XCUITest(シミュレータ)・Info.plist の検査
make ios-gen                         # api/openapi.yaml を変えたら(ルートの make gen には含めない)
make ios-sim-run IOS_SCREEN=calc     # モックで起動してスクリーンショット(root / calc / reverse / team)
cd ios/PokeCalcKit && swift test     # ロジックだけを macOS で手早く
```

Xcode 27 が要る(`xcode-select` が CommandLineTools のままでも、スクリプトが `DEVELOPER_DIR` を Xcode に向ける)。
失敗・スキップ・テスト 0 件は失敗として扱う。署名チームの設定と実機インストールは人間の作業(P6-4)。

「このアプリについて」(ホーム右上の i)の「データの扱い」から「この端末のデータを削除」(サーバーの履歴・お気に入り・構築。
端末内の構築は消さない)。モックの挙動は `POKECALC_MOCK_DEVICE_DATA=partial|fail-once` で切り替える(ADR-0501「P6-7」)。

### 構築のテキスト書き出し・取り込み(P6-20)

構築編集画面の「テキストで書き出し・取り込み」でシートを開く。メンバーカードの「この1体を書き出す」は、その1体を書き出した状態で開く。
書き出しはコピーと共有(ShareLink)。取り込みは貼り付けて「内容を確認」を押す。取り込めなかった行は行番号・内容・理由で一覧に出し、
取り込める体があれば「取り込める N 体だけ追加」か「やめる」を選ぶ(取り込めなかった行が無ければ「N 体を追加」)。
追加は構築編集の画面に入るだけで、保存は画面の「保存」。

書式は Showdown と同じ行構成で、値は**日本語名**(実 Showdown の英語名のテキストとは互換にしない。取り込もうとすると名前が
解決できず「取り込めなかった行」に出る)。能力は努力値ではなく SP(`SP: 32 Atk / 20 Spe`。0〜32・合計66)。`EVs:`・`IVs:` は取り込めない。
決定は [ADR-0502](../docs/adr/0502-ios-showdown-text.md)。

```
ニック (ポケモン名) @ 持ち物
Ability: 特性
Tera Type: タイプ
SP: 32 Atk / 20 Spe
Nature: 性格
- 技名
```

## 関連 ADR

[0500](../docs/adr/0500-ios-app-architecture.md)(構成・生成・モック・設定・テスト)・
[0501](../docs/adr/0501-ios-screen-acceptance.md)(画面ごとの受け入れ条件と判断)・
[0009](../docs/adr/0009-bulk-calc-presets.md)(一括計算のプリセット)・[0010](../docs/adr/0010-reverse-estimation.md) §R(逆算)・
[0200](../docs/adr/0200-calc-svc-api-contract.md)・[0202](../docs/adr/0202-gateway-routing-and-headers.md)(API 契約)。
依存(完全固定): swift-openapi-generator 1.13.1・swift-openapi-runtime 1.12.1・swift-openapi-urlsession 1.3.1・swift-http-types 1.8.0(Apache-2.0)。
