# balance の手順書(ローカル k3d)

前提: k3d の `pokecalc` クラスタが起動している(`make up`)。方式の説明は ADR-0018、API は `services/balance/api/openapi.yaml`。

## 1. テストと静的検査を通す

```sh
cd "$(git rev-parse --show-toplevel)"
make test lint build check-publishable
```
確認: 最後の行が `check-publishable: 0 件` で、エラーで止まらない。

## 2. local overlay(架空データの read model)で k3d にデプロイして疎通を確かめる

```sh
cd "$(git rev-parse --show-toplevel)"
make balance-k3d-deploy
make balance-smoke
```
確認: 最後の行が `balance smoke: health=200 analyze=200 unknown=422 coverage=200 unknown_move=422 ability=200 unknown_ability=422 threats=200 threats_unknown_move=422 recommendations=200`
(1回目がロールアウト直後で失敗したら `make balance-smoke` をもう一度)。

## 2b. pokedex export の実データで動かす(export があるときだけ)

データレーンの pokedex export で `data/generated/readmodel/` に3つの JSON ができてから実行する。

```sh
cd "$(git rev-parse --show-toplevel)"
make balance-k3d-deploy-readmodel
make balance-smoke-readmodel
```
確認: 最後の行が `balance readmodel smoke: pokemon=<ID> analyze=200 recommendations=200`。
ファイルが無い・不正なときは `read model check failed` や `missing ...` で止まり、デプロイされない。

GitOps(Argo CD)を確かめるときだけ、以降の 3〜9 を続ける(3〜5 は初回だけ)。

## 3. Argo CD を入れる(初回だけ)

コミットSHA固定の取得・ハッシュ検証・イメージの digest 固定は `scripts/argocd-bootstrap.sh` にまとめてある(ADR-0405)。

```sh
cd "$(git rev-parse --show-toplevel)"
./scripts/argocd-bootstrap.sh
```
確認: `deployment "argocd-server" successfully rolled out`。

## 4. リポジトリの認証を登録する(初回だけ。人が自分のターミナルで)

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

## 5. レジストリと Application を作る(初回だけ)

```sh
cd "$(git rev-parse --show-toplevel)"
make balance-registry-apply
make balance-argocd-app
```
確認: `application.argoproj.io/pokecalc-balance created`(2回目以降は `unchanged`)。

## 6. イメージを push して digest を GitOps の定義に書く

```sh
cd "$(git rev-parse --show-toplevel)"
digest=$(make -s balance-registry-push 2>/dev/null | tail -1 | sed 's/.*@//')
sed -i '' "s/digest: .*/digest: ${digest}/" services/balance/deploy/k8s/overlays/gitops/kustomization.yaml
git diff services/balance/deploy/k8s/overlays/gitops/kustomization.yaml
```
確認: diff の `digest:` が `sha256:` で始まる値に変わる(変わらなければ同じイメージなので、7 と 8 は不要)。
この変更をブランチに commit し、PR で main に入れる。

## 7. 同期する(main に入った後)

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n argocd annotate application pokecalc-balance argocd.argoproj.io/refresh=normal --overwrite
kubectl config set-context --current --namespace=argocd
argocd --core app sync pokecalc-balance --timeout 180
kubectl config set-context --current --namespace=default
```
確認: 出力に `Sync Status: Synced to main (<main の commit>)` と `Phase: Succeeded`。

## 8. Pod が更新されたことを確かめる

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc rollout status deployment/balance --timeout=120s
kubectl -n pokecalc get deploy balance -o jsonpath='{.spec.template.spec.containers[0].image}{"\n"}'
grep digest services/balance/deploy/k8s/overlays/gitops/kustomization.yaml
for i in $(seq 1 15); do code=$(curl -s -o /dev/null -w '%{http_code}' http://localhost:8080/api/balance/healthz); [ "$code" = 200 ] && break; sleep 2; done; echo "health=$code"
```
確認: 2つ目と3つ目の `sha256:` の値が一致し、最後が `health=200`(ロールアウト直後の 502 は再試行で消える)。

## 9. local の read model で動かす状態に戻す

```sh
cd "$(git rev-parse --show-toplevel)"
make balance-k3d-deploy
make balance-smoke
```
確認: 最後の行が `balance smoke: health=200 ... recommendations=200`(Argo CD の Application は OutOfSync になる)。
