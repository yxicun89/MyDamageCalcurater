# Current State

## Damage Calculator
Lane: データ(engine・マスタ・pokedex。どの AI が進めてもよい。COORDINATION.md)
Active: Claude Code
Branch: feat/claude-p1-engine(作業ディレクトリ ~/MyDamageCalcurater)
Status: Phase 1・P2-1・P1-10・Phase R(R-2-9 の公開用クリーンコピーは公開時に実施)・R-3・P1-13(タイプ相性表のデータ化。ADR-0013)・P1-11(表示%の分離)・P1-12(逆算の再設計。ADR-0010 §R)・P2-1b(ゴールデンを Champions へ)・P2-1c(技の使用可否の裁定)・P2-2a(pokedex のスキーマと migrate。ADR-0100)・P2-2b(importer の取得・変換・投入。ADR-0101)・P2-2c(照合と差分報告・版の固定・習得技は進化前から継がない。ADR-0103。実データの dry-run が通る)は完了(critic レビュー済み)
Next: P2-2d(CronJob と make import)→ P2-3(pokedex-svc。内部 API・natures・balance/speed 向けの export を含む) → P3-1〜3 → P4-1〜7。人間の確認待ち(plan.md ブロッカー): 観測%の丸め方(整数%表示は確認済み)、公開のタイミング(LICENSE・クリーンコピー)、P2-1c の裁定

## API
Lane: API(calc-svc・gateway・契約テスト。`api/openapi.yaml` の持ち主。どの AI が進めてもよい)
Active: なし
Branch: (次の作業で main から feat/api-<名前> を切る。作業ディレクトリ ~/MyDamageCalcurater-api)
Status: Phase 3 完了。P3-1(ADR-0200・0201。PR #14)・P3-2 gateway(ADR-0202。PR #23)・P3-3 契約表と k3d のデプロイ・スモーク(ADR-0203。critic PASS)。k3d の既存クラスタで make api-k3d-deploy && make api-smoke 成功
Next: 他レーン待ち。(1) データレーンの pokedex-svc(P2-3)が main に入ったら、gateway の local overlay に GATEWAY_POKEDEX_URL を設定し、smoke.sh の /api/pokedex の期待値 503→200 と TestManifestGatewayLocalConfig を変える。(2) 共通マスタ(P2-2a の services/internal/master)の写像が入ったら calc-svc の暫定 Store(services/calc/internal/master)を差し替える。(3) Web(P4-5)・iOS から API 契約の要望があれば DECISIONS.md で受ける

## Web
Lane: Web(`web/`・Playwright。どの AI が進めてもよい)
Active: なし
Branch: feat/web-p4(作業ディレクトリ ~/MyDamageCalcurater-web)
Status: P4-1〜P4-6・P4-8・P4-9 完了・main に統合(PR #22・#28・#33 と P4-9 の PR。critic PASS)。Web のテストはルートの make test / lint / build に含まれる。
人間待ち: P4-5 のブラウザ実機確認(Chrome・Safari。手順は docs/verify-m1.md §2)。P4-7 は verify-m1.md のドラフト(M1 の残りを待つ)
Next: (1) P4-7 の完成: P2-2c/d・P2-3 pokedex-svc・P3-3 が main に入ったら、オンラインのときにマスタを API から読む MasterSource を作り(ADR-0301 §4)、
verify-m1.md §4 を手順に置き換える。(2) P5-5(M2 の Web: 履歴・よく計算する相手・構築ビルダー)は record/team の API を待つ

## iOS
Lane: iOS(`ios/`。M3 の Phase 6。どの AI が進めてもよい)
Active: Claude Code
Branch: feat/ios-p6(作業ディレクトリ ~/MyDamageCalcurater-ios)
Status: P6-1(ADR-0500)・P6-2a 計算画面・P3-1/P3-2 の契約変更への追従(逆算も API で呼ぶ)は完了(critic PASS)。`make ios-test`(ios-gen-check・XCTest 119 件・XCUITest 5 件・Info.plist)が緑。iOS 27 / Swift 6.4
Next: P6-2b 逆算画面(観測はテンキー入力。与えたダメージ = 相手 HP の減少%(整数)、受けたダメージ = 自分 HP の減少量)→ P6-2c 構築(端末内保存の TeamStore、Showdown 形式は後回し)→ P6-3 → P6-4。ViewModel は PokeCalcCore で XCTest、主要操作は XCUITest

## Type Balance Checker
Lane: タイプバランス(どの AI が進めてもよい。COORDINATION.md)
Active: Claude Code
Branch: 次は main から feat/tb-<名前> を切る(作業ディレクトリ ~/MyDamageCalcurater-tb。git worktree)
Status: TB0〜TB5 と整備(ADR-0402 の read model の JSON Schema を含む)は完了・main に統合済み。balance は Echo v5.3.1。ポケモン・技・特性は temporary の read model(架空データの example。実データは BALANCE_*_PATH でマウント)。k3d には Argo CD v3.5.3・クラスタ内レジストリ・Application pokecalc-balance(manual sync)。新しい ADR はタイプバランスの帯 0400〜
Next: データレーンの `pokedex export`(P2-3。nameJa・abilityIds・レギュレーションで絞る・特性の read model。受諾済み。形は services/balance/schema/ の JSON Schema)が main に入ったら、balance の read model をそれに差し替え、実データで TB5 を確認する。それまでは待ち(ブロッカーではない)。Web / iOS から balance を使う画面は各レーンの範囲(必要なら DECISIONS.md で依頼)
メモ: `make balance-k3d-deploy`(local overlay)で上書きすると Application は OutOfSync になる(manual sync なので戻らない)。GitOps に戻すときは Argo CD で Sync

## Speed
Lane: 素早さ(素早さ比較サービス。`services/speed/`・`web/src/speed/`。どの AI が進めてもよい)
Active: Claude Code
Branch: 次は main から feat/speed-s2 を切る(SP1 は feat/speed-s1 → PR で main に統合。作業ディレクトリ ~/MyDamageCalcurater-speed)
Status: SP0(ADR-0600。基盤・計算コア・read model・一覧 API・Kustomize)と SP1(ADR-0601。6 行のプリセット・速い順・同速の段・`presets` での絞り込み・`GET /api/speed/v1/table`)は完了・main に統合
Next: SP2(自分の位置: 最小の選択 = プリセット uninvested / neutral-max / max + スカーフ on/off、オプション = SP 0〜32・性格3通り・ランク -6〜+6・スカーフ、または実数値の直接入力 → 実数値と表の中の位置(速い段・同速の段・遅い段の境目))→ SP3(`web/src/speed/`。web/ は main にできたので、画面部品を作りタブを1項目登録)→ SP4(pokedex の read model・k3d・GitOps)。SP4 までに決める: 空の roster の扱い(いまは read model が空を拒否。pokedex の adapter では 503 か空配列か。SP1 critic 軽微)

## Maintenance
Lane: 整備(Claude の上限時に Codex が進める。COORDINATION.md「Claude の上限時の Codex」)
Active: なし
Branch: なし(次回は origin/main から新しい fix/maint-<名前> を切る)
Status: MT-1(統合検証)・MT-2(check-publishable の自己テスト修正・lint 組み込み)は PR #24 で main に統合済み
Next: docs/plan.md の整備レーン MT-3 から順に進める

## Shared Interfaces
- Pokemon ID: pokedex-svc の `{図鑑番号4桁}-{フォルム3桁}` 形式に準拠
- Type: 18タイプの英語小文字ID(fire, water, ...)。表示名・色は docs/design.md のトークンに準拠
- Type multiplier: 分数ではなく整数表現(claude-review.md 参照)
- サービス境界: damage-calc と balance は兄弟。相互の実行時 API へ直接依存しない
- 共通マスタ: 正本は1つ。`feat/claude-p1-engine` の ADR-0002 にあるコミット済みスナップショット案を候補とし、人間の確認待ち
- balance の type chart: P1-13 の `testdata/golden/typechart.json` をバイト複製して同梱(ADR-0015)。正式マスタ確定後に provider を差し替える
- 共有状態の正本: `origin/main` の `docs/ai-shared/`。未マージの続きは各レーン欄の `Branch` の最新コミット(COORDINATION.md)
- 開発の正本: private の `origin` の `main`(PR でのみ更新。Argo CD が参照)。作業ディレクトリはレーンごとに `~/MyDamageCalcurater`(ダメージ計算)と `~/MyDamageCalcurater-tb`(タイプバランス)
