# Current State

## Damage Calculator
Lane: データ(engine・マスタ・pokedex。どの AI が進めてもよい。COORDINATION.md)
Active: Claude Code
Branch: feat/claude-p1-engine(作業ディレクトリ ~/MyDamageCalcurater)
Status: Phase 1・P2-1・P1-10・Phase R(R-2-9 の公開用クリーンコピーは公開時に実施)・R-3・P1-13(タイプ相性表のデータ化。ADR-0013)・P1-11(表示%の分離)・P1-12(逆算の再設計。ADR-0010 §R)・P2-1b(ゴールデンを Champions へ)・P2-1c(技の使用可否の裁定)・P2-2a(pokedex のスキーマと migrate。ADR-0100)・P2-2b(importer の取得・変換・投入。ADR-0101)・P2-2c(照合と差分報告・版の固定・習得技は進化前から継がない。ADR-0103。実データの dry-run が通る)・P2-2d(マスタの定期取込の CronJob。毎週土曜 12:00 JST・固定版だけ投入・新しい版は報告だけ。ADR-0104)は完了(critic レビュー済み)
Next: DOC-data(engine・pokedex・importer・golden の README と docs/runbooks/data.md)→ P2-3(pokedex-svc。内部 API・natures・balance/speed 向けの export を含む) → P3-1〜3 → P4-1〜7。人間の確認待ち(plan.md ブロッカー): 観測%の丸め方(整数%表示は確認済み)、公開のタイミング(LICENSE・クリーンコピー)、P2-1c の裁定

## API
Lane: API(calc-svc・gateway・契約テスト。`api/openapi.yaml` の持ち主。どの AI が進めてもよい)
Active: Claude Code
Branch: feat/api-gateway-web(作業ディレクトリ ~/MyDamageCalcurater-api)
Status: Phase 3 完了(PR #14・#23・#30)、P3-4 マスタを pokedex-svc の内部 API から(ADR-0204。PR #42)。gateway の GATEWAY_WEB_URL(ADR-0205)は critic PASS(NG 2回のあと3回目)。origin/main を取り込み中
Next: (1) GATEWAY_WEB_URL(P3-5)の PR → main。(2) データレーンの依頼 a〜c: openapi の /api/pokedex/* の description(503 は pokedex-svc 自身の master_unavailable もある・検索は既定レギュレーションの使用可能集合を ID 順・format は v1 で無影響・getSpecies は集合外も返し learnset は使用可能な技だけ・404 not_found・limit 範囲外は 400 invalid_input)と MasterSpeciesAbility.slot を 1..4 に。(3) DOC-api(calc・gateway の README を coding-rules §8 に、docs/runbooks/api.md)。(4) pokedex-svc(P2-3)が main に入ったら、gateway に GATEWAY_POKEDEX_URL=http://pokedex、calc の local overlay を CALC_MASTER_URL=http://pokedex に、smoke の /api/pokedex を 503→200 に

## Web
Lane: Web(`web/`・Playwright。どの AI が進めてもよい)
Active: Claude Code
Branch: feat/web-p4(作業ディレクトリ ~/MyDamageCalcurater-web)
Status: P4-1〜P4-6・P4-8〜P4-11・P4-12a(タイプバランス画面の防御相性・攻撃範囲。ADR-0303)・P4-14・DOC-web 完了(critic PASS。main 統合済み)。
GATEWAY_WEB_URL(API レーンの ADR-0205)が main に入り、k3d の http://localhost:8080 で画面(/calc・/reverse・/balance)と API が揃うことを実地確認し verify-m1.md に反映。
P4-5 は Chrome で確認済み(Safari は未確認)。P4-7 は verify-m1.md のドラフト(pokedex-svc・契約テストを待つ)
Next: P4-12b(仮想敵 threats・おすすめタイプ recommendations)。続いて P5-5(構築ビルダー等)は record/team の API 待ち

## iOS
Lane: iOS(`ios/`。M3 の Phase 6。どの AI が進めてもよい)
Active: Claude Code
Branch: feat/ios-p6(作業ディレクトリ ~/MyDamageCalcurater-ios)
Status: P6-1(ADR-0500)・P6-2a 計算画面・契約追従は main に統合済み(PR #31)。P6-2b 逆算画面(critic PASS)・生成の internal タグ除外・DOC-ios(ios/README.md を coding-rules §8 の形に、ADR-0501・docs/runbooks/ios.md。critic PASS)はブランチにあり未 PR。`make ios-test`(XCTest 174 件・XCUITest 9 件)が緑
Next: PR(P6-2b・internal タグ除外・DOC-ios をまとめて main へ)→ P6-2c 構築(端末内保存の TeamStore、Showdown 形式は後回し)→ P6-3 → P6-4(手順書は AGENTS.md「手順書の書き方」)

## Type Balance Checker
Lane: タイプバランス(どの AI が進めてもよい。COORDINATION.md)
Active: Claude Code
Branch: 次は main から feat/tb-<名前> を切る(作業ディレクトリ ~/MyDamageCalcurater-tb。git worktree)
Status: TB0〜TB5 と整備(ADR-0402 の read model の JSON Schema を含む)は完了・main に統合済み。balance は Echo v5.3.1。ポケモン・技・特性は temporary の read model(架空データの example。実データは BALANCE_*_PATH でマウント)。k3d には Argo CD v3.5.3・クラスタ内レジストリ・Application pokecalc-balance(manual sync)。新しい ADR はタイプバランスの帯 0400〜
Next: データレーンの pokedex export(ADR-0105)が main に入ったら、`make balance-k3d-deploy-readmodel && make balance-smoke-readmodel`(docs/runbooks/balance.md の 2b)で実データの動作を確かめる。無効・吸収の特性は export に出ない(データレーンの P2-3b でユーザーに要否を確認中)
メモ: `make balance-k3d-deploy`(local overlay)で上書きすると Application は OutOfSync になる(manual sync なので戻らない)。GitOps に戻すときは Argo CD で Sync

## Speed
Lane: 素早さ(素早さ比較サービス。`services/speed/`・`web/src/speed/`。どの AI が進めてもよい)
Active: Claude Code
Branch: feat/speed-s2(main から作成済み。SP1 は feat/speed-s1 → PR で main に統合。作業ディレクトリ ~/MyDamageCalcurater-speed)
Status: SP0(ADR-0600。基盤・計算コア・read model・一覧 API・Kustomize)と SP1(ADR-0601。6 行のプリセット・速い順・同速の段・`presets` での絞り込み・`GET /api/speed/v1/table`)は完了・main に統合。DOC-speed(README・手順書 docs/runbooks/speed.md。k3d 疎通を確認済み)も完了
Next: SP2(自分の位置: 最小の選択 = プリセット uninvested / neutral-max / max + スカーフ on/off、オプション = SP 0〜32・性格3通り・ランク -6〜+6・スカーフ、または実数値の直接入力 → 実数値と表の中の位置(速い段・同速の段・遅い段の境目))→ SP3(`web/src/speed/` の画面は素早さレーンのまま(ユーザー決定。Web の P4-13 は取り消し)。タブ・URL は Web の P4-10 のルート表(1か所)に `/speed` の1項目を足すだけ。P4-10 は PR #44 で main に統合済み。足すのは3か所に1件ずつ(App.tsx は触らない): web/src/app/routes.ts の SCREEN_ROUTES に `{ id: "speed", segment: "speed", label: appText.speedTabLabel }`、web/src/i18n/ja.ts の appText に speedTabLabel、web/src/app/screens.tsx の SCREEN_COMPONENTS に `speed: SpeedScreen`(props は ScreenProps = {engine, master}。使わなくてよい)。テストの例は web/src/App.routing.test.tsx と web/e2e/routing.spec.ts)→ SP4(pokedex の read model・k3d・GitOps)。SP4 までに決める: 空の roster の扱い(いまは read model が空を拒否。pokedex の adapter では 503 か空配列か。SP1 critic 軽微)。SP4 の read model: データレーン P2-3 の pokedex export(ADR-0105)が素早さ専用の `data/generated/readmodel/speed-pokemon.json` を ADR-0600 §4 の形(baseSpeed。既定レギュレーションの使用可能集合・ID 昇順)で出す。SP4 はそれを `SPEED_POKEMON_PATH` で読む(adapter の差し替えは不要の見込み。local overlay への載せ方と k3d の疎通を行う)。P2-3 が main に入るまでは架空データ

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
