// Package readtx は、pokedex-svc の読み出しを1つの読み取り専用トランザクション(一貫したスナップショット)
// の中で行うための小さなインターフェース(ADR-0127・issue #220)。
//
// importer は全テーブルを1トランザクションで置き換える。autocommit の SELECT を並べると、SELECT ごとに
// 別のスナップショットになり、その間に置き換えの commit が入ると新旧が混在した組を返す。
// 内部 API(/internal/pokedex/master)と `pokedex export` は、BeginTx で開いた Tx の Querier
// (store.New(tx))だけで全 SELECT を行い、最後に Commit する。
//
// sqlc の生成物(store)は変えない。*sql.Tx は sqlc の DBTX を満たすので store.New(tx) で足りる。
package readtx

import (
	"context"
	"database/sql"

	"example.com/pokecalc/services/pokedex/internal/store"
)

// Tx は読み取り専用トランザクションの中の Querier。Commit / Rollback の意味は *sql.Tx と同じ
// (終わった後の呼び出しは sql.ErrTxDone)。
type Tx interface {
	store.Querier
	Commit() error
	Rollback() error
}

// Beginner はトランザクションを開ける。opts は *sql.DB.BeginTx にそのまま渡る
// (読み出しの側は ReadOnly: true を渡す)。
type Beginner interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error)
}

// DB は autocommit の読み出し(検索の各操作。1回の SELECT で完結する)と、
// スナップショットの読み出し(BeginTx)の両方ができる。本番は *sql.DB を包んだもの、
// テストは storetest.Querier が満たす。
type DB interface {
	store.Querier
	Beginner
}
