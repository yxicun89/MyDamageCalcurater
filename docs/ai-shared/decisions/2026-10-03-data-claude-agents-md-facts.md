## 2026-10-03: CLAUDE.md・AGENTS.md の事実の食い違いを直す(issue #225・#253・#256 の残り。データレーン → 全レーン)
Decision: ユーザー決定 2026-10-03「既定案で OK」。CLAUDE.md・AGENTS.md・docs/development-workflow.md の**事実の食い違いだけ**を直した。運用ルール(マージ規則・人間の確認事項・絶対ルール)の意味は変えない。
- リポジトリ構成: `services/balance・speed・judge・record・team・internal`、`scripts/ai-guard` 等の実在に合わせた。
- レーン: データ・API・Web・iOS・タイプバランス・素早さ・判定(+空席時の運用)に合わせた。
- `known_diffs.yaml` は存在しない。差分の許容の仕組みは無く、ゴールデンは全件一致(ADR-0002 追記 P2-1b)。「人間の確認が必要なこと」から外した。
- Argo CD: Application は balance・speed・judge の gitops overlay を見る(Sync は manual)。`overlays/local`・`overlays/cloud` を見る Application は無い。
- 共有状態は `docs/ai-shared/state/`(レーン別)と `decisions/`(1 件 1 ファイル。ADR-0805)。
- #253: 役目を終えた `KICKOFF.md`・`CODEX_KICKOFF.md` を `docs/history/` へ移し、「最初に読むもの」から外した(参照を直した)。
Reason: 文書が実態とずれ、AI が存在しないファイルや古いレーン数を前提に動いていた。
Impact: issue #225・#253・#256 は閉じてよい。
