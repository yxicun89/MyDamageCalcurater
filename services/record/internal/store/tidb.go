// TiDB(MySQL 互換)への Store の実装(ADR-0209・ADR-0211)。
//
// すべてのクエリは device_id で絞る(package doc の規則)。TiDB に届かない失敗はすべて
// ErrUnavailable で包んで返し、httpapi / events はそれを見て 503 / Nak に写す。DB のエラー文自体は
// ログにだけ残し、クライアントへは出さない(package doc・ADR-0105 §2 と同じ扱い)。
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/go-sql-driver/mysql"
)

// errMySQLDuplicateEntry は MySQL/TiDB の一意制約違反のエラー番号(ER_DUP_ENTRY)。
const errMySQLDuplicateEntry = 1062

// isDuplicateKeyError は err が一意制約違反(重複挿入)かどうかを返す(critic 指摘 R-5)。
func isDuplicateKeyError(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == errMySQLDuplicateEntry
}

// lastSeenWriteThreshold は devices.last_seen_at の書き込み抑止(ADR-0209 §4 AC-R5)。
// 保存済みの値からこの時間が経っていなければ書かない。
const lastSeenWriteThreshold = 24 * time.Hour

// businessTables は PurgeDevice が (2) → (3) → (4) の順に消すテーブル(ADR-0209 §5.2)。
var businessTables = []string{"calc_events", "frequent_opponents", "favorites"}

// TiDBStore は Store の TiDB(MySQL 互換)実装。ゼロ値は使わない(New で作る)。
type TiDBStore struct {
	db            *sql.DB
	decayHalfLife time.Duration
	purgeBatch    int
}

var _ Store = (*TiDBStore)(nil)

// New は TiDBStore を作る。decayHalfLife は「よく使う相手」の時間減衰の半減期
// (ADR-0209 §4。生イベントの保持期間より短いことは呼び出し側〈cmd/record の loadConfig〉が検証する)。
// purgeBatch は PurgeDevice が1回で消す行数の上限(ADR-0209 §5.2)。
func New(db *sql.DB, decayHalfLife time.Duration, purgeBatch int) *TiDBStore {
	return &TiDBStore{db: db, decayHalfLife: decayHalfLife, purgeBatch: purgeBatch}
}

// wrapUnavailable は DB 由来の失敗を ErrUnavailable で包む(err が nil ならそのまま nil)。
func wrapUnavailable(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("record store: %w: %w", ErrUnavailable, err)
}

// Ping は TiDB への到達可否だけを確かめる(record-svc の /readyz が使う。Store インターフェースには
// 含めない — httpapi は型アサーションで使い、fake には無いので httpapi のテストは
// FrequentOpponents 経由の判定にフォールバックする)。
func (s *TiDBStore) Ping(ctx context.Context) error {
	return wrapUnavailable(s.db.PingContext(ctx))
}

// TouchDevice は devices.last_seen_at を upsert する。24時間以内なら書かない(AC-R5)。
func (s *TiDBStore) TouchDevice(ctx context.Context, deviceID string, now time.Time) error {
	const q = `
		INSERT INTO devices (device_id, last_seen_at) VALUES (?, ?)
		ON DUPLICATE KEY UPDATE last_seen_at = IF(
			TIMESTAMPDIFF(SECOND, last_seen_at, VALUES(last_seen_at)) >= ?,
			VALUES(last_seen_at), last_seen_at)
	`
	_, err := s.db.ExecContext(ctx, q, deviceID, now.UTC(), int(lastSeenWriteThreshold.Seconds()))
	return wrapUnavailable(err)
}

// FrequentOpponents は device_id の集計を読み、時間減衰を現在時刻まで進めてから並べ替えて返す
// (frequent_opponents は last_calculated_at 時点までの score しか持たないため。ADR-0209 §4)。
func (s *TiDBStore) FrequentOpponents(ctx context.Context, deviceID string, limit int) ([]FrequentOpponent, error) {
	const q = `SELECT species_key, score, count, last_calculated_at FROM frequent_opponents WHERE device_id = ?`
	rows, err := s.db.QueryContext(ctx, q, deviceID)
	if err != nil {
		return nil, wrapUnavailable(err)
	}
	defer rows.Close()

	now := time.Now().UTC()
	out := make([]FrequentOpponent, 0)
	for rows.Next() {
		var fo FrequentOpponent
		var lastCalc time.Time
		if err := rows.Scan(&fo.SpeciesKey, &fo.Score, &fo.Count, &lastCalc); err != nil {
			return nil, wrapUnavailable(err)
		}
		fo.LastCalculatedAt = lastCalc
		fo.Score *= decayFactor(now.Sub(lastCalc), s.decayHalfLife)
		out = append(out, fo)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapUnavailable(err)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].SpeciesKey < out[j].SpeciesKey
	})
	if limit >= 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// decayFactor は経過時間 elapsed(≧0 とみなす)に対する半減期 halfLife の減衰係数(2^(-elapsed/halfLife))。
func decayFactor(elapsed, halfLife time.Duration) float64 {
	if halfLife <= 0 {
		return 1
	}
	if elapsed < 0 {
		elapsed = 0
	}
	return math.Pow(2, -elapsed.Seconds()/halfLife.Seconds())
}

// PurgeDevice は ADR-0209 §5.2 の順序で消す: (1) 墓石 + journal → (2) calc_events → (3) 集計 → (4) favorites。
// (1) は他のステップと別の commit にする(critic 指摘なら別。途中で DB が落ちても墓石だけは残ってほしい
// ため。TestDeleteDeviceDataStoreUnavailable が固定する挙動)。
func (s *TiDBStore) PurgeDevice(ctx context.Context, deviceID string, now time.Time) (PurgeResult, error) {
	now = now.UTC()
	if err := s.setTombstone(ctx, deviceID, now); err != nil {
		return PurgeResult{}, wrapUnavailable(err)
	}

	budget := s.purgeBatch
	var del Deleted
	var err error
	if del.CalcEvents, budget, err = deleteBatch(ctx, s.db, "calc_events", deviceID, budget); err != nil {
		return PurgeResult{}, wrapUnavailable(err)
	}
	if del.Aggregates, budget, err = deleteBatch(ctx, s.db, "frequent_opponents", deviceID, budget); err != nil {
		return PurgeResult{}, wrapUnavailable(err)
	}
	if del.Favorites, _, err = deleteBatch(ctx, s.db, "favorites", deviceID, budget); err != nil {
		return PurgeResult{}, wrapUnavailable(err)
	}

	remaining, err := s.hasRemaining(ctx, deviceID)
	if err != nil {
		return PurgeResult{}, wrapUnavailable(err)
	}
	return PurgeResult{PurgedAt: now, Deleted: del, Remaining: remaining}, nil
}

// setTombstone は devices.purged_at を now に更新し(行が無ければ作り)、purge_journal に追記する
// (同じトランザクションで。ADR-0209 §5b・§7)。
func (s *TiDBStore) setTombstone(ctx context.Context, deviceID string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // Commit 後は no-op

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO devices (device_id, last_seen_at, purged_at) VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE purged_at = VALUES(purged_at)
	`, deviceID, now, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO purge_journal (device_id, requested_at) VALUES (?, ?)`, deviceID, now); err != nil {
		return err
	}
	return tx.Commit()
}

// deleteBatch は table から device_id = ? の行を budget を上限に消す。budget が尽きていれば何もしない。
func deleteBatch(ctx context.Context, db *sql.DB, table, deviceID string, budget int) (deleted, remainingBudget int, err error) {
	if budget <= 0 {
		return 0, budget, nil
	}
	res, err := db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE device_id = ? LIMIT ?", table), deviceID, budget)
	if err != nil {
		return 0, budget, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, budget, err
	}
	return int(n), budget - int(n), nil
}

// hasRemaining は device_id の行が businessTables のどれかにまだ残っているかを返す。
func (s *TiDBStore) hasRemaining(ctx context.Context, deviceID string) (bool, error) {
	for _, table := range businessTables {
		var exists int
		err := s.db.QueryRowContext(ctx, fmt.Sprintf("SELECT 1 FROM %s WHERE device_id = ? LIMIT 1", table), deviceID).Scan(&exists)
		switch {
		case err == nil:
			return true, nil
		case errors.Is(err, sql.ErrNoRows):
			continue
		default:
			return false, err
		}
	}
	return false, nil
}

// SaveCalcEvent は1件のイベントを保存し、集計と devices.last_seen_at へ反映する(冪等。ADR-0212 §6)。
func (s *TiDBStore) SaveCalcEvent(ctx context.Context, ev CalcEvent) (SaveOutcome, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, wrapUnavailable(err)
	}
	defer tx.Rollback() //nolint:errcheck // Commit 後は no-op

	var purgedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT purged_at FROM devices WHERE device_id = ?`, ev.DeviceID).Scan(&purgedAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, wrapUnavailable(err)
	}
	if purgedAt.Valid && !ev.OccurredAt.After(purgedAt.Time) {
		if err := tx.Commit(); err != nil {
			return 0, wrapUnavailable(err)
		}
		return Tombstoned, nil
	}

	payload := ev.Payload
	if len(payload) == 0 {
		payload = []byte("{}")
	}
	// INSERT IGNORE ではなく素の INSERT + 一意制約違反(MySQL/TiDB エラー1062)の判定にする
	// (critic 指摘 R-5)。INSERT IGNORE は event_id の重複以外の制約違反(NOT NULL 違反等)も
	// 同じように黙って無視してしまい、想定外の失敗が Duplicate に化けて気づけなくなるため。
	_, err = tx.ExecContext(ctx, `
		INSERT INTO calc_events
			(event_id, device_id, session_id, operation, occurred_at, defender_species_key, payload, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, ev.EventID, ev.DeviceID, ev.SessionID, ev.Operation, ev.OccurredAt, ev.DefenderSpeciesKey, payload, time.Now().UTC())
	if err != nil {
		if isDuplicateKeyError(err) {
			// event_id が既に存在する(at-least-once の再配送)。
			if cerr := tx.Commit(); cerr != nil {
				return 0, wrapUnavailable(cerr)
			}
			return Duplicate, nil
		}
		return 0, wrapUnavailable(err)
	}

	if ev.DefenderSpeciesKey != "" {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO frequent_opponents (device_id, species_key, score, count, last_calculated_at)
			VALUES (?, ?, 1, 1, ?)
			ON DUPLICATE KEY UPDATE
				score = score * POW(2, -GREATEST(0, TIMESTAMPDIFF(SECOND, last_calculated_at, VALUES(last_calculated_at))) / ?) + 1,
				count = count + 1,
				last_calculated_at = GREATEST(last_calculated_at, VALUES(last_calculated_at))
		`, ev.DeviceID, ev.DefenderSpeciesKey, ev.OccurredAt, s.decayHalfLife.Seconds()); err != nil {
			return 0, wrapUnavailable(err)
		}
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO devices (device_id, last_seen_at) VALUES (?, ?)
		ON DUPLICATE KEY UPDATE last_seen_at = IF(
			TIMESTAMPDIFF(SECOND, last_seen_at, VALUES(last_seen_at)) >= ?,
			VALUES(last_seen_at), last_seen_at)
	`, ev.DeviceID, ev.OccurredAt, int(lastSeenWriteThreshold.Seconds())); err != nil {
		return 0, wrapUnavailable(err)
	}

	if err := tx.Commit(); err != nil {
		return 0, wrapUnavailable(err)
	}
	return Stored, nil
}
