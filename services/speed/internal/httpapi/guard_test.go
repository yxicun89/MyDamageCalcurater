package httpapi

// 過負荷・締め切りの守り(issue #299・ADR-0801)。上限超過は待たせず 503 overloaded + Retry-After、
// 締め切り後は計算(BuildTable・Position)を新しく始めない。/healthz・/metrics は対象外。

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"example.com/pokecalc/services/speed/internal/api"
	"example.com/pokecalc/services/speed/internal/httpguard"
	"example.com/pokecalc/services/speed/internal/speed"
)

// blockingProvider は最初の Roster 呼び出しで止まる(そのリクエストが枠を占有し続ける)。
type blockingProvider struct {
	once    sync.Once
	entered chan struct{}
	release chan struct{}
	roster  speed.Roster
	mu      sync.Mutex
	calls   int
}

func newBlockingProvider() *blockingProvider {
	return &blockingProvider{entered: make(chan struct{}), release: make(chan struct{}), roster: unorderedRoster()}
}

func (b *blockingProvider) Roster() (speed.Roster, error) {
	b.mu.Lock()
	b.calls++
	b.mu.Unlock()
	b.once.Do(func() {
		close(b.entered)
		<-b.release
	})
	return b.roster, nil
}

func TestOverloadReturns503WithRetryAfter(t *testing.T) {
	provider := newBlockingProvider()
	handler := New(Dependencies{Pokemon: provider, Guard: httpguard.Config{MaxInflight: 1, Timeout: 5 * time.Second}})

	done := make(chan int, 1)
	go func() {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, newRequest(pokemonPath, validHeaders))
		done <- rec.Code
	}()
	<-provider.entered

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newRequest(pokemonPath, validHeaders))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if got := decodeError(t, rec).Code; got != api.Overloaded {
		t.Errorf("code = %q, want overloaded", got)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After が無い")
	}
	for _, path := range []string{"/healthz", "/metrics"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, newRequest(path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s = %d, want 200(上限の対象外)", path, rec.Code)
		}
	}
	close(provider.release)
	if code := <-done; code != http.StatusOK {
		t.Errorf("先行リクエスト = %d, want 200", code)
	}
}

// 締め切りが過ぎた後は計算を始めない: 表(BuildTable)・位置(Position)とも 503 overloaded。
func TestExpiredDeadlineSkipsComputation(t *testing.T) {
	deps := Dependencies{
		Pokemon: fakeProvider{roster: unorderedRoster()},
		Guard:   httpguard.Config{MaxInflight: 4, Timeout: time.Nanosecond},
	}
	table := newRequest("/api/speed/v1/table", validHeaders)
	if rec := serve(deps, table); rec.Code != http.StatusServiceUnavailable || decodeError(t, rec).Code != api.Overloaded {
		t.Errorf("table = %d %s, want 503 overloaded", rec.Code, rec.Body.String())
	}

	position := newPositionRequest(validHeaders, `{"mode":"raw","value":200}`)
	if rec := serve(deps, position); rec.Code != http.StatusServiceUnavailable || decodeError(t, rec).Code != api.Overloaded {
		t.Errorf("position = %d %s, want 503 overloaded", rec.Code, rec.Body.String())
	}
}
