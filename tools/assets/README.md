# tools/assets

手元のポケモン画像を WebP 2サイズと `manifest.json` に変換する(ADR-0807)。取得ツールは作らない(権利。ADR-0002)。

```sh
cd "$(git rev-parse --show-toplevel)"
make assets
```

- 入力 `data/generated/images/src/{key}.{png|jpg|jpeg|webp}`(`ASSETS_SRC` で変更)。`key` は `{図鑑番号4桁}-{フォルム3桁}`(例 `0445-000`)。
- 出力 `data/generated/images/dist/`(`ASSETS_OUT` で変更)。どちらも `.gitignore` 済みで Git に載らない。
- `thumb/{key}.{hash8}.webp`(長辺128px・20KB以下)と `detail/{key}.{hash8}.webp`(長辺512px・100KB以下)。拡大せず縦横比を保つ。容量は品質を下げて守る。
- `manifest.json`: `{"version":1,"images":{"0445-000":{"thumb":"thumb/0445-000.ab12cd34.webp","detail":"detail/0445-000.ef567890.webp"}}}`。
  キーは昇順、パスは `/images/` からの相対。manifest に無いキーは画像なし(クライアントはエンブレム)。出力は決定的(同じ入力なら byte 同一)。
- 形式違反のファイル名・壊れた画像は警告してスキップし、残りを処理する(終了コード 0)。入力が無くても成功し、空の manifest を書く。
- 配信は gateway の `/images/*`(`GATEWAY_IMAGES_DIR`)。手順は `docs/runbooks/images.md`。

テスト: `node --test tools/assets/convert.test.mjs`(先に `cd tools/assets && npm ci`)。
