# 改善要望(type-balance レーン)

/improve で追加する。タスク ID は `I-type-balance-<連番>`(ADR-0172)。

- [x] I-type-balance-1 = ARCH S8(docs/plan/architecture-adoption.md の S8。データレーンが代行 2026-10-21): balance の `internal/balance` を `internal/domain` へ、`master`・`httpapi`・`httpguard`・`httpmetrics` を `internal/adapters` へ移し、`internal/app` は doc.go だけ(ADR-0423・ADR-0811。振る舞いは不変)。同梱の相性表を使う domain の外部テスト 6 ファイルは `internal/adapters/master` へ移した(期待値は不変)。tools/archcheck の未移行の一覧から balance を外した
