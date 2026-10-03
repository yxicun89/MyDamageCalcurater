## 2026-09-21: 作業ディレクトリを ~/MyDamageCalcurater の1つに整理(Codex の worktree は必要なときだけ作る)
Decision: 旧ディレクトリ(~/pokecalc・~/pokecalc-main・~/pokecalc-codex-tb0。旧リポジトリの本体と worktree)をユーザー指示で削除した。旧 ~/pokecalc の未コミット分(P1-13 の作業。新リポジトリの `0e4616a` 以降に取り込み済み)と Git 管理外の `.reviews/`(Codex による規約レビューの出力。内容は coding-rules.md v2 に反映済み)は残していない。
さらに、Codex が止まっている間は使わない ~/MyDamageCalcurater-codex(git worktree)も `git worktree remove` で片付けた。ブランチ `feat/codex-tb0-foundation`(285a46f)は origin に push 済みで、失われたものはない。
Codex が再開するときは、次で worktree を作り直す(COORDINATION.md の「ディレクトリとブランチ」と同じ):
  `git -C ~/MyDamageCalcurater fetch origin && git -C ~/MyDamageCalcurater worktree add ~/MyDamageCalcurater-codex feat/codex-tb0-foundation`
その後 `cd ~/MyDamageCalcurater-codex && codex`。最初に `git fetch origin` し、origin/main の `docs/ai-shared/CURRENT_STATE.md` と `DECISIONS.md` を読む。
Reason: ユーザーが「複数ディレクトリがあるのが混乱の元なので、使うものだけにして」と指示した。Codex は同じ作業ツリーで Claude と同時に動かせない(COORDINATION.md)ため、Codex が動くときだけ worktree を作る。
Impact: 今あるディレクトリは ~/MyDamageCalcurater だけ。CURRENT_STATE.md の Type Balance 欄(Codex の担当)の `worktree: ~/pokecalc-codex-tb0` の記述は古い。Codex が次のセッションで更新すること(Claude は編集しない)。
