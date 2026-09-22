# pokedex(マスタの DB と取込)

ポケモン・技・持ち物・特性・タイプ相性・レギュレーション(v1 は M-C)の共通マスタを MySQL に持つ。
スキーマは golang-migrate で管理し、取得済みスナップショット(`tools/importer`)を照合して冪等に投入する CLI がある。
HTTP は1バイナリ `pokedex`(`cmd/pokedex`)。`serve` は公開の検索 API `/api/pokedex/*`(種族・技・持ち物・
性格。既定のレギュレーションの使用可能集合に絞る)と、calc-svc が起動時に読む内部 API
`GET /internal/pokedex/master`(絞らない全件・`/internal` は gateway でも 404)を提供する。
`export -out <dir>` は balance・speed が読む read model(pokemon-types / moves / abilities / speed-pokemon の
4 JSON。`make pokedex-export`)を書く。手順書は [`docs/runbooks/data.md`](../../docs/runbooks/data.md)。

```mermaid
flowchart LR
  Src["取得元<br/>calc 0.12.0 / Showdown / PokeAPI"] --> Fetch["tools/importer<br/>(取得・Node)"]
  Fetch --> Gen[("data/generated/<br/>Git 管理外")]
  Gen --> Rec["pokedex/importer<br/>Reconcile(照合)→ Run(投入)"]
  Mig["db/migrations<br/>(golang-migrate)"] --> DB[("MySQL<br/>pokedex DB")]
  Rec --> DB
  Cron["CronJob pokedex-import<br/>(週1回)"] -.-> Rec
```

## ディレクトリ

| パス | 役割 |
|---|---|
| `db/migrations` | スキーマ(golang-migrate の up/down) |
| `db/query` | sqlc 用の SQL(生成先は `internal/store`) |
| `internal/store` | sqlc の生成物(手で書かない。`make gen-sql`) |
| `importer` | 取得済みスナップショットの変換・照合(`Reconcile`)・DB への投入(`RunStore`) |
| `cmd/migrate` | migrate CLI(`up` / `down` / `version`) |
| `cmd/import` | import CLI(照合・報告・投入。`-dry-run` で DB に触れず確認できる) |
| `cmd/pokedex` | HTTP サーバ(`serve`)と read model の書き出し(`export -out <dir>`) |
| `internal/httpapi` | HTTP 境界。生成物 `services/internal/api` の `ServerInterface` を実装する |
| `internal/readmodel` | `pokedex export` の中身(DB → balance・speed 向け JSON)。HTTP に依存しない |
| `internal/storetest` | テスト専用の偽の `store.Querier` と架空データ(本番のコードから import しない) |

## よく使うコマンド

```sh
cd "$(git rev-parse --show-toplevel)"
make migrate-up          # POKEDEX_DATABASE_DSN が必須
make import-dry-run      # 変換と報告だけ(DB には触らない)
make import              # 投入(POKEDEX_DATABASE_DSN が必須。取得は make import-fetch)
make import-k8s          # k3d 上の CronJob を手動で1回流す
make pokedex-export      # read model を data/generated/readmodel/ に書く(POKEDEX_DATABASE_DSN が必須)
make test-db             # DB を使うテスト(POKEDEX_TEST_DSN が必須。make test には含めない)
```

## 関連 ADR

[0002](../../docs/adr/0002-master-data-source.md)(取得元と責務分離)・
[0100](../../docs/adr/0100-pokedex-schema-and-migrate.md)(スキーマ・migrate)・
[0101](../../docs/adr/0101-importer-fetch-convert-load.md)(取得・変換・投入)・
[0102](../../docs/adr/0102-data-lane-dependency-pins-2026-09.md)(依存の版固定)・
[0103](../../docs/adr/0103-importer-reconcile-report-and-pins.md)(照合・差分報告)・
[0104](../../docs/adr/0104-importer-cronjob-and-make-import.md)(CronJob・make import)・
[0105](../../docs/adr/0105-pokedex-svc-internal-api-export-natures.md)(検索 API・内部 API・natures・export)。
