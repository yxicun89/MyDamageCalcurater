package httpmetrics_test

import (
	"bytes"
	"os"
	"testing"
)

// 正は services/internal/httpmetrics/httpmetrics.go(ADR-0406 §2)。このモジュールの複製は
// バイト一致でなければならない(package 名も同じなので正規化はしない)。正本や複製が無い場合も失敗にする。
const sharedHTTPMetricsPath = "../../../internal/httpmetrics/httpmetrics.go"

func TestCopyMatchesSharedHTTPMetrics(t *testing.T) {
	t.Parallel()

	shared, err := os.ReadFile(sharedHTTPMetricsPath)
	if err != nil {
		t.Fatalf("read shared httpmetrics: %v", err)
	}
	local, err := os.ReadFile("httpmetrics.go")
	if err != nil {
		t.Fatalf("read local httpmetrics copy: %v", err)
	}
	if !bytes.Equal(shared, local) {
		t.Fatalf("httpmetrics.go differs from %s; mirror the change in all 4 copies (ADR-0406 §2)", sharedHTTPMetricsPath)
	}
}
