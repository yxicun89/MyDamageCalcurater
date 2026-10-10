## 2026-10-11: ポケモン画像の取得(I-data-8。データレーンから Web・iOS へ)
Decision: 個人利用に限りポケモン画像を使う(ユーザー決定 2026-10-11)。画像は Git に入れない。入手元は Pokémon Showdown の sprites(home → gen5)。
取得ツール `make assets-fetch` を足し(ADR-0810。ADR-0808 の「取得ツールは作らない」を改めた)、`data/generated/images/src/{key}.png` に置く。以降は既存の `make assets` → gateway `/images/*`。
Reason: key と URL を取り込み済みの Showdown データから機械的に作れる。実測で 349 種族のうち約 98% に画像がある(新しいメガの一部だけ無い)。
Impact(Web・iOS はこれだけで表示できる。契約は ADR-0808 のまま変わらない):
- URL 規則: `GET /images/manifest.json` → `{"version":1,"images":{"0006-001":{"thumb":"thumb/0006-001.ab12cd34.webp","detail":"detail/0006-001.ef567890.webp"}}}`。画像は `/images/` + manifest の相対パス。
- key は pokedex の `pokemonId`(`{図鑑番号4桁}-{フォルム3桁}`)そのまま。メガ・地方の姿も同じ規則。manifest に無い key(2% ほど)・manifest が 404 のときは今のタイプ色エンブレム。
- thumb は長辺 128px、detail は長辺 512px 以下だが入手元の元画像は 192px なので、detail は拡大されず 192px 前後になる(`object-fit: contain` で枠に収める。透過 PNG 由来なので背景は枠側の色)。
- 手順: `make assets-fetch`(ネットワークが要る。LIMIT=5・DRY_RUN=1) → `make assets` → k3d は `make images-k3d`、`make dev` は自動(docs/runbooks/images.md)。
- 取得は直列(同時 1 本が既定かつ上限)・500ms 待ち・取得済みスキップ・User-Agent 明示。robots.txt は空(Disallow なし)、sprites 専用の利用規約は無く、権利は任天堂等のため個人の手元だけで使う(ADR-0810 決定 6)。
- 画像は個人の手元だけ。クラウド overlay・CI には載せない。
