# ADR-0413: balance の直結 Ingress を撤去し、gateway 経由に一本化する(issue #284 のタイプバランス分)

- 状態: 採用(2026-10-02。実装済み・共有クラスタ未適用)
- 関連: issue #284、ADR-0012(当初は独自 Ingress)、ADR-0202(gateway の転送と §4 ヘッダ検証。PR #416 の追記)、ADR-0408(GitOps)、
  DECISIONS 2026-09-25 ユーザー決定 #2(balance・speed・judge も gateway の後ろにまとめる)

## 決定
1. `services/balance/deploy/k8s/base/ingress.yaml` を削除し、kustomization から外す。balance は Ingress を持たない。
2. gateway の base Deployment に `GATEWAY_BALANCE_URL=http://balance`(Service 名・80 番)を置く。Service 名はクラウドでも同じなので base。
   NetworkPolicy `allow-gateway-upstream` は既に balance(:8080)を含むため変更しない。
3. balance の smoke(`make balance-smoke`)は gateway 経由になり、gateway は正準形の UUID だけを通すので、固定の架空 UUID を付ける。
   gateway の `api-smoke` の balance 確認は「balance の Ingress があるとき」から「balance の Service があるとき」に変える。
4. 検査: balance の `cmd/api/manifest_test.go`(Ingress が無い)、gateway の `TestManifestGatewayBalanceURL`(base に URL がある)。

## 結果・注意
- Argo CD は prune が無効(ADR-0408)なので、共有クラスタに残る旧 `Ingress/balance` は sync しても消えない。手動で削除する
  (`kubectl -n pokecalc delete ingress balance`。クラスタ操作なので人間確認)。gateway を先に更新してから消す(経路が途切れない)。
- balance 未デプロイの間、`/api/balance/*` は gateway が上流に届かず失敗する(base に URL を置いたため。以前は 503 `upstream_unavailable`)。
- NetworkPolicy `allow-traefik-ingress` には balance が残る(Traefik からの到達はもう使わない)。共有 base のため別 PR・人間確認で外す。

## 残り(各レーン。このタスクでは触らない)
- speed(`services/speed/deploy/k8s/base/ingress.yaml`)・judge(`services/judge/deploy/k8s/base/ingress.yaml`)の直結 Ingress の撤去と
  `GATEWAY_SPEED_URL`・`GATEWAY_JUDGE_URL` の配線。これらが残る間、`/api/speed`・`/api/judge` は gateway の検証を通らない。
- web の `BALANCE_PROXY_TARGET`(`web/vite.config.ts`)は開発サーバー用で、本番経路ではないので変更しない。
