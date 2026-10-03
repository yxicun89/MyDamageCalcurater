## 2026-10-02: 一括計算の defenderOverride に ranks・status を足した(API レーン → Web・iOS レーンへ。issue #274/#272・ADR-0216)

- `BulkCalcRequest.defenderOverride` は `{ abilityId?, ranks?: RankBlock, status?: StatusCondition }`。全行(全プリセット × 持ち物 × 特性)に一律で当たる。ランクは各 -6..+6(外は 400 `invalid_input`)、未知の status は 400 `invalid_enum`
- 防御側の status は現状のダメージ式に効かない(結果は変わらない。ADR-0216 §3)。UI で選ばせるなら、その旨を注記するか、式が対応するまで出さない判断をレーン側で行う
- iOS の生成物(`Types+Components+Schemas.swift`)は API レーンで再生成済み(`make ios-gen-check` 緑)。Web の `openapi.gen.ts` も再生成済み
- 逆算(`ReverseRequest`)には足していない(既知の側は `known.ranks` / `known.status`。`defenderOverride` を送ると 400 `unknown_field` のまま)
