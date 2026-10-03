package item

import (
	"context"
	"time"
)

// ListPriceHistory は PriceHistoryRepository の実装。
func (m *MemoryRepository) ListPriceHistory(_ context.Context, itemID int64, since time.Time) ([]PricePoint, error) {
	_, _ = itemID, since
	return []PricePoint{}, nil // TODO(implementer): docs/phase4-spec.md AC-H2〜H5
}

// PrunePriceHistory は PriceHistoryRepository の実装。
func (m *MemoryRepository) PrunePriceHistory(_ context.Context, before time.Time) (int64, error) {
	_ = before
	return 0, nil // TODO(implementer): docs/phase4-spec.md AC-H6
}
