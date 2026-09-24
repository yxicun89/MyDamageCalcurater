// Package db は pokedex-svc の MySQL スキーマの migration を持つ(ADR-0100 §1・§5)。
//
// migrations/ の SQL は embed.FS に埋め込み、実行版がコードと一致するようにする。
// golang-migrate の公式 CLI を使わず自前コマンド(cmd/migrate)にする理由は
// ADR-0100 §1 を参照。実行ロジック本体は services/internal/dbmigrate にあり、
// ここは自分の embed.FS を渡すだけの薄いラッパー(ADR-0211 §5。record-svc / team-svc も
// 同じ形のラッパーを自分の embed.FS で持つ)。
package db

import (
	"embed"

	"example.com/pokecalc/services/internal/dbmigrate"
)

// Migrations は migrations/ 配下の SQL を埋め込む。
//
//go:embed migrations/*.sql
var Migrations embed.FS

// ErrDownNotConfirmed は DownAll の確認用 DB 名が DSN の DB 名と一致しないときに返す
// (CLAUDE.md: DB のデータ削除は人間の確認が必要。接続前に拒否する)。layout_test.go 等が
// errors.Is で参照するため dbmigrate 側の値をそのまま再エクスポートする。
var ErrDownNotConfirmed = dbmigrate.ErrDownNotConfirmed

// Up は最新版まで migrate する。差分が無ければ何もしない。
func Up(dsn string) error {
	return dbmigrate.Up(dsn, Migrations)
}

// DownAll はすべての migration を戻す(全テーブルを削除する)。
// confirmDatabase が DSN の DB 名と一致しないときは接続前に ErrDownNotConfirmed を返す。
// スキーマが未適用の DB に対しては何もせず nil を返す。
func DownAll(dsn, confirmDatabase string) error {
	return dbmigrate.DownAll(dsn, confirmDatabase, Migrations)
}

// Version は現在の migrate バージョンと dirty フラグを返す。migrate が一度も
// 実行されていない DB では ok=false(バージョンは意味を持たない)。
func Version(dsn string) (version uint, dirty bool, ok bool, err error) {
	return dbmigrate.Version(dsn, Migrations)
}
