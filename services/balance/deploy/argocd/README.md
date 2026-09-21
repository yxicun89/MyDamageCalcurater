# balance GitOps setup

balance の Argo CD Application。方式は ADR-0018(ローカル k3d での検証)。同期は manual。

## Git に入れる値 / 入れない値

- 入れる: `../k8s/overlays/gitops/kustomization.yaml` の image(`newName` と `digest`。tag や `latest` は使わない)。
- 入れない: リポジトリの URL(アカウント名を含む)、Git の access token、registry の password、Kubernetes Secret の実値、ローカルの絶対パス。
  `application.yaml` の `repoURL` は placeholder のままにし、適用時に `git remote get-url origin` から埋め込む。

## 初回の準備(ローカル k3d)

1. Argo CD(版を固定。2026-09-22 時点の最新 v3.5.3):
   ```sh
   kubectl create namespace argocd
   kubectl apply -n argocd --server-side -f https://raw.githubusercontent.com/argoproj/argo-cd/v3.5.3/manifests/install.yaml
   ```
2. private リポジトリの認証(**ユーザーが自分のターミナルで**。読み取り専用・このリポジトリだけの fine-grained PAT):
   ```sh
   read -rs PAT && kubectl -n argocd create secret generic repo-pokecalc \
     --from-literal=type=git --from-literal=url="$(git remote get-url origin)" \
     --from-literal=username=x-access-token --from-literal=password="$PAT" \
   && kind_label="argocd.argoproj.io/secret-type" \
   && kubectl -n argocd label secret repo-pokecalc "${kind_label}=repository"; unset PAT kind_label
   ```
   (最後の label は Argo CD が repository の認証情報として認識するための印。値 `repository` は秘密ではない)
3. クラスタ内レジストリと Application:
   ```sh
   make balance-registry-apply
   make balance-argocd-app
   ```

## デプロイ(Git 変更 → manual sync → Pod 更新)

```sh
make balance-registry-push          # 表示された localhost:5000/pokecalc/balance@sha256:... の digest を
                                    # ../k8s/overlays/gitops/kustomization.yaml に書き、PR で main に入れる
argocd --core app sync pokecalc-balance   # または Argo CD の UI から Sync
kubectl -n argocd get application pokecalc-balance
kubectl -n pokecalc get deploy balance -o jsonpath='{.spec.template.spec.containers[0].image}'
```

Git の commit、Argo CD の同期 revision、稼働 Pod の image digest が一致することを確認する。
gitops overlay には read model のマウントが無いので、Argo CD で同期した balance の analyze / coverage は 503(ADR-0018「影響と制約」)。
local の read model で動かすときは `make balance-k3d-deploy`(Application は OutOfSync になる)。
