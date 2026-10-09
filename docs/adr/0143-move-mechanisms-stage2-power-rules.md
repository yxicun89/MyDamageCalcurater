# ADR-0143: 技の機構の段階2 — 既存の入力だけで決まる「威力の式・条件・タイプ・相性・優先度・壁」を閉じた語彙にし、種族の重さを取り込む

- 状態: 採用(実装済み。実データの dry-run で moveRules 36 行・重さの食い違い 0・印が残る技 67 → 31 を確認。2026-10-09)
- 日付: 2026-10-09
- レーン: データ(取り込み・DB・共通マスタ・効果定義)+ engine(計算)+ API 契約(省略可のキーの追加)
- 関連: ADR-0121(技の機構のデータ)、ADR-0123(未対応の印)、ADR-0142(段階1)、ADR-0178(技のフラグ)、
  ADR-0139(素早さの補正の語彙。SpeedCondition と同じ流儀で閉じた語彙にする)、ADR-0116(接地)、ADR-0118(effects.json 2つの一致)、
  ADR-0175(メガストーン)、ADR-0222(ダブルの全体技)、ADR-0223(内部 API の入れ替え順序)、ADR-0124(適用済みの migration を書き換えない)、
  ADR-0807(生成物は Git に置かない)

## 背景

ADR-0142 で取得元のフィールドだけで中身が決まる 7 機構を計算に入れた。残る機構(`variable_power`・`move_specific`・
`type_change`・`effectiveness_change`・`field_specific`・`priority_change`・`damageCallback` 由来の `fixed_damage`)は
Showdown では技ごとの関数(ハンドラ)で、取得物には**ハンドラの名前しか出ない**。値を機械的に導けないので、
「威力の式の語彙」を engine の閉じた型にし、技 → 語彙の対応は手書きのデータ(effects.json の新しい節)で持つ。

## 測定(2026-10-09。実データ: ピン留めした Showdown champions mod と @smogon/calc 0.12.0 Champions 世代。技の ID は書かない)

使用可能な攻撃技(Showdown の isNonstandard が null)335 のうち、段階1の後もなお未対応の印が付く技は **67**。機構別(重複あり):

| 機構 | 技数 | 段階2で対象 | 段階3に残す |
|---|---|---|---|
| variable_power | 41 | 21 | 20 |
| move_specific | 20 | 15 | 5 |
| fixed_damage(`damageCallback` 由来) | 7 | 0 | 7 |
| type_change | 4 | 2 | 2 |
| effectiveness_change | 2 | 2 | 0 |
| field_specific | 2 | 2 | 0 |
| priority_change | 1 | 1 | 0 |
| (技数) | **67** | **36** | **31** |

威力 0 で登録された技(`zero_power` の印)は 19。うち段階2で 6 が外れる(重さ 4・素早さ比 2。威力の式を持つもの)。

### 段階2で対象にする 36 技(語彙ごとの件数)

既存の入力(両側の個体・状態異常・持ち物・特性・ランク・場の天候/フィールド/壁)と、新しく取り込む**種族の重さ**だけで決まるもの。

| 何で決まるか | 語彙 | 技数 |
|---|---|---|
| 攻撃側/防御側の状態異常 | `PowerBoosts` の `attacker_status`・`defender_status`(+ からげんきの `IgnoresBurn`) | 5 |
| 攻撃側の持ち物の有無・防御側の持ち物 | `attacker_no_item`・`defender_item_removable`・`FailsWithoutDefenderItem` | 3 |
| 天候 | `weather`(+ `TypeByWeather`) | 3 |
| フィールドと接地 | `terrain_attacker_grounded`・`terrain_defender_grounded`(+ `TypeByTerrain`・`SpreadInTerrain`)、`TerrainPowerMods`、`PriorityBoost` | 7 |
| 攻撃側のランク | `PowerFormula: attacker_positive_boosts` | 2 |
| 両側の素早さ(実数値・ランク・素早さの補正・まひ) | `speed_ratio`・`inverse_speed_ratio` | 2 |
| 2タイプの相性・特定タイプへの相性 | `ExtraEffectivenessType`・`SuperEffectiveAgainst` | 2 |
| 壁を壊す | `BreaksScreens` | 2 |
| 何発目か | `hit_index`(多段の中身と組) | 1 |
| ハンドラがダメージに効かない(命中・味方への処理・ダイマックス等) | `MoveSpecificResolved` だけ | 5 |
| 種族の重さ(+ 特性の重さの補正) | `target_weight`・`weight_ratio` | 4 |

### 段階3に残す 31 技(理由ごとの件数)

| 理由 | 技数 | 要るもの |
|---|---|---|
| 残り HP(攻撃側・防御側) | 8 | 現在 HP の入力(公開 API・Web・iOS) |
| 受けたダメージ・倒れた味方・手持ち・たくわえた回数・行動の順と履歴 | 15 | 対戦の状態の入力(画面で選ぶ) |
| 持ち物ごとの威力の表(なげつける) | 1 | 持ち物のデータ |
| 場の状態のうち engine に無いもの(じゅうりょく) | 1 | 場の入力 |
| 乱数(確率で威力が変わる) | 1 | 確率の表示の設計 |
| フォルム依存(タイプ・威力が種族のフォルムで変わる。使用不可のフォルムだけで効くものを含む) | 3 | 種族のフォルムの語彙 |
| 分類の切り替え(攻撃と特攻の比較) | 1 | 語彙の追加 |
| タイプなし(わるあがき) | 1 | 選べない技。対象外のままでよい |
| 攻撃側のくろいてっきゅうによる接地(ADR-0116 からの穴。engine は持ち物による接地を持たず、テラバースト型・ワイドフォース・ミストバーストの攻撃側の接地判定に影響する) | — | 持ち物による接地の語彙(段階3) |
| 多段の回数を利用者が選ぶ入力(公開 API `CalcRequest`・Web・iOS) | — | 段階1の既定(oracle と同じ 最小+1 / スキルリンクで最大)で誤った値は出ない。UI の設計とともに段階3 |

## oracle(@smogon/calc 0.12.0 champions.js / util.js)で確かめた挙動

- **基本威力の式**(`calculateBasePowerChampions` の switch。整数): ジャイロボール `min(150, floor(25 × 防御側の素早さ / 攻撃側の素早さ) + 1)`
  (攻撃側 0 は 1)、エレキボール `r = floor(攻撃側 / 防御側)` で `≥4:150, ≥3:120, ≥2:80, ≥1:60, else 40`(防御側 0 は 40)、
  けたぐり型 `重さ(kg) ≥200:120, ≥100:100, ≥50:80, ≥25:60, ≥10:40, else 20`、ヘビーボンバー型 `比 ≥5:120, ≥4:100, ≥3:80, ≥2:60, else 40`、
  アシストパワー型 `20 + 20 × Σ(攻撃側の正のランク atk..spe)`、たたりめ型 `威力 × (防御側に状態異常 ? 2 : 1)`、
  ベノムトラップ型の2倍は状態が毒・猛毒、アクロバット `威力 × (攻撃側が持ち物なし ? 2 : 1)`、ウェザーボール `威力 × (天候あり ? 2 : 1)`、
  ライジングボルト `威力 × (防御側が接地 かつ エレキフィールド ? 2 : 1)`、ワイドフォース等は下の連鎖、トリプルアクセル `h 発目 = h × 20`。
  素早さは `computeFinalStats` の `getFinalSpeed`(ランク → [おいかぜ・特性・持ち物] の chainMods(410..131172) → pokeRound → まひで ×50/100 floor → 0..10000)。
  重さは `getWeight`(hg で `heavymetal ×2`・`lightmetal ×0.5` を trunc・最小 1、`floatstone` は Champions で使用不可)。
- **威力の補正の連鎖**(`calculateBPModsChampions`。4096 基準): **最初**に技の群(からげんき・ベノムトラップ 8192 / はたきおとす・ミストバースト 6144 /
  ワイドフォース 6144 / ソーラービーム・ソーラーブレード(雨・砂・雪)2048。else-if で1つだけ)→ てだすけ → 攻撃側の接地のフィールド 5325 →
  **防御側の接地のフィールド**(ミストのドラゴン 2048・**グラスフィールドのじしん・じならし 2048**)→ 特性 … → 持ち物。
- **タイプ**: ウェザーボール(晴れ fire・雨 water・砂 rock・雪 ice)、ライジングボルトは変えない、テラバースト型のフィールドの技は
  攻撃側が接地のときだけ(エレキ electric・グラス grass・ミスト fairy・サイコ psychic)。どちらも `noTypeChange`(スキン系の特性で変えない)。
- **相性**(`getMoveEffectiveness`): フリーズドライは水タイプへの相性を 2 にする、フライングプレスは各タイプの相性に飛行の相性を掛ける。
- **壁**: かわらわり・サイコファングは計算の前に防御側の壁(リフレクター・ひかりのかべ・オーロラベール)を外す。
- **失敗**: ポルターガイストは防御側が持ち物なしなら 0(タイプ相性の無効の後・特性の無効の前)。
- **はたきおとす**: 防御側が持ち物を持ち、それがその防御側のメガストーンでなければ 6144(`item.megaStone` が防御側の名前を含むと補正なし)。
- **からげんき**: 攻撃側がやけど・まひ・毒・猛毒なら 8192、**やけどの攻撃半減を受けない**(`applyBurn` が技名で除く)。こおりは含まない(Showdown は含む。正は oracle)。
- **差が出ない技**: ふぶき・かみなり・ぼうふう(命中だけ)、さわぐ(眠りを覚ます)、かふんだんご(味方への処理)、けたぐり等の onTryHit(ダイマックス)は
  oracle に分岐が無く、通常の式と同じ。
- **浮動小数の誤差**: oracle のヘビーボンバー型は `kg / kg` を浮動小数で割る(`6.9 / 2.3 = 2.9999…` が 3 未満になる)。実機(Showdown)は
  hg の整数で `攻撃側 >= 防御側 × k` を比べる。engine は整数(実機)に合わせ、ゴールデンは**比がちょうど整数になる組を使わない**(生成器が止める)。

## 決定

### 1. 語彙(engine の閉じた型。値の正は engine。`engine/move_rule.go`)

```go
type PowerFormula string // "" はなし
const (
	PowerFormulaPositiveBoosts    PowerFormula = "attacker_positive_boosts" // 威力 × (1 + 攻撃側の正のランクの合計)
	PowerFormulaSpeedRatio        PowerFormula = "speed_ratio"              // エレキボール型の表
	PowerFormulaInverseSpeedRatio PowerFormula = "inverse_speed_ratio"      // ジャイロボール型の式
	PowerFormulaTargetWeight      PowerFormula = "target_weight"            // けたぐり型の表(防御側の重さ)
	PowerFormulaWeightRatio       PowerFormula = "weight_ratio"             // ヘビーボンバー型の表(攻撃側 / 防御側)
	PowerFormulaHitIndex          PowerFormula = "hit_index"                // h 発目の威力 = 威力 × h(多段の中身が要る)
)
type MoveCondition string
const (
	MoveConditionAttackerStatus          MoveCondition = "attacker_status"           // Statuses のどれか
	MoveConditionDefenderStatus          MoveCondition = "defender_status"           // Statuses のどれか
	MoveConditionAttackerNoItem          MoveCondition = "attacker_no_item"
	MoveConditionDefenderItemRemovable   MoveCondition = "defender_item_removable"   // 持ち物あり かつ メガストーンでない
	MoveConditionWeather                 MoveCondition = "weather"                   // Weathers のどれか
	MoveConditionTerrainAttackerGrounded MoveCondition = "terrain_attacker_grounded" // Terrains のどれか かつ 攻撃側が接地
	MoveConditionTerrainDefenderGrounded MoveCondition = "terrain_defender_grounded" // Terrains のどれか かつ 防御側が接地
)
type MovePowerBoost struct {
	Condition      MoveCondition
	Statuses       []Status  // *_status だけ。空不可
	Weathers       []Weather // weather だけ。空不可・none 不可
	Terrains       []Terrain // terrain_* だけ。空不可・none 不可
	BaseMultiplier int       // 威力を整数倍(2 以上)。Modifier と排他
	Modifier       int       // 4096 基準(4096 不可)。威力の補正の連鎖の最初
}
type TerrainPowerMod struct { Terrain Terrain; Modifier int } // 防御側が接地しているとき。連鎖のフィールドの防御側の位置
type PriorityBoost struct { Terrain Terrain; Delta int }       // 攻撃側が接地 かつ そのフィールドで優先度 + Delta(Delta ≠ 0)
type MoveRule struct {
	PowerFormula             PowerFormula
	PowerBoosts              []MovePowerBoost
	IgnoresBurn              bool             // やけどの攻撃半減を受けない(からげんき)
	TerrainPowerMods         []TerrainPowerMod
	TypeByWeather            map[Weather]Type
	TypeByTerrain            map[Terrain]Type // 攻撃側が接地のときだけ
	ExtraEffectivenessType   Type             // 各タイプの相性にこのタイプの相性も掛ける
	SuperEffectiveAgainst    []Type           // このタイプへの相性を 2 にする
	PriorityBoost            *PriorityBoost
	BreaksScreens            bool
	FailsWithoutDefenderItem bool             // 防御側が持ち物なしなら 0(Nullified = move_failed)
	SpreadInTerrain          Terrain          // そのフィールドで攻撃側が接地なら全体技(ダブルの補正)
	MoveSpecificResolved     bool             // move_specific のハンドラは上の中身以外にダメージへ効かない(oracle で確かめた)
}
// Move.Rule *MoveRule(nil は定義なし)。Species.WeightHg int(hg。0 は不明)。Item.MegaStone bool。
// AbilityEffect.WeightMod int(4096 基準。重さ = max(1, trunc(hg × WeightMod / 4096)))。値域は 1..32 倍(`engine.MaxWeightModifier` = 32 × 4096)。
// NullifyMoveFailed NullifyKind = "move_failed"。ErrInvalidMoveRule。
// AllPowerFormulas()・AllMoveConditions()(定義順のコピー)・Known()。
```

- 表(素早さ比・重さの段)と式の定数はゲームの規則なので engine に持つ(フィールドの補正値と同じ扱い)。技の一覧は持たない。
- 素早さ(`finalSpeed`): ランク → [特性の `SpeedMods`(成立した最初の1つ)・持ち物の `SpeedMods`(同)] の chainMods → pokeRound →
  まひ(特性の `IgnoresParalysisSpeedDrop` が無いとき ×50/100 floor)→ 0..10000。条件 `item_lost` は engine の入力に無いので常に不成立
  (oracle の `abilityOn` の既定と同じ)。おいかぜは engine の入力に無い(oracle も既定なし)。ADR-0139 の「ダメージ計算は読まない」を、
  素早さ比の式に限って改める。
- **検証**(`CalcDamage` が `ErrInvalidMoveRule` を包んで返す。`Move.ValidateRule(chart)`): 変化技の定義、語彙に無い値、空の定義、
  中身と機構の対応違反(威力の中身 = `PowerFormula`・`PowerBoosts`・`IgnoresBurn` は `variable_power` か `move_specific`、
  `TerrainPowerMods` は `field_specific`、`TypeBy*` は `type_change`、相性は `effectiveness_change`、`PriorityBoost` は `priority_change`、
  `BreaksScreens`・`FailsWithoutDefenderItem`・`SpreadInTerrain`・`MoveSpecificResolved` は `move_specific`)、`hit_index` で `Params.MultiHit` が無い、
  条件と一覧の組の誤り、`BaseMultiplier` と `Modifier` の両方/どちらも無い、補正値の範囲外、`Delta` 0。タイプが相性表に無ければ `ErrUnknownType`。

### 2. 計算の順(oracle と同じ)

1. 前処理(特性の段階1の後): `PriorityBoost` を優先度に足す(サイコフィールドの判定に使う。グラスフィールドとサイコフィールドは同時に無いので、グラススライダー型では結果は変わらない。優先度が上がると、じょおうのいげん・テイルアーマーを持つ防御側には当たらなくなる(engine は優先度による無効を表せないが、この2特性は effects.json で `UnsupportedDefender` の印が付いているので、黙って誤らず印で守られる)。
   `TypeByWeather`(場の天候)・`TypeByTerrain`(攻撃側が接地)で技のタイプを変える(type_change の機構を持つのでスキン系では変えない。既存)。
   `SpreadInTerrain` が成立すれば `Target = spread`。
2. 相性: `SuperEffectiveAgainst` のタイプは 2、`ExtraEffectivenessType` は各タイプで掛ける。
3. 無効: タイプ相性 → **`FailsWithoutDefenderItem`(`move_failed`)** → 特性 → サイコフィールド → 一撃必殺・固定ダメージ(既存)。
4. 威力: `PowerFormula` で基本威力を決め(威力 0 の技もここで決まる)、`BaseMultiplier` の成立した条件を掛ける(整数)。テクニシャン等の
   条件はこの基本威力で判定する(oracle の `basePower`)。威力の補正の連鎖は **[成立した `Modifier` …] → 既存のフィールド(攻撃側)→
   `TerrainPowerMods`(防御側が接地)→ 既存の特性・オーラ・持ち物**。
5. 壁: `BreaksScreens` なら壁の補正を掛けない。やけど: `IgnoresBurn` なら半減しない。
6. 多段: `hit_index` は h 発目を威力 `Power × h` で計算する(1発ごとに計算するので `HitRolls` が発ごとに違う。確定数は既存の
   `computeKOHits` がそのまま扱う)。他の多段は従来どおり全発同じ。

### 3. 印の扱い(ADR-0142 §10 の差し替え。`Move.Rule` が nil のときは従来どおり)

| 機構 | 印を外す条件 |
|---|---|
| variable_power | 威力の中身(`PowerFormula`・`PowerBoosts`)があり、この入力で計算できる(重さの式で要る側の `WeightHg` が 0 なら不可。`defender_item_removable` で防御側がメガストーンを持つなら不可 — 防御側のメガストーンかを engine は知らないので安全側) |
| zero_power | `PowerFormula` があり、同じく計算できる |
| move_specific | `MoveSpecificResolved` |
| type_change | `TypeByWeather` か `TypeByTerrain` |
| effectiveness_change | `ExtraEffectivenessType` か `SuperEffectiveAgainst` |
| field_specific | `TerrainPowerMods` |
| priority_change | `PriorityBoost` |

計算できない(印を残す)ときの数値は従来どおり(式を使わない通常の式)。

### 4. データ: effects.json の新しい節 `moveRules`

- 正は `data/importer/effects.json` の `moveRules`(キーは Showdown の技 ID、値は §1 の `MoveRule` の JSON。キーは Go のフィールド名、
  真偽は `true` だけを書く。effects の流儀)。写しは `testdata/golden/effects.json` の `moveRules`(キーは @smogon/calc の英語名)。
  `TestGoldenEffectsMatchImporterEffects` と同じ方法(toID で正規化)で一致を確かめる(ADR-0118)。あわせてゴールデンの写しに
  `speedItems`・`speedAbilities` の節を足す(素早さ比の照合に使う。キーは英語名。一致を同じ方法で確かめる)。
- 特性の効果: `heavymetal {"WeightMod":8192,"Breakable":true}`・`lightmetal {"WeightMod":2048,"Breakable":true}`
  (oracle の `defenderAbilityIgnored` に両方ある)。
- 共通マスタ: `master.DecodeMoveRule(raw, chart)`(厳格。未知のキー・空・`false`・語彙外は `ErrInvalidEffect`)・`master.EncodeMoveRule`(正準形)。
  `MoveRow.Rule []byte` を `master.Move` が検証し(機構との対応は `engine.Move.ValidateRule`。違反は `ErrInvalidRow`)、`engine.Move.Rule` に写す。
  `master.Species`: `SpeciesRow.WeightHg`(0 は不明)を `engine.Species.WeightHg` に写す。
- importer: `EffectsFile.MoveRules`。moves 表に採る攻撃技のキーは `Output.MoveRules []EffectRow`(正準形)。採らない技のキーは警告
  `effect-unused`(持ち物と同じ)。変化技・検証の失敗は `ErrInvalidData`。照合の要約に `moveRules: <行数>`。
  変換結果の版(ADR-0122)は `MoveRules` と種族の重さを含む。
- 種族の重さ: `tools/importer/fetch-showdown.mjs` が種族ごとに `weightkg`(取得元の数値のまま)を、`fetch-calc.mjs` も `weightkg` を出す。
  importer は両方を**必須**としてデコードし(無い古い取得物は `ErrInvalidInput`。`make import-fetch` で取り直す)、小数1桁までの10進として
  **浮動小数を経ずに** hg の整数にする(`6.9` → 69。小数2桁以上・0 以下は `ErrInvalidData`)。両方にある種族で値が違えば Blocker
  `species-mismatch`(Detail `weightkg`)。

### 5. DB

- migration **000015** `move_rules`(`move_id` 主キー・`moves` への外部キー ON DELETE CASCADE・`rule JSON NOT NULL`・
  `CHECK (JSON_TYPE(rule) = 'OBJECT')`。`item_effects` と同じ形)。down は DROP TABLE。sqlc: `ListMoveRules`・`DeleteMoveRules`・`InsertMoveRule`。
- migration **000016** `species.weight_hg SMALLINT UNSIGNED NULL`・`CHECK (weight_hg IS NULL OR weight_hg > 0)`(ADR-0136 の
  `moves.target` と同じ「列を足すだけ」)。down は CHECK と列を落とす。species の投入・一覧のクエリに列を足す。
- origin/main の最新は 000014(2026-10-09 時点)。**マージ前に main の最新の版を確かめ、先に別の 000015/000016 が入っていたら番号を繰り下げる**
  (layout テストの定数も合わせる)。

### 6. 契約の差分(`api/openapi.yaml`。どれも省略可のキーの追加)

内部 API(`GET /internal/pokedex/master`):
- `MasterMove.rule`: `MasterEffect` と同じ不透明なオブジェクト・**省略可**。定義の無い技はキーを省く(生成型が `omitempty` のため null を書かない)。
  calc-svc はキーが無い・null を「定義なし」として扱う(古い pokedex-svc と入れ替えの順序に依存しない)。
- `MasterSpecies.weightHg`: integer・**省略可**。NULL(取り込み前)はキーを省く。calc-svc は省略を 0(不明)にする。

公開 API(Web のオフライン計算に届けるため):
- `Move.mechanismParams`(`MasterMoveMechanismParams` と同じ形。中身が無ければ**キーごと省く**)と `Move.rule`(同じ不透明なオブジェクト。無ければ省く)。
- `SpeciesDetail.weightHg`(省略可。不明なら省く)。`Item.isMegaStone` は既存。

calc-svc は `Item.MegaStone` を、取り込んだ種族の `requiredItemId` に現れる持ち物として導く(ADR-0175 と同じ定義。契約の変更なし)。

WASM(Web 内部の境界): 技の `rule`(キーは camelCase。Web が公開 API の PascalCase のまま渡しても受ける)、種族の `weightHg`、持ち物の `isMegaStone`、
特性の効果の `weightMod`。語彙に無い値は `invalid_enum`、値域・機構との対応の誤りは `invalid_input`、未知のキーは `unknown_field`。

**公開 API の入力(多段の回数・現在 HP 等)は変えない**(段階3)。

### 7. ゴールデン(`testdata/golden/mechanisms-stage2.json`。Champions 世代・SP そのまま)

生成器は `effects.json` の `moveRules` の各技について、条件の成立/不成立の対(apply/control)を作り、oracle の結果を期待値にする。
既存のファイル(fixed・mechanisms・effects 等)のバイト列・件数は変えない。ベクタは種族の `WeightHg`(oracle の `weightkg × 10`)と
持ち物・特性の効果(素早さの補正を含む)を入力に載せる。最低限のラベル:

| ラベル | 内容 |
|---|---|
| `status-attacker/*`・`status-defender/*`・`status-none/*` | からげんき(やけど・まひ)・たたりめ型・毒の2倍・状態なし |
| `item-none/*`・`item-held/*`・`item-removable/*`・`item-failed/*` | アクロバット・はたきおとす・ポルターガイスト |
| `weather/*` | ウェザーボールの4天候・天候なし・ソーラービームの雨 |
| `terrain-attacker/*`・`terrain-defender/*`・`terrain-airborne/*` | テラ型のフィールドの技・ミストバースト・ワイドフォース・ライジングボルト・グラスのじしん・浮いている側 |
| `terrain-spread/*` | ダブルのサイコフィールドのワイドフォース |
| `priority-terrain/*` | グラススライダー(サイコフィールドで当たる) |
| `ranks/*` | アシストパワー型(正負のランクの混在) |
| `speed-ratio/*`・`speed-inverse/*`・`speed-paralysis/*`・`speed-ability/*`・`speed-item/*` | エレキボール・ジャイロボール・まひ・すいすい等・くろいてっきゅう |
| `effectiveness/*` | フライングプレス・フリーズドライ(水・水複合) |
| `screens-broken/*` | かわらわり × リフレクター・オーロラベール |
| `hit-index/*` | トリプルアクセル(テクニシャンを含む) |
| `resolved/*` | ハンドラがダメージに効かない技 |
| `weight-target/*`・`weight-ratio/*`・`weight-ability/*` | けたぐり型・ヘビーボンバー型・ヘヴィメタル/ライトメタル(かたやぶりを含む) |

生成器は (a) `moveRules` の全技に1件以上のベクタ、(b) 重さの比がちょうど整数(2〜5)になる組を使わない、(c) apply と control で oracle の
結果が違う(条件が効く)ことを確かめ、崩れたら止まる。`metadata.json` の `exclusions` の技の理由を「段階2の技は mechanisms-stage2.json で照合」に改める。
engine の golden テスト `TestGoldenMechanismsStage2` が、印が残らないこと・rolls・hitRolls・確定数・ラベルの網羅を確かめる。

### 8. デプロイ順

**migrate(000015・000016)→ アプリ(pokedex-svc・calc-svc・Web の WASM)→ `make import-fetch`(重さの取り直し)→ 取り込み → master-release**。

- 先に取り込むと、新しい節(`moveRules`)と効果のキー(`WeightMod`)を古い importer / calc-svc の厳格なデコードが拒否する。
- 新しいアプリ + 取り込み前の DB は、`move_rules` が空・`weight_hg` が NULL なので印が残るだけ(従来どおり。誤った数値を出さない)。
- `docs/ai-shared/decisions/2026-10-09-data-move-mechanisms-stage2.md` に同じ手順と、Web・iOS への依頼を書く。

### 9. PR の分け方

1 PR を基本とする。大きすぎる場合は **2a(重さ以外: 語彙・`moveRules`・000015・`rule` の契約)→ 2b(重さ: 取得・000016・`weightHg`・重さの式)**
に分けてよい(テストもファイルで分かれている。2a の段階では重さの式の技は `WeightHg` が 0 なので印が残るだけで壊れない)。

## 受け入れ条件

1. engine: §1 の語彙で定義した技を、等価な通常の入力(同じ威力・タイプ・壁なし 等)と同じ値で計算し、§3 の条件で印を外す。
   条件が不成立のときは通常の値。定義が無いときは従来どおりの印(既存のテストは不変)。不正な定義は `ErrInvalidMoveRule`。
2. engine: 素早さ比はランク・素早さの補正・まひを入れた素早さで、重さの式は `WeightHg` と `WeightMod`(かたやぶりで防御側を無視)で決まる。
   重さが不明なら印が残る。
3. ゴールデン `mechanisms-stage2.json` が全件一致し、§7 のラベルを網羅する。既存のゴールデンは全件そのまま一致する。
4. データ: `data/importer/effects.json` と写しの `moveRules`・`speedItems`・`speedAbilities` が一致し、本番の `moveRules` は 36 技で
   §測定の語彙ごとの件数どおり。どの定義も master のデコードと engine の検証を通る。
5. importer: `moveRules` を `Output.MoveRules` に正準形で出し、種族の重さを hg の整数で取り込み、食い違いを Blocker にする。
6. DB・pokedex-svc・calc-svc・WASM・Web: §5・§6 の形で `rule`・`weightHg`・`isMegaStone` が engine まで届き、古い本文(キーなし)も受け付ける。

## 実装の手順(implementer 向け。テストは spec-writer が先に置いた)

1. engine: `engine/move_rule.go`(§1 の型・`AllPowerFormulas`・`AllMoveConditions`・`Known`・`ValidateRule`)、`Move.Rule`・
   `Species.WeightHg`・`Item.MegaStone`・`AbilityEffect.WeightMod`(validate の値域)・`NullifyMoveFailed`。`calcDamageNoKO` に §2 の順で組み込み、
   `powerModifier` の連鎖の先頭・`TerrainPowerMods` の位置、`screenDamageMod`・`burnModifier`、多段の `hit_index` を1発ごとに計算。
   `finalSpeed`・`effectiveWeight` を足す。`moveMarks`・`mechanismHandled` を §3 に。一括・逆算・調整は CalcDamage の合成なので
   変更不要のはず(`hit_index` の多段の逆算の照合が発ごとに違う段を扱えることを確かめる)。
   テスト: `engine/move_rule_stage2_test.go`。
2. `services/internal/master`: `DecodeMoveRule`・`EncodeMoveRule`(キー・値の厳格な検証。effects と同じ流儀)、`MoveRow.Rule`、
   `SpeciesRow.WeightHg`、`effects.go` の特性のキーに `WeightMod`。テスト: `services/internal/master/move_rule_test.go`。
3. importer: `EffectsFile.MoveRules`(`DecodeEffectsFile` の節の許可)、`buildMoveRules`、`Output.MoveRules`、`Apply`(Delete → Insert。
   `move_mechanism_params` と同じ場所・`moves` より先に消す)、`storetest`、種族の `weightkg`(Showdown・calc)のデコード・変換・照合、
   `SpeciesRow.WeightHg`、要約、変換結果の版。fixture(`testdata/fictional`)の取得物は weightkg を持つ(spec で追加済み)。
   テスト: `convert_move_rules_test.go`・`source_of_truth_test.go`(写しの一致)。
4. DB: `migrations/000015_create_move_rules.{up,down}.sql`・`000016_add_species_weight.{up,down}.sql`・`query/pokedex.sql`・`make gen`。
   テスト: `services/pokedex/db/move_rules_layout_test.go`。
5. pokedex-svc: 内部 API の `rule`・`weightHg`、公開 API の `Move.mechanismParams`・`Move.rule`・`SpeciesDetail.weightHg`。storetest の fixture に
   テストが期待する値(テストの冒頭のコメント)を入れる。テスト: `services/pokedex/internal/httpapi/move_rules_test.go`。
6. calc-svc `internal/master/export.go`: `rule` → `MoveRow.Rule`、`weightHg` → `SpeciesRow.WeightHg`、`Item.MegaStone` の導出。
   テスト: `services/calc/internal/master/move_rule_test.go`。
7. wasmapi: `moveDTO.Rule`・`speciesDTO.WeightHg`・`itemDTO.IsMegaStone`・`abilityEffectDTO.WeightMod`。テスト: `engine/wasmapi/move_rule_stage2_test.go`。
   Go/WASM 一致のベクタ(`testdata/vectors.json`)に `rule` の1件を足す。
8. Web: `web/src/engine/types.ts`・`web/src/master/onlineSource.ts` が `mechanismParams`・`rule`・`weightHg`・`isMegaStone` を応答のまま写す
   (表示は変えない)。WASM に渡す持ち物にも `isMegaStone` を載せる(いまの WASM は未知のキーとして拒否するので、境界の DTO と同時に)。
   テスト: `web/src/master/onlineSource.moveRules.test.ts`。
9. effects.json 2つに `moveRules`・特性の `WeightMod`、写しに `speedItems`・`speedAbilities`。`tools/importer` の重さの取得。
   `tools/golden/generate.mjs` に §7(重さ・素早さの補正をベクタに載せる・特性の網羅調査に重さの技を足す・
   `effects/heavymetal|lightmetal/weight/...` の apply/control/breakable)。`engine/golden_effects_test.go` の型付きの効果の項目に `WeightMod` を足す。
   `make golden-generate` → `make test-golden`。
10. `make import-fetch` → 実データの dry-run で `moveRules: 36`・重さの食い違い 0・段階2の技の印が消えることを確かめ、件数をこの ADR に追記する。

## 結果

- 良い点: 対戦でよく使う重さ・素早さ比・状態・天候・フィールド依存の技の数値が正しくなり、印が 67 → 31 技に減る。技名を engine に持たない
  (語彙は閉じた型、技 → 語彙はデータ)。
- 注意: はたきおとすは防御側がメガストーンを持つとき印を残す(防御側の種族に合うメガストーンか engine が知らないため)。段階3で種族とメガストーンの対応を持たせれば外せる。
- 注意: からげんきのこおり・ヘビーボンバー型の境界は oracle と実機で違う。前者は oracle、後者は実機(整数)に合わせ、ゴールデンは境界を避ける。
- 実データ(2026-10-09。Showdown champions mod の pin と @smogon/calc 0.12.0 の取得物)での確認: `make import-fetch` 相当の取り直し(`tools/importer` の `node fetch.mjs`)→
  `go run ./pokedex/cmd/import -dry-run` が終了コード 0・`blockers: none`・`moveRules: 36`・重さの食い違い(`species-mismatch` の `weightkg`)0。
  取り込んだ全攻撃技を、フィールドなしの Snorlax × Garchomp で engine に通した「技の未対応の印」は、定義なし(従来)で 64 技(測定の 67 技のうち
  フィールドなしで印が付かないじしん・じならし・グラススライダーを除く)、定義あり(この変更)で **31 技**。残るのは段階3の技
  (残り HP・対戦の履歴・フォルム・持ち物の表・わるあがき 等)だけで、測定表の「段階3に残す 31 技」と一致する。
- 実装中に見つけた注意(ADR 本文の決定は変えない): (1) `finalSpeed` の補正の連鎖の範囲は oracle の getFinalSpeed どおり 410..131172
  (威力・実数値の範囲とは別)。(2) 特性の `WeightMod` は engine で 1..32 倍(`engine.MaxWeightModifier`)に絞る(他の補正の上限 512 倍では、
  1 回の掛け算が現実の重さを大きく超えるため)。(3) Showdown の取得物は使用不可の巨大化フォーム等に `weightkg: 0` を持つので、取得時は 0 以上を
  通し、取り込む種族の 0 だけを Go 側(`ErrInvalidData`)で止める。(4) メガストーンを防御側が持つときの `isMegaStone` は、Web が防御側の個体にだけ
  残す(攻撃側には要らず、既存の「要求の持ち物は engine の Item の形」の確認も変えない)。
- 種族の畳み込み(ADR-0101 §5)の「性能」の署名に重さ(hg)を含める: 重さだけ違う姿(実データの maushold 2.3kg と mausholdfour 2.8kg)を畳むと、重さで威力が決まる技の計算が黙って誤るため。実データの dry-run(exit 0・blockers: none)で取り込む種族は 349 → 350 件、`form-folded` は 34 → 33 件。
