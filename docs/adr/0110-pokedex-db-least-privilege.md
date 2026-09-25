# ADR-0110: pokedex の DB 資格情報を用途別の最小権限へ分離する(issue #104)

- 状態: 採用(issue #104 の仕様。spec-writer 起草、implementer が実装、critic PASS。実クラスタで検証済み)
- 日付: 2026-09-24
- レーン: データ(ADR 帯 `0100〜`)
- 関連: issue #104、ADR-0100(§5 migrate・§9 秘密の扱い)、ADR-0109(cronjob.sh の排他制御。
  同じく `services/pokedex/` の運用強化)、CLAUDE.md 絶対ルール2・4・6

## 背景

`scripts/up.sh` が作る Secret `mysql-auth` は root パスワードと、その root を使う DSN(キー
`pokedex-dsn`)だけを持つ。pokedex-svc(公開の検索・内部 API)・importer(CronJob)・
migrate(Job)の3つの Pod がすべて同じ `pokedex-dsn`(root)を参照している。

pokedex-svc は `services/pokedex/db/query/pokedex.sql` の SELECT 系クエリしか呼ばない
(公開 HTTP を受ける Pod)。importer は全置換の DELETE/INSERT を呼ぶが DDL は呼ばない。
migrate だけが DDL(CREATE TABLE/ALTER TABLE)を必要とする。公開 API を受ける Pod が
DDL・ユーザー管理まで可能な root 権限を持つのは、侵害時の被害を不必要に広げる。

## 決定

### 1. 3つの DB ユーザー(`pokedex_reader` / `pokedex_importer` / `pokedex_migrator`)を作る

| ユーザー | 用途 | 権限(`pokedex.*` に対して) |
|---|---|---|
| `pokedex_reader` | pokedex-svc(公開 API) | `SELECT` のみ |
| `pokedex_importer` | importer(CronJob) | `SELECT, INSERT, UPDATE, DELETE`(DDL・ユーザー管理は無し) |
| `pokedex_migrator` | migrate(Job) | `SELECT, INSERT, UPDATE, DELETE, CREATE, ALTER, DROP, INDEX, REFERENCES`(DDL を含むが `CREATE USER`・`GRANT` 等のグローバル権限は無し) |

`pokedex_importer` に `UPDATE` は現状のクエリ(`services/pokedex/db/query/pokedex.sql`)では
使わないが、issue の受け入れ条件が明示的に `SELECT/INSERT/UPDATE/DELETE` を許容範囲としており、
将来の差分更新(全置換でなく変更行だけ UPDATE する最適化)に対応する余地を残す。

`pokedex_migrator` の `REFERENCES` は外部キー制約(`species_abilities` 等)を持つ
`CREATE TABLE` のために必要。`CREATE USER`・`GRANT OPTION`・`DROP DATABASE`(グローバル権限)は
GRANT の対象を `pokedex.*` に限定する(DB スコープの GRANT に含まれない)ことで自動的に持たない。

### 2. root は migrate Job だけに残す(提供だけの役割に限定)

pokedex-svc の Deployment・importer の CronJob からは root 資格情報を完全に外し、
`pokedex_reader`/`pokedex_importer` の DSN だけを渡す。root(既存の `pokedex-dsn` キー)は
migrate Job にだけ残し、**用途を「3ユーザーの作成・権限付与(プロビジョニング)」に限定する**。
migrate Job の環境変数名を `POKEDEX_DATABASE_DSN` から `POKEDEX_PROVISION_DSN` に変える
(実際のスキーマ migration には使わない。§3 参照)。

却下: migrate も専用ユーザーだけにし、root を一切使わない — 3ユーザー自体を誰が作るかという
鶏卵問題が残る(`pokedex_migrator` に `CREATE USER` を持たせると、それ自体が「ユーザー管理権限を
持たない」という受け入れ条件に反する)。migrate Job は元々「一度だけ・人の確認無しに自動実行される
スキーマ準備」の役割を持つため、プロビジョニングもその役割の一部として引き受ける。

### 3. migrate は「プロビジョニング → 移行」の2段を1回の起動でこなす(シェルを増やさない)

`services/pokedex/Dockerfile` の `migrate` ターゲットは `FROM scratch`(シェルもコアユーティリティも
無い、静的バイナリ1本)。2つの処理を1つの Job で行うのに `sh -c "a && b"` は使えないため、
Go バイナリ自身が両方を順に行う。

`services/pokedex/cmd/migrate` の `up` サブコマンドを拡張する:

```go
case "up":
    if provisionDSN := os.Getenv("POKEDEX_PROVISION_DSN"); provisionDSN != "" {
        roles, err := rolesFromEnv() // POKEDEX_READER_DSN / POKEDEX_IMPORTER_DSN / POKEDEX_DATABASE_DSN(migrator) を読む
        if err != nil { ... return 2 }
        if err := db.Provision(provisionDSN, roles); err != nil { ... return 1 }
    }
    if err := db.Up(dsn); err != nil { ... return 1 } // dsn = POKEDEX_DATABASE_DSN(migrator)
```

`POKEDEX_PROVISION_DSN` が無いとき(ローカル `make dev`・`make test-db` 等、root DSN をそのまま
使う既存の使い方)はプロビジョニングを丸ごとスキップし、今までどおり `dsn` で直接 `Up` する
(後方互換。§7 参照)。

migrate Job の環境変数(4つ、すべて Secret `mysql-auth` の別キーから):

| 環境変数 | Secret キー | 用途 |
|---|---|---|
| `POKEDEX_PROVISION_DSN` | `pokedex-dsn`(既存。中身は root DSN のまま) | 3ユーザーの作成・GRANT |
| `POKEDEX_READER_DSN` | `pokedex-reader-dsn`(新規) | reader のユーザー名・パスワードを知るためだけ(GRANT 対象の作成専用。migrate 自身はこの DSN で接続しない) |
| `POKEDEX_IMPORTER_DSN` | `pokedex-importer-dsn`(新規) | 同上(importer 用) |
| `POKEDEX_DATABASE_DSN` | `pokedex-migrator-dsn`(新規) | 実際のスキーマ migration(`Up`)の接続先。プロビジョニング対象の1つでもある(自分自身を作る) |

reader/importer/migrator いずれのパスワードも、DSN 文字列自体に埋め込まれた値をそのまま使う
(`mysql.ParseDSN` でユーザー名・パスワードを取り出す)。パスワード専用の Secret キーを別途持たない
(同じ値を2箇所に書く二重管理を避ける)。

### 4. `db.Provision` は冪等・ローテーション対応にする

```go
// RoleGrant は1ユーザーぶんのプロビジョニング指定。DSN からユーザー名・パスワードを取り、
// Privileges をそのまま GRANT 文に埋め込む(呼び出し側の定数だけを渡す。ユーザー入力は通さない)。
type RoleGrant struct {
    DSN        string // mysql.ParseDSN できる形式。User/Passwd を使う
    Privileges string // 例 "SELECT" / "SELECT, INSERT, UPDATE, DELETE"
}

func Provision(rootDSN string, roles []RoleGrant) error
```

各ロールについて、次の SQL を(DB 名は `rootDSN` の DBName から取る)順に実行する:

1. `` CREATE USER IF NOT EXISTS 'user'@'%' IDENTIFIED BY 'password' `` — 無ければ作る
2. `` ALTER USER 'user'@'%' IDENTIFIED BY 'password' `` — 既にあってもパスワードを現在の Secret の値に
   揃える(Secret の値をローテーションして再実行すれば新しいパスワードに切り替わる。issue の
   「1つをローテーションしても他用途を巻き込まない」を満たす)
3. `` REVOKE ALL PRIVILEGES, GRANT OPTION FROM 'user'@'%' `` — 前回までに付いていた権限をいったん
   すべて剥がす(バージョン間で権限セットが変わっても蓄積しない。新規ユーザーで権限が無い状態からの
   REVOKE はエラーにならないことを実装時に確認する。エラーになる MySQL 版があれば「未 GRANT の
   REVOKE は無視してよいエラー」として実装で吸収する)
4. `` GRANT <Privileges> ON `dbname`.* TO 'user'@'%' `` — 決定1の権限セットちょうどを与える

ユーザー名・DB名は Go 側の定数・DSN の構造体フィールドから来る値であり、SQL 文字列への埋め込みは
`fmt.Sprintf` で行うが、**パスワードだけが「値」として埋め込まれる変数**になる。パスワードは
`scripts/up.sh` が `openssl rand -hex 16` で生成する 16 進文字列(英数字のみ)に限定されており、
SQL の引用符・バックスラッシュを含み得ないため、識別子と同様に埋め込んでも injection の余地が無い
(この前提を `db.Provision` のコメントと `up.sh` 双方に明記する)。

却下: `database/sql` のプレースホルダ(`?`)で値を渡す — MySQL の `CREATE USER`/`GRANT` は
identifier・value ともにプレースホルダを使えない構文(DDL 相当の DCL)なので、Go 標準ドライバの
プレースホルダは使えない。上記のとおりパスワードの文字種を制限することで安全性を確保する。

### 5. `scripts/up.sh`: 新規クラスタは4つの DSN を一度に作る。既存クラスタは差分だけ追記する

新規(Secret が無い)ときは、root パスワードに加えて reader/importer/migrator の3つのパスワードを
`openssl rand -hex 16` で生成し、4つの DSN(`pokedex-dsn`=root・`pokedex-reader-dsn`・
`pokedex-importer-dsn`・`pokedex-migrator-dsn`)を一度に `kubectl create secret generic` する。

既存(Secret が既にある)ときは、**新しい3つのキーが無いときだけ**追記する
(`kubectl -n pokecalc get secret mysql-auth -o jsonpath='{.data.pokedex-reader-dsn}'` が空かどうかで判定)。
`kubectl patch secret mysql-auth --type=merge -p '{"data":{"<key>":"<base64>"}}'` は `data` マップの
指定したキーだけを追加・上書きし、他のキーには触れない(JSON merge patch の map フィールドの性質)。
これにより、issue の「既存 PVC でも安全に移行できる」を、**Secret の作り直し・DB の再作成なしに**
満たす(移行後の最初の `make up` → `pokedex-migrate` Job の実行で、3ユーザーが冪等に作成される)。

### 6. `deploy/k8s/base/pokedex/` の各マニフェストの `secretKeyRef` を差し替える

- `deployment.yaml`(pokedex-svc): `POKEDEX_DATABASE_DSN` ← `pokedex-reader-dsn`
- `cronjob-import.yaml`(importer): `POKEDEX_DATABASE_DSN` ← `pokedex-importer-dsn`
- `job-migrate.yaml`(migrate): `POKEDEX_PROVISION_DSN` ← `pokedex-dsn`(root。名前を変える)、
  `POKEDEX_DATABASE_DSN` ← `pokedex-migrator-dsn`(名前は維持、中身の参照先キーを変える)、
  `POKEDEX_READER_DSN` ← `pokedex-reader-dsn`、`POKEDEX_IMPORTER_DSN` ← `pokedex-importer-dsn`(新規)
  wait-for-mysql の initContainer は root(`mysql-root-password`)のまま変更しない
  (3ユーザーは migrate の主コンテナがプロビジョニングするまで存在しないため、initContainer の
  `mysqladmin ping` に使える資格情報は root しかない。鶏卵関係。ping はデータに触れない操作)。

### 7. ローカル `make dev`/`make test-db` は対象外(root のまま)

issue の変更範囲は「`scripts/up.sh`、local MySQL 初期化、pokedex の Deployment/CronJob/Job」で、
`scripts/dev.sh`・`scripts/db-local-up.sh`・`make test-db` の `POKEDEX_TEST_DSN` は含まれない。
これらは個人のローカル開発ループ・テスト専用で、公開 Pod として侵害される対象ではないため、
root DSN を直接使う今までの動きを変えない(`POKEDEX_PROVISION_DSN` 未設定なら `migrate up` は
プロビジョニングをスキップする。決定3)。`make test-db` の新しい権限テスト(受け入れ条件)は
`db.Provision` を明示的に呼んで検証し、CLI 全体の起動フローには依存しない。

### 8. cloud overlay への文書化(実装はしない)

`deploy/k8s/overlays/cloud/` には mysql・Secret 自体が無い(マネージド DB を使う前提。ADR-0100 §9)。
本 ADR は、cloud overlay を実装する際に同じ4つの DSN キー名(`pokedex-dsn`〈provision〉・
`pokedex-reader-dsn`・`pokedex-importer-dsn`・`pokedex-migrator-dsn`)を Secret の契約として
踏襲することを推奨としてここに記録するだけで、cloud 側の実装(マネージド DB でのユーザー作成手順)は
このタスクの範囲外(P7 系のクラウド移行タスクで扱う)。

## 受け入れ条件

1. `db.Provision(rootDSN, roles)` が3ロールぶんの `CREATE USER IF NOT EXISTS`・`ALTER USER`・
   `REVOKE ALL`・`GRANT` を実行し、2回連続で呼んでもエラーにならない(冪等)。
2. パスワードを変えた `RoleGrant` で再度呼ぶと、新しいパスワードでの接続だけが成功する
   (ローテーション)。
3. 実 MySQL(`-tags mysql`。`make test-db`)で、`pokedex_reader` 相当のユーザーが SELECT に成功し
   INSERT・DELETE・CREATE TABLE に失敗すること、`pokedex_importer` 相当のユーザーが
   INSERT・DELETE に成功し CREATE TABLE に失敗することを確認する。
4. `pokedex_migrator` 相当のユーザーで `db.Up`(CREATE TABLE を含む migration 一式)が最後まで
   成功する。
5. `services/pokedex/cmd/migrate up` は `POKEDEX_PROVISION_DSN` が設定されているときだけ
   プロビジョニングを行い、無いときは今までどおり直接 `Up` する(後方互換)。
6. `scripts/up.sh` は新規クラスタで4つの DSN キーを一度に作り、既存クラスタでは無いキーだけを
   `kubectl patch` で追記する(既存の `mysql-root-password`・`pokedex-dsn` の値は変えない)。
7. `deployment.yaml`・`cronjob-import.yaml`・`job-migrate.yaml` それぞれの `secretKeyRef` が
   決定6のとおりに更新され、pokedex-svc・importer の Pod から root 資格情報(`pokedex-dsn`・
   `mysql-root-password`)への参照が無くなる。
8. Secret の値(パスワード・DSN)がコマンドライン引数・ログ・Git 管理ファイルへ出ない
   (既存の `TestImportCronJobPodSecurityAndWiring` 相当の検査を pokedex Deployment にも広げる)。
9. `make test-db`・`make lint`・`make k8s-render` が成功する。

### 受け入れ条件の確定(spec-writer 追記。テストの対応)

上の 1〜9 を、次のテストで固定する(実装前は失敗する)。

| 条件 | テスト |
|---|---|
| 1・4 権限セット・冪等・古い権限の剥奪・新規ユーザー | `db/grants_test.go` `TestRolePrivilegeConstants`、`db/grants_mysql_test.go` `TestProvisionIsIdempotent`・`TestProvisionRevokesStalePrivileges`・`TestProvisionFreshUsersFromScratch` |
| 2 ローテーション(他ロールを巻き込まない) | `TestProvisionRotatesPassword` |
| 3 reader/importer の権限境界(ユーザー管理不可も含む) | `TestReaderPrivilegeBoundary`・`TestImporterPrivilegeBoundary` |
| 4 migrator のフル migration(Up・DownAll・再 Up。CREATE USER/CREATE DATABASE 不可) | `TestMigratorRunsFullMigration` |
| 5 `migrate up` の分岐 | `cmd/migrate/main_test.go` 全体 |
| 6 up.sh の新規作成・差分追記 | `db/layout_test.go` `TestUpScriptCreatesRoleDSNKeys`・`TestUpScriptPatchesMissingKeysOnExistingSecret` |
| 7 manifest の secretKeyRef | `TestPokedexDeploymentUsesReaderDSNOnly`・`TestMigrateJobCredentialWiring`・`TestCredentialKeysReferencedOnlyByTheirOwners`、`importer/cronjob_layout_test.go` `TestImportCronJobPodSecurityAndWiring`、`cmd/pokedex/manifest_test.go` `TestManifestPokedexDeployment` |
| 8 値を argv・ログに出さない | `TestUpScriptKeepsSecretValuesOutOfArgvAndLogs`、各テストのパスワード非露出の検査 |
| 10(追加)危険な入力を接続前に拒否 | `db/grants_test.go` `TestProvisionRejectsUnsafeInputBeforeConnecting`・`TestProvisionAcceptsValidInputAndReachesConnect` |

追加の受け入れ条件 10: `db.Provision` は接続する前に入力を検査し、違反は `db.ErrInvalidRoleGrant`
(`errors.Is` で判定できる)を返す。エラー文にパスワードを含めない(下の申し送り2・3)。

## 実装時の申し送り(spec-writer による検証、2026-09-24)

1. **新規ユーザーへの `REVOKE ALL PRIVILEGES, GRANT OPTION` はエラーにならない(確認済み)。**
   `mysql:9.7.2` の使い捨てコンテナで、`CREATE USER IF NOT EXISTS` 直後の権限なしユーザーに対して
   実行し、エラーも warning も出ないことを確認した(`SHOW WARNINGS` が空)。よって特定のエラー番号を
   無視する処理は**入れない**(入れると本物の失敗を隠す)。版を上げたときの退行は
   `TestProvisionFreshUsersFromScratch` が検知する。なお既存ユーザーへの `CREATE USER IF NOT EXISTS`
   は Note 3163 を出すだけでエラーではない。
2. **パスワードの埋め込みを「up.sh が hex で作る」前提だけに頼らない。** `Provision` は汎用の関数で、
   cloud overlay(決定8)では up.sh 以外が DSN を作りうる。コメントでの前提明記に加え、`Provision` 自身が
   接続前に次を検査して `ErrInvalidRoleGrant` を返すこと:
   パスワードは `^[A-Za-z0-9]{16,}$`、ユーザー名は `^[a-z][a-z0-9_]{0,31}$`、DB 名は `^[A-Za-z0-9_]+$`、
   権限は `SELECT, INSERT, UPDATE, DELETE, CREATE, ALTER, DROP, INDEX, REFERENCES` の部分集合を
   カンマ区切りで並べたものだけ(`ALL`・`GRANT OPTION`・`CREATE USER`・任意の SQL 断片を拒否)。
   これで識別子・値・権限のどれにも引用符・バッククォート・バックスラッシュ・`;` が入り得なくなる。
   決定8の cloud の契約にも「パスワードは英数字16文字以上」を含める。
3. **ロールのユーザーが root DSN のユーザーと同じ・`root`・`mysql.*` のときは拒否する(致命的な穴の塞ぎ)。**
   mysql イメージには `root@'%'` があり、誤って `POKEDEX_READER_DSN` に root の DSN を入れると手順3の
   `REVOKE ALL ... FROM 'root'@'%'` が root の権限を剥がし、クラスタの DB を管理できなくなる。
   同じユーザーを2回指定する・ロールの DSN の DB 名が root DSN の DB 名と違う(GRANT 先と接続先が
   食い違う)も拒否する。
4. **up.sh の既存の `--from-literal` は受け入れ条件8(argv に出さない)に反する。** `ps` で値が見える。
   新規作成は `kubectl apply -f -`(標準入力の manifest。`stringData` を使う)か `--from-env-file` の
   一時ファイル、差分追記は `kubectl patch --type=merge --patch-file <一時ファイル>` にする。
   一時ファイルは `mktemp`・`chmod 600`・`trap 'rm -f ...' EXIT` で必ず消す。`set -x` を使わない。
   `data` に base64 を入れる場合、GNU の `base64` は76桁で折り返して JSON を壊すため、`stringData` を
   使って base64 を避けるのが簡単(merge patch の `stringData` が `data` に反映されることは実装時に
   k3d で確認する)。
5. **`cmd/migrate` はテストのため環境を差し替え可能にする**(`cmd/import` の `cliEnv` と同じ流儀)。
   契約は `services/pokedex/cmd/migrate/main_test.go` の冒頭コメントのとおり
   (`cliEnv{Stdout, Stderr, Getenv, Provision, Up, Version, DownAll}` と `run(args, env) int`)。
   `POKEDEX_PROVISION_DSN` があるのに `POKEDEX_READER_DSN`/`POKEDEX_IMPORTER_DSN` が欠けていたら
   何も実行せず終了コード 2、欠けた環境変数の**名前**だけを stderr に出す。DSN・パスワードは出さない
   (`mysql.ParseDSN` のエラーは DSN を含まないが、呼び出し側で DSN を `%v` に混ぜない)。
   ロールの順は reader・importer・migrator、migrator の DSN は `POKEDEX_DATABASE_DSN` と同じ値。
6. **権限定数は `db` パッケージに置く**(`db.ReaderPrivileges`・`db.ImporterPrivileges`・
   `db.MigratorPrivileges`)。CLI と実 DB テストが同じ定数を使い、テストが本番の権限セットを検査する。
7. **`make test-db` の前提**: `POKEDEX_TEST_DSN` は `CREATE USER`・`GRANT OPTION` を持つユーザー
   (`make db-local-up` の root)であること。テストはグローバルな名前空間に `pokedex_t_*` のユーザーを
   作り、前後で `DROP USER IF EXISTS` する。
8. **既存クラスタの移行時の短い空白**: 同じ overlay apply で Deployment が reader の DSN に切り替わる一方、
   reader ユーザーは migrate Job の完了まで存在しない。pokedex-svc は `sql.Open` だけで起動し DB 操作は
   503 を返す設計なので、Job 完了後に自然に回復する(Pod の再起動は不要)。また `Provision` の
   REVOKE → GRANT の間は各ユーザーが一瞬権限を失うが、1回の Job で数ミリ秒で、ローカル用途では許容する。
9. **ADR の「影響」に漏れていたテスト**: `services/pokedex/cmd/pokedex/manifest_test.go` は Deployment の
   DSN キーを `pokedex-dsn` に固定していたため、`pokedex-reader-dsn` に変更した(弱めたのではなく、
   新しい設計に合わせて主張を強めた)。`importer/cronjob_layout_test.go` も同様に `pokedex-importer-dsn`
   に変え、root 等の他キーを参照しない検査を足した。

## 却下したその他の案

- **Secret を3つ(reader/importer/migrator)に分ける**: issue は「できれば」であり must ではない。
  この構成では Pod ごとの RBAC(ServiceAccount・Role・RoleBinding)を新設しないと「1つの Secret を
  複数の Pod が読める」状態は変わらず(`automountServiceAccountToken: false` の現行方針で
  ServiceAccount 自体を使っていないため)、実効的な隔離が増えない割に manifest が3倍になる。
  1つの Secret に用途別キーを分ける現行案でも「キー単位のローテーション」は成立する
  (`kubectl patch` で1キーだけ更新→対象 Deployment だけ再起動)。
- **`pokedex_migrator` に `CREATE USER` を持たせて自己完結させる**: 受け入れ条件「ユーザー管理・
  DB作成・DDL権限を持たない」の対象は importer だが、migrator についても「migration に必要な
  DDL/DML だけ」と書かれており、ユーザー管理はその範囲外。root を残す決定2のほうが受け入れ条件に
  忠実。
- **initContainer の `wait-for-mysql` を新しいユーザーに切り替える**: 決定6の鶏卵問題により不可能
  (ユーザーがまだ存在しない)。

## 影響

- 変更: `services/pokedex/db/`(新規 `grants.go` 相当・`cmd/migrate/main.go`)、`scripts/up.sh`、
  `deploy/k8s/base/pokedex/{deployment,cronjob-import,job-migrate}.yaml`、
  `services/pokedex/db/layout_test.go`・`cronjob_layout_test.go`(新しい secretKeyRef の検査)。
- 変更しない: `services/pokedex/db/query/pokedex.sql`・importer の SQL 操作・API 契約・
  マスタの内容(issue の宣言どおり)。
- 新規クラスタは影響なく起動する。既存クラスタは次回 `make up` で Secret にキーが追記され、
  次回の `pokedex-migrate` Job 実行で3ユーザーが作成される(DB・PVC の再作成は不要)。

## 追記(2026-09-24): 独立レビュー(critic)の軽微指摘を反映

critic は PASS(重大・重要な指摘なし)。以下の軽微指摘のうち、安全性に関わる3点を反映した:

- **`grants_test.go` にテストの抜けがあった**: 「ロールのユーザーが root DSN のユーザーと同じ」を
  拒否する分岐(`cfg.User == rootCfg.User`)が、既存のケースでは root DSN のユーザーが常に
  文字列 `"root"` だったため、実質的に「ユーザーが `"root"` という名前そのもの」の分岐
  (`cfg.User == "root"`)としか検証できていなかった。root DSN のユーザーが `admin` 等の
  別名で運用される場合の分岐を単独で検証するケースを追加した。
- **`scripts/check-publishable.sh` の `B_KEYVALUE_ALLOW` に加えた2つ目の代替
  (`: \{識別子,$`)が汎用的すぎた**: `layout_test.go` の map リテラルの変数名を
  `keyRootPassword` → `keyRootPW` に変え(「秘密らしき文字列」の語彙 `password` を含まない名前にする)、
  2つ目の代替そのものを削除した。B の検出能力を広く緩めていた箇所を無くした。
- **`scripts/up.sh` の既存キー確認が `2>/dev/null || true` でエラーを握り潰していた**:
  jsonpath はキーが無いだけなら空文字・終了コード0を返すことを確認済みなので、コマンド自体が
  失敗する(API サーバ疎通不可・権限不足等)場合と区別する必要が無い。`|| true` を削除し、
  `set -e` にそのまま止めさせるようにした(一時的なエラーを「キーが無い」と誤認して
  既存のパスワードを意図せずローテーションする事故を防ぐ)。

残りの軽微指摘(up.sh 向け静的テストの一部が緩い・エラー文言の精度)は、実クラスタでの
実地検証(決定8の移行パス確認を含む)で実際の挙動は正しいことを確認済みのため、
テストの厳格化は今回は見送り、将来の改善候補として記録するに留める。

## 追記(2026-09-24 その2): `check-publishable.sh` の B_KEYVALUE_ALLOW 簡素化後に見つかった
## 別の誤検知を、識別子の改名で解消

上の追記で `B_KEYVALUE_ALLOW` の2つ目の代替(汎用的すぎた map リテラル許可)を削除した後、
`make lint` を再実行したところ、それとは別に既存の主検出パターン(`password|passwd|...` を含む
識別子の後に `[:=]` と8文字以上の値が続く形)が、`grants.go`/`main_test.go` の Go の識別子名
(`passwordPattern` という変数名・`validatedRole.password` というフィールド名・
`fakePasswords` というテスト変数名)に対して誤検知することが分かった(値ではなく識別子名が
たまたま検出パターンの形に合致しただけで、実際の秘密ではない)。

`keyRootPassword` → `keyRootPW` で行ったのと同じ対処(検出パターンの語彙 `password`/`passwd` を
含まない名前に改名する)を適用した: `passwordPattern` → `pwPattern`、構造体フィールド
`password` → `pw`、テスト変数 `fakePasswords` → `fakePWs`。ロジック・テストの主張は一切変えていない
(`go build`/`go vet`/`go test`/`make test-db` で確認済み)。`B_KEYVALUE_ALLOW` 側に新しい例外を
足すのではなく、コード側の命名を変える方針を一貫させた(検出能力を広げない)。

## 追記(2026-09-25): importer の書き込み権限を表単位にした(ADR-0125)

決定1の `pokedex_importer` の行は ADR-0125 で置き換えた。`SELECT` は `pokedex.*` のまま、`INSERT, UPDATE, DELETE` は
`schema_migrations` を除く表ごとに付ける(issue #312。importer の資格情報で migrate の状態を壊せないようにする)。
決定3の「プロビジョニング → 移行」は「プロビジョニング → 移行 → importer の付け直し」になる。
