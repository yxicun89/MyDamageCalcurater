# ADR-0100: pokedex のスキーマ(MySQL)・migrate・DB 行から engine 型への写像

- 状態: 提案(P2-2a の仕様。spec-writer 起草、implementer が実装、critic がレビュー)
- 日付: 2026-09-21
- 関連: plan.md P2-2a、ADR-0002(マスタの取得元・確定した方針)、ADR-0005(データ駆動の効果定義)、
  ADR-0012(サービス境界と共通マスタ)、ADR-0013(タイプ相性表はデータ)、ADR-0014(balance TB1 の read model)、
  DECISIONS.md 2026-09-21(フォームの登録単位・CronJob・技の使用可否の既定案)、CLAUDE.md 絶対ルール 2/4/6・ドメイン規約
- 番号: `origin/main` に ADR-0014(balance TB1)があるため 0015 とする
- 更新(P2-2c §10 の実データでの確認で判明。ADR-0103 §12): `species_abilities.slot` の CHECK は実データでは 1..4
  (4 = Showdown の特殊枠 `"S"`。`"H"`(隠れ特性。3)と同じ種族に共存することがある)。既に main に取り込み済みの
  `000002_create_master.up.sql` は書き換えず、`000005_widen_species_ability_slot.up/down.sql` で ALTER TABLE により広げた。

## 背景

P2-2 で共通マスタ(ポケモン・技・持ち物・特性・タイプ相性・レギュレーション)を MySQL に置く。
ADR-0002 の §P2-2 はテーブルの骨格だけを示し、ADR-0013 は `types` / `type_chart` を DB に置くとだけ決めている。
次を1つの ADR で固める必要がある: スキーマの所有者と配置、テーブル・列・制約、migrate の実行方法、
engine の型への写像の置き場所、ローカル環境、架空データの example、balance から見た契約。

## 決定

### 1. 所有者と配置

- スキーマの所有者は **pokedex-svc**。DB 名は `pokedex`。他サービスはこの DB に接続しない(絶対ルール4)。
  calc-svc(P3-1)は pokedex-svc の API から読むか、pokedex が出力する read model を読む(§8)。
- 配置(Go module は既存の `services`。新しい module は作らない):

| パス | 内容 |
|---|---|
| `services/pokedex/db/migrations/` | golang-migrate の SQL。`NNNNNN_<snake_title>.up.sql` / `.down.sql`(6桁連番、1 から隙間なし。`migrate create -seq -digits 6` の形) |
| `services/pokedex/db/query/*.sql` | sqlc のクエリ(`-- name: Xxx :many` 形式) |
| `services/pokedex/db/sqlc.yaml` | sqlc 設定(version "2"、engine mysql、schema `migrations`、queries `query`、gen.go の package `store`・out `../internal/store`) |
| `services/pokedex/internal/store/` | sqlc の生成物(コミットする。`make gen` の差分検査の対象) |
| `services/pokedex/db/migrate.go` | package `db`。`//go:embed migrations/*.sql` の `Migrations embed.FS`、`Up`、`DownAll`、`ErrDownNotConfirmed`(§5) |
| `services/pokedex/cmd/migrate/` | migrate の CLI(`up` / `version` / `down -confirm <DB名>`)。golang-migrate の library + `iofs` source + mysql driver |
| `services/pokedex/db/testdata/example_seed.sql` | 架空データの example(§7) |
| `services/internal/master/` | DB 行 → engine 型の写像(§6)。pokedex-svc と calc-svc が共有する |

- migrate の CLI をバイナリにせず自前コマンドにする理由: golang-migrate の公式 CLI は mysql driver をビルドタグで選ぶため
  `go tool` で固定しづらい。自前コマンドなら migrations を embed でき、版がコードと一致し、`down` の確認(§5)を強制できる。
- sqlc は `tools/go.mod` の `tool github.com/sqlc-dev/sqlc/cmd/sqlc` で版を固定し、`make gen-sql` で実行する。
  `make gen` は `gen-go gen-sql gen-ts` にする(既存の oapi-codegen に sqlc を足す)。services の go.mod に sqlc の
  依存を入れないため tools module に置く。sqlc のビルドは cgo を使う(macOS は Xcode Command Line Tools で足りる)。

### 2. 共通の型規則

- 文字コードは `utf8mb4`。**ID 列は `CHARACTER SET ascii COLLATE ascii_bin`**。大文字小文字を区別しない照合順序だと
  `REGEXP '^[a-z0-9]+$'` の CHECK が大文字を通してしまうため。外部キーの列は参照先と同じ文字コード・照合順序にする。
- `name_ja` は `VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_ja_0900_as_cs NOT NULL`、`CHECK (CHAR_LENGTH(name_ja) > 0)`。
  前方一致検索(P2-3)のためにこの照合順序にする。
- ID の形式:
  - タイプ: `^[a-z]+$`(ADR-0013・Shared Interfaces の英語小文字 ID。例 `fire`)
  - 技・持ち物・特性: `^[a-z0-9]+$`(Showdown / @smogon/calc の `toID` と同じ形。例 `flamethrower`)。照合に使うため取得元の ID をそのまま主キーにする
  - 種族: `key` = `{図鑑番号4桁}-{フォルム3桁}`(Shared Interfaces。例 `0445-000`)。`showdown_id`(`^[a-z0-9]+$`)を別列に持ち、取得元との突き合わせに使う
  - レギュレーション: `^[a-z0-9]+(-[a-z0-9]+)*$`(例 `m-c`)。**値は DB のデータで、コードに書かない**
- 真偽は `TINYINT(1) NOT NULL` + `CHECK (col IN (0,1))`。
- MySQL の制約: CHECK に使う列は `ON DELETE/UPDATE CASCADE | SET NULL` の外部キーに使えない。
  そのため `species.base_species_key` / `species.required_item_id` の外部キーは既定(RESTRICT)にする。
- **migrations に実データの INSERT を書かない**。タイプの一覧もマスタ(ADR-0013)なので importer が入れる。
- MySQL は 8.4(LTS)を前提とする(CHECK 制約・`REGEXP_LIKE`・JSON・生成列を使う)。

### 3. テーブル

migrations の分け方: `000001_create_types`(types, type_chart)/ `000002_create_master`(abilities, items, moves, species,
species_abilities, item_effects, ability_effects, learnsets)/ `000003_create_regulations`(regulations と4つの集合)/
`000004_create_data_versions`。down は up の逆順で DROP する。

| テーブル | 列(型は要点のみ) | 制約 |
|---|---|---|
| `types` | `id` VARCHAR(16)、`sort_order` SMALLINT UNSIGNED、`name_ja`、`name_ja_source` | PK(id)、UNIQUE(sort_order)、CHECK id `^[a-z]+$` |
| `type_chart` | `attack_type`、`defense_type`、`code` TINYINT UNSIGNED | PK(attack_type, defense_type)、FK 両方 → types(id)、CHECK `code IN (0,1,2,4)`(ADR-0013 の ×2 整数コード) |
| `abilities` | `id` VARCHAR(64)、`name_ja`、`name_ja_source`、`name_en` | PK(id)、CHECK id `^[a-z0-9]+$` |
| `items` | 同上 | 同上 |
| `moves` | `id`、`name_ja`、`name_ja_source`、`name_en`、`type`、`category` VARCHAR(16)、`power` SMALLINT UNSIGNED、`accuracy` TINYINT UNSIGNED NULL(NULL=必中)、`pp` TINYINT UNSIGNED、`priority` TINYINT | PK(id)、FK type → types、CHECK id 形式・`category IN ('physical','special','status')`・`power BETWEEN 0 AND 999`・`category <> 'status' OR power = 0`・`accuracy IS NULL OR accuracy BETWEEN 1 AND 100`・`pp BETWEEN 1 AND 64`・`priority BETWEEN -7 AND 5` |
| `species` | `key` CHAR(8)、`dex_no` SMALLINT UNSIGNED、`form` SMALLINT UNSIGNED、`showdown_id`、`name_ja`、`name_ja_source`、`name_en`、`type1`、`type2` NULL、`base_hp`〜`base_spe`(6列 SMALLINT UNSIGNED)、`is_mega`、`base_species_key` NULL、`required_item_id` NULL | PK(key)、UNIQUE(showdown_id)、UNIQUE(dex_no, form)、FK type1/type2 → types、FK base_species_key → species(key)、FK required_item_id → items(id)。CHECK: `key = CONCAT(LPAD(dex_no,4,'0'),'-',LPAD(form,3,'0'))`、`dex_no BETWEEN 1 AND 9999`、`form BETWEEN 0 AND 999`、`type2 IS NULL OR type2 <> type1`、各種族値 `BETWEEN 1 AND 255`、**メガの整合** `(is_mega = 1 AND base_species_key IS NOT NULL AND required_item_id IS NOT NULL) OR (is_mega = 0 AND base_species_key IS NULL AND required_item_id IS NULL)`、`base_species_key IS NULL OR base_species_key <> key` |
| `species_abilities` | `species_key`、`slot` TINYINT UNSIGNED、`ability_id` | PK(species_key, slot)、UNIQUE(species_key, ability_id)、FK species(CASCADE)・abilities、CHECK `slot IN (1,2,3)`(3 = 隠れ特性)。**更新**: 実データでは 1..4(4 = Showdown の特殊枠 `"S"`)。`000005_widen_species_ability_slot` で広げた。ADR-0103 §12 参照 |
| `item_effects` | `item_id`、`effect` JSON | PK(item_id)、FK items(CASCADE)、CHECK `JSON_TYPE(effect) = 'OBJECT'` |
| `ability_effects` | `ability_id`、`effect` JSON | 同上(abilities) |
| `learnsets` | `species_key`、`move_id` | PK(species_key, move_id)、FK species(CASCADE)・moves(CASCADE) |
| `regulations` | `id` VARCHAR(32)、`name_ja`、`is_default`、`default_marker` TINYINT 生成列 `AS (IF(is_default = 1, 1, NULL)) STORED`、`starts_on` DATE NULL、`ends_on` DATE NULL | PK(id)、**UNIQUE(default_marker)**(既定のレギュレーションは高々1件)、CHECK id 形式・`ends_on IS NULL OR starts_on IS NULL OR starts_on <= ends_on` |
| `regulation_species` / `regulation_moves` / `regulation_items` / `regulation_abilities` | `regulation_id` + `species_key` / `move_id` / `item_id` / `ability_id` | PK(2列)、FK 両方(CASCADE) |
| `data_versions` | `source` VARCHAR(32)、`version` VARCHAR(128)、`checksum` CHAR(64)、`imported_at` DATETIME(6) | PK(source)、CHECK source `^[a-z0-9]+(-[a-z0-9]+)*$`・`CHAR_LENGTH(version) > 0`・checksum `^[0-9a-f]{64}$`(sha256 の16進) |

PokeAPI 由来の名前を持つテーブル(`regulations` を除く)の `name_ja_source` は `VARCHAR(16)` +
`CHECK (name_ja_source IN ('pokeapi','override','fallback_en'))`。`regulations` は人が管理する
定義(名前は override 前提。§3 判断)で PokeAPI から取得しないため、この列を持たない
(`mysql_test.go`・`example_seed.sql` の `regulations` への INSERT も `name_ja_source` を渡さない)。

判断と理由:

- **日本語名は各テーブルの列**に、override を反映した**解決済みの値**を入れる。別テーブル(多言語化)にしない:
  v1 は日本語だけで、検索・表示のたびに JOIN するほどの利点が無い。override の反映は importer(P2-2b)が行い、
  出どころを `name_ja_source` に残す(欠落件数の報告と、英語名フォールバックの検出に使う。ADR-0002 未決 #8)。
- **使用可否は行の `available` 列でなく、レギュレーションごとの集合テーブル**にする(ADR-0002 §P2-2 草案の `available` を置き換える)。
  M-C を直書きせず、M-D を足すときは `regulations` と集合の行を足すだけで済む。「現行」はコードに書かず
  `regulations.is_default` で持つ。逆算の持ち物候補は `regulation_items ∩ item_effects` から導出し、専用のテーブルを持たない。
  メガのポケモンが集合にあるなら `required_item_id` も同じ集合の `regulation_items` にあること、の検査は行をまたぐため importer の検証で行う。
- **効果定義は JSON 列**(正規化しない)。engine の `ItemEffect` / `AbilityEffect` は map(`StatMods`・`DefResistType`)を含み、
  効果の種類が増えるたびに列や EAV の行を足すのは重い。キーは ADR-0005 の `testdata/golden/effects.json` と同じ
  (engine の Go のフィールド名。例 `{"DamageMod":5324}`)にし、ゴールデンのアダプタと DB が同じ形になるようにする。
  型の安全は写像側の厳格なデコード(§6)で担保する。効果は items と別テーブル: 取得元(人が管理する定義ファイル。ADR-0002 §P2-2)が
  items と違い、行が無いこと = 補正なし、とできるため。空のオブジェクト `{}` は「補正なし」と区別がつかないので不正とする。
- **習得技はレギュレーションに依存させない**。使える技 = `learnsets ∩ regulation_moves`(DECISIONS の既定案: 断定できない技は使用可として残す。P2-1c)。
- **フォーム**: 性能(種族値・タイプ・特性)が同じ見た目違いは1行、性能が違うフォームとメガは別行(DECISIONS 2026-09-21)。
  フォルム番号の採番表は importer(P2-2b)の管理とし、スキーマは `form` と `showdown_id` だけを持つ。
- **性格はマスタにしない**。25 種の補正はゲームのルール(engine の固定)で、取得元の版で変わるデータではない。
  `/api/pokedex/natures` は engine の定義から返す(P2-3)。
- **SP の制約は pokedex のスキーマに無い**。SP は個体(team-svc / 計算の入力)の値で、マスタの列ではない。
- 重量・技の追加効果・命中以外の詳細は engine が使っていないので v1 のスキーマに入れない(必要になったら migration を足す)。
- `data_versions` は取得元ごとの**最新の適用済み版**を1行で持つ。importer はデータの置き換えと同じトランザクションで更新する。
  CronJob(P2-2d)は取得元の `checksum` が一致すれば取り込まない。取得元の名前の一覧は DB の CHECK に列挙しない(形式だけ検査)。

### 4. engine の型との対応

| DB | engine |
|---|---|
| `types` + `type_chart` | `engine.TypeChartData`(Types は `sort_order` 順) → `engine.NewTypeChart` |
| `species` + `species_abilities` | `engine.Species`(Types は `[type1]` か `[type1, type2]`、Abilities は slot 順) |
| `moves` | `engine.Move`(ID, NameJa, Type, Category, Power, Priority) |
| `items` + `item_effects` | `engine.Item`(行が無ければ `Effect == nil`) |
| `abilities` + `ability_effects` | `engine.Ability`(同上) |

メガの3列・`showdown_id`・`name_en`・`accuracy`・`pp` は engine の型に無い(計算に使わない)。写像はそれらを検証だけする。

### 5. migrate の実行

- package `example.com/pokecalc/services/pokedex/db`:
  - `var Migrations embed.FS`(`migrations/*.sql`)
  - `func Up(dsn string) error` — 最新まで上げる。DSN は go-sql-driver/mysql 形式。`multiStatements=true` は関数内で付ける
  - `func DownAll(dsn, confirmDatabase string) error` — すべて戻す。**`confirmDatabase` が DSN の DB 名と一致しないときは接続せずに
    `ErrDownNotConfirmed` を返す**。スキーマが無い DB に対しては何もせず nil
  - `var ErrDownNotConfirmed`
- Makefile(implementer が追加):
  - `migrate-up`: `POKEDEX_DATABASE_DSN` に対して `go run ./pokedex/cmd/migrate up`(services ディレクトリで)
  - `migrate-version`
  - `migrate-down`: `CONFIRM_DESTROY=<DB名>` が無ければ失敗する。**他のターゲット・スクリプト・k8s から呼ばない**
    (クラスタ削除・DB のデータ削除は人間の確認が必要。CLAUDE.md)
  - `test-db`: `cd services && go test -tags mysql ./pokedex/db/...`。`POKEDEX_TEST_DSN` が無ければ**失敗**する(スキップしない)。
    テストは DB 名が `_test` で終わらない DSN を拒否する(全テーブルを消すため)。`make test` には含めない
  - `db-local-up`: `make dev` 用に docker で `mysql:8.4` を `127.0.0.1:3306` に起動(パスワードは `.env`。Git に入れない)
  - `gen-sql`(§1)と `gen: gen-go gen-sql gen-ts`
- サービスの起動時に自動で migrate しない。k8s では Job `pokedex-migrate`(`migrate up` だけ)で流す。
  複数レプリカの起動競合を避け、`down` を流す経路を作らないため。

### 6. 写像 `services/internal/master`

engine は純粋に保つ(絶対ルール2)ので engine の外に置く。sqlc の生成型には依存させず、写像は素朴な行の型を受ける
(store 層が sqlc の行をこの型に詰め替える)。sqlc の出力名の変化に写像とそのテストを巻き込まないため。

```go
var ErrInvalidRow, ErrInvalidEffect error

type TypeRow struct{ ID string; SortOrder int; NameJa string }
type TypeChartRow struct{ AttackType, DefenseType string; Code int }
func TypeChartData(types []TypeRow, chart []TypeChartRow) (engine.TypeChartData, error)
func TypeChart(types []TypeRow, chart []TypeChartRow) (engine.TypeChart, error)

type SpeciesRow struct {
    Key string; DexNo, Form int; ShowdownID, NameJa, NameEn string
    Type1, Type2 string // "" = NULL
    BaseHP, BaseAtk, BaseDef, BaseSpA, BaseSpD, BaseSpe int
    IsMega bool; BaseSpeciesKey, RequiredItemID string // "" = NULL
}
type SpeciesAbilityRow struct{ Slot int; AbilityID string }
func Species(row SpeciesRow, abilities []SpeciesAbilityRow, chart engine.TypeChart) (engine.Species, error)

type MoveRow struct{ ID, NameJa, Type, Category string; Power, Priority int }
func Move(row MoveRow, chart engine.TypeChart) (engine.Move, error)

type ItemRow struct{ ID, NameJa string; Effect []byte }    // Effect nil = item_effects に行が無い
type AbilityRow struct{ ID, NameJa string; Effect []byte }
func Item(row ItemRow, chart engine.TypeChart) (engine.Item, error)
func Ability(row AbilityRow, chart engine.TypeChart) (engine.Ability, error)

func DecodeItemEffect(raw []byte, chart engine.TypeChart) (*engine.ItemEffect, error)
func EncodeItemEffect(e engine.ItemEffect) ([]byte, error)
func DecodeAbilityEffect(raw []byte, chart engine.TypeChart) (*engine.AbilityEffect, error)
func EncodeAbilityEffect(e engine.AbilityEffect) ([]byte, error)
```

- 行の検証(DB の CHECK と同じ規則を写像でも持つ。DB を通らない経路=read model・テストでも同じ規則にするため)は
  `ErrInvalidRow` で包む。タイプの妥当性は**渡された表(`chart.Has`)で判定**し、タイプの一覧をコードに書かない。
  `TypeChart` の失敗は `engine.ErrInvalidTypeChart` でも判別できる(engine の検証をそのまま通す)。同じ組の重複は `ErrInvalidRow`。
- 効果の JSON のデコードは厳格にし、失敗は `ErrInvalidEffect` で包む:
  未知のフィールド・JSON オブジェクト以外・後続のデータ・空(補正が1つも無い)・負の値・小数(4096 基準の**整数**のみ)・
  `StatMods` のキーが `atk/def/spa/spd/spe` 以外・タイプが表に無い・`PowerCategory` が `physical/special` 以外・
  片方だけの組(`BoostType` と `BoostTypeMod`、`OffBoostType` と `OffBoostTypeMod`、`PowerCategory` だけで `PowerMod` が無い、
  `OnlySuperEffective` だけで `DamageMod` が無い)を拒否する。
- エンコードは正準形: ゼロ値のフィールドを省く、フィールドは struct の定義順、map のキーは昇順。空の効果はエラー。
  `Decode(Encode(e)) == e`(情報が落ちない)。engine にフィールドが増えたら、全フィールドを埋めた fixture のテストが落ちて写像の更新を強制する。

### 7. 架空データの example

`services/pokedex/db/testdata/example_seed.sql`。図鑑番号 9001 以降・ID は `test` で始まる英小文字・日本語名は `テスト` で始まる
(ADR-0014 の example と同じ規則。check-publishable の D 検査の趣旨)。タイプ ID だけは一般名(`fire` 等)を使うが、
相性のコードは架空の小さな表でよい。`make test-db` のテストがこれを流し込み、写像まで通す。
implementer は `scripts/check-publishable.sh` の `NAMEJA_PATHSPECS` に `services/*_test.go` を足す。

### 8. balance から見た契約(ADR-0012・ADR-0014)

- 共通マスタの恒久正本は **pokedex の DB(このスキーマ)と、それを満たす importer の入力**の1つ。
- balance は pokedex の DB に接続しない。ADR-0012 §6(共通マスタのためだけに実行時のサービス間依存を足さない)に従い、
  **pokedex が出力する read model** を読む。形は ADR-0014 §2 の JSON(`{"schemaVersion":1,"pokemon":[{"pokemonId","types"}]}`)で、
  `species.key` → `pokemonId`、`[type1, type2]` → `types`。タイプ相性は `types` / `type_chart` から同様に出力する(形は balance の TB0 契約に合わせて別途)。
  出力コマンド(`pokedex export`)は P2-3 で作る。これで balance の adapter を差し替えるだけで正本に切り替わる。
- DECISIONS.md 2026-09-21「balance-svc は pokedex-svc の API を呼ぶ」は ADR-0012 で置き換え済み。実行時 API にするなら ADR-0012 の見直しが先。

### 9. ローカル環境(k8s)

- `deploy/k8s/overlays/local/mysql/`: StatefulSet(`mysql:8.4`、replicas 1、volumeClaimTemplates 1Gi)、headless Service `mysql`、
  文字コード設定の ConfigMap。cloud overlay には入れない(マネージド DB を想定。後続)。
- Secret はコミットしない。`scripts/up.sh` が `mysql-auth` が無いときだけ乱数で作る(既存は上書きしない)。
  manifest には `kind: Secret` の `data` / `stringData` を書かない。
- `deploy/k8s/base/pokedex/`: Job `pokedex-migrate`(`migrate up`。DSN は Secret から)。イメージは `services/pokedex/Dockerfile` の
  migrate ターゲットで作り、`make up` が k3d に import する。
- データの削除(PVC の削除・`migrate-down`)は自動化しない。
- `deploy/k8s/base/pokedex/` は cloud overlay にも(`base` 経由で)入るが、cloud overlay には
  `mysql` StatefulSet・`mysql-auth` Secret・`pokedex-migrate` イメージの供給経路が無い。
  cloud を実際に使う前に、cloud overlay で Job を外すか、マネージド DB 用の Secret・イメージ配布
  (レジストリ push 等)を用意すること。

## 却下案

- **migrations を importer(tools/importer)に置く**: スキーマの所有者は pokedex-svc で、importer はその DB への書き手の1つに過ぎない。
- **効果定義を正規化(効果の種類ごとの列・行)**: engine の型が増えるたびに migration が要り、map を持つ型と往復しづらい。
- **日本語名の別テーブル**: v1 で多言語は要らない。
- **`available` 列**: レギュレーションが1つに固定され、M-C の直書きと同じ問題になる。
- **起動時の自動 migrate**: 複数レプリカの競合、down の経路の混入。
- **sqlc の生成型を写像の入力にする**: 生成名の変化で写像とテストが壊れる。

## 限界・未決

- sqlc と golang-migrate は services / tools の go.mod に依存を足す(DEPENDENCIES の記録は implementer)。
- balance 向け read model の出力と相性表の形は P2-3 で詰める。
- CHECK 制約の文法は MySQL 8.4 で確認する(TiDB には移さない。TiDB は record/team 用)。
