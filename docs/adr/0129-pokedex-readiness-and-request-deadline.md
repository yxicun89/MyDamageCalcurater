# ADR-0129: pokedex-svc の readiness を DB に連動させ、DB の呼び出しに締め切りを持たせる(issue #107・#323・#324・#299)

- 状態: 採用(spec-writer 起草。実装前)
- 日付: 2026-10-01
- レーン: データ(ADR 帯 `0100〜`)
- 関連: issue #107、#323、#324(pokedex 分)、#299(pokedex 分)、#437(MySQL の OOMKill 中に calc がマスタを取れなかった)、
  issue #403(パッケージ D11)、ADR-0105 §1(運用エンドポイント)、ADR-0111(HTTP タイムアウト・graceful shutdown)、
  ADR-0112(接続プール)、ADR-0127(読み取りスナップショット)

## 背景

- `GET /healthz` は DB に触れず常に 200 で、Deployment の readiness と liveness の両方がそれを見ていた(ADR-0105 §1)。
  DB 停止・migration 未実施・初回 import 前でも Pod は Ready になり、Service は必ず 503 を返す Pod に要求を送る。
  #437 では取り込み中に MySQL が OOMKill され、pokedex が数十秒 `master_unavailable` を返す間に起動した calc-svc が
  マスタを取れなかった。
- ハンドラにも DB の呼び出しにも締め切りが無い。`http.Server.WriteTimeout`(15秒)は書き込みを失敗させるだけで
  ハンドラを止めない(Go の仕様)。MySQL が固まる(`docker pause`)と、直接の呼び出しは70秒以上止まる(#323)。
- `lifecycle.preStop` が無く、SIGTERM で `Shutdown` がすぐにリスナーを閉じる。Endpoints からの削除が伝わる前に
  届いた接続は拒否される(#324)。

## 決定

### 1. `GET /readyz` を足す。readiness は DB の最小条件に連動させ、liveness(`/healthz`)は DB に触れないまま

- `/readyz` は `ListDataVersions`・`ListTypes`・`ListNatures`・`GetDefaultRegulation` の4つ(既存の sqlc クエリ)を読み、
  どれかが失敗(DB に届かない・テーブルが無い = migration 未実施)・空(未投入)・既定のレギュレーションが無い
  (`sql.ErrNoRows`)なら 503 `master_unavailable`(Error 形式。固定文。DB のエラー文・DSN を出さない。理由はログにだけ)。
  揃っていれば 200 `{"status":"ok"}`(calc-svc の `/readyz` と同じ形)。
- 種族・技・持ち物などマスタ全体は読まない(probe の周期で重い処理をしない)。新しい sqlc クエリは足さない。
  4つは autocommit で順に読む(各確認は独立した存在確認で、importer の全置換は1トランザクションなので、
  どのテーブルも「旧の全体」か「新の全体」しか見えない。ADR-0127 のスナップショットは要らない)。
  Tx で読む実装にしてもテストは通るが、開いたままの Tx を残さないこと。
- 結果をキャッシュしない。import が終われば再起動なしで次の probe から 200、DB が戻れば 200 に戻る。
- `/readyz` と `/healthz` は運用エンドポイントで、`api/openapi.yaml` の契約に載せない(既存の `/healthz`、
  calc-svc の `/readyz` と同じ扱い。契約の変更は無い)。
- ADR-0105 §1 の「readiness も `/healthz`。データの有無は各操作の 503 で表す」を、この決定で置き換える。
  #323 の既定案は「readiness を DB に連動させるのは見送る」だったが、#107 の受け入れ条件と #437 の事故を優先する。
  replicas 1 で DB が落ちると Service の Endpoints が空になり、gateway は接続できずに 503 を返す
  (DB が落ちていれば pokedex 自身も 503 なので、利用者から見た結果は同じ。違いは、Ready が「使える」を表すこと)。

### 2. DB を使う操作に締め切りを掛ける(`DefaultRequestTimeout` = 5秒)

- `httpapi` が、DB を使う全ての操作(公開の検索7 + 内部 API)の context に `context.WithTimeout` を掛ける
  (ミドルウェアでも各ハンドラでもよい)。締め切りで DB の呼び出しが `context.DeadlineExceeded` を返したら、
  他の DB の失敗と同じく 503 `master_unavailable`。go-sql-driver/mysql は context の終了で接続を切って戻るので、
  DSN の `readTimeout` は足さない(接続プール(ADR-0112)の接続が締め切りで捨てられるだけ)。
- 値は 5秒(#323 の既定案)。大小関係: `DefaultRequestTimeout`(5s)< `writeTimeout`(15s。ADR-0111)、
  かつ外側の gateway の `GATEWAY_UPSTREAM_TIMEOUT`(10s)・calc の `defaultMasterFetchTimeout`(10s)より短い
  (内側 < 外側。#299)。内部 API の全件 export は実測でミリ秒オーダー(ADR-0111)なので、5秒は十分に余る。
- `/healthz` と `/metrics` は DB に触れないので締め切りの対象外。

### 3. readiness の締め切り(`DefaultReadinessTimeout` = 2秒)と probe の値

- `/readyz` は要求の締め切りより短い 2秒の締め切りで DB を読む(`DefaultReadinessTimeout ≤ DefaultRequestTimeout`)。
- Deployment の readinessProbe: `httpGet /readyz`・`periodSeconds: 5`・`timeoutSeconds: 3`(2秒の締め切りより長く、
  kubelet が待ち切る前にハンドラが 503 を返す)・`failureThreshold: 2`(DB 障害から約10秒で Service から外れる。
  #437 の数十秒の障害を拾える)。livenessProbe は `/healthz` のまま(DB 障害で再起動ループにしない)。
- `httpapi` の定数は `DefaultRequestTimeout`・`DefaultReadinessTimeout` として export し、テストは
  `NewHandler(q, WithRequestTimeout(d), WithReadinessTimeout(d))` で短い値を渡す。本番(`cmd/pokedex`)は
  オプションを渡さない(既定値が必ず掛かる)。環境変数にはしない(変える理由がまだ無い。使われない設定項目を作らない)。

### 4. 停止は preStop の sleep(5秒)→ SIGTERM → Shutdown(10秒)

- `lifecycle.preStop.sleep.seconds: 5`(k8s の sleep アクション。1.29 alpha・1.30 beta 既定有効・1.34 GA。
  k3d の k3s は v1.35.5 で確認。イメージは distroless で `sleep` コマンドが無いので exec にしない)。
  その間に Pod の削除が Endpoints に伝わり、新しい接続が来なくなってから SIGTERM が届く。
- `shutdownTimeout`(10秒)≥ `DefaultRequestTimeout`(5秒): 処理中の要求は締め切りまでに終わり、Shutdown が待ち切れる。
- `terminationGracePeriodSeconds`(30秒)> preStop(5秒)+ `shutdownTimeout`(10秒)。値は今のまま。

### 5. テスト

- `internal/httpapi/readyz_test.go`・`deadline_test.go`: storetest の偽 Querier と、ctx の終了まで固まる偽物
  (実 MySQL ドライバと同じく ctx.Err() を返す)で、締め切り・readiness の各条件・再起動なしの回復を確かめる。
- `cmd/pokedex/deadline_test.go`: 締め切りの大小関係を定数どうしの不等式で固定する。
- `cmd/pokedex/manifest_lifecycle_test.go`・`manifest_test.go`: probe の分離・timeoutSeconds・preStop・grace を静的に検査する。
- `importer/readyz_mysql_test.go`(`-tags mysql`): 実 MySQL で 未投入 503 → import 後 200 → migrate 前 503。

## 影響

- 初回 import 前のクラスタでは pokedex の Deployment が Available にならない。`rollout status deployment/pokedex` を
  待つスクリプト(`scripts/k3d-deploy-latest.sh`)は、import 済みのクラスタを前提にする(新規クラスタでは `make import-k8s` の後)。
  replicas 1 のローリング更新では、新しい Pod が Ready になるまで古い Pod が残る(maxUnavailable 0)。
- calc-svc の readiness(マスタ取得済み)と合わせ、DB 障害時は pokedex → calc の順に Service から外れ、DB が戻れば自動で戻る。
- 同時実行の上限と過負荷時の 503 + `Retry-After`(#299 の残り)は、pokedex では扱わない(検索は DB の接続プールの上限
  (ADR-0112)で待ち、締め切りで 503 になる。計算の重い calc・judge 側の課題)。
