# ADR-0140: メガストーンの判定を取得元の持ち物データからも導き、日本語名の無いものは推測せず上書きで補う(issue #607)

- 状態: 採用
- 日付: 2026-10-04
- レーン: データ(importer・pokedex)。表示(「{基本種名}のメガストーン」へのフォールバック)は Web・iOS レーン
- 関連: issue #607、ADR-0175(持ち物の役割とメガストーンの判定。§2 を本 ADR で改める)、ADR-0324(メガ種族の日本語名の生成。範囲を本 ADR で明確化)、
  ADR-0101 §6(日本語名の解決)・ADR-0103 §8(names の報告)・ADR-0104 §7(上書きの ConfigMap)、ADR-0121・ADR-0136(古いスナップショットの拒否)、
  ADR-0124(適用済み migration を書き換えない)、docs/mega-evolution-spec.md

## 背景(実データの測定。2026-10-04・Showdown champions mod・PokeAPI の固定版)

ユーザー要望「メガ種族を選んだら、持ち物にストーンの正式名称が出てほしい」(表示は Web・iOS が持ち物の `nameJa` を出し、日本語の文字が無い
ときだけ「{基本種名}のメガストーン」にする)。その前提のマスタを整える。取り込みのドライランで測った件数(名前はコミットしない):

- 持ち物 166 件のうち、日本語名が上流に無い(`name_ja_source = fallback_en`)ものが 40 件。うち 39 件は Showdown の持ち物データで
  `megaStone` を持つメガストーン、1 件は通常の持ち物。
- Showdown の使用可能な持ち物のうち `megaStone` を持つものは 81 件。いまの判定(`species.is_mega = 1 AND required_item_id = 持ち物`)では
  そのうち 1 件が漏れる。原因はレギュレーション外ではなく、**importer のメガの判定がフォーム名の前方一致(`Mega` で始まる)だったこと**。
  Showdown にはフォーム名が `M-Mega`・`F-Mega` のメガがあり(Showdown 自身は `forme.includes('Mega')` でメガとする。`sim/dex-species.ts`)、
  importer はこれを通常の姿として取り込み、`is_mega`・`required_item_id` を持たせていなかった(そのため種族も英語名のまま・持ち物の固定もされない)。
- 特性 3 件・種族の姿 39 件が英語名のまま。姿の内訳は、地方の姿(Hisui 7・Alola 3・Galar 3・Paldea-* 3)、性別・天気・時間帯などの姿、
  上記のメガ 1 件。PokeAPI の固定版では、地方の姿(alola 19・galar 20・hisui 16・paldea 4 件)に日本語名を持つものが **1 件も無い**。

## 決定

### 1. メガの判定を Showdown と同じにする(importer)

種族がメガ ⇔ Showdown のフォーム名に `Mega` を含む(Showdown の `isMega` と同じ式)。`Mega`・`Mega-X` に加えて `M-Mega`・`F-Mega` もメガとして
取り込み、`is_mega`・`base_species_key`・`required_item_id` を持たせる。種族の key は変わらない(フォーム番号は formeOrder から決まる)。

### 2. メガストーンの判定を取得元の持ち物データからも導く(importer・pokedex)

- 取得: `tools/importer/fetch-showdown.mjs` は持ち物ごとに `megaStone`(基本種名 → メガ種族名。Showdown の `Item.megaStone`)を出す。
  ストーンでない持ち物も `{}` を出し、キーを必ず持たせる。importer はキーの無い古いスナップショットを `ErrInvalidInput` で拒否し、
  取り直し(`make import-fetch`)を案内する(ADR-0121・0136 と同じ。古い形を黙って「ストーンでない」と読まない)。
- 変換: 持ち物の行に `IsMegaStone` を持たせる。真 ⇔ Showdown の `megaStone` が空でない **または** 取り込んだメガ種族の `required_item_id` に現れる。
  対応するメガ種族が取り込まれない(レギュレーション外の)ストーンも真(ADR-0175 §2「使用可能集合で絞らない」と同じ考え)。
- 食い違い(メガ種族が要求するのに `megaStone` が空)は判定を真のままにし、警告 `item-mega-stone-mismatch`(ID は持ち物)を出す。取り込みは止めない。
  逆向き(`megaStone` があるのに要求するメガが取り込まれない)はレギュレーション外のメガとして正常なので警告しない。
- DB: migration 000012 で `items.is_mega_stone TINYINT(1) NOT NULL DEFAULT 0` を足す(既存の migration は書き換えない。ADR-0124)。
  行は importer が全置換で書く(migration にデータを書かない)。
- 判定の場所: `SearchItems`(`services/pokedex/db/query/pokedex.sql`)の `is_mega_stone` を `i.is_mega_stone OR EXISTS (メガ種族が要求する)` の1つの式にする。
  EXISTS を残すのは、列が既定値のままの行(migrate 直後で未投入・手で入れた行)でも従来の判定を保つため。公開 API(`Item.isMegaStone`)・役割の規則
  (ADR-0175 §1: ストーンは役割が空)・契約は変えない。

### 3. 日本語名が上流に無いものは推測しない(持ち物・特性・種族の姿)

- 持ち物・特性: 名前を生成しない。英語名のまま(`fallback_en`)取り込み、reconcile の `names.<種類>.fallbackIds` に出す(従来の仕組み。ADR-0103 §8)。
  日本語にしたいものは上書き(`data/local/name_ja_overrides.json`。Git に入れない。k3d は ConfigMap `pokedex-name-overrides`。ADR-0104 §7)で補う。
  手順は `docs/runbooks/data.md` §3a。
- メガ種族の名前の生成(ADR-0324 §1)は、フォーム名が `Mega` または `Mega-<識別子>` のときだけに限る(`master.MegaNameJa` はそれ以外で "" を返す)。
  `M-Mega` のように識別子の位置が違うものは、上流で規則を検証できないので生成せず、英語名のまま報告して上書きで補う。
- 地方の姿(ヒスイ・アローラ・ガラル・パルデア)の「基本種名(○○のすがた)」の生成は**行わない(後続)**。上流に同じ接尾辞の姿の日本語名が 1 件も無く、
  規則(接尾辞の訳・括弧の表記・パルデアの3種の書き分け)を上流のデータで検証できないため。上流に名前が入るか、検証できる参照が得られたら、
  別の ADR で「同じ接尾辞の姿の上流の名前と一致することを確かめたうえで生成する」規則を検討する。それまでは上書きで補う。
  性別・天気・時間帯などの姿も同じく上書きで補う。

## 検証

- importer(架空データ): `services/pokedex/importer/convert_mega_stone_test.go`(`M-Mega`・`F-Mega` のメガ判定、`megaStone` からの判定・
  メガ種族の無いストーン・使用不可のストーン・食い違いの警告・古いスナップショットの拒否・名前を推測しない)、
  `convert_names_fallback_test.go`(持ち物・特性・地方の姿の fallbackIds と上書き)。
- 取得: `tools/importer/fetch-showdown-megastone.test.mjs`(`megaStone` を必ず出す)。
- 共通: `services/internal/master/mega_name_test.go`(`MegaNameJa` の範囲)。
- DB: `services/pokedex/db/item_mega_stone_layout_test.go`(migration 000012・SearchItems の式)、`item_mega_stone_column_mysql_test.go`(-tags mysql)、
  `services/pokedex/importer/item_mega_stone_mysql_test.go`(-tags mysql。importer が列を書く)。
- API: `services/pokedex/internal/httpapi/item_mega_stone_column_test.go`(列が真のストーンの `isMegaStone`・役割が空)。
- 手順書: `services/pokedex/db/runbook_name_overrides_test.go`。

## 影響

- 取得物の形が変わる: 既存の `data/generated/showdown/<commit>/snapshot.json` は取り直しが要る(CronJob は毎回取り直すので自動で直る)。
- 変換結果の版(importer-output)と DB の items が変わる。種族の key は変わらない。read model(balance・speed)の種族の key・形は変わらない。
- `M-Mega` のメガがメガとして扱われるので、Web・iOS ではメガ種族の選択でストーンが固定され、持ち物の選択肢から外れる(ADR-0175 §4 の既存の動作)。
- engine・ゴールデン・計算 API・`api/openapi.yaml` は変わらない。
