## 2026-10-03: issue #284 の speed・judge 分(直結 Ingress を撤去し gateway 経由に。ADR-0416)
Decision: speed・judge の `base/ingress.yaml` を削除し、gateway の base Deployment に `GATEWAY_SPEED_URL=http://speed`・`GATEWAY_JUDGE_URL=http://judge` を配線した(ADR-0414 と同じ方式)。speed の smoke の端末ID・セッションIDを正準形の UUID に直した。
Reason: 2026-09-25 ユーザー決定 #2(balance・speed・judge も gateway の後ろにまとめる)。balance 分(ADR-0414)の残り。
Impact: 共有クラスタは未適用。旧 `Ingress/speed`・`Ingress/judge` は Argo CD が prune しないため、gateway 更新後に `kubectl -n pokecalc delete ingress speed judge` の手動削除が要る(人間確認)。`allow-traefik-ingress` の speed・judge・balance 除外は共有 base のため別 PR。
