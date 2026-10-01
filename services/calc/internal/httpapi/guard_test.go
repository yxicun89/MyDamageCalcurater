package httpapi

// 過負荷・締め切りの守り(issue #299・ADR-0801)。
//   - 同時実行の上限を超えたら、待たせずに 503 + Retry-After(計算を始めない)
//   - 締め切りが過ぎたら engine を新しく呼ばない(503)
//   - /healthz・/readyz・/metrics は上限・締め切りの対象外(probe を 503 にしない)

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"example.com/pokecalc/services/calc/internal/master"
	"example.com/pokecalc/services/internal/calcevents"
	"example.com/pokecalc/services/internal/httpguard"
)

// blockingPublisher は最初の Publish で止まる(= そのリクエストが同時実行の枠を占有し続ける)。
type blockingPublisher struct {
	once    sync.Once
	entered chan struct{}
	release chan struct{}
	mu      sync.Mutex
	calls   int
}

func newBlockingPublisher() *blockingPublisher {
	return &blockingPublisher{entered: make(chan struct{}), release: make(chan struct{})}
}

func (b *blockingPublisher) Publish(string, string, string, time.Time, *calcevents.CalcDetail) {
	b.mu.Lock()
	b.calls++
	b.mu.Unlock()
	b.once.Do(func() {
		close(b.entered)
		<-b.release
	})
}

func (b *blockingPublisher) count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls
}

func validCalcBody(t *testing.T) []byte {
	t.Helper()
	c := calcCase{
		name:     "guard",
		attacker: indiv{speciesKey: speciesAttacker, natureID: natureNeutral},
		defender: indiv{speciesKey: speciesDefender, natureID: natureNeutral},
		moveID:   movePhysical,
	}
	return mustJSON(t, c.httpBody())
}

func TestOverloadReturns503WithRetryAfter(t *testing.T) {
	pub := newBlockingPublisher()
	h := NewHandler(newFakeStore(t), pub, WithGuard(httpguard.Config{MaxInflight: 1, Timeout: 5 * time.Second}))
	body := validCalcBody(t)

	done := make(chan int, 1)
	go func() { done <- serve(t, h, http.MethodPost, "/api/calc", validHeaders(), body).Code }()
	<-pub.entered

	rec := serve(t, h, http.MethodPost, "/api/calc", validHeaders(), body)
	assertError(t, rec, http.StatusServiceUnavailable, "upstream_unavailable")
	if got := rec.Header().Get("Retry-After"); got == "" {
		t.Error("Retry-After が無い")
	}
	if pub.count() != 1 {
		t.Errorf("拒否されたリクエストが計算まで進んだ(Publish 回数 = %d, want 1)", pub.count())
	}
	// 運用エンドポイントは枠が埋まっていても答える。
	for _, path := range []string{"/healthz", "/readyz", "/metrics"} {
		if rec := serve(t, h, http.MethodGet, path, nil, nil); rec.Code != http.StatusOK {
			t.Errorf("%s = %d, want 200(上限の対象外)", path, rec.Code)
		}
	}
	close(pub.release)
	if code := <-done; code != http.StatusOK {
		t.Errorf("先行リクエスト = %d, want 200", code)
	}
}

func TestDeferredHandlerAlsoGuards(t *testing.T) {
	pub := newBlockingPublisher()
	store := newFakeStore(t)
	h := NewDeferredHandler(func() master.Store { return store }, pub, WithGuard(httpguard.Config{MaxInflight: 1, Timeout: 5 * time.Second}))
	body := validCalcBody(t)

	done := make(chan int, 1)
	go func() { done <- serve(t, h, http.MethodPost, "/api/calc", validHeaders(), body).Code }()
	<-pub.entered
	assertError(t, serve(t, h, http.MethodPost, "/api/calc", validHeaders(), body), http.StatusServiceUnavailable, "upstream_unavailable")
	close(pub.release)
	<-done
}

// 締め切りが過ぎた後は engine を新しく呼ばない(Publish は engine の成功後にだけ呼ばれる)。
func TestExpiredDeadlineSkipsEngine(t *testing.T) {
	store := newFakeStore(t)
	for _, tt := range []struct {
		path string
		body []byte
	}{
		{"/api/calc", validCalcBody(t)},
		{"/api/calc/bulk", mustJSON(t, bulkBody(movePhysical, nil, nil))},
		{"/api/calc/reverse", mustJSON(t, reverseCases(t, store)[0].httpBody())},
	} {
		t.Run(tt.path, func(t *testing.T) {
			pub := &fakePublisher{}
			h := NewHandler(store, pub, WithGuard(httpguard.Config{MaxInflight: 4, Timeout: time.Nanosecond}))
			rec := serve(t, h, http.MethodPost, tt.path, validHeaders(), tt.body)
			assertError(t, rec, http.StatusServiceUnavailable, "upstream_unavailable")
			if len(pub.calls) != 0 {
				t.Errorf("締め切り後に計算が走った: %+v", pub.calls)
			}
		})
	}
}

func TestDefaultsLeaveRoomForWriteTimeout(t *testing.T) {
	if DefaultMaxInflight <= 0 {
		t.Errorf("DefaultMaxInflight = %d, want 正", DefaultMaxInflight)
	}
	if DefaultRequestTimeout <= 0 {
		t.Errorf("DefaultRequestTimeout = %v, want 正", DefaultRequestTimeout)
	}
}
