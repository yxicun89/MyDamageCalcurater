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
確認: 出力に `job.batch/pokedex-migrate condition met`(`make up` が migrate Job の完了まで待つ)が含まれ、
最後に `完了。http://localhost:8080 ...` の案内が出る。この後に importer のイメージの build と案内文が続くので、
`condition met` は最後の行ではない。`kubectl -n pokecalc get pods` で `mysql-0` が `Running`、`pokedex` は初回の投入(§4)が済むまで `0/1`(異常ではない)。
`make up` は kubectl の context が `k3d-pokecalc` でなければ、何も apply せずに止まる。

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
確認: 最後の行が `job.batch/<job名> condition met`。数秒〜10秒ほどで `kubectl -n pokecalc get pods` の `pokedex` が `1/1` になる。
`make import-k8s` は Job を作るだけで完了を待たないので、`wait` までを1回で流す。

## 5. DB に行が入ったことを確認する

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc exec mysql-0 -- sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -u root -N -e "SELECT COUNT(*) FROM pokedex.species;"'
```
確認: 画面にパスワードは出ず、0 より大きい数値が1行表示される(パスワードは `mysql-0` の環境変数から読むので、コマンド引数や API サーバの記録に出ない)。

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
kubectl -n pokecalc get jobs pokedex-import-manual-race1 pokedex-import-manual-race2 \
  -o custom-columns=NAME:.metadata.name,SUCCEEDED:.status.succeeded,FAILED:.status.failed
kubectl -n pokecalc get pods -l 'job-name in (pokedex-import-manual-race1,pokedex-import-manual-race2)'
```
確認: 片方の Job の `FAILED` が 1 以上になっている(または Pod が2つ以上ある)。これがロックまたは引き渡し(取得段と投入段の間の予約)に負けた Pod で、排他が働いた証拠。
Job の Pod は `restartPolicy: Never` なので `RESTARTS` は常に 0 で、失敗すると別の Pod が作られる(`backoffLimit: 2`)。
負けた側は終了コード1で失敗してから作り直され、勝った側の完了後に成功する(`SUCCEEDED` 1)。
勝った側の処理が長引くと、負けた側は3回失敗して `Failed` で終わることもある。これも排他が働いた結果で異常ではない。
どちらの `FAILED` も空(0)なら、2つが重ならずに順に走っただけなので、もう一度 6a の最初から流す。

### 6b. Pod のログでロック競合を確認する(秘密が出ていないことも見る)

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc logs -l job-name=pokedex-import-manual-race1 --all-containers --prefix | grep 'ロック\|引き渡し' || true
kubectl -n pokecalc logs -l job-name=pokedex-import-manual-race2 --all-containers --prefix | grep 'ロック\|引き渡し' || true
```
確認: どちらかの Job のログに、`cronjob: 別の import が実行中(ロック ... を取得できない)` または
`cronjob: 別の Pod(...)の引き渡しが有効(期限まで待つ)。今回は諦める` のどちらかが出ている(DSN・パスワード等の秘密は出ない)。

### 6c. 後片付け

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc delete job pokedex-import-manual-race1 pokedex-import-manual-race2
```
確認: 両方とも `job.batch "pokedex-import-manual-raceN" deleted` と表示される。

## importer の PVC の容量(issue #111・ADR-0104 追記)

CronJob は取得の前に PVC の空きを確かめ(予約容量 既定 400 MiB)、足りなければ終了コード 3 で止まる(取得・DB 更新・prune のどれにも進まない)。
取り込みが成功した後にだけ、現在版+直前の成功版と report 直近 52 件を残して旧版を自動で消す。

k3d の `local-path` では空きの値がホストのディスクのものになり、PVC の 2Gi では制限されないため、容量の確認はローカルでは実質効かない
(PVC に容量の制限があるクラウドの StorageClass では効く)。以下はクラウド相当の環境での手順。

### a. 使用量を確かめる(副作用なし)

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc get jobs --sort-by=.metadata.creationTimestamp | grep pokedex-import
JOB=$(kubectl -n pokecalc get jobs -o name --sort-by=.metadata.creationTimestamp | grep pokedex-import | tail -1)
kubectl -n pokecalc logs "$JOB" --all-containers | grep 'importer-'
```
確認: 直近の Job のログに `importer-capacity: total=... used=... free=... reserve=...` と、版ごとの `usage` 行が出る。
(Job を新しく作ると取得から DB 投入までの取り込みが全部走る。確認だけなら作らない。)

### b. 閾値を超えたかを判定する

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc get pods | grep pokedex-import
JOB=$(kubectl -n pokecalc get jobs -o name --sort-by=.metadata.creationTimestamp | grep pokedex-import | tail -1)
kubectl -n pokecalc logs "$JOB" --all-containers | grep 'importer-capacity: 空き'
```
確認: 直近の Pod が `Init:Error`(取得段 `fetch` の失敗)で、ログに `importer-capacity: 空き ... byte が予約容量 ... byte を下回る` が出ていれば超過(終了コード 3)。
出ていなければ容量は足りているので、ここで終わり。

### c. PVC の拡張可否を確かめて広げる

```sh
cd "$(git rev-parse --show-toplevel)"
SC=$(kubectl -n pokecalc get pvc pokedex-import-cache -o jsonpath='{.spec.storageClassName}')
kubectl get storageclass "$SC" -o jsonpath='{.allowVolumeExpansion}{"\n"}'
```
確認: `true` なら拡張できる。まず `deploy/k8s/base/pokedex/pvc-import-cache.yaml` の `storage` を上げて通常の PR で入れる
(Argo CD が main を見ているため、base の yaml と実機の値がずれると差分として戻される)。急ぐときだけ先に実機へ patch し、同じ値を base に揃える。

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc patch pvc pokedex-import-cache -p '{"spec":{"resources":{"requests":{"storage":"4Gi"}}}}'
kubectl -n pokecalc get pvc pokedex-import-cache
```
確認: `CAPACITY` が 4Gi になる。

`false`(k3d の local-path)なら拡張できない。作り直すしかなく、取得キャッシュ・スナップショット・報告が消える(取得元から再取得される。
報告は戻らない)。CronJob を消すと、過去の Job とそのログも消える。**データ削除なので人間の確認が要る操作**。
確認を得てから、base の `storage` を上げた状態で次を流す。作り直すのは PVC `pokedex-import-cache` と CronJob `pokedex-import` の2つだけ
(overlay 全体は apply しない。他のサービスや migrate の Job まで作り直してしまうため。base を単独で apply すると namespace が付かないので、
overlay の描画からラベル `app.kubernetes.io/name=pokedex-import` の2つだけを取り出す)。

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc delete cronjob pokedex-import
kubectl -n pokecalc delete pvc pokedex-import-cache
kubectl kustomize deploy/k8s/overlays/local | kubectl apply -l app.kubernetes.io/name=pokedex-import -f -
```
確認: `kubectl -n pokecalc get pvc pokedex-import-cache` の `CAPACITY` が新しい値で、`STATUS` が `Bound`(または最初の Job 実行まで `Pending`)。

### d. 取り込みを成功させて旧版を prune する

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc create job --from=cronjob/pokedex-import pokedex-import-prune
kubectl -n pokecalc wait --for=condition=complete job/pokedex-import-prune --timeout=900s
kubectl -n pokecalc logs job/pokedex-import-prune --all-containers | grep importer-prune
```
確認: `importer-prune: 削除 <相対パス> <byte> byte` の行と、最後の `削除 N 件・回収 M byte・残量 K byte` が出る。
消えるのは現在版・直前の成功版以外の `.cache/<source>/<版>`・`<source>/<版>` と、52 件より古い `reports/import-*.json` だけ。
`upstream/`・`reports/latest*`・ロック・台帳は消えない。

### e. 失敗後に再実行する

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc delete job pokedex-import-prune --ignore-not-found
kubectl -n pokecalc create job --from=cronjob/pokedex-import pokedex-import-retry
kubectl -n pokecalc wait --for=condition=complete job/pokedex-import-retry --timeout=900s
kubectl -n pokecalc delete job pokedex-import-retry
```
確認: Job が `complete` になる。途中で止まった取得は次回に自己回復し(#102)、prune の中断は再実行で残りを消す(何度流しても同じ結果)。

## importer が終了コード 3 で止まったとき(ADR-0104 §3・§9)

終了コード 3 は「再試行しても同じ結果になり、人の対応が要る」。DB は変わらず、Job は再試行されずに `Failed` で終わる
(`podFailurePolicy` が 2・3 を `FailJob` にしている)。ロックに負けた(終了コード 1)のとは別。

### a. 止まった理由を読む

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc get pods -l app.kubernetes.io/name=pokedex-import --sort-by=.metadata.creationTimestamp
kubectl -n pokecalc logs "$(kubectl -n pokecalc get pods -l app.kubernetes.io/name=pokedex-import --sort-by=.metadata.creationTimestamp -o name | tail -1)" | tail -20
```
確認: 直近の Pod が `Error` で、ログの最後の `import:` か `importer-capacity:` の行に理由が出ている。次の表で対応する。

| ログの行 | 意味 | 次にすること |
|---|---|---|
| `importer-capacity: 空き ... を下回る` | PVC の空き不足 | 上の「importer の PVC の容量」の c・d |
| `import: 消える ID: ...` | DB にある ID が新しい出力から消える | 下の「ID が消えて CronJob が終了コード 3 で止まったとき」 |
| `import: 人間の裁定が必要な食い違いが N 件ある` | 照合の Blocker(上流の更新で、裁定済みの件数・集合や種族・技の値が変わった) | b |
| `既存の種族の key が変わる投入` | `ErrKeyChanged`(上流の formeOrder の並びが変わった) | c |
| `DB のスキーマが未整備` | migrate が済んでいない DB(`ErrSchemaNotReady`) | `make deploy-latest`(migrate-up まで流す)。dirty なら下の「migration が途中で失敗して dirty になったとき」 |
| そのほかの `import:` の行(`ErrInvalidInput`・`ErrInvalidData`) | 入力の誤り(config・取得データの矛盾) | 行のとおりに入力を直す PR を出す |
| `import: 投入に失敗:` に MySQL のエラー番号(1062・1264・1406・1452・3819)、または効果データの検証エラー(`ErrInvalidEffect`) | 取得データか migration の制約との不整合。DB は1トランザクションなので巻き戻っている | 行の内容を issue に書き、取得データ・効果データ・migration のどれを直すかを決めて PR を出す |

### b. 照合の Blocker のとき

CronJob のイメージは Git の `data/importer/config.json` の版で作られている。同じ版をホストで再現して報告を読む(DB には触れない)。

```sh
cd "$(git rev-parse --show-toplevel)"
make import-fetch
make import-dry-run
cat data/generated/reports/latest-summary.txt
```
確認: 出力に `blockers <種類>: <件数>` の行がある。`data/generated/reports/latest.json` の `blockers` に ID と詳細がある。
Git の config がクラスタのイメージと同じ版であること(`git log -1 -- data/importer/config.json` の後にイメージを作り直していること)を確かめる。
どう直すかは人が決める。いずれも通常の PR で行い、クラスタ上の Pod や DB を直接直さない。

- 上流の版を上げたことが原因なら、`data/importer/config.json` の `sources` を前の版に戻す PR を出す。
- 食い違いが正しく、使えるデータとして受け入れるなら、ADR-0002 に裁定を追記してから `reconcile.verdicts` を更新する PR を出す(ADR-0103 §5)。

PR が main に入ったら、イメージを入れ替えて流し直す。

```sh
cd "$(git rev-parse --show-toplevel)"
git switch main && git pull --ff-only
make deploy-latest
created=$(make import-k8s)
job_name=$(echo "$created" | grep -o 'pokedex-import-manual-[0-9]*' | tail -1)
kubectl -n pokecalc wait --for=condition=complete "job/$job_name" --timeout=600s
```
確認: 最後の行が `job.batch/<job名> condition met`。

### c. 種族の key が変わるとき(ErrKeyChanged)

ログの行は `showdown_id "<ID>" の key が "<旧>" → "<新>"`(または `key "<key>" の showdown_id が ...`)。
team-svc・record-svc・端末に保存された key が別の種族を指さないよう、投入は DB(台帳 `species_key_ledger` を含む)を変えずに止まる。
承認用のフラグは無い(`-allow-removed` でも通らない。ADR-0131)。人が次のどちらかを決める。

- 上流の並びが変わっただけなら、`data/importer/config.json` の `sources` を前の版に戻す PR を出す(b と同じ流し方)。
- 保存済みの key を壊してでも新しい key にする判断は、ADR を書いてから行う。DB と台帳の書き換えを伴い、この手順書の範囲外(人間の確認が要る)。

## PVC や DB を失ったとき(pokedex のマスタは再生成できる)

pokedex の DB の中身は、Git に固定した版の取得物から importer が作り直せるので、バックアップは取らない。
`pokedex-import-cache`(取得キャッシュ・報告)も、取得元から取り直せる(過去の報告は戻らない)。
失うものは、DB の投入履歴(`data_versions`)と、importer の過去の報告だけ。record・team の DB(TiDB)はこの手順の対象外。

### a. クラスタごと失ったとき(`make down`・`k3d cluster delete`)

`make down` はクラスタ・PVC・Secret を消す。**データ削除なので人間の確認が要る操作**。消えた後は新しいクラスタを作る。

```sh
cd "$(git rev-parse --show-toplevel)"
make up
created=$(make import-k8s)
job_name=$(echo "$created" | grep -o 'pokedex-import-manual-[0-9]*' | tail -1)
kubectl -n pokecalc wait --for=condition=complete "job/$job_name" --timeout=600s
```
確認: 最後の行が `job.batch/<job名> condition met`。`make up` が Secret を新しく作り、migrate が DB と用途別ユーザーを作る。
取得はこの Job の中で行う(ネットワークが要る)。ここまでで `pokedex` が `1/1` になる。
calc・gateway・web・balance・speed・judge は `docs/verify-m1.md` の §3・§4(`make pokedex-export` → `make deploy-latest`)で入れ直す。

### b. MySQL の PVC(`data-mysql-0`)だけ失ったとき

Secret `mysql-auth` は残っているので、空の MySQL が同じ root パスワードで起動し、DB `pokedex` だけが作られる。
用途別ユーザーとテーブルは無いので、`make deploy-latest` の migrate-up(プロビジョニング込み)で作り直してから投入する。
PVC を消す操作は**人間の確認が要る**。

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc get pod mysql-0
make deploy-latest
```
確認: `mysql-0` が `Running`。`deploy-latest` の `== pokedex の DB(migrate-up)` の後に `up: 完了` と `version=<最新の版> dirty=false`。
DB が空の間 `pokedex` は Ready にならないので、`deploy-latest` は 180 秒待って「pokedex が Ready にならない」で止まる(想定どおり。
`up: 完了` はその前に出ている)。止まったら上の `import-k8s` と `wait` を流し、終わってからもう一度 `make deploy-latest` を実行する
(migrate-up は何度流しても同じ結果)。

```sh
cd "$(git rev-parse --show-toplevel)"
created=$(make import-k8s)
job_name=$(echo "$created" | grep -o 'pokedex-import-manual-[0-9]*' | tail -1)
kubectl -n pokecalc wait --for=condition=complete "job/$job_name" --timeout=600s
make deploy-latest
```
確認: `wait` が `condition met` で終わり、2回目の `deploy-latest` が `全サービスを <コミット> の内容で入れ替えた` で終わる
(balance・speed の read model が無ければ、先に `make pokedex-export`。`docs/verify-m1.md` §3)。

### c. `pokedex-import-cache` だけ失ったとき

何もしなくてよい。次の Job(`make import-k8s` か CronJob)が固定版を取り直す。
PVC を作り直す手順は、上の「importer の PVC の容量」の c にある。

## ID が消えて CronJob が終了コード 3 で止まったとき(issue #277・ADR-0131)

上流の更新で、DB にある種族 key・技/持ち物/特性の ID が新しい出力から消えると、投入は DB を変えずに終了コード 3 で止まる
(消えた key の別の種族での再利用は、承認しても止まる)。

### a. 消える ID を確かめる

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc logs "$(kubectl -n pokecalc get pods -l app.kubernetes.io/name=pokedex-import --sort-by=.metadata.creationTimestamp -o name | tail -1)" | grep "import:"
```
確認: `import: 消える ID: species:9002-002,move:teststrike` のように `<種類>:<ID>` が並ぶ。
保存済みの構築が使っている ID なら、消してよいかを人が判断する(使っていなければそのまま承認してよい)。

### b. 消えてよいときだけ、承認して手動 Job を1回流す

```sh
cd "$(git rev-parse --show-toplevel)"
job=pokedex-import-allow-removed
kubectl -n pokecalc create job --from=cronjob/pokedex-import "$job" --dry-run=client -o json \
  | jq --arg v "species:9002-002,move:teststrike" '.spec.template.spec.containers[0].env += [{name:"IMPORT_ALLOW_REMOVED",value:$v}]' \
  | kubectl -n pokecalc create -f -
kubectl -n pokecalc wait --for=condition=complete "job/$job" --timeout=900s
kubectl -n pokecalc delete job "$job"
```
`$v` には a で見た ID のうち承認するものだけを写す。実際には消えない ID を書くと終了コード 3(打ち間違い)で止まる。
確認: Job が `complete` になる。承認で消えた種族 key は台帳に残るので、後で別の種族に付く投入は引き続き止まる。
`wait` が timeout したら Job が止まっているので、a のコマンドでその Job のログを見て原因を確かめる。

## 引き渡しファイルが残って Job が終了コード 1 で待たされるとき(issue #301・ADR-0101 追記)

取得段 `fetch` が成功すると PVC に `.import.handoff` を書き、投入段 `import` の終了時に消す。Pod が強制終了(OOM・ノード停止)されると
残り、有効期限(既定 3600 秒)まで、別の Job は `cronjob: 別の Pod(...)の引き渡しが有効(期限まで待つ)。今回は諦める` で終了コード 1 になる。

### a. 実行中の Job が無いことを確かめる

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc get jobs,pods | grep pokedex-import
```
確認: `Running`・`Init:` の Pod が1つも無い(あるなら、その Job が終わるまで待つ。ここで止める)。

### b. 引き渡しファイルの中身を見る

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc run handoff-peek --rm -i --restart=Never --image=busybox:1.37.0 \
  --overrides='{"spec":{"containers":[{"name":"handoff-peek","image":"busybox:1.37.0","command":["sh","-c","cat /g/.import.handoff; date +%s"],"volumeMounts":[{"name":"g","mountPath":"/g"}]}],"volumes":[{"name":"g","persistentVolumeClaim":{"claimName":"pokedex-import-cache"}}]}}'
```
確認: `owner=<Pod 名>` と `expires=<epoch 秒>` の2行と、現在の epoch 秒が出る(ファイルが無ければ残っていない。`cat` が失敗するので、ここで終わり)。`expires` が現在より未来なら、その差の秒数だけ待てば自然に無視される。

### c. 待てないときだけ、手で消す

先に a をもう一度流し、実行中の Job が無いことを確かめ直す。

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc run handoff-rm --rm -i --restart=Never --image=busybox:1.37.0 \
  --overrides='{"spec":{"containers":[{"name":"handoff-rm","image":"busybox:1.37.0","command":["sh","-c","rm -f /g/.import.handoff && ls -a /g"],"volumeMounts":[{"name":"g","mountPath":"/g"}]}],"volumes":[{"name":"g","persistentVolumeClaim":{"claimName":"pokedex-import-cache"}}]}}'
```
確認: 一覧に `.import.handoff` が無い。そのあと Job を流し直せる。

## 取得段が終了コード 3 で止まったとき(取得物のハッシュ不一致。issue #222・ADR-0101 追記)

Pod が `Init:Error`(取得段 `fetch` の失敗)で、再試行されずに Job が失敗する。内容が `data/importer/config.json` の `integrity` と違うと止まる(DB には触れていない)。

### a. 実際のハッシュを見る

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc logs "$(kubectl -n pokecalc get pods -l app.kubernetes.io/name=pokedex-import --sort-by=.metadata.creationTimestamp -o name | tail -1)" -c fetch | grep '内容ハッシュが一致しない'
```
確認: `<ファイル名または showdown>: 内容ハッシュが一致しない。期待=... 実際=...` が出る。

### b. 上流の内容を確かめて、config.json を更新する PR を出す

上流(Showdown の固定コミット・PokeAPI の固定コミット)の該当ファイルが、コミットのとおりで改ざんされていないことを人が確かめる。
確かめたら `data/importer/config.json` の `integrity` の該当の値を a の「実際」に書き換えて PR を出す。確かめられない間は更新しない(投入は止まったまま)。

```sh
cd "$(git rev-parse --show-toplevel)"
make import-fetch
```
確認: config.json を直したあとで、取得が `fetch-pokeapi: ... snapshot.json を書いた` まで終了コード 0 で通る。

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
kubectl -n pokecalc exec mysql-0 -- sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -u root -N -e "SHOW TABLES FROM pokedex;"'
```
確認: 版 `<N>` の `.down.sql` が `DROP TABLE IF EXISTS` だけか(ALTER だけの migration は1文なので途中までの状態が無い。c を飛ばして d へ)。
`.down.sql` にあるテーブルのうち、どれが既に作られているか。

### c. 版 N のテーブルを down で片付ける(データ削除。人間の確認が要る操作)

`.down.sql` にある名前のテーブルは中身ごと消える(衝突の原因になった同名の既存テーブルも消える。中身が要るなら先に退避する)。
消してよいことを人が確かめてから流す。

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc exec -i mysql-0 -- sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -u root pokedex' < services/pokedex/db/migrations/$(printf '%06d' <N>)_*.down.sql
kubectl -n pokecalc exec mysql-0 -- sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -u root -N -e "SHOW TABLES FROM pokedex;"'
```
確認: `.down.sql` にあるテーブルが一覧から消えている。

### d. dirty を解いて1つ前の版にする(人間の確認が要る操作)

migration の記録(`schema_migrations`)を書き換える。スキーマは変えない。`CONFIRM_FORCE` に DB 名を書くことが確認の代わりで、
DB 名が接続先と違えば接続前に止まる。`<N-1>` は `<N>` から1を引いた数(版 1 で止まったときは 0 = 未適用)。

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
kubectl -n pokecalc exec mysql-0 -- sh -c "MYSQL_PWD=\"\$MYSQL_ROOT_PASSWORD\" mysql -u root -N -e \"SHOW GRANTS FOR 'pokedex_importer'@'%';\""
```
確認: `` ON `pokedex`.* `` の行は `GRANT SELECT` だけで、`INSERT, UPDATE, DELETE` は表ごとの行にあり、`schema_migrations` の行が無い。

## pokedex イメージを共有レジストリへ push する(タイプバランスレーン issue #237 の依頼)

pokedex(server イメージ。`/pokedex` バイナリ、export サブコマンド持ち。ADR-0105 §5)を、balance・speed と共有する
クラスタ内レジストリ `balance-registry`(ADR-0018 §2・ADR-0605 §1)へ digest 固定で push する。タイプバランスレーンが
gitops overlay の initContainer から使う想定(issue #237)。balance・speed の `*-registry-push` と同じ方式
(`docker save` した tar を `crane` で push。Docker Desktop のデーモンからは Mac の localhost に届かないため)。

```sh
cd "$(git rev-parse --show-toplevel)"
POKEDEX_REGISTRY_PUSH_CONFIRM=1 make pokedex-registry-push
```
確認: 最後の行が `localhost:5000/pokecalc/pokedex@sha256:...`(digest 参照)。kubectl の context が `k3d-pokecalc`
でないときは、別クラスタへ push しないよう理由を出して止まる(apply・delete はしない。push のみ)。
この digest 参照を balance・speed の gitops overlay に書くのはタイプバランスレーンの担当(このリポジトリでは行わない)。
