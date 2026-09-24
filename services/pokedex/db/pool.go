// pokedex-svc の DB 接続プール生成を1か所に集約する(issue #112・ADR-0112)。
//
// OpenPool は sql.Open と4つの Set* 呼び出しをまとめるだけで、値の検証はしない
// (検証は呼び出し側の cmd/pokedex.loadConfig が済ませている前提)。P7-1 でメトリクスを
// 公開するときも、この関数が返す *sql.DB の Stats() を使えばよいようにするための集約。
package db

import (
	"database/sql"
	"time"
)

// PoolConfig は sql.DB の接続プール設定(issue #112 の4環境変数に対応)。
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxIdleTime time.Duration
	ConnMaxLifetime time.Duration
}

// OpenPool は sql.Open → SetMaxOpenConns → SetMaxIdleConns → SetConnMaxIdleTime →
// SetConnMaxLifetime → 返す、だけを行う(値の検証はしない)。
func OpenPool(dsn string, cfg PoolConfig) (*sql.DB, error) {
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(cfg.MaxOpenConns)
	conn.SetMaxIdleConns(cfg.MaxIdleConns)
	conn.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	conn.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	return conn, nil
}

// ForExport は export 用に MaxOpenConns=1 へ上書きし、MaxIdleConns をそれ以下に丸めた
// PoolConfig を返す(純粋関数。ADR-0112 決定4)。
func (cfg PoolConfig) ForExport() PoolConfig {
	cfg.MaxOpenConns = 1
	if cfg.MaxIdleConns > cfg.MaxOpenConns {
		cfg.MaxIdleConns = cfg.MaxOpenConns
	}
	return cfg
}
