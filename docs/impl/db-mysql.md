# MySQL(pokedex DB)の起動・初期化・migration・接続・投入

- 基準: `origin/main` 3379b03 取り込み後。DB を持つのは **pokedex だけ**(`sql.Open` は `services/pokedex/` の 4 か所のみ。record・team・TiDB は未実装。architecture.md)。
- 関連: リソース定義は [k8s-local.md](k8s-local.md)、コマンドの順序は [runbook-commands.md](runbook-commands.md)、環境変数は [config-env.md](config-env.md)。

## 1. データの流れ

```mermaid
flowchart LR
  UP["上流: calc 0.12.0 / Showdown / PokeAPI<br/>(版は data/importer/config.json で固定)"]
  FE["tools/importer/fetch.mjs<br/>(Node)"]
  GEN[("data/generated/<br/>Git 管理外。k8s では PVC pokedex-import-cache")]
  IMP["services/pokedex/cmd/import<br/>importer.Reconcile → RunStore → Apply"]
  DB[("MySQL db=pokedex<br/>18 テーブル + schema_migrations")]
  SVC["pokedex-svc (serve)<br/>/api/pokedex/* · /internal/pokedex/master"]
  EXP["pokedex export<br/>(read model 4 ファイル)"]
  UP --> FE --> GEN --> IMP -->|"1 トランザクションで全置換"| DB
  MIG["pokedex-migrate (up)"] -->|"スキーマ"| DB
  DB --> SVC
  DB --> EXP --> RM["balance / speed の read model"]
```

- 実データは Git に置かない(絶対ルール・ADR-0002)。`data/generated/` は `.gitignore`。Git に入るのは `data/importer/{config,effects,regulations}.json` と `examples/`。
- スキーマ作成(migrate)とデータ投入(import)は**別の担当**。import はスキーマを作らず、テーブルが無ければ `ErrSchemaNotReady`(exit 3。`importer/store.go:52-58`)。サービス起動時の自動 migrate は無い(複数レプリカの競合回避。`job-migrate.yaml:1-2`)。

## 2. MySQL の起動(2 経路。互いに独立)

| | k3d(本番相当のローカル) | `make dev` 用の docker |
|---|---|---|
| 起動 | `make up` → `kubectl apply -k deploy/k8s/overlays/local`(`scripts/up.sh:66`)、`rollout status statefulset/mysql`(`:69`) | `make db-local-up` → `scripts/db-local-up.sh` |
| 実体 | StatefulSet `mysql`(ns pokecalc)、Pod `mysql-0`、`mysql:9.7.2@sha256:29abb0a1…` | docker コンテナ `pokecalc-mysql-local`(同 image・同 digest) |
| 到達 | クラスタ内 `mysql:3306`(headless Service)。ホストからは `kubectl port-forward svc/mysql 3306:3306`(手動。`docs/runbooks/speed.md:33`) | `127.0.0.1:3306`(`-p 127.0.0.1:3306:3306`。`db-local-up.sh:38`) |
| root パスワード | Secret `mysql-auth` の `mysql-root-password`(`up.sh` が `openssl rand -hex 16` で生成) | `.env` の `MYSQL_ROOT_PASSWORD`(空なら失敗。`.env.example` は値なし) |
| DB 作成 | `MYSQL_DATABASE=pokedex`(**初回起動時のみ有効**。`statefulset.yaml:35-38`) | 作らない(コンテナは DB を作らない。DSN 例は `.../pokedex?parseTime=true` だが `pokedex` DB の作成手順はスクリプトに無い) |
| 文字コード | ConfigMap `mysql-config` の `charset.cnf`(utf8mb4 / `utf8mb4_0900_ai_ci`。`configmap.yaml`) | 起動引数 `--character-set-server=utf8mb4 --collation-server=utf8mb4_0900_ai_ci`(`:41-42`) |
| 永続化 | PVC `data-mysql-0` 1Gi(`local-path`)。`make down` でクラスタごと消える | コンテナ内のみ(volume なし。`docker rm` で消える) |
| 停止 | `k3d cluster stop pokecalc`(データは保持) | スクリプトの範囲外(データ削除は人間の確認。CLAUDE.md) |
| 用途 | pokedex・migrate・import が接続 | `make dev` は calc・gateway だけを起動し **DB を使わない**(`scripts/dev.sh:28` は例 JSON を読む)。`make migrate-up`・`make import`・`make test-db` をホストから流す用 |

## 3. 初期化と migration の順序(`make up` の DB 部分)

```mermaid
sequenceDiagram
  participant U as scripts/up.sh
  participant K as kubectl / k3d
  participant M as mysql-0
  participant J as Job pokedex-migrate
  U->>K: k3d cluster create --config deploy/k3d.yaml(無ければ)
  U->>K: apply base/namespace.yaml
  U->>K: Secret mysql-auth を無ければ作成(root pw + pokedex-dsn)
  U->>K: docker build --target migrate/server → k3d image import
  U->>K: delete job pokedex-migrate --ignore-not-found
  U->>K: apply -k overlays/local(mysql・Job・全 base を一括)
  J->>M: initContainer: mysqladmin ping(MYSQL_PWD)を成功まで待つ
  J->>M: /pokedex-migrate up(埋め込み SQL を順に適用)
  U->>K: rollout status statefulset/mysql (180s) → wait job/pokedex-migrate complete (300s)
  U->>K: docker build --target importer → k3d image import
  Note over U,K: データ投入は自動では走らない。make import-k8s(手動 1 回)/ CronJob(土曜 12:00 JST)
```

- Job を `up.sh` が事前に delete するのは、Job の `spec.template` が immutable で再 apply が失敗するため(`up.sh:57-63`)。
- Secret を Git に置かない理由: ADR-0100 §9(`up.sh:30-32`)。

### migration の仕組み

| 項目 | 内容 | 根拠 |
|---|---|---|
| ライブラリ | `golang-migrate/migrate/v4`(mysql driver・iofs source)。公式 CLI は使わず自前の `cmd/migrate`(理由は ADR-0100 §1) | `services/pokedex/db/migrate.go:1-18` |
| SQL の持ち方 | `migrations/*.sql` を `//go:embed` でバイナリに埋め込む(実行版とコードが一致) | `migrate.go:20-23` |
| 接続 | `mysql.ParseDSN` → `MultiStatements = true` を付けて `sql.Open` | `migrate.go:32-41` |
| コマンド | `migrate up` / `migrate version` / `migrate down -confirm <DB名>` | `cmd/migrate/main.go:1-9` |
| `down` の防護 | `-confirm` が空、または DSN の DB 名と不一致なら**接続前に** `ErrDownNotConfirmed`。k8s・スクリプトからは呼ばない | `migrate.go:80-89` |
| 適用済みの記録 | golang-migrate の管理テーブル(既定名 `schema_migrations`。ライブラリの既定で、実クラスタでは未確認) | `migrate.go:45` |
| Job の image | `pokecalc/pokedex-migrate:0.1.0`(`FROM scratch`、`ENTRYPOINT /pokedex-migrate`、`CMD up`。down はイメージに含めない意図) | `services/pokedex/Dockerfile:16-22` |
| make | `migrate-up` `migrate-version`(要 `POKEDEX_DATABASE_DSN`)、`migrate-down`(要 `CONFIRM_DESTROY=<DB名>`) | `Makefile:100-115` |

### migrations(全 14 ファイル = 7 版 × up/down)

| 版 | 名前 | up | down |
|---|---|---|---|
| 000001 | create_types | `types` `type_chart` | 逆順に DROP |
| 000002 | create_master | `abilities` `items` `moves` `species` `species_abilities` `item_effects` `ability_effects` `learnsets` | 逆順に DROP |
| 000003 | create_regulations | `regulations` `regulation_species` `regulation_moves` `regulation_items` `regulation_abilities` | 逆順に DROP |
| 000004 | create_data_versions | `data_versions` | DROP |
| 000005 | widen_species_ability_slot | `species_abilities` の CHECK を slot 1〜3 → 1〜4 | CHECK を戻す |
| 000006 | create_natures | `natures` | DROP |
| 000007 | create_move_effects | `move_effects` | DROP |

## 4. 接続(DSN)

| 利用者 | DSN の出所 | 加工 | 根拠 |
|---|---|---|---|
| pokedex-svc(`serve`) | env `POKEDEX_DATABASE_DSN` ← Secret `mysql-auth` | `parseTime=true` を必ず付与。`sql.Open` のみで**起動時に接続しない**(DB 不在でも起動し、DB を使う操作は 503) | `cmd/pokedex/main.go:43-59, 87-95` |
| `pokedex export` | 同上 | 同上 | `main.go:109-` |
| `pokedex-migrate` | 同上 | `MultiStatements=true` を付与 | `migrate.go:37` |
| `pokedex-import`(CronJob/手動 Job) | 同上(`-dry-run` のときは不要) | なし。DSN 無しは exit 2 | `cmd/import/main.go:116-120` |
| ホストから(`make migrate-up` `make import` `make pokedex-export`) | 手動で `export POKEDEX_DATABASE_DSN=...` | k3d の場合 `kubectl get secret … \| base64 -d \| sed 's/@tcp(mysql:/@tcp(127.0.0.1:/'` でホストを書換え、port-forward 越しに繋ぐ | `docs/runbooks/speed.md:33-38` |

- DSN の形式: go-sql-driver/mysql。`up.sh:42` が作る値は `root:<pw>@tcp(mysql:3306)/pokedex?parseTime=true`(**root 接続**)。エラー文に DSN(パスワード)を含めない(`cmd/pokedex/main.go:42`)。
- calc-svc・gateway・judge・balance・speed は **DB に接続しない**。マスタは HTTP(`http://pokedex`)または read model ファイルで受ける(絶対ルール4。[request-flows.md](request-flows.md))。
- 実クラスタの Secret は 5 キー(役割別 DSN)で Git の `up.sh` と食い違う。[k8s-local.md §9 #1](k8s-local.md)。**このドキュメントの DSN 記述は Git の版**。

## 5. データ投入(import)

| 経路 | 実行者 | 何が走るか |
|---|---|---|
| `make import-fetch` | ホスト | `cd tools/importer && npm ci && node fetch.mjs` → `data/generated/` に取得(要ネットワーク) |
| `make import-dry-run` | ホスト | `go run ./pokedex/cmd/import -data ../data -dry-run`(照合・報告だけ。DB 非接触) |
| `make import` | ホスト | 同上(`-dry-run` なし)。要 `POKEDEX_DATABASE_DSN` |
| `make import-k8s` | k3d | `kubectl -n pokecalc create job --from=cronjob/pokedex-import pokedex-import-manual-<時刻>`(`Makefile:179-184`) |
| CronJob `pokedex-import` | k3d | `0 12 * * 6`(土曜 12:00 JST)。`concurrencyPolicy: Forbid`。cloud overlay では `suspend: true` |

コンテナ内(`tools/importer/cronjob.sh`):

| 手順 | 内容 | 失敗時 |
|---|---|---|
| 1. flock | `data/generated/.import.lock` を非ブロッキングで取得(手動 Job と CronJob の同時実行の排他。ADR-0109) | 取れなければ exit 1(再試行) |
| 2. `node fetch.mjs` | Git に固定した版だけを取得 | — |
| 3. `node check-upstream.mjs` | 上流の最新版の検出(**失敗しても続行**。ログと報告に出すだけ) | 警告のみ |
| 4. `pokedex-import -data /app/data -upstream …/latest.json` | 下記 | 下表の exit code |

`pokedex-import`(`cmd/import/main.go`)のフラグ: `-data`(既定 `../data`)`-dry-run` `-force` `-upstream` `-upstream-max-age`(既定 24h)。

| exit | 意味 | CronJob の扱い(`podFailurePolicy`) |
|---|---|---|
| 0 | 投入した / 版が同じでスキップ / dry-run | 成功 |
| 1 | 再試行で直りうる(DB 接続・I/O) | `backoffLimit: 2` で再試行 |
| 2 | 使い方・設定の誤り(DSN 無し等) | `FailJob`(再試行しない) |
| 3 | 人間の対応が要る(`ErrBlocked` `ErrKeyChanged` `ErrInvalidInput` `ErrInvalidData` `ErrInvalidEffect` `ErrSchemaNotReady`)。DB は変えない | `FailJob` |

投入の中身(`importer.Reconcile` → `RunStore` → `Apply`):

| 段 | 場所 | 内容 |
|---|---|---|
| Reconcile | `importer/reconcile*.go` | 3 ソース(calc・Showdown・PokeAPI)を照合し、報告を `data/generated/reports/` へ書く。食い違いがあれば `ErrBlocked`(exit 3) |
| RunStore | `importer/store.go:65-83` | `AppliedVersions`(`data_versions`)→ `NeedsImport` → 版が同じなら**スキップ**(`-force` で強制)。版が読めない(=migrate 未実施かも)ときは `Apply` を呼ばない |
| Apply | `importer/apply.go:54-` | **1 トランザクションで全置換**: 全テーブルを FK 順に DELETE → INSERT → `data_versions` 更新 → Commit。自己参照 FK(`species.base_species_key`)のためメガは削除が先・挿入が後 |
| key の保護 | `apply.go:71-81` | 既存の `showdown_id` の `key` が変わる/別の `showdown_id` が同じ `key` を奪う投入は `ErrKeyChanged` で拒否(team-svc 等が保存した key の指す種族が入れ替わるのを防ぐ) |

## 6. テーブル(全 18 + 管理テーブル)

ID 列は `ascii_bin`、日本語名は `utf8mb4_ja_0900_as_cs`(ADR-0100 §2)。`name_ja_source` は `pokeapi | override | fallback_en` の CHECK。JSON 列(`effect`)は `JSON_TYPE = 'OBJECT'` の CHECK。

| # | テーブル | 主キー | 主要列・制約 | 版 |
|---|---|---|---|---|
| 1 | `types` | `id` | `sort_order`(UNIQUE)`name_ja` `name_ja_source` | 000001 |
| 2 | `type_chart` | `(attack_type, defense_type)` | `code` ∈ {0,1,2,4}。FK → types ×2 | 000001 |
| 3 | `abilities` | `id` | `name_ja` `name_ja_source` `name_en` | 000002 |
| 4 | `items` | `id` | 同上 | 000002 |
| 5 | `moves` | `id` | `type`(FK) `category`(physical/special/status)`power`(0〜999。status は 0)`accuracy`(NULL 可)`pp` `priority`(-7〜5) | 000002 |
| 6 | `species` | `key`(`CHAR(8)`。`dex_no`-`form` の `NNNN-FFF`。CHECK で導出値と一致) | `dex_no` `form` `showdown_id`(UNIQUE)`type1` `type2` `base_hp/atk/def/spa/spd/spe`(1〜255)`is_mega` `base_species_key`(自己 FK)`required_item_id`(FK)。CHECK: メガは基底種族+必須持ち物が必須 | 000002 |
| 7 | `species_abilities` | `(species_key, slot)` | `ability_id`。slot ∈ 1〜4(000005 で 3→4 に拡張) | 000002/005 |
| 8 | `item_effects` | `item_id` | `effect` JSON | 000002 |
| 9 | `ability_effects` | `ability_id` | `effect` JSON | 000002 |
| 10 | `learnsets` | `(species_key, move_id)` | FK → species・moves(CASCADE) | 000002 |
| 11 | `regulations` | `id` | `name_ja` `is_default`(生成列 `default_marker` の UNIQUE で既定は 1 件だけ)`starts_on` `ends_on` | 000003 |
| 12 | `regulation_species` | `(regulation_id, species_key)` | FK CASCADE | 000003 |
| 13 | `regulation_moves` | `(regulation_id, move_id)` | FK CASCADE | 000003 |
| 14 | `regulation_items` | `(regulation_id, item_id)` | FK CASCADE | 000003 |
| 15 | `regulation_abilities` | `(regulation_id, ability_id)` | FK CASCADE | 000003 |
| 16 | `data_versions` | `source` | `version` `checksum`(64 桁 hex)`imported_at` DATETIME(6)。取り込み版のスキップ判定に使う | 000004 |
| 17 | `natures` | `id` | `plus` `minus`(atk/def/spa/spd/spe。両方 NULL = 無補正)。`(plus, minus)` UNIQUE | 000006 |
| 18 | `move_effects` | `move_id` | `effect` JSON(FK → moves) | 000007 |
| — | `schema_migrations` | — | golang-migrate が作る(既定名。実 DB では未確認) | migrate |

## 7. SQL の書き方と生成物

| 項目 | 内容 | 根拠 |
|---|---|---|
| クエリ | 全 68 本を `services/pokedex/db/query/pokedex.sql` に集約(`-- name:` 68 件)。手書きの SQL 文字列はコードに持たない | `importer/apply.go:3-4` |
| 生成 | `make gen-sql` → sqlc(`engine: mysql`、`schema: migrations`、`out: ../internal/store`、`emit_interface: true`) | `db/sqlc.yaml`、`Makefile:35-37` |
| 生成物 | `internal/store/{db,models,pokedex.sql,querier}.go`(コミットする。`make gen` の差分検査対象) | `sqlc.yaml:1-3` |
| 読み取り側の利用 | `internal/httpapi`(検索・master)、`internal/readmodel`(export) | `cmd/pokedex/main.go:20-22` |

## 8. DB を使うテスト

| 項目 | 内容 |
|---|---|
| `make test-db` | `POKEDEX_TEST_DSN` 必須(未設定は**失敗**)。`go test -tags mysql -p 1 ./pokedex/...`。`make test` には含まれない(`Makefile:117-123`) |
| 架空 seed | `services/pokedex/db/testdata/example_seed.sql`(図鑑番号 9001〜、ID は `test` 始まり、日本語名は「テスト」始まり。実データなし。ADR-0100 §7)。`db/mysql_test.go` が migrate 直後の空 DB に流す |
| レイアウト検査 | `db/layout_test.go` `move_effects_layout_test.go` `natures_layout_test.go`(DB 不要。migration SQL の構造を検査) |
| fake | `internal/storetest`(httpapi のテスト用ストア) |

## 9. 動作確認で DB を見るとき

| 目的 | 見る場所 |
|---|---|
| migrate が通ったか | `kubectl -n pokecalc get job pokedex-migrate`(`Complete`)、`kubectl -n pokecalc logs job/pokedex-migrate`(`up: 完了`) |
| MySQL が Ready か | `kubectl -n pokecalc get pod mysql-0` / `rollout status statefulset/mysql` |
| 投入済みか | `api-smoke` の `pokedex=200`(503 `master_unavailable` なら未投入 → `make import-k8s`)、`docs/runbooks/data.md` の `SELECT COUNT(*) FROM pokedex.species` |
| import の失敗理由 | `kubectl -n pokecalc logs job/<pokedex-import-manual-…>`、exit code は上表 |

## カバレッジ

- 読んだ範囲: `services/pokedex/db/` の `migrate.go`・`sqlc.yaml`・全 migration の up/down(14 ファイル)、`cmd/{migrate,import,pokedex}/main.go`(主要部)、`importer/{apply,store}.go`(主要部)、`tools/importer/cronjob.sh` 全行、`scripts/{up,db-local-up}.sh` 全行、`deploy/k8s` の mysql・pokedex 関連全 YAML、`services/pokedex/Dockerfile` 全行。
- 読めていない箇所: `importer/` の変換・照合ロジック(`convert*.go` `reconcile*.go`)の中身、`tools/importer/*.mjs`(fetch・check-upstream)、`internal/httpapi`・`internal/readmodel` の SQL の使い方、`db/query/pokedex.sql` の各クエリ本文、`db/mysql_test.go`。
- 実 DB で未確認: テーブル一覧・`schema_migrations` の実在(DB のパスワードを読む操作を避け、実クラスタへの `mysql` 接続はしていない。migration SQL からの記述)。役割別 DSN(k8s-local.md §9 #1)の DB 側のユーザー・権限定義は Git に無く読めていない。
- 件数の突き合わせ: `CREATE TABLE` は migration 全体で 000001=2・000002=8・000003=5・000004=1・000006=1・000007=1 = **18**(000005 は ALTER のみ)。表の行数 18 と一致。`-- name:` = 68。
- 未実装・スタブ: TiDB・record DB・team DB(architecture.md)、cloud の MySQL(マネージド DB 想定で overlay に無い。`cloud/cronjob-import-suspend-patch.yaml:1-3`)。
