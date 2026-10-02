# ADR-0803: PR のマージは対象 PR の CI が全件成功のときだけ自動で許可する

- 状態: 採用(2026-10-03。ユーザー決定)
- 日付: 2026-10-03
- 関連: ADR-0800 §2(bash-guard の block 対象)・§5、COORDINATION.md「main への統合(PR)」

## 背景
ADR-0800 §2 は PR のマージを常に人間確認にしていた。ユーザーは 2026-10-03 に次のとおり決定した。
「全レーンでテストと CI が通っていれば AI が PR をマージしてよい。クラウドへの勝手なデプロイ(お金が発生するもの)と
git の機密情報公開以外は作業を止めたくない」。

## 決定
`scripts/ai-guard/bash-guard.sh` の PR マージ(`gh pr merge`)を、無条件ブロックから
「**対象 PR の CI が全件成功のときだけ通す**」に変える。ADR-0800 §2 の「`gh pr merge` は常に確認」はこの ADR で置き換える。

### 判定(許可リスト方式。想定した形だけ通し、未知のものはすべてブロック)
- **単独コマンドのみ**: 他のコマンドとの連結(`&&`・`;`・`|` 等。`git push ... && マージ` を含む)、gh より前の `cd`・`pushd`・`xargs`
  (対象が空になる・別 worktree を見る)はブロック。
- **対象の明示が必須**: PR 番号か `https://host/owner/repo/pull/N` の URL を1つだけ。引数なし(現在のブランチ)・ブランチ名・複数はブロック。
  `-` で始まる対象・`-R` の値(`-R --admin 5` 等の引数注入)もブロック。
- **HEAD の SHA を固定(TOCTOU 対策)**: ガードが読み取り専用の `gh pr view <対象> --json headRefOid` で HEAD を取り、
  コマンドに `--match-head-commit <その SHA>` が明示されているときだけ通す。省略・不一致はブロック(CI 確認後に push された commit をマージしない)。
- 次に読み取り専用の `gh pr checks <対象>` を実行し、終了コード 0(全件 pass)のときだけ通す。`-R`/`--repo` は両方に同じ値を渡す。
- フラグは許可リスト: 長いフラグは `--auto --disable-auto --delete-branch --merge --rebase --squash` と、値を取る
  `--repo --body --body-file --subject --author-email --match-head-commit`(`=` 形も)。短いフラグは `-d -m -r -s` と、値を取る `-R -b -F -t -A`。
  まとめ書き(`-sb 504`)は1文字ずつ見て、値を取る文字が末尾なら次のトークン、途中ならそれ以降を値とする。`-R<v>`・`-R=<v>`・まとめ書き中の R からも
  リポジトリの値を取り出す。未知のフラグはブロック。`--admin`(チェック回避)は常にブロック。
- コマンド中の `$`・バッククォート・クォート内の区切り文字、環境変数 `GH_REPO`・`GH_HOST`・`GH_TOKEN`・`GH_ENTERPRISE_TOKEN`・`GH_CONFIG_DIR`
  (`GITHUB_TOKEN`・`XDG_CONFIG_HOME` も。export 形を含む)があるものはブロック。
- `gh alias set/import` はブロック。gh の組み込みでない第1サブコマンド(alias/extension の可能性)の後ろに `merge` があるものもブロック。
- fail-safe: 終了コード 1(失敗)・8(未完了)・gh が無い・認証切れ・ネットワーク不可・時間切れなど、判定不能はすべてブロックし、理由を stderr に出す。
- `gh api` の `/merge`・`/merges`・GraphQL `mergePullRequest` の直叩きも CI 検証を迂回するため引き続きブロックする。
- ガードが実行する外部コマンドは、`git rev-parse`(push 宛先の解決。ADR-0800 §2)と、検証済み引数の読み取り専用の
  `gh pr view --json headRefOid`・`gh pr checks` だけ。判定対象のコマンド(マージ自体)は実行しない。
- **タイムアウト**: macOS には `timeout`/`gtimeout` が無いため、バックグラウンド実行と自前のタイマー(0.1 秒刻みのポーリング)で 20 秒打ち切り、
  打ち切ったらブロックする(`gh pr view`・`gh pr checks` の両方)。フック自体のタイムアウト時の挙動は**未検証**
  (ガード自身の 20 秒打ち切りでブロックする)。`.claude/settings.json` のフック・権限設定は変更しない。

### 既知の限界(critic 再レビューで記録。GitHub 側のブランチ保護が無いため、ガードだけが関門である前提)
- `gh api graphql --input <ファイル>`・`-F/-f query=@<ファイル>` で mergePullRequest をファイルに書いて呼ぶ形は、文字列判定では見えない(ADR-0800 §2 から続く既存の限界)。
- `~/.config/gh/config.yml` を直接書き換えて作った gh の別名(`merge` の語を含まない)経由のマージ。`gh alias set/import` はブロックする。
- 空のクォート(`''`)でガードと実際のシェルの引数の区切りがずれる形。ただし `--match-head-commit` の SHA 照合で、検証した commit 以外はマージされない。
- `env -C <dir>`・`GIT_DIR=` でリポジトリの解決先がずれる形(同じ SHA を持つ別リポジトリの PR がある場合だけ)。
- 検証の順序による競合(view と checks の間に HEAD を往復させる敵対的な操作)。GitHub 側のブランチ保護(必須チェック)を有効にすれば根本的に塞がる(人間の設定。今は未設定)。

### 変えないもの(引き続き人間の確認・禁止)
- main への直接 push・force push・`--mirror`・`--all`(PR 経由でのみ main に入れる)
- クラウドへのデプロイ・課金が発生する操作
- 機密情報の読取・公開(`.env`・SSH 鍵・`kubectl get secret`)
- kubectl の一括/データ削除・`k3d cluster delete`・`make down` 系・`known_diffs.yaml` への追加

### 設定
`.claude/settings.json` の `permissions.allow` には `gh pr merge` の許可を置かない(判定は常にこのフックで行う)。
`.codex/config.toml` も同じ `bash-guard.sh` を呼ぶため、Codex にも同じ規則が効く。

## 結果
- マージの手順は、PR 作成 → `gh pr checks N` で全件成功を確認 → `gh pr view N --json headRefOid -q .headRefOid` の SHA を
  `--match-head-commit` に付けて、PR 番号を明示した単独のマージ。CI が赤・未完了なら止まって直す。
- CI が緑でも、PR の内容そのもの(テストの弱体化・機密の混入)は保証されない。レーンのテスト・lint・`make check-publishable`・
  独立レビューを PR 前に行う運用(COORDINATION.md)は変わらない。
- 自動テスト: `scripts/ai-guard/bash-guard_test.sh`(偽の gh を PATH に置き、終了コード 0/1/8/127 での挙動・危険な引数・
  `--admin`・gh が無い場合・マージ自体を実行しないことを検証)。
