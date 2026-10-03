## 2026-10-03: GitHub リポジトリ設定を有効化(データレーン。issue #243・ユーザー決定「既定案で OK」)
Decision: リポジトリが public になっていて費用なしで使えるため、#243 の受け入れ条件の設定を適用した:
- `delete_branch_on_merge: true`(マージ後のブランチを自動で消す)
- `allow_squash_merge: false`・`allow_rebase_merge: false`・`allow_merge_commit: true`(COORDINATION.md の「マージコミットで入れる」と一致)
- Dependabot alerts(vulnerability-alerts)と Dependabot security updates(automated-security-fixes)を有効化
Reason: ユーザー決定 2026-10-03「既定案で OK」。
Impact:
- **main のブランチ保護(PR 必須・CI 〈`test / lint / build / golden / wasm / kustomize / check-publishable`〉必須・force push と削除の禁止・
  enforce_admins は false)は未適用**: Claude Code の自動モードが「CI 設定の変更」として止めたため、人間が実行する(PENDING.md)。
  実行例(リポジトリのルートで。値は既定案): `gh api -X PUT repos/<owner>/<repo>/branches/main/protection --input <JSON>`(JSON は
  required_status_checks.contexts に上の CI 名・strict false、required_pull_request_reviews.required_approving_review_count 0、
  allow_force_pushes false、allow_deletions false、enforce_admins false、restrictions null)
- リポジトリを非公開に戻すと、無料プランではブランチ保護が使えなくなり、Argo CD(認証なしの https で取得)も取得できなくなる。非公開に戻すかは
  #328 とあわせて人間が決める(PENDING.md)
