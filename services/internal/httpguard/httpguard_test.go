package httpguard_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"example.com/pokecalc/services/internal/httpguard"
	"github.com/labstack/echo/v5"
)

func newEcho(cfg httpguard.Config, h echo.HandlerFunc) *echo.Echo {
	e := echo.New()
	e.Use(httpguard.Middleware(cfg))
	e.GET("/x", h)
	return e
}

func do(e *echo.Echo) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	return rec
}

func TestPassesThroughUnderLimit(t *testing.T) {
	t.Parallel()
	e := newEcho(httpguard.Config{MaxInflight: 2, Timeout: time.Second}, func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})
	if rec := do(e); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

// 同時実行の上限を超えたら、待たせずに 503 + Retry-After + Error 形式の JSON を返す。
func TestRejectsWhenInflightLimitReached(t *testing.T) {
	t.Parallel()
	started := make(chan struct{})
	release := make(chan struct{})
	first := true
	e := newEcho(httpguard.Config{MaxInflight: 1, Timeout: time.Second}, func(c *echo.Context) error {
		if first {
			first = false
			close(started)
			<-release
		}
		return c.String(http.StatusOK, "ok")
	})
	done := make(chan int)
	go func() { done <- do(e).Code }()
	<-started

	rec := do(e)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Errorf("Retry-After = %q, want \"1\"", got)
	}
	var body struct{ Code, Message string }
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Code != "overloaded" || body.Message == "" {
		t.Errorf("body = %q (%v), want Error 形式の overloaded", rec.Body.String(), err)
	}
	close(release)
	if code := <-done; code != http.StatusOK {
		t.Errorf("先行リクエストの status = %d, want 200", code)
	}
	// 解放後は再び受け付ける(セマフォが戻る)。
	if rec := do(e); rec.Code != http.StatusOK {
		t.Errorf("解放後の status = %d, want 200", rec.Code)
	}
}

// ハンドラの context に締め切りが付く。
func TestSetsRequestDeadline(t *testing.T) {
	t.Parallel()
	var remaining time.Duration
	e := newEcho(httpguard.Config{MaxInflight: 1, Timeout: 500 * time.Millisecond}, func(c *echo.Context) error {
		dl, ok := c.Request().Context().Deadline()
		if !ok {
			t.Error("context に締め切りが無い")
		}
		remaining = time.Until(dl)
		return c.NoContent(http.StatusNoContent)
	})
	do(e)
	if remaining <= 0 || remaining > 500*time.Millisecond {
		t.Errorf("残り = %v, want (0, 500ms]", remaining)
	}
}

// Timeout が 0 なら締め切りを張らない(上限だけ使うサービス用)。上限が 0 なら無制限。
func TestZeroValuesDisable(t *testing.T) {
	t.Parallel()
	e := newEcho(httpguard.Config{}, func(c *echo.Context) error {
		if _, ok := c.Request().Context().Deadline(); ok {
			t.Error("締め切りが付いている")
		}
		return c.NoContent(http.StatusNoContent)
	})
	if rec := do(e); rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestExpired(t *testing.T) {
	t.Parallel()
	if httpguard.Expired(context.Background()) {
		t.Error("締め切りの無い context は期限切れでない")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !httpguard.Expired(ctx) {
		t.Error("終了した context は期限切れ")
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		cfg          httpguard.Config
		writeTimeout time.Duration
		wantErr      bool
	}{
		{"締め切りが writeTimeout 未満", httpguard.Config{Timeout: 14 * time.Second}, 15 * time.Second, false},
		{"同じは不可", httpguard.Config{Timeout: 15 * time.Second}, 15 * time.Second, true},
		{"超過は不可", httpguard.Config{Timeout: 20 * time.Second}, 15 * time.Second, true},
		{"0 の締め切りは writeTimeout 検査の対象外", httpguard.Config{}, 15 * time.Second, false},
		{"負は不可", httpguard.Config{Timeout: -1}, 15 * time.Second, true},
	}
	for _, tt := range tests {
		if err := tt.cfg.Validate(tt.writeTimeout); (err != nil) != tt.wantErr {
			t.Errorf("%s: err = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestDeadlineFor(t *testing.T) {
	t.Parallel()
	if got := httpguard.DeadlineFor(15 * time.Second); got != 14*time.Second {
		t.Errorf("DeadlineFor(15s) = %v, want 14s", got)
	}
}

func TestCustomCode(t *testing.T) {
	t.Parallel()
	block := make(chan struct{})
	entered := make(chan struct{})
	e := newEcho(httpguard.Config{MaxInflight: 1, Code: "upstream_unavailable"}, func(c *echo.Context) error {
		select {
		case <-entered:
		default:
			close(entered)
			<-block
		}
		return c.NoContent(http.StatusNoContent)
	})
	go do(e)
	<-entered
	rec := do(e)
	close(block)
	var body struct{ Code string }
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Code != "upstream_unavailable" {
		t.Errorf("body = %q (%v), want code upstream_unavailable", rec.Body.String(), err)
	}
}
