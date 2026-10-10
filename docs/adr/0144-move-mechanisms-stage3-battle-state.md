# ADR-0144: 技の機構の段階3 — 残り HP・多段の回数を「対戦の状態」の入力にし、なげつける・分類の切り替え・持ち物による接地を計算に入れる

- 状態: 採用(実装済み。実データの dry-run の結果は §結果)
- 日付: 2026-10-10
- レーン: データ(取り込み・DB・共通マスタ・効果定義)+ engine(計算)+ API 契約(省略可のキーの追加)
- 関連: ADR-0142(段階1)、ADR-0143(段階2。MoveRule の語彙・effects.json の moveRules)、ADR-0123(未対応の印)、ADR-0116(接地)、
  ADR-0175(持ち物の役割)、ADR-0228 / ADR-0230(お気に入り・計算履歴の CalcRequest)、ADR-0212(計算イベント)、ADR-0124(適用済みの
  migration を書き換えない)、ADR-0118(effects.json 2つの一致)、ADR-0807(生成物は Git に置かない)

## 背景

ADR-0143 の後も、使用可能な攻撃技(Showdown の isNonstandard が null)335 のうち 31 技に未対応の印が残る。理由は「engine の入力に無い状態」
(残り HP・対戦の履歴・場のじゅうりょく)、「データが無い」(なげつけるの持ち物ごとの威力)、「語彙が無い」(分類の切り替え・フォルム)。
あわせて、攻撃側の多段の回数を利用者が選べない(段階1の既定のみ)・攻撃側のくろいてっきゅうによる接地を engine が知らない、の2つの穴がある。

ユーザー方針(2026-10-10): 段階3で (1) 残り HP の入力、(2) 多段の回数の入力、(3) 入力が無くても決まる・データで足りる残りのうち無理なく
入るものを入れる。対戦の履歴が要る 15 技と確率で威力が変わる技は「未対応」の印のまま、理由と件数を書く。

## 測定(2026-10-10。実データ: ピン留めした Showdown champions mod f10d6798 と @smogon/calc 0.12.0 Champions 世代。技の ID は書かない)

段階2の後に印が残る 31 技(ADR-0143 §段階3に残す と同じ集合。取得物の hooks と effects.json の moveRules から再集計して一致を確かめた):

| 理由 | 技数 | 段階3 | 残す |
|---|---|---|---|
| 残り HP(攻撃側の割合の威力 2・攻撃側の 48 分率の威力 2・防御側の割合の威力 1・攻撃側の HP の固定ダメージ 1・防御側の HP の半分 1・防御側 − 攻撃側 1) | 8 | **8** | 0 |
| なげつける(持ち物ごとの威力) | 1 | **1** | 0 |
| 分類の切り替え(攻撃 / 防御 と 特攻 / 特防 の比) | 1 | **1** | 0 |
| 対戦の履歴(受けたダメージ・倒れた味方・手持ち・たくわえた回数・行動の順・前のターンの失敗・使った回数・能力が下がったか) | 15 | 0 | **15** |
| フォルム依存(攻撃側のフォルムでタイプ・威力・回数が変わる) | 3 | 0 | **3** |
| 場の状態のうち engine に無いもの(じゅうりょく) | 1 | 0 | **1** |
| 乱数(確率で威力が変わる) | 1 | 0 | **1** |
| タイプなし(わるあがき) | 1 | 0 | **1** |
| (技数) | **31** | **10** | **21** |

威力 0 で登録された技(`zero_power` の印)は段階2の後 13 技。段階3で 7 技が外れ(残り HP の 6 + なげつける)、6 技が残る(すべて対戦の履歴)。
技の数に数えない穴: 多段の回数の入力(回数が範囲の多段技 8 技に効く)、攻撃側のくろいてっきゅうによる接地(ADR-0143 §段階3に残す)。

### 残す 21 技の理由

- **対戦の履歴 15 技**: 受けたダメージの倍返し型 4・このターンに受けたダメージで威力が倍の型 2・前のターンの失敗で倍の型 2・後攻で倍 1・
  倒れた味方の数 1・殴られた回数 1・手持ちの数 1・たくわえた回数 1・同じターンに続けて使ったか 1・このターンに能力が下がったか 1。どれも「この1回の攻撃」の外の情報で、
  画面で選ばせる入力の設計(何を・どの既定で)が技ごとに違う。oracle(0.12.0)もほとんどを計算しない(倍返し型・手持ちの数は分岐が無い)ので
  照合の正が無い。対戦の状態の入力を足す次の段階で、技ごとの入力を設計してから扱う。
- **フォルム依存 3 技**: タイプ・威力・回数が攻撃側の種族のフォルムで決まる。engine の種族は図鑑キー(dex-form)だけを持ち、Showdown の
  種族 ID ↔ キーの対応をデータの語彙に持たせる設計(「その種族以外は失敗」も含む)が要る。2 技は使用不可のフォルム・戦闘中だけの姿で
  効くものを含み、実害が小さい。段階4に回す。
- **じゅうりょく 1 技**: 場の入力にじゅうりょくを足すと、接地・ふゆうの無効・命中に広く効く(この技だけの話ではない)。場の入力の設計とともに扱う。
- **確率で威力が変わる 1 技**: 結果を1つの分布で表すと確定数の意味が変わる。表示の設計が要る(ユーザー方針で対象外)。
- **わるあがき 1 技**: 実機はタイプなし(相性・タイプ一致なし)だが oracle はノーマルとして計算し(ゴーストに無効)、照合の正が実機と食い違う。
  技選びに出ない(覚える種族が無い)ので対象外のまま。

## oracle(@smogon/calc 0.12.0 champions.js / pokemon.js / util.js / items.js)で確かめた挙動

- **残り HP**: `Pokemon` の `curHP` オプション。`curHP || originalCurHP` なので 0 は「満タン」、最大を超える値は最大に丸める。
  確定数(`getKOChance`)は `defender.curHP()` を倒す回数で数え、表示%(`toDisplay`)の分母は `maxHP()`。
- **威力の式**(`calculateBasePowerChampions`): ふんか型 `max(1, floor(150 × 残り / 最大))`(150 は登録された威力と同じ。engine は技の威力を使う)、
  きしかいせい型 `p = floor(48 × 残り / 最大)` で `≤1:200, ≤4:150, ≤9:100, ≤16:80, ≤32:40, else 20`、
  ハードプレス型 `b = 100 × floor(防御側の残り × 4096 / 最大)`、`floor(floor((100 × b + 2047) / 4096) / 100) || 1`(Showdown と同じ)。
- **固定ダメージ**: いのちがけ型は `attacker.curHP()`(タイプ相性の無効の後)。いかりのまえば型・がむしゃら型は oracle に分岐が無く、
  威力 0 として 0 を返す(**照合の正にできない**)。engine は Showdown の damageCallback(いかりのまえば型 `max(1, floor(残り / 2))`、
  がむしゃら型 `防御側の残り − 攻撃側の残り`、攻撃側の残りが防御側以上なら失敗)に合わせ、単体テストで確かめる。
- **なげつける**: `getFlingPower(attacker.item)`(持ち物なしは 0 → ダメージ 0)。攻撃側の持ち物の他の効果(いのちのたま 等)は計算中も持ったまま。
- **分類の切り替え**: `getShellSideArmCategory`: ランク補正後の実数値で `攻撃 / 防御側の防御 > 特攻 / 防御側の特防` なら物理(接触あり)、
  同じなら特殊。実機(Showdown)はダメージの式の途中の値で比べ、同じときは乱数。engine は oracle に合わせ、整数の掛け算
  (`攻撃 × 特防 > 特攻 × 防御`)で比べる(浮動小数を使わない)。
- **くろいてっきゅう**: `isGrounded` は `field.isGravity || hasItem('Iron Ball') || (飛行でない && ふゆうでない && ふうせんでない)`。持ち物の接地が
  タイプ・特性より先。防御側の地面技の相性(浮いている側に当たる。ぶきよう で無効)は別の分岐で、engine は表せない(防御側の印は残す)。
- **多段の回数**: `Move` の `hits` オプション。範囲の多段技は指定がスキルリンクに勝つ(`options.hits || ...`)。値は丸めない。

## 決定

### 1. 語彙の追加(engine。`engine/move_rule.go`・`engine/model.go`・`engine/modifiers.go`)

```go
// PowerFormula の追加(定義順の末尾)
PowerFormulaAttackerHPScaled  PowerFormula = "attacker_hp_scaled"  // max(1, floor(威力 × 攻撃側の残り / 最大))。威力 1 以上が必要
PowerFormulaAttackerHPLow     PowerFormula = "attacker_hp_low"     // 48 分率の段(上の表)
PowerFormulaDefenderHPRatio   PowerFormula = "defender_hp_ratio"   // ハードプレス型の式(定数 100)
PowerFormulaAttackerItemFling PowerFormula = "attacker_item_fling" // 攻撃側の持ち物の Item.FlingPower

// 固定ダメージの式(新しい閉じた型。機構 fixed_damage と対応)
type FixedDamageFormula string
const (
	FixedDamageAttackerCurrentHP       FixedDamageFormula = "attacker_current_hp"        // 攻撃側の残り HP
	FixedDamageDefenderHalfHP          FixedDamageFormula = "defender_current_hp_half"   // max(1, floor(防御側の残り / 2))
	FixedDamageDefenderMinusAttackerHP FixedDamageFormula = "defender_minus_attacker_hp" // 防御側の残り − 攻撃側の残り(≤ 0 は失敗)
)
func AllFixedDamageFormulas() []FixedDamageFormula // 定義順のコピー
func (f FixedDamageFormula) Known() bool

// MoveRule の追加
FixedDamageFormula FixedDamageFormula // "" はなし。機構 fixed_damage が必要。Params.FixedDamage・威力の中身と排他
CategoryByStats    bool               // 分類の切り替え。機構 move_specific が必要

// 対戦の状態(DamageInput.State。ゼロ値は従来どおり)
type BattleState struct {
	AttackerCurrentHP int // 0 は満タン。1..攻撃側の最大 HP
	DefenderCurrentHP int // 0 は満タン。1..防御側の最大 HP
	Hits              int // 0 は段階1の既定。範囲の多段技(Min < Max)だけ・Min..Max
}
var ErrInvalidBattleState = errors.New(...)

Item.FlingPower int        // 0 は不明・投げられない(印を残す)。負は Individual.Validate で拒否
ItemEffect.Grounds bool    // 持ち物で接地する(isGrounded でタイプ・特性より先)
```

- 表と式の定数(48 分率の段・ハードプレス型の 100 と 4096)はゲームの規則として engine に持つ(段階2の表と同じ扱い)。技の一覧は持たない。
- 検証(`ValidateRule` → `ErrInvalidMoveRule`): 固定ダメージの式の語彙外・`fixed_damage` 無し・`Params.FixedDamage` との両立・
  威力の中身との両立、`CategoryByStats` に `move_specific` 無し、`attacker_hp_scaled` で威力 0。
  状態の検証(`CalcDamage` → `ErrInvalidBattleState`): 残り HP が負・最大超え、回数が負・範囲の多段でない技・範囲外。

### 2. 計算の順(oracle と同じ)

1. 状態の検証 → 既存の検証 → 特性の段階1 → 技の処理の前処理。`CategoryByStats` はこの前処理で分類を決め、物理なら技のフラグに
   接触を足す(FlagsKnown のとき。接触に依存する特性に効く)。
2. 相性 → 無効(タイプ → 失敗(`FailsWithoutDefenderItem`・**攻撃側の持ち物なしのなげつける型**・**がむしゃら型の残り HP の比較**)→ 特性 → サイコフィールド)。
3. 固定ダメージ: 既存の `Params.FixedDamage` と同じ位置で `FixedDamageFormula` を計算し、16 段階を同じ値で埋める。
4. 威力: `PowerFormula` の新しい式。持ち物が `FlingPower` 0 またはメガストーンのなげつける型は「計算できない」(印を残し、数値は従来どおり 0)。
5. 多段: `State.Hits` が 0 でなければその回数(スキルリンクより優先)。0 は段階1の既定。
6. 確定数: `DefenderCurrentHP`(0 は最大)を倒す回数で数える。`DamageResult.DefenderHP` と表示%は最大 HP のまま。
7. 接地: `isGrounded` は持ち物の `Grounds` を最初に見る(両側)。

**HP が条件の特性(もうか 等・マルチスケイル 等)は段階3で計算に入れない**(従来どおり効果データの印)。残り HP の入力があるので、次の段で
効果の語彙(`HPAtMost`・`FullHP` の条件)を足せば外せる(別 ADR)。

### 3. 印の扱い(ADR-0143 §3 への追加)

| 機構 | 印を外す条件 |
|---|---|
| variable_power / zero_power | 新しい式は常に計算できる。ただし `attacker_item_fling` は攻撃側の持ち物が無い(失敗)か、`FlingPower` > 0 かつメガストーンでないとき |
| fixed_damage | `Params.FixedDamage` か `FixedDamageFormula` がある |
| move_specific | 従来どおり `MoveSpecificResolved`(なげつける型・分類の切り替え型も書く) |

### 4. データ

- `data/importer/effects.json` の `moveRules` に 10 技(式 6・固定ダメージ 3・分類 1。§測定)を足し、写し(`testdata/golden/effects.json`。キーは
  oracle の英語名)にも同じ定義を足す。`items.ironball` を `{"Grounds":true,"UnsupportedDefender":true}` にする(写しも)。
- 共通マスタ: `DecodeMoveRule`・`EncodeMoveRule` が `FixedDamageFormula`(語彙)・`CategoryByStats`(true だけ)を、`DecodeItemEffect`・
  `EncodeItemEffect` が `Grounds`(true だけ)を扱う。`ItemRoles`: `Grounds` は両側の役割。`ItemRow.FlingPower`(0 は NULL)を `master.Item` が
  `engine.Item.FlingPower` に写す(負は `ErrInvalidRow`)。
- なげつけるの威力の取得: `tools/importer/fetch-showdown.mjs` が持ち物ごとに `flingBasePower`(Showdown の `fling.basePower`。無ければ null)を出す。
  importer はキーを**必須**としてデコードし(無い古い取得物は `ErrInvalidInput`。`make import-fetch` で取り直す)、1..255 の整数を `NamedRow.FlingPower`
  に写す(null は 0)。それ以外は `ErrInvalidData`。calc の取得物との照合はしない(oracle の `getFlingPower` は技名の表で、取得物に出ないため)。
  実データの dry-run で、取り込む持ち物の Showdown の値と oracle の `getFlingPower` の食い違いを数え、§結果に書く(ゴールデンは一致する持ち物だけを使う)。
- 変換結果の版(ADR-0122)に `FlingPower` を含める。

### 5. DB

- migration **000017** `items.fling_power SMALLINT UNSIGNED NULL`・`CHECK (fling_power IS NULL OR fling_power > 0)`(000016 と同じ「列を足すだけ」)。
  down は CHECK を落としてから列を落とす。sqlc: `InsertItem`・`ListItems`・`SearchItems` が列を運ぶ(この ADR の spec で済み)。
- origin/main の最新は 000016(2026-10-10 時点)。**マージ前に main の最新の版を確かめ、先に別の 000017 が入っていたら番号を繰り下げる**
  (`services/pokedex/db/item_fling_power_layout_test.go` の定数も合わせる)。

### 6. 契約の差分(`api/openapi.yaml`。どれも省略可のキーの追加)

公開 API:
- `CalcRequest.battleState`(新しいスキーマ `CalcBattleState`: `attackerCurrentHp`・`defenderCurrentHp`(integer ≥ 1)・`hits`(1..10))。
  省略・null・`{}` は従来と同じ結果。値域の外(0 を含む)・範囲の多段でない技の回数は 400 `invalid_input`、未知のキーは 400 `unknown_field`。
  入力の形は**実数値**(oracle の curHP と同じ。割合を受けると最大 HP の丸めの規則を契約に持ち込むことになる。割合の表示・入力の補助は画面の役目)。
  `POST /api/calc` だけが使う。一括・逆算・調整には足さない(探索で最大 HP が変わり、実数値の残り HP の意味が定まらないため。必要になったら別 ADR)。
- `Item.flingPower`(integer ≥ 1。無ければキーを省く)。
- 応答(`CalcResult`)は変えない。確定数は残り HP で数えた値になる(`defenderHP`・表示%は最大 HP のまま)。

内部 API: `MasterItem.flingPower`(省略可。NULL はキーを省く。calc-svc は省略を 0 に)。`MasterMove.rule` の中身に段階3のキーが増える(形は不透明のまま)。

WASM(Web 内部): `calc` の `battleState`(同じ3つのキー。未知のキーは `unknown_field`)、持ち物の `flingPower`、持ち物の効果の `grounds`、
技の `rule` の `fixedDamageFormula`・`categoryByStats`(PascalCase も受ける)。語彙外は `invalid_enum`、値域は `invalid_input`。

**お気に入り・計算履歴**: `CalcRequest` を共有する(ADR-0228・ADR-0230)ので、record-svc の手書きの受け口も `battleState` を受けて保存・返却する
(ユーザー指示 2026-10-10 で当初の「record レーンへの依頼」をこの実装に入れた)。省略・null・`{}` は省略のまま(`calc` のバイト列・重複判定が変わらない)、
指定したキーだけを保存し、値域は `CalcBattleState` と同じ(残り HP は 1 以上・回数は 1..10。最大 HP との照合はマスタが要るのでしない。範囲外は 400 `invalid_input`・
未知のキーは 400 `unknown_field`)。計算履歴は calc-svc の計算イベント `calcevents.CalcDetail` に `battleState`(omitempty)を足し、record-svc が同じ正規化で返す。
DB のスキーマ変更は無い(お気に入りの snapshot・イベントの payload は JSON)。**デプロイ順**: record-svc(受け手)を先、その後 calc-svc。

### 7. ゴールデン(`testdata/golden/mechanisms-stage3.json`。Champions 世代・SP そのまま。既存のファイルは変えない)

生成器は oracle の `curHP`・`hits` を使い、入力の `State` に載せる(残り HP は 1..最大。0 は oracle が満タンと読むので使わない)。

| ラベル | 内容 |
|---|---|
| `hp-attacker-scaled/*` | ふんか・しおふき型(満タン・半分・1・境界) |
| `hp-attacker-low/*` | きしかいせい・じたばた型(6 つの段の両端) |
| `hp-defender-ratio/*` | ハードプレス型(満タン・半分・1) |
| `hp-fixed/*` | いのちがけ型(残り HP・ゴーストへの無効) |
| `hp-ko/*` | 通常の技・多段の技で防御側の残り HP が確定数に効く(定義を持たない技) |
| `hits/*` | 範囲の多段技の回数の指定(最小・最大・スキルリンク + 指定) |
| `fling/*`・`fling-none/*` | なげつける型(効果を持たない持ち物・効果を持つ持ち物)・持ち物なし |
| `category-physical/*`・`category-special/*` | 分類の切り替え型(ランクで入れ替わる組・同値) |
| `grounded-item/*` | 飛行・ふゆうの攻撃側がくろいてっきゅうでフィールドの補正を受ける(定義を持たない技) |

生成器は (a) 段階3の語彙の `moveRules` の全技(oracle が計算しない いかりのまえば型・がむしゃら型を除く)に1件以上、(b) apply/control で
oracle の結果が違う、(c) 状態の値域、を確かめて崩れたら止まる。段階2のゴールデンの網羅(`TestGoldenMechanismsStage2`)は段階3の語彙の技を
数えない(段階3のファイルが網羅を見る)。`metadata.json` の `exclusions` の技の理由を「残り HP の技は mechanisms-stage3.json で照合、
対戦の履歴が要る技は対象外」に改める。

### 8. デプロイ順

**migrate(000017)→ アプリ(pokedex-svc・calc-svc・Web の WASM)→ `make import-fetch`(flingBasePower の取り直し)→ 取り込み → master-release**。

- 先に取り込むと、新しいキー(`FixedDamageFormula`・`CategoryByStats`・新しい式・`Grounds`)を古い calc-svc / WASM の厳格なデコードが拒否する(503)。
- 新しいアプリ + 取り込み前の DB は、段階3の技に印が残るだけ(従来どおり)。`battleState` は計算に効く(データに依らない)。
- Web は WASM と `onlineSource.ts`(`flingPower` の写し)を同じリリースに入れる(古い WASM は未知のキー `flingPower` を拒否する)。

## 受け入れ条件

1. engine: 残り HP・多段の回数の入力(`DamageInput.State`)の省略は従来と同じ結果。指定すると §1 の式・確定数・回数に効き、値域外は `ErrInvalidBattleState`。
2. engine: 段階3の 10 技の語彙(式 4・固定ダメージの式 3・分類の切り替え)を等価な通常の入力と同じ値で計算し、§3 の条件で印を外す。
   不正な定義は `ErrInvalidMoveRule`。持ち物の `Grounds` で両側が接地する。
3. ゴールデン `mechanisms-stage3.json` が全件一致し §7 のラベルを網羅する。既存のゴールデンは全件そのまま一致する。
4. データ: 本番の `moveRules` は 46 技(段階2の 36 + 段階3の 10)で語彙ごとの件数どおり、写しと一致。`ironball` は `Grounds`。
5. importer・DB・pokedex-svc・calc-svc: なげつけるの威力が取得物 → `items.fling_power` → 内部 API / 公開 API → engine まで届き、無い値はキーを省く。
6. 公開 API `POST /api/calc` と WASM の `calc` が `battleState` を受けて engine に渡し、値域外を 400 / エラー封筒で返す。

## 実装の手順(implementer 向け。テストは spec-writer が先に置いた)

1. engine: §1 の型と定数(`AllPowerFormulas` の末尾に4つ)、`BattleState`・`DamageInput.State`・`ErrInvalidBattleState`、`Item.FlingPower`
   (`Individual.Validate` で負を拒否)、`ItemEffect.Grounds`(`isGrounded`)、`ValidateRule` の追加の検証。`calcDamageNoKO` に §2 の順で組み込み、
   `multiHitCount` に `State.Hits`、`computeKO` は残り HP(`DamageResult` に内部用の残り HP を持たせるか、KO を作る経路で渡す。`DefenderHP` は最大のまま)。
   `mechanismHandled`・`moveMarks`・`ruleHasComputableFormula` を §3 に。一括・逆算・調整は `State` のゼロ値で従来どおり(変更不要のはず)。
   テスト: `engine/move_rule_stage3_test.go`(と、語彙の並びを足した `move_rule_stage2_test.go` の `TestMoveRuleVocabulary`)。
2. `services/internal/master`: `DecodeMoveRule`・`EncodeMoveRule`・`DecodeItemEffect`・`EncodeItemEffect` のキー、`ItemRoles` の `Grounds`
   (`item_role_test.go` の表に `Grounds` の行を足す。`TestItemRolesTableCoversEveryItemEffectField` が要求する)、`ItemRow.FlingPower`。
   テスト: `services/internal/master/move_rule_stage3_test.go`。
3. importer: `ShowdownItem.FlingBasePower`(キーの有無を見分ける)・デコードの必須化・`NamedRow.FlingPower`・`Apply` の `InsertItem`・変換結果の版。
   fixture(`testdata/fictional/generated/showdown/.../snapshot.json`)の持ち物に `flingBasePower`(testmonite 80・testorb 30・testberry 10・
   testrelic null)を足す(足さないと全テストのデコードが止まる)。`effects.json` 2つに §4 の定義。`tools/importer/fetch-showdown.mjs` の取得。
   テスト: `convert_fling_power_test.go`・`convert_move_rules_test.go`(件数 46)・`source_of_truth_test.go`(写しの一致)。
4. DB: migration 000017 と sqlc のクエリは spec で置いた。`make gen` の後、`storetest`(`ListItems` は `store.Item` を返す)に `FlingPower` を運ぶ。
   テスト: `services/pokedex/db/item_fling_power_layout_test.go`。
5. pokedex-svc: 内部 API の `MasterItem.flingPower`、公開 API の `Item.flingPower`(NULL はキーを省く)。
   テスト: `services/pokedex/internal/httpapi/item_fling_power_test.go`。
6. calc-svc: `internal/master/export.go` で `flingPower` → `ItemRow.FlingPower`(0 以下は `ErrInvalidMaster`)。`httpapi` の `CalcDamage` で
   `battleState` を検証(1 未満は `invalid_input`。engine の 0 = 満タンと取り違えない)して `DamageInput.State` に写し、`ErrInvalidBattleState` を
   `invalid_input` に写す。計算イベントには載せない(§6)。テスト: `services/calc/internal/master/fling_power_test.go`・
   `services/calc/internal/httpapi/battle_state_test.go`。
7. wasmapi: `calcRequest.BattleState`(`*battleStateDTO`。未知のキーは `unknown_field`)、`itemDTO.FlingPower`、`itemEffectDTO.Grounds`、
   `moveRuleDTO.FixedDamageFormula`・`CategoryByStats`。`ErrInvalidBattleState` → `invalid_input`。Go/WASM 一致のベクタ(`testdata/vectors.json`)に
   `battleState` の1件を足す。テスト: `engine/wasmapi/battle_state_stage3_test.go`。
8. Web: `web/src/engine/types.ts` の持ち物に `flingPower?`、`web/src/master/onlineSource.ts` で応答のまま写す(表示は変えない。画面の入力欄は
   Web レーンへの依頼)。テスト: `web/src/master/onlineSource.flingPower.test.ts`。
9. `tools/golden/generate.mjs` に §7(`curHP`・`hits` を oracle に渡し `State` に載せる・持ち物の `FlingPower` を載せる)、
   `engine/golden_effects_test.go` の型付きの効果の項目に `Grounds`。`make golden-generate` → `make test-golden`。
   テスト: `engine/golden_mechanisms_stage3_test.go`(と、段階3の技を網羅から外した `golden_mechanisms_stage2_test.go`)。
10. `make import-fetch` → 実データの dry-run で `moveRules: 46`・印が残る技が 21・なげつけるの威力の食い違い(Showdown と oracle)を確かめ、§結果に追記する。

## 結果

- 良い点: 残り HP で決まる技・なげつける・分類の切り替えの数値が正しくなり、印が 31 → 21 技に減る(残りはすべて対戦の履歴・フォルム・
  じゅうりょく・乱数・わるあがき)。防御側の残り HP で確定数を数えられる。範囲の多段技の回数を選べる。
- 注意: いかりのまえば型・がむしゃら型は oracle が計算しないので、ゴールデンではなく単体テスト(Showdown の規則)で守る。
- 注意: 分類の切り替えは oracle(実数値の比・同値は特殊)に合わせ、実機(ダメージの途中の値・同値は乱数)とは境界で違いうる。
- 注意: なげつける型は oracle に合わせ、攻撃側の持ち物の他の効果を計算中も残す。実機で投げた持ち物の効果が残るかは未確認(段階4の確認事項)。
- 注意: お気に入り・計算履歴は `battleState` を保存・返却する(上の §6)。最大 HP を超える残り HP は保存時には弾かず、計算するとき calc-svc が 400 にする。
- 実データの dry-run の結果(2026-10-10。Showdown champions mod f10d6798・@smogon/calc 0.12.0): `go run ./pokedex/cmd/import -data ../data -typechart ../testdata/golden/typechart.json -dry-run` は
  exit 0・blockers: none・`moveRules: 46`(段階2の 36 + 段階3の 10〈ふんか・しおふき型 2・きしかいせい・じたばた型 2・ハードプレス型 1・いのちがけ・いかりのまえば・がむしゃら型 3・なげつける 1・分類の切り替え 1〉)。
  攻撃技 335 のうち、攻撃側・防御側とも持ち物を持つ単純な対戦(単体・フィールドなし)で技の印が残るのは **21 技**(測定の 31 から 10 減。assurance・aurawheel・avalanche・beatup・comeuppance・counter・ficklebeam・gravapple・lashout・lastrespects・metalburst・mirrorcoat・payback・ragefist・ragingbull・round・spitup・stompingtantrum・struggle・temperflare・watershuriken。
  理由別の印の数: variable_power 14・zero_power 6・fixed_damage 4・move_specific 3・type_change 2〈1 技が複数の印を持つ〉)。
- なげつけるの威力の食い違い(Showdown の `fling.basePower` と oracle の `getFlingPower`): 使用可能な持ち物 166 件のうち 87 件。メガストーン 81 件は Showdown 80・oracle 0(engine はメガストーンを投げられないものとして印を残す)、
  それ以外 6 件(Big Root 10・Binding Band 30・Bright Powder 10・Fairy Feather 10・Metronome 30・Shell Bell 30)は oracle の表に無く 0。engine は取り込んだ Showdown の値で計算するので、この 6 件は oracle と違う(ゴールデンは一致する持ち物だけを使う)。
- ゴールデン mechanisms-stage3.json は 380 件・全件一致。既存のファイルは expected が全件そのまま一致する。ただし `mechanisms-stage2.json` はくろいてっきゅうの効果定義(effects.json)が `Grounds` を持つようになったため、
  それを持つ攻撃側のベクタの入力の `Item.Effect` に `"Grounds": true` が 12 行増えた(ダメージ・期待値は不変。ベクタの入力は効果定義と同じ内容を載せる規則のため)。

