# ADR-0230: 計算履歴(生の計算イベント)の一覧 API(record-svc)

- 状態: 採用(critic PASS。2026-10-09)
- 日付: 2026-10-09
- 関連: ADR-0209(保持・削除・端末 ID 境界。§3 の calc_events の目的「履歴表示」・§4・§5・§6・§7 が前提)、
  ADR-0212(計算イベントのワイヤフォーマット。§7.1 の Detail は calc のときだけ)、ADR-0227(お気に入りの API。
  record の操作の流儀)、ADR-0228(お気に入りの calc = CalcRequest と正規化)、ADR-0208(上限を契約に書く・新しい code を足さない)、
  ADR-0802(サービス間で揃えたエラーの形)、ADR-0211 §7(保持日数は環境変数で、既定値へのフォールバックをしない)、
  ADR-0807(生成物はコミットしない)、requirements.md §2「計算結果の自動保存」・§6 の calc_events の列、
  docs/plan/m2.md P5-5c(「履歴一覧は record-svc に取得 API が無いため対象外」)、
  docs/ai-shared/decisions/2026-10-04-ios-favorites-history-spec.md(iOS は「契約待ち」と注記)

## 背景

calc-svc は1件の計算(`POST /api/calc`)・一括計算・逆算のたびに NATS へイベントを発行し、record-svc はそれを
`calc_events`(migrations/000003)に保存している。公開しているのは集計の `GET /api/record/frequent-opponents` だけで、
生の履歴を一覧する API が無い。Web の P5-5c は履歴一覧を対象外にし、iOS は「お気に入り・履歴」画面に「契約待ち」と
注記している。ADR-0209 §3 は calc_events の目的を「履歴表示・推薦の集計」と定めており、前者が未実装のまま残っている。

決めること: (1) 経路・並び・ページング・上限、(2) 返す内容(payload に何があり、何を返すか)、(3) 端末分離・ヘッダ・
store の規則・`last_seen_at`、(4) 保持期間・全削除・墓石・失効ジョブとの整合、(5) 個別削除の要否、(6) エラー、
(7) 索引、(8) 実装の形(store と httpapi の境界)。

### calc_events.payload の中身(調べた結果)

record-svc の consumer(`services/record/internal/events/consumer.go`)は受け取った `calcevents.Event` を
**丸ごと** `json.Marshal` して `payload` に入れている。つまり payload は
`{schemaVersion, deviceId, sessionId, operation, occurredAt, detail?}` で、`detail`(`calcevents.CalcDetail`)は
operation が `calc` のときだけあり、`format`・`attacker`・`defender`(`api.Individual`)・`moveId`・`field`・`options`・
`minPercent`・`maxPercent`・`viaRecommendation` を持つ。calc-svc はこれを `api.CalcRequest` の各項目から**そのまま**埋めている
(`services/calc/internal/httpapi/server.go`。成功した計算の後にだけ発行する)。

したがって **1件の計算は「その計算をもう一度出す」のに必要な入力(CalcRequest と同じ6項目)をすでに payload に持つ**。
イベントの拡張(schemaVersion の更新)は要らない。一方、一括計算・逆算は envelope だけで入力を持たない。
また payload には端末 ID・セッション ID が入っているので、そのまま返してはいけない。

## 決定

### 1. 経路と形: `GET /api/record/calc-history`(`listCalcHistory`)。新しい順・keyset のカーソル

| 項目 | 決定 |
|---|---|
| 経路 | `GET /api/record/calc-history`。operationId `listCalcHistory`。タグ `record` |
| 並び | `occurred_at` の降順、同時刻は `event_id` の降順(決定的な全順序) |
| 1ページ | `limit`: 1〜50、既定 20(範囲外・整数でない値は 400 `invalid_input`) |
| 続き | `cursor`(任意)。応答の `nextCursor` をそのまま渡す。続きが無ければ `nextCursor: null` |
| 応答 | `CalcHistoryPage = { items: CalcHistoryEntry[] (maxItems 50), nextCursor: string \| null }`(両方必須) |

- **経路名を `calc-events` にしない理由**: 返すのは生のイベントではなく、1件の計算だけを選び、項目を絞った「履歴」という
  見え方(§2)。イベントそのものを返すと読める名前にすると、payload の丸ごと露出や bulk/reverse の行を期待させる。
  集計の `frequent-opponents` と同じく、画面の言葉(requirements.md §2「履歴」)で名付ける。
- **ページングを持つ理由**: お気に入り(上限100件・ページングなし。ADR-0227 §3)と違い、履歴は保持期間(90日)の間
  上限なく増える(個人利用の見積もりで 1日数百件 × 90日 = 数万件)。全件を返すと応答が数 MB になる。
- **keyset(時刻 + event_id)にし、offset にしない理由**: 履歴は先頭(最新)に行が増え続ける。offset だと、ページの間に
  新しい計算が保存されると次のページの先頭がずれて同じ行が2回出る。keyset なら「この位置より古い行」なので、
  新しい行が増えても続きは変わらない(AC-H4)。`occurred_at` だけでは同時刻(マイクロ秒が同じ)の行で取りこぼすため、
  一意な `event_id` を第2キーに含める。
- **カーソルの形式**: サーバーが作る不透明な文字列。契約では `^[A-Za-z0-9_-]+$`・1〜200文字(URL にそのまま入る文字だけ)
  とだけ決め、中身はクライアントに見せない(実装の既定案: `occurred_at` の UnixNano と `event_id` を区切って
  base64url〈パディングなし〉にしたもの)。読めない値は 400 `invalid_input`。カーソルは端末を指さない(位置だけを持つ)ので、
  他端末のカーソルを渡されても、問い合わせは常に `X-Device-Id` の端末で絞られ、他端末の行は出ない(§3)。
  カーソルに含まれる event_id(ストリームのシーケンス由来)から「全端末の計算回数のおおよそ」が推測できるが、
  個人利用・Tailscale 内(ADR-0209 §1)では実害が無く、暗号化・署名の複雑さに見合わないので許容する。
- **`limit` の上限 50・既定 20**: `listFrequentOpponents`(1〜50)と揃える。1行は計算の入力全体で約 1〜1.5KB
  (ADR-0212 §4 の見積もり)なので、50件で 75KB 程度。画面の1スクロール分(20件)を既定にする。
- **最後のページで `nextCursor` を null にする**(残りがちょうど `limit` 件でも、空のページを余計に1回返さない)。
  実装は store に `limit + 1` 件を問い合わせ、`limit` 件を超えたときだけ `limit` 件目の位置をカーソルにする(既定案)。

### 2. 返す内容: 1件の計算だけ・項目はホワイトリスト(occurredAt・calc・result)

`CalcHistoryEntry = { occurredAt: date-time, calc: CalcRequest, result: { minPercent, maxPercent } }`(3つとも必須。
これ以外の項目を持たない)。

- **operation = `calc` の行だけ返す**。一括計算・逆算のイベントは入力を持たない(ADR-0212 §7.1)ので、行を選んでも
  計算を出し直せず、「いつ一括計算をした」だけの行は画面の役に立たない。イベントを拡張して bulk/reverse の入力も持たせる案は
  §「却下した案」。
- **`calc` は `CalcRequest` そのもの**(契約は `$ref` だけで参照し、Go・TS・Swift の生成型も `CalcRequest`)。
  お気に入りの `Favorite.calc` と**同じ正規化**(httpapi の `normalizeCalc` → `calcFrom`。既定値を補った形)で返す。
  Web・iOS は「お気に入りの calc から計算を出す」処理(ADR-0228)をそのまま履歴に使える(AC-H2)。
  正規化は保存時ではなく**返すとき**に行う(payload は calc-svc が受け取った形のまま保存されている。既存の行も読める)。
- **`result` は表示%の幅だけ**(`CalcDetail.minPercent` / `maxPercent`。CalcResult と同じ値・同じ丸め)。乱数の16段階・
  確定数・ダメージ実数は payload に無い。一覧の1行に「41.2〜48.9%」と出すには十分で、詳しく見たければ `calc` で計算し直す。
- **返さないもの**: 端末 ID・セッション ID(ADR-0209 §3「入れる情報」の限定。クライアントは自分の端末 ID を知っているが、
  サーバーが payload から写して返す経路を作らない)・`schemaVersion`・`operation`・`viaRecommendation`(常に false の
  内部項目)・`event_id`(行の ID は持たない。§5 で個別削除も取得も持たないため)・payload の未知のキー。
  httpapi は payload を `calcevents.Event` に**寛容に**(未知のキーを許して)読み、上の項目だけを詰め替える
  (payload を丸ごと、または `json.RawMessage` で中継しない。AC-H3)。
- **読めない行は飛ばす**(壊れた JSON・calc なのに detail が無い・`schemaVersion` が未知・正規化できない入力)。
  1行の破損で履歴全体を 500 にしない。飛ばした行もページの位置には数えるので、ページの件数は `limit` 未満になりうる
  (`nextCursor` が null でなければ続きがある、という規則は変わらない)。ログには event_id と端末 ID だけを出し、
  payload の中身は出さない(AC-H12)。お気に入りの壊れた snapshot を 500 にする(ADR-0228)のと扱いを変えるのは、
  履歴は利用者が作ったものではなく自動で貯まる行で、1行を見せられないことより一覧全体が使えないことの方が害が大きいため。

### 3. 端末分離・ヘッダ・store の規則・last_seen_at

- 端末 ID はヘッダ(`X-Device-Id`)だけ。クエリ・ボディに `deviceId` / `device_id` が来たら 400 `unknown_field`
  (`checkNoDeviceIDInQuery`・`decodeNoBody`。既存の操作と同じ)。ヘッダが無ければ 400 `missing_header`(生成ラッパ)。
- store の新しいメソッドも `deviceID` を必ず受け、SQL は `WHERE device_id = ?` を持つ(store.go の規則)。
- `touchAndRun` を通す(端末 ID を含む要求を受けたら `devices.last_seen_at` を更新する。ADR-0209 §4。24時間規則つき)。
  TouchDevice が失敗したら一覧は読まない。
- 記録の無い端末は `{"items":[],"nextCursor":null}`(404 にしない・他端末の存在を漏らさない。ADR-0209 §6-5)。

### 4. 保持期間・全削除・墓石・失効ジョブとの整合

- **保持期間を過ぎた行は返さない**: 失効ジョブ(日次)は `occurred_at < now − 保持期間` の行を消す。ジョブが走る前の
  最大1日分、期限切れの行が残りうるので、一覧は `occurred_at >= now − 保持期間` の行だけを返す(ちょうど境界の行は返す。
  失効ジョブが消す行と返す行がちょうど補集合になる)。保持期間は `RECORD_CALC_EVENTS_RETENTION_DAYS`(既定値を持たない。
  ADR-0211 §7)をそのまま使い、cmd/record の `runServe` が `httpapi.WithCalcEventsRetention(cfg.CalcEventsRetention)` で渡す。
  httpapi に既定の日数を持たせない(Option を渡さなければ下限なし。テストのためだけの形)。
- **全削除の後は空**: `DELETE /api/record/device-data` は calc_events を消すので、`completed` の後は空になる。
  `partial`(1回で消しきれない)の途中でも、**墓石 `devices.purged_at` 以前の行は返さない**(store の SQL で
  `occurred_at > purged_at`)。利用者が「削除」を押した直後に一部の履歴が見え続けることを防ぐ。墓石より後に発生した計算は
  返す(ADR-0209 §7 と同じ境界。`SaveCalcEvent` の Tombstoned と同じ「`<=` は捨てる」)。
- **読むだけで書かない**: 一覧は calc_events・集計・favorites のどの行も変えない(ADR-0209 §4「行ごとに last_accessed_at を
  持たない」)。保持期間は作成から数えるので、読んでも寿命は延びない。書くのは touchAndRun の `devices.last_seen_at` だけ
  (他の操作と同じ。540日の判定に使う値で、90日の calc_events には効かない)。

### 5. 個別の行の削除は持たない

requirements.md §2 は「計算するたびにイベントとして保存」と「履歴から集計」だけを求め、履歴の編集・削除を求めていない。
Web(P5-5c・ADR-0317)・iOS(ADR-0511)の画面要望にも1件の削除は無い。履歴は90日で自動的に消え、まとめて消したいときは
端末単位の全削除(ADR-0209 §5)がある。1件削除を足すと、行の ID を公開し(§2 で返さないと決めた event_id)、
集計(frequent_opponents)からの減算が要る(集計は加算と減衰しか持たない)。要望が出たら別の ADR で足す(後方互換)。

### 6. エラーと上限

| 状況 | 応答 |
|---|---|
| `limit` が範囲外(0・51・負)・整数でない | 400 `invalid_input` |
| `cursor` が空・文字が契約外・長すぎる・読めない | 400 `invalid_input`(store を呼ぶ前に弾く) |
| クエリ・ボディに端末 ID | 400 `unknown_field` |
| `X-Device-Id` が無い | 400 `missing_header`(gateway でも検証する。ADR-0202 §4) |
| record DB に届かない | 503 `store_unavailable`(ADR-0209 §5.3) |
| gateway から record-svc に届かない | 503 `upstream_unavailable`(gateway。ADR-0202) |

新しい ErrorCode は足さない(ADR-0208・ADR-0802)。応答の形は既存の `Error`。

### 7. 索引: (device_id, operation, occurred_at, event_id) を新しい版の migration で足す

既存の `idx_calc_events_device_id (device_id, occurred_at)` では、(a) `operation = 'calc'` の絞り込みで bulk/reverse の
行を読み飛ばす必要があり、(b) 同時刻の `event_id` の並びを索引で決められない(VARCHAR の主キーは TiDB の既定では
非クラスタ化で、二次索引の末尾は `_tidb_rowid` になる)。その結果、端末の全行を読んで並べ替える問い合わせになりうる。
`KEY idx_calc_events_device_history (device_id, operation, occurred_at, event_id)` を **000008** で足す
(適用済みの 000003 は書き換えない。down はこの索引だけを落とす)。spec の段階で TiDB v8.5.8(unistore)に試作の
問い合わせを流し、EXPLAIN が `IndexRangeScan`(この索引)+ `limit embedded` になることを確かめた。
既存の索引は失効ジョブ・全削除が使うので残す。書き込み1件あたりの索引の更新が1つ増えるが、個人利用の書き込み量では無視できる。

### 8. 実装の形(store と httpapi の境界)

store(`services/record/internal/store/store.go`)に足す:

```go
type CalcHistoryCursor struct {
	OccurredAt time.Time
	EventID    string
}
type CalcHistoryQuery struct {
	Since  time.Time          // occurred_at >= Since(ゼロ値なら下限なし)
	Before *CalcHistoryCursor // nil なら先頭から。(occurred_at, event_id) < (Before) の行だけ
	Limit  int
}
type CalcHistoryRow struct {
	EventID    string
	OccurredAt time.Time
	Payload    []byte // store は中身を解釈しない
}
// Store に追加。operation = 'calc'・WHERE device_id = ?・occurred_at > devices.purged_at(あれば)・Since・Before を満たす行を
// occurred_at DESC, event_id DESC で最大 Limit 行。0件は長さ0のスライス。読むだけ。届かなければ ErrUnavailable。
ListCalcHistory(ctx context.Context, deviceID string, q CalcHistoryQuery) ([]CalcHistoryRow, error)
```

httpapi(`services/record/internal/httpapi`)に足す: `type Option func(*Server)`・`WithCalcEventsRetention(d time.Duration) Option`・
`NewHandler(st store.Store, opts ...Option)`(既存の `NewHandler(st)` の呼び出しはそのまま動く)・`(*Server).ListCalcHistory`・
`registerRecordRoutes` に `e.GET("/api/record/calc-history", wrapper.ListCalcHistory)`。payload の読み替えは
`calcevents.Event` → `calcRequest`(favorites.go の受け口)→ `normalizeCalc` → `calcFrom` で、お気に入りと同じ関数を使う。

## 受け入れ条件

番号はテストのコメントと対応する。

- **AC-H1**: 1件の計算(operation = calc)だけを新しい順に返す。一括計算・逆算は載らない。`occurredAt` はイベントの時刻、
  `result` はイベントの minPercent / maxPercent そのもの
  (`httpapi/calc_history_test.go` TestCalcHistoryListsSingleCalcsNewestFirst・`store/calc_history_tidb_test.go` TestTiDBListCalcHistoryOrderAndOperationFilter)
- **AC-H2**: `calc` は同じ入力で作ったお気に入りの `calc` と JSON として一致する(既定値を補った CalcRequest)
  (TestCalcHistoryCalcIsNormalizedLikeFavorite)
- **AC-H3**: 返す項目はページ `{items, nextCursor}`・行 `{occurredAt, calc, result}`・result `{minPercent, maxPercent}` だけ。
  端末 ID・セッション ID・event_id・schemaVersion・viaRecommendation・payload の未知のキーを返さない。未知のキーがあっても 200
  (TestCalcHistoryReturnsOnlyWhitelistedFields・契約 TestContractHasCalcHistoryOperation)
- **AC-H4**: `nextCursor` をたどると全件を重複・欠落なく得る(同時刻の行をまたいでも)。最後のページは null で空のページを
  返さない(ちょうど割り切れる場合も)。ページの間に新しい行が増えても続きはずれない
  (TestCalcHistoryPagination・TestCalcHistoryCursorIsStableAgainstNewEvents・TestTiDBListCalcHistoryKeyset)
- **AC-H5**: `limit` の既定 20・有効範囲 1〜50。0・51・-1・abc・1.5 は 400 `invalid_input`(TestCalcHistoryLimit)
- **AC-H6**: 空・契約外の文字・読めない・201文字以上の `cursor` は 400 `invalid_input` で、store を呼ばない(TestCalcHistoryRejectsBadCursor)
- **AC-H7**: 他端末の行は出ない(他端末のカーソルを使っても)。store は自端末の ID でだけ呼ばれる。記録の無い端末は
  `{"items":[],"nextCursor":null}`。クエリ・ボディの端末 ID は 400 `unknown_field`、ヘッダが無ければ 400 `missing_header`
  (TestCalcHistoryIsScopedToTheDevice・TestCalcHistoryDeviceIDOnlyFromHeader・TestTiDBListCalcHistoryIsolation)
- **AC-H8**: 保持期間を過ぎた行は返さない。httpapi は Since = now − 保持期間を渡し、store は `occurred_at >= Since`
  (ちょうど境界は返す・1マイクロ秒古い行は返さない)(TestCalcHistoryRespectsRetention・TestTiDBListCalcHistorySinceBoundary)
- **AC-H9**: 全削除の後は空。`partial` の途中でも墓石以前の行は返さない。削除の後に計算した行は返す
  (TestCalcHistoryAfterDeviceDataDeletion・TestTiDBListCalcHistoryHidesRowsBeforeTombstone)
- **AC-H10**: 読むだけで calc_events・集計・last_seen_at を変えない(store)。httpapi は TouchDevice → ListCalcHistory の順に呼び、
  書き込み系を呼ばない(TestCalcHistoryIsReadOnlyAndTouchesDevice・TestTiDBListCalcHistoryIsReadOnly)
- **AC-H11**: DB に届かなければ 503 `store_unavailable`(TouchDevice の失敗なら一覧を読まない)。store は ErrUnavailable で包む
  (TestCalcHistoryStoreUnavailable・TestTiDBListCalcHistoryUnavailable)
- **AC-H12**: 読めない行は飛ばして 200、続きは正しくたどれる。ログには event_id を出し payload の中身を出さない。
  成功・失敗のどちらの経路でも個体の中身・ダメージの数値をログに出さない
  (TestCalcHistorySkipsUnreadableRows・TestCalcHistoryLogsDoNotLeakContent)
- **AC-H13**: 索引 (device_id, operation, occurred_at, event_id) を新しい版の migration で足し、000003 は書き換えず、down は
  その索引だけを落とす(`db/calc_history_index_test.go`・TestTiDBCalcEventsHasHistoryIndex)
- **AC-H14**: 応答は契約どおり(200・空・任意項目を埋めた計算・400・503・2ページ目)。契約の limit・cursor の範囲、
  items の maxItems 50、nextCursor が nullable、calc が `$ref CalcRequest`、GET 以外の操作が無いこと
  (`httpapi/calc_history_contract_test.go`)
- **AC-H15**: gateway は変更なしで `/api/record/calc-history`(クエリつき)を record-svc の上流へ送る
  (`services/gateway/internal/httpapi/record_routing_test.go` に行を追加)
- **AC-H16**: Web・iOS の生成型が上の形になる(`web/src/record/calcHistoryContract.test.ts`・
  `ios/PokeCalcKit/Tests/PokeCalcCoreTests/CalcHistoryContractTests.swift`)

## Web・iOS レーンへの依頼(画面が使う前提)

- 呼ぶ経路: `GET /api/record/calc-history?limit=20`(最初のページ)→ 続きは `&cursor=<nextCursor>`。同じ `limit` で続ける。
  `nextCursor` が null なら終わり(「もっと見る」を消す)。カーソルの中身を解釈・加工しない。
- 型: 生成型の `CalcHistoryPage` / `CalcHistoryEntry` / `CalcHistoryResult`。行の `calc` は `CalcRequest` そのもので、
  お気に入りの `calc` と同じ扱い(そのまま `POST /api/calc` の本文。ADR-0228 の「お気に入りから計算を出す」処理を流用する)。
  一覧の1行の表示は `calc.attacker.speciesKey`・`calc.defender.speciesKey`・`calc.moveId`(名前はマスタから引く)・
  `result.minPercent`〜`result.maxPercent`・`occurredAt`。行の ID は無いので、表示上のキーは配列の位置を使う。
- 行の `calc` で計算し直すと、マスタの更新で `unknown_move` 等の 400 になることがある(計算の失敗として表示する)。
- 計算した直後の行は、非同期の保存(NATS 経由)のため少し遅れて載る。画面を開いたときに読み直せば足りる(ポーリングしない)。
- エラー: 503(`store_unavailable` / `upstream_unavailable`)は「履歴を読み込めない」として一覧の場所に出し、計算画面は塞がない
  (CLAUDE.md 絶対ルール5)。400 は通常起きない(カーソルを加工しない限り)。起きたら先頭から読み直す。
- 「この端末のデータを削除」の後は履歴を読み直す(空になる)。
- 個別の行の削除・編集は無い(§5)。iOS の「契約待ち」の注記は、この API が main に入った後に外せる。
  iOS の `RequestLimits` に1ページの上限(50)を持たせるなら `ios/scripts/check-request-limits.sh` の照合に
  `CalcHistoryPage.items` を足す(任意)。

## 却下した案

- **offset/page 番号のページング**: 先頭に行が増える一覧で重複・欠落が起きる(§1)。
- **ページングなしで全件**: 90日で数万件・数 MB になりうる。
- **payload をそのまま(または detail を丸ごと)返す**: 端末 ID・セッション ID・内部項目・将来足す項目まで露出する(§2)。
- **一括計算・逆算も行として返す(入力なし)**: 選んでも計算を出し直せない行で、画面の役に立たない。
- **calc-svc のイベントを拡張して一括計算・逆算の入力も持たせる(schemaVersion 2)**: 一括計算の入力は個体1体と防御側の
  プリセット・持ち物候補の組で、1件の計算と形が違い、履歴の行の形(`calc: CalcRequest`)に収まらない。payload も数倍になる。
  要望が出たら、行に `kind` を持たせる別の ADR で足す(この契約には後方互換で足せる)。
- **イベントに含まれる結果の要約を増やす(確定数など)**: calc-svc の発行側の変更(schemaVersion の扱い)が要り、
  一覧の1行の表示には表示%で足りる。
- **保持期間の下限を httpapi に既定値(90日)として持たせる**: 日数を設定とコードの2か所に持つことになる(ADR-0211 §7)。
- **墓石を見ずに calc_events を読むだけ**: `partial` の途中で削除済みのはずの行が見え続ける(§4)。

## 影響

- `api/openapi.yaml` に `listCalcHistory` と4つの schema(`CalcHistoryCursor`・`CalcHistoryResult`・`CalcHistoryEntry`・
  `CalcHistoryPage`)を追加。`make gen` の後、`api.ServerInterface` に `ListCalcHistory` が増えるので、calc-svc・pokedex-svc
  (`record_notfound.go`)・team-svc(`stubs.go`)にも 404 のスタブが要る(実装者)。
- record-svc の `store.Store` にメソッドが増えるので、`events/consumer_test.go` の fake にも足す(実装者。テストを弱めない)。
- migration 000008(索引)を足す。k3d / クラウドの record DB には migrate ジョブで当たる。
- gateway は変更なし(前方一致の既存ルート)。

## 未決事項

- 一括計算・逆算の履歴(要望が出たら別 ADR)。
- 履歴の1件削除(要望が出たら別 ADR。集計からの減算の設計が要る)。
- カーソルに含まれる event_id から全端末の計算回数が推測できる点(§1。公開範囲を広げるときに見直す)。
