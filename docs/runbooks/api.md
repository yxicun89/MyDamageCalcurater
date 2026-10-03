# API レーンの手順書(ローカル k3d)

calc-svc・gateway を k3d で動かして疎通を確かめる。設計は
[`services/calc/README.md`](../../services/calc/README.md)・[`services/gateway/README.md`](../../services/gateway/README.md)。

## 1. テストと静的検査を通す

```sh
cd "$(git rev-parse --show-toplevel)"
make test lint build check-publishable
```
確認: 最後の行が `check-publishable: 0 件(既定(A〜E))` で、エラーで止まらない。

## 2. k3d クラスタを起動する

```sh
cd "$(git rev-parse --show-toplevel)"
make up
```
確認: 出力に `job.batch/pokedex-migrate condition met` が含まれ、最後に `完了。http://localhost:8080 ...` の案内が出る
(migrate の後に importer のイメージの build が続くので、`condition met` は最後の行ではない)。
`kubectl -n pokecalc get pods` で `mysql-0` が `Running`、`pokedex` は手順3の投入が済むまで `0/1`(異常ではない)。
calc・gateway・web はこの時点では起動しない(手順4で入れる)。

## 3. マスタを投入する(初回だけ。投入済みならスキップしてよい)

calc-svc は pokedex-svc の内部 API からマスタを取るため、DB が空だと `/readyz` が `503` のままになる。

```sh
cd "$(git rev-parse --show-toplevel)"
created=$(make import-k8s)
job_name=$(echo "$created" | grep -o 'pokedex-import-manual-[0-9]*' | tail -1)
kubectl -n pokecalc wait --for=condition=complete "job/$job_name" --timeout=600s
```
確認: 最後の行が `job.batch/<job名> condition met`(`make import-k8s` は Job を作るだけで完了を待たないので `wait` まで流す)。
数秒〜10秒ほどで `kubectl -n pokecalc get pods` の `pokedex` が `1/1` になる。

## 4. calc・gateway を k3d にデプロイする

```sh
cd "$(git rev-parse --show-toplevel)"
make api-k3d-deploy
```
確認: 最後の2行が `deployment "calc" successfully rolled out` と `deployment "gateway" successfully rolled out`
(マスタ未投入だと `calc` の rollout が `--timeout=120s` でタイムアウトする。手順3を先にやり直す)。

## 5. スモークで疎通を確かめる

```sh
cd "$(git rev-parse --show-toplevel)"
make api-smoke
```
確認: 1行目が `api smoke: master=pokedex species=<key> move=<id> nature=<id>`
(pokedex-svc への配線が実際に検証できている。`master=example` は未接続時の fallback)。
2行目が `api smoke: calc=200 bulk=200 reverse=200 missing_header=400 invalid_header=400 pokedex=200 internal=404 balance=200 web=200`
(`balance`・`web` は各レーンが未デプロイなら `skipped`・`503` でもよい)。
1回目がロールアウト直後の一時的な失敗なら、`make api-smoke` をもう一度実行する。

## 6. k3d を使わずに起動して確かめる(開発ループ)

k3d の loadbalancer が 8080 を使っているときは `DEV_GATEWAY_PORT` で変える(既定 8080)。

```sh
cd "$(git rev-parse --show-toplevel)"
DEV_GATEWAY_PORT=18080 DEV_CALC_PORT=18081 make dev
```
別の端末で確認する。

```sh
curl -s http://localhost:18080/healthz
```
確認: `{"status":"ok"}` が返る(`Ctrl-C` で calc-svc・gateway とも止まる)。

## 7. 後片付け(クラスタは残したまま止める)

```sh
cd "$(git rev-parse --show-toplevel)"
k3d cluster stop pokecalc
```
確認: 出力に `Stopped cluster 'pokecalc'` が含まれる(削除ではない。クラスタ削除の `make down` は人間の確認が要る)。

## 8. 失効ジョブ(record-expire・team-expire)の手動実行と有効化

承認(ADR-0209 の「人間の確認」。ADR-0220 未決事項 0)が済むまで、local・cloud とも CronJob は `suspend: true` で、定期実行されない。
手動実行も実データの削除なので、承認の後か、人間が自分で実行する(承認前に Claude が流さない)。
手動で1回流す(TiDB と Secret がある k3d で。件数はログの `expire done` の1行。team は `record` を `team` に読み替える):

```sh
cd "$(git rev-parse --show-toplevel)"
JOB=record-expire-manual-$(date +%s)
kubectl -n pokecalc create job --from=cronjob/record-expire "$JOB"
kubectl -n pokecalc wait --for=condition=complete "job/$JOB" --timeout=10m
kubectl -n pokecalc logs "job/$JOB"
```
確認: ログに `"msg":"expire done"` があり、`remaining` が `false`(`true` なら続きは次回。もう一度流す)。

承認後に定期実行を有効にする: `deploy/k8s/overlays/local/cronjob-expire-suspend-patch.yaml` を消し、`overlays/local/kustomization.yaml` の `patches` の参照を外して `make up`(または overlay を apply)。

