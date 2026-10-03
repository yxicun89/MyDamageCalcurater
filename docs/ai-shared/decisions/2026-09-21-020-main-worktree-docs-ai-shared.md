## 2026-09-21: 共有状態は main 専用 worktree の docs/ai-shared を正本にする
Decision: 新しい coordination branch は作らない。`main` 専用 worktree
(`~/pokecalc-main`)の `docs/ai-shared/` だけを現在状態の正本とし、feature branch 内の
同名ファイルは履歴上のスナップショットとして扱う。Claude/Codex は個別 worktree で実装する。
Reason: 既存の共有 MD とブランチ運用を維持しつつ、feature branch ごとの CURRENT_STATE 分岐と、
1 worktree のブランチ切り替えによる相互干渉を解消するため。
Impact: 共有 MD のみ main へ直接コミットしてよい。実装は従来どおり feature branch のみ。
共有 MD の同期だけを目的とする merge/cherry-pick は行わない。詳細は README_AI_SHARED.md。
