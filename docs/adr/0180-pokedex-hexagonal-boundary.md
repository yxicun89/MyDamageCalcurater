# ADR-0180: pokedex-svc をヘキサゴナルの形にそろえる(検索・内部 API・readiness・read model)

- 状態: 採用(2026-10-12。実装済み)
- 日付: 2026-10-12
- レーン: データ
- 関連: ADR-0811(ヘキサゴナルの採用)、ADR-0252(calc の同じ移行。流儀の先行例)、ADR-0105(pokedex-svc の境界・storetest)、
  ADR-0127(読み取り専用スナップショット)、ADR-0128(dataVersion)、ADR-0129(readiness)、ADR-0807(生成物を Git に置かない)、
  docs/plan/architecture-adoption.md §6 S5・§2.3 G7

## 背景

S5(docs/plan/architecture-adoption.md §6)で pokedex-svc を `internal/domain`・`internal/app`・`internal/adapters` にそろえる。
G7(HTTP 層が sqlc を直接呼び、検索の組み立てもハンドラにある)を解消する。テストの偽の DB(`storetest`)は sqlc の
`store.Querier` を実装し、呼ばれたメソッド・引数・トランザクションの内外を記録して確かめているので、DB の呼び出しの列を
変えずに手順を app に移す必要がある。importer(`services/pokedex/importer`)は対象外(§8 Q6)で、sqlc の生成物を使い続ける。

## 決定

1. **sqlc の生成物 `internal/store` と、その偽物 `internal/storetest` は場所を変えない**。生成先(Makefile の gen-sql・
   `scripts/ensure-gen.sh`・`.gitignore`・`check-publishable.sh`)と importer の import を動かさずに済むため(§8 Q8 と同じ考え方)。
   `internal/store` は oapi-codegen の `internal/api` と同じく外側として扱い、domain・app は import しない
   (archcheck は同じサービスの store を見ないので、`app/service_test.go` の `TestInnerLayersDoNotImportStore` で固定する)。
   storetest の import 検査(ADR-0105)はパスが変わらないので、そのまま有効。
2. **DB のポートは app で宣言し、メソッドは sqlc のクエリと 1 対 1**(`app.Reader`。スナップショットは `app.Snapshot`・
   `app.SnapshotBeginner`、readiness 用の `BeginReadOnly` を持つ `app.Store`)。値は app の型(NULL は nil)。
   まとめたり足したりしないので、偽の DB が見る呼び出しの列・引数・分離レベルは移行の前と同じ。
   実装は `adapters/store`(sqlc を `adapters/readtx` 経由で呼び、行を写す)。1 行を引く読み出しの `sql.ErrNoRows` は
   `app.ErrNotFound` でも判定できるように包み、文面は元のまま(ログに出る値を変えない)。
3. **失敗の語彙は `domain.Code`(契約の ErrorCode と同じ文字列)と `domain.Error`**(ADR-0252 と同じ)。master_unavailable は
   `Reason`(ログの理由)と `Cause`(元の失敗)を持ち、adapters/httpapi の `fail` が今までと同じ文面でログに出して 503 にする。
   format の列挙が契約に含まれるかは adapters が生成型の `Valid()` で判定し、`domain.Enum` で渡す。
4. **置き場所**: 入力の検証・LIKE のパターン・効果定義を字面のまま読むこと・dataVersion(旧 `internal/dataversion`)・
   read model の特性の効果の正規化は `domain`。検索 8 操作・内部 API のマスタ一式・readiness・read model の組み立ては `app`。
   `httpapi`・`readtx`・`readmodel`(DB の口の用意とファイルの書き出し)は `adapters` の下。`readmodel.Export(ctx, readtx.Beginner)` と
   `httpapi.NewHandler(readtx.DB, ...)` の形は変えず、cmd・importer のテストの呼び方を保つ(adapters が app と adapters/store を組む。
   calc の `httpapi.NewServer` が `app.New` を組むのと同じ)。

## 影響

- `services/pokedex/internal/{httpapi,readtx,readmodel}` は `internal/adapters/` の下へ、`internal/dataversion` は `internal/domain` へ移った。
  cmd・importer・db の(`-tags mysql` を含む)テストの import と、read model のテストの相対パス(1 段深く)を追従させた。
- pokedex は tools/archcheck の未移行の一覧から外れ、向きの検査の対象になった。
- 振る舞い(openapi.yaml・応答・エラーの code とステータス・ログ・export の出力・DB の呼び出しの列)は変えない。
- importer を同じ形にするかは S5 の後で判断する(§8 Q6)。
