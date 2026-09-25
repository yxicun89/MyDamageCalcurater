// team-svc の起動設定(環境変数。ADR-0211 §7・ADR-0209 §3・§4・§7)。
package main

import (
	"fmt"
	"strconv"
	"time"
)

// 環境変数の名前(運用の manifest・README が依存する)。
const (
	envAddr    = "TEAM_ADDR"
	envAppDSN  = "TEAM_APP_DSN"
	envNATSURL = "TEAM_NATS_URL"

	envRetentionDays             = "TEAM_RETENTION_DAYS"
	envDeviceRowExpiryDays       = "TEAM_DEVICE_ROW_EXPIRY_DAYS"
	envPurgeJournalRetentionDays = "TEAM_PURGE_JOURNAL_RETENTION_DAYS"
	envPurgeBatchLimit           = "TEAM_PURGE_BATCH_LIMIT"
)

// defaultAddr は TEAM_ADDR が未設定・空のときの待ち受けアドレス。
const defaultAddr = ":8080"

// jetStreamMaxAge は calc-svc が発行する CALC_EVENTS ストリームの max_age(固定7日。
// services/calc/internal/events/events.go の streamMaxAge と同じ値。ADR-0212 §4)。
// TEAM_DEVICE_ROW_EXPIRY_DAYS はこれより大きいことを起動時に検証する(ADR-0211 §7・AC-R8)。
const jetStreamMaxAge = 7 * 24 * time.Hour

// day は日数を Duration に換算する単位(ADR-0209 §3 の「日」の定義: 経過時間 24h × 日数)。
const day = 24 * time.Hour

// config は team-svc の設定(環境変数から1度だけ読む)。
type config struct {
	Addr        string
	DatabaseDSN string
	// NATSURL は空なら購読を無効化する(calc-svc の CALC_NATS_URL と同じ流儀。CLAUDE.md 絶対ルール5)。
	NATSURL string

	// Retention・PurgeJournalRetention は、この serve バイナリの起動時検証(ADR-0211 §7・AC-R8)にだけ
	// 使う。これらを実際に消す失効ジョブ(ADR-0209 §4)は別バイナリで追加する(P5-4b。record-svc の
	// config.go と同じ先取りの考え方)。DeviceRowExpiry は §7 の起動時検証(JetStream max_age との関係)に
	// 使うため、参照はゼロではない。
	Retention             time.Duration
	DeviceRowExpiry       time.Duration
	PurgeJournalRetention time.Duration
	PurgeBatchLimit       int
}

// loadConfig は環境変数から config を組み立てる。ADR-0211 §7 の日数は未設定・0以下なら
// 起動エラーにする(既定値へのフォールバックをしない)。
func loadConfig(getenv func(string) string) (config, error) {
	addr := getenv(envAddr)
	if addr == "" {
		addr = defaultAddr
	}

	dsn := getenv(envAppDSN)
	if dsn == "" {
		return config{}, fmt.Errorf("%s が設定されていない", envAppDSN)
	}

	natsURL := getenv(envNATSURL)

	retention, err := parseDays(getenv, envRetentionDays)
	if err != nil {
		return config{}, err
	}
	deviceRowExpiry, err := parseDays(getenv, envDeviceRowExpiryDays)
	if err != nil {
		return config{}, err
	}
	purgeJournalRetention, err := parseDays(getenv, envPurgeJournalRetentionDays)
	if err != nil {
		return config{}, err
	}

	// ADR-0211 §7 追加の検証要件: 墓石判定の猶予は JetStream の max_age より大きいこと
	// (削除後に遅れて届くイベントを確実に墓石で捨てられるように。ADR-0209 §7)。
	if deviceRowExpiry <= jetStreamMaxAge {
		return config{}, fmt.Errorf("%s(%s)は JetStream の max_age(%s)より大きくなければならない",
			envDeviceRowExpiryDays, deviceRowExpiry, jetStreamMaxAge)
	}

	purgeBatchLimit, err := parsePositiveInt(getenv, envPurgeBatchLimit)
	if err != nil {
		return config{}, err
	}

	return config{
		Addr:                  addr,
		DatabaseDSN:           dsn,
		NATSURL:               natsURL,
		Retention:             retention,
		DeviceRowExpiry:       deviceRowExpiry,
		PurgeJournalRetention: purgeJournalRetention,
		PurgeBatchLimit:       purgeBatchLimit,
	}, nil
}

// parseDays は「日数」の環境変数を Duration に換算する。未設定・空・0以下・整数でない値は
// 変数名を含むエラーにする(ADR-0211 §7: 既定値へのフォールバックをしない)。
func parseDays(getenv func(string) string, name string) (time.Duration, error) {
	raw := getenv(name)
	if raw == "" {
		return 0, fmt.Errorf("%s が設定されていない", name)
	}
	days, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s の形式が不正(整数の日数である必要がある): %v", name, err)
	}
	if days <= 0 {
		return 0, fmt.Errorf("%s は正の整数でなければならない(%d)", name, days)
	}
	return time.Duration(days) * day, nil
}

// parsePositiveInt は正の整数の環境変数を読む(未設定・0以下・整数でない値はエラー)。
func parsePositiveInt(getenv func(string) string, name string) (int, error) {
	raw := getenv(name)
	if raw == "" {
		return 0, fmt.Errorf("%s が設定されていない", name)
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s の形式が不正(整数である必要がある): %v", name, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s は正の整数でなければならない(%d)", name, n)
	}
	return n, nil
}
