# M4(運用)の動作確認

上から順に実行する。各コマンドの下の「→」が成功の見え方。障害時の一次切り分けなどの詳細は [runbooks/observability.md](runbooks/observability.md)。
前提: k3d の `pokecalc` クラスタが動いている(§3・§4。[verify-m1.md](verify-m1.md) §3・§4)。

## 1. 自動テスト(クラスタ不要)

```sh
cd "$(git rev-parse --show-toplevel)"
make test-scripts
make lint
bash scripts/check-publishable.sh
```
→ `test-scripts` の出力に `observability-bootstrap test: all <N> checks passed` と `observability-slo_test.sh` の成功が含まれる。最後に `check-publishable: 0 件`。

## 2. 記録ルールの式を検査する(docker が要る。クラスタ不要)

PrometheusRule から `groups` だけを取り出し、`promtool` で検査する。

```sh
cd "$(git rev-parse --show-toplevel)"
d=$(mktemp -d)
for n in calc balance; do
  ruby -ryaml -e 'puts({"groups"=>YAML.load_file(ARGV[0])["spec"]["groups"]}.to_yaml)' \
    deploy/k8s/base/observability/prometheusrules/$n-slo.yaml > "$d/$n-slo.yml"
done
docker run --rm -v "$d:/r" --entrypoint promtool prom/prometheus:latest check rules /r/calc-slo.yml /r/balance-slo.yml
rm -rf "$d"
```
→ `calc-slo.yml`・`balance-slo.yml` とも `SUCCESS: 2 rules found`(2026-10-03 に確認)。

## 3. 実クラスタ: 監視スタック

人間の作業: Grafana の管理者パスワードの Secret を作る(パスワードは画面に出さず貼り付ける。AI には見せない)。

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
→ `secret/grafana-admin-credentials created`。

スタックを入れて、画面にアクセスする。

```sh
cd "$(git rev-parse --show-toplevel)"
./scripts/observability-bootstrap.sh
kubectl -n observability get pods
kubectl -n observability port-forward svc/kube-prometheus-stack-grafana 3000:80 >/tmp/grafana-pf.log 2>&1 &
kubectl -n observability port-forward svc/kube-prometheus-stack-prometheus 9090:9090 >/tmp/prometheus-pf.log 2>&1 &
```
→ 最後の行が `observability-bootstrap: 完了しました`。Pod が Running(Alloy は DaemonSet の `DESIRED` と `READY` が一致)。
`http://localhost:9090/targets` で balance・speed・judge・gateway・pokedex・calc の ServiceMonitor が `UP`(PromQL `count(up{namespace="pokecalc"} == 1)` が `6`)。
`http://localhost:3000`(ユーザー `admin`)のダッシュボード一覧の `calc-slo`(p99 < 100ms・可用性。ADR-0407)と `balance-slo`(p99 < 500ms・可用性。ADR-0420)に時系列が出る。
計算 API と balance にリクエストが無いと「No data」なので、先に画面で計算とタイプバランスを何度か操作する。

```sh
kill %1 %2
```
→ port-forward が止まる。

## 4. GitOps(Argo CD)

手順は [runbooks/balance.md](runbooks/balance.md) §3〜10 をそのまま実行する。人間の作業と順序は次のとおり。

- 先に済ませる(人間の作業): `deploy/k8s/base/networkpolicy/allow-mysql-ingress.yaml` の承認・適用と、`make pokedex-registry-push`(共有クラスタへは `POKEDEX_REGISTRY_PUSH_CONFIRM=1`)による pokedex の実 digest の確定(ADR-0412)。これが済むまで balance を sync しない。
- リポジトリ認証の PAT の登録(balance.md §4)は人間の作業(トークンは AI に見せない)。

→ balance.md §7 で `Sync Status: Synced to main`・`Phase: Succeeded`、§8 の最後が `health=200`。

## 5. 結果の記録

確かめた日付と、うまくいかなかった番号を `docs/plan/m4.md` の該当行に書く。
