# ADR-0408: Argo CD の AppProject 限定・スクリプト共通化・レジストリ永続化(issue #263・#292)

- 状態: 採用(2026-09-25。全体レビュー issue #263〈タイプバランス主担当〉・#292〈タイプバランス/素早さ/運用〉への対応。
  #292 はユーザー決定〈GitHub issue #292 コメント、2026-09-25〉で既定案を採用)
- 日付: 2026-09-25
- 関連: ADR-0018(balance の Argo CD 導入)、ADR-0405(install物のハッシュ/digest固定)、ADR-0605(speed の GitOps 共有)、
  docs/runbooks/balance.md、docs/runbooks/speed.md、issue #263・#292・#241(#292に統合済み)

## 背景
- **#263**: 両サービスの Argo CD `Application` が `project: default`(制限なし。任意の repo・任意の namespace・
  cluster スコープ資源への同期を許す)。`argocd-initial-admin-secret` が削除されずクラスタに残る。
  balance/speed の GitOps スクリプト5本(`argocd-local-app.sh`・`check-gitops.sh`・`publish-image.sh`・
  `local-registry-push.sh`・`k3d-deploy-readmodel.sh`)はサービス名・環境変数名の差だけのほぼ複製。
- **#292**: balance の Argo CD Application は恒常的に OutOfSync(`docs/runbooks/balance.md` §9 の「local overlay で
  上書きして動作確認する」手順の終状態が OutOfSync のまま)。クラスタ内レジストリが `emptyDir` のため、
  registry Pod が作り直されると push 済みイメージの実体が消え、次の sync で `ImagePullBackOff` になる。
  speed の gitops overlay は digest が全0の placeholder のまま(speed 側が実イメージを push すれば解消。
  本ADRの対象外)。

## 決定

### 1. AppProject `pokecalc` を新設し、両 Application が参照する(#263)
`deploy/argocd/appproject.yaml`(リポジトリルート。balance/speed どちらの所有物でもない共有インフラなので
`scripts/argocd-bootstrap.sh` と同じ置き場所の考え方に揃える)に以下を定義する。
- `sourceRepos`: このリポジトリの HTTPS URL のプレースホルダ(`application.yaml` と同じ「Git に書かず適用時に
  埋め込む」方式。ADR-0018 §4)。
- `destinations`: namespace `pokecalc`、server `https://kubernetes.default.svc` の組のみ。
- `clusterResourceWhitelist`: 空(cluster スコープ資源への同期を許さない)。
- `namespaceResourceWhitelist`: Service・Deployment・Ingress の3種(gitops overlay が実際に描画する種別)だけに絞る(ワイルドカード `*` は使わない)。

`services/balance/deploy/argocd/application.yaml`・`services/speed/deploy/argocd/application.yaml` の
`spec.project` を `default` から `pokecalc` に変更する。`check-gitops.sh`(共通化後は `scripts/gitops/check-gitops.sh`。
§2)に、`project: default` を検出したら失敗する検査を追加する(#263 の境界値条件)。

### 2. GitOps スクリプトを `scripts/gitops/` に共通化し、`SERVICE=` 引数を取る1本にする(#263)
`services/{balance,speed}/scripts/{argocd-local-app,check-gitops,publish-image,local-registry-push,
k3d-deploy-readmodel}.sh` を、`scripts/gitops/{argocd-local-app,check-gitops,publish-image,
local-registry-push,k3d-deploy-readmodel}.sh` の5本(サービス名ではなく機能名)に統合する。各スクリプトは
`SERVICE=balance` または `SERVICE=speed` を必須の環境変数として受け取り、これまでサービス名で分岐していた
パス・環境変数名を導出する(例 `services/$SERVICE/deploy/argocd`、`${SERVICE^^}_GITOPS_REPO_URL` の代わりに
`SERVICE` から一意に導出する命名规則にする。既存の環境変数名〈`BALANCE_GITOPS_REPO_URL`等〉との後方互換は
不要。呼び出し元の Makefile を書き換える)。

`services/balance/Makefile`・`services/speed/Makefile` の該当ターゲット(`balance-argocd-app`・
`speed-argocd-app` 等)は、共通スクリプトを `SERVICE=` 付きで呼ぶだけの1行にする(ターゲット名自体は変えない。
呼び出し側の互換性を保つ)。ADR-0405 で「balance/speed は Go module の境界があるので複製する」と決めたのは
**Go コード**の話であり、この5本は shell script(モジュール境界に関係しない)なので共通化して問題ない。

### 3. クラスタ内レジストリを emptyDir から PVC(local-path)へ変える(#292)
`services/balance/deploy/local-registry/registry.yaml` の `emptyDir` を `PersistentVolumeClaim`(k3d 既定の
`local-path` StorageClass。P7-1 の Loki と同じ storageClass)に変える。push したイメージが registry Pod の
再作成後も残るようにし、#292 の「registry Pod を作り直しても image を戻せる(または永続化されている)」を
**永続化する**ことで満たす(runbook 手順の追加ではなく、そもそも消えなくする)。

容量は balance・speed 2サービス分の複数世代を見込んで 2Gi にする(importer の PVC 上限〈issue #111〉と同じ
桁数感。世代管理〈古いイメージの自動削除〉は今回作らない。先回りしない。容量が問題になったら別issueにする)。

### 4. runbook: 通常経路(PRマージ→sync)を Synced に保つ手順を明記する(#292 正常系)
`docs/runbooks/balance.md`(speed も同様)に、`main` へマージされた後の標準手順として
「`make balance-registry-push` → 表示された digest を `deploy/k8s/overlays/gitops/kustomization.yaml` に書いて
PR → main マージ後 `argocd app sync pokecalc-balance`(または Argo CD UI で Sync)」を明記する。
local overlay(`make balance-k3d-deploy`)で動作確認した後は、この手順で GitOps 側に戻すことを runbook に
明記する(#292 異常系の「戻し方」)。自動 sync(`syncPolicy.automated`)は導入しない(ADR-0018 §6 の manual 運用を
維持する。ユーザー決定は「常時 Synced」を自動化の意味ではなく、標準手順を踏めば Synced に**戻せる**状態を指す
と解釈する)。

### 5. `argocd-initial-admin-secret` の削除(#263)
削除操作自体は人間が行う(クラスタの認証情報に関わる操作。ADR-0018 §5 の「AI はトークンを見ない」と同じ扱い)。
`scripts/argocd-bootstrap.sh` の完了メッセージと runbook に、初回ログイン後に
`kubectl -n argocd delete secret argocd-initial-admin-secret` を実行する旨を追記する。

### 6. CLAUDE.md の「Argo CD が main を見ているので PR 必須」の文言(#292 決定に含まれる。保留)
ユーザー決定には「main を常に緑に保つため(CI・レビューの記録)」への書き換えが含まれるが、決定コメント自身が
「CLAUDE.mdの文言はユーザーに1回確認してから」としている。本ADRの実装ではCLAUDE.mdを変更せず、別途ユーザーに
確認してから対応する(このADRの範囲外として明記)。

## 影響と制約
- balance/speed の GitOps スクリプトの呼び出し方(Makefile ターゲット名)は変えない。中身が共通スクリプトの
  呼び出しに変わるだけなので、他レーン・runbook からの利用に影響しない。
- speed の gitops overlay の digest placeholder は本ADRでは解消しない(speed レーンが実イメージを push する
  作業が別途必要。AppProject・スクリプト共通化・レジストリ永続化の恩恵は speed にも及ぶ)。
- レジストリを PVC化すると、k3d クラスタを削除して作り直すとレジストリの内容も失われる(PVC は k3d クラスタの
  ボリュームに紐づく)。この場合は最初から push し直す(既存の運用と同じ)。

## 却下した案
- 全サービス(calc・gateway・web・pokedex・judge)を GitOps 化する: イメージ配布経路(レジストリ)が balance/speed
  用にしか無く、先に整備が必要。ユーザー決定により見送り(#292 既定案)。
- `syncPolicy.automated` を有効にする: ADR-0018 §6 の manual 運用の決定を変える大きな変更で、#292 のユーザー決定は
  そこまで求めていないと判断(§4 参照)。
- local overlay 用に別 namespace を新設し、GitOps 管理下の Deployment と衝突しないようにする: 変更範囲が
  Service・Ingress・レーンの動作確認手順全体に広がり、#292 の受け入れ条件(regressionテスト含む)を満たすには
  過剰。レジストリの永続化と runbook の明記で受け入れ条件は満たせると判断した。
