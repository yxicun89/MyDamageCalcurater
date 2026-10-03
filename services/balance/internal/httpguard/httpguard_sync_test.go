package httpguard_test

import (
	"bytes"
	"os"
	"testing"
)

// 正は services/internal/httpguard/httpguard.go(ADR-0801。ADR-0406 §2 と同じ複製の方針)。
// このモジュールの複製はバイト一致でなければならない。正本や複製が無い場合も失敗にする。
const sharedHTTPGuardPath = "../../../internal/httpguard/httpguard.go"

func TestCopyMatchesSharedHTTPGuard(t *testing.T) {
	t.Parallel()

	shared, err := os.ReadFile(sharedHTTPGuardPath)
	if err != nil {
		t.Fatalf("read shared httpguard: %v", err)
	}
	local, err := os.ReadFile("httpguard.go")
	if err != nil {
		t.Fatalf("read local httpguard copy: %v", err)
	}
	if !bytes.Equal(shared, local) {
		t.Fatalf("httpguard.go differs from %s; mirror the change in all 4 copies (ADR-0801)", sharedHTTPGuardPath)
	}
}
