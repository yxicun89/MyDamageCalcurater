package httpapi

// 上流の失敗・未設定と panic 回復の受け入れテスト(ADR-0202 §5・§7。AC-G5・AC-G7)。

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
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

const panicDetail = "internal-panic-detail-for-test"

func (panicTransport) RoundTrip(*http.Request) (*http.Response, error) { panic(panicDetail) }

// AC-G7: panic は回復して 500 internal(panic の値・スタックを message に出さない)。
func TestPanicIsRecoveredAsInternal(t *testing.T) {
	env := newTestEnv(t, func(c *Config) { c.transport = panicTransport{} })
	rec := serve(t, env.handler, http.MethodPost, "/api/calc", validHeaders(), []byte(`{}`))
	assertGatewayError(t, rec, http.StatusInternalServerError, "internal")
	if strings.Contains(rec.Body.String(), panicDetail) {
		t.Errorf("panic の値が本文に出ている: %s", rec.Body.String())
	}
	assertContract(t, http.MethodPost, "/api/calc", validHeaders(), []byte(`{}`), rec, false)
}

// blockUntilCanceledTransport は RoundTrip が呼ばれた印を started に送ってから、リクエストの
// context が終わるまで応答を返さない(クライアントの中断を wall-clock sleep なしで確定的に再現する)。
// ReverseProxy は override 済み RoundTripper を1回しか呼ばない前提だが、呼ばれ方が変わっても
// 二重 close で panic しないよう sync.Once で守る。
type blockUntilCanceledTransport struct {
	started    chan struct{}
	startedOne sync.Once
}

func (tr *blockUntilCanceledTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tr.startedOne.Do(func() { close(tr.started) })
	<-req.Context().Done()
	return nil, req.Context().Err()
}

// assertNoResponseWritten は「応答を書かなかった」ことをまとめて確かめる(rec の既定値との区別を
// 明確にするため Code・Content-Type・本文を個別に見る。critic 指摘: 否定条件だけでは
// 将来 499/500 等の別のエラー応答を通してしまう)。
func assertNoResponseWritten(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d(recorder の既定値。何も書いていないこと)", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "" {
		t.Errorf("Content-Type = %q, want 空(ヘッダも書いていないこと)", ct)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("本文を書いている: %s(上流障害ではないので書かない)", rec.Body.String())
	}
}

// issue #113(クライアントのcancel伝播): ブラウザの AbortSignal・iOS の Task cancel で
// クライアントが要求を中断すると、Go の http.Server が r.Context() を context.Canceled で
// 終える。これは上流の障害ではないので、gateway は upstream_unavailable として扱わない
// (応答を書かない。相手はもう居ない)。フェイクの RoundTripper 版(分岐そのものの単体確認)と
// 実 Transport 版(本番と同じ http.Transport が実際に context.Canceled を返すことの固定。
// critic 指摘: フェイク版だけだと Go 側の実装が変わったときに本番だけ退行しテストは緑のまま)
// の両方で確認する。
func TestClientCancelIsNotUpstreamUnavailable(t *testing.T) {
	t.Run("フェイクの RoundTripper", func(t *testing.T) {
		started := make(chan struct{})
		env := newTestEnv(t, func(c *Config) { c.transport = &blockUntilCanceledTransport{started: started} })

		req := httptest.NewRequest(http.MethodPost, "/api/calc", strings.NewReader("{}"))
		req.Header = validHeaders()
		ctx, cancel := context.WithCancel(req.Context())
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		done := make(chan struct{})
		go func() {
			env.handler.ServeHTTP(rec, req)
			close(done)
		}()

		<-started // RoundTrip が上流を待ち始めるまで待つ(cancel との競合を避ける)
		cancel()  // クライアントが要求を中断した体で context をキャンセルする
		<-done

		assertNoResponseWritten(t, rec)
	})

	t.Run("実 Transport(本番と同じ http.Transport が実際に返すエラー)", func(t *testing.T) {
		entered := make(chan struct{})
		release := make(chan struct{})
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(entered)
			select {
			case <-r.Context().Done():
			case <-release:
			}
		}))
		// slowServerURL と同じ順序(release を先に解いてから Close。遅いハンドラの終了を待たない)。
		t.Cleanup(srv.Close)
		t.Cleanup(func() { close(release) })
		env := newTestEnv(t, func(c *Config) { c.CalcURL = mustParseURL(t, srv.URL) })

		req := httptest.NewRequest(http.MethodPost, "/api/calc", strings.NewReader("{}"))
		req.Header = validHeaders()
		ctx, cancel := context.WithCancel(req.Context())
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		done := make(chan struct{})
		go func() {
			env.handler.ServeHTTP(rec, req)
			close(done)
		}()

		<-entered // 上流ハンドラに実際に届くまで待つ(キャンセル伝播そのものの固定も兼ねる)
		cancel()
		<-done

		assertNoResponseWritten(t, rec)
	})
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
