# ADR-0226: M2(NATS・TiDB・record・team)を k3d に実適用し、`make deploy-latest` に含める

- 状態: 実装済み(2026-10-03。実機の k3d〈Apple Silicon・arm64〉で適用・確認)
- 日付: 2026-10-03
- 関連: ADR-0211(TiDB のプロビジョニング。AC-T3・AC-T8)、ADR-0132(NetworkPolicy。「TiDB は未確認」)、ADR-0220(record・team の Deployment)、
  ADR-0806(コミット識別のタグでの k3d デプロイ)、docs/plan.md P5-1、docs/verify-m2.md、
  docs/ai-shared/decisions/2026-10-03-api-m2-k3d-deploy.md

## 背景

M2 のバックエンド(record-svc・team-svc。TiDB と NATS が前提)は実装・main 統合済みだが、共有 k3d には一度も実適用されていなかった
(TiDB Operator の Pod が無く、TidbCluster・record・team も無い)。そのため `docs/verify-m2.md` §2 の実機確認と、
Web の `make web-k3d-e2e`(`/api/record/frequent-opponents` が 503 で 2 件失敗)が通らなかった。
ADR-0211 は実適用(AC-T3・AC-T8)を「他レーンへの影響を確かめてから」と未実施にしていた。

## 実測したこと(Apple Silicon の k3d で、順に踏んだ問題)

1. **TiDB Operator の helm リポジトリが消えている**。`https://charts.pingcap.org/` は DNS が引けない(NXDOMAIN。2026-10-03)。
   `scripts/tidb-operator-bootstrap.sh` の `helm repo add` は `|| true` で握りつぶされており、後続の `helm repo update` が失敗していた。
   CRD(GitHub の raw)は取得できる。
2. **NetworkPolicy が operator を止める**。default-deny の下で、tidb-admin の controller-manager が PD の 2379 に届かず、
   `PD(s) are not healthy` のまま TiKV・TiDB が作られなかった(ADR-0132 が「未確認」とした経路)。
   TidbInitializer の Pod も TiDB の 4000 に届かなかった(許可の対象が record-migrate・team-migrate・record・team 等だけで、初期化 Pod が無かった)。
3. **TiKV が起動直後に OOMKilled を繰り返す**(limit 1Gi でも 1.5Gi でも)。単独の docker 実行で調べると、既定の設定で常駐メモリが
   約 2.2GiB まで増えて安定した(スレッドプール・block cache・memtable を絞る設定、jemalloc の THP・アリーナ設定、fd 上限の変更では変わらない。
   RocksDB・raft-engine は数十 MB で、割り当ての大半の出どころは特定できていない)。ADR-0211 §3.2 が AC-T3 で懸念した「TiKV の limit が厳しすぎないか」の答え。
4. **`tnir/mysqlclient`(amd64 専用)は Apple Silicon の k3d でも起動する**。Docker Desktop のエミュレーションで、init コンテナ(nc)も
   本体(python + MySQLdb)も動いた。つまり、依頼元が懸念した「amd64 専用で起動しない」は起きなかった。**実際の失敗は別の原因**:
   TiDB Operator の初期化スクリプトは `initSql` を**行ごとに** `cursor.execute()` へ渡すため、1行に複数の文
   (`CREATE DATABASE ...; CREATE DATABASE ...; SET GLOBAL ...;`)を書いた ADR-0211 のマニフェストは
   `Commands out of sync`(MySQLdb の 2014)で失敗した。しかもスクリプトは先に root のパスワードを設定する(`set password for 'root'`)ため、
   失敗した時点で root にパスワードが付き、TidbInitializer は**再実行できない**(スクリプトはパスワード無しの root で接続する)。

## 決定

### 1. TiDB Operator は、同じ版の Git タグから helm で入れる

`charts.pingcap.org` をやめ、GitHub のソースアーカイブ(`https://github.com/pingcap/tidb-operator/archive/refs/tags/v1.6.6.tar.gz`)の
`charts/tidb-operator` を `helm upgrade --install` へ直接渡す。アーカイブは **sha256 を固定して検証**する(不一致なら導入しない)。
版(v1.6.6)・operator イメージ・`--wait` は ADR-0211 のまま。CRD は従来どおり GitHub の raw から `--server-side` で適用する。
`helm repo add` の握りつぶしは廃止した(失敗を隠さない)。

### 2. NetworkPolicy を足す

- `allow-tidb-operator-ingress`(新設): tidb-admin の `app.kubernetes.io/name=tidb-operator`・`component=controller-manager` から、TiDB クラスタの Pod
  (`instance=pokecalc-tidb`・`managed-by=tidb-operator`)の 2379(PD health)・10080(TiDB status)・20180(TiKV status)へ。
  namespaceSelector と podSelector を同じ peer に置く(別 namespace の同名ラベルを通さない)。ラベルは k3d で実測した値。
- `allow-tidb-client-ingress`: 送信元に TidbInitializer の Pod(`app.kubernetes.io/name=pokecalc-tidb-initializer`。実測)を足す。
- `services/gateway/deploytest/networkpolicy_test.go` の許可表・拒否表に追加(許可: operator→pd/tidb/tikv、初期化 Pod→tidb。
  拒否: default namespace の同名ラベルの偽 operator→pd、operator→tidb の 4000、operator→mysql、初期化 Pod→mysql)。

### 3. TiKV の limit は 3Gi(request は据え置き)

`deploy/k8s/overlays/local/tidb/tidbcluster.yaml` の tikv の `limits.memory` を `1Gi` → `3Gi`。request(512Mi)は変えない(スケジューリングは request で決まる)。
ADR-0211 §3.2 の資源の見積り(limit 合計 2.03GiB)はこれで約 4GiB になる。Docker Desktop の VM は 7.75GiB で、他サービス・TiDB・PD・operator を含めた実測の
node メモリ使用は約 6GiB。余裕は小さい。メモリを減らせる設定が見つかるまでの暫定で、見つかったら下げる(未決事項)。

### 4. TidbInitializer は残し、`initSql` を1行1文にする(`tnir/mysqlclient` は arm64 でそのまま使える)

- `initSql` を `|-` のブロックにして1行に1文だけ書く。
- 代替案(TidbInitializer をやめ、`mysql:9.7.2`〈arm64 対応。migrate Job が既に使っているイメージ〉の Job で DB 作成・root パスワード設定をする)は、
  mysql 9.7 のクライアントが TiDB 8.5 に接続できることまで確かめた(復旧手順で実際に使った)。**採用しない理由**: エミュレーションで動くことが
  確かめられ、TiDB Operator が正式にサポートする経路を保てる。1回きりの初期化なので速度も問題にならない。
  **再実行できない**弱点は残る(下の復旧手順)。この弱点が運用で効いてくるようなら、冪等な Job(パスワード付きで接続を試し、だめなら無しで
  `ALTER USER` → `CREATE DATABASE IF NOT EXISTS`)へ切り替える。

### 5. `make deploy-latest` と `make up` は同じスクリプト `scripts/k3d-m2-deploy.sh` で M2 を入れる

- 新設 `scripts/k3d-m2-deploy.sh`(冪等・要 k3d context。`scripts/require-k3d-context.sh`): operator の導入 → tidb-root-auth → TidbCluster/TidbInitializer の適用と
  PD/TiKV/TiDB・TidbInitializer の完了待ち → record-db-auth・team-db-auth → record-migrate・team-migrate(古い Job を消して適用・完了待ち)→
  record・team・NATS・NetworkPolicy(`deploy/k8s/overlays/local-m2`)をコミット識別のタグ(`scripts/image-tag.sh`・`scripts/k3d-deploy-tagged.sh`)で apply。
  順序は ADR-0211 AC-T8(TidbInitializer の完了待ち → migrate Job)を守る。operator が非同期に作る StatefulSet は、出現を待ってから `rollout status` する
  (従来の up.sh は出現前に呼ぶと NotFound で即失敗する競合があった)。
- **非致命**: ある段が失敗しても、依存しない後続は続ける(TiDB が使えなくても NATS は入れる)。失敗は最後にまとめて表示し、**終了コード 1** で終わる。
  `deploy-latest` はこれを受けて他サービスの入れ替えを終えたうえで非ゼロで終わる(黙って成功扱いにしない)。`make up` は従来どおり警告だけで続行する
  (6レーン共通の入口のため。ADR-0211 §3.1)。calc・gateway・pokedex は計算を TiDB に依存しないので影響を受けない(絶対ルール5)。
- `deploy/k8s/overlays/local-m2`(新設): NATS・record・team・NetworkPolicy だけを描画する。共有の `overlays/local` を丸ごと apply すると、他レーンの Deployment の
  image が base の固定タグへ戻るため(local-api と同じ考え方)。失効 CronJob は承認までの `suspend: true` を引き続き当てる(kustomize は root の外のパッチを読めないので複製)。
- Secret は従来どおり `kubectl create`(`apply` しない)で作り、値を出力に出さない(`scripts/up-secrets_test.sh`・`scripts/k3d-m2-deploy_test.sh` で固定)。
- `scripts/up.sh` の M2 の部分(operator・Secret・TiDB・migrate)はこのスクリプトの呼び出しに置き換えた(重複を持たない)。

### 6. TiDB の既存の状態の復旧手順(TidbInitializer が `Failed` のとき)

初期化 SQL が途中で止まり root のパスワードだけ設定済みになった場合。DB のデータは消さない(PVC・TiKV には触れない)。
**k3d ローカル専用。cloud・本番では流さない**(root のパスワードを空に戻すため)。
(a) root のパスワードを空に戻す一回限りの Job を流し(`tidb-root-auth` は `secretKeyRef` で参照し、値は表示しない)、
(b) 失敗した TidbInitializer を削除して作り直す(`kubectl apply -k deploy/k8s/overlays/local/tidb`)。

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl config current-context   # k3d-pokecalc であること
kubectl apply -f - <<'EOF'
apiVersion: batch/v1
kind: Job
metadata:
  name: tidb-root-reset-once
  namespace: pokecalc
spec:
  backoffLimit: 0
  ttlSecondsAfterFinished: 600
  template:
    metadata:
      labels:
        app.kubernetes.io/name: pokecalc-tidb-initializer
    spec:
      restartPolicy: Never
      containers:
        - name: mysql
          image: mysql:9.7.2@sha256:29abb0a179982e4a8928138bfc7f918af9eda64e7eeb1b1d084c1720a20159e6
          env:
            - name: MYSQL_PWD
              valueFrom: { secretKeyRef: { name: tidb-root-auth, key: root } }
          command: ["sh", "-c", "mysql -h pokecalc-tidb-tidb -P 4000 -uroot -e \"ALTER USER 'root'@'%' IDENTIFIED BY ''\""]
EOF
kubectl -n pokecalc wait --for=condition=complete job/tidb-root-reset-once --timeout=120s
kubectl -n pokecalc delete tidbinitializer pokecalc
kubectl -n pokecalc delete job pokecalc-tidb-tidb-initializer --ignore-not-found
./scripts/k3d-m2-deploy.sh
```
確認: `kubectl -n pokecalc get tidbinitializer pokecalc` の `phase` が `Completed`、`k3d-m2-deploy: NATS・TiDB・record・team を入れた`。

## 補足(確認方法と既知の限界)

- `archive_sha256` は `f69dc040956302fa2a9cd61987158cf88978db9b1f5a1a4a70849310118d2a2a`(`scripts/tidb-operator-bootstrap.sh`)。取得元は
  `https://github.com/pingcap/tidb-operator/archive/refs/tags/v1.6.6.tar.gz`。確認方法: `curl -fsSL <取得元> | shasum -a 256` を2回流して同じ値になること(2026-10-03 に確認)。
  不一致なら helm を呼ばず非ゼロで終わる(`scripts/tidb-operator-bootstrap_test.sh`)。GitHub のタグは動かせるので、版を上げるときは値を取り直す。
- 既知の限界: CRD(`crd.yaml`)は `raw.githubusercontent.com` のタグ参照で、ハッシュを固定していない。
- initializer の許可(`allow-tidb-client-ingress`)は `app.kubernetes.io/name` ラベルの一致だけを見る。同じ namespace の Pod がその名前を名乗れば TiDB の 4000 に届く。
  ローカル(個人開発の k3d)用の設計で、認証の代わりにしない(TiDB には root パスワードがある)。
- メモリが逼迫したとき(TiKV が約 2.2GiB 常駐): まず `kubectl top nodes` と `kubectl -n pokecalc get pods` で OOMKilled・Evicted を見る。
  Docker Desktop の VM のメモリを増やすか、M2 が不要なら `kubectl -n pokecalc scale deploy/record deploy/team --replicas=0` で record・team を止める(TiDB は残す)。

## 結果(実機。2026-10-03)

- TidbCluster `pokecalc-tidb` が Ready(PD・TiKV・TiDB 各 1)、TidbInitializer が `Completed`(AC-T3・AC-T8)。record・team の migrate Job が `Complete`。
- record・team・NATS が `Running`。`make api-smoke`・`make web-k3d-smoke`・`make web-k3d-e2e`(2 件)が成功(以前は `/api/record/frequent-opponents` が 503 で失敗)。

## 影響・未決事項

- TiKV のメモリ(約 2.2GiB 常駐)の原因が未特定。小さくできれば limit を下げる(未決)。
- TidbInitializer の再実行不可は残る(上の復旧手順。冪等な Job への切り替えは必要になったら)。
- 他レーンが古い overlay を丸ごと apply すると、NetworkPolicy(operator 向け許可)が古い状態へ戻る可能性がある(実測中に一度、`allow-tidb-client-ingress` が古い内容に戻っていた)。
  クラスタ全体の操作は API レーンだけが行う取り決めにした(docs/ai-shared/decisions/2026-10-03-api-m2-k3d-deploy.md)。
- ADR-0211 の `helm repo add pingcap https://charts.pingcap.org/`(§3.1)は本 ADR §1 で置き換わった。
