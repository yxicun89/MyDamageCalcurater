package app

// app の手順を HTTP なしで、偽のポートで検査する(ADR-0255)。応答の形は adapters/httpapi のテストが固定する。

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"example.com/pokecalc/services/team/internal/domain"
)

var fixedNow = time.Date(2026, 10, 12, 3, 0, 0, 0, time.UTC)

// spyStore は呼ばれたメソッドの順と、渡された時刻を記録する Store。touchErr があれば TouchDevice が失敗する。
type spyStore struct {
	calls    []string
	touchErr error
	nows     []time.Time
}

func (s *spyStore) TouchDevice(_ context.Context, _ string, now time.Time) error {
	s.calls = append(s.calls, "TouchDevice")
	s.nows = append(s.nows, now)
	return s.touchErr
}

func (s *spyStore) TouchDeviceFromEvent(context.Context, string, time.Time) (TouchOutcome, error) {
	s.calls = append(s.calls, "TouchDeviceFromEvent")
	return Touched, nil
}

func (s *spyStore) ListTeams(context.Context, string) ([]Team, error) {
	s.calls = append(s.calls, "ListTeams")
	return nil, nil
}

func (s *spyStore) GetTeam(context.Context, string, string) (Team, error) {
	s.calls = append(s.calls, "GetTeam")
	return Team{}, ErrNotFound
}

func (s *spyStore) CreateTeam(_ context.Context, _ string, t Team, now time.Time) (Team, error) {
	s.calls = append(s.calls, "CreateTeam")
	s.nows = append(s.nows, now)
	t.CreatedAt, t.UpdatedAt = now, now
	return t, nil
}

func (s *spyStore) UpdateTeam(_ context.Context, _, _ string, t Team, now time.Time) (Team, error) {
	s.calls = append(s.calls, "UpdateTeam")
	t.UpdatedAt = now
	return t, nil
}

func (s *spyStore) DeleteTeam(context.Context, string, string) error {
	s.calls = append(s.calls, "DeleteTeam")
	return nil
}

func (s *spyStore) PurgeDevice(_ context.Context, _ string, now time.Time) (PurgeResult, error) {
	s.calls = append(s.calls, "PurgeDevice")
	return PurgeResult{PurgedAt: now}, nil
}

func newService(st Store) *Service {
	return New(st, func() time.Time { return fixedNow.In(time.FixedZone("JST", 9*60*60)) })
}

// 全操作で TouchDevice が本体より先に呼ばれ、時刻は UTC で渡る。
func TestServiceTouchesDeviceFirst(t *testing.T) {
	ctx := context.Background()
	for name, tt := range map[string]struct {
		run  func(*Service) error
		want []string
	}{
		"ListTeams": {func(s *Service) error { _, err := s.ListTeams(ctx, "d"); return err }, []string{"TouchDevice", "ListTeams"}},
		"GetTeam":   {func(s *Service) error { _, err := s.GetTeam(ctx, "d", "t"); return err }, []string{"TouchDevice", "GetTeam"}},
		"CreateTeam": {func(s *Service) error {
			_, err := s.CreateTeam(ctx, "d", domain.TeamInput{})
			return err
		}, []string{"TouchDevice", "CreateTeam"}},
		"UpdateTeam": {func(s *Service) error {
			_, err := s.UpdateTeam(ctx, "d", "t", domain.TeamInput{})
			return err
		}, []string{"TouchDevice", "UpdateTeam"}},
		"DeleteTeam":  {func(s *Service) error { return s.DeleteTeam(ctx, "d", "t") }, []string{"TouchDevice", "DeleteTeam"}},
		"PurgeDevice": {func(s *Service) error { _, err := s.PurgeDevice(ctx, "d"); return err }, []string{"TouchDevice", "PurgeDevice"}},
	} {
		t.Run(name, func(t *testing.T) {
			st := &spyStore{}
			err := tt.run(newService(st))
			if err != nil && !errors.Is(err, ErrNotFound) {
				t.Fatalf("err = %v", err)
			}
			if !slices.Equal(st.calls, tt.want) {
				t.Errorf("calls = %v, want %v", st.calls, tt.want)
			}
			for _, n := range st.nows {
				if n.Location() != time.UTC || !n.Equal(fixedNow) {
					t.Errorf("渡された時刻 = %v, want UTC の %v", n, fixedNow)
				}
			}
		})
	}
}

// TouchDevice が失敗したら本体を呼ばず、その失敗をそのまま返す。
func TestServiceStopsWhenTouchFails(t *testing.T) {
	st := &spyStore{touchErr: ErrUnavailable}
	_, err := newService(st).ListTeams(context.Background(), "d")
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v, want ErrUnavailable", err)
	}
	if !slices.Equal(st.calls, []string{"TouchDevice"}) {
		t.Errorf("calls = %v, want TouchDevice だけ", st.calls)
	}
}

// 検証に通らない入力は store に触れずに domain.Error を返す(作成・置換とも)。
func TestServiceValidatesBeforeTouch(t *testing.T) {
	long := strings.Repeat("あ", domain.MaxTeamNameRunes+1)
	in := domain.TeamInput{Name: &long}
	for name, run := range map[string]func(*Service) error{
		"CreateTeam": func(s *Service) error { _, err := s.CreateTeam(context.Background(), "d", in); return err },
		"UpdateTeam": func(s *Service) error { _, err := s.UpdateTeam(context.Background(), "d", "t", in); return err },
	} {
		t.Run(name, func(t *testing.T) {
			st := &spyStore{}
			err := run(newService(st))
			var de *domain.Error
			if !errors.As(err, &de) || de.Code != domain.CodeInvalidInput {
				t.Errorf("err = %v, want domain.Error(invalid_input)", err)
			}
			if len(st.calls) != 0 {
				t.Errorf("calls = %v, want 呼ばない", st.calls)
			}
		})
	}
}

// Pinger を持たない Store では、読み取り専用の ListTeams(空の端末 ID)で到達を確かめ、TouchDevice は呼ばない。
func TestServiceReadyFallsBackToListTeams(t *testing.T) {
	st := &spyStore{}
	if err := newService(st).Ready(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(st.calls, []string{"ListTeams"}) {
		t.Errorf("calls = %v, want ListTeams だけ", st.calls)
	}
}
