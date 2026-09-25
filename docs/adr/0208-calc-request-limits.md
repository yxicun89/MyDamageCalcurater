# ADR-0208: calc の候補・観測件数の上限

- 状態: 採用(2026-09-23。issue #110 の API レーン担当部分。critic PASS。実 HTTP で境界値と issue の再現手順(2,000×2,000 が 9.43 秒 → 0.9 ms・engine 未到達)を確認済み)
- 日付: 2026-09-23
- 関連: issue #110、ADR-0200(calc-svc の API 契約・エラー語彙)、ADR-0009(一括計算)、ADR-0010 §R(逆算)、
  ADR-0011(WASM 境界)、ADR-0202(gateway の上流タイムアウト 10 秒)

## 背景

Codex のセキュリティレビュー(監査ベース `264ae5963bb7e867a88ff5c79fade257a10edeb4`)で、
`POST /api/calc/bulk` と `POST /api/calc/reverse` の配列に件数上限が無いことが指摘された。

- 契約に `maxItems` / `uniqueItems` が無い(`presets` / `itemVariants` / `itemCandidates` / `observations`)。
- calc-svc は本文全体を 1MiB に制限する(ADR-0200 の critic 指摘 R7)が、**配列の件数は見ない**。
- engine は `presets × itemVariants` の全行(`engine/bulk.go`)、
  `itemCandidates × 33 SP × 2 性格クラス × observations × 16 rolls`(`engine/reverse.go`)を計算する。
  `maxCandidates` は**並べた後に切るだけ**なので、1 件しか返さなくても計算量は減らない。
- 結果として、1MiB 未満(再現手順では約 62KB)の本文で CPU・割当・応答サイズを増幅できる。
  issue のローカル測定では候補・観測が各 2,000 件で約 9.43 秒。
  gateway の上流タイムアウト 10 秒(ADR-0202)を超えても calc 側の計算は続きうる。
- 認証が無い公開 API(CLAUDE.md 技術規約)で、calc-svc は 1 Pod・CPU limit 200m。低コストな DoS になる。

## 決定

### 1. 上限の値(契約に書く)

| 場所 | 制約 | 上限内の最大の仕事量 |
|---|---|---|
| `BulkCalcRequest.presets` | `maxItems: 8` + `uniqueItems: true` | `DefenderPreset` の enum は 8 値なので「全種類を1回ずつ」が上限 |
| `BulkCalcRequest.itemVariants` | `maxItems: 64` + `uniqueItems: true` | 行数 = 8 × 64 = **512 行** |
| `ReverseRequest.itemCandidates` | `maxItems: 64` + `uniqueItems: true` | 候補 = 2 性格クラス × 64 = **128 件** |
| `ReverseRequest.observations` | `maxItems: 16`(`minItems: 1` は据え置き) | ダメージ計算 2 × 64 × 33 SP = 4,224 回、照合 4,224 × 16 観測 × 16 rolls |
| `ReverseRequest.maxCandidates` | `minimum: 0`(据え置き)+ `maximum: 128` | 0 は従来どおり「許可された入力から生まれる候補の全件」 |

値の妥当性(issue の既定案どおりにした理由):

- 想定する使い方は「代表持ち物は攻撃側 5 分類」「候補は高々十数件」(`web/src/domain/requests.ts`)で、
  64 は現実の UI の 4〜5 倍の余裕がある。将来レギュレーションで持ち物分類が増えても当面足りる。
- 観測は「同じ技・同じ場での別々の1発」で、16 発も撃てば SP は十分に絞れる(ADR-0010 §R)。
- `maxCandidates` の上限 128 は「2 性格クラス × itemCandidates の上限 64」と一致させた。
  返却件数の上限が入力から作れる候補数を超える意味は無いので、契約として同じ数にした。
  0(全件)を残すのは issue の受け入れ条件どおりで、既存クライアントの互換のため。
- 実測(2026-09-23、Apple M5 Pro、架空マスタ、`go test -bench AtLimit`):
  上限ちょうどで bulk が約 3.0ms / 7.9MB alloc、reverse が約 25ms / 57MB alloc。
  issue の 2,000×2,000(約 9.4 秒)から約 375 分の 1 で、gateway の 10 秒タイムアウトに対しても十分小さい。
  CPU limit 200m でも 1 リクエストあたり 0.2 秒未満に収まる見込み。
  **上限は「契約で許す最大」であって推奨値ではない**。実機で reverse の 25ms が重いとわかれば、
  `itemCandidates` を 32 に下げる余地を残す(契約を緩める方向ではないので後方互換で変更できる)。

### 2. 上限超過・重複の code とステータス

すべて **400 `invalid_input`**(ADR-0200 の語彙表のまま)。新しい ErrorCode は足さない。

- ADR-0200 の `invalid_input` は「入力検証(SP の範囲・合計、ランク、レベル…)」で、
  「契約で決めた範囲を外れた入力」という意味がすでにある。件数上限と重複はその一種。
- 語彙を増やすと WASM 境界(`engine/wasmapi` の `Code*`)と ADR-0200 の対応表、
  Web・iOS の文言をすべて増やすことになる。クライアントは message を見れば理由がわかる。
- `api/openapi.yaml` の `ErrorCode` の表では `invalid_input` の行に
  「候補・観測の件数上限(`maxItems`)超過、持ち物候補の重複(`uniqueItems`)、`maxCandidates` の範囲外を含む」と追記した。

例外が1つある。**`presets` の重複は既存どおり `duplicate_preset`**(engine の sentinel)。

- ADR-0200 §1.1 と既存テスト `TestCalcBulkErrors`「重複した preset」が `duplicate_preset` を固定しており、
  ここを `invalid_input` に変えると既存テストを弱めることになる(CLAUDE.md 絶対ルール6)。
- `duplicate_preset` は `uniqueItems` 違反に対する**より具体的な** code で、WASM 境界とも一致している。
- ただし**件数(9 件以上)の検査が先**なので、9 件以上の `presets` は重複を含んでいても `invalid_input` になる。
  8 件以内の重複だけが `duplicate_preset` に到達する(検査の順序は §3)。

### 3. calc-svc は自前で検証する(生成ラッパに任せない)

調査結果(実際の生成物と実行で確認。ドキュメントの記述ではない):

- `services/internal/api/cfg.yaml` は `echo5-server` + `strict-server` + `embedded-spec` を生成するが、
  **リクエスト本文のスキーマ検証は生成されない**。`ServerInterfaceWrapper.CalcBulk` /
  `CalcReverse`(`services/internal/api/openapi.gen.go`)が行うのはヘッダ `X-Device-Id` /
  `X-Session-Id` の存在と個数の検証だけで、本文には触れない。
- さらに httpapi は strict-server 側(`strictHandler`。本文を `echo.BindBody` するだけで、やはり検証はしない)を使わず、
  `api.ServerInterface` を直接実装して自前の `decodeStrict` で読む(ADR-0200)。
  つまり `maxItems` / `uniqueItems` / `maximum` を見る層が**どこにも無い**。
- 実測: 上限を契約に足して `make gen` した直後の時点で、`itemVariants` 65 件の要求は 200 で成功した
  (`TestCalcBulkRequestLimits/itemVariants_65_件`)。生成物の差分はコメント(description)だけで、
  Go の型・検証コードは増えない。TypeScript も同じ(コメントのみ)。
- 代替案の `middleware.OapiRequestValidator`(kin-openapi でリクエスト全体を検証する)は採らない。
  ADR-0200 が「同じ失敗は HTTP と WASM で同じ code」を最優先しており、kin-openapi に任せると
  未知フィールド・列挙・観測などの既存の code(`unknown_field` / `invalid_enum` / `invalid_observation` …)が
  kin-openapi の語彙に化ける。契約は**テスト**(`TestContractRejectsOverLimitRequests`)で検証し、
  実行時は自前の検証で同じ結論を出す、という既存の構えを保つ。

**検証の位置**: `decodeStrict` の直後、`format` の列挙検証より前(= ID 解決・engine の入力検証・engine 呼び出しより前)。
理由は、件数の検査が解決済みのマスタを必要としない純粋に構造的な検査であり、
「巨大な入力では一切の追加の仕事をしない」ことを最も強く保証できるため。
テストで固定するのは「**ID 解決(`master.Store` の参照)も engine 呼び出しも起きない**」という性質までで、
`format` の列挙との前後は実装の裁量に残す(`TestRequestLimitsRunBeforeStoreLookup` は
未知の種族・技・持ち物を混ぜた上限超過の要求で `master.Store` の参照回数が 0 であることを見る)。

**重複の判定**は解決前の生の値で行う: `itemVariants` / `itemCandidates` は `[]*string` なので、
`null`(持ち物なし)も1つの値として数える。`null` が2つあれば重複として拒否する
(JSON Schema の `uniqueItems` も `null` を値として区別する。kin-openapi の既定の
比較器で確認済み: `TestContractRejectsOverLimitRequests/itemVariants_に_null_の重複`)。

### 4. 他レーンへの依頼(この ADR では実装しない)

- **データレーン(engine)**: `engine.CalcBulk` / `engine.CalcReverse` にも同じ防御上限を置く
  (presets 8 / ItemVariants 64 / ItemCandidates 64 / Observations 16 / MaxCandidates 0 または 1..128)。
  HTTP を通らない直接呼び出し・WASM でも巨大入力を計算しないため(CLAUDE.md 絶対ルール2 の純粋性は保てる。
  外部依存を増やさない定数と sentinel だけ)。sentinel の命名はデータレーンの判断。
- **データレーン(engine/wasmapi)**: 同じ上限を wasmapi の語彙で `invalid_input` 相当に写し、
  HTTP/WASM parity テストを足す(ADR-0011 §5)。
- **Web レーン**: 観測追加 UI を 16 件で無効化(理由表示・アクセシビリティ通知)。
  持ち物候補が 64 件を超える場合の扱い(明示的なエラー、または仕様で決めた決定的な絞り込み)を決める。黙って切り捨てない。
- **iOS レーン**: 同じ対応。
- API レーンは `ios-test` と Web のテストを書かない(レーン境界。COORDINATION.md)。

## 受け入れ条件と担当テスト

| AC | 内容 | テスト |
|---|---|---|
| AC-1 | 契約に上限が入っている(presets 8+unique、itemVariants/itemCandidates 64+unique、observations 16(unique は付けない)、maxCandidates 0..128)。observations の `minItems: 1` は据え置き | `httpapi.TestContractDefinesRequestLimits` |
| AC-2 | 契約の検証器(kin-openapi)で、上限ちょうどは通り、上限+1・重複(null の重複を含む)・maxCandidates 129 と負は落ちる | `httpapi.TestContractRejectsOverLimitRequests` |
| AC-3 | `/api/calc/bulk`: presets 9 件・itemVariants 65 件・itemVariants の重複(同じ ID / null どうし)は 400 `invalid_input`。8 件以内の preset 重複は従来どおり `duplicate_preset` | `httpapi.TestCalcBulkRequestLimits` |
| AC-4 | `/api/calc/reverse`: itemCandidates 65 件・その重複・observations 17 件・maxCandidates 129 は 400 `invalid_input`。負は従来どおり `invalid_input` | `httpapi.TestCalcReverseRequestLimits` |
| AC-5 | 上限ちょうどは受理し、上限内の最大組合せ数になる(bulk 8×64 = 512 行、reverse 2×64 = 128 候補、observations 16・maxCandidates 128 も 200) | `TestCalcBulkRequestLimits` / `TestCalcReverseRequestLimits` の「上限ちょうど」小テスト |
| AC-6 | 上限超過は ID 解決(`master.Store` の参照)も engine 呼び出しも起こさずに落ちる(壁時計時間を見ない) | `httpapi.TestRequestLimitsRunBeforeStoreLookup`(参照回数 0 を数える `countingStore`) |
| AC-7 | 上限内の最大組合せでの計算コストを継続的に観測できる | `BenchmarkCalcBulkAtLimit` / `BenchmarkCalcReverseAtLimit` |
| AC-8 | 既存の 1MiB 本文制限・候補順・`exactCount`・Recall 基準・既存のエラー語彙を変えない | 既存テスト一式(変更しない) |

## 却下した案

- **`invalid_input` ではなく新しい code(`too_many_items` など)を足す**: ADR-0200 の語彙表・WASM 境界・
  Web/iOS の文言をすべて増やす割に、クライアントの分岐が増えない(どちらも「入力を直せ」)。
- **`presets` の重複も `invalid_input` に統一する**: 既存テスト `TestCalcBulkErrors` を弱めることになる(絶対ルール6)。
  `duplicate_preset` はより具体的で、WASM とも一致している。
- **`middleware.OapiRequestValidator` で契約全体を検証する**: 既存のエラー語彙と検証順序(ADR-0200 §4)が崩れ、
  HTTP と WASM の code が食い違う。§3 参照。
- **本文サイズ(1MiB)だけで守る**: 62KB の本文で 10 秒級に到達できるので効かない(issue の再現手順)。
- **`maxCandidates` だけで守る**: 出力を切るだけで計算量は減らない(`engine/reverse.go` は全候補を計算してから切る)。
- **gateway のタイムアウトだけで守る**: 503 を返した後も calc 側の計算は続きうる。対象外(issue の「変更範囲/対象外」)。
- **calc-svc だけで守る(engine には上限を置かない)**: WASM と直接呼び出しが素通しになる。データレーンへ依頼する(§4)。
- **上限を「サイズ」ではなく「組合せ数」で動的に決める**(例 presets × itemVariants ≤ 512 だけを見る):
  契約に JSON Schema として書けず、クライアントが事前に判断できない。固定の件数上限にした。

## 影響

- 契約(`api/openapi.yaml`)と生成物(`services/internal/api/openapi.gen.go`・`web/src/api/openapi.gen.ts`)。
  生成物の差分はコメントのみで、型は変わらない。
- calc-svc の `services/calc/internal/httpapi`(検証の追加。implementer の担当)。
- engine・wasmapi・Web・iOS は §4 の依頼として別レーンが追従する。
- 64 件・16 件を超える要求を送っていたクライアントは 400 になる(現行 UI は上限内なので実害は無い見込み)。
- **残存リスク(本 ADR の範囲外)**: 本 ADR が防ぐのは「1リクエストあたりの計算量の増幅」であり、**同時実行数・レート制限は扱わない**。
  上限ちょうどの reverse は約 22.6ms の CPU を要する(critic 実測)ため、calc-svc の CPU limit(`deploy/k8s/base/calc/deployment.yaml` の `200m`)は
  理論上 約9 req/s 程度で飽和しうる(クラウドの遅い vCPU ではさらに少ない)。レート制限・同時実行数の制御は gateway かクラスタ側の別課題とする。
  実機で重ければ `itemCandidates`/`itemVariants` の上限を 32 に下げる余地を残す(§1)。

**実装時の追記(2026-09-25。ADR-0126・ADR-0214。issue 272 の特性候補)**: 一括計算・逆算に防御側/相手側の
特性候補(`engine.MaxAbilityCandidates` = 3)が既定で加わったため、行数・候補数の**基本の上限**
(一括 512 行・逆算 128 件)は変わらないが、特性ごとに結果が違う技では実際の行数・候補数がその最大3倍
(一括 1536 行・逆算 384 件)まで増えうる。これは新しいクライアント制御の増幅経路ではない:
特性候補の件数はクライアントが直接指定できる配列ではなく(`defenderOverride.abilityId`/`unknownAbilityId`
は1件だけ、省略時は種族が実際に持つ特性数で ADR-0214 §2 により3件が上限)、リクエストの中身を変えて
更に増幅することはできない(§4「本 ADR が防ぐのは1リクエストあたりの計算量の増幅」という前提は保たれる)。
ADR-0126 の実測(逆算最悪ケースで特性3つのとき約38ms)を基礎コストの底上げとして許容する
(追加の増幅経路が無いことの確認が本追記の主旨)。**`BenchmarkCalcBulkAtLimit`・`BenchmarkCalcReverseAtLimit`
(`services/calc/internal/httpapi/limits_test.go`)は現状 fakeStore の1特性種族を使っており、特性3つの
コストはまだ計測できていない**(critic レビューで指摘。ADR-0214 の実装時点では未対応)。3特性で分岐する
fixture に変えて計測に含めるのは今後の課題とする。
