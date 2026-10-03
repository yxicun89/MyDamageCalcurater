# ADR-0416: speed・judge の直結 Ingress を撤去し、gateway 経由に一本化する(issue #284 の speed・judge 分)

- 状態: 採用(2026-10-03。実装済み・共有クラスタ未適用)
- 関連: ADR-0414(balance で同じ方式。このADRは同じ決定を speed・judge に適用する)、ADR-0202(gateway の転送と §4 ヘッダ検証)、
  ADR-0408(GitOps)、DECISIONS 2026-09-25 ユーザー決定 #2(balance・speed・judge も gateway の後ろにまとめる)

## 決定
1. `services/speed/deploy/k8s/base/ingress.yaml`・`services/judge/deploy/k8s/base/ingress.yaml` を削除し、各 kustomization から外す。
2. gateway の base Deployment に `GATEWAY_SPEED_URL=http://speed`・`GATEWAY_JUDGE_URL=http://judge`(Service 名・80 番)を置く。
   NetworkPolicy `allow-gateway-upstream` は既に speed・judge を含む(ADR-0414 と同じ)。
3. smoke: speed の端末ID・セッションIDは gateway の正準形(版 4 の形)に直す。judge は既に正準形。healthz はヘッダ不要のまま(ADR-0202 §3 追記)。
4. 検査: speed・judge の `cmd/api/manifest_test.go`(Ingress が無い)、gateway の `TestManifestGatewaySpeedJudgeURL`(base の URL が Service を指す)。

## 結果・注意
- Argo CD は prune が無効(ADR-0408)なので、共有クラスタに残る旧 `Ingress/speed`・`Ingress/judge` は消えない。gateway を更新してから
  手動で削除する(`kubectl -n pokecalc delete ingress speed judge --ignore-not-found`。クラスタ操作なので人間確認)。
- speed・judge 未デプロイの間、`/api/speed/*`・`/api/judge/*` は gateway が上流に届かず失敗する(以前は 503 `upstream_unavailable`)。

## 残り(このタスクでは触らない)
- NetworkPolicy `allow-traefik-ingress`(共有 base)には speed・judge・balance が残る。Traefik からの到達はもう使わない。別 PR・人間確認で外す。
- web の `BALANCE_PROXY_TARGET` 等の開発サーバー用プロキシは本番経路ではないので変更しない。

## 追記: AppProject の許可種別
- gitops overlay が Ingress を描画しなくなったので、`deploy/argocd/appproject.yaml` の `namespaceResourceWhitelist` から `networking.k8s.io/Ingress` を外した
  (`scripts/gitops_test.sh` が「overlay の種別とちょうど一致」を検査するため。最小権限にも合う)。クラスタの旧 Ingress には影響しない(prune 無効)。
