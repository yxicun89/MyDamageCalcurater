//go:build mysql

package importer_test

// 内部 API(/internal/pokedex/master)と `pokedex export`(readmodel.Export)が、import の全置換と重なっても
// 1回の import の結果だけを返すことを実 MySQL で確かめる(issue #220・ADR-0127)。`make test-db` だけが実行する。
//
// 読み出しの最初の SELECT(内部 API は ListDataVersions、export は GetDefaultRegulation)が返った直後に、
// 別の接続で importer.Apply(全置換。1トランザクション)を commit させる。読み出しが1つのスナップショットなら、
// 応答は置換前と完全に同じになる。autocommit の SELECT を並べていると、後の SELECT が置換後の行を読み、混在する。

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"example.com/pokecalc/services/pokedex/importer"
	"example.com/pokecalc/services/pokedex/internal/httpapi"
	"example.com/pokecalc/services/pokedex/internal/readmodel"
	"example.com/pokecalc/services/pokedex/internal/readtx"
	"example.com/pokecalc/services/pokedex/internal/store"
)

// replaceTimeout は読み出しの途中に挟む全置換の待ち時間の上限(読み取り専用の Tx は行ロックを取らないので、
// 置換が待たされるなら設計の誤り。固まらずに失敗させる)。
const replaceTimeout = 30 * time.Second

// renamedOutput は out と同じ行で、種族・技の日本語名と取得元の版だけを変えたもの(2回目の import を模す)。
func renamedOutput(out importer.Output, versions []importer.SourceVersion) (importer.Output, []importer.SourceVersion) {
	next := out
	next.Species = append([]importer.SpeciesRow(nil), out.Species...)
	for i := range next.Species {
		next.Species[i].NameJa += "改"
	}
	next.Moves = append([]importer.MoveRow(nil), out.Moves...)
	for i := range next.Moves {
		next.Moves[i].NameJa += "改"
	}
	nextVersions := append([]importer.SourceVersion(nil), versions...)
	for i := range nextVersions {
		nextVersions[i].Version += "-next"
	}
	return next, nextVersions
}

// replaceAfterFirstRead は readtx.DB を包み、BeginTx が返す Tx の最初の読み出し(ListDataVersions か
// GetDefaultRegulation)が返った直後に replace を1回だけ実行する。fired で実行したかを確かめる
// (読み出しが Tx を使っていなければ発火しない = 検査が空振りしたことが分かる)。
type replaceAfterFirstRead struct {
	readtx.DB
	replace func(ctx context.Context) error

	once  sync.Once
	fired bool
	err   error
}

func (r *replaceAfterFirstRead) BeginTx(ctx context.Context, opts *sql.TxOptions) (readtx.Tx, error) {
	tx, err := r.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &replacingTx{Tx: tx, owner: r}, nil
}

func (r *replaceAfterFirstRead) fire() {
	r.once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), replaceTimeout)
		defer cancel()
		r.fired = true
		r.err = r.replace(ctx)
	})
}

type replacingTx struct {
	readtx.Tx
	owner *replaceAfterFirstRead
}

func (t *replacingTx) ListDataVersions(ctx context.Context) ([]store.DataVersion, error) {
	rows, err := t.Tx.ListDataVersions(ctx)
	t.owner.fire()
	return rows, err
}

func (t *replacingTx) GetDefaultRegulation(ctx context.Context) (store.GetDefaultRegulationRow, error) {
	row, err := t.Tx.GetDefaultRegulation(ctx)
	t.owner.fire()
	return row, err
}

func getMaster(t *testing.T, h http.Handler) []byte {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/internal/pokedex/master", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody=%s", rec.Code, rec.Body.String())
	}
	return rec.Body.Bytes()
}

// TestMasterExportIsOneSnapshotDuringImport は、内部 API の SELECT の間に全置換が commit されても、
// 応答が置換前の import の結果と完全に一致する(dataVersion と各テーブルが同じ commit 由来)ことを確かめる。
func TestMasterExportIsOneSnapshotDuringImport(t *testing.T) {
	conn := freshImportDB(t)
	out, versions := fixtureOutput(t)
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if err := importer.Apply(context.Background(), conn, out, versions, now); err != nil {
		t.Fatalf("Apply(1回目): %v", err)
	}
	db := readtx.NewDB(conn)
	before := getMaster(t, httpapi.NewHandler(db))

	nextOut, nextVersions := renamedOutput(out, versions)
	hooked := &replaceAfterFirstRead{DB: db, replace: func(ctx context.Context) error {
		return importer.Apply(ctx, conn, nextOut, nextVersions, now.Add(time.Hour))
	}}
	during := getMaster(t, httpapi.NewHandler(hooked))
	if !hooked.fired {
		t.Fatal("Tx の ListDataVersions が呼ばれず、読み出しの途中に全置換を挟めなかった(Tx を使っていない)")
	}
	if hooked.err != nil {
		t.Fatalf("読み出しの途中の Apply(2回目): %v", hooked.err)
	}
	if string(during) != string(before) {
		t.Errorf("全置換と重なった応答が置換前と一致しない(新旧の混在)\nbefore=%s\nduring=%s", before, during)
	}

	after := getMaster(t, httpapi.NewHandler(db))
	if string(after) == string(before) {
		t.Fatal("2回目の import の後も応答が変わらない(置換が効いておらず、検査が空振りしている)")
	}
}

// TestExportIsOneSnapshotDuringImport は readmodel.Export について同じことを確かめる。
func TestExportIsOneSnapshotDuringImport(t *testing.T) {
	conn := freshImportDB(t)
	out, versions := fixtureOutput(t)
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if err := importer.Apply(context.Background(), conn, out, versions, now); err != nil {
		t.Fatalf("Apply(1回目): %v", err)
	}
	ctx := context.Background()
	db := readtx.NewDB(conn)
	before, _, err := readmodel.Export(ctx, db)
	if err != nil {
		t.Fatalf("Export(置換前): %v", err)
	}

	nextOut, nextVersions := renamedOutput(out, versions)
	hooked := &replaceAfterFirstRead{DB: db, replace: func(ctx context.Context) error {
		return importer.Apply(ctx, conn, nextOut, nextVersions, now.Add(time.Hour))
	}}
	during, _, err := readmodel.Export(ctx, hooked)
	if err != nil {
		t.Fatalf("Export(置換と重なる): %v", err)
	}
	if !hooked.fired {
		t.Fatal("Tx の GetDefaultRegulation が呼ばれず、読み出しの途中に全置換を挟めなかった(Tx を使っていない)")
	}
	if hooked.err != nil {
		t.Fatalf("読み出しの途中の Apply(2回目): %v", hooked.err)
	}
	if !reflect.DeepEqual(during, before) {
		t.Errorf("全置換と重なった出力が置換前と一致しない(新旧の混在)\nbefore=%s\nduring=%s", before.PokemonTypes, during.PokemonTypes)
	}

	after, _, err := readmodel.Export(ctx, db)
	if err != nil {
		t.Fatalf("Export(置換後): %v", err)
	}
	if reflect.DeepEqual(after, before) {
		t.Fatal("2回目の import の後も出力が変わらない(置換が効いておらず、検査が空振りしている)")
	}
}

// TestReadTxIsReadOnly は readtx.NewDB の BeginTx が TxOptions を *sql.DB にそのまま渡し、
// ReadOnly の Tx では書き込みが DB に拒否されることを確かめる。
func TestReadTxIsReadOnly(t *testing.T) {
	conn := freshImportDB(t)
	out, versions := fixtureOutput(t)
	if err := importer.Apply(context.Background(), conn, out, versions, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	ctx := context.Background()
	tx, err := readtx.NewDB(conn).BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	defer tx.Rollback() //nolint:errcheck // 検査の後始末
	if _, err := tx.ListDataVersions(ctx); err != nil {
		t.Fatalf("Tx の中で読めない: %v", err)
	}
	if err := tx.DeleteDataVersions(ctx); err == nil {
		t.Error("ReadOnly の Tx で DELETE が通った(TxOptions が DB に渡っていない)")
	}
}
