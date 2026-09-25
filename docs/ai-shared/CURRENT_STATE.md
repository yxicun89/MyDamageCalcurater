# Current State

## Damage Calculator
Lane: データ(engine・マスタ・pokedex。どの AI が進めてもよい。COORDINATION.md)
Active: Claude Code
Branch: feat/claude-p1-engine(作業ディレクトリ ~/MyDamageCalcurater)
Status: Phase 1・P2-1・P1-10・Phase R・P1-13・P1-11・P1-12・P2-1b・P2-1c・P2-2a・P2-2b・P2-2c・P2-2d・P2-3(pokedex-svc。内部 API・公開 API・natures・balance/speed 向け export。ADR-0105)は完了(critic レビュー済み)
Status(追記): P2-3b(無効・吸収の特性)も完了・main 統合済み(ADR-0106)。calc・gateway の pokedex-svc 接続(API レーンの依頼)も PR #87 で解決済み(api-smoke で master=pokedex 確認済み)。
Status(追記): P5-6(技の追加効果によるランク変化。ADR-0107)完了・critic PASS(1往復)・**main 統合済み(PR #132)**。engine は乱数を持たず「発動した場合の値」だけを返す。ゴールデン不変。`move_effects` 別表・`MasterMove.effect`(内部API)まで。公開APIへの露出(判定レーンが技IDからランク変化を引く経路)は判定レーンの要件確定後に別途対応。
Status(追記): issue #110 のデータレーン担当分(ADR-0108)完了・critic PASS(1往復)・**main 統合済み(PR #138)**。`engine.CalcBulk`/`CalcReverse` と `engine/wasmapi` に ADR-0208 §1 と同じ件数・範囲の上限を追加し、wasmapi は DTO 変換より前に検査して HTTP との parity を確保。issue #110 は Web・iOS レーンの追従が残っている限りクローズしない。
Status(追記): issue #106(排他制御)完了・critic PASS・**main 統合済み(PR #155)**。`tools/importer/cronjob.sh` に `flock`(非ブロッキング)を追加し、手動Job(`make import-k8s`)と定期CronJobの同時実行を防ぐ(ADR-0109)。Docker(Linux)と実クラスタ(k3d)の両方で実際の排他動作を確認済み(2026-09-23。2つの手動Job同時作成→片方がロック競合で即exit 1→backoffLimitで再試行して成功)。
Status(追記): 2026-09-23、全レーンの main 統合済みの変更をまとめてk3dに再デプロイし、実データ(種族349・技515・move_effects 59件)で計算・一括計算・逆算・タイプバランス・素早さ・判定(JD3複数候補)まで実HTTPで動作確認済み。すべてgreen。既知の制約: 技を個別IDで引く公開APIが無く(`GET /api/pokedex/moves/{id}` は404)、Webのオンライン技選択・持ち物候補比較・判定JD4(相手の技を含めた返り討ち判定)がブロックされたまま。
Status(追記): 2026-09-24、getMove(P3-7。APIレーンが`services/pokedex/`へ越境実装)をレビュー。既存の設計判断(命名・エラー変換・テストの流儀)と食い違いなく、修正不要と判断(DECISIONS.md参照)。判定レーンはJD4に着手可能。上記の「技を個別IDで引く公開APIが無い」制約はこれで解消(バッチ解決はまだ無いのでWebのオンライン技選択は引き続きブロック)。
Status(追記): issue #104(DB資格情報の最小権限分離)完了・critic PASS(1往復)・**main 統合済み(PR #176)**。`pokedex_reader`/`pokedex_importer`/`pokedex_migrator`の3ロールに分離(ADR-0110)。実クラスタで`SHOW GRANTS`により権限が過不足なく一致することを確認済み。既存クラスタからの無停止移行も実地確認済み。
Status(追記): issue #109(HTTPタイムアウト・graceful shutdown)完了・critic PASS(1往復。指摘なし)・**main 統合済み(PR #178)**。`newHTTPServer`/`serve`/`runServe`の3層分離(ADR-0111。services/balanceと同じ値)。`terminationGracePeriodSeconds: 30`を追加。実クラスタで再デプロイ・確認済み。
Status(追記): issue #112(DB接続プール上限)完了・critic PASS(1往復。軽微指摘1件反映)。4環境変数を`services/pokedex/db.OpenPool`経由で適用(ADR-0112)。実クラスタで再デプロイ・確認済み。**main 統合済み(PR #180)**。**これでデータレーン主担当のCodexレビューissue(#104・#106・#109・#112)はすべて完了・main統合済み**。
Status(追記): issue #102(importer の中断キャッシュ自己回復。ADR-0113)を修正。`showdown-cache.mjs` へ切り出し、一時名+検証+rename。テスト7件を `make test-tools` に接続。
Next: 他レーンからの依頼待ち。人間の確認待ち(plan.md ブロッカー): 観測%の丸め方(整数%表示は確認済み)、公開のタイミング(LICENSE・クリーンコピー)

## API
Lane: API(calc-svc・gateway・契約テスト。`api/openapi.yaml` の持ち主。どの AI が進めてもよい)
Active: Claude Code(M2完遂の依頼〈2026-09-25〉でP5-2〜P5-4を継続中)
Branch: feat/api-p5-2-calc-events(作業ディレクトリ ~/MyDamageCalcurater-api。P5-2・P5-3を含む。PR #372)
Status: Phase 3・issue #110(ADR-0208。PR #130)・issue #103の設計(M2保存データの保持・削除・端末ID境界。ADR-0209。critic PASS。PR #150)は main に統合済み
Status(追記): issue #148のAPIレーン担当分(ADR-0210。私設サービスの境界)完了・critic PASS・**main 統合済み(PR #157)**。`deploy/k8s/overlays/cloud` から gateway の Ingress を削除 patch で除去し、public Ingress/LoadBalancer/NodePort/externalIPs/hostNetwork/hostPort が無いことを構造検査+`kubectl kustomize`実描画検査の2層で固定。端末ID/CORSを認証・到達制御として扱わない回帰テストも追加。
Status(追記): P3-7 `GET /api/pokedex/moves/{key}`(getMove)を実装(判定レーン JD4 の依頼。ADR-0105 §3 追記)。契約・`services/pokedex/`(データレーンの範囲。越境理由と触ったファイル一覧は DECISIONS.md)まで一括実装。critic PASS(3往復)・**main 統合済み(PR #161)**。判定レーンは JD4 に着手し main 統合済み(PR #169)。
Status(追記): issue #69(検索の並びがOpenAPI契約と一致しない)・issue #73(OpenAPIとengineの防御プリセット集合の同期検査)を修正・**main 統合済み(PR #167・#172)**。いずれも契約・テストの整合修正で、SQL・engine・ADR は無変更(既存の設計は元々正しかった)。両issueともclose済み。
Status(追記): issue #113(入力変更時の古い計算要求を抑止・キャンセル)のAPIレーン連携分(「クライアントのcancel伝播」)を修正。gatewayの`ReverseProxy.ErrorHandler`がクライアントの要求中断(`context.Canceled`)を上流障害と区別せず「上流に到達できない」WARN・503 `upstream_unavailable`を返していたのを、クライアント起因のときは何もしない(応答を書かない)よう修正(ADR-0202 §5 追記・AC-G10)。**限界**: gateway→calc-svcへのcontextキャンセル伝播自体は効くが、calc-svc・engineはcontextを見ないため(engineを純粋に保つ絶対ルール2)、issue本文の「calc-svc CPU消費も止める」は未達成のまま(中断された逆算は完走する。ADR-0208の上限で最悪計算量は有界)。Web欄の「APIレーンの連携分が残っているか未確認」はこれで解消(Web欄の更新はWebレーンに委ねる)。issue #113はWeb・iOS・APIすべてのレーン分が完了としてクローズ可能と判断(詳細はDECISIONS.md)
Status(追記): P4-17(ADR-0304 §3。技のID解決の欠落)を解消・**main 統合済み(PR #196)**。`GET /api/pokedex/moves/batch`(`getMovesByIds`)を新設。ADR-0304が当初推していた案A(`learnset`を`Move[]`に変える)は不採用: iOS(M3)が`learnset: string[]`前提の出荷済み機能を持つため、型変更より新エンドポイント新設の方が契約変更として小さいと判断。critic PASS(3往復)。
Status(追記): `learnset`が64件を超える場合の実データを確認(クラスタ復旧後)。**349種族中151種族(43%)が64件超・最大106件**で、まれな例外ではなく日常的なケース。Web側の分割呼び出しは主経路として実装が必要と訂正・連絡済み(DECISIONS.md 2026-09-24)。
Status(追記): M2 P5-1(record-svc/team-svc用TiDB)着手。ADR-0211でバージョン固定(TiDB v8.5.8・TiDB Operator v1.6.6)・
ローカル/k3dプロビジョニング・DB/ユーザー分離・migrationツール共通化・スキーマ・保持日数の環境変数契約を確定
(critic 3ラウンド。**main 統合済み PR #204**)。実装は`services/internal/dbmigrate`への切り出し(pokedexのUp/DownAll/Versionを
`fs.FS`引数化し、pokedexは薄いラッパーに)・grants.goへの`AppPrivileges`追加・services/record・services/teamの
devices/purge_journal migrationとmigrate CLI(app/migratorの2ロール)まで完了(critic 2ラウンド。**main 統合済み PR #205**)。
TiDB Operatorのk8sマニフェスト(TidbCluster・TidbInitializer)・`up.sh`配線(bootstrap非致命化・Secret作成・
namespace・完了待ちの順序)・Makefileのmigrate-up/down/version-record/team・tidb-local-upターゲットも完了
(critic 2ラウンド。**main 統合済み PR #266**。1回目FAILはTiDB Operator v1.6.6の実ソースを取得して
裏取りした結果判明した`passwordSecret`のキー名誤り・初期化用imageの誤り・namespace不一致・AC-T7違反、
2回目FAILは新設したk8s-renderがクラスタ未起動環境で失敗/ハングする退行)。
**残**: 共有k3dクラスタへの実適用(AC-T3・AC-T8)は、他レーンが使う共有クラスタへの影響を先に確認する
必要があるため意図的に未実施。次の`make up`実行時にTidbCluster/TidbInitializerが実際にReady/Completedに
なることを確認すること(TidbInitializerが使う`tnir/mysqlclient`はamd64専用イメージのため、Apple Silicon
のk3dノードでの起動可否も未確認)。`grants_tidb_test.go`・`migrate_tidb_test.go`(`-tags tidb`)も
このサンドボックスでは実TiDBに対して未実行(tiup playgroundのpdがdarwin/arm64でクラッシュ)。
`make test-db`により検証してからP5-1完了とする
Status(追記): P5-2(NATS JetStream。calc-svcからのイベント発行)完了・**main統合済み**(ADR-0212。critic 2ラウンド)。
ストリーム`CALC_EVENTS`・Retentionは意図的にLimits(Interest ではない。理由はADR-0212 §4)・
`services/internal/calcevents`のワイヤフォーマットを確定。
Status(追記): P5-3(record-svc)完了・critic 2ラウンド(1回目FAIL〈重大1・重要3・軽微7件〉→修正→2回目PASS
〈軽微4件〉、軽微も反映済み)。`api/openapi.yaml`にrecordの契約(`deleteRecordDeviceData`・
`listFrequentOpponents`・`store_unavailable`)を追加、record-svcの保存(TiDB実装。SaveCalcEvent/
PurgeDevice/TouchDevice/FrequentOpponents)・NATS購読(`services/record/internal/events`)・
gatewayルーティング/CORSのDELETE許可まで実装。critic指摘で判明した重要事項: (1)
`services/record/internal/store`にfakeしかテストが無かった欠落を`tidb_test.go`(`-tags tidb`。
`make test-db`)で解消し実TiDB(`pingcap/tidb --store=unistore`)で全緑を確認、(2)
`frequent_opponents.last_calculated_at`が再配送の順序次第で巻き戻る不具合を`GREATEST`で修正・回帰テストで
修正前に落ちることを確認、(3) golang-migrateのmysqlドライバがTiDBでSERIALIZABLE分離レベルを要求し失敗する
既知の非互換を発見(`Lock()`と`SetVersion()`の両方が原因で`x-no-lock`でも回避不可)・
`tidb_skip_isolation_level_check=1`をTidbInitializer/tidb-local-up.shに追加(ADR-0211追記。既存クラスタでは
手動設定かTidbInitializer再作成が必要)。失効ジョブ(ADR-0209 §4)とrecord-svcのDeployment/Service配線は
**P5-3bへ切り出し**(plan.md参照。現状k3dでは`/api/record/*`はupstream_unavailableのまま)。
main未統合(PR #372。P5-2と同じブランチ・PRでまとめている。ユーザーのテスト確認・マージ待ち)。
Status(追記): P5-4(team-svc構築CRUD)実装完了。契約(`api/openapi.yaml`のteam操作。ADR-0213 spec-writer工程)・
team-svcの保存(TiDB実装。CreateTeam/UpdateTeam/DeleteTeam/GetTeam/ListTeams/PurgeDevice/TouchDevice(FromEvent))・
NATS購読(`services/team/internal/events`。durable名`team-svc`はrecord-svcと別、イベントのDetailは一切保存しない。
ADR-0213 §5)・gatewayルーティング/CORSのPUT許可まで実装。critic 1回目FAIL(重要3・軽微6)→修正対応中:
(1) team_members への3クエリ(loadMembers・UpdateTeam/DeleteTeamのDELETE)にdevice_id絞り込みが抜けていた
(ADR-0209 §6-1違反。実害は無いが規律違反)のを修正し、device_idが食い違う行を使った回帰テストで固定、
(2) speciesKey/itemId/abilityId/natureId/teraTypeの文字数上限(DB列幅と対応)を検証せず、超過するとINSERT失敗が
503 store_unavailableに化けていたのを400 invalid_inputに修正、(3) `services/team/Dockerfile`にserverターゲット
未追加だったのをrecord-svcと同形で追加。失効ジョブ(ADR-0209 §4)とteam-svcのDeployment/Service配線・
`GATEWAY_TEAM_URL`は**P5-4bへ切り出し**(plan.md参照。現状k3dでは`/api/team/*`はupstream_unavailableのまま)。
main未統合(critic再レビュー待ち)。
Next: critic 2回目レビュー→PASSしたらP5-3・P5-4をまとめてmain統合。その後P5-3b・P5-4b(失効ジョブ・
Deployment配線。優先度低)は後回しにして次の区切りへ。データレーンからの依頼(issue #271・#270。
`MasterMove.mechanisms`の公開・calc応答への`unsupported`印。DECISIONS.md 2026-09-25参照)を次の区切りで対応。
iOSレーンからの提案(issue #274/#272。BulkCalcRequestへの`defenderOverride`追加。DECISIONS.md 2026-09-25
参照)はM2完了後に着手。issue #103・#148の依頼(データ・Web・iOS・運用レーンへ)、getMove 実装の再レビュー依頼(データレーンへ。60fbe25で対応済み)・iOS再生成依頼(a1f5d5eで対応済み)、P4-17完了(Webレーンへ連絡予定)はDECISIONS.mdに記録済み

## Web
Lane: Web(`web/`・Playwright。どの AI が進めてもよい)
Active: Claude Code(ユーザー指示で再開)
Branch: feat/web-p4(作業ディレクトリ ~/MyDamageCalcurater-web)
Status: P4-1〜P4-6・P4-8〜P4-12(仮想敵 threats・おすすめタイプ recommendations を含む。ADR-0303)・P4-14・P4-15・DOC-web・P4-7(M1 完了報告)完了(critic PASS。main 統合済み。PR #82・#84)。
データレーンの依頼(P2-3b・ADR-0106)にも追従済み(PR #84): AbilityEffect に defImmuneTypes・defAbsorbTypes、
exportBalanceReadModel に absorb と ADR-0106 §決定7の出力順・無効優先。本物の engine.wasm で無効・吸収を結合テスト確認済み。
verify-m1.md を完成版にした: P2-2c/d・P2-3・P3-3 が main に入り、k3d(gateway 経由 http://localhost:8080)で
計算・逆算・タイプバランス(仮想敵・おすすめタイプ含む)を実地確認(pokedex-svc は実データ投入済みだが、
gateway/calc-svc のマスタ参照先はまだ pokedex-svc に向いていない。API レーンの依頼 d が一時停止中)。
P4-5 は Chrome で確認済み(Safari は未確認。人間の作業)。
**P4-16・P4-16b・P4-16c(オンライン MasterSource。ADR-0304)完了・main 統合済み(PR #128・#134・#153)**:
`createOnlineMasterSource`(持ち物・性格を全件取得、種族は検索専用インターフェース)、`MasterData.capabilities`
(公開 API に無い機能を明示的に無効化)、画面側(CalcScreen・ReverseScreen は技・持ち物候補比較が無効なとき
disabled+案内、種族一覧が無効なとき検索欄 `SpeciesSearchField`。BalanceScreen は speciesList・moves が両方
そろうまで画面ごと無効化)、検索欄のキーボード操作(WAI-ARIA "List Autocomplete with Automatic Selection")と
CSS。3段階とも critic 1回目 FAIL→修正→2回目 PASS で完了(重大バグ・文言バグ・IME変換中のキー横取りなど、
いずれも critic が発見)。既存752件は無変更のまま最終907件。技の ID→実体化(`getSpecies.learnset`)は公開 API
に手段が無く、データ/API レーンへ既定案付きで提案済み(DECISIONS.md 2026-09-23、未回答・急ぎではない)。
**P4-19(issue #110 セキュリティ。ADR-0300 §10)も完了・main 統合済み(PR #142)**: 持ち物候補を配列を作る
最終地点で64件に決定的に絞り込み、観測は16件で disabled+案内。critic PASS(境界値の網羅探索と変異テストで
上限超過が起きないことを確認)。データレーンの engine/wasmapi 側(ADR-0108・PR #138)も main 統合済み。
issue #110 は iOS の追従待ちで Web 単独ではクローズしない。
**P4-18 issue #99(ライトテーマの danger コントラスト不足)完了・issueクローズ済み**: Web 分(PR #164)は
danger のライト値を `#E5484D`→`#CD1D23` に変更(WCAG 2.2 SC 1.4.3 の4.5:1を bg.base・bg.glass 合成後の
両方で満たす)。`web/src/test/colorContrast.ts`・`web/src/styles/contrast.test.ts` を新規追加。critic PASS
(独立実装での検算・変異テストで確認)。iOS 側も完了(`fix: ライトテーマの danger コントラスト不足を修正
(issue #99)`。`PokeCalcDesign.swift`・`ColorContrast.swift`・`DangerContrastTests.swift`)。issue #99 は
クローズ済み。
**issue #113(逆算の古い計算要求の抑止・キャンセル)は Web 分完了・main 統合済み(PR #174)**: 観測のテキスト
編集のみ200msのtrailing debounce、確定操作は待たない。`CalcEngine` に任意引数 `signal?: AbortSignal` を
追加、取り消しは `REQUEST_ABORTED_CODE` で区別(ADR-0300 §11)。critic PASS(非同期・競合状態を重点検証。
mutation テスト9件で確認)。iOS 側も完了(`fix: iOS の計算・逆算で古い計算要求をキャンセル・観測入力を
debounce (issue #113)`。`LatestTaskRunner.swift`・`CalcInput.swift`)。**issue #113 自体はまだ open**
(API レーンの「クライアントのcancel伝播」連携分が残っているかは未確認。Web・iOS とも自レーン分は完了)。
**P4-17(技の ID 解決。ADR-0304 A-13)完了・main 統合済み(PR #202)**: API レーンが新設した
`GET /api/pokedex/moves/batch`(`getMovesByIds`)を使い、オンラインモードの技選択を復活させた。
`resolveSpecies` が learnset を技の実体に解決して返す設計(`MasterSpeciesResolution.moves`)。`learnset` が
64件を超える場合はチャンク分割して並列に複数回呼ぶ(API レーンが実データで確認: 349種族中151種族・43%が
64件超・最大106件。稀な例外ではなく主経路として実装・テスト)。`capabilities.moves` の意味は変えずオンラインで
`false` のまま(`true` にすると BalanceScreen が「有効なのに技が選べない」壊れた状態になるため)。技セレクトの
disabled 判定は「今の種族の技候補があるか」に変更。critic レビュー: チャンク分割・結合順序・全体失敗の扱いを
mutation テストで確認(全滅)。重要指摘1件(攻守入れ替え・与えた/受けた切り替え後の選択中の技〈候補一覧だけで
なく実際にリクエストに乗る技〉が未検証。`<select>` の DOM 値は状態が壊れていても先頭候補にフォールバック表示
するため見逃しやすい)を受け、実際のリクエストを検査する形に既存テスト2件を強化。既存1192件は無変更・新規
28件追加(1220件)。BalanceScreen 自体の種族検索・技選択は P4-17b として積み残し(下記で完了)。
**P4-17b(BalanceScreen のオンライン対応。ADR-0304 A-14)完了・main 統合済み(PR #207)**: P4-17 で
`capabilities.moves` が永続的に false のままと決まった結果、従来のゲート `speciesList && moves` では
BalanceScreen がオンラインで永久に使えなかった。可否の判定を「一覧がそろっているか」から「入力の口が
あるか」(`(speciesList || masterSearch) && (moves || (!speciesList && masterSearch))`)に置き換え、
パーティ・仮想敵の12枠(6枠×2)それぞれで種族検索→技解決(`useSpeciesResolutions` を1画面で共有)を
独立に行えるようにした。`moveById`/`hasDamagingMove` を fail-closed に直し、実体不明の技 ID を攻撃技と
誤判定して誤解を招く診断(coverage の誤呼び出し)を出さないようにした。critic PASS(mutation testing で
ゲート条件・fail-closed 判定・種族解決の登録漏れ等の主要な変異を全て検知)。critic 指摘の軽微3件は
その場で直接修正: 種族解決の適用を index ではなく枠の id で引くよう変更(検索解決を待つ間に他の枠が
削除されると index が別の枠を指しうる競合の根治)、A-14.1 の境界表(7パターン)の未カバー2行のテスト追加、
特性名解決の重複ロジックの統一。新規15件追加(1243件)。
**P4-21(issue #67・#98)完了・main 統合済み(PR #189・#192)。Codexレビューissue(P4-18・P4-21)はこれで
すべて完了**:
- #67(2xxの契約外JSONでAPIクライアントが例外を投げる): `apiEngine.ts`・`balanceClient.ts` の `postJson` を
  型ガード経由にし(`as Schemas[...]` の型アサーションを除去)、契約外の2xxで例外を投げず
  `engine_unavailable`/`balance_unavailable` を返すようにした(ADR-0301 §4・ADR-0303 §6)。calc側は写像関数が
  読む全フィールドを再帰的に検査、balance側は画面がたどる形だけを検査(leafスカラー・enumは見ない)。
  critic PASS(mutation テスト12件で型ガードの過不足なしを確認)。新規143件追加。
- #98(モバイル幅で計算・逆算画面が横に溢れる): ブレークポイント600px(`docs/design.md`「幅への対応」に記録)。
  600px未満はmobile-firstで縦積み(DOM順・フォーカス順は不変)、600px以上は従来の左右配置。伸縮列を
  `minmax(0, 1fr)` に、カード・selectに `min-width: 0`/`width: 100%` を追加。critic が実ブラウザで13幅×5画面を
  実測し横溢れゼロを確認。新規34件追加(単体18・E2E16)。
  作業中に発見した無関係の既存退行(JD5の判定タブ追加で `a11y.spec.ts` が壊れていた)を別途修正・main統合済み
  (PR #191)。
  最終テスト数: 既存1166件は無変更のまま vitest 1184件・Playwright 31件、すべて green。
**issue #268(gateway経由:8080の白画面)の検証テスト修正・main統合済み(PR #338。実装は別セッション)**:
`services/gateway/scripts/smoke.sh` に追加された「index.htmlが読むJSを実際に取得して200」の検査により、
`TestSmokeScriptAcceptsWebDeployed` のfixtureがscript srcの無いHTMLを返していて落ちていた(タイプバランス
レーンが検証・指摘)。fixtureにscript srcとその配信先を持たせ、JSが404のケースで検知できることの回帰テスト
(`TestSmokeScriptFailsWhenEntryJSMissing`)も追加。
**issue #334(相性表記の倍率併記。iOSとの語の統一)完了・main統合済み(PR #341)**: iOSのDisplayLabels.swiftの語
(「ばつぐん(×2)」「いまひとつ(×0.5)」)にWeb側を揃えた(「効果は」接頭辞は削除)。format.test.tsの0.25/0.5・
2/4のケースが倍率を書き分けるようになり検証強化。新規0件(既存アサーション4件の強化)。
**issue #72(ルートmake e2eが未実装スタブ)完了・main統合済み(PR #350。ADR-0306)**: k3dクラスタ不要な3件
(`web-e2e`→`web-e2e-online`→`web-e2e-balance`)を必ずこの順で実行し、kubectlの現在のコンテキストが
`k3d-$CLUSTER`のときだけ`api-smoke`→`web-k3d-smoke`→`web-k3d-e2e`を追加実行するよう`scripts/e2e.sh`を実装。
クラスタが無ければ黙らずスキップを明示、`E2E_REQUIRE_K3D=1`でスキップさせない逃げ道も用意。critic PASS
(mutation testing 4件で全て検知)。`scripts/e2e_test.sh`(新規80件)を`make test-scripts`に追加。
**issue #71のWeb側(攻撃側プリセット単一化。ADR-0114)完了・main統合済み(PR #352)**:
`web/src/domain/attackerPresets.contract.test.ts`を新規追加。毎回`engine/presets/attacker.json`を読み、
カタログ(順序・既定値・relevantStat・boostMinus・relevantSp・nature)から導いた期待値と
`resolveAttackerPreset`の実際の出力を突き合わせる契約テスト(現状の値は一致済み、実装変更なし)。
JSONを一時的に書き換えるmutationで実際に検知することを確認済み。新規17件追加。iOSの追従が済めば
データレーンが#71をcloseする想定(2026-09-25時点、Web側は完了を連絡済み)。
**issue #333(375px幅でタブの名前が1文字ずつ縦に折り返す)PR #356オープン中(2026-09-25、マージは
オーケストレーターが検証後に実施)**: `App.css`の`.app-tabs__list`にoverflow-x: auto・safe center、
`.app-tabs__tab`にwhite-space: nowrap・flex-shrink: 0。**critic 1回目FAIL**: 素のcenterのままだと
はみ出した先頭タブがscrollLeft=0でも戻れない(centered flexbox overflow clipping。320pxで実測再現)→
`justify-content: safe center`に修正、design.mdに記録。safeキーワードのSafari対応はP4-5のSafari確認
(ブロッカー節)に追記。回帰テスト2件(1行であることの直接確認・スクロールで先頭末尾に到達できることの確認)。
**2026-09-25、オーケストレーター(damage calculation bug resolution)から13件のissue消化を依頼された**
(open 113件中、優先度順): (1) bug: #333(完了・main統合済み。PR #356)・#306(タイプ名コントラスト・
ダメージバー読み上げ名。完了・main統合済み。PR #362)・#275(逆算「受けたダメージ」で自分の耐久が無振り固定。
high severity。完了・main統合済み。PR #366)。
(2) ready-for-implementation: #304(完了・main統合済み。PR #375)・#308・#305・#248・#218・
#219(APIレーン連携)・#211(APIレーン連携)・#332(devDependencies更新)・#226(README等の実装状況)は
#304以外未着手。(3) needs-decisionだが「要望済み機能は実装しきる」方針で既定案付きで実装: #272(特性選択)・
#274(急所・やけど・天候・フィールド・ランク・壁・特性の指定。iOSへも連絡済み)・#210(オフライン実データ)は
未着手。P5-5はAPIレーンの契約が出たら最優先。
**#71のWeb側(攻撃側プリセット単一化)完了・main統合済み(PR #352)**。
**`make e2e`のweb-e2e-onlineがmainで壊れていた件(廃止済みCALC_TYPECHART_PATHをcalc-svcが拒否)を発見・
2PRに分けて修理・両方main統合済み**: PR1(#382、MasterExport追従。ADR-0204/ADR-0301§5追記)で
`web/src/master/exportSnapshot.ts`をMasterExportの形に全面書き換え(types/typeChart本体化、
species/moves/items/abilitiesの明示フィールド化、効果のPascalCase変換)、例データのID(move/item/ability)から
ハイフンを除去(codeIDPattern対応)。PR2(#385、pokedexフィクスチャ。ADR-0307)でpokedex-svc(MySQL必須)の
代わりにWeb例データから公開API応答を返す軽量フィクスチャ(`e2e/support/pokedexFixture.ts`+
`pokedexFixtureServer.mjs`)を追加し、`/api/pokedex`を`vite.config.ts`でそちらへ転送。
両PRとも`npm run e2e:online`実弾実行(3/3 pass)まで確認済み。critic指摘(playwright.container.config.tsの
testMatch漏れ、POKEDEX_PROXY_TARGETのvite proxyルーティングが無検査だった点等)は全て修正・検証済み。
Next: オーケストレーターの新しい優先度キュー(2026-09-25時点)で #308 → #305 → #248 → #218 の順に着手する。
その後 #219・#211(APIレーン連携。`mydamagecalcurater-api-67`に契約を確認)、#272・#274(iOSレーンが
既に確定させた文言・順序に合わせる。DECISIONS.md参照)、#210、#332(devDependencies更新)、#226(README等の
実装状況の精度確認)。APIレーンがmechanisms/unsupportedの契約を出したら #271・#270(Web側の未対応表示)も追加。
(3) P4-20: issue #148(アクセス境界・認証方針)。Web 側は既にコード上で条件を満たしていることを確認済み
(apiBaseUrl の既定値は同一オリジン、CORSはgateway側の設定)。実際のtailnet名が決まってから運用レーンより
連絡が来る想定。(4) P5-5(構築ビルダー等)は record/team の API 待ち(M2。2026-09-24 時点で record/team-svc
の DB マイグレーション・TiDB 導入方針〈ADR-0211〉はデータレーンで進行中)。(5) 人間へのお願い:
docs/verify-m1.md §4 を Safari で確認(P4-5。issue #333のsafeキーワード確認も合わせて)

## iOS
Lane: iOS(`ios/`。M3 の Phase 6。どの AI が進めてもよい)
Active: なし(P6-6 完了・main 統合済み。残る P6-7 は record/team の API 待ち)
Branch: feat/ios-p6(作業ディレクトリ ~/MyDamageCalcurater-ios)
Status: **M3(iPhone で使える)は完了**。P6-1(ADR-0500)・P6-2a 計算画面・契約追従・P6-2b 逆算画面・P6-2c 構築ビルダー
(一覧・編集・ニックネーム)・P6-2d(構築から個体を呼び出す配線)・P6-3・P6-4(手順書 `docs/runbooks/ios-device-install.md`)・
生成の internal タグ除外・DOC-ios は main に統合済み(PR #31・#53・#91・#119・#122)。
続けて Codex レビュー issue のうち iOS 主担当分を修正・main 統合済み: #100(種族変更後の特性ID残留)・#101(負のSPの
検証漏れ、PR #131)、#68(検索上限200件。種族・技ピッカーを `Menu` 一括取得から `.searchable()` 検索UIへ変更。
Web の ADR-0304 と同じ方針。PR #136)、#113(Web/iOS/API共同主担当。入力操作ごとの計算Taskを最新の1つだけ保持し
新入力・画面破棄で先行Taskをcancel、逆算の観測文字入力に200msのtrailing debounce。`CancellationError`は画面
エラーにしない。PR #166。1周目critic FAIL→2周目PASS)、#99(ライトテーマの danger コントラスト不足。
`ColorToken.danger`のライト値を`#E5484D`→`#CD1D23`に変更。Web PR #164 と同じ値。PR #170。issue #99 は
Web・iOS 両方完了でクローズ済み)、P6-6(issue #110の iOS側追従。ADR-0501「issue #110」章。`RequestLimits`/
`RequestLimitLabels` を新設し、観測16件・持ち物候補/比較64件(null込み。選べるのは63件)の上限を実装。
観測は追加ボタンを無効化、持ち物候補・比較トグルは上限到達中のON操作だけ拒否(OFFは常時可。Webの
決定的切り捨てとはあえて変えた判断はADR参照)。critic指摘でguardの位置(`beginInput()`より前)を固定する
回帰テストを追補。引き継ぎ検証で契約との同期検査 `ios/scripts/check-request-limits.sh` と観測上限の XCUITest を
追加。**PR #186 で main 統合済み**)。`make ios-test`(gen-check・件数上限の同期検査・XCTest 340件〈xcresult 集計353件〉・XCUITest 17件・Info.plist 検査)が緑。
issue #68 の残り(一度も検索結果に出ていない選択中の技IDを名前解決できない)は P6-9 で `getMove` による個別解決を
実装して解消(ADR-0501「issue #68 の残り」。持ち物の先頭ページが上限に達したら黙って切り捨てず案内を出す。critic
1周目 FAIL〈逆算の古いエラー消去条件の退行〉→修正→2周目 PASS。**PR #199 で main 統合済み、issue #68 クローズ済み**)。
issue #110 は API・データ・Web・iOS すべて完了したためクローズ済み(2026-09-24)。
P6-10(構築編集の load の技解決を `getMovesByIds` のまとめ取り1回へ。ADR-0501「getMovesByIds による構築編集の技の一括解決」。
critic 1周目 FAIL〈分割境界のテスト不足〉→テスト追加→2周目 PASS)完了。
P6-11(issue #334。攻撃側プリセットの表示名を技の分類に追従。PR #348、issue クローズ済み)・P6-12(issue #71 の iOS 追従。
`engine/presets/attacker.json` との契約テスト、並び 無振り→特化→振り、既定を無振りに変更。ADR-0501「P6-12」)完了。
P6-13(issue #274。計算画面の「詳細」: 急所・やけど・天候・フィールド・防御側の壁・攻撃側のランク・特性。PR #377。語は
DECISIONS.md に記録し Web が合わせる)・P6-14(最大の文字サイズで計算画面が横にはみ出す既存の不具合。結果行の `.fixedSize()` が原因)完了。
P6-15(アクセシビリティ域でプリセットのピルを縦積み等)完了。
Next: (1) API レーンが `BulkCalcRequest.defenderOverride`(防御側のランク・特性・状態異常。
DECISIONS.md 2026-09-25 で採用、M2 の後に実装予定)を入れたら、iOS の「詳細」に防御側の入力を追加。
(2) P6-7(issue #103・ADR-0209 §8の削除UI。record-svc/team-svc実装待ち、急ぎではない)。将来の候補:
engine の Champions マスタが pokedex-svc 経由になったら iOS のモック/実マスタの差し替え動作を再確認、Web の
record/team-svc(M2)が進んだら iOS の構築を端末内保存から API 保存へ移行するかを検討。

## Type Balance Checker
Lane: タイプバランス(どの AI が進めてもよい。COORDINATION.md)
Active: Claude Code(M4 P7-1完了、P7-2に着手予定)
Branch: 次は main から feat/tb-<名前> を切る(作業ディレクトリ ~/MyDamageCalcurater-tb。git worktree)
Status: 設計書(docs/type-balance-design.md)の TB0〜TB6 はすべて main に統合済み(TB6: 技範囲チェッカー、PR #65)。P2-3b(特性の無効・吸収)の実データ確認を完了(2026-09-23): データレーンが再生成した export(348 pokemon・moves・216 abilities)で `make balance-k3d-deploy-readmodel && make balance-smoke-readmodel` を実行し、`POST .../team-balance/analyze` でチリーン(levitate)への ground 攻撃が `{"category":"immune","effect":"immune","multiplier":"0","source":"ability"}` になること、`POST .../move-range/analyze`(thunderbolt)の `walledByAbility` にエモンガ(motordrive)が正しく含まれることを実データで確認済み。メガフォームの nameJa が英語表記のままの件はデータレーンへ確認候補として残る(ブロッカーではない)。
Codexレビュー issue #105(Argo CD導入のハッシュ・digest固定)対応完了(2026-09-23。ADR-0405。PR #140)。
**別セッションからの依頼(ユーザー承認済み)で M4 P7-1(kube-prometheus-stack / Loki、各サービスのメトリクス)に着手・完了**
(2026-09-24〜25。ADR-0406。PR #201・#336): 6サービス(gateway・pokedex・calc・balance・speed・judge)に `GET /metrics`
(Prometheus text format、method/pathはカーディナリティ対策で正規化)、`scripts/observability-bootstrap.sh` で
kube-prometheus-stack・Loki(SingleBinary)・Alloy を版・SHA-256固定で導入。実クラスタ(k3d-pokecalc)で実行し、
全Pod起動・PVC Bound・6 ServiceMonitor適用・balance/calc/gatewayのscrapeがup・GrafanaのLokiデータソースで
実ログ取得まで確認済み(judge/pokedex/speedは`/metrics`追加前の古いイメージのため404。各レーン再デプロイで解消見込み)。
Next: M4 P7-2(SLO: 計算API p99<100ms・可用性、ダッシュボード)に着手予定。それ以外はタイプバランス設計書・issue対応は
すべて完了、以後はユーザーからの新規要望待ち
メモ: `make balance-k3d-deploy`(local overlay)で上書きすると Application は OutOfSync になる(manual sync なので戻らない)。GitOps に戻すときは Argo CD で Sync

## Speed
Lane: 素早さ(素早さ比較サービス。`services/speed/`・`web/src/speed/`。どの AI が進めてもよい)
Active: なし(SP0〜SP5 すべて完了。次の要望待ち)
Branch: 次は main から feat/speed-<名前> を切る(作業ディレクトリ ~/MyDamageCalcurater-speed。SP5 は feat/speed-sp5 → PR #97 で main に統合)
Status: SP0〜SP5 すべて完了・main に統合(PR #32・#36・#52・#83・#86・#93・#97)。SP4 の実データ確認はユーザーが2026-09-24 に実施:
`mysql` Service はクラスタ内部の DNS 名で Mac からは解決できないため、`kubectl -n pokecalc port-forward svc/mysql 3306:3306` を張り、
DSN のホストを `127.0.0.1` に付け替えて `make pokedex-export`(348 pokemon)→ `make speed-k3d-deploy-readmodel` →
`make speed-smoke-readmodel` を実行(初回はロールアウト直後で 504、再実行で `speed readmodel smoke: pokemon=0003-000 list=200 table=200`)。
SP5 の実際の Argo CD への適用(`speed-argocd-app`・`speed-registry-push`・sync)は未実施のまま(ADR-0605 §4。共有クラスタへの変更のため
人間の確認のもとで、必要になったときに)
Status(追記): DOC-arch(docs/architecture.mdを全レーンの現行構成に合わせて更新。ユーザー依頼)を素早さレーンが担当・完了(PR #168・#194)。
judge-svc を全体図・コンポーネント表に追加(先に判定レーンの抜けを見つけて#168で対応)、record-svc・team-svc(M2。計画中。
services/record・services/teamはまだ.gitkeepのみ)をTiDB・NATS JetStreamとあわせて追加、Kustomize overlay(local/cloud)の
節を新設。gateway・pokedex・calc・balance・speed・judge・データの流れ・WASMはコードを確認し既に現行と一致(変更なし)。
Status(追記): 2026-09-25、全体レビュー issue の割り当てミス(#71・#74・#76・#77 は素早さ担当ではなかった)を指摘し、
データレーンへ差し戻し済み。素早さが実際に担当に入る open issue を洗い出し: #263(タイプバランス主・素早さ・運用。Argo CD
Applicationのproject: default・初期admin Secret残存・GitOpsスクリプト5本の重複)・#237(タイプバランス・素早さ。needs-decision。
GitOps overlayがread modelを持たずbalance/speedの業務APIが全て503。既知の制約はADR-0605 §2aに記載済み)・#236(タイプバランス・
素早さ・判定・API連携。端末ID/セッションIDの検証とエラーコードがgatewayと3サービスで不一致)・#108(既知・データレーン主担当)。
タイプバランスレーンと分担を確認済み: #263はタイプバランスレーンが主担当(speed側の差分は連絡が来たら対応)、#237は既定案
(initContainerでpokedex exportを起動時に実行)でユーザー確認中(タイプバランスレーンが担当)、#236は共通パッケージの置き場所を
タイプバランスレーンがAPIレーンと相談中。#237の実装には「pokedex-svcのserverイメージをbalance-registryへdigest固定でpush」という
データレーンへの新しい依頼が発生することをタイプバランスレーンに共有済み。
Status(追記): 2026-09-25、#236のspeed側を完了(ADR-0606。PR作成中)。gatewayのcheckAPIHeaders/isCanonicalUUIDを
`services/speed/internal/httpapi/requestctx.go`に複製(httpmetricsと同じ前例。共通パッケージ新設なし、APIレーン合意済み)。
X-Device-Id/X-Session-Idの検証を正準形UUIDに強化し、エラーcodeを`invalid_request`から`missing_header`/`invalid_header`
へ分離(契約の破壊的変更)。openapi.yaml 0.4.0・web/src/speed/speed.gen.tsを再生成・critic PASS。balance・judgeは各自対応。
Next: #263・#237 はタイプバランスレーン/APIレーンからの連絡待ち(連絡が来たら speed 側の overlay・scripts を対応)。
#105(Argo CD導入・digest固定の共有スクリプト化)は完了・追加対応不要。#108は データレーンからの連絡待ち(今は着手不要)。他は
balance-registry → pokecalc-registry への改名提案(タイプバランスレーンへ既定案で提示済み。DECISIONS.md 2026-09-23)かユーザーからの
新規要望待ち。

## Judge
Lane: 判定(素早さ×ダメージ連動。`services/judge/`・`web/src/judge/`。どの AI が進めてもよい)
Active: なし(**judge-design.md §3 が定めた JD0〜JD5 すべて完了・main 統合済み**。次のユーザー要望待ち)
Branch: 次は main から feat/judge-<名前> を切る(作業ディレクトリ ~/MyDamageCalcurater-judge)
Status: JD0(基盤。PR #92)・JD1(判定API本体。PR #118)・JD2(場の効果。PR #127)・JD3(複数の相手候補。PR #143)・
JD4(返り討ち判定。PR #169)・JD5(Web の画面。PR #182。ADR-0705)まで全段階が完了。`POST /api/judge/v1/outspeed-and-ko`
は自分1体対相手1〜6体の素早さ判定・場の効果(トリックルーム・追い風)・返り討ち判定まで対応し、`web/src/judge/`
(`/judge` タブ)から呼べる。技はID自由入力(ADR-0304 §3の技一覧APIの欠落を踏襲)、相手側の追い風は全候補共通の
1チェックボックス(ADR-0703 §5)、送信ボタンでのみ呼ぶ(1回で上流最大27回)。
Next: 新規要望待ち。軽微な積み残しは解消済み(2026-09-25。`attacker`単数の`Individual`にも`defenders`候補と
同じ大文字小文字厳密なキー検査〈`individualWireKeys`〉を適用。PR #342 main 統合済み)。
issue #234(moveId/natureId の形式検証。ADR-0706)も解消(2026-09-25。critic 2ラウンド。PR #365 main 統合済み):
名前付きスキーマ `MoveId`/`NatureId`(pattern `^[a-z0-9]+(-[a-z0-9]+)*$`・maxLength 64)を契約に追加し、
`outspeed.go` の3箇所(attacker moveId・natureId共有・候補moveId)で上流呼び出し前に検査、
`pokedex.go` は `url.PathEscape` で二重の守り。`web/src/judge/judge.gen.ts` も手動再生成(ADR-0705 §2)。
issue #213(重大度 high。リクエスト全体の期限。ADR-0707)も解消(2026-09-25。critic PASS〈1回目〉。PR #370 main 統合済み):
`JUDGE_REQUEST_TIMEOUT`(既定12秒。`writeTimeout`=15秒未満を起動時検証)を新設し、`outspeedAndKo` の
先頭で ctx を1回だけ `context.WithTimeout` でラップして以降の上流呼び出しに使い回す(呼び出し順序・
逐次打ち切り規約〈ADR-0703 §3〉は無変更)。`internal/client` は無変更(`http.NewRequestWithContext` の
既存の context 統合だけで「進行中呼び出しの中断」「未着手呼び出しの即時失敗」の両方が成立)。
上流が遅くても期限内に503 JSONを返すようになり、クライアントが空応答(HTTP 000)を受け取ることが無くなった。
issue #329(重大度 low。SP合計67の境界値テスト欠落)も解消(2026-09-25。テストのみ・実装無変更。PR #371 main 統合済み):
`validateSP` の合計超過検査の既存テストが境界〈67〉から遠い(96)ため、
`> engine.MaxSPTotal` を `+1` する退行を検出できなかった。境界値(合計66は受け付け・67は拒否)の
テストを `internal/judge`・`internal/httpapi` 両方に追加し、mutation test で実際に検出できることを確認。
issue #257(重大度 low。smoke.sh が healthz のみ)も解消(2026-09-25。テスト用スクリプトのみ。PR で main へ):
gateway smoke の ID取得部分を流用し `POST /api/judge/v1/outspeed-and-ko` の 200(hits含む)・
ヘッダなし400・未知speciesKey 422・7候補400 を実クラスタ(k3d-pokecalc、実データ)で確認済み。
`Makefile` に `API_URL` を追加、README の古い「JD0完了」表記も修正。
iOS版JD5は要望が出たら判断(ADR-0705 却下案)

## Shared Interfaces
- Pokemon ID: pokedex-svc の `{図鑑番号4桁}-{フォルム3桁}` 形式に準拠
- Type: 18タイプの英語小文字ID(fire, water, ...)。表示名・色は docs/design.md のトークンに準拠
- Type multiplier: 分数ではなく整数表現(claude-review.md 参照)
- サービス境界: damage-calc と balance は兄弟。相互の実行時 API へ直接依存しない
- 共通マスタ: 正本は1つ。`feat/claude-p1-engine` の ADR-0002 にあるコミット済みスナップショット案を候補とし、人間の確認待ち
- balance の type chart: P1-13 の `testdata/golden/typechart.json` をバイト複製して同梱(ADR-0015)。正式マスタ確定後に provider を差し替える
- 共有状態の正本: `origin/main` の `docs/ai-shared/`。未マージの続きは各レーン欄の `Branch` の最新コミット(COORDINATION.md)
- 開発の正本: private の `origin` の `main`(PR でのみ更新。Argo CD が参照)。作業ディレクトリはレーンごとに `~/MyDamageCalcurater`(ダメージ計算)と `~/MyDamageCalcurater-tb`(タイプバランス)
