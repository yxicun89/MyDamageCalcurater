# ADR-0804: judge・speed・balance の停止を preStop と待ち切れる Shutdown に揃える(issue #324)

- 状態: 採用
- 日付: 2026-10-03
- 関連: ADR-0129 §4(pokedex の方式)・ADR-0801(締め切りの連鎖)・PR #467(calc・gateway・pokedex・web)

## 背景
issue #324 の残り。judge・speed・balance の Deployment には preStop も terminationGracePeriodSeconds も無く、
SIGTERM で即座にリスナーが閉じて Endpoints から外れる前の新規接続を拒否していた。
また `Shutdown` が 10 秒で終わらないと(writeTimeout は 15 秒)、judge・speed・balance の main は `os.Exit(1)` で異常終了していた。

## 決定
1. 3 つの Deployment に `lifecycle.preStop.sleep.seconds: 5` と `terminationGracePeriodSeconds: 30` を置く(ADR-0129 §4 と同じ)。
2. 各 `cmd/api/shutdown.go` に `shutdownTimeout = 15s`(= writeTimeout)を置く。処理中のリクエストは writeTimeout までに必ず終わるので、
   停止時に途中で切らない。30 > preStop 5 + 15 を deploytest(`services/gateway/deploytest/prestop_test.go`)が固定する。
3. `gracefulShutdown` は Shutdown が時間内に終わらなければ残りの接続を `Close` し、error を返す。main はそれを Warn にして
   正常終了する(停止要求への応答としての終了は異常ではない)。
4. 各サービスは独立した Go module のため、小さな `shutdown.go` を 3 つに複製した(共有パッケージを作るほどの量ではない)。

## 対象外
calc の shutdownTimeout(5 秒)< writeTimeout(10 秒)は PR #467 の決定のまま。変更するときは別 issue にする。
