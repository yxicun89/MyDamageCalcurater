# ADR-0324: メガ種族の日本語名の生成と、名前検索の規則(issue #515)

- 状態: 採用
- 日付: 2026-10-03
- レーン: データ(importer・pokedex・Web の検索)。iOS の入力 UI は iOS レーン
- 関連: issue #515、docs/mega-evolution-spec.md §4-1・§4-2、ADR-0002 決定 10、ADR-0101 §6(日本語名の解決)、ADR-0105 §3(検索 API)、
  ADR-0320(Web のメガ持ち物固定)、ADR-0321(wasmapi のメガ持ち物検証)

## 背景

取得元(PokeAPI)の日本語名は、新しいメガフォームで空のことがある(例: `lucario-mega` の `names` が `ja-Hrkt: ""`・`ja: ""`)。
importer は上書き → PokeAPI → 英語名の順で `nameJa` を決めるので、空のメガは英語名(`Lucario-Mega`)になり、
「メガルカリオ」と入力しても検索に当たらない。さらに検索は `name_ja LIKE 'q%'`(前方一致)なので、仮に名前があっても
`ルカリオ` と入力したときにメガルカリオは出ない。

## 決定

### 1. メガ種族の `nameJa` の生成(importer)

- 優先順位: 上書き設定 > PokeAPI(`nameJaLanguages` の順。空白だけは採らない)> **生成** > 英語名。
  上流に日本語名があれば生成しない。
- 生成: `メガ` + 基本種の日本語名 + フォーム識別子。識別子は Showdown のフォーム名から導く(`Mega` → なし、`Mega-X` → `X`、`Mega-Y` → `Y`、`Mega-Z` → `Z`。
  `services/internal/master` の `MegaNameJa`)。例: メガルカリオ、メガリザードンX。名前の表は持たない(基本種の名前とフォーム名だけから導く)。
- 基本種の日本語名が無い(基本種も英語名フォールバック)ときは生成しない。メガは英語名のまま `fallback_en` として報告する
  (英語名を混ぜた「メガLucario」のような名前を作らない)。
- 実装は、メガ以外の種族を先に解決してから、メガを解決する(`convert_species.go`)。
- `species.name_ja_source` に値 `generated` を足す(migration 000011。species だけ。他の表は生成しない)。
  down は `generated` の行を `fallback_en` に寄せてから CHECK を戻す。
- 報告: 生成した種族ごとに警告 `name-generated`(`KindNameGenerated`。止めない)を出し、reconcile の `names.species` に `generated`(件数)と
  `generatedIds` を足す。英語名のままのものは従来どおり `name-fallback`・`fallbackIds`。
- 既存 DB は、変換結果の版(ADR-0122)が変わるので、次の取り込みで全行が入れ直される(`nameJa` が変わる)。

### 2. メガストーン(持ち物)の日本語名: 生成しない(既知の制約)

メガストーンの名前は基本種の名前から機械的には作れない(例: リザードン → リザードナイトX は基本種名の末尾が落ちる。基本種名をそのまま使う石もある)。
規則が揃わないので、種族のような生成は入れない。欠落は次の既存の仕組みで扱う。

- PokeAPI に名前があるものはそれを使う。
- 欠落は英語名のまま `name-fallback` の警告と `names.items.fallbackIds` に出る(件数・ID が report で分かる)。
- 日本語名は `data/local/name_ja_overrides.json` の `items` で補う(取り込みのたびに上書きとして使われ、未使用なら `override-unused` で分かる)。

メガ種族を選ぶと持ち物はメガストーンに自動で固定される(ADR-0320)ので、ストーンの名前が英語のままでも入力の操作は成立する(表示だけの問題)。

### 3. 種族の名前検索の規則(全経路で同じ)

種族 S が検索語 q に当たるのは、次のどちらかのとき。

1. `S.nameJa` が `q` で始まる(前方一致。既存どおり)
2. `S` がメガ種族(`isMega`)で、`S.nameJa` が `メガ` + `q` で始まる

| q | 結果 |
|---|---|
| `メガルカリオ` | メガルカリオ(1 の規則) |
| `メガ` | 全メガ種族(1 の規則。レギュレーションの使用可能集合の中だけ) |
| `ルカリオ` | ルカリオ(基本種)とメガルカリオの両方(基本種は 1、メガは 2) |
| `リザードン` | リザードン、メガリザードンX、メガリザードンY |
| `リザードンX` | メガリザードンX だけ |

- 並びは図鑑番号・フォーム番号の昇順(基本種のフォーム 0 が先)。件数は `limit`(既定 50・最大 200)。空の q は全件。
- 使えないメガ(レギュレーションの使用可能集合の外)は、従来どおり検索に出ない。
- 接頭辞 `メガ` は命名規則の 1 語であり、名前の表ではない(CLAUDE.md のハードコード禁止の対象外)。importer の生成(`master.MegaNamePrefix`)・
  pokedex の検索・Web のオフライン検索が同じ語を使う。上流が別の接頭辞の名前を持つメガは、規則 2 では当たらず(自分の名前の前方一致だけ)、害は小さい。
- 実装の場所(すべて同じ規則):
  - **オンライン(pokedex-svc)**: `SearchSpecies` の SQL が `name_ja LIKE pattern OR (is_mega AND name_ja LIKE mega_pattern)`。
    パターンの組は `httpapi.SpeciesSearchPatterns(q)`(`pattern` = q の前方一致、`mega_pattern` = `メガ` + q の前方一致。LIKE の特殊文字はエスケープ)。
    SQL に名前は書かない。
  - **オフライン(Web のキャッシュ)**: `web/src/master/speciesNameMatch.ts` の `searchSpeciesByName`(`MEGA_NAME_PREFIX`)。
    `createCachedOfflineMasterSource` の `searchSpecies` が使う。オフラインはキャッシュ済み(解決済み)の種族だけが対象という既存の制約は変わらない。
  - **テストの偽物**: Web の `test/onlineMaster.ts` の fake と、E2E の pokedex フィクスチャ(`e2e/support/pokedexFixture.ts`)も同じ規則を使う。
  - WASM(`engine/wasmapi`)・計算・逆算に種族検索は無い(種族を引くのは Web のマスタ側)ので変更なし。
  - Web の `exampleSource`(例データの全件)は検索を持たず、全種族を選択肢にするので変更なし。
- API 契約(`api/openapi.yaml`)は変えない(`q` の意味が広がるだけで、応答の形は同じ)。
- 照合順序(`utf8mb4_ja_0900_as_cs`)はひらがなとカタカナを区別しない既存の挙動のまま。Web のオフライン検索は完全一致の前方一致(既存と同じ。この差は今回扱わない)。

## 検証

- importer: テーブル駆動(`convert_mega_name_test.go`)。上流に名前が無い(メガ・X・Y・Z)・上流に名前がある・上書きがある・基本種名が無い・メガ以外は生成しない。
- migration: 静的な確認(`species_name_source_layout_test.go`)と、実 MySQL(`make test-db-docker`)。
- pokedex の検索: パターンの組(`TestSearchSpeciesPassesMegaPattern`)と、実 MySQL の結果(`species_search_mysql_test.go`: メガの名前・`メガ`・基本種名・X)。
- Web: 規則の単体(`speciesNameMatch.test.ts`)、キャッシュのオフライン検索(`cachedSources.mega.test.ts`)、
  検索から選んだメガ種族の持ち物固定との結合(`CalcScreen.mega.test.tsx`。ADR-0320 の既存の固定の回帰)。

## iOS への申し送り(iOS レーン)

- 検索は pokedex-svc の `GET /api/pokedex/species?q=` を呼ぶだけなので、上の規則は iOS にもそのまま届く(クライアントでの再実装は不要)。
  iOS が種族をローカルで絞り込む箇所があれば、同じ規則(§3)にそろえること。
- `SpeciesSummary` には `isMega` が無い(`SpeciesDetail` で解決後に分かる)。持ち物の自動固定は解決後に行う(ADR-0320 の Web と同じ流れ)。
- メガ種族の名前は pokedex が生成する(クライアントで組み立てない)。入力 UI の自動固定・UI テストは iOS レーンの issue #515 の残り。

## 影響

- 次の取り込みで、上流に日本語名が無いメガ種族の `nameJa` が英語名から生成した日本語名に変わる(`name_ja_source` = `generated`)。
  実データの件数は import report の `names.species.generated` で見る。
- CLAUDE.md の「持ち物・技・ポケモンのリストをハードコードしない」には反しない(接頭辞 1 語の命名規則と、フォーム名から導く識別子だけ)。
