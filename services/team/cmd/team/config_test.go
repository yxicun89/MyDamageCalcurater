package main

// team-svc の起動設定(環境変数)の受け入れテスト。ADR-0211 §7・ADR-0209 §3・§4・§7。AC-R8。
//
// 実装済み(record-svc の services/record/cmd/record/config.go と同じ流儀)。この形を固定する:
//
//	type config struct {
//		Addr                  string        // TEAM_ADDR(既定 ":8080")
//		DatabaseDSN           string        // TEAM_APP_DSN(必須。app ロール。ADR-0211 §4)
//		NATSURL               string        // TEAM_NATS_URL(空なら購読を無効化。CLAUDE.md 絶対ルール5)
//		Retention             time.Duration // TEAM_RETENTION_DAYS(必須。構築の保持。ADR-0211 §7)
//		DeviceRowExpiry       time.Duration // TEAM_DEVICE_ROW_EXPIRY_DAYS(必須。墓石判定の猶予も兼ねる)
//		PurgeJournalRetention time.Duration // TEAM_PURGE_JOURNAL_RETENTION_DAYS(必須)
//		PurgeBatchLimit       int           // TEAM_PURGE_BATCH_LIMIT(1回の削除で消す行数の上限。ADR-0209 §5.2)
//	}
//	func loadConfig(getenv func(string) string) (config, error)
//
// ADR-0211 §7 の「既定値はコード側のフォールバック値として使わない」に従い、保持日数の
// 環境変数は**未設定なら起動エラー**にする(0 以下も同じ)。

import (
	"strings"
	"testing"
	"time"
)

// validEnv は ADR-0211 §7 の推奨値で起動できる環境。
func validEnv() map[string]string {
	return map[string]string{
		"TEAM_APP_DSN":                      "app:pw@tcp(127.0.0.1:4000)/team?parseTime=true",
		"TEAM_NATS_URL":                     "nats://127.0.0.1:4222",
		"TEAM_RETENTION_DAYS":               "540",
		"TEAM_DEVICE_ROW_EXPIRY_DAYS":       "30",
		"TEAM_PURGE_JOURNAL_RETENTION_DAYS": "90",
		"TEAM_PURGE_BATCH_LIMIT":            "1000",
	}
}

func getenvFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// 推奨値で起動でき、ADR-0209 の日数がそのまま読めること。
func TestLoadConfig(t *testing.T) {
	cfg, err := loadConfig(getenvFrom(validEnv()))
	if err != nil {
		t.Fatalf("loadConfig = %v, want nil", err)
	}
	day := 24 * time.Hour
	checks := []struct {
		name string
		got  time.Duration
		want time.Duration
	}{
		{"構築の保持", cfg.Retention, 540 * day},
		{"devices 行の失効 / 墓石の猶予", cfg.DeviceRowExpiry, 30 * day},
		{"purge journal の保持", cfg.PurgeJournalRetention, 90 * day},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
	if cfg.PurgeBatchLimit != 1000 {
		t.Errorf("PurgeBatchLimit = %d, want 1000", cfg.PurgeBatchLimit)
	}
}

// ADR-0211 §7: 未設定・0以下は起動エラー(既定値へのフォールバックをしない)。
func TestLoadConfigRejectsMissingOrNonPositiveDays(t *testing.T) {
	vars := []string{
		"TEAM_RETENTION_DAYS",
		"TEAM_DEVICE_ROW_EXPIRY_DAYS",
		"TEAM_PURGE_JOURNAL_RETENTION_DAYS",
	}
	for _, name := range vars {
		for _, value := range []string{"", "0", "-1", "abc"} {
			t.Run(name+"="+value, func(t *testing.T) {
				env := validEnv()
				if value == "" {
					delete(env, name)
				} else {
					env[name] = value
				}
				if _, err := loadConfig(getenvFrom(env)); err == nil {
					t.Errorf("loadConfig = nil, want エラー(%s=%q。ADR-0211 §7)", name, value)
				} else if !strings.Contains(err.Error(), name) {
					t.Errorf("エラーに変数名 %s が入っていない: %v", name, err)
				}
			})
		}
	}
}

// DSN は必須(自分の DB に繋がらなければ起動しない)。
func TestLoadConfigRequiresDSN(t *testing.T) {
	env := validEnv()
	delete(env, "TEAM_APP_DSN")
	if _, err := loadConfig(getenvFrom(env)); err == nil {
		t.Error("loadConfig = nil, want エラー(TEAM_APP_DSN は必須)")
	}
}

// AC-R8 / ADR-0211 §7: TEAM_DEVICE_ROW_EXPIRY_DAYS は JetStream の max_age(7日。P5-2)より
// 大きいこと。これを壊すと、削除後に遅れて届くイベントを墓石で捨てきれなくなる(ADR-0209 §7)。
func TestDeviceRowExpiryMustExceedStreamMaxAge(t *testing.T) {
	for _, days := range []string{"1", "7"} {
		t.Run("days="+days, func(t *testing.T) {
			env := validEnv()
			env["TEAM_DEVICE_ROW_EXPIRY_DAYS"] = days
			if _, err := loadConfig(getenvFrom(env)); err == nil {
				t.Errorf("TEAM_DEVICE_ROW_EXPIRY_DAYS=%s で起動できてしまう(JetStream の max_age 7日より大きいこと)", days)
			}
		})
	}
	env := validEnv()
	env["TEAM_DEVICE_ROW_EXPIRY_DAYS"] = "8"
	if _, err := loadConfig(getenvFrom(env)); err != nil {
		t.Errorf("TEAM_DEVICE_ROW_EXPIRY_DAYS=8 は許されるべき: %v", err)
	}
}

// NATS が未設定でも起動できる(購読が無効になるだけ。CLAUDE.md 絶対ルール5)。
// team-svc の購読は「計算 API だけを使う端末の last_seen_at を保つ」ためのもので、
// 止まっても構築の CRUD は動き続ける。
func TestNATSIsOptional(t *testing.T) {
	env := validEnv()
	delete(env, "TEAM_NATS_URL")
	cfg, err := loadConfig(getenvFrom(env))
	if err != nil {
		t.Fatalf("loadConfig = %v, want nil(NATS は任意)", err)
	}
	if cfg.NATSURL != "" {
		t.Errorf("NATSURL = %q, want 空", cfg.NATSURL)
	}
}

// PurgeBatchLimit は正の整数であること(1回の全削除で消す行数の上限。ADR-0209 §5.2)。
func TestPurgeBatchLimitMustBePositive(t *testing.T) {
	for _, value := range []string{"", "0", "-1", "abc"} {
		t.Run("value="+value, func(t *testing.T) {
			env := validEnv()
			if value == "" {
				delete(env, "TEAM_PURGE_BATCH_LIMIT")
			} else {
				env["TEAM_PURGE_BATCH_LIMIT"] = value
			}
			if _, err := loadConfig(getenvFrom(env)); err == nil {
				t.Errorf("TEAM_PURGE_BATCH_LIMIT=%q で起動できてしまう", value)
			}
		})
	}
}
