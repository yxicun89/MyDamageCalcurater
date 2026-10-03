package main

import (
	"testing"
	"time"
)

// TestSpeedEffectsTTLFromEnv: 素早さ効果の表(マスタから抜き出した小表)の TTL(issue 235 第2段・ADR-0714 §1)。
// 既定は 10 分。マスタの更新(master-release)は数日に 1 回なので、10 分遅れで反映されれば足りる。
// 不正な値は起動を失敗させる(JUDGE_UPSTREAM_TIMEOUT と同じ姿勢)。失敗後の再取得の間隔
// (RetryAfterFailure)は TTL より短い定数(既定 30 秒)で、TTL がそれ以下なら起動を失敗させる。
func TestSpeedEffectsTTLFromEnv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		env     map[string]string
		want    time.Duration
		wantErr bool
	}{
		{"未設定なら既定の10分", nil, 10 * time.Minute, false},
		{"空文字なら既定の10分", map[string]string{"JUDGE_SPEED_EFFECTS_TTL": ""}, 10 * time.Minute, false},
		{"duration として読む", map[string]string{"JUDGE_SPEED_EFFECTS_TTL": "1h"}, time.Hour, false},
		{"duration でない", map[string]string{"JUDGE_SPEED_EFFECTS_TTL": "600"}, 0, true},
		{"0 は不可", map[string]string{"JUDGE_SPEED_EFFECTS_TTL": "0s"}, 0, true},
		{"負は不可", map[string]string{"JUDGE_SPEED_EFFECTS_TTL": "-1m"}, 0, true},
		{"失敗後の再取得の間隔以下は不可", map[string]string{"JUDGE_SPEED_EFFECTS_TTL": "30s"}, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := speedEffectsTTLFromEnv(envLookup(tt.env))
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
				t.Errorf("speedEffectsTTLFromEnv = %v, want %v", got, tt.want)
			}
		})
	}
	if speedEffectsRetryAfterFailure != 30*time.Second {
		t.Errorf("speedEffectsRetryAfterFailure = %v, want 30s", speedEffectsRetryAfterFailure)
	}
}
