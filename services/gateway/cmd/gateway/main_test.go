package main

// gateway の起動(設定の読み込み・待ち受け・停止)の受け入れテスト(ADR-0202 §2。AC-G9)。

import (
	"context"
	"errors"
	"net"
	"net/http"
	"slices"
	"testing"
	"time"
)

const testCalcURL = "http://calc.example.test:8080"

func lookupFrom(env map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}
}

func envWith(mutate func(map[string]string)) map[string]string {
	e := map[string]string{envCalcURL: testCalcURL}
	if mutate != nil {
		mutate(e)
	}
	return e
}

// 環境変数の名前は運用(k8s の manifest・README)が依存するので固定する。
func TestEnvNames(t *testing.T) {
	got := []string{envAddr, envCalcURL, envPokedexURL, envAssetsURL, envCORSAllowedOrigins, envUpstreamTimeout}
	want := []string{"GATEWAY_ADDR", "GATEWAY_CALC_URL", "GATEWAY_POKEDEX_URL", "GATEWAY_ASSETS_URL",
		"GATEWAY_CORS_ALLOWED_ORIGINS", "GATEWAY_UPSTREAM_TIMEOUT"}
	if !slices.Equal(got, want) {
		t.Fatalf("環境変数名 = %q, want %q", got, want)
	}
}

// wantConfig は config の比較用の平たい形(*url.URL は String で比べる。nil は "")。
type wantConfig struct {
	addr, calc, pokedex, assets string
	origins                     []string
	timeout                     time.Duration
}

func flatten(c config) wantConfig {
	str := func(u interface{ String() string }, isNil bool) string {
		if isNil {
			return ""
		}
		return u.String()
	}
	return wantConfig{
		addr:    c.Addr,
		calc:    str(c.Gateway.CalcURL, c.Gateway.CalcURL == nil),
		pokedex: str(c.Gateway.PokedexURL, c.Gateway.PokedexURL == nil),
		assets:  str(c.Gateway.AssetsURL, c.Gateway.AssetsURL == nil),
		origins: c.Gateway.CORSAllowedOrigins,
		timeout: c.Gateway.UpstreamTimeout,
	}
}

func equalConfig(a, b wantConfig) bool {
	return a.addr == b.addr && a.calc == b.calc && a.pokedex == b.pokedex && a.assets == b.assets &&
		slices.Equal(a.origins, b.origins) && a.timeout == b.timeout
}

// AC-G9: 必須・既定・任意の値の読み込み。
func TestLoadConfig(t *testing.T) {
	defaults := wantConfig{addr: ":8080", calc: testCalcURL, timeout: 10 * time.Second}
	tests := []struct {
		name string
		env  map[string]string
		want wantConfig
	}{
		{"最小(CALC_URL だけ)は既定値", envWith(nil), defaults},
		{"空の任意値は未設定と同じ", envWith(func(e map[string]string) {
			e[envAddr], e[envPokedexURL], e[envAssetsURL], e[envCORSAllowedOrigins], e[envUpstreamTimeout] = "", "", "", "", ""
		}), defaults},
		{"すべて指定", envWith(func(e map[string]string) {
			e[envAddr] = "127.0.0.1:9090"
			e[envPokedexURL] = "http://pokedex.example.test:8080"
			e[envAssetsURL] = "http://minio.example.test:9000"
			e[envCORSAllowedOrigins] = "https://app.example.test, http://localhost:5173"
			e[envUpstreamTimeout] = "250ms"
		}), wantConfig{
			addr: "127.0.0.1:9090", calc: testCalcURL,
			pokedex: "http://pokedex.example.test:8080", assets: "http://minio.example.test:9000",
			origins: []string{"https://app.example.test", "http://localhost:5173"}, timeout: 250 * time.Millisecond,
		}},
		{"https の上流", envWith(func(e map[string]string) { e[envCalcURL] = "https://calc.example.test" }),
			wantConfig{addr: ":8080", calc: "https://calc.example.test", timeout: 10 * time.Second}},
		{"許可オリジン1つ", envWith(func(e map[string]string) { e[envCORSAllowedOrigins] = "https://app.example.test" }),
			wantConfig{addr: ":8080", calc: testCalcURL, origins: []string{"https://app.example.test"}, timeout: 10 * time.Second}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadConfig(lookupFrom(tt.env))
			if err != nil {
				t.Fatalf("loadConfig = %v, want nil", err)
			}
			g := flatten(got)
			if len(g.origins) == 0 {
				g.origins = nil
			}
			if !equalConfig(g, tt.want) {
				t.Errorf("loadConfig = %+v, want %+v", g, tt.want)
			}
		})
	}
}

// AC-G9: 必須値の欠落・不正な URL・CORS の "*"・オリジンでない値・不正なタイムアウトは起動エラー(errInvalidConfig)。
func TestLoadConfigRejects(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{"CALC_URL が無い", map[string]string{}},
		{"CALC_URL が空", envWith(func(e map[string]string) { e[envCalcURL] = "" })},
		{"CALC_URL が URL でない", envWith(func(e map[string]string) { e[envCalcURL] = "not a url" })},
		{"CALC_URL にスキームが無い", envWith(func(e map[string]string) { e[envCalcURL] = "calc:8080" })},
		{"CALC_URL が http(s) でない", envWith(func(e map[string]string) { e[envCalcURL] = "ftp://calc.example.test" })},
		{"CALC_URL にホストが無い", envWith(func(e map[string]string) { e[envCalcURL] = "http://" })},
		{"CALC_URL が解析できない", envWith(func(e map[string]string) { e[envCalcURL] = "http://calc.example.test:port" })},
		{"POKEDEX_URL が不正", envWith(func(e map[string]string) { e[envPokedexURL] = "::" })},
		{"ASSETS_URL が http(s) でない", envWith(func(e map[string]string) { e[envAssetsURL] = "s3://bucket" })},
		{"CORS が *", envWith(func(e map[string]string) { e[envCORSAllowedOrigins] = "*" })},
		{"CORS に * が混ざる", envWith(func(e map[string]string) { e[envCORSAllowedOrigins] = "https://app.example.test,*" })},
		{"CORS のオリジンにパスがある", envWith(func(e map[string]string) { e[envCORSAllowedOrigins] = "https://app.example.test/app" })},
		{"CORS のオリジンが末尾スラッシュ", envWith(func(e map[string]string) { e[envCORSAllowedOrigins] = "https://app.example.test/" })},
		{"CORS のオリジンにスキームが無い", envWith(func(e map[string]string) { e[envCORSAllowedOrigins] = "app.example.test" })},
		{"TIMEOUT が duration でない", envWith(func(e map[string]string) { e[envUpstreamTimeout] = "10" })},
		{"TIMEOUT が文字列", envWith(func(e map[string]string) { e[envUpstreamTimeout] = "abc" })},
		{"TIMEOUT が 0", envWith(func(e map[string]string) { e[envUpstreamTimeout] = "0s" })},
		{"TIMEOUT が負", envWith(func(e map[string]string) { e[envUpstreamTimeout] = "-1s" })},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadConfig(lookupFrom(tt.env))
			if err == nil {
				t.Fatalf("loadConfig = %+v, nil; want errInvalidConfig", flatten(got))
			}
			if !errors.Is(err, errInvalidConfig) {
				t.Errorf("err = %v, want errInvalidConfig を包む", err)
			}
		})
	}
}

// freeAddr は今空いている 127.0.0.1 のポートを返す(閉じてから run に渡すので、わずかな競合はありうる)。
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

// AC-G9: run は待ち受けて /healthz に答え、ctx の終了で nil を返して止まる。
func TestRunServesAndStopsOnContextCancel(t *testing.T) {
	addr := freeAddr(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, lookupFrom(envWith(func(e map[string]string) { e[envAddr] = addr })))
	}()

	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(5 * time.Second)
	for {
		select {
		case err := <-done:
			t.Fatalf("run が待ち受け前に終わった: %v", err)
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
			t.Fatalf("run が待ち受けを始めない: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run = %v, want nil(ctx の終了で正常に止まる)", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("ctx を終えても run が止まらない")
	}
}

// AC-G9: 設定が不正なら run は待ち受けずにエラーを返す(非ゼロ終了の元)。
func TestRunFailsOnInvalidConfig(t *testing.T) {
	err := run(context.Background(), lookupFrom(map[string]string{}))
	if !errors.Is(err, errInvalidConfig) {
		t.Fatalf("run = %v, want errInvalidConfig を包む", err)
	}
}
