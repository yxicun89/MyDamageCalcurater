package main

// record-svc の k8s マニフェストの静的検査(ADR-0220 §1・§3。AC-K1〜K4)。kubectl を使わず YAML を読む。
// 環境変数名はこのパッケージの定数と突き合わせ、manifest の値で loadConfig / loadExpireConfig が通ることまで確かめる。

import (
	"strconv"
	"testing"

	"example.com/pokecalc/services/gateway/deploytest"
)

var recordManifest = deploytest.TiDBService{
	Service:          "record",
	ImageRepo:        "pokecalc/record",
	AddrEnv:          envAddr,
	DefaultAddr:      defaultAddr,
	DSNEnv:           envAppDSN,
	Secret:           "record-db-auth",
	SecretKey:        "record-app-dsn",
	ForbiddenSecrets: []string{"team-db-auth", "tidb-root-auth", "mysql-auth"},
	ConfigMap:        "record-retention",
	CronJob:          "record-expire",
	ShutdownTimeout:  shutdownTimeout,
}

// AC-K1・K2・K4
func TestManifestRecordDeploymentServiceCronJob(t *testing.T) {
	deploytest.AssertTiDBService(t, recordManifest)
}

// AC-K3: manifest の値で serve・expire の設定が通る(既定値へのフォールバックが無いので、書き漏らしは起動エラーになる)。
func TestManifestRecordEnvLoads(t *testing.T) {
	env := deploytest.AssertTiDBService(t, recordManifest)

	cfg, err := loadConfig(func(k string) string { return env.Deployment[k] })
	if err != nil {
		t.Fatalf("Deployment の環境変数で loadConfig が失敗: %v(env=%v)", err, env.Deployment)
	}
	if cfg.NATSURL != "nats://nats:4222" {
		t.Errorf("RECORD_NATS_URL = %q, want nats://nats:4222(calc と同じ NATS)", cfg.NATSURL)
	}
	ecfg, err := loadExpireConfig(func(k string) string { return env.CronJob[k] })
	if err != nil {
		t.Fatalf("CronJob の環境変数で loadExpireConfig が失敗: %v(env=%v)", err, env.CronJob)
	}
	// serve の起動時検証と失効ジョブが同じ日数を使う(ConfigMap 1つ)。
	if ecfg.Policy.CalcEventsRetention != cfg.CalcEventsRetention || ecfg.Policy.FavoritesRetention != cfg.FavoritesRetention ||
		ecfg.Policy.DeviceRowExpiry != cfg.DeviceRowExpiry || ecfg.Policy.PurgeJournalRetention != cfg.PurgeJournalRetention {
		t.Errorf("serve と expire で日数が違う: serve=%+v expire=%+v", cfg, ecfg.Policy)
	}
}

// AC-K3: ConfigMap の日数は ADR-0211 §7 の既定値(人間の了承済みの値。変えるときは ADR を更新する)。
func TestManifestRecordRetentionMatchesADR0211(t *testing.T) {
	env := deploytest.AssertTiDBService(t, recordManifest)
	want := map[string]int{
		envCalcEventsRetentionDays:   90,
		envFavoritesRetentionDays:    540,
		envDeviceRowExpiryDays:       30,
		envPurgeJournalRetentionDays: 90,
	}
	for name, days := range want {
		got, err := strconv.Atoi(env.CronJob[name])
		if err != nil || got != days {
			t.Errorf("%s = %q, want %d(ADR-0211 §7)", name, env.CronJob[name], days)
		}
	}
}
