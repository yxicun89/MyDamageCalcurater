// TiDB(MySQL 互換)への Store の実装(ADR-0209・ADR-0211・ADR-0213)。
//
// すべてのクエリは device_id で絞る(package doc の規則)。TiDB に届かない失敗はすべて
// ErrUnavailable で包んで返し、httpapi / events はそれを見て 503 / Nak に写す。DB のエラー文自体は
// ログにだけ残し、クライアントへは出さない(package doc・ADR-0105 §2 と同じ扱い)。
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// lastSeenWriteThreshold は devices.last_seen_at の書き込み抑止(ADR-0209 §4 AC-R5)。
// 保存済みの値からこの時間が経っていなければ書かない。
const lastSeenWriteThreshold = 24 * time.Hour

// businessTables は PurgeDevice が (2) → (3) の順に消すテーブル(ADR-0209 §5.2)。
var businessTables = []string{"team_members", "teams"}

// TiDBStore は Store の TiDB(MySQL 互換)実装。ゼロ値は使わない(New で作る)。
type TiDBStore struct {
	db         *sql.DB
	purgeBatch int
}

var _ Store = (*TiDBStore)(nil)

// New は TiDBStore を作る。purgeBatch は PurgeDevice が1回で消す行数の上限(ADR-0209 §5.2)。
func New(db *sql.DB, purgeBatch int) *TiDBStore {
	return &TiDBStore{db: db, purgeBatch: purgeBatch}
}

// wrapUnavailable は DB 由来の失敗を ErrUnavailable で包む(err が nil ならそのまま nil)。
func wrapUnavailable(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("team store: %w: %w", ErrUnavailable, err)
}

// Ping は TiDB への到達可否だけを確かめる(httpapi の /readyz が使う。Store インターフェースには
// 含めない — httpapi は型アサーションで使い、fake には無いので httpapi のテストは
// ListTeams 経由の判定にフォールバックする)。
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

// TouchDeviceFromEvent は devices.last_seen_at を occurredAt で更新する(ADR-0209 §4・§7)。
// occurredAt <= devices.purged_at なら Tombstoned、24時間以内なら Skipped、それ以外は更新して Touched。
func (s *TiDBStore) TouchDeviceFromEvent(ctx context.Context, deviceID string, occurredAt time.Time) (TouchOutcome, error) {
	occurredAt = occurredAt.UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, wrapUnavailable(err)
	}
	defer tx.Rollback() //nolint:errcheck // Commit 後は no-op

	var lastSeenAt sql.NullTime
	var purgedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT last_seen_at, purged_at FROM devices WHERE device_id = ? FOR UPDATE`, deviceID).
		Scan(&lastSeenAt, &purgedAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, wrapUnavailable(err)
	}

	if purgedAt.Valid && !occurredAt.After(purgedAt.Time) {
		if err := tx.Commit(); err != nil {
			return 0, wrapUnavailable(err)
		}
		return Tombstoned, nil
	}
	if lastSeenAt.Valid && occurredAt.Sub(lastSeenAt.Time) < lastSeenWriteThreshold {
		if err := tx.Commit(); err != nil {
			return 0, wrapUnavailable(err)
		}
		return Skipped, nil
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO devices (device_id, last_seen_at) VALUES (?, ?)
		ON DUPLICATE KEY UPDATE last_seen_at = VALUES(last_seen_at)
	`, deviceID, occurredAt); err != nil {
		return 0, wrapUnavailable(err)
	}
	if err := tx.Commit(); err != nil {
		return 0, wrapUnavailable(err)
	}
	return Touched, nil
}

// ListTeams はその端末の構築を UpdatedAt の降順(同時刻は ID の昇順)で全件返す。
func (s *TiDBStore) ListTeams(ctx context.Context, deviceID string) ([]Team, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, created_at, updated_at FROM teams
		WHERE device_id = ? ORDER BY updated_at DESC, id ASC
	`, deviceID)
	if err != nil {
		return nil, wrapUnavailable(err)
	}
	defer rows.Close()

	teams := make([]Team, 0)
	for rows.Next() {
		var t Team
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, wrapUnavailable(err)
		}
		teams = append(teams, t)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapUnavailable(err)
	}

	for i := range teams {
		members, err := s.loadMembers(ctx, deviceID, teams[i].ID)
		if err != nil {
			return nil, err
		}
		teams[i].Members = members
	}
	return teams, nil
}

// GetTeam はその端末の構築を1件返す。その端末が持っていなければ ErrNotFound(§6-2)。
func (s *TiDBStore) GetTeam(ctx context.Context, deviceID, teamID string) (Team, error) {
	var t Team
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, created_at, updated_at FROM teams WHERE device_id = ? AND id = ?
	`, deviceID, teamID).Scan(&t.ID, &t.Name, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Team{}, ErrNotFound
	}
	if err != nil {
		return Team{}, wrapUnavailable(err)
	}
	members, err := s.loadMembers(ctx, deviceID, teamID)
	if err != nil {
		return Team{}, err
	}
	t.Members = members
	return t, nil
}

// loadMembers はその構築のメンバーを slot 順(= パーティの並び順)で返す。ADR-0209 §6-1: team_members
// も device_id で絞る(team_id が呼び出し側で他端末のものと確認済みであることに頼らない)。
func (s *TiDBStore) loadMembers(ctx context.Context, deviceID, teamID string) ([]Member, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT species_key, nickname, move_ids, item_id, ability_id, nature_id,
			sp_hp, sp_atk, sp_def, sp_spa, sp_spd, sp_spe, tera_type
		FROM team_members WHERE device_id = ? AND team_id = ? ORDER BY slot ASC
	`, deviceID, teamID)
	if err != nil {
		return nil, wrapUnavailable(err)
	}
	defer rows.Close()

	members := make([]Member, 0)
	for rows.Next() {
		var m Member
		var nickname, itemID, abilityID, teraType sql.NullString
		var moveJSON []byte
		if err := rows.Scan(&m.SpeciesKey, &nickname, &moveJSON, &itemID, &abilityID, &m.NatureID,
			&m.SP.HP, &m.SP.Atk, &m.SP.Def, &m.SP.Spa, &m.SP.Spd, &m.SP.Spe, &teraType); err != nil {
			return nil, wrapUnavailable(err)
		}
		if nickname.Valid {
			v := nickname.String
			m.Nickname = &v
		}
		if itemID.Valid {
			v := itemID.String
			m.ItemID = &v
		}
		if abilityID.Valid {
			v := abilityID.String
			m.AbilityID = &v
		}
		if teraType.Valid {
			v := teraType.String
			m.TeraType = &v
		}
		if len(moveJSON) > 0 {
			var ids []string
			if err := json.Unmarshal(moveJSON, &ids); err != nil {
				return nil, wrapUnavailable(err)
			}
			m.MoveIDs = ids
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapUnavailable(err)
	}
	return members, nil
}

// CreateTeam は構築を1件作る(ID・時刻は store が決める)。件数の確認と挿入は同じトランザクションで
// 行い、MaxTeamsPerDevice をすり抜けさせない。
func (s *TiDBStore) CreateTeam(ctx context.Context, deviceID string, t Team, now time.Time) (Team, error) {
	now = now.UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Team{}, wrapUnavailable(err)
	}
	defer tx.Rollback() //nolint:errcheck // Commit 後は no-op

	// 注意(critic 指摘・軽微2): `SELECT ... FOR UPDATE` は既存行をロックするだけで、TiDB はギャップ
	// ロックを持たないため、同じ端末からの同時 CreateTeam を厳密には排他できない(既存行が0件のときは
	// 何もロックされない)。実害は「ごく短い競合で上限をわずかに超える」程度で、v1(個人利用+単一
	// クライアント想定)ではブロッカーにしない。厳密にするなら devices の行を先に FOR UPDATE で
	// ロックしてから数える形にする(devices は必ず存在するとは限らないため INSERT ... ON DUPLICATE
	// KEY UPDATE で行を用意してからロックする必要があり、TouchDevice との競合も考慮が要る)。
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM teams WHERE device_id = ? FOR UPDATE`, deviceID).Scan(&count); err != nil {
		return Team{}, wrapUnavailable(err)
	}
	if count >= MaxTeamsPerDevice {
		return Team{}, ErrTeamLimitReached
	}

	id := uuid.NewString()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO teams (id, device_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)
	`, id, deviceID, t.Name, now, now); err != nil {
		return Team{}, wrapUnavailable(err)
	}
	if err := insertMembers(ctx, tx, id, deviceID, t.Members); err != nil {
		return Team{}, wrapUnavailable(err)
	}
	if err := tx.Commit(); err != nil {
		return Team{}, wrapUnavailable(err)
	}

	t.ID, t.CreatedAt, t.UpdatedAt = id, now, now
	return t, nil
}

// UpdateTeam はその端末の構築の名前とメンバー全体を置き換える(部分更新はしない)。
func (s *TiDBStore) UpdateTeam(ctx context.Context, deviceID, teamID string, t Team, now time.Time) (Team, error) {
	now = now.UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Team{}, wrapUnavailable(err)
	}
	defer tx.Rollback() //nolint:errcheck // Commit 後は no-op

	var createdAt time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT created_at FROM teams WHERE device_id = ? AND id = ? FOR UPDATE
	`, deviceID, teamID).Scan(&createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Team{}, ErrNotFound
	}
	if err != nil {
		return Team{}, wrapUnavailable(err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE teams SET name = ?, updated_at = ? WHERE device_id = ? AND id = ?
	`, t.Name, now, deviceID, teamID); err != nil {
		return Team{}, wrapUnavailable(err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM team_members WHERE device_id = ? AND team_id = ?`, deviceID, teamID); err != nil {
		return Team{}, wrapUnavailable(err)
	}
	if err := insertMembers(ctx, tx, teamID, deviceID, t.Members); err != nil {
		return Team{}, wrapUnavailable(err)
	}
	if err := tx.Commit(); err != nil {
		return Team{}, wrapUnavailable(err)
	}

	t.ID = teamID
	t.CreatedAt = createdAt.UTC()
	t.UpdatedAt = now
	return t, nil
}

// insertMembers は members を slot = 配列の添字で挿入する(並び順の保存。ADR-0213 §3)。
func insertMembers(ctx context.Context, tx *sql.Tx, teamID, deviceID string, members []Member) error {
	for i, m := range members {
		moveIDs := m.MoveIDs
		if moveIDs == nil {
			moveIDs = []string{}
		}
		moveJSON, err := json.Marshal(moveIDs)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO team_members
				(team_id, slot, device_id, species_key, nickname, move_ids, item_id, ability_id, nature_id,
				 sp_hp, sp_atk, sp_def, sp_spa, sp_spd, sp_spe, tera_type)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, teamID, i, deviceID, m.SpeciesKey, m.Nickname, moveJSON, m.ItemID, m.AbilityID, m.NatureID,
			m.SP.HP, m.SP.Atk, m.SP.Def, m.SP.Spa, m.SP.Spd, m.SP.Spe, m.TeraType); err != nil {
			return err
		}
	}
	return nil
}

// DeleteTeam はその端末の構築を1件消す(team_members も一緒に消す)。
func (s *TiDBStore) DeleteTeam(ctx context.Context, deviceID, teamID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return wrapUnavailable(err)
	}
	defer tx.Rollback() //nolint:errcheck // Commit 後は no-op

	res, err := tx.ExecContext(ctx, `DELETE FROM teams WHERE device_id = ? AND id = ?`, deviceID, teamID)
	if err != nil {
		return wrapUnavailable(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return wrapUnavailable(err)
	}
	if n == 0 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM team_members WHERE device_id = ? AND team_id = ?`, deviceID, teamID); err != nil {
		return wrapUnavailable(err)
	}
	return wrapUnavailable(tx.Commit())
}

// PurgeDevice は ADR-0209 §5.2 の順序で消す: (1) 墓石 + journal → (2) team_members → (3) teams。
func (s *TiDBStore) PurgeDevice(ctx context.Context, deviceID string, now time.Time) (PurgeResult, error) {
	now = now.UTC()
	if err := s.setTombstone(ctx, deviceID, now); err != nil {
		return PurgeResult{}, wrapUnavailable(err)
	}

	budget := s.purgeBatch
	var del Deleted
	var err error
	if del.TeamMembers, budget, err = deleteBatch(ctx, s.db, "team_members", deviceID, budget); err != nil {
		return PurgeResult{}, wrapUnavailable(err)
	}
	if del.Teams, _, err = deleteBatch(ctx, s.db, "teams", deviceID, budget); err != nil {
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
