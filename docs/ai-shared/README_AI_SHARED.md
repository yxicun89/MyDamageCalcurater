# docs/ai-shared/ の使い方

AI同士は記憶を共有していない前提で運用する。共有記憶はこのディレクトリだけとし、
**`main` 専用 worktree にあるコピーを正本**とする。feature branch 内の同名ファイルは履歴上の
スナップショットであり、現在状態の参照先にしない。

## 正本と worktree

- 正本: `main` の `docs/ai-shared/`
- 標準の main worktree: `~/pokecalc-main`
- Claude worktree: `~/pokecalc`
- Codex worktree: `~/pokecalc-codex-tb0`
- パスを移した場合は `git worktree list` で `main` の worktree を探す。
- main worktree が利用できない場合、参照だけなら `git show main:docs/ai-shared/CURRENT_STATE.md` と
  `git show main:docs/ai-shared/DECISIONS.md` を使う。feature branch のローカルコピーへ代替記録しない。

## 開始・終了時

- 作業開始時: main worktree の `CURRENT_STATE.md` と `DECISIONS.md` を読む。相手のログは必要なときだけ読む。
- 作業中: ソースは各自の feature worktree だけで編集する。共有 MD へコード差分を持ち込まない。
- 作業終了時: main worktree で自分のログに追記し、`CURRENT_STATE.md` の自分の担当欄だけを更新する。
- 共有 MD を更新する前に main worktree の `git status --short --branch` を確認する。他者の未コミット変更が
  あれば上書きせず、その更新が完了するまで待つ。
- 共有 MD のみを main にコミットしてよい。実装を main に直接コミットしてはならない。
- 影響の大きい判断は `DECISIONS.md` に追記する。既存エントリは編集しない。
- feature branch へ共有 MD だけを同期するための merge/cherry-pick は行わない。実装ブランチを main に
  取り込む通常のタイミングで履歴を統合する。
- 相手の担当コード・DB・Deploymentには触れない。
- ログには会話全文を書かず、事実・決定・未解決事項だけを書く。
