package main

import "testing"

// 締め切りの連鎖(#299・ADR-0801・ADR-0707): 全体の締め切り(JUDGE_REQUEST_TIMEOUT の既定)< writeTimeout。
// guard 自体は上限だけを持つ(Timeout は 0)。
func TestDefaultRequestTimeoutIsShorterThanWriteTimeout(t *testing.T) {
	if defaultRequestTimeout >= writeTimeout {
		t.Errorf("defaultRequestTimeout(%v)が writeTimeout(%v)以上", defaultRequestTimeout, writeTimeout)
	}
	if err := guard.Validate(writeTimeout); err != nil {
		t.Fatal(err)
	}
	if guard.MaxInflight <= 0 || guard.Code == "" {
		t.Errorf("guard = %+v, want 正の上限と code", guard)
	}
}
