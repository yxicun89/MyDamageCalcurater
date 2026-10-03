package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHealthReportsDataVersion: 読み込んだ read model の版が両方のヘルスに出る。無ければ従来の本文のまま(ADR-0138)。
func TestHealthReportsDataVersion(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"/healthz", "/api/speed/healthz"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			New(Dependencies{DataVersion: "v1-abcd1234"}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d", recorder.Code)
			}
			if got := strings.TrimSpace(recorder.Body.String()); got != `{"dataVersion":"v1-abcd1234","status":"ok"}` {
				t.Errorf("body = %s", got)
			}
		})
	}
}
