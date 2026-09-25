package main

// record-svc の起動設定(環境変数)の受け入れテスト。ADR-0211 §7・ADR-0209 §3・§4・§7。
//
// **test-first(ADR-0003)**: 実装はまだ無い。実装担当が満たすべき形:
//
//	type config struct {
//		Addr                  string        // RECORD_ADDR(既定 ":8080")
//		DatabaseDSN           string        // RECORD_APP_DSN(必須。app ロール。ADR-0211 §4)
//		NATSURL               string        // RECORD_NATS_URL(空なら購読を無効化。calc-svc の CALC_NATS_URL と同じ流儀)
//		CalcEventsRetention   time.Duration // RECORD_CALC_EVENTS_RETENTION_DAYS(必須。日 → Duration)
//		FavoritesRetention    time.Duration // RECORD_FAVORITES_RETENTION_DAYS(必須)
//		DeviceRowExpiry       time.Duration // RECORD_DEVICE_ROW_EXPIRY_DAYS(必須。墓石判定の猶予も兼ねる)
//		PurgeJournalRetention time.Duration // RECORD_PURGE_JOURNAL_RETENTION_DAYS(必須)
//		DecayHalfLife         time.Duration // RECORD_DECAY_HALF_LIFE_DAYS(必須。時間減衰の半減期)
//		PurgeBatchLimit       int           // RECORD_PURGE_BATCH_LIMIT(1回の削除で消す行数の上限。ADR-0209 §5.2)
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
		"RECORD_APP_DSN":                      "app:pw@tcp(127.0.0.1:4000)/record?parseTime=true",
		"RECORD_NATS_URL":                     "nats://127.0.0.1:4222",
		"RECORD_CALC_EVENTS_RETENTION_DAYS":   "90",
		"RECORD_FAVORITES_RETENTION_DAYS":     "540",
		"RECORD_DEVICE_ROW_EXPIRY_DAYS":       "30",
		"RECORD_PURGE_JOURNAL_RETENTION_DAYS": "90",
		"RECORD_DECAY_HALF_LIFE_DAYS":         "14",
		"RECORD_PURGE_BATCH_LIMIT":            "1000",
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
		{"生イベントの保持", cfg.CalcEventsRetention, 90 * day},
		{"お気に入りの保持", cfg.FavoritesRetention, 540 * day},
		{"devices 行の失効 / 墓石の猶予", cfg.DeviceRowExpiry, 30 * day},
		{"purge journal の保持", cfg.PurgeJournalRetention, 90 * day},
		{"時間減衰の半減期", cfg.DecayHalfLife, 14 * day},
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
		"RECORD_CALC_EVENTS_RETENTION_DAYS",
		"RECORD_FAVORITES_RETENTION_DAYS",
		"RECORD_DEVICE_ROW_EXPIRY_DAYS",
		"RECORD_PURGE_JOURNAL_RETENTION_DAYS",
		"RECORD_DECAY_HALF_LIFE_DAYS",
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
	delete(env, "RECORD_APP_DSN")
	if _, err := loadConfig(getenvFrom(env)); err == nil {
		t.Error("loadConfig = nil, want エラー(RECORD_APP_DSN は必須)")
	}
}

// ADR-0211 §7 の追加の検証要件: RECORD_DEVICE_ROW_EXPIRY_DAYS は JetStream の max_age(7日。P5-2)より
// 大きいこと。これを壊すと、削除後に遅れて届くイベントを墓石で捨てきれなくなる(ADR-0209 §7)。
func TestDeviceRowExpiryMustExceedStreamMaxAge(t *testing.T) {
	for _, days := range []string{"1", "7"} {
		t.Run("days="+days, func(t *testing.T) {
			env := validEnv()
			env["RECORD_DEVICE_ROW_EXPIRY_DAYS"] = days
			if _, err := loadConfig(getenvFrom(env)); err == nil {
				t.Errorf("RECORD_DEVICE_ROW_EXPIRY_DAYS=%s で起動できてしまう(JetStream の max_age 7日より大きいこと)", days)
			}
		})
	}
	env := validEnv()
	env["RECORD_DEVICE_ROW_EXPIRY_DAYS"] = "8"
	if _, err := loadConfig(getenvFrom(env)); err != nil {
		t.Errorf("RECORD_DEVICE_ROW_EXPIRY_DAYS=8 は許されるべき: %v", err)
	}
}

// ADR-0209 §4: 時間減衰の半減期は生イベントの保持期間より短いこと
// (「減衰で無視できるほど古くなった分は持たない」という関係を壊さない。plan.md P5-3)。
func TestHalfLifeMustBeShorterThanRetention(t *testing.T) {
	env := validEnv()
	env["RECORD_DECAY_HALF_LIFE_DAYS"] = "90" // = 保持期間
	if _, err := loadConfig(getenvFrom(env)); err == nil {
		t.Error("半減期 = 保持期間 で起動できてしまう(半減期は保持期間より短いこと。ADR-0209 §4)")
	}
	env["RECORD_DECAY_HALF_LIFE_DAYS"] = "120" // > 保持期間
	if _, err := loadConfig(getenvFrom(env)); err == nil {
		t.Error("半減期 > 保持期間 で起動できてしまう")
	}
}

// NATS が未設定でも起動できる(購読が無効になるだけ。CLAUDE.md 絶対ルール5 と同じ流儀で、
// record-svc は NATS の不在で起動に失敗しない)。
func TestNATSIsOptional(t *testing.T) {
	env := validEnv()
	delete(env, "RECORD_NATS_URL")
	cfg, err := loadConfig(getenvFrom(env))
	if err != nil {
		t.Fatalf("loadConfig = %v, want nil(NATS は任意)", err)
	}
	if cfg.NATSURL != "" {
		t.Errorf("NATSURL = %q, want 空", cfg.NATSURL)
	}
}
