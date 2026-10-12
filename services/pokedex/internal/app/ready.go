package app

import (
	"context"
	"errors"

	"example.com/pokecalc/services/pokedex/internal/domain"
)

// Ready は readiness(GET /readyz。ADR-0129 §1)の手順。最小条件の4クエリを読み取り専用 Tx の中で読み、
// Tx を開けない・失敗・空・既定レギュレーション無しなら 503 master_unavailable。キャッシュしない。
// Tx は必ず閉じる(開いたままの Tx を残さない)。締め切りは呼び出し側が ctx に掛ける。
func (s *Service) Ready(ctx context.Context) error {
	tx, err := s.store.BeginReadOnly(ctx)
	if err != nil {
		return domain.Unavailable("readyz: BeginTx", err)
	}
	defer func() { _ = tx.Rollback() }()
	if dv, err := tx.ListDataVersions(ctx); err != nil {
		return domain.Unavailable("readyz: data_versions", err)
	} else if len(dv) == 0 {
		return domain.Unavailable("readyz: data_versions が空", nil)
	}
	if ts, err := tx.ListTypes(ctx); err != nil {
		return domain.Unavailable("readyz: types", err)
	} else if len(ts) == 0 {
		return domain.Unavailable("readyz: types が空", nil)
	}
	if ns, err := tx.ListNatures(ctx); err != nil {
		return domain.Unavailable("readyz: natures", err)
	} else if len(ns) == 0 {
		return domain.Unavailable("readyz: natures が空", nil)
	}
	if _, err := tx.GetDefaultRegulation(ctx); err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Unavailable("readyz: 既定のレギュレーションが無い", err)
		}
		return domain.Unavailable("readyz: regulation", err)
	}
	return nil
}
