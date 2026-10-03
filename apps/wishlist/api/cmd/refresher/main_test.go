package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"

	"example.com/pokecalc/apps/wishlist/api/internal/refresh"
)

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

const goodDSN = "wishlist:pw@tcp(mysql.pokecalc.svc.cluster.local:3306)/wishlist?multiStatements=true&loc=UTC"

// AC-C6: 設定。WISHLIST_DATABASE_DSN は必須(parseTime を付け multiStatements を外す)。WISHLIST_YAHOO_APPID は任意。
func TestLoadConfig(t *testing.T) {
	c, err := loadConfig(envOf(map[string]string{"WISHLIST_DATABASE_DSN": goodDSN, "WISHLIST_YAHOO_APPID": " unit-test-appid "}))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := mysql.ParseDSN(c.DSN)
	if err != nil {
		t.Fatalf("DSN = %q: %v", c.DSN, err)
	}
	if !cfg.ParseTime || cfg.MultiStatements || cfg.DBName != "wishlist" || cfg.Loc.String() != "UTC" {
		t.Errorf("DSN = %+v", cfg)
	}
	if c.YahooAppID != "unit-test-appid" {
		t.Errorf("YahooAppID = %q", c.YahooAppID)
	}

	c, err = loadConfig(envOf(map[string]string{"WISHLIST_DATABASE_DSN": goodDSN}))
	if err != nil || c.YahooAppID != "" {
		t.Errorf("appid なし = %+v, %v", c, err)
	}

	for name, v := range map[string]string{"無い": "", "空白だけ": "  "} {
		t.Run("DSN "+name, func(t *testing.T) {
			_, err := loadConfig(envOf(map[string]string{"WISHLIST_DATABASE_DSN": v}))
			if !errors.Is(err, errMissingEnv) || !strings.Contains(err.Error(), "WISHLIST_DATABASE_DSN") {
				t.Errorf("err = %v, want errMissingEnv と変数名", err)
			}
		})
	}
	t.Run("不正な DSN(中身を出さない)", func(t *testing.T) {
		_, err := loadConfig(envOf(map[string]string{"WISHLIST_DATABASE_DSN": "secret-pass not a dsn"}))
		if err == nil {
			t.Fatal("不正な DSN を受け付けた")
		}
		if strings.Contains(err.Error(), "secret-pass") {
			t.Errorf("エラーに DSN が出ている: %v", err)
		}
	})
}

// AC-C7: 終了コード。商品があって全件失敗なら 1、それ以外は 0。
func TestExitCode(t *testing.T) {
	cases := []struct {
		r    refresh.AllReport
		want int
	}{
		{refresh.AllReport{Items: 0, Failed: 0}, 0},
		{refresh.AllReport{Items: 3, Failed: 0}, 0},
		{refresh.AllReport{Items: 3, Failed: 2}, 0},
		{refresh.AllReport{Items: 3, Failed: 3}, 1},
		{refresh.AllReport{Items: 1, Failed: 1}, 1},
	}
	for _, c := range cases {
		if got := exitCode(c.r); got != c.want {
			t.Errorf("exitCode(%+v) = %d, want %d", c.r, got, c.want)
		}
	}
}

// AC-C7: 最後に件数と失敗数を 1 行で出す。
func TestPrintReport(t *testing.T) {
	var b bytes.Buffer
	printReport(&b, refresh.AllReport{Items: 3, Failed: 1})
	out := b.String()
	if !strings.Contains(out, "items=3") || !strings.Contains(out, "failed=1") || strings.Count(out, "\n") != 1 {
		t.Errorf("出力 = %q, want `refresher: items=3 failed=1` の 1 行", out)
	}
}

// AC-C8: 設定の誤りは 1(理由に変数名)、引数を付けたら 2(DB には繋がない)。
func TestRun_Usage(t *testing.T) {
	var out, errb bytes.Buffer
	if got := run(context.Background(), nil, envOf(nil), &out, &errb); got != 1 || !strings.Contains(errb.String(), "WISHLIST_DATABASE_DSN") {
		t.Errorf("DSN なし = %d(stderr %q), want 1 と変数名", got, errb.String())
	}
	errb.Reset()
	if got := run(context.Background(), []string{"extra"}, envOf(map[string]string{"WISHLIST_DATABASE_DSN": goodDSN}), &out, &errb); got != 2 || errb.Len() == 0 {
		t.Errorf("引数あり = %d(stderr %q), want 2", got, errb.String())
	}
}
