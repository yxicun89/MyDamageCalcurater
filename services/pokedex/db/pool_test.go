package db

// pokedex-svc の接続プール設定(issue #112 / ADR-0112)の純粋なテスト。実 DB は使わない
// (make test で走る)。
//
// 以下は issue #112 の受け入れテスト。この時点では実装が無いため失敗する(コンパイルも
// 通らない)。実装者は ADR-0112 の「実装時の申し送り」に従い、少なくとも次の識別子を
// このパッケージ(新規 pool.go 等)に用意すること:
//
//	type PoolConfig struct {
//		MaxOpenConns    int
//		MaxIdleConns    int
//		ConnMaxIdleTime time.Duration
//		ConnMaxLifetime time.Duration
//	}
//	func OpenPool(dsn string, cfg PoolConfig) (*sql.DB, error)
//	func (cfg PoolConfig) ForExport() PoolConfig
//
// OpenPool は sql.Open → SetMaxOpenConns → SetMaxIdleConns → SetConnMaxIdleTime →
// SetConnMaxLifetime → 返す、だけを行う(値の検証はしない。検証は呼び出し側の loadConfig
// が済ませている前提)。

import (
	"testing"
	"time"
)

// fakePoolDSN は go-sql-driver/mysql の Open が受理する形式の DSN。go-sql-driver/mysql は
// driver.DriverContext を実装しており、sql.Open は OpenConnector 経由で ParseDSN するだけで
// ネットワークには触れない(接続は最初のクエリまで遅延する)。そのためこの DSN で
// Stats() の設定値だけを実 DB 無しに確認できる。
const fakePoolDSN = "user:pass@tcp(127.0.0.1:1)/pokedex"

// AC2: OpenPool は sql.Open 直後に4つの Set* を呼び、返す *sql.DB の
// Stats().MaxOpenConnections が設定値と一致する。
func TestOpenPoolAppliesMaxOpenConns(t *testing.T) {
	cfg := PoolConfig{
		MaxOpenConns:    7,
		MaxIdleConns:    3,
		ConnMaxIdleTime: 2 * time.Minute,
		ConnMaxLifetime: 9 * time.Minute,
	}
	pool, err := OpenPool(fakePoolDSN, cfg)
	if err != nil {
		t.Fatalf("OpenPool: %v", err)
	}
	defer pool.Close()
	if got := pool.Stats().MaxOpenConnections; got != cfg.MaxOpenConns {
		t.Errorf("Stats().MaxOpenConnections = %d, want %d", got, cfg.MaxOpenConns)
	}
}

// OpenPool は DSN が壊れていれば sql.Open の時点でエラーを返す(ネットワークに触れない)。
func TestOpenPoolRejectsMalformedDSN(t *testing.T) {
	cfg := PoolConfig{MaxOpenConns: 1, MaxIdleConns: 1, ConnMaxIdleTime: time.Minute, ConnMaxLifetime: time.Minute}
	if _, err := OpenPool("not a dsn", cfg); err == nil {
		t.Fatal("OpenPool(壊れた DSN) = nil error, want エラー")
	}
}

// AC4: export は MaxOpenConns=1 に上書きし、MaxIdleConns はそれを超えないよう丸める。
// database/sql の DBStats には設定した MaxIdleConns の値そのものを表すフィールドが無いため
// (Stats().Idle は「現在アイドル中の実接続数」であって設定値ではない)、この丸め変換自体を
// 実 DB 無しのユニットテストで固定する。実 DB での Idle の実測は pool_mysql_test.go で行う。
func TestPoolConfigForExport(t *testing.T) {
	tests := []struct {
		name        string
		cfg         PoolConfig
		wantMaxOpen int
		wantMaxIdle int
	}{
		{
			name:        "既定値(open10/idle5)",
			cfg:         PoolConfig{MaxOpenConns: 10, MaxIdleConns: 5, ConnMaxIdleTime: 5 * time.Minute, ConnMaxLifetime: 30 * time.Minute},
			wantMaxOpen: 1,
			wantMaxIdle: 1,
		},
		{
			name:        "MaxIdleConns が既に1",
			cfg:         PoolConfig{MaxOpenConns: 3, MaxIdleConns: 1, ConnMaxIdleTime: time.Minute, ConnMaxLifetime: time.Minute},
			wantMaxOpen: 1,
			wantMaxIdle: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.ForExport()
			if got.MaxOpenConns != tt.wantMaxOpen {
				t.Errorf("MaxOpenConns = %d, want %d", got.MaxOpenConns, tt.wantMaxOpen)
			}
			if got.MaxIdleConns != tt.wantMaxIdle {
				t.Errorf("MaxIdleConns = %d, want %d", got.MaxIdleConns, tt.wantMaxIdle)
			}
			if got.ConnMaxIdleTime != tt.cfg.ConnMaxIdleTime {
				t.Errorf("ConnMaxIdleTime = %v, want %v(そのまま引き継ぐ)", got.ConnMaxIdleTime, tt.cfg.ConnMaxIdleTime)
			}
			if got.ConnMaxLifetime != tt.cfg.ConnMaxLifetime {
				t.Errorf("ConnMaxLifetime = %v, want %v(そのまま引き継ぐ)", got.ConnMaxLifetime, tt.cfg.ConnMaxLifetime)
			}
		})
	}
}
