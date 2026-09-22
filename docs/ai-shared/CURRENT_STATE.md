# Current State

## Damage Calculator
Lane: データ(engine・マスタ・pokedex。どの AI が進めてもよい。COORDINATION.md)
Active: Claude Code
Branch: feat/claude-p1-engine(作業ディレクトリ ~/MyDamageCalcurater)
Status: Phase 1・P2-1・P1-10・Phase R(R-2-9 の公開用クリーンコピーは公開時に実施)・R-3・P1-13(タイプ相性表のデータ化。ADR-0013)・P1-11(表示%の分離)・P1-12(逆算の再設計。ADR-0010 §R)・P2-1b(ゴールデンを Champions へ)・P2-1c(技の使用可否の裁定)・P2-2a(pokedex のスキーマと migrate。ADR-0100)・P2-2b(importer の取得・変換・投入。ADR-0101。実データの取得は未実施で版はプレースホルダ=取り込みは明示的に止まる)は完了(critic レビュー済み)
Next: P2-2c(照合と差分報告。実データの取得と版の固定を含む) → P2-2d → P2-3 → P3-1〜3 → P4-1〜7。人間の確認待ち(plan.md ブロッカー): 観測%の丸め方(整数%表示は確認済み)、公開のタイミング(LICENSE・クリーンコピー)、P2-1c の裁定

## API
Lane: API(calc-svc・gateway・契約テスト。`api/openapi.yaml` の持ち主。どの AI が進めてもよい)
Active: Claude Code
Branch: feat/api-master-adapter(作業ディレクトリ ~/MyDamageCalcurater-api)
Status: Phase 3 完了(PR #14・#23・#30)。P3-4 calc-svc のマスタを pokedex-svc の内部 API(MasterExport)から受け取る形に変更(ADR-0204。critic PASS)。k3d と dev はファイル方式
Next: (1) Web レーンの依頼: gateway に任意の GATEWAY_WEB_URL(設定時は /api・/assets・/healthz 以外を Service web:80 へ転送。k3d の local overlay は http://web)。(2) pokedex-svc(P2-3。データレーンが /internal/pokedex/master・natures・showdownId を受け入れ済み)が main に入ったら、calc の local overlay を URL 方式(CALC_MASTER_URL=http://pokedex)に、gateway に GATEWAY_POKEDEX_URL を設定し smoke の /api/pokedex を 503→200 に。(3) 手順書(gateway・calc の README)を AGENTS.md「手順書の書き方」に合わせる

## Web
Lane: Web(`web/`・Playwright。どの AI が進めてもよい)
Active: Claude Code
Branch: feat/web-p4(作業ディレクトリ ~/MyDamageCalcurater-web)
Status: P4-1〜P4-6 完了(critic PASS。P4-1〜P4-5 は PR #22 で main 済み)。Web のテストはルートの make test / lint / build に含まれる。
P4-5 のブラウザ実機確認(Chrome・Safari)は人間待ち。P4-7 は docs/verify-m1.md のドラフト(M1 の残りを待つ)
Next: P4-7 の完成(P2-2c/d・P2-3 pokedex-svc・P3-3 が main に入ったら、オンラインのときにマスタを API から読む MasterSource を作り、verify-m1.md §4 を手順に置き換える)。
持ち越し: 逆算の「型名でまとめる表示」と絞り込みの演出(ADR-0300 §7)、攻撃側プリセットの engine への移設(データレーンへの提案)

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
Status: TB0〜TB4 は完了・main に統合済み。TB5 は完了し PR で統合する(TB5 おすすめタイプと該当ポケモン /recommendations。ADR-0401、§8 はユーザー回答による範囲の変更)。balance は Echo v5.3.1。ポケモン・技・特性は temporary の read model(架空データの example。実データは BALANCE_*_PATH でマウント)。k3d には Argo CD v3.5.3・クラスタ内レジストリ・Application pokecalc-balance(manual sync)。新しい ADR はタイプバランスの帯 0400〜
Next: (1) データレーンの `pokedex export`(P2-3。nameJa・abilityIds・レギュレーションで絞る・特性の read model。受諾済み)が main に入ったら、balance の read model をそれに差し替え、実データで TB5 を確認する。(2) 軽微の残り: HTTP で相性表が失敗したときの 500 テスト、typed nil の provider、read model の JSON Schema、CoverageMultiplier の nullable enum、HTTP 層の検証・422 変換の重複(analyze・coverage・threats・recommendations)、recommend の穴の算出を AnalyzeDefense/AnalyzeCoverage の集計に寄せる。(3) Web / iOS から balance を使う画面は各レーンの範囲(必要なら DECISIONS.md で依頼)
メモ: `make balance-k3d-deploy`(local overlay)で上書きすると Application は OutOfSync になる(manual sync なので戻らない)。GitOps に戻すときは Argo CD で Sync

## Speed
Lane: 素早さ(素早さ比較サービス。`services/speed/`・`web/src/speed/`。どの AI が進めてもよい)
Active: なし
Branch: feat/speed-s0(作業ディレクトリ ~/MyDamageCalcurater-speed)
Status: 未着手(2026-09-22 にレーンを新設。ユーザーの仕様は docs/plan.md の「SP: 素早さ比較」と DECISIONS.md)
Next: SP0 から。docs/speed-design.md(設計の正)と ADR-0600 を書き、services/speed の基盤(タイプバランスの services/balance と同じ構成: 純粋な Go のコア・HTTP API・自前の openapi・Kustomize)を作る。種族の素早さ種族値と使用可能集合は pokedex の read model(データレーン P2-3 の `pokedex export`)から読む。それまでは架空データで作る。実数値の式は engine の公開 API(RealStats 等)を呼ぶだけで、自前で持たない

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
