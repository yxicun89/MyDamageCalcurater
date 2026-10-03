//go:build mysql

// MySQL 実装の契約テスト。`make wishlist-test-mysql`(docker の mysql:9.7.2)か、
// WISHLIST_TEST_DSN(CREATE/DROP DATABASE ができるユーザー。DB 名は無視して使い捨ての DB を作る)で流す。
package item_test

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	migratemysql "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/item/itemtest"
	"example.com/pokecalc/apps/wishlist/api/migrations"
)

var dbSeq atomic.Int64

// newMySQLRepo は使い捨ての DB を作り、migrations を up(seed を含む)してから Repository を返す。
func newMySQLRepo(t *testing.T) item.Repository {
	t.Helper()
	raw := os.Getenv("WISHLIST_TEST_DSN")
	if raw == "" {
		t.Fatal("WISHLIST_TEST_DSN が無い(make wishlist-test-mysql で流す。スキップはしない)")
	}
	base, err := mysql.ParseDSN(raw)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("wishlist_test_%d_%d", time.Now().UnixNano()%1_000_000, dbSeq.Add(1))

	admin := base.Clone()
	admin.DBName = ""
	adminDB, err := sql.Open("mysql", admin.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { adminDB.Close() })
	if _, err := adminDB.Exec("CREATE DATABASE " + name + " CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { adminDB.Exec("DROP DATABASE IF EXISTS " + name) })

	mcfg := base.Clone()
	mcfg.DBName = name
	mcfg.MultiStatements = true
	mdb, err := sql.Open("mysql", mcfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer mdb.Close()
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	drv, err := migratemysql.WithInstance(mdb, &migratemysql.Config{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "mysql", drv)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatal(err)
	}

	cfg := base.Clone()
	cfg.DBName = name
	cfg.ParseTime = true
	cfg.MultiStatements = false
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return item.NewMySQLRepository(db)
}

func TestMySQLRepositoryContract(t *testing.T) {
	itemtest.RunRepositoryContract(t, newMySQLRepo)
}

func TestMySQLPriceRepositoryContract(t *testing.T) {
	itemtest.RunPriceRepositoryContract(t, func(t *testing.T) itemtest.FullRepository {
		return newMySQLRepo(t).(*item.MySQLRepository)
	})
}
