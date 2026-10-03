package main

import (
	"testing"

	"example.com/pokecalc/services/calc/internal/httpapi"
	"example.com/pokecalc/services/internal/httpguard"
)

// 締め切りの連鎖(#299・ADR-0801): ハンドラの締め切り < writeTimeout(WriteTimeout はハンドラを止めない)。
func TestHandlerDeadlineIsShorterThanWriteTimeout(t *testing.T) {
	cfg := httpguard.Config{MaxInflight: httpapi.DefaultMaxInflight, Timeout: httpapi.DefaultRequestTimeout}
	if err := cfg.Validate(writeTimeout); err != nil {
		t.Fatal(err)
	}
	if cfg.Timeout != httpguard.DeadlineFor(writeTimeout) {
		t.Errorf("Timeout = %v, want writeTimeout - 1s = %v", cfg.Timeout, httpguard.DeadlineFor(writeTimeout))
	}
}
