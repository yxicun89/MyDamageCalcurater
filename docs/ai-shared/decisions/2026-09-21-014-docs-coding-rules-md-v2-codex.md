## 2026-09-21: コーディング規約 docs/coding-rules.md(v2)を共通規約として起草。Codex の再確認は未了
Decision: Claude Code と Codex 共通のコーディング規約を docs/coding-rules.md に置く(目的: いつでも GitHub に公開できる状態・人が読みやすいコード・ハードコードしない)。
CLAUDE.md の「最初に読むもの」と AGENTS.md から参照する。既存コードの是正は plan.md の Phase R(R-1 監査 → R-2 是正 → R-3 make check-publishable)で、挙動を変えずに行う。
Codex の1回目レビュー(scripts 外で codex exec --sandbox read-only を実行)は「要修正」。指摘(文書の優先順位、DRY の例外、検証の置き場所、float の例外(ADR-0006)、
テスト条項の緩和、言語別ルールの追加、check-publishable は実装まで手動確認、golden の扱い)を v2 に反映した。v2 への Codex の再レビューは実行中で、結果は未確認。
Reason: ユーザーが「実装のハードコードをしない・人が読みやすいコード・いつでも GitHub に公開できる状態」の規約を Claude と Codex で決めてほしいと依頼した。
Impact: Codex が v2 を承認するまで「Codex の承認済み」とは扱わない。Codex は自分の担当セクション(AGENTS.md の Codex 担当範囲)と DECISIONS.md への追記で意見を残す。
LICENSE の方針は未定(公開前にユーザーが決める)。
