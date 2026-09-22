# ADR-0605: 素早さ比較 SP5 GitOps

- 状態: 採用(2026-09-23。素早さレーンの判断)
- 日付: 2026-09-23
- 関連: ADR-0600 §2(SP4 ではなく digest が決まる段階でと変更)、ADR-0603(SP4 で GitOps を分離)、
  ADR-0018(balance TB0。GitOps・クラスタ内レジストリの元になった設計)、ADR-0403(balance の read model 配線。同じ運用の overlay 使い分け)

## 背景
SP0〜SP4 で speed-svc はローカル k3d の `deploy/k8s/overlays/local`(架空データ)と `local-readmodel`(pokedex export の実データ)で
動く。SP5 では、balance(タイプバランスレーン。TB0・ADR-0018)がすでに構築した GitOps の型(Argo CD・クラスタ内イメージレジストリ)を
speed にも適用し、`main` への push だけでクラスタの speed の中身が変わる状態にする。

## 決定

### 1. クラスタ内イメージレジストリは balance のものを共有する(新設しない)
- k3d ノードの containerd は `localhost:5000`(hostPort)からしか平文 pull を許可しない設定になっている(ADR-0018)。
  hostPort はノードにつき1つの Pod しか持てないため、**speed 専用のレジストリ Deployment は作らない**。
  すでに `balance-registry` namespace(`services/balance/deploy/local-registry/`。balance レーンの持ち物)で動いているレジストリを、
  push 先の namespace としてそのまま使う(push するイメージ名は `pokecalc/speed:<tag>` で `pokecalc/balance:<tag>` とは衝突しない)。
- speed のスクリプト(`scripts/local-registry-push.sh`)は `kubectl -n balance-registry port-forward svc/registry` を呼ぶだけで、
  `services/balance/` のファイルは一切変更しない(COORDINATION.md のレーン境界を守る)。
- 提案(DECISIONS.md に記録・タイプバランスレーンへ): 名前が `balance-registry` のままだと分かりにくいので、いずれかのレーンが手すきのときに
  `pokecalc-registry`(共有のクラスタ内レジストリ)へ改名するとよい。急ぎではないので、balance レーンの都合の良いときに任せる。
- クラウド(本番相当)のレジストリは別途決める(このレジストリはローカル k3d の検証専用。ADR-0018 §結論のまま)。

### 2. GitOps の overlay と Argo CD Application(balance と同じ形)
- `services/speed/deploy/k8s/overlays/gitops/kustomization.yaml`: `newTag` ではなく `digest`(kustomize の images フィールド名。digest 固定。
  balance の overlay と同じキー名。実装時の確認: `newDigest` という語はここでは使わず `digest:` を使う)。
  digest のプレースホルダは `sha256:` + 64桁の `0`(balance と同じ)。
- `services/speed/deploy/argocd/application.yaml`: `metadata.name: pokecalc-speed`、`spec.source.path:
  services/speed/deploy/k8s/overlays/gitops`、`repoURL` はプレースホルダ `https://git.example.invalid/pokecalc.git`
  (アカウント名を含む実 URL は Git に書かない。ADR-0018 と同じ理由)。`spec.source.repoURL` 以外は balance と同じ形。
  **automated sync は無効のまま**(TB0 の判断を踏襲。手動 sync で確認する)。
- `services/speed/scripts/check-gitops.sh`(`template`/`ready` の2モード)・`argocd-local-app.sh`・`local-registry-push.sh`・
  `publish-image.sh` は balance の同名スクリプトの構造をそのまま speed 向けに移植する(`docs/adr/0018-*.md`・
  `services/balance/scripts/` を写経元とする)。相違点は次の2つだけ:
  - `local-registry-push.sh`・`publish-image.sh` の `docker build` は **speed の Dockerfile がリポジトリのルートをビルドコンテキストに
    要求する**ため(ADR-0600 §2。engine を含む)、`docker build -f services/speed/Dockerfile .`(`-t "$balance_dir"` 相当の
    ディレクトリ指定ではなくルート)にする。
  - `kubectl -n balance-registry port-forward`(balance-registry の namespace)を使う(§1)。Mac 側の port-forward は
    `SPEED_REGISTRY_PORT`(既定 5002。balance の 5001 と衝突しないよう1つ空ける)。
- Makefile: `speed-gitops-template-check`・`speed-gitops-check`・`speed-registry-push`・`speed-argocd-app`・`speed-docker-push`
  を追加する(balance の `balance-registry-apply` に相当するものは無い。専用レジストリを持たないため)。
  `speed-kustomize` に `overlays/gitops` の描画確認を追加する(ADR-0600 §2 の「GitOps は SP4 で作る」を変更した ADR-0603 の続き)。

### 3. 適用の手順(手順書 docs/runbooks/speed.md に反映)
balance の手順書(`docs/runbooks/balance.md`)と同じ順序: テスト・lint → `speed-k3d-deploy`(既存)で疎通 → (GitOps を確かめるときだけ)
Argo CD は導入済み(TB0 で導入。全レーン共通のクラスタ内 Argo CD をそのまま使う。**speed が Argo CD 自体をインストールし直すことはしない**)→
リポジトリの認証(Argo CD の repository Secret。すでに balance で登録済みならそのまま使える。無ければ人間が登録)→
`speed-argocd-app` → `speed-registry-push` で digest を得て `overlays/gitops/kustomization.yaml` に書く → PR で main に入れる →
Argo CD の refresh・sync → Pod の image digest が一致することを確認。

### 4. 未決(このセッションでは実施しない)
- Argo CD への実際の Application 適用(`speed-argocd-app`)・レジストリへの push・sync の実行は、**クラスタの状態を変える操作**であり、
  balance の Application(`pokecalc-balance`)と同じ Argo CD インスタンスを共有する。誤って balance の Application に影響しないよう、
  実際の適用はこの ADR とスクリプト・overlay を PR で main に入れたあと、人間の確認のもとで行う
  (CLAUDE.md「人間の確認が必要なこと」には明記されていないが、共有クラスタへの outward-facing な変更として確認を挟む)。
  このセッションでは `speed-gitops-template-check`(kustomize のレンダリングと overlay の形式検査。クラスタを変更しない)までを
  検証範囲とする。

## 却下した案
- speed 専用のクラスタ内レジストリを新設する: hostPort の競合で balance のものと同時に動かせない。
- Argo CD を speed 用に別途インストールする: TB0 で導入済みの1つの Argo CD をクラスタ全体で共有する(複数レーンで複数の Argo CD を
  動かす理由が無い)。
