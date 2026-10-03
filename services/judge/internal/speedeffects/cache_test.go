package speedeffects

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"example.com/pokecalc/services/judge/internal/client"
)

// 素早さ効果の表のキャッシュ(ADR-0714 §1): 遅延ロード・TTL・同時取得は 1 回(singleflight)・
// 取得失敗はフェイルソフト(ok=false。呼び出し側は「確定できない」として扱う)・失敗後は RetryAfterFailure の間
// 取りに行かない・取得済みの表があれば更新の失敗中も古い表を使い続ける。時計は Config.Now で差し替える。

// fakeClock は手で進める時計。
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// countingFetcher は呼ばれた回数を数え、results を順に返す(尽きたら最後の結果を繰り返す)。
type countingFetcher struct {
	calls   atomic.Int32
	mu      sync.Mutex
	results []fetchResult
}

type fetchResult struct {
	master client.MasterEffects
	err    error
}

func (f *countingFetcher) fetch(context.Context) (client.MasterEffects, error) {
	n := int(f.calls.Add(1))
	f.mu.Lock()
	defer f.mu.Unlock()
	r := f.results[min(n, len(f.results))-1]
	return r.master, r.err
}

func masterWithAbility(id string, modifier string) client.MasterEffects {
	return client.MasterEffects{Abilities: []client.MasterEffectEntry{
		{ID: id, Effect: json.RawMessage(`{"SpeedMods":[{"Condition":"always","Modifier":` + modifier + `}]}`)},
	}}
}

const (
	testTTL   = 10 * time.Minute
	testRetry = 30 * time.Second
)

func newTestCache(t *testing.T, fetch Fetcher, clock *fakeClock) *Cache {
	t.Helper()
	cache, err := NewCache(fetch, Config{TTL: testTTL, RetryAfterFailure: testRetry, Now: clock.Now})
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	return cache
}

func TestNewCacheRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	fetch := func(context.Context) (client.MasterEffects, error) { return client.MasterEffects{}, nil }
	tests := map[string]struct {
		fetch Fetcher
		cfg   Config
	}{
		"fetch が nil":                {nil, Config{TTL: testTTL, RetryAfterFailure: testRetry}},
		"TTL が 0":                    {fetch, Config{TTL: 0, RetryAfterFailure: testRetry}},
		"TTL が負":                     {fetch, Config{TTL: -time.Second, RetryAfterFailure: testRetry}},
		"RetryAfterFailure が 0":      {fetch, Config{TTL: testTTL, RetryAfterFailure: 0}},
		"RetryAfterFailure が TTL 超え": {fetch, Config{TTL: time.Minute, RetryAfterFailure: 2 * time.Minute}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewCache(tt.fetch, tt.cfg); err == nil {
				t.Errorf("NewCache(%+v) err = nil, want error", tt.cfg)
			}
		})
	}
	// Now が nil なら time.Now を使う(エラーにしない)。
	if _, err := NewCache(fetch, Config{TTL: testTTL, RetryAfterFailure: testRetry}); err != nil {
		t.Errorf("Now 省略: %v", err)
	}
}

// TestCacheLazyLoadAndTTL: 最初の Table で 1 回取り、TTL の間は取りに行かない。TTL を過ぎたら取り直す。
func TestCacheLazyLoadAndTTL(t *testing.T) {
	t.Parallel()

	clock := &fakeClock{now: time.Unix(1_700_000_000, 0)}
	f := &countingFetcher{results: []fetchResult{
		{master: masterWithAbility("test-a", "8192")},
		{master: masterWithAbility("test-a", "2048")},
	}}
	cache := newTestCache(t, f.fetch, clock)

	if f.calls.Load() != 0 {
		t.Fatalf("NewCache の時点で取得している(遅延ロードにする)")
	}
	table, ok := cache.Table(context.Background())
	if !ok {
		t.Fatal("最初の Table が ok=false")
	}
	if got, _ := table.Ability("test-a"); got.Mods[0].Modifier != 8192 {
		t.Errorf("1 回目の表 = %+v", got)
	}

	clock.Advance(testTTL - time.Second)
	for range 5 {
		if _, ok := cache.Table(context.Background()); !ok {
			t.Fatal("TTL 内で ok=false")
		}
	}
	if n := f.calls.Load(); n != 1 {
		t.Errorf("TTL 内の取得回数 = %d, want 1", n)
	}

	clock.Advance(2 * time.Second)
	table, ok = cache.Table(context.Background())
	if !ok {
		t.Fatal("TTL 後の Table が ok=false")
	}
	if n := f.calls.Load(); n != 2 {
		t.Errorf("TTL 後の取得回数 = %d, want 2", n)
	}
	if got, _ := table.Ability("test-a"); got.Mods[0].Modifier != 2048 {
		t.Errorf("TTL 後は新しい表を使う: %+v", got)
	}
}

// TestCacheSingleflight: 空のキャッシュに同時に来た要求でも、取得は 1 回だけ。
func TestCacheSingleflight(t *testing.T) {
	t.Parallel()

	clock := &fakeClock{now: time.Unix(1_700_000_000, 0)}
	release := make(chan struct{})
	var calls atomic.Int32
	fetch := func(context.Context) (client.MasterEffects, error) {
		calls.Add(1)
		<-release
		return masterWithAbility("test-a", "8192"), nil
	}
	cache := newTestCache(t, fetch, clock)

	const n = 20
	var wg sync.WaitGroup
	oks := make([]bool, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, oks[i] = cache.Table(context.Background())
		}()
	}
	// 全員が待ちに入るまで少し待ってから取得を終わらせる。
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Errorf("同時の取得回数 = %d, want 1", got)
	}
	for i, ok := range oks {
		if !ok {
			t.Errorf("要求 %d が ok=false", i)
		}
	}
}

// TestCacheFailSoftAndRetryAfterFailure: 取得に失敗したら ok=false(エラーにしない)。失敗から
// RetryAfterFailure の間は取りに行かず ok=false を返す(上流が落ちている間、要求ごとに叩かない)。過ぎたら取り直す。
func TestCacheFailSoftAndRetryAfterFailure(t *testing.T) {
	t.Parallel()

	clock := &fakeClock{now: time.Unix(1_700_000_000, 0)}
	f := &countingFetcher{results: []fetchResult{
		{err: client.ErrUpstreamUnavailable},
		{master: masterWithAbility("test-a", "8192")},
	}}
	cache := newTestCache(t, f.fetch, clock)

	if _, ok := cache.Table(context.Background()); ok {
		t.Fatal("取得失敗で ok=true")
	}
	clock.Advance(testRetry - time.Second)
	if _, ok := cache.Table(context.Background()); ok {
		t.Error("RetryAfterFailure 内で ok=true")
	}
	if n := f.calls.Load(); n != 1 {
		t.Errorf("RetryAfterFailure 内の取得回数 = %d, want 1", n)
	}
	clock.Advance(2 * time.Second)
	table, ok := cache.Table(context.Background())
	if !ok {
		t.Fatal("RetryAfterFailure 後の取り直しで ok=false")
	}
	if _, known := table.Ability("test-a"); !known {
		t.Error("取り直した表に test-a が無い")
	}
	if n := f.calls.Load(); n != 2 {
		t.Errorf("取得回数 = %d, want 2", n)
	}
}

// TestCacheKeepsStaleTableOnRefreshFailure: 取得済みの表があれば、TTL 後の更新に失敗しても古い表を使い続け
// (ok=true)、RetryAfterFailure の後にまた取り直す。
func TestCacheKeepsStaleTableOnRefreshFailure(t *testing.T) {
	t.Parallel()

	clock := &fakeClock{now: time.Unix(1_700_000_000, 0)}
	f := &countingFetcher{results: []fetchResult{
		{master: masterWithAbility("test-a", "8192")},
		{err: errors.New("boom")},
		{master: masterWithAbility("test-a", "2048")},
	}}
	cache := newTestCache(t, f.fetch, clock)

	if _, ok := cache.Table(context.Background()); !ok {
		t.Fatal("1 回目で ok=false")
	}
	clock.Advance(testTTL + time.Second)
	table, ok := cache.Table(context.Background())
	if !ok {
		t.Fatal("更新の失敗中に古い表を捨てた(ok=false)")
	}
	if got, _ := table.Ability("test-a"); got.Mods[0].Modifier != 8192 {
		t.Errorf("古い表 = %+v", got)
	}
	clock.Advance(testRetry - time.Second)
	_, _ = cache.Table(context.Background())
	if n := f.calls.Load(); n != 2 {
		t.Errorf("RetryAfterFailure 内の取得回数 = %d, want 2", n)
	}
	clock.Advance(2 * time.Second)
	table, _ = cache.Table(context.Background())
	if got, _ := table.Ability("test-a"); got.Mods[0].Modifier != 2048 {
		t.Errorf("取り直し後の表 = %+v", got)
	}
}

// TestCacheCallerCancelDoesNotAbortFetch: 待っている要求の ctx が終わったらその要求は ok=false で
// すぐ戻るが、取得そのものは止めない(他の要求・次の要求がその結果を使う。取得は呼び出し元の ctx から切り離す)。
func TestCacheCallerCancelDoesNotAbortFetch(t *testing.T) {
	t.Parallel()

	clock := &fakeClock{now: time.Unix(1_700_000_000, 0)}
	release := make(chan struct{})
	var calls atomic.Int32
	var fetchCtxErr atomic.Value
	fetch := func(ctx context.Context) (client.MasterEffects, error) {
		calls.Add(1)
		<-release
		if err := ctx.Err(); err != nil {
			fetchCtxErr.Store(err)
		}
		return masterWithAbility("test-a", "8192"), nil
	}
	cache := newTestCache(t, fetch, clock)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan bool)
	go func() {
		_, ok := cache.Table(ctx)
		done <- ok
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case ok := <-done:
		if ok {
			t.Error("取り消した要求が ok=true")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ctx を取り消しても Table が戻らない")
	}

	close(release)
	// 取得が終わるのを待ってから次の要求(取得し直さずに結果を使う)。
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, ok := cache.Table(context.Background()); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("取得の結果が次の要求に使われない")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("取得回数 = %d, want 1", n)
	}
	if err := fetchCtxErr.Load(); err != nil {
		t.Errorf("取得の ctx が呼び出し元の取り消しで終わった: %v", err)
	}
}

// TestNilCacheIsUnavailable: nil の *Cache は「データなし」(ok=false)。Dependencies で未設定のとき。
func TestNilCacheIsUnavailable(t *testing.T) {
	t.Parallel()
	var cache *Cache
	if _, ok := cache.Table(context.Background()); ok {
		t.Error("nil の Cache が ok=true")
	}
}
