## DOC: 文書(全レーン。docs/coding-rules.md §8。2026-09-22 ユーザー要望)
各レーンが自分の範囲の README(何をするか・mermaid の構成図・ディレクトリ・コマンド・関連 ADR。80 行以内)と、動かして確かめられるレーンは手順書(`docs/runbooks/<レーン>.md`。AGENTS.md「手順書の書き方」に従う)を書く。全体図は `docs/architecture.md`。
- [x] DOC-data
- [x] DOC-api: `services/calc/README.md`・`services/gateway/README.md` を §8 の形に、手順書
- [x] DOC-web: `web/README.md`、手順書
- [x] issue #284 のタイプバランス分(ADR-0414): balance の直結 Ingress を撤去し gateway の `GATEWAY_BALANCE_URL=http://balance` を base に配線。**残り**: speed・judge の直結 Ingress の撤去と URL 配線(各レーン)、共有クラスタの旧 `Ingress/balance` の手動削除と `allow-traefik-ingress` の balance 除外(人間確認)
- [x] issue #284 の speed・judge 分(ADR-0416): speed・judge の直結 Ingress を撤去し gateway の `GATEWAY_SPEED_URL=http://speed`・`GATEWAY_JUDGE_URL=http://judge` を base に配線(ADR-0414 と同じ方式。静的検査は各 `cmd/api/manifest_test.go` と gateway の `TestManifestGatewaySpeedJudgeURL`)。**残り(人間確認)**: 共有クラスタの旧 `Ingress/speed`・`Ingress/judge` の手動削除(gateway 更新後)と `allow-traefik-ingress` の speed・judge・balance 除外。これで issue #284 の Ingress 撤去は3サービスとも完了
- [x] DOC-tb: `services/balance/README.md` を §8 の形に、手順書 `docs/runbooks/balance.md`
- [x] DOC-speed
- [x] DOC-ios: `ios/README.md`
- [x] DOC-arch
