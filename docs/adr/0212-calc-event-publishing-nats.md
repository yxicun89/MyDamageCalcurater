# ADR-0212: calc-svc の NATS JetStream イベント発行(P5-2)

- 状態: 採用(2026-09-25。critic 3回目 PASS。1回目 FAIL〈全面改訂〉・2回目 FAIL〈残2件+軽微4件〉を経て確定)
- 日付: 2026-09-25
- 関連: ADR-0209(保持・削除・端末 ID 境界。§3 #6・§4・§7 がこの ADR の直接の前提)、
  ADR-0211(TiDB 導入。record-svc/team-svc のプロビジョニングと対称のインフラ導入判断)、
  ADR-0200(calc-svc の API 契約)、ADR-0202(gateway のヘッダ検証。§4 が `isCanonicalUUID` の根拠)、
  ADR-0204(calc-svc の起動とマスタ取得の背景再試行。§3 が §5 の再試行の型の出典)、
  CLAUDE.md 絶対ルール5(計算はイベント保存に依存しない)、
  docs/requirements.md §6(`calc_events` の列)、docs/plan.md P5-2

## 背景

M2 P5-2 は、calc-svc が計算のたびに NATS JetStream へイベントを発行し、後続の record-svc(P5-3)・
team-svc(P5-4)がそれぞれ別々の durable consumer で購読する土台を作る(ADR-0209 §3 #6・§4)。
この ADR はイベントの配信基盤(NATS のバージョン・ローカル/k3d 導入・ストリーム設定)と、
イベント自体のワイヤフォーマット(calc-svc が発行し、record-svc/team-svc が読む契約)を決める。
record-svc/team-svc 自体の実装(消費・保存・集計)は P5-3/P5-4。

## 決定

### 1. バージョン固定(2026-09-25 時点の最新安定版)

| コンポーネント | バージョン | 用途 |
|---|---|---|
| NATS Server | v2.15.0(イメージ `nats:2.15.0`。digest 固定) | JetStream 本体 |
| nats.go | v1.54.0 | calc-svc(発行)・record-svc/team-svc(購読。P5-3/P5-4)の Go クライアント |

いずれも GitHub Releases で prerelease でないことを確認済み。

### 2. ローカル開発: docker run(tiup playground ではなく単純なコンテナ)

NATS は TiDB と違い単一バイナリで JetStream を内蔵しており、Operator や複数コンポーネントの
オーケストレーションを必要としない。`scripts/db-local-up.sh`(MySQL)と同じ「単純なコンテナを
docker run する」流儀を踏襲する(`scripts/nats-local-up.sh`。ADR-0211 のように tiup 相当の
専用ツールは不要):

```
docker run -d --name pokecalc-nats-local -p 127.0.0.1:4222:4222 \
  nats:2.15.0@sha256:cd3fcd4ecdda44e3a66728a5334af0a959bc3979b32810e033d1c547241cd0f4 -js -sd /data
```

`-js` で JetStream を有効化、`-sd /data` でファイルストレージ(コンテナ内。永続化はしない。
ローカル開発用の使い捨てコンテナで、消えても実害は次回の計算からイベントが再び貯まるだけ)。

### 3. k3d: 単純な StatefulSet(TiDB のような Operator は使わない)

NATS は Kubernetes 上でも Operator 無しで安定して動く(公式の Helm chart も内部的には
単純な StatefulSet)。個人開発規模・単一レプリカでは Operator 導入のコストに見合わないため、
`deploy/k8s/overlays/local/mysql/` と同じ「単純な StatefulSet + Service + PVC」の形にする
(`deploy/k8s/overlays/local/nats/`)。

| リソース | request | limit |
|---|---|---|
| CPU | 50m | 200m |
| メモリ | 64Mi | 256Mi |
| storage(JetStream ファイルストレージ用 PVC) | ― | 1Gi |

`-js -sd /data`(PVC をマウント)で起動する。個人開発規模のメッセージ量なので、この程度の
リソースで十分(ADR-0211 の TiDB のような大きなリソース確保は不要)。

### 4. ストリーム設計

| 項目 | 値 | 理由 |
|---|---|---|
| ストリーム名 | `CALC_EVENTS` | |
| Subjects(ストリーム設定) | `["calc.events.*"]` | 1メッセージの subject は `calc.events.<device_id>`(ADR-0209 §3 #6「subject に device_id を含む」)。`device_id` は UUID で正準形(8-4-4-4-12)は gateway が検証する(ADR-0202 §4・`services/gateway/internal/httpapi/headers.go` の `isCanonicalUUID`)ため `.` を含まず、ワイルドカードは `*` 1トークンで足りる。**calc-svc 自身は書式を検証しない**(`checkHeaders` は欠落・空だけを見る)ため、gateway を経由しない内部呼び出しから `.` を含む値が来た場合、subject がストリームの `Subjects` に一致せず発行が失敗しうる(NATS の no-stream-response。応答は書き終えた後の goroutine 内の失敗なので警告ログに残るだけで、計算 API 自体は成功する。絶対ルール5) |
| Storage | File | Memory だと NATS 再起動でイベントが消える(record-svc/team-svc がまだ読んでいない分も含めて) |
| Replicas | 1 | 個人開発規模の単一 NATS(§3)。クラスタリングしない |
| MaxAge | 7日(`168h`) | ADR-0209 §3 #6・§7 |
| MaxBytes | 512Mi | PVC(1Gi。§3)の半分を上限にし、想定外の増加でディスクを使い切って発行そのものが失敗する事態を避ける(下記「容量の見積もり」) |
| Discard | Old(上限到達時は古いメッセージから捨てる) | 新しいイベントの発行を失敗させない(絶対ルール5。上限に達しても最新の計算のイベント発行自体は成功する) |
| Retention | **Limits**(年齢ベースの削除のみ。Interest ではない) | 下記参照 |

**容量の見積もり**: `CalcDetail` 入りのイベント(JSON)は攻撃側/防御側の個体2件・技・状況・ダメージ幅で
概ね 1〜2KB、envelope だけ(`calcBulk`/`calcReverse`)は数百バイト。個人利用で1日に数百リクエストを
想定しても(例: 500件/日 × 2KB × 7日 ≒ 7MB)、512Mi の上限に対して十分な余裕がある。

**Retention に Interest ではなく Limits を選ぶ理由(ADR-0209 §3 #6 の文言からの意図的な逸脱。
NATS Server v2.15.0 の実ソース `server/stream.go` で挙動を確認済み)**:
ADR-0209 §3 #6 は「`max_age` 7日、**両方の consumer が ack したら破棄**」と書いており、文字どおりには
JetStream の **Interest** retention policy を指しているように読める。しかし実際の挙動はより厳しい:
`stream.go` の `noInterest := numConsumers == 0 || !mset.csl.HasInterest(subject)` が真のとき、
そのメッセージは「削除される」のではなく**そもそもストアに書き込まれない**(`SkipMsgNoInterest`)。
発行元(calc-svc)には通常どおり成功の PubAck が返るため、**発行が失敗した形跡すら残らない**。
この条件は「consumer が0個」だけでなく「consumer は存在するが `FilterSubject` がそのメッセージの
subject に一致しない」場合にも成立する。P5-2(このタスク)の時点では record-svc(P5-3)・team-svc
(P5-4)のどちらの durable consumer もまだ存在しないため、Interest retention でストリームを作ると
**calc-svc が発行するイベントは P5-3/P5-4 が実装されるまでの間ずっと保存されずに消え続ける**
(エラーにならず静かにデータが失われる。最も気づきにくい種類の欠陥)。
Limits retention(`max_age`・`MaxBytes` だけで削除。consumer の有無に関係なく残る)であれば、
consumer が後から追加されても、その時点で7日以内に発行された未読分をそのまま拾える。
「両方 ack したら早期に消す」という省スペースの最適化は行わないが、上記の容量見積もりのとおり
コストは無視できる。この判断により、**P5-3(record-svc の consumer 追加)より前に P5-2 をデプロイしても、
イベントを取りこぼさない**。稼働中のストリームの `Retention` は Storage と違い後から変更できる
(WorkQueue との相互変換だけが拒否される)が、P5-3/P5-4 で consumer が揃った後も Interest へは
**恒久的に移行しない**と決める: `FilterSubject` の設定を一つでも誤ると同じ「静かな消失」が
再発するリスクを、ディスク使用量の早期削減という小さなメリットのために背負う理由が無いため。

**却下した案(追加)**: **P5-2 の時点で record-svc/team-svc の durable consumer を先に作り、
Interest retention を使う**。技術的には consumer はサーバ側オブジェクトで、購読する
record-svc/team-svc の実装より先に作ることもできる。しかし consumer の `FilterSubject` は
本来 P5-3/P5-4(実際に購読する側)が決めるべき設定であり、P5-2 が名前・filter を先取りして
「片方でも取り違えると発行イベントが静かに保存されなくなる」構造を作るのは、責務の分離として
悪い。Limits retention なら consumer の存在・設定に発行の成否が依存しないため、この問題自体が起きない。

**実装時の追記(2026-09-25。P5-3 の critic レビュー R-9)**: record-svc の重複排除キー
(`services/record/internal/events.EventID`。`calc_events.event_id` の一意制約に使う。ADR-0212 §6・
ADR-0209 AC-R7)は `CALC_EVENTS` ストリームのシーケンス番号だけから組み立てる
(`"calc-events-" + streamSeq`)。これは「同じメッセージの再配送では常に同じ値になる」という
at-least-once の重複排除には十分だが、**ストリームを作り直す(delete → 再作成)とシーケンスが1から
再開する**ため、過去に処理済みの `event_id`(例: `calc-events-42`)と、作り直した後に届く新しい
イベントの `event_id` が衝突しうる。衝突すると新しいイベントは `calc_events` への `INSERT` が
一意制約違反になり、record-svc は「重複」と誤認して(`store.Duplicate`)保存せずに ack してしまう
(サイレントなデータ欠損)。ストリームを作り直す運用(バージョンアップでの `Subjects`/`Retention` の
再作成、障害復旧での re-provision 等)を行うときは、**同じ操作で record DB の `calc_events`
テーブルも合わせて空にする**(または record-svc を再作成前に一時停止し、再作成後に空の状態から
再開する)こと。ストリームの通常の再起動(NATS Pod の再起動・`CreateOrUpdateStream` によるべき等な
再適用)ではシーケンスは維持されるため、この対応が要るのは「ストリームを明示的に delete して
作り直す」場合に限る。恒久対策(シーケンス以外の情報を event_id に混ぜる、ストリームの作成時刻を
含める等)は、実際にストリームの再作成が運用上必要になったとき(P7-4 のバックアップ/復元設計や
NATS のバージョンアップ手順を書くとき)に判断する。

### 5. Go クライアントは新 `jetstream` パッケージを使う(legacy の `JetStreamContext` ではない)

nats.go v1.54.0 自身が `JetStreamContext`(`nats.Conn.JetStream()` で得られる旧 API)を
legacy と明記し、新しい `github.com/nats-io/nats.go/jetstream` パッケージへの移行を推奨している。
「依存は常に最新安定版を固定」という方針は API の版にも及ぶと解釈し、新 `jetstream` パッケージ
(`jetstream.New(nc)` → `jetstream.JetStream`)を使う。ストリームの作成は
`CreateOrUpdateStream(ctx, cfg)`(存在しなければ作成・存在すれば設定を合わせて更新。何度実行しても
安全)を呼ぶだけでよく、旧 API のような「`AddStream` を試して失敗したら `UpdateStream`」という
手動の分岐は不要。**NATS に接続できない・ストリーム作成に失敗しても calc-svc の起動は失敗させない**
(CLAUDE.md 絶対ルール5)。record-svc/team-svc より先に calc-svc が動き出す前提は、calc-svc が
「クライアントの唯一の入口」だからではなく(それは gateway。CLAUDE.md リポジトリ構成・
requirements.md §4)、単純に docs/plan.md の実装順(P5-2 が P5-3/P5-4 より先)による。

**起動時に1回失敗したら発行を諦めない(`fetchMasterLoop` と同じ背景再試行の型を流用する)**:
`nats.RetryOnFailedConnect(true)` を付けた `nats.Connect` は、サーバに接続できない状態でも
「再接続中」の `*nats.Conn` を成功として返す。つまり `CreateOrUpdateStream` を起動時に1回だけ
呼んで失敗を確認する設計だと、「接続はいずれ回復するが、ストリーム作成の再試行はどこにも書かれていない」
状態になり、**NATS が calc-svc より遅れて Ready になっただけで、そのプロセスの寿命中ずっと
発行が無効のまま**になってしまう(§4 が Limits retention を選んでまで防ごうとした「静かな
イベント喪失」が、この経路で `make up` のたびに再現しうる)。
これを避けるため、ストリーム作成は `services/calc/cmd/calc/main.go` の `fetchMasterLoop`
(指数バックオフで再試行し、`atomic.Pointer` に成功結果を格納する。ADR-0204 §3 で決めた既存の型)と同じ型で書く:
バックグラウンドの goroutine が `CreateOrUpdateStream` を指数バックオフで再試行し続け、成功したら
`atomic.Bool`(または `atomic.Pointer[jetstream.Stream]`)に成功を記録して以後の発行を有効にする。
NATS 自体が完全に無効(`CALC_NATS_URL` 未設定)の場合はこの goroutine 自体を起動しない。

### 6. 接続・発行はリクエストの応答を絶対にブロックしない

- `CALC_NATS_URL` 環境変数(任意)。未設定なら発行を最初から無効にする(NATS 無しでも calc-svc は
  今までどおり動く。ローカルの単体テストで NATS を用意しなくてよいようにするため)。
- 設定されていれば起動時に1回だけ接続を試みる(`nats.Connect` に `nats.RetryOnFailedConnect(true)`・
  `nats.MaxReconnects(-1)`(無限に再接続を試みる)・`nats.Timeout(500 * time.Millisecond)`
  (`connectTimeout`。nats.go の既定2秒のままだと接続先が無応答〈TCP は繋がるが INFO を返さない等〉の
  ときに calc-svc の起動〈`newHandler` → `srv.ListenAndServe`〉をその秒数だけ遅らせる。実測して判明。
  この値は初回接続だけでなく `MaxReconnects(-1)` による再接続1回ごとの上限にもなる)を渡し、
  **起動時に NATS が落ちていても calc-svc の起動をブロック・失敗させない**)。
- 発行は `jetstream.Publisher.PublishAsync(subject, payload)`(応答を待たない)を別 goroutine で行う。
  この関数(`Publish`)自体は `json.Marshal(event)` を同期的に行うだけで、`PublishAsync` の呼び出しと
  `select` での確認は別 goroutine 側にあるため、ハンドラが `ctx.JSON(...)` を呼ぶ前後どちらで
  `Publish` を呼んでも「レスポンスを書き終えるまで発行が応答を遅らせない」という目的は保たれる
  (実装は `services/calc/internal/httpapi/server.go` 参照。`ctx.JSON` の前に呼んでいるが、
  これは書きやすさの都合であって安全性の理由ではない)。
  **goroutine へ渡す値は、起動前にすべてコピーする**(`params.XDeviceId`・`params.XSessionId` の文字列と、
  イベントに詰める構造体はコピー渡しでよいが、`*echo.Context` そのものは goroutine に持ち出さない。
  echo v5 は `Context` をリクエストごとにプールして使い回すため、ハンドラの return 後に goroutine から
  読むとデータ競合になる)。
- **タイムアウト**: 新 `jetstream` パッケージの `PublishAsync(subject string, payload []byte, opts
  ...PublishOpt) (PubAckFuture, error)` はそもそも `context.Context` を引数に取らない(legacy の
  `JetStreamContext.PublishAsync` が `nats.Context()` オプションを渡すとエラーで拒否するのとは
  別の理由で、型として渡せない)。パッケージ全体の既定タイムアウトを `jetstream.New(nc,
  jetstream.WithPublishAsyncTimeout(dur))` で設定する方法もあるが、発行 goroutine 1件ごとに
  確実に終わらせたい(接続全体の設定に依存させない)ため、`PubAckFuture` の `Ok()`/`Err()`
  チャンネルと `time.After(2 * time.Second)` を `select` で待つ形にする:
  ```go
  future, err := js.PublishAsync(subject, payload)
  if err != nil {
      slog.Warn("calc-svc: イベント発行に失敗", "operation", op, "error", err)
      return
  }
  select {
  case <-future.Ok():
  case err := <-future.Err():
      slog.Warn("calc-svc: イベント発行に失敗", "operation", op, "error", err)
  case <-time.After(2 * time.Second):
      slog.Warn("calc-svc: イベント発行の確認がタイムアウトした", "operation", op)
  }
  ```
  `PublishAsync` は未確定の発行が既定4000件に達すると内部で最大200msブロックしてから
  `stalled with too many outstanding async published messages` を返すことがある(v1.54.0 の実装)。
  この待ちは呼び出し元の goroutine 内で完結し(HTTP 応答は既に書き終えている)、絶対ルール5には
  抵触しないが、ログの警告対象として明記しておく。
- **終了時の扱い(シャットダウン時にイベントを失わない)**: `services/calc/cmd/calc/main.go` の
  `http.Server.Shutdown`(`shutdownTimeout` 5秒)は in-flight の HTTP リクエストだけを待ち、
  発行 goroutine やまだ確定していない `PublishAsync` の future までは待たない。ローリング更新
  (SIGTERM)のたびに直前の発行が失われては、§4 で Limits retention を選んでまで守ろうとした
  「イベントを静かに失わない」という目的と矛盾する。そこで HTTP の `Shutdown` が完了した**後**、
  発行 goroutine を `sync.WaitGroup` で数えて待つが、**この待ち自体にも上限を付ける**
  (`WaitGroup.Wait()` を直接呼ばず、`Wait()` を別 goroutine で行い `done` チャンネルの
  close を `select` で待つ形にし、上限2秒とする。個々の発行 goroutine は §6 の `select` で
  最大2秒+stall 200ms 程度に収まるが、多数の発行が同時に残っている場合でも合計の待ちを
  有界にするため)。続けて `<-js.PublishAsyncComplete()` を上限2秒で待ってから `nc.Drain()` する
  (`PublishAsyncComplete()` は未確定の発行が0件ならすぐ close される)。
  合計のプロセス終了予算は概ね「HTTP の `shutdownTimeout` 5秒 + 発行 drain の上限 約4秒」になる
  (この待ちは HTTP のリクエスト処理そのものはブロックしない。プロセス終了シーケンスの一部)。
- **配送保証**: at-least-once(JetStream の既定。重複排除〈`Nats-Msg-Id` ヘッダによる dedup window〉は
  P5-2 では設定しない)。record-svc/team-svc(P5-3/P5-4)は同じイベントを2回受け取りうる前提で、
  消費側の処理を冪等に設計すること(「影響」に申し送りとして明記する)。

### 7. イベントのワイヤフォーマット(`services/internal/calcevents`)

record-svc(P5-3)・team-svc(P5-4)の両方が読む共有パッケージとして新設する
(`services/internal/api` と同じ「複数サービスが読む契約」の置き場所)。JSON でエンコードする
(NATS のペイロードはバイト列。JSON は他言語からの調査もしやすい)。

```go
package calcevents

// SchemaVersion はワイヤフォーマットの版(1始まり)。calc-svc と record-svc/team-svc は
// 独立にデプロイされ、最長7日分のイベントがストリームに滞留しうる(§4)ため、
// 構造を変えるときはこの値で分岐できるようにしておく。
const SchemaVersion = 1

// Event は1回の計算 API 呼び出しに対応する(ADR-0209 §3 #6)。
type Event struct {
	SchemaVersion int       `json:"schemaVersion"`
	DeviceID      string    `json:"deviceId"`
	SessionID     string    `json:"sessionId"`
	Operation     string    `json:"operation"` // "calc" | "calcBulk" | "calcReverse"
	OccurredAt    time.Time `json:"occurredAt"` // calc-svc が計算した時刻(ADR-0209 §7。record-svc の受信時刻ではない)

	// Detail は Operation == "calc" のときだけ入る(§7.1)。team-svc は Detail を一切読まない
	// (devices.last_seen_at の更新に使うのは envelope〈DeviceID・OccurredAt〉だけ。ADR-0209 §4)。
	Detail *CalcDetail `json:"detail,omitempty"`
}

// CalcDetail は POST /api/calc(1件の攻撃側 vs 防御側)の内容(requirements.md §6 の calc_events 列)。
// フィールドは api.CalcRequest・api.CalcResult(services/internal/api の生成型)から直接埋める。
type CalcDetail struct {
	Format   string          `json:"format"`
	Attacker api.Individual  `json:"attacker"`
	Defender api.Individual  `json:"defender"`
	// MoveID は req.MoveId(CalcRequest のトップレベル)から取る。api.Individual.MoveId ではない
	// (calc-svc の resolveIndividual は Individual.MoveId を読まない。「状況」を決めるのは
	// リクエストの moveId・field・options であり、個体側の任意フィールドではない)。
	MoveID  string           `json:"moveId"`
	Field   *api.FieldState  `json:"field,omitempty"`
	Options *api.CalcOptions `json:"options,omitempty"`

	// MinPercent/MaxPercent は api.CalcResult.MinPercent/MaxPercent(表示%。丸め後の値)をそのまま写す。
	// engine の 0.1% 単位の整数(tenths)ではなく、クライアントに返すのと同じ表示%(ADR-0010 §3。
	// MinPercent は切り捨て・MaxPercent は四捨五入で、丸め方向が異なることに注意)。
	MinPercent float64 `json:"minPercent"`
	MaxPercent float64 `json:"maxPercent"`

	ViaRecommendation bool `json:"viaRecommendation"`
}
```

- `api.Individual`/`api.FieldState`/`api.CalcOptions`(`services/internal/api` の生成型)をそのまま使う
  (重複定義しない。攻撃側/防御側の個体・状況は公開 API のリクエスト形式そのものであり、record-svc が
  履歴に必要とする内容〈requirements.md §6〉と一致するため。将来この結合が問題になったら〈公開契約を
  変えたいがイベント契約は変えたくない、等〉そのとき専用の型に切り出す。今の時点で先取りしない)。
  この結合により、`api/openapi.yaml` の破壊的変更(CLAUDE.md 絶対ルール1)が `calcevents` の
  ワイヤフォーマットを黙って変えうる。この ADR(P5-2)の**受け入れ条件 AC-N8** として、
  `calcevents` 側に契約テスト(`services/internal/api/client_id_semantics_test.go` と同じ発想。
  openapi の変更で `Individual`/`FieldState`/`CalcOptions`/`CalcResult` の該当フィールドが
  変わったら検知して落ちるテスト)を今このタスクで用意する(P5-3 に持ち越さない。
  `calcevents` パッケージ自体は P5-2 で新設するため、そのテストも同時に書ける)。
- `Options`(急所固定等。requirements.md §6 の「状況」の一部と解釈する)も含める。`Field` と対称に
  「計算結果を左右する入力のうち、個体そのものではないもの」として扱う。
- `ViaRecommendation`(推薦経由か。requirements.md §6)は今の calc-svc の入力に対応するフィールドが
  無いため、**常に `false` を送る**(推薦機能〈Web の「おすすめタイプ」等〉が calc API 呼び出しの
  どこかにその情報を持たせるようになったら埋める。今は false 固定の理由をコードコメントに残す)。

**§7.1 `calc`・`calcBulk`・`calcReverse` で発行内容を分ける理由**:

- `POST /api/calc`(1攻撃側 vs 1防御側の具体的な組み合わせ)は、requirements.md §6 の `calc_events` の
  列(攻撃側/防御側の個体・技〈リクエストの `moveId`〉・状況〈`field`・`options`〉・ダメージ幅)に
  そのまま対応する。「よく使う相手」の頻度集計にも実際に行った1組の対戦として意味を持つ。
- `POST /api/calc/bulk`(1攻撃側 vs 最大 **512件**〈`len(presets) × len(itemVariants)` = 8 × 64。
  ADR-0208。128件ではない〉の**仮想的な**候補防御側)は、利用者が「比較検討している」状態であり、
  「実際にその相手と対戦した」頻度としてカウントすると「よく使う相手」の集計が比較検討中の候補で
  汚染される。したがって `calcBulk` も **envelope を1件だけ発行し、`Detail` は付けない**
  (`Operation: "calcBulk"`、`Detail: nil`)。team-svc の `devices.last_seen_at` 更新にはこれで十分で、
  record-svc 側の履歴・推薦への算入は P5-3 で「計算 API を使った」という軽量な活動記録として
  扱うか、記録しないかを P5-3 で決める(この ADR ではスコープ外。1リクエストにつき1件の
  envelope イベントを発行することだけをここで確定する)。
  candidate ごとに1件ずつ発行しない(最大512件 ×毎リクエストは、個人開発規模でも発行数が
  跳ね上がり、上記の「頻度の汚染」の問題も解決しない)。
- `POST /api/calc/reverse`(SP の組み合わせの逆算。最大候補数 `maxCandidates` の上限は128件
  〈2性格クラス×64 itemCandidates〉)は1回の呼び出しで単一の「攻撃側 vs 防御側」の組ではなく
  総当たり探索(ADR-0208)であり、`calc` と同じ形の `CalcDetail` にそのまま当てはめられない。
  `calcBulk` と同様に **envelope を1件だけ発行する**(`Operation: "calcReverse"`、`Detail: nil`)。

### 8. ログに残す項目(ADR-0209 §3 #1 の「ログに残してよい項目」に従う。#6 ではない)

calc-svc が発行の成功・失敗をログに出すときは、端末 ID・セッション ID・`operation`・成功/失敗だけを出す
(ADR-0209 §3 #1 の行の基準。同 #6 の行〈ストリーム名・シーケンス番号・consumer 名〉はストリーム自体の
運用ログ向けで、この ADR の calc-svc 側の発行ログはリクエスト起因のログとして #1 に従う)。
`CalcDetail` の中身(個体・技・ダメージ)は出さない(coding-rules §1 のログ方針と同一)。

### 9. NATS を同じ `deploy/k8s/overlays/local` kustomization に含めてよい理由(TiDB との違い)

ADR-0211 §3.2 は TiDB(TidbCluster/TidbInitializer)をメインの overlay から意図的に分離した。
その理由は「CRD が無い」という条件そのものではなく、**CRD の有無が `scripts/tidb-operator-bootstrap.sh`
という実行時のステップの成否に左右され、事前に静的検査で検出できない**ことだった(bootstrap が
失敗すれば CRD が無く、`kubectl apply -k` 全体が失敗する。しかも `make lint`/`make k8s-render` は
CRD の実在をローカルでは検証できない)。
NATS の StatefulSet/Service は CRD に依存せず、`kubectl kustomize deploy/k8s/overlays/local`
(`make k8s-render`。`make lint` の一部)で**常に**事前に構文・構成の妥当性を検証できる。
つまり「NATS 側のマニフェストが壊れている」という失敗モードは `make lint` の時点で必ず捕まり、
実際の `make up` 実行時に初めて分かることがない。これが TiDB との本質的な違いであり、
同じ overlay に含めても「事前に検出できない理由で mysql・pokedex を巻き込んで apply が失敗する」
リスクを増やさない。

## 却下した案

- **Interest retention をそのまま使う**: §4 のとおり、P5-3/P5-4 の consumer が揃うまで(あるいは
  consumer の `FilterSubject` を誤ると揃った後も)イベントがストアに書き込まれすらしない欠陥が
  あるため却下。
- **P5-2 の時点で record-svc/team-svc の durable consumer を先に作り、Interest retention を使う**:
  §4 のとおり、consumer の `FilterSubject` は本来 P5-3/P5-4 が決めるべき設定で、P5-2 が先取りすると
  取り違えたときに同じ「静かな消失」を再発させる。
- **candidate ごとに bulk イベントを分割発行する**: §7.1 のとおり、「よく使う相手」の集計を
  比較検討中の候補で汚染し、かつ発行数が増える(最大512件)。
- **NATS を Kubernetes Operator(NACK 等)で導入する**: 個人開発規模の単一レプリカでは
  Operator 導入のコスト(CRD・controller の追加)に見合わない。TiDB(ADR-0211)は複数コンポーネントの
  トポロジ管理が本質的に必要だったため Operator を選んだが、NATS はそれ自体が1バイナリで完結する。
- **calc-svc の応答を JetStream の ack 確認まで待たせる(同期発行)**: CLAUDE.md 絶対ルール5に反する
  (NATS が遅い・落ちているときに計算 API が遅くなる/失敗する経路を作ってしまう)。
- **legacy の `JetStreamContext` を使う**: §5 のとおり nats.go v1.54.0 自身が非推奨としている。

## 影響

- 新規: `services/internal/calcevents/`(イベント型)、`scripts/nats-local-up.sh`、
  `deploy/k8s/overlays/local/nats/`(kustomization・statefulset・service)、
  `services/calc/internal/events/`(NATS 接続・発行ロジック。ハンドラから呼ぶ薄い層)。
- 変更: `services/calc/cmd/calc/main.go`(`CALC_NATS_URL` 環境変数の追加)、
  `services/calc/internal/httpapi/server.go`(`Server` に publisher を追加。3ハンドラが成功時に発行を呼ぶ)、
  `scripts/up.sh`(NATS の StatefulSet 適用・calc の Deployment への `CALC_NATS_URL` 追加)、
  `deploy/k8s/overlays/local/kustomization.yaml`(`nats` を追加。§9 参照)、
  `Makefile`(`nats-local-up` ターゲット。`db-local-up`・`tidb-local-up` と同形。`make k8s-render` に
  `deploy/k8s/overlays/local/nats` を含む local overlay の描画確認が既に含まれる〈§9〉ため
  `k8s-render` 自体への追記は不要)、
  `docs/plan.md`(P5-2 の記述は既にこの ADR の内容と整合しているため、チェックのみ更新)、
  `docs/adr/0209-record-team-data-retention.md`(§3 #6 の行に「Retention は Limits で確定
  〈ADR-0212 §4〉。`両方の consumer が ack したら破棄` の早期削除は実装しない。7日以内に自然に
  消える結論〈ADR-0209 §5〉自体は変わらない」を追記する。ADR-0211 が ADR-0209 に既知ギャップを
  追記した前例と同じ扱い)。
- P5-3/P5-4 はこのイベント型(`services/internal/calcevents.Event`)・ストリーム名(`CALC_EVENTS`)・
  subject パターン(`calc.events.<device_id>`)をそのまま前提にする。**at-least-once 配送**
  (重複排除は設定しない。§6)なので、record-svc/team-svc の消費処理は同じイベントを2回受け取っても
  安全なように(冪等に)設計すること。

## 受け入れ条件

- AC-N1: `CALC_NATS_URL` が未設定でも calc-svc は今までどおり起動・応答する(発行しないだけ)。
- AC-N2: `CALC_NATS_URL` が設定されているが接続先が無応答でも、calc-svc の起動・`/api/calc` 系の応答時間は
  変わらない(発行はリクエスト処理と並行/事後の goroutine で行われ、応答をブロックしない)。
- AC-N3: 実 NATS(ローカルの `nats-local-up.sh`)に対して `POST /api/calc` を呼ぶと、
  `calc.events.<device_id>` に `SchemaVersion`・`DeviceID`・`SessionID`・`OccurredAt`・
  `Operation: "calc"` かつ `Detail` 入りのメッセージが1件発行される。
  `POST /api/calc/bulk`・`POST /api/calc/reverse` は同じ envelope フィールドを持ち `Detail` が
  `nil` のメッセージを1件発行する。
- AC-N4: ストリームの `MaxAge` が7日、`MaxBytes` が512Mi、`Retention` が Limits であること
  (consumer が無い状態で `POST /api/calc` を呼んでもメッセージが保存されること〈`nats stream info`
  の `state.messages` が増えることを確認〉を実 NATS で確認する)。
- AC-N5: calc-svc の既存のゴールデン/契約テストに一切回帰が無い(発行はテスト用の偽 publisher で
  差し替え、実 NATS 無しで `make test` が通る)。
- AC-N6: calc-svc のプロセス終了(SIGTERM)時、発行 goroutine と未確定の `PublishAsync` を
  `http.Server.Shutdown` 完了後に上限付きで待ってから終了すること(§6)。実装時に「待たずに
  即終了する」よう一時的に壊し、発行し損ねが再現してから戻す(mutation 確認)。
- AC-N7: `ViaRecommendation` が常に `false` で発行されること(回帰テスト)。
- AC-N8: `api/openapi.yaml` の `Individual`/`FieldState`/`CalcOptions`/`CalcResult` の該当フィールドが
  変わったら `calcevents` 側のテストが検知して落ちること(§7 の結合を許容する条件)。
- AC-N9: calc-svc の起動時に NATS が未起動(`CreateOrUpdateStream` が失敗する状態)でも、その後
  NATS が起動すれば、**calc-svc を再起動しなくても**それ以降の計算イベントが発行されるようになること
  (§5 の背景再試行を実 NATS で確認する。`make up` で NATS より calc-svc が先に Ready になる
  通常の順序でも発行が永久に無効化されないことの確認)。

## 人間の確認が必要なこと

なし(NATS の新規導入は取り消しやすく〈StatefulSet・PVC を消すだけ。TiDB と違い `pvReclaimPolicy` を
`Retain` にしない。JetStream のファイルストレージは使い捨てのイベントキューであり、消えても
実害は直近7日分のイベントを失うだけで、record/team 側の TiDB のような「本体データ」ではない〉、
クラウド構成・認証の判断にも依存しない)。共有 k3d クラスタへの実適用そのものは、§9 のとおり
`make lint`(`k8s-render`)で事前検証できるため、TiDB(ADR-0211)ほど慎重な扱いを要しないが、
実際に適用した際は他サービスの Pod が影響を受けないことを確認する(AC-N4 の確認と合わせて行う)。

## 変更履歴

- 2026-09-25 第1回 critic レビュー FAIL を受けて改訂: Interest retention 却下の理由を NATS Server
  v2.15.0 の実ソース(`SkipMsgNoInterest`。保存されず PubAck は正常に返る)に基づいて補強し、
  「consumer を先に作る」案を却下案に追加。ストリームに `Subjects`(ワイルドカード)・`MaxBytes`・
  `Discard` を追加し、容量の見積もりを明記。nats.go の legacy `JetStreamContext` ではなく新
  `jetstream` パッケージ(`CreateOrUpdateStream`)を使うと確定。発行のタイムアウトを
  `context.WithTimeout`(`PublishAsync` は受け付けない)から `PubAckFuture` の `Ok()`/`Err()` と
  `time.After` の `select` に修正。goroutine への値コピー規約とプロセス終了時の drain
  (`PublishAsyncComplete`)を追加し AC-N6 で固定。`CalcDetail` に `MoveID`(`Attacker.MoveId` ではなく
  リクエストの `moveId`。従来の記述は実コードと不一致だった)・`Options`・`SchemaVersion` を追加し、
  `DamagePercentMin`/`Max` を実際の生成型に合わせて `MinPercent`/`MaxPercent` に改名。
  「calc-svc が公開 API の入口(ADR-0204)」という誤った引用(実際の入口は gateway)を修正し、
  P5-2 が P5-3/P5-4 より先に動く理由を実装順序に修正。ADR-0208 の bulk 上限を 128 から
  正しい 512(`8 × 64`)に修正。NATS を同じ overlay に含めてよい理由を「CRD 不要」から
  「静的検査〈`make lint`〉で事前に必ず検出できる」に書き直し(§9 を新設)。
  「影響」に plan.md・ADR-0209 への追記・Makefile を追加し、at-least-once 配送(重複排除なし)を
  P5-3/P5-4 への申し送りとして明記。
- 2026-09-25 第2回 critic レビュー FAIL を受けて改訂: `Subjects` の根拠が ADR-0209 §2 への誤引用
  だったのを ADR-0202 §4・`isCanonicalUUID` に訂正し、calc-svc 自身は書式検証をしないこと・
  内部呼び出しからの不正な値で発行が失敗しうることを明記。`nats.RetryOnFailedConnect` は
  接続の再試行だけを保証し、`CreateOrUpdateStream` が起動時に1回失敗すると恒久的に発行が
  無効化されたままになる設計の穴を修正: `fetchMasterLoop` と同じ背景再試行の型でストリーム作成を
  再試行し、成功したら発行を有効化する(AC-N9 で固定)。§6 のタイムアウト理由を legacy API 固有の
  記述から新 `jetstream` パッケージの実際の型制約に訂正し、`WithPublishAsyncTimeout` を検討した
  上で per-goroutine の `select` を選ぶ理由を追記。`sync.WaitGroup` の待ちにも明示的な上限を付け、
  プロセス終了の合計予算(約9秒)を明記。AC-N8(契約テスト)の所属を「P5-3 の受け入れ条件」から
  この ADR(P5-2)自身の受け入れ条件に統一。AC-N4 の確認コマンドを `nats stream get` から
  `nats stream info` に訂正。
- 2026-09-25 第3回 critic レビュー PASS(軽微1件の推奨修正あり)。§5 の `fetchMasterLoop` の出典が
  裸の「§3」(ADR-0212 自身の§3〈k3d の StatefulSet〉と誤読されうる)になっていたのを
  「ADR-0204 §3」に明示し、「関連」に ADR-0202・ADR-0204 を追加した。
- 2026-09-25 実装(implementer)完了後の critic レビュー(第1回)を受けて改訂:
  `docs/plan.md` の P5-2 チェックと ADR-0209 §3 #6 への参照追記が未実施だった(CLAUDE.md 絶対ルール8)
  のを両方とも同じコミットに含めるよう修正。AC-N6 の drain テストがポーリング後に確認していたため
  drain 自体の破壊を検知できなかったのを、ポーリングを挟まず即 `Shutdown()` してから確認する
  `TestShutdownDrainsPendingPublish` に差し替えた(mutation テストで5回連続の検知を確認)。
  AC-N8 の契約テストがコンパイル時チェックのみで、埋め込み型(`api.Individual` 等)の内部フィールド
  リネームを検知できなかったのを、JSON をリテラル比較する `golden_test.go` を追加して補強した。
  `nats.Connect` が到達不能なホストに対して既定で約2秒ブロックする実測を受けて
  `nats.Timeout`(`connectTimeout` = 500ms)を追加し、`TestNewReturnsQuicklyForUnresponsiveHost` で固定。
  `go.mod`/`go.sum` が `go mod tidy` 未実施だったのを修正。
  **`CALC_NATS_URL` を `deploy/k8s/base/calc/deployment.yaml` に置いたことで cloud overlay にも
  同じ値が乗る(cloud には本 ADR のスコープ〈local/k3d まで〉として NATS を用意していない)問題**
  について: NATS 未到達時も `events.New` は接続をブロックせずに継続し(上記の `connectTimeout` 修正
  および `RetryOnFailedConnect`/`ensureStreamLoop` の背景再試行により)、`Publish` は `p.ready` が
  立たない限り何もしない no-op であり続けるため、cloud 環境では calc-svc は
  「`ensureStreamLoop` が到達不能で warn ログを出し続けるだけ」で HTTP API 自体への影響は無いと判断し、
  cloud 用に値を分岐させる対応はしない(CLAUDE.md 絶対ルール5)。この判断は
  `deploy/k8s/base/calc/deployment.yaml` のコメントに記録し、cloud 側に NATS を実際に用意するかどうかは
  `+α` 判断として後続(P5-3/P5-4 以降、または人間の確認)に委ねる。
