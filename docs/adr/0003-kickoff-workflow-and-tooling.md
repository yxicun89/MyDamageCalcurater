# ADR-0003: キックオフ時のワークフロー適応と前提ツール

- 状態: 承認
- 日付: 2026-09-21

## 背景

M1 まで自動で進めるキックオフを行う。`/phase` ワークフローは各タスクを
quick-scanner → spec-writer → implementer → critic のサブエージェントで進めると定義されている。

## 決定

### 前提ツール(P0-1 の結果)

`scripts/doctor.sh` の結果、以下を brew で導入した。

| ツール | 状態 |
|---|---|
| go 1.27.1 | 導入した |
| k3d 5.9.0 | 導入した |
| helm 4.3.0 | 導入した |
| docker (Docker.app) | デーモン起動を確認 |
| tiup | **未導入**。M2 の TiDB 用で M1 に不要。M2 開始時に curl インストーラで導入する |
| xcodebuild | あり(M3 で使用) |
| codex / tailscale | 任意。未導入。Codex レビューは未ログインのためスキップ扱い(P0-5) |

### ワークフローの適応

- Phase 0 のスキャフォールディング(P0-2〜P0-4)は既存コードが無くスキャン対象が
  存在しないため、quick-scanner を省略し、orchestrator が implementer を兼ねて
  docs(requirements/design/test-strategy)を正として直接実装する。critic 相当の
  確認(ルール違反・越境・テスト有無)は orchestrator が行う。
- 正しさが要となる engine(P1)・逆算・API契約(P3)は
  spec-writer → implementer → critic のフルチェーンを使う。
- Codex レビュー(P0-5)は未ログインのためスキップし、`.reviews/` に記録しない。
  該当タスクでは critic のみで判定する。

## 影響

- 1タスク=1コミット、plan.md 更新後にコミットの原則は維持する。
- サブエージェント省略は Phase 0 に限定し、engine 以降は本来のチェーンに戻す。
