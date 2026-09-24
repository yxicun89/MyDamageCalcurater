# ADR-0112: pokedex の DB 接続プールに上限と寿命を設定する(issue #112)

- 状態: 採用(issue #112 の仕様。spec-writer 起草、implementer が実装、critic PASS。実クラスタで検証済み)
- 日付: 2026-09-24
- レーン: データ(ADR 帯 `0100〜`)
- 関連: issue #112、ADR-0105(pokedex-svc の起動処理)、ADR-0110(用途別最小権限。同じ
  `services/pokedex/cmd/pokedex/main.go`/`db`パッケージ周りの運用強化)、ADR-0111(HTTP
  タイムアウト・graceful shutdown。同じ流れの3件目)、CLAUDE.md 絶対ルール6

## 背景

`services/pokedex/cmd/pokedex/main.go` は `sql.Open("mysql", cfg.DSN)` の直後、
`SetMaxOpenConns`・`SetMaxIdleConns`・`SetConnMaxIdleTime`・`SetConnMaxLifetime` を
一度も呼ばずにハンドラへ渡す。Go 標準ライブラリの既定は `MaxOpenConns=0`(無制限)・
`MaxIdleConns=2`・寿命は年齢で閉じない。突発的な同時要求がそのまま新規 MySQL 接続要求になり、
1 Pod でも DB 側の接続枠を占有しうる。将来 replica を増やすと「Pod 数 × 無制限」になり、
importer・migrate・運用接続まで巻き込んで失敗させうる。

## 決定

### 1. 4つの環境変数でプール設定を持つ(issue の既定案どおりの値)

| 環境変数 | 既定値 | `sql.DB` のメソッド |
|---|---|---|
| `POKEDEX_DB_MAX_OPEN_CONNS` | 10 | `SetMaxOpenConns` |
| `POKEDEX_DB_MAX_IDLE_CONNS` | 5 | `SetMaxIdleConns` |
| `POKEDEX_DB_CONN_MAX_IDLE_TIME` | 5m | `SetConnMaxIdleTime` |
| `POKEDEX_DB_CONN_MAX_LIFETIME` | 30m | `SetConnMaxLifetime` |

整数2つ(`strconv.Atoi`)・`time.Duration` 文字列2つ(`time.ParseDuration`。Go の標準形式
`5m`・`30m` 等)として読む。

### 2. 検証は `sql.Open` より前(接続を開く前)に行う。DSN を含めない

`loadConfig` に検証を足す(`services/pokedex/cmd/pokedex/main.go` の既存の DSN 検証と
同じ関数・同じタイミング):

- `MaxOpenConns` は正の整数(`> 0`)。
- `MaxIdleConns` は正の整数(`> 0`)かつ `MaxOpenConns` 以下。
- `ConnMaxIdleTime`・`ConnMaxLifetime` は正の `time.Duration`(`> 0`)。
- エラー文には変数**名**だけを含め、値・DSN は含めない(既存の `loadConfig` の流儀と同じ)。

却下: `sql.Open` 後に検証する — issue の受け入れ条件が「起動前に失敗する」ことを明示しており、
DB に一切触れない設定エラーは DB を開く前に弾くべき(`sql.Open` 自体は接続しないので
今回は実害が無いが、検証の位置を「入力を読んだ直後」に揃えておくのが既存の流儀に合う)。

### 3. プール生成を1か所に集約する(P7-1 のメトリクス化に備える。issue の受け入れ条件)

**spec-writer によるレビューでの修正(2026-09-24)**: 当初案は `openPool` を
`services/pokedex/cmd/pokedex/main.go`(package main)に置くとしていたが、それでは
`-tags mysql` の実 DB 統合テストを `services/pokedex/db/`(package db)に置けない
(Go は package main を他パッケージから import できない)。`services/pokedex/db` は
既に `migrate.go`・`grants.go` のように DB 生成・権限操作を持ち、`cmd/migrate` から
import される実績があるので、同じ置き場にする。

`services/pokedex/db` パッケージに以下をエクスポートする:

```go
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxIdleTime time.Duration
	ConnMaxLifetime time.Duration
}

func OpenPool(dsn string, cfg PoolConfig) (*sql.DB, error)
```

`OpenPool` は `sql.Open` → 4つの `Set*` 呼び出し → 返す、だけを行う(検証は呼び出し側の
`loadConfig` が済ませている前提。ここでは検証しない)。`services/pokedex/cmd/pokedex/main.go`
の `runServe`・`runExport` の両方がこれを経由する(現状はそれぞれが個別に `sql.Open` を
呼んでいる)。`main.go` は `db.OpenPool` を直接呼ばず、差し替え可能な package 変数
`var openPool = db.OpenPool` 経由で呼ぶ(テストが `runServe`/`runExport` に実際に渡った
`PoolConfig` を横取りして検証できるようにするため。DI 用の `cliEnv` 構造体を新設するほどでは
ないので、`cmd/migrate` より軽い「関数変数」の形にする)。

理由: issue が「P7-1 のメトリクス実装時に `OpenConnections`・`InUse`・`Idle`・`WaitCount`・
`WaitDuration` を公開できるよう、pool 生成を1か所へ閉じる」ことを明示的に求めている。
`db.Stats()` を呼べる `*sql.DB` の生成元が1つなら、将来の `/metrics` 相当のハンドラは
この関数が返した値を保持するだけで済む(今回メトリクス自体は実装しない)。

### 4. `export` は同じ設定を使うが `MaxOpenConns` だけ 1 に上書きする

`readmodel.Export` は内部で並行処理をしておらず(逐次のクエリ)、CLI として1回実行して
終わるバッチ処理(ADR-0111 決定4で `serve` とは別枠と決めた対象と同じ)。issue の
「`export` コマンドは同じ設定を利用するか、単発処理として最大open=1を明示し、無制限の
別経路を残さない」の二択のうち、**後者(`MaxOpenConns=1` を明示)を取る**。

理由: `export` はそもそも1接続あれば足りる実装(逐次)であり、`serve` と同じ10接続を
許すのは実態に合わない。`loadConfig` が読んだ `MaxIdleConns`・`ConnMaxIdleTime`・
`ConnMaxLifetime` の値はそのまま使う(`MaxIdleConns` が `MaxOpenConns=1` を超えないよう
`min(MaxIdleConns, MaxOpenConns)` に丸める。決定2の「idle は open 以下」を `export` の
上書き後も保つため)。

却下: `export` にも同じ `MaxOpenConns`(既定10)をそのまま使わせる — 実装(逐次処理)と
設定の意図(同時実行の上限)が一致せず、「無制限の別経路を残さない」という issue の意図には
沿うが、実態と違う数値を持たせる理由が無い。

**spec-writer によるレビューでの修正(2026-09-24)**: `database/sql` の `DBStats` には
`MaxOpenConnections` はあるが `MaxIdleConns` を表すフィールドが無い(標準ライブラリの
仕様。`Idle`(現在アイドル中の実接続数)はあるが、設定値そのものは返らない)。そのため
「`export` の `MaxIdleConns` が1に丸まっている」ことを `Stats()` だけで直接は確認できない。
丸め処理を `openPool` 内に埋め込まず、独立した純粋関数として切り出しテスト可能にする:

```go
// ForExport は export 用に MaxOpenConns=1 へ上書きし、MaxIdleConns をそれ以下に丸めた
// PoolConfig を返す(この関数自体をユニットテストで固定する。実 DB での Idle の実測は
// 別途 -tags mysql の統合テストで間接確認する)。
func (cfg PoolConfig) ForExport() PoolConfig
```

`runExport` は `openPool(cfg.DSN, cfg.Pool.ForExport())` を呼ぶ。

### 5. Deployment に既定値を環境変数として明示する

`deploy/k8s/base/pokedex/deployment.yaml` の `pokedex` コンテナに、決定1の4つの環境変数を
既定値のまま(`value:` で直接。秘密ではないので `secretKeyRef` は使わない)追加する。
コードのデフォルト値と二重管理になるが、issue が「Deployment へ既定値を明示し」と
明確に求めており、運用者がマニフェストを見るだけで Pod ごとの DB 接続予算を把握できる
ようにする(コードのデフォルトと manifest の値が食い違わないことは、両方をテストで
固定することで検知できるようにする)。

### 6. runbook に replica 数を増やすときの注記を足す

`docs/runbooks/data.md` に、「replica を増やすときは
`replicas × MaxOpenConns + importer/migrate の接続予算 < DB の max_connections` を
確認すること」という一文を追加する(issue の受け入れ条件どおり。今回は
`max_connections` 自体の変更・DB 製品選定はしない)。

## 受け入れ条件

1. 4つの環境変数が既定値(10/5/5m/30m)で読める。不正な値(0以下の整数・idle>open・
   0以下の duration・壊れた duration 文字列)は `sql.Open` より前にエラーになり、
   エラー文に DSN を含まない。
2. `db.OpenPool` が `sql.Open` 直後に4つの `Set*` を呼び、返す `*sql.DB` の
   `Stats().MaxOpenConnections` が設定値と一致する(実 DB 不要。`sql.Open` はネットワークに
   触れない)。
3. 実 MySQL(`-tags mysql`)で、`MaxOpenConns` を超える同時クエリを投げても
   `Stats().OpenConnections` が `MaxOpenConns` を超えない(超過分はプールで待機するか、
   context の締切で cancel される。新規接続を無制限に開かない)。あわせて `MaxIdleConns` も
   実 DB で(`Stats().Idle` の実測により間接的に)確認する。
4. `export` は `PoolConfig.ForExport()` で `MaxOpenConns=1` に上書きした設定で
   `db.OpenPool` を呼ぶ(`MaxIdleConns` は1以下に丸められる)。`ForExport()` 自体はユニット
   テストで固定し、`runExport` がそれを実際に使うことは `main.go` の `openPool` 変数を
   差し替えるテストで確認する。
5. `deployment.yaml` に4つの環境変数が既定値のまま明示され、コードのデフォルト値と
   一致することをテストで固定する(文字列の完全一致ではなく、`strconv.Atoi`・
   `time.ParseDuration` した値どうしの比較にする。値を2箇所にハードコードしない)。
6. `docs/runbooks/data.md` に replica 数を増やすときの注記がある。
7. `make test`・`make test-db`・`make lint`・`make build`・pokedex の k3d smoke
   (`make api-smoke`)が成功する。

## 実装時の申し送り(spec-writer、2026-09-24)

失敗するテストを次のファイルに用意した(実装は無い。implementer が実装すること):

- `services/pokedex/db/pool.go`(新規、未実装): `PoolConfig` 構造体・`OpenPool`・
  `(PoolConfig) ForExport()` をこのファイルに置く想定。テストは
  `services/pokedex/db/pool_test.go`(実 DB 不要)・`services/pokedex/db/pool_mysql_test.go`
  (`-tags mysql`)。
- `services/pokedex/cmd/pokedex/main.go`: 少なくとも次の識別子が要る。
  - `envDBMaxOpenConns = "POKEDEX_DB_MAX_OPEN_CONNS"`
  - `envDBMaxIdleConns = "POKEDEX_DB_MAX_IDLE_CONNS"`
  - `envDBConnMaxIdleTime = "POKEDEX_DB_CONN_MAX_IDLE_TIME"`
  - `envDBConnMaxLifetime = "POKEDEX_DB_CONN_MAX_LIFETIME"`
  - `defaultDBMaxOpenConns = 10`(int)
  - `defaultDBMaxIdleConns = 5`(int)
  - `defaultDBConnMaxIdleTime = 5 * time.Minute`
  - `defaultDBConnMaxLifetime = 30 * time.Minute`
  - `config` 構造体に `Pool db.PoolConfig` フィールドを足す
  - `loadConfig` がこの4変数を読み・検証し・`config.Pool` を埋める(DSN 検証と同じ関数・
    同じタイミング。エラー文に変数名だけを含め、値・DSN は含めない)
  - `var openPool = db.OpenPool`(package 変数。`runServe`・`runExport` はこれ経由で呼ぶ。
    テストが差し替えて渡された `PoolConfig` を検証する)
  - `runExport` は `openPool(cfg.DSN, cfg.Pool.ForExport())` を呼ぶ
- `services/pokedex/cmd/pokedex/main_test.go`・`manifest_test.go`・
  `services/pokedex/db/pool_test.go`・`pool_mysql_test.go`・`runbook_test.go` に
  spec-writer が追加した失敗テストがある(下記「追加したテスト」参照)。
- `deploy/k8s/base/pokedex/deployment.yaml`・`docs/runbooks/data.md` の書き換えは
  implementer が行う(spec-writer は書き換えない)。

## 影響

- 変更: `services/pokedex/db/`(新規 `pool.go`・`pool_test.go`・`pool_mysql_test.go`・
  `runbook_test.go`)、`services/pokedex/cmd/pokedex/main.go`(`loadConfig` の拡張・
  `openPool` 変数の追加)、`services/pokedex/cmd/pokedex/main_test.go`・`manifest_test.go`、
  `deploy/k8s/base/pokedex/deployment.yaml`、`docs/runbooks/data.md`。
- 変更しない: MySQL サーバー自体の `max_connections`、クエリ・index、DB 製品・クラウド選定、
  公開 API 契約(issue の宣言どおり)。
- P7-1(メトリクス)がこのプールの `Stats()` を後で観測に繋げられる。今回はメトリクス
  基盤自体を実装しない。

## 追記(2026-09-24): 独立レビュー(critic)の軽微指摘を反映

critic は PASS(重大・重要な指摘なし)。軽微指摘1件(`MaxIdleConns == MaxOpenConns` という
境界値〈バリデーションを通るべきケース〉を直接検証するテストが無かった)を反映し、
`services/pokedex/cmd/pokedex/main_test.go` に `TestLoadConfigPoolAllowsIdleEqualToOpen` を
追加した(`loadPoolConfig` の検査が `MaxIdleConns > MaxOpenConns` という厳密不等号であり、
等しい場合を誤って拒否しないことを直接固定する)。
