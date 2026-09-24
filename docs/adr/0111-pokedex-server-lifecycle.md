# ADR-0111: pokedex-svc に HTTP タイムアウトと graceful shutdown を追加する(issue #109)

- 状態: 採用(issue #109 の仕様。spec-writer 起草、implementer が実装、critic PASS。実クラスタで検証済み)
- 日付: 2026-09-24
- レーン: データ(ADR 帯 `0100〜`)
- 関連: issue #109、ADR-0105(pokedex-svc の起動処理)、CLAUDE.md 絶対ルール6

## 背景

`services/pokedex/cmd/pokedex/main.go` の `runServe` は `http.ListenAndServe(cfg.Addr, handler)` を
直接呼ぶだけで、タイムアウト(`ReadHeaderTimeout`・`ReadTimeout`・`WriteTimeout`・`IdleTimeout`・
`MaxHeaderBytes`)を一切設定しておらず、SIGINT/SIGTERM も購読しない。同じリポジトリの
`services/calc`・`services/gateway`・`services/balance`・`services/judge` はいずれも明示的な
`http.Server` とタイムアウト、`signal.NotifyContext` による graceful shutdown を持っており、
pokedex-svc だけがこの運用契約から外れている。

遅い・不完全な接続がリソースを無期限に保持しうる(可用性の低下)ことに加え、Kubernetes の
rollout・node drain・Pod 再配置のたびに処理中のリクエスト(検索・内部 master export)が
即座に打ち切られる。

## 決定

### 1. `services/balance`・`services/judge` の lifecycle をそのまま踏襲する

`services/calc`・`services/gateway` は `shutdownTimeout` を定数にして `select`/goroutine で
組み立てる、より古い形。`services/balance`・`services/judge` はそれに加えて `MaxHeaderBytes` も
持つ、より新しく完全な形。issue の受け入れ条件が `MaxHeaderBytes` を明示的に要求しているため、
**`services/balance/cmd/api/main.go` の形を手本にする**(値も含めてほぼ同じにする。issue 自身が
「サービス間の共通化リファクタリングはこの Issue に含めない」と述べているため、共通パッケージへの
抽出はしない。単純に pokedex にも同じ形を書き写す)。

| 定数 | 値 | 出典 |
|---|---|---|
| `readHeaderTimeout` | 5秒 | balance/judge と同じ |
| `readTimeout` | 10秒 | balance/judge と同じ |
| `writeTimeout` | 15秒 | balance/judge と同じ(内部 master export の応答本文が大きくても15秒あれば十分。ADR-0105 の実測ではミリ秒オーダー) |
| `idleTimeout` | 60秒 | balance/judge と同じ |
| `maxHeaderBytes` | 16KiB | balance/judge と同じ |
| shutdown timeout | 10秒 | issue の既定案どおり。balance と同じ値 |

### 2. `runServe` をテスト可能な `run(ctx, lookup) error` に分離する(`services/calc` と同じ形)

現状の `runServe() int` は `os.Exit` 前提でテストしにくい。`services/calc/cmd/calc/main.go` の
`run(ctx context.Context, lookup func(string) (string, bool)) error` と同じ形にし、
`main`(`signal.NotifyContext` で ctx を作る)→`run`(サーバを起動し ctx の終了を待つ)→
`runServe`(戻り値をエラーメッセージと終了コードに変換するだけの薄い層)の3層にする。

`run` の中身(`services/balance`・`services/calc` を合成した形):

1. `http.Server` を決定1の値で組み立てる。
2. `signal.NotifyContext` で作った ctx を `main` から受け取る(`run` 自身は signal を購読しない。
   `services/calc` と同じくテストから任意の ctx を渡せるようにするため)。
3. `ListenAndServe` を goroutine で走らせ、エラーをチャネルで受け取る。
4. `select` で「serve が先に終わった(起動失敗)」か「ctx が終わった(シャットダウン要求)」かを見る。
5. ctx が終わったら `context.WithTimeout(context.Background(), shutdownTimeout)` で `Shutdown` を呼び、
   goroutine の終了を待つ。
6. `http.ErrServerClosed` は正常終了として扱う(`errors.Is`)。それ以外の serve/shutdown エラーは
   `%w` でそのまま返す(DSN・パスワードを含まない。`http.Server`/`net`のエラーはそもそも
   接続情報を含まないため、写す際に追加のマスクは不要)。

### 3. `terminationGracePeriodSeconds` を明示し、shutdown timeout より長いことをテストで固定する

Kubernetes の既定値(30秒)は決定1の shutdown timeout(10秒)より長いが、暗黙の既定値に頼ると
将来どちらかの値だけが変わったときに壊れる。`deploy/k8s/base/pokedex/deployment.yaml` の
`spec.template.spec` に `terminationGracePeriodSeconds: 30` を明示し、
`services/pokedex/cmd/pokedex/manifest_test.go` にこの値と `main.go` の `shutdownTimeout` 定数を
比較するテストを足す(値を2箇所にハードコードする代わりに、テストが不等式
`terminationGracePeriodSeconds > shutdownTimeout` を検査することで、どちらかを変更したときに
検知できるようにする)。

却下: 何も書かず Kubernetes の既定値(30秒)に頼る — 検査のしようがなく、issue の受け入れ条件
「manifest テストで保証する」を満たせない。

### 4. 対象は `serve` だけ。`export` サブコマンドは変更しない

`pokedex export` は CLI として1回実行して終わるバッチ処理で、HTTP サーバーを持たない
(issue の変更範囲が「pokedex の起動処理」と言っているのは `serve` を指す)。

## 受け入れ条件

(「実装時の申し送り」で導入する識別子 `newHTTPServer`・`serve`・`runServe` を前提にした、
テスト可能な形。`main_test.go` の対応するテスト名を併記する。)

1. `newHTTPServer(addr, handler)` が返す `*http.Server` が `ReadHeaderTimeout=5s`・
   `ReadTimeout=10s`・`WriteTimeout=15s`・`IdleTimeout=60s`・`MaxHeaderBytes=16KiB`
   (決定1の値そのもの)を持つ。`shutdownTimeout` 定数は10秒。
   (`TestHTTPServerTimeouts`・`TestShutdownTimeoutValue`)
2. `serve(ctx, addr, handler)` は `ctx` の終了(SIGINT/SIGTERM 相当)を受けて `Shutdown` を呼び、
   エラー無く(nil で)戻る。`runServe(ctx, lookup)` も同様に、`/healthz` へ応答できる状態から
   `ctx` の終了で正常に停止する。
   (`TestServeStopsOnContextCancel`・`TestRunServeStopsOnContextCancel`)
3. ヘッダを送り切らない TCP 接続が `ReadHeaderTimeout` の直後(httptest ではなく生 TCP 接続で検証)
   で切断される。(`TestServeClosesConnectionMissingHeaders`)
4. shutdown 開始(ctx キャンセル)後も、既に受理した短い(タイムアウトより十分短い)in-flight
   リクエストは完了してから `serve` が停止する(意図的に遅いテスト用ハンドラで検証する)。
   (`TestServeWaitsForInFlightRequestOnShutdown`)
5. `http.ErrServerClosed` は成功として扱い、それ以外の serve/shutdown エラー(例: アドレス使用中)
   は非nilで返り(`errors.Is(err, http.ErrServerClosed)` が false)、非0終了の元になる。エラー文に
   DSN・パスワード(`pass@`)を含まない。
   (`TestServeReturnsErrorWhenAddrInUse`・`TestRunServeReturnsErrorWhenAddrInUse`・
   `TestRunServeFailsOnInvalidConfig`)
6. `deployment.yaml` の `terminationGracePeriodSeconds` が明示され、`main.go` の `shutdownTimeout`
   定数より長いことを、値を2箇所にハードコードせず不等式で比較する manifest テストで固定する。
   (`TestPokedexTerminationGracePeriodExceedsShutdownTimeout`)
7. `make test`・`make lint`・`make build`・pokedex の k3d smoke(`make api-smoke`)が成功する。

## 影響

- 変更: `services/pokedex/cmd/pokedex/main.go`(`run`/`runServe` の分離とサーバ構成)、
  `services/pokedex/cmd/pokedex/main_test.go`・`manifest_test.go`、
  `deploy/k8s/base/pokedex/deployment.yaml`(`terminationGracePeriodSeconds` の追加)。
- 変更しない: HTTP パス・公開 OpenAPI・DB クエリ・マスタ内容(issue の宣言どおり)、
  `pokedex export` サブコマンド、readiness/liveness probe(issue #107 の範囲)。

## 実装時の申し送り(spec-writer 追記。2026-09-24)

受け入れ条件とテストを先に書く過程で、決定2の原文どおりには実装できない名前衝突と、
決定を裏付けるテストが書けない箇所を見つけたため、ここで補う。critic のレビュー対象に含めること。

### 1. 名前衝突: `run` は既に別の意味で使われている

`services/pokedex/cmd/pokedex/main.go` には既に `func run(args []string) int`
(`serve`/`export` のサブコマンド振り分け。`main()` から `os.Exit(run(os.Args[1:]))` で呼ばれる)が
存在する。決定2の原文がそのまま指す「`run(ctx context.Context, lookup func(string) (string, bool)) error`」
を同じ名前で追加すると衝突する(calc-svc・gateway にはサブコマンドが無いため、この衝突が起きない)。

**解消**: ctx 版の関数は `runServe(ctx context.Context, lookup func(string) (string, bool)) error`
と改名する(現行の `func runServe() int` を置き換える形)。サブコマンド振り分けの `run(args []string) int`
はそのまま残す。`run` の `"serve"` ケースは、`signal.NotifyContext` で ctx を作り `runServe(ctx, lookupEnv)`
を呼んでエラーを終了コードに変換する薄いラッパー(仮称 `runServeCmd() int`。実装者が命名してよい)を
経由する。3層の対応は次のとおり:

| 決定2原文の層 | pokedex での実体 |
|---|---|
| `main`(signal.NotifyContext で ctx を作る) | `run(args)` の `"serve"` ケースが担う(または新設する薄いラッパー) |
| `run(ctx, lookup) error`(テストから ctx を注入できる核) | `runServe(ctx, lookup) error` |
| `runServe`(戻り値をエラーメッセージと終了コードに変換する薄い層) | 上記の新設ラッパー(`run(args)` の `"serve"` ケース内、または `runServeCmd() int`) |

### 2. `runServe` の中をさらに2段に割る(テスト容易性のため。決定2を補う)

決定2の「`run` の中身」をそのまま1関数に書くと、AC3(ヘッダ未完了接続の切断)と AC4(shutdown 中の
in-flight 完了)を検証する術がない。`httptest` はネットワークタイムアウトを再現できず、かつ pokedex の
本番ハンドラ(`httpapi.NewHandler`)は DB(`store.Querier`)前提のため、意図的に遅いハンドラを注入する
すべが無いと AC4 を検証できない(`storetest.Querier` を薄いラッパーで包んで遅延させる手も検討したが、
本番のルーティング・DB 層を経由する分テストが重く壊れやすくなるため採らなかった)。

**追加する分離**(`runServe` の内部だけの話。外部シグネチャ・決定1の値は変えない):

- `newHTTPServer(addr string, handler http.Handler) *http.Server` — 決定1のタイムアウト値を持つ
  `*http.Server` を組み立てるだけの純粋関数。DB・設定を一切知らない。
- `serve(ctx context.Context, addr string, handler http.Handler) error` — `ListenAndServe` を
  goroutine で走らせ、ctx 終了で `shutdownTimeout` 付きの `Shutdown` を呼ぶ(決定2 手順3〜6と同じ)。
  DB・DSN を一切知らない。
- `runServe(ctx, lookup) error` は `loadConfig` → `sql.Open` → `httpapi.NewHandler(store.New(db))` の後、
  最後に `return serve(ctx, cfg.Addr, handler)` を呼ぶだけにする。

この分離により、spec-writer が書いたテスト(`main_test.go`)は `serve` に任意の `http.Handler`
(意図的に `time.Sleep` するテスト用ハンドラや `http.NotFoundHandler()`)を渡して、DB 無しで
AC1・AC3・AC4・AC5 を直接検証できる。`runServe` の結線自体は `/healthz`(DB に触れない)を使った
別の統合テストで確認する(`TestRunServeStopsOnContextCancel` 等)。

### 3. shutdown timeout は10秒(calc-svc・gateway の5秒と混同しないこと)

決定1の表どおり10秒。calc-svc・gateway の `shutdownTimeout`(5秒)をコピーしないよう注意。

### 4. serve/runServe のエラーに DSN が混ざらないことは構造で保証する

上記の分離により `serve` は `cfg`/DSN を一切参照しないため、`serve` が返すエラーに DSN が
混ざることは構造的にありえない。`runServe` 内の `loadConfig`・`sql.Open` のエラーは既存の
`loadConfig` のエラー文言(DSN を含めない実装のまま。`main_test.go` の `TestLoadConfig` が既に検査)
をそのまま使う。追加のマスク処理は不要という原文の記述は、この分離を前提にするとより確実になる。

### 5. 追加したテストが前提にする識別子

`services/pokedex/cmd/pokedex/main_test.go`(追記)と `manifest_test.go`(追記)は、実装者が
上記の名前で次を用意することを前提にしている。用意するまでコンパイルが通らない(意図的):

- 定数: `readHeaderTimeout`・`readTimeout`・`writeTimeout`・`idleTimeout`・`maxHeaderBytes`・
  `shutdownTimeout`(いずれも決定1の値)
- `newHTTPServer(addr string, handler http.Handler) *http.Server`
- `serve(ctx context.Context, addr string, handler http.Handler) error`
- `runServe(ctx context.Context, lookup func(string) (string, bool)) error`
