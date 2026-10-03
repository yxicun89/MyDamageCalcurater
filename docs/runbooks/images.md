# ポケモン画像の手順書(ADR-0807)

画像は Git に入れない(ADR-0002)。手元の画像を変換し、gateway が `/images/*` で配信する。画像が無くても全機能が動く(タイプ色のエンブレム)。

## 1. 変換する

```sh
cd "$(git rev-parse --show-toplevel)"
mkdir -p data/generated/images/src
# {図鑑番号4桁}-{フォルム3桁}.png|jpg|jpeg|webp(例 0445-000.png)を data/generated/images/src に置く
make assets
```
確認: `assets: 変換 N 件・スキップ M 件` が出て、`data/generated/images/dist/manifest.json` と `thumb/`・`detail/` ができる。スキップは理由付きで表示される。

## 2. `make dev` で見る

```sh
cd "$(git rev-parse --show-toplevel)"
make dev
```
別のターミナルで、確認: `curl -s http://localhost:8080/images/manifest.json` が変換した画像のキーを含む JSON を返す。
`dist/manifest.json` が無いときは `/images/*` は 404(画像なし)。

## 3. k3d で見る(手動。make up には組み込まれていない)

local overlay の gateway には画像ディレクトリが無く、`/images/*` は 404 のまま全機能が動く。見たいときだけ、次をその場限りで行う
(k3d の volume mount は make up を壊すので恒久配線は別タスク)。

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc set env deploy/gateway GATEWAY_IMAGES_DIR=/tmp/images
kubectl -n pokecalc rollout status deploy/gateway
kubectl -n pokecalc cp data/generated/images/dist "$(kubectl -n pokecalc get pod -l app.kubernetes.io/name=gateway -o name | head -1 | cut -d/ -f2)":/tmp/images
```
確認: `curl -s http://localhost:8080/images/manifest.json` が JSON を返す。Pod が作り直されたら `kubectl cp` をやり直す(Pod の一時領域のため)。
`set env` は GitOps の差分になるので、終わったら `kubectl -n pokecalc set env deploy/gateway GATEWAY_IMAGES_DIR-` で戻す。

注意: `kubectl cp` は Pod に `tar` が要る。gateway のイメージに無い場合はこの手順は使えない(未検証。恒久配線の別タスクで volume mount を検討する)。
