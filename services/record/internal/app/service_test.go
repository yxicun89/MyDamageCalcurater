package app

// app の手順を HTTP なしで、偽のポートで検査する(ADR-0254)。応答の形は adapters/httpapi のテストが固定する。

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"example.com/pokecalc/services/record/internal/domain"
)

var fixedNow = time.Date(2026, 10, 12, 3, 0, 0, 0, time.UTC)

// spyStore は呼ばれたメソッドの順を記録する Store。touchErr があれば TouchDevice が失敗する。
type spyStore struct {
	calls    []string
	touchErr error
	history  []CalcHistoryRow
	queries  []CalcHistoryQuery
}

func (s *spyStore) TouchDevice(context.Context, string, time.Time) error {
	s.calls = append(s.calls, "TouchDevice")
	return s.touchErr
}

func (s *spyStore) FrequentOpponents(context.Context, string, int) ([]FrequentOpponent, error) {
	s.calls = append(s.calls, "FrequentOpponents")
	return nil, nil
}

func (s *spyStore) PurgeDevice(context.Context, string, time.Time) (PurgeResult, error) {
	s.calls = append(s.calls, "PurgeDevice")
	return PurgeResult{}, nil
}

func (s *spyStore) SaveCalcEvent(context.Context, CalcEvent) (SaveOutcome, error) {
	s.calls = append(s.calls, "SaveCalcEvent")
	return Stored, nil
}

func (s *spyStore) ListFavorites(context.Context, string) ([]Favorite, error) {
	s.calls = append(s.calls, "ListFavorites")
	return nil, nil
}

func (s *spyStore) CreateFavorite(_ context.Context, _ string, f Favorite, now time.Time) (Favorite, FavoriteOutcome, error) {
	s.calls = append(s.calls, "CreateFavorite")
	f.CreatedAt, f.UpdatedAt = now, now
	return f, FavoriteCreated, nil
}

func (s *spyStore) DeleteFavorite(context.Context, string, int64) error {
	s.calls = append(s.calls, "DeleteFavorite")
	return nil
}

func (s *spyStore) ListCalcHistory(_ context.Context, _ string, q CalcHistoryQuery) ([]CalcHistoryRow, error) {
	s.calls = append(s.calls, "ListCalcHistory")
	s.queries = append(s.queries, q)
	return s.history[:min(len(s.history), q.Limit)], nil
}

func anyValue(string) bool { return true }

var testEnums = domain.Enums{
	Status: anyValue, Type: anyValue, Format: anyValue, Weather: anyValue, Terrain: anyValue,
	StatusNone: "none", WeatherNone: "none", TerrainNone: "none",
}

func newService(st Store, retention time.Duration) *Service {
	return New(st, testEnums, func() time.Time { return fixedNow }, retention)
}

// どの操作も TouchDevice を先に呼び、失敗したら本体を呼ばない(ADR-0209 §4)。
func TestEveryOperationTouchesFirst(t *testing.T) {
	ctx := context.Background()
	ops := map[string]func(*Service) error{
		"FrequentOpponents": func(s *Service) error { _, err := s.FrequentOpponents(ctx, "d", 10); return err },
		"PurgeDevice":       func(s *Service) error { _, err := s.PurgeDevice(ctx, "d"); return err },
		"ListFavorites":     func(s *Service) error { _, err := s.ListFavorites(ctx, "d"); return err },
		"DeleteFavorite":    func(s *Service) error { return s.DeleteFavorite(ctx, "d", 1) },
		"ListCalcHistory":   func(s *Service) error { _, _, err := s.ListCalcHistory(ctx, "d", nil, 5); return err },
	}
	for name, op := range ops {
		t.Run(name, func(t *testing.T) {
			ok := &spyStore{}
			if err := op(newService(ok, 0)); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(ok.calls, []string{"TouchDevice", name}) {
				t.Errorf("呼び出し = %v", ok.calls)
			}
			ng := &spyStore{touchErr: ErrUnavailable}
			if err := op(newService(ng, 0)); !errors.Is(err, ErrUnavailable) {
				t.Errorf("err = %v, want ErrUnavailable", err)
			}
			if !slices.Equal(ng.calls, []string{"TouchDevice"}) {
				t.Errorf("TouchDevice の失敗後に呼んだ: %v", ng.calls)
			}
		})
	}
}

// 検証に失敗した作成は store に触らない(TouchDevice も呼ばない。HTTP 境界にあったときと同じ順)。
func TestCreateFavoriteValidatesBeforeStore(t *testing.T) {
	st := &spyStore{}
	_, _, err := newService(st, 0).CreateFavorite(context.Background(), "d", domain.FavoriteInput{})
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != domain.CodeInvalidInput {
		t.Fatalf("err = %v, want invalid_input", err)
	}
	if len(st.calls) != 0 {
		t.Errorf("store を呼んだ: %v", st.calls)
	}
}

// 履歴は 1 行多く読み、続きがあれば最後に返した行を次の位置にする。保持期間は時計から下限にする(ADR-0230)。
func TestListCalcHistoryPagesAndRetention(t *testing.T) {
	st := &spyStore{history: []CalcHistoryRow{
		{EventID: "e3", OccurredAt: fixedNow.Add(-1 * time.Minute)},
		{EventID: "e2", OccurredAt: fixedNow.Add(-2 * time.Minute)},
		{EventID: "e1", OccurredAt: fixedNow.Add(-3 * time.Minute)},
	}}
	rows, next, err := newService(st, 90*24*time.Hour).ListCalcHistory(context.Background(), "d", nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || next == nil || next.EventID != "e2" || !next.OccurredAt.Equal(rows[1].OccurredAt) {
		t.Fatalf("rows = %v next = %+v", rows, next)
	}
	q := st.queries[0]
	if q.Limit != 3 || !q.Since.Equal(fixedNow.Add(-90*24*time.Hour)) {
		t.Errorf("query = %+v", q)
	}

	st2 := &spyStore{history: st.history[:2]}
	if _, next, _ := newService(st2, 0).ListCalcHistory(context.Background(), "d", nil, 2); next != nil {
		t.Errorf("続きが無いのに next = %+v", next)
	}
	if !st2.queries[0].Since.IsZero() {
		t.Errorf("保持期間 0 で下限がある: %v", st2.queries[0].Since)
	}
}
