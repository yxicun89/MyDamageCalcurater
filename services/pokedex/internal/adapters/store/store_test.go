package store_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"example.com/pokecalc/services/pokedex/internal/adapters/store"
	"example.com/pokecalc/services/pokedex/internal/app"
	"example.com/pokecalc/services/pokedex/internal/storetest"
)

// 1行を引く読み出しで行が無いとき、app.ErrNotFound と sql.ErrNoRows の両方で判定でき、文面は元のまま
// (ログに出る文面を変えない)。
func TestNoRowsIsNotFound(t *testing.T) {
	db := store.New(storetest.New())
	_, err := db.GetSpeciesByKey(context.Background(), "9999-999")
	if !errors.Is(err, app.ErrNotFound) || !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("err = %v, app.ErrNotFound と sql.ErrNoRows の両方で判定できるはず", err)
	}
	if err.Error() != sql.ErrNoRows.Error() {
		t.Errorf("文面 = %q, 元のまま %q のはず", err.Error(), sql.ErrNoRows.Error())
	}
}

// 行が無い以外の失敗は包まない(app.ErrNotFound にならない)。
func TestOtherErrorsAreNotNotFound(t *testing.T) {
	q := storetest.New()
	q.Err = storetest.ErrDB
	_, err := store.New(q).GetMove(context.Background(), "testflame")
	if !errors.Is(err, storetest.ErrDB) || errors.Is(err, app.ErrNotFound) {
		t.Fatalf("err = %v, ErrDB のまま(app.ErrNotFound ではない)のはず", err)
	}
}
