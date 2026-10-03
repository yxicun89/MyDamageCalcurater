package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

const testToken = "unit-test-placeholder"

const goodDSN = "wishlist:pw@tcp(mysql.pokecalc.svc.cluster.local:3306)/wishlist?parseTime=true&loc=UTC"

func fullEnv() map[string]string {
	return map[string]string{
		"PORT":                  "9090",
		"WISHLIST_DATABASE_DSN": goodDSN,
		"WISHLIST_API_TOKEN":    testToken,
		"WISHLIST_IMAGE_DIR":    "/data/images",
	}
}

// AC-C1: serve の設定の読み込み。
func TestLoadServeConfig(t *testing.T) {
	c, err := loadServeConfig(envOf(fullEnv()))
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != "9090" || c.Token != testToken || c.ImageDir != "/data/images" {
		t.Errorf("config = %+v", c)
	}
	cfg, err := mysql.ParseDSN(c.DSN)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ParseTime || cfg.MultiStatements || cfg.DBName != "wishlist" {
		t.Errorf("DSN = %+v", cfg)
	}

	env := fullEnv()
	delete(env, "PORT")
	c, err = loadServeConfig(envOf(env))
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != DefaultPort {
		t.Errorf("既定の PORT = %q", c.Port)
	}
}

// AC-C1: 必須の値が無ければ失敗し、どの変数かを示す。トークンは表示しない。
func TestLoadServeConfig_Missing(t *testing.T) {
	for _, k := range []string{"WISHLIST_DATABASE_DSN", "WISHLIST_API_TOKEN", "WISHLIST_IMAGE_DIR"} {
		for name, v := range map[string]string{"無い": "", "空白だけ": "  "} {
			t.Run(k+" "+name, func(t *testing.T) {
				env := fullEnv()
				env[k] = v
				_, err := loadServeConfig(envOf(env))
				if !errors.Is(err, errMissingEnv) {
					t.Fatalf("err = %v, want errMissingEnv", err)
				}
				if !strings.Contains(err.Error(), k) {
					t.Errorf("エラーに変数名 %s が無い: %v", k, err)
				}
			})
		}
	}
	for _, p := range []string{"abc", "0", "65536", "-1"} {
		t.Run("PORT="+p, func(t *testing.T) {
			env := fullEnv()
			env["PORT"] = p
			if _, err := loadServeConfig(envOf(env)); err == nil {
				t.Error("不正な PORT を受け付けた")
			}
		})
	}
	t.Run("不正な DSN", func(t *testing.T) {
		env := fullEnv()
		env["WISHLIST_DATABASE_DSN"] = "not a dsn"
		if _, err := loadServeConfig(envOf(env)); err == nil {
			t.Error("不正な DSN を受け付けた")
		}
	})
}

// AC-C2: migrate は DSN だけが必須(トークン・画像ディレクトリは要らない)。
func TestLoadMigrateConfig(t *testing.T) {
	c, err := loadMigrateConfig(envOf(map[string]string{"WISHLIST_DATABASE_DSN": goodDSN}))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := mysql.ParseDSN(c.DSN)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MultiStatements {
		t.Error("migrate の DSN に multiStatements=true が無い")
	}
	if _, err := loadMigrateConfig(envOf(nil)); !errors.Is(err, errMissingEnv) {
		t.Errorf("err = %v, want errMissingEnv", err)
	}
}

// AC-C3: DSN の加工(serve は parseTime を付けて multiStatements を外す。migrate は multiStatements を足す)。
func TestDSN(t *testing.T) {
	s, err := serveDSN("u:p@tcp(h:3306)/wishlist?multiStatements=true&loc=UTC")
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := mysql.ParseDSN(s)
	if cfg == nil || !cfg.ParseTime || cfg.MultiStatements || cfg.Loc.String() != "UTC" || cfg.Passwd != "p" || cfg.Addr != "h:3306" {
		t.Errorf("serveDSN = %q", s)
	}
	m, err := migrateDSN("u:p@tcp(h:3306)/wishlist?parseTime=true")
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ = mysql.ParseDSN(m)
	if cfg == nil || !cfg.MultiStatements || !cfg.ParseTime || cfg.DBName != "wishlist" {
		t.Errorf("migrateDSN = %q", m)
	}
	if _, err := migrateDSN("::bad::"); err == nil {
		t.Error("不正な DSN を受け付けた")
	}
}

// AC-C4: サブコマンドの扱い(DB に繋がない範囲)。
func TestRun_Usage(t *testing.T) {
	cases := []struct {
		name string
		args []string
		env  map[string]string
		want int
	}{
		{"引数なし", nil, fullEnv(), 2},
		{"不明なサブコマンド", []string{"nope"}, fullEnv(), 2},
		{"migrate だけ", []string{"migrate"}, fullEnv(), 2},
		{"migrate の不明な動作", []string{"migrate", "sideways"}, fullEnv(), 2},
		{"serve で必須の値が無い", []string{"serve"}, map[string]string{}, 1},
		{"migrate up で DSN が無い", []string{"migrate", "up"}, map[string]string{}, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			if got := run(c.args, envOf(c.env), &out, &errb); got != c.want {
				t.Errorf("run(%v) = %d, want %d(stderr %s)", c.args, got, c.want, errb.String())
			}
			if errb.Len() == 0 {
				t.Error("stderr に理由が出ていない")
			}
			if strings.Contains(errb.String(), testToken) {
				t.Error("トークンを出力した")
			}
		})
	}
}
