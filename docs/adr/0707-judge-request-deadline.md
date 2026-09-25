# ADR-0707: 判定 1 リクエスト全体の期限(`JUDGE_REQUEST_TIMEOUT`)

- 状態: 採用(2026-09-25)
- 日付: 2026-09-25
- 関連: ADR-0700 §1(上流のタイムアウト`JUDGE_UPSTREAM_TIMEOUT`・「設定されているのに不正なら起動を失敗させる」立場)、
  ADR-0700 §2(1 回の呼び出しは request スコープの `context` と設定のタイムアウトのうち早い方で打ち切る)、
  ADR-0700 §3(上流のエラーの正規化・`ErrUpstreamUnavailable` が呼び出し側の ctx 終了も含む)、
  ADR-0703 §3(index 昇順で最初に失敗した候補で打ち切る、逐次動作)、issue #213(全体レビュー指摘。2026-09-25)

## 背景

`POST /api/judge/v1/outspeed-and-ko` は、性格 1 回 + 種族 (1+N) 回 + 技 (1+N) 回 + calc 2N 回
(N = defenders の件数、最大 6)を**逐次**に上流へ呼ぶ(`outspeedAndKo`。ADR-0703 §4・ADR-0704 §5)。
1 回ごとの呼び出しは `JUDGE_UPSTREAM_TIMEOUT`(既定 3 秒。ADR-0700 §1)で打ち切られるが、**リクエスト全体の期限が無い**。
上流が「遅いが完全には止まっていない」(例: 1 回 1 秒で応答する)とき、1 回ごとのタイムアウトは 1 回も発動しないまま、
N=6 なら最大 26 回の呼び出しが律儀に完走し、応答まで約 26 秒かかる。一方 `http.Server.WriteTimeout`
(15 秒。`services/judge/cmd/api/main.go`)は期限後の書き込みを失敗させるだけでハンドラの goroutine を止めないため、
クライアントは JSON のエラーではなく空応答(HTTP 000 / Empty reply)を受け取る。

決めるべきものは、**(a)** リクエスト全体の期限をどう表す設定にするか、**(b)** どこでその期限を作り、
どう上流呼び出しへ伝えるか、**(c)** それだけで「進行中の呼び出しを打ち切る」ことと「まだ始めていない呼び出しを
行わない」ことの両方が成立するか(`internal/client` に追加の変更が要るか)、の 3 つ。

## 決定

### 1. `JUDGE_REQUEST_TIMEOUT`(既定 12 秒)を新設し、`writeTimeout` 未満であることを起動時に検証する

`services/judge/cmd/api/config.go` に `requestTimeoutEnv = "JUDGE_REQUEST_TIMEOUT"` ・
`defaultRequestTimeout = 12 * time.Second` を、`upstreamTimeoutEnv` / `defaultUpstreamTimeout`
(ADR-0700 §1)と同じ形で置く。読み取り関数は

```go
func requestTimeoutFromEnv(lookup func(string) (string, bool), writeTimeout time.Duration) (time.Duration, error)
```

とし、`upstreamTimeoutFromEnv` と同じ「未設定・空文字は既定値、`time.ParseDuration` に失敗、
または 0 以下は起動失敗」に加えて、**`writeTimeout` 以上なら起動失敗**にする。

- 12 秒という既定は、issue #213 の既定案をそのまま採用する。`http.Server.WriteTimeout`
  (15 秒。`cmd/api/main.go` の `writeTimeout`)より確実に短く、`ReadTimeout`(10 秒)より長い
  (request 全体には body の読み取り時間も既に含まれているため、`ReadTimeout` より短いと
  「body を読み終える前に判定の期限が来る」という本末転倒が起きうる。12 秒はその中間)。
- **起動時に `requestTimeout >= writeTimeout` を拒否する**のは、ADR-0700 §1 の
  「設定されているのに不正(scheme・ホスト・タイムアウト)なら起動を失敗させる」立場をそのまま引き継ぐ。
  この関係が崩れると、判定の期限が来る前に `WriteTimeout` が本文の書き込みを失敗させてしまい、
  この ADR が直そうとしている「空応答」がそっくり復活する。起動時に気づける方が、
  本番で `JUDGE_REQUEST_TIMEOUT` を大きくしすぎたときにすぐ分かる。
- `writeTimeout` を関数の引数として明示的に渡す(`main.go` の定数を `config.go` が黙って前提にしない)。
  `config.go` 単体でテストでき、将来 `writeTimeout` が環境変数化されても呼び出し側を直すだけで済む。

### 2. ハンドラの先頭で ctx を 1 回だけラップし、以降のすべての上流呼び出しに使う

`services/judge/internal/httpapi/server.go` の `Dependencies` に `RequestTimeout time.Duration` を足す。
`outspeedAndKo`(`outspeed.go`)の `ctx := c.Request().Context()` を、

```go
ctx := c.Request().Context()
if deps.RequestTimeout > 0 {
    var cancel context.CancelFunc
    ctx, cancel = context.WithTimeout(ctx, deps.RequestTimeout)
    defer cancel()
}
```

に置き換える。以降の `deps.Pokedex.Natures/Species/Move` ・ `deps.Calc.Damage` はすべて今まで通りこの
`ctx` を受け取る(呼び出し順序・逐次であること自体は変えない。ADR-0703 §3 の維持)。

- **`RequestTimeout` が 0(既定値)なら期限を張らない**。`cmd/api/main.go` は必ず
  `requestTimeoutFromEnv` の結果(既定 12 秒。正の値)を渡すので本番では常に期限が張られるが、
  `internal/httpapi` の既存テスト(`newUpstreams` が組み立てる `Dependencies` は `RequestTimeout` を
  指定していない)はこのフィールドを一切知らなくても今まで通り動く。**後方互換のための唯一のスイッチ**
  であり、「期限を無効化する」設定項目として運用者に公開するものではない(README には
  `JUDGE_REQUEST_TIMEOUT` の既定と検証だけを書き、「0 で無効化できる」とは書かない)。

### 3. `internal/client` は変更不要(§4 で検証した理由による)

`internal/client` の `Species` / `Move` / `Natures` / `Damage` は既に `http.NewRequestWithContext(ctx, ...)`
でリクエストを組み立て、`send()` はその `req` をそのまま `httpClient.Do(req)` に渡している
(ADR-0700 §2「1 回の呼び出しは request スコープの `context` と設定のタイムアウトのうち早い方で打ち切る」の
実装そのもの)。Go の `net/http` は、`http.NewRequestWithContext` で結び付けた `context` が
**キャンセルされると進行中の呼び出しを打ち切り**(`Client.Do` が `ctx.Err()` を包んだエラーで返る)、
**既に終わっている `context` で `Do` を呼ぶと新しい接続を張らずに即座にエラーで返す**
(`net/http` の `context` 統合の仕様。`http.NewRequestWithContext` のドキュメントが明記する動作)。

したがって §2 のラップだけで、期限が来た時点で

1. **その時点で進行中の呼び出しは中断される**(`Client.Do` が早期に返る)。
2. **まだ始めていない呼び出しは、次のループでの `Do` 呼び出しが即座に失敗して先に進む**
   (実際に TCP 接続を張らない。上流の偽サーバーは新しいリクエストを 1 件も受け取らない)。

の両方が自動的に成立する。`send()` は元々 `ctx.Err()` を見て `ErrUpstreamUnavailable` に包む分岐
(ADR-0700 §3)を持っているため、この経路は変更なしでそのまま使える。`writeUpstreamError` /
`writeSpeciesError` / `writeMoveError` / `writeCalcError` の呼び出しエラーの畳み込みも変更しない。

- **§3 の検証**: `services/judge/internal/httpapi/outspeed_deadline_test.go`
  (このコミットで追加。`TestOutspeedAndKoOverallDeadline`)が、遅い偽上流 × 6 候補に対して
  「期限内に 503 が返ること」と「上流の呼び出し回数が全件完走(27 回)よりずっと少ないこと」を
  実測で確かめる。これが緑になれば §3 の結論(`internal/client` は変更不要)が実証されたことになる。
  実装段階でこのテストが `internal/client` の変更なしに緑にならない場合、この ADR の §3 の前提が
  誤っていたことになるので、そのときは改めて ADR を追記する。

- **`api/openapi.yaml` は変更しない**: 503 `upstream_unavailable` の description(既に「pokedex-svc /
  calc-svc が未設定・接続できない・タイムアウト・5xx・契約に合わない応答」を含む)・ステータスコード・
  エラーコード・JSON スキーマはこの変更で一切変わらない。外部から観測できる差分は「503 が返るまでの
  時間の上限が下がる(体感で速くなる)」だけで、既存の契約が想定する応答の範囲内。よって
  `make judge-gen` は不要と判断した(見落としではなく明示の判断)。

### 4. クライアント切断の伝播は今回の変更に含まれない(既に成立している)

`ctx := c.Request().Context()` は元から (JD0 から) 上流呼び出しに素通しされており、
「呼び出し元の ctx が終わっている(クライアントが切断した)」場合も ADR-0700 §3 の
`ErrUpstreamUnavailable` 分岐に既に乗っている。この ADR は「サーバー自身が期限を持つ」ことを足すだけで、
クライアント切断の伝播経路そのものは変えない。issue #213 の受け入れ条件が挙げる
「クライアント切断時も同様」は、§2 のラップ後も `context.WithTimeout` は親 `ctx`(切断で終わる方)を
包むだけなので、そのまま成立し続ける。

`internal/httpapi` は `httptest.NewRecorder()` 経由(`ServeHTTP` を直接呼ぶ)でテストしており、
実際の TCP 接続を張らないため「クライアントが切断した」を `httptest.NewRequest` の `context` に
再現できない(`net/http` が接続クローズで `context` を終わらせる仕組みは実サーバー・実接続が前提)。
このため、クライアント切断の伝播そのものを検査する新しいテストはこの ADR では追加しない
(§2 のコード変更が無いので壊れようがなく、退行テストとしての価値も薄い)。実接続を使う検証が
要るなら `services/judge/scripts/smoke.sh` 側の課題とし、この ADR のスコープには含めない。

### 5. 候補ごとの並列化・リトライはスコープ外

ADR-0703 §3 の「index 昇順で最初に失敗した候補で打ち切る」逐次動作は変えない。この ADR は
「逐次呼び出しの合計に上限を付ける」だけで、呼び出し順序や並列化には触れない。issue #213 の
「変更範囲/対象外」がそう定めている。リトライ(ADR-0700 §2 で JD0 から意図的に持たない)も同様に増やさない。

## 受け入れ条件

1. **正常系不変**: 上流が速いとき(`JUDGE_REQUEST_TIMEOUT` の期限内に全呼び出しが終わるとき)の応答・
   ステータス・上流への呼び出し回数・呼び出し順序は 1 つも変わらない(既存の `internal/httpapi` テスト全件が緑のまま)。
2. `JUDGE_REQUEST_TIMEOUT`(既定 12 秒)がリクエスト全体の期限になり、上流の合計所要時間がこれを超えたら、
   **期限内に** 503 `upstream_unavailable` の JSON を返す(クライアントが空応答を受け取らない)。
3. 期限超過の時点で、**まだ発行していない上流呼び出しを行わない**。6 候補・遅い偽上流に対する回帰テストで、
   実際の呼び出し回数が「全件完走したときの回数」よりずっと少ないことを確かめる。
4. `JUDGE_REQUEST_TIMEOUT` が `writeTimeout`(15 秒)以上のとき、judge は起動しない。
5. `deps.RequestTimeout` が 0(未指定)のときは期限を張らない(`internal/httpapi` の既存テストの挙動を変えない)。
6. `internal/client` のコードは変更しない(§3 の検証テストで実証する)。

## テストの期待値

- `services/judge/internal/httpapi/outspeed_deadline_test.go`
  - `TestOutspeedAndKoOverallDeadline`: `upstreams.delay`(このコミットで `outspeed_test.go` に追加する
    テスト専用のフィールド)で pokedex-svc・calc-svc の応答を一律に遅らせ、defenders 6 件(ADR-0703 §1 の上限)の
    request を送る。`deps.RequestTimeout` を実時間で短い値(数百 ms)に設定し、実際の待ち時間も数百 ms に抑える
    (12 秒既定をそのまま使って本当に 12 秒待つテストは書かない)。
    - 応答が 503 `upstream_unavailable` であること。
    - 応答までの実時間が「全件完走したときの所要時間」よりずっと短いこと。
    - 記録された上流呼び出し回数の合計(natures + species + moves + calc)が、全件完走時の回数
      (natures 1 + species (1+6) + moves (1+6) + calc 2×6 = 27)よりずっと少ないこと。特に calc-svc は
      1 回も呼ばれない(期限は species/moves の途中で尽きる想定の delay・timeout を選ぶ)。
  - `upstreams` の delay は `r.Context().Done()` も見て早期に返る(`internal/client/client_test.go` の
    `blockingServer` と同じ考え方)。ゼロ値(未指定)では即座に応答するので、この 1 フィールドを足しても
    既存の全テストの挙動は変わらない。
- `services/judge/cmd/api/config_test.go`
  - `TestRequestTimeoutFromEnv`: `TestUpstreamTimeoutFromEnv` と同じ形(未設定/空文字は既定、
    duration でない/0/負は起動失敗)に加えて、`writeTimeout` 以上(境界の一致・超過の両方)を拒否すること、
    `writeTimeout` 未満の境界値(1ms 未満の差)は通ることを確認する。`writeTimeout` を関数の引数として
    複数の値で試し、`main.go` の実際の定数(15 秒)に固定した検証ではないことも確かめる。
- `make judge-test` が緑になることが実装段階の完了条件。ADR-0700〜0706 と同じく、judge は計算式を持たないので
  `make test-golden` の対象は増えない。

## 却下した案

- **候補ごとの並列化(goroutine で species/moves/calc を並行に呼ぶ)**: issue #213 の「変更範囲/対象外」が
  明示的に除外している。ADR-0703 §3 の「index 昇順で最初に失敗した候補で打ち切る」という順序保証を
  壊さずに並列化するのは別途の設計判断が要り、この ADR のスコープを超える。
- **`internal/client.send()` 自身にリクエスト全体用のタイマーを持たせる**: `send()` は 1 回の呼び出しの
  ことしか知らず、「リクエスト全体で何回目か」を意識させると ADR-0700 §4 の「`internal/client` は
  judge が読む欄だけの小さな型を持つ薄い層」という立場と衝突する。§3 の通り、呼び出し元
  (`outspeedAndKo`)が 1 回だけ `context.WithTimeout` でラップすれば `send()` の変更なしで足りる。
- **`http.Server.WriteTimeout` を伸ばす**: そもそもの問題(ハンドラの goroutine が止まらない)を隠すだけで、
  上流が遅い間ずっとクライアントを待たせ続ける点は変わらない。むしろ「いつまで待たされるか分からない」を
  「12 秒待たされたら確実に JSON が返る」に変えるのがこの ADR の目的そのもの。
- **`JUDGE_REQUEST_TIMEOUT` と `writeTimeout` の関係を検証せず、README に注意書きするだけ**: ADR-0700 §1 の
  「設定されているのに不正なら起動を失敗させる」という既存の一貫した立場と食い違う。誤設定が本番で
  気づかれず、この ADR が直そうとした症状(空応答)がそのまま再発する経路を残してしまう。
- **`deps.RequestTimeout` を必須(0 を許さない)にする**: 0 を「期限なし」として許すことで、
  `internal/httpapi` の既存の全テスト(`Dependencies` を直接組み立てるもの)を 1 つも書き換えずに済む。
  必須にすると、この ADR に無関係な既存テストまで `RequestTimeout` の指定を強いられ、
  無関係な変更の範囲が広がる。
