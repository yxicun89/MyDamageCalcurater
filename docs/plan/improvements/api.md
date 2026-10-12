# 改善要望(api レーン)

/improve で追加する。タスク ID は `I-api-<連番>`(ADR-0172)。

- [x] I-api-1 = CB-4(docs/plan/combo-calc.md §6。データレーンが代行 2026-10-12): `POST /api/calc/combo`(calc-svc。engine.CalcCombo の写し・手 1〜3 の件数は ID の解決より前に 400・計算イベントは発行しない)と公開 `Move.effect`(pokedex-svc。確率 100% のランク変化だけ。検証に通らなければ 503)。ADR-0253。受け入れ条件とテストは services/calc/internal/adapters/httpapi/combo_test.go(手順は services/calc/internal/app/combo.go)・services/gateway/internal/httpapi/combo_routing_test.go・services/pokedex/internal/httpapi/public_move_effect_test.go
- [x] I-api-2 = ARCH S3b(docs/plan/architecture-adoption.md の S3。データレーンが代行 2026-10-12): calc-svc の調整の5操作(`/api/calc/adjust/*`)の手順を app 層へ移す(値域の検査と列挙の失敗は services/calc/internal/domain/adjust.go、手順は services/calc/internal/app/adjust.go、生成型の写しは services/calc/internal/adapters/httpapi/adjust.go)。振る舞い(検査の順番・code・文言・応答)は変えない。ADR-0252 追記 S3b。既存の services/calc/internal/adapters/httpapi/adjust_test.go・adjust_goals_test.go が期待値を変えずに通る
