// Package dbmigrate は pokedex-svc / record-svc / team-svc に共通する MySQL 互換 DB
// (MySQL・TiDB)の migration 実行ロジックを持つ(ADR-0100 §1・ADR-0211 §5)。
//
// スキーマの実体(embed.FS)はサービスごとに別々に持ち、この package には渡さない
// (1つの embed.FS に複数サービス分の migration を混在させない)。呼び出し側の
// 各サービスの db パッケージが、自分の embed.FS を渡す薄いラッパー(Up/DownAll/Version)を持つ。
package dbmigrate

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"

	"github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	mysqlmigrate "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// ErrDownNotConfirmed は DownAll の確認用 DB 名が DSN の DB 名と一致しないときに返す
// (CLAUDE.md: DB のデータ削除は人間の確認が必要。接続前に拒否する)。
var ErrDownNotConfirmed = errors.New("dbmigrate: down の確認用 DB 名が DSN の DB 名と一致しない")

// newMigrate は fsys(呼び出し側の embed.FS。"migrations" ディレクトリを含む)と
// dsn(go-sql-driver/mysql 形式)から *migrate.Migrate を作る。multiStatements はここで付ける。
// 戻り値の2番目は呼び出し側が最後に呼ぶべき DB クローズ関数。
func newMigrate(dsn string, fsys fs.FS) (m *migrate.Migrate, closeDB func() error, dbName string, err error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, nil, "", fmt.Errorf("dsn を解釈できない: %w", err)
	}
	cfg.MultiStatements = true

	conn, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, nil, "", fmt.Errorf("db を開けない: %w", err)
	}
	closeDB = conn.Close

	driver, err := mysqlmigrate.WithInstance(conn, &mysqlmigrate.Config{})
	if err != nil {
		_ = closeDB()
		return nil, nil, "", fmt.Errorf("mysql driver を作れない: %w", err)
	}
	src, err := iofs.New(fsys, "migrations")
	if err != nil {
		_ = closeDB()
		return nil, nil, "", fmt.Errorf("埋め込み migrations を読めない: %w", err)
	}
	m, err = migrate.NewWithInstance("iofs", src, cfg.DBName, driver)
	if err != nil {
		_ = closeDB()
		return nil, nil, "", fmt.Errorf("migrate を作れない: %w", err)
	}
	return m, closeDB, cfg.DBName, nil
}

// Up は fsys の migrations を最新版まで dsn に適用する。差分が無ければ何もしない。
func Up(dsn string, fsys fs.FS) error {
	m, closeDB, _, err := newMigrate(dsn, fsys)
	if err != nil {
		return err
	}
	defer func() { _ = closeDB() }()
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// DownAll はすべての migration を戻す(全テーブルを削除する)。
// confirmDatabase が DSN の DB 名と一致しないときは接続前に ErrDownNotConfirmed を返す。
// スキーマが未適用の DB に対しては何もせず nil を返す。
func DownAll(dsn, confirmDatabase string, fsys fs.FS) error {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return fmt.Errorf("dsn を解釈できない: %w", err)
	}
	// confirmDatabase・cfg.DBName のどちらかが空だと、DSN に DB 名が無いケースで
	// 「確認なし(空文字)」同士が一致してしまい、確認になっていない。
	if confirmDatabase == "" || cfg.DBName == "" || confirmDatabase != cfg.DBName {
		return ErrDownNotConfirmed
	}

	m, closeDB, _, err := newMigrate(dsn, fsys)
	if err != nil {
		return err
	}
	defer func() { _ = closeDB() }()
	defer m.Close()
	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}

// Version は現在の migrate バージョンと dirty フラグを返す。migrate が一度も
// 実行されていない DB では ok=false(バージョンは意味を持たない)。
func Version(dsn string, fsys fs.FS) (version uint, dirty bool, ok bool, err error) {
	m, closeDB, _, err := newMigrate(dsn, fsys)
	if err != nil {
		return 0, false, false, err
	}
	defer func() { _ = closeDB() }()
	defer m.Close()

	v, dirty, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, false, nil
	}
	if err != nil {
		return 0, false, false, err
	}
	return v, dirty, true, nil
}
