## 2026-09-21: Codex ブランチの main 取り込みはマージコーディネーター(Claude Code)が行う
Decision: Codex の feat/codex-* を main に取り込むのは Claude Code。Codex が完了を報告し、ユーザーが取り込みを指示したときだけ実施する。
共有ファイル5点(CURRENT_STATE.md=自セクションのみ / DECISIONS.md=追記のみ / go.work=Codex は追記しない /
ルート Makefile=Codex は services/balance/Makefile を作り include の1行は取り込み時に追記 / AGENTS.md・CLAUDE.md=自セクションのみ)の
編集規約を AGENTS.md に、取り込み手順を CLAUDE.md に定めた。
Reason: 担当ディレクトリを分けても共有ファイルでコンフリクトが起きるため。原因を規約で先に潰し、
それでも起きたコンフリクトは異常のサインとして自動解決せず報告する。
Impact: Codex は go.work とルート Makefile を編集しない。取り込み時のテスト確認は docs/type-balance-test-strategy.md を基準にするが、
この文書は未作成(Codex/ユーザーによる作成待ち)。作成されるまで Claude は取り込みを実行せず報告する。
