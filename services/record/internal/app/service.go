package app

import (
	"context"
	"time"

	"example.com/pokecalc/services/record/internal/domain"
)

// Service は record-svc の操作の手順。ゼロ値は使わない(New で作る)。
type Service struct {
	store Store
	enums domain.Enums
	now   func() time.Time
	// calcEventsRetention は計算履歴の下限(これより古い行は返さない。ADR-0230 §4)。0 なら下限なし。
	calcEventsRetention time.Duration
}

// New は Service を作る。now は時計(adapters が time.Now を渡す)、enums は契約の列挙の判定。
func New(st Store, enums domain.Enums, now func() time.Time, calcEventsRetention time.Duration) *Service {
	return &Service{store: st, enums: enums, now: now, calcEventsRetention: calcEventsRetention}
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

// Ready は record DB に届くかを確かめる。Store が Pinger を実装していればそれを、していなければ
// 読み取り専用の FrequentOpponents(空の deviceID。副作用が無い)を1件だけ呼ぶ。
func (s *Service) Ready(ctx context.Context) error {
	if p, ok := s.store.(Pinger); ok {
		return p.Ping(ctx)
	}
	_, err := s.store.FrequentOpponents(ctx, "", 1)
	return err
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

// FrequentOpponents はその端末のよく使う相手を最大 limit 件返す(ADR-0209 §3 #2。limit の範囲は adapters が検証済み)。
func (s *Service) FrequentOpponents(ctx context.Context, deviceID string, limit int) ([]FrequentOpponent, error) {
	var rows []FrequentOpponent
	err := s.touchAndRun(ctx, deviceID, func() error {
		var err error
		rows, err = s.store.FrequentOpponents(ctx, deviceID, limit)
		return err
	})
	return rows, err
}

// ListFavorites はその端末のお気に入りを返す(ADR-0227)。
func (s *Service) ListFavorites(ctx context.Context, deviceID string) ([]Favorite, error) {
	var rows []Favorite
	err := s.touchAndRun(ctx, deviceID, func() error {
		var err error
		rows, err = s.store.ListFavorites(ctx, deviceID)
		return err
	})
	return rows, err
}

// CreateFavorite は内容を検証・正規化して保存する(ADR-0227・ADR-0228)。検証の失敗は domain.Error、
// 上限は ErrFavoriteLimitReached。同じ内容が既にあれば FavoriteExisted(UpdatedAt を進める)。
func (s *Service) CreateFavorite(ctx context.Context, deviceID string, in domain.FavoriteInput) (Favorite, FavoriteOutcome, error) {
	snap, err := domain.NormalizeFavorite(in, s.enums)
	if err != nil {
		return Favorite{}, 0, err
	}
	raw, err := domain.EncodeFavorite(snap)
	if err != nil {
		return Favorite{}, 0, err
	}
	var (
		saved   Favorite
		outcome FavoriteOutcome
	)
	err = s.touchAndRun(ctx, deviceID, func() error {
		var err error
		saved, outcome, err = s.store.CreateFavorite(ctx, deviceID,
			Favorite{SpeciesKey: snap.Individual.SpeciesKey, Snapshot: raw}, s.now().UTC().Truncate(time.Microsecond))
		return err
	})
	return saved, outcome, err
}

// DeleteFavorite はその端末のお気に入りを1件消す(持っていなければ ErrNotFound)。
func (s *Service) DeleteFavorite(ctx context.Context, deviceID string, favoriteID int64) error {
	return s.touchAndRun(ctx, deviceID, func() error {
		return s.store.DeleteFavorite(ctx, deviceID, favoriteID)
	})
}

// ListCalcHistory はその端末の計算履歴を最新から最大 limit 行返す(ADR-0230)。before は keyset の位置(nil なら先頭)。
// まだ続きがあれば、次の位置(最後に返した行)を next に返す。
func (s *Service) ListCalcHistory(ctx context.Context, deviceID string, before *CalcHistoryCursor, limit int) (rows []CalcHistoryRow, next *CalcHistoryCursor, err error) {
	// 1行多く読み、続きがあるかを知る。
	q := CalcHistoryQuery{Before: before, Limit: limit + 1}
	if s.calcEventsRetention > 0 {
		q.Since = s.now().UTC().Add(-s.calcEventsRetention)
	}
	err = s.touchAndRun(ctx, deviceID, func() error {
		var err error
		rows, err = s.store.ListCalcHistory(ctx, deviceID, q)
		if err != nil {
			return err
		}
		if len(rows) > limit {
			rows = rows[:limit]
			next = &CalcHistoryCursor{OccurredAt: rows[limit-1].OccurredAt, EventID: rows[limit-1].EventID}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return rows, next, nil
}
