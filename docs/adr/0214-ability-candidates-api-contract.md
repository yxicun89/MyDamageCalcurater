# ADR-0214: 一括計算・逆算の特性候補を HTTP 契約に出す(issue 272 の API レーン担当分)

- 状態: 採用
- 日付: 2026-09-25
- 関連: ADR-0126(engine の特性候補。データレーン。この ADR の前提)、ADR-0009(一括計算のプリセット)、
  ADR-0010(逆算)、ADR-0011(WASM 境界。`engine/wasmapi` の `defenderAbilities`/`unknownAbilities` が
  参考実装)、ADR-0105 §5(内部 API がスロット4を落とす前例)、ADR-0100 §3(`species_abilities.slot` が
  1..4 で 4 = Showdown の特殊枠 `"S"`)、ADR-0208(件数の上限)、ADR-0200(API 契約とエラー語彙)、
  DECISIONS.md 2026-09-25「issue #274/#272 の防御側の詳細」(`defenderOverride` を先に採用済み)

## 背景

ADR-0126 は engine に `BulkInput.DefenderAbilities []Ability` と `ReverseInput.UnknownAbilities []Ability`
を追加した。0〜3件の解決済み特性を受け取り、結果が完全に同じになる特性は1行/1候補にまとめ、違うときだけ
分ける。engine はマスタを持たないので、「その種族がどの特性を持つか」を解決して渡すのは呼び出し側(calc-svc)
の仕事になる。ADR-0126 §他レーンへの依頼で、API レーンには次を依頼されている:

1. 既に採用済みの `BulkCalcRequest.defenderOverride.abilityId` を `DefenderAbilities` の1件として渡す。
2. **指定が無ければ、calc-svc が種族の全特性をマスタから解決して渡す**(1つしか特性を持たない種族は
   必ずその特性が効くようにするため。issue の境界値の受け入れ条件)。
3. 逆算に `ReverseRequest.unknownAbilityId`(任意)を足し、同じ既定にする。
4. `BulkCalcRow`・`ReverseCandidate` に `abilityId`(string)と `abilityIds`(string[])を足す。

## 決定

### 1. `defenderOverride.abilityId` / `unknownAbilityId` は1件の ID(配列にしない)

WASM 境界(`engine/wasmapi`)は呼び出し側が解決済みの特性実体を配列で渡す設計だが、HTTP は ID を渡し
calc-svc が解決する既存の設計(ADR-0200)と揃える。`defenderOverride`(DECISIONS.md 2026-09-25で採用済み)
に `abilityId` を1つ追加するだけでよく、逆算にも `unknownAbilityId`(直接のフィールド。`unknownSpeciesKey`
と同じ命名規則)を1つ追加する。将来 `defenderOverride.ranks`/`status` を追加する余地は残す(このタスクの
範囲外)。

### 2. 省略時は種族の特性を解決するが、**4件目(Showdown の特殊枠 `"S"`)は落とす**

`engine.MaxAbilityCandidates = 3`(通常特性2つ + 隠れ特性1つ)だが、pokedex のマスタは実データで
`species_abilities.slot` が 1..4 まで存在する(4 = Showdown の特殊枠 `"S"`。ADR-0100 §3・ADR-0103 §12)。
指定が無いときに `species.Abilities`(スロット順)をそのまま `DefenderAbilities` に渡すと、4件持つ種族で
`engine.ErrInvalidAbilityCandidates`(件数超過)が常に発生し、そうした種族の一括計算・逆算が既定のまま
使えなくなる(regression)。

pokedex-svc の内部 API export が既に同じ問題を解決済み(ADR-0105 §5): balance readmodel 向けの
`abilityIds` が4件を上限とする JSON Schema を持つため、4件ある種族はスロット4を落として `Report` に
記録している。本 ADR も同じ判断を踏襲し、`species.Abilities` の先頭 `engine.MaxAbilityCandidates`(3)件
だけを候補にする(スロット順で 1・2・3 = 通常2つ+隠れ特性。4 = `"S"` は落ちる)。ADR-0105 §5と違い、
落としたことをレスポンスや報告には残さない(利用者が `abilityId` を明示すれば `"S"` も選べるため、
「黙って選べなくなる」わけではない。頻度も低いと見て報告の仕組みまでは作らない)。

### 3. エラー語彙: マスタに無い ID は `unknown_ability`、種族が持たない ID は `invalid_input`

`defenderOverride.abilityId`/`unknownAbilityId` を calc-svc がまず `store.Ability(id)` で解決し、
見つからなければ 400 `unknown_ability`(`resolveIndividual` の `abilityId` と同じ語彙)。解決できた場合は
そのまま `engine.CalcBulk`/`CalcReverse` に渡し、種族が持たない特性であることの検査(`abilityCandidates`
の `slices.Contains(species.Abilities, a.ID)`)は engine に任せる。`engine.ErrInvalidAbilityCandidates` は
`errors.go` の `engineSentinels` に `api.InvalidInput` として追加した(新しい code は増やさない。
WASM 境界〈`engine/wasmapi`〉と同じ判断)。

### 4. 応答の `abilityId`/`abilityIds` は必須(WASM の `omitempty` とは異なる)

WASM は「特性を送らなかったときは出さない」(`omitempty`)ことで、Web がまだ送らない間は応答をバイト単位で
従来と同じに保っている。HTTP は既定で必ず種族の特性を解決して渡す設計にした(§背景 2)ため、
`species.Abilities` が常に1件以上を持つ(`services/internal/master/species.go` の `speciesAbilities` が
0件を `ErrInvalidRow` にする)ことから、**`abilityId`/`abilityIds` は常に入る**。omitempty にする理由が
無いので、両方を `required` にした。これにより「一括計算・逆算の応答には防御側/相手側の特性が必ず入る」と
いう契約上の保証を作れる(Web・iOS が画面に表示するかどうかは各レーンの判断)。

### 5. 行数・候補数の見積もりを openapi.yaml と ADR-0208 に反映する

一括計算の行数の基本式 `len(presets) × len(itemVariants)`(上限 8 × 64 = 512)は変わらないが、特性ごとに
結果が違う技では、その基本数のうち最大3倍(`engine.MaxAbilityCandidates`)まで行が分かれる
(上限 512 × 3 = 1536)。結果が同じ特性は1行にまとまるため、特性が効かない技(大半)では行数は変わらない。
逆算も同様に、候補数の基本上限 `2 性格クラス × 64 itemCandidates = 128` が、特性が効く技では
`2 × 3 × 64 = 384` まで増えうる(`maxCandidates` の指定できる範囲〈1..128〉自体は変えていない。
これは「返す件数をこの値で切り詰める」ための上限であり、384件から絞り込みたいときにそのまま使える)。
`api/openapi.yaml` の該当箇所(`itemVariants`・`maxCandidates` の description)と ADR-0208 にこの旨を
追記した。

**計算量への影響(セキュリティ上の判断)**: ADR-0208 の狙いは「小さいリクエストで計算量を増幅されない」
ことだが、特性候補の件数はクライアントが直接指定できる配列ではなく(`abilityId` は1件だけ、省略時は
種族が実際に持つ特性数〈最大3。本 ADR §2 で上限も固定〉)、**攻撃者が増やせる新しい増幅経路ではない**。
ADR-0126 の実測(逆算最悪ケースで特性3つのとき約38ms)は既存の目安(ADR-0108 の目安 約25ms)を超えるが、
これは「1リクエストあたりの基礎コストが約1.5倍になった」という話であり、リクエストの中身を変えて
更に増幅できるわけではない。ADR-0126 が追記している最適化(特性ごとの逐次比較で打ち切る)は
データレーンの後続タスク。

## 実装

- `api/openapi.yaml`: 新規スキーマ `DefenderOverride { abilityId?: string }`(`BulkCalcRequest` に追加)。
  `ReverseRequest.unknownAbilityId?: string`。`BulkCalcRow`・`ReverseCandidate` に
  `abilityId`(必須)・`abilityIds`(必須・`minItems: 1`)。
- `services/calc/internal/httpapi/convert.go`: `resolveAbilityCandidates(label, species, overrideID) ([]engine.Ability, error)`
  を新設。`overrideID` があれば `store.Ability` で解決した1件、無ければ `species.Abilities` の先頭
  `engine.MaxAbilityCandidates` 件を `store.Ability` で解決する(`species.Abilities` の各 ID がマスタに
  無いことは起きない。`buildSpecies` が起動時に検証済み)。`bulkResultFrom`・`reverseResultFrom` に
  `row.Ability.ID`/`row.AbilityIDs`・`c.Ability.ID`/`c.AbilityIDs` を配線。
- `services/calc/internal/httpapi/errors.go`: `engineSentinels` に
  `{engine.ErrInvalidAbilityCandidates, api.InvalidInput}` を追加。
- `services/calc/internal/httpapi/server.go`: `CalcBulk`・`CalcReverse` で `resolveAbilityCandidates` を
  呼び、`engine.BulkInput.DefenderAbilities`・`engine.ReverseInput.UnknownAbilities` に渡す。
- HTTP/WASM パリティテスト(`parity_test.go`)は、WASM 側のテスト入力にも calc-svc の既定と同じ特性
  (`wasmAbilitiesForSpecies`)を渡すよう更新した(HTTP が既定で特性を渡すようになったため、パリティを
  保つには WASM 側テストの入力も合わせる必要がある。省略時に両者が異なる前提のまま比べると意味が無い)。
- 新規テスト `services/calc/internal/httpapi/ability_candidates_test.go`: 4特性(うち1つだけ効果を持つ)の
  架空種族で、既定(3件に切り詰め・4件目は使わない)・`defenderOverride`/`unknownAbilityId` での固定・
  種族が持たない特性の `invalid_input`・マスタに無い特性の `unknown_ability` を一括計算・逆算の両方で
  固定した(mutation testing で truncation ロジックの実効性を確認済み)。

## 影響

- Web・iOS: `BulkCalcRow`・`ReverseCandidate` の応答に `abilityId`/`abilityIds` が必須で増える(生成物の
  再生成が必要)。防御側/相手側の特性を選べる画面(ADR-0126 の依頼)は各レーンの担当。
- 計算量: 一括計算・逆算とも既定で最大3倍の `CalcDamage` 呼び出しになる(§5)。
- 既存の数値: 特性を渡さない経路が無くなった(既定で必ず種族の特性を渡す)ため、**特性が効く技では
  一括計算・逆算の行数/候補数が増える**(意図した挙動。issue の目的そのもの)。特性が効かない技では
  数値・行数とも従来と不変。
