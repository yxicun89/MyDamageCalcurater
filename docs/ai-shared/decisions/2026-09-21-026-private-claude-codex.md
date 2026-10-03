## 2026-09-21: 開発の正本を private のクリーンコピーへ移し、Claude と Codex の協調運用を「互いを待たない」形に改める(ユーザー指示)
Decision: (1) 履歴を消毒(作者を `pokecalc-dev <noreply@example.com>` に統一、個人用の手順書を全履歴から除去、module path とホームの絶対パスを置換)したクリーンコピーを
`~/MyDamageCalcurater` に作り、private の GitHub リポジトリ(origin)へ push した(`main` / `feat/claude-p1-engine` / `feat/codex-tb0-foundation`)。以後の開発の正本は origin の main。
旧ディレクトリはアーカイブ。(2) 互いのレートリミットで作業が止まらないよう、docs/ai-shared/COORDINATION.md を新設した: 各 AI が自分のブランチの統合まで単独で完了できる
(マージコーディネーター Claude Code の廃止、Codex の go.work / Makefile 追記を許可)、止まる前に WIP を commit・push して `Next` を書く、相手を待たず既定値付きの提案で進める、
レビューは各 AI の自己完結(相手に依頼しない)、ADR 番号は統合時に後から統合する側が振り直す。
Reason: ユーザーが「お互いのレートリミットでお互いの作業が止まるのが問題。先に調整して、それぞれレートリミットに関係なく作業できる状態にしたい」と依頼した。
Impact: AGENTS.md の共有ファイル編集規約と CLAUDE.md の「Codexブランチの取り込み手順」を更新(後者は廃止)。Claude 側の type-chart ADR は、Codex の ADR-0012(domain-service-boundaries)との衝突を避けて 0013 に振り直した。
Codex は次のセッションで COORDINATION.md を確認し、異議・修正があれば DECISIONS.md に追記する。それまでは本文の内容で進めてよい(既定案で進む原則)。
