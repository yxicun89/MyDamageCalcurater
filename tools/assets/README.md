# tools/assets

手元のポケモン画像を WebP 2サイズと `manifest.json` に変換する(ADR-0808)。個人利用に限り、画像を取得する道具もある(`fetch.mjs`。ADR-0810)。画像は Git に入れない(ADR-0002)。

```sh
cd "$(git rev-parse --show-toplevel)"
make assets-fetch LIMIT=5   # 取得(任意。ネットワークが要る。DRY_RUN=1 で計画のみ。LIMIT なしで全件)
make assets                 # 変換
```

### 取得(`fetch.mjs`)
- 対象: 取り込み済みの種族(`data/generated/readmodel/pokemon-types.json`)。key と URL は Showdown スナップショット(`data/importer/config.json` の版)から組む(名前は書かない)。
- 入手元(上から順に試す): 主は GitHub raw の `smogon/sprites`(`src/champions/s{名前}[-o{フォーム}].png` → `src/dex/`。Champions の新しいメガを含む・128px 透過)、補助は Showdown 本体の `sprites/home/{id}.png` → `gen5/`。User-Agent を名乗り、直列(同時 1 本)・500ms 待ち・429/5xx は 2 回まで再試行。
- 既存の `{key}.png` は取り直さない。失敗は理由付きで一覧に出すが終了コード 0。`LIMIT` は未取得のうち N 件。環境変数 `ASSETS_SRC`・`ASSETS_KEYS_FILE`・`ASSETS_SHOWDOWN_SNAPSHOT`・`ASSETS_SPRITES_BASE_URL`(Showdown)・`ASSETS_GITHUB_BASE_URL`。

- 入力 `data/generated/images/src/{key}.{png|jpg|jpeg|webp}`(`ASSETS_SRC` で変更)。`key` は `{図鑑番号4桁}-{フォルム3桁}`(例 `0445-000`)。
- 出力 `data/generated/images/dist/`(`ASSETS_OUT` で変更)。どちらも `.gitignore` 済みで Git に載らない。
- `thumb/{key}.{hash8}.webp`(長辺128px・20KB以下)と `detail/{key}.{hash8}.webp`(長辺512px・100KB以下)。拡大せず縦横比を保つ。容量は品質を下げて守る。
- `manifest.json`: `{"version":1,"images":{"0445-000":{"thumb":"thumb/0445-000.ab12cd34.webp","detail":"detail/0445-000.ef567890.webp"}}}`。
  キーは昇順、パスは `/images/` からの相対。manifest に無いキーは画像なし(クライアントはエンブレム)。出力は決定的(同じ入力なら byte 同一)。
- 形式違反のファイル名・壊れた画像は警告してスキップし、残りを処理する(終了コード 0)。入力が無くても成功し、空の manifest を書く。
- 配信は gateway の `/images/*`(`GATEWAY_IMAGES_DIR`)。手順は `docs/runbooks/images.md`。

テスト: `node --test tools/assets/convert.test.mjs tools/assets/fetch.test.mjs`(先に `cd tools/assets && npm ci`)。
