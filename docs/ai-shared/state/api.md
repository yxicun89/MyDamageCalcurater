## API
Lane: API(calc-svc・gateway・契約テスト。`api/openapi.yaml` の持ち主。どの AI が進めてもよい)
Active: なし(2026-10-03 時点で、このレーンの実装・issue 対応は main 統合済み。次のセッションは下の Next から続ける)
Branch: なし(作業ディレクトリ ~/MyDamageCalcurater-api。直近の PR: #463・#468・#469・#483・#488・#490・#506・#510・#536・#539・#542 はすべて main 統合済み)
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
Status(追記): P5-2・P5-3・issue #271/#270(mechanisms公開・unsupported印)を**main統合済み(PR #372)**。
Status(追記): **M2(P5-1〜P5-4)完了**。P5-4(team-svc構築CRUD)実装完了。契約(`api/openapi.yaml`のteam操作。
ADR-0213 spec-writer工程)・team-svcの保存(TiDB実装。CreateTeam/UpdateTeam/DeleteTeam/GetTeam/ListTeams/
PurgeDevice/TouchDevice(FromEvent))・NATS購読(`services/team/internal/events`。durable名`team-svc`は
record-svcと別、イベントのDetailは一切保存しない。ADR-0213 §5)・gatewayルーティング/CORSのPUT許可まで実装。
critic 2ラウンド(1回目FAIL〈重要3・軽微6〉: (1) team_members への3クエリにdevice_id絞り込みが抜けていた
〈ADR-0209 §6-1違反。実害は無いが規律違反〉のを修正し回帰テストで固定、(2) 文字数上限未検証で入力エラーが
503 store_unavailableに化けていたのを400 invalid_inputに修正、(3) Dockerfileのserverターゲット欠落を修正
→ 2回目PASS)。実TiDB(`pingcap/tidb --store=unistore`)で全テスト確認済み。失効ジョブとDeployment配線は
**P5-4bへ切り出し**(plan.md参照)。**main統合済み(PR #409)**。
Status(追記): issue 272のAPI分(defenderOverride.abilityId・ReverseRequest.unknownAbilityId。ADR-0214。
engine側はPR #402・ADR-0126で完了済み)を実装。省略時は種族の全特性(最大3件。4件目=Showdownの特殊枠"S"は
ADR-0105 §5と同じ判断で落とす)を解決して渡すため、1つしか特性を持たない種族は必ずその特性が効くように
なる。`BulkCalcRow`・`ReverseCandidate`に`abilityId`/`abilityIds`(必須)を追加。critic 2ラウンド(1回目
FAIL: 省略時に解決した特性のEffectが実際にengineへ届くことが無テストだった→対照種族ペアのテストを追加して
修正→2回目PASS)。一括計算・逆算の行数/候補数上限(ADR-0208)が特性分岐で最大3倍まで増えうることを追記。
**main未統合(PR #411。ユーザーのテスト確認・マージ待ち)**。
Status(追記): issue #284(balance/speed/judgeをgatewayの後ろにまとめる。ユーザー決定・DECISIONS.md
2026-09-25「ユーザー決定 4 件」#2)を実装。`routing.go`に`routeBalance`/`routeSpeed`/`routeJudge`と対応する
prefix(record・teamと同じ前方一致・末尾スラッシュ必須の規則)、`requiresHeaderCheck`にも3つを追加して
端末ID・セッションIDの検証(issue #236で判明していたTraefik直結の穴)をgatewayでも課すようにした。
`server.go`に`Config.BalanceURL`/`SpeedURL`/`JudgeURL`と対応するReverseProxy、`main.go`に
`GATEWAY_BALANCE_URL`/`GATEWAY_SPEED_URL`/`GATEWAY_JUDGE_URL`を追加。CORS許可メソッドは変更なし。
`/api/{balance,speed,judge}/healthz`(完全一致のみ)はヘッダ検証を課さない(3サービスの契約の
`publicHealth`・ADR-0600/ADR-0700がIngress越しの疎通確認用としてヘッダ不要と明記しているため。
critic 1回目FAILで発覚し修正済み)。deployment.yamlへの実URL配線はrecord・team(P5-3b/P5-4b)と
同じく別タスクとして残す(コードのみ今回のスコープ)。critic 2ラウンド(1回目FAIL〈重要2件:
healthz例外の欠如・README.mdのルーティング表が古いまま〉→修正→2回目PASS)。**main未統合**。
Status(追記): 2026-10-01 PR #416(issue #284: balance・speed・judgeをgatewayの後ろに統一)を main 統合。続けて UnsupportedMark の target・reason を string に緩めた(ADR-0215。Web・iOS の追従込み)。
Status(追記): 2026-10-02、issue #236 の judge 分を完了(ADR-0219。ブランチ fix/api-236-header-validation、PR 待ち。balance は PR #458)。端末ID・セッションIDを gateway・speed と同じ正準 UUID 検証に揃え、code は `missing_header`/`invalid_header`。judge は非 UUID を calc へ転送しない。judge の openapi・Web 生成型・ja.ts を更新。
Status(追記): 2026-10-03、ダブルの壁・全体技を engine・wasmapi に反映(issue #232 のダブル分。ADR-0222。実装済み・critic 待ち)。API 契約は形・enum 不変(説明のみ)。`move.target` は engine・WASM のみ(OpenAPI には無い)。技の対象がマスタに無い間はダブルの全攻撃技に move_target_unknown の印。

Status(追記): 2026-10-03 issue #315 のメガ部分実装済み(メガ種族+requiredItemId 以外の持ち物は 400 invalid_input。ADR-0200 §4 追記。テラスタイプは別作業)。

Status(追記): 2026-10-02 issue #211 の API 分(ADR-0218)実装済み・critic PASS・コミット前。公開の `Item` / `Ability` に省略可の `effect` を足し、pokedex-svc が共通マスタで検証して返す(不正は 503)。Web・iOS への連絡は DECISIONS.md。
Next(2026-10-03 更新):
(1) 人間の判断待ち: teraType を calc に送られたときの扱い(#497 の未対応の印のまま/黙って無視/400/印を消す。iOS の構築メンバーが送っている。ADR-0222 §4)。ダブルで相手1体の全体技の見せ方。失効ジョブ(record-expire・team-expire)を実データへ初めて向ける承認(ADR-0209。承認までは suspend: true。runbooks/api.md §8)。
(2) 追跡中: #498(calc の打ち切り。engine は純粋なまま)、#505(wasmapi のメガ持ち物検証。データ・Web)、#211 の Web 追従(effect の写し)。
(3) マージ後の実機確認: k3d で gateway の /metrics が公開側 404・専用ポート 9090 で取得、/healthz に version、/api/record・/api/team の疎通(#469・#490)。
Status(追記): P5-3b・P5-4b 実装済み(ADR-0220。critic PASS・PR #490 で main 統合済み。失効 CronJob は承認まで suspend)。`deploy/k8s/base/{record,team}`(Deployment・Service・保持日数の ConfigMap・日次の失効 CronJob)、gateway の `GATEWAY_RECORD_URL`・`GATEWAY_TEAM_URL`(base)、`record expire`・`team expire`(同じバイナリのサブコマンド。`internal/expire`。冪等・1回の上限・終了コード 0/1/2)、NetworkPolicy 4本、/metrics と ServiceMonitor、cloud overlay での失効ジョブ suspend、up.sh の server イメージ build。TiDB 実機(`make test-db-docker`)の expire テスト含め green。k3d への実デプロイは未確認(人間が確認)。
Status(追記): issue #288 の API 分(ADR-0223)実装済み(critic PASS・PR #536 で main 統合済み)。内部 API `MasterMove.target`(必須・nullable)・calc-svc→`engine.Move.Target`(`master.MoveTarget.Engine()`)・公開 `Move.target`(省略可 single/spread。NULL は省く・未知は 503)を配線。使い捨て mysql:9.7.2 で `go test -tags mysql -p 1 ./pokedex/...` 全緑。Web への連絡は decisions/2026-10-03-api-move-target-wiring.md
Status(追記): 2026-10-03 issue 514 完了(PR #539 で main 統合済み。ADR-0802 追記)。gateway が `/api/*` の上流の非 JSON 5xx を 503 `upstream_unavailable` に正規化(上流の 500 も 503 になり `Retry-After` 等は落ちる)。閉じた下書き(fix/api-325-error-shape)の gateway 部分だけを現 main の proxy.go に手で再適用。critic PASS・PR #542 で main 統合済み。
Status(追記): 2026-10-03 issue 538 完了(ブランチ fix/api-538-flaky-deadline-test。ADR-0801 追記)。`httpguard.Expired` が期限直後の context を取りこぼす競合を修正(4 複製)。critic PASS・PR #542 で main 統合済み。
