package reqlog

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestValidID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want bool
	}{
		{"英数字", "abc123", true},
		{"UUID 形式", "123e4567-e89b-12d3-a456-426614174000", true},
		{"ちょうど 64 文字", strings.Repeat("a", 64), true},
		{"空", "", false},
		{"65 文字", strings.Repeat("a", 65), false},
		{"空白を含む", "a b", false},
		{"改行を含む", "a\nb", false},
		{"非 ASCII", "あ", false},
		{"制御文字", "a\x01", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidID(tt.id); got != tt.want {
				t.Errorf("ValidID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestNewIDIsValidAndUnique(t *testing.T) {
	a, b := NewID(), NewID()
	if !ValidID(a) || !ValidID(b) {
		t.Fatalf("NewID が不正な ID を返した: %q %q", a, b)
	}
	if a == b {
		t.Errorf("NewID が同じ値を返した: %q", a)
	}
}

// lines は JSON Lines のログを 1 行ずつ decode する。JSON でない行があれば失敗する(1 形式の保証)。
func lines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("JSON でないログ行: %q: %v", l, err)
		}
		out = append(out, m)
	}
	return out
}

func newApp(t *testing.T, buf *bytes.Buffer, h echo.HandlerFunc) http.Handler {
	t.Helper()
	e := echo.New()
	e.Use(Middleware(NewLogger(buf, slog.LevelInfo)))
	e.Any("/*", h)
	return e
}

func TestMiddlewareGeneratesIDAndLogsAccess(t *testing.T) {
	var buf bytes.Buffer
	var seen string
	app := newApp(t, &buf, func(c *echo.Context) error {
		seen = c.Request().Header.Get(Header)
		slog.New(NewLogger(&buf, slog.LevelInfo).Handler()).InfoContext(c.Request().Context(), "inner")
		return c.String(http.StatusTeapot, "x")
	})
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/some/path", nil))

	id := rec.Header().Get(Header)
	if !ValidID(id) {
		t.Fatalf("応答の %s = %q, want 生成された有効な ID", Header, id)
	}
	if seen != id {
		t.Errorf("ハンドラが見た要求ヘッダ = %q, want 応答と同じ %q(上流へ転送される)", seen, id)
	}
	logs := lines(t, &buf)
	if len(logs) != 2 {
		t.Fatalf("ログ行数 = %d, want 2(ハンドラ内 1 + アクセスログ 1): %v", len(logs), logs)
	}
	if logs[0]["request_id"] != id {
		t.Errorf("ハンドラ内のログの request_id = %v, want %q(context から付く)", logs[0]["request_id"], id)
	}
	acc := logs[1]
	if acc["request_id"] != id || acc["method"] != "GET" || acc["path"] != "/some/path" || acc["status"] != float64(http.StatusTeapot) {
		t.Errorf("アクセスログ = %v", acc)
	}
	if _, ok := acc["duration_ms"]; !ok {
		t.Errorf("アクセスログに duration_ms が無い: %v", acc)
	}
}

func TestMiddlewareKeepsValidClientIDAndReplacesInvalid(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		wantSame bool
	}{
		{"有効", "client-id-1", true},
		{"長すぎる", strings.Repeat("a", 65), false},
		{"空白入り", "a b", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			var seen string
			app := newApp(t, &buf, func(c *echo.Context) error {
				seen = c.Request().Header.Get(Header)
				return c.NoContent(http.StatusNoContent)
			})
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set(Header, tt.header)
			rec := httptest.NewRecorder()
			app.ServeHTTP(rec, req)
			got := rec.Header().Get(Header)
			if (got == tt.header) != tt.wantSame {
				t.Errorf("応答の ID = %q(要求 %q), 通す = %v のはず", got, tt.header, tt.wantSame)
			}
			if !ValidID(got) || seen != got {
				t.Errorf("応答 ID = %q, ハンドラが見た ID = %q", got, seen)
			}
		})
	}
}

func TestMiddlewareLogsHandlerErrorStatus(t *testing.T) {
	var buf bytes.Buffer
	e := echo.New()
	e.HTTPErrorHandler = func(c *echo.Context, err error) { _ = c.NoContent(http.StatusBadGateway) }
	e.Use(Middleware(NewLogger(&buf, slog.LevelInfo)))
	e.GET("/x", func(c *echo.Context) error { return http.ErrAbortHandler })
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	logs := lines(t, &buf)
	if len(logs) != 1 || logs[0]["status"] != float64(http.StatusBadGateway) {
		t.Errorf("アクセスログ = %v, want status 502 の 1 行", logs)
	}
}

func TestMiddlewareLogsProbesAtDebugOnly(t *testing.T) {
	var buf bytes.Buffer
	app := newApp(t, &buf, func(c *echo.Context) error { return c.NoContent(http.StatusOK) })
	for _, p := range []string{"/healthz", "/readyz", "/metrics"} {
		app.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, p, nil))
	}
	if buf.Len() != 0 {
		t.Errorf("probe・scrape が info でログされた: %s", buf.String())
	}
}

func TestLoggerAddsRequestIDFromContext(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf, slog.LevelInfo)
	l.WarnContext(WithID(context.Background(), "rid-1"), "m")
	l.Warn("no ctx")
	logs := lines(t, &buf)
	if logs[0]["request_id"] != "rid-1" {
		t.Errorf("request_id = %v, want rid-1", logs[0]["request_id"])
	}
	if _, ok := logs[1]["request_id"]; ok {
		t.Errorf("context に ID が無いのに request_id が付いた: %v", logs[1])
	}
}

func TestInstallSetsJSONDefault(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	Install(&buf)
	slog.Warn("hello", "k", "v")
	logs := lines(t, &buf)
	if len(logs) != 1 || logs[0]["msg"] != "hello" || logs[0]["k"] != "v" {
		t.Errorf("既定ロガーが JSON でない: %q", buf.String())
	}
}
