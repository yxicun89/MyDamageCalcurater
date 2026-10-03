# ADR-0250: 調整 API の契約(HTTP `/api/calc/adjust/*` と WASM 境界)

- 状態: 採用(2026-10-02。plan.md AJ4)
- 日付: 2026-10-02
- レーン: ダメージ計算(API 帯 `0200〜`。直近の `0217` は他ブランチが使うため、調整機能の API 側を `0250〜` に置く)
- 関連: ADR-0150(調整の指数・16n・最小 SP 探索・SP 配分。engine の定義)、ADR-0200(calc-svc の API 契約・エラー語彙)、
  ADR-0011(WASM 境界)、ADR-0108 / ADR-0208(件数・範囲の上限と、HTTP / WASM の検査順)、ADR-0123 / ADR-0215(未対応の印)、
  ADR-0202(gateway のルーティング)、ADR-0212(計算イベント)、CLAUDE.md 絶対ルール1・2・5

## 背景

AJ0〜AJ3 で engine に5つの純粋関数(`FirepowerIndex`・`BulkIndex`・`HPLines`・`MinSPToKO`・`MinSPToSurvive`・
`SuggestSPAllocation`)を置いた(ADR-0150)。AJ4 はこれを API 契約(`api/openapi.yaml`)・WASM 境界(`engine/wasmapi`)・
calc-svc に出す。ADR-0150 §8 は「`Ceiling` のゼロ値は『振らない』で、既定 32 を補うのは AJ4 の境界の責務」とし、
§結果 は「補正の組み立て(タイプ一致・持ち物)の渡し方を AJ4 で API 契約として決める」としている。

## 決定

### 1. エンドポイントは「1操作1エンドポイント」、置き場所は `/api/calc/adjust/*`

| 操作(operationId) | HTTP | WASM(`globalThis.pokecalc.*` / `wasmapi.*`) | engine |
|---|---|---|---|
| `adjustIndices` | `POST /api/calc/adjust/indices` | `adjustIndices` / `AdjustIndices` | `RealStats`・`FirepowerIndex`・`BulkIndex`(物理・特殊)・`HPLines` |
| `adjustMinSpToKo` | `POST /api/calc/adjust/min-sp-to-ko` | `adjustMinSpToKo` / `AdjustMinSPToKO` | `MinSPToKO` |
| `adjustMinSpToSurvive` | `POST /api/calc/adjust/min-sp-to-survive` | `adjustMinSpToSurvive` / `AdjustMinSPToSurvive` | `MinSPToSurvive` |
| `adjustAllocation` | `POST /api/calc/adjust/allocation` | `adjustAllocation` / `AdjustAllocation` | `SuggestSPAllocation` |

- `mode` 付きの1本にしない。応答の形が操作ごとに違い(KO は1能力、耐久は2能力の組、配分は2つの案)、1本にすると
  応答が `oneOf` になって3クライアント(Go / TS / Swift)の生成型が扱いにくい。既存の calc・bulk・reverse も1操作1本。
- 指数の3関数は1本にまとめる。どれも「自分の個体1体」だけで決まり、探索を伴わず、画面(AJ6)が同時に出すため。
- 倒す / 耐えるを `side` 付きの1本にしない理由は上の応答の形と同じ(逆算の `side` は応答の形が同じだから1本でよかった)。
- `/api/calc/` の下に置くので、**gateway のルーティングは変えない**(`/api/calc` と `/api/calc/*` は既に calc-svc へ転送する。
  ADR-0202 §3)。gateway のテストは新パスが calc の上流に届くことと、実物の calc-svc を通した応答が契約どおりであることを固定する。

### 2. 個体・技・場の指定は既存の表現を再利用する

- HTTP の個体は `Individual`(`speciesKey`・`natureId`・`sp` 必須、`abilityId`・`itemId`・`ranks`・`teraType`・`status` 任意)。
  種族・技・性格・持ち物・特性はマスタ(Store)から引き、無ければ既存の `unknown_*`。技は `moveId`、場は `FieldState`、
  急所は `CalcOptions.critical`。WASM は既存の個体・技・場の DTO(解決済みの実体)と `typeChart`。
- 探索系(min-sp-to-ko / survive)は `AdjustSearchRequest` を共有する。`attacker` / `defender` の意味はダメージ計算と同じで、
  **どちらが自分かは操作で決まる**(ko は attacker、survive は defender)。engine の `AdjustSearchInput` と同じ。
- 探索する能力の SP(ko の atk / spa、survive の hp と def / spd)は**無視する**(engine が上書きする)。
  したがって境界は自分の個体を `Validate` で**先に検査しない**(探索する能力に 32 が入っていて合計 66 を超えても、
  上書き後に収まるなら成功する)。検査は engine の `ErrInvalidAdjustInput` に一本化する。
- 配分の `self.sp` は下限。`ceiling`(`AdjustCeiling`)は能力ごとに省略できる。
- 指数の補正(`modifier` / `damageModifier`)は **4096 基準の整数をクライアントが組み立てて渡す**(タイプ一致・持ち物・特性・
  天候の倍率の掛け合わせ)。境界はマスタから補正を組み立てない。補正の組み立ては「どの効果を指数に含めるか」の
  判断で、ダメージ式の再実装に近く、境界に置くと HTTP と WASM で二重に持つことになるため(ADR-0150 §結果)。
  個体の `itemId`・`abilityId`・`ranks`・`status` は指数に使わない(解決はするので未知の ID は `unknown_*`)。

WASM のリクエスト(個体・技・場は既存の DTO。HTTP の `moveId` / `opponent` などの ID は解決済みの実体になる):

| 関数 | リクエストのキー |
|---|---|
| `adjustIndices` | `individual`、`move`(省略可)、`modifier`、`damageModifier`(**`typeChart` を持たない**) |
| `adjustMinSpToKo` / `adjustMinSpToSurvive` | `format`、`attacker`、`defender`、`move`、`field`、`critical`、`hits`、`thresholdPercent`、`typeChart` |
| `adjustAllocation` | `self`、`ceiling`、`mode`、`focus`、`offenseCategory`、`minSpeed`、`goal`(`format`・`opponent`・`move`・`field`・`critical`・`hits`・`thresholdPercent`)、`typeChart` |

応答は HTTP と**同じキー・同じ形**(調整の応答は `natureId` のようなマスタ由来の項目を持たないので、JSON として等しい)。
Go/WASM 一致テストのベクタは先頭の `typeChart` を各リクエストに注入するが、`adjustIndices` には注入しない
(`engine/cmd/wasmexpect` と `scripts/wasm-conformance.mjs` の両方)。

### 3. 省略可の項目と既定(engine のゼロ値をそのまま渡さない)

| 項目 | 省略 / null | 明示した 0 | 根拠 |
|---|---|---|---|
| `adjustIndices.moveId` | `firepowerIndex: null` | — | 技なしでも耐久・HP ラインは出せる |
| `modifier` / `damageModifier` | 4096 | 400 `invalid_input` | engine は 0 を拒否する。HTTP / WASM とも「キーの有無」で区別する |
| `thresholdPercent` | 100(確定) | 400 `invalid_input` | engine は 0 を「既定」と読むが、契約は (0, 100] とし、0 を意味の違う値にしない |
| `ceiling.<stat>` | 32 | 0(下限より上に振らない) | ADR-0150 §8。境界が補う |
| `minSpeed` | 0(目標なし) | 0(同じ) | engine と同じ |
| `goal` | `minSp: null`・`unsupported: []` | — | engine の `Goal == nil` |
| `focus`(bulk)/ `offenseCategory`(offense) | 400 `invalid_input` | — | 既定を置くと意図しない最適化を黙って返すため必須 |
| `hits` | 必須 | 400 `invalid_input` | 「確定 n 発」の n は使用者が決める量 |

WASM の DTO もこれらを「キーの有無」で区別する(ポインタで受ける)。同じ入力は HTTP と WASM で同じ数値・同じ code になる。

### 4. 列挙とエラーコード(新しい code は足さない)

- `mode`(`AllocMode`: bulk / offense)・`focus`(`BulkFocus`: physical / special / both)・`offenseCategory`(`MoveCategory`)・
  `format`・`field.*`・`status`・`teraType` の未知の値は 400 `invalid_enum`(境界で検査。HTTP / WASM 同じ)。
  `offenseCategory: status` は列挙としては正しいので engine が拒否し `invalid_input`。
  `mode` は `format` と同じく必須の列挙なので、欠落も `invalid_enum`。`focus` / `offenseCategory` は使う側でだけ必須で、
  欠落は engine が拒否して `invalid_input`(使わない側では値があれば列挙だけ検査し、中身は読まない)。
- engine の `ErrInvalidAdjustInput` は **`invalid_input`**(ADR-0208 §2 と同じ考え方。語彙を増やさない)。HTTP の
  `engineSentinels` と wasmapi の `errorResponse` の両方に足す。
- 変化技・威力 0 の技での探索・指数は `invalid_input`(逆算の `ErrMoveDealsNoDamage` と同じ扱い)。
  タイプ相性で無効なときは**エラーにしない**(`feasible: false`。ADR-0150 §7)。
- 相性表の不足は既存どおり(HTTP では起動時のマスタの不備なので 500 `type_chart_missing`、WASM はリクエストの
  `typeChart` の欠落で `type_chart_missing`)。
  `adjustIndices` は相性表を使わないので WASM のリクエストに `typeChart` を持たない(送れば `unknown_field`)。
  `adjustAllocation` の `typeChart` は `goal` があるときだけ必須で、`goal` が無ければ読まない。

### 5. 探索量の上限(ADR-0208 の流儀)

調整のリクエストは配列を持たないので件数の上限は無い。仕事量は engine の定数で上から抑えられる。

| 操作 | 上限内の最大の仕事量 |
|---|---|
| indices | 定数(`RealStats` 1回 + 指数3つ + HP ライン 33 点) |
| min-sp-to-ko | `CalcDamage` 33 回 + n 発の確率(`hits` ≤ 10) |
| min-sp-to-survive | `CalcDamage` 最大 33² = 1,089 回 + 各組の n 発の確率 |
| allocation(bulk) | 最大 33³ = 35,937 候補(`CalcDamage` は (H, B/D) の実数値で使い回し最大 1,089 回)+ 各候補の n 発の確率 |
| allocation(offense) | 33² = 1,089 候補 |

- **`hits` は 1..10**(engine の `MaxAdjustHits`)を契約の `minimum` / `maximum` に書き、境界は `hits`・`thresholdPercent`・
  `ceiling` の値域・`minSpeed` の符号・`modifier` / `damageModifier` の値域(明示した値は `MinEffectModifier`..`MaxEffectModifier`。
  `adjustIndices` の `moveId` / `move` を省略しても検査する)を **ID の解決・DTO の変換より前に** 検査して `invalid_input` にする
  (ADR-0108 決定3・ADR-0208 §3 の検査順。上限違反とほかの違反が重なったら上限違反が先に出る。HTTP と WASM で同じ)。
- 個体ごとの列挙(`status`・`teraType`)は個体の ID 解決と同時に検査する(既存の `CalcDamage` と同じ)。
- HTTP の探索系(min-sp-to-ko / min-sp-to-survive)は `field` を個体の解決より前に parse する(意図。場の列挙は
  ID を引かずに検査できるので、`format` と同じく列挙の段で先に見る)。
- 実測(2026-10-02、ネイティブ Go、HP 種族値 255・B/D 230 の自分に威力 10〜150・hits 1〜10・しきい値 50% の耐久側配分):
  17〜280 ms。gateway の上流タイムアウト 10 秒(ADR-0202)に対して十分小さい。実機の WASM で重いとわかったら、
  まず engine で n 発の確率を (H, B/D) の組ごとに使い回す(契約は変えない)。それでも足りなければ `hits` の上限を下げる
  (契約を狭める変更なので ADR を足す)。
- calc-svc に上限ちょうどのベンチマーク(`BenchmarkAdjustAllocationAtLimit`)を置き、退行を見る(壁時計の閾値は判定しない)。

### 6. 応答

- 探索系の結果は engine の結果の写し。確率は engine の生値(`chancePercent`、%、丸めない。ADR-0006)で、表示用の
  0.1% 単位の値は出さない(探索の結果は「満たすか」と SP が主で、確率は補助。AJ6 で要れば足す)。
- `unsupported` は常に配列(印なしは `[]`、null にしない)。値は `UnsupportedMark`(ADR-0215 の開いた文字列)。
- 配分の `minSp` は `goal` が無ければ `null`(キーは必ず出す)。`maxIndex` / `minSp` の各組は SP・合計・実数値・
  等倍の物理/特殊耐久指数・`speedMet`・`goalMet`・`chancePercent`。
- 指数は `int64`。現実のマスタの威力(最大 250 程度)と補正の上限 2^21 で 2^53 未満に収まり、JS でも精度を失わない。
- `HPLineReport.current` は `none` / `16n` / `16n-1`(engine の空文字は `none` に写す)。次・前のラインは無ければ null。
- 生成コードの定数名: `BulkFocus` は `MoveCategory` と同じ値(physical / special)を持つので、`x-enum-varnames` で
  `BulkFocusPhysical` などを明示する(付けないと oapi-codegen が衝突を避けて既存の `api.Physical` などを
  `api.MoveCategoryPhysical` に改名し、pokedex・gateway のテストが壊れる)。`AllocMode` も同じ理由で型名を前に付ける。

### 7. 計算イベントは発行しない

調整の4操作は `calcevents` を発行しない。調整は「自分の育成」の試行で「よく使う相手」の集計(ADR-0212 §7.1)に
寄与せず、`Operation` の値を増やすと record / team の消費側と calcevents の golden を変えることになる。
計算はステートレスで、record / team / NATS が落ちていても成功する(絶対ルール5)。

### 8. calc-svc の配線

- 4操作を生成ラッパ(`api.ServerInterfaceWrapper`)経由で `registerCalcRoutes` と `registerDeferredCalcRoutes` の
  両方に登録する(マスタ準備中は 503 `master_unavailable`。ADR-0204 §3)。
- pokedex / record / team は生成物 `api.ServerInterface` を満たすためだけに 404 `not_found` のメソッドを持つ(既存と同じ)。

## 結果

- Web(AJ6)は WASM、iOS(AJ7)は HTTP で同じ結果を得る。HTTP と WASM のパリティは calc-svc のテストが固定する。
- 補正の組み立てはクライアントの責務になる。Web / iOS で同じ組み立てを2回書くことになるが、どの効果を指数に
  含めるかは表示の選択であり、engine の計算(倒せるか・耐えるか)は探索 API がダメージ式で判定するので結果はぶれない。
- `/api/calc/adjust/*` は gateway の既存の前方一致に乗るため、ルーティングの変更・NetworkPolicy の変更は要らない。
