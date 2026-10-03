## 2026-09-23: issue #110 のデータレーン担当分(engine/wasmapi)を実装(データレーン)
Decision: 上記「calc の候補・観測件数に上限を置く」の依頼(API レーンから)に応え、
`engine.CalcBulk`/`CalcReverse` と `engine/wasmapi` に ADR-0208 §1 と同じ値の上限を実装した
(presets 8 / itemVariants 64 / itemCandidates 64 / observations 16 / maxCandidates 0..128。ADR-0108)。
- 検証は選択・内容検証(selectPresets・validateObservations)より前、`engine/wasmapi` では DTO 変換
  より前に置き、HTTP と同じ `invalid_input` が複数の違反が重なっても先に出るようにした(parity)。
- `MaxCandidates` が負のとき、従来「無制限」だった挙動を「不正(ErrInvalidMaxCandidates)」に変更した
  (ADR-0208 の契約が `minimum: 0` のため。既存テスト `TestReverseOrderDeterministic` の期待値を更新。
  理由は ADR-0108 決定4)。
- 新しい ErrorCode は足さず、5つの engine sentinel をすべて `wasmapi.CodeInvalidInput` に写した
  (ADR-0208 §2 と同じ判断)。
- 独立レビュー PASS(1往復。指摘: 古いフィールドコメントの修正、plan.md 未更新、wasmapi の DTO 変換順を
  上限検査より前に揃える、境界値テストの補強)。`make test`(790件)/`lint`/`build`/`test-golden`/
  `test-all-species`/`test-wasm` すべて green。
Reason: HTTP を経由しない直接呼び出し(ネイティブ Go)・WASM(ブラウザ)は calc-svc の検証を通らないため、
上限が無いままだと issue #110 の計算量増幅がそのまま残る。
Impact: issue #110 は Web・iOS レーンの追従(観測16件でUI無効化・持ち物候補64件超の扱い)が残っている限り
クローズしない。docs/plan.md の改善要望節・ADR-0108 参照。
