# ADR-0127: 内部 API と `pokedex export` を1つの読み取り専用トランザクションで読む(issue #220)

- 状態: 採用
- 日付: 2026-10-01
- レーン: データ(ADR 帯 `0100〜`)
- 関連: issue #220、issue #403(パッケージ D10)、ADR-0105 §2・§5(内部 API と export)、ADR-0101(importer の全置換)

## 背景

`GET /internal/pokedex/master`(`buildMasterExport`)と `readmodel.Export` は、`store.Querier` の SELECT を
autocommit で順に発行していた。importer の `Apply` は全テーブルを1トランザクションで DELETE → INSERT する。
InnoDB の REPEATABLE READ でも autocommit の SELECT は文ごとに新しいスナップショットになるので、
SELECT の間に置換の commit が入ると「`data_versions` は旧、`moves` は新」のような混在した組を返しうる。
calc-svc が起動時に読むマスタと、balance/speed 向けの export 4 ファイルが、1回の import の結果だけで
構成されることを保証したい。

## 検討した案

- (A) 小さなインターフェース `readtx.Beginner`(`BeginTx(ctx, *sql.TxOptions) (readtx.Tx, error)`)を足し、
  読み出しの先頭で ReadOnly の Tx を開いて、その Tx の Querier(本番は `store.New(*sql.Tx)`)だけで全 SELECT を
  行い Commit する。**採用**(issue の既定案)。
- (B) `*sql.DB` をそのまま渡す。偽の DB(storetest)で「Tx の中で読んだか」を検査できず、テストに実 DB が要る。
- (C) 1本の SQL(UNION 等)にまとめる。sqlc のクエリと生成物を大きく変える。対象外の変更が増える。

## 決定

1. 新しいパッケージ `services/pokedex/internal/readtx` に `Tx`(`store.Querier` + `Commit` / `Rollback`)、
   `Beginner`、`DB`(`store.Querier` + `Beginner`)を置く。本番の実装は `readtx.NewDB(*sql.DB) readtx.DB`
   (autocommit の読み出しは `store.New(db)`、`BeginTx` は `db.BeginTx` の Tx から `store.New(tx)`)。
   sqlc の生成物(`internal/store`)は変えない。
2. `httpapi.NewHandler` / `httpapi.NewServer` は `readtx.DB` を受け取る。検索の各操作は従来どおり autocommit で
   よい(本 ADR の対象は内部 API のマスタ一式)。`GetMasterExport` は `BeginTx(ctx, &sql.TxOptions{ReadOnly: true})`
   で開いた Tx の中で全 SELECT を行い、最後に Commit する。
3. `readmodel.Export` は `readtx.Beginner` を受け取り、同じく1つの ReadOnly Tx の中で全 SELECT を行う。
4. 失敗の写し方: Tx を開けない・Tx の中の失敗・Commit の失敗は、内部 API では既存の `unavailable`
   (503 `master_unavailable`)、export では既存のエラー(ゼロ値の Files)にする。失敗時は Rollback で閉じる
   (Commit 後の Rollback は `sql.ErrTxDone` で無害なので `defer tx.Rollback()` でよい)。
5. 分離レベルは読み出しのスナップショットが Tx 全体で1つになるもの(MySQL の既定 REPEATABLE READ。
   READ COMMITTED 以下にしない)。InnoDB の一貫性読み出しは最初の SELECT でスナップショットを取る。

## 影響

- 読み取り専用 Tx は MVCC の一貫性読み出しだけで、行ロックを取らない。import の commit を待たせない
  (メタデータロックは共有読み取りで、importer の DML とは競合しない)。
- Tx の間は接続を1本占有する(従来も SELECT ごとに1本使っていた。プールの設定は変えない)。
- テスト: storetest の偽 Querier が `BeginTx` を記録し、`SnapshotViolations` / `RollbackViolations` で検査する。
  実 MySQL では、最初の SELECT の直後に別接続で全置換を commit させても、応答が置換前と一致することを確かめる
  (`services/pokedex/importer/snapshot_mysql_test.go`、`make test-db`)。
