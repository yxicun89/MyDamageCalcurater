//go:build mysql

package db

// 実 MySQL を使う接続プールの統合テスト(issue #112 / ADR-0112 受け入れ条件3・4)。
// `make test-db` だけが実行する(POKEDEX_TEST_DSN が必須。mysql_test.go の testDSN と同じ
// 流儀。DB に届かないときは失敗する。スキップしない)。

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"
)

// waitUntilStats は cond を満たすまで最大 timeout ポーリングする(達しなければ Fatal)。
func waitUntilStats(t *testing.T, pool *sql.DB, timeout time.Duration, cond func(sql.DBStats) bool) sql.DBStats {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		s := pool.Stats()
		if cond(s) {
			return s
		}
		if time.Now().After(deadline) {
			t.Fatalf("条件を満たさないまま %v 経過: %+v", timeout, s)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// AC3: MaxOpenConns を超える同時クエリを投げても Stats().OpenConnections が MaxOpenConns を
// 超えない(超過分はプールで待機し、新規接続を無制限に開かない)。SLEEP() で各クエリを
// 意図的に遅くし、直列化されていることを経過時間からも確認する。
func TestOpenPoolLimitsConcurrentOpenConnections(t *testing.T) {
	dsn, _ := testDSN(t)
	const maxOpen = 2
	const workers = 6
	const sleepPerQuery = 300 * time.Millisecond

	pool, err := OpenPool(dsn, PoolConfig{
		MaxOpenConns:    maxOpen,
		MaxIdleConns:    maxOpen,
		ConnMaxIdleTime: time.Minute,
		ConnMaxLifetime: time.Minute,
	})
	if err != nil {
		t.Fatalf("OpenPool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	if err := pool.Ping(); err != nil {
		t.Fatalf("DB に届かない(スキップしない): %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stop := make(chan struct{})
	var mu sync.Mutex
	var maxObserved int
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			s := pool.Stats()
			mu.Lock()
			if s.OpenConnections > maxObserved {
				maxObserved = s.OpenConnections
			}
			mu.Unlock()
			time.Sleep(5 * time.Millisecond)
		}
	}()

	start := time.Now()
	var qwg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		qwg.Add(1)
		go func() {
			defer qwg.Done()
			var v int
			errCh <- pool.QueryRowContext(ctx, "SELECT SLEEP(?)", sleepPerQuery.Seconds()).Scan(&v)
		}()
	}
	qwg.Wait()
	close(stop)
	wg.Wait()
	elapsed := time.Since(start)
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Errorf("クエリが失敗した(pool の待機ではなくエラーになった): %v", err)
		}
	}

	mu.Lock()
	got := maxObserved
	mu.Unlock()
	if got > maxOpen {
		t.Errorf("Stats().OpenConnections の観測最大値 = %d, want %d 以下", got, maxOpen)
	}

	// MaxOpenConns=2 で6件を捌くなら直列化されて最低3周ぶんの待ちが要る。
	// (無制限に接続を開いて全部並行に終わらせた場合との違いを検知する)。
	wantMin := sleepPerQuery * time.Duration(workers/maxOpen)
	if elapsed < wantMin {
		t.Errorf("経過時間 %v が短すぎる(直列化されず無制限に接続を開いた疑い。want %v 以上)", elapsed, wantMin)
	}
}

// SetMaxIdleConns の効果を実 DB で確認する(database/sql の DBStats には設定値そのものを
// 表すフィールドが無いため、複数接続を使ってプールへ返却した後の Idle の実測で間接的に
// 確認する。pool_test.go の TestPoolConfigForExport が丸め計算そのものを固定している)。
func TestOpenPoolRespectsMaxIdleConns(t *testing.T) {
	dsn, _ := testDSN(t)
	const maxOpen = 4
	const maxIdle = 1

	pool, err := OpenPool(dsn, PoolConfig{
		MaxOpenConns:    maxOpen,
		MaxIdleConns:    maxIdle,
		ConnMaxIdleTime: time.Minute,
		ConnMaxLifetime: time.Minute,
	})
	if err != nil {
		t.Fatalf("OpenPool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	var wg sync.WaitGroup
	for i := 0; i < maxOpen; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var v int
			_ = pool.QueryRow("SELECT 1").Scan(&v)
		}()
	}
	wg.Wait() // 全接続がプールへ返却される

	s := waitUntilStats(t, pool, 2*time.Second, func(s sql.DBStats) bool { return s.InUse == 0 })
	if s.Idle > maxIdle {
		t.Errorf("Stats().Idle = %d, want %d 以下(MaxIdleConns が効いていない)", s.Idle, maxIdle)
	}
}

// AC4: PoolConfig.ForExport() で MaxOpenConns=1 にしたプールは、実 DB でも同時クエリを
// 直列化する(Stats().OpenConnections が1を超えない)。
func TestOpenPoolExportConfigSerializesQueries(t *testing.T) {
	dsn, _ := testDSN(t)
	cfg := PoolConfig{MaxOpenConns: 5, MaxIdleConns: 5, ConnMaxIdleTime: time.Minute, ConnMaxLifetime: time.Minute}.ForExport()
	if cfg.MaxOpenConns != 1 {
		t.Fatalf("前提: ForExport().MaxOpenConns = %d, want 1", cfg.MaxOpenConns)
	}

	pool, err := OpenPool(dsn, cfg)
	if err != nil {
		t.Fatalf("OpenPool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	if err := pool.Ping(); err != nil {
		t.Fatalf("DB に届かない(スキップしない): %v", err)
	}

	const workers = 3
	const sleepPerQuery = 300 * time.Millisecond
	start := time.Now()
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var v int
			errCh <- pool.QueryRow("SELECT SLEEP(?)", sleepPerQuery.Seconds()).Scan(&v)
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Errorf("クエリが失敗した: %v", err)
		}
	}
	elapsed := time.Since(start)
	want := sleepPerQuery * time.Duration(workers)
	if elapsed < want {
		t.Errorf("経過時間 %v が短すぎる(MaxOpenConns=1 で直列化されていない疑い。want %v 以上)", elapsed, want)
	}
	if got := pool.Stats().MaxOpenConnections; got != 1 {
		t.Errorf("Stats().MaxOpenConnections = %d, want 1", got)
	}
}
