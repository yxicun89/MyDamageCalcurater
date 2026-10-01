package readtx

import (
	"context"
	"database/sql"

	"example.com/pokecalc/services/pokedex/internal/store"
)

type sqlDB struct {
	*store.Queries
	db *sql.DB
}

// NewDB は *sql.DB を包む。autocommit の読みは store.New(db)、BeginTx は開いた tx から store.New(tx) を作る。
func NewDB(db *sql.DB) DB {
	return &sqlDB{Queries: store.New(db), db: db}
}

type sqlTx struct {
	*store.Queries
	tx *sql.Tx
}

func (d *sqlDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	tx, err := d.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &sqlTx{Queries: store.New(tx), tx: tx}, nil
}

func (t *sqlTx) Commit() error   { return t.tx.Commit() }
func (t *sqlTx) Rollback() error { return t.tx.Rollback() }
