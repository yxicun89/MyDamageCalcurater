# ローカル k3d の版の確認とロールバック(issue #291・ADR-0806)

対象は `make api-k3d-deploy`・`web-k3d-deploy`・`balance-k3d-deploy`・`speed-k3d-deploy`・`judge-k3d-deploy` で入れた Deployment
(calc・gateway・web・balance・speed・judge)。Argo CD(GitOps)で入れたものは digest 固定で追跡できるので対象外。

## 1. 動いている版を確かめる

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc get deploy -o custom-columns=NAME:.metadata.name,IMAGE:.spec.template.spec.containers[0].image
curl -s http://localhost:8080/healthz
```
確認: image のタグが `<コミット12桁>` か `<コミット12桁>-dirty`(`:local` ではない)。`/healthz` の `version` が gateway の image タグと同じ。
`-dirty` は未コミットの変更を含んでビルドしたことを表す(その中身は git からは復元できない)。
コミットを知りたいときは `git log --oneline -1 <コミット12桁>`。

## 2. 履歴を見て、1つ前の版へ戻す

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc rollout history deployment/gateway
kubectl -n pokecalc rollout undo deployment/gateway
kubectl -n pokecalc rollout status deployment/gateway --timeout=120s
kubectl -n pokecalc get deploy gateway -o jsonpath='{..image}'
```
確認: 最後の行のタグが、手順1で見た「1つ前の」コミットに変わっている。`deployment/gateway` は戻したい Deployment 名に読み替える。
2つ以上前へ戻すときは `rollout history` で番号を確かめ、`rollout undo deployment/gateway --to-revision=<番号>`。

## 3. 戻せないとき

k3d ノードに前のタグのイメージが無いと Pod が `ImagePullBackOff` になる(クラスタを作り直した・`docker system prune` した場合など)。
前のコミットを checkout してデプロイし直す(`<名前>` は `api`・`web`・`balance`・`speed`・`judge`)。

```sh
cd "$(git rev-parse --show-toplevel)"
git switch --detach <コミット>
make <名前>-k3d-deploy
git switch -
```
確認: `kubectl -n pokecalc get deploy -o custom-columns=NAME:.metadata.name,IMAGE:.spec.template.spec.containers[0].image` のタグが `<コミット>` の12桁。

## 注意

- 未コミットの変更が無い状態(`git status` が clean)でデプロイすると、同じコミットの再実行は何も変えない(rollout も起きない)。
- `-dirty` のデプロイは同じタグのまま中身が変わるので、毎回 `rollout restart` する。この履歴は `rollout undo` で区別して戻せないので、
  戻したくなりそうな版はコミットしてからデプロイする。
- 履歴の件数は Deployment の `revisionHistoryLimit`(既定 10)まで。
