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
	"github.com/golang-migrate/migrate/v4/database"
	mysqlmigrate "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// MigrationsTable は適用済みの版と dirty を記録する表の名前(golang-migrate の mysql driver の既定名と
// 同じ値を明示して使う。既に作られた DB の表名を変えないため)。権限の付与(pokedex の importer は
// 書き換え不可。issue #312)もこの名前を参照する。
const MigrationsTable = "schema_migrations"

// ErrDownNotConfirmed は DownAll の確認用 DB 名が DSN の DB 名と一致しないときに返す
// (CLAUDE.md: DB のデータ削除は人間の確認が必要。接続前に拒否する)。
var ErrDownNotConfirmed = errors.New("dbmigrate: down の確認用 DB 名が DSN の DB 名と一致しない")

// ErrForceNotConfirmed は Force の確認用 DB 名が DSN の DB 名と一致しないときに返す
// (migration の状態を人が書き換える操作なので、DownAll と同じく接続前に拒否する。issue #221)。
var ErrForceNotConfirmed = errors.New("dbmigrate: force の確認用 DB 名が DSN の DB 名と一致しない")

// ErrForceUnknownVersion は Force に渡した版が migrations に無い(負数を含む)ときに返す。
// 0 は「未適用に戻す」の意味で受け付ける。
var ErrForceUnknownVersion = errors.New("dbmigrate: force の版が migrations に無い(0 は未適用に戻す)")

// ErrForceNotDirty は dirty でない DB に、今と違う版を force しようとしたときに返す。
// force は途中で失敗した migration の復旧専用で、正常な DB の版を書き換える用途には使わせない
// (今と同じ版なら何もせず成功する)。
var ErrForceNotDirty = errors.New("dbmigrate: DB は dirty でない(force は途中で失敗した migration の復旧にだけ使う)")

// runner は1回の操作ぶんの migrate と、失敗した migration の名前を引くための source を持つ。
type runner struct {
	m       *migrate.Migrate
	src     source.Driver
	closeDB func() error
}

func (r *runner) close() {
	r.m.Close()
	_ = r.src.Close()
	_ = r.closeDB()
}

// newRunner は fsys(呼び出し側の embed.FS。"migrations" ディレクトリを含む)と
// dsn(go-sql-driver/mysql 形式)から runner を作る。multiStatements はここで付ける。
func newRunner(dsn string, fsys fs.FS) (*runner, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("dsn を解釈できない: %w", err)
	}
	cfg.MultiStatements = true

	conn, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("db を開けない: %w", err)
	}

	driver, err := mysqlmigrate.WithInstance(conn, &mysqlmigrate.Config{MigrationsTable: MigrationsTable})
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("mysql driver を作れない: %w", err)
	}
	// migrate 本体に渡す source と、失敗した migration の名前を引く source は別に開く
	// (migrate が Close したあとも名前を引けるように)。
	src, err := iofs.New(fsys, "migrations")
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("埋め込み migrations を読めない: %w", err)
	}
	names, err := iofs.New(fsys, "migrations")
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("埋め込み migrations を読めない: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, cfg.DBName, driver)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("migrate を作れない: %w", err)
	}
	return &runner{m: m, src: names, closeDB: conn.Close}, nil
}

// failedMigration は失敗直後の(dirty な)版から、失敗した migration の名前を返す。
// golang-migrate は up の失敗ではその migration の版を、down の失敗では戻し先の版を dirty にする。
func (r *runner) failedMigration(up bool) string {
	v, dirty, err := r.m.Version()
	var target uint
	switch {
	case up && err == nil && dirty:
		target = v
	case !up && err == nil && dirty:
		next, nerr := r.src.Next(v)
		if nerr != nil {
			return ""
		}
		target = next
	case !up && errors.Is(err, migrate.ErrNilVersion):
		first, ferr := r.src.First()
		if ferr != nil {
			return ""
		}
		target = first
	default:
		return ""
	}
	_, id, err := r.src.ReadUp(target)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%06d_%s", target, id)
}

// describeMigrationError は migration の SQL の失敗を「migration 名: MySQL のエラー」の1行にする
// (golang-migrate のエラー文は SQL 全文を含み、原因の1行が埋もれるため。issue #221・#279)。
// 元のエラーは %w で包むので errors.As で MySQL のエラー番号を取り出せる。SQL 由来でないエラーはそのまま返す。
func describeMigrationError(err error, name string) error {
	var dbErr database.Error
	var dbErrPtr *database.Error
	switch {
	case errors.As(err, &dbErr):
	case errors.As(err, &dbErrPtr):
		dbErr = *dbErrPtr
	default:
		return err
	}
	if name == "" {
		name = "(名前不明)"
	}
	if dbErr.OrigErr == nil {
		return fmt.Errorf("migration %s が失敗: %s", name, dbErr.Err)
	}
	if dbErr.Line > 0 {
		return fmt.Errorf("migration %s が失敗(line %d): %w", name, dbErr.Line, dbErr.OrigErr)
	}
	return fmt.Errorf("migration %s が失敗: %w", name, dbErr.OrigErr)
}

// confirmed は確認用 DB 名が DSN の DB 名と一致するか。どちらかが空なら一致とみなさない
// (DSN に DB 名が無いケースで「確認なし(空文字)」同士が一致してしまい、確認にならないため)。
func confirmed(dsn, confirmDatabase string) (bool, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return false, fmt.Errorf("dsn を解釈できない: %w", err)
	}
	return confirmDatabase != "" && cfg.DBName != "" && confirmDatabase == cfg.DBName, nil
}

// Up は fsys の migrations を最新版まで dsn に適用する。差分が無ければ何もしない。
func Up(dsn string, fsys fs.FS) error {
	r, err := newRunner(dsn, fsys)
	if err != nil {
		return err
	}
	defer r.close()
	if err := r.m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", describeMigrationError(err, r.failedMigration(true)))
	}
	return nil
}

// DownAll はすべての migration を戻す(全テーブルを削除する)。
// confirmDatabase が DSN の DB 名と一致しないときは接続前に ErrDownNotConfirmed を返す。
// スキーマが未適用の DB に対しては何もせず nil を返す。
func DownAll(dsn, confirmDatabase string, fsys fs.FS) error {
	ok, err := confirmed(dsn, confirmDatabase)
	if err != nil {
		return err
	}
	if !ok {
		return ErrDownNotConfirmed
	}

	r, err := newRunner(dsn, fsys)
	if err != nil {
		return err
	}
	defer r.close()
	if err := r.m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate down: %w", describeMigrationError(err, r.failedMigration(false)))
	}
	return nil
}

// knownVersion は version が fsys の migrations にあるか(DB に接続せずに調べる)。
func knownVersion(fsys fs.FS, version uint) (bool, error) {
	src, err := iofs.New(fsys, "migrations")
	if err != nil {
		return false, fmt.Errorf("埋め込み migrations を読めない: %w", err)
	}
	defer src.Close()
	rc, _, err := src.ReadUp(version)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	_ = rc.Close()
	return true, nil
}

// Force は dirty になった DB の dirty を解き、版を version にする(スキーマ自体は変えない。issue #221)。
// version は migrations にある版か、0(未適用に戻す)。途中まで作られたテーブルの片付けは
// 呼び出し側(人)が手順書どおりに行う。
//
// 接続前に拒否するもの: confirmDatabase が DSN の DB 名と違う(ErrForceNotConfirmed)、
// version が負・migrations に無い(ErrForceUnknownVersion)。
// 接続後: dirty でない DB で今と同じ版なら何もせず nil、違う版なら ErrForceNotDirty。
func Force(dsn, confirmDatabase string, version int, fsys fs.FS) error {
	ok, err := confirmed(dsn, confirmDatabase)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForceNotConfirmed
	}
	if version < 0 {
		return ErrForceUnknownVersion
	}
	if version > 0 {
		known, err := knownVersion(fsys, uint(version))
		if err != nil {
			return err
		}
		if !known {
			return ErrForceUnknownVersion
		}
	}

	r, err := newRunner(dsn, fsys)
	if err != nil {
		return err
	}
	defer r.close()

	cur, dirty, err := r.m.Version()
	switch {
	case errors.Is(err, migrate.ErrNilVersion):
		// 版が無い(一度も migrate していない、または最初の migration の down が途中で失敗した)。
		// golang-migrate はこのとき dirty を返さないので、0 への force は常に流す(dirty を解くだけで冪等)。
		if version != 0 {
			return ErrForceNotDirty
		}
	case err != nil:
		return fmt.Errorf("migrate version: %w", err)
	case !dirty && int(cur) == version:
		return nil
	case !dirty:
		return ErrForceNotDirty
	}

	target := version
	if version == 0 {
		target = database.NilVersion
	}
	if err := r.m.Force(target); err != nil {
		return fmt.Errorf("migrate force: %w", err)
	}
	return nil
}

// Version は現在の migrate バージョンと dirty フラグを返す。migrate が一度も
// 実行されていない DB では ok=false(バージョンは意味を持たない)。
func Version(dsn string, fsys fs.FS) (version uint, dirty bool, ok bool, err error) {
	r, err := newRunner(dsn, fsys)
	if err != nil {
		return 0, false, false, err
	}
	defer r.close()

	v, dirty, err := r.m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, false, nil
	}
	if err != nil {
		return 0, false, false, err
	}
	return v, dirty, true, nil
}
