package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

// issue #325 / ADR-0802: 契約に無い経路・メソッド違い・panic も、他の失敗と同じ {code, message} の JSON で返す。

func decodeCodeBody(t *testing.T, recorder *httptest.ResponseRecorder) (code, message string) {
	t.Helper()
	if ct := recorder.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json; body=%s", ct, recorder.Body.String())
	}
	var body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v; body=%s", err, recorder.Body.String())
	}
	return body.Code, body.Message
}

func TestUnknownRouteAndMethodReturnNotFoundJSON(t *testing.T) {
	t.Parallel()

	cases := []struct{ method, path string }{
		{http.MethodGet, "/nope"},
		{http.MethodDelete, "/healthz"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			New(Dependencies{}).ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.path, nil))
			if recorder.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404; body=%s", recorder.Code, recorder.Body.String())
			}
			if code, message := decodeCodeBody(t, recorder); code != "not_found" || message == "" {
				t.Errorf("code=%q message=%q, want not_found with a message", code, message)
			}
		})
	}
}

func TestPanicIsRecoveredAsInternalErrorJSON(t *testing.T) {
	t.Parallel()

	e := New(Dependencies{})
	e.GET("/boom", func(*echo.Context) error { panic("secret internal detail") })
	recorder := httptest.NewRecorder()
	e.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/boom", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", recorder.Code, recorder.Body.String())
	}
	code, message := decodeCodeBody(t, recorder)
	if code != "internal_error" {
		t.Errorf("code = %q, want internal_error", code)
	}
	if strings.Contains(message, "secret") || strings.Contains(recorder.Body.String(), "goroutine") {
		t.Errorf("internal detail leaked: %s", recorder.Body.String())
	}
}
