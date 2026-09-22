package deploytest_test

// Web を gateway の後ろに置く変更の文書の検査(ADR-0205。AC-W10)。README の §8 の書き直しは DOC-api で行うので、
// ここでは環境変数の表に GATEWAY_WEB_URL の行があり、ADR-0205 があることだけを固定する。

import (
	"strings"
	"testing"
)

// AC-W10: gateway の README の環境変数の表に GATEWAY_WEB_URL の行がある。ADR-0205 が置かれている。
func TestWebUpstreamIsDocumented(t *testing.T) {
	readme := readRepoFile(t, "services/gateway/README.md")
	found := false
	for _, line := range strings.Split(readme, "\n") {
		if strings.HasPrefix(line, "| `GATEWAY_WEB_URL`") {
			found = true
			break
		}
	}
	if !found {
		t.Error("services/gateway/README.md の環境変数の表に `GATEWAY_WEB_URL` の行が無い")
	}
	adr := readRepoFile(t, "docs/adr/0205-gateway-web-upstream.md")
	if !strings.Contains(adr, "GATEWAY_WEB_URL") {
		t.Error("docs/adr/0205-gateway-web-upstream.md に GATEWAY_WEB_URL の記述が無い")
	}
}
