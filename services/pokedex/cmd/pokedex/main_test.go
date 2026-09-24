package main

// pokedex-svc のコマンド(`pokedex serve` / `pokedex export`)の設定のテスト(ADR-0105 §1・§5)。
//
// 以下は issue #109(ADR-0111: HTTP タイムアウトと graceful shutdown)の受け入れテスト。
// この時点では実装が無いため失敗する(コンパイルも通らない)。実装者は ADR-0111 の
// 「実装時の申し送り」に従い、少なくとも次の識別子を main.go に用意すること:
//
//	readHeaderTimeout, readTimeout, writeTimeout, idleTimeout, maxHeaderBytes time.Duration/int 定数
//	shutdownTimeout time.Duration 定数(10秒)
//	newHTTPServer(addr string, handler http.Handler) *http.Server
//	serve(ctx context.Context, addr string, handler http.Handler) error
//	runServe(ctx context.Context, lookup func(string) (string, bool)) error
//
// 既存の `run(args []string) int`(serve/export の振り分け)とは別物なので名前を衝突させない
// (ADR-0111 の原文にある `run(ctx, lookup) error` は、この衝突を避けて `runServe` と改名する)。

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"example.com/pokecalc/services/pokedex/db"
)

func lookupFrom(env map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := env[k]
		return v, ok
	}
}

// AC-K0: 環境変数の名前(k8s のマニフェスト・Makefile と共有する)。
func TestEnvNames(t *testing.T) {
	if envAddr != "POKEDEX_ADDR" || envDatabaseDSN != "POKEDEX_DATABASE_DSN" || defaultAddr != ":8080" {
		t.Errorf("envAddr=%q envDatabaseDSN=%q defaultAddr=%q", envAddr, envDatabaseDSN, defaultAddr)
	}
}

// AC-K0: DSN は必須(空は未設定と同じ)。待ち受けアドレスは未設定・空なら既定値。
// DSN は go-sql-driver/mysql の形式で、parseTime を必ず有効にする(regulations の DATE 列を sql.NullTime で読むため)。
func TestLoadConfig(t *testing.T) {
	const dsn = "user:pass@tcp(mysql:3306)/pokedex"
	tests := []struct {
		name     string
		env      map[string]string
		wantErr  string
		wantAddr string
	}{
		{"DSN が無い", map[string]string{}, envDatabaseDSN, ""},
		{"DSN が空", map[string]string{envDatabaseDSN: ""}, envDatabaseDSN, ""},
		{"DSN が壊れている", map[string]string{envDatabaseDSN: "not a dsn"}, envDatabaseDSN, ""},
		{"既定のアドレス", map[string]string{envDatabaseDSN: dsn}, "", defaultAddr},
		{"アドレスが空なら既定", map[string]string{envDatabaseDSN: dsn, envAddr: ""}, "", defaultAddr},
		{"アドレスの指定", map[string]string{envDatabaseDSN: dsn, envAddr: ":9090"}, "", ":9090"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := loadConfig(lookupFrom(tt.env))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %s を含むエラー", err, tt.wantErr)
				}
				if strings.Contains(err.Error(), "pass@") {
					t.Errorf("エラーに DSN(パスワード)を含めている: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if cfg.Addr != tt.wantAddr {
				t.Errorf("Addr = %q, want %q", cfg.Addr, tt.wantAddr)
			}
			if !strings.Contains(cfg.DSN, "parseTime=true") {
				t.Errorf("DSN に parseTime=true が無い: %q", cfg.DSN)
			}
		})
	}
}

// --- issue #109 / ADR-0111: HTTP タイムアウトと graceful shutdown -----------------------------

// fakeDSN は sql.Open が受理する形の DSN(mysql.ParseDSN を通る)。sql.Open は接続を遅延するので、
// 実 MySQL が無くても loadConfig・runServe の起動(/healthz まで)はこれで足りる
// (/healthz は DB に触れない。services/pokedex/internal/httpapi/server.go)。
const fakeDSN = "user:pass@tcp(127.0.0.1:1)/pokedex"

// freeAddr は今空いている 127.0.0.1 のポートを返す(閉じてから使うので、わずかな競合はありうる。
// services/gateway/cmd/gateway/main_test.go の freeAddr と同じ手法)。
func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("空きポートを得られない: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

// waitForListen は addr が待ち受けを始めるまで(最大5秒)待つ。
func waitForListen(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("待ち受けが5秒以内に始まらない: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// 受け入れ条件1: http.Server が ADR-0111 決定1の値(balance/judge と同じ)で構成されている。
// 名前(定数)ではなく実際の値を固定する(定数名だけ変わっても検知できるように)。
func TestHTTPServerTimeouts(t *testing.T) {
	srv := newHTTPServer("127.0.0.1:0", http.NotFoundHandler())
	tests := []struct {
		name string
		got  time.Duration
		want time.Duration
	}{
		{"ReadHeaderTimeout", srv.ReadHeaderTimeout, 5 * time.Second},
		{"ReadTimeout", srv.ReadTimeout, 10 * time.Second},
		{"WriteTimeout", srv.WriteTimeout, 15 * time.Second},
		{"IdleTimeout", srv.IdleTimeout, 60 * time.Second},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
		}
	}
	if srv.MaxHeaderBytes != 16*1024 {
		t.Errorf("MaxHeaderBytes = %d, want %d(16KiB)", srv.MaxHeaderBytes, 16*1024)
	}
}

// 受け入れ条件2: shutdown timeout は10秒(ADR-0111 決定1の表。calc-svc/gateway の5秒とは異なる値)。
func TestShutdownTimeoutValue(t *testing.T) {
	if shutdownTimeout != 10*time.Second {
		t.Errorf("shutdownTimeout = %v, want 10s", shutdownTimeout)
	}
}

// 受け入れ条件3: serve は ctx の終了(SIGINT/SIGTERM 相当)で Shutdown を呼び、エラー無く戻る。
func TestServeStopsOnContextCancel(t *testing.T) {
	addr := freeAddr(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, addr, http.NotFoundHandler()) }()
	waitForListen(t, addr)

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve = %v, want nil(ctx の終了による正常な停止)", err)
		}
	case <-time.After(shutdownTimeout + 5*time.Second):
		t.Fatal("ctx を終えても serve が止まらない")
	}
}

// 受け入れ条件4: ヘッダを送り切らない接続は ReadHeaderTimeout で切断される(httptest では検証できないため
// 生の TCP 接続を開き、リクエスト行だけ送って空行(ヘッダ終端)を送らずに待つ)。
func TestServeClosesConnectionMissingHeaders(t *testing.T) {
	addr := freeAddr(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serve(ctx, addr, http.NotFoundHandler()) }()
	waitForListen(t, addr)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("接続できない: %v", err)
	}
	defer conn.Close()
	// リクエスト行だけ送り、ヘッダの終端(空行)を送らない = ヘッダが完了しない接続。
	if _, err := conn.Write([]byte("GET / HTTP/1.1\r\nHost: example.test\r\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(readHeaderTimeout + 5*time.Second))
	start := time.Now()
	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("readHeaderTimeout を過ぎても接続が切れない(応答が読めてしまった)")
	}
	elapsed := time.Since(start)
	if elapsed < readHeaderTimeout {
		t.Errorf("readHeaderTimeout(%v) より早く切れた: %v", readHeaderTimeout, elapsed)
	}
	if elapsed > readHeaderTimeout+3*time.Second {
		t.Errorf("readHeaderTimeout(%v) の直後に切れていない(切断まで %v かかった)", readHeaderTimeout, elapsed)
	}
}

// 受け入れ条件5: shutdown 開始(ctx キャンセル)後も、既に受理した短い in-flight リクエストは
// 完了してから serve が止まる(意図的に遅いハンドラで検証する)。
func TestServeWaitsForInFlightRequestOnShutdown(t *testing.T) {
	addr := freeAddr(t)
	const handlerDelay = 300 * time.Millisecond
	started := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		time.Sleep(handlerDelay)
		w.WriteHeader(http.StatusOK)
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, addr, handler) }()
	waitForListen(t, addr)

	reqDone := make(chan error, 1)
	go func() {
		resp, err := http.Get("http://" + addr + "/")
		if err != nil {
			reqDone <- err
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			reqDone <- fmt.Errorf("status = %d, want 200", resp.StatusCode)
			return
		}
		reqDone <- nil
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("リクエストが handler に届かない")
	}
	// handler がまだ sleep している間に shutdown を要求する。
	cancel()

	select {
	case err := <-reqDone:
		if err != nil {
			t.Fatalf("shutdown 中に開始した in-flight リクエストが失敗した(打ち切られた可能性): %v", err)
		}
	case <-time.After(handlerDelay + 5*time.Second):
		t.Fatal("in-flight リクエストが完了しない")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve = %v, want nil", err)
		}
	case <-time.After(shutdownTimeout + 5*time.Second):
		t.Fatal("in-flight リクエストの完了後も serve が止まらない")
	}
}

// 受け入れ条件6: http.ErrServerClosed 以外の起動エラー(アドレス使用中)は非nilで返り、
// ErrServerClosed としては扱われない。
func TestServeReturnsErrorWhenAddrInUse(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("空きポートを得られない: %v", err)
	}
	defer l.Close()
	addr := l.Addr().String()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = serve(ctx, addr, http.NotFoundHandler())
	if err == nil {
		t.Fatal("serve = nil, want アドレス使用中のエラー")
	}
	if errors.Is(err, http.ErrServerClosed) {
		t.Errorf("ErrServerClosed を返した(起動失敗を正常終了として扱っている): %v", err)
	}
}

// 受け入れ条件3(全体結線): runServe は loadConfig → ハンドラ構築 → serve を結び、
// ctx の終了で待ち受けを止めて nil を返す(/healthz が DB 無しで応答することを使う。
// services/gateway/cmd/gateway/main_test.go の TestRunServesAndStopsOnContextCancel と同じ手法)。
func TestRunServeStopsOnContextCancel(t *testing.T) {
	addr := freeAddr(t)
	env := map[string]string{envDatabaseDSN: fakeDSN, envAddr: addr}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- runServe(ctx, lookupFrom(env)) }()

	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(5 * time.Second)
	for {
		select {
		case err := <-done:
			t.Fatalf("runServe が待ち受け前に終わった: %v", err)
		default:
		}
		resp, err := client.Get("http://" + addr + "/healthz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("/healthz status = %d, want 200", resp.StatusCode)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("runServe が待ち受けを始めない: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runServe = %v, want nil(ctx の終了による正常な停止)", err)
		}
	case <-time.After(shutdownTimeout + 5*time.Second):
		t.Fatal("ctx を終えても runServe が止まらない")
	}
}

// 受け入れ条件6(全体結線): 設定が不正(DSN 未設定)なら runServe は待ち受けずにエラーを返す。
// エラー文に DSN(パスワード)を含めない。
func TestRunServeFailsOnInvalidConfig(t *testing.T) {
	err := runServe(context.Background(), lookupFrom(map[string]string{}))
	if err == nil {
		t.Fatal("runServe = nil, want エラー(DSN 未設定)")
	}
	if strings.Contains(err.Error(), "pass@") {
		t.Errorf("エラーに DSN(パスワード)を含めている: %v", err)
	}
}

// 受け入れ条件6(全体結線): アドレス使用中なら runServe は非0終了の元になるエラーを返す。
// ErrServerClosed としては扱わず、エラー文に DSN(パスワード)を含めない。
func TestRunServeReturnsErrorWhenAddrInUse(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("空きポートを得られない: %v", err)
	}
	defer l.Close()
	addr := l.Addr().String()

	env := map[string]string{envDatabaseDSN: fakeDSN, envAddr: addr}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = runServe(ctx, lookupFrom(env))
	if err == nil {
		t.Fatal("runServe = nil, want アドレス使用中のエラー")
	}
	if errors.Is(err, http.ErrServerClosed) {
		t.Errorf("ErrServerClosed を返した(起動失敗を正常終了として扱っている): %v", err)
	}
	if strings.Contains(err.Error(), "pass@") {
		t.Errorf("エラーに DSN(パスワード)を含めている: %v", err)
	}
}

// --- issue #112 / ADR-0112: pokedex の DB 接続プールに上限と寿命を設定する ---------------------
//
// 以下は issue #112(ADR-0112)の受け入れテスト。この時点では実装が無いため失敗する
// (コンパイルも通らない)。実装者は ADR-0112 の「実装時の申し送り」に従い、少なくとも
// 次の識別子を main.go に用意すること:
//
//	envDBMaxOpenConns, envDBMaxIdleConns, envDBConnMaxIdleTime, envDBConnMaxLifetime string 定数
//	defaultDBMaxOpenConns, defaultDBMaxIdleConns int 定数(10 / 5)
//	defaultDBConnMaxIdleTime, defaultDBConnMaxLifetime time.Duration 定数(5分 / 30分)
//	config 構造体に Pool db.PoolConfig フィールド
//	loadConfig がこの4変数を読み・検証し・config.Pool を埋める(エラー文に DSN を含めない)
//	var openPool = db.OpenPool(runServe・runExport はこれ経由で db.OpenPool を呼ぶ)
//	runExport は openPool(cfg.DSN, cfg.Pool.ForExport()) を呼ぶ
//
// db.PoolConfig・db.OpenPool・(db.PoolConfig).ForExport() は services/pokedex/db/pool_test.go・
// pool_mysql_test.go が要求する(そちらは package db 側の実装)。

const dsnWithCredentials = "user:pass@tcp(mysql:3306)/pokedex"

// AC1: 4つの環境変数の名前と既定値(main.go の定数)。
func TestPoolEnvNames(t *testing.T) {
	if envDBMaxOpenConns != "POKEDEX_DB_MAX_OPEN_CONNS" {
		t.Errorf("envDBMaxOpenConns = %q", envDBMaxOpenConns)
	}
	if envDBMaxIdleConns != "POKEDEX_DB_MAX_IDLE_CONNS" {
		t.Errorf("envDBMaxIdleConns = %q", envDBMaxIdleConns)
	}
	if envDBConnMaxIdleTime != "POKEDEX_DB_CONN_MAX_IDLE_TIME" {
		t.Errorf("envDBConnMaxIdleTime = %q", envDBConnMaxIdleTime)
	}
	if envDBConnMaxLifetime != "POKEDEX_DB_CONN_MAX_LIFETIME" {
		t.Errorf("envDBConnMaxLifetime = %q", envDBConnMaxLifetime)
	}
	if defaultDBMaxOpenConns != 10 {
		t.Errorf("defaultDBMaxOpenConns = %d, want 10", defaultDBMaxOpenConns)
	}
	if defaultDBMaxIdleConns != 5 {
		t.Errorf("defaultDBMaxIdleConns = %d, want 5", defaultDBMaxIdleConns)
	}
	if defaultDBConnMaxIdleTime != 5*time.Minute {
		t.Errorf("defaultDBConnMaxIdleTime = %v, want 5m", defaultDBConnMaxIdleTime)
	}
	if defaultDBConnMaxLifetime != 30*time.Minute {
		t.Errorf("defaultDBConnMaxLifetime = %v, want 30m", defaultDBConnMaxLifetime)
	}
}

// AC1: 4変数が未設定なら既定値(10/5/5m/30m)になる。
func TestLoadConfigPoolDefaults(t *testing.T) {
	cfg, err := loadConfig(lookupFrom(map[string]string{envDatabaseDSN: fakeDSN}))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	want := db.PoolConfig{
		MaxOpenConns:    defaultDBMaxOpenConns,
		MaxIdleConns:    defaultDBMaxIdleConns,
		ConnMaxIdleTime: defaultDBConnMaxIdleTime,
		ConnMaxLifetime: defaultDBConnMaxLifetime,
	}
	if cfg.Pool != want {
		t.Errorf("cfg.Pool = %+v, want %+v", cfg.Pool, want)
	}
}

// AC1: 4変数を指定すれば、その値がそのまま反映される。
func TestLoadConfigPoolOverrides(t *testing.T) {
	env := map[string]string{
		envDatabaseDSN:       fakeDSN,
		envDBMaxOpenConns:    "20",
		envDBMaxIdleConns:    "8",
		envDBConnMaxIdleTime: "90s",
		envDBConnMaxLifetime: "2h",
	}
	cfg, err := loadConfig(lookupFrom(env))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	want := db.PoolConfig{MaxOpenConns: 20, MaxIdleConns: 8, ConnMaxIdleTime: 90 * time.Second, ConnMaxLifetime: 2 * time.Hour}
	if cfg.Pool != want {
		t.Errorf("cfg.Pool = %+v, want %+v", cfg.Pool, want)
	}
}

// AC1: 不正な値(0以下の整数・idle>open・0以下の duration・壊れた duration 文字列)は
// sql.Open より前にエラーになり、エラー文に DSN(パスワード)を含めない。
func TestLoadConfigPoolRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name    string
		extra   map[string]string
		wantErr string
	}{
		{"MaxOpenConns が0", map[string]string{envDBMaxOpenConns: "0"}, envDBMaxOpenConns},
		{"MaxOpenConns が負", map[string]string{envDBMaxOpenConns: "-1"}, envDBMaxOpenConns},
		{"MaxOpenConns が整数でない", map[string]string{envDBMaxOpenConns: "abc"}, envDBMaxOpenConns},
		{"MaxIdleConns が0", map[string]string{envDBMaxIdleConns: "0"}, envDBMaxIdleConns},
		{"MaxIdleConns が整数でない", map[string]string{envDBMaxIdleConns: "abc"}, envDBMaxIdleConns},
		{"MaxIdleConns が MaxOpenConns を超える", map[string]string{envDBMaxOpenConns: "5", envDBMaxIdleConns: "6"}, envDBMaxIdleConns},
		{"ConnMaxIdleTime が0", map[string]string{envDBConnMaxIdleTime: "0s"}, envDBConnMaxIdleTime},
		{"ConnMaxIdleTime が負", map[string]string{envDBConnMaxIdleTime: "-1m"}, envDBConnMaxIdleTime},
		{"ConnMaxIdleTime の形式が壊れている", map[string]string{envDBConnMaxIdleTime: "five minutes"}, envDBConnMaxIdleTime},
		{"ConnMaxLifetime が0", map[string]string{envDBConnMaxLifetime: "0"}, envDBConnMaxLifetime},
		{"ConnMaxLifetime が負", map[string]string{envDBConnMaxLifetime: "-30m"}, envDBConnMaxLifetime},
		{"ConnMaxLifetime の形式が壊れている", map[string]string{envDBConnMaxLifetime: "not-a-duration"}, envDBConnMaxLifetime},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := map[string]string{envDatabaseDSN: dsnWithCredentials}
			for k, v := range tt.extra {
				env[k] = v
			}
			_, err := loadConfig(lookupFrom(env))
			if err == nil {
				t.Fatalf("loadConfig = nil error, want %s を含むエラー", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want %s を含む", err, tt.wantErr)
			}
			if strings.Contains(err.Error(), "pass@") {
				t.Errorf("エラーに DSN(パスワード)を含めている: %v", err)
			}
		})
	}
}

// AC1: MaxIdleConns が MaxOpenConns と等しい(境界値)ときはエラーにならない
// (loadPoolConfig の検査は「MaxIdleConns > MaxOpenConns」の厳密不等号で、等しい場合を拒否しない)。
func TestLoadConfigPoolAllowsIdleEqualToOpen(t *testing.T) {
	env := map[string]string{
		envDatabaseDSN:    dsnWithCredentials,
		envDBMaxOpenConns: "5",
		envDBMaxIdleConns: "5",
	}
	cfg, err := loadConfig(lookupFrom(env))
	if err != nil {
		t.Fatalf("loadConfig = %v, want nil(MaxIdleConns == MaxOpenConns は許可される)", err)
	}
	if cfg.Pool.MaxOpenConns != 5 || cfg.Pool.MaxIdleConns != 5 {
		t.Errorf("Pool = %+v, want MaxOpenConns=5 MaxIdleConns=5", cfg.Pool)
	}
}

// AC2・AC3(全体結線): runServe は loadConfig が組み立てた cfg.Pool をそのまま openPool に渡す。
func TestRunServeUsesConfiguredPool(t *testing.T) {
	addr := freeAddr(t)
	env := map[string]string{
		envDatabaseDSN:    fakeDSN,
		envAddr:           addr,
		envDBMaxOpenConns: "3",
		envDBMaxIdleConns: "2",
	}
	var got db.PoolConfig
	orig := openPool
	openPool = func(dsn string, cfg db.PoolConfig) (*sql.DB, error) {
		got = cfg
		return orig(dsn, cfg)
	}
	defer func() { openPool = orig }()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runServe(ctx, lookupFrom(env)) }()
	waitForListen(t, addr)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runServe = %v, want nil", err)
		}
	case <-time.After(shutdownTimeout + 5*time.Second):
		t.Fatal("ctx を終えても runServe が止まらない")
	}

	want := db.PoolConfig{MaxOpenConns: 3, MaxIdleConns: 2, ConnMaxIdleTime: defaultDBConnMaxIdleTime, ConnMaxLifetime: defaultDBConnMaxLifetime}
	if got != want {
		t.Errorf("openPool に渡された cfg = %+v, want %+v", got, want)
	}
}

// AC4: runExport は openPool を cfg.Pool.ForExport()(MaxOpenConns=1、MaxIdleConns はそれ以下)
// で呼ぶ。実 DB が無くても openPool に渡された PoolConfig だけを検証できる
// (openPool を横取りし、DB 到達の成否は問わない)。
func TestRunExportUsesExportPoolConfig(t *testing.T) {
	t.Setenv(envDatabaseDSN, fakeDSN)
	t.Setenv(envDBMaxOpenConns, "9")
	t.Setenv(envDBMaxIdleConns, "4")

	var got db.PoolConfig
	var calls int
	orig := openPool
	openPool = func(dsn string, cfg db.PoolConfig) (*sql.DB, error) {
		calls++
		got = cfg
		return orig(dsn, cfg)
	}
	defer func() { openPool = orig }()

	dir := t.TempDir()
	_ = runExport([]string{"-out", dir}) // 実 DB が無いので失敗してよい。cfg の中身だけ見る

	if calls == 0 {
		t.Fatal("openPool が呼ばれていない")
	}
	if got.MaxOpenConns != 1 {
		t.Errorf("export の MaxOpenConns = %d, want 1", got.MaxOpenConns)
	}
	if got.MaxIdleConns > 1 {
		t.Errorf("export の MaxIdleConns = %d, want 1以下", got.MaxIdleConns)
	}
}
