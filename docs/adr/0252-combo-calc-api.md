# ADR-0252: 連続技 API `POST /api/calc/combo` と公開 `Move.effect`(CB-4)

- 状態: 採用(2026-10-12 実装済み。データレーンが API レーンを代行)
- 日付: 2026-10-12
- レーン: ダメージ計算(API 帯 `0200〜`。0251 の次)
- 関連: docs/plan/combo-calc.md §2・§4-2・§5(AC-13・AC-16・AC-17)・§7、ADR-0179 追記 CB-3(engine の `CalcCombo`)・CB-5(WASM の DTO)、
  ADR-0200(calc-svc の境界)、ADR-0208 §3(件数の検査は ID の解決より前)、ADR-0204(マスタの準備中)、ADR-0202 §3(gateway の前方一致)、
  ADR-0107(move_effects)、ADR-0218・ADR-0143 §6(公開 API に効果定義を出す流儀)、CLAUDE.md 絶対ルール 1・5

## 背景

engine の `CalcCombo`(CB-3)と WASM の `calcCombo`(CB-5)ができた。Web(CB-6)・iOS(CB-8)がオンラインで使う HTTP の契約と、
技の候補で「変化技でランクが変わる技」を選べるように、公開 API の技に確率 100% のランク変化を出す必要がある。

## 決定

### 1. `POST /api/calc/combo`(operationId `calcCombo`。calc-svc)

- リクエスト `ComboRequest`: `format`・`attacker`・`defender`・`field` は `CalcRequest` と同じ形(UC-5: ダメージ計算の入力をそのまま渡せる)。
  `steps`(1〜3。`ComboStepInput{moveId, critical?, hits?}`)、`opponentMoveId?`、`opponentSpeed?`(`unknown` 既定 / `from_defender`)、
  `speedField?`(`trickRoom`・`selfTailwind`・`opponentTailwind`)。
- 応答 `ComboResult`: engine の `ComboResult` の写しで、**形は WASM の `calcCombo`(CB-5)と同じ**(Web が型を共有できるように)。
  - `steps[]`: `moveId`・手の開始/終了時の自分/相手のランク(`ComboRanks`。5 キー必須)・`damage`(`CalcResult`。変化技は null)・
    `selfSpeed`・`opponentSpeed`(unknown は 0)・`selfPriority`・`opponentPriority`・`order`・`line{selfSpeed, selfFirst, opponentFirst}`
  - `total`: `minDamage`・`maxDamage`・`minPercent`・`maxPercent`・`defenderHP`・`ko`・`koByStep[]`(`ComboKOChance{chancePercent, guaranteed, displayChancePercent}`)
  - `speed`: `firstSelfSpeed`・`lastSelfSpeed`・`opponentSpeed`・`opponentSpeedKnown`
  - `opponentAttack`: null か `{selfHp, minActions, maxActions, fast, slow}`(シナリオは `{actions[], surviveChancePercent}`)
  - `unsupported`: 印なしは `[]`
- 表示%・表示用の確率は engine の 0.1% 単位の整数を 10 で割るだけ(独自の丸めを持たない)。
- 検査の順(AC-16): 厳格デコード → **`steps` の件数 1〜3(範囲外は ID の解決より前に 400 `invalid_input`。マスタを引かない)** →
  列挙(`format`・`opponentSpeed`・天候・フィールド。400 `invalid_enum`)→ ID の解決(400 `unknown_species`・`unknown_move` 等)→
  個体の検証 → engine。ランク変化の無い変化技の手・変化技の相手の技は `engine.ErrInvalidComboInput`、多段の回数は `ErrInvalidBattleState`
  を 400 `invalid_input` に写す(新しい code は足さない)。
- **未知の技は 400 `unknown_move`**。combo-calc.md AC-16 の「422」は HTTP の語彙(ErrorCode の表。全 ID 不明は 400)と合わないので、
  「エラーの形は `/api/calc` と同じ」を優先した。
- 計算イベント(record)は**発行しない**(combo-calc.md §4-2。絶対ルール 5: record・team・NATS が止まっていても成功する)。
  連続技の履歴は要望が出たら別に決める。
- 準備中(`NewDeferredHandler`)は 503 `master_unavailable`。guard(締め切り・同時実行の上限)・ログ・メトリクスはほかの計算と同じミドルウェアを通る。
- gateway は変えない(`/api/calc/*` の前方一致で calc-svc に届く)。NetworkPolicy も変わらない(同じサービス・ポート)。

### 2. 公開 `Move.effect`(pokedex-svc)

- `getMove`・`getMovesByIds`・`searchMoves` の `Move` に `effect`(`MasterEffect`)を足す。**move_effects の行の `Chance` が 100 のときだけ**返し、
  確率 100% 未満・行の無い技はキーごと省く(combo-calc.md §7 Q4 (b)。連続技は確率 100% の効果だけを扱う)。
- 値と形は内部 API の `MasterMove.effect` と同じ(`{"Chance":100,"Target":"self","Stages":{"atk":1,"spe":1}}`。数値の字面を保つ)。
  公開 API の `rule`・特性/持ち物の `effect` と同じく PascalCase のまま出す。WASM の `moveEffectDTO`(`chance`・`target`・`stages`)は
  encoding/json の照合が大文字小文字を区別しないので、Web はこの値をそのまま WASM の技の `effect` に渡せる(`rule` と同じ扱い)。
- 返す前に共通マスタ `master.DecodeMoveEffect` で厳格に検証し、通らなければ 503 `master_unavailable`(確率 100% 未満の行も黙って捨てない)。
- 新しいクエリ `ListMoveEffectsByMoveIDs`(sqlc)。migration・権限は変わらない(move_effects の SELECT は既にある)。

### 3. AC-18(WASM と API の結果の一致)の担保

`wasmapi.CalcCombo`(CB-5。feat/wasm-combo)はこの時点で main に入っていないため、`parity_test.go` の流儀の突き合わせ(同じ入力を
HTTP と wasmapi に通す)はまだ書けない。代わりに次の 2 つで担保し、CB-5 が main に入ったら突き合わせのテストを足す(後続):

- 値: HTTP の応答 = 同じ入力の `engine.CalcCombo` の結果の写し(`TestCalcComboHTTPMatchesEngine`)。WASM も同じ engine を素通しする。
- 形: 応答のキーの集合が CB-5 の DTO と同じ(`TestCalcComboResponseKeysMatchWasmShape`)。

## 互換

- 追加だけ(新しい操作・任意のキー)。古いクライアントは無視する。古いサーバーは `Move.effect` を返さない(クライアントは「キーが無い」を
  「ランク変化なし」と「返さない版」の区別に使わない)。
- iOS: 生成型(swift-openapi-generator)に `calcCombo` と `Move.effect` が増える。`APIProtocol` に準拠する手書きの型は無いので既存のビルドは壊れない。

## 却下した案

- **unknown の相手の素早さを null で返す**: 表現としては素直だが、WASM(CB-5)が `0` + `opponentSpeedKnown` で確定していたので揃えた
  (Web がオンラインとオフラインで同じ型を使えることを優先)。
- **公開 `Move.effect` を camelCase に変換して返す**: `rule` や特性の `effect` は PascalCase のまま出しており、1 つだけ変えると読み手が混乱する。
  WASM は両方受けるので変換の必要が無い。
- **計算イベントを発行する**: 履歴の形(`CalcDetail`)は単発の計算用で、連続技を載せる形が決まっていない(combo-calc.md §4-2)。
- **judge-svc に置く**: combo-calc.md §4-2・§7 Q2(判定サービスは廃止の方向)。
