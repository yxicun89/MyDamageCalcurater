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
など、破壊的でない Makefile ターゲット)。既存の `ask`/`deny` の書き方(先頭一致に近い glob)は維持するが、
issue #239/#273 が列挙した回避形の網羅的な追加パターンを `permissions.ask` にまでは実装しない(公式ドキュメントが
認める通り、コマンド文字列の書き方を変える回避は glob の追加だけでは防げず、いたちごっこになるため)。主防御は
次の PreToolUse フックに置き、`permissions` 側は既存の非破壊許可に絞ることだけを役割とする。

### 2. PreToolUse フックで実際のコマンド文字列を検査する(主防御)
`.claude/settings.json` に `hooks.PreToolUse`(matcher: `"Bash"`)を追加し、`scripts/ai-guard/bash-guard.sh`
(新規。Claude Code・Codex 両方から呼べるよう共通化)を呼ぶ。スクリプトは stdin の JSON から `tool_input.command` を
読み、クォート文字とシェルのメタ文字(`&&`・`||`・`;`・`|`・`&`・`(`・`)`・`{`・`}`・`` ` ``・改行・タブ等)を空白に
置き換えて1本のトークン列に平坦化した上で判定する(`timeout`・`env`・`nohup` 等の前置ラッパー、`bash -c`・`sh -c`・
サブシェル・`eval` 等の形に関わらず、`git`・`make`・`kubectl`・`k3d`・`gh` のトークンがどの位置に出現しても拾える
方式。前置きの種類を個別に列挙しない)。コマンド名の比較は大文字小文字を区別せず、エイリアス無効化の先頭 `\` も
剥がしてから行う。該当するトークンが見つかるたびに、次のいずれかに一致したら **exit code 2** で無条件に block する
(`permissionDecisionReason`/stderr に理由と、人間が自分の端末で実行する想定であることを書く):
- クラスタ削除: `k3d cluster (delete|rm)` 相当
- データ削除: `kubectl` の `delete` で対象が `ns`/`namespace`/`pvc`/`persistentvolumeclaim`/`pv`/`persistentvolume`/
  `statefulset`/`secret`/`all`(引数の順序に依存しない)、`kubectl delete -k`/`--kustomize`/`-f`/`--filename`/`-R`/
  `--recursive`(kustomize・ファイル指定・再帰の一括/間接削除)、対象がコマンド上で特定できない(パイプ/xargs越し・
  コマンド置換/変数展開で動的に決まる・空)場合も安全側でブロックする
- DB ロールバック: `make migrate-down*`(`CONFIRM_DESTROY` の有無に関わらず)
- マスタ投入: `make import`・`make import-k8s`(`import-dry-run`・`import-fetch`・`import-check-upstream` は対象外。
  ネットワーク取得・報告のみで DB に触れないため)
- クラスタ削除(Make経由): `make down`
- 秘密の読み取り: `kubectl.*get secret`・コマンド文字列に `.env`・`.ssh` を含むもの(前置き不問。`$HOME/.ssh`・
  絶対パスも含む。サブプロセス経由の迂回・リダイレクト対象そのものを指す形を含む)
- main への反映: `git push` かつ push 先が `main` と解釈できるもの(`main`・`:main`・`main:`・`HEAD:main`・
  `HEAD:refs/heads/main`・`refs/heads/main`。単語境界チェックで「ドメイン」等の偽陽性は許容し、見逃しを優先して防ぐ。
  宛先省略・`HEAD`・`@` の場合は現在のブランチを解決する。`git -C <dir> push`・`cd <dir> && git push` の形では
  フック自身の cwd ではなく `<dir>` を基準に解決する)
- 強制系の push: `--force`・`-f`(gitのオプションとして)・`+`(refspecの強制記法)・`--mirror`・`--all`
- `gh pr merge`(main への GitOps 反映を伴うため)、`gh api` での `/merge`・`/merges` パス直叩き・GraphQL の
  `mergePullRequest`

該当しなければ何も出力せず exit 0(通常の許可フローに委ねる)。**exit 2 を使うのは、JSON の `permissionDecision`
だけでは `permissions.allow` に上書きされる余地が残るため**(公式ドキュメントが明言)。

**対象外とする既知の限界**: 以下は critic レビューで対象外と明記されたもので、今回のスコープでは対応しない。
- 変数展開による難読化(ただし git push の宛先が `$` 始まりの場合と kubectl の対象が `$` 始まりの場合は安全側でブロックする)
- 波括弧展開による難読化(例: `ma{in,}`)
- 別チェックアウトを指す `GIT_DIR=`・`--git-dir`・`--work-tree`・`pushd`(拾うのは `git -C` と `cd` のみ)
- `gh api` の merge URL にクエリが付く形(`.../merge?x=1`)
- `.envrc`(direnv固有ファイル)
- base64等によるコマンド文字列そのものの難読化

### 3. Codex 側にも同じスクリプトを使う(**実効性は未検証**)
`.codex/config.toml` の `[[hooks.PreToolUse]]`(`matcher = "^Bash$"`)から同じ `scripts/ai-guard/bash-guard.sh` を呼ぶ。
Codex の PreToolUse は `permissionDecision: "ask"` を公式にサポートしない(`allow`/`deny` のみ)ため、Codex 側は
該当パターンを **常に deny**(exit 2 相当)にする。加えて `.codex/config.toml` に `approval_policy = "on-request"`・
`sandbox_mode = "workspace-write"` を明示し(未設定だと利用者のグローバル既定に委ねられ、リポジトリ側の意図が保証
されないため)、フックが対応しない操作(未知のツール等)についても対話的な承認を既定にする。

**注(critic 1回目レビューで判明)**: codex-cli のバイナリ文字列調査で、プロジェクトローカルの hooks 設定は
「ディレクトリの信頼」と「フックごとの人間の確認(TUI)」の両方が済むまで読み込まれないらしい痕跡
(`trusted_hash`・"1 hook needs review before it can run."・"Trusting the directory allows project-local
config, hooks ... to load")が見つかった。加えて、`command` フィールドがシェル経由で `$(git rev-parse
--show-toplevel)` を展開するか、Codex が Bash 実行時に `tool_name` を `"Bash"` として渡すかは、実際に Codex を
起動して確認できていない。**したがって現時点では、Codex 側でこのフックが実際に機能することは保証できない**
(展開されない・`tool_name` が一致しない場合、フックは何も知らせずに無効化される)。設定は「意図の表明」として
入れるが、有効化には人間が実際に Codex を起動してフックを信頼・確認する手順が要る可能性が高い。実地確認は
運用レーン(代行中の素早さレーン、または次に担当する AI・人間)の宿題として残す。

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
  (人間が自分の端末で実行する必要がある)(Codex 側はフックの信頼・確認が済むまで効かない可能性がある。§3参照)。
  日常の `make test`・`make lint`・`make build`・`go test`・`kubectl get/describe/logs`・
  featureブランチへの `git push`・`gh pr create` は今までどおり確認なしで通る。
- 既存の各レーンの `docs/runbooks/*.md` に `gh pr merge` を含む手順があれば、その箇所だけ「人間が実行」に変わる
  (レーン自身のセッションが最後まで自動でマージできない)。影響を受けるレーンには気づき次第、個別に連絡する。

## 却下した案
- `permissions.allow` の glob パターンを網羅的に増やすだけで対応する: 公式ドキュメントが「コマンド文字列の書き方を
  変える回避は防げない」と明言しており、いたちごっこになるため主防御にしない(#1 の通り補助防御に留める)。
- Codex 側を放置し Claude Code だけ直す: 調整役の指示(1本のPRで全レーンのAIに効くように)と、issue #239 が
  Codex の欠落を明示的に指摘しているため、両方を同じスクリプトで対応する。

## 追記(2026-10-03): CI が通った PR のマージを許可(ユーザー決定)

- ユーザー決定: 「テストと CI が通っていたら PR をマージしてよい。作業を止めるのは、お金が発生する操作(クラウドへのデプロイ等)と、
  機密情報の公開だけにしたい」
- 変更: `gh pr merge` を、次をすべて満たすときだけ許可する(`check_gh_pr_merge`)。満たさなければ従来どおりブロック
  - PR の指定(番号・URL・ブランチ名)がちょうど1つで、動的に決まらない(`$`・`` ` `` を含まない)
  - `--admin`(保護の迂回)・`--auto`(条件が揃う前の予約)を使わない
  - `gh pr checks <PR>` が終了コード 0(全チェック成功。実行中は 8 でブロック)で、チェックが1件以上ある
  - `gh pr view <PR>` が `OPEN MERGEABLE`(GitHub が遅延して計算する `UNKNOWN` の間は2秒おきに最大5回問い合わせ直す)
- 変えないもの: `gh api` での merge 直叩き・GraphQL の mergePullRequest(CI の確認を迂回するため)、main への直接 push・強制 push、
  秘密の読み取り、クラスタ削除・データ削除などの他のブロック(これらの緩和は別途ユーザーの判断)
- テスト: `scripts/ai-guard/bash-guard_test.sh` の `test_pr_merge_requires_green_ci`(偽の gh で、成功・失敗・実行中・チェック無し・
  競合・判定中・クローズ済み・指定なし・複数・動的・`--admin`・`--auto`・gh が無い)。従来の「常にブロック」のテストは期待を反転した。
  他の既存テストは存在しない gh を渡して決定的にブロック側に倒す
- 実物での確認(2026-10-03): 存在しない PR はブロック、CI が全部通っていて競合の無い実在の PR は許可されることを、フックに JSON を
  渡して確認した(マージはしていない)
- 限界: フックが GitHub に問い合わせるので、ネットワークが無いとマージはブロックされる(安全側)。手元のテストが通ったことはフックでは
  確かめない(CLAUDE.md・COORDINATION.md の運用ルールとして「手元のテスト・lint・公開前検査が通ってから」を明記した)
