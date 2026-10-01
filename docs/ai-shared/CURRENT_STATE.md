Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

# Current State
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。


Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

## Damage Calculator
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Lane: データ(engine・マスタ・pokedex。どの AI が進めてもよい。COORDINATION.md)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Active: Claude Code(2026-10-01〜。issue #403 の残りパッケージを D22 から順に)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Branch: 次は main から feat/data-<名前> か fix/data-<名前> を切る(作業ディレクトリ ~/MyDamageCalcurater は main 追従の確認用。作業は git worktree で)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status: Phase 1・P2-1・P1-10・Phase R・P1-13・P1-11・P1-12・P2-1b・P2-1c・P2-2a・P2-2b・P2-2c・P2-2d・P2-3(pokedex-svc。内部 API・公開 API・natures・balance/speed 向け export。ADR-0105)は完了(critic レビュー済み)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): P2-3b(無効・吸収の特性)も完了・main 統合済み(ADR-0106)。calc・gateway の pokedex-svc 接続(API レーンの依頼)も PR #87 で解決済み(api-smoke で master=pokedex 確認済み)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): P5-6(技の追加効果によるランク変化。ADR-0107)完了・critic PASS(1往復)・**main 統合済み(PR #132)**。engine は乱数を持たず「発動した場合の値」だけを返す。ゴールデン不変。`move_effects` 別表・`MasterMove.effect`(内部API)まで。公開APIへの露出(判定レーンが技IDからランク変化を引く経路)は判定レーンの要件確定後に別途対応。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): issue #110 のデータレーン担当分(ADR-0108)完了・critic PASS(1往復)・**main 統合済み(PR #138)**。`engine.CalcBulk`/`CalcReverse` と `engine/wasmapi` に ADR-0208 §1 と同じ件数・範囲の上限を追加し、wasmapi は DTO 変換より前に検査して HTTP との parity を確保。issue #110 は Web・iOS レーンの追従が残っている限りクローズしない。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): issue #106(排他制御)完了・critic PASS・**main 統合済み(PR #155)**。`tools/importer/cronjob.sh` に `flock`(非ブロッキング)を追加し、手動Job(`make import-k8s`)と定期CronJobの同時実行を防ぐ(ADR-0109)。Docker(Linux)と実クラスタ(k3d)の両方で実際の排他動作を確認済み(2026-09-23。2つの手動Job同時作成→片方がロック競合で即exit 1→backoffLimitで再試行して成功)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): 2026-09-23、全レーンの main 統合済みの変更をまとめてk3dに再デプロイし、実データ(種族349・技515・move_effects 59件)で計算・一括計算・逆算・タイプバランス・素早さ・判定(JD3複数候補)まで実HTTPで動作確認済み。すべてgreen。既知の制約: 技を個別IDで引く公開APIが無く(`GET /api/pokedex/moves/{id}` は404)、Webのオンライン技選択・持ち物候補比較・判定JD4(相手の技を含めた返り討ち判定)がブロックされたまま。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): 2026-09-24、getMove(P3-7。APIレーンが`services/pokedex/`へ越境実装)をレビュー。既存の設計判断(命名・エラー変換・テストの流儀)と食い違いなく、修正不要と判断(DECISIONS.md参照)。判定レーンはJD4に着手可能。上記の「技を個別IDで引く公開APIが無い」制約はこれで解消(バッチ解決はまだ無いのでWebのオンライン技選択は引き続きブロック)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): issue #104(DB資格情報の最小権限分離)完了・critic PASS(1往復)・**main 統合済み(PR #176)**。`pokedex_reader`/`pokedex_importer`/`pokedex_migrator`の3ロールに分離(ADR-0110)。実クラスタで`SHOW GRANTS`により権限が過不足なく一致することを確認済み。既存クラスタからの無停止移行も実地確認済み。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): issue #109(HTTPタイムアウト・graceful shutdown)完了・critic PASS(1往復。指摘なし)・**main 統合済み(PR #178)**。`newHTTPServer`/`serve`/`runServe`の3層分離(ADR-0111。services/balanceと同じ値)。`terminationGracePeriodSeconds: 30`を追加。実クラスタで再デプロイ・確認済み。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): issue #112(DB接続プール上限)完了・critic PASS(1往復。軽微指摘1件反映)。4環境変数を`services/pokedex/db.OpenPool`経由で適用(ADR-0112)。実クラスタで再デプロイ・確認済み。**main 統合済み(PR #180)**。**これでデータレーン主担当のCodexレビューissue(#104・#106・#109・#112)はすべて完了・main統合済み**。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): issue #102(importer の中断キャッシュ自己回復。ADR-0113)を修正。`showdown-cache.mjs` へ切り出し、一時名+検証+rename。テスト7件を `make test-tools` に接続。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): 2026-09-25「open issue 全件解決」(3 回の全体レビューの 143 件を仕分け)。データレーンで main に入れた PR: #343(#76)・#346(#303・#77・#71 engine)・#347(#269・#310・#311)・#354(#347 の後退の修正)・#355(#231)・#358(#251・#74 データ)・#359(#255・#317)・#361(CI #215)・#364(#280)・#368(#270 効果データ)・#369(防御プリセット)・#376(#271 技の機構)・#378(deploy-latest の migrate・importer)・#380(#379)・#381(未対応の印)・#384(#221・#278・#312)・#387(#386 MySQL probe)・#401(pokedex イメージの registry push)・#402(#272 engine)。あわせて #338(8080 の白画面 #268・手順書の作り直し)。#271・#270・#272 は API・Web・iOS・判定の表示待ち(各 issue のチェックリスト)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): 2026-10-01、D22(Makefile の help・未実装ターゲット・k8s-render の全レーン描画・tidy/deps-outdated の全モジュール・/verify・importer の未来 checkedAt と破壊操作の検査。#261・#294・#321-lint・#75 の一部・#286-assets)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): 2026-10-01、D10(issue #220・ADR-0127)完了。内部 API(/internal/pokedex/master)と pokedex export の全 SELECT を1つの読み取り専用トランザクション(readtx)に入れ、importer の全置換と重なっても新旧が混在しないようにした。検索系は autocommit のまま。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): 2026-10-01、D07(共通の scripts/require-k3d-context.sh・scripts/image-tag.sh とテスト、up.sh・deploy-latest・pokedex-registry-push・import-k8s の context 検査を共通化、Secret を kubectl create で作る。#327・#295-shared・#291-shared)。各レーンの *-k3d-deploy への組み込みは各レーン(DECISIONS.md 2026-10-01)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): 2026-10-01、D23(`make test-db-docker`: Docker の使い捨て MySQL・TiDB で `make test-db` を流して消す。verify-m1 §2・test-strategy L8。#223)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): issue #403 D20 のデータ部分(#281-data・#108-a・#259-a。ADR-0128)実装済み。dataVersion を source=version@checksum先頭8桁にし(内部 API と export が internal/dataversion を共有)、export に metadata.json・type-chart.json を追加(6ファイル)。#211-data は API レーンの契約待ちで残る。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): 2026-10-01、D18(issue #111・ADR-0104 追記)。importer の PVC に容量の事前確認(`prune.mjs check`、不足は終了コード3)と、現在版+直前の成功版・report 52 件の保持 prune(`prune.mjs prune`、DB apply 成功後)を追加。手順は docs/runbooks/data.md。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): 2026-10-01、issue #437(取り込み中に MySQL が OOMKill)を修正。memory.cnf(performance_schema=OFF 等)と limit 768Mi。k3d に反映済み(待機 499Mi → 165Mi、全置換3回で restart 0)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): 2026-10-01、D11(issue #107・#323・#324・#299 の pokedex 分。ADR-0129)。`/readyz`(DB の最小条件に連動)・DB 呼び出しの締め切り5秒・preStop sleep 5秒。新規クラスタは初回 import 前に pokedex が Ready にならない(deploy-latest は失敗時に make import-k8s を案内)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): issue #403 D24(#300・#74 のスクリプト部分。ADR-0130)実装済み。check-publishable の B に DSN・URL 資格情報・MYSQL_PWD・Secret の base64・Bearer・各種トークン接頭辞・短い値・2行に分かれる値を、C と .gitignore に .envrc・id_rsa 系・credentials.json・*.p8・*.sql.gz・ダンプ・secret*.yaml を追加(gitleaks は足さない)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status: D12(issue #277・ADR-0131 採用)実装済み。migration 000009 の種族 key の台帳(追記だけ)と、ID の消滅(ErrKeyRemoved・終了コード3・`-allow-removed <種類>:<ID>`)・消滅後の再利用(ErrKeyChanged)の検出を Apply に追加。実データの dry-run は blockers: none。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): 2026-10-01、D25(issue #240・ADR-0132)。pokecalc に ingress の default-deny と許可リスト10本(`deploy/k8s/base/networkpolicy/`)を実装。受け入れテスト AC-N1〜N5 は green。k3d での実地確認(apply・smoke・拒否の確認)はメイン。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status: D27(#252・#319・#290・#221 の runbook 部分)実装済み。`docs/runbooks/{data,api}.md`・`docs/impl/{k8s-local,db-mysql,make-targets}.md`・`docs/verify-m1.md` §3 を今の main に合わせて直した(確認方法・Secret 5キー・NetworkPolicy・終了コード3と PVC 消失の復旧手順・行番号の除去)。新しいクラスタでの verify-m1 §3 の通し実行は人間の確認待ち。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status: D29(#226・#296・#314・#224・#254 の索引・#256 の C・#227 のデータ分)実装済み(文書のみ)。README を現状(動くもの・未実装は assets のみ・起動手順)に、overview の状態列を plan.md への委譲に、requirements に3機能と契約4本の索引、test-strategy にサービス別の索引、ゴールデン関連(known_diffs・gen9 表記・1,392 種族)を実態に直し、ADR-0002・0011・0012・0100・0101・0104・0105・0108 の状態欄を実装後の事実(PR 番号)に更新。CLAUDE.md・docs/impl の known_diffs 記述は他担当(D31)。0207 欠番は API レーンの判断。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status: D19(issue #222 案A〈ユーザー確認待ち。DECISIONS.md〉・#301。ADR-0101 追記)実装済み。取得物の内容ハッシュ(Showdown 展開後ツリー・PokeAPI の各 CSV)を config.json の integrity と照合し、不一致は終了コード3。`npm ci --ignore-scripts`。CronJob を initContainer `fetch`(DSN なし)と `import` に分離し、cronjob.sh は fetch|import|引数なしと引き渡しファイルでロックの隙間を埋める。実データの再取得でハッシュ一致・dry-run は blockers: none。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Next: データレーンの open issue を解消中(2026-10-02。triage → 実装済みのクローズ → 残りの実装)。#403 の残り: D26・D28〈T05 待ち〉・D30・D31〈CLAUDE.md・AGENTS.md はユーザー確認〉・D32・D21〈T04・S04・A06 待ち〉・#211-data〈API レーン待ち〉。後続: ADR-0126 の計算量の最適化。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。


Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

## API
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Lane: API(calc-svc・gateway・契約テスト。`api/openapi.yaml` の持ち主。どの AI が進めてもよい)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Active: Claude Code(2026-10-01 再開。#284 は PR #416 で main 統合済み。キューを順に消化中)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Branch: feat/api-unsupported-mark-string(作業ディレクトリ ~/MyDamageCalcurater-api。P5-4はPR #409、#284はPR #416で統合済み。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

P5-2・P5-3・issue #271/#270はmain統合済み〈PR #372〉)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status: Phase 3・issue #110(ADR-0208。PR #130)・issue #103の設計(M2保存データの保持・削除・端末ID境界。ADR-0209。critic PASS。PR #150)は main に統合済み
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): issue #148のAPIレーン担当分(ADR-0210。私設サービスの境界)完了・critic PASS・**main 統合済み(PR #157)**。`deploy/k8s/overlays/cloud` から gateway の Ingress を削除 patch で除去し、public Ingress/LoadBalancer/NodePort/externalIPs/hostNetwork/hostPort が無いことを構造検査+`kubectl kustomize`実描画検査の2層で固定。端末ID/CORSを認証・到達制御として扱わない回帰テストも追加。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): P3-7 `GET /api/pokedex/moves/{key}`(getMove)を実装(判定レーン JD4 の依頼。ADR-0105 §3 追記)。契約・`services/pokedex/`(データレーンの範囲。越境理由と触ったファイル一覧は DECISIONS.md)まで一括実装。critic PASS(3往復)・**main 統合済み(PR #161)**。判定レーンは JD4 に着手し main 統合済み(PR #169)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): issue #69(検索の並びがOpenAPI契約と一致しない)・issue #73(OpenAPIとengineの防御プリセット集合の同期検査)を修正・**main 統合済み(PR #167・#172)**。いずれも契約・テストの整合修正で、SQL・engine・ADR は無変更(既存の設計は元々正しかった)。両issueともclose済み。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): issue #113(入力変更時の古い計算要求を抑止・キャンセル)のAPIレーン連携分(「クライアントのcancel伝播」)を修正。gatewayの`ReverseProxy.ErrorHandler`がクライアントの要求中断(`context.Canceled`)を上流障害と区別せず「上流に到達できない」WARN・503 `upstream_unavailable`を返していたのを、クライアント起因のときは何もしない(応答を書かない)よう修正(ADR-0202 §5 追記・AC-G10)。**限界**: gateway→calc-svcへのcontextキャンセル伝播自体は効くが、calc-svc・engineはcontextを見ないため(engineを純粋に保つ絶対ルール2)、issue本文の「calc-svc CPU消費も止める」は未達成のまま(中断された逆算は完走する。ADR-0208の上限で最悪計算量は有界)。Web欄の「APIレーンの連携分が残っているか未確認」はこれで解消(Web欄の更新はWebレーンに委ねる)。issue #113はWeb・iOS・APIすべてのレーン分が完了としてクローズ可能と判断(詳細はDECISIONS.md)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): P4-17(ADR-0304 §3。技のID解決の欠落)を解消・**main 統合済み(PR #196)**。`GET /api/pokedex/moves/batch`(`getMovesByIds`)を新設。ADR-0304が当初推していた案A(`learnset`を`Move[]`に変える)は不採用: iOS(M3)が`learnset: string[]`前提の出荷済み機能を持つため、型変更より新エンドポイント新設の方が契約変更として小さいと判断。critic PASS(3往復)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): `learnset`が64件を超える場合の実データを確認(クラスタ復旧後)。**349種族中151種族(43%)が64件超・最大106件**で、まれな例外ではなく日常的なケース。Web側の分割呼び出しは主経路として実装が必要と訂正・連絡済み(DECISIONS.md 2026-09-24)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): M2 P5-1(record-svc/team-svc用TiDB)着手。ADR-0211でバージョン固定(TiDB v8.5.8・TiDB Operator v1.6.6)・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

ローカル/k3dプロビジョニング・DB/ユーザー分離・migrationツール共通化・スキーマ・保持日数の環境変数契約を確定
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(critic 3ラウンド。**main 統合済み PR #204**)。実装は`services/internal/dbmigrate`への切り出し(pokedexのUp/DownAll/Versionを
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`fs.FS`引数化し、pokedexは薄いラッパーに)・grants.goへの`AppPrivileges`追加・services/record・services/teamの
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

devices/purge_journal migrationとmigrate CLI(app/migratorの2ロール)まで完了(critic 2ラウンド。**main 統合済み PR #205**)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

TiDB Operatorのk8sマニフェスト(TidbCluster・TidbInitializer)・`up.sh`配線(bootstrap非致命化・Secret作成・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

namespace・完了待ちの順序)・Makefileのmigrate-up/down/version-record/team・tidb-local-upターゲットも完了
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(critic 2ラウンド。**main 統合済み PR #266**。1回目FAILはTiDB Operator v1.6.6の実ソースを取得して
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

裏取りした結果判明した`passwordSecret`のキー名誤り・初期化用imageの誤り・namespace不一致・AC-T7違反、
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

2回目FAILは新設したk8s-renderがクラスタ未起動環境で失敗/ハングする退行)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**残**: 共有k3dクラスタへの実適用(AC-T3・AC-T8)は、他レーンが使う共有クラスタへの影響を先に確認する
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

必要があるため意図的に未実施。次の`make up`実行時にTidbCluster/TidbInitializerが実際にReady/Completedに
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

なることを確認すること(TidbInitializerが使う`tnir/mysqlclient`はamd64専用イメージのため、Apple Silicon
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

のk3dノードでの起動可否も未確認)。`grants_tidb_test.go`・`migrate_tidb_test.go`(`-tags tidb`)も
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

このサンドボックスでは実TiDBに対して未実行(tiup playgroundのpdがdarwin/arm64でクラッシュ)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`make test-db`により検証してからP5-1完了とする
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): P5-2(NATS JetStream。calc-svcからのイベント発行)完了・**main統合済み**(ADR-0212。critic 2ラウンド)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

ストリーム`CALC_EVENTS`・Retentionは意図的にLimits(Interest ではない。理由はADR-0212 §4)・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`services/internal/calcevents`のワイヤフォーマットを確定。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): P5-3(record-svc)完了・critic 2ラウンド(1回目FAIL〈重大1・重要3・軽微7件〉→修正→2回目PASS
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

〈軽微4件〉、軽微も反映済み)。`api/openapi.yaml`にrecordの契約(`deleteRecordDeviceData`・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`listFrequentOpponents`・`store_unavailable`)を追加、record-svcの保存(TiDB実装。SaveCalcEvent/
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

PurgeDevice/TouchDevice/FrequentOpponents)・NATS購読(`services/record/internal/events`)・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

gatewayルーティング/CORSのDELETE許可まで実装。critic指摘で判明した重要事項: (1)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`services/record/internal/store`にfakeしかテストが無かった欠落を`tidb_test.go`(`-tags tidb`。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`make test-db`)で解消し実TiDB(`pingcap/tidb --store=unistore`)で全緑を確認、(2)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`frequent_opponents.last_calculated_at`が再配送の順序次第で巻き戻る不具合を`GREATEST`で修正・回帰テストで
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

修正前に落ちることを確認、(3) golang-migrateのmysqlドライバがTiDBでSERIALIZABLE分離レベルを要求し失敗する
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

既知の非互換を発見(`Lock()`と`SetVersion()`の両方が原因で`x-no-lock`でも回避不可)・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`tidb_skip_isolation_level_check=1`をTidbInitializer/tidb-local-up.shに追加(ADR-0211追記。既存クラスタでは
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

手動設定かTidbInitializer再作成が必要)。失効ジョブ(ADR-0209 §4)とrecord-svcのDeployment/Service配線は
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**P5-3bへ切り出し**(plan.md参照。現状k3dでは`/api/record/*`はupstream_unavailableのまま)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

main未統合(PR #372。P5-2と同じブランチ・PRでまとめている。ユーザーのテスト確認・マージ待ち)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): P5-2・P5-3・issue #271/#270(mechanisms公開・unsupported印)を**main統合済み(PR #372)**。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): **M2(P5-1〜P5-4)完了**。P5-4(team-svc構築CRUD)実装完了。契約(`api/openapi.yaml`のteam操作。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

ADR-0213 spec-writer工程)・team-svcの保存(TiDB実装。CreateTeam/UpdateTeam/DeleteTeam/GetTeam/ListTeams/
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

PurgeDevice/TouchDevice(FromEvent))・NATS購読(`services/team/internal/events`。durable名`team-svc`は
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

record-svcと別、イベントのDetailは一切保存しない。ADR-0213 §5)・gatewayルーティング/CORSのPUT許可まで実装。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

critic 2ラウンド(1回目FAIL〈重要3・軽微6〉: (1) team_members への3クエリにdevice_id絞り込みが抜けていた
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

〈ADR-0209 §6-1違反。実害は無いが規律違反〉のを修正し回帰テストで固定、(2) 文字数上限未検証で入力エラーが
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

503 store_unavailableに化けていたのを400 invalid_inputに修正、(3) Dockerfileのserverターゲット欠落を修正
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

→ 2回目PASS)。実TiDB(`pingcap/tidb --store=unistore`)で全テスト確認済み。失効ジョブとDeployment配線は
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**P5-4bへ切り出し**(plan.md参照)。**main統合済み(PR #409)**。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): issue 272のAPI分(defenderOverride.abilityId・ReverseRequest.unknownAbilityId。ADR-0214。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

engine側はPR #402・ADR-0126で完了済み)を実装。省略時は種族の全特性(最大3件。4件目=Showdownの特殊枠"S"は
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

ADR-0105 §5と同じ判断で落とす)を解決して渡すため、1つしか特性を持たない種族は必ずその特性が効くように
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

なる。`BulkCalcRow`・`ReverseCandidate`に`abilityId`/`abilityIds`(必須)を追加。critic 2ラウンド(1回目
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

FAIL: 省略時に解決した特性のEffectが実際にengineへ届くことが無テストだった→対照種族ペアのテストを追加して
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

修正→2回目PASS)。一括計算・逆算の行数/候補数上限(ADR-0208)が特性分岐で最大3倍まで増えうることを追記。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**main未統合(PR #411。ユーザーのテスト確認・マージ待ち)**。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): issue #284(balance/speed/judgeをgatewayの後ろにまとめる。ユーザー決定・DECISIONS.md
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

2026-09-25「ユーザー決定 4 件」#2)を実装。`routing.go`に`routeBalance`/`routeSpeed`/`routeJudge`と対応する
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

prefix(record・teamと同じ前方一致・末尾スラッシュ必須の規則)、`requiresHeaderCheck`にも3つを追加して
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

端末ID・セッションIDの検証(issue #236で判明していたTraefik直結の穴)をgatewayでも課すようにした。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`server.go`に`Config.BalanceURL`/`SpeedURL`/`JudgeURL`と対応するReverseProxy、`main.go`に
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`GATEWAY_BALANCE_URL`/`GATEWAY_SPEED_URL`/`GATEWAY_JUDGE_URL`を追加。CORS許可メソッドは変更なし。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`/api/{balance,speed,judge}/healthz`(完全一致のみ)はヘッダ検証を課さない(3サービスの契約の
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`publicHealth`・ADR-0600/ADR-0700がIngress越しの疎通確認用としてヘッダ不要と明記しているため。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

critic 1回目FAILで発覚し修正済み)。deployment.yamlへの実URL配線はrecord・team(P5-3b/P5-4b)と
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

同じく別タスクとして残す(コードのみ今回のスコープ)。critic 2ラウンド(1回目FAIL〈重要2件:
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

healthz例外の欠如・README.mdのルーティング表が古いまま〉→修正→2回目PASS)。**main未統合**。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): 2026-10-01 PR #416(issue #284: balance・speed・judgeをgatewayの後ろに統一)を main 統合。続けて UnsupportedMark の target・reason を string に緩めた(ADR-0215。Web・iOS の追従込み)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Next: キュー順に対応:
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(2) defenderOverride.ranks/status は実装済み(ADR-0216。critic・コミット・PR 待ち)、(3) P5-3b・P5-4b(失効ジョブ・Deployment配線。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

issue #284のdeployment.yaml配線も含む。優先度低)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

issue #103・#148の依頼(データ・Web・iOS・運用レーンへ)、getMove 実装の再レビュー依頼(データレーンへ。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

60fbe25で対応済み)・iOS再生成依頼(a1f5d5eで対応済み)、P4-17完了(Webレーンへ連絡予定)はDECISIONS.mdに記録済み
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。


Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

## Web
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Lane: Web(`web/`・Playwright。どの AI が進めてもよい)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Active: Claude Code(ユーザー指示で再開)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Branch: feat/web-p4(作業ディレクトリ ~/MyDamageCalcurater-web)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status: P4-1〜P4-6・P4-8〜P4-12(仮想敵 threats・おすすめタイプ recommendations を含む。ADR-0303)・P4-14・P4-15・DOC-web・P4-7(M1 完了報告)完了(critic PASS。main 統合済み。PR #82・#84)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

データレーンの依頼(P2-3b・ADR-0106)にも追従済み(PR #84): AbilityEffect に defImmuneTypes・defAbsorbTypes、
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

exportBalanceReadModel に absorb と ADR-0106 §決定7の出力順・無効優先。本物の engine.wasm で無効・吸収を結合テスト確認済み。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

verify-m1.md を完成版にした: P2-2c/d・P2-3・P3-3 が main に入り、k3d(gateway 経由 http://localhost:8080)で
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

計算・逆算・タイプバランス(仮想敵・おすすめタイプ含む)を実地確認(pokedex-svc は実データ投入済みだが、
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

gateway/calc-svc のマスタ参照先はまだ pokedex-svc に向いていない。API レーンの依頼 d が一時停止中)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

P4-5 は Chrome で確認済み(Safari は未確認。人間の作業)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**P4-16・P4-16b・P4-16c(オンライン MasterSource。ADR-0304)完了・main 統合済み(PR #128・#134・#153)**:
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`createOnlineMasterSource`(持ち物・性格を全件取得、種族は検索専用インターフェース)、`MasterData.capabilities`
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(公開 API に無い機能を明示的に無効化)、画面側(CalcScreen・ReverseScreen は技・持ち物候補比較が無効なとき
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

disabled+案内、種族一覧が無効なとき検索欄 `SpeciesSearchField`。BalanceScreen は speciesList・moves が両方
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

そろうまで画面ごと無効化)、検索欄のキーボード操作(WAI-ARIA "List Autocomplete with Automatic Selection")と
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

CSS。3段階とも critic 1回目 FAIL→修正→2回目 PASS で完了(重大バグ・文言バグ・IME変換中のキー横取りなど、
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

いずれも critic が発見)。既存752件は無変更のまま最終907件。技の ID→実体化(`getSpecies.learnset`)は公開 API
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

に手段が無く、データ/API レーンへ既定案付きで提案済み(DECISIONS.md 2026-09-23、未回答・急ぎではない)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**P4-19(issue #110 セキュリティ。ADR-0300 §10)も完了・main 統合済み(PR #142)**: 持ち物候補を配列を作る
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

最終地点で64件に決定的に絞り込み、観測は16件で disabled+案内。critic PASS(境界値の網羅探索と変異テストで
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

上限超過が起きないことを確認)。データレーンの engine/wasmapi 側(ADR-0108・PR #138)も main 統合済み。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

issue #110 は iOS の追従待ちで Web 単独ではクローズしない。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**P4-18 issue #99(ライトテーマの danger コントラスト不足)完了・issueクローズ済み**: Web 分(PR #164)は
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

danger のライト値を `#E5484D`→`#CD1D23` に変更(WCAG 2.2 SC 1.4.3 の4.5:1を bg.base・bg.glass 合成後の
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

両方で満たす)。`web/src/test/colorContrast.ts`・`web/src/styles/contrast.test.ts` を新規追加。critic PASS
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(独立実装での検算・変異テストで確認)。iOS 側も完了(`fix: ライトテーマの danger コントラスト不足を修正
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(issue #99)`。`PokeCalcDesign.swift`・`ColorContrast.swift`・`DangerContrastTests.swift`)。issue #99 は
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

クローズ済み。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**issue #113(逆算の古い計算要求の抑止・キャンセル)は Web 分完了・main 統合済み(PR #174)**: 観測のテキスト
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

編集のみ200msのtrailing debounce、確定操作は待たない。`CalcEngine` に任意引数 `signal?: AbortSignal` を
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

追加、取り消しは `REQUEST_ABORTED_CODE` で区別(ADR-0300 §11)。critic PASS(非同期・競合状態を重点検証。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

mutation テスト9件で確認)。iOS 側も完了(`fix: iOS の計算・逆算で古い計算要求をキャンセル・観測入力を
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

debounce (issue #113)`。`LatestTaskRunner.swift`・`CalcInput.swift`)。**issue #113 自体はまだ open**
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(API レーンの「クライアントのcancel伝播」連携分が残っているかは未確認。Web・iOS とも自レーン分は完了)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**P4-17(技の ID 解決。ADR-0304 A-13)完了・main 統合済み(PR #202)**: API レーンが新設した
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`GET /api/pokedex/moves/batch`(`getMovesByIds`)を使い、オンラインモードの技選択を復活させた。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`resolveSpecies` が learnset を技の実体に解決して返す設計(`MasterSpeciesResolution.moves`)。`learnset` が
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

64件を超える場合はチャンク分割して並列に複数回呼ぶ(API レーンが実データで確認: 349種族中151種族・43%が
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

64件超・最大106件。稀な例外ではなく主経路として実装・テスト)。`capabilities.moves` の意味は変えずオンラインで
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`false` のまま(`true` にすると BalanceScreen が「有効なのに技が選べない」壊れた状態になるため)。技セレクトの
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

disabled 判定は「今の種族の技候補があるか」に変更。critic レビュー: チャンク分割・結合順序・全体失敗の扱いを
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

mutation テストで確認(全滅)。重要指摘1件(攻守入れ替え・与えた/受けた切り替え後の選択中の技〈候補一覧だけで
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

なく実際にリクエストに乗る技〉が未検証。`<select>` の DOM 値は状態が壊れていても先頭候補にフォールバック表示
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

するため見逃しやすい)を受け、実際のリクエストを検査する形に既存テスト2件を強化。既存1192件は無変更・新規
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

28件追加(1220件)。BalanceScreen 自体の種族検索・技選択は P4-17b として積み残し(下記で完了)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**P4-17b(BalanceScreen のオンライン対応。ADR-0304 A-14)完了・main 統合済み(PR #207)**: P4-17 で
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`capabilities.moves` が永続的に false のままと決まった結果、従来のゲート `speciesList && moves` では
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

BalanceScreen がオンラインで永久に使えなかった。可否の判定を「一覧がそろっているか」から「入力の口が
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

あるか」(`(speciesList || masterSearch) && (moves || (!speciesList && masterSearch))`)に置き換え、
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

パーティ・仮想敵の12枠(6枠×2)それぞれで種族検索→技解決(`useSpeciesResolutions` を1画面で共有)を
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

独立に行えるようにした。`moveById`/`hasDamagingMove` を fail-closed に直し、実体不明の技 ID を攻撃技と
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

誤判定して誤解を招く診断(coverage の誤呼び出し)を出さないようにした。critic PASS(mutation testing で
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

ゲート条件・fail-closed 判定・種族解決の登録漏れ等の主要な変異を全て検知)。critic 指摘の軽微3件は
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

その場で直接修正: 種族解決の適用を index ではなく枠の id で引くよう変更(検索解決を待つ間に他の枠が
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

削除されると index が別の枠を指しうる競合の根治)、A-14.1 の境界表(7パターン)の未カバー2行のテスト追加、
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

特性名解決の重複ロジックの統一。新規15件追加(1243件)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**P4-21(issue #67・#98)完了・main 統合済み(PR #189・#192)。Codexレビューissue(P4-18・P4-21)はこれで
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

すべて完了**:
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

- #67(2xxの契約外JSONでAPIクライアントが例外を投げる): `apiEngine.ts`・`balanceClient.ts` の `postJson` を
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

  型ガード経由にし(`as Schemas[...]` の型アサーションを除去)、契約外の2xxで例外を投げず
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

  `engine_unavailable`/`balance_unavailable` を返すようにした(ADR-0301 §4・ADR-0303 §6)。calc側は写像関数が
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

  読む全フィールドを再帰的に検査、balance側は画面がたどる形だけを検査(leafスカラー・enumは見ない)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

  critic PASS(mutation テスト12件で型ガードの過不足なしを確認)。新規143件追加。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

- #98(モバイル幅で計算・逆算画面が横に溢れる): ブレークポイント600px(`docs/design.md`「幅への対応」に記録)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

  600px未満はmobile-firstで縦積み(DOM順・フォーカス順は不変)、600px以上は従来の左右配置。伸縮列を
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

  `minmax(0, 1fr)` に、カード・selectに `min-width: 0`/`width: 100%` を追加。critic が実ブラウザで13幅×5画面を
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

  実測し横溢れゼロを確認。新規34件追加(単体18・E2E16)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

  作業中に発見した無関係の既存退行(JD5の判定タブ追加で `a11y.spec.ts` が壊れていた)を別途修正・main統合済み
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

  (PR #191)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

  最終テスト数: 既存1166件は無変更のまま vitest 1184件・Playwright 31件、すべて green。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**issue #268(gateway経由:8080の白画面)の検証テスト修正・main統合済み(PR #338。実装は別セッション)**:
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`services/gateway/scripts/smoke.sh` に追加された「index.htmlが読むJSを実際に取得して200」の検査により、
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`TestSmokeScriptAcceptsWebDeployed` のfixtureがscript srcの無いHTMLを返していて落ちていた(タイプバランス
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

レーンが検証・指摘)。fixtureにscript srcとその配信先を持たせ、JSが404のケースで検知できることの回帰テスト
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(`TestSmokeScriptFailsWhenEntryJSMissing`)も追加。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**issue #334(相性表記の倍率併記。iOSとの語の統一)完了・main統合済み(PR #341)**: iOSのDisplayLabels.swiftの語
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(「ばつぐん(×2)」「いまひとつ(×0.5)」)にWeb側を揃えた(「効果は」接頭辞は削除)。format.test.tsの0.25/0.5・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

2/4のケースが倍率を書き分けるようになり検証強化。新規0件(既存アサーション4件の強化)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**issue #72(ルートmake e2eが未実装スタブ)完了・main統合済み(PR #350。ADR-0306)**: k3dクラスタ不要な3件
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(`web-e2e`→`web-e2e-online`→`web-e2e-balance`)を必ずこの順で実行し、kubectlの現在のコンテキストが
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`k3d-$CLUSTER`のときだけ`api-smoke`→`web-k3d-smoke`→`web-k3d-e2e`を追加実行するよう`scripts/e2e.sh`を実装。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

クラスタが無ければ黙らずスキップを明示、`E2E_REQUIRE_K3D=1`でスキップさせない逃げ道も用意。critic PASS
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(mutation testing 4件で全て検知)。`scripts/e2e_test.sh`(新規80件)を`make test-scripts`に追加。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**issue #71のWeb側(攻撃側プリセット単一化。ADR-0114)完了・main統合済み(PR #352)**:
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`web/src/domain/attackerPresets.contract.test.ts`を新規追加。毎回`engine/presets/attacker.json`を読み、
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

カタログ(順序・既定値・relevantStat・boostMinus・relevantSp・nature)から導いた期待値と
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`resolveAttackerPreset`の実際の出力を突き合わせる契約テスト(現状の値は一致済み、実装変更なし)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

JSONを一時的に書き換えるmutationで実際に検知することを確認済み。新規17件追加。iOSの追従が済めば
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

データレーンが#71をcloseする想定(2026-09-25時点、Web側は完了を連絡済み)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**issue #333(375px幅でタブの名前が1文字ずつ縦に折り返す)PR #356オープン中(2026-09-25、マージは
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

オーケストレーターが検証後に実施)**: `App.css`の`.app-tabs__list`にoverflow-x: auto・safe center、
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`.app-tabs__tab`にwhite-space: nowrap・flex-shrink: 0。**critic 1回目FAIL**: 素のcenterのままだと
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

はみ出した先頭タブがscrollLeft=0でも戻れない(centered flexbox overflow clipping。320pxで実測再現)→
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`justify-content: safe center`に修正、design.mdに記録。safeキーワードのSafari対応はP4-5のSafari確認
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(ブロッカー節)に追記。回帰テスト2件(1行であることの直接確認・スクロールで先頭末尾に到達できることの確認)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**2026-09-25、オーケストレーター(damage calculation bug resolution)から13件のissue消化を依頼された**
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(open 113件中、優先度順): (1) bug: #333(完了・main統合済み。PR #356)・#306(タイプ名コントラスト・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

ダメージバー読み上げ名。完了・main統合済み。PR #362)・#275(逆算「受けたダメージ」で自分の耐久が無振り固定。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

high severity。完了・main統合済み。PR #366)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(2) ready-for-implementation: #304(完了・main統合済み。PR #375)・#308・#305・#248・#218・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

#219(APIレーン連携)・#211(APIレーン連携)・#332(devDependencies更新)・#226(README等の実装状況)は
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

#304以外未着手。(3) needs-decisionだが「要望済み機能は実装しきる」方針で既定案付きで実装: #272(特性選択)・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

#274(急所・やけど・天候・フィールド・ランク・壁・特性の指定。iOSへも連絡済み)・#210(オフライン実データ)は
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

未着手。P5-5はAPIレーンの契約が出たら最優先。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**#71のWeb側(攻撃側プリセット単一化)完了・main統合済み(PR #352)**。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**`make e2e`のweb-e2e-onlineがmainで壊れていた件(廃止済みCALC_TYPECHART_PATHをcalc-svcが拒否)を発見・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

2PRに分けて修理・両方main統合済み**: PR1(#382、MasterExport追従。ADR-0204/ADR-0301§5追記)で
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`web/src/master/exportSnapshot.ts`をMasterExportの形に全面書き換え(types/typeChart本体化、
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

species/moves/items/abilitiesの明示フィールド化、効果のPascalCase変換)、例データのID(move/item/ability)から
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

ハイフンを除去(codeIDPattern対応)。PR2(#385、pokedexフィクスチャ。ADR-0307)でpokedex-svc(MySQL必須)の
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

代わりにWeb例データから公開API応答を返す軽量フィクスチャ(`e2e/support/pokedexFixture.ts`+
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`pokedexFixtureServer.mjs`)を追加し、`/api/pokedex`を`vite.config.ts`でそちらへ転送。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

両PRとも`npm run e2e:online`実弾実行(3/3 pass)まで確認済み。critic指摘(playwright.container.config.tsの
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

testMatch漏れ、POKEDEX_PROXY_TARGETのvite proxyルーティングが無検査だった点等)は全て修正・検証済み。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**issue #308(オンラインでマスタ読み込み失敗時に再試行・オフライン切替ができずタブ一覧ごと消える)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

完了・main統合済み(2026-09-25。PR #390)**: 失敗時もタブ一覧を残し、マスタを使わない「素早さ」画面
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(ADR-0604 §5)は選んで使えるようにした。マスタを使う4画面(計算/逆算/タイプバランス/判定)が選ばれている
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

ときは、原因(Errorのmessage)・「再試行」・(オンラインのときだけ)「オフラインに切り替える」を出す。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

自動フォールバックはしない(ADR-0301 §4の既定方針を維持)。`app/routes.ts`の`SCREEN_ROUTES`に
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`usesMaster: boolean`を追加し、`app/screens.tsx`の`MASTERLESS_SCREEN_COMPONENTS`(`ScreenProps`から
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`master`・`engine`を除いた`MasterlessScreenProps`)経由で描画(設計判断はADR-0304追記6)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

spec-writer→implementer→critic(PASS、mutation testing 7件で全て検知)を経て、critic指摘のうち
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

マスタ不要画面へengineが静かに伝播するリスクは直接修正、残り2件(再試行中のフィードバック欠如・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

素早さタブでの通知非表示)はplan.mdに申し送り。`npx vitest run App`101/101・`npm test`1605/1605・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

typecheck/lint無回帰。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**issue #305(逆算で観測を説明できる候補が1つも無くても、その旨が出ず「近い候補」とSP範囲が並ぶだけ)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

完了・main統合済み(2026-09-25。PR #392)**: `result.exactCount === 0 && result.candidates.length > 0`
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

のときだけ、結果の先頭に`role="status"`の案内(「入力した観測を説明できる調整がありません」相当)を出し、
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

各候補のSP範囲に「参考」の印をテキストで添える(候補一覧自体は消さない)。各候補の%欄には常時「予測」の
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

ラベルを添える。一致判定はengineが返す`ReverseResult.exactCount`をそのまま使い、TS側での再判定は追加して
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

いない(ADR-0300 §8)。spec-writer→implementer→critic(PASS、mutation testing 5件で全て検知)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

critic指摘の軽微な1件(role="status"テストの頑健性)は直接修正、他は非ブロッキングのため見送り
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(候補0件時の文言欠如は別issue候補として観察のみ)。`npx vitest run ReverseScreen`94/94・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`npm test`1611/1611・typecheck/lint無回帰。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**issue #248(計算画面のオンライン計算にAbortSignalを渡していない)完了・main統合済み(2026-09-25。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

PR #396)**: `ReverseScreen.tsx`(issue #113)と全く同じ形で`CalcScreen.tsx`のcalcBulk用`useEffect`に
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`AbortController`を追加(effectごとに作り、cleanupで`abort()`)。WASM(オフライン)モードは`signal`を
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

無視するだけなので挙動不変。`web/src/test/fakeEngine.ts`の`PendingBulk`に`signal`フィールドを追加
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(`PendingReverse`と対称)。severity低・既存承認済みパターンの横展開のため、CLAUDE.md「軽微な作業は
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

メインのみでよい」に従いメインセッションで直接実装し、独立criticでレビュー(1回目はopusのセッション
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

利用枠上限で失敗、CLAUDE.mdのモデル割り当て方針に従いsonnetで再実施してPASS。mutation testing 2件・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

WASM無回帰を確認)。`npx vitest run CalcScreen.test`33/33・`npm test`1612/1612・typecheck/lint無回帰。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**APIレーンのPR #372(issue #271/#270のunsupported: UnsupportedMark[]追加。CalcResult/ReverseCandidate)が
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

main統合済み**: Web側の対応は不要(`apiEngine.ts`のmapCalcResult/mapReverseCandidateが明示的フィールド
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

写像のため増えたフィールドは自動的に無視される。issue #67の前方互換どおり)。印を画面に表示するかどうかは
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Webレーンの判断(DECISIONS.md 2026-09-25参照)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**issue #218(タブを切り替えると計算・逆算の入力状態が消える)完了・main統合済み(2026-09-25。PR #407)**:
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

ADR-0308に沿って実装。(1) lazy-mount-then-keep-alive(一度選ばれたタブだけmount、以後unmountしない。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

SpeedScreenのマウント時eager fetchを避けるため全画面の先読みはしない) (2) `role="tabpanel"`は1つのまま、
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

非選択画面はネイティブ`hidden`属性で隠す (3) 計算モード切り替え(マスタ入れ替え)ではタブの殻ごと
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

リセットしてよい(古いマスタの計算結果が残るより安全。ADR-0304 A-6の既存unmount挙動を利用) (4) リロードは
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

初期状態(storage不使用)。`visitedTabs`はマスタ取得口が変わるたびに作り直される`AppTabPanel`子コンポーネント
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

自身のstateに置き、モード切替時に隠れた素早さタブが余分にAPIを叩かない設計。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**critic 1回目FAIL(2点、実測込み)**: `masterEpoch`がデッドコードでADR決定3が未検証/マスタ再読み込みの
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

たびに隠れた素早さタブが再マウントしてspeed APIを二重に叩く実害(2件→4件を実測)。implementerが
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`masterEpoch`削除・`visitedTabs`の置き場所変更で対応、**critic 2回目PASS**(mutation testing 5種・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

実測プローブで両問題の解消を確認)。`npx vitest run`1625/1625・`make web-e2e`37/37・typecheck/lint無回帰。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**運用インシデント**: 実装1回目の際、worktree競合で実装者エージェントが`git update-ref`でブランチ参照を
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

強制移動する場面があった(データ損失は無し、コーディネーターが検証済み)。次回以降はスキル間で
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

worktreeを都度削除してから次段階へ進む運用に修正済み(メモリに記録)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**issue #271・#270(計算・逆算・bulkの結果に「未対応」の印を表示)完了・main統合済み(2026-09-25。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

PR #412)**: ADR-0123に沿って`unsupported: UnsupportedMark[]`(target/reason/id)を表示。全行(全候補)に
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

共通する印は結果一覧の先頭に1回、一部の行(候補)だけの印はその行だけ(`splitUnsupportedMarks`。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`web/src/domain/unsupportedLabels.ts`)。色は`--danger`でなく`--text-secondary`(エラーではなく目安の
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

ため警告色にしない)。spec-writer→implementer→**critic 1回目PASS**→**レビュー直後にiOSレーンが
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

DECISIONS.mdへクロスプラットフォームの文言・配置・色の決定を追加**したため追加のimplementerラウンドで
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

整合(iOSの`DisplayLabels.swift`と文言を1件ずつ突き合わせ完全一致)→**critic 2回目PASS**。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

JudgeScreen(JD5)は別contract(`attackerKoUnsupported`/`defenderKoUnsupported`)のため対象外、別タスクとして
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

plan.mdに記載。`npx vitest run`1674/1674・`make web-e2e`37/37・typecheck/lint無回帰。opusのセッション
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

利用枠上限で両criticともsonnetで代替実施(CLAUDE.mdのモデル割り当て方針どおり)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**P5-5a(構築ビルダーの骨格。一覧・新規作成・名前変更・削除)完了・main統合済み(2026-09-25。PR #417)**:
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

新規タブ「構築」(`/team`、末尾、`usesMaster: true`)。`web/src/team/teamClient.ts`(専用`.gen.ts`は作らず
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

ルート共有の`openapi.gen.ts`を使う。team/recordはルート契約に同居しgateway経由のため)。削除確認は
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`window.confirm`を使わず行内の2段階ボタン。書き込み後は`list()`を呼び直さず応答の`Team`で手元を書き換える。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**メンバー編集(種族・技・持ち物・特性・性格・SP・テラスタイプ)は次のPR(P5-5b)で別途**(ADR-0309「却下した案」)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

spec-writer→implementer→**critic 1回目PASS**(重要指摘2件: 名前変更の送信前検査漏れ・list()応答と
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

create/update/removeのレースコンディションで作成直後の構築が消えて見える不具合)→implementer(修正)→
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**critic 2回目PASS**。`npx vitest run`1745/1745・`make web-e2e`37/37・typecheck/lint無回帰。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

判定レーンがShowdown形式インポート/エクスポートをブランチ`feat/web-team-showdown-format`(`web/src/team/`
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

配下)で並行して進めている(分担合意済み。member editorとファイルが重ならないよう次のPR着手前に確認)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Next: P5-5b(構築ビルダーのメンバー編集。種族検索・技/持ち物/特性選択・SP直接入力グリッド0〜32・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

テラスタイプ。特性セレクト〈ADR-0311〉と `selectableAbilities` を再利用できる)に着手する。判定レーンのShowdown形式ブランチとの統合順を確認してから進める。その後
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

P5-5c(よく計算する相手の表示。`GET /api/record/frequent-opponents`、design.mdに既にチップのモックアップ
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

枠あり)・P5-5d(ADR-0209 §8の文言で「この端末のデータを削除」UI、record/team両方のdevice-data削除を呼ぶ)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

P5-5完了時はdocs/verify-m1.md(またはM2用手順書)にM2動作確認手順を追加し、make deploy-latestの対象に
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

record・team・TiDB・NATSが要るかAPIレーンと確認すること(オーケストレーターの依頼)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**2026-10-01〜02 に消化済み(1 issue = 1 PR)**: #218(PR #407 で解決済み・クローズ)・#219(PR #420・ADR-0310。nginx にセキュリティヘッダ)・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

#272 Web 分(PR #430・ADR-0311。特性セレクト)・#274 Web 分(PR #433・ADR-0312。計算画面の「詳細」)・#210(PR #451・ADR-0313。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**既定の計算モードをオンラインに変更**〈ユーザー決定 2026-10-01〉+ IndexedDB キャッシュのオフライン)・#332 Web 分(PR #454。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

devDependencies 7 件と Web の Node 26.10.0)・#226(D29 でクローズ済み。Web 分に古い記述なし)・#271/#270 の Web 分は PR #412 で
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

完了済み(issue にコメント)。**待ち**: #211(公開 API の Item/Ability に effect が必要。DECISIONS.md 2026-10-01 に API レーンへの提案を
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

記録。入ったら Web が `effects:true` へ追従。オフラインのキャッシュは effect を持たないので「持ち物の候補も比較」は#211まで無効)・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

#332 の残り(`services/pokedex/Dockerfile` の Node〈データ〉・golang タグ統一・定期検出〈運用〉)・#274 の防御側ランク/状態異常
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(API の `defenderOverride.ranks/status` が未実装)・#271/#270 の判定画面表示(判定レーン)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**PR のマージは ADR-0800 §2 のガードで人間の端末実行が必須**(Claude は PR 作成と CI 確認まで。マージはユーザーが自分の端末で行う)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**新規キュー項目**: issue #328(非公開・私的利用・LICENSEなしで決定。design.mdに追記のうえ
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

既存画面の邪魔にならない位置に出典・非公式である旨を表示。iOSは既にPR #415でmain統合済み〈AboutView.swift。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

非公式注記+データ出典4件〉。Webは同じ文言〈DECISIONS.md参照〉でフッターリンク→情報ページの形にする)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

issue #284(balance/speed/judgeがgatewayの後ろに統一される。APIレーンの転送実装が出たら`/api/balance`・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`/api/speed`・`/api/judge`の接続先を切り替える)。両方ともキューの末尾。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(3) P4-20: issue #148(アクセス境界・認証方針)。Web 側は既にコード上で条件を満たしていることを確認済み
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(apiBaseUrl の既定値は同一オリジン、CORSはgateway側の設定)。実際のtailnet名が決まってから運用レーンより
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

連絡が来る想定。(5) 人間へのお願い:
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

docs/verify-m1.md §4 を Safari で確認(P4-5。issue #333のsafeキーワード確認も合わせて)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。


Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

## iOS
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Lane: iOS(`ios/`。M3 の Phase 6。どの AI が進めてもよい)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Active: なし(P6-19〈PR #432〉・P6-7 完了。残る Next は他レーン待ちのみ)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Branch: feat/ios-p6(作業ディレクトリ ~/MyDamageCalcurater-ios)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status: **M3(iPhone で使える)は完了**。P6-1(ADR-0500)・P6-2a 計算画面・契約追従・P6-2b 逆算画面・P6-2c 構築ビルダー
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(一覧・編集・ニックネーム)・P6-2d(構築から個体を呼び出す配線)・P6-3・P6-4(手順書 `docs/runbooks/ios-device-install.md`)・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

生成の internal タグ除外・DOC-ios は main に統合済み(PR #31・#53・#91・#119・#122)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

続けて Codex レビュー issue のうち iOS 主担当分を修正・main 統合済み: #100(種族変更後の特性ID残留)・#101(負のSPの
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

検証漏れ、PR #131)、#68(検索上限200件。種族・技ピッカーを `Menu` 一括取得から `.searchable()` 検索UIへ変更。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Web の ADR-0304 と同じ方針。PR #136)、#113(Web/iOS/API共同主担当。入力操作ごとの計算Taskを最新の1つだけ保持し
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

新入力・画面破棄で先行Taskをcancel、逆算の観測文字入力に200msのtrailing debounce。`CancellationError`は画面
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

エラーにしない。PR #166。1周目critic FAIL→2周目PASS)、#99(ライトテーマの danger コントラスト不足。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`ColorToken.danger`のライト値を`#E5484D`→`#CD1D23`に変更。Web PR #164 と同じ値。PR #170。issue #99 は
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Web・iOS 両方完了でクローズ済み)、P6-6(issue #110の iOS側追従。ADR-0501「issue #110」章。`RequestLimits`/
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`RequestLimitLabels` を新設し、観測16件・持ち物候補/比較64件(null込み。選べるのは63件)の上限を実装。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

観測は追加ボタンを無効化、持ち物候補・比較トグルは上限到達中のON操作だけ拒否(OFFは常時可。Webの
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

決定的切り捨てとはあえて変えた判断はADR参照)。critic指摘でguardの位置(`beginInput()`より前)を固定する
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

回帰テストを追補。引き継ぎ検証で契約との同期検査 `ios/scripts/check-request-limits.sh` と観測上限の XCUITest を
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

追加。**PR #186 で main 統合済み**)。`make ios-test`(gen-check・件数上限の同期検査・XCTest 340件〈xcresult 集計353件〉・XCUITest 17件・Info.plist 検査)が緑。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

issue #68 の残り(一度も検索結果に出ていない選択中の技IDを名前解決できない)は P6-9 で `getMove` による個別解決を
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

実装して解消(ADR-0501「issue #68 の残り」。持ち物の先頭ページが上限に達したら黙って切り捨てず案内を出す。critic
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

1周目 FAIL〈逆算の古いエラー消去条件の退行〉→修正→2周目 PASS。**PR #199 で main 統合済み、issue #68 クローズ済み**)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

issue #110 は API・データ・Web・iOS すべて完了したためクローズ済み(2026-09-24)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

P6-10(構築編集の load の技解決を `getMovesByIds` のまとめ取り1回へ。ADR-0501「getMovesByIds による構築編集の技の一括解決」。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

critic 1周目 FAIL〈分割境界のテスト不足〉→テスト追加→2周目 PASS)完了。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

P6-11(issue #334。攻撃側プリセットの表示名を技の分類に追従。PR #348、issue クローズ済み)・P6-12(issue #71 の iOS 追従。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`engine/presets/attacker.json` との契約テスト、並び 無振り→特化→振り、既定を無振りに変更。ADR-0501「P6-12」)完了。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

P6-13(issue #274。計算画面の「詳細」: 急所・やけど・天候・フィールド・防御側の壁・攻撃側のランク・特性。PR #377。語は
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

DECISIONS.md に記録し Web が合わせる)・P6-14(最大の文字サイズで計算画面が横にはみ出す既存の不具合。結果行の `.fixedSize()` が原因)完了。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

P6-15(アクセシビリティ域でプリセットのピルを縦積み等)・P6-16(issue #250。http は非修飾ホスト名と .local のみ受理、
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

NSAllowsLocalNetworking。PR #394/#395)・PR #372 追従の再生成(PR #398)・P6-17(未対応の印〈unsupported〉の表示。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

文言は DECISIONS.md に記録し Web が合わせる)完了。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Next: (1) 第三者データの出典・非公式の表示(#328 のユーザー決定。文言は iOS が DECISIONS.md に既定案を書き Web が合わせる。Web と合意済み)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(2) #272: API レーンが特性の契約(abilityId・unknownAbilityId・defenderOverride.abilityId)を出したら追従。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(3) API レーンが UnsupportedMark の reason/target を string に緩めたら、未知の値の扱いを追加(P5-4 の後に検討と連絡あり)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(4) API レーンが `BulkCalcRequest.defenderOverride`(防御側のランク・特性・状態異常。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

DECISIONS.md 2026-09-25 で採用、M2 の後に実装予定)を入れたら、iOS の「詳細」に防御側の入力を追加。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(5) P6-7(issue #103・ADR-0209 §8の削除UI)は完了(2026-10-01)。(1)〜(3) も完了済み(P6-18・P6-19・ADR-0215)。将来の候補:
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

engine の Champions マスタが pokedex-svc 経由になったら iOS のモック/実マスタの差し替え動作を再確認、Web の
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

record/team-svc(M2)が進んだら iOS の構築を端末内保存から API 保存へ移行するかを検討。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。


Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

## Type Balance Checker
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Lane: タイプバランス(どの AI が進めてもよい。COORDINATION.md)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Active: Claude Code(2026-10-02 再開。#263・#292・#298・#260・#276・#237 を実装。残りは下記 Next)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Branch: 次は main から feat/tb-<名前> を切る(作業ディレクトリ ~/MyDamageCalcurater-tb。git worktree)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status: 設計書(docs/type-balance-design.md)の TB0〜TB6 はすべて main に統合済み(TB6: 技範囲チェッカー、PR #65)。P2-3b(特性の無効・吸収)の実データ確認を完了(2026-09-23): データレーンが再生成した export(348 pokemon・moves・216 abilities)で `make balance-k3d-deploy-readmodel && make balance-smoke-readmodel` を実行し、`POST .../team-balance/analyze` でチリーン(levitate)への ground 攻撃が `{"category":"immune","effect":"immune","multiplier":"0","source":"ability"}` になること、`POST .../move-range/analyze`(thunderbolt)の `walledByAbility` にエモンガ(motordrive)が正しく含まれることを実データで確認済み。メガフォームの nameJa が英語表記のままの件はデータレーンへ確認候補として残る(ブロッカーではない)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Codexレビュー issue #105(Argo CD導入のハッシュ・digest固定)対応完了(2026-09-23。ADR-0405。PR #140)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**別セッションからの依頼(ユーザー承認済み)で M4 P7-1(kube-prometheus-stack / Loki、各サービスのメトリクス)に着手・完了**
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(2026-09-24〜25。ADR-0406。PR #201・#336): 6サービス(gateway・pokedex・calc・balance・speed・judge)に `GET /metrics`
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(Prometheus text format、method/pathはカーディナリティ対策で正規化)、`scripts/observability-bootstrap.sh` で
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

kube-prometheus-stack・Loki(SingleBinary)・Alloy を版・SHA-256固定で導入。実クラスタ(k3d-pokecalc)で実行し、
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

全Pod起動・PVC Bound・6 ServiceMonitor適用・balance/calc/gatewayのscrapeがup・GrafanaのLokiデータソースで
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

実ログ取得まで確認済み(judge/pokedex/speedは`/metrics`追加前の古いイメージのため404。各レーン再デプロイで解消見込み)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): 2026-10-01〜02、タイプバランス担当 issue を処理。#263・#292(ADR-0408。AppProject pokecalc 限定・GitOps スクリプトを scripts/gitops/ に共通化・レジストリ PVC 化。PR #422)、#298(ADR-0409。recommendations のアロケーション削減 3.1→0.53MB/op・同時実行上限〈既定4。超過は 503 overloaded〉・GOMEMLIMIT。PR #426)、#260(type-balance-design.md を現在の設計に書き換え、旧版を ADR-0410 へ。PR #429)、#276(ADR-0411。API専用画面は計算モードに関係なくオンラインのマスタ・エラー日本語化。PR #448)、#237(ADR-0412。gitops overlay の pokedex export initContainer。PR #452。実クラスタ未適用)。P7-2(SLO。ADR-0407)は実装済み・実クラスタ未確認。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Next: #237 の残り(人間確認が必要): ①`make pokedex-registry-push`(POKEDEX_REGISTRY_PUSH_CONFIRM=1)で pokedex image を balance-registry へ push し実 digest を overlay へ PR、②`deploy/k8s/base/networkpolicy/allow-mysql-ingress.yaml` に balance・speed を足す別 PR(ADR-0132 の許可表も)、③Argo CD sync と `make balance-smoke-readmodel`。①は データレーンへ依頼可。#259 は ADR-0118(DECISIONS 2026-09-25)により必須でなくなったので、タイプバランス側で閉じてよいか判断する(embedded と golden の一致テストは既存)。#236(balance/speed の端末ID検証・エラーコード不一致)と #284(balance・speed・judge を gateway の後ろへ)は別途。判定画面の補助行に英語 message が残る点は別 issue 候補(#276 の ADR-0411)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

メモ: `make balance-k3d-deploy`(local overlay)で上書きすると Application は OutOfSync になる(manual sync なので戻らない)。GitOps に戻すときは Argo CD で Sync
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。


Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

## Speed
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Lane: 素早さ(素早さ比較サービス。`services/speed/`・`web/src/speed/`。どの AI が進めてもよい)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Active: なし(SP0〜SP5 すべて完了。次の要望待ち)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Branch: 次は main から feat/speed-<名前> を切る(作業ディレクトリ ~/MyDamageCalcurater-speed。SP5 は feat/speed-sp5 → PR #97 で main に統合)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status: SP0〜SP5 すべて完了・main に統合(PR #32・#36・#52・#83・#86・#93・#97)。SP4 の実データ確認はユーザーが2026-09-24 に実施:
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`mysql` Service はクラスタ内部の DNS 名で Mac からは解決できないため、`kubectl -n pokecalc port-forward svc/mysql 3306:3306` を張り、
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

DSN のホストを `127.0.0.1` に付け替えて `make pokedex-export`(348 pokemon)→ `make speed-k3d-deploy-readmodel` →
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`make speed-smoke-readmodel` を実行(初回はロールアウト直後で 504、再実行で `speed readmodel smoke: pokemon=0003-000 list=200 table=200`)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

SP5 の実際の Argo CD への適用(`speed-argocd-app`・`speed-registry-push`・sync)は未実施のまま(ADR-0605 §4。共有クラスタへの変更のため
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

人間の確認のもとで、必要になったときに)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): DOC-arch(docs/architecture.mdを全レーンの現行構成に合わせて更新。ユーザー依頼)を素早さレーンが担当・完了(PR #168・#194)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

judge-svc を全体図・コンポーネント表に追加(先に判定レーンの抜けを見つけて#168で対応)、record-svc・team-svc(M2。計画中。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

services/record・services/teamはまだ.gitkeepのみ)をTiDB・NATS JetStreamとあわせて追加、Kustomize overlay(local/cloud)の
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

節を新設。gateway・pokedex・calc・balance・speed・judge・データの流れ・WASMはコードを確認し既に現行と一致(変更なし)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): 2026-09-25、全体レビュー issue の割り当てミス(#71・#74・#76・#77 は素早さ担当ではなかった)を指摘し、
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

データレーンへ差し戻し済み。素早さが実際に担当に入る open issue を洗い出し: #263(タイプバランス主・素早さ・運用。Argo CD
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Applicationのproject: default・初期admin Secret残存・GitOpsスクリプト5本の重複)・#237(タイプバランス・素早さ。needs-decision。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

GitOps overlayがread modelを持たずbalance/speedの業務APIが全て503。既知の制約はADR-0605 §2aに記載済み)・#236(タイプバランス・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

素早さ・判定・API連携。端末ID/セッションIDの検証とエラーコードがgatewayと3サービスで不一致)・#108(既知・データレーン主担当)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

タイプバランスレーンと分担を確認済み: #263はタイプバランスレーンが主担当(speed側の差分は連絡が来たら対応)、#237は既定案
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(initContainerでpokedex exportを起動時に実行)でユーザー確認中(タイプバランスレーンが担当)、#236は共通パッケージの置き場所を
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

タイプバランスレーンがAPIレーンと相談中。#237の実装には「pokedex-svcのserverイメージをbalance-registryへdigest固定でpush」という
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

データレーンへの新しい依頼が発生することをタイプバランスレーンに共有済み。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): 2026-09-25、#236のspeed側を完了(ADR-0606。PR作成中)。gatewayのcheckAPIHeaders/isCanonicalUUIDを
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`services/speed/internal/httpapi/requestctx.go`に複製(httpmetricsと同じ前例。共通パッケージ新設なし、APIレーン合意済み)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

X-Device-Id/X-Session-Idの検証を正準形UUIDに強化し、エラーcodeを`invalid_request`から`missing_header`/`invalid_header`
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

へ分離(契約の破壊的変更)。openapi.yaml 0.4.0・web/src/speed/speed.gen.tsを再生成・critic PASS。balance・judgeは各自対応。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): 2026-10-02 issue 307(素早さ画面の範囲外入力)を解消(critic PASS〈1回目〉)。カスタムの SP(0〜32)・ランク(-6〜+6)と
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

実数値(1以上の整数)の範囲外は、送信前に日本語の role=alert・aria-invalid で止めて API を呼ばない。実数値の上限は契約に無いので
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

画面では検査せず、API の 400 を `errorByCode` で日本語にする。空欄はカスタム SP・ランクは 0 とみなし、実数値は未入力で呼ばない。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

既知の積み残し: 数値欄で「-」を打つと値が空になり 0 に戻るため負数をキー入力しづらい(従来からの挙動。直すなら欄の state を文字列で持つ)・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

検証規則が JudgeScreen の validationMessage と二重管理(将来の共通化候補)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Next: #263・#237 はタイプバランスレーン/APIレーンからの連絡待ち(連絡が来たら speed 側の overlay・scripts を対応)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

#105(Argo CD導入・digest固定の共有スクリプト化)は完了・追加対応不要。#108は データレーンからの連絡待ち(今は着手不要)。他は
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

balance-registry → pokecalc-registry への改名提案(タイプバランスレーンへ既定案で提示済み。DECISIONS.md 2026-09-23)かユーザーからの
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

新規要望待ち。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。


Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

## Judge
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Lane: 判定(素早さ×ダメージ連動。`services/judge/`・`web/src/judge/`。どの AI が進めてもよい)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Active: なし(**judge-design.md §3 が定めた JD0〜JD5 すべて完了・main 統合済み**。次のユーザー要望待ち)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Branch: 次は main から feat/judge-<名前> を切る(作業ディレクトリ ~/MyDamageCalcurater-judge)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status: JD0(基盤。PR #92)・JD1(判定API本体。PR #118)・JD2(場の効果。PR #127)・JD3(複数の相手候補。PR #143)・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

JD4(返り討ち判定。PR #169)・JD5(Web の画面。PR #182。ADR-0705)まで全段階が完了。`POST /api/judge/v1/outspeed-and-ko`
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

は自分1体対相手1〜6体の素早さ判定・場の効果(トリックルーム・追い風)・返り討ち判定まで対応し、`web/src/judge/`
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(`/judge` タブ)から呼べる。技はID自由入力(ADR-0304 §3の技一覧APIの欠落を踏襲)、相手側の追い風は全候補共通の
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

1チェックボックス(ADR-0703 §5)、送信ボタンでのみ呼ぶ(1回で上流最大27回)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): 2026-10-01 issue #258 judge の GitOps(gitops overlay・Argo CD Application・image 公開スクリプト。ADR-0709)を実装。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

実クラスタへの適用(`judge-argocd-app`・registry push・sync)は人間確認待ちで未実施。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Next: 新規要望待ち。軽微な積み残しは解消済み(2026-09-25。`attacker`単数の`Individual`にも`defenders`候補と
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

同じ大文字小文字厳密なキー検査〈`individualWireKeys`〉を適用。PR #342 main 統合済み)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

issue #234(moveId/natureId の形式検証。ADR-0706)も解消(2026-09-25。critic 2ラウンド。PR #365 main 統合済み):
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

名前付きスキーマ `MoveId`/`NatureId`(pattern `^[a-z0-9]+(-[a-z0-9]+)*$`・maxLength 64)を契約に追加し、
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`outspeed.go` の3箇所(attacker moveId・natureId共有・候補moveId)で上流呼び出し前に検査、
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`pokedex.go` は `url.PathEscape` で二重の守り。`web/src/judge/judge.gen.ts` も手動再生成(ADR-0705 §2)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

issue #213(重大度 high。リクエスト全体の期限。ADR-0707)も解消(2026-09-25。critic PASS〈1回目〉。PR #370 main 統合済み):
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`JUDGE_REQUEST_TIMEOUT`(既定12秒。`writeTimeout`=15秒未満を起動時検証)を新設し、`outspeedAndKo` の
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

先頭で ctx を1回だけ `context.WithTimeout` でラップして以降の上流呼び出しに使い回す(呼び出し順序・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

逐次打ち切り規約〈ADR-0703 §3〉は無変更)。`internal/client` は無変更(`http.NewRequestWithContext` の
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

既存の context 統合だけで「進行中呼び出しの中断」「未着手呼び出しの即時失敗」の両方が成立)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

上流が遅くても期限内に503 JSONを返すようになり、クライアントが空応答(HTTP 000)を受け取ることが無くなった。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

issue #329(重大度 low。SP合計67の境界値テスト欠落)も解消(2026-09-25。テストのみ・実装無変更。PR #371 main 統合済み):
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`validateSP` の合計超過検査の既存テストが境界〈67〉から遠い(96)ため、
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`> engine.MaxSPTotal` を `+1` する退行を検出できなかった。境界値(合計66は受け付け・67は拒否)の
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

テストを `internal/judge`・`internal/httpapi` 両方に追加し、mutation test で実際に検出できることを確認。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

issue #257(重大度 low。smoke.sh が healthz のみ)も解消(2026-09-25。テスト用スクリプトのみ。PR で main へ):
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

gateway smoke の ID取得部分を流用し `POST /api/judge/v1/outspeed-and-ko` の 200(hits含む)・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

ヘッダなし400・未知speciesKey 422・7候補400 を実クラスタ(k3d-pokecalc、実データ)で確認済み。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`Makefile` に `API_URL` を追加、README の古い「JD0完了」表記も修正。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

iOS版JD5は要望が出たら判断(ADR-0705 却下案)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。


Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

## Ops
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Lane: 運用(deploy・scripts・AIエージェントの権限設定。専任セッションなし。空席時は手が空いたレーンが調整役の
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

割り当てで代行できる。COORDINATION.md)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Active: 素早さレーン(調整役「damage calculation bug resolution」からの割り当て。**ユーザーの直接指示ではない**。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

運用の担当欄は変わらず「運用」のまま、今回だけ空席を代行。DECISIONS.md 参照)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Branch: 次は main から fix/ops-issue273-239 を切る
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status: 2026-09-25、issue #273・#239(AIエージェントの権限設定が、CLAUDE.mdの「人間の確認が必要」な操作(クラスタ削除・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

DBのデータ削除・main反映・秘密の読み取り)を止められない)に着手。既定案(ADR-0800):
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(1) `.claude/settings.json` の `permissions.allow` から `Bash(make *)`・`Bash(kubectl *)`・`Bash(k3d *)`・`Bash(docker *)`・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`Bash(git push *)`・`Bash(gh pr merge *)` 等の広い許可を外し、読み取り・非破壊のサブコマンド単位に絞る。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(2) 主防御として `hooks.PreToolUse`(Bash)から `scripts/ai-guard/bash-guard.sh` を呼び、コマンド文字列を検査して
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

該当すれば exit code 2 で無条件 block(`permissions.allow` があっても上書きされない。公式ドキュメントで確認済み)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

対象: `make down`・`make import`・`make import-k8s`・`make migrate-down*`・`kubectl` での ns/namespace/pvc/pv/
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

statefulset/secret の delete・`kubectl get secret`・`.env`/SSH鍵ディレクトリを含むコマンド・`k3d cluster delete/rm`・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`git push` の main 反映(`main`・`:main`・`HEAD:refs/heads/main` 等、書き方によらず)・force push 系・`gh pr merge`。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(3) `.codex/config.toml` にも同じスクリプトを `[[hooks.PreToolUse]]` から呼ぶ設定を追加し、`approval_policy`・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`sandbox_mode` を明示する(Codexのpermissionは`deny`のみ対応、`ask`は無い)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**影響**: 上記に該当する操作は、どのレーンのセッションでも(Claude Code・Codex とも)AIエージェントからは実行できなく
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

なり、人間が自分の端末で実行する必要がある。`make test`・`make lint`・`make build`・`go test`・`kubectl get/describe/logs`・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

featureブランチへの `git push`・`gh pr create` は従来どおり確認なしで通る。各レーンの runbook に `gh pr merge` を
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

AIが実行する手順があれば、そこだけ「人間が実行」に変わる(気づき次第、該当レーンへ個別連絡)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

**このPRは実装後もこのセッションはマージしない**(設定ファイルの変更で全レーンに影響するため、ユーザーの確認・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

承認を経てからのマージとする。ADR-0800 §5)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Status(追記): 2026-09-25、critic(Opus)1回目 FAIL。テスト268件は green だったが、テストに無い普通の書き方
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(`git push origin main 2>&1`・`bash -c "make test && make down"`・`kubectl delete statefulsets mysql`(複数形)・
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

`$HOME/.ssh` 等)でガードを迂回できる穴が複数見つかった。加えて **Codex 側の実効性は未検証**と判明: codex-cli の
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

バイナリ文字列調査で、プロジェクトローカルの hooks はディレクトリの信頼+フックごとの人間確認(TUI)を経るまで
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

読み込まれない可能性があり、`$(git rev-parse ...)` のシェル展開や `tool_name` が `"Bash"` になるかも実機未確認
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(ADR-0800 §3 に追記)。`services/pokedex/db/layout_test.go` の `TestNoAutomaticDown` が `scripts/ai-guard/` 配下の
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

検知用文字列 `migrate-down` に誤反応する既知の1件は、調整役(damage calculation bug resolution)経由でデータレーン
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

(4f)へ最小除外を依頼済み(ai-guardの実装自体の欠陥ではない)。critic指摘の修正をimplementerへ差し戻し中(2回目)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

Next: critic 2回目レビュー → PASSしたらPRを作成してユーザーに提示する(マージは求めない。データレーンの
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

TestNoAutomaticDown修正がmainに入るまでこのPRはマージ不可であることをPR本文に明記する。Codex側は「未検証」と
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

明記し、実地確認〈Codexを起動してフックを信頼・確認する手順〉を宿題として残す)。
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。


Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

## Shared Interfaces
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

- Pokemon ID: pokedex-svc の `{図鑑番号4桁}-{フォルム3桁}` 形式に準拠
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

- Type: 18タイプの英語小文字ID(fire, water, ...)。表示名・色は docs/design.md のトークンに準拠
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

- Type multiplier: 分数ではなく整数表現(claude-review.md 参照)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

- サービス境界: damage-calc と balance は兄弟。相互の実行時 API へ直接依存しない
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

- 共通マスタ: 正本は1つ。`feat/claude-p1-engine` の ADR-0002 にあるコミット済みスナップショット案を候補とし、人間の確認待ち
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

- balance の type chart: P1-13 の `testdata/golden/typechart.json` をバイト複製して同梱(ADR-0015)。正式マスタ確定後に provider を差し替える
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

- 共有状態の正本: `origin/main` の `docs/ai-shared/`。未マージの続きは各レーン欄の `Branch` の最新コミット(COORDINATION.md)
Status(追記): 2026-10-02 調整機能(docs/plan.md「AJ: 調整」。ADR-0150・0250・0251・0319)の AJ0〜AJ6 を実装(engine・調整 API・技の逆引き・Web の「調整」タブ)。PR #476(AJ0〜AJ3)→ #491(AJ4〜AJ6)の順にマージ。残りは AJ7(iOS。Web で触って確認してから。iOS の生成物は再生成済み)。

- 開発の正本: private の `origin` の `main`(PR でのみ更新。Argo CD が参照)。作業ディレクトリはレーンごとに `~/MyDamageCalcurater`(ダメージ計算)と `~/MyDamageCalcurater-tb`(タイプバランス)
