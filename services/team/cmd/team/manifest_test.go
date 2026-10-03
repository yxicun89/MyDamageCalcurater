package main

// team-svc の k8s マニフェストの静的検査(ADR-0220 §1・§3。AC-K1〜K4)。record の manifest_test.go と対。

import (
	"strconv"
	"testing"

	"example.com/pokecalc/services/gateway/deploytest"
)

var teamManifest = deploytest.TiDBService{
	Service:           "team",
	ImageRepo:         "pokecalc/team",
	AddrEnv:           envAddr,
	DefaultAddr:       defaultAddr,
	DSNEnv:            envAppDSN,
	AuthRef:           "team-db-auth",
	AuthKeyName:       "team-app-dsn",
	ForbiddenAuthRefs: []string{"record-db-auth", "tidb-root-auth", "mysql-auth"},
	ConfigMap:         "team-retention",
	CronJob:           "team-expire",
	ShutdownTimeout:   shutdownTimeout,
}

// AC-K1・K2・K4
func TestManifestTeamDeploymentServiceCronJob(t *testing.T) {
	deploytest.AssertTiDBService(t, teamManifest)
}

// AC-K3
func TestManifestTeamEnvLoads(t *testing.T) {
	env := deploytest.AssertTiDBService(t, teamManifest)
	cfg, err := loadConfig(func(k string) string { return env.Deployment[k] })
	if err != nil {
		t.Fatalf("Deployment の環境変数で loadConfig が失敗: %v(env=%v)", err, env.Deployment)
	}
	if cfg.NATSURL != "nats://nats:4222" {
		t.Errorf("TEAM_NATS_URL = %q, want nats://nats:4222(購読で last_seen_at を進める。ADR-0213 §5)", cfg.NATSURL)
	}
	ecfg, err := loadExpireConfig(func(k string) string { return env.CronJob[k] })
	if err != nil {
		t.Fatalf("CronJob の環境変数で loadExpireConfig が失敗: %v(env=%v)", err, env.CronJob)
	}
	if ecfg.Policy.Retention != cfg.Retention || ecfg.Policy.DeviceRowExpiry != cfg.DeviceRowExpiry ||
		ecfg.Policy.PurgeJournalRetention != cfg.PurgeJournalRetention {
		t.Errorf("serve と expire で日数が違う: serve=%+v expire=%+v", cfg, ecfg.Policy)
	}
}

// AC-K3: ADR-0211 §7 の既定値。
func TestManifestTeamRetentionMatchesADR0211(t *testing.T) {
	env := deploytest.AssertTiDBService(t, teamManifest)
	for name, days := range map[string]int{
		envRetentionDays:             540,
		envDeviceRowExpiryDays:       30,
		envPurgeJournalRetentionDays: 90,
	} {
		if got, err := strconv.Atoi(env.CronJob[name]); err != nil || got != days {
			t.Errorf("%s = %q, want %d(ADR-0211 §7)", name, env.CronJob[name], days)
		}
	}
}
