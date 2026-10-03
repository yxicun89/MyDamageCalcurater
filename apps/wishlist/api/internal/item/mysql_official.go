package item

import "context"

// フェーズ4-3 公式サイトの販売状況(official_status。migration 000008。docs/phase4-spec.md AC-O*)。

var _ OfficialRepository = (*MySQLRepository)(nil)

// SaveOfficialCheck は OfficialRepository の実装。
func (r *MySQLRepository) SaveOfficialCheck(ctx context.Context, itemID int64, c OfficialCheck) (OfficialStatus, error) {
	return OfficialStatus{}, errNotImplemented // TODO(implementer)
}
