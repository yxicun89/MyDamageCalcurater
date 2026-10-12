# 改善要望(api レーン)

/improve で追加する。タスク ID は `I-api-<連番>`(ADR-0172)。

- [x] I-api-1 = CB-4(docs/plan/combo-calc.md §6。データレーンが代行 2026-10-12): `POST /api/calc/combo`(calc-svc。engine.CalcCombo の写し・手 1〜3 の件数は ID の解決より前に 400・計算イベントは発行しない)と公開 `Move.effect`(pokedex-svc。確率 100% のランク変化だけ。検証に通らなければ 503)。ADR-0253。受け入れ条件とテストは services/calc/internal/adapters/httpapi/combo_test.go(手順は services/calc/internal/app/combo.go)・services/gateway/internal/httpapi/combo_routing_test.go・services/pokedex/internal/httpapi/public_move_effect_test.go
- [x] I-api-2 = ARCH S4(docs/plan/architecture-adoption.md §6。データレーンが代行 2026-10-12): record-svc を domain・app・adapters の形にそろえる。お気に入りの検証・正規化(SP・レベルの上限。G6)→ `internal/domain`、操作の手順・record DB のポート → `internal/app`、失効ジョブの手順 → `internal/app/expire`(TiDB 実装は `internal/adapters/expirestore`)、httpapi・store・events → `internal/adapters`。振る舞いは変えない。ADR-0254。record を tools/archcheck の未移行の一覧から外した
