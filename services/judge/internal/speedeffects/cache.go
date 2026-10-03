package speedeffects

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"example.com/pokecalc/services/judge/internal/client"
)

// Fetcher fetches the master's ability and item effects (client.Pokedex.MasterEffects).
type Fetcher func(ctx context.Context) (client.MasterEffects, error)

// Config is the Cache's input. Now defaults to time.Now (tests swap it).
type Config struct {
	// TTL is how long a fetched table is used before it is fetched again.
	TTL time.Duration
	// RetryAfterFailure is how long to leave the master alone after a failed fetch. It must be
	// positive and no longer than TTL.
	RetryAfterFailure time.Duration
	Now               func() time.Time
}

// Cache loads the table lazily and refreshes it after TTL (ADR-0714 §1). Concurrent callers share
// one fetch. A failed fetch is fail-soft: Table reports ok=false (no table yet) or keeps serving the
// stale table, and the master is not asked again until RetryAfterFailure has passed.
type Cache struct {
	fetch Fetcher
	cfg   Config

	mu          sync.Mutex
	table       Table
	has         bool
	nextRefresh time.Time
	inflight    *flight
}

type flight struct{ done chan struct{} }

// NewCache validates cfg and builds a Cache without fetching.
func NewCache(fetch Fetcher, cfg Config) (*Cache, error) {
	if fetch == nil {
		return nil, errors.New("speed effects fetcher is required")
	}
	if cfg.TTL <= 0 {
		return nil, errors.New("speed effects TTL must be positive")
	}
	if cfg.RetryAfterFailure <= 0 || cfg.RetryAfterFailure > cfg.TTL {
		return nil, errors.New("speed effects retry interval must be positive and no longer than the TTL")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Cache{fetch: fetch, cfg: cfg}, nil
}

// Table returns the current table. ok is false when there is none (never fetched successfully, or
// ctx ended while waiting for the first fetch); callers then treat every id as undeterminable. A nil
// Cache has no table. The fetch itself is detached from ctx, so one caller leaving does not abort
// it for the others.
func (c *Cache) Table(ctx context.Context) (Table, bool) {
	if c == nil {
		return Table{}, false
	}
	c.mu.Lock()
	if c.cfg.Now().Before(c.nextRefresh) {
		table, has := c.table, c.has
		c.mu.Unlock()
		return table, has
	}
	if c.inflight == nil {
		f := &flight{done: make(chan struct{})}
		c.inflight = f
		go c.refresh(context.WithoutCancel(ctx), f)
	}
	f := c.inflight
	c.mu.Unlock()

	select {
	case <-f.done:
	case <-ctx.Done():
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.table, c.has
}

func (c *Cache) refresh(ctx context.Context, f *flight) {
	master, err := c.fetch(ctx)

	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.cfg.Now()
	if err != nil {
		// Only the sentinel text is logged: never the upstream's body or URL (ADR-0700 §3).
		slog.Warn("judge speed effects fetch failed; speed effects of abilities and items are not applied", "error", err)
		c.nextRefresh = now.Add(c.cfg.RetryAfterFailure)
	} else {
		c.table = BuildTable(master)
		c.has = true
		c.nextRefresh = now.Add(c.cfg.TTL)
	}
	c.inflight = nil
	close(f.done)
}
