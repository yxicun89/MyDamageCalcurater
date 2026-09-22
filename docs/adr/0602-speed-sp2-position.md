# ADR-0602: 素早さ比較 SP2 自分のポケモンの位置

- 状態: 採用(2026-09-22。§1 はユーザー決定・回答。§2 以降は素早さレーンの判断)
- 日付: 2026-09-22
- 関連: docs/speed-design.md §6、ADR-0600(計算コア・read model)、ADR-0601(表・プリセット)

## 背景
SP1 で全体の表(6 行 × 使用可能な全ポケモン)ができた。SP2 では、自分のポケモンの素早さがその表のどこに入るかを返す API を作る。

## 決定

### 1. ユーザー決定・回答(2026-09-22)
- 最小の選択: ポケモンを選び、「無振り / 準速 / 最速」を選んで、こだわりスカーフの on/off を切り替えるだけ。
- オプション: 素早さ SP 0〜32・性格の補正 3 通り・ランク -6〜+6・スカーフ on/off を自由に選ぶ。または実数値を直接入力して位置だけを見る。
- 結果: 自分の実数値と、表の中の位置(自分より速い行・同速の行・遅い行の境目)。同速なら明示する。

### 2. 入力の3つの型
1 つの endpoint `POST /api/speed/v1/position` を、`mode` で分ける(oapi-codegen の oneOf は複雑になるため、1 つのスキーマに全フィールドを持たせ、
HTTP 層が `mode` ごとに要る・要らないフィールドを検査する)。

| mode | 要るフィールド | 用途 |
|---|---|---|
| `preset` | `pokemonId`・`preset`(`uninvested`/`neutral-max`/`max`)・`scarf` | 最小の選択。SP1 の6行の定義のうち最初の3つ(ADR-0601 §2)の SP・性格・ランクを使い、スカーフだけ独立に on/off にする(SP1 の `max-scarf` は最速固定だが、SP2 では無振り・準速にもスカーフを乗せられる) |
| `custom` | `pokemonId`・`sp`(0〜32)・`nature`(`minus`/`neutral`/`plus`)・`rank`(-6〜+6)・`scarf` | オプションの自由入力(種族値は `pokemonId` から) |
| `raw` | `value`(実数値そのもの)。`pokemonId` は任意(表示用の名前・タイプだけに使う) | 実数値の直接入力 |

指定した mode に要らないフィールドが 1 つでも入っていたら 400 `invalid_request`(取り違えたリクエストをそのまま通さない)。

### 3. 計算
- `preset`/`custom`: `speed.Input{BaseSpeed: pokemon.BaseSpeed, ...}` を組み立てて `speed.Speed` を呼ぶ(ADR-0600 §3 のまま。式を複製しない)。
  `preset` の SP・性格・ランクは `speed.Presets()` の `uninvested`/`neutral-max`/`max` の定義をそのまま使う(1 か所の定義を再利用。ADR-0601 §2 と同じ値)。
- `raw`: `Speed` を呼ばず `value` をそのまま使う。範囲は engine の式から導ける理論上の最小・最大(種族値 1〜255・SP 0〜32・性格3通り・ランク -6〜+6・スカーフ有無の全組み合わせの中の最小/最大)を
  `speed.Speed` 自身で計算して定数にする(ハードコードしない。coding-rules §2)。外なら 400 `invalid_request`。
- 位置: SP1 と同じ `speed.BuildTable(roster, 全6プリセット)` で作った表に対して、自分の値を比べる。
  - `faster`: 自分より値が大きい段の行数の合計。
  - `slower`: 自分より値が小さい段の行数の合計。
  - `tie`: 自分と同じ値の段があれば、その段の行(ポケモン × プリセット)の一覧。無ければ空配列(空 = 同速なし。真偽値を別に持たない)。
- `pokemonId` は openapi の `PokemonId` と同じ形式(`^\d{4}-\d{3}$`)を HTTP 層で検査する(balance の `validatePokemonID` と同じ形。ADR-0014)。
  **形式が不正なら 400 `invalid_request`**、形式は正しいが read model に無ければ 422 `unknown_pokemon`(balance の ADR-0014 と同じ考え方。新しい ErrorCode)。
  raw で `pokemonId` を渡したとき(表示用の解決だけ)も同じ扱い。

### 4. API
`POST /api/speed/v1/position` → 200
```json
{"speed": 301, "pokemon": {"pokemonId": "9001-000", "nameJa": "...", "types": ["fire"], "baseSpeed": 100},
 "faster": 12, "slower": 30, "tie": [{"pokemonId": "9002-000", "...": "...", "preset": "max"}]}
```
`pokemon` は `pokemonId` を渡したときだけ含む。

判定順: ヘッダー(400)→ body のサイズ(4 KiB 超は 413 `request_too_large`。balance の `MaxBytesReader`/`decodeJSONBody` と同じ形。
妥当な body はスカラーのフィールドだけで 200 バイト未満のため、十分な余裕を持たせつつ際限なく大きい body を読み込まない上限)→
body の JSON・mode ごとの必須フィールド・pokemonId の形式・範囲(400 `invalid_request`)→ read model 未設定(503 `master_unavailable`)→
`pokemonId` が read model に無い(422 `unknown_pokemon`)→ 200。それ以外(計算・表の組み立てのエラー)は 500 の固定文言。

### 5. 細部
- `preset` の3つの ID(`uninvested`/`neutral-max`/`max`)は、SP1 の `PresetId` の enum のうちスカーフを含まない3つ。OpenAPI では新しい `MinimalPresetId` の enum にする
  (SP1 の `PresetId` そのままだと `max-scarf` 等も選べてしまい、`scarf` フィールドと二重に指定できてしまうため)。
- `custom` の `nature` は ADR-0600 §3 の `NatureEffect`(`minus`/`neutral`/`plus`)と同じ enum・同じ意味。
- `tie` の要素は SP1 の `SpeedTableEntry` をそのまま再利用する(新しい型を作らない)。
- 空の roster(SP1 critic の軽微。ADR-0601 では read model が空を拒否するので起きない)は SP2 でも同じ前提のまま。

## 却下した案
- `oneOf` で3つの別スキーマにする: oapi-codegen の生成型が複雑になり、HTTP 層の分岐も結局要る。フラットな1つのスキーマに `mode` で足りる。
- 位置を「自分より速いポケモンの数(重複ポケモンを1体と数える)」にする: ユーザーの仕様は「行」(SP1 の6行の表)の中の位置なので、行数で数える。
- 実数値の範囲をハードコードした定数にする: engine の式から `speed.Speed` 自身で導出し、値の変更(ADR-0600 §3 の式が変わったとき)に追従できるようにする。
