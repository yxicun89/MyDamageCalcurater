// record-svc の起動設定(環境変数。ADR-0211 §7・ADR-0209 §3・§4・§7)。
package main

import (
	"fmt"
	"strconv"
	"time"
)

// 環境変数の名前(運用の manifest・README が依存する)。
const (
	envAddr    = "RECORD_ADDR"
	envAppDSN  = "RECORD_APP_DSN"
	envNATSURL = "RECORD_NATS_URL"

	envCalcEventsRetentionDays   = "RECORD_CALC_EVENTS_RETENTION_DAYS"
	envFavoritesRetentionDays    = "RECORD_FAVORITES_RETENTION_DAYS"
	envDeviceRowExpiryDays       = "RECORD_DEVICE_ROW_EXPIRY_DAYS"
	envPurgeJournalRetentionDays = "RECORD_PURGE_JOURNAL_RETENTION_DAYS"
	envDecayHalfLifeDays         = "RECORD_DECAY_HALF_LIFE_DAYS"
	envPurgeBatchLimit           = "RECORD_PURGE_BATCH_LIMIT"
)

// defaultAddr は RECORD_ADDR が未設定・空のときの待ち受けアドレス。
const defaultAddr = ":8080"

// jetStreamMaxAge は calc-svc が発行する CALC_EVENTS ストリームの max_age(固定7日。
// services/calc/internal/events/events.go の streamMaxAge と同じ値。ADR-0212 §4)。
// RECORD_DEVICE_ROW_EXPIRY_DAYS はこれより大きいことを起動時に検証する(ADR-0211 §7・AC-R8)。
const jetStreamMaxAge = 7 * 24 * time.Hour

// day は日数を Duration に換算する単位(ADR-0209 §3 の「日」の定義: 経過時間 24h × 日数)。
const day = 24 * time.Hour

// config は record-svc の設定(環境変数から1度だけ読む)。
type config struct {
	Addr        string
	DatabaseDSN string
	// NATSURL は空なら購読を無効化する(calc-svc の CALC_NATS_URL と同じ流儀。CLAUDE.md 絶対ルール5)。
	NATSURL string

	// CalcEventsRetention・FavoritesRetention・PurgeJournalRetention は、この serve バイナリの
	// 起動時検証(ADR-0211 §7・AC-R8。DeviceRowExpiry との関係・半減期との関係を含む)にだけ使う。
	// これらを実際に消す**失効ジョブ(ADR-0209 §4)はまだ実装していない**(critic レビュー R-3。
	// docs/plan.md P5-3b に切り出し済み)。値は先に環境変数として固定しておき、失効ジョブを別バイナリ
	// (例: cmd/record-expire)として追加するときにそのままこの config を再利用できるようにするための
	// 意図的な先取り。DeviceRowExpiry は §7 の起動時検証(JetStream max_age との関係)に使うため、
	// 3つと違って参照はゼロではない。
	CalcEventsRetention   time.Duration
	FavoritesRetention    time.Duration
	DeviceRowExpiry       time.Duration
	PurgeJournalRetention time.Duration
	DecayHalfLife         time.Duration
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

	calcRetention, err := parseDays(getenv, envCalcEventsRetentionDays)
	if err != nil {
		return config{}, err
	}
	favoritesRetention, err := parseDays(getenv, envFavoritesRetentionDays)
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
	decayHalfLife, err := parseDays(getenv, envDecayHalfLifeDays)
	if err != nil {
		return config{}, err
	}

	// ADR-0211 §7 追加の検証要件: 墓石判定の猶予は JetStream の max_age より大きいこと
	// (削除後に遅れて届くイベントを確実に墓石で捨てられるように。ADR-0209 §7)。
	if deviceRowExpiry <= jetStreamMaxAge {
		return config{}, fmt.Errorf("%s(%s)は JetStream の max_age(%s)より大きくなければならない",
			envDeviceRowExpiryDays, deviceRowExpiry, jetStreamMaxAge)
	}
	// ADR-0209 §4: 時間減衰の半減期は生イベントの保持期間より短いこと。
	if decayHalfLife >= calcRetention {
		return config{}, fmt.Errorf("%s(%s)は %s(%s)より短くなければならない",
			envDecayHalfLifeDays, decayHalfLife, envCalcEventsRetentionDays, calcRetention)
	}

	purgeBatchLimit, err := parsePositiveInt(getenv, envPurgeBatchLimit)
	if err != nil {
		return config{}, err
	}

	return config{
		Addr:                  addr,
		DatabaseDSN:           dsn,
		NATSURL:               natsURL,
		CalcEventsRetention:   calcRetention,
		FavoritesRetention:    favoritesRetention,
		DeviceRowExpiry:       deviceRowExpiry,
		PurgeJournalRetention: purgeJournalRetention,
		DecayHalfLife:         decayHalfLife,
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
