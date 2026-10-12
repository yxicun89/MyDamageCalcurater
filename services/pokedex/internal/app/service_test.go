package app_test

// app のユースケースを HTTP・DB なしで偽のポートで確かめる(ADR-0811・ADR-0180)。

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"example.com/pokecalc/services/pokedex/internal/app"
	"example.com/pokecalc/services/pokedex/internal/domain"
)

// fakeStore は app.Store の偽物。使うメソッドだけを実装し、呼ばれたメソッド名を記録する
// (実装していないメソッドが呼ばれたら埋め込みの nil で panic する)。
type fakeStore struct {
	app.Store
	calls  []string
	regErr error
	move   app.Move
	getErr error
}

func (f *fakeStore) GetDefaultRegulation(context.Context) (app.Regulation, error) {
	f.calls = append(f.calls, "GetDefaultRegulation")
	return app.Regulation{ID: "reg"}, f.regErr
}

func (f *fakeStore) GetMove(_ context.Context, id string) (app.Move, error) {
	f.calls = append(f.calls, "GetMove")
	return f.move, f.getErr
}

func codeOf(t *testing.T, err error) *domain.Error {
	t.Helper()
	var de *domain.Error
	if !errors.As(err, &de) {
		t.Fatalf("err = %v, domain.Error のはず", err)
	}
	return de
}

// 入力の検証は DB を呼ぶ前に行う(ADR-0105 §3)。
func TestValidationBeforeStore(t *testing.T) {
	ctx := context.Background()
	zero, negative := 0, -1
	tests := []struct {
		name string
		call func(*app.Service) error
		code domain.Code
	}{
		{"limit", func(s *app.Service) error {
			_, err := s.SearchSpecies(ctx, app.SpeciesQuery{Limit: &zero})
			return err
		}, domain.CodeInvalidInput},
		{"format", func(s *app.Service) error {
			_, err := s.SearchSpecies(ctx, app.SpeciesQuery{Format: &domain.Enum{Value: "triple"}})
			return err
		}, domain.CodeInvalidEnum},
		{"species key", func(s *app.Service) error {
			_, err := s.GetSpecies(ctx, "pikachu")
			return err
		}, domain.CodeInvalidInput},
		{"ids", func(s *app.Service) error {
			_, err := s.GetMovesByIDs(ctx, nil)
			return err
		}, domain.CodeInvalidInput},
		{"offset", func(s *app.Service) error {
			_, err := s.ListMoveLearners(ctx, app.LearnersQuery{MoveKey: "x", Offset: &negative})
			return err
		}, domain.CodeInvalidInput},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeStore{}
			if got := codeOf(t, tt.call(app.New(f))).Code; got != tt.code {
				t.Errorf("code = %q, want %q", got, tt.code)
			}
			if len(f.calls) != 0 {
				t.Errorf("検証の失敗なのに DB を呼んだ: %v", f.calls)
			}
		})
	}
}

// 行が無い(app.ErrNotFound)は not_found、それ以外の失敗は理由つきの master_unavailable。
func TestGetMoveErrors(t *testing.T) {
	ctx := context.Background()
	f := &fakeStore{getErr: errors.Join(app.ErrNotFound)}
	de := codeOf(t, func() error { _, err := app.New(f).GetMove(ctx, "nope"); return err }())
	if de.Code != domain.CodeNotFound || de.Message != "技が無い: nope" {
		t.Errorf("got %q %q, want not_found 技が無い: nope", de.Code, de.Message)
	}

	cause := errors.New("db down")
	f = &fakeStore{getErr: cause}
	de = codeOf(t, func() error { _, err := app.New(f).GetMove(ctx, "x"); return err }())
	if de.Code != domain.CodeMasterUnavailable || de.Reason != "GetMove" || !errors.Is(de.Cause, cause) || de.Message != domain.MessageUnavailable {
		t.Errorf("got %+v, want master_unavailable(理由 GetMove・元の失敗つき)", de)
	}
}

// 既定のレギュレーションが引けなければ、技の存在を確かめる前に 503(ADR-0251 の判定の順)。
func TestListMoveLearnersRegulationFirst(t *testing.T) {
	f := &fakeStore{regErr: errors.New("db down")}
	_, err := app.New(f).ListMoveLearners(context.Background(), app.LearnersQuery{MoveKey: "x"})
	if de := codeOf(t, err); de.Code != domain.CodeMasterUnavailable || de.Reason != "ListMoveLearners/GetDefaultRegulation" {
		t.Errorf("got %+v", de)
	}
	if strings.Join(f.calls, ",") != "GetDefaultRegulation" {
		t.Errorf("calls = %v, 既定のレギュレーションだけのはず", f.calls)
	}
}

// app・domain は sqlc の生成物(internal/store)と readtx を import しない(DB はポート経由。G7)。
// archcheck が見ない同じサービスの DB のパッケージを、ここで見る。
func TestInnerLayersDoNotImportStore(t *testing.T) {
	forbidden := []string{"services/pokedex/internal/store", "services/pokedex/internal/adapters", "database/sql"}
	for _, dir := range []string{".", "../domain"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, imp := range f.Imports {
				p, _ := strconv.Unquote(imp.Path.Value)
				for _, x := range forbidden {
					if p == x || strings.HasSuffix(p, x) || strings.Contains(p, x+"/") {
						t.Errorf("%s が %q を import している(DB は app のポート経由で触る)", path, p)
					}
				}
			}
		}
	}
}
