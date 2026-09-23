# ADR-0107: 技の追加効果(命中時のランク変化)をデータとして持つ

- 状態: 採用
- 日付: 2026-09-23
- レーン: データ(ADR 帯 `0100〜`。前は ADR-0106)
- 関連: ADR-0005(データ駆動の効果定義) / ADR-0100 §3・§6(スキーマと効果 JSON) / ADR-0101 §4(技の変換) /
  ADR-0103 §6(Showdown からの抽出) / ADR-0106(engine が持つが計算に使わないデータの前例) /
  ADR-0204(calc-svc のマスタ) / ADR-0700・ADR-0701(判定レーン) / DECISIONS.md 2026-09-22「JD0 の決定と、技の追加効果によるランク変化のデータをデータレーンへ提案」
- 対象タスク: plan.md P5-6

## 背景

判定レーン(JD1)は「ニトロチャージで自分の素早さを上げてから相手を抜けるか」を1回の入力で判定したいが、
技 ID から「使用者自身のランクがどう変わるか」を引けるデータがどこにも無い。そのため JD1 は
呼び出し側が `Individual.ranks` に「技を撃った後のランク」を手で指定する形で回避している(ADR-0700 §6-5・docs/judge-design.md §4-5)。

判定レーンからの提案(DECISIONS.md 2026-09-22)の既定案は
`{"secondary": {"chance": 100, "self": {"boosts": {"spe": 1}}}}` 相当のデータを ADR-0005 に沿って持たせること。
本 ADR はその実装の設計を決める。

## 調査: 取得元に生データはあるか

固定版の取得元(ADR-0102 のピン)を実際に読んで確認した。

### @smogon/calc 0.12.0 — 使えない

`Generations.get(0)` の Move オブジェクトを実際にダンプした結果:

| 技 | calc が持つ内容 |
|---|---|
| Flame Charge | `"secondaries": true`(**真偽値だけ**。確率もランク変化量も無い) |
| Draco Meteor | `"self": {"boosts": {"spa": -2}}`(ある) |
| Close Combat | **何も無い**(実際には自分の防御・特防が下がる) |
| Swords Dance | **何も無い** |

確率・変化量が落ちている(`secondaries: true`)うえ、Close Combat のように欠落もある。**取得元にできない**。

### Pokémon Showdown(ピン留めした mod のソース)— 使える

`src/data/moves.ts` は構造化された完全なデータを持つ。形は3通り:

1. 命中時に必ず起きる使用者のランク変化 — トップレベルの `self: {boosts: {...}}`(例 Draco Meteor・Close Combat)
2. 確率つきの使用者のランク変化 — `secondary: {chance, self: {boosts: {...}}}`(例 Flame Charge chance 100・Metal Claw chance 10)
3. 確率つきの相手のランク変化 — `secondary: {chance, boosts: {...}}`(例 Crunch chance 20・防御 -1)

ピン留めした mod の全技を走査した実測値:

- 1 の技 17 件 / 2 の技 19 件 / 3 の技 60 件、合計 **97 件**
- **1つの技が2つ以上のランク変化エントリを持つ例は 0 件**(`secondaries` 配列で複数持つ技も 0 件)
- `chance` の実測値は {10, 20, 30, 40, 50, 70, 100}
- 変化量の実測値は {-2, -1, +1, +2}
- 変化するステータスのキーは atk/def/spa/spd/spe に加えて `accuracy`・`evasion` が出る。
  ただし accuracy/evasion は**必ずそれ単独**のエントリで、atk 等と混ざる例は 0 件(該当 8 技)

**決定: 取得元は Showdown 1本**。手動ファイル(`data/importer/effects.json` 相当)は作らない。
持ち物・特性の効果を手で書いているのは、Showdown が関数フック名(`hooks`)しか出せず値を機械的に取れないためであり(ADR-0103 §6)、
技の追加効果にはその制約が無い。手で 97 件を書き写すのは規約の「ハードコードしない」に照らしても不利。

## 決定

### 決定1: engine は「発動した場合の値」だけを持ち、乱数を持たない

`chance` が 100 でないとき、engine は発動するかどうかを**判定しない**。
`MoveEffect` は「命中して追加効果が発動したとき、何がどれだけ変わるか」と「その確率」を持つだけのメタデータで、
実際に発動したかどうかを決めるのは呼び出し側(judge / Web / iOS)の責務とする。

理由:
- CLAUDE.md の絶対ルール2「`engine/` は純粋に保つ。乱数の外部取得を持ち込まない」。ADR-0010 の逆算も engine 内に乱数を持たない。
- 呼び出し側は「発動した場合」「しなかった場合」の両方を `Individual.Ranks` を差し替えて計算でき、
  judge が `outspeeds`(発動時)と `outspeedsWithoutSecondary`(不発時)のように**両方を返す**選択肢を残せる。
  engine が内部で乱数を振ると、この2通りを返せなくなる。
- ダメージの乱数(16通りのロール)を全部返して呼び出し側に委ねている既存の設計と揃う。

### 決定2: ダメージ計算には一切混ぜない(ゴールデン不変)

`CalcDamage` は `Move.Effect` を読まない。追加効果は技のメタデータとして持つだけ。
CLAUDE.md の絶対ルール3のとおり `make test-golden` の期待値は1件も変わらない。
「engine が持つが `CalcDamage` は読まないデータ」は ADR-0106 の `AbsorbEffect` に前例がある(同じ立場を踏襲する)。

judge がランク変化込みの実数値を出すときは、`Move.Effect` を読んで `Individual.Ranks` を自分で作り直し、
`engine.RealStats` / `engine.EffectiveStat` を呼び直す(ADR-0701 §1 と同じ経路)。

### 決定3: engine の型は「1技につき最大1エントリ」の平たい形

```go
// RankTarget はランク変化の対象。
type RankTarget string
const (
    RankTargetSelf     RankTarget = "self"   // 使用者自身
    RankTargetOpponent RankTarget = "target" // 技の対象
)

// MoveEffect は技の追加効果(命中時のランク変化)。ダメージ計算はこの値を読まない(決定2)。
type MoveEffect struct {
    Chance int             // 発動確率(1..100)。100 は命中すれば必ず発動
    Target RankTarget      // self / target
    Stages map[StatKey]int // 変化量(-6..+6、0 は不可)。HP は不可
}

// Move に1フィールド足す。nil は追加効果なし。
type Move struct {
    ...
    Effect *MoveEffect
}
```

`[]MoveEffect` のスライスにしない理由: 上の調査のとおり、ピン留めした取得元に**2エントリ以上を持つ技は 1 件も無い**。
無い需要に合わせてスライスにすると、正準エンコードの並び順・重複の扱いという決めごとが増える。
将来2エントリ以上の技が現れたら、そのときスライスに変える(取り消しやすい)。
importer は2エントリ以上の技を見つけたら blocker として止めるので、黙って落ちることはない(決定6)。

`Validate() error` を持たせ、`Chance` 1..100・`Stages` が空でない・キーが HP でない・値が -6..+6 かつ 0 でないことを確認する。

### 決定4: 効果定義 JSON は既存の効果定義と同じ正準形(PascalCase)

`services/internal/master` に `DecodeMoveEffect` / `EncodeMoveEffect` を足す。effects.go の既存の作法をそのまま使う:
未知のフィールド拒否・大文字小文字の厳密比較・オブジェクト以外や後続データの拒否・空の拒否・正準エンコード(ゼロ値省略・struct 定義順・map キー昇順)・
`Decode(Encode(e)) == e`。

正準形の例:

```json
{"Chance":100,"Target":"self","Stages":{"spe":1}}
```

判定レーンの既定案は `{"secondary": {"chance": 100, "self": {"boosts": {"spe": 1}}}}` という Showdown 風の入れ子だったが、
本 ADR は**既存の `item_effects` / `ability_effects` の効果 JSON と同じ書き方**(engine の struct のフィールド名そのまま・平たい)に揃える。
理由: 同じ列種・同じデコーダ群を使うのに2つの流儀が並ぶと、どちらで書くかを毎回考えることになる。入れ子を平たくしても表現力は落ちない(決定3)。

`Stages` のキーは effects.go の既存の `statModKeys`(atk/def/spa/spd/spe)を再利用する。
したがって `accuracy` / `evasion` は**未知のキーとして拒否**される(engine の `Ranks` に持つ場所が無い)。

### 決定5: DB は `move_effects` 別表(moves への列追加ではない)

```sql
CREATE TABLE move_effects (
  move_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  effect JSON NOT NULL,
  PRIMARY KEY (move_id),
  CONSTRAINT fk_move_effects_move FOREIGN KEY (move_id) REFERENCES moves (id) ON DELETE CASCADE,
  CONSTRAINT chk_move_effects_effect CHECK (JSON_TYPE(effect) = 'OBJECT')
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;
```

新しい migration `000007_create_move_effects.{up,down}.sql` を足す(既存の migration は書き換えない。ADR-0100 §1)。

理由:
- `item_effects` / `ability_effects` と**まったく同じ形**になり、読む側・書く側の作法が1つで済む。
- 効果を持つのは技全体の 1 割弱(97/約900)で疎。`moves` に NULL 許容の JSON 列を足すより、行が有る=効果が有るの方が読み違えにくい。
- `moves` は公開の検索 API が引く表で、`SELECT *` 相当のクエリの結果が広がるのを避けられる。

却下: `moves` に `effect JSON NULL` を1列足す案 — 表は増えないが、上の3点をすべて失う。

### 決定6: importer は Showdown から機械的に作る。落とすものは指摘に残す

`tools/importer/fetch-showdown.mjs` の `moves` に、取得元の表現のまま次を足す(ID 化・正準化は Go 側。ADR-0101 §3 の方針どおり):

```js
self: m.self?.boosts ? { boosts: m.self.boosts } : null,
secondary: m.secondary ? { chance: m.secondary.chance, self: m.secondary.self?.boosts ? { boosts: m.secondary.self.boosts } : null, boosts: m.secondary.boosts ?? null } : null,
secondaries: Array.isArray(m.secondaries) ? m.secondaries.length : 0,
```

Go 側(`services/pokedex/importer`)の規則:

1. `moves` 表に採った技だけを対象にする(除外した技の効果は作らない)。
2. トップレベルの `self.boosts` は `Chance: 100, Target: "self"` に正規化する。
3. `secondary.self.boosts` は `Chance: secondary.chance, Target: "self"`、`secondary.boosts` は `Target: "target"`。
4. `accuracy` / `evasion` しか含まないエントリは**落とし**、警告 `KindMoveEffectUnsupportedStat` を残す(engine に持つ場所が無い。実測 8 技)。
5. ランク変化のエントリが1つも無い技は `move_effects` の行を作らない(`{}` の行は作らない)。
6. 1つの技に2つ以上のエントリがある / `secondaries` が2件以上ある場合は **blocker** `KindMoveEffectAmbiguous` にして止める(決定3 の前提が崩れたことを人に知らせる)。
7. 行の値は `master.EncodeMoveEffect` の正準形で入れる(`ItemEffects` / `AbilityEffects` と同じ)。

`@smogon/calc` 側には対応するデータが無いので**突き合わせはしない**(ADR-0101 §4 の「両方にある技だけ採る」規則は技そのものの採否の話で、効果の値の照合相手が無い)。

Go 側の型(失敗するテストが前提にしている名前):

```go
// snapshot.go: 取得元の表現のまま(ID 化・正準化は変換で行う)
type ShowdownBoosts struct {
    Boosts map[string]int `json:"boosts"`
}
type ShowdownSecondary struct {
    Chance int             `json:"chance"`
    Self   *ShowdownBoosts `json:"self"`
    Boosts map[string]int  `json:"boosts"`
}
// ShowdownMove に3フィールド足す(LoadInput は DisallowUnknownFields なので必須)
//   Self        *ShowdownBoosts    `json:"self"`
//   Secondary   *ShowdownSecondary `json:"secondary"`
//   Secondaries int                `json:"secondaries"` // 取得元の配列の件数。2 以上で blocker

// convert.go: Output に1フィールド足す(MoveRow は比較可能なまま保つため、効果は別の行にする)
//   MoveEffects []EffectRow  // ID 昇順。値は master.EncodeMoveEffect の正準形
```

`importer.MoveRow` には効果を持たせない。既存の `TestConvertMovesFollowP21cRules` が `MoveRow` を `==` で比較しており、
スライスのフィールドを足すとコンパイルできなくなる。DB も別表なので、行も別に持つのが素直。

`services/internal/master` 側は `MoveRow` に `Effect []byte`(nil / 空は効果なし)を足し、`Move()` が `DecodeMoveEffect` に通す。
`services/pokedex/internal/store` には sqlc が `MoveEffect{MoveID, Effect}` と `ListMoveEffects` を生成する
(`storetest.Querier` にも `MoveEffects` の欄が要る)。

### 決定7: export は `MasterMove.effect`(既存の `MasterEffect` をそのまま再利用)

`api/openapi.yaml` の `MasterMove` に `effect: MasterEffect` を足す(`MasterItem` / `MasterAbility` とまったく同じ)。
`MasterEffect` は `additionalProperties: true, nullable: true` の素通しで、受け取った calc-svc が
`services/internal/master`(`DecodeMoveEffect`)で厳格に検証する — これも既存2つと同じ。新しいスキーマは作らない。

**`services/pokedex/internal/readmodel` の `moveEntry` は変えない。** 調査して判断した:

- judge は read model のファイルを読まない。ADR-0700 §2・§4(4) のとおり、リクエストごとに
  pokedex-svc / calc-svc の**公開 API** を呼ぶ。read model(ADR-0012・0402)はタイプバランスと素早さのための経路。
- `services/balance/internal/master/moves.go` の `LoadMoves` は `decoder.DisallowUnknownFields()` を使い、
  `services/balance/schema/moves.schema.json` も `additionalProperties: false`。
  `moveEntry` にフィールドを足すと **balance の読み込みがその場で壊れる**(別レーンのディレクトリ)。
- 読む相手がいないうえに別レーンを壊すので、足さない。

却下: read model に足して balance のローダとスキーマも同時に直す案 — 別レーンのファイルを、そのレーンが必要としていない理由で書き換えることになる。

### 決定8: 公開 API(`Move` スキーマ)への追加は、judge が経路を決めてから

`MasterMove` は pokedex-svc の**内部** API(`GET /internal/pokedex/master`)で、読むのは calc-svc だけ。
judge が技の追加効果を直接読むには、公開 API 側(`Move` スキーマ)にも足す必要がある。
ただし公開側には技 ID 1件を引く endpoint が無く(`GET /api/pokedex/moves` の前方一致検索だけ)、
`getMove` を足すか検索で引くかは API レーン・判定レーンの決めごとになる。
本 ADR ではデータの正(DB・importer・engine・内部 export)までを作り、公開の形は判定レーンの要件が固まってから決める。
DECISIONS.md に判定レーンへの連絡として記録する。

**追記(2026-09-23)**: 判定レーンの JD4(返り討ち判定に技の優先度が要る)を受けて、API レーンが
`GET /api/pokedex/moves/{key}`(`getMove`)を実装した(ADR-0105 §3 追記)。これは「技 ID 1件を引く経路が無い」
という上記の欠落そのものを解消したもので、**`getMove` を新設する側の答え**として決まった(検索の前方一致で
代替する案は採らなかった)。ただし本決定8がもともと問題にしていた「技の追加効果(ランク変化)を公開 `Move`
スキーマに載せるか」は依然として**未決のまま**(`priority` を足しただけで `effect` は載せていない)。判定レーンが
技IDからランク変化を自動で出す要件が固まったら、`Move` スキーマへの `effect` 追加は改めて判断する。

## 受け入れ条件

1. `engine.MoveEffect` と `Move.Effect *MoveEffect` があり、`MoveEffect.Validate()` が
   確率 1..100 の外・空の `Stages`・HP キー・変化量 0・変化量 -6..+6 の外を拒否する。
2. `Move.Effect` を設定しても・しなくても `CalcDamage` の結果(16 ロール・相性・STAB)が完全に一致する。
   `make test-golden` の期待値は1件も変わらない。
3. `master.DecodeMoveEffect` が未知のフィールド・大文字小文字違い・`accuracy`/`evasion` キー・
   範囲外の確率と変化量・変化量 0・空のオブジェクト・後続データ・オブジェクト以外を `ErrInvalidEffect` で拒否する。
4. `master.EncodeMoveEffect` が正準形(ゼロ値省略・struct 定義順・`Stages` はキー昇順)を返し、
   すべての有効な値で `Decode(Encode(e))` が元と等しい。
5. `master.MoveRow` が効果 JSON を受け取り、`master.Move()` が `engine.Move.Effect` に写す。
   効果が無い行は `Effect == nil`。不正な効果 JSON は行ごとエラーになる。
6. importer が Showdown のスナップショットから `MoveEffects` の行を作る。
   トップレベル `self.boosts` は確率 100 に正規化され、`accuracy`/`evasion` だけのエントリは警告つきで落ち、
   取得元に情報が無い技・`moves` 表に採らなかった技は行を作らず、2エントリ以上の技は blocker になる。
   出力は決定的(同じ入力で同じバイト列)。
7. `move_effects` 表の migration `000007` があり、up に `CREATE TABLE move_effects`、down に `DROP TABLE move_effects` がある。
8. pokedex-svc の `GET /internal/pokedex/master` が `MasterMove.effect` を返し、
   calc-svc の `master.FromExport` がそれを `engine.Move.Effect` にして `Store.Move()` から引ける。
   効果を持たない技は `effect: null` / `Effect == nil`。
9. balance の read model(`moves.json`)の形は変わらない(`moveId` / `type` / `category` の3つだけ)。

## 影響

- 変更: `engine/`(型の追加のみ。計算は不変)、`services/internal/master/`、`services/pokedex/db/migrations/`・`db/query/`、
  `services/pokedex/importer/`、`services/pokedex/internal/httpapi/master.go`、`services/calc/internal/master/export.go`、
  `api/openapi.yaml`(`MasterMove` に1フィールド)、`tools/importer/fetch-showdown.mjs`。
- `api/openapi.yaml` を触るので `make gen` を実行する。追加は任意フィールド1つで、
  既存の生成コード(`services/internal/api`・`web/src/api/openapi.gen.ts`)の利用側は変更なしでコンパイルできる。
- 判定レーンへ: JD1 の `Individual.ranks` 方式(ADR-0701)はそのままでよい。
  技 ID からランク変化を自動で出すには、公開 API への露出(決定8)がもう1段要る。
- タイプバランスレーンへの影響なし(決定7)。

## 追記(2026-09-23): 実データでの検証を受けて規則4・規則6 を修正

実装後、ピン留めした取得元の実データ(`make import-fetch`)で検証した結果、2点を修正した(独立レビュー指摘)。
ADR 本文(決定6・受け入れ条件6)は当時の設計判断の記録として書き換えず、ここに追記する。

**規則4(accuracy/evasion を落とす)の穴**: `atk` 等のキーと `accuracy`/`evasion` が同じエントリに混ざったとき、
実装が `stages` が空のときしか警告を積んでいなかった(黙って `accuracy` だけが消える)。決定6 が「前提が崩れたら
黙って落とさず人に知らせる」という考え方だったのに対し、この経路だけ無音だったのは一貫していなかった。
**修正**: `accuracy`/`evasion` を落としたら、他のキーが残るかどうかに関係なく常に `KindMoveEffectUnsupportedStat`
の警告を積む(`services/pokedex/importer/convert_move_effects.go`)。「調査」の実測(混在は0件)は変わらないが、
今後混在が現れても気づけるようにする。

**規則6(secondaries が2件以上で blocker)の前提崩れ**: 決定3 の前提「2エントリ以上の技は無い」は、
「ランク変化(boosts)を伴うエントリ」の件数としては実データでも成立していたが、決定6 は代理指標として
Showdown の `secondaries` 配列の**件数**をそのまま使っていた。実データでは firefang / icefang / thunderfang /
triplearrows の4技が `secondaries` を2件持つが、中身はいずれも `boosts` を持たない(火傷・氷結・麻痺・ひるみ等、
ランク変化ではない副次効果)。この4技は決定3 の前提を脅かしていないのに、代理指標の粗さのせいで
`import-dry-run` が毎回この4技を blocker として止め続ける(blocker の信号価値が下がる)。
**修正**: `secondaries` の**件数**を無条件に見るのをやめ、要素のうち `boosts` か `self.boosts` を持つものだけを
数えるようにした。`tools/importer/fetch-showdown.mjs` は取得元の `secondaries` 配列を(件数でなく)そのまま
`{self, boosts}` の配列として出す(ID化・正準化は Go 側という決定6 の方針どおり)。件数を数える判断そのものは
Go 側の `services/pokedex/importer/convert_move_effects.go`(`secondaryBoostCount`)に置き、
`ShowdownMove.Secondaries` の型を `int` から `[]ShowdownSecondary` に変更した(`snapshot.go`)。
boosts を持たない副次効果(状態異常・ひるみ等)は本 ADR の対象外として無視する。判定基準そのもの
(「ランク変化エントリが2件以上なら blocker」)は変えていない。

いずれも `docs/plan.md` P5-6 の実装(未マージ)への修正であり、別タスクは起票しない。
