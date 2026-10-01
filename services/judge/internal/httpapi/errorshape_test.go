package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/judge/internal/api"
)

// 未知ルート・メソッド違い・panic は、契約の Error 形({code, message})で返す(issue #325)。
// メソッド違いは calc・gateway と同じく 404 not_found(ADR-0200・ADR-0217)。
func TestErrorShapeForRoutingAndPanic(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantCode   api.ErrorCode
	}{
		{"unknown route", http.MethodGet, "/nope", http.StatusNotFound, api.NotFound},
		{"method not allowed", http.MethodDelete, "/healthz", http.StatusNotFound, api.NotFound},
		{"panic is recovered", http.MethodGet, "/__panic", http.StatusInternalServerError, api.InternalError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := New(Dependencies{})
			e.GET("/__panic", func(*echo.Context) error { panic("secret detail") })
			recorder := httptest.NewRecorder()
			e.ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.path, nil))

			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tc.wantStatus, recorder.Body.String())
			}
			var body api.Error
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("body is not an Error: %v; body=%s", err, recorder.Body.String())
			}
			if body.Code != tc.wantCode || body.Message == "" {
				t.Errorf("body = %+v, want code %q with a message", body, tc.wantCode)
			}
			if strings.Contains(recorder.Body.String(), "secret detail") {
				t.Errorf("panic detail leaked: %s", recorder.Body.String())
			}
		})
	}
}

// panic の 500 もメトリクスに数える(回復 middleware はメトリクス middleware の内側)。
func TestRecoveredPanicIsCountedInMetrics(t *testing.T) {
	t.Parallel()
	e := New(Dependencies{})
	e.GET("/__panic", func(*echo.Context) error { panic("boom") })

	rec := assertCountedOnce(t, e, metricsRequest{method: http.MethodGet, target: "/__panic"}, "/__panic")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

// 本文を書き出した後の panic は、Error 本文を連結しない(書き出し済みなら何も足さない)。
func TestPanicAfterPartialWriteDoesNotAppendBody(t *testing.T) {
	t.Parallel()
	e := New(Dependencies{})
	e.GET("/__partial", func(c *echo.Context) error {
		_ = c.String(http.StatusOK, "partial")
		panic("boom")
	})
	recorder := httptest.NewRecorder()
	e.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/__partial", nil))
	if got := recorder.Body.String(); got != "partial" {
		t.Errorf("body = %q, want %q", got, "partial")
	}
}

// http.ErrAbortHandler は net/http の「応答を中断する」合図なので、回復せず再 panic する。
func TestAbortHandlerPanicIsNotRecovered(t *testing.T) {
	t.Parallel()
	e := New(Dependencies{})
	e.GET("/__abort", func(*echo.Context) error { panic(http.ErrAbortHandler) })
	defer func() {
		if recovered := recover(); recovered != http.ErrAbortHandler {
			t.Errorf("recovered = %v, want http.ErrAbortHandler", recovered)
		}
	}()
	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/__abort", nil))
}
