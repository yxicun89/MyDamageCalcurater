// Package store は team-svc の team DB(TiDB)のアダプタ(ADR-0811・ADR-0255)。
//
// 契約(型とインターフェース・規則)は使う側の app(`internal/app/ports.go`)が宣言し、ここはその TiDB 実装
// (`tidb.go`。`//go:build tidb` のテストは `tidb_test.go`)を持つ。下の別名は、既存の呼び名
// (store.Store・store.ErrUnavailable など)をテスト・実装で保つためのもので、中身は app の型そのもの。
package store

import "example.com/pokecalc/services/team/internal/app"

// 型の別名(app のポートの型)。
type (
	Store        = app.Store
	StatBlock    = app.StatBlock
	Member       = app.Member
	Team         = app.Team
	Deleted      = app.Deleted
	PurgeResult  = app.PurgeResult
	TouchOutcome = app.TouchOutcome
)

// 定数の別名。
const (
	MaxTeamsPerDevice = app.MaxTeamsPerDevice
	MaxMembersPerTeam = app.MaxMembersPerTeam
	MaxMovesPerMember = app.MaxMovesPerMember

	Touched    = app.Touched
	Skipped    = app.Skipped
	Tombstoned = app.Tombstoned
)

// 失敗の別名(同じ値なので errors.Is はどちらの名前でも一致する)。
var (
	ErrUnavailable      = app.ErrUnavailable
	ErrNotFound         = app.ErrNotFound
	ErrTeamLimitReached = app.ErrTeamLimitReached
)

var _ app.Pinger = (*TiDBStore)(nil)
