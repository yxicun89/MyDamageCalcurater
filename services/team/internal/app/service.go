package app

import (
	"context"
	"time"

	"example.com/pokecalc/services/team/internal/domain"
)

// Service は team-svc の操作の手順。ゼロ値は使わない(New で作る)。
type Service struct {
	store Store
	now   func() time.Time
}

// New は Service を作る。now は時計(adapters が time.Now を渡す)。
func New(st Store, now func() time.Time) *Service {
	return &Service{store: st, now: now}
}

// touchAndRun は「端末 ID を含む要求を受けたら devices.last_seen_at を更新する」(ADR-0209 §4)を
// 全操作で徹底したうえで run を呼ぶ。TouchDevice が失敗したら run は呼ばない(どちらも同じ
// store_unavailable になるだけなので、二重に呼んで待たせない)。
func (s *Service) touchAndRun(ctx context.Context, deviceID string, run func() error) error {
	if err := s.store.TouchDevice(ctx, deviceID, s.now().UTC()); err != nil {
		return err
	}
	return run()
}

// Ready は team DB に届くかを確かめる。Store が Pinger を実装していればそれを、していなければ
// 読み取り専用の ListTeams(空の deviceID。副作用が無い)を1件だけ呼ぶ。
func (s *Service) Ready(ctx context.Context) error {
	if p, ok := s.store.(Pinger); ok {
		return p.Ping(ctx)
	}
	_, err := s.store.ListTeams(ctx, "")
	return err
}

// ListTeams はその端末の構築を全件返す。
func (s *Service) ListTeams(ctx context.Context, deviceID string) ([]Team, error) {
	var rows []Team
	err := s.touchAndRun(ctx, deviceID, func() error {
		var err error
		rows, err = s.store.ListTeams(ctx, deviceID)
		return err
	})
	return rows, err
}

// GetTeam はその端末の構築を1件返す(持っていなければ ErrNotFound)。
func (s *Service) GetTeam(ctx context.Context, deviceID, teamID string) (Team, error) {
	var t Team
	err := s.touchAndRun(ctx, deviceID, func() error {
		var err error
		t, err = s.store.GetTeam(ctx, deviceID, teamID)
		return err
	})
	return t, err
}

// CreateTeam は入力を検証して構築を1件作る。検証の失敗は domain.Error、上限は ErrTeamLimitReached。
func (s *Service) CreateTeam(ctx context.Context, deviceID string, in domain.TeamInput) (Team, error) {
	name, members, err := domain.ValidateTeamInput(in)
	if err != nil {
		return Team{}, err
	}
	var created Team
	err = s.touchAndRun(ctx, deviceID, func() error {
		var err error
		created, err = s.store.CreateTeam(ctx, deviceID, Team{Name: name, Members: members}, s.now().UTC())
		return err
	})
	return created, err
}

// UpdateTeam は入力を検証して構築の名前とメンバー全体を置き換える(部分更新はしない。ADR-0213 §2)。
// 検証の失敗は domain.Error、その端末が持っていなければ ErrNotFound。
func (s *Service) UpdateTeam(ctx context.Context, deviceID, teamID string, in domain.TeamInput) (Team, error) {
	name, members, err := domain.ValidateTeamInput(in)
	if err != nil {
		return Team{}, err
	}
	var updated Team
	err = s.touchAndRun(ctx, deviceID, func() error {
		var err error
		updated, err = s.store.UpdateTeam(ctx, deviceID, teamID, Team{Name: name, Members: members}, s.now().UTC())
		return err
	})
	return updated, err
}

// DeleteTeam はその端末の構築を1件消す(持っていなければ ErrNotFound)。
func (s *Service) DeleteTeam(ctx context.Context, deviceID, teamID string) error {
	return s.touchAndRun(ctx, deviceID, func() error {
		return s.store.DeleteTeam(ctx, deviceID, teamID)
	})
}

// PurgeDevice はその端末のデータを消す(ADR-0209 §5)。
func (s *Service) PurgeDevice(ctx context.Context, deviceID string) (PurgeResult, error) {
	var res PurgeResult
	err := s.touchAndRun(ctx, deviceID, func() error {
		var err error
		res, err = s.store.PurgeDevice(ctx, deviceID, s.now().UTC())
		return err
	})
	return res, err
}
