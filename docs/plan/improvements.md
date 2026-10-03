## 改善要望(/improve で追加)
(ここに要望と対応状況を書く)
- [ ] お気に入り(手動ピン留め)のCRUD API(requirements.md §2「あれば便利(マストではない)」)。
  `favorites`テーブル・保持期間(540日)・全削除時の件数カウントはADR-0209で設計・実装済みだが、
  作成・削除・一覧のAPI自体は未着手(ADR-0209にも「recordにお気に入りのCRUDを足すときに検証する」と
  将来課題として記述されている)。Webレーンからの問い合わせ(2026-09-25。P5-5着手時)で未実装であることを
  確認・回答済み。着手するかどうかはユーザー判断待ち(急ぎではない)
- [x] issue #271/#270(データレーンからの依頼。ADR-0121 §4・ADR-0123 §7。DECISIONS.md 2026-09-25)の API レーン
  担当分: `api/openapi.yaml` に `MasterMove.mechanisms: string[]`(必須・昇順・通常の技は空配列)と
  `CalcResult`(`BulkCalcRow.result` も同じ型)・`ReverseCandidate` への `unsupported: UnsupportedMark[]`
  (必須・印なしは `[]`)を追加(`make gen`)。pokedex-svc の内部マスタ export に `ListMoveMechanisms` を
  配線(SQL の並びに頼らずこの層で昇順ソート)、calc-svc は `sharedmaster.MoveRow.Mechanisms` にそのまま渡す
  だけ(検証は既存の `MoveMechanismsOf` が担当)。calc-svc の応答変換(`calcResultFrom`・`reverseResultFrom`)
  に `unsupportedFrom`(`engine/wasmapi` と同じ変換)を配線し、HTTP/WASM パリティテストの
  `dropEmptyUnsupported`(印を比較対象から除外する暫定処置)を削除して印も比べるようにした。
  Web の例データ(`exportSnapshot.ts`)に `mechanisms: []` を追加(Web は `unsupported` をまだ受け取らない
  設計のまま。`mapCalcResult` 等の明示的フィールド写像により自動的に弾かれる。issue #67 の前方互換どおり)。
  データレーン・Web レーン・iOS レーンへ連絡済み(iOS は生成物の再生成が必要)
- [x] issue #274/#272(iOS レーンからの提案。DECISIONS.md 2026-09-25)の API レーン担当分のうち **abilityId**:
  `BulkCalcRequest.defenderOverride.abilityId`(ADR-0126・ADR-0214)。データレーンが engine 側
  (`BulkInput.DefenderAbilities`・`ReverseInput.UnknownAbilities`。PR #402)を実装済みで、API レーンは
  `defenderOverride.abilityId`(既に採用済みの概念)をその1件として渡す配線と、`ReverseRequest.unknownAbilityId`
  (新規)・`BulkCalcRow`/`ReverseCandidate` への `abilityId`/`abilityIds`(必須)を実装。省略時は種族の全特性
  (最大3件。4件目は Showdown の特殊枠 `"S"` として落とす。ADR-0105 §5 と同じ判断)を解決して渡すため、
  1つしか特性を持たない種族は必ずその特性が効くようになる(issue の境界値の受け入れ条件を満たす)。
  一括計算・逆算の行数/候補数の上限(ADR-0208)が特性分岐で最大3倍まで増えうることを openapi.yaml と
  ADR-0208 に追記。critic レビュー予定。**残り(ranks・status の上書き)は別タスクとして残す**(このタスクの
  スコープ外。abilityId とは独立に追加できる)。入ったら iOS・Web へ連絡(生成物の再生成・追従は各レーン)
- [x] issue #272 の Web 分(ADR-0311): 計算・逆算画面に特性セレクトを追加。攻撃側(自分)は種族の特性から選び(既定は先頭)、
  防御側(相手)は「おまかせ(種族の全特性)」+各特性から選ぶ(おまかせは先頭3件を `defenderAbilities` /
  `unknownAbilities` に渡す)。WASM は候補をそのまま、API 実装は候補がちょうど1件のときだけ
  `defenderOverride.abilityId` / `unknownAbilityId` で送る。行・候補の `abilityId` / `abilityIds` を DTO に写し、
  特性で分かれた行は特性名つきの別行、まとめられた行は名前を並べて表示。`MAX_ABILITY_CANDIDATES` は
  `domain/requestLimits.ts`、選択肢・候補の組み立ては `domain/requests.ts`。iOS は別レーン
- [x] issue #274 の Web 分(ADR-0312): 計算画面に「詳細」(既定は閉じる disclosure)を追加。急所・やけど(攻撃側 burn のみ)・
  天候・フィールド・防御側の壁(3つ独立)・攻撃側のランク(選択中の技の分類で A か C を ±1、-6..+6)を入力でき、
  触った分だけ `critical` / `attacker.status` / `field` / `attacker.ranks` を要求に載せる(既定は従来とバイト同一)。
  要求の形は `domain/calcConditions.ts`、文言は `calcConditionsText`、部品は `screens/CalcConditionsPanel.tsx`。
  条件は攻守入れ替え・種族・技の変更で消さず、変えた直後は古い結果を出さない。iOS は別レーン
- [x] issue #274/#272 の API レーン担当分の残り(ADR-0216): `BulkCalcRequest.defenderOverride` に `ranks: RankBlock` /
  `status: StatusCondition` を追加(全行の防御側に一律で上書き)。engine は `BulkInput.DefenderOverride`
  (`ErrInvalidDefenderOverride`。件数上限の後・計算の前に検証し、results・rows の両ループで特性の直後に当てる)、
  wasmapi は `defenderOverride{ranks,status}`、calc-svc は `parseStatusCondition`+`ranksFromBlock`。範囲外のランクは
  400 `invalid_input`、未知の status は `invalid_enum`。防御側の状態異常は今のダメージ式に効かない(結果不変)。
  逆算には足していない(ADR-0216 §4)。Web・iOS 生成物は再生成済み
- [x] issue #284(ユーザー決定。DECISIONS.md 2026-09-25「ユーザー決定 4 件」#2)balance・speed・judge も
  gateway の後ろにまとめる: `services/gateway/internal/httpapi/routing.go` に `routeBalance`/`routeSpeed`/
  `routeJudge` と `prefixBalance`/`prefixSpeed`/`prefixJudge`(record・team と同じ前方一致・末尾スラッシュ
  必須・不一致は404の規則)を追加し、`requiresHeaderCheck` に3つとも加えて `/api/{balance,speed,judge}/*`
  にも端末ID・セッションIDの検証(ADR-0202 §4)を課すようにした(issue #236 で判明していた「Traefik 直結だと
  gateway の検証を経由しない」穴をこれで塞ぐ。balance/speed/judge 自身が持つ複製の検証〈issue #236〉は
  二重になるが害はなく、削除するかどうかは各レーンの判断のまま残す)。`server.go` に `Config.BalanceURL`/
  `SpeedURL`/`JudgeURL`(nilなら503 `upstream_unavailable`)と対応する `ReverseProxy` を追加、`main.go` に
  `GATEWAY_BALANCE_URL`/`GATEWAY_SPEED_URL`/`GATEWAY_JUDGE_URL` を追加。CORS の許可メソッドは変更なし
  (balance・speed・judge の契約〈`services/{balance,speed,judge}/api/openapi.yaml`〉はいずれも GET/POST の
  みで確認済み)。`deploy/k8s` にはbalance/speed/judge自体のDeployment/Serviceがまだ無く、gatewayの
  deployment.yamlへの実際のURL配線も record・team(P5-3b/P5-4b)と同じく別タスクとして残す(コードのみ
  今回のスコープ)。新規 `balance_speed_judge_routing_test.go` で3サービス共通のルーティング・404境界・
  ヘッダ検証・実転送を固定、既存の `TestUnroutedPathsAreNotFound` から `/api/balance/defense`(今は503に
  変わるため404の例として不適切)を削除、ADR-0202 §3 の表と関連ADR行を更新(ADR-0012の「balanceは独自の
  Ingress」の記述を更新し、ADR-0606〈issue #236 のspeed側〉への参照を追加)。
  **critic 1回目FAIL(重要2件)→修正**: (1) `/api/{balance,speed,judge}/healthz`(完全一致)がgatewayで
  ヘッダ検証必須になっており、3サービスの契約(`publicHealth`)・ADR-0600・ADR-0700の「ヘッダ不要」と食い違う
  時限爆弾だった→ `requiresHeaderCheck` にpathを渡し完全一致だけ例外にする修正+テーブル駆動テスト4本を追加。
  (2) `services/gateway/README.md` のルーティング表がrecord/team/balance/speed/judge抜けの古いままだった
  →5サービス分の行・環境変数を追加。軽微2件(ingress.yaml・manifest_test.goの「独自Ingress」コメントに
  補足、`main_test.go` の `TestEnvNames`/`TestLoadConfig`/`TestLoadConfigRejects` にrecord/team/balance/
  speed/judgeの5URLを追加してURL取り違えmutationのすり抜けを閉じた)も反映。critic 2回目レビュー予定。
  タイプバランス・素早さ・判定レーンへ、直結Ingressを撤去できる旨を連絡予定
- [x] issue #110(セキュリティ。Codex レビュー)の API レーン担当分: `POST /api/calc/bulk`・`/api/calc/reverse` の候補・観測配列に件数上限が無く、1MiB未満の小さな本文で計算量を増幅できた(2,000×2,000 で約9.4秒)。契約(`maxItems`/`uniqueItems`/`maximum`。ADR-0208)を追加し、calc-svc の生成ラッパは検証しないため(実測確認済み)自前検証をID解決・engine呼び出しより前に実装。critic PASS、実HTTPで境界値と再現手順の解消(0.9ms・engine未到達)を確認。engine/wasmapi(データレーン)・Web・iOSへの追従は DECISIONS.md に既定案付きで依頼(issue はレーンの完了までクローズしない)
- [x] issue #110 のデータレーン担当分: `engine.CalcBulk`/`CalcReverse` と `engine/wasmapi` に ADR-0208 §1 と同じ件数・範囲の上限(presets 8・itemVariants 64・itemCandidates 64・observations 16・maxCandidates 0..128)を追加(ADR-0108)。HTTP を経由しない直接呼び出し・WASM でも計算量を増幅できないようにした。wasmapi は DTO 変換より前に同じ検査を重ねて置き、複数の違反が重なっても HTTP と同じ `invalid_input` が先に出るようにした(parity)。`MaxCandidates` の負の値は、従来「無制限」扱いだったのを ADR-0208 の契約(`minimum: 0`)に合わせて拒否するよう変更(既存テストの期待値を更新。理由は ADR-0108 決定4)。critic PASS(1往復)。Web・iOS の追従(観測16件でUI無効化・持ち物候補64件超の扱い)は ADR-0208 §4 のまま未着手
- [x] issue #148(クラウド公開前のアクセス境界・認証方針。ユーザー決定「私設サービスを維持する」)の API レーン担当分: `deploy/k8s/overlays/cloud` から gateway の Ingress を削除 patch で除去し、public Ingress/LoadBalancer/NodePort/externalIPs/hostNetwork/hostPort が無いことを構造検査+`kubectl kustomize`実描画検査の2層で固定(ADR-0210)。TLS 終端は gateway/クラスタの Ingress では行わず Tailscale(`tailscale serve`)に任せる方針を決定。端末IDが認証として機能しないこと・CORSが到達制御でないことの回帰テストを追加(`TestDeviceIDIsNotAuthentication`・`TestCORSIsNotAccessControl`・`TestContractHasNoAuthentication`)。`base`のgateway Ingress本体は local(k3d)専用として残し、先頭コメントで明記。ADR-0209 §1(クラウド公開へ進む判断)は「公開しない」で確定した旨を追記。critic PASS。運用(tailnet ACL・失効手順のrunbook)・Web/iOS(接続先をtailnet名に)への依頼はDECISIONS.mdに既定案付きで記録(issue はレーンの完了までクローズしない)
- [x] issue #106(データ・運用レーン。Codex レビュー)手動 import Job(`make import-k8s`)と定期 CronJob が同時実行できる問題: `concurrencyPolicy: Forbid` は同じ CronJob が作る Job 同士にしか効かず、`kubectl create job --from=cronjob/...` が作る独立した手動 Job とは排他しないため、共有 PVC(`pokedex-import-cache`)上の取得キャッシュ・DB 投入が競合しうる実バグだった。`tools/importer/cronjob.sh` に busybox の `flock`(非ブロッキング)を `fetch.mjs` 呼び出しより前に追加し、取得〜投入の全工程をアプリ側で排他(ADR-0109)。ロック取得失敗は既存の終了コード規約どおり終了コード1(再試行可能)にし、`cronjob-import.yaml`(podFailurePolicy・concurrencyPolicy とも既存のまま)・`services/pokedex/cmd/import`(Go CLI)・Makefile は無変更。2プロセス同時起動の統合テスト(`cronjob_lock_test.go`)を追加し、Docker(Linux・busybox flock)で実際にロックが機能することを確認済み(macOS はローカルに flock が無いため自動 Skip)。critic PASS。k3d での手動確認手順は docs/runbooks/data.md §6 に追記し、2026-09-23 に実クラスタで実施: 2つの手動 Job を同時作成し、片方が「別の import が実行中」のログで即座に終了コード1、`backoffLimit` の再試行で成功したことを確認(秘密は出力に含まれない)
- [x] issue #104(データ・運用レーン。Codex レビュー)pokedex の DB 資格情報を用途別の最小権限へ分離する: server(検索API)・importer(CronJob)・migrate(Job)がすべて root 相当の同じ資格情報(Secret `mysql-auth`/`pokedex-dsn`)を使っており、公開 HTTP Pod が侵害されると DDL・ユーザー管理まで可能だった実リスクだった。`pokedex_reader`(SELECT専用)・`pokedex_importer`(SELECT/INSERT/UPDATE/DELETE)・`pokedex_migrator`(+ CREATE/ALTER/DROP/INDEX/REFERENCES)の3ロールを作り、server/importer/migrateそれぞれに最小限のDSNだけを渡す(ADR-0110)。`services/pokedex/db.Provision`が冪等・ローテーション対応で3ユーザーを作成・GRANT(接続前に正規表現でパスワード・ユーザー名・DB名・権限を検証し、root自身を対象にする入力は拒否)。`services/pokedex/cmd/migrate`の`up`は`POKEDEX_PROVISION_DSN`があるときだけプロビジョニングしてから実際のmigrationを行う(無ければ後方互換で直接migration。ローカルmake dev/make test-dbは対象外)。`scripts/up.sh`は新規クラスタで4DSNを一度に作成、既存クラスタは無いキーだけ`kubectl patch`で追記(値をargv/ログに出さない設計に修正)。critic PASS(1往復。指摘は軽微5件、うちroot保護の抜け穴を塞ぐテスト追加・check-publishable.shの過剰な許可パターン修正・up.shのエラー握り潰し修正の3件を反映)。**実クラスタ(k3d)で実際に`make up`を実行し、`SHOW GRANTS`で3ユーザーの権限がADR決定1と過不足なく一致することを確認済み**。既存クラスタからの無停止移行(Secretへのキー追記のみ)も実地確認済み
- [x] issue #109(データ・運用レーン。Codex レビュー)pokedex HTTPサーバーにタイムアウトとgraceful shutdownを追加する: `services/pokedex/cmd/pokedex/main.go`の`runServe`が`http.ListenAndServe`を直接呼ぶだけでタイムアウト(ReadHeaderTimeout等)を一切設定せず、SIGINT/SIGTERMも購読しないため、遅い・不完全な接続がリソースを無期限に保持し、Kubernetesのrollout・node drainで処理中リクエストが即座に打ち切られていた実リスクだった(calc/gateway/balance/judgeは既に対応済みでpokedexだけが欠けていた)。`services/balance`と同じ値(readHeaderTimeout=5s・readTimeout=10s・writeTimeout=15s・idleTimeout=60s・maxHeaderBytes=16KiB・shutdownTimeout=10s)で`newHTTPServer`/`serve`/`runServe`の3層に分離(ADR-0111)。既存の`run(args) int`(サブコマンド振り分け)との名前衝突を`runServe`への改名と`runServeCmd`の新設で解消。`deployment.yaml`に`terminationGracePeriodSeconds: 30`を追加し、main.goの`shutdownTimeout`定数より長いことをハードコードせず不等式でmanifestテストに固定。critic PASS(1往復。指摘なし)。ヘッダ未完了接続の切断(生TCP接続で実測)・shutdown中のin-flightリクエスト完了・DSN非露出をすべてテストで固定し、`-race`・`-count=3`でも安定を確認。**実クラスタ(k3d)でpokedexを再ビルド・再デプロイし、`terminationGracePeriodSeconds`が実際に30になっていること・`api-smoke`が正常応答することを確認済み**
- [x] issue #112(データ・運用レーン。Codex レビュー)pokedexのDB接続プールに上限と寿命を設定する: `services/pokedex/cmd/pokedex/main.go`が`sql.Open`後に`SetMaxOpenConns`等を一度も呼ばず(Go標準の既定は無制限)、突発的な同時要求がそのままMySQL接続数に転嫁され、1 Podでも接続枠を占有しうる実リスクだった。4環境変数(`POKEDEX_DB_MAX_OPEN_CONNS`=10・`POKEDEX_DB_MAX_IDLE_CONNS`=5・`POKEDEX_DB_CONN_MAX_IDLE_TIME`=5m・`POKEDEX_DB_CONN_MAX_LIFETIME`=30m)を追加し、`services/pokedex/db.OpenPool`(プール生成を1か所に集約。P7-1のメトリクス化に備える)経由で適用(ADR-0112)。検証(open/idleは正の整数・idle<=open・durationは正値)は`sql.Open`より前、エラー文にDSNを含めない。`export`サブコマンドは`(PoolConfig).ForExport()`で`MaxOpenConns=1`に上書き(逐次処理の実態に合わせる)。`deployment.yaml`に4環境変数を既定値のまま明示し、`docs/runbooks/data.md`にreplica数を増やすときの接続予算の注記を追加。critic PASS(1往復。軽微指摘1件〈idle==openの境界値テスト追加〉を反映)。実MySQLで同時クエリがMaxOpenConnsを超えないこと(直列化の実測込み)を確認。**実クラスタ(k3d)でpokedexを再ビルド・再デプロイし、4環境変数が実際に設定されていること・`api-smoke`が正常応答することを確認済み**
- [x] issue #69(データ/APIレーン)技・持ち物検索の並びがOpenAPI契約と一致しない: `api/openapi.yaml` の `searchMoves`/`searchItems` の description が「並びは ID 順」としていたが、`services/pokedex/db/query/pokedex.sql` の `SearchMoves`/`SearchItems` は導入時(P2-3)から一貫して `ORDER BY <table>.name_ja, <table>.id`(日本語名の照合順序が正。ADR-0105 §3 に「技・持ち物は name_ja, id」と既に明記されており、SQL 側もこれに一致していた)。つまり誤っていたのは契約の説明文だけで、SQL・ADR は無変更(新規 ADR は不要)。`searchSpecies`(`dex_no, form`。SpeciesKey が固定幅ゼロ埋めのためこれは文字列としての ID 順と一致)と `listNatures`(`ORDER BY id`)は元から契約どおりで対象外。契約の description を実態(名前順・同順位は ID)に訂正して `make gen`・`make ios-gen`(絶対ルール1。iOS 生成物は getMove〈P3-7〉分も含めて追従していなかったため合わせて解消)。DB 層(`db.TestSearchMovesAndItemsOrderIsNameJaNotID`。実 MySQL で確認、`-tags mysql`)と httpapi 層(`TestSearchMovesAndItemsPreserveGivenOrderAndLimitCutsThatOrder`。ハンドラが並べ替えず、`limit` がその並びの先頭から切ることを固定。ID 順に並べ替えてから切ると集合自体が変わることを変異テストで確認済み)の両方にテストを追加。critic PASS(1往復)
- [x] issue #73(API/データレーン)OpenAPIとengineの防御プリセット集合を同期検査する: `api/openapi.yaml` の `DefenderPreset` enum と `engine.DefenderPresetCatalog()` は1対1対応が前提(`services/calc/internal/httpapi/convert.go` の `presetKeysFrom` は変換テーブルを持たず契約の列挙値をそのまま `engine.PresetKey` に型変換するだけ)だが、それを固定するテストが無かった。`services/calc/internal/httpapi/preset_sync_test.go`(`TestDefenderPresetEnumMatchesEngineCatalog`)を追加: ハードコードした一覧同士を比較する既存の `vocabulary_test.go` の流儀ではなく、契約(埋め込まれた spec。`loadContract` 経由の kin-openapi)から `DefenderPreset` の enum を直接読み、`engine.DefenderPresetCatalog()` のキー集合・順序(契約の description が「耐久が上がる順」と明記。ADR-0009 §1)と比較する(ハードコードした一覧は「足し忘れ」自体を検出できないため避けた)。変異テストで両方向(engineだけに追加・openapi.yamlだけから削除・件数一致のまま重複させて列としてだけ崩す)を実際に検知することを確認済み(確認後 revert)。critic 1回目FAIL(件数不一致を黙ってskipすると重複を見逃す穴。修正: 件数不一致を明示的な失敗にしてから列を比較)→ 修正 → 2回目相当でPASS
- [x] issue #113(Web/iOS/APIレーン)入力変更時の古い計算要求を抑止・キャンセルする、のAPIレーン連携分(「クライアントのcancel伝播」): Web/iOSは自レーン分(200ms debounce・AbortSignal/Task cancel)を完了済み(PR #174・iOS側コミット)だったが、実測で調べたところ gateway 側に見落としがあった。クライアントが要求を中断すると Go の `http.Server` が `r.Context()` を `context.Canceled` で終えるが、`services/gateway/internal/httpapi/proxy.go` の `ReverseProxy.ErrorHandler` はこれを区別せず「上流に到達できない」WARN ログを出し 503 `upstream_unavailable` を返していた(実際は上流もgatewayも正常で、クライアントが単に離脱しただけ)。`errors.Is(err, context.Canceled)` のときだけ特別扱いし、WARN ログを出さず(Debug に留める)応答も書かない(相手は既に居ない)ように修正。自前のタイムアウト(`net/http: timeout awaiting response headers`)は別のエラー文言になるため混同しないことを実測で確認。`TestClientCancelIsNotUpstreamUnavailable`(wall-clock sleep 不使用。フェイクRoundTripper版・実`http.Transport`版の両方)を追加、変異テストで実効性を確認。ADR-0202 §5 に追記(AC-G10)。**限界**: gateway→calc-svcへのcontextキャンセル伝播自体は効くが、calc-svcのハンドラ・engineはcontextを見ない(engineを純粋に保つ絶対ルール2)ため、issue本文の「calc-svc CPU消費も止める」は本修正の範囲では未達成(中断された逆算は完走する。ADR-0208の上限で最悪計算量は有界なので実害は限定的。詳細はDECISIONS.md)。issue本文の受け入れ条件・対象範囲はいずれもWeb/iOS固有かgateway/calcのtimeout値等を明示的に除外しており、この限界を残したままissue #113はWeb/iOS/APIすべてのレーン分が完了としてクローズ可
- [x] P4-17(Web/APIレーン)技のID解決の欠落を解消(ADR-0304 §3): `GET /api/pokedex/moves/batch?ids=...`(`getMovesByIds`)を新設。ADR-0304が当初推していた案A(`getSpecies.learnset`をID配列からMove実体配列に変える)は採らなかった。理由: iOS(M3)が`SpeciesDetail.learnset`を`string[]`のまま前提にした機能(CalcViewModel・ReverseViewModel・TeamEditViewModelがlearnsetをID集合として扱い、検索結果との積集合を取る)を既に出荷済みで、案Aはその完成済み機能を壊す破壊的変更になり「契約変更が小さい方」の基準に反すると判断。新設したエンドポイントは既存のlearnsetを無変更のまま、1回の呼び出しでIDの配列をMove実体の配列に解決する。ids 1〜64件(ADR-0208の前例。生成ラッパは配列のmaxItemsを検証しないため`services/pokedex/internal/httpapi/search.go`で自前検査)、見つからないIDは黙って省く、応答順はidsと同じ(DBのIN句は順序を保証しないためハンドラで並べ替え)、`getMove`と同様に既定のレギュレーションで絞らない。ルーティング(`/moves/batch`が`/moves/:key`に食われないこと)を含めテスト済み(`TestGetMovesByIds`)。ADR-0105 §3・ADR-0304 §3に追記。**ids 64件超の実データ確認はまだ行っていない**(1種族のlearnsetが64件を超える場合はWeb側で分割呼び出しが必要。詳細はADR-0304 §3・DECISIONS.md)。critic 1回目FAIL(コメント・ドキュメントの事実誤り3件。実装・テストへの指摘なし)→ 修正・推奨事項も反映 → 2回目FAIL(1: 64件で1回に収まるという未検証の約束をしていた。実データ確認できず〈k3dクラスタ停止中〉、分割呼び出し前提に書き換えて対応 2: calc-svcのエラーコードのコメントが実際と不一致〈missing_headerが正・invalid_inputは誤り〉 3: 契約のmaxItemsとGo定数64の同期テストが無かった。`api.GetSwagger()`から契約のmaxItemsを読んで期待値にする`contractQueryParamMaxItems`ヘルパーを追加し、変異テスト〈契約だけ32に変更〉で同期の実効性を確認)→ 修正済み → **3回目PASS**(重要2件を反映してcommit: Web欄のP4-17の行が「未回答」のまま古くなっていたのを解決済みに更新、`TestGetMovesByIds`に未検証だった3つの契約どおりの挙動〈マスタ未投入→200 []・重複ids→重複したまま返る・空要素は黙って省く〉のテストを追加。軽微なコメントの言い回しの訂正も反映)。Webレーンへ実装完了を連絡(learnsetの解決に使ってP4-17の技オンライン未対応を解消できる。64件超は分割呼び出しが必要である旨も伝える)
- MySQL の manifest に MYSQL_DATABASE が無く、初回起動時に pokedex DB が自動作成されない実バグを発見(データレーンが k3d に初めて実デプロイした際に発生)。deploy/k8s/overlays/local/mysql/statefulset.yaml に MYSQL_DATABASE: pokedex を追加し、layout_test.go に検知テストを追加して修正(2026-09-22)。**新規クラスタでは直るが、この修正前にすでに初期化済みの PVC は MYSQL_DATABASE の効果を受けない**(コンテナ起動時にしか実行されない仕様のため)。既存の PVC に対しては CREATE DATABASE を手動実行するしかない。docs/runbooks/data.md に一言注記するとよい
- P2-3 の critic の軽微(2026-09-22。4件。#74): 未反映は `check-publishable.sh` の `B_KEYVALUE_ALLOW` を self-test の基準リポジトリにも播く、の1件。
  pokedex 側の3件は対応済み(2026-09-25): readmodel の `maxCatalogAbilityCount` を balance の schema の maxItems と直接比べる同期テスト(balance の loader 側の定数はタイプバランスレーン) /
  nature-mismatch の Blocker の Detail に `make import-fetch` を含む復旧案内 / `TestPublicInputValidation` の 400 応答を契約検証(kin-openapi)に通す

- P2-2d の critic の軽微(2026-09-22): `cronjob_layout_test.go` の「消さない」検査を secret・statefulset・configmap にも広げる / `make lint` が kubectl に依存する(kubectl の無い環境では失敗する)/ upstream の `checkedAt` が未来でも fresh 扱い / **コンテナの中で取得スクリプト(Showdown の build 等)を実際に流した記録が無い。初回の `make import-k8s` で確かめる**

- [x] issue #76(データレーン)`services/pokedex/db/mysql_test.go` に「species_abilities.slot = 4 が入る」ことを確かめるケースを足す(P2-2c の critic の軽微。000005 は使い捨てコンテナで手動確認済みだったが自動テストが無かった): `TestConstraintsRejectInvalidRows` のスロット5拒否・特性重複拒否は負方向だけだったため、`TestSpeciesAbilitiesSlot4RoundTrip` を追加し slot 4 への挿入成功と読み戻し(`SELECT ... ORDER BY slot`)を固定。確認・ロールバックはトランザクション内(コミットして残すと、他テストの `freshDB` が呼ぶ `DownAll` が migration 000005 の down〈CHECK を 1..3 へ戻す〉で失敗するため)。migration 000005 の CHECK を一時的に `(1,2,3)` に戻して新テストが失敗することを確認した上で revert(退行検知の実効性を確認)。`-tags mysql`(使い捨て MySQL コンテナ、pin 済み `mysql:9.7.2`)・`make test`・`make lint` 成功

- [x] `scripts/check-publishable.sh --self-test` の既存の失敗2件を MT-2 で修正し、`make lint` に自己テストを追加(2026-09-22)

P1-6 独立レビューで出た軽微・任意の指摘(コードは未変更。次の engine タスクに合わせて対応を検討):
- `engine/golden_test.go`: `speciesCount` の下限アサート追加(現在は 0 だけ検査。少数種で再生成しても通ってしまう)
- `engine/golden_test.go`: `DamageInput` の json タグ明示または `DisallowUnknownFields`(フィールド改名でフィクスチャ値が黙ってゼロ値になる)
- [x] `Makefile`: `go vet -tags golden` を lint に追加 / `golden-generate` は説明どおり `npm ci` を実行するか未導入で明示的に失敗させる(#77。vet は allspecies も。`golden-generate` は `npm ci` してから生成)
- `tools/golden/package.json`: `^0.10.0` を `0.10.0` に完全固定
- [x] `engine/damage.go` `chainMods`: @smogon/calc はクランプ(41/410〜131072/2097152)を持つ。現在の補正集合では到達しないが、補正追加時に再確認(#77。同じクランプを実装し境界テストを追加)
- ゴールデン未カバー: リフレクターとオーロラベールの同時成立、`Effectiveness` / `STAB` の直接照合(L1 では確認済み。壁の同時成立は #77 で L1 の回帰テストを追加)
- [x] issue #276(担当: タイプバランス・Web。ADR-0411): API 専用の画面(タイプバランス・判定)は計算モードに関係なくオンラインのマスタを使う
      (`web/src/app/withOnlineMaster.tsx`。読めなければ日本語の案内と再試行)。balance のエラーはコードを日本語の文言に写像し、
      英語の message を出さない(`balanceErrorText`)。ヘッダーの切替の名前を「ダメージ計算の実行場所」に変更。契約・生成物の変更なし。
      判定のエラー補助行(サーバー message)は未対応(別 issue 候補)
- [x] issue #316・#245(APIレーン。ADR-0200 §4 追記): (#316) calc-svc が契約で必須の `sp`(と StatBlock の6キー)の欠落を 400 `invalid_input` にする
      (calc の attacker・defender、bulk の attacker、reverse の known。`decodeStrict` が生の JSON でキーの有無を確かめる。judge と同じ方式、生成型は不変)。
      (#245) `Individual.moveId` を契約から削除(Web は参照なし、iOS は同じ PR で追従。`attacker.moveId` は `unknown_field`)、pokedex の searchSpecies・getSpecies・searchMoves・searchItems に
      `'400'` を明記、`services/internal/api/cfg.yaml` の `strict-server: false`(StrictServerInterface は未使用。生成差分のみ)。`make gen`・`make ios-gen` 済み。
      Web・iOS レーンへの連絡は DECISIONS.md

- [x] issue #322(担当: API。ADR-0204 追記): calc-svc のマスタ本文の上限を 16MiB から 4MiB に下げた
      (`master.MaxExportBytes`。実マスタは見積り 0.5〜1MB で数倍の余裕)。上限ちょうど・+1(ErrInvalidMaster)のテストと、
      「基礎 32MiB + 4 × 上限 ≤ limits.memory の 8 割」を固定する `TestMasterBodyLimitFitsMemoryLimit` を追加。契約・生成物の変更なし。
      実マスタの export の実測(pokedex 起動が必要)と Linux コンテナでの RSS は未実施

- [x] issue #271/#270
- [x] issue #274/#272
- [x] issue #272 の Web 分
- [x] issue #274 の Web 分
- [x] issue #274/#272 の API レーン担当分の残り
- [x] issue #284
- [x] issue #110
- [x] issue #110 のデータレーン担当分
- [x] issue #148
- [x] issue #106
- [x] issue #104
- [x] issue #109
- [x] issue #112
- [x] issue #69(データ/APIレーン)技・持ち物検索の並びがOpenAPI契約と一致しない
- [x] issue #73(API/データレーン)OpenAPIとengineの防御プリセット集合を同期検査する
- [x] issue #113(Web/iOS/APIレーン)入力変更時の古い計算要求を抑止・キャンセルする、のAPIレーン連携分(「クライアントのcancel伝播」)
- [x] P4-17(Web/APIレーン)技のID解決の欠落を解消(ADR-0304 §3)
- [x] issue #276
- [x] issue #211(API レーン分。ADR-0218): 公開 API の Item / Ability に省略可の `effect` を足した(searchItems・getSpecies.abilities。共通マスタで厳格に検証し、不正は 503 master_unavailable。内部 API は変更なし。critic PASS)

- [x] issue #236 の balance 分(ADR-0413。X-Device-Id/X-Session-Id を gateway と同じ正準 UUID 検証に。openapi 0.8.0)
- [x] issue #210 の Web 分(ADR-0313): 既定の計算モードをオンラインに変更(ユーザー決定 2026-10-01。保存済みのモードは尊重)。
  オンラインで取得した持ち物・性格と、解決した種族・特性・技を IndexedDB に保存(`master/cache/`。`MasterCacheStore`・
  スキーマ版つき。書き込み・読み出しの失敗は握りつぶす)し、オフラインはそのキャッシュだけから読む(オンラインを呼ばない・
  架空データを出さない。空・壊れ・版違いは `appText.masterCacheEmptyError` の案内+再試行)。`main.tsx` は例データをやめて
  `createCachedMasterSources` に差し替え。オフラインでは持ち物の候補比較は選べない(公開 API に効果データが無い)。
  実装中に見つけた退行も直した: 攻守入れ替えで種族の検索欄の名前が追従しない(`SpeciesSearchField.selectedNameJa`)、
  種族の解決待ちの間に打った逆算の観測が計算に反映されない(`ReverseScreen` の `latestObservationsRef`)。
  E2E は pokedex フィクスチャでオンライン→オフラインを確かめる(コンテナは CSP の下の WASM 計算を `container.spec.ts` だけで確認)
- [ ] issue #274/#272 の API レーン担当分の残り: `defenderOverride.ranks: RankBlock` / `defenderOverride.status:
  StatusCondition`(全行に一律で上書き)。abilityId(上記)とは独立に追加できる。engine 側の変更
  (`BulkInput`/`ReverseInput` へのオーバーライド追加。プリセット解決後・計算前に当てる)を伴うため
  ADR-0003 の test-first + 独立 critic の対象。優先度は低い(iOS レーンから「急ぎではない」と明記済み)
- [x] issue #236 の judge 分(speed は #360/ADR-0606、balance は PR #458/ADR-0413)端末ID・セッションIDの検証を gateway と同じ正準 UUID に揃えた(ADR-0219)
- [x] issue #328 の Web 分(ADR-0314): アプリ下部のフッター(`<footer>`、main の外)に「このアプリについて」リンクを置き、
  `/about`(ADR-0300 §1 のパス連動。タブには入れない)で非公式の注記とデータの出典4件を出す。文言は `aboutText`
  (iOS の `AboutText` と一字一句同じ)、画面は `AboutScreen.tsx`。タブ列は出さず他タブは hidden で DOM に残す(入力を保つ)。
  axe(`@axe-core/playwright` 4.13.0)の検査を `e2e/a11y-about.spec.ts` に追加。マスタ・engine は使わない
- [x] issue #274 の Web 分の残り(ADR-0315): 「詳細」に防御側のランク(選択中の技の分類で B か D を ±1、-6..+6、def / spd は別保持)を追加。
  触った分だけ `defenderOverride.ranks`(5項目)を要求に載せ(既定は従来とバイト同一)、API は特性(#272)と同じ `defenderOverride` に合成、
  WASM は素通し(特性は従来どおり `defenderAbilities`)。条件の置き場は `domain/calcConditions.ts`。防御側の状態異常は式に効かないので出さない。iOS は別レーン
  - [x] **P5-5c よく計算する相手(チップ。ADR-0317)**: recordClient(`web/src/record/`)・CalcScreen の結果の下のチップ(マウント時1回取得・失敗/0件は黙って非表示)・App はオンラインのときだけ接続・`SpeciesSearchField` に任意 prop `selectedName`。履歴一覧(API 無し)は対象外
  - [ ] **P5-5d 端末データの削除 UI**: record-svc の API と ADR-0209 §8 の文言
