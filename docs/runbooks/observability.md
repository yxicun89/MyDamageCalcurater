# observability(監視スタック)の手順書(ローカル k3d)

前提: k3d の `pokecalc` クラスタが起動している(`make up`)。設計は ADR-0406(§1〜3 は各サービスの `/metrics`、
§4〜5 は kube-prometheus-stack・Loki・Alloy の導入)。取得元のバージョン・SHA-256 固定は
`scripts/observability-bootstrap.sh` にまとめてある(ADR-0405 と同じ「取得→検証→適用」の流儀)。

## 1. テストと静的検査を通す

```sh
cd "$(git rev-parse --show-toplevel)"
make test lint build check-publishable
```
確認: `scripts/observability-bootstrap_test.sh` の行が `observability-bootstrap test: all <N> checks passed`、
最後の行が `check-publishable: 0 件` で、エラーで止まらない。

## 2. Grafana の管理者パスワードを登録する(初回だけ)

values(`deploy/k8s/base/observability/values/kube-prometheus-stack.yaml`)にパスワードは書かない。導入前に
Secret を作る(ADR-0018 §5 の「AI はトークンを見ない」と同じ扱い。人が自分のターミナルで、パスワードは画面に出さず貼り付けて Enter)。

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl get namespace observability >/dev/null 2>&1 || kubectl create namespace observability
pw_dir=$(mktemp -d) && trap 'rm -rf "$pw_dir"' EXIT
( umask 077; read -rs GRAFANA_PASSWORD; printf '%s' "$GRAFANA_PASSWORD" > "$pw_dir/admin-password" )
unset GRAFANA_PASSWORD
kubectl -n observability create secret generic grafana-admin-credentials \
  --from-literal=admin-user=admin --from-file="$pw_dir/admin-password"
rm -rf "$pw_dir"
```
確認: `secret/grafana-admin-credentials created`。

## 3. スタックを導入する

```sh
cd "$(git rev-parse --show-toplevel)"
./scripts/observability-bootstrap.sh
```
確認: 最後の行が `observability-bootstrap: 完了しました`。3チャートいずれかの SHA-256 が取得物と一致しなければ、
`helm upgrade` を1つも呼ばずに非0で終了する(出力に「SHA-256 が一致しません」)。

## 4. Pod が起動したことを確かめる

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n observability get pods
kubectl -n observability rollout status deployment/kube-prometheus-stack-grafana --timeout=180s
kubectl -n observability rollout status statefulset/loki --timeout=180s
```
確認: いずれも `successfully rolled out`。Alloy は DaemonSet なので `kubectl -n observability get daemonset alloy` の
`DESIRED` と `READY` が一致することを見る。

## 5. Grafana・Prometheus にアクセスする(port-forward のみ。Ingress は作らない。issue #148)

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n observability port-forward svc/kube-prometheus-stack-grafana 3000:80 >/tmp/grafana-pf.log 2>&1 &
kubectl -n observability port-forward svc/kube-prometheus-stack-prometheus 9090:9090 >/tmp/prometheus-pf.log 2>&1 &
```
確認: `http://localhost:3000`(ユーザー名 `admin`、パスワードは手順 2 で登録したもの)で Grafana が開く。
`http://localhost:9090/targets` で6サービス(balance・speed・judge・gateway・pokedex・calc)の ServiceMonitor が
`UP` になっていることを確認する。Grafana の Explore で データソース `Loki` を選び、適当なクエリ(例
`{namespace="pokecalc"}`)で各サービスのログが表示されることを確認する(Alloy が正しく Loki へ送れているかの確認)。
計算API の SLO(ADR-0407。P7-2)は、Grafana のダッシュボード一覧から `calc-slo` を開くと、p99レイテンシ(100msの
しきい値線つき)と可用性の時系列・直近値が見られる。終わったら `kill %1 %2` で port-forward を止める。

## 6. バージョンを上げるとき

`scripts/observability-bootstrap.sh` の3つの版・3つの SHA-256(`KUBE_PROMETHEUS_STACK_VERSION` /
`KUBE_PROMETHEUS_STACK_SHA256` / `LOKI_VERSION` / `LOKI_SHA256` / `ALLOY_VERSION` / `ALLOY_SHA256`)を明示的に
更新し、`docs/adr/0406-observability-metrics-and-stack.md` §4 の記載も合わせて更新する(自動追従はしない。
`scripts/observability-bootstrap_test.sh` が ADR の記載とスクリプトの定数の一致を検査する)。

## 7. 障害時の一次切り分け(issue #293)

前提: 手順 5 の port-forward(Grafana 3000・Prometheus 9090)が張ってある。PromQL は `http://localhost:9090/graph` に貼る
(Grafana の Explore で データソース `Prometheus` を選んでもよい)。ログは Explore の `Loki`。
クエリはすべて読み取り専用。**まず全体を1行で見る**: `count(up{namespace="pokecalc"} == 1)` が `6`(balance・speed・judge・gateway・pokedex・calc)
でなければ、下の症状 A から。`6` なら、サービスは動いていて、遅い・エラーなら症状 B。

### 症状 A: Pod が Ready にならない

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc get pods
kubectl -n pokecalc describe pod <Ready でない Pod>
kubectl -n pokecalc logs <Pod> --previous
```
```promql
# Ready でない Pod(完了済みの Job の Pod は除く)
(kube_pod_status_ready{namespace="pokecalc",condition="false"} == 1) unless on(namespace,pod) (kube_pod_status_phase{namespace="pokecalc",phase="Succeeded"} == 1)
# 待ち状態の理由(ImagePullBackOff・CrashLoopBackOff・CreateContainerConfigError など)
kube_pod_container_status_waiting_reason{namespace="pokecalc"} == 1
# 直近1時間に再起動した container
increase(kube_pod_container_status_restarts_total{namespace="pokecalc"}[1h]) > 0
# 落ちているサービス(スクレイプできない)
up{namespace="pokecalc"} == 0
```
次に確かめる: `describe` 末尾の Events(イメージを引けない・Secret/ConfigMap が無い・probe 失敗)。起動時に落ちるなら `logs --previous`。
DB を使う Pod(pokedex)は `mysql-0` が Ready か、依存する Secret `mysql-auth` があるかを先に見る。

### 症状 B: gateway が 502・503 を返す

```promql
# サービスごとの 5xx 率(結果が空なら 5xx は出ていない)
sum by (job)(rate(http_requests_total{status=~"5.."}[5m]))
# サービスごとのリクエスト率と p99 レイテンシ(どのサービスが遅いか)
sum by (job)(rate(http_requests_total[5m]))
histogram_quantile(0.99, sum by (le, job)(rate(http_request_duration_seconds_bucket[5m])))
```
```logql
{namespace="pokecalc", container="gateway"} |~ "(?i)error|502|503"
```
次に確かめる: gateway だけ 5xx なら上流(calc・pokedex・balance・speed・judge)のどれが `up == 0` か(症状 A)。上流の Service に Pod が
付いているか `kubectl -n pokecalc get endpoints` の ENDPOINTS 列が空でないかを見る。計算 API が遅いときは Grafana のダッシュボード `calc-slo`(p99 と可用性)。

### 症状 C: import Job が失敗する

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc get cronjob,jobs
kubectl -n pokecalc logs job/<失敗した Job 名>
kubectl -n pokecalc describe job <失敗した Job 名>
```
```promql
kube_job_status_failed{namespace="pokecalc"} == 1
```
次に確かめる: `logs` の最後のエラー行(取り込み元・DB 接続・マイグレーション未適用)。DB 側は症状 A の `mysql-0`。
過去の失敗 Job(`BackoffLimitExceeded`)が `== 1` に残り続けることがある。TTL で消えない手動 Job は
`kubectl -n pokecalc delete job <名>` で片づけるまで残るので、新しい Job が `Complete` かどうかを見て判断する。
手順の本体は [`data.md`](data.md) を見る。

### 症状 D: Argo CD の Application が OutOfSync

Argo CD は Prometheus に載せていないため、Application の状態は kubectl で見る(読み取りだけ)。

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n argocd get applications
kubectl -n argocd describe application pokecalc-balance
kubectl -n argocd get application pokecalc-balance -o jsonpath='{.status.conditions}{"\n"}{.status.sync.revision}{"\n"}'
```
次に確かめる: `describe` の `Conditions`(ComparisonError ならリポジトリ・パスを引けていない)と、`sync.revision` が main の先頭と一致するか。
差分があるだけなら意図した状態か(自動 sync は入れない。ADR-0408 §4)を確かめ、同期は [`balance.md`](balance.md) §7・[`speed.md`](speed.md) §9 の手順で行う。
