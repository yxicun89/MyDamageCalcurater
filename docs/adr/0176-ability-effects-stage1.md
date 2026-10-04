# ADR-0176: 未対応の特性の段階1(engine と効果データだけで足りる系統)を計算に入れる

- 状態: 採用(2026-10-04 実装。ゴールデン全件一致)
- 日付: 2026-10-04
- レーン: データ(効果定義・共通マスタ)+ engine(計算)。契約は変えない
- 関連: ADR-0005(データ駆動の効果定義)、ADR-0101(effects.json が本番の正)、ADR-0106(特性の無効・吸収)、
  ADR-0118(効果定義のゴールデンの写し)、ADR-0120(効果データの網羅)、ADR-0121(技の機構)、ADR-0123(未対応の印)、
  ADR-0139(SpeedMods の Condition の形)、ADR-0224(テラスタル)

## 背景

2026-10-04 のユーザーの実使用で「ニンフィアのフェアリースキンが未対応」「パンクロックが未対応」と報告があった。
調査(事実): 特性 216 件のうち効果あり 17・攻撃側だけ未対応 32・防御側だけ未対応 16・両方未対応 3・効果なし 148。
最初の特性が未対応の種族が 94/348。さらに「効果なし」に分類されていたもののうち、かたやぶり(11 種族)・てんねん・
ひとでなし・えんかくは、相手の特性・ランク・状態と組み合わさってダメージを変える(oracle の調査が単独の持ち主しか
見ていなかったため漏れていた)。

技のフラグ(接触・音・パンチ等)は取り込みにも engine の `Move` にも無いので、フラグに依存する特性(パンクロック等)は段階2(別 PR)。
HP に依存する特性(もうか・げきりゅう・しんりょく・むしのしらせ・マルチスケイル)は「満タン前提」の判断が要るので段階3。
この ADR は、**engine と効果データだけで足りて、oracle(@smogon/calc 0.12.0 の Champions 世代)で照合できるもの**を扱う。

## 決定

### 1. AbilityEffect の新しい項目(engine/modifiers.go。どれもゼロ値は「その効果なし」)

| 項目 | 側 | 意味 | oracle の位置 | 使う特性 |
|---|---|---|---|---|
| `TypeConvert *TypeConvert{From, To, PowerMod}` | 攻撃 | From タイプの技を To にし、威力に PowerMod | 技のタイプ決定の直後。威力補正はオーラの後・持ち物の前 | pixilate / refrigerate / aerilate / dragonize(normal→X, 4915) |
| `PowerMods []ConditionalPowerMod{Condition, MaxPower, MoveType, Modifier}` | 攻撃 | 条件つきの威力補正 | フィールドの後・オーラの前 | technician(max_base_power 60, 6144)/ steelyspirit(move_type steel, 6144) |
| `AuraType` + `AuraMod` | 両側 | どちらが持っていても、そのタイプの技の威力に1回 | PowerMods の後・タイプ変換の補正の前 | fairyaura(fairy, 5448) |
| `StatMods map[StatKey]int` | 両側 | 持ち物の StatMods と同じ読み方(攻撃時 atk/spa、受け時 def/spd) | 実数値の連鎖。攻撃側の特性 → 防御側の特性 → 持ち物の順 | hugepower / purepower({atk:8192})・furcoat({def:8192}) |
| `SeparateStatMods map[StatKey]int` | 攻撃 | ランクの直後に単独で丸める(連鎖しない)。atk/spa のみ | 連鎖の前 | hustle({atk:6144}) |
| `CritDamageMod int` | 攻撃 | 急所のときの最終ダメージ補正 | 最終補正の連鎖で壁の直後・抜群軽減の前 | sniper(6144) |
| `PreventsCritical bool` | 防御 | 急所の指定を無効にする(ランク・壁・×1.5 も急所でない扱い) | 急所の判定 | battlearmor / shellarmor |
| `IgnoresOpponentRanks bool` | 両側 | 攻撃時は相手の防御ランク、受け時は相手の攻撃ランクを 0 にする | 実数値のランク | unaware |
| `IgnoresDefenderAbility bool` | 攻撃 | 防御側の Breakable な特性を無いものとして計算する(印も付けない) | 計算の最初 | moldbreaker |
| `Breakable bool` | 防御 | IgnoresDefenderAbility の対象になる | — | 防御側でダメージを変え、oracle で無視されるもの(§3) |

- **語彙**: `PowerCondition` は閉じた語彙 `max_base_power` / `move_type`(ADR-0139 の SpeedCondition と同じ形。
  `AllPowerConditions()` / `Known()`)。段階2でフラグの条件(`move_flag` 等)を足す想定。
- **値域**: 4096 基準の補正値はすべて `MinEffectModifier..MaxEffectModifier`(ADR-0117)。`PowerMods[].Modifier` は 4096(中立)不可。
  `TypeConvert` は 3 つとも必須・From != To。`max_base_power` は MaxPower ≥ 1 かつ MoveType 空、`move_type` は MoveType 必須かつ MaxPower 0。
  `AuraType` と `AuraMod` は組。`StatMods` は spe/hp 不可(素早さは SpeedMods)、`SeparateStatMods` は atk/spa のみ。
  `Individual.Validate` と共通マスタのデコードの両方で拒否する(engine は表に無い型を ErrUnknownType にする)。
- **タイプ変換の範囲**: 技の機構に `type_change` を持つ技(ウェザーボール等。ADR-0121)は変換しない(oracle の noTypeChange と同じ扱いで、
  技の印はそのまま残る)。変換後のタイプで、相性・タイプ一致・天候・フィールド・タイプ強化の持ち物・半減きのみ・防御側の特性の半減/無効・
  オーラ・`move_type` の条件を引く。`DamageResult.Effectiveness` / `STAB` も変換後の値になる。
- **かたやぶりの範囲**: 防御側の特性が Breakable なら、その効果(半減・無効・吸収・浮遊・実数値・急所無効・ランク無視)と未対応の印をすべて
  無視する。攻撃側の特性は無視しない。オーラ(Breakable でない)は無視しない。
- **印の扱いの細部**: 必ず急所の技(always_crit)は防御側が PreventsCritical なら通常の式で正しいので印を付けない。
  防御ランク無視の技(ignore_defense_ranks)は攻撃側が IgnoresOpponentRanks なら印を付けない。

### 2. 効果データ(data/importer/effects.json が正。testdata/golden/effects.json はその写し。ADR-0118)

- 上の表の特性を「未対応の印」から効果の定義へ移し、tools/golden/unsupported-effects.json から外す。
- 新たに oracle の調査で見つかった漏れのうち、段階1で扱わない **merciless**(毒の相手に必ず急所)・**longreach**(接触の半減を受けない。
  段階2のフラグ待ち)は `UnsupportedAttacker` の印を付け、unsupported-effects.json に理由を書く。
- Breakable は既存の防御側の定義(thickfat・filter・solidrock・waterbubble・levitate・waterabsorb・voltabsorb・eartheater・flashfire・
  sapsipper・motordrive・lightningrod・heatproof・purifyingsalt・eelevate)と、furcoat・battlearmor・shellarmor・unaware に付ける。
- **段階1では未対応の印の定義に Breakable を付けない**(印の定義は印だけを持つ、という生成器の検査を保つ)。
  そのため、かたやぶり × マルチスケイル等は「印あり」のまま返る(数値は oracle と同じ。安全側の過検出)。

### 3. 網羅の仕組み(ADR-0120 を保つ・広げる)

- 生成器の調査条件に「両側のランク ±2」「毒の防御側」と、**攻撃側の調査だけで使う「防御側に特性を持たせた条件」**
  (Thick Fat・Fur Coat・Fluffy・Multiscale・Levitate・急所 × Shell Armor)を足した。これで「単独では効かないが相手と組み合わさって
  ダメージを変える」特性(かたやぶり・てんねん・ひとでなし・えんかく)を拾い、定義か印のどちらかを必須にする(差分は両方向とも失敗)。
  基準は持ち主の側ごと(attackerOnly の条件があるため)。
- **Breakable は手で列挙しない**: 防御側でダメージを変える特性ごとに、攻撃側に IgnoresDefenderAbility の特性を持たせたときに
  「防御側の特性なし」と同じダメージになるかを oracle で調べ、effects.json の Breakable(印の定義を除く)と両方向で一致させる。
- ベクタ(fixed.json に足すだけ。random 系の乱数列・期待値は不変):
  `effects/<id>/convert|stat|separate|power|aura|crit|nocrit|ranks|ignore/.../apply|control` と、Breakable の定義ごとに
  `.../breakable`(無効・吸収の特性は既存の命名 `<slug>/breakable`)。生成器が各組で「効果の有無で oracle が変わる/変わらない」を確かめる。
- random.jsonl.gz・legacy-effects.jsonl.gz 等は、入力の効果定義に Breakable が載ったのでバイト列が変わる(期待値は全件同じ)。
  engine/golden_immunity_test.go の sha256 の固定値を更新した。

### 4. 契約・WASM・クライアント

- `MasterEffect` は自由形(`additionalProperties: true`)なので openapi は変えない(`make gen` の差分なし)。
- WASM の境界(engine/wasmapi の `abilityEffectDTO`)はトップレベルの新しいキーを camelCase で受け付け
  (`typeConvert`・`powerMods`・`auraType`・`auraMod`・`statMods`・`separateStatMods`・`critDamageMod`・`preventsCritical`・
  `ignoresOpponentRanks`・`ignoresDefenderAbility`・`breakable`)、入れ子(typeConvert・powerMods の要素)は Web の渡し方どおり
  PascalCase も受ける(encoding/json の大小無視。ADR-0139 §4 と同じ)。語彙・タイプ・キーの不正は invalid_enum、未知のキーは unknown_field。
- calc-svc の Store は新しい項目をディープコピーして返す(ポインタ・スライス・map)。
- クライアントへの影響は「印が減る(結果の数値が oracle どおりに変わる)」だけ。

### 5. 未対応の印を外す条件

効果を計算に入れ、apply/control(と Breakable なら breakable)のベクタが fixed.json で oracle と全件一致したものだけ外す。

## 結果

- フェアリースキン系・ちからもち系・はりきり・ファーコート・テクニシャン・はがねのせいしん・フェアリーオーラ・スナイパー・
  カブトアーマー/シェルアーマー・かたやぶり・てんねんが計算に入る。
- 既存の効果あり特性の挙動は変わらない(Breakable は IgnoresDefenderAbility の相手が居るときだけ意味を持つ)。

## 対象外(後続)

- 段階2: 技のフラグ(接触・音・パンチ・かみつき・切る・波動・弾)と、それに依存する特性(パンクロック・てつのこぶし・かたいツメ・
  がんじょうあご・きれあじ・メガランチャー・ぼうだん・ぼうおん・もふもふ・えんかく・うるおいボイス・ちからずく・すてみ 等)。
  連鎖の順に注意: oracle は 5325 群(かたいツメ等)をオーラの**後**に掛けるので、`PowerMods` の位置(オーラの前)とは別の段が要る。
- 段階3: HP 条件(もうか等・マルチスケイル)。満タン前提の判断の ADR が要る。
- ひとでなし(毒の相手への急所)は、防御側の状態異常を条件にした急所の項目で表せるが段階1では印のまま。
- 未対応の印の定義への Breakable(かたやぶり × 印のある特性の印を外す)。
