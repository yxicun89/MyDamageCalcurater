package httpapi_test

// DB を使う操作の同時実行の上限(issue #299 の pokedex 分・ADR-0801)。上限を超えた要求は DB を待たずに
// 503 upstream_unavailable + Retry-After。/healthz・/readyz は対象外(probe を 503 にしない)。

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/pokedex/internal/httpapi"
)

func TestDBRoutesRejectOverTheInflightLimit(t *testing.T) {
	db := newStuckDB()
	// 先行の要求は stuckDB が締め切り(testDeadline)まで DB の呼び出しを止めるので、その間は枠を占有する。
	h := httpapi.NewHandler(db, httpapi.WithRequestTimeout(testDeadline*10), httpapi.WithMaxInflight(1))

	var wg sync.WaitGroup
	wg.Go(func() { do(t, h, http.MethodGet, "/api/pokedex/natures", true) })
	deadline := time.Now().Add(stuckCap)
	for len(db.calls()) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("先行の要求が DB に届かなかった")
		}
		time.Sleep(time.Millisecond)
	}

	rec := do(t, h, http.MethodGet, "/api/pokedex/natures", true)
	assertError(t, rec, http.StatusServiceUnavailable, api.UpstreamUnavailable)
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After が無い")
	}
	for _, path := range []string{"/healthz", "/metrics"} {
		probe := httptest.NewRecorder()
		h.ServeHTTP(probe, httptest.NewRequest(http.MethodGet, path, nil))
		if probe.Code != http.StatusOK {
			t.Errorf("%s = %d, want 200(上限の対象外)", path, probe.Code)
		}
	}
	wg.Wait()
}

func TestDefaultMaxInflightIsPositive(t *testing.T) {
	if httpapi.DefaultMaxInflight <= 0 {
		t.Errorf("DefaultMaxInflight = %d, want 正", httpapi.DefaultMaxInflight)
	}
}
