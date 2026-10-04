# ADR-0139: 素早さに効く特性・持ち物の補正を、効果定義の正規化データとしてマスタに持たせる(issue #235 第2段のデータ)

- 状態: 採用
- 日付: 2026-10-04
- レーン: データ(効果定義・importer・共通マスタ)。engine は型と語彙だけ、wasmapi は受け口だけ
- 関連: issue #235、ADR-0710(判定の Applied / Ignored。第2段の依頼)、ADR-0712(まひ)、ADR-0005(データ駆動の効果定義)、
  ADR-0101(effects.json が本番の正)、ADR-0103 §6(網羅性)、ADR-0118(効果定義のゴールデンの写し)、ADR-0120(効果データの網羅)、
  ADR-0121(技の機構のマスタ化)、ADR-0123(未対応の印)、ADR-0128・ADR-0138(read model の互換)、ADR-0175(持ち物の役割)、
  ADR-0204(内部 API)、ADR-0218(公開 API の effect)

## 背景

判定(judge)の素早さは追い風とこだわりスカーフだけを反映し、天候・場・状態異常に依存する特性と、スカーフ以外の素早さの持ち物は
`*SpeedIgnored` の印で「反映していない」と返している(ADR-0710)。判定レーンは ID の switch を持たずに反映するため、
素早さの補正を正規化データとしてマスタ側に持ち、pokedex-svc の内部 API(`/internal/pokedex/master`)から引けるようにしてほしいと依頼した
(DECISIONS 2026-10-02。範囲は「全て」でユーザー決定)。

## 決定

### 1. 型と発動条件の語彙(engine に置く単一の正)

engine に型と語彙だけを置く(engine は素早さを計算しない。ダメージ計算はこの値を読まない)。
共通マスタ(services/internal/master)と判定(services/judge。services モジュールの internal を import できないが engine は import できる)が
同じ定数を参照するため。

```go
type SpeedCondition string // 閉じた語彙。Known() と AllSpeedConditions() を持つ
type SpeedMod struct {
    Condition SpeedCondition
    Modifier  int // 4096 基準(例 8192 = ×2、6144 = ×1.5、2048 = ×0.5)
}
// ItemEffect に SpeedMods []SpeedMod を足す(ResistBerryType の後・未対応の印の前)。
// AbilityEffect に SpeedMods []SpeedMod と IgnoresParalysisSpeedDrop bool を足す(Airborne の後・未対応の印の前)。
```

| 値(JSON にそのまま書く) | 成立する条件(判定が評価する) |
|---|---|
| `always` | 常に |
| `weather_sun` / `weather_rain` / `weather_sand` / `weather_snow` | 天候がその値(判定の `field.weather` と同じ4値) |
| `terrain_electric` | エレキフィールド |
| `has_status` | 状態異常が none 以外 |
| `item_lost` | 持ち物を失った後(特性だけ。持ち物の定義では不正) |

- 倍率は 4096 基準の整数(効果定義の他の項目・判定の追い風 8192 / スカーフ 6144 と同じ単位。分数にしない)。
  balance の abilities read model の effects と同じく「種類 + 係数」の正規化された行で、ID ごとの分岐を持たない。
- `IgnoresParalysisSpeedDrop`(特性だけ): まひの素早さ半減(ADR-0712 §3)を受けない。
- 語彙を足すときは ADR を書く(判定が評価できない条件を黙って入れない)。

### 2. 効果定義の JSON(共通マスタの厳格なデコード)

```json
{"SpeedMods": [{"Condition": "weather_rain", "Modifier": 8192}]}
{"SpeedMods": [{"Condition": "has_status", "Modifier": 6144}], "IgnoresParalysisSpeedDrop": true}
```

- `SpeedMods` は空でない配列。要素は `Condition` と `Modifier` のちょうど2キー(大文字小文字を区別・未知のキーを拒否)。
- `Condition` は語彙の文字列だけ。`Modifier` は 1..`engine.MaxEffectModifier` の整数で、**4096(×1.0)は拒否**(補正なしと区別できない)。
- 1つの `SpeedMods` に同じ条件を2回書かない。持ち物で `item_lost` は不正。`IgnoresParalysisSpeedDrop` は特性だけ・true だけ。
- **配列の順は評価の優先順**: 成立した最初の要素だけを掛ける(@smogon/calc の getFinalSpeed は特性・持ち物それぞれ1つの補正しか積まない)。
  正準形でも並べ替えない。要素のキーは `Condition` → `Modifier`。
- **素早さの補正は SpeedMods だけで表す**: `StatMods` の `spe` を拒否する(同じ補正を2通りに書けないようにする。既存データに `spe` は無い)。
- 素早さの項目が無い効果の正準形は変わらない(既存の行・dataVersion を動かさない)。

### 3. 定義の置き場(importer)

- 正は `data/importer/effects.json` の新しい節 `speedItems`・`speedAbilities`(ID → 素早さの項目だけのオブジェクト)。
  ダメージの節(`items`・`abilities`)には素早さの項目を書かない。逆も同じ(混ぜたら `ErrInvalidInput`)。
- 別の節にした理由: `testdata/golden/effects.json` はダメージの節の写しで(ADR-0118)、生成器は「定義があるのにダメージが変わらない」
  定義で止まる(ADR-0120 §3)。素早さだけの定義をダメージの節に入れると、ゴールデンの生成器・engine のゴールデンテストまで変える必要がある。
  節を分ければゴールデン・oracle の照合は一切変わらない。
- 取り込みで、同じ ID のダメージの定義と素早さの定義を1つのオブジェクトに合わせ、共通マスタでデコードして正準形で
  `item_effects` / `ability_effects` に入れる(テーブル・migration は変えない)。取り込まない ID は `effect-unused` の警告。
- 網羅性(ADR-0103 §6)はダメージの指標のまま。素早さの節は数えない(`onModifySpe` を effectHooks に足さない)。

### 4. 内部 API・公開 API(契約は変えない)

- `getMasterExport` の `MasterItem.effect` / `MasterAbility.effect` は `MasterEffect`(`additionalProperties: true`)で、
  pokedex-svc は DB の JSON をそのまま返す。素早さの項目は追加のキーとして運ばれるので、**`api/openapi.yaml` の形の変更は要らない**。
  判定はここから読み、`engine.SpeedCondition` で条件を評価する(judge 側のデコードと NetworkPolicy・設定は判定レーン)。
- 公開 API(`searchItems` 等)の effect も同じ値・同じ形(ADR-0218)。素早さだけの持ち物の `roles` は空(ADR-0175 §1 の表に
  「SpeedMods → 役割なし」の行を足す)。判定画面で素早さの持ち物を選ばせる役割が要るなら、別の値として別 ADR で足す。
- calc-svc はロード時に共通マスタで検証するので、本変更と同時に更新される(Lookup のコピーは SpeedMods のスライスも複製する)。
- WASM の入力境界(engine/wasmapi)は `speedMods`(`condition`・`modifier`)・`ignoresParalysisSpeedDrop` を受け付け、engine の型に写す
  (Web の `fromPublicEffect` はトップレベルのフィールド名だけを camelCase にするので、WASM には
  `"speedMods":[{"Condition":"always","Modifier":2048}]` のように**配列の要素は PascalCase のまま**渡る。境界は未知のフィールドを拒否するが、
  キー名は大文字小文字を区別せずに受ける(encoding/json の照合)。この形を仕様として固定し、wasmapi・Web のテストで確かめる)。
  語彙に無い条件は `invalid_enum`。`modifier` の範囲は境界では検証しない(ダメージ計算が読まない値で、共通マスタが取り込み時に検証済み)。
- **デプロイ順**: 先にアプリ(calc-svc・Web の WASM)をロールアウトし、その後に master-release で再取り込みする
  (逆だと旧アプリが SpeedMods を未知のフィールドとして拒否し、計算が失敗する)。

### 5. read model(変えない)

balance の `abilities.json` の effects は防御側のタイプ相性の効果だけ(ADR-0017 §2)で、素早さの補正は出さない。
`speed-pokemon.json` も変えない。6ファイルの形は変わらず、balance・speed の loader(`DisallowUnknownFields`・schema の
`additionalProperties: false`。ADR-0128・ADR-0138)に影響しない。dataVersion は取り込みの入力が変わるので、再取り込みで通常どおり変わる。

### 6. ダメージ計算(変えない)

engine のダメージ計算は SpeedMods・IgnoresParalysisSpeedDrop を読まない。ゴールデン(fixed / random / legacy)は不変。
`TestItemRolesMatchEngineDamage`(持ち物)と `TestSpeedEffectsDoNotChangeDamage`(特性・持ち物の両側)が、素早さの項目で
Rolls・未対応の印が変わらないことを確かめる。

## 対象(件数)

Showdown の取得物(champions mod。`onModifySpe` のハンドラ。持ち物を失った後の特性は付随する状態のハンドラ)と
@smogon/calc 0.12.0 の Champions 世代(`getFinalSpeed`)を突き合わせ、既定のレギュレーションの使用可能集合(特性 216・持ち物 166)で数えた。

- **特性 7 件**: 雨・晴れ・砂・雪・エレキフィールドで ×2(5 件)、状態異常で ×1.5 かつまひの半減を受けない(1 件)、持ち物を失った後に ×2(1 件)。
- **持ち物 1 件**: 常に ×0.5(接地させる効果は従来どおり未対応の印。ADR-0120)。こだわりスカーフ(×1.5)は対象外で、判定の設定(ADR-0701)のまま。
- 対象外(使用可能集合に無い・Champions 世代に無い): 最初の数ターン ×0.5 の特性、最も高い能力が素早さのとき ×1.5 の特性 2 件、
  努力値系の持ち物 7 件(×0.5)、特定の種族だけ ×2 の持ち物。必要になったら語彙を足す ADR とともに足す。
- 実データの ID は ADR・テストに書かない。テストは件数と「条件 → 倍率」の組で固定する(`TestRepoSpeedEffectsMatchADR`)。

## 判定が使うときの意味(判定レーンへの申し送り)

- 連結の位置は getFinalSpeed と同じ: 追い風 → 特性(成立した最初の要素)→ 持ち物(成立した最初の要素)の順に 4096 基準で連結し、
  五捨五超入を1回。まひの半減はその後(ADR-0712 §3)で、`IgnoresParalysisSpeedDrop` の特性なら掛けない。
- `item_lost` が成立する(持ち物を失った)ときは、持ち物の補正を掛けない(getFinalSpeed と同じ。持ち物が無いので当然でもある)。
  判定の入力に「持ち物を失った」を表す欄が要るかは判定レーンが決める。
- 天候を打ち消す特性・持ち物(天候の影響を受けなくする傘など)は oracle も扱わないので、本 ADR でも扱わない(既知の限界)。

## 却下した案

- read model(balance の abilities.json・speed-pokemon.json)に素早さの欄を足す: loader が未知のフィールド・kind を拒否し、
  balance・speed は素早さの特性を使わない。判定は内部 API で足りる。
- 素早さの定義を新しいテーブル・`MasterItem` の新しい欄にする: migration と契約変更(API レーン)が要る。効果定義の JSON の項目で表せる。
- ダメージの節(items・abilities)に素早さの項目を混ぜる: ゴールデンの写しと生成器の検査(ADR-0118・ADR-0120)を変える必要がある。
- 型を services/internal/master だけに置く: 判定(別モジュール)から語彙を参照できず、判定が条件の文字列を二重に持つ。
- 倍率を分数(分子/分母)にする: 効果定義の他の項目・判定の既存の補正(4096 基準)と単位がずれ、変換が二重になる。
