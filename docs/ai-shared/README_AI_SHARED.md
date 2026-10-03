# docs/ai-shared/ の使い方

AI 同士は記憶を共有していない前提で運用する。共有記憶はこのディレクトリだけ。
運用の正は [COORDINATION.md](COORDINATION.md)(レーン制・PR での統合・止まるときの作法)。

## 正本と作業ディレクトリ

- 現在状態の正本: `origin/main` の `docs/ai-shared/state/<lane>.md`(読むだけなら `git show origin/main:docs/ai-shared/state/<lane>.md`。索引は `CURRENT_STATE.md`)。
  ただし未マージの作業の続きは、レーンのファイルの `Branch` の最新コミットにある(止まる前にそこへ commit・push するため)。
- 作業ディレクトリ(レーンごとに1つ。どの AI が使ってもよい。同時に2セッションで開かない):
  - ダメージ計算: `~/MyDamageCalcurater`
  - タイプバランス: `~/MyDamageCalcurater-tb`(同じリポジトリの git worktree)

## 開始・終了時

- 開始時: `git fetch origin`、`origin/main` の `state/<lane>.md` と未決の判断(`scripts/list-decisions.sh`)を読み、`Branch` の `Next` から続ける。`Active` を自分にする。
- 作業中: 共有 MD へコード差分を持ち込まない。影響の大きい判断は `docs/decisions/` に1件1ファイルで書く(`scripts/new-decision.sh <lane> <slug>`。既存エントリは編集しない)。`DECISIONS.md` は 2026-10-03 までの凍結アーカイブ(ADR-0170)。
- 終了時(止まる前も): 自分のログ(`CLAUDE_LOG.md` / `CODEX_LOG.md`)に追記し、`state/<lane>.md` の `Status`・`Next` を具体的に書き、
  `Active` を `なし` にして、作業ブランチに commit・push する。main へは PR でのみ入れる。
- ログには会話全文を書かず、事実・決定・未解決事項だけを書く。
