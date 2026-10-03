# ADR-0218: 公開 API の Item / Ability に効果定義(effect)を省略可で載せる(issue #211 の API レーン担当分)

- 状態: 採用(critic PASS。2026-10-02)
- 日付: 2026-10-02
- 関連: ADR-0105 §2・§3(pokedex-svc の内部 API・公開の検索 API)、ADR-0204(calc-svc が内部 API から取るマスタ一式。
  効果の形は共通マスタで厳格に検証する)、ADR-0005・ADR-0100 §6(効果定義の JSON。`services/internal/master`)、
  ADR-0127(内部 API の1スナップショット)、ADR-0208(calc の件数の上限)、ADR-0304 A-1(Web のオンライン MasterSource
  で効果が無いため持ち物候補を無効化している)、DECISIONS.md 2026-10-01「issue #211 の API 側の提案」

## 背景

Web のオンライン MasterSource(ADR-0304)は、公開 API の `Item`・`Ability` が `{id, nameJa}` だけで効果を持たないため、
計算画面の「持ち物の候補も比較」と逆算の持ち物候補を無効化している(`ONLINE_MASTER_CAPABILITIES.effects=false`)。
効果は internal-only の `getMasterExport`(ADR-0204)にしか無く、gateway はこれを公開しない(ADR-0210)。

## 決定

### 1. 契約: `Item.effect` / `Ability.effect` を省略可で足す

- 形は内部 API の `MasterEffect` を `allOf` で参照する(公開用に別の効果スキーマを作らない。二重定義しない)。
  値は `getMasterExport` の `MasterItem.effect` / `MasterAbility.effect` と同じ(item_effects / ability_effects の JSON
  そのまま。数値の字面も保つ。ADR-0105 §2)。
- `required` に入れない。効果を持たない持ち物・特性は**キーごと省く**(null を返さない)。内部 API の
  「キー必須・null 可」とは違うが、公開側は既存クライアントとの互換(足すだけ)を優先する。
- 対象の操作は Item / Ability を返す公開の操作すべて: `searchItems`(`Item[]`)と `getSpecies`
  (`SpeciesDetail.abilities`)。ほかに `Item` / `Ability` を返す公開の操作は無い(2026-10-02 時点で確認)。
- 「効果を持つ持ち物だけ」の検索条件は足さない(クライアントが `effect` の有無で絞る)。

### 2. pokedex-svc: 返す前に共通マスタで厳格に検証する

- 応答に載る行の効果を `services/internal/master` の `DecodeItemEffect` / `DecodeAbilityEffect` で検証する
  (未知のキー・大文字小文字違い・小数・0 以下・`engine.MaxEffectModifier` 超え・空のオブジェクト・相性表に無いタイプ)。
  相性表は同じ DB の `types` / `type_chart` から `master.TypeChart` で作る。pokedex 側に別の規則を書かない。
- 1件でも通らなければ、効果を応答に出さず 503 `master_unavailable`(内部 API の「壊れた JSON は 503」・calc の
  「不正なマスタは全体を拒否」と同じ fail-closed)。importer は投入前に同じ共通マスタで検証している
  (`services/pokedex/importer`)ので、ここで落ちるのは DB の破損か版のずれだけ。
- 検証するのは**その応答に載る行だけ**。使用可能集合の外・`q` に一致しない持ち物・別の種族の特性の不正で
  検索を止めない(全件の効果を毎回読まない)。
- **影響範囲(fail-closed)**: 載る行に1件でも不正があれば、`searchItems` は応答全体が 503(オンライン持ち物ピッカー
  全体が使えない)、`getSpecies` はその種族の詳細が 503。importer が投入前に同じ共通マスタで検証するので、起こるのは
  DB の破損か版のずれだけ。**デプロイの順序**: 共通マスタの検証規則(効果の新しいキーなど)を変える版は、
  importer より先に pokedex-svc を上げる(古い pokedex が新しい効果を不正として 503 にしないため)。
- 内部 API(`getMasterExport`)は従来どおり検証せずに運ぶ(検証は受け取った calc-svc。ADR-0204)。変えない。

### 3. DB の読み方

効果は既存の `item_effects` / `ability_effects`(行が無い = 効果なし。JSON 列は `JSON_TYPE = 'OBJECT'` の CHECK 付き)
から読む。新しいテーブル・カラム・マイグレーションは足さない。公開の検索クエリ(`SearchItems`・
`ListSpeciesAbilityNames`)に効果の列を LEFT JOIN で足す(または同等の sqlc クエリ)。
JSON 列は NULL になりうるので、sqlc の `overrides` で `*json.RawMessage` に受ける(nil = 行なし)。

トランザクション: `searchItems` と `getSpecies` は、一覧と効果の検証用の相性表(`ListTypes`・`ListTypeChart`)を
**1つの読み取り専用トランザクション(RepeatableRead)**で読む(ADR-0127 と同じ。importer の全置換と重なっても、
行と相性表が別の版にならない)。入力検証の後に開く(不正入力では DB を呼ばない)。読み終えたら Commit で閉じて
接続を早く返し、失敗時は Rollback で閉じる。相性表は効果を持つ行があるときだけ読む。

### 4. 互換・クライアント

- 既存クライアントは新しいキーを無視できる(Go の生成型・openapi-typescript・swift-openapi-generator とも
  省略可のプロパティ。iOS は `make ios-gen` の再生成だけで追従し、`swift build --build-tests` が通る)。
- 「キーが無い」は「効果なし」と「効果を返さない古いサーバー」を区別しない。Web は capabilities で判定する
  (Web レーンの追従。DECISIONS.md 2026-10-01)。
- 効果の JSON は DB の形(`DamageMod` などの PascalCase。ADR-0005)で、Web の engine 型(`damageMod` などの camelCase)
  とは違う。変換は Web 側(`onlineSource.ts` の `mapItem` / `mapAbility`)で行う。
- Web の E2E フィクスチャ(`web/e2e/support/pokedexFixture.ts`)は、Web レーンの追従まで effect を返さない
  (契約上は省略可なので契約どおり)。契約テストの型レベルのキー一覧には `effect` を省略可として足した。

### 5. 応答サイズ・上限・イベント・キャッシュ

- 実データ(2026-10-01 の import レポート)は持ち物 166 件のうち効果あり 41 件、特性 216 件のうち効果あり 17 件。
  効果の JSON は1件あたり数十バイトなので、`searchItems?limit=200`(全件)の増分は数 KB。数百 KB にはならない。
  `searchItems` の `limit` の上限(200)は変えない。`getSpecies` の特性は1種族4件まで。
- ADR-0208 の上限(`itemCandidates` などの `maxItems: 64`)は calc の入力の話で、この変更では変えない。
  Web が候補を 64 件に切り詰める(Web レーン)。
- calc-svc は公開 API を読まない(マスタは内部 API。ADR-0204)ので、計算・計算イベント(ADR-0212)は変わらない。
  gateway は本文をそのまま中継し、キャッシュを持たない。Web の IndexedDB のマスタキャッシュ(#210)は、
  effect を持たない古い応答を覚えている場合があるので、Web レーンが版の切り替えを扱う。

## 却下した案

- **効果一覧の公開 API を別に切る**: 往復が増え、ID の突き合わせがクライアントに要る(DECISIONS.md 2026-10-01)。
- **公開用の効果スキーマ(camelCase など)を新しく定義する**: 共通マスタの形と二重になり、変換と検証が2か所になる。
- **不正な効果はその行だけ effect を省いて 200**: 「効果なし」と区別できず、候補から黙って消える(誤った結果を出すより止める)。
- **pokedex 側で検証しない(内部 API と同じく運ぶだけ)**: 公開 API の受け手(Web・iOS)には共通マスタの検証が無い。

## テスト

- `services/pokedex/internal/httpapi/public_effect_test.go`(`make test`): 効果あり・なし(キー省略)・内部 API との値の一致・
  不正な効果は 503 で本文に出ない・載らない行は検証しない・契約の検証が effect の形を見る。
- `services/pokedex/internal/httpapi/pokedex_test.go` の `TestGetSpecies`: 期待値に特性の effect を足した。
- `services/internal/api/public_effect_contract_test.go`(`make test`): effect は省略可で `MasterEffect` を参照する。
- `services/pokedex/importer/public_effect_mysql_test.go`(`make test-db`・`-tags mysql`): 実 MySQL で公開と内部の effect が一致する。
- `web/e2e/support/pokedexFixture.contract.test.ts`: 省略可の `effect` を型レベルのキー一覧に足し、フィクスチャはまだ返さないことを確かめる。
