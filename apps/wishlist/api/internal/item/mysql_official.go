package item

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"example.com/pokecalc/apps/wishlist/api/internal/store"
)

// フェーズ4-3 公式サイトの販売状況(official_status。migration 000008。docs/phase4-spec.md AC-O*)。

var _ OfficialRepository = (*MySQLRepository)(nil)

func toOfficial(row store.OfficialStatus) (OfficialStatus, error) {
	o := OfficialStatus{
		Status: OfficialState(row.Status), Evidence: []string{}, CheckedAt: row.CheckedAt.UTC(),
		LastResult: OfficialState(row.LastResult), LastAttemptAt: row.LastAttemptAt.UTC(),
	}
	if err := json.Unmarshal(row.Evidence, &o.Evidence); err != nil {
		return OfficialStatus{}, err
	}
	if o.Evidence == nil {
		o.Evidence = []string{}
	}
	if row.ChangedAt.Valid {
		t := row.ChangedAt.Time.UTC()
		o.ChangedAt = &t
	}
	if row.PreviousStatus.Valid {
		s := OfficialState(row.PreviousStatus.OfficialStatusPreviousStatus)
		o.PreviousStatus = &s
	}
	return o, nil
}

func officialParams(itemID int64, o OfficialStatus) (store.UpsertOfficialStatusParams, error) {
	ev, err := json.Marshal(o.Evidence)
	if err != nil {
		return store.UpsertOfficialStatusParams{}, err
	}
	p := store.UpsertOfficialStatusParams{
		ItemID: itemID, Status: store.OfficialStatusStatus(o.Status), Evidence: ev, CheckedAt: o.CheckedAt,
		LastResult: store.OfficialStatusLastResult(o.LastResult), LastAttemptAt: o.LastAttemptAt,
	}
	if o.ChangedAt != nil {
		p.ChangedAt = sql.NullTime{Time: *o.ChangedAt, Valid: true}
	}
	if o.PreviousStatus != nil {
		p.PreviousStatus = store.NullOfficialStatusPreviousStatus{OfficialStatusPreviousStatus: store.OfficialStatusPreviousStatus(*o.PreviousStatus), Valid: true}
	}
	return p, nil
}

// SaveOfficialCheck は OfficialRepository の実装。
func (r *MySQLRepository) SaveOfficialCheck(ctx context.Context, itemID int64, c OfficialCheck) (OfficialStatus, error) {
	var out OfficialStatus
	err := r.inTx(ctx, func(q *store.Queries) error {
		cur, err := q.GetItemForUpdate(ctx, itemID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if err := validateOfficialCheck(c); err != nil {
			return err
		}
		if c.SourceURL != "" && (!cur.SourceUrl.Valid || cur.SourceUrl.String != c.SourceURL) {
			return ErrSourceChanged
		}
		var prev *OfficialStatus
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
		out = MergeOfficial(prev, c)
		p, err := officialParams(itemID, out)
		if err != nil {
			return err
		}
		return q.UpsertOfficialStatus(ctx, p)
	})
	if err != nil {
		return OfficialStatus{}, err
	}
	return out, nil
}

// officialByItem は全商品の販売状況を商品 ID で引ける形で返す。
func officialByItem(ctx context.Context, q *store.Queries) (map[int64]OfficialStatus, error) {
	rows, err := q.ListOfficialStatuses(ctx)
	if err != nil {
		return nil, err
	}
	m := make(map[int64]OfficialStatus, len(rows))
	for _, row := range rows {
		o, err := toOfficial(row)
		if err != nil {
			return nil, err
		}
		m[row.ItemID] = o
	}
	return m, nil
}
