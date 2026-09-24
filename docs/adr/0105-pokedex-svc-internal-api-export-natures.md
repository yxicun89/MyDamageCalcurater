# ADR-0105: pokedex-svc(検索 API・内部 API・natures・pokedex export・k8s)

- 状態: 提案(P2-3 の仕様。spec-writer 起草、implementer が実装、critic がレビュー)
- 日付: 2026-09-22
- 関連: plan.md P2-3、ADR-0100(スキーマ・§8 balance の read model・「性格はマスタにしない」)、ADR-0101(importer)、
  ADR-0104(CronJob・checksum)、ADR-0012(§6 実行時のサービス間依存を足さない)、ADR-0017(特性の正規化された効果)、
  ADR-0200 §2(NatureID)、ADR-0202(gateway)、ADR-0204(内部 API の契約 MasterExport)、ADR-0401 §5・§7(TB5 のカタログ)、
  ADR-0402(balance の JSON Schema)、ADR-0600 §4(speed の read model)、DECISIONS.md 2026-09-22(API レーン・TB レーン・素早さレーンの依頼)、
  CLAUDE.md 絶対ルール 1・2・4・6
- 番号: データレーンの帯(0100〜)の6本目

## 背景

pokedex の DB(ADR-0100)と importer(ADR-0101〜0104)はできたが、DB を読むサービスが無い。3つのレーンから依頼がある:
API レーン(calc-svc が起動時に取るマスタ一式 `GET /internal/pokedex/master`・性格のマスタ)、タイプバランスレーン
(`pokedex export` に nameJa・abilityIds・レギュレーションでの絞り込み・特性の read model)、素早さレーン(素早さの種族値)。
公開の検索 API の契約(`/api/pokedex/*`)は api/openapi.yaml に既にある(API レーンの持ち物。ここでは変えない)。

## 決定

### 1. 構成

| パス | 内容 |
|---|---|
| `services/pokedex/cmd/pokedex/` | 1つのバイナリ `pokedex`。サブコマンド `serve`(HTTP)と `export -out <dir>`(read model の出力)。設定は環境変数 `POKEDEX_DATABASE_DSN`(必須)・`POKEDEX_ADDR`(既定 `:8080`) |
| `services/pokedex/internal/httpapi/` | Echo v5 の HTTP 境界。生成物 `services/internal/api`(API レーンの `make gen` の出力)の `ServerInterface` を実装し、`ServerInterfaceWrapper` で公開操作(検索5 + `getMove` + `getMovesByIds`。2026-09-23・2026-09-24 追記)を登録する。**新しい生成物を作らない・生成型を手書きしない** |
| `services/pokedex/internal/readmodel/` | `pokedex export` の中身(DB → balance・speed 向けの JSON)。HTTP に依存しない |
| `services/pokedex/internal/store/` | sqlc の生成物(既存。§2 のクエリを追加して再生成済み) |
| `services/pokedex/internal/storetest/` | テスト専用の偽の `store.Querier` と架空データ(本番のコードから import しない) |

- httpapi と readmodel は **`store.Querier`(sqlc の生成インターフェース)を受ける**。テストはこれを storetest の偽物に差し替える。
  sqlc の生成型は pokedex の中に閉じる(共通マスタ `services/internal/master` には渡さない。ADR-0100 §6 の方針のまま)。
- 別の module は作らない(`services` module。oapi-codegen の生成の流儀は calc-svc と同じ: 生成物を import して実装するだけ)。
- `serve` は起動時に DB へ接続しない(`sql.Open` だけ)。DB が無くても起動し、DB を使う操作が 503 を返す(calc-svc の URL 方式と同じ考え)。
  DSN は go-sql-driver/mysql の形式で、`parseTime=true` を必ず付ける(`regulations` の DATE を `sql.NullTime` で読むため)。
  DSN が壊れていれば起動エラー。エラー文に DSN(パスワード)を含めない。
- 運用エンドポイント: `GET /healthz` → 200 `{"status":"ok"}`(DB に触れない。readiness も同じ。データの有無は各操作の 503 で表す)。
- calc の3操作は pokedex-svc の担当外なので、生成ラッパを経由させず 404 `not_found`(calc-svc の R1 と対称)。
  ルート無し・メソッド違いも 404 `not_found`、panic は 500 `internal`(Error 形式・内部情報を出さない)。

### 2. 内部 API `GET /internal/pokedex/master`(契約は api/openapi.yaml の MasterExport。ADR-0204)

- ヘッダを要求しない(生成ラッパを経由しても引数が無いので検証は無い)。gateway は `/internal/*` を 404 にする(API レーンで固定済み)。
- 読み出し(sqlc。`db/query/pokedex.sql`): `ListDataVersions`・`ListTypes`・`ListTypeChart`・`ListSpecies`・`ListAllSpeciesAbilities`・
  `ListMoves`・`ListItems`・`ListItemEffects`・`ListAbilities`・`ListAbilityEffects`・`ListNatures`。**使用可能集合で絞らない**。
  v1 は1リクエストの読み出しを1つのトランザクションにまとめない(ハンドラは `store.Querier` だけを受ける)。importer の置き換え
  (1トランザクション)の最中に読むと、クエリごとに新旧が混ざり得る(週1回の CronJob の数秒だけ)。混ざった結果の参照の欠落は
  calc-svc の検証で失敗して再試行される(ADR-0204 §3)。限界の5に記す。
- 写し方:
  - `dataVersion` = `data_versions` の `source=version` を source 昇順に `,` で連結(例 `calc=0.12.0,pokeapi=…,showdown=…`)。
  - species: 行 + その種族の `species_abilities`(slot 昇順。slot 4 もそのまま。内部 API は切り詰めない)。`type2`・`baseSpeciesKey`・
    `requiredItemId` の NULL は JSON の null。`showdownId` を含める。
  - items / abilities: `effect` は `item_effects` / `ability_effects` の JSON を**そのまま**。行が無ければ `"effect": null`(キーは省かない)。
    **数値の字面を保つ**(`json.Decoder.UseNumber` で `api.MasterEffect` に入れるか、`json.RawMessage` を経由する。float64 を経由すると
    `5324.0` → `5324`・`2^53+1` → `2^53` になり、calc-svc の厳格デコードと共通マスタの整数検査をすり抜ける。ADR-0204 §2)。
    JSON が壊れている・オブジェクトでない行は 503(DB の CHECK があるので通常は起きない)。
  - natures: `plus` / `minus` の NULL は null。
- 503 `master_unavailable`(本文は Error。DB のエラー文を出さない): DB に接続できない・クエリの失敗・**`data_versions` が0行(未投入)・
  `types` が0行・`natures` が0行(000006 の後に未投入)**・効果の JSON が不正。部分的なマスタを 200 で返さない。
- pokedex 側では値の検証(ID の形式・参照の整合)をしない。受け取った calc-svc が共通マスタの写像で検証する(ADR-0204 §2)。

### 3. 公開の検索 API(契約は既存。API レーンへの依頼は末尾)

- 必須ヘッダ(X-Device-Id / X-Session-Id)の有無は生成ラッパで検証し、欠落・空は 400 `missing_header`、重複は `invalid_header`
  (calc-svc と同じ写し方。UUID の形式は gateway だけが見る。ADR-0202)。
- 既定のレギュレーションは `GetDefaultRegulation`(`is_default = 1`)で毎回 DB から引く。**レギュレーション ID をコードに書かない**。
  無ければ(未投入)503 `master_unavailable`。
- `searchSpecies` / `searchMoves` / `searchItems`: 既定のレギュレーションの使用可能集合(`regulation_*`)だけを返す。
  - `q` は日本語名の前方一致。LIKE の特殊文字 `\` `%` `_` を `\` でエスケープし、末尾に `%` を付けたパターンを `Search*` の `pattern` に渡す
    (エスケープの責任はハンドラの1か所。SQL 側で CONCAT しない)。省略・空は `%`(全件)。前方一致とひらがな/カタカナの同一視は
    DB の照合順序 `utf8mb4_ja_0900_as_cs`(ADR-0100 §2)に任せる。
  - `limit` は既定 50、1〜200(契約の min/max)。範囲外・整数でない値は 400 `invalid_input`(生成ラッパのクエリの bind 失敗も
    `invalid_input` に写す。ヘッダの失敗と区別する)。
  - `format`(species のみ)は `single` / `double` を受け付け、v1 では結果に影響しない(使用可能集合は形式で分かれていない)。
    未知の値は 400 `invalid_enum`。
  - 並び: 種族は `dex_no, form`、技・持ち物は `name_ja, id`(照合順序の五十音順)。一致なしは 200 `[]`(null にしない)。
  - 応答: `SpeciesSummary.types` は `[type1]` か `[type1, type2]`。`Move.priority` は常に出す。
- `getSpecies`: キーの形式(`^[0-9]{4}-[0-9]{3}$`)が違えば DB を呼ばずに 400 `invalid_input`。無ければ 404 `not_found`。
  **使用可能集合の外の種族も返す**(詳細はマスタの参照。一覧に出すかどうかは検索の仕事)。`abilities` は slot 順の `{id, nameJa}`、
  `learnset` は習得技 ∩ 既定のレギュレーションの使用可能な技(ID 昇順。`ListSpeciesLearnset`)。
- `getMove`(2026-09-23 追記。判定レーン JD4 の依頼。DECISIONS.md 2026-09-22・2026-09-23): 技1件を ID で引く。
  `getSpecies` と同じく既定のレギュレーションで絞らない(**使用可能集合の外の技も返す**。絞り込みは検索の仕事)。
  `GetDefaultRegulation` を経由しないため、`searchMoves`/`listNatures` と異なり **マスタが未投入(技が0件)でも
  503 ではなく 404 `not_found`** になる(単一行取得は「レギュレーションが無い」と「この ID の技が無い」を区別しない)。
  DB 自体に届かない失敗は 503 `master_unavailable`。`docs/adr/0107-move-secondary-rank-changes.md` §未決事項の
  「技 ID 1件を引く経路が無い」を解消する形として `GET /api/pokedex/moves/{key}` を採った(案:検索経由ではなく詳細
  エンドポイントを新設)。
- `getMovesByIds`(2026-09-24 追記。ADR-0304 §3 の決定): `GET /api/pokedex/moves/batch?ids=...`。`getMove` の複数版。
  `getSpecies` の `learnset`(ID配列)を1回の呼び出しで `Move[]` に解決するための経路(**`docs/adr/0304-web-online-mastersource.md`
  §3 の「学習技を技の実体にする」欠落は、`learnset` の型を変える案A ではなくこちらで解消した**。理由: iOS(M3)が
  `SpeciesDetail.learnset` を `string[]` のまま前提にした機能を先に出荷済みで、案Aは iOS の完成済み機能を壊す
  破壊的変更になり「契約変更が小さい方」の基準に反するため)。`getMove` と同様に既定のレギュレーションで絞らず、
  見つからなかった ID は黙って省き、応答は `ids` の順(DB の `IN` 句は順序を保証しないためハンドラで並べ替える)。
  `ids` は1〜64件。64 は issue #110 の教訓(候補・観測配列には必ず上限を置く。ADR-0208)を踏まえた保守的な
  初期値で、ADR-0208 の itemVariants/itemCandidates とは「1回のクエリで増幅させない」という考え方だけを
  借りている(持ち物の分類数が根拠のADR-0208の64をそのまま転用したものではない)。**2026-09-24 追記(実測)**:
  実クラスタで確認したところ、既定のレギュレーション(M-C)で349種族中151種族(43%)が64件を超え、最大106件
  (図鑑番号0475)だった。**64件を超える learnset はまれな例外ではない**ため、呼び出し側〈Web〉が64件ずつ
  分割して複数回呼ぶ設計にした(ADR-0304 §3 参照。1回で必ず収まる前提は置いていない)。
  件数の上限(`maxItems`)は生成ラッパが配列のスキーマを検証しないため、`services/pokedex/internal/httpapi/search.go`
  で自前に検査する(ADR-0208 の前例。DB を呼ぶ前に 400 `invalid_input`)。`ids` パラメータ自体の欠落
  (`minItems`/`required`)は生成ラッパが先に 400 にする(ハンドラの `len(ids)==0` の検査は HTTP 経由では
  通常到達しない防御的な二重検査)。マスタ未投入(技0件)は `searchMoves` と異なり 503 ではなく 200 `[]`
  になる(`GetDefaultRegulation` を経由しないため。`getMove` の404とも異なる。全件が「見つからない」の
  延長として扱われる)。
  ルーティング: echo v5.3.1 のルーターは静的セグメントをパラメータより優先するため、`/api/pokedex/moves/batch`
  は `/api/pokedex/moves/:key`(`key="batch"`)に食われない。`TestGetMovesByIds` が固定しているのは
  「食われないこと」自体(現在の登録順で)。登録順を入れ替えても同じ結果になることは調査時に使い捨てテストで
  確認しただけで、恒久テストには含まれない。
- `listNatures`: natures の全件(ID 昇順)。0行なら 503 `master_unavailable`。
- DB の失敗はすべて 503 `master_unavailable`。入力の検証(400)は DB を呼ぶ前に行う。

### 4. 性格のマスタ natures(ADR-0100 §3 の「性格はマスタにしない」を改める)

- 理由: calc-svc が MasterExport の `natures` で性格を受け取る契約になった(ADR-0204)。性格の日本語名は PokeAPI+override の
  データで、補正も取得元のデータとして持てば、engine・calc-svc・Web が同じ一覧を見られる。25 件という件数はコードにも CHECK にも書かない。
- migration `000006_create_natures`(取り込み済みの 000001〜000005 は書き換えない):
  `id`(ascii_bin・`^[a-z0-9]+$`)、`name_ja`(utf8mb4_ja_0900_as_cs・空でない)、`name_ja_source`(pokeapi/override/fallback_en)、
  `name_en`、`plus` / `minus`(NULL か `atk/def/spa/spd/spe`。HP は置けない)。CHECK: 無補正は両方 NULL、補正ありは両方あり・`plus <> minus`。
  UNIQUE(plus, minus): 同じ補正の組は1つ(calc-svc の `NatureID` が補正の構造値から ID を引くため。無補正の NULL は対象外)。
  依頼の `(id, name_ja, plus, minus)` に `name_ja_source`・`name_en` を足したのは、他の PokeAPI 由来のテーブルと同じく
  英語名フォールバックの検出と報告に使うため。
- importer(取得 → スナップショット → 変換 → 投入。ADR-0101 の流れに1種類足す):
  - 取得(Node。implementer): Showdown の `data/natures.ts` → `ShowdownSnapshot.natures: [{id, name, plus?, minus?}]`(無補正は plus/minus なし)。
    calc の `NATURES`(`{Name: [plus, minus]}`。無補正は同じ能力が2つ)→ `CalcSnapshot.natures: [{name, plus, minus}]`。
    PokeAPI の `natures.csv` + `nature_names.csv` → `PokeAPISnapshot.natures: [{slug, names}]`。override は `NameOverrides.natures`。
  - 変換 `Convert`: **補正の正は Showdown**。まず Showdown の natures を検証し、不正は `ErrInvalidData`(空・ID の形式・ID の重複・
    plus/minus が HP・未知・片方だけ・同じ・補正の組の重複)。次に calc と突き合わせ、補正の違い・片方にだけある性格は
    Blocker(`KindNatureMismatch = "nature-mismatch"`、ID は Showdown の ID / calc の名前の `toID`)で `ErrBlocked`。
    日本語名は他のテーブルと同じ `resolveJaName`(override > PokeAPI の nameJaLanguages 順 > 英語名)で、fallback と使われない override は Warnings。
  - 投入 `Apply`: 同じトランザクションで `DeleteNatures` → `InsertNature`(ID 順)。
  - 古いスナップショット(natures キーなし)は `Convert` が `ErrInvalidData`(「make import-fetch で取り直す」旨)。取り直したスナップショットは
    checksum が変わるので、CronJob の「版が同じなら取り込まない」(ADR-0104)でも自然に再投入される。
- `/api/pokedex/natures`(§3)と内部 API(§2)の両方がこのテーブルを読む。

### 5. `pokedex export`(balance・speed 向けの read model。ADR-0100 §8)

- 受け渡し: **ファイル**。`pokedex export -out <dir>` が DB を読んで4ファイルを書き、balance・speed はそれを起動時に読む
  (ADR-0012 §6: 実行時のサービス間依存を足さない。balance・speed は pokedex-svc にも DB にも接続しない)。
  ローカルは `make pokedex-export`(出力先 `data/generated/readmodel/`。Git 管理外。ADR-0002)。k8s への載せ方(ConfigMap 化・マウント)は
  読む側のレーンの仕事(balance の SP/TB 整備、speed の SP4)で、ファイル名と形をこの ADR で固定する。
- ファイル(`readmodel.File*` 定数)と形:

| ファイル | 読む側の設定 | 形 |
|---|---|---|
| `pokemon-types.json` | `BALANCE_POKEMON_TYPES_PATH` | `services/balance/schema/pokemon-types.schema.json`(ADR-0014 §2 + ADR-0401 §5)。`{schemaVersion:1, pokemon:[{pokemonId, nameJa, types, abilityIds}]}` |
| `moves.json` | `BALANCE_MOVES_PATH` | `moves.schema.json`(ADR-0016 §3)。`{schemaVersion:1, moves:[{moveId, type, category}]}` |
| `abilities.json` | `BALANCE_ABILITIES_PATH` | `abilities.schema.json`(ADR-0017 §2)。`{schemaVersion:1, abilities:[{abilityId, effects}]}` |
| `speed-pokemon.json` | `SPEED_POKEMON_PATH` | ADR-0600 §4。`{schemaVersion:1, regulationId, pokemon:[{pokemonId, nameJa, types, baseSpeed}]}` |

- 規則:
  - 対象は**既定のレギュレーションの使用可能集合**(`regulation_species` / `regulation_moves` / `regulation_abilities`)。無ければ
    `ErrNoDefaultRegulation`。どれかが0件なら `ErrInvalidExport`(schema の minItems 1。balance・speed の loader も拒否する)。
  - 並びは ID 昇順(pokemonId / moveId / abilityId)。DB の行の順によらずバイト単位で決定的。JSON は空白なしの正準形(`json.Marshal`)+ 末尾の改行。
    数値は整数の字面だけ(`2`。`2.0` は不可。ADR-0402 §3)。
  - `types` は `[type1]` か `[type1, type2]`。`nameJa` は species.name_ja(DB で 1〜64 文字)。
  - `abilityIds` は slot 昇順(隠れ特性 slot 3 を含む)。balance の上限は3件(schema の maxItems・loader の `maxCatalogAbilityCount`)なので、
    **4件ある種族は slot 4(Showdown の特殊枠 "S"。ADR-0103 §12)を落とし、`Report.TruncatedAbilities` に `{pokemonId, abilityId}` を残す**
    (CLI は件数と ID を標準エラーに出す)。上限の引き上げはタイプバランスレーンへの依頼(末尾)。
  - 特性の効果: `ability_effects` を共通マスタの `DecodeAbilityEffect`(相性表は `types`/`type_chart` から `master.TypeChart`)で厳格に読み、
    防御側のタイプ相性に関わるものだけを正規化する:
    `DefResistType{t: m}` → `{kind:"type_multiplier", attackType:t, numerator, denominator}`(m/4096 を約分。攻撃タイプ ID 昇順)、
    `ReduceSuperEffective m` → `{kind:"super_effective_multiplier", numerator, denominator}`(同)。約分後に分子・分母が 1〜16 に収まらなければ
    `ErrInvalidExport`(近似しない)。それ以外のフィールド(StabMod・OffBoost*・IgnoresBurn)は攻撃側の効果なので出さない。
    効果の行が無い・防御側の効果が無い特性は `effects: []`(null にしない)。デコードに失敗した行は `ErrInvalidExport`。
  - `immune` / `absorb`(ふゆう・ちょすい等)は engine の `AbilityEffect` に無く DB にも無いので、v1 の export には出ない(限界。末尾の確認事項)。
  - 失敗時は `Files` のゼロ値を返す(部分的な出力をしない)。DB のエラーは `%w` で包む。
- `Files.WriteDir(dir)`: 無ければ作る。各ファイルは同じディレクトリの一時ファイルに書いてから rename(読む側が書きかけを読まない)。
  一時ファイルを残さない。`dir` がファイルならエラー。
- タイプ相性表は出さない(balance は `testdata/golden/typechart.json` を埋め込む。ADR-0015)。

### 6. k8s・イメージ

- `deploy/k8s/base/pokedex/`: `deployment.yaml`(Deployment `pokedex`、image `pokecalc/pokedex:0.1.0`・IfNotPresent、args `[serve]`、
  port `http` 8080、`POKEDEX_DATABASE_DSN` は `secretKeyRef mysql-auth / pokedex-dsn`、probe は両方 `/healthz`、resources、
  automountServiceAccountToken false、非 root・readOnlyRootFilesystem・drop ALL・seccomp RuntimeDefault)と `service.yaml`
  (Service `pokedex`、ClusterIP、`http` 80 → `http`)。kustomization の resources に足す(既存の Job・CronJob・PVC は残す)。
- **Ingress を作らない**。公開の `/api/pokedex/*` は gateway 経由(ADR-0202)、内部 API は gateway でも 404。calc-svc は `http://pokedex` を使う。
  local overlay の calc を URL 方式に切り替えるのは API レーン(ADR-0204 §4・§5)。
- `services/pokedex/Dockerfile` に `server` ターゲット(`go build -o /out/pokedex ./pokedex/cmd/pokedex`、`FROM scratch AS server`、
  `USER 65532:65532`、`ENTRYPOINT ["/pokedex"]`、`CMD ["serve"]`)。`scripts/up.sh` が build して k3d に import する(migrate と同じ流儀)。
- MySQL の DSN・パスワードを manifest に書かない(Secret は up.sh が作る。ADR-0100 §9)。

### 7. テスト用の架空データ

`internal/storetest.New()`: ADR-0100 §7 の規則(9001 以降・`test` 始まりの ID・`テスト` 始まりの日本語名。タイプ ID だけは一般名)。
既定のレギュレーション `test-a` の外に種族・技・持ち物・特性を1つずつ置き(絞らない/絞るの両方を確かめるため)、slot 1〜4 の4特性を持つ種族を1つ置く。
importer の `testdata/fictional` にも natures(4件。無補正1件・fallback 1件・override 1件)を足した。

## 受け入れ条件と担当テスト

| AC | 内容 | テスト |
|---|---|---|
| AC-I1〜I3 | 内部 API: ヘッダ無しで 200・MasterExport どおり。DB の全行(絞らない)・showdownId・slot 順・dataVersion。effect はそのまま(null のキー・数値の字面) | `httpapi.TestMasterExportMatchesContract` / `TestMasterExportContent` / `TestMasterExportEffectIsVerbatim` |
| AC-I4 | 未投入(data_versions / types / natures が空)・DB の失敗・効果の JSON の不正は 503 master_unavailable(契約どおり・内部情報なし) | `TestMasterExportUnavailable` |
| AC-I5〜I8 | ヘッダの有無で変わらない・GET 以外 404。/healthz は DB に触れない。calc の操作・未知のパスは 404 not_found。panic は 500 internal | `TestMasterExportHeadersAndMethods` / `TestHealthzDoesNotTouchDB` / `TestCalcRoutesAndUnknownPathsAreNotFound` / `TestPanicIsInternalError` |
| AC-P1〜P5 | 公開 API: 契約どおり。パターンのエスケープ・limit の既定・既定のレギュレーション。応答の中身・[]。詳細(集合の外も引ける・learnset は集合で絞る・404)。性格の一覧 | `TestPublicEndpointsMatchContract` / `TestSearchPassesPatternLimitAndRegulation` / `TestSearchResponses` / `TestGetSpecies` / `TestListNatures` |
| AC-P4b | `getMove`(2026-09-23 追記): 集合の外の技も引ける・未知の ID は404・未投入(0件)も404(searchMoves と違い503にならない) | `TestGetMove` |
| AC-P4c | `getMovesByIds`(2026-09-24 追記): 応答は `ids` の順・未知の ID は省く・集合の外の技も引ける・`ids` 1〜64件の範囲外は400・`/moves/batch` が `/moves/:key` に食われない | `TestGetMovesByIds` |
| AC-P6〜P7 | 入力の検証(missing_header / invalid_input / invalid_enum。DB を呼ばない)。未投入・DB の失敗は 503 | `TestPublicInputValidation` / `TestPublicUnavailable` |
| AC-P2(DB) | 検索クエリの前方一致・ひらがな/カタカナの同一視・集合での絞り込み・LIMIT | `db.TestSearchSpeciesPrefixAndRegulation`(-tags mysql) |
| AC-N1〜N3 | natures の変換(Showdown が正・名前の優先順・Warnings)。calc との不一致は Blocker。不正は ErrInvalidData。件数を仮定しない | `importer.TestConvertNatures` / `TestConvertNaturesNameFindings` / `TestConvertNaturesNameLanguageOrder` / `TestConvertBlocksOnNatureMismatch` / `TestConvertRejectsInvalidNatures` / `TestConvertNaturesDoNotAssumeCount` |
| AC-N4 | migration 000006 の形と制約(静的)・クエリの宣言。DB の制約が効く | `db.TestNaturesMigrationIsNewPair` / `TestNaturesMigrationDeclaresConstraints` / `TestQueriesDeclareP23Reads`、`TestNaturesAcceptValidRows` / `TestNaturesRejectInvalidRows`(-tags mysql) |
| AC-E1〜E2 | export が balance の JSON Schema に合う(空振りしない)。数値は整数の字面 | `readmodel.TestExportSatisfiesBalanceSchemas` / `TestBalanceSchemaCheckIsNotVacuous` / `TestExportWritesPlainIntegers` |
| AC-E3〜E6 | 集合で絞る・並び・abilityIds(slot 4 を落として報告)・技・特性の正規化・speed の形 | `TestExportPokemonTypes` / `TestExportMoves` / `TestExportAbilities` / `TestExportSpeedPokemon` |
| AC-E7〜E10 | 決定的・失敗時はゼロ値・WriteDir・ファイル名 | `TestExportIsDeterministic` / `TestExportFailures` / `TestWriteDir` / `TestFileNames` |
| AC-K0 | 設定(DSN 必須・parseTime・既定のアドレス・エラーに DSN を出さない) | `cmd/pokedex.TestEnvNames` / `TestLoadConfig` |
| AC-K1〜K5 | Deployment(Secret の DSN・probe・非 root)・Service はクラスタ内だけ・どの Ingress も pokedex / /internal を指さない・DSN の平文なし・kustomization・Dockerfile の server・up.sh | `TestManifestPokedexDeployment` / `TestManifestPokedexServiceIsInternal` / `TestPokedexIsNotExposed` / `TestBaseKustomizationListsServiceResources` / `TestServerImageAndUpScript` |

## Go の API(テストが前提にする名前)

```go
// package example.com/pokecalc/services/pokedex/internal/httpapi
func NewHandler(q store.Querier) http.Handler

// package example.com/pokecalc/services/pokedex/internal/readmodel
const FilePokemonTypes, FileMoves, FileAbilities, FileSpeedPokemon = "pokemon-types.json", "moves.json", "abilities.json", "speed-pokemon.json"
var ErrNoDefaultRegulation, ErrInvalidExport error
type Files struct{ PokemonTypes, Moves, Abilities, SpeedPokemon []byte }
type TruncatedAbility struct{ PokemonID, AbilityID string }
type Report struct{ TruncatedAbilities []TruncatedAbility }
func Export(ctx context.Context, q store.Querier) (Files, Report, error)
func (f Files) WriteDir(dir string) error

// package main(services/pokedex/cmd/pokedex)
const envAddr, envDatabaseDSN, defaultAddr = "POKEDEX_ADDR", "POKEDEX_DATABASE_DSN", ":8080"
type config struct{ Addr, DSN string } // DSN は parseTime=true を付けた後の値
func loadConfig(lookup func(string) (string, bool)) (config, error)

// package importer(追加)
type NatureRow struct{ ID, NameJa, NameJaSource, NameEn, Plus, Minus string } // Plus/Minus "" = NULL
// Output.Natures []NatureRow(ID 順)
type CalcNature struct{ Name, Plus, Minus string }            // CalcSnapshot.Natures(json "natures")
type ShowdownNature struct{ ID, Name, Plus, Minus string }    // ShowdownSnapshot.Natures(plus/minus は省略可)
// PokeAPISnapshot.Natures []PokeAPIName、NameOverrides.Natures map[string]string
const KindNatureMismatch FindingKind = "nature-mismatch"
```

## 却下した案

- **balance の pokemon-types に baseStats を足して speed と共有する**: balance の schema と loader が未知のフィールドを拒否する
  (additionalProperties false・DisallowUnknownFields)。speed の既存の形(ADR-0600 §4)で別ファイルを出せば、speed はファイルの差し替えだけで
  切り替えられ、どちらのレーンのコードも変えずに済む。
- **balance・speed が pokedex-svc の API を実行時に呼ぶ**: ADR-0012 §6。calc-svc の例外(ADR-0204)はユーザー決定による calc だけのもの。
- **公開 API をレギュレーションで絞らない**: 画面のピッカーに使えないポケモン・技が混ざる。詳細(getSpecies)だけは絞らない。
- **内部 API で効果を `map[string]any` に float64 で読む**: 数値の字面が変わる(§2)。
- **natures を migration の INSERT で入れる**: ADR-0100 §2(migrations に実データを書かない)。
- **natures の補正を calc を正にする**: calc は無補正を「同じ能力の上昇と下降」で表し、ID を持たない。Showdown は ID と省略形を持つ。
- **abilityIds が4件の種族で export を失敗させる**: 実データで起きる(ADR-0103 §12)ので、balance 全体が止まる。落として報告する方が影響が小さい。

## 限界・人間の確認事項(既定案で進める)

1. **特性の immune / absorb が export に出ない**(ふゆう・ちょすい等)。engine の効果定義に無いため DB にも無い。既定案: 後続タスク(P2-3b)で
   `data/importer/effects.json` に balance 用の防御の効果(`TypeImmunity` 等)を足すか、別の定義ファイルとテーブルを足す。TB3/TB5 は
   その間、タイプ由来と倍率変更の特性だけで判断する。
2. **abilityIds の slot 4 を落とす**(§5)。既定案: タイプバランスレーンに上限を 4 に上げてもらい、上がったら落とすのをやめる。
3. **公開 API の `format` は v1 では無視**(§3)。
4. 000006 の適用後、DB の natures は次の取り込みまで空で、その間 `/internal/pokedex/master` と `/api/pokedex/natures` は 503。
   既定案: 000006 の適用と同時に `make import-fetch && make import` を流す(手順は implementer が runbook の担当に伝える)。
5. 内部 API・export の読み出しは1トランザクションでない(§2)。既定案: v1 はこのまま。混ざりが問題になったら、`store.New(tx)` を渡す
   読み取りトランザクションの入口(`func(ctx, func(store.Querier) error) error`)を httpapi / readmodel に足す。

## 他レーンへの依頼(DECISIONS.md に既定案付きで書く。メインが行う)

- **API レーン**(api/openapi.yaml の description だけ。形は変えない): (a) `/api/pokedex/*` の 503 の description に「pokedex-svc 自身が
  `master_unavailable`(未投入・DB に接続できない)を返すこともある」を足す、(b) `searchSpecies` / `searchMoves` / `searchItems` の description に
  「既定のレギュレーションの使用可能集合だけ」「並び順」、`format` に「v1 では結果に影響しない」、`getSpecies` に「使用可能集合の外の種族も返す。
  learnset は使用可能な技だけ」「404 は not_found」、`limit` の範囲外は 400 `invalid_input` を足す、(c) `MasterSpeciesAbility.slot` の description
  「1..3」を「1..4(4 = 特殊枠。ADR-0103 §12)」に直す。
- **タイプバランスレーン**: `abilityIds` の上限(schema の maxItems・loader の `maxCatalogAbilityCount`)を 4 に上げる。k8s で export の4ファイルの
  うち3つを ConfigMap 等でマウントする(置き方は balance の判断)。
- **素早さレーン**(SP4): `speed-pokemon.json` を `SPEED_POKEMON_PATH` で読む(形は ADR-0600 §4 のまま)。
