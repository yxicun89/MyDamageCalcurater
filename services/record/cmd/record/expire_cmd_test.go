package main

// `record serve` / `record expire` のサブコマンドと、失効ジョブの設定(ADR-0220 §3。AC-E10)。
//
// **test-first(ADR-0003)**: まだ無い。実装者は次の形にする:
//
//	// run は os.Args[1:] を受けてサブコマンドに振り分ける(pokedex の `pokedex serve | export` と同じ形)。
//	// 戻り値は終了コード: 0 成功(失効ジョブの「残りあり」を含む)・1 実行時の失敗(DB に届かない等)・
//	// 2 使い方の誤り(サブコマンド無し・不明)。設定の誤りは DB に触れる前に 1。
//	func run(args []string, getenv func(string) string, stderr io.Writer) int
//
//	type expireConfig struct {
//		DatabaseDSN string        // RECORD_APP_DSN(必須)
//		Policy      expire.Policy // 保持日数4つ + RECORD_EXPIRE_BATCH_LIMIT
//	}
//	// loadExpireConfig は expire が読む環境変数だけを読む(NATS・半減期・PURGE_BATCH_LIMIT・ADDR は読まない)。
//	func loadExpireConfig(getenv func(string) string) (expireConfig, error)

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

const envExpireBatchLimitName = "RECORD_EXPIRE_BATCH_LIMIT"

// validExpireEnv は expire だけが要る環境(serve 用の NATS・半減期・PURGE_BATCH_LIMIT を含めない)。
func validExpireEnv() map[string]string {
	return map[string]string{
		"RECORD_APP_DSN":                      "app:pw@tcp(127.0.0.1:4000)/record?parseTime=true",
		"RECORD_CALC_EVENTS_RETENTION_DAYS":   "90",
		"RECORD_FAVORITES_RETENTION_DAYS":     "540",
		"RECORD_DEVICE_ROW_EXPIRY_DAYS":       "30",
		"RECORD_PURGE_JOURNAL_RETENTION_DAYS": "90",
		envExpireBatchLimitName:               "1000",
	}
}

func TestLoadExpireConfig(t *testing.T) {
	cfg, err := loadExpireConfig(getenvFrom(validExpireEnv()))
	if err != nil {
		t.Fatalf("loadExpireConfig: %v(NATS・半減期・PURGE_BATCH_LIMIT が無くても通ること)", err)
	}
	if cfg.DatabaseDSN != validExpireEnv()["RECORD_APP_DSN"] {
		t.Errorf("DatabaseDSN = %q", cfg.DatabaseDSN)
	}
	p := cfg.Policy
	if p.CalcEventsRetention != 90*day || p.FavoritesRetention != 540*day || p.DeviceRowExpiry != 30*day ||
		p.PurgeJournalRetention != 90*day || p.BatchLimit != 1000 {
		t.Errorf("Policy = %+v, want 90日・540日・30日・90日・1000", p)
	}
	if err := p.Validate(); err != nil {
		t.Errorf("読んだ Policy が Validate を通らない: %v", err)
	}
}

func TestLoadExpireConfigRejects(t *testing.T) {
	cases := []struct {
		name, key, value string
	}{
		{"DSN 未設定", "RECORD_APP_DSN", ""},
		{"イベント保持 未設定", "RECORD_CALC_EVENTS_RETENTION_DAYS", ""},
		{"お気に入り保持 0", "RECORD_FAVORITES_RETENTION_DAYS", "0"},
		{"devices 行 7日(max_age 以下)", "RECORD_DEVICE_ROW_EXPIRY_DAYS", "7"},
		{"journal 保持 負", "RECORD_PURGE_JOURNAL_RETENTION_DAYS", "-1"},
		{"上限 未設定", envExpireBatchLimitName, ""},
		{"上限 0", envExpireBatchLimitName, "0"},
		{"上限 整数でない", envExpireBatchLimitName, "many"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := validExpireEnv()
			env[tc.key] = tc.value
			_, err := loadExpireConfig(getenvFrom(env))
			if err == nil {
				t.Fatal("エラーにならない")
			}
			if !strings.Contains(err.Error(), tc.key) {
				t.Errorf("エラーに変数名 %s が無い: %v", tc.key, err)
			}
		})
	}
}

// serve の設定(loadConfig)は失効ジョブ専用の RECORD_EXPIRE_BATCH_LIMIT を要求しない(Deployment に書かなくてよい)。
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
		wantErr  string // stderr に含まれるべき文字列
	}{
		{"サブコマンド無し", nil, validExpireEnv(), 2, "serve"},
		{"不明なサブコマンド", []string{"vacuum"}, validExpireEnv(), 2, "expire"},
		{"expire の設定誤り(DB に触れずに 1)", []string{"expire"}, map[string]string{}, 1, "RECORD_APP_DSN"},
		{"serve の設定誤り", []string{"serve"}, map[string]string{}, 1, "RECORD_APP_DSN"},
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
				t.Fatal("run が終わらない(設定の誤りで待ち受けを始めている)")
			}
		})
	}
}

// expire は DB に届かなければ 1 で終わる(CronJob の失敗として見える。ADR-0220 §3)。
func TestRunExpireFailsWhenDBUnreachable(t *testing.T) {
	env := validExpireEnv()
	env["RECORD_APP_DSN"] = "app:pw@tcp(127.0.0.1:1)/record?parseTime=true&timeout=1s"
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
