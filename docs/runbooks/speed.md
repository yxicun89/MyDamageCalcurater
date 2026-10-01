# speed の手順書(ローカル k3d)

前提: k3d の `pokecalc` クラスタが起動している(`make up`)。API は `services/speed/api/openapi.yaml`、設計は ADR-0600・ADR-0601。

## 1. テストと静的検査を通す

```sh
cd "$(git rev-parse --show-toplevel)"
make test lint build check-publishable
```
確認: 最後の行が `check-publishable: 0 件` で、エラーで止まらない。

## 2. local overlay(架空データの read model)で k3d にデプロイして疎通を確かめる

```sh
cd "$(git rev-parse --show-toplevel)"
make speed-k3d-deploy
SPEED_URL=http://localhost:8080 make speed-smoke
```
確認: 最後の行が `speed smoke: health=200 pokemon=200 (count=8) missing_headers=400 table=200 (tiers=7, tie 219) invalid_presets=400 position=200 (146, 29/2/17) invalid_position=400 unknown_pokemon=422`
(1回目がロールアウト直後で失敗したら、`SPEED_URL=http://localhost:8080 make speed-smoke` をもう一度)。

## 3. pokedex export の read model で確かめる(export があるときだけ)

データレーンの pokedex export(`make pokedex-export`。`POKEDEX_DATABASE_DSN` が必要)で
`data/generated/readmodel/speed-pokemon.json` ができてから実行する。DB を用意する手順は [`data.md`](data.md) を見る。

`mysql` Service はクラスタ内部の DNS 名で、`make pokedex-export` は Mac 上で動くため直接は解決できない。
ポートフォワードして DSN のホストを付け替えてから実行する(2026-09-24 実機確認済み)。

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc port-forward svc/mysql 3306:3306 >/tmp/mysql-pf.log 2>&1 &
PF_PID=$!
sleep 2
export POKEDEX_DATABASE_DSN=$(kubectl -n pokecalc get secret mysql-auth -o jsonpath='{.data.pokedex-dsn}' | base64 -d | sed 's/@tcp(mysql:/@tcp(127.0.0.1:/')
make pokedex-export
kill $PF_PID
```
確認: `export: ../data/generated/readmodel に書いた`。

```sh
cd "$(git rev-parse --show-toplevel)"
make speed-k3d-deploy-readmodel
SPEED_URL=http://localhost:8080 make speed-smoke-readmodel
```
確認: 最後の行が `speed readmodel smoke: pokemon=<ID> list=200 table=200`
(1回目がロールアウト直後で `504 Gateway Timeout` になったら、`SPEED_URL=http://localhost:8080 make speed-smoke-readmodel` をもう一度)。
ファイルが無い・不正なときは `missing ...` や `read model check failed` で止まり、デプロイされない(ADR-0603 §3)。

架空データに戻すときは 2 をもう一度実行する(あとから適用したほうが k3d の speed の中身になる)。

## 4. GitOps(digest 固定)で確かめる

GitOps を確かめるときだけ、以降の 4〜10 を続ける。5〜10 はクラスタと共有の Argo CD を変えるので、
**人の確認のもとで実行する**(ADR-0605 §4。SP5 を作った作業では 4 までしか実行していない)。
4 はクラスタを変えないので、いつ実行してもよい。

```sh
cd "$(git rev-parse --show-toplevel)"
make speed-gitops-template-check
```
確認: 最後の行が `speed GitOps template: valid`。

## 5. Argo CD を入れる(初回だけ。すでに `argocd` namespace にあればとばす)

speed 専用の Argo CD は入れない(ADR-0605 §1・§3。クラスタに1つの共有インスタンスを使う)。balance の手順(TB0)で
すでに入れていればこの節はとばす(`scripts/argocd-bootstrap.sh` は namespace 作成が冪等なので、再実行しても壊れない。ADR-0405)。
`argocd` CLI(節 9 で使う)も入れておく(`brew install argocd`)。

```sh
cd "$(git rev-parse --show-toplevel)"
./scripts/argocd-bootstrap.sh
```
確認: `deployment "argocd-server" successfully rolled out`。

## 6. リポジトリの認証を登録する(初回だけ。人が自分のターミナルで)

balance で `repo-pokecalc` を登録済みならこの節はとばす(同じ Argo CD・同じリポジトリを使う)。
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

## 7. Application を作る(初回だけ)

```sh
cd "$(git rev-parse --show-toplevel)"
make speed-argocd-app
```
確認: `application.argoproj.io/pokecalc-speed created`(2回目以降は `unchanged`)。
`pokecalc-balance` は別の Application なので、この操作では変わらない。

## 8. イメージを push して digest を GitOps の定義に書く

`crane` が要る(`brew install crane`)。push 先は balance レーンのクラスタ内レジストリ(`balance-registry` namespace)。

```sh
cd "$(git rev-parse --show-toplevel)"
digest=$(make -s speed-registry-push 2>/dev/null | tail -1 | sed 's/.*@//')
# digest 行は images に2つある(pokecalc/speed と pokecalc/pokedex)。name ごとの範囲で本体の行だけを書き換える。
sed -i '' "/name: pokecalc\/speed$/,/digest:/ s/digest: .*/digest: ${digest}/" services/speed/deploy/k8s/overlays/gitops/kustomization.yaml
git diff services/speed/deploy/k8s/overlays/gitops/kustomization.yaml
```
確認: diff の `digest:` が `sha256:` で始まる値に変わる(変わらなければ同じイメージなので、9 と 10 は不要)。
この変更をブランチに commit し、PR で main に入れる。

### read model(initContainer)と pokedex image の digest(issue #237・ADR-0412)

- gitops overlay の Deployment は、initContainer `readmodel-export`(pokedex image の `export -out`)で read model を `emptyDir` に作り、
  本体は読み取り専用でマウントして読む。ConfigMap は使わない。DSN は Secret `mysql-auth` の `pokedex-reader-dsn`(SELECT のみ)で、initContainer だけに渡す。
- **Argo CD 有効時は手動 apply しない**。`make speed-k3d-deploy-readmodel` は Application が在ると既定で拒否する
  (意図して上書きするときだけ `ALLOW_MANUAL_OVERLAY=1`。戻すときは `argocd app sync pokecalc-speed`)。
- pokedex image の digest は `make pokedex-registry-push`(共有クラスタへは `POKEDEX_REGISTRY_PUSH_CONFIRM=1` が要る。人間の確認のもとで)の
  出力を使い、`kustomization.yaml` の `name: pokecalc/pokedex` の `digest:` に書く。全0のままだと `speed-gitops-check`(ready)が失敗する。
- **前提: NetworkPolicy(`allow-mysql-ingress.yaml`)の承認・適用と pokedex の実 digest の確定が済むまで、speed(と balance) を sync しない。**
- initContainer が MySQL に届くには NetworkPolicy の許可が要る(`deploy/k8s/base/networkpolicy/allow-mysql-ingress.yaml`)。共有 base なので別 PR・人間確認(ADR-0412 §4)。
- **speed の本体 image は未配備**(digest が全0の placeholder)。overlay の構造だけ先に入れてあり、template 検査は通るが ready 検査は意図どおり失敗する。実 image への置き換えは節 8 で、人間の確認のもとで。

## 9. 同期する(main に入った後)

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n argocd annotate application pokecalc-speed argocd.argoproj.io/refresh=normal --overwrite
kubectl config set-context --current --namespace=argocd
argocd --core app sync pokecalc-speed --timeout 180
kubectl config set-context --current --namespace=default
```
確認: 出力に `Sync Status: Synced to main (<main の commit>)` と `Phase: Succeeded`。

## 10. Pod が更新されたことを確かめる

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc rollout status deployment/speed --timeout=120s
kubectl -n pokecalc get deploy speed -o jsonpath='{.spec.template.spec.containers[0].image}{"\n"}'
grep digest services/speed/deploy/k8s/overlays/gitops/kustomization.yaml
for i in $(seq 1 15); do code=$(curl -s -o /dev/null -w '%{http_code}' http://localhost:8080/api/speed/healthz); [ "$code" = 200 ] && break; sleep 2; done; echo "health=$code"
```
確認: 2つ目と3つ目の `sha256:` の値が一致し、最後が `health=200`(ロールアウト直後の 502 は再試行で消える)。
この overlay は read model をマウントしないので、`/api/speed/v1/pokemon` 等は `503 master_unavailable` のままでよい
(ADR-0605 §2a。実データを GitOps でどう配るかは未決)。

local の read model で動かす状態に戻すときは 2 をもう一度実行する(Argo CD の Application は OutOfSync になる)。

## 11. GitOps の状態に戻す(通常経路)

10 で local overlay に上書きすると OutOfSync のままになる。main へマージされた後の通常経路は 8〜9 と同じ
(`make speed-registry-push` → 出力された digest を `services/speed/deploy/k8s/overlays/gitops/kustomization.yaml`
に書いて PR で main に入れる → `argocd --core app sync pokecalc-speed`)。

自動 sync(`syncPolicy.automated`)は導入しないので(ADR-0408 §4)、この手順を踏んで明示的に Synced へ戻す。

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n argocd annotate application pokecalc-speed argocd.argoproj.io/refresh=normal --overwrite
kubectl config set-context --current --namespace=argocd
argocd --core app sync pokecalc-speed --timeout 180
kubectl config set-context --current --namespace=default
```
確認: 出力に `Sync Status: Synced to main (<main の commit>)` と `Phase: Succeeded`。
