# ADR-0709: 判定 GitOps(gitops overlay・Argo CD Application・image 公開)

- 状態: 採用(2026-10-01。判定レーンの判断。issue #258)
- 日付: 2026-10-01
- 関連: ADR-0700(JD0。設計書は「Kustomize・Argo CD」としたが GitOps は先送りのまま記録が無かった)、ADR-0605(speed の GitOps。写経元)、
  ADR-0018(balance TB0)、ADR-0405(Argo CD の bootstrap)

## 背景
judge は `deploy/k8s/overlays/local` だけで、balance・speed と違い Argo CD から配備できなかった。

## 決定
ADR-0605 の一式を judge 向けに移植する(`speed` → `judge` の置換が基本)。

1. **レジストリは balance のものを共有する**(新設しない。ADR-0605 §1)。Mac 側 port-forward は `JUDGE_REGISTRY_PORT`(既定 **5003**。balance 5001・speed 5002 と衝突させない)。
2. `services/judge/deploy/k8s/overlays/gitops`(`digest:` 固定。placeholder は `sha256:` + 64桁の `0`)、
   `services/judge/deploy/argocd/application.yaml`(`pokecalc-judge`。repoURL は placeholder。automated sync は無効)。
3. `services/judge/scripts/{check-gitops,argocd-local-app,local-registry-push,publish-image}.sh`。image のビルド入力は `engine` と `services/judge`
   (Dockerfile の COPY と一致)なので、dirty 判定も両方を見る。
4. Makefile: `judge-gitops-template-check`・`judge-gitops-check`・`judge-registry-push`・`judge-argocd-app`・`judge-docker-push`。`judge-kustomize` に gitops overlay と Application の描画確認を足す。

## 影響と制約
- judge は read model を持たない(judge-design.md §2)ので、speed の ADR-0605 §2a にある「GitOps では実データが載らない」問題は無い。
  上流は pokedex-svc・calc-svc の Service 名で、どの overlay でも同じ。
- 実クラスタへの適用(`judge-argocd-app`・registry push・sync)は共有の Argo CD を変える操作なので、このPRでは行わない。人間の確認のもとで行う
  (検証は `judge-gitops-template-check` まで。ADR-0605 §4 と同じ)。
