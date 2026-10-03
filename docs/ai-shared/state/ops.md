## Ops
Lane: 運用(deploy・scripts・AIエージェントの権限設定。専任セッションなし。空席時は手が空いたレーンが調整役の
割り当てで代行できる。COORDINATION.md)
Active: 素早さレーン(調整役「damage calculation bug resolution」からの割り当て。**ユーザーの直接指示ではない**。
運用の担当欄は変わらず「運用」のまま、今回だけ空席を代行。DECISIONS.md 参照)
Branch: 次は main から fix/ops-issue273-239 を切る
Status: 2026-09-25、issue #273・#239(AIエージェントの権限設定が、CLAUDE.mdの「人間の確認が必要」な操作(クラスタ削除・
DBのデータ削除・main反映・秘密の読み取り)を止められない)に着手。既定案(ADR-0800):
(1) `.claude/settings.json` の `permissions.allow` から `Bash(make *)`・`Bash(kubectl *)`・`Bash(k3d *)`・`Bash(docker *)`・
`Bash(git push *)`・`Bash(gh pr merge *)` 等の広い許可を外し、読み取り・非破壊のサブコマンド単位に絞る。
(2) 主防御として `hooks.PreToolUse`(Bash)から `scripts/ai-guard/bash-guard.sh` を呼び、コマンド文字列を検査して
該当すれば exit code 2 で無条件 block(`permissions.allow` があっても上書きされない。公式ドキュメントで確認済み)。
対象: `make down`・`make import`・`make import-k8s`・`make migrate-down*`・`kubectl` での ns/namespace/pvc/pv/
statefulset/secret の delete・`kubectl get secret`・`.env`/SSH鍵ディレクトリを含むコマンド・`k3d cluster delete/rm`・
`git push` の main 反映(`main`・`:main`・`HEAD:refs/heads/main` 等、書き方によらず)・force push 系・`gh pr merge`。
(3) `.codex/config.toml` にも同じスクリプトを `[[hooks.PreToolUse]]` から呼ぶ設定を追加し、`approval_policy`・
`sandbox_mode` を明示する(Codexのpermissionは`deny`のみ対応、`ask`は無い)。
**影響**: 上記に該当する操作は、どのレーンのセッションでも(Claude Code・Codex とも)AIエージェントからは実行できなく
なり、人間が自分の端末で実行する必要がある。`make test`・`make lint`・`make build`・`go test`・`kubectl get/describe/logs`・
featureブランチへの `git push`・`gh pr create` は従来どおり確認なしで通る。各レーンの runbook に `gh pr merge` を
AIが実行する手順があれば、そこだけ「人間が実行」に変わる(気づき次第、該当レーンへ個別連絡)。
**このPRは実装後もこのセッションはマージしない**(設定ファイルの変更で全レーンに影響するため、ユーザーの確認・
承認を経てからのマージとする。ADR-0800 §5)。
Status(追記): 2026-09-25、critic(Opus)1回目 FAIL。テスト268件は green だったが、テストに無い普通の書き方
(`git push origin main 2>&1`・`bash -c "make test && make down"`・`kubectl delete statefulsets mysql`(複数形)・
`$HOME/.ssh` 等)でガードを迂回できる穴が複数見つかった。加えて **Codex 側の実効性は未検証**と判明: codex-cli の
バイナリ文字列調査で、プロジェクトローカルの hooks はディレクトリの信頼+フックごとの人間確認(TUI)を経るまで
読み込まれない可能性があり、`$(git rev-parse ...)` のシェル展開や `tool_name` が `"Bash"` になるかも実機未確認
(ADR-0800 §3 に追記)。`services/pokedex/db/layout_test.go` の `TestNoAutomaticDown` が `scripts/ai-guard/` 配下の
検知用文字列 `migrate-down` に誤反応する既知の1件は、調整役(damage calculation bug resolution)経由でデータレーン
(4f)へ最小除外を依頼済み(ai-guardの実装自体の欠陥ではない)。critic指摘の修正をimplementerへ差し戻し中(2回目)。
Next: critic 2回目レビュー → PASSしたらPRを作成してユーザーに提示する(マージは求めない。データレーンの
TestNoAutomaticDown修正がmainに入るまでこのPRはマージ不可であることをPR本文に明記する。Codex側は「未検証」と
明記し、実地確認〈Codexを起動してフックを信頼・確認する手順〉を宿題として残す)。
