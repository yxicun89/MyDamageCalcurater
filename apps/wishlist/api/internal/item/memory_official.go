package item

import (
	"context"
	"slices"
)

// フェーズ4-3 公式サイトの販売状況(docs/phase4-spec.md AC-O*)。

var _ OfficialRepository = (*MemoryRepository)(nil)

func cloneOfficial(o OfficialStatus) OfficialStatus {
	o.Evidence = slices.Clone(o.Evidence)
	if o.Evidence == nil {
		o.Evidence = []string{}
	}
	o.ChangedAt = cloneP(o.ChangedAt)
	o.PreviousStatus = cloneP(o.PreviousStatus)
	return o
}

// SaveOfficialCheck は OfficialRepository の実装。
func (m *MemoryRepository) SaveOfficialCheck(_ context.Context, itemID int64, c OfficialCheck) (OfficialStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	it, ok := m.items[itemID]
	if !ok {
		return OfficialStatus{}, ErrNotFound
	}
	if err := validateOfficialCheck(c); err != nil {
		return OfficialStatus{}, err
	}
	if c.SourceURL != "" && (it.SourceURL == nil || *it.SourceURL != c.SourceURL) {
		return OfficialStatus{}, ErrSourceChanged
	}
	var prev *OfficialStatus
	if cur, ok := m.officials[itemID]; ok {
		prev = &cur
	}
	next := MergeOfficial(prev, c)
	m.officials[itemID] = cloneOfficial(next)
	return cloneOfficial(next), nil
}

// withOfficial は it に保存済みの販売状況を載せる。呼び出し側が m.mu を持つこと。
func (m *MemoryRepository) withOfficial(it Item) Item {
	if o, ok := m.officials[it.ID]; ok {
		c := cloneOfficial(o)
		it.Official = &c
	} else {
		it.Official = nil
	}
	return it
}
