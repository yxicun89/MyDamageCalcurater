// Package dbwait は起動直後の DB 接続の再試行。Pod の起動直後は NetworkPolicy の許可が反映されず、
// 数秒間 MySQL に繋がらない(connection refused)ことがあるため、Ping を間隔を伸ばしながら再試行する。
package dbwait

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const (
	// InitialInterval は最初の待ち。以後 2 倍ずつ伸ばす。
	InitialInterval = time.Second
	// MaxInterval は待ちの上限。
	MaxInterval = 5 * time.Second
	// Timeout は再試行を続ける合計時間。
	Timeout = 30 * time.Second
)

// Options は時計と待ちの差し替え(テスト用。ゼロ値なら実際の時計)。
type Options struct {
	Now   func() time.Time
	Sleep func(ctx context.Context, d time.Duration) error
}

func realSleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Retry は ping が成功するまで、Timeout を上限に再試行する。ctx が終わったらその err を返す。
// 期限切れのときは最後の失敗を包んで返す(ping のエラーは DSN を含まない前提)。
func Retry(ctx context.Context, ping func(ctx context.Context) error, o Options) error {
	now, sleep := o.Now, o.Sleep
	if now == nil {
		now = time.Now
	}
	if sleep == nil {
		sleep = realSleep
	}
	deadline := now().Add(Timeout)
	interval := InitialInterval
	for {
		err := ping(ctx)
		if err == nil {
			return nil
		}
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		remaining := deadline.Sub(now())
		if remaining <= 0 {
			return fmt.Errorf("database is not reachable after %s: %w", Timeout, err)
		}
		if err := sleep(ctx, min(interval, remaining)); err != nil {
			return err
		}
		interval = min(interval*2, MaxInterval)
	}
}

// Ping は db への接続を Retry で確かめる。
func Ping(ctx context.Context, db *sql.DB) error {
	return Retry(ctx, db.PingContext, Options{})
}
