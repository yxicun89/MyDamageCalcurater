# ADR-0211: record-svc / team-svc の TiDB 導入(P5-1)

- 状態: 採用(2026-09-24。critic 3回目 PASS。1回目 FAIL〈全面改訂〉・2回目 FAIL〈残4件〉を経て確定)
- 日付: 2026-09-24
- 関連: ADR-0209(保持・削除・端末 ID 境界。この ADR が定めるスキーマの中身の正)、
  ADR-0100(pokedex の DB 運用。migration・migrate CLI 自作の理由)、ADR-0110(pokedex の最小権限。
  この ADR の migrator/app 分離・資格情報の境界が踏襲する元)、CLAUDE.md 絶対ルール4・5、
  docs/plan.md P5-1(この ADR で確定した範囲に合わせて同時に更新する。「影響」)

## 背景

M2(docs/plan.md P5-1〜P5-4)は record-svc / team-svc を TiDB の上に作る。docs/plan.md:209 は
ローカルを `tiup playground`、k3d を「TiDB Operator 最小構成」と既に方向づけているが、具体的な
バージョン・リソース上限・DB/ユーザーの分け方・migration ツールの流用可否は決めていない。この ADR は
それを決める(P5-1 のスコープはプロビジョニングと `devices`・`purge_journal` の2表までで、
業務テーブル〈`calc_events`・`favorites`・`teams`・`team_members`〉と record-svc / team-svc 自体の
ハンドラ実装は P5-3/P5-4。「影響」で docs/plan.md の記述をこのスコープに合わせる)。

## 決定

### 1. バージョン固定(2026-09-24 時点の最新安定版。ユーザー方針「依存は常に最新安定版を固定」)

| コンポーネント | バージョン | 用途 | 固定するか |
|---|---|---|---|
| TiDB | v8.5.8 | `tiup playground`・TidbCluster の `spec.version` | 固定する |
| TiDB Operator | v1.6.6 | k3d への Operator 本体・CRD | 固定する |
| tiup | 1.17.1 目安 | ローカル開発専用のインストーラ | **固定しない**(`tiup update --self` で開発者が随時追従してよい。tiup 自体は TiDB クラスタの一部ではなく、`playground` サブコマンドに `v8.5.8` を明示して呼ぶため tiup 自身のバージョン差は結果に影響しない) |

TiDB Operator の `main`/v2 系(`v2.2.0-alpha.*`)は 2026-09-24 時点で alpha のみで安定版タグが無いため使わない
(GitHub Releases 確認済み。安定版の最新は v1.6.6)。cert-manager はこのバージョンの Operator では必須ではない
(公式手順が CRD 適用 + `helm install` だけで完結し、cert-manager に触れていないことを確認済み)。

### 2. ローカル開発: `tiup playground`

`make dev`(k8s を使わないローカル起動)向けに、既存の `scripts/db-local-up.sh`(MySQL を docker run する形)と
並ぶ形で `scripts/tidb-local-up.sh` を追加する。

```
tiup playground v8.5.8 --host 127.0.0.1 --db.port 4000 --pd.port 2379 --without-monitor &
```

- `--without-monitor` で Prometheus/Grafana を起動しない(個人開発に不要。起動を軽くする)。
- PD/TiKV/TiDB は既定の1台ずつ(tiup playground の既定そのものが最小構成)。
- 起動後、`scripts/tidb-local-up.sh` は使い捨ての `mysql` クライアントで
  `CREATE DATABASE IF NOT EXISTS record; CREATE DATABASE IF NOT EXISTS team;` を実行する
  (root はパスワード無し。tiup playground の既定)。`IF NOT EXISTS` なので複数回起動しても安全
  (既存データを壊さない)。`tiup playground` 自体を毎回まっさらにするか(既定の揮発)、
  `--tag pokecalc` を付けて tiup のデータディレクトリに永続化するかは開発者の選択に委ねる
  (揮発させても実害は次回起動時に `make migrate-up` をもう一度叩き直すだけで、DB 名の再作成は
  `tidb-local-up.sh` が毎回冪等に行うため手順が増えない)。
- 停止は人間が明示的に行う(`Ctrl+C` または `tiup clean local`。CLAUDE.md「クラスタ削除・DBのデータ削除」は
  人間の確認事項だが、これは使い捨ての開発用プロセスであり永続データの削除ではないため対象外)。

### 3. k3d: TiDB Operator 最小構成

`deploy/k8s/overlays/local/mysql/`(単純な1レプリカ StatefulSet)とは異なり、TiDB は Operator 経由でしか
安全に運用できない(PD/TiKV/TiDB 間のトポロジ管理を手書き StatefulSet で再実装しない)ため、Operator を導入する。

**3.1 Operator 本体(`scripts/tidb-operator-bootstrap.sh`。冪等)**

```
kubectl apply --server-side -f https://raw.githubusercontent.com/pingcap/tidb-operator/v1.6.6/manifests/crd.yaml
helm repo add pingcap https://charts.pingcap.org/
helm repo update pingcap
helm upgrade --install tidb-operator pingcap/tidb-operator \
  --namespace=tidb-admin --create-namespace --version=v1.6.6 \
  --set operatorImage=pingcap/tidb-operator:v1.6.6 \
  --set scheduler.create=false
```

- `kubectl apply --server-side`(`create` ではない)。CRD を2回目以降も安全に当て直せる。TiDB Operator の
  CRD はサイズが大きく、クライアントサイド `apply` のアノテーション上限(`metadata.annotations`
  `kubectl.kubernetes.io/last-applied-configuration` の 256KB 制限)に触れうるため、サーバーサイド
  apply を使う(この理由からも `create` への差し替えだけでなく `apply` の方式そのものを選ぶ)。
- `helm upgrade --install`(`helm install` ではない)。2回目以降の `make up` で既存リリースがあっても失敗しない。
- `scheduler.create=false`: TiDB Operator 独自のスケジューラ拡張(Pod のトポロジ分散最適化)は
  レプリカ1台ずつの最小構成では効果が無く、k3d の既定スケジューラで十分なため無効化する(Pod を1つ減らす)。
- **順序制約**: このスクリプトは `up.sh` の中で **`kubectl apply -k deploy/k8s/overlays/local` より前**に
  呼ぶ(§3.2 の `TidbCluster`/`TidbInitializer` の CRD が無い状態で overlay を適用すると
  `no matches for kind "TidbCluster"` で失敗するため)。
- **他レーンへの影響を避ける非致命化**: `up.sh` は6レーン共通の入口(`make up`)なので、この bootstrap が
  失敗しても record/team 以外のサービスの起動を止めない。`up.sh` 内では
  `scripts/tidb-operator-bootstrap.sh || echo "警告: TiDB Operator の導入に失敗。record/team 以外は続行します" >&2`
  のように非致命扱いにする(`set -euo pipefail` の既定の即時終了から意図的に外す、この1行だけの例外)。
  P5-1 の時点では gateway はまだ record/team にルーティングしておらず、CLAUDE.md 絶対ルール5
  (計算 API は record/team/TiDB が落ちても成功する)を先取りする形になる。

**3.2 TidbCluster / TidbInitializer(`deploy/k8s/overlays/local/tidb/`)**

**実装時の追記(2026-09-24)**: `deploy/k8s/overlays/local/kustomization.yaml` の `resources` には
`tidb` を含めない。`kubectl apply -k deploy/k8s/overlays/local`(base・mysql を含む1回の呼び出し)に
TidbCluster/TidbInitializer を混ぜると、TiDB Operator の CRD が無いとき(§3.1 の bootstrap が
失敗したとき)に **この1回の apply コマンド全体が失敗し**、mysql・pokedex を含む他サービスの適用まで
巻き込んで止まる(§3.1「他レーンへの影響を避ける非致命化」の趣旨に反する)。したがって
`deploy/k8s/overlays/local/tidb` は独立した kustomization のまま(ディレクトリ配置は mysql と対称)、
`up.sh` から**別の** `kubectl apply -k deploy/k8s/overlays/local/tidb` として非致命的に適用する
(§3.1 の bootstrap 失敗時と同様、`|| echo 警告 ... 続行` で扱う。「影響」の up.sh 呼び出し順序を参照)。

PD/TiKV/TiDB 各1レプリカ、CPU/メモリ/ストレージの request・limit を明示する(既存の mysql overlay が
明示している慣習に揃える。上流の `examples/basic/tidb-cluster.yaml` は CPU/メモリを書いていないが、
それは「どの Kubernetes クラスタでも動く最小例」を優先しているためで、共有の k3d クラスタで他サービスの
Pod と資源を取り合う本リポジトリでは上限を明示する):

| コンポーネント | CPU request | CPU limit | メモリ request | メモリ limit | storage |
|---|---|---|---|---|---|
| PD | 100m | 300m | 256Mi | 512Mi | 1Gi |
| TiKV | 200m | 500m | 512Mi | 1Gi | 1Gi |
| TiDB | 100m | 300m | 256Mi | 512Mi | ―(ステートレス) |

- `pvReclaimPolicy: Retain`(上流の基本例と同じ)。**`TidbCluster` を削除しても PD/TiKV の PVC は残る**。
  PVC の手動削除はデータ削除そのもの(CLAUDE.md「クラスタ削除・DBのデータ削除」)であり、
  `migrate-down` の `CONFIRM_DESTROY` と同様に人間の確認が必要(「影響」に明記)。
  `storageClassName` は指定しない(k3d 既定の `local-path` を使う。mysql overlay と同じ判断)。
- `TidbInitializer`(TiDB Operator が正式にサポートする「初回だけ SQL を流す」CR。tiup playground には
  相当物が無いため §2 は別手段〈使い捨て `mysql` クライアント〉を使うが、k3d 側はこちらを使う)で
  root パスワードを設定し(`passwordSecret` フィールドに Secret 名 `tidb-root-auth` を指定する。値は `up.sh` が `openssl rand -hex 16` で
  生成し、`kubectl -n pokecalc get secret tidb-root-auth` が既に無いときだけ作る。mysql-auth と同じ流儀)、
  `initSql: "CREATE DATABASE IF NOT EXISTS record; CREATE DATABASE IF NOT EXISTS team;"` を実行する。
  **`tidb-root-auth` はどのサービス Pod にも一切マウントしない**(§4)。
- **`TidbCluster`・`TidbInitializer` の Ready/完了待ち(順序制約。API の実際の形を確認して決定)**:
  `record`/`team` の migrate Job は、DB が実在し root パスワードが設定済みであることを前提に
  `RECORD_PROVISION_DSN`(root 相当・`/record` 付き)へ接続する。
  - `TidbCluster` には(`Job`・多くの Kubernetes 組み込みリソースと違い)`.status.conditions[].type=Ready`
    のような汎用条件が無い(TiDB Operator v1.6.6 の API 定義で確認済み)。かわりに PD/TiKV/TiDB は
    それぞれ独立した `StatefulSet`(名前は `<TidbCluster 名>-pd`・`-tikv`・`-tidb`。本 ADR の
    `metadata.name: pokecalc-tidb` なら `pokecalc-tidb-pd` 等)として作られるため、
    `kubectl -n pokecalc rollout status statefulset/pokecalc-tidb-pd`(以下 `-tikv`・`-tidb` も同様)を
    3回呼ぶ(mysql の `rollout status statefulset/mysql` と同じ手段。新しい待ち方を持ち込まない)。
  - `TidbInitializer` も `Job` ではなく独自の CR で、完了は `.status.phase == "Completed"`
    (`Pending`/`Running`/`Completed`/`Failed`。TiDB Operator v1.6.6 のソースで確認済み)で表される。
    `kubectl wait` の `--for=jsonpath=...`(kubectl 1.23+)を使い、
    `kubectl -n pokecalc wait --for=jsonpath='{.status.phase}'=Completed tidbinitializer/pokecalc --timeout=180s`
    で待つ(`TidbInitializer` の `metadata.name: pokecalc` を対象にする。Job 名を推測する必要が無い)。
  - 待つ順序: PD/TiKV/TiDB の `StatefulSet` Ready(3つ)→ `TidbInitializer` の `Completed` → record/team の
    migrate Job 適用・完了待ち(後述「影響」の up.sh 変更点に反映)。この待ちがあるため、record/team の
    migrate Job 自身の initContainer は pokedex の `mysqladmin ping`(サーバ到達性だけを見る)と同じ
    簡潔さでよい(DB・ユーザーの実在は外側の up.sh の待ちで保証済みのため、initContainer 側で
    二重に確かめない)。
- **リソースの実測との照合**: 2026-09-24 時点の k3d クラスタ実測(`docker stats`・`kubectl top nodes`)は
  15 CPU / 7.75GiB 中、CPU 使用 11.9%(≈1.8 CPU)・メモリ使用 33%(2.6GiB)。今回追加する limit 合計は
  CPU 1.1 / メモリ 2.03GiB、request 合計は CPU 0.4 / メモリ 1.02GiB。スケジューリングを左右するのは
  request(15 CPU / 7.75GiB のうち request 使用分に request 0.4/1.02GiB を足すだけなので支障は無い)。
  limit 到達時の最悪ケース(TiKV が 1Gi まで使う等)でもメモリ使用は 2.6GiB(実測)+2.03GiB(limit 合計)
  ≈ 4.6GiB で、7.75GiB 中 59%に収まる。他サービス(balance/judge/speed/web/gateway/calc/pokedex/mysql)を
  圧迫しない。
- **Pod 自身の安定性**(TiKV はメモリ 1Gi limit が実運用ではやや厳しい既知の傾向があるため、
  他サービスへの影響だけでなく TiDB 自身が安定するかも確認する。受け入れ条件 AC-T3)。

### 4. DB とユーザーの分け方・資格情報の境界(ADR-0110 の踏襲。CLAUDE.md 絶対ルール4)

1つの TiDB クラスタに、`record`・`team` の2つの論理 DB を作る(MySQL 側で pokedex 用に1 DB だけ持つのと
対称。物理クラスタの共有は「サービスが他サービスの DB に触る」ことを意味しない。触るのは自分の DB の
ユーザーだけ)。DB の作成そのものは §3.2 の `TidbInitializer`(k3d)・§2 の `tidb-local-up.sh`(ローカル)が
担い、その後の**ユーザー作成は record/team それぞれの migrate Job が担う**(pokedex と対称。以下)。

各 DB ごとに2ロール(pokedex の reader/importer/migrator の3分割ではなく2分割。record-svc / team-svc は
importer のような別プロセスの書き込み経路を持たないため):

| ロール | 権限 | 用途 |
|---|---|---|
| migrator | `SELECT, INSERT, UPDATE, DELETE, CREATE, ALTER, DROP, INDEX, REFERENCES`(ADR-0110 の `MigratorPrivileges` と同一定数を再利用) | migrate Job |
| app | `SELECT, INSERT, UPDATE, DELETE` | record-svc / team-svc 本体・失効 CronJob |

`services/pokedex/db/grants.go` の `Provision`・`RoleGrant`・`MigratorPrivileges` は DB 名を引数に取るだけで
pokedex 固有の処理を含まないため、record/team はこのパッケージを import して再利用する(コピーしない)。
ただし app ロールには pokedex の `ImporterPrivileges` を**そのまま使わず**、record/team 用の中立な名前の
定数 `AppPrivileges`(値は同じ `"SELECT, INSERT, UPDATE, DELETE"`)を `grants.go` に追加する
(pokedex の importer 都合で `ImporterPrivileges` の値が将来変わったときに、record/team の app 権限が
無関係に変わってしまうのを防ぐ)。パッケージ import 名の衝突(`services/record/db` も `services/team/db` も
package 名が `db` になる)は呼び出し側で `pokedexdb "example.com/pokecalc/services/pokedex/db"` のように
別名を付ける。`grants.go` のエラー文言の接頭辞(`"pokedex/db: ..."`。`ErrInvalidRoleGrant` 等)は
`AppPrivileges` を追加する同じコミットで中立な `"db: ..."` に変更する(pokedex 固有の接頭辞のまま
record/team の migrate Job のログに出るのを避ける。エラー文言の変更なので `errors.Is` 判定には影響しない)。

**資格情報の消費者(1本化)**:

- `RECORD_PROVISION_DSN`・`TEAM_PROVISION_DSN`(root 相当。DB 名を含む。§3.2 の root パスワードを使い、
  `.../record`・`.../team` を指す)は**それぞれのサービスの migrate Job だけ**が読む(pokedex の
  `cmd/migrate` が `POKEDEX_PROVISION_DSN` を読むのと同じ役割。up.sh 自身はこの DSN を消費しない。
  up.sh が触るのは §3.2 の `tidb-root-auth`〈CREATE DATABASE 用〉と、ここで新しく作る
  `record-db-auth`/`team-db-auth`〈値の生成と Secret への格納〉まで)。
- Secret は **サービスごとに分ける**(`record-db-auth`・`team-db-auth`。pokedex の `mysql-auth` 1本に
  全ロールをまとめる形を踏襲しない)。理由: TiDB クラスタは1つで root は両 DB に対して全権のため、
  もし1本の Secret に record・team 両方の DSN を入れると、record の Deployment/Job マニフェストが
  誤って team のキーを参照しても構文的には成立してしまう(pokedex は DB もクラスタも1つなので
  この種の取り違えが起こり得なかった)。Secret を分けることで、record の Pod 定義がそもそも
  `team-db-auth` という名前の Secret を知らない状態にできる。
  `tidb-root-auth` のキー(root パスワードそのもの)は record-db-auth・team-db-auth のどちらにも
  そのままの形では再掲しない。ただし up.sh が `*_PROVISION_DSN` を生成する際に root パスワードを
  **DSN 文字列へ埋め込んで**書くため、実際には `RECORD_PROVISION_DSN`/`TEAM_PROVISION_DSN` という形で
  root 相当の資格情報が record-db-auth・team-db-auth それぞれに含まれ、それぞれの migrate Job Pod に
  マウントされる(pokedex の `POKEDEX_PROVISION_DSN` が migrate Job にだけ渡るのと同じ水準。
  §2 のローカル開発〈tiup playground〉では root にパスワードが無いため、この埋め込みは k3d 側だけの話)。
  「露出面を1 Secret に閉じる」という意味ではなく、**「露出先を migrate Job 2つに限り、record-svc/team-svc
  本体〈`*_APP_DSN` を使う長時間稼働 Pod〉には root 相当を一切渡さない」**という境界であることに注意する。
- 環境変数名: `RECORD_PROVISION_DSN`・`RECORD_APP_DSN`・`RECORD_DATABASE_DSN`(migrator)、
  `TEAM_PROVISION_DSN`・`TEAM_APP_DSN`・`TEAM_DATABASE_DSN`(migrator)。

### 5. migration ツール: 共通パッケージへ切り出して再利用する

TiDB は MySQL ワイヤプロトコル互換で、`go-sql-driver/mysql` と `golang-migrate/migrate/v4/database/mysql` は
そのまま繋がる(ADR-0209 はスキーマに外部キーを要求せず、本 ADR のテーブルも外部キーを使わないため、
TiDB の外部キー対応の制限は影響しない)。

**pokedex・record・team の3例で `Up`/`DownAll`/`Version` のロジックが完全に同一になる**ため(rule of three)、
`services/pokedex/db/migrate.go` の中身を新設パッケージ `services/internal/dbmigrate` へ切り出す:

```go
// services/internal/dbmigrate/migrate.go
package dbmigrate

func Up(dsn string, fsys fs.FS) error
func DownAll(dsn, confirmDatabase string, fsys fs.FS) error
func Version(dsn string, fsys fs.FS) (version uint, dirty bool, ok bool, err error)
var ErrDownNotConfirmed = errors.New("dbmigrate: down の確認用 DB 名が DSN の DB 名と一致しない")
```

(現状の `newMigrate` はパッケージ変数 `Migrations embed.FS` を直接参照する非公開関数で、これを
そのまま import することはできない。`fsys fs.FS` を引数に取る形に変えるのがこの切り出しの本体。)

`services/pokedex/db/migrate.go` は薄いラッパーとして残す(既存の呼び出し側
`services/pokedex/cmd/migrate/main.go` の `db.Up`/`db.DownAll`/`db.Version` 呼び出しを変えないため):

```go
//go:embed migrations/*.sql
var Migrations embed.FS

var ErrDownNotConfirmed = dbmigrate.ErrDownNotConfirmed // layout_test.go 等が errors.Is で参照するため再エクスポートする

func Up(dsn string) error                                    { return dbmigrate.Up(dsn, Migrations) }
func DownAll(dsn, confirmDatabase string) error               { return dbmigrate.DownAll(dsn, confirmDatabase, Migrations) }
func Version(dsn string) (uint, bool, bool, error)            { return dbmigrate.Version(dsn, Migrations) }
```

`services/record/db/migrate.go`・`services/team/db/migrate.go` は同じ形の薄いラッパーを、それぞれ自分の
`//go:embed migrations/*.sql` で持つ(スキーマの実体を混同しないため。1つの `dbmigrate.Migrations` に
3サービス分の migration を混在させない)。

CLI 部分(`cmd/migrate/main.go` の `cliEnv`・`run`・`runUp`/`runVersion`/`runDown`)はサービスごとに
コピーする(pokedex を含め3例になるが、複製の理由は「例の数が少ないから」ではなく「読む環境変数名
〈`POKEDEX_DATABASE_DSN` 等〉がサービスごとに違う」という具体的な事情による。各 `main.go` は50行程度で
`main_test.go` もサービスごとに独立して持てるため、今この3例の重複が保守コストとして顕在化していない。
環境変数名をパラメータ化した共通 CLI への統合は、実際に困ったときに検討する)。

**受け入れ条件への反映**(後述の AC-T4・AC-T5 に追加):
- 切り出し後も pokedex の既存テスト(`TestMigrationsAreNumberedPairs` 等 `services/pokedex/db/layout_test.go`
  一式、`grants_test.go`・`grants_mysql_test.go`)が変更無しで緑のままであること。
- record の `migrate up` 実行後、record DB に `devices`・`purge_journal` **だけ**が存在し、
  pokedex のテーブル(`types`・`species` 等)が存在しないこと(旧設計の誤り〈後述「変更履歴」〉の再発防止)。

**実装時の追記(2026-09-25。P5-3 の critic レビュー対応で判明)**: `golang-migrate/migrate/v4/database/mysql`
の `Lock()`(migration 実行前後の排他制御)は `sql.TxOptions{Isolation: sql.LevelSerializable}` を指定して
トランザクションを開始する。TiDB は `SERIALIZABLE` 分離レベルをサポートしないため、これを設定しないまま
`record-migrate`/`team-migrate` を TiDB に対して実行すると
`Error 8048 (HY000): The isolation level 'SERIALIZABLE' is not supported` で必ず失敗する
(ローカルの `tidb-server --store=unistore` で確認済み。TiKV 構成の TiDB でも同じ制限)。
**`Lock()` だけの問題ではない**: 同ドライバの `SetVersion()`(`mysql.go:357`。migration 適用後にバージョンを
記録する処理)も無条件に同じ `sql.LevelSerializable` を使うため、`x-no-lock=true`(同ドライバが対応する
DSN オプション。`Lock()` の排他制御を無効化できる)を付けても `SetVersion()` 側で同じエラーになり回避でき
ない。つまりクラスタ側の設定を変える以外に現実的な回避策が無い。
対策として TiDB のグローバル変数 `tidb_skip_isolation_level_check=1` を一度設定する(TiDB は
global 変数を永続化するため、クラスタ生成後に1回でよい)。
`deploy/k8s/overlays/local/tidb/tidbinitializer.yaml` の `initSql` と `scripts/tidb-local-up.sh` に追加した。
`dbmigrate` package 自体を変更する(例: 内部で `SET GLOBAL` を自動実行する)選択肢もあったが、
DB 接続ユーザーに `SUPER`/`SYSTEM_VARIABLES_ADMIN` 相当の権限が要ることになり、ADR-0211 §4 の
最小権限方針(migrator ロールは DDL のみ)に反するため採らなかった。クラスタ側の一度きりの設定に留める。
**既存クラスタへの注意**: `TidbInitializer` は一度 Completed になると `initSql` を再実行しない。すでに
`make up` で TiDB を立てたことがあるローカルクラスタでは、この `initSql` の変更が反映されないまま
`record-migrate`/`team-migrate` が同じ 8048 エラーで失敗し続ける。その場合は (a) TiDB に直接
`SET GLOBAL tidb_skip_isolation_level_check=1` を一度流すか、(b) `TidbInitializer`(と必要なら
`TidbCluster` の該当部分)を作り直すこと。P5-1 の実機確認(共有 k3d クラスタへの初回適用)がまだ未実施の
ため、この対応が実際に必要になるのは P5-3b(deploy/k8s の record-svc 配線)の実機確認時点の見込み。

### 6. スキーマ(ADR-0209 §3 の #5・#5b。record DB・team DB がそれぞれ自分の分を持つ)

`services/record/db/migrations/`・`services/team/db/migrations/` に同一内容の2本の migration を置く
(テーブル定義は同じだが、DB が分かれているため migration ファイルも分ける。共有 migration にすると
「record だけ先に進める」ができなくなり、サービスの独立性に反する):

```sql
-- 000001_create_devices
CREATE TABLE devices (
  device_id       VARCHAR(36) NOT NULL,
  last_seen_at    DATETIME(6) NOT NULL,
  purged_at       DATETIME(6) NULL,
  orphaned_since  DATETIME(6) NULL,
  PRIMARY KEY (device_id),
  KEY idx_devices_last_seen_at (last_seen_at),
  KEY idx_devices_orphaned_since (orphaned_since)
);

-- 000002_create_purge_journal
CREATE TABLE purge_journal (
  id           BIGINT NOT NULL AUTO_INCREMENT,
  device_id    VARCHAR(36) NOT NULL,
  requested_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_purge_journal_device_id (device_id),
  KEY idx_purge_journal_requested_at (requested_at)
);
```

- `device_id` は `VARCHAR(36)`(UUID 文字列。gateway が検証する形式と同じ長さ。ADR-0209 §2)。
- `purge_journal` に `device_id` の一意制約を付けない(同じ端末が複数回削除要求を送っても、そのたびに
  1行追記する追記専用ログとして扱う。ADR-0209 §5b「削除要求を受け付けた…時刻だけの追記専用ログ」)。
  `idx_purge_journal_device_id` は復元時の再適用(特定端末の削除要求だけを見る)、
  `idx_purge_journal_requested_at` は期限切れ行(90日超)の削除ジョブの範囲スキャン用。
- **`orphaned_since`**(ADR-0209 §3 #5「行自体はその端末のデータ〈#1〜#4〉が全て無くなってから30日で
  失効ジョブが消す」の起点)。この列が無いと「データが空になった時刻」を判定できず、
  `*_DEVICE_ROW_EXPIRY_DAYS`(30)が起点の無い日数になってしまう(第1回レビューでの指摘)。
  運用: 日次の失効ジョブ(P5-3/P5-4 が実装)が、ある `device_id` について業務テーブル
  (`calc_events`・`favorites`・`teams`・`team_members`)に1行も残っていないことを初めて観測したときに
  `orphaned_since = 現在時刻` を書き、以後その端末に新しいデータが作られたら `orphaned_since` を
  `NULL` に戻す。`orphaned_since` が `*_DEVICE_ROW_EXPIRY_DAYS` より過去なら `devices` 行自体を削除する。
  `idx_devices_orphaned_since` はこの走査に使う(`last_seen_at` 用の索引と対になる非対称を残さない)。
  業務テーブルは P5-3/P5-4 で作るため、この列の書き手は今は存在しない(スキーマの契約だけを今固定する)。
- 業務テーブル(`favorites`・`teams`・`team_members`・`calc_events`)は P5-3/P5-4 がハンドラと同時に追加する。

**5b の未充足ギャップ(既知。隠さず記録する)**: ADR-0209 §5b・§9-2 は purge journal を「DB 側とは別に、
DB のバックアップ世代とは独立の場所(P7-4 が決める)にも同時に追記する」ことを求めているが、
その独立保存先は P7-4(バックアップ/復元。plan.md では P5-3/P5-4 より後)が決めるまで存在しない。
したがって **P5-3(record-svc の削除 API 実装)から P7-4 完了までの間、ADR-0209 §5b/§9-2 の
「DB 外への同時追記」は満たされない**(DB 内の `purge_journal` テーブルへの記録だけになる)。
これは黙って先送りせず、ADR-0209 の「人間の確認が必要なこと」に既知のギャップとして追記し
(この ADR の「影響」で ADR-0209 に短い追記を行う)、docs/plan.md の P7-4 の項目でも参照できるようにする。
今この時点で暫定の二重書き先(ファイル等)を作らない理由: P5-3 より前の今は削除 API 自体が無く
書き込む主体が無いため、暫定実装を先に作ると実装対象が無いまま設計だけが進み、P7-4 で
正式な保存先が決まったときに手戻りが起きる可能性が高い。

### 7. 保持日数・墓石猶予の環境変数名(P5-3/P5-4 の失効ジョブが読む契約を今決めておく)

ADR-0209 §3・§4・§7 の日数を、サービスごとの環境変数名として固定する。**値は起動時に検証し、
未設定または0以下ならサービスを起動しない**(ADR-0209 の却下案〈起動時に検証する〉を採用する側で確定する。
下表の「既定値」は本 ADR が推奨する日数であって、コード側のフォールバック値として使わない。
検証コード自体は record-svc / team-svc 本体を作る P5-3/P5-4 で書く。ここで決めるのは名前・既定値・
満たすべき関係だけ):

| 環境変数 | 既定値 | 対応する ADR-0209 の行 | 追加の検証要件 |
|---|---|---|---|
| `RECORD_CALC_EVENTS_RETENTION_DAYS` | 90 | §3 #1 | ― |
| `RECORD_FAVORITES_RETENTION_DAYS` | 540 | §3 #3 | ― |
| `RECORD_DEVICE_ROW_EXPIRY_DAYS` | 30 | §3 #5(行の失効)・§7(削除の墓石判定の猶予。ADR-0209 は同じ30日を両方に使うと定めており、変数を分けない) | JetStream の `max_age`(7日固定。P5-2)より大きいこと |
| `RECORD_PURGE_JOURNAL_RETENTION_DAYS` | 90 | §3 #5b | ― |
| `TEAM_RETENTION_DAYS` | 540 | §3 #4 | ― |
| `TEAM_DEVICE_ROW_EXPIRY_DAYS` | 30 | §3 #5・§7 | JetStream の `max_age` より大きいこと |
| `TEAM_PURGE_JOURNAL_RETENTION_DAYS` | 90 | §3 #5b | ― |

半減期(推薦の時間減衰)は集計ロジックの一部であり保持期間ではないため、この表に含めない(P5-3 が決める)。

### 8. Go module の置き場所

`services/record`・`services/team`・新設 `services/internal/dbmigrate` は `services/go.mod`
(pokedex・calc・gateway と同じモジュール)の配下に置く。`go.work` への追記は不要(balance/judge/speed の
ような独立モジュールにしない。record/team は判定・タイプバランスのような「別レーンの姉妹サービス」ではなく、
gateway が直接ルーティングする中核サービスで、pokedex/calc と同じ扱いにするのが実態に合う)。

## 却下した案

- **TiDB Operator v2(main/alpha)を使う**: 2026-09-24 時点で安定版タグが無い。「依存は最新安定版を固定」の
  方針は alpha を含まない。
- **k3d でも tiup playground を使う(Operator を導入しない)**: `tiup playground` はマルチノードの
  Kubernetes 環境向けではなく単一プロセスの開発用ツールで、Deployment/PVC としての再起動耐性・
  リソース分離が無い。
- **record・team それぞれに別々の TiDB クラスタを作る**: PD/TiKV/TiDB を2セット動かすとリソース表が
  倍になり、他サービスとの共存の余裕が大きく減る。DB を分ける目的(CLAUDE.md 絶対ルール4)は
  論理的な分離で十分に満たせ、物理クラスタを分ける必要はない(pokedex 用 MySQL も他サービスと共有の
  StatefulSet 1台であり対称)。
- **1本の `tidb-auth` Secret に record・team 両方の DSN をまとめる(pokedex の `mysql-auth` と同形)**:
  §4 のとおり、TiDB クラスタが1つで root が両 DB に全権を持つ構造では、Secret を1本にまとめると
  マニフェストの取り違えで越境しうる余地が生まれる。サービスごとに Secret を分ける追加コストは小さいため、
  pokedex の前例をそのまま踏襲しない。
- **各サービスが完全に独立した migrate 実装(`Up`/`DownAll`/`Version`)を持つ**: pokedex・record・team の
  3例でロジックが完全に同一になるため(§5)、コードの重複を許容する理由が無くなった
  (第1回レビュー時点の判断〈時期尚早〉を撤回。使う側が3例そろった時点で切り出す)。

## 影響

- 新規: `scripts/tidb-local-up.sh`、`scripts/tidb-operator-bootstrap.sh`、
  `deploy/k8s/overlays/local/tidb/`(kustomization・tidbcluster.yaml・tidbinitializer.yaml)、
  `services/internal/dbmigrate/`、
  `services/record/db/migrations/`・`services/record/db/migrate.go`・`services/record/cmd/migrate/`、
  `services/team/db/migrations/`・`services/team/db/migrate.go`・`services/team/cmd/migrate/`。
  k3d 上でスキーマを当てる migrate Job(`deploy/k8s/base/record/job-migrate.yaml`・
  `deploy/k8s/base/team/job-migrate.yaml`。Job 定義自体は pokedex の `job-migrate.yaml` と同形)も
  P5-1 の範囲に含める(これが無いと「k3d 上で record/team のスキーマがいつ適用されるか」が決まらない)。
  ただし pokedex と異なり、**この2つの Job は `deploy/k8s/overlays/local` の kustomize `resources` には
  含めない**(§3.2 の順序制約により、TidbInitializer 完了前に overlay 一括 apply で Job が動き出すと
  `Unknown database` で失敗しうるため)。`up.sh` が TidbInitializer の完了を待った**後**に、この2つだけ
  個別に `kubectl apply -f` する(pokedex-migrate のように overlay の一部として一括適用しない)。
- 変更: `services/pokedex/db/migrate.go`(§5 の薄いラッパー化。公開 API は変えない)、
  `services/pokedex/db/grants.go`(`AppPrivileges` 定数を追加。既存の `ReaderPrivileges`/
  `ImporterPrivileges`/`MigratorPrivileges` は変更しない)、
  `scripts/up.sh`(§3.1 の bootstrap 呼び出し〈非致命〉・`tidb-root-auth`/`record-db-auth`/`team-db-auth`
  Secret 作成・`deploy/k8s/overlays/local/tidb` の apply〈非致命。§3.2 実装時の追記〉・TidbCluster の
  Ready 待ち。呼び出し順序は bootstrap(非致命)→ `tidb-root-auth` 作成 → 通常の overlay apply
  (base・mysql・pokedex-migrate。既存のまま)→ `deploy/k8s/overlays/local/tidb` の apply(非致命)→
  PD/TiKV/TiDB の StatefulSet Ready 待ち → TidbInitializer の `Completed` 待ち〈§3.2〉→
  `record-db-auth`/`team-db-auth` 作成 →
  record/team migrate Job 適用・完了待ち。TiDB 関連の非致命ステップがどこで失敗しても、それより後の
  TiDB 関連ステップは実行時にスキップされる〈存在しない Secret・CR に対する待ちにならないよう、
  各ステップの前に前段の成功を確認する〉)、`.env.example`(TiDB ローカル DSN の例を追記)、
  `Makefile`(`tidb-local-up`・record/team それぞれの `migrate-up`/`migrate-version`。down は pokedex と
  同様に破壊的操作として `CONFIRM_DESTROY` 必須。`test-db` に record/team を追加し、
  `RECORD_TEST_DSN`/`TEAM_TEST_DSN`〈root 相当。`grants_tidb_test.go` 用〉を新しい必須環境変数として追加)。
  `docs/adr/0209-record-team-data-retention.md` に、§6 で記録した purge journal の既知のギャップ
  (P5-3〜P7-4 の間 §5b/§9-2 が未充足)を追記する。`docs/plan.md` の P5-1 の記述を、この ADR が確定した
  範囲(devices・purge_journal・プロビジョニングまで。業務テーブルと保持日数の起動時検証コードは
  P5-3/P5-4 に明記して移す)に合わせて実装コミットで同時に更新する。
- P5-3/P5-4 はこの ADR の DSN 環境変数名・保持日数環境変数名・スキーマ・Secret 名をそのまま前提にする。

## 受け入れ条件(P5-1 の完了条件)

- AC-T1: ローカルで `scripts/tidb-local-up.sh` 実行後、`record`・`team` の2 DB が存在する。続けて
  `RECORD_PROVISION_DSN`・`RECORD_APP_DSN`・`RECORD_DATABASE_DSN`(migrator。3本とも必須。1本でも
  欠けると `Provision` の呼び出し元が exit 2 で拒否する。pokedex の `rolesFromEnv` と同じ規則)を使って
  `make migrate-up` 相当を実行すると `devices`・`purge_journal` が作成される(team も同様)。
  DB・ユーザー作成からスキーマ適用までがスクリプト+1コマンドで完了する。
- AC-T2: `services/record/db`・`services/team/db` それぞれで migrate up → down → up が冪等に成功する
  (pokedex の migration テストと同じ形〈layout_test.go 相当〉で確認する。実 DB 接続が要るテストは
  `test-db` 相当に含め `make test` には含めない。pokedex の慣習と同じ)。
- AC-T3: k3d で `scripts/tidb-operator-bootstrap.sh` → `deploy/k8s/overlays/local/tidb` の適用後、
  PD/TiKV/TiDB の `StatefulSet`(`pokecalc-tidb-pd`・`-tikv`・`-tidb`)の Ready を
  `kubectl -n pokecalc rollout status statefulset/pokecalc-tidb-pd` 等で確認できる。
  PD/TiKV/TiDB の全 Pod が Running を5分以上維持し `restartCount` が増えないこと(TiKV のメモリ limit が
  実運用で厳しすぎないかの確認)。他サービス(balance/judge/speed/web/gateway/calc/pokedex/mysql)の Pod が
  Evicted/OOMKilled にならないことを `kubectl get pods -n pokecalc` で確認する。
- AC-T4: `services/pokedex/db` の `Provision`/`RoleGrant`/`MigratorPrivileges`/`AppPrivileges` を
  record/team から import してビルドが通ること(コピーしていないことをコードレビューで確認する)。
  さらに実 TiDB(v8.5.8)に対して `Provision` を**2回連続**実行してもエラーにならず、
  `record_migrator` は `CREATE TABLE` に成功し `record_app` は失敗すること(ADR-0110 受け入れ条件1・3・4と
  同形。`services/record/db/grants_tidb_test.go`〈`-tags tidb`〉で確認し、`make test-db` 相当に含める)。
  record の `migrate up` 後、record DB に `devices`・`purge_journal` だけが存在し pokedex のテーブルが
  存在しないこと。§5 切り出し後も pokedex の既存テストが変更無しで緑のままであること。
- AC-T5: `go build ./...`(`services` モジュール)が record/team/dbmigrate 追加後も通る。`go.work` に
  record/team の独立エントリを追加していないことを確認する。
- AC-T6: TiDB(TidbCluster・TidbInitializer)を一切デプロイしない状態でも `/api/calc`・`/api/calc/bulk`・
  `/api/calc/reverse` が成功すること(CLAUDE.md 絶対ルール5。P5-1 の時点では自明に成り立つが、
  以後のフェーズでも壊さないための回帰確認として明記する)。
- AC-T7: `record-db-auth`・`team-db-auth`・`tidb-root-auth` それぞれについて、どのマニフェスト
  (Deployment/Job)がどの Secret のどのキーを参照するかを一覧し、record 側のマニフェストが
  `team-db-auth`・`tidb-root-auth` のキーを一切参照しないこと(ADR-0110 の資格情報境界の確認と同形)。
- AC-T8: `up.sh` が `TidbInitializer` の `.status.phase == "Completed"` を待ってから record/team の
  migrate Job を適用すること(§3.2 の順序制約)。この待ちを外すと migrate Job が `Unknown database`
  または認証エラーで失敗しうることを、実装時に一度わざと待ちを外して再現させてから戻す(mutation 確認)。

## 人間の確認が必要なこと

TiDB クラスタ・データ自体の新規作成(クラスタ削除・DB のデータ削除の対象ではない)は自動で進めるが、
以下は取り消しにくい/共有への影響があるため慎重に扱う:

- **PD/TiKV の PVC 削除**(`pvReclaimPolicy: Retain` により `TidbCluster` を消しても残る永続データ)は
  CLAUDE.md「クラスタ削除・DBのデータ削除」に該当する人間の確認事項。`migrate-down`(`CONFIRM_DESTROY`
  必須)と同様、PVC の削除は AI が自動で行わない。
- **共有 k3d クラスタへのクラスタスコープ CRD 導入**(`make up` は6レーン共通の入口)。§3.1 のとおり
  `up.sh` 側でこの導入失敗を非致命化し、他レーンの `make up` 全体を止めない設計にした。導入自体は
  取り消しやすい操作(`helm uninstall tidb-operator`・`TidbCluster`/`TidbInitializer` の削除は
  他サービスに影響しない)だが、実クラスタでの初回適用(AC-T3)は Pod の安定性を実測で確認してから進める。

## 変更履歴

- 2026-09-24 第1回 critic レビュー FAIL を受けて全面改訂: §5 の migrate 再利用方式を「import」から
  「`fs.FS` を取る共通パッケージへの切り出し」に変更(旧案は非公開関数 `newMigrate` がパッケージ変数
  `Migrations` を直接参照する構造のため、そのままでは record/team が pokedex のスキーマを誤って
  当ててしまう欠陥があった)。DB 作成の責任者(`TidbInitializer`/`tidb-local-up.sh`)を明記。
  `devices.orphaned_since` を追加し ADR-0209 §3 #5 の失効起点を定義。purge journal の外部保存先の
  ギャップを既知の限界として明記。保持日数の起動時検証方針を確定。bootstrap の冪等化・適用順序を明記。
  ストレージサイズと PVC 削除の扱いを追加。Secret を `record-db-auth`/`team-db-auth`/`tidb-root-auth` に
  分割し、provision DSN の消費者を record/team それぞれの migrate Job に一本化。CLAUDE.md/ADR-0012 への
  誤った引用を修正。
- 2026-09-24 第2回 critic レビュー(第1回指摘はすべて RESOLVED)で残った4件を修正:
  `TidbInitializer` の完了待ちが順序制約から欠けていた点(§3.2 に追加、AC-T8 新設)、
  §5 の pokedex 側ラッパー雛形が `ErrDownNotConfirmed` を再エクスポートしておらず既存テストが
  コンパイルできなくなる欠陥(雛形に追加)、存在しない節番号(§7/§8/§9)への自己参照6箇所を
  節名参照に修正(docs/plan.md の同種の誤りも修正)、root 資格情報の露出範囲の説明が
  「1 Secret に閉じる」という不正確な表現だった点を「migrate Job 2つに限る」に是正。
  あわせて AC-T1 に `RECORD_APP_DSN` を追加、`grants.go` のエラー接頭辞の中立化、
  `test-db`/`RECORD_TEST_DSN`・`TEAM_TEST_DSN` を「影響」に追記(参考指摘への対応)。
- 2026-09-24 第3回 critic レビュー PASS。実装時の申し送り2点を反映: record/team の migrate Job を
  kustomize overlay の `resources` に含めず、TidbInitializer 完了待ちの後に個別 apply する運用に修正
  (含めると Initializer 完了前に Job が動き出し `Unknown database` で失敗しうるため)。
  `job/<initializer名>` のプレースホルダ表記を明確化。
- 2026-09-24 k8s マニフェスト・`up.sh` 配線の実装(critic レビュー)で判明した、設計時には
  気づけなかった実装上の誤りを修正:
  - `TidbInitializer` の `passwordSecret` は Secret の**キー名をそのままユーザー名として**扱う
    (TiDB Operator v1.6.6 のソース `pkg/manager/member/startscript/v1/template.go` で確認)。
    キー名を `root-password` としていたのは誤りで、`root` に修正(§3.2・`scripts/up.sh` の
    `tidb-root-auth` 作成箇所)。
  - `TidbInitializer` の `image` に `mysql:9.7.2`(サーバイメージ)を指定していたのは誤り。
    Operator の初期化スクリプトは python + `MySQLdb` を要求する(同ソース `pkg/manager/member/
    tidb_init_manager.go`)ため、上流の `examples/initialize`・`manifests/initializer` が指す
    `tnir/mysqlclient` に変更し、digest を固定した(§3.2)。
  - `deploy/k8s/overlays/local/tidb/kustomization.yaml` に `namespace: pokecalc` が無く、
    独立した kustomization として apply すると k3d の既定 namespace(`default`)に作られ、
    `up.sh` の `-n pokecalc` な待ち・削除と食い違っていたため追加。
  - record/team の migrate Job の initContainer が到達性確認のためだけに `tidb-root-auth` を
    参照していたのは AC-T7 違反(§4 の「root 相当は migrate Job にだけ渡る」という境界は
    「自分の Secret の migrate Job」を指し、他サービスの Secret ではない)。`mysqladmin ping` は
    認証に失敗してもサーバが応答していれば成功する(終了コード0)仕様のため、資格情報無しの
    到達性確認に変更した。
  あわせて、このクラスの誤り(namespace 不一致・資格情報の越境)を静的に検出する回帰テストを
  `services/pokedex/db/layout_test.go` に追加(`TestUpScriptAppliesRecordTeamJobsWithNamespace`・
  `TestTidbOverlayHasNamespace`・`TestRecordTeamJobsDoNotCrossReferenceSecrets`)。
  `scripts/check-publishable.sh` の B(秘密らしき文字列)許可リストに `tidb-root-auth`(Secret 名の
  参照)を追加し、シェル変数参照の許可条件を「値の末尾が `${...}`」から「値の全体が `${...}`」に
  絞った(本物の値へ無害な変数参照を継ぎ足す細工を通さないため。self-test に確認ケースを追加)。
