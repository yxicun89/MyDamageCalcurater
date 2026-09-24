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
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
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
