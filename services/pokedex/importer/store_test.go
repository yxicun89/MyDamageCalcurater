package importer_test

// RunStore(版に変化が無ければ投入しない判定と投入の順序)のテスト(ADR-0104 §10)。
// DB の代わりに偽の Store を使う(ネットワーク・DB なし。make test で走る)。

import (
	"context"
	"errors"
	"testing"
	"time"

	"example.com/pokecalc/services/pokedex/importer"
)

// fakeStore は importer.Store の偽物。Apply の呼び出しを記録する。
type fakeStore struct {
	applied    []importer.SourceVersion
	appliedErr error
	applyErr   error
	applyCalls [][]importer.SourceVersion
}

func (f *fakeStore) AppliedVersions(context.Context) ([]importer.SourceVersion, error) {
	if f.appliedErr != nil {
		return nil, f.appliedErr
	}
	return append([]importer.SourceVersion(nil), f.applied...), nil
}

func (f *fakeStore) Apply(_ context.Context, _ importer.Output, versions []importer.SourceVersion, _ time.Time) error {
	f.applyCalls = append(f.applyCalls, append([]importer.SourceVersion(nil), versions...))
	return f.applyErr
}

var _ importer.Store = (*fakeStore)(nil)

var storeNow = time.Date(2026, 9, 26, 3, 0, 0, 0, time.UTC)

func TestRunStoreDecision(t *testing.T) {
	pinned := []importer.SourceVersion{sv("calc", "v1", "a"), sv("showdown", "c1", "b")}
	tests := []struct {
		name        string
		applied     []importer.SourceVersion
		force       bool
		wantApplied bool
		wantCalls   int
	}{
		{"版と checksum が DB と同じなら何もしない", pinned, false, false, 0},
		{"DB が空(初回)なら投入する", nil, false, true, 1},
		{"DB が固定版と食い違っていれば投入し直す", []importer.SourceVersion{sv("calc", "v0", "a"), sv("showdown", "c1", "b")}, false, true, 1},
		{"force なら同じ版でも投入する", pinned, true, true, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &fakeStore{applied: tt.applied}
			got, err := importer.RunStore(context.Background(), s, importer.Output{}, pinned, storeNow, tt.force)
			if err != nil {
				t.Fatalf("RunStore: %v", err)
			}
			if got != tt.wantApplied {
				t.Errorf("RunStore = %v, want %v", got, tt.wantApplied)
			}
			if len(s.applyCalls) != tt.wantCalls {
				t.Fatalf("Apply の呼び出し %d 回, want %d", len(s.applyCalls), tt.wantCalls)
			}
			if tt.wantCalls == 1 {
				if ok, _ := importer.NeedsImport(s.applyCalls[0], pinned); ok {
					t.Errorf("Apply に渡った版 %+v が入力の版 %+v と違う", s.applyCalls[0], pinned)
				}
			}
		})
	}
}

func TestRunStoreDoesNotApplyWhenVersionsUnreadable(t *testing.T) {
	pinned := []importer.SourceVersion{sv("calc", "v1", "a")}
	tests := []struct {
		name string
		err  error
	}{
		{"migrate が済んでいない", importer.ErrSchemaNotReady},
		{"接続の失敗など", errors.New("connection refused")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &fakeStore{appliedErr: tt.err}
			got, err := importer.RunStore(context.Background(), s, importer.Output{}, pinned, storeNow, true)
			if !errors.Is(err, tt.err) {
				t.Fatalf("err = %v, want %v を包む", err, tt.err)
			}
			if got {
				t.Error("失敗したのに true を返した")
			}
			if len(s.applyCalls) != 0 {
				t.Errorf("版が読めないのに Apply を呼んだ(%d 回。force でも呼ばない)", len(s.applyCalls))
			}
		})
	}
}

func TestRunStoreRejectsInvalidIncomingVersions(t *testing.T) {
	s := &fakeStore{}
	bad := []importer.SourceVersion{{Source: "calc", Version: "", Checksum: "x"}}
	if _, err := importer.RunStore(context.Background(), s, importer.Output{}, bad, storeNow, false); !errors.Is(err, importer.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if len(s.applyCalls) != 0 {
		t.Error("不正な版で Apply を呼んだ")
	}
}

func TestRunStorePropagatesApplyError(t *testing.T) {
	s := &fakeStore{applyErr: importer.ErrKeyChanged}
	got, err := importer.RunStore(context.Background(), s, importer.Output{}, []importer.SourceVersion{sv("calc", "v1", "a")}, storeNow, false)
	if !errors.Is(err, importer.ErrKeyChanged) {
		t.Fatalf("err = %v, want ErrKeyChanged", err)
	}
	if got {
		t.Error("Apply が失敗したのに true を返した")
	}
}

func TestSchemaNotReadyIsDistinct(t *testing.T) {
	if importer.ErrSchemaNotReady == nil {
		t.Fatal("ErrSchemaNotReady が nil")
	}
	for _, other := range []error{importer.ErrInvalidInput, importer.ErrInvalidData, importer.ErrBlocked, importer.ErrKeyChanged} {
		if errors.Is(importer.ErrSchemaNotReady, other) {
			t.Errorf("ErrSchemaNotReady が %v と区別できない", other)
		}
	}
}
