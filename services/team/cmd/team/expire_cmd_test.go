package main

// `team serve` / `team expire` のサブコマンドと、失効ジョブの設定(ADR-0220 §3。AC-E10)。
//
// **test-first(ADR-0003)**: まだ無い。record と同じ形にする:
//
//	func run(args []string, getenv func(string) string, stderr io.Writer) int // 0 成功・1 実行時/設定の失敗・2 使い方
//	type expireConfig struct {
//		DatabaseDSN string        // TEAM_APP_DSN
//		Policy      expire.Policy // TEAM_RETENTION_DAYS・TEAM_DEVICE_ROW_EXPIRY_DAYS・TEAM_PURGE_JOURNAL_RETENTION_DAYS・TEAM_EXPIRE_BATCH_LIMIT
//	}
//	func loadExpireConfig(getenv func(string) string) (expireConfig, error)

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

const envExpireBatchLimitName = "TEAM_EXPIRE_BATCH_LIMIT"

func validExpireEnv() map[string]string {
	return map[string]string{
		"TEAM_APP_DSN":                      "app:pw@tcp(127.0.0.1:4000)/team?parseTime=true",
		"TEAM_RETENTION_DAYS":               "540",
		"TEAM_DEVICE_ROW_EXPIRY_DAYS":       "30",
		"TEAM_PURGE_JOURNAL_RETENTION_DAYS": "90",
		envExpireBatchLimitName:             "1000",
	}
}

func TestLoadExpireConfig(t *testing.T) {
	cfg, err := loadExpireConfig(getenvFrom(validExpireEnv()))
	if err != nil {
		t.Fatalf("loadExpireConfig: %v(NATS・PURGE_BATCH_LIMIT が無くても通ること)", err)
	}
	p := cfg.Policy
	if cfg.DatabaseDSN == "" || p.Retention != 540*day || p.DeviceRowExpiry != 30*day ||
		p.PurgeJournalRetention != 90*day || p.BatchLimit != 1000 {
		t.Errorf("expireConfig = %+v, want DSN あり・540日・30日・90日・1000", cfg)
	}
	if err := p.Validate(); err != nil {
		t.Errorf("読んだ Policy が Validate を通らない: %v", err)
	}
}

func TestLoadExpireConfigRejects(t *testing.T) {
	cases := []struct{ name, key, value string }{
		{"DSN 未設定", "TEAM_APP_DSN", ""},
		{"保持 未設定", "TEAM_RETENTION_DAYS", ""},
		{"devices 行 7日(max_age 以下)", "TEAM_DEVICE_ROW_EXPIRY_DAYS", "7"},
		{"journal 保持 0", "TEAM_PURGE_JOURNAL_RETENTION_DAYS", "0"},
		{"上限 未設定", envExpireBatchLimitName, ""},
		{"上限 負", envExpireBatchLimitName, "-3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := validExpireEnv()
			env[tc.key] = tc.value
			_, err := loadExpireConfig(getenvFrom(env))
			if err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Errorf("err = %v, want %s を含むエラー", err, tc.key)
			}
		})
	}
}

func TestServeConfigDoesNotRequireExpireBatchLimit(t *testing.T) {
	env := validEnv()
	delete(env, envExpireBatchLimitName)
	if _, err := loadConfig(getenvFrom(env)); err != nil {
		t.Errorf("loadConfig が %s を要求している: %v", envExpireBatchLimitName, err)
	}
}

func TestRunSubcommands(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		env      map[string]string
		wantCode int
		wantErr  string
	}{
		{"サブコマンド無し", nil, validExpireEnv(), 2, "serve"},
		{"不明なサブコマンド", []string{"vacuum"}, validExpireEnv(), 2, "expire"},
		{"expire の設定誤り(DB に触れずに 1)", []string{"expire"}, map[string]string{}, 1, "TEAM_APP_DSN"},
		{"serve の設定誤り", []string{"serve"}, map[string]string{}, 1, "TEAM_APP_DSN"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer
			done := make(chan int, 1)
			go func() { done <- run(tc.args, getenvFrom(tc.env), &stderr) }()
			select {
			case code := <-done:
				if code != tc.wantCode {
					t.Errorf("終了コード = %d, want %d(stderr=%s)", code, tc.wantCode, stderr.String())
				}
				if !strings.Contains(stderr.String(), tc.wantErr) {
					t.Errorf("stderr に %q が無い: %s", tc.wantErr, stderr.String())
				}
			case <-time.After(5 * time.Second):
				t.Fatal("run が終わらない")
			}
		})
	}
}

func TestRunExpireFailsWhenDBUnreachable(t *testing.T) {
	env := validExpireEnv()
	env["TEAM_APP_DSN"] = "app:pw@tcp(127.0.0.1:1)/team?parseTime=true&timeout=1s"
	var stderr bytes.Buffer
	done := make(chan int, 1)
	go func() { done <- run([]string{"expire"}, getenvFrom(env), &stderr) }()
	select {
	case code := <-done:
		if code != 1 {
			t.Errorf("終了コード = %d, want 1(stderr=%s)", code, stderr.String())
		}
		if strings.Contains(stderr.String(), "pw@") {
			t.Errorf("stderr に DSN の資格情報が出ている: %s", stderr.String())
		}
	case <-time.After(30 * time.Second):
		t.Fatal("DB に届かないのに expire が終わらない")
	}
}
