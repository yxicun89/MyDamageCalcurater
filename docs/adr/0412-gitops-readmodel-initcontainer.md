# ADR-0412: gitops overlay の read model は initContainer(pokedex export)で起動時に作る

- 状態: 採用(2026-09-25。ユーザー決定: issue #237 の既定案 (a)。実装済み・実クラスタ未適用)
- 関連: issue #237、ADR-0002(実データを Git に置かない)、ADR-0018・ADR-0403・ADR-0605(追記あり)、ADR-0105 §5(`pokedex export`)、
  ADR-0110(pokedex_reader)、ADR-0132(NetworkPolicy)、ADR-0405(digest 固定)、ADR-0408(GitOps の運用)、#108(read model の版照合)

## 背景
Argo CD が同期する gitops overlay には read model が無く、balance は analyze 等が 503、speed は digest が placeholder で未配備だった。
手動の `*-k3d-deploy-readmodel`(ConfigMap を手で作る)を上書きで挟むと Argo CD の desired state と乖離し、OutOfSync になる。

## 決定
1. **方式(a)**: gitops overlay の Deployment に initContainer `readmodel-export` を足す。image は `pokecalc/pokedex`(server イメージ。
   ENTRYPOINT=/pokedex、シェル無し)で、`args: [export, -out, <mount>]`。出力先は `emptyDir`(volume `readmodel`)。本体は同じ volume を
   readOnly でマウントし、`BALANCE_*_PATH` / `SPEED_POKEMON_PATH` をそのマウント先のファイルに向ける。ConfigMap は使わない(ADR-0002・1 MiB 上限)。
   Pod は export が成功するまで Ready にならないので、sync 直後に業務 API が 503 を返さない。
2. **DSN**: initContainer だけに `POKEDEX_DATABASE_DSN` を Secret `mysql-auth` の `pokedex-reader-dsn`(SELECT のみ。ADR-0110)から渡す。
   本体(業務 API)には DSN を渡さず、DB に届かない形を保つ(ADR-0012 §6)。DSN を平文の value にしない。
3. **image の digest 固定**: gitops overlay の `images` に `pokecalc/pokedex` を digest 固定で足す(ADR-0405)。digest は
   `make pokedex-registry-push`(`scripts/pokedex-registry-push.sh`。balance-registry へ push。共有クラスタへは `POKEDEX_REGISTRY_PUSH_CONFIRM=1` が必要)の
   出力を使う。`scripts/gitops/check-gitops.sh` は本体と pokedex の両方の digest を見る(ready は placeholder を拒否)。
   awk の先頭一致ではなく image の name ごとに値を取ること。
4. **NetworkPolicy**: `deploy/k8s/base/networkpolicy/allow-mysql-ingress.yaml` は pokedex 系の Pod だけ mysql:3306 を許す(ADR-0132)。
   balance・speed の Pod からの export には、送信元の許可が要る。Pod 単位の許可なので本体も同じ Pod として通ってしまう点を限界として記録する
   (initContainer だけを分けることは NetworkPolicy ではできない)。この変更は共有の base(データ・運用レーンの持ち物)なので、
   別 PR で依頼し、人間の確認を得る(ADR-0132 の許可表とテストも合わせる)。
5. **手動 overlay と取り合わない**: Argo CD の Application が在るクラスタでは `k3d-deploy-readmodel.sh` を既定で拒否する
   (`ALLOW_MANUAL_OVERLAY=1` を明示したときだけ進む)。`check-gitops.sh ready` は生きている Deployment に local-readmodel の annotation
   (`pokecalc.example/readmodel-hash`)が残っていたら失敗し、`argocd app sync` で戻す案内を出す(クラスタに届かないときは検査を飛ばす)。
   runbook に「Argo CD 有効時は手動 apply しない」を明記する。
6. **speed の digest**: speed の本体 image はまだ push されていない(Application 未適用)。placeholder(全0)のままにし、
   **「未配備」と明示**する(docs/speed-design.md・docs/runbooks/speed.md・本 ADR)。template 検査は通り、ready 検査は意図どおり失敗する。
   実 image に置き換えるのは、人間の確認のもとで speed を配備するとき(runbook 節 8)。overlay の構造(initContainer・emptyDir)は balance と同時に入れる。
   **balance も、pokedex の digest が全0の placeholder のままなので未配備扱い**とする(ready 検査は意図どおり失敗する)。
   **前提: NetworkPolicy(`allow-mysql-ingress.yaml`)の承認・適用と pokedex の実 digest の確定が済むまで、balance / speed とも sync しない。**

## 段階
- 今回(spec): 受け入れ条件・テスト(`scripts/gitops_test.sh`)・本 ADR。
- 実装(balance と speed の overlay 構造・check-gitops・k3d-deploy-readmodel のガード・runbook): 実装者。
- 人間 / データレーン: 共有クラスタへの pokedex image の push(digest の確定)、NetworkPolicy の変更の承認、クラスタへの適用と sync。

## 影響と制約
- initContainer が MySQL に届かない・マスタが空・既定レギュレーションが無いと Pod は Ready にならず再試行する(503 ではなく Pod の起動待ち)。
- read model の版は Pod の起動時点で決まる(更新は Pod の再作成)。全 consumer への版の収束は #108。
- balance・speed の Pod が `mysql-auth` Secret を参照する(pokedex_reader の SELECT のみ)。権限を広げない。

## 却下した案
- (b) ConfigMap を Argo CD 管理外の別リソースにする: 手動 apply が残り、1 MiB 上限もある。
- (c) 現状追認(GitOps は image 配布だけ): 業務 API が機能しない状態を残す。
