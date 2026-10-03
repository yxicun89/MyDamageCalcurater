## 2026-09-26: issue #284 の API レーン担当分(balance・speed・judge を gateway の後ろにまとめる)を実装(API レーン → タイプバランス・素早さ・判定レーンへ)
Decision: ユーザー決定(2026-09-25「ユーザー決定 4 件」#2)を実装。`services/gateway/internal/httpapi/routing.go`
に `routeBalance`/`routeSpeed`/`routeJudge` と `prefixBalance`/`prefixSpeed`/`prefixJudge`(`/api/balance/`・
`/api/speed/`・`/api/judge/`。record・team と同じ前方一致・末尾スラッシュ必須・不一致は404)を追加し、
`requiresHeaderCheck` にも3つとも加えた(`/api/{balance,speed,judge}/*` にも端末ID・セッションIDの検証
〈ADR-0202 §4〉がgatewayでもかかるようになった)。`server.go` に `Config.BalanceURL`/`SpeedURL`/`JudgeURL`
(nilなら503 `upstream_unavailable`)と対応する `ReverseProxy` を追加、`main.go` に `GATEWAY_BALANCE_URL`/
`GATEWAY_SPEED_URL`/`GATEWAY_JUDGE_URL` を追加。CORSの許可メソッドは変更なし(3サービスの契約は
GET/POSTのみとソースで確認済み)。`deploy/k8s` にbalance/speed/judge自体のDeployment/Serviceがまだ無い
ため、gatewayのdeployment.yamlへの実URL配線はrecord・team(P5-3b/P5-4b)と同じく別タスクとして残す
(コードのみ今回のスコープ)。
Reason: issue #236で判明していた「balance/speed/judgeがTraefik直結でgatewayを経由しないため、端末ID・
セッションIDの検証がgatewayと各サービスで別々に実装され食い違いうる」問題を、CLAUDE.mdの「gatewayが
唯一の入口」原則どおりgatewayに統合することで解消する。
Impact: **タイプバランス・素早さ・判定レーンへ**: gatewayに `/api/balance/*`・`/api/speed/*`・
`/api/judge/*` の転送が実装された(現時点ではcritic未レビュー・deployment.yamlの実URL未配線のため
まだ有効化されていない)。各レーンが持つ直結Traefik Ingressの撤去は、gateway側のdeployment.yaml配線が
完了し実クラスタで動作確認できてから行うこと(先に撤去すると経路が無くなる)。各サービス自身が持つ
端末ID・セッションIDの検証(issue #236)は二重になるが害はなく、削除するかどうかは各レーンの判断のまま
残す。critic レビュー後、deployment.yaml配線と実クラスタ確認を別途行い、完了したら改めて連絡する。
