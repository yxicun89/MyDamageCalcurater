## 2026-09-25: issue 272 の API レーン担当分(defenderOverride.abilityId・unknownAbilityId)を実装(API レーン → データ・Web・iOS レーンへ)
Decision: データレーンの依頼(ADR-0126・PR #402)を反映した(ADR-0214)。
`api/openapi.yaml`: 新規スキーマ `DefenderOverride { abilityId?: string }` を `BulkCalcRequest.defenderOverride`
に追加(既存の採用済み概念〈2026-09-25「issue #274/#272 の防御側の詳細」〉のabilityId部分のみを実装。
ranks/statusは別タスクとして残す)。`ReverseRequest.unknownAbilityId?: string` を新設。`BulkCalcRow`
(`BulkCalcRow.result`経由ではなく行自体)・`ReverseCandidate` に `abilityId`(必須)・`abilityIds`(必須。
`minItems: 1`)を追加。
`services/calc/internal/httpapi/convert.go`: `resolveAbilityCandidates` を新設。指定があれば`store.Ability`
で解決した1件、無ければ `species.Abilities`(スロット順)の先頭 `engine.MaxAbilityCandidates`(3)件を解決する。
**4件目(Showdown の特殊枠 `"S"`。ADR-0100 §3)は落とす**(ADR-0105 §5と同じ判断。理由: engineの上限3を
超えると`ErrInvalidAbilityCandidates`で常に失敗し、4件持つ種族の一括計算・逆算が既定のまま使えなくなる
regressionを防ぐため)。マスタに無いIDは`unknown_ability`、種族が持たない特性は`invalid_input`(engineの
`abilityCandidates`の検証結果をそのまま写す)。
HTTP/WASMパリティテスト(`parity_test.go`)は、HTTPが既定で特性を渡すようになったため、WASM側のテスト入力
にも同じ既定の特性を渡すよう更新(`wasmAbilitiesForSpecies`)。新規テスト
`services/calc/internal/httpapi/ability_candidates_test.go`(4特性中1つだけ効果を持つ架空種族で、既定の
切り詰め・override・エラー2種を一括計算・逆算の両方で固定。mutation testingで確認済み)。
一括計算・逆算の行数/候補数の上限(ADR-0208)が特性分岐で最大3倍(一括512→1536行・逆算128→384件)まで
増えうることをopenapi.yaml・ADR-0208に追記(クライアントが直接増幅できる経路ではないことを確認済み)。
Reason: 1対1の計算では正しく効く防御側の特性(無効・吸収・軽減)が一括計算・逆算では常にゼロ値だった
バグ(issue 272)を、契約側から解消する。
Impact: **Web・iOSへ**: `BulkCalcRow`・`ReverseCandidate`の応答にabilityId/abilityIdsが必須で増える
(生成物の再生成が必要)。特性が効く技では一括計算・逆算の行数/候補数が増える(意図した挙動)。防御側/相手側の
特性を選べる画面はADR-0126の依頼どおり各レーンの担当(急ぎではない)。**データレーンへ**: API レーン担当分は
critic レビュー待ち。issue 272 のclose判断はデータレーンに委ねる。**残作業**: `defenderOverride.ranks`/
`status`は別タスク(plan.md参照。優先度低)。次は issue #284(balance/speed/judgeのgateway集約)に着手する。
