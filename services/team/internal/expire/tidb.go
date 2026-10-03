package expire

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// tidbStore は Store の TiDB(MySQL 互換)実装。時刻は UTC で渡す(internal/store と同じ)。
type tidbStore struct {
	db *sql.DB
}

// NewTiDB は Store の TiDB 実装を作る。
func NewTiDB(db *sql.DB) Store { return &tidbStore{db: db} }

// noBusinessRows は「その端末に構築が1つも無い」条件(devices を外側に持つ文の中で使う)。
// team_members は構築に従属するが、構築の無いメンバーも「データ」に数える(取りこぼさない)。
const noBusinessRows = `NOT EXISTS (SELECT 1 FROM teams WHERE teams.device_id = devices.device_id)
	AND NOT EXISTS (SELECT 1 FROM team_members WHERE team_members.device_id = devices.device_id)`

// anyBusinessRows は noBusinessRows の否定。
const anyBusinessRows = `(EXISTS (SELECT 1 FROM teams WHERE teams.device_id = devices.device_id)
	OR EXISTS (SELECT 1 FROM team_members WHERE team_members.device_id = devices.device_id))`

func (s *tidbStore) exec(ctx context.Context, what, query string, args ...any) (int, error) {
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", what, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("%s: %w", what, err)
	}
	return int(n), nil
}

// DeleteInactiveTeams は構築とそのメンバーを同じトランザクションで消す(メンバーが先)。
// 対象の構築は FOR UPDATE で押さえるので、判定から削除の間に更新された構築は消えない。
func (s *tidbStore) DeleteInactiveTeams(ctx context.Context, cutoff time.Time, limit int) (int, int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("teams の失効: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `SELECT t.id, t.device_id FROM teams t LEFT JOIN devices d ON d.device_id = t.device_id
		WHERE GREATEST(COALESCE(d.last_seen_at, t.updated_at), t.updated_at) < ?
		LIMIT ? FOR UPDATE`, cutoff.UTC(), limit)
	if err != nil {
		return 0, 0, fmt.Errorf("teams の失効(対象の選択): %w", err)
	}
	var ids []any
	deviceSet := map[string]bool{}
	for rows.Next() {
		var id, device string
		if err := rows.Scan(&id, &device); err != nil {
			_ = rows.Close()
			return 0, 0, fmt.Errorf("teams の失効(対象の選択): %w", err)
		}
		ids = append(ids, id)
		deviceSet[device] = true
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, 0, fmt.Errorf("teams の失効(対象の選択): %w", err)
	}
	_ = rows.Close()
	if len(ids) == 0 {
		return 0, 0, nil
	}

	in := "(" + strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",") + ")"
	// team_members も device_id を持つので、条件に含める(internal/store と同じ流儀)。
	devices := make([]any, 0, len(deviceSet))
	for d := range deviceSet {
		devices = append(devices, d)
	}
	devIn := "(" + strings.TrimSuffix(strings.Repeat("?,", len(devices)), ",") + ")"
	mres, err := tx.ExecContext(ctx, `DELETE FROM team_members WHERE team_id IN `+in+` AND device_id IN `+devIn,
		append(append([]any{}, ids...), devices...)...)
	if err != nil {
		return 0, 0, fmt.Errorf("team_members の失効: %w", err)
	}
	members, err := mres.RowsAffected()
	if err != nil {
		return 0, 0, fmt.Errorf("team_members の失効: %w", err)
	}
	tres, err := tx.ExecContext(ctx, `DELETE FROM teams WHERE id IN `+in, ids...)
	if err != nil {
		return 0, 0, fmt.Errorf("teams の失効: %w", err)
	}
	teams, err := tres.RowsAffected()
	if err != nil {
		return 0, 0, fmt.Errorf("teams の失効: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("teams の失効(commit): %w", err)
	}
	return int(teams), int(members), nil
}

func (s *tidbStore) DeletePurgeJournal(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	return s.exec(ctx, "purge_journal の失効", `DELETE FROM purge_journal WHERE requested_at < ? LIMIT ?`, cutoff.UTC(), limit)
}

// MarkOrphans は構築が無くなった端末に orphaned_since = now を付け、構築が戻った端末は NULL に戻す
// (ADR-0211 §6)。それぞれ最大 limit 件。
func (s *tidbStore) MarkOrphans(ctx context.Context, now time.Time, limit int) (int, int, error) {
	marked, err := s.exec(ctx, "orphaned_since の印付け",
		`UPDATE devices SET orphaned_since = ? WHERE orphaned_since IS NULL AND `+noBusinessRows+` LIMIT ?`, now.UTC(), limit)
	if err != nil {
		return 0, 0, err
	}
	cleared, err := s.exec(ctx, "orphaned_since の解除",
		`UPDATE devices SET orphaned_since = NULL WHERE orphaned_since IS NOT NULL AND `+anyBusinessRows+` LIMIT ?`, limit)
	if err != nil {
		return marked, 0, err
	}
	return marked, cleared, nil
}

// DeleteOrphanDevices は devices 行を消す。orphaned_since・last_seen_at が cutoff より前で、墓石が無いか
// cutoff より前で、いま構築が無い端末だけ(ADR-0220 §4 の 6)。最後の条件も同じ文で確かめる。
func (s *tidbStore) DeleteOrphanDevices(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	c := cutoff.UTC()
	return s.exec(ctx, "devices 行の失効", `DELETE FROM devices
		WHERE orphaned_since < ? AND last_seen_at < ? AND (purged_at IS NULL OR purged_at < ?)
		AND `+noBusinessRows+` LIMIT ?`, c, c, c, limit)
}
