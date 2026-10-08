# ADR-0142: 技の機構の段階1 — 取得元のフィールドで決まる機構を engine で正しく計算し、未対応の印を外す(issue #271 / #270 の次の段)

- 状態: 採用
- 日付: 2026-10-09
- レーン: データ(ADR 帯 `0100〜`)
- 関連: ADR-0121(技の機構のデータ)、ADR-0123(未対応の印)、ADR-0176(特性の段階1。かたやぶり・急所無効・てんねん)、
  ADR-0136 / ADR-0223(MasterMove に列を足すときの作法・入れ替えの順序)、ADR-0118(effects.json 2つの一致)、
  ADR-0150(調整の探索)、ADR-0010(逆算)、ADR-0006(確定数)、ADR-0807(生成物は Git に置かない)

## 背景

ADR-0123 で engine は技の機構(ADR-0121 の 13 種)を持つ技を通常の式で計算し「未対応」の印を付けるだけにした。
対戦でよく使う多段技・固定ダメージ技・一撃必殺・必ず急所・防御ランク無視・参照する能力値が違う技(防御で攻撃する・
相手の攻撃で攻撃する・特殊技で防御を参照する)は、印が付くだけで数値は誤ったまま。

このうち次の 7 種は、取得元(Showdown の技データ)の**フィールドの値だけで機構の中身が決まる**。技名の分岐を
持たずに engine の一般の規則として計算でき、@smogon/calc 0.12.0 の Champions 世代と照合できる(一撃必殺を除く)。

| 機構 | 取得元のフィールド | 中身 |
|---|---|---|
| `multi_hit` | `multihit`(回数 or [最小, 最大]) | 当たる回数 |
| `fixed_damage` | `damage`(`"level"` or 数値) | ダメージの値。`damageCallback` 由来(残り HP の半分 等)は中身が無い |
| `ohko` | `ohko`(`true` or タイプ名) | 一撃必殺。タイプ名はそのタイプの相手に効かない |
| `always_crit` | `willCrit` | なし(機構だけで決まる) |
| `ignore_defense_ranks` | `ignoreDefensive` | なし(機構だけで決まる) |
| `alt_offense_stat` | `overrideOffensiveStat`・`overrideOffensivePokemon` | 攻撃に使う能力値・ポケモン |
| `alt_defense_stat` | `overrideDefensiveStat` | 防御に使う能力値 |

`tools/importer/fetch-showdown.mjs` は ADR-0121 の時点でこれらを取得元の表現のまま `mechanism` に出している
(`multihit`・`damage`・`ohko`・`willCrit`・`overrideOffensiveStat`・`overrideOffensivePokemon`・
`overrideDefensiveStat`・`ignoreDefensive`)。importer は真偽だけを見て値を捨てていた。

## oracle(@smogon/calc 0.12.0 champions.js)で確かめた挙動

`tools/golden/node_modules/@smogon/calc/dist/move.js` と `mechanics/champions.js` を読んで確かめた。

- **多段**: `Move` の構築時に回数を決める。固定回数はその値。範囲は `options.ability === 'Skill Link'` なら最大、
  それ以外は **最小 + 1**([2,5] → 3)。`multiaccuracy` の固定回数(トリプルアクセル等)も同じ値(`options.hits` が無ければ)。
  計算は1発ごとに16段階を作り(`damageMatrix[hit]`)、2発目以降は `checkMultihitBoost`(受けると能力が上がる特性・
  きのみ 等)と `calculateFinalModsChampions(…, hitCount)`(マルチスケイルは1発目だけ)で変わりうる。これらの特性・
  持ち物は ADR-0123 で既に `UnsupportedDefender` の印が付き、ベクタでは使わない。よって段階1の対象では**全発が同じ16段階**。
  `ability` は `Move` の構築時の引数で、`calculate()` は攻撃側の特性から回数を決め直さない(生成器は Move に特性を渡す)。
- **固定ダメージ**: `handleFixedDamageMoves` が技名(ちきゅうなげ・ナイトヘッド = レベル)で決める。判定の位置は
  タイプ相性の無効 → 特性の無効・吸収 → サイコフィールドの先制技 の**後**。一致・相性の倍率・急所・やけど・壁・持ち物は掛からない。
- **一撃必殺**: oracle は扱わない(威力 0 のままダメージ 0)。照合できないので engine の単体テストだけで決める。
- **必ず急所**: `isCrit = options.isCrit || data.willCrit`。急所に当たらない特性(カブトアーマー等)は急所を外す
  (かたやぶりで無視される)。
- **防御ランク無視**: `calculateDefenseChampions` で `move.ignoreDefensive` なら防御側のランクを正負とも使わない。
- **攻撃に使う能力値**: oracle は技名で分岐(ボディプレス = 攻撃側の防御とその**ランク**、イカサマ = 防御側の攻撃と
  その**ランク**)。急所で負のランクを無視・防御側のてんねんでランクを無視するのは通常と同じ。攻撃側の実数値の補正
  (`calculateAtModsChampions`: ちからもち・はりきり 等)は**技の分類(物理)で**掛かる(防御の補正は掛からない)。
  やけども分類で掛かる。
- **防御に使う能力値**: `hitsPhysical = overrideDefensiveStat === 'def' || 物理`。防御側の実数値・ランク・天候の
  防御補正(すなあらしの岩の特防 / ゆきの氷の防御)・防御側の実数値の補正は `hitsPhysical` で決まる。
  **壁(リフレクター・ひかりのかべ)とやけどは技の分類で**決まる(サイコショックはひかりのかべで減り、リフレクターでは減らない)。

## 決定

### 1. 段階1と段階2の境界

- **段階1(この ADR)**: 上の 7 種を engine で計算し、印を外す。回数は oracle と同じ既定(下の §3)。
  逆算・調整・一括計算も CalcDamage の合成なので同じ計算になる(§6)。
- **段階2(別 ADR・API / Web / iOS レーン)**: 利用者が多段の回数を選ぶ入力(公開 API `CalcRequest` 等・WASM・画面)、
  1発ごとのダメージを公開 API の応答に出すこと、一撃必殺の命中率の表示。段階1では公開 API(`CalcResult`)の契約を変えない
  (合計の `rolls` と確定数が正しくなるだけ。1発ごとの値は WASM の応答にだけ出す)。
- 対象外のまま: `variable_power`・`move_specific`・`field_specific`・`type_change`・`effectiveness_change`・
  `priority_change`、`fixed_damage` のうち `damageCallback` 由来(中身が無い)。

### 2. 機構の中身(engine.Move.Params)

```go
type MultiHit struct{ Min, Max int }          // Min == Max は固定回数
type FixedDamage struct{ Level bool; Value int } // Level は攻撃側のレベル。どちらか一方だけ
type OHKO struct{ ImmuneType Type }            // ImmuneType は TypeNone(なし)か、そのタイプの相手に効かないタイプ
type OffensePokemon string                     // "" | "attacker" | "defender"
type MechanismParams struct {
	MultiHit       *MultiHit
	FixedDamage    *FixedDamage
	OHKO           *OHKO
	OffenseStat    StatKey        // "" は分類どおり(物理 atk / 特殊 spa)
	OffensePokemon OffensePokemon // "" と attacker は攻撃側、defender は防御側の能力値とランク
	DefenseStat    StatKey        // "" は分類どおり(物理 def / 特殊 spd)
}
// Move.Params MechanismParams
```

- `always_crit`・`ignore_defense_ranks` は中身を持たない(機構だけで計算する)。
- `multi_hit`・`fixed_damage`・`ohko`・`alt_offense_stat`・`alt_defense_stat` は**中身があるときだけ**計算し印を外す。
  機構があって中身が無い(取り込み前の DB・`damageCallback` の固定ダメージ・WASM の入力で省略)ときは従来どおり
  通常の式 + 印(ADR-0123 のまま)。
- 検証(`CalcDamage` が `ErrInvalidMechanismParams` を包んで返す): 対応する機構の無い中身(例: `MultiHit` があるのに
  `multi_hit` が無い)、変化技の中身、`MultiHit` が `1 <= Min <= Max`・`Max >= 2`・`Max <= MaxMultiHits(10)` でない、
  `FixedDamage` が `Level` と `Value > 0` のちょうど一方でない、`OffenseStat` / `DefenseStat` が HP・未知、
  `OffensePokemon` が未知。`OHKO.ImmuneType` が相性表に無いタイプは `ErrUnknownType`。
  `MaxMultiHits = 10` は取得元の最大(10 回)で、確定数の畳み込みの計算量の上限でもある。

### 3. 多段技

- 回数: 固定回数はその値。範囲は、攻撃側の特性の効果 `MaxMultiHit`(スキルリンク。effects.json のデータ。
  特性名をコードに持たない)があれば `Max`、無ければ `Min + 1`(oracle と同じ。期待値は約 3.1 回だが oracle に合わせる)。
- 1発のダメージは通常の計算と同じ(段階1では全発が同じ)。`DamageResult.HitRolls [][16]int` に1発ごとの16段階
  (長さ = 回数)、`DamageResult.Rolls[i]` は**同じ段の合計**(`Σ HitRolls[h][i]`。最小・最大の合計が正しく、
  表示の % の範囲もそのまま使える)。単発の技は `HitRolls == nil`(既存の結果の形を変えない)。ダメージが出ない
  (無効・威力 0)ときも nil。
- 確定数は**1回の使用 = 回数ぶんの独立な乱数**として数える: `n = ceil(HP / ΣMax)`、`n × ΣMin >= HP` なら確定、
  それ以外は n 回の使用(n × 回数 の独立な16段階)の合計が HP 以上になる確率。全発が同じなら
  `koProbability(per, HP, n × 回数)` と一致する。`ComputeKO(rolls, hp)` は単発用にそのまま残し、
  CalcDamage・一括計算は結果から確定数を作る関数(1発ごとの段を見るもの)を使う。

### 4. 固定ダメージ・一撃必殺

- 固定ダメージ: 無効の判定(タイプ相性 → 特性の無効・吸収 → サイコフィールドの先制技)の後、全 16 段を
  `Level` なら攻撃側のレベル(Lv50)、`Value` ならその値にする。一致・相性・急所・やけど・壁・持ち物・特性の倍率は掛けない。
  威力 0 でも `zero_power` の印を付けない(中身があるとき)。
- **一撃必殺の表現**: 当たれば倒す、を**全 16 段 = 防御側の最大 HP・確定 1 発(`KO{Hits:1, Guaranteed:true}`)**で表す。
  命中率は数値に入れない(他の技も命中率を計算に入れていない。表示は段階2)。無効は次の順で 0:
  タイプ相性の無効 → 特性の無効・吸収 → サイコフィールドの先制技 → `OHKO.ImmuneType` を防御側が持つ /
  防御側の特性の効果 `PreventsOHKO`(がんじょう。effects.json のデータ。かたやぶりで無視される = `Breakable`)。
  最後の2つは `Nullified = "ohko_immune"`(新しい値。`NullifyKind` は「タイプ相性以外の理由」)。
  レベル差による失敗は Lv50 固定なので扱わない。

### 5. 必ず急所・防御ランク無視・参照する能力値

- 必ず急所: 入力の `Critical` が false でも急所として計算する(かたやぶりの判定の後、急所に当たらない特性の判定の前に立てる)。
  印は付けない(ADR-0123 §3 の条件つきの印は不要になる)。
- 防御ランク無視: 防御に使う能力値のランクを正負とも 0 として扱う。印は付けない。
- 攻撃に使う能力値: 実数値とランクは `OffensePokemon` の個体の `OffenseStat`(空なら分類どおり)から取る。
  急所の負のランク無視・防御側のてんねんは通常と同じ。攻撃側の実数値の補正(持ち物・特性の `StatMods`・
  `SeparateStatMods`・`OffBoostType`、防御側の `DefResistType`)とやけどは**技の分類のキー**で引く(oracle と同じ)。
  実機(Showdown)はボディプレスに防御の補正を掛けるが、ゴールデンの正は oracle なので oracle に合わせる。
- 防御に使う能力値: 実数値・ランク・天候の防御補正・防御側の実数値の補正を `DefenseStat` のキーで引く。
  壁・やけどは技の分類で決める。

### 6. 逆算・調整・一括計算

- 一括計算: 行ごとの結果は CalcDamage と同じ(`HitRolls`・確定数を含む)。
- 逆算: 探索する能力は、攻撃側なら技の攻撃に使う能力値(ボディプレスは防御)、防御側なら技の防御に使う能力値
  (サイコショックは防御)。多段技は観測を「1回の使用の合計」として、**取り得る合計の集合**(各発の16段階の和の全組合せ)と
  照合する(段の合計 `Rolls` だけと照合しない)。`OffensePokemon = defender`(イカサマ)で防御側を逆算するときは、
  防御側の攻撃の SP・性格を探索しないので候補の `Unsupported` に `alt_offense_stat` の印を残す。
- 調整: `MinSPToKO` は攻撃側の攻撃に使う能力値を探索する(`KOSearchResult.Stat` がボディプレスで `def`)。
  `MinSPToSurvive` は防御に使う能力値と H の組を探索する(サイコショックで `def`)。多段技の倒す/耐える確率は §3 の数え方。
  確率の比較を組数(16^発数)で行う既存の経路は、多段で 16^(発数×回数) が桁あふれするので、多段のときは桁あふれしない
  方法で比べる(単発の既存の結果は変えない)。`SuggestSPForGoals`・配分(alloc)は、目標の技が `alt_offense_stat`・
  `alt_defense_stat` の中身を持つとき、探索する能力の組が技と合わないので結果の `Unsupported` に該当の印を残す(段階2)。

### 7. データの流れ

1. `tools/importer/fetch-showdown.mjs`: 変更なし(既に取得元の値を出している)。取り直し(`make import-fetch`)は不要。
2. `services/pokedex/importer`: `classifyMoveMechanisms` が値を捨てずに `Output.MoveMechanismParams`
   (`MoveMechanismParamsRow`。1技1行、中身を持つ攻撃技だけ)を作る。`ohko` のタイプ名は相性表のタイプ ID に写し、
   無ければ `ErrInvalidData`。`overrideOffensivePokemon` は `target` → `defender`・`source` → `attacker`、他は
   `ErrInvalidData`。能力値は `atk`/`def`/`spa`/`spd`/`spe` 以外を `ErrInvalidData`。
3. DB: migration **000013** `move_mechanism_params`(主キー `move_id`・`moves` への外部キー ON DELETE CASCADE・
   列はすべて NULL 可・値の CHECK)。既存の migration は書き換えない。sqlc に
   `ListMoveMechanismParams`・`DeleteMoveMechanismParams`・`InsertMoveMechanismParams`。
4. `services/internal/master`: `MoveRow.Params *MoveMechanismParamsRow` を `Move` で検証し `engine.Move.Params` に写す
   (中身と機構の対応・値域・タイプ。不正は `ErrInvalidRow`)。効果定義に `MaxMultiHit`・`PreventsOHKO` を足す。
5. 内部 API `MasterMove.mechanismParams`(下の §8)。pokedex-svc が返し、calc-svc が `MoveRow.Params` に写す。
6. `engine/wasmapi`: 入力 `move.mechanismParams`、特性の効果 `maxMultiHit`・`preventsOHKO`、結果 `hitRolls`(§8)。
7. effects.json(`data/importer/effects.json` と `testdata/golden/effects.json` を同じ内容で。ADR-0118):
   スキルリンク `{"MaxMultiHit":true}`、がんじょう `{"PreventsOHKO":true,"Breakable":true}`
   (oracle の `defenderAbilityIgnored` に がんじょう が入っているので `Breakable` の照合が通る)。
8. ゴールデン(`tools/golden/generate.mjs`): `mechanisms.json` を新設(Champions 世代)。§9。

### 8. 契約の差分

**内部 API(`api/openapi.yaml`)**: `MasterMove` に `mechanismParams`(必須キー・`null` は中身なし)。
受け取る calc-svc は、キーが無い(古い pokedex-svc)ことを `null` と同じに扱う(ADR-0223 §3 と同じ。入れ替えの順序に依存させない)。

```yaml
MasterMove.mechanismParams: { allOf: [ $ref: MasterMoveMechanismParams ] }   # required・null は中身なし
MasterMoveMechanismParams:   # nullable。required: 6 キーすべて(値は null 可)
  multiHit:       MasterMoveMultiHit      # nullable { min: int, max: int }
  fixedDamage:    MasterMoveFixedDamage   # nullable { level: bool, value: int }
  ohko:           MasterMoveOHKO          # nullable { immuneType: string | null }
  offenseStat:    string | null           # atk | def | spa | spd | spe
  offensePokemon: string | null           # attacker | defender
  defenseStat:    string | null
```

enum は付けない(ADR-0121 の `mechanisms` と同じ理由。検証は受け取った calc-svc の `services/internal/master`)。
**公開 API は変えない**(段階2)。WASM(公開 API ではない Web 内部の境界)は次を足す:

- 入力 `move.mechanismParams`(上と同じ形。キーは camelCase。省略・`null` は中身なし。能力値・ポケモン・タイプが語彙に無ければ
  `invalid_enum`、値域・機構との対応の誤りは `invalid_input`)。
- 特性の効果 `maxMultiHit`・`preventsOHKO`。
- 結果 `hitRolls`(calc の結果・bulk の各行の結果。常に配列。単発・ダメージなしは `[]`)。

### 9. ゴールデン

`testdata/golden/mechanisms.json`(Champions 世代・SP そのまま)。期待値は oracle の `result.damage`(多段は行列)。
生成器の `vector` は `m.hits === 1` の前提を外し、ベクタの `expected.hitRolls`(多段だけ)・`expected.rolls`
(段の合計)・`expected.ko`(生成器の独立な畳み込み。多段は 1回の使用 = 回数ぶんの独立な乱数)を出す。入力の
`Move` に `Mechanisms` と `Params` を載せる。一撃必殺は oracle が扱わないので入れない。最低限のラベル:

| ラベル | 内容 |
|---|---|
| `multi-hit-fixed/*` | 固定回数(2回)の物理・特殊 |
| `multi-hit-range/*` | 範囲 [2,5] の既定 3 回 |
| `multi-hit-skill-link/*` | スキルリンクで 5 回(生成器は `Move` に特性を渡す) |
| `multi-hit-ten/*` | 10 回 |
| `always-crit/*`・`always-crit-shell-armor/*` | 急所の指定なしで急所 / 急所に当たらない特性 |
| `ignore-defense-ranks/*` | 防御側の +6 / -6 |
| `alt-offense-def/*` | 攻撃側の防御とランク・急所 × 負のランク |
| `alt-offense-target/*` | 防御側の攻撃とランク |
| `alt-defense-def/*` | ひかりのかべ / リフレクター・すなあらし × 岩・ゆき × 氷 |
| `fixed-damage-level/*`・`fixed-damage-immune/*` | Lv50 のダメージ / タイプで無効 |

既存のファイルのバイト列・件数は変えない(新しいファイルだけ。既存のプール・乱数列に足さない)。
`metadata.json` の `exclusions` の技の理由を「段階1の機構は mechanisms.json で照合」に改める。

### 10. 印の扱い(ADR-0123 §3 の差し替え)

| 機構 | 印を付ける条件(段階1の後) |
|---|---|
| `always_crit`・`ignore_defense_ranks` | 付けない |
| `multi_hit`・`fixed_damage`・`ohko`・`alt_offense_stat`・`alt_defense_stat` | 中身が無いときだけ |
| `zero_power` | 威力 0 で、固定ダメージ・一撃必殺の中身が無いとき |
| その他 | ADR-0123 のまま |

同じ技が段階1外の機構も持つ(例: 多段 + 威力変動のトリプルアクセル)なら、段階1の分は計算し(回数を掛ける)、段階1外の機構の
印だけが残る。

### 11. デプロイの順序

**アプリ(calc-svc・pokedex-svc・Web の WASM)→ 取り込み(`make import`)→ master-release**。

- 先に取り込むと、新しい効果定義のキー(`MaxMultiHit`・`PreventsOHKO`)を古い calc-svc の厳格なデコードが拒否し、
  マスタの読み込みに失敗する。
- 新しいアプリ + 取り込み前の DB は、`move_mechanism_params` が空なので印が残るだけ(従来どおり。誤った数値を出さない)。
- migration 000013 はアプリの起動時(または取り込み前)に適用する。表を足すだけで既存の行・列に触らない。

## 実装の手順(implementer 向け。テストは spec-writer が先に置いた)

1. engine: `MechanismParams` 等の型・`Move.Params`・`DamageResult.HitRolls`・`AbilityEffect.MaxMultiHit` /
   `PreventsOHKO`・`NullifyOHKOImmune`・`ErrInvalidMechanismParams`・`MaxMultiHits`。`calcDamageNoKO` で
   中身の検証 → `applyAbilityPreconditions` の中で always_crit を立てる(PreventsCritical の判定の前)→
   無効の判定の後に 一撃必殺 / 固定ダメージ を分岐(威力 0 の早期 return より前)→ `attackDefenseStats` を
   「使う個体・能力値のキー」と「補正を引くキー(分類)」に分ける → 多段は1発を計算して回数ぶん並べる。
   確定数は結果から作る関数(`ComputeKO` は残す)。`moveMarks` / `mechanismHandled` を §10 に合わせる。
   一括(`bulk.go` の `ComputeKO(row.Result.Rolls, …)`)・逆算(`reverseStat`・`reverseCandidate` の照合)・
   調整(`koChancePercent`・`goalEventCount` の桁あふれ・探索する能力・目標の印)を §6 に合わせる。
   テスト: `engine/move_mechanism_stage1_test.go`・`engine/unsupported_test.go`。
2. `services/internal/master`: `MoveMechanismParamsRow`・`MoveRow.Params`・`Move` の検証と写像、
   `effects.go` の `abilityEffectFields` / デコード / エンコードに `MaxMultiHit`・`PreventsOHKO`。
   テスト: `move_mechanism_params_test.go`。
3. importer: `MoveMechanismParamsRow`・`Output.MoveMechanismParams`・変換(`convert_move_mechanisms.go` の parse 関数が値を返す)・
   `Apply`(Delete → Insert。`move_mechanisms` と同じ場所)・`storetest`・照合の要約(任意で `moveMechanismParams: <行数>`)。
   テスト: `convert_move_mechanism_params_test.go`。
4. DB: `migrations/000013_create_move_mechanism_params.{up,down}.sql`・`query/pokedex.sql` の3クエリ・`make gen`(sqlc)。
   テスト: `services/pokedex/db/move_mechanism_params_layout_test.go`(`-tags mysql` の投入テストも既存に倣って足す)。
5. pokedex-svc `internal/httpapi/master.go`: `ListMoveMechanismParams` を技 ID ごとに `MechanismParams` に写す(行が無ければ null)。
   calc-svc `internal/master/export.go` `buildMoves`: `MechanismParams` → `MoveRow.Params`。テスト:
   `services/calc/internal/master/move_mechanism_params_test.go`。例のファイル(`readExample`)・Web の
   `web/src/master/exportSnapshot.ts`(`CalcSnapshotMove`)に `mechanismParams: null` を足す(契約の必須キー)。
6. wasmapi: `moveDTO.MechanismParams`・`abilityEffectDTO` の2項目・`calcResultDTO.HitRolls`(常に配列)。
   テスト: `engine/wasmapi/move_mechanism_stage1_test.go`。Go/WASM 一致のベクタ(`testdata/vectors.json`)に多段の1件を足す。
7. effects.json 2つにスキルリンク・がんじょう(§7-7)。`tools/golden/generate.mjs` に `mechanisms.json`(§9)。
   `make golden-generate` → `make test-golden`(`engine/golden_mechanisms_test.go` を含む全件一致)。
8. iOS の生成物は iOS レーンでの再生成(内部 API なので iOS は使わないが、生成の差分だけ確認する)。

## 結果

- 良い点: 多段技・ちきゅうなげ系・必ず急所・防御ランク無視・ボディプレス/イカサマ/サイコショックの結果が正しくなり、印が消える。
  技名をコードに持たない(取得元のフィールドの一般の規則と、特性の効果データ)。
- 注意: oracle と実機の差(ボディプレスの防御の補正)は oracle に合わせた。一撃必殺は oracle で照合できない。
- 注意: 逆算でイカサマを防御側から求めるとき、調整の目標探索・配分で参照する能力値が違う技を使うときは印が残る(段階2)。
- 既存のゴールデンは全件そのまま一致すること(新しいファイルの追加だけ)。
