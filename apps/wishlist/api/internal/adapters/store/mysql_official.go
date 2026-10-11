package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"example.com/pokecalc/apps/wishlist/api/internal/adapters/store/sqlcgen"
	"example.com/pokecalc/apps/wishlist/api/internal/app/items"
	"example.com/pokecalc/apps/wishlist/api/internal/domain/item"
)

// フェーズ4-3 公式サイトの販売状況(official_status。migration 000008。docs/phase4-spec.md AC-O*)。

var _ items.OfficialRepository = (*MySQLRepository)(nil)

func toOfficial(row sqlcgen.OfficialStatus) (item.OfficialStatus, error) {
	o := item.OfficialStatus{
		Status: item.OfficialState(row.Status), Evidence: []string{}, CheckedAt: row.CheckedAt.UTC(),
		LastResult: item.OfficialState(row.LastResult), LastAttemptAt: row.LastAttemptAt.UTC(),
	}
	if err := json.Unmarshal(row.Evidence, &o.Evidence); err != nil {
		return item.OfficialStatus{}, err
	}
	if o.Evidence == nil {
		o.Evidence = []string{}
	}
	if row.ChangedAt.Valid {
		t := row.ChangedAt.Time.UTC()
		o.ChangedAt = &t
	}
	if row.PreviousStatus.Valid {
		s := item.OfficialState(row.PreviousStatus.OfficialStatusPreviousStatus)
		o.PreviousStatus = &s
	}
	return o, nil
}

func officialParams(itemID int64, o item.OfficialStatus) (sqlcgen.UpsertOfficialStatusParams, error) {
	ev, err := json.Marshal(o.Evidence)
	if err != nil {
		return sqlcgen.UpsertOfficialStatusParams{}, err
	}
	p := sqlcgen.UpsertOfficialStatusParams{
		ItemID: itemID, Status: sqlcgen.OfficialStatusStatus(o.Status), Evidence: ev, CheckedAt: o.CheckedAt,
		LastResult: sqlcgen.OfficialStatusLastResult(o.LastResult), LastAttemptAt: o.LastAttemptAt,
	}
	if o.ChangedAt != nil {
		p.ChangedAt = sql.NullTime{Time: *o.ChangedAt, Valid: true}
	}
	if o.PreviousStatus != nil {
		p.PreviousStatus = sqlcgen.NullOfficialStatusPreviousStatus{OfficialStatusPreviousStatus: sqlcgen.OfficialStatusPreviousStatus(*o.PreviousStatus), Valid: true}
	}
	return p, nil
}

// SaveOfficialCheck は OfficialRepository の実装。
func (r *MySQLRepository) SaveOfficialCheck(ctx context.Context, itemID int64, c item.OfficialCheck) (item.OfficialStatus, error) {
	var out item.OfficialStatus
	err := r.inTx(ctx, func(q *sqlcgen.Queries) error {
		cur, err := q.GetItemForUpdate(ctx, itemID)
		if errors.Is(err, sql.ErrNoRows) {
			return item.ErrNotFound
		} else if err != nil {
			return err
		}
		if err := item.ValidateOfficialCheck(c); err != nil {
			return err
		}
		if c.SourceURL != "" && (!cur.SourceUrl.Valid || cur.SourceUrl.String != c.SourceURL) {
			return item.ErrSourceChanged
		}
		var prev *item.OfficialStatus
		row, err := q.GetOfficialStatus(ctx, itemID)
		switch {
		case err == nil:
			o, err := toOfficial(row)
			if err != nil {
				return err
			}
			prev = &o
		case !errors.Is(err, sql.ErrNoRows):
			return err
		}
		out = item.MergeOfficial(prev, c)
		p, err := officialParams(itemID, out)
		if err != nil {
			return err
		}
		return q.UpsertOfficialStatus(ctx, p)
	})
	if err != nil {
		return item.OfficialStatus{}, err
	}
	return out, nil
}

// officialByItem は全商品の販売状況を商品 ID で引ける形で返す。
func officialByItem(ctx context.Context, q *sqlcgen.Queries) (map[int64]item.OfficialStatus, error) {
	rows, err := q.ListOfficialStatuses(ctx)
	if err != nil {
		return nil, err
	}
	m := make(map[int64]item.OfficialStatus, len(rows))
	for _, row := range rows {
		o, err := toOfficial(row)
		if err != nil {
			return nil, err
		}
		m[row.ItemID] = o
	}
	return m, nil
}
