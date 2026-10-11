//go:build mysql

package store_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"

	"example.com/pokecalc/apps/wishlist/api/internal/adapters/store"
	"example.com/pokecalc/apps/wishlist/api/internal/domain/item"
)

// フェーズ5 5-1d: GET /api/items(Repository.ListItems)は商品数に依らない回数のクエリで作る(N+1 にしない。docs/phase5-spec.md)。
// ドライバーを包んで、DB に届いたクエリ(Query・Prepare・Exec)の回数を数える。

type countingConnector struct {
	inner driver.Connector
	n     *atomic.Int64
}

func (c countingConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &countingConn{Conn: conn, n: c.n}, nil
}

func (c countingConnector) Driver() driver.Driver { return c.inner.Driver() }

// countingConn は mysql の接続を包む。ErrSkip(プレースホルダー付きは Prepare に回る)は数えない。
type countingConn struct {
	driver.Conn
	n *atomic.Int64
}

func (c *countingConn) PrepareContext(ctx context.Context, q string) (driver.Stmt, error) {
	c.n.Add(1)
	return c.Conn.(driver.ConnPrepareContext).PrepareContext(ctx, q)
}

func (c *countingConn) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	rows, err := c.Conn.(driver.QueryerContext).QueryContext(ctx, q, args)
	if !errors.Is(err, driver.ErrSkip) {
		c.n.Add(1)
	}
	return rows, err
}

func (c *countingConn) ExecContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	res, err := c.Conn.(driver.ExecerContext).ExecContext(ctx, q, args)
	if !errors.Is(err, driver.ErrSkip) {
		c.n.Add(1)
	}
	return res, err
}

func (c *countingConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	return c.Conn.(driver.ConnBeginTx).BeginTx(ctx, opts)
}

func (c *countingConn) ResetSession(ctx context.Context) error {
	if r, ok := c.Conn.(driver.SessionResetter); ok {
		return r.ResetSession(ctx)
	}
	return nil
}

func (c *countingConn) IsValid() bool {
	if v, ok := c.Conn.(driver.Validator); ok {
		return v.IsValid()
	}
	return true
}

func (c *countingConn) CheckNamedValue(nv *driver.NamedValue) error {
	if v, ok := c.Conn.(driver.NamedValueChecker); ok {
		return v.CheckNamedValue(nv)
	}
	return driver.ErrSkip
}

// AC-5-1d-3: 商品 2 件と 12 件(どれも目安・上書き・販売状況あり)で、ListItems のクエリ回数が同じ。
func TestMySQLListItems_NotNPlusOne(t *testing.T) {
	ctx := context.Background()
	cfg := newMySQLConfig(t)
	inner, err := mysql.NewConnector(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var n atomic.Int64
	db := sql.OpenDB(countingConnector{inner: inner, n: &n})
	t.Cleanup(func() { db.Close() })
	repo := store.NewMySQLRepository(db)

	site, err := repo.CreateSite(ctx, item.NewSite{Name: "N+1 サイト", SearchURLTemplate: "https://n.example.com/s?q={q}", FetchType: item.FetchScrape})
	if err != nil {
		t.Fatal(err)
	}
	g, err := repo.CreateGenre(ctx, item.NewGenre{Name: "N+1 ジャンル", QueryTemplate: "{name}", SiteIDs: []int64{site.ID}, Color: "#1d5fd1", Icon: item.IconBox})
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 10, 3, 0, 0, 0, time.UTC)
	add := func(k int) {
		t.Helper()
		for i := 0; i < k; i++ {
			it, err := repo.CreateItem(ctx, item.NewItem{GenreID: g.ID, Name: fmt.Sprintf("N+1 商品 %d", i), ImagePath: "11111111-1111-4111-8111-111111111111.png"})
			if err != nil {
				t.Fatal(err)
			}
			q := "上書き"
			if _, err := repo.UpdateItem(ctx, it.ID, item.ItemPatch{SiteOverrides: &[]item.SiteOverride{{SiteID: site.ID, Query: &q, Enabled: true}}}); err != nil {
				t.Fatal(err)
			}
			low := 1000 + i
			if err := repo.SaveSiteResult(ctx, item.Estimate{ItemID: it.ID, SiteID: site.ID, Low: &low, Status: item.EstimateOK, FetchedAt: at}, nil); err != nil {
				t.Fatal(err)
			}
		}
	}
	queries := func() int64 {
		t.Helper()
		before := n.Load()
		list, err := repo.ListItems(ctx, &g.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range list {
			if it.Summary == nil || it.Summary.Status != item.SummaryOK {
				t.Errorf("%s の Summary = %+v, want ok", it.Name, it.Summary)
			}
		}
		return n.Load() - before
	}
	add(2)
	few := queries()
	add(10)
	many := queries()
	if few == 0 {
		t.Fatal("クエリを数えられていない")
	}
	if many != few {
		t.Errorf("ListItems のクエリ回数: 商品 2 件で %d 回、12 件で %d 回(商品数に依らない回数にする)", few, many)
	}
}
