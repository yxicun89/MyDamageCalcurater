# balance の GitOps(Argo CD、ローカル k3d)

方式と、Git に入れる値・入れない値は ADR-0018。前提: k3d の `pokecalc` クラスタが起動している(`make up`)。

## 1. Argo CD を入れる(初回だけ)

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl create namespace argocd
kubectl apply -n argocd --server-side -f https://raw.githubusercontent.com/argoproj/argo-cd/v3.5.3/manifests/install.yaml
kubectl -n argocd rollout status deployment/argocd-server --timeout=300s
```
確認: `deployment "argocd-server" successfully rolled out`。

## 2. リポジトリの認証を登録する(初回だけ。人が自分のターミナルで)

GitHub で、このリポジトリだけ・Contents: Read-only の fine-grained token を作ってから実行する。トークンは画面に出さずに貼り付けて Enter。

```sh
cd "$(git rev-parse --show-toplevel)"
read -rs PAT && kubectl -n argocd create secret generic repo-pokecalc \
  --from-literal=type=git --from-literal=url="$(git remote get-url origin)" \
  --from-literal=username=x-access-token --from-literal=password="$PAT" \
&& kind_label="argocd.argoproj.io/secret-type" \
&& kubectl -n argocd label secret repo-pokecalc "${kind_label}=repository"; unset PAT kind_label
```
確認: `secret/repo-pokecalc labeled`。

## 3. レジストリと Application を作る(初回だけ)

```sh
cd "$(git rev-parse --show-toplevel)"
make balance-registry-apply
make balance-argocd-app
```
確認: `application.argoproj.io/pokecalc-balance created`(2回目以降は `unchanged`)。

## 4. イメージを push して digest を GitOps の定義に書く

```sh
cd "$(git rev-parse --show-toplevel)"
digest=$(make -s balance-registry-push 2>/dev/null | tail -1 | sed 's/.*@//')
sed -i '' "s/digest: .*/digest: ${digest}/" services/balance/deploy/k8s/overlays/gitops/kustomization.yaml
git diff services/balance/deploy/k8s/overlays/gitops/kustomization.yaml
```
確認: diff の `digest:` が `sha256:` で始まる値に変わる(変わらなければ同じイメージなので、5 と 6 は不要)。
この変更をブランチに commit し、PR で main に入れる。

## 5. 同期する(main に入った後)

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n argocd annotate application pokecalc-balance argocd.argoproj.io/refresh=normal --overwrite
kubectl config set-context --current --namespace=argocd
argocd --core app sync pokecalc-balance --timeout 180
kubectl config set-context --current --namespace=default
```
確認: 出力に `Sync Status: Synced to main (<main の commit>)` と `Phase: Succeeded`。

## 6. Pod が更新されたことを確かめる

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc rollout status deployment/balance --timeout=120s
kubectl -n pokecalc get deploy balance -o jsonpath='{.spec.template.spec.containers[0].image}{"\n"}'
grep digest services/balance/deploy/k8s/overlays/gitops/kustomization.yaml
for i in $(seq 1 15); do code=$(curl -s -o /dev/null -w '%{http_code}' http://localhost:8080/api/balance/healthz); [ "$code" = 200 ] && break; sleep 2; done; echo "health=$code"
```
確認: 2つ目と3つ目の `sha256:` の値が一致し、最後が `health=200`(ロールアウト直後の 502 は再試行で消える)。

## 7. local の read model で動かす状態に戻す(必要なとき)

```sh
cd "$(git rev-parse --show-toplevel)"
make balance-k3d-deploy
make balance-smoke
```
確認: 最後の行が `balance smoke: health=200 ... recommendations=200`(Argo CD の Application は OutOfSync になる)。
