package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"example.com/pokecalc/services/judge/internal/api"
	"example.com/pokecalc/services/judge/internal/httpguard"
)

// issue #299(ADR-0801): 同時実行の上限を超えたリクエストは、上流を呼ばずに待たせず
// 503 upstream_unavailable + Retry-After。全体の締め切りは RequestTimeout(ADR-0707)が担うので、
// judge の guard は上限だけを使う(Timeout は 0)。/healthz は対象外。
func TestOverloadReturns503WithRetryAfter(t *testing.T) {
	t.Parallel()

	stub := &upstreams{delay: 200 * time.Millisecond}
	deps := newUpstreams(t, stub)
	deps.Guard = httpguard.Config{MaxInflight: 1, Code: string(api.UpstreamUnavailable)}

	// 枠は Handler ごとに数えるので、1 つの Handler に 2 本を同時に送る。
	handler := New(deps)
	var wg sync.WaitGroup
	recorders := make([]*httptest.ResponseRecorder, 2)
	for i := range recorders {
		wg.Go(func() {
			request := httptest.NewRequest(http.MethodPost, outspeedPath, bytes.NewReader(mustJSON(validBody())))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Device-Id", testDeviceID)
			request.Header.Set("X-Session-Id", testSessionID)
			recorders[i] = httptest.NewRecorder()
			handler.ServeHTTP(recorders[i], request)
		})
	}
	wg.Wait()

	var ok, rejected *httptest.ResponseRecorder
	for _, r := range recorders {
		switch r.Code {
		case http.StatusOK:
			ok = r
		case http.StatusServiceUnavailable:
			rejected = r
		}
	}
	if ok == nil || rejected == nil {
		t.Fatalf("status = %d / %d, want one 200 and one 503", recorders[0].Code, recorders[1].Code)
	}
	assertStatusAndCode(t, rejected, http.StatusServiceUnavailable, api.UpstreamUnavailable)
	if rejected.Header().Get("Retry-After") == "" {
		t.Error("Retry-After が無い")
	}
}
