package main

// pokedex-svc のコマンド(`pokedex serve` / `pokedex export`)の設定のテスト(ADR-0105 §1・§5)。

import (
	"strings"
	"testing"
)

func lookupFrom(env map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := env[k]
		return v, ok
	}
}

// AC-K0: 環境変数の名前(k8s のマニフェスト・Makefile と共有する)。
func TestEnvNames(t *testing.T) {
	if envAddr != "POKEDEX_ADDR" || envDatabaseDSN != "POKEDEX_DATABASE_DSN" || defaultAddr != ":8080" {
		t.Errorf("envAddr=%q envDatabaseDSN=%q defaultAddr=%q", envAddr, envDatabaseDSN, defaultAddr)
	}
}

// AC-K0: DSN は必須(空は未設定と同じ)。待ち受けアドレスは未設定・空なら既定値。
// DSN は go-sql-driver/mysql の形式で、parseTime を必ず有効にする(regulations の DATE 列を sql.NullTime で読むため)。
func TestLoadConfig(t *testing.T) {
	const dsn = "user:pass@tcp(mysql:3306)/pokedex"
	tests := []struct {
		name     string
		env      map[string]string
		wantErr  string
		wantAddr string
	}{
		{"DSN が無い", map[string]string{}, envDatabaseDSN, ""},
		{"DSN が空", map[string]string{envDatabaseDSN: ""}, envDatabaseDSN, ""},
		{"DSN が壊れている", map[string]string{envDatabaseDSN: "not a dsn"}, envDatabaseDSN, ""},
		{"既定のアドレス", map[string]string{envDatabaseDSN: dsn}, "", defaultAddr},
		{"アドレスが空なら既定", map[string]string{envDatabaseDSN: dsn, envAddr: ""}, "", defaultAddr},
		{"アドレスの指定", map[string]string{envDatabaseDSN: dsn, envAddr: ":9090"}, "", ":9090"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := loadConfig(lookupFrom(tt.env))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %s を含むエラー", err, tt.wantErr)
				}
				if strings.Contains(err.Error(), "pass@") {
					t.Errorf("エラーに DSN(パスワード)を含めている: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if cfg.Addr != tt.wantAddr {
				t.Errorf("Addr = %q, want %q", cfg.Addr, tt.wantAddr)
			}
			if !strings.Contains(cfg.DSN, "parseTime=true") {
				t.Errorf("DSN に parseTime=true が無い: %q", cfg.DSN)
			}
		})
	}
}
