# balance GitOps setup

このディレクトリは、公開用クリーンコピーをprivate Git repositoryへpushした後に使う。
現在の開発repositoryへ公開用remoteを追加せず、履歴をそのままpushしない。

公開用コピーの作成担当はClaude Code/Codexのどちらかへ固定しない。ユーザーから依頼された側が、
root `docs/plan.md` のR-2-9とmain正本の`docs/ai-shared/DECISIONS.md`に従い、別ディレクトリのlocal cloneで
作者名・メールと個人accountを含むmodule pathを置換する。`check-publishable-full`を含む履歴全体の
検査が成功したコピーにだけ、作成したprivate remoteを追加する。balance側から元repositoryの履歴や
remoteを変更しない。R-2-9のスクリプトが未完成なら、手作業で代替せず完成を待つ。

作成・更新した担当は、自分のfeature branchだけで完了を記録しない。元repositoryのmain正本にある
`docs/ai-shared/CURRENT_STATE.md`の担当欄と自分のlogへ、秘密を含まない相対path、source commit、
clean copyのbranch/commit、実行した公開前検査、remote設定/pushの成否を記録する。切替時はclean copy側の
`docs/ai-shared/`も更新し、以後どちらを開発正本にするかを明記して、次のClaude Code/Codexが迷わない状態にする。

## Gitへ入れる値

1. `application.yaml` の `repoURL` を、作成したrepositoryのclone URLへ置換する。
2. 必要なら `targetRevision` を公開用repositoryのdefault branchへ合わせる。
3. `../k8s/overlays/gitops/kustomization.yaml` の `newName` と `digest` を、registryへpushした
   balance imageのrepository名とdigestへ置換する。`latest`、可変tag、ローカルimageは使用しない。
4. repository rootで次を実行し、placeholderが残っていないことを確認する。

```sh
make -f services/balance/Makefile balance-gitops-check
```

registryへ安全な方法でloginした後、commit SHA等の一意なtagでmulti-platform imageをpushできる。
コマンドの最後に表示されるdigest参照をGitOps overlayへ記録する。

```sh
BALANCE_RELEASE_IMAGE=registry.example.invalid/pokecalc/balance:COMMIT_SHA \
  make -f services/balance/Makefile balance-docker-push
```

上の`.invalid` repositoryは例であり、そのまま実行しない。スクリプトもplaceholderを拒否する。

## Gitへ入れない値

- Git access token、SSH private key
- container registryのpassword/token
- Kubernetes Secretの実値
- ローカル絶対パス

private Git repositoryのcredentialはArgo CDへrepository credentialとして登録する。
private container imageを使う場合は、`private-registry-patch.example.yaml` を参考にDeploymentへ
Secret名だけを設定し、Secret本体はクラスタへ別経路で作成する。patchを利用する場合は公開用コピーで
`.example.yaml` を `private-registry-patch.yaml` として複製し、GitOps overlayの`patches`へ追加する。
公開用コピーにも秘密を含めない。

## 初回適用とmanual sync

Argo CDはversionを固定してクラスタへ導入する。`latest`のinstall manifestは使わない。
Application CRDとrepository credentialの準備後に、次を実行する。

```sh
make -f services/balance/Makefile balance-gitops-apply
argocd app sync pokecalc-balance
kubectl -n argocd get application pokecalc-balance
kubectl -n pokecalc rollout status deployment/balance --timeout=120s
make -f services/balance/Makefile balance-smoke
```

自動syncは設定しない。TB0ではmanual syncで、Gitのcommit、Argo CDの同期revision、稼働Podの
image digestを記録して一致を確認する。
