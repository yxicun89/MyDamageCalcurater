package storetest_test

// 偽の BeginTx / Tx と、その検査(SnapshotViolations・RollbackViolations)が空振りしないことのテスト
// (issue #220・ADR-0127)。httpapi・readmodel のスナップショットのテストはこの検査に頼るため。

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"example.com/pokecalc/services/pokedex/internal/storetest"
)

var readOnly = &sql.TxOptions{ReadOnly: true}

func TestSnapshotViolationsAcceptsReadsInsideCommittedTx(t *testing.T) {
	q := storetest.New()
	ctx := context.Background()
	tx, err := q.BeginTx(ctx, readOnly)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback() //nolint:errcheck // Commit 後は sql.ErrTxDone になるだけ(本番の書き方と同じ)
	if _, err := tx.ListTypes(ctx); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); !errors.Is(err, sql.ErrTxDone) {
		t.Errorf("Commit 後の Rollback = %v, want sql.ErrTxDone", err)
	}
	if _, err := tx.ListTypes(ctx); !errors.Is(err, sql.ErrTxDone) {
		t.Errorf("Commit 後の読み出し = %v, want sql.ErrTxDone", err)
	}
	if p := q.SnapshotViolations([]string{"ListTypes"}); len(p) != 0 {
		t.Errorf("正しい使い方を違反と判定した: %v", p)
	}
	if open := q.OpenTxCount(); open != 0 {
		t.Errorf("OpenTxCount = %d, want 0", open)
	}
}

func TestSnapshotViolationsRejects(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		run  func(q *storetest.Querier)
	}{
		{"Tx を開かない", func(q *storetest.Querier) { _, _ = q.ListTypes(ctx) }},
		{"Tx の外で読む", func(q *storetest.Querier) {
			tx, _ := q.BeginTx(ctx, readOnly)
			_, _ = q.ListTypes(ctx)
			_ = tx.Commit()
		}},
		{"ReadOnly でない", func(q *storetest.Querier) {
			tx, _ := q.BeginTx(ctx, nil)
			_, _ = tx.ListTypes(ctx)
			_ = tx.Commit()
		}},
		{"READ COMMITTED", func(q *storetest.Querier) {
			tx, _ := q.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelReadCommitted})
			_, _ = tx.ListTypes(ctx)
			_ = tx.Commit()
		}},
		{"Commit しない", func(q *storetest.Querier) {
			tx, _ := q.BeginTx(ctx, readOnly)
			_, _ = tx.ListTypes(ctx)
		}},
		{"2つの Tx に分ける", func(q *storetest.Querier) {
			for i := 0; i < 2; i++ {
				tx, _ := q.BeginTx(ctx, readOnly)
				_, _ = tx.ListTypes(ctx)
				_ = tx.Commit()
			}
		}},
		{"Commit の後に読む", func(q *storetest.Querier) {
			tx, _ := q.BeginTx(ctx, readOnly)
			_ = tx.Commit()
			_, _ = q.ListTypes(ctx)
		}},
		{"必要なクエリを読まない", func(q *storetest.Querier) {
			tx, _ := q.BeginTx(ctx, readOnly)
			_ = tx.Commit()
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := storetest.New()
			tt.run(q)
			if p := q.SnapshotViolations([]string{"ListTypes"}); len(p) == 0 {
				t.Error("違反を検出しなかった")
			}
		})
	}
}

func TestRollbackViolations(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name      string
		beginErr  bool
		run       func(q *storetest.Querier)
		wantClean bool
	}{
		{"Rollback で閉じる", false, func(q *storetest.Querier) {
			tx, _ := q.BeginTx(ctx, readOnly)
			_, _ = tx.ListTypes(ctx)
			_ = tx.Rollback()
		}, true},
		{"BeginTx が失敗して何も読まない", true, func(q *storetest.Querier) {
			_, _ = q.BeginTx(ctx, readOnly)
		}, true},
		{"BeginTx が失敗して autocommit で読む", true, func(q *storetest.Querier) {
			_, _ = q.BeginTx(ctx, readOnly)
			q.ErrByMethod = nil
			_, _ = q.ListTypes(ctx)
		}, false},
		{"失敗したのに Commit", false, func(q *storetest.Querier) {
			tx, _ := q.BeginTx(ctx, readOnly)
			_ = tx.Commit()
		}, false},
		{"閉じない", false, func(q *storetest.Querier) {
			tx, _ := q.BeginTx(ctx, readOnly)
			_, _ = tx.ListTypes(ctx)
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := storetest.New()
			if tt.beginErr {
				q.ErrByMethod = map[string]error{storetest.MethodBeginTx: storetest.ErrDB}
			}
			tt.run(q)
			p := q.RollbackViolations()
			if tt.wantClean && len(p) != 0 {
				t.Errorf("正しい使い方を違反と判定した: %v", p)
			}
			if !tt.wantClean && len(p) == 0 {
				t.Error("違反を検出しなかった")
			}
		})
	}
}

func TestBeginTxFailureReturnsInjectedError(t *testing.T) {
	q := storetest.New()
	q.ErrByMethod = map[string]error{storetest.MethodBeginTx: storetest.ErrDB}
	tx, err := q.BeginTx(context.Background(), readOnly)
	if !errors.Is(err, storetest.ErrDB) || tx != nil {
		t.Errorf("BeginTx = (%v, %v), want (nil, ErrDB)", tx, err)
	}
}
