## 2026-10-02: issue #284 のタイプバランス分(balance の直結 Ingress を撤去し gateway 経由に。ADR-0414)
Decision: `services/balance/deploy/k8s/base/ingress.yaml` を削除し、gateway の base Deployment に `GATEWAY_BALANCE_URL=http://balance` を配線した。balance の smoke は固定の架空 UUID を付ける(gateway は UUID だけを通す)。
Reason: 2026-09-25 ユーザー決定 #2(balance・speed・judge も gateway の後ろにまとめる)。API レーンの「gateway の配線が済んでから直結 Ingress を撤去」の申し送りに沿った。
Impact: **speed・judge レーンへ**: 同じ形(Ingress 削除・`GATEWAY_SPEED_URL`/`GATEWAY_JUDGE_URL` を gateway base に追加・smoke の UUID 化)が残っている。共有クラスタは未適用で、旧 `Ingress/balance` は Argo CD が prune しないため手動削除が要る(人間確認)。
