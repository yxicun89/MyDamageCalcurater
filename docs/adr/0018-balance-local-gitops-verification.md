# ADR-0018: balance の GitOps(Argo CD)をローカル k3d で検証する

- 状態: 採用(2026-09-22。方式はユーザー回答「ローカル k3d で試す」。細部はタイプバランスレーンの判断)
- 日付: 2026-09-22
- 関連: docs/type-balance-design.md §4「GitOps / Argo CD」・§14、DECISIONS.md「Argo CD を共通デプロイ基盤にする」「最新の安定版」、
  services/balance/deploy/argocd/README.md、COORDINATION.md(リモートの URL・認証情報を文書・コミットに書かない)

## 背景
TB0 の最後の項目「Git 変更 → Argo CD 同期 → Pod 更新」は、Argo CD・配布イメージの置き場所・private リポジトリの認証が無く未実施だった。
リポジトリの URL にはアカウント名が入るため Git に書けない。k3d クラスタ(`deploy/k3d.yaml`)はレジストリ無しで作られており、
作り直すと他のレーンの作業を止める。

## 決定
1. **Argo CD** は 2026-09-22 時点の最新の安定版 **v3.5.3** を、版を固定した公式 install manifest で `argocd` namespace に入れる
   (`kubectl apply -n argocd --server-side -f https://raw.githubusercontent.com/argoproj/argo-cd/v3.5.3/manifests/install.yaml`)。
2. **イメージの置き場所**は、クラスタ内のレジストリ(`services/balance/deploy/local-registry`。registry 3.1.1、digest 固定、hostPort 5000、emptyDir)。
   ノードの containerd は `localhost:5000` を平文で pull できる(containerd は localhost を HTTP で許可する)ので、クラスタの作り直しも
   containerd の設定変更も要らない。Mac からは `kubectl port-forward`(Mac 側は既定で 5001。macOS の AirPlay 受信が 5000 を使うことがあるため)経由で `crane` で push する(Docker Desktop のデーモンからは Mac の
   localhost に届かないため `docker save` した tar を push する)。`make balance-registry-apply` / `make balance-registry-push`。
3. **GitOps overlay**(`deploy/k8s/overlays/gitops`)は `localhost:5000/pokecalc/balance@sha256:...` を Git に持つ(digest 固定。秘密でない)。
   イメージを変えるときは push して表示された digest を overlay に書き、PR で main に入れる。
4. **Application の repoURL は Git に書かない**。`application.yaml` は placeholder のままにし、`make balance-argocd-app`
   (`scripts/argocd-local-app.sh`)が `git remote get-url origin` の HTTPS URL を適用時に埋め込んで `kubectl apply` する。
   `check-gitops.sh ready` は `BALANCE_GITOPS_REPO_URL` で適用時の値を検査する。
5. **private リポジトリの認証**は、ユーザーが作った読み取り専用の fine-grained PAT を、ユーザー自身が `argocd` namespace の repository Secret
   (Argo CD の repository 用ラベル付き)に登録する。AI はトークンを見ない・Git に入れない。
6. 同期は manual のまま(自動 sync・prune・selfHeal は有効にしない)。`targetRevision` は `main`。

## 影響と制約
- gitops overlay には read model(ポケモン・技・特性)のマウントが無いので、Argo CD で同期した balance は health は 200、analyze / coverage は 503
  (実データの配布は ADR-0014 の未決。P2-2 に合わせる)。local overlay(`make balance-k3d-deploy`)で上書きすると Application は OutOfSync になる
  (manual sync なので自動では戻さない)。
- クラスタ内レジストリは認証が無く、k3d の docker ネットワーク内(ノード IP:5000・Service)から push できる。Mac には公開していないので検証用として許容する。registry イメージは root で動く(seccomp は RuntimeDefault)。
- クラスタ内レジストリは emptyDir なので、Pod が作り直されるとイメージは消える。そのときは push し直す(digest は同じイメージなら同じ)。
- クラウドへ出すときは、レジストリ(ECR / Artifact Registry)と overlay の newName を差し替える。

## 却下した案
- k3d クラスタをレジストリ付きで作り直す: 他のレーンの作業を止める(クラスタ削除は人間の確認事項)。
- repoURL を Git に書く: アカウント名を公開しない方針(R-2-8)に反する。
- ユーザーの gh の認証トークンを流用する: 権限が広すぎる。読み取り専用・リポジトリ限定の PAT にする。
