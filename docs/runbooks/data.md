# データレーンの手順書(ローカル k3d)

マスタの取得から MySQL への投入までを、ローカルの k3d で確かめる。設計の理由は
[`services/pokedex/README.md`](../../services/pokedex/README.md)・[`tools/importer/README.md`](../../tools/importer/README.md)。

## replica を増やすときの接続予算(issue #112・ADR-0112)

pokedex の `deploy/k8s/base/pokedex/deployment.yaml` は接続プールの上限
(`POKEDEX_DB_MAX_OPEN_CONNS`、コードの既定値は `main.go` の `defaultDBMaxOpenConns`)を
Pod ごとに持つ。`pokedex` の `replicas` を増やす前に、
`replicas × MaxOpenConns + importer/migrate の接続予算 < DB の max_connections`
を確認すること(importer の CronJob・migrate の Job も同じ MySQL に別枠で接続する)。
今回は `max_connections` 自体の変更はしない。

## 1. k3d を起動する

```sh
cd "$(git rev-parse --show-toplevel)"
make up
```
確認: 最後の行が `job.batch/pokedex-migrate condition met`(`make up` が migrate Job の完了まで待つ)。

## 2. 取得する

```sh
cd "$(git rev-parse --show-toplevel)"
make import-fetch
```
確認: 最後に `fetch: ./fetch-pokeapi.mjs を実行` が表示され、エラーなく終了する(終了コード0)。

## 3. 試運転する(DB には触らない)

```sh
cd "$(git rev-parse --show-toplevel)"
make import-dry-run
```
確認: 最後の行が `import: -dry-run のため DB には投入しない`(終了コード0)。

## 4. 投入する(k3d 上の CronJob を手動で1回流す)

```sh
cd "$(git rev-parse --show-toplevel)"
created=$(make import-k8s)
echo "$created"
job_name=$(echo "$created" | grep -o 'pokedex-import-manual-[0-9]*' | tail -1)
kubectl -n pokecalc wait --for=condition=complete "job/$job_name" --timeout=600s
```
確認: 最後の行が `job.batch/<job名> condition met`。

## 5. DB に行が入ったことを確認する

```sh
cd "$(git rev-parse --show-toplevel)"
pw="$(kubectl -n pokecalc get secret mysql-auth -o jsonpath='{.data.mysql-root-password}' | base64 -d)"
kubectl -n pokecalc exec mysql-0 -- env MYSQL_PWD="$pw" mysql -u root -N -e "SELECT COUNT(*) FROM pokedex.species;"
unset pw
```
確認: 画面にパスワードは出ず、0 より大きい数値が1行表示される。

## 6. 手動実行と CronJob の重複を確かめる(issue #106 / ADR-0109)

`make import-k8s` の手動 Job と CronJob `pokedex-import` の定期 Job が同時に走っても、片方だけが最後まで処理を
進め、もう片方はロックを取れずに終わることを確かめる(`services/pokedex/importer/cronjob_lock_test.go` の
ネイティブ Go の統合テストとは別に、実物の CronJob・PVC・busybox の `flock` で確認する)。

### 6a. 定期実行が動いている間に手動実行を重ねる

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc create job --from=cronjob/pokedex-import pokedex-import-manual-race1
kubectl -n pokecalc create job --from=cronjob/pokedex-import pokedex-import-manual-race2
kubectl -n pokecalc wait --for=condition=complete job/pokedex-import-manual-race1 job/pokedex-import-manual-race2 --timeout=600s || true
kubectl -n pokecalc get jobs pokedex-import-manual-race1 pokedex-import-manual-race2
```
確認: 2つの Job がどちらも最終的に `COMPLETIONS` 1/1 になる(ロックに負けた側は終了コード1で
1〜2回自動再試行してから成功する。`kubectl -n pokecalc get pods -l job-name=pokedex-import-manual-race1`・
`...race2` の `RESTARTS` 列のどちらかが 0 より大きければ、実際に排他が働いた証拠)。

### 6b. Pod のログでロック競合を確認する(秘密が出ていないことも見る)

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc logs -l job-name=pokedex-import-manual-race1 --all-containers --prefix | grep -i 'ロック\|lock' || true
kubectl -n pokecalc logs -l job-name=pokedex-import-manual-race2 --all-containers --prefix | grep -i 'ロック\|lock' || true
```
確認: どちらかの Job のログに、ロックを取れず諦めた旨のメッセージが出ている(DSN・パスワード等の秘密は出ない)。

### 6c. 後片付け

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc delete job pokedex-import-manual-race1 pokedex-import-manual-race2
```
確認: 両方とも `job.batch "pokedex-import-manual-raceN" deleted` と表示される。

## 7. 後片付け(クラスタは残したまま止める)

```sh
cd "$(git rev-parse --show-toplevel)"
k3d cluster stop pokecalc
```
確認: 出力に `Stopped cluster 'pokecalc'` が含まれる(削除ではない。クラスタ削除の `make down` は人間の確認が要る)。

## migration が途中で失敗して dirty になったとき(issue #221)

`make up` の `pokedex-migrate` Job や `make deploy-latest` の migrate-up が失敗し、以後の up が
`Dirty database version N. Fix and force version.` で止まるときの戻し方。データを全部消す `make migrate-down` は使わない。

### a. 版を確かめる

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc port-forward svc/mysql 13306:3306 >/dev/null 2>&1 &
pf_pid=$!
sleep 2
export POKEDEX_DATABASE_DSN="$(kubectl -n pokecalc get secret mysql-auth -o jsonpath='{.data.pokedex-migrator-dsn}' | base64 -d | sed -E 's/@tcp\(mysql:[0-9]+\)/@tcp(127.0.0.1:13306)/')"
make migrate-version
```
確認: `version=N dirty=true` が表示される。この N が途中で失敗した migration の版(以下の `<N>`)。

### b. 途中まで作られたテーブルを確かめる

```sh
cd "$(git rev-parse --show-toplevel)"
ls services/pokedex/db/migrations/ | grep "^$(printf '%06d' <N>)_"
pw="$(kubectl -n pokecalc get secret mysql-auth -o jsonpath='{.data.mysql-root-password}' | base64 -d)"
kubectl -n pokecalc exec mysql-0 -- env MYSQL_PWD="$pw" mysql -u root -N -e "SHOW TABLES FROM pokedex;"
unset pw
```
確認: 版 `<N>` の `.down.sql` が `DROP TABLE IF EXISTS` だけか(ALTER だけの migration は1文なので途中までの状態が無い。c を飛ばして d へ)。
`.down.sql` にあるテーブルのうち、どれが既に作られているか。

### c. 版 N のテーブルを down で片付ける

`.down.sql` にある名前のテーブルは中身ごと消える(衝突の原因になった同名の既存テーブルも消える。中身が要るなら先に退避する)。

```sh
cd "$(git rev-parse --show-toplevel)"
pw="$(kubectl -n pokecalc get secret mysql-auth -o jsonpath='{.data.mysql-root-password}' | base64 -d)"
kubectl -n pokecalc exec -i mysql-0 -- env MYSQL_PWD="$pw" mysql -u root pokedex < services/pokedex/db/migrations/$(printf '%06d' <N>)_*.down.sql
kubectl -n pokecalc exec mysql-0 -- env MYSQL_PWD="$pw" mysql -u root -N -e "SHOW TABLES FROM pokedex;"
unset pw
```
確認: `.down.sql` にあるテーブルが一覧から消えている。

### d. dirty を解いて1つ前の版にする

`<N-1>` は `<N>` から1を引いた数(版 1 で止まったときは 0 = 未適用)。

```sh
cd "$(git rev-parse --show-toplevel)"
make migrate-force FORCE_VERSION=<N-1> CONFIRM_FORCE=pokedex
make migrate-version
```
確認: `force: 完了` の後に `version=<N-1> dirty=false`(0 のときは `version: 未適用`)。

### e. もう一度 up する

失敗の原因(権限・接続・衝突したテーブル)を取り除いてから流す。

```sh
cd "$(git rev-parse --show-toplevel)"
make migrate-up
make migrate-version
kill "$pf_pid"
unset POKEDEX_DATABASE_DSN pf_pid
```
確認: `up: 完了` の後に `version=<最新の版> dirty=false`(最新の版は `ls services/pokedex/db/migrations/` の最後の番号)。

### f. c でデータの入ったテーブルを消したときだけ、投入し直す

```sh
cd "$(git rev-parse --show-toplevel)"
created=$(make import-k8s)
job_name=$(echo "$created" | grep -o 'pokedex-import-manual-[0-9]*' | tail -1)
kubectl -n pokecalc wait --for=condition=complete "job/$job_name" --timeout=600s
```
確認: 最後の行が `job.batch/<job名> condition met`。

## `make migrate-down` の途中で止まったとき(issue #278)

down が失敗すると `version=<V> dirty=true` になる。このとき失敗したのは版 `<V+1>` の down(旧 000005 の down では V = 4)。
down を流し直すので、全テーブルが消えてよいこと(`make migrate-down` を流したときと同じ判断)を人が確かめてから行う。

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc port-forward svc/mysql 13306:3306 >/dev/null 2>&1 &
pf_pid=$!
sleep 2
export POKEDEX_DATABASE_DSN="$(kubectl -n pokecalc get secret mysql-auth -o jsonpath='{.data.pokedex-migrator-dsn}' | base64 -d | sed -E 's/@tcp\(mysql:[0-9]+\)/@tcp(127.0.0.1:13306)/')"
make migrate-version
```
確認: `version=<V> dirty=true` が表示される。

```sh
cd "$(git rev-parse --show-toplevel)"
make migrate-force FORCE_VERSION=<V+1> CONFIRM_FORCE=pokedex
make migrate-down CONFIRM_DESTROY=pokedex
make migrate-version
kill "$pf_pid"
unset POKEDEX_DATABASE_DSN pf_pid
```
確認: `down: 完了` の後に `version: 未適用`。

## importer の権限を付け直す(issue #312・ADR-0125)

importer の書き込み権限は表ごと(`schema_migrations` を除く)に付く。`make deploy-latest` の migrate-up が
プロビジョニング → up → importer の付け直しまで行うので、表を足す migration も `make deploy-latest` だけで追随する。
この変更より前から動いている k3d の DB は、`make deploy-latest` を1回流すと importer の権限が表ごとに絞られる。
付け直しは importer の権限をいったん全部外してから付けるので、その数秒の間に CronJob `pokedex-import` が走ると
権限不足で失敗することがある(CronJob は再試行する。失敗したら `make import-k8s` で流し直す)。

```sh
cd "$(git rev-parse --show-toplevel)"
make deploy-latest
```
確認: `== pokedex の DB(migrate-up)` の後に `up: 完了` と `version=<最新の版> dirty=false` が出る(DSN・パスワードは表示されない)。

```sh
cd "$(git rev-parse --show-toplevel)"
pw="$(kubectl -n pokecalc get secret mysql-auth -o jsonpath='{.data.mysql-root-password}' | base64 -d)"
kubectl -n pokecalc exec mysql-0 -- env MYSQL_PWD="$pw" mysql -u root -N -e "SHOW GRANTS FOR 'pokedex_importer'@'%';"
unset pw
```
確認: `` ON `pokedex`.* `` の行は `GRANT SELECT` だけで、`INSERT, UPDATE, DELETE` は表ごとの行にあり、`schema_migrations` の行が無い。

## pokedex イメージを共有レジストリへ push する(タイプバランスレーン issue #237 の依頼)

pokedex(server イメージ。`/pokedex` バイナリ、export サブコマンド持ち。ADR-0105 §5)を、balance・speed と共有する
クラスタ内レジストリ `balance-registry`(ADR-0018 §2・ADR-0605 §1)へ digest 固定で push する。タイプバランスレーンが
gitops overlay の initContainer から使う想定(issue #237)。balance・speed の `*-registry-push` と同じ方式
(`docker save` した tar を `crane` で push。Docker Desktop のデーモンからは Mac の localhost に届かないため)。

```sh
cd "$(git rev-parse --show-toplevel)"
make pokedex-registry-push
```
確認: 最後の行が `localhost:5000/pokecalc/pokedex@sha256:...`(digest 参照)。kubectl の context が `k3d-pokecalc`
でないときは、別クラスタへ push しないよう理由を出して止まる(apply・delete はしない。push のみ)。
この digest 参照を balance・speed の gitops overlay に書くのはタイプバランスレーンの担当(このリポジトリでは行わない)。
