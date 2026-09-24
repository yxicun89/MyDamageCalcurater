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
