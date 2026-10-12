// Package store は record-svc の record DB(TiDB)のアダプタ(ADR-0811・ADR-0254)。
//
// 契約(型とインターフェース・規則)は使う側の app(`internal/app/ports.go`)が宣言し、ここはその TiDB 実装
// (`tidb.go`。`//go:build tidb` のテストは `tidb_test.go` ほか)を持つ。下の別名は、既存の呼び名
// (store.Store・store.ErrUnavailable など)をテスト・実装で保つためのもので、中身は app の型そのもの。
package store

import "example.com/pokecalc/services/record/internal/app"

// 型の別名(app のポートの型)。
type (
	Store             = app.Store
	Favorite          = app.Favorite
	FavoriteOutcome   = app.FavoriteOutcome
	FrequentOpponent  = app.FrequentOpponent
	Deleted           = app.Deleted
	PurgeResult       = app.PurgeResult
	SaveOutcome       = app.SaveOutcome
	CalcEvent         = app.CalcEvent
	CalcHistoryCursor = app.CalcHistoryCursor
	CalcHistoryQuery  = app.CalcHistoryQuery
	CalcHistoryRow    = app.CalcHistoryRow
)

// 定数の別名。
const (
	MaxFavoritesPerDevice = app.MaxFavoritesPerDevice

	FavoriteCreated = app.FavoriteCreated
	FavoriteExisted = app.FavoriteExisted

	Stored     = app.Stored
	Duplicate  = app.Duplicate
	Tombstoned = app.Tombstoned
)

// 失敗の別名(同じ値なので errors.Is はどちらの名前でも一致する)。
var (
	ErrUnavailable          = app.ErrUnavailable
	ErrNotFound             = app.ErrNotFound
	ErrFavoriteLimitReached = app.ErrFavoriteLimitReached
)

var _ app.Pinger = (*TiDBStore)(nil)
