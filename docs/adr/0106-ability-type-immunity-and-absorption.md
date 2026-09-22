# ADR-0106: 特性によるタイプの無効・吸収を効果定義に足す(P2-3b)

- 状態: 採用
- 日付: 2026-09-22
- 関係: ADR-0005(効果定義をデータにする)・ADR-0013(データにするもの/コードにするものの線引き)・
  ADR-0017(balance の特性の効果 `immune` / `absorb`)・ADR-0100 §6(効果定義の JSON 列)・
  ADR-0101(`data/importer/effects.json` が本番の正)・ADR-0103 §6(`effect-no-hook` の照合)・
  ADR-0105 §限界1(「特性の immune / absorb が export に出ない」の解消)・ADR-0002 追記 P2-1b(ゴールデンの oracle)

## 背景

`engine.AbilityEffect`(ADR-0005)には倍率を変える効果しか無い(`StabMod` / `OffBoostType` /
`DefResistType` / `ReduceSuperEffective` / `IgnoresBurn`)。「そのタイプを**無効**にする」(ふゆう)と
「そのタイプを**吸収**する」(ちょすい・もらいび・そうしょく等)を表せないため、

- ダメージ計算はふゆう持ちに地面技が通ってしまう(engine の欠陥)
- `pokedex export` の特性の read model に `immune` / `absorb` が一度も出ず、タイプバランス(TB3/TB5)は
  「タイプ由来の相性と倍率を変える特性だけ」で判定している(ADR-0105 §限界1・DECISIONS.md 2026-09-22)

ADR-0017 §2 は balance 側の受け口として `immune` / `absorb` の kind を既に定義しており、
`services/balance/schema/abilities.schema.json` の enum と loader も対応済み。**足りないのは engine 側の
効果定義と、そこから DB・importer・export へ流れる経路だけ**である。

## oracle(@smogon/calc 0.12.0)の実装をどう確認したか

`tools/golden/node_modules/@smogon/calc/src/mechanics/champions.ts` を読み、さらに
`Generations.get(0)`(Champions)で実際に `calculate()` を呼んで出力を確認した。

1. **無効の判定位置**(champions.ts L262〜283)。`typeEffectiveness === 0` で先に return したあと、
   特性による無効を見る:
   ```ts
   if ((move.hasType('Grass') && defender.hasAbility('Sap Sipper')) ||
       (move.hasType('Fire') && defender.hasAbility('Flash Fire')) ||
       (move.hasType('Water') && defender.hasAbility('Dry Skin', 'Water Absorb')) ||
       (move.hasType('Electric') && defender.hasAbility('Lightning Rod','Motor Drive','Volt Absorb')) ||
       (move.hasType('Ground') && !field.isGravity && defender.hasAbility('Levitate','Eelevate')) ||
       (move.flags.bullet && defender.hasAbility('Bulletproof')) || … ||
       (move.hasType('Ground') && defender.hasAbility('Earth Eater'))
   ) { desc.defenderAbility = defender.ability; return result; }
   ```
   `result.damage` は初期値のまま(数値 `0`)で返る。つまり oracle は**ダメージを 0 にするだけ**で、
   回復・能力上昇は一切モデル化していない。
2. **実測**(`calculate(genC, …)` の `result.damage`)。
   | 防御側の特性 | 技 | damage |
   |---|---|---|
   | Levitate | Earth Power | `0` |
   | Water Absorb | Surf | `0` |
   | Flash Fire | Flamethrower | `0` |
   | Volt Absorb / Motor Drive / Lightning Rod | Thunderbolt | `0` |
   | Sap Sipper | Energy Ball | `0` |
   | Earth Eater | Drill Run | `0` |
   | Dry Skin | Surf | `0` |
   | Dry Skin | Flamethrower | `63…75`(×1.25) |
3. **Champions 世代に在るか**(`genC.abilities.get(toID(name))`)。Levitate / Water Absorb / Volt Absorb /
   Flash Fire / Lightning Rod / Sap Sipper / Motor Drive / Dry Skin / Earth Eater / Bulletproof /
   Soundproof / Eelevate / Purifying Salt / Heatproof は**在る**。**Storm Drain は無い**(かつ
   champions.ts の水の無効の列に Storm Drain が入っていない)。
4. **副次効果を扱う箇所は無い**。champions.ts で `Flash Fire` が倍率に効くのは
   `attacker.hasAbility('Flash Fire') && attacker.abilityOn`(L919、攻撃側の「発動済み」状態を
   呼び出し側が渡したとき)だけで、防御側で吸収した結果として自動的に立つ状態ではない。

## 決定

### 1. `engine.AbilityEffect` に2つのフィールドを足す

```go
// AbsorbEffect は吸収したときの副次効果(マスタの記述)。ゼロ値は「吸収するが副次効果は持たない」。
// ダメージ計算はこの値を読まない(§4)。回復・能力上昇は戦闘の状態(現在HP・現在のランク)が要るため、
// ダメージ計算機の責務ではない。
type AbsorbEffect struct {
	HealNumerator   int     // 最大HPに対する回復の分子。0 は回復なし
	HealDenominator int     // 回復の分母(1..16)。HealNumerator が 0 でないときだけ意味を持つ
	BoostStat       StatKey // 上げる能力(HP 不可)。"" は無し
	BoostStages     int     // 上げる段階(1..6)。BoostStat があるときだけ意味を持つ
}

// AbilityEffect に追加
DefImmuneTypes []Type                  // 無効にする攻撃タイプ(ふゆう)。ダメージ 0、副次効果なし
DefAbsorbTypes map[Type]AbsorbEffect   // 吸収する攻撃タイプ(ちょすい等)→ 副次効果。ダメージ 0
```

- **無効と吸収を別のフィールドにする**理由: ADR-0017 §2 の kind `immune` / `absorb` と 1:1 に対応し、
  export の写像に判定ロジックが要らない。1つのフィールドに `Absorb bool` を持たせる案は、
  「ゼロ値=無効か吸収か」が値の中身に依存して読みにくい。
- 無効は副次効果を持たないので**配列**(タイプ ID 昇順・重複不可)、吸収は副次効果を持つので
  `DefResistType` と同じ **map**。
- **同じタイプを両方に書くのは不正**。落とすのは厳格デコード(`services/internal/master`)と WASM 境界の
  責務で、`engine` は入力の効果定義を検証しない(`DefResistType` と同じ扱い。ADR-0002 の「engine は
  与えられた効果を適用するだけ」)。それでも結果が揺らがないよう、**無効が吸収に勝つ**と決めておく。
- `Levitate` のように攻撃タイプが1つでも、複数タイプを無効にする特性(将来)に備えて配列/map にする。
  ハードコードした特性名はどこにも書かない(CLAUDE.md ドメイン規約)。

**データにするもの / コードにするもの**(ADR-0013 の線引きにならう):
- データ = どの特性がどの攻撃タイプを無効/吸収するか、回復の分数、上げる能力と段階。
- コード = 「無効・吸収ならダメージは 0」「タイプ由来の無効が先」という**手順**。

### 2. `AbsorbEffect` に入れてよい副次効果の種類

oracle がモデル化している種類は **無い**(§oracle 2・4)。それでも `AbsorbEffect` を空の struct に
しないのは、`immune` と `absorb` を区別するためだけなら `bool` で足りるが、「その吸収が何をするか」は
**マスタが記述すべき事実**であり、engine の外(UI の説明文・将来の戦闘シミュレーション・balance の
説明)が使うから。ゲームに実在する種類だけに絞り、v1 は次の2つに限る:

| 種類 | フィールド | 例 |
|---|---|---|
| 最大HPの n/m を回復 | `HealNumerator` / `HealDenominator` | ちょすい・ちくでん・つちけむり(1/4) |
| 能力を n 段階上げる | `BoostStat` / `BoostStages` | そうしょく(atk +1)・でんきエンジン(spe +1)・ひらいしん(spa +1) |

**扱わない**(限界。§限界):もらいび の「発動後に自分の炎技が ×1.5」は、engine の入力に「発動済み」の
状態が無く oracle も `abilityOn` を明示的に渡したときだけ適用する。もらいびは
`DefAbsorbTypes{fire: {}}`(副次効果なしの吸収)として持つ。

### 3. ダメージ計算への組み込み(`damage.go`)

`otherModifiers`(chainMods される「その他補正」)には**入れない**。倍率 0 を chainMods に混ぜると
丸めの経路が変わり、最低1ダメージの床(`if d < 1 { d = 1 }`)とも矛盾する。oracle と同じく
**早期 return** にする。

`CalcDamage` の既存の早期 return を次のように広げる(位置は変えない。`eff` を引いた直後、
`attackDefenseStats` より前):

```go
res.Effectiveness = eff.Multiplier()
_, res.STAB = stabModifier(in, moveType)

// タイプ由来の無効が先(oracle champions.ts L262 / ADR-0017 §5)。
if !eff.IsImmune() {
	res.Nullified = abilityNullification(in.Defender.Ability.Effect, moveType) // "" / "immune" / "absorb"
}
if in.Move.Category == CategoryStatus || in.Move.Power <= 0 || eff.IsImmune() || res.Nullified != "" {
	return res, nil
}
```

- **タイプ由来の無効が先**: oracle は `typeEffectiveness === 0` で先に return し、ADR-0017 §5 の
  「タイプ由来の無効と同じタイプを特性でも無効にする場合はタイプ由来として報告する」と一致する。
  したがって `eff.IsImmune()` のときは `res.Nullified` を空のままにする。
- **`Rolls` は 16 個とも 0**、`KO` はゼロ値(`Hits:0`)。相性 0 の既存の早期 return と同じ形で、
  生成器の `ko(rolls, hp)` も `max === 0` で `{Hits:0, Guaranteed:false, ChancePercent:0}` を返すため
  ゴールデンと一致する。
- **急所・壁・持ち物・天候・テラス・やけどは一切関係しない**(早期 return が最優先)。

### 4. `DamageResult` の形を変える(1フィールド追加)

```go
// NullifyKind は特性でダメージが 0 になった理由。"" は無効化されていない。
type NullifyKind string
const (
	NullifyNone   NullifyKind = ""
	NullifyImmune NullifyKind = "immune"
	NullifyAbsorb NullifyKind = "absorb"
)

// DamageResult に追加
Nullified NullifyKind
```

**変える理由**: `Effectiveness` だけでは表せない。ふゆう持ちに地面技(相性 ×2)を撃つと
`Effectiveness = 2.0` のまま `Rolls` が全部 0 になり、呼び出し側からは計算の不具合と区別が付かない。

**副次効果(回復・能力上昇)は返さない**。ダメージ計算機は現在HPも現在のランクも受け取らないので
結果を出せない。必要になったら呼び出し側が `AbilityEffect.DefAbsorbTypes` をそのまま読む。

**影響範囲**(追加だけで、既存フィールドの意味は変えない):
- `engine`: `CalcDamage` / `CalcBulk` / `CalcReverse` の戻り値の struct に無害な追加。
- `services/calc`・`api/openapi.yaml`: **このタスクでは出さない**。`CalcResult` に
  `nullified` を足すかは API レーンの判断で、足すまでは HTTP の応答は今までどおり(ダメージは 0 になる)。
- WASM(`engine/wasmapi`): 出力 DTO には**足さない**(HTTP と同じ扱いにする。ADR-0011 の契約差分を
  広げない)。入力 DTO には足す(§6)。
- `api/openapi.yaml` は**変更しない**ので `make gen` は不要(§6 の理由)。

### 5. 効果定義の JSON(`data/importer/effects.json`・DB の JSON 列)

`item_effects` / `ability_effects` の `effect` は JSON 列なので **migration は不要**。
`services/internal/master/effects.go` の厳格デコード/正準エンコードに2フィールドを足す:

- `abilityEffectFields` に `DefImmuneTypes` / `DefAbsorbTypes` を追加。
- `DefImmuneTypes`: JSON 配列。空配列は不正、重複は不正、表(`chart.Has`)に無いタイプは不正。
- `DefAbsorbTypes`: JSON オブジェクト。空は不正、キーは表にあるタイプ、値は
  `{"HealNumerator":1,"HealDenominator":4}` / `{"BoostStat":"atk","BoostStages":1}` / `{}`。
  値の中の未知のキーは不正。`HealNumerator` があるのに `HealDenominator` が無い(逆も)は不正、
  分母は 1..16、`BoostStat` は `hp` 不可、`BoostStages` は 1..6、`BoostStat` と `BoostStages` は組。
  **`{}`(副次効果なしの吸収)は正しい値**で、トップレベルの「効果が空」の禁止(ADR-0100 §6)は
  適用しない。
- 同じタイプが `DefImmuneTypes` と `DefAbsorbTypes` の両方にあるのは不正。
- 正準エンコードの順序は struct 定義順のまま、`DefResistType` の次・`ReduceSuperEffective` の前に
  `DefImmuneTypes` → `DefAbsorbTypes` を置く。配列はタイプ ID 昇順、map のキーも昇順。
  `Decode(Encode(e)) == e` はこれまでどおりテストで守る。

`data/importer/effects.json` の更新は**人が書く**(ADR-0101 §2: このファイルが本番の正)。
初期値として、ゴールデンで oracle と照合できた特性を英語 ID(`toID`)で足す:

```json
"levitate":     {"DefImmuneTypes": ["ground"]},
"waterabsorb":  {"DefAbsorbTypes": {"water":    {"HealNumerator": 1, "HealDenominator": 4}}},
"voltabsorb":   {"DefAbsorbTypes": {"electric": {"HealNumerator": 1, "HealDenominator": 4}}},
"eartheater":   {"DefAbsorbTypes": {"ground":   {"HealNumerator": 1, "HealDenominator": 4}}},
"flashfire":    {"DefAbsorbTypes": {"fire":     {}}},
"sapsipper":    {"DefAbsorbTypes": {"grass":    {"BoostStat": "atk", "BoostStages": 1}}},
"motordrive":   {"DefAbsorbTypes": {"electric": {"BoostStat": "spe", "BoostStages": 1}}},
"lightningrod": {"DefAbsorbTypes": {"electric": {"BoostStat": "spa", "BoostStages": 1}}}
```

**`data/importer/config.json` の `reconcile.effectHooks` に `onTryHit` と `onImmunity` を足す**。
今の一覧は倍率系のハンドラ(`onModifyAtk` 等)だけで、Showdown の無効・吸収の特性は
`onTryHit`(ちょすい・もらいび・そうしょく等)/ `onImmunity`(ふゆう)で実装されているため、
足さないと新しい定義がすべて `effect-no-hook` 警告になる(ADR-0103 §6)。

`services/pokedex/importer` の変換(`buildAbilityEffects`)は `master.DecodeAbilityEffect` →
`master.EncodeAbilityEffect` を呼ぶだけの汎用処理なので**変更不要**。

### 6. API 契約(`api/openapi.yaml`)は変えない

`MasterEffect` は `type: object` / `additionalProperties: true` で、形の正は
`services/internal/master` の Decode だと description に書いてある。新しいフィールドはその範囲に収まる
ので schema の変更は無く、`make gen` も不要。`CalcResult` に `nullified` を足すかは §4 のとおり別件。

WASM の**入力** DTO(`engine/wasmapi/dto.go` の `abilityEffectDTO`)には足す。足さないと、
ブラウザ(WASM)だけ無効・吸収が落ちて **HTTP と WASM でダメージが食い違う**(ADR-0011 の
「同じ入力で同じ結果」が壊れる)。JSON キーは既存にならって lowerCamel:
`defImmuneTypes: string[]` / `defAbsorbTypes: {[type]: {healNumerator?, healDenominator?, boostStat?, boostStages?}}`。
`web/src/engine/types.ts` の `AbilityEffect` も同じ形にする(Web レーンへの依頼。§他レーン)。

### 7. balance 向け export(`services/pokedex/internal/readmodel`)

`effectEntry` に **新しい Kind は要らない**。ADR-0017 §2 の `immune` / `absorb` をそのまま使う。

- `DefImmuneTypes` の各タイプ → `{"kind":"immune","attackType":t}`
- `DefAbsorbTypes` の各タイプ → `{"kind":"absorb","attackType":t}`(副次効果は出さない。ADR-0017 §2 が
  「回復・能力上昇などの副次効果は扱わない」と決めている)
- **並び順**: `immune`(タイプ ID 昇順)→ `absorb`(タイプ ID 昇順)→ `type_multiplier`(タイプ ID 昇順)
  → `super_effective_multiplier`。無効・吸収を先にするのは ADR-0017 §4 の計算順(無効が先に確定する)と
  同じ読み順にするため。
- **`effectEntry.Numerator` / `Denominator` に `omitempty` を付ける**。今は付いていないので、
  `immune` / `absorb` の行が `"numerator":0,"denominator":0` になり、
  `abilities.schema.json`(`factor` の `minimum: 1`、immune/absorb では `numerator` の存在自体が不正)に
  落ちる。既存の `type_multiplier` / `super_effective_multiplier` は分子・分母とも 1 以上なので
  `omitempty` を付けても出力は変わらない。

balance 側(schema の enum・loader・集計)は対応済みで、**依頼は無い**。

### 8. ゴールデン(oracle との照合)

`testdata/golden/effects.json`(ADR-0005 の形。生成器が読む)に §5 と同じ8件を
@smogon/calc の表示名で足す。8件とも Champions 世代に在るので `legacyEffects` の導出
(generate.mjs L38〜43 の assert)は変わらない。

`tools/golden/generate.mjs` は **`championsFixed` に専用のベクタだけを足す**。
Champions の `randomCases` の特性プールは**変えない**。プールを変えると乱数列が動いて
`random.jsonl.gz` の 10,000 件すべての期待値が変わり、「無効以外の計算は何も変わっていない」ことを
示せなくなるため。各特性について次の3種を置く:

1. **無効**: その特性が無効にするタイプの技 → `rolls` が全部 0
2. **対照**: 同じ防御側に別タイプの技 → 今までどおりのダメージ(過剰適用の検出)
3. **組合せ**: 無効のケースに急所・壁・天候・ランクを乗せる → それでも 0

ベクタの `id` は特性名の slug(`levitate` / `water-absorb` / `lightning-rod` …)を含める。
無効にする技は既存の代表技(`moveNames`)から選ぶ。対応(oracle の実装で確認済み。§oracle 2):

| 特性 | 無効の技 | 対照の技 |
|---|---|---|
| Levitate / Earth Eater | Earth Power(特殊)・Drill Run(物理) | Flamethrower |
| Water Absorb | Surf | Body Slam |
| Volt Absorb / Motor Drive / Lightning Rod | Thunderbolt | Body Slam |
| Flash Fire | Flamethrower | Body Slam |
| Sap Sipper | Energy Ball | Body Slam |

攻撃側・防御側は既存の Champions 種族(Goodra / Blastoise / Typhlosion / Raichu / Venusaur /
Snorlax など、`fixedPairs` で使っている種族)から選ぶ。フィールドを使うケースは飛行タイプを
避ける(既存の assert のとおり)。

結果として `fixed.json` の件数と sha256 だけが変わり、`random.jsonl.gz` /
`attack-species.jsonl.gz` / `defense-species.jsonl.gz` / `stats-species.jsonl.gz` /
`legacy-effects.jsonl.gz` / `typechart.json` は**バイト単位で変わらない**(テストで固定する)。
`koCrossCheck` は特性の付いたケースを除外するので `koCrossChecks` の数も変わらない。

`known_diffs.yaml` には**何も足さない**(全件一致する見込み)。

## 限界(ADR に残す)

1. **Dry Skin(かんそうはだ)は入れない**。水を吸収するのと同時に炎技の**威力**を ×1.25 する
   (champions.ts L807 の `bpMods.push(5120)`)。今の `AbilityEffect` には「防御側の特性が攻撃の
   **威力**を上げる」フィールドが無く、`DefResistType`(攻撃実数値の補正)では位置が違って
   ゴールデンが一致しない。既定案: 後続で `DefPowerType map[Type]int`(威力段階)を足す。
2. **Storm Drain(よびみず)は Champions 世代に無い**(§oracle 3)。gen9 の legacy-effects へ回す案も
   あるが、無効の判定は Champions と gen9 で同じコードなので追加の検証価値が薄い。入れない。
3. **技のフラグ由来の無効は扱わない**(ぼうだん=bullet フラグ、ぼうおん=sound フラグ、
   じょおうのいげん=優先度)。`Move` に flags が無く、タイプの効果でもない。ADR-0017 §7 の
   「足りない特性は扱わない」と同じ扱い。
4. **じゅうなん・ふゆう とじゅうりょく/でんじふゆう の相互作用は扱わない**(`field.isGravity` に
   相当する入力が無い)。`Field` に `Gravity` を足すのは別タスク。
5. **もらいび の発動後の炎技 ×1.5 は扱わない**(§決定 2)。
6. **副次効果(回復・能力上昇)は計算結果に出ない**(§決定 4)。ADR-0017 §2 の balance と同じ立場。

## 人間の確認事項(既定案で進める)

1. **`data/importer/effects.json` に足す8件の内容**(どの特性がどのタイプを無効/吸収し、回復量・
   上昇量がいくつか)。既定案: §5 の表のまま(ゲームの一般的な仕様。oracle は 0 ダメージだけを裏付ける)。
2. **`data/importer/config.json` の `effectHooks` に `onTryHit` / `onImmunity` を足すこと**。
   `onTryHit` は無効・吸収以外の特性(へんげんじざい等)も持つため、`effect-no-hook` の
   検出力がわずかに下がる。既定案: 足す(足さないと新しい定義が全部警告になる方が困る)。
3. **`CalcResult` に `nullified` を出すか**(API レーン)。既定案: 出さない(§決定 4)。画面で
   「ふゆう により無効」と出したくなったら API レーンへ依頼する。

## 他レーンへの依頼(メインが DECISIONS.md に書く)

- **Web レーン**: (a) `web/src/engine/types.ts` の `AbilityEffect` に `defImmuneTypes` /
  `defAbsorbTypes` を足す(WASM の入力 DTO と同じ形。足さないと WASM へ渡す時点で落ちる)、
  (b) `web/src/master/exportBalanceReadModel.ts` の `BalanceAbilityEffect` に `absorb` を足し、
  `toBalanceAbilityEffects` が §7 と同じ順序で `immune` / `absorb` を出すようにする。
- **タイプバランスレーン**: 依頼なし(schema・loader とも対応済み)。export に `immune` / `absorb` が
  出るようになるので、TB3/TB5 の結果が変わることだけ共有する。
- **API(calc)レーン**: `services/calc/internal/master/master.go` の `copyAbilityEffect` に
  `DefImmuneTypes`(slice)・`DefAbsorbTypes`(map)のコピーを足す(現状は `DefResistType` しか
  ディープコピーしておらず、`engine.AbilityEffect` に増えた2つの参照型フィールドが共有マスタと
  同じメモリを指したままになる)。`TestLookupReturnsCopiesOfEffects` にも両フィールドを書き換えて
  共有マスタが変わらないことを確かめるケースを足す(現状は `DefResistType` しか見ておらず、この穴を
  検出できない)。critic(P2-3b の独立レビュー)の必須指摘。

## 却下した案

- **`otherModifiers` に倍率 0 を足す**: chainMods の丸め経路に 0 を混ぜることになり、
  最低1ダメージの床と矛盾する。oracle も早期 return。
- **`DefResistType` に 0 を書けるようにする**: `decodePositiveInt` が 1 以上を要求しており、
  「倍率」と「無効」を同じ列で表すと `reduceRatio`(1..16 の比)も壊れる。export で
  `immune` と `type_multiplier` を区別できなくなる。
- **新しいテーブル `ability_immunities` を足す**: JSON 列にそのまま入る(ADR-0100 §6 が
  「効果の種類が増えるたびに列や行を足すのは重い」として JSON 列を選んだ理由そのもの)。
- **1つの `map[Type]Absorption` にまとめる**: 「ゼロ値が無効か吸収か」が読み手に分からず、
  export の写像に判定が入る。
- **Champions の random の特性プールに足す**: 10,000 件すべての期待値が動き、
  「無効以外は何も変わっていない」ことを示せなくなる(§決定 8)。
- **吸収の副次効果を `DamageResult` に返す**: 現在HP・現在のランクを受け取らない計算機では
  結果を出せない(§決定 4)。
