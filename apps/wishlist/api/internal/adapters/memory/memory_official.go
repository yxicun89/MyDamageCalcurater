package memory

import (
	"context"
	"slices"

	"example.com/pokecalc/apps/wishlist/api/internal/app/items"
	"example.com/pokecalc/apps/wishlist/api/internal/domain/item"
)

// フェーズ4-3 公式サイトの販売状況(docs/phase4-spec.md AC-O*)。

var _ items.OfficialRepository = (*MemoryRepository)(nil)

func cloneOfficial(o item.OfficialStatus) item.OfficialStatus {
	o.Evidence = slices.Clone(o.Evidence)
	if o.Evidence == nil {
		o.Evidence = []string{}
	}
	o.ChangedAt = item.CloneP(o.ChangedAt)
	o.PreviousStatus = item.CloneP(o.PreviousStatus)
	return o
}

// SaveOfficialCheck は OfficialRepository の実装。
func (m *MemoryRepository) SaveOfficialCheck(_ context.Context, itemID int64, c item.OfficialCheck) (item.OfficialStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	it, ok := m.items[itemID]
	if !ok {
		return item.OfficialStatus{}, item.ErrNotFound
	}
	if err := item.ValidateOfficialCheck(c); err != nil {
		return item.OfficialStatus{}, err
	}
	if c.SourceURL != "" && (it.SourceURL == nil || *it.SourceURL != c.SourceURL) {
		return item.OfficialStatus{}, item.ErrSourceChanged
	}
	var prev *item.OfficialStatus
	if cur, ok := m.officials[itemID]; ok {
		prev = &cur
	}
	next := item.MergeOfficial(prev, c)
	m.officials[itemID] = cloneOfficial(next)
	return cloneOfficial(next), nil
}

// withOfficial は it に保存済みの販売状況を載せる。呼び出し側が m.mu を持つこと。
func (m *MemoryRepository) withOfficial(it item.Item) item.Item {
	if o, ok := m.officials[it.ID]; ok {
		c := cloneOfficial(o)
		it.Official = &c
	} else {
		it.Official = nil
	}
	return it
}
