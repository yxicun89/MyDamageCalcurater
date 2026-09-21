# Current State

## Damage Calculator
Lane: データ(engine・マスタ・pokedex。どの AI が進めてもよい。COORDINATION.md)
Active: Claude Code
Branch: feat/claude-p1-engine(作業ディレクトリ ~/MyDamageCalcurater)
Status: Phase 1・P2-1・P1-10・Phase R(R-2-9 の公開用クリーンコピーは公開時に実施)・R-3・P1-13(タイプ相性表のデータ化。ADR-0013)・P1-11(表示%の分離)・P1-12(逆算の再設計。ADR-0010 §R)・P2-1b(ゴールデンを Champions へ)・P2-1c(技の使用可否の裁定)・P2-2a(pokedex のスキーマと migrate。ADR-0100)・P2-2b(importer の取得・変換・投入。ADR-0101。実データの取得は未実施で版はプレースホルダ=取り込みは明示的に止まる)は完了(critic レビュー済み)
Next: P2-2c(照合と差分報告。実データの取得と版の固定を含む) → P2-2d → P2-3 → P3-1〜3 → P4-1〜7。人間の確認待ち(plan.md ブロッカー): 観測%の丸め方(整数%表示は確認済み)、公開のタイミング(LICENSE・クリーンコピー)、P2-1c の裁定

## API
Lane: API(calc-svc・gateway・契約テスト。`api/openapi.yaml` の持ち主。どの AI が進めてもよい)
Active: なし
Branch: feat/api-p3(作業ディレクトリ ~/MyDamageCalcurater-api)
Status: 未着手(2026-09-21 にレーンを新設)
Next: docs/plan.md の P3-1 から。P3-1 の小項目のとおり api/openapi.yaml を先に直して make gen(絶対ルール1)。一括計算・逆算(engine の P1-12 の新しい形。ADR-0010 §R8)・WASM 境界との契約差分(ADR-0011 §10)。マスタの読み込みは services/internal/master(データレーンの P2-2a。main に入るまで)を待たず、差し替え可能なインターフェースと架空データで作る。続いて P3-2 gateway、P3-3 契約テストと k3d のスモーク

## Web
Lane: Web(`web/`・Playwright。どの AI が進めてもよい)
Active: なし
Branch: feat/web-p4(作業ディレクトリ ~/MyDamageCalcurater-web)
Status: 未着手(2026-09-21 にレーンを新設)
Next: docs/plan.md の P4-1(docs/design.md のデザイントークンを CSS 変数に)から。P4-2〜P4-4 は WASM(make wasm の engine.wasm と engine/wasmapi の JSON 契約。ADR-0011)で先に作り、マスタ(種族・技・持ち物)は pokedex-svc ができるまで架空データで作る。P4-5 の API 接続は API レーンが api/openapi.yaml を main に入れてから

## iOS
Lane: iOS(`ios/`。M3 の Phase 6。どの AI が進めてもよい)
Active: なし
Branch: feat/ios-p6(作業ディレクトリ ~/MyDamageCalcurater-ios)
Status: 未着手(2026-09-21 にレーンを新設)。Xcode はユーザーが導入中(App Store)。導入後に `sudo xcode-select -s /Applications/Xcode.app` 等が要る
Next: docs/plan.md の P6-1 から。Xcode が使えるか(`xcodebuild -version`)を最初に確認し、無ければ Swift Package(swift-openapi-generator で `api/openapi.yaml` から生成したクライアント・モデル・docs/design.md のデザイントークン)と `swift test` から始める。Xcode が使えるようになったら SwiftUI の Xcode プロジェクトとシミュレータのテスト(`make ios-test`)。サーバー(P3)ができるまで API はモック

## Type Balance Checker
Lane: タイプバランス(どの AI が進めてもよい。COORDINATION.md)
Active: Claude Code
Branch: 次は main から feat/tb-tb4-<名前> を切る(作業ディレクトリ ~/MyDamageCalcurater-tb。git worktree)
Status: TB0〜TB3 は完了・main に統合済み(TB0 の Argo CD 実同期もローカル k3d で確認。ADR-0018)。balance は Echo v5.3.1(最新の安定版)。ポケモン・技・特性は temporary の read model(架空データの example。実データは BALANCE_*_PATH でマウント。P2-2 のスナップショットができたら差し替え)。k3d には Argo CD v3.5.3・クラスタ内レジストリ(balance-registry)・Application pokecalc-balance(manual sync)がある
Next: TB4(仮想敵診断。ユーザー回答: 仮想敵を最大 6 体、pokemonId・技 ID 最大 4・特性は任意で入力し、各仮想敵について自分の各メンバーが受ける最大倍率と与えられる最大倍率を表にし、安全に受けられるメンバー数を集計。TB1〜3 を再利用)の ADR-0019 → spec-writer → implementer → critic。その後 TB5(おすすめタイプと該当ポケモン。DECISIONS.md 2026-09-22)。軽微の残り: HTTP で相性表が失敗したときの 500 テスト、typed nil の provider、read model の JSON Schema、CoverageMultiplier の nullable enum
メモ: `make balance-k3d-deploy`(local overlay)で上書きすると Application は OutOfSync になる(manual sync なので戻らない)。GitOps に戻すときは Argo CD で Sync

## Shared Interfaces
- Pokemon ID: pokedex-svc の `{図鑑番号4桁}-{フォルム3桁}` 形式に準拠
- Type: 18タイプの英語小文字ID(fire, water, ...)。表示名・色は docs/design.md のトークンに準拠
- Type multiplier: 分数ではなく整数表現(claude-review.md 参照)
- サービス境界: damage-calc と balance は兄弟。相互の実行時 API へ直接依存しない
- 共通マスタ: 正本は1つ。`feat/claude-p1-engine` の ADR-0002 にあるコミット済みスナップショット案を候補とし、人間の確認待ち
- balance の type chart: P1-13 の `testdata/golden/typechart.json` をバイト複製して同梱(ADR-0015)。正式マスタ確定後に provider を差し替える
- 共有状態の正本: `origin/main` の `docs/ai-shared/`。未マージの続きは各レーン欄の `Branch` の最新コミット(COORDINATION.md)
- 開発の正本: private の `origin` の `main`(PR でのみ更新。Argo CD が参照)。作業ディレクトリはレーンごとに `~/MyDamageCalcurater`(ダメージ計算)と `~/MyDamageCalcurater-tb`(タイプバランス)
