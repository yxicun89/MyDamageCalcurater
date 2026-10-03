package dbwait_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/dbwait"
)

type fakeClock struct {
	now    time.Time
	sleeps []time.Duration
}

func (c *fakeClock) opts() dbwait.Options {
	return dbwait.Options{
		Now: func() time.Time { return c.now },
		Sleep: func(ctx context.Context, d time.Duration) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			c.sleeps = append(c.sleeps, d)
			c.now = c.now.Add(d)
			return nil
		},
	}
}

func TestRetry_SucceedsAfterFailures(t *testing.T) {
	c := &fakeClock{now: time.Unix(0, 0)}
	n := 0
	err := dbwait.Retry(context.Background(), func(context.Context) error {
		if n++; n <= 5 {
			return errors.New("connection refused")
		}
		return nil
	}, c.opts())
	if err != nil {
		t.Fatal(err)
	}
	want := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 5 * time.Second, 5 * time.Second}
	if !slices.Equal(c.sleeps, want) {
		t.Errorf("待ち = %v, want %v", c.sleeps, want)
	}
}

func TestRetry_Timeout(t *testing.T) {
	c := &fakeClock{now: time.Unix(0, 0)}
	boom := errors.New("connection refused")
	err := dbwait.Retry(context.Background(), func(context.Context) error { return boom }, c.opts())
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want 最後の失敗を包む", err)
	}
	var total time.Duration
	for _, d := range c.sleeps {
		total += d
	}
	if total != dbwait.Timeout {
		t.Errorf("待ちの合計 = %v, want %v", total, dbwait.Timeout)
	}
}

func TestRetry_Canceled(t *testing.T) {
	c := &fakeClock{now: time.Unix(0, 0)}
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	err := dbwait.Retry(ctx, func(context.Context) error {
		calls++
		cancel()
		return errors.New("x")
	}, c.opts())
	if !errors.Is(err, context.Canceled) || calls != 1 || len(c.sleeps) != 0 {
		t.Errorf("err = %v, calls = %d, sleeps = %v", err, calls, c.sleeps)
	}
}
