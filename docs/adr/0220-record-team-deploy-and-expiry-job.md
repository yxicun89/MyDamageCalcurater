# ADR-0220: record-svc / team-svc の k3d 配線と失効ジョブ(P5-3b・P5-4b)

- 状態: 採用(critic PASS。2026-10-02)
- 日付: 2026-10-02
- 関連: ADR-0209(保持期間・失効ジョブ・墓石)、ADR-0211(TiDB・資格情報・環境変数名・`orphaned_since`)、
  ADR-0212(JetStream `max_age` 7日)、ADR-0213(team-svc)、ADR-0202(gateway)、ADR-0104(pokedex-import の CronJob)、
  ADR-0129 §4(preStop と terminationGracePeriodSeconds)、ADR-0132(NetworkPolicy)、ADR-0406(/metrics と ServiceMonitor)、
  issue #298(GOMEMLIMIT)、plan.md P5-3b・P5-4b

## 背景

P5-3 / P5-4 で record-svc・team-svc の本体(HTTP・JetStream の購読・全削除 API)はできたが、
(1) k3d に Deployment / Service が無く gateway の `GATEWAY_RECORD_URL` / `GATEWAY_TEAM_URL` も未設定のため
`/api/record/*`・`/api/team/*` は常に 503 `upstream_unavailable`、(2) ADR-0209 §4 の失効ジョブが無い
(保持期間の環境変数は起動時検証にしか使われていない)。この ADR はその2つの形を決める。

## 決定

### 1. 配置(deploy/k8s/base/record・team)

pokedex・calc・gateway の base と同じ流儀にする。

| ファイル | 中身 |
|---|---|
| `deployment.yaml` | `record`(team は `team`)。image `pokecalc/record:0.1.0`(server ターゲット)・`args: ["serve"]`・port `http` 8080・readiness `/readyz`・liveness `/healthz`・`automountServiceAccountToken: false`・Pod/コンテナの securityContext(非 root 65532・RuntimeDefault・readOnlyRootFilesystem・drop ALL)・requests/limits・`GOMEMLIMIT`(limits.memory の 75% 以上・未満)・preStop sleep と terminationGracePeriodSeconds(preStop + shutdownTimeout より長い) |
| `service.yaml` | ClusterIP・`http` 80 → `http`。Ingress は作らない(公開は gateway 経由) |
| `configmap-retention.yaml` | `record-retention`(team は `team-retention`)。ADR-0211 §7 の日数だけを持つ。Deployment と CronJob の両方が `envFrom.configMapRef` で読む(同じ値を2か所に書かない) |
| `cronjob-expire.yaml` | 失効ジョブ(§3) |
| `kustomization.yaml` | 上の4つ。**`job-migrate.yaml` は入れない**(ADR-0211 §3.2 のとおり up.sh が TidbInitializer の完了後に個別 apply する) |

- DSN は Secret `record-db-auth` の `record-app-dsn`(team は `team-db-auth` の `team-app-dsn`)を
  `secretKeyRef` で渡す(app ロール = `SELECT, INSERT, UPDATE, DELETE`。失効ジョブも同じロール。ADR-0211 §4)。
  record の Pod は `team-db-auth`・`tidb-root-auth` を参照しない(逆も同じ)。平文の DSN をマニフェストに書かない。
- Secret が無い・TiDB が無いときは record/team の Pod が Ready にならないだけで、calc・gateway・pokedex には影響しない
  (gateway は `/api/record/*` を 503 `upstream_unavailable` にするだけ。CLAUDE.md 絶対ルール5)。
  up.sh は record/team の Ready を待たない(TiDB の導入失敗が非致命なのと揃える)。
- `RECORD_NATS_URL` / `TEAM_NATS_URL` は `nats://nats:4222`(calc と同じ)。NATS が落ちていても起動する(P5-3/P5-4 の実装どおり)。
- 半減期 `RECORD_DECAY_HALF_LIFE_DAYS` は 14(ADR-0209 §4 の仮置き)、`*_PURGE_BATCH_LIMIT` は 500、
  `*_EXPIRE_BATCH_LIMIT` は 1000 を manifest の値とする(調整は manifest だけで行う)。
- base の `kustomization.yaml` に `record`・`team` を足す。gateway の base に
  `GATEWAY_RECORD_URL=http://record`・`GATEWAY_TEAM_URL=http://team` を足す(Service 名はクラウドでも同じ。ADR-0206 §1 と同じ理由)。
- cloud overlay には TiDB も Secret も無いので、`record-expire`・`team-expire` の CronJob を `suspend: true` にする
  (pokedex-import と同じ。ADR-0104 §8)。Deployment は suspend できないが、Secret が無ければ Ready にならないだけで害は無く、
  cloud は使わない方針(ADR-0210)なので CronJob だけを止める(毎日失敗し続けないため)。
- **local overlay も `suspend: true`**(`deploy/k8s/overlays/local/cronjob-expire-suspend-patch.yaml`)。ADR-0209 が「人間の確認が必要」と
  する「失効ジョブを本番データに初めて向けるとき」(と CLAUDE.md の DB データ削除)の承認が済むまで、自動で実データを消さない既定案。
  承認後は local の patch ファイルと kustomization の参照を外す(cloud は TiDB 導入まで suspend のまま)。
  承認前の動作確認・手動実行は `kubectl -n pokecalc create job --from=cronjob/record-expire record-expire-manual-$(date +%s)`(team も同じ)で行える
  (suspend は定期実行だけを止め、手動 Job は作れる)。
- `scripts/up.sh` は record・team の server イメージ(`--target server`)を build して k3d に import する。

### 2. NetworkPolicy と監視

- `allow-gateway-upstream` に `record`・`team` を足す(gateway → 8080)。
- `allow-tidb-client-ingress` に `record`・`team`・`record-expire`・`team-expire` を足す(→ TiDB 4000)。
- `allow-nats-ingress` に `record`・`team` を足す(購読。→ 4222)。
- `allow-prometheus-metrics` に `record`・`team` を足し、record/team の HTTP に `/metrics`(`services/internal/httpmetrics`。
  pokedex・calc・gateway と同じ)を付け、`deploy/k8s/base/observability/servicemonitors/{record,team}.yaml` を足す。
- 拒否を明示的に検査する: web・calc → record/team、record ↔ team、record/team → mysql、gateway・calc → TiDB。

### 3. 失効ジョブの形: 同じバイナリのサブコマンド `expire` を、同じイメージの CronJob で起動する

- `record serve` / `record expire`(team も同じ)。pokedex の `pokedex serve | export` と同じ形。
  サブコマンドが無い・不明なら使い方を出して終了コード 2。Deployment は `args: ["serve"]`、CronJob は `args: ["expire"]`。
  理由: 保持日数の環境変数の解釈(`parseDays`・ADR-0211 §7 の検証)を serve と共有でき、イメージを増やさない。
  別 cmd(別イメージ)にする案は、同じ設定の解釈を2つの main に複製するか共有パッケージを新設するかになり、得るものが無い。
- `expire` は `RECORD_APP_DSN` と保持日数4つ・`RECORD_EXPIRE_BATCH_LIMIT` だけを読む(`loadExpireConfig`)。
  NATS・半減期・`*_PURGE_BATCH_LIMIT`・待ち受けアドレスは要らない(未設定でも動く)。
  `RECORD_DEVICE_ROW_EXPIRY_DAYS` > JetStream `max_age` 7日の検証は serve と同じく行う。
- 1回起動して、§4 の手順を1巡だけ行って終わる(常駐しない・ループしない)。上限に達したら「残りあり」をログに出して
  **終了コード 0**(次回が続きを消す。ADR-0209 §4)。DB に届かない等の失敗と設定の誤り(環境変数の欠落・不正。DB に触れる前に検出)は終了コード 1、使い方の誤り(サブコマンド無し・不明)は 2。
- CronJob: 名前と Pod ラベル `record-expire` / `team-expire`、`schedule` は日次(k3d のノート PC が起動している時間帯。
  `timeZone: Asia/Tokyo`)、`concurrencyPolicy: Forbid`、`startingDeadlineSeconds` は1日未満、`backoffLimit`・
  `activeDeadlineSeconds`・`ttlSecondsAfterFinished` を持ち、`podFailurePolicy` で終了コード 2 を再試行しない(`expire` 自体は 2 を返さないので保険。引数の誤りへの備え)。
  securityContext・resources は Deployment と同じ水準。
- 失効の SQL は「全端末を横断する」ので、`internal/store` の Store インターフェース(規則: 全メソッドが deviceID を取る)には
  **足さない**。新しいパッケージ `internal/expire` に、失効専用のインターフェース `expire.Store` と TiDB 実装、
  手順を組む `expire.Run` を置く。HTTP・NATS の経路からは使わない(httpapi・events は import しない)。

### 4. 失効の判定(ADR-0209 §3・§4・ADR-0211 §6 を SQL に落とした形)

`now` は1回の実行の開始時刻(UTC)。保持期間 `R` に対し `cutoff = now − R` とし、**`ts < cutoff`(経過時間 > R)の行だけを消す**。
ちょうど R の行は残る(AC-R1・AC-R2 の「ちょうどは残る」)。

record(順序どおり。1〜4・6 は1回の実行で共有する削除の上限 `RECORD_EXPIRE_BATCH_LIMIT` を順に使う):

1. `calc_events`: `occurred_at < now − CALC_EVENTS_RETENTION`。
   「作成から90日」の「作成」は calc-svc がイベントに載せる `occurred_at` と読む(受信時刻 `created_at` ではない。
   墓石の判定〈ADR-0209 §7〉・集計の `last_calculated_at` と同じ時計に揃えるため。索引 `(device_id, occurred_at)` もこれに合う)。
2. `frequent_opponents`(集計 #2): `last_calculated_at < now − CALC_EVENTS_RETENTION`。
   `last_calculated_at` はその (端末, 種族) に寄与したイベントの `occurred_at` の最大値なので、この条件は
   「その (端末, 種族) の生イベントが1件も残っていない」と同値になる(AC-R6: 生イベントが消えたあとに集計だけが残らない)。
   一部のイベントだけが失効した (端末, 種族) の `count` は再計算しない(§未決 1)。
3. `favorites`: `GREATEST(COALESCE(devices.last_seen_at, favorites.updated_at), favorites.updated_at) < now − FAVORITES_RETENTION`
   (`devices` 行が無い端末は行の `updated_at` だけで判定する)。
4. `purge_journal`: `requested_at < now − PURGE_JOURNAL_RETENTION`。
5. `devices.orphaned_since` の更新(削除の上限とは別に、同じ値を上限にする):
   業務テーブル(`calc_events`・`frequent_opponents`・`favorites`)に1行も無く `orphaned_since IS NULL` の端末に `orphaned_since = now`、
   `orphaned_since IS NOT NULL` で業務テーブルに行がある端末は `NULL` に戻す(ADR-0211 §6)。
6. `devices` 行の削除: 次を**すべて**満たす行だけ(`DEVICE_ROW_EXPIRY` を D とする):
   - `orphaned_since < now − D`(データが無くなってから D を超えた。ADR-0209 §3 #5)
   - `last_seen_at < now − D`(使われ続けている端末の行は消さない。#5「使われ続ける端末の行は消えない」)
   - `purged_at IS NULL OR purged_at < now − D`(墓石は削除から D の間は必ず残す。ADR-0209 §7 の猶予 30日 > `max_age` 7日)
   - 削除する瞬間にも業務テーブルに行が無い(`NOT EXISTS` を同じ文で確かめる。5 と 6 の間に作られたデータで行を消さない)
   `purge_journal` は「その端末のデータ」に数えない(独立の90日で消える。#5b)。

team は 3 の代わりに `teams` を `GREATEST(COALESCE(devices.last_seen_at, teams.updated_at), teams.updated_at) < now − TEAM_RETENTION` で消し、
**その構築の `team_members` を先に同じトランザクションで消す**(`team_members` は `updated_at` を持たず、構築の丸ごと置換で
構築の `updated_at` が進むので、構築に従う。ADR-0213 §2)。上限は構築の件数で数える(メンバーは1構築6体まで)。
1・2 は無い。5・6 の業務テーブルは `teams`・`team_members`。team の `last_seen_at` は P5-4 の購読(イベント消費)でも進むので、
計算 API だけを使い続ける端末の構築は消えない(AC-R2d)。

共通:

- **冪等**: 同じ `now` で2回走らせても2回目は何も消えない。途中で落ちても次回が続きを消す(各文は自分だけで完結する)。
- **残りあり**: どこかの段で上限まで消した(または上限が尽きて段を飛ばした)ら `Remaining = true`。保守的に倒す
  (ちょうど上限ぶんで残りが0でも true になりうる。次回が 0 件・false で確かめる)。
- **墓石が立っている端末**(ADR-0209 §5.2「失効ジョブは `purged_at` が立っている端末の行には触らない」)の読み方:
  失効ジョブは墓石・purge journal を**書かず**、集計を**作り直さない**。業務テーブルの期限切れの行は消す
  (全削除 API と同じ「消す」方向の操作で、順序が入れ替わっても結果が変わらない。全削除の後に保存された行も保持期間で消えるべきで、
  「墓石のある端末を永久に対象外にする」と読むと無期限保持になりユーザー決定に反する)。`devices` 行だけは上の 6 の条件で墓石を守る。
- **ログ**: 1回の実行の終わりに構造化ログ1行(`msg="expire done"`、各表の削除件数・`orphans_marked`・`orphans_cleared`・
  `remaining`・所要時間)。端末 ID・行の中身は出さない(件数だけ)。
- **メトリクス**: CronJob の Pod は短命で Prometheus から scrape できず、Pushgateway も無い。失敗は kube-state-metrics の
  Job の状態(`kube_job_status_failed`)で見る。削除件数はログで見る(§未決 2)。
- team の `SELECT ... FOR UPDATE`(§4 の構築の選択)は `devices` 行もロックするため、serve の購読(last_seen_at 更新)と
  デッドロックして Job が失敗しうる。終了コード 1 の再試行(`backoffLimit`)と次回の実行で回復する(削除は冪等)。
- 計算 API への影響なし: `expire` は calc・gateway・NATS に触らない。CronJob が止まっても失敗しても計算は成功する(AC-R4)。

## 受け入れ条件

配置(make test。deploytest と各 cmd の manifest_test):

- **AC-K1** base/record・base/team に Deployment・Service・ConfigMap(retention)・CronJob(expire)があり、kustomization の resources に並ぶ。`job-migrate.yaml` は並ばない。
- **AC-K2** Deployment は §1 の形(args serve・readiness /readyz・liveness /healthz・securityContext・resources・GOMEMLIMIT・preStop と grace)。DSN は自サービスの Secret の app DSN からだけ渡し、他サービスの Secret・`tidb-root-auth` を参照しない。
- **AC-K3** ConfigMap の日数は ADR-0211 §7 の既定値で、Deployment と CronJob が同じ ConfigMap を `envFrom` で読む。その値と Deployment / CronJob の env で `loadConfig` / `loadExpireConfig` が通る。
- **AC-K4** CronJob は §3 の形(同じイメージ・args expire・日次・Forbid・期限・podFailurePolicy・securityContext・resources)。cloud overlay では suspend。
- **AC-K5** gateway の base(と local)で `GATEWAY_RECORD_URL=http://record`・`GATEWAY_TEAM_URL=http://team`。
- **AC-K6** base の kustomization に record・team。up.sh が record・team の server ターゲットを build・import する。
- **AC-K7** NetworkPolicy の許可表・拒否表(§2)。calc の Deployment は record/team/TiDB の Secret・URL を持たない(絶対ルール5)。
- **AC-K8** record・team は `/metrics` を返し、ServiceMonitor がある。

失効ジョブ(make test は偽ストア、make test-db は TiDB):

- **AC-E1** `expire.Run` は各段に `now − 保持期間` をそのまま cutoff として渡す(偽ストア)。
- **AC-E2** 境界: イベント 89日・ちょうど90日は残り、90日+1秒・91日は消える。お気に入り・構築・purge journal も同じ規則(偽ストア + TiDB)。
- **AC-E3** お気に入り・構築は `max(last_seen_at, updated_at)` で判定(AC-R2・R2b。`devices` 行が無い場合を含む。TiDB)。
- **AC-E4** 上限: 上限 3 で期限切れ 5 件 → 1回目 3件・残りあり、2回目 2件・残りなし、3回目 0件(冪等)。上限は段をまたいで共有し、尽きた段は呼ばない。
- **AC-E5** 他端末の行・期限内の行は消えない。集計は生イベントが残っている (端末, 種族) では消えない(AC-R6)。
- **AC-E6** `devices` 行: データが無くなって `orphaned_since` が立ち、D をちょうど経過では残り、超えたら消える。`last_seen_at` が新しい・墓石が D 以内・データが戻った、のどれかなら消えない(データが戻ったら `orphaned_since` は NULL に戻る)。
- **AC-E7** 途中の段で DB の失敗が起きたら `Run` はエラーを返し、後の段を呼ばない(終了コード 1。次回が続きから消す)。
- **AC-E8** 実行の終わりに件数と残りありのログが1行出て、端末 ID を含まない。
- **AC-E9** `expire` パッケージは HTTP・NATS・他サービスのパッケージを import しない(record は team を、team は record を参照しない)。
- **AC-E10** `record expire` / `team expire` サブコマンド: 設定の検証(§3)。サブコマンド無し・不明は 2、設定の誤りは DB に触れる前に 1。

## 却下した案

- **別バイナリ・別イメージ(`cmd/record-expire`)**: §3。設定の解釈が2つに分かれる。
- **serve の中で goroutine として日次に走らせる**: レプリカ数だけ同時に走る・Pod の再起動で時刻がずれる・失敗が見えにくい。
  CronJob なら `concurrencyPolicy: Forbid` と Job の状態でまとめて扱える。
- **`internal/store` の Store に失効のメソッドを足す**: 「全メソッドが deviceID を取る」規則(ADR-0209 §6-1)を崩す。
- **保持日数を Deployment と CronJob にそれぞれ value で書く**: 同じ値の二重管理(coding-rules §2)。ConfigMap 1つにする。
- **墓石が立っている端末を失効ジョブの対象外にする**: §4 のとおり無期限保持になる。

## 未決事項(人間・後続タスク)

0. **k3d の実データへ初めて失効ジョブを向けることの承認**(ADR-0209 の「人間の確認」)。承認までは local も suspend(既定案)。承認後に local の suspend patch を外す。

1. 一部だけ失効した (端末, 種族) の集計 `count` を生イベントから数え直すか(今は数え直さない。`score` は減衰でほぼ寄与しない)。
2. 削除件数をメトリクスとして残すか(Pushgateway の導入、または serve 側に「直近の失効結果」を DB から読む gauge を足す)。
3. CronJob の時刻(既定は日本時間の日中。ノート PC の k3d で取りこぼしたときは `startingDeadlineSeconds` の範囲で追いつく)。
4. `purge_journal` の DB 外の独立保存先(ADR-0211 §6 の既知のギャップ。P7-4)は変わらず未充足。

## 影響

- plan.md P5-3b・P5-4b はこの ADR に従う。ADR-0209 §4「失効ジョブ」・ADR-0211 §6 `orphaned_since` の書き手はこの ADR の §4。
- `services/record/cmd/record/config.go`・`services/team/cmd/team/config.go` の「失効ジョブはまだ無い」旨のコメントは実装時に更新する。
- `scripts/observability-bootstrap_test.sh` の ServiceMonitor 対象は8サービスになる。
