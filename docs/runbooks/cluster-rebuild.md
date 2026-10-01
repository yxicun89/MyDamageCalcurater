# クラスタ再作成の手順書(ローカル k3d。issue #297)

`make down` → `make up` の後に、Argo CD・レジストリの image・DB・Secret を戻す順序。
GitOps(Argo CD)の対象は **balance だけ**(ユーザー決定 2026-09-25)。calc・gateway・web・pokedex などは `make deploy-latest` で入れる。

## 0. 何が消えて、何が戻るか

| もの | `make down` で | 戻し方 |
|---|---|---|
| Secret `mysql-auth`・`tidb-root-auth` | 消える | `make up` が乱数で作り直す(新しい DB と整合する) |
| pokedex のマスタ | 消える | 再取込(手順 3) |
| record・team の保存データ(TiDB) | 消える | 戻せない(ローカルにバックアップ手順は無い) |
| Argo CD 本体・リポジトリ認証・Application | 消える | 手順 4〜5 |
| レジストリ(emptyDir)の image | 消える | push し直す(手順 6) |

**`make down` は人間の確認が要る操作**(クラスタ・PVC・Secret の削除)。消す前に、残したい TiDB のデータが無いことを確かめる。

## 1. 消す前に孤児 PVC を確かめる(人間の確認が要るデータ削除)

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl get pvc -A
```
確認: `pokecalc` namespace 以外(特に `default` の `data-mysql-0`)に PVC があるか。
`default` の PVC は、base/pokedex を単独で apply した事故の痕跡で、どの Pod にも使われていなければ孤児。

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n default get pods,sts
kubectl -n default describe pvc data-mysql-0 | grep -i "used by"
```
判断基準: `Used By: <none>` かつ `default` に StatefulSet・Pod が無い → 消してよい候補。
消す(`kubectl -n default delete pvc data-mysql-0`)のは**人間の確認が要る操作**。確認が取れるまで実行しない。
`make down` はクラスタごと消えるので、この PVC も一緒に消える。

## 2. クラスタを作って基盤を入れる

```sh
cd "$(git rev-parse --show-toplevel)"
make down
make up
```
確認: `make up` が最後まで通り、`kubectl -n pokecalc get pod mysql-0` が `Running`。

## 3. マスタを再取込してサービスを入れ直す

```sh
cd "$(git rev-parse --show-toplevel)"
created=$(make import-k8s)
job_name=$(echo "$created" | grep -o 'pokedex-import-manual-[0-9]*' | tail -1)
kubectl -n pokecalc wait --for=condition=complete "job/$job_name" --timeout=600s
make pokedex-export
make deploy-latest
```
確認: `wait` が `condition met`、`deploy-latest` が `全サービスを <コミット> の内容で入れ替えた` で終わる。
取得元が取れないときは、同じ版のキャッシュ(PVC `pokedex-import-cache`)もクラスタごと消えているので戻せない(ネットワークが要る)。

## 4. Argo CD を入れてリポジトリ認証を登録する

```sh
cd "$(git rev-parse --show-toplevel)"
./scripts/argocd-bootstrap.sh
kubectl -n argocd delete secret argocd-initial-admin-secret
```
確認: `deployment "argocd-server" successfully rolled out`。

リポジトリの認証は、GitHub の fine-grained token(このリポジトリだけ・Contents: Read-only)を新しく作り、人が自分のターミナルで貼り付ける。

```sh
cd "$(git rev-parse --show-toplevel)"
read -rs PAT && kubectl -n argocd create secret generic repo-pokecalc \
  --from-literal=type=git --from-literal=url="$(git remote get-url origin)" \
  --from-literal=username=x-access-token --from-literal=password="$PAT" \
&& kind_label="argocd.argoproj.io/secret-type" \
&& kubectl -n argocd label secret repo-pokecalc "${kind_label}=repository"; unset PAT kind_label
```
確認: `secret/repo-pokecalc labeled`。

## 5. レジストリと Application を作る

```sh
cd "$(git rev-parse --show-toplevel)"
make balance-registry-apply
make balance-argocd-app
kubectl -n argocd get applications
```
確認: `pokecalc-balance` が出る。image が無いこの時点では `Synced` でも Pod は `ImagePullBackOff` になる(想定どおり。次の手順で解消)。

## 6. image を push し直して digest を合わせる

```sh
cd "$(git rev-parse --show-toplevel)"
digest=$(make -s balance-registry-push 2>/dev/null | tail -1 | sed 's/.*@//')
echo "$digest"
grep -n "digest:" services/balance/deploy/k8s/overlays/gitops/kustomization.yaml
```
確認: `$digest` が、Git の gitops overlay の `name: pokecalc/balance` の `digest:` と同じ。
同じなら手順 7 へ。違う(docker build は同じ digest を再現しない)ときは、`docs/runbooks/balance.md` の §6 のとおり digest を書き換えて PR で main に入れてから §7 で同期する。
pokedex image(read model の initContainer 用)も同様で、`make pokedex-registry-push` の出力を `name: pokecalc/pokedex` の `digest:` と比べる。

## 7. 同期して確かめる

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n argocd get applications
kubectl -n pokecalc get pods
make api-smoke
```
確認: Application が以前と同じ件数で `Synced`・`Healthy`、Pod が全て `Running`、`api-smoke` が成功。
