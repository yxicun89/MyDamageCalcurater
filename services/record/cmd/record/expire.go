// `record expire`: 失効ジョブ(ADR-0209 §4・ADR-0220 §3)。CronJob が同じイメージの引数 expire で起動し、
// 1巡して終わる(常駐しない)。上限に達しても「残りあり」をログに出して終了コード 0(次回が続きを消す)。
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"example.com/pokecalc/services/record/internal/expire"
)

// envExpireBatchLimit は1回の実行で消す行数の上限(失効ジョブだけが読む。serve は要求しない)。
const envExpireBatchLimit = "RECORD_EXPIRE_BATCH_LIMIT"

// expireConfig は失効ジョブの設定。DSN・保持日数・上限だけを読み、NATS・半減期・待ち受けは読まない。
type expireConfig struct {
	DatabaseDSN string
	Policy      expire.Policy
}

func loadExpireConfig(getenv func(string) string) (expireConfig, error) {
	dsn := getenv(envAppDSN)
	if dsn == "" {
		return expireConfig{}, fmt.Errorf("%s が設定されていない", envAppDSN)
	}
	var p expire.Policy
	var err error
	if p.CalcEventsRetention, err = parseDays(getenv, envCalcEventsRetentionDays); err != nil {
		return expireConfig{}, err
	}
	if p.FavoritesRetention, err = parseDays(getenv, envFavoritesRetentionDays); err != nil {
		return expireConfig{}, err
	}
	if p.DeviceRowExpiry, err = parseDays(getenv, envDeviceRowExpiryDays); err != nil {
		return expireConfig{}, err
	}
	if p.PurgeJournalRetention, err = parseDays(getenv, envPurgeJournalRetentionDays); err != nil {
		return expireConfig{}, err
	}
	if p.BatchLimit, err = parsePositiveInt(getenv, envExpireBatchLimit); err != nil {
		return expireConfig{}, err
	}
	// serve と同じ検証: DeviceRowExpiry は JetStream の max_age より大きい(ADR-0211 §7)。
	if p.DeviceRowExpiry <= jetStreamMaxAge {
		return expireConfig{}, fmt.Errorf("%s(%s)は JetStream の max_age(%s)より大きくなければならない",
			envDeviceRowExpiryDays, p.DeviceRowExpiry, jetStreamMaxAge)
	}
	if err := p.Validate(); err != nil {
		return expireConfig{}, err
	}
	return expireConfig{DatabaseDSN: dsn, Policy: p}, nil
}

func runExpire(ctx context.Context, getenv func(string) string, stderr io.Writer) int {
	cfg, err := loadExpireConfig(getenv)
	if err != nil {
		fmt.Fprintln(stderr, "record expire:", err)
		return 1
	}
	db, err := openDB(cfg.DatabaseDSN)
	if err != nil {
		fmt.Fprintln(stderr, "record expire: DB を開けない:", err)
		return 1
	}
	defer db.Close()

	log := slog.New(slog.NewJSONHandler(stderr, nil))
	if _, err := expire.Run(ctx, expire.NewTiDB(db), cfg.Policy, time.Now().UTC(), log); err != nil {
		fmt.Fprintln(stderr, "record expire:", err)
		return 1
	}
	return 0
}
