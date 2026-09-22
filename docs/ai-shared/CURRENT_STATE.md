# Current State

## Damage Calculator
Lane: データ(engine・マスタ・pokedex。どの AI が進めてもよい。COORDINATION.md)
Active: Claude Code
Branch: feat/claude-p1-engine(作業ディレクトリ ~/MyDamageCalcurater)
Status: Phase 1・P2-1・P1-10・Phase R・P1-13・P1-11・P1-12・P2-1b・P2-1c・P2-2a・P2-2b・P2-2c・P2-2d・P2-3(pokedex-svc。内部 API・公開 API・natures・balance/speed 向け export。ADR-0105)は完了(critic レビュー済み)
Next: P2-3b(無効・吸収の特性を engine・DB・export に足す)→ P3-1〜3(API レーンが実装中。gateway を pokedex-svc に向ける依頼は main 統合後に送る)→ P4-1〜7。人間の確認待ち(plan.md ブロッカー): 観測%の丸め方(整数%表示は確認済み)、公開のタイミング(LICENSE・クリーンコピー)、P2-1c の裁定

## API
Lane: API(calc-svc・gateway・契約テスト。`api/openapi.yaml` の持ち主。どの AI が進めてもよい)
Active: なし
Branch: (次は main から feat/api-<名前> を切る。作業ディレクトリ ~/MyDamageCalcurater-api)
Status: Phase 3 完了(PR #14・#23・#30)、P3-4 マスタを pokedex-svc の内部 API から(ADR-0204。PR #42)、P3-5 gateway の GATEWAY_WEB_URL(ADR-0205。PR #54)、pokedex-svc の契約 description を ADR-0105 に合わせる(PR #59)は main に統合済み。ユーザー指示(2026-09-22)により利用枠をデータレーンに集中させるため一旦停止
Next: データレーン依頼 d(gateway の /api/pokedex/* を pokedex-svc(Service 名 pokedex、ポート80)へ、calc の local overlay を CALC_MASTER_URL=http://pokedex の URL 方式へ切り替え)。疎通確認には pokedex-svc の実データ投入が要る(データレーンの docs/runbooks/data.md の手順: make up → make import-fetch/import-dry-run → make import または make import-k8s → kubectl -n pokecalc get deploy pokedex で Ready 確認)。その後は DOC-api(calc・gateway の README を coding-rules §8 に、docs/runbooks/api.md)

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
Active: Claude Code(ユーザー指示で TB6 完成まで再開)
Branch: 次は main から feat/tb-<名前> を切る(作業ディレクトリ ~/MyDamageCalcurater-tb。git worktree)
Status: 設計書(docs/type-balance-design.md)の TB0〜TB6 はすべて完了・main に統合済み。P2-3b(特性の無効・吸収。PR #61)の実データ確認はデータレーンの export 再生成待ち(クラスタ再デプロイが要るため他レーン再開のタイミングで実施予定)。メガフォームの nameJa が英語表記のままの件はデータレーンへ確認候補として残る(ブロッカーではない)
Next: (1) データレーンが export を再生成したら `make balance-k3d-deploy-readmodel && make balance-smoke-readmodel` で特性の無効・吸収を実データ確認する。(2) `web/src/api/balance.gen.ts` の再生成(TB6 の move-range 追加分)は Web レーンの範囲(このレーンでは行わない。DECISIONS.md に申し送り済み)。設計書の TB0〜TB6 はすべて完了
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

## Judge(判定。素早さ×ダメージ連動)
Lane: 判定(どの AI が進めてもよい。COORDINATION.md)
Active: なし
Branch: feat/judge-j0(作業ディレクトリ ~/MyDamageCalcurater-judge。git worktree。origin/main から作成済み)
Status: 未着手(2026-09-22 にレーンを新設。ユーザー要望)。設計は docs/judge-design.md(起草のみ。ADR は未作成)
Next: docs/judge-design.md §4 の未決事項(同速の扱い・相手の技を含めるか・gateway 経由か)を確認してから JD0(基盤)→ JD1(抜ける+倒せるの最小構成)。engine を直接呼び、pokedex-svc と calc-svc の公開 API だけに依存する(speed-svc には依存しない)

## Shared Interfaces
- Pokemon ID: pokedex-svc の `{図鑑番号4桁}-{フォルム3桁}` 形式に準拠
- Type: 18タイプの英語小文字ID(fire, water, ...)。表示名・色は docs/design.md のトークンに準拠
- Type multiplier: 分数ではなく整数表現(claude-review.md 参照)
- サービス境界: damage-calc と balance は兄弟。相互の実行時 API へ直接依存しない
- 共通マスタ: 正本は1つ。`feat/claude-p1-engine` の ADR-0002 にあるコミット済みスナップショット案を候補とし、人間の確認待ち
- balance の type chart: P1-13 の `testdata/golden/typechart.json` をバイト複製して同梱(ADR-0015)。正式マスタ確定後に provider を差し替える
- 共有状態の正本: `origin/main` の `docs/ai-shared/`。未マージの続きは各レーン欄の `Branch` の最新コミット(COORDINATION.md)
- 開発の正本: private の `origin` の `main`(PR でのみ更新。Argo CD が参照)。作業ディレクトリはレーンごとに `~/MyDamageCalcurater`(ダメージ計算)と `~/MyDamageCalcurater-tb`(タイプバランス)
