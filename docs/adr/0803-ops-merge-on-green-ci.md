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

### 判定
- 対象は `merge` の後の最初の非フラグトークン(PR 番号・URL・ブランチ)。無ければ現在のブランチの PR。
- 読み取り専用の `gh pr checks <対象>` を実行し、終了コード 0(全件 pass)のときだけ通す。
  `-R`/`--repo` は `gh pr checks` にも同じ値を渡し、検証する PR とマージする PR を一致させる。
- 引数は `^[A-Za-z0-9._/:@#-]+$` の単純な文字だけ許可する。コマンド置換・変数・バッククォート・クォート内の区切り文字・
  位置引数が複数などで対象を静的に確定できないものはブロックする。
- fail-safe: 終了コード 1(失敗)・8(未完了)・gh が無い・認証切れ・ネットワーク不可・タイムアウトなど、判定不能はすべてブロックし、理由を stderr に出す。
- `--admin`(チェック回避)は常にブロック。`gh api` の `/merge`・`/merges`・GraphQL `mergePullRequest` の直叩きも
  CI 検証を迂回するため引き続きブロックする。
- ガードが実行する外部コマンドは、`git rev-parse`(push 宛先の解決。ADR-0800 §2)と、検証済み引数の `gh pr checks` だけ。
  判定対象のコマンド(マージ自体)は実行しない。`timeout` / `gtimeout` があれば 20 秒で打ち切る(macOS には標準で無いため、
  無ければ gh の標準動作に任せる)。

### 変えないもの(引き続き人間の確認・禁止)
- main への直接 push・force push・`--mirror`・`--all`(PR 経由でのみ main に入れる)
- クラウドへのデプロイ・課金が発生する操作
- 機密情報の読取・公開(`.env`・SSH 鍵・`kubectl get secret`)
- kubectl の一括/データ削除・`k3d cluster delete`・`make down` 系・`known_diffs.yaml` への追加

### 設定
`.claude/settings.json` の `permissions.allow` には `gh pr merge` の許可を置かない(判定は常にこのフックで行う)。
`.codex/config.toml` も同じ `bash-guard.sh` を呼ぶため、Codex にも同じ規則が効く。

## 結果
- マージの手順は、PR 作成 → `gh pr checks` で全件成功を確認 → `gh pr merge`。CI が赤・未完了なら止まって直す。
- CI が緑でも、PR の内容そのもの(テストの弱体化・機密の混入)は保証されない。レーンのテスト・lint・`make check-publishable`・
  独立レビューを PR 前に行う運用(COORDINATION.md)は変わらない。
- 自動テスト: `scripts/ai-guard/bash-guard_test.sh`(偽の gh を PATH に置き、終了コード 0/1/8/127 での挙動・危険な引数・
  `--admin`・gh が無い場合・マージ自体を実行しないことを検証)。
