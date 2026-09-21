# Codex Log

(Codex のセッション終了時にここへ追記する。書式は README_AI_SHARED.md 参照)

## 2026-09-21 (Claude Code による代理記録。Codex はレートリミットで操作不可)
- Claude Code が fix/codex-workflow-golden(未コミットだった作業)を確認。engine コア・API 契約に触れるため保留と判断し、
  内容を変えずに 6d86382 として保全。main へはマージしていない。理由と再検討の方針は DECISIONS.md 参照。
- 同日、ユーザー判断で fix/codex-workflow-golden を main へマージ(Claude Code が実施)。AGENTS.md / CLAUDE.md の衝突は両方残して統合。
  Codex は次回 main から自分のブランチを切って作業する。詳細は DECISIONS.md。

## 2026-09-21 TB0 開始・取得元確認待ち

### Done
- Git 状態と指定文書、関連 OpenAPI・pokedex-svc・importer の有無を確認した。
- main の 8049702 から `feat/codex-tb0-foundation` を作成した。
- タイプ相性データの取得経路が存在しないことを確認し、指示どおり実装を停止した。

### Changed files
- `docs/ai-shared/DECISIONS.md`
- `docs/ai-shared/CURRENT_STATE.md` の Type Balance Checker 欄
- `docs/ai-shared/CODEX_LOG.md`

### Decisions
- 未決。pokedex-svc のタイプ相性 API 契約、または MySQL エクスポート仕様を確認待ち。

### Open issues
- OpenAPI にタイプ相性取得エンドポイントがなく、`services/pokedex/` と importer も未実装。
- `docs/type-balance-test-strategy.md` はまだ存在せず、マージコーディネーターの取り込み条件も未充足。
- main へ切り替えた時点で既存の未追跡 `docs/local/` が見える状態になった。今回の変更には含めない。

### Next
- 取得元の契約確定後、TB0 のテスト設計と `services/balance/` 実装を再開する。
