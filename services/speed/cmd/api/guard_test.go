package main

import "testing"

// 締め切りの連鎖(#299・ADR-0801): ハンドラの締め切り < writeTimeout(WriteTimeout はハンドラを止めない)。
func TestHandlerDeadlineIsShorterThanWriteTimeout(t *testing.T) {
	if err := guard.Validate(writeTimeout); err != nil {
		t.Fatal(err)
	}
	if guard.Timeout <= 0 || guard.MaxInflight <= 0 {
		t.Errorf("guard = %+v, want 正の締め切りと上限", guard)
	}
}
