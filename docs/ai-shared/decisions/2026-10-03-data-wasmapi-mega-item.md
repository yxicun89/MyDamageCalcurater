## 2026-10-03: wasmapi(オフライン計算)がメガ種族の持ち物規則を検証する(データレーン → Web レーンへ。issue #505・ADR-0321)
Decision: `engine/wasmapi` の種族 DTO に `isMega`・`requiredItemId` を足し、calc の attacker・defender、bulk の attacker と itemVariants(防御側がメガ種族のとき)、
reverse の known と itemCandidates(推定側がメガ種族のとき)で calc-svc と同じ規則(メガ種族は requiredItemId の持ち物か持ち物なしだけ)を検証する。
違反は `invalid_input`、メッセージも calc-svc と同じ文言。engine は変更しない。HTTP と WASM の parity は `mega_parity_test.go` と Go/WASM 一致ベクタで固定。
Reason: オンラインとオフラインで不正入力の扱いを揃えるため(issue #505)。
Impact: **Web レーンへの依頼**: `toEngineSpecies`(`web/src/domain/requests.ts`)が `isMega`・`requiredItemId` を境界へ渡していない(ADR-0320 は当時の境界が
`unknown_field` で拒否するため落としていた)。境界は両方を受け付けるようになったので、渡すように変更し、対応テスト `requests.mega.test.ts` と ADR-0320 の記述を更新する
(渡すまでオフライン計算に規則は効かない。通常の操作では持ち物欄がストーンに固定されるので起きない入力)。通常種族の応答は変わらない。
