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

k3d では現状は画像を出せない(gateway イメージは `FROM scratch` で `kubectl cp` できず、読み取り専用ファイルシステムで、Argo CD の selfHeal が `set env` を戻す)。`/images/*` は 404 のまま全機能が動く。恒久配線(volume mount)は別タスク。
