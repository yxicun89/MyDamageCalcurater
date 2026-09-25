package main

import (
	"testing"
	"time"

	"example.com/pokecalc/services/judge/internal/judge"
)

// envLookup は os.LookupEnv の差し替え(テストは実環境の環境変数を読まない)。
func envLookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func TestPortFromEnv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"未設定なら既定", nil, "8080"},
		{"空文字なら既定", map[string]string{"PORT": ""}, "8080"},
		{"設定されていればその値", map[string]string{"PORT": "9090"}, "9090"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := portFromEnv(envLookup(tt.env)); got != tt.want {
				t.Errorf("portFromEnv = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestUpstreamTimeoutFromEnv: 上流 1 回ぶんのタイムアウト(ADR-0700 §2。既定 3 秒)。
// 判定はクライアントの 1 リクエストの中で上流を呼ぶので、既定は「人が待てる」範囲に収める。
func TestUpstreamTimeoutFromEnv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		env     map[string]string
		want    time.Duration
		wantErr bool
	}{
		{"未設定なら既定の3秒", nil, 3 * time.Second, false},
		{"空文字なら既定の3秒", map[string]string{"JUDGE_UPSTREAM_TIMEOUT": ""}, 3 * time.Second, false},
		{"duration として読む", map[string]string{"JUDGE_UPSTREAM_TIMEOUT": "1500ms"}, 1500 * time.Millisecond, false},
		{"duration でない", map[string]string{"JUDGE_UPSTREAM_TIMEOUT": "3"}, 0, true},
		{"0 は不可", map[string]string{"JUDGE_UPSTREAM_TIMEOUT": "0s"}, 0, true},
		{"負は不可", map[string]string{"JUDGE_UPSTREAM_TIMEOUT": "-1s"}, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := upstreamTimeoutFromEnv(envLookup(tt.env))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("err = nil, want an error (env=%v)", tt.env)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if got != tt.want {
				t.Errorf("upstreamTimeoutFromEnv = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestRequestTimeoutFromEnv: 1 リクエスト全体の期限(issue #213。既定 12 秒。ADR-0707 §1)。
// http.Server の WriteTimeout(main.go の writeTimeout。既定 15 秒)より確実に短くする
// (期限内に 503 の本文を書き終えられるように)ため、writeTimeout 以上の値は起動を失敗させる。
func TestRequestTimeoutFromEnv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		env          map[string]string
		writeTimeout time.Duration
		want         time.Duration
		wantErr      bool
	}{
		{"未設定なら既定の12秒", nil, writeTimeout, 12 * time.Second, false},
		{"空文字なら既定の12秒", map[string]string{"JUDGE_REQUEST_TIMEOUT": ""}, writeTimeout, 12 * time.Second, false},
		{"duration として読む", map[string]string{"JUDGE_REQUEST_TIMEOUT": "5s"}, writeTimeout, 5 * time.Second, false},
		{"duration でない", map[string]string{"JUDGE_REQUEST_TIMEOUT": "12"}, writeTimeout, 0, true},
		{"0 は不可", map[string]string{"JUDGE_REQUEST_TIMEOUT": "0s"}, writeTimeout, 0, true},
		{"負は不可", map[string]string{"JUDGE_REQUEST_TIMEOUT": "-1s"}, writeTimeout, 0, true},
		{"writeTimeout と同じは不可", map[string]string{"JUDGE_REQUEST_TIMEOUT": "15s"}, writeTimeout, 0, true},
		{"writeTimeout 超過は不可", map[string]string{"JUDGE_REQUEST_TIMEOUT": "20s"}, writeTimeout, 0, true},
		{"writeTimeout 未満なら良い(境界)", map[string]string{"JUDGE_REQUEST_TIMEOUT": "14999ms"}, writeTimeout, 14999 * time.Millisecond, false},
		{"別の writeTimeout でも同じ関係を守る", map[string]string{"JUDGE_REQUEST_TIMEOUT": "3s"}, 4 * time.Second, 3 * time.Second, false},
		{"別の writeTimeout: 一致は不可", map[string]string{"JUDGE_REQUEST_TIMEOUT": "4s"}, 4 * time.Second, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := requestTimeoutFromEnv(envLookup(tt.env), tt.writeTimeout)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("err = nil, want an error (env=%v, writeTimeout=%v)", tt.env, tt.writeTimeout)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if got != tt.want {
				t.Errorf("requestTimeoutFromEnv = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestUpstreamsFromEnvWithoutBaseURL: 上流の URL が未設定でも起動はする(ADR-0700 §5)。
// クライアントは nil のままで、ヘルスは 200、判定の API は 503 になる(speed の read model と同じ扱い)。
func TestUpstreamsFromEnvWithoutBaseURL(t *testing.T) {
	t.Parallel()

	for name, env := range map[string]map[string]string{
		"両方未設定": nil,
		"両方空文字": {"JUDGE_POKEDEX_BASE_URL": "", "JUDGE_CALC_BASE_URL": ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			deps, err := upstreamsFromEnv(envLookup(env))
			if err != nil {
				t.Fatalf("err = %v, want nil(未設定は起動を妨げない)", err)
			}
			if deps.Pokedex != nil || deps.Calc != nil {
				t.Errorf("deps = %+v, want どちらも nil", deps)
			}
		})
	}
}

// TestUpstreamsFromEnvRejectsInvalidBaseURL: 設定されているのに不正なら起動を失敗させる
// (一部だけ使わない、という中途半端な状態を作らない。ADR-0600 §4 と同じ立場)。
func TestUpstreamsFromEnvRejectsInvalidBaseURL(t *testing.T) {
	t.Parallel()

	tests := map[string]map[string]string{
		"pokedex の scheme が不正": {"JUDGE_POKEDEX_BASE_URL": "pokedex:8080", "JUDGE_CALC_BASE_URL": "http://calc"},
		"calc の scheme が不正":    {"JUDGE_POKEDEX_BASE_URL": "http://pokedex", "JUDGE_CALC_BASE_URL": "ftp://calc"},
		"ホストが無い":               {"JUDGE_POKEDEX_BASE_URL": "http://", "JUDGE_CALC_BASE_URL": "http://calc"},
		"タイムアウトが不正":            {"JUDGE_POKEDEX_BASE_URL": "http://pokedex", "JUDGE_CALC_BASE_URL": "http://calc", "JUDGE_UPSTREAM_TIMEOUT": "0s"},
	}
	for name, env := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := upstreamsFromEnv(envLookup(env)); err == nil {
				t.Errorf("err = nil, want an error (env=%v)", env)
			}
		})
	}
}

func TestUpstreamsFromEnvBuildsBothClients(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"JUDGE_POKEDEX_BASE_URL": "http://pokedex.pokecalc.svc.cluster.local",
		"JUDGE_CALC_BASE_URL":    "http://calc.pokecalc.svc.cluster.local",
	}
	deps, err := upstreamsFromEnv(envLookup(env))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if deps.Pokedex == nil || deps.Calc == nil {
		t.Errorf("deps = %+v, want どちらも非 nil", deps)
	}
}

// TestChoiceScarfItemIDFromEnv: こだわりスカーフの持ち物 ID は決め打ちにせず、環境変数で
// 上書きできる(ADR-0701 §3)。実マスタは Git に無く(ADR-0002)、既定値が実際の命名と違っていた
// 場合に、コードを直さず overlay の環境変数 1 行で直せるようにするため。
func TestChoiceScarfItemIDFromEnv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"未設定なら既定", nil, judge.DefaultChoiceScarfItemID},
		{"空文字なら既定", map[string]string{"JUDGE_CHOICE_SCARF_ITEM_ID": ""}, judge.DefaultChoiceScarfItemID},
		{"空白だけなら既定", map[string]string{"JUDGE_CHOICE_SCARF_ITEM_ID": "  "}, judge.DefaultChoiceScarfItemID},
		{"設定されていればその値", map[string]string{"JUDGE_CHOICE_SCARF_ITEM_ID": "scarf-v2"}, "scarf-v2"},
		{"前後の空白は落とす", map[string]string{"JUDGE_CHOICE_SCARF_ITEM_ID": " scarf-v2 "}, "scarf-v2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := choiceScarfItemIDFromEnv(envLookup(tt.env)); got != tt.want {
				t.Errorf("choiceScarfItemIDFromEnv = %q, want %q", got, tt.want)
			}
		})
	}
}
