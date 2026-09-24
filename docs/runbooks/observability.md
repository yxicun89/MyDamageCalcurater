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
終わったら `kill %1 %2` で port-forward を止める。

## 6. バージョンを上げるとき

`scripts/observability-bootstrap.sh` の3つの版・3つの SHA-256(`KUBE_PROMETHEUS_STACK_VERSION` /
`KUBE_PROMETHEUS_STACK_SHA256` / `LOKI_VERSION` / `LOKI_SHA256` / `ALLOY_VERSION` / `ALLOY_SHA256`)を明示的に
更新し、`docs/adr/0406-observability-metrics-and-stack.md` §4 の記載も合わせて更新する(自動追従はしない。
`scripts/observability-bootstrap_test.sh` が ADR の記載とスクリプトの定数の一致を検査する)。
