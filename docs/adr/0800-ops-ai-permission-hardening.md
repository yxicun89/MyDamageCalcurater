# ADR-0800: AIエージェントの許可設定を「読み取り・非破壊」に絞り、破壊操作を本文検査で止める(issue #273・#239)

- 状態: 提案(実装後、設定ファイルの変更として **マージはユーザー確認・承認を経てから行う**。2026-09-25)
- 日付: 2026-09-25
- 担当: 運用(deploy・scripts)レーンの空席を、調整役の割り当てで素早さレーンが代行(ユーザーの直接指示ではない。
  DECISIONS.md 参照)。ADR 番号帯は運用レーン用に新設(`docs/ai-shared/COORDINATION.md` の帯はデータ0100・API0200・
  Web0300・タイプバランス0400・iOS0500・素早さ0600・判定0700まで割当済みのため、次の空き帯として運用に0800を使う)
- 関連: issue #273(全体レビュー第3回)・issue #239(全体レビュー)、PR #399(DECISIONS.md、ユーザー決定4件のうち3件目)

## 背景
`.claude/settings.json` の `permissions.allow` に `Bash(make *)`・`Bash(kubectl *)`・`Bash(k3d *)`・`Bash(docker *)`・
`Bash(node *)`・`Bash(gh pr merge *)`・`Bash(git push *)` 等の広い許可があり、`defaultMode: "auto"`。CLAUDE.md が
「人間の確認が必要」とする操作(クラスタ削除・DBのデータ削除)や、意図せず main へ反映される操作・秘密の読み取りが、
コマンドの書き方を変えるだけで `ask`/`deny` のパターンに一致せず `allow` に落ちる:
- `make down`(内部で `k3d cluster delete`)は `ask` の `Bash(k3d cluster delete *)` という文字列と一致しないコマンド文字列
  (`make down`)なので `Bash(make *)` の `allow` がそのまま通る。
- `kubectl -n pokecalc delete pvc mysql-data` は `ask` の `Bash(kubectl delete pvc *)` (先頭が `kubectl delete pvc`)
  に一致しない(`-n pokecalc` が間に挟まる)。
- `git push origin HEAD:refs/heads/main`・`git push origin main -q` は `deny` の `Bash(git push * main)` 系のパターン
  (末尾が `main` で終わる前提)に一致しない(接頭辞に別の引数がある、または末尾に引数が付く)。
- `kubectl get secret mysql-auth -o jsonpath=...` は `allow` の `Bash(kubectl *)` にそのまま一致し、秘密を読める。
- `.codex/config.toml` には `approval_policy`・`sandbox_mode` が設定されておらず、Codex 側の破壊操作の抑止は
  `developer_instructions` の文書上の注意書きだけ(強制力なし)。

Claude Code公式ドキュメント(2026-09-25 確認)によれば、`permissions` の `allow`/`ask`/`deny` はコマンド文字列への
前方一致に近い glob 一致であり、「同じプログラムの別の呼び方を止める」ようには設計されていない(セキュリティ境界ではない)。
確実に止めるには `PreToolUse` フックが exit code 2 で unconditional に block する経路を使う必要がある(このフックは
`permissions.allow` の有無に関係なく先に評価され、exit 2 はどの allow ルールも上書きできない)。

## 決定

### 1. 許可設定は「読み取り・非破壊」に絞り、glob の穴は塞ぐが主防御にしない
`.claude/settings.json` の `permissions.allow` から、破壊的操作を含みうる広いワイルドカード
(`Bash(make *)`・`Bash(kubectl *)`・`Bash(k3d *)`・`Bash(docker *)`・`Bash(git push *)`・`Bash(gh pr merge *)`)を外し、
非破壊なサブコマンド単位の許可に置き換える(例: `kubectl get *`・`kubectl describe *`・`kubectl logs *`・`kubectl apply *`・
`make test*`・`make lint*`・`make build*`・`make dev*`・`make doctor*`・`make gen*`・`make wasm*`・`make up`・`make deploy*`
など、破壊的でない Makefile ターゲット)。`ask` に、issue #239/#273 が列挙した回避形を **プレフィックスだけでなく複数の
書き方を明示**して追加する(`kubectl * delete pvc *`・`kubectl * delete namespace *`・`kubectl * delete ns *` 等)。
ただし glob パターンの追加だけでは網羅できない(公式ドキュメントが認める通り)ため、これは補助的な多層防御とし、
主防御は次の PreToolUse フックに置く。

### 2. PreToolUse フックで実際のコマンド文字列を検査する(主防御)
`.claude/settings.json` に `hooks.PreToolUse`(matcher: `"Bash"`)を追加し、`scripts/ai-guard/bash-guard.sh`
(新規。Claude Code・Codex 両方から呼べるよう共通化)を呼ぶ。スクリプトは stdin の JSON から `tool_input.command` を
読み、正規化(`timeout`・`env`・`nohup` 等の前置ラッパーを剥がす)した上で、次のいずれかに一致したら **exit code 2**
で無条件に block する(`permissionDecisionReason`/stderr に理由と、人間が自分の端末で実行する想定であることを書く):
- クラスタ削除: `k3d cluster (delete|rm)` 相当
- データ削除: `kubectl` の `delete` で対象が `ns`/`namespace`/`pvc`/`persistentvolumeclaim`/`pv`/`persistentvolume`/
  `statefulset`/`secret`(引数の順序に依存しない)、`kubectl delete -k` (kustomize 一括削除)
- DB ロールバック: `make migrate-down*`(`CONFIRM_DESTROY` の有無に関わらず)
- マスタ投入: `make import`・`make import-k8s`(`import-dry-run`・`import-fetch`・`import-check-upstream` は対象外。
  ネットワーク取得・報告のみで DB に触れないため)
- クラスタ削除(Make経由): `make down`
- 秘密の読み取り: `kubectl.*get secret`・コマンド文字列に `.env`・`~/.ssh` を含むもの(サブプロセス経由の迂回を含む)
- main への反映: `git push` かつ push 先が `main` と解釈できるもの(`main`・`:main`・`main:`・`HEAD:main`・
  `HEAD:refs/heads/main`・`refs/heads/main`。単語境界チェックで「ドメイン」等の偽陽性は許容し、見逃しを優先して防ぐ)
- 強制系の push: `--force`・`-f`(gitのオプションとして)・`+`(refspecの強制記法)・`--mirror`・`--all`
- `gh pr merge`(main への GitOps 反映を伴うため)

該当しなければ何も出力せず exit 0(通常の許可フローに委ねる)。**exit 2 を使うのは、JSON の `permissionDecision`
だけでは `permissions.allow` に上書きされる余地が残るため**(公式ドキュメントが明言)。

### 3. Codex 側にも同じスクリプトを使う
`.codex/config.toml` の `[[hooks.PreToolUse]]`(`matcher = "^Bash$"`)から同じ `scripts/ai-guard/bash-guard.sh` を呼ぶ。
Codex の PreToolUse は `permissionDecision: "ask"` を公式にサポートしない(`allow`/`deny` のみ)ため、Codex 側は
該当パターンを **常に deny**(exit 2 相当)にする。加えて `.codex/config.toml` に `approval_policy = "on-request"`・
`sandbox_mode = "workspace-write"` を明示し(未設定だと利用者のグローバル既定に委ねられ、リポジトリ側の意図が保証
されないため)、フックが対応しない操作(未知のツール等)についても対話的な承認を既定にする。

### 4. 1本のPRにまとめる
「全レーンのAIに効くので運用として1本のPRにしてください」という調整役の指示どおり、`.claude/settings.json`・
`.codex/config.toml`・`scripts/ai-guard/bash-guard.sh`・回帰テスト(`scripts/ai-guard/bash-guard_test.sh` 相当。
issue #239/#273 が列挙した回避形がすべて block されることを確認する静的テスト)を1つのブランチ・1つのPRにまとめる。

### 5. マージはユーザー承認を経る
設定ファイルの変更は全レーンの全セッションに即座に影響する(`gh pr merge`・`git push` に確認が出るようになる等、
体験が変わる)。実装後、PR は作成するが **このタスクを担当したセッション(素早さレーン代行・調整役)のどちらもマージ
しない**。CURRENT_STATE.md に既定案と影響を書いた上でユーザーの確認・承認を待つ。

## 影響(各レーンへ)
- 今後、`make down`・`make import`・`make import-k8s`・`make migrate-down*`・`kubectl` での ns/pvc/secret 等の削除・
  取得、`git push` の main 反映、`gh pr merge` は、Claude Code・Codex のどちらでも AI エージェントからは実行できなくなる
  (人間が自分の端末で実行する必要がある)。日常の `make test`・`make lint`・`make build`・`go test`・`kubectl get/describe/logs`・
  featureブランチへの `git push`・`gh pr create` は今までどおり確認なしで通る。
- 既存の各レーンの `docs/runbooks/*.md` に `gh pr merge` を含む手順があれば、その箇所だけ「人間が実行」に変わる
  (レーン自身のセッションが最後まで自動でマージできない)。影響を受けるレーンには気づき次第、個別に連絡する。

## 却下した案
- `permissions.allow` の glob パターンを網羅的に増やすだけで対応する: 公式ドキュメントが「コマンド文字列の書き方を
  変える回避は防げない」と明言しており、いたちごっこになるため主防御にしない(#1 の通り補助防御に留める)。
- Codex 側を放置し Claude Code だけ直す: 調整役の指示(1本のPRで全レーンのAIに効くように)と、issue #239 が
  Codex の欠落を明示的に指摘しているため、両方を同じスクリプトで対応する。
