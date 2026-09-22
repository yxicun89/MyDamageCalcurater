# データレーンの手順書(ローカル k3d)

マスタの取得から MySQL への投入までを、ローカルの k3d で確かめる。設計の理由は
[`services/pokedex/README.md`](../../services/pokedex/README.md)・[`tools/importer/README.md`](../../tools/importer/README.md)。

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

## 6. 後片付け(クラスタは残したまま止める)

```sh
cd "$(git rev-parse --show-toplevel)"
k3d cluster stop pokecalc
```
確認: 出力に `Stopped cluster 'pokecalc'` が含まれる(削除ではない。クラスタ削除の `make down` は人間の確認が要る)。
