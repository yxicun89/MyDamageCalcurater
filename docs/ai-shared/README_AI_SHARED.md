# docs/ai-shared/ の使い方

AI同士は記憶を共有していない前提で運用する。**このディレクトリだけが共有記憶**。

- 作業開始時: まず `CURRENT_STATE.md` と `DECISIONS.md` を読む。相手のログ(`CLAUDE_LOG.md`/`CODEX_LOG.md`)は必要なときだけ読む
- 作業終了時: 自分のログに追記し、`CURRENT_STATE.md` の自分の担当欄を更新する
- 影響の大きい判断をしたら `DECISIONS.md` に追記する(書式は既存のエントリに合わせる)
- 相手の担当ディレクトリ・DB・Deploymentには触れない
- ログには会話全文を書かない。事実・決定・未解決事項だけ
