package main

import (
	"testing"

	"example.com/pokecalc/services/balance/internal/httpapi"
)

// issue #298(ADR-0409): BALANCE_MAX_CONCURRENT_RECOMMENDATIONS(recommendations の同時実行数の上限。ハードコードしない)。

func TestMaxConcurrentRecommendationsEnvName(t *testing.T) {
	t.Parallel()

	if maxConcurrentRecommendationsEnv != "BALANCE_MAX_CONCURRENT_RECOMMENDATIONS" {
		t.Fatalf("maxConcurrentRecommendationsEnv = %q, want BALANCE_MAX_CONCURRENT_RECOMMENDATIONS", maxConcurrentRecommendationsEnv)
	}
}

func TestMaxConcurrentRecommendationsFromEnv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		env     map[string]string
		want    int
		wantErr bool
	}{
		{name: "unset uses the default", env: map[string]string{}, want: httpapi.DefaultMaxConcurrentRecommendations},
		{name: "empty uses the default", env: map[string]string{maxConcurrentRecommendationsEnv: ""}, want: httpapi.DefaultMaxConcurrentRecommendations},
		{name: "explicit value", env: map[string]string{maxConcurrentRecommendationsEnv: "8"}, want: 8},
		{name: "one is allowed", env: map[string]string{maxConcurrentRecommendationsEnv: "1"}, want: 1},
		{name: "zero is rejected", env: map[string]string{maxConcurrentRecommendationsEnv: "0"}, wantErr: true},
		{name: "negative is rejected", env: map[string]string{maxConcurrentRecommendationsEnv: "-1"}, wantErr: true},
		{name: "not a number is rejected", env: map[string]string{maxConcurrentRecommendationsEnv: "many"}, wantErr: true},
		{name: "decimal is rejected", env: map[string]string{maxConcurrentRecommendationsEnv: "1.5"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := maxConcurrentRecommendationsFromEnv(lookupFrom(tt.env))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("got %d, want an error (main must exit non-zero)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}
