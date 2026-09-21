package httpapi

// 上流の失敗・未設定と panic 回復の受け入れテスト(ADR-0020 §5・§7。AC-G5・AC-G7)。

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// closedServerURL は「さっきまで待ち受けていたが今は閉じた」アドレス(接続拒否になる)。
func closedServerURL(t *testing.T) *url.URL {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	u := mustParseURL(t, srv.URL)
	srv.Close()
	return u
}

// slowServerURL は応答ヘッダを delay の間(またはリクエストが打ち切られるまで)返さない上流。
func slowServerURL(t *testing.T, delay time.Duration) *url.URL {
	t.Helper()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(delay):
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
		case <-release:
		}
	}))
	// Cleanup は後に登録したものから走る: 先に release で待ちを解き、その後 Close する
	// (Close が遅いハンドラの終了を delay いっぱい待たないように)。
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })
	return mustParseURL(t, srv.URL)
}

// AC-G5: 上流に接続できない・タイムアウト・未設定は gateway のエラー(Go の内部情報を message に出さない)。
// pokedex 未設定は 503 upstream_unavailable、assets 未設定は 404 not_found。
func TestUpstreamFailures(t *testing.T) {
	const shortTimeout = 100 * time.Millisecond
	tests := []struct {
		name       string
		mutate     func(*Config)
		method     string
		path       string
		header     http.Header
		wantStatus int
		wantCode   string
	}{
		{"calc に接続できない", func(c *Config) { c.CalcURL = closedServerURL(t) },
			http.MethodPost, "/api/calc", validHeaders(), http.StatusServiceUnavailable, "upstream_unavailable"},
		{"pokedex に接続できない", func(c *Config) { c.PokedexURL = closedServerURL(t) },
			http.MethodGet, "/api/pokedex/natures", validHeaders(), http.StatusServiceUnavailable, "upstream_unavailable"},
		{"assets に接続できない", func(c *Config) { c.AssetsURL = closedServerURL(t) },
			http.MethodGet, "/assets/0445-000.webp", http.Header{}, http.StatusServiceUnavailable, "upstream_unavailable"},
		{"calc がタイムアウト", func(c *Config) {
			c.CalcURL = slowServerURL(t, 5*time.Second)
			c.UpstreamTimeout = shortTimeout
		}, http.MethodPost, "/api/calc/bulk", validHeaders(), http.StatusServiceUnavailable, "upstream_unavailable"},
		{"pokedex がタイムアウト", func(c *Config) {
			c.PokedexURL = slowServerURL(t, 5*time.Second)
			c.UpstreamTimeout = shortTimeout
		}, http.MethodGet, "/api/pokedex/species", validHeaders(), http.StatusServiceUnavailable, "upstream_unavailable"},
		{"pokedex 未設定", func(c *Config) { c.PokedexURL = nil },
			http.MethodGet, "/api/pokedex/species/9001-000", validHeaders(), http.StatusServiceUnavailable, "upstream_unavailable"},
		{"assets 未設定", func(c *Config) { c.AssetsURL = nil },
			http.MethodGet, "/assets/manifest.json", http.Header{}, http.StatusNotFound, "not_found"},
		{"assets 未設定(HEAD)", func(c *Config) { c.AssetsURL = nil },
			http.MethodHead, "/assets/manifest.json", http.Header{}, http.StatusNotFound, ""},
		// ヘッダ検証は上流の有無より先(pokedex 未設定でも不正なヘッダは 400)。
		{"pokedex 未設定でもヘッダ欠落は missing_header", func(c *Config) { c.PokedexURL = nil },
			http.MethodGet, "/api/pokedex/natures", http.Header{}, http.StatusBadRequest, "missing_header"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnv(t, tt.mutate)
			var body []byte
			if tt.method == http.MethodPost {
				body = []byte(`{}`)
			}
			start := time.Now()
			rec := serve(t, env.handler, tt.method, tt.path, tt.header, body)
			if elapsed := time.Since(start); elapsed > 2*time.Second {
				t.Errorf("応答まで %v かかった(UpstreamTimeout で打ち切られていない)", elapsed)
			}
			if tt.wantCode == "" { // HEAD は本文が無いのでステータスだけ見る
				if rec.Code != tt.wantStatus {
					t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
				}
				return
			}
			assertGatewayError(t, rec, tt.wantStatus, tt.wantCode)
		})
	}
}

// panicTransport は RoundTrip で panic する(gateway 内部の想定外の失敗の代わり)。
type panicTransport struct{}

const panicSecret = "secret-internal-detail-for-test"

func (panicTransport) RoundTrip(*http.Request) (*http.Response, error) { panic(panicSecret) }

// AC-G7: panic は回復して 500 internal(panic の値・スタックを message に出さない)。
func TestPanicIsRecoveredAsInternal(t *testing.T) {
	env := newTestEnv(t, func(c *Config) { c.transport = panicTransport{} })
	rec := serve(t, env.handler, http.MethodPost, "/api/calc", validHeaders(), []byte(`{}`))
	assertGatewayError(t, rec, http.StatusInternalServerError, "internal")
	if strings.Contains(rec.Body.String(), panicSecret) {
		t.Errorf("panic の値が本文に出ている: %s", rec.Body.String())
	}
	assertContract(t, http.MethodPost, "/api/calc", validHeaders(), []byte(`{}`), rec, false)
}

// NewHandler は必須の CalcURL が無い・タイムアウトが 0 以下・許可オリジンに "*" があれば ErrInvalidConfig。
func TestNewHandlerRejectsInvalidConfig(t *testing.T) {
	calc := mustParseURL(t, "http://calc.example.test:8080")
	tests := []struct {
		name string
		cfg  Config
	}{
		{"CalcURL が無い", Config{UpstreamTimeout: time.Second}},
		{"タイムアウトが 0", Config{CalcURL: calc}},
		{"タイムアウトが負", Config{CalcURL: calc, UpstreamTimeout: -time.Second}},
		{"許可オリジンに *", Config{CalcURL: calc, UpstreamTimeout: time.Second, CORSAllowedOrigins: []string{"*"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, err := NewHandler(tt.cfg)
			if err == nil || h != nil {
				t.Fatalf("NewHandler = %v, %v; want nil, ErrInvalidConfig", h, err)
			}
			if !errors.Is(err, ErrInvalidConfig) {
				t.Errorf("err = %v, want ErrInvalidConfig を包む", err)
			}
		})
	}
	t.Run("最小の設定(CalcURL とタイムアウト)は通る", func(t *testing.T) {
		if _, err := NewHandler(Config{CalcURL: calc, UpstreamTimeout: time.Second}); err != nil {
			t.Fatalf("NewHandler = %v, want nil", err)
		}
	})
}
