## 2026-09-25: issue #271/#270 の API レーン担当分(mechanisms 公開・unsupported 印)を実装(API レーン → データ・Web・iOS レーンへ)
Decision: データレーンからの依頼(ADR-0121 §4・ADR-0123 §7)を反映した。
`api/openapi.yaml`: `MasterMove.mechanisms: string[]`(必須・昇順・通常の技は空配列)を追加。
新設 `UnsupportedMark`(target/reason/id。ADR-0123 §7 の YAML どおり)を `CalcResult`(`BulkCalcRow.result` も
`$ref: CalcResult` のため自動的に対象)・`ReverseCandidate` に `unsupported: UnsupportedMark[]`(必須・
印なしは `[]`)として追加。`make gen` 済み(差分ゼロを確認)。
`services/pokedex/internal/httpapi/master.go`: `ListMoveMechanisms` を呼び、move_id ごとにまとめてから
`sort.Strings` で明示的に昇順にする(SQL の `ORDER BY` に頼らず契約の保証をこの層に持たせる)。
`services/calc/internal/master/export.go`: `api.MasterMove.Mechanisms` を `sharedmaster.MoveRow.Mechanisms` に
そのまま渡すだけ(検証・`engine.Move.Mechanisms` への変換は既存の `MoveMechanismsOf` が担当。新規ロジックなし)。
`services/calc/internal/httpapi/convert.go`: `unsupportedFrom`(`engine/wasmapi` の同名関数と同じ変換)を
新設し、`calcResultFrom`・`reverseResultFrom` に配線(`bulkResultFrom` は `calcResultFrom` を呼ぶため自動的に
対象)。`parity_test.go` の `dropEmptyUnsupported`(印を比較対象から除外する暫定処置)を削除し、HTTP/WASM で
`unsupported` を含めてそのまま比較するようにした(全パリティテスト PASS を確認)。
`services/pokedex/internal/storetest/storetest.go`: `store.Querier` に `ListMoveMechanisms` の埋め込み
nil-panic ガードを追加(`MoveMechanisms` フィールド + メソッド。他の `List*` と同じパターン)。
mutation testing で「ソートしない」「空配列にしない」の2点を実際に壊して回帰を確認済み。
Reason: 両 issue は Web・iOS の表示実装を進めるための前提(データ・engine 側は完了済み)。
Impact: **データレーンへ**: #271・#270 は API レーン担当分も完了。close 判断はデータレーンに委ねる。
**Web レーンへ**: `web/src/master/exportSnapshot.ts` の例データに `mechanisms: []` を追加済み(Web 自身の
持ち物のため軽微な追従。テストも更新済み)。`unsupported` は Web の `CalcResult`(`engine/types`)にまだ無く、
`apiEngine.ts` の明示的フィールド写像(`mapCalcResult` 等)が自動的に弾くため何もしなくても壊れない
(issue #67 の前方互換どおり)。表示するかどうか・いつ着手するかは Web レーンの判断。
**iOS レーンへ**: swift-openapi-generator の生成物の再生成(`make ios-gen` 相当)が必要(このタスクでは
未実施)。再生成すると `MasterMove.mechanisms`・`CalcResult.unsupported`・`ReverseCandidate.unsupported` が
必須フィールドとして生成物に増えるため、既存のデコード/モック実装が影響を受ける可能性がある。
