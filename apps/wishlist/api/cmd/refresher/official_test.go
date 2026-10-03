package main

import (
	"bytes"
	"testing"

	"example.com/pokecalc/apps/wishlist/api/internal/refresh"
)

// フェーズ4-3(docs/phase4-spec.md AC-O31):refresher は公式ページの監視の件数も 1 行に出す。監視の失敗は終了コードに影響しない。
func TestPrintReport_Official(t *testing.T) {
	var b bytes.Buffer
	printReport(&b, refresh.AllReport{Items: 3, Failed: 1, OfficialChecked: 2, OfficialFailed: 1})
	out := b.String()
	if out != "refresher: items=3 failed=1 official=2 official_failed=1\n" {
		t.Errorf("出力 = %q, want %q", out, "refresher: items=3 failed=1 official=2 official_failed=1\n")
	}
	if got := exitCode(refresh.AllReport{Items: 2, Failed: 0, OfficialChecked: 2, OfficialFailed: 2}); got != 0 {
		t.Errorf("監視だけ全件失敗の終了コード = %d, want 0", got)
	}
}
