package expire

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// tidbStore は Store の TiDB(MySQL 互換)実装。時刻は UTC で渡す(internal/store と同じ)。
type tidbStore struct {
	db *sql.DB
}

// NewTiDB は Store の TiDB 実装を作る。
func NewTiDB(db *sql.DB) Store { return &tidbStore{db: db} }

// noBusinessRows は「その端末に業務テーブルの行が1つも無い」条件(devices を外側に持つ文の中で使う)。
const noBusinessRows = `NOT EXISTS (SELECT 1 FROM calc_events WHERE calc_events.device_id = devices.device_id)
	AND NOT EXISTS (SELECT 1 FROM frequent_opponents WHERE frequent_opponents.device_id = devices.device_id)
	AND NOT EXISTS (SELECT 1 FROM favorites WHERE favorites.device_id = devices.device_id)`

// anyBusinessRows は noBusinessRows の否定。
const anyBusinessRows = `(EXISTS (SELECT 1 FROM calc_events WHERE calc_events.device_id = devices.device_id)
	OR EXISTS (SELECT 1 FROM frequent_opponents WHERE frequent_opponents.device_id = devices.device_id)
	OR EXISTS (SELECT 1 FROM favorites WHERE favorites.device_id = devices.device_id))`

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

func (s *tidbStore) DeleteCalcEvents(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	return s.exec(ctx, "calc_events の失効", `DELETE FROM calc_events WHERE occurred_at < ? LIMIT ?`, cutoff.UTC(), limit)
}

func (s *tidbStore) DeleteStaleAggregates(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	return s.exec(ctx, "frequent_opponents の失効", `DELETE FROM frequent_opponents WHERE last_calculated_at < ? LIMIT ?`, cutoff.UTC(), limit)
}

// DeleteInactiveFavorites は max(devices.last_seen_at, favorites.updated_at) < cutoff の行を消す。
// devices 行が無い端末は updated_at だけで判定する。LIMIT は複数表の DELETE に付けられないので、
// 派生表で対象の id を絞ってから1文で消す。外側にも `updated_at < cutoff` を足し、判定から削除の間に更新された行は消さない
// (派生表で読んだ devices.last_seen_at はロックされないので、その間に端末が使われ始める理論上ごく短い隙間は残る。
// 次の削除は24時間後のジョブの1回だけで、影響は1行の早期失効にとどまる)。
func (s *tidbStore) DeleteInactiveFavorites(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	return s.exec(ctx, "favorites の失効", `DELETE FROM favorites WHERE id IN (
		SELECT id FROM (
			SELECT f.id FROM favorites f LEFT JOIN devices d ON d.device_id = f.device_id
			WHERE GREATEST(COALESCE(d.last_seen_at, f.updated_at), f.updated_at) < ?
			LIMIT ?
		) AS expired) AND updated_at < ?`, cutoff.UTC(), limit, cutoff.UTC())
}

func (s *tidbStore) DeletePurgeJournal(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	return s.exec(ctx, "purge_journal の失効", `DELETE FROM purge_journal WHERE requested_at < ? LIMIT ?`, cutoff.UTC(), limit)
}

// MarkOrphans は業務テーブルに行が無くなった端末に orphaned_since = now を付け、行が戻った端末は NULL に戻す
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
// cutoff より前で、いま業務テーブルに行が無い端末だけ(ADR-0220 §4 の 6)。最後の条件も同じ文で確かめる。
func (s *tidbStore) DeleteOrphanDevices(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	c := cutoff.UTC()
	return s.exec(ctx, "devices 行の失効", `DELETE FROM devices
		WHERE orphaned_since < ? AND last_seen_at < ? AND (purged_at IS NULL OR purged_at < ?)
		AND `+noBusinessRows+` LIMIT ?`, c, c, c, limit)
}
