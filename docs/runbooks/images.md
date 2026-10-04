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

## 3. k3d

変換した画像を k3d のノードへ置き、gateway の `/images/*` で配信する(ADR-0807 追記)。`make up` 済みのクラスタをそのまま使う(作り直し不要)。

```bash
cd "$(git rev-parse --show-toplevel)"
make assets          # data/generated/images/src → dist(WebP + manifest.json)
make images-k3d      # dist の中身をノード(k3d-pokecalc-server-0)の /var/lib/pokecalc-images へ docker cp
```

gateway の local overlay だけが、そのディレクトリを hostPath(読み取り専用・`DirectoryOrCreate`)で `GATEWAY_IMAGES_DIR=/data/images` に見せる(`deploy/k8s/overlays/local/api/gateway-images-patch.yaml`。base と cloud には無い)。
`make images-k3d` に gateway の再起動は要らない(2回目以降の入れ替えも同じ)。ただし先にこの overlay の gateway へ入れ替える(`make api-k3d-deploy`)必要がある。入れ替え前の gateway は volume を持たない。

確認: `curl -s -w '\n%{http_code} %{content_type}\n' http://localhost:8080/images/manifest.json` が `200 application/json` で画像のキーを含む JSON を返す。
入口(8080)は未知のパスを画面の HTML で返す場合があるので、ステータスだけでなく Content-Type が `application/json` かも見る。

- 画像が無い(`dist/manifest.json` が無い)ときの `make images-k3d` は「画像なし」と表示して終了コード 0。gateway は volume が空でも起動し、`/images/*` は 404(エンブレム)。`make deploy-latest` は `api-k3d-deploy` の後にこれを呼ぶが、失敗しても止まらない。
- 消すとき: `docker exec k3d-pokecalc-server-0 find /var/lib/pokecalc-images -mindepth 1 -delete`(ディレクトリ自体は消さない。作り直すと動作中の Pod の bind mount が古い実体を指して 404 になる。その場合は `kubectl -n pokecalc rollout restart deployment/gateway`)。
- クラスタ(ノード)を作り直すとノード上の画像は消える。もう一度 `make images-k3d`。
- Argo CD が gateway を管理している場合、手元の overlay は上書きされる。この手順は `make api-k3d-deploy` で入れた gateway 向け。
