package httpapi

// ログ・リクエスト ID(issue #246)。gateway が付けた(または直接届いた)X-Request-Id を応答に返し、
// アクセスログ 1 行(JSON)に載せる。NewHandler・NewDeferredHandler のどちらも同じ。

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"example.com/pokecalc/services/internal/reqlog"
)

func TestRequestIDAndAccessLog(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(reqlog.NewLogger(&buf, slog.LevelInfo))

	for name, h := range calcMetricsHandlers(t) {
		t.Run(name, func(t *testing.T) {
			buf.Reset()
			header := http.Header{reqlog.Header: {"from-gateway-1"}}
			rec := serve(t, h, http.MethodGet, "/api/pokedex/species", header, nil) // 担当外 → 404(1 行のアクセスログ)
			if got := rec.Header().Get(reqlog.Header); got != "from-gateway-1" {
				t.Errorf("応答の %s = %q, want 受け取った値", reqlog.Header, got)
			}
			var line map[string]any
			if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &line); err != nil {
				t.Fatalf("アクセスログが JSON 1 行でない: %q: %v", buf.String(), err)
			}
			if line["request_id"] != "from-gateway-1" || line["status"] != float64(http.StatusNotFound) || line["path"] != "/api/pokedex/species" {
				t.Errorf("アクセスログ = %v", line)
			}

			// ID が無い(直接アクセス)・不正なら生成する。
			rec = serve(t, h, http.MethodGet, "/nope", http.Header{reqlog.Header: {"bad id"}}, nil)
			if got := rec.Header().Get(reqlog.Header); !reqlog.ValidID(got) || got == "bad id" {
				t.Errorf("不正な ID の応答 %s = %q, want 生成された有効な ID", reqlog.Header, got)
			}
		})
	}
}
