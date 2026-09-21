# ADR-0013: タイプ相性表をデータ(マスタ)として engine に渡す

- 状態: 採用(2026-09-21、ユーザー決定)。実装は plan.md の P1-13(engine)と P2-2/P3-1(DB・calc-svc)
- 日付: 2026-09-21
- 関連: docs/audit-r1.md B-1、ADR-0002(マスタの取得元)、ADR-0005(データ駆動の効果定義)、ADR-0011(WASM 境界)、
  docs/coding-rules.md §2、CLAUDE.md ドメイン規約(リストをコードに埋め込まない)

## 背景

`engine/typechart.go` にタイプ相性表(18 タイプ × 18 タイプ)がコードとして埋め込まれている。一方、docs/requirements.md と ADR-0002 は
DB の `type_chart` テーブル(マスタ)を前提にしており、**どちらが正かを決めた ADR が無かった**(R-1 監査 B-1)。
ユーザーの方針は「ハードコードは修正が手間になるので、できるだけ変数(データ)にする」。

## 決定

### 1. タイプと相性は「マスタデータ」。engine は表を持たず、入力として受け取る

- **タイプの集合(ID)と相性(攻撃タイプ × 防御タイプ → 倍率)をマスタ**とする。DB の `types` / `type_chart` に登録し(P2-2)、
  calc-svc が起動時に読み込んで engine に渡す(P3-1)。WASM では、他のマスタと同様に、解決済みの値を JSON で渡す(ADR-0011)。
- engine は `TypeChart` 型(値)を定義し、`DamageInput` の入力の一つとして受け取る(場 `Field` と同じ「入力」の扱い)。
  engine は表の中身をコードに持たない。相性の引き方(複合タイプの掛け合わせ、無効の扱い)は**ルール**なのでコードのまま。
- **倍率は整数で表す**(既存の `num`/`den` の整数表現を保つ。float でダメージを近似しない)。
- 未知のタイプ(表に無い ID)が技・種族・テラスタイプに現れたら、検証エラーにする(黙って等倍にしない)。

### 2. 「データにするもの」と「コードにするもの」の線引き

| データ(マスタ・入力。変数として渡す) | コード(engine のルール。名前付き定数で持つ) |
|---|---|
| ポケモン・技・持ち物・特性 | ダメージ式・丸め順・補正の連鎖の手順(ADR-0004/0008) |
| **タイプの一覧・タイプ相性表** | 複合タイプの相性の掛け合わせ方・無効の扱い |
| 持ち物・特性の補正値(ADR-0005) | 天候・フィールド・壁などの機構の手順と補正値(ADR-0005) |
| レギュレーション(使用可能集合) | ゲーム機構の定数(Lv50・SP 上限・4096 基準。名前付き定数。R-2-4) |
| 防御プリセット等の「入力の作り方の型」(ADR-0009。engine が既定を持ち、上書き可) | |

原則: **ゲームの版やレギュレーションで変わりうる「表・一覧」はデータ、計算の「手順・式」はコード**。

### 3. テストとゴールデン

- engine のテストは、タイプ相性表を**テストデータ(fixture)から読む**。ゴールデン生成器(`tools/golden`)が oracle(@smogon/calc)の相性表を
  出力し、ゴールデンテストはそれを engine の入力に渡す(oracle と engine が同じ表で照合される。表そのものの正しさは oracle の責務)。
- 単体テストは、小さな表(数タイプ)の fixture で、複合タイプ・無効・未知タイプを検証する。
- 日本語のタイプ名は不要(ID と倍率だけ)。

### 4. 影響範囲(P1-13 で行う)

- engine: `TypeChart` 型、`DamageInput` への追加、`typechart.go` の表の削除(引き方だけ残す)、テストの fixture 化
- WASM 境界(`engine/wasmapi`): リクエストにタイプ相性表(解決済み)を追加。ADR-0011 の DTO・ベクタを更新
- `api/openapi.yaml`: calc API のリクエストにはタイプ相性表を載せない(サーバー側でマスタから解決する)。変更は無し(または説明の追記のみ)
- `tools/golden`: 相性表の出力を追加し、`testdata/golden` を再生成(期待値は不変。件数・sha256 の変化は理由をコミットメッセージに書く)
- importer / スキーマ(P2-2): `types` / `type_chart` テーブルと、oracle(Champions 世代)の相性表からの生成(実データは `data/generated/`。Git 管理外)

## P1-13 の具体設計

ここから下は P1-13(engine)の実装仕様。受け入れ条件(AC-1〜AC-8)は §P1-13.8。

### P1-13.1 公開 API

`engine/typechart.go` に置く。表の中身はコードに持たず、**検証済みの値**として受け取って引くだけ。

```go
// マスタから渡す素データ(検証前)。
type TypeChartData struct {
    Types         []Type                 // 表に載るタイプ ID。空・重複・TypeNone は不正
    Effectiveness map[Type]map[Type]int  // [攻撃タイプ][防御タイプ] = 倍率コード
}

// 検証済みの相性表(不変の値)。ゼロ値は「未設定」。
type TypeChart struct{ /* 非公開 */ }

func NewTypeChart(d TypeChartData) (TypeChart, error)
func (c TypeChart) IsZero() bool                 // 表が未設定か
func (c TypeChart) Types() []Type                // 定義順のコピー(呼び出し側が壊せない)
func (c TypeChart) Has(t Type) bool              // その ID が表にあるか
func (c TypeChart) Code(atk, def Type) (int, error)                        // 単一マッチアップ
func (c TypeChart) Effectiveness(atk Type, defTypes []Type) (Effectiveness, error)

// 相性の結果。倍率は整数比のまま持ち、float でダメージを近似しない。
type Effectiveness struct{ Num, Den int }
func (e Effectiveness) Multiplier() float64      // 表示用(0/0.25/0.5/1/2/4)
func (e Effectiveness) IsImmune() bool           // Num == 0
func (e Effectiveness) IsSuperEffective() bool   // Num > Den
```

- **倍率コードは「×2 した整数」**(現行の表と同じ): `0`=無効 / `1`=いまひとつ / `2`=等倍 / `4`=抜群。
  これ以外の値は `ErrInvalidTypeChart`。
- **等倍は省略できる**。`Effectiveness` に無い組は等倍(2)。ただし**キーに現れる ID は `Types` に含まれていなければならない**
  (省略は「等倍」、未知 ID は「エラー」。両者を混同しない)。
- `Num`/`Den` は約分しない(`den = 2^防御タイプ数`)。ダメージ式の `d * Num / Den` が現行と同じ整数演算になる。
- `Effectiveness` の導入で `num > den`(抜群判定)の重複が `IsSuperEffective` の1か所になる
  (`modifiers.go` の `superEffective` は自前で比較しない)。
- **旧 `func TypeEffectiveness(atk Type, defTypes []Type) (num, den int, mult float64)` は削除する**。
  表が無ければ答えられない関数をパッケージ関数として残さない(暗黙の表に戻る道を塞ぐ)。

`TypeNone`(`""`)の扱いは現行の挙動を変えない。

| 入力 | 結果 |
|---|---|
| 攻撃タイプが `TypeNone` | 等倍 `{1,1}`(エラーにしない。タイプなしの技) |
| 防御タイプ列の要素が `TypeNone` | その要素は飛ばす(掛けない) |
| 防御タイプ列が空 / 全て `TypeNone` | 等倍 `{1,1}` |
| `Code(atk, def)` のどちらかが `TypeNone` | `2`(等倍)、エラーなし |
| 表がゼロ値 | 常に `ErrTypeChartMissing`(`TypeNone` でも先に鳴る) |
| `TypeNone` でない未知の ID | `ErrUnknownType` |

### P1-13.2 入力への載せ方

`Field` と同じ「入力」として持たせる。パッケージ変数・`SetTypeChart` の類は作らない(§却下案)。

- `DamageInput.TypeChart TypeChart` を追加する。`CalcDamage` の引数・戻り値・他フィールドは変えない。
- `BulkInput.TypeChart` / `ReverseInput.TypeChart` を追加し、`CalcBulk` / `CalcReverse` が組み立てる
  `DamageInput` に**素通し**する(既定値を補わない。未設定のまま渡し、`CalcDamage` が鳴る)。
- `CalcBulk` / `CalcReverse` は表を自分で解釈しない(相性の判断は `CalcDamage` の中だけ)。
- `TypeChart` はポインタ1本ぶんの値にする(内部データは非公開ポインタ)。`DamageInput` は逆算の格子で
  何千回もコピーされるので、`map` を値で持たせない。

### P1-13.3 検証とエラー

sentinel は3つ。すべて `%w` で包み、呼び出し側は `errors.Is` で判別する(ADR-0010/0009 と同じ作法)。

| sentinel | 起点 |
|---|---|
| `ErrInvalidTypeChart` | `NewTypeChart` が拒否した定義(`Types` が空・重複・`TypeNone`、行/列のキーが `Types` に無い、コードが 0/1/2/4 以外) |
| `ErrTypeChartMissing` | 計算の入力に表が無い(`TypeChart` がゼロ値) |
| `ErrUnknownType` | 入力に現れたタイプ ID が表に無い |

`CalcDamage` は既存の `Attacker.Validate()` / `Defender.Validate()` の**後**に、表に対する検証を行う:

1. `in.TypeChart.IsZero()` なら `ErrTypeChartMissing`
2. 次の ID が `TypeNone` でなければ表にあること。無ければ `ErrUnknownType`
   - `Move.Type`
   - `Attacker.Species.Types[]` / `Defender.Species.Types[]`(両側。マスタ不整合をここで止める)
   - `Attacker.TeraType` / `Defender.TeraType`(engine はまだ使わないが、未知 ID を黙って通さない)

`Individual.Validate()` は**変えない**。個体だけでは表を知らないため、タイプ**数**(1〜2)の検証は従来どおり
`Validate` が、タイプ **ID** の検証は表を持つ `CalcDamage` が受け持つ(検証は責務ごとに置く。coding-rules §3)。

性能のため、`CalcDamage` は相性を**1回だけ**引いて `otherModifiers` に渡す(現行は `damage.go` と
`modifiers.go` で2回引いている)。`otherModifiers(in DamageInput, eff Effectiveness)` のように引数で渡す。

### P1-13.4 WASM 境界(ADR-0011 §13 も参照)

リクエストに `typeChart` を追加する。値は**整数コードのまま**で、等倍は省略できる(engine と同じ規則)。

```jsonc
TypeChart = {"types":["normal","fire",…],
             "effectiveness":{"fire":{"grass":4,"water":1,"ghost":0}, …}}   // 等倍(2)は省略可
```

- `calc` / `calcBulk` / `calcReverse` の3リクエストすべてが `typeChart` を**必須**で受ける。
  省略(または空)は `type_chart_missing`。黙って既定の表を使わない。
- エラーコードの対応(ADR-0011 §5 の表に3行追加):
  `type_chart_missing` ← `ErrTypeChartMissing` / `invalid_type_chart` ← `ErrInvalidTypeChart` /
  `unknown_type` ← `ErrUnknownType`。
  `types[]` の要素・`effectiveness` のキーは既存の Type 列挙検証(`invalid_enum`)も通る
  (18 タイプの綴り誤りは `invalid_enum`、綴りは正しいが表に無い ID は `unknown_type`)。
- **`engine/wasmapi` の `validTypes`(18 タイプの綴りの一覧)は残す**。これは相性表ではなく
  **境界の列挙検証**(ADR-0011 §4)で、タイプミスを早く止めるためのもの。相性(どのタイプが
  どのタイプに強いか)は持たない。engine の `Type` 定数と同じ「識別子の集合」であり、
  §決定1 の「engine は表を持たない」に反しない。タイプが増える版に対応するときは ADR-0011 §4 ごと見直す。
- ベクタ(`engine/wasmapi/testdata/vectors.json`)は **`schemaVersion` を 2 に上げ、`typeChart` を
  ファイルの先頭で1度だけ定義**する。各ベクタのリクエストには書かない(18×18 を 33 件ぶん書くと
  ファイルが数百 KB になり、差分が読めなくなる)。
  ネイティブ側(`engine/wasmapi/vectors_test.go`)・期待値生成(`engine/cmd/wasmexpect`)・
  Node ハーネス(`scripts/wasm-conformance.mjs`)が、**呼び出し直前に各リクエストへ注入する**。
  比較対象はレスポンスなので、注入が Go 側と JS 側で別実装でも**バイト一致の仕組みは壊れない**
  (リクエストのキー順序は結果に影響しない)。
  `schemaVersion` を上げることで、注入を実装し忘れた側は「未知の schemaVersion」で必ず失敗する。

### P1-13.5 ゴールデン

`tools/golden/generate.mjs` が oracle(@smogon/calc 0.10.0、gen9)の相性表を `testdata/golden/typechart.json` に出力する。
**表そのものの正しさは oracle の責務**で、engine は同じ表を渡されて同じ答えを出すことだけを見る。

```jsonc
{
  "schemaVersion": 1, "source": "@smogon/calc", "version": "0.10.0", "generation": 9,
  "note": "倍率は ×2 した整数コード(0=無効 / 1=いまひとつ / 2=等倍 / 4=抜群)。ADR-0013",
  "excludedTypes": ["???", "stellar"],
  "types": ["bug","dark",…],                     // 18件・ID 昇順(決定的にするため並べ替える)
  "effectiveness": {"bug":{"bug":2,"dark":4,…}, …}  // 18×18=324 件すべてを明示(生成物なので省略しない)
}
```

- oracle の `gen.types` は 20 件で、`???` と `Stellar` を含む(実測)。この2つは**除外**し、
  残りが engine の 18 タイプと一致することを生成器が `assert` する(将来 oracle が増えたら生成が止まる)。
- 倍率は oracle の `type.effectiveness`(攻撃側視点、値は 0 / 0.5 / 1 / 2)を**2倍して整数コード**にする。
  それ以外の値が出たら生成器が `assert` で止まる。
- `metadata.json` の `files` に `typechart.json` を追加する(`count` = 324 = 18×18、`sha256`)。
  **他のファイルの件数・sha256・期待値は変わらない**(engine に渡す表が同じである以上、ダメージは1件も変わらない)。
- ゴールデンテストは各ケースの `input` に**この表を注入**してから `CalcDamage` を呼ぶ
  (ゴールデン JSON の `input` に 324 件の表を 93,000 行ぶん書かない)。
- **単体テストも同じファイルを fixture として読む**(`make test`)。engine の相性の期待値を
  2か所に持たない(coding-rules §2「単一の正」)。読み込み時に `metadata.json` の sha256 を照合するので、
  手書きで差し替えると落ちる = 再生成が強制される。
- 代償: `make test`(タグ無し)が `testdata/golden/typechart.json` に依存する。
  fixture が無いときは**スキップせず明示的に落とす**(`TestTypeChartFixtureIsAvailable`)。

### P1-13.6 単体テストの fixture

「表の中身」ではなく「引き方(ルール)」を見るテストは、**小さな独立した fixture**(数タイプ)で書く。
実装の写しにしない(coding-rules §2 の例外)。

- 複合タイプ(4倍 / 1/4 倍)、無効の優先、等倍の省略、未知タイプ、`TypeNone`、表未設定(ゼロ値)。
- `NewTypeChart` の拒否(重複・空・未知キー・不正コード)。
- `Types()` の返り値と、渡した `TypeChartData` の map を後から書き換えても表が変わらないこと(不変性)。

### P1-13.7 性能

ダメージ計算のホットパス(逆算は格子 3,267 点 × 持ち物候補ぶん `CalcDamage` を呼ぶ)。
表の保持は **`map[Type]int` の添字 + フラットな `[]int8`** にする(`map[Type]map[Type]int` の二段引きにしない)。

実測(darwin/arm64、複合タイプ1回の引き当て、この環境):

| 実装 | 1回 |
|---|---|
| 現行の `map[Type]map[Type]int` 二段引き | 25.9 ns |
| 添字 `map[Type]int` + フラット `[]int8` | 14.0 ns |

- 逆算1回(6,534 回の `CalcDamage`、相性は1回に統一して 6,534 回の引き当て)で **約 0.09 ms**。
  ADR-0011 §9 の実測(逆算 16〜17 ms)に対して 1% 未満で、現行より速くなる。
- `make test` の engine パッケージは現状 4.2 秒(実測)。この変更で目立って増えないこと。
  全種族テスト(`-tags allspecies`)・ゴールデンも同様。
- `NewTypeChart` は起動時・リクエスト境界で1回だけ呼ぶ想定(1リクエストごとに 324 件を検証するのは
  WASM でも μs オーダーだが、Web 側は解決した表を使い回す)。

### P1-13.8 受け入れ条件と担当テスト

| AC | 内容 | 担当テスト |
|---|---|---|
| AC-1 | engine のコードにタイプ相性表(18×18 の値)が無い。相性は入力の `TypeChart` からのみ引く | `TestNoTypeChartTableInEngineSource` / `TestCalcDamageUsesSuppliedTypeChart` |
| AC-2 | 引き方のルールは engine のまま: 複合タイプは掛け合わせ、無効は他を無視して 0、等倍の省略は等倍、`TypeNone` は現行どおり | `TestTypeChartEffectivenessRules` / `TestTypeChartCodeAndNone` |
| AC-3 | 倍率は整数比のまま(`Num`/`Den`)。ダメージ結果は表を渡すだけでは**変わらない**(既存の期待値が全件そのまま通る) | `TestTypeChartFixtureMatchesKnownMatchups` / 既存の `damage_test` 他 + `make test-golden` |
| AC-4 | 表が未設定なら `ErrTypeChartMissing`。`CalcDamage` / `CalcBulk` / `CalcReverse` のいずれも黙って計算しない | `TestCalcDamageRequiresTypeChart` / `TestCalcBulkRequiresTypeChart` / `TestCalcReverseRequiresTypeChart` |
| AC-5 | 表に無いタイプ(技・種族・テラス)は `ErrUnknownType`。等倍にフォールバックしない | `TestCalcDamageRejectsUnknownType` |
| AC-6 | 不正な表は `NewTypeChart` が `ErrInvalidTypeChart` で拒否する。検証済みの表は不変 | `TestNewTypeChartRejectsInvalidData` / `TestTypeChartIsImmutable` |
| AC-7 | `BulkInput` / `ReverseInput` は表を `DamageInput` へ素通しする(自分で解釈しない) | `TestCalcBulkPassesTypeChartThrough` / `TestCalcReversePassesTypeChartThrough` |
| AC-8 | WASM 境界は `typeChart` を必須で受け、engine の素通しのままである。ベクタは先頭で1度だけ表を定義する | `TestCalcRequiresTypeChart`(wasmapi)/ `TestTypeChartEnvelopeCodes` / 既存の `TestCalcMatchesEngineCalcDamage` / `TestVectorsDefineSharedTypeChart` |

### P1-13.9 移行手順(implementer 向け)

1. `tools/golden` に相性表の出力を足し、`npm run generate` で `testdata/golden/typechart.json` と
   `metadata.json` を再生成する(**他のファイルの sha256 が変わらないこと**を確認。変わったら止めて報告する)
2. `engine/typechart.go` を新 API に置き換え、埋め込み表と `TypeEffectiveness` を削除する
3. `DamageInput` / `BulkInput` / `ReverseInput` に `TypeChart` を足し、`CalcDamage` の検証と
   `CalcBulk` / `CalcReverse` の素通しを実装する。相性の引き当ては `CalcDamage` 内で1回にする
4. `engine/wasmapi` に `typeChartDTO` と3つのリクエストの `typeChart`、エラーコード3種を足す
5. `engine/cmd/wasmexpect` と `scripts/wasm-conformance.mjs` に、ベクタ先頭の `typeChart` を
   各リクエストへ注入する処理を足す(`schemaVersion` 2)
6. `make test` / `make test-golden` / `make test-all-species` / `make test-wasm` を通す

## 却下案

- **engine にハードコードしたまま**: ユーザーの方針(修正が手間)に反する。要件・ADR-0002 の DB 前提とも矛盾する。
- **パッケージ変数に注入する**(`SetTypeChart(...)`): 暗黙の状態になり、並行呼び出しとテストの独立性を損なう。入力として渡す。
- **表が未設定なら既定(第9世代)の表にフォールバックする**: 呼び出し側が渡し忘れても動いてしまい、
  「engine は表を持たない」が実質的に崩れる。未設定は `ErrTypeChartMissing`(P1-13.3)。
- **未知のタイプを等倍として扱う**: 現行の挙動だが、マスタの綴り誤りが「等倍のダメージ」という
  もっともらしい答えになって表に出ない。`ErrUnknownType` で止める(§決定1)。
- **ベクタの各リクエストに表を書く**: 18×18 × 33 件でベクタが数百 KB になり、差分が読めなくなる。
  先頭で1度だけ定義して注入する(P1-13.4)。
- **単体テスト用に engine 内へ「正しい表」を fixture として持つ**: 削除したはずの表がテスト側に復活する。
  oracle が出す `testdata/golden/typechart.json` を単体テストでも使う(P1-13.5)。

## 限界

- 18 タイプ・固定の値域(0 / 1/4 / 1/2 / 1 / 2 / 4)を前提にした整数表現。将来、別の倍率が入るゲーム改変があれば、整数表現の見直しが要る。
- WASM に表を渡す分、リクエストが少し大きくなる(18 × 18 の整数表で数百バイト程度)。マスタと同様に Web 側で1度だけ解決して使い回す。
- P1-13 が終わるまで、engine は現行のハードコード表のまま動く。
