package item

import "context"

// フェーズ4-3 公式サイトの販売状況(docs/phase4-spec.md AC-O*)。

var _ OfficialRepository = (*MemoryRepository)(nil)

// SaveOfficialCheck は OfficialRepository の実装。
func (m *MemoryRepository) SaveOfficialCheck(_ context.Context, itemID int64, c OfficialCheck) (OfficialStatus, error) {
	return OfficialStatus{}, errNotImplemented // TODO(implementer)
}
