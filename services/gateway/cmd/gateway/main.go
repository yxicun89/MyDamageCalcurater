// Command gateway はクライアントの唯一の入口(ADR-0202)。/api/calc・/api/pokedex・/assets を各上流へ転送する。
//
// 設定は環境変数で渡す(1か所で読み込み、必須値は起動時に検証する。docs/coding-rules.md §2):
//
//	GATEWAY_ADDR                  待ち受けアドレス(既定 ":8080")
//	GATEWAY_CALC_URL              calc-svc の基底 URL。必須
//	GATEWAY_POKEDEX_URL           pokedex-svc の基底 URL。任意(未設定なら /api/pokedex/* は 503)
//	GATEWAY_RECORD_URL            record-svc の基底 URL。任意(未設定なら /api/record/* は 503。ADR-0209 §10)
//	GATEWAY_ASSETS_URL            画像配信の基底 URL。任意(未設定なら /assets/* は 404)
//	GATEWAY_WEB_URL               Web の静的配信の基底 URL。任意(設定時は予約パス以外の GET / HEAD を転送。ADR-0205)
//	GATEWAY_CORS_ALLOWED_ORIGINS  カンマ区切りの許可オリジン(完全一致)。任意。"*" は起動エラー
//	GATEWAY_UPSTREAM_TIMEOUT      上流の応答ヘッダを待つ上限(Go の duration)。既定 10s
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"example.com/pokecalc/services/gateway/internal/httpapi"
)

// 環境変数の名前(運用の manifest・README が依存する)。
const (
	envAddr               = "GATEWAY_ADDR"
	envCalcURL            = "GATEWAY_CALC_URL"
	envPokedexURL         = "GATEWAY_POKEDEX_URL"
	envRecordURL          = "GATEWAY_RECORD_URL"
	envAssetsURL          = "GATEWAY_ASSETS_URL"
	envCORSAllowedOrigins = "GATEWAY_CORS_ALLOWED_ORIGINS"
	envUpstreamTimeout    = "GATEWAY_UPSTREAM_TIMEOUT"
	// envWebURL は Web の静的配信の基底 URL(任意。ADR-0205)。
	envWebURL = "GATEWAY_WEB_URL"

	// defaultAddr は GATEWAY_ADDR が未設定・空のときの待ち受けアドレス。
	defaultAddr = ":8080"
	// defaultUpstreamTimeout は GATEWAY_UPSTREAM_TIMEOUT が未設定・空のときの既定値。
	defaultUpstreamTimeout = 10 * time.Second

	// shutdownTimeout は ctx 終了後、進行中のリクエストを待つ猶予。
	shutdownTimeout = 5 * time.Second

	// http.Server のタイムアウト(calc-svc と同じ理由。遅い・止まったクライアントに接続を占有され続けない)。
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 60 * time.Second // assets の転送を早期に打ち切らない(ADR-0202 §5)
	idleTimeout       = 60 * time.Second
)

// errInvalidConfig は設定の読み込みに失敗したとき loadConfig が包んで返すエラー。
var errInvalidConfig = errors.New("gateway の設定が不正")

// config は gateway の設定(環境変数から1度だけ読む)。
type config struct {
	Addr    string
	Gateway httpapi.Config
}

// loadConfig は環境変数から設定を読む。不正なら errInvalidConfig を包んで返す。
func loadConfig(lookup func(string) (string, bool)) (config, error) {
	addr, _ := lookup(envAddr)
	if addr == "" {
		addr = defaultAddr
	}

	calcURL, ok := lookup(envCalcURL)
	if !ok || calcURL == "" {
		return config{}, fmt.Errorf("%w: %s が未設定", errInvalidConfig, envCalcURL)
	}
	calc, err := parseUpstreamURL(calcURL)
	if err != nil {
		return config{}, fmt.Errorf("%w: %s が不正: %v", errInvalidConfig, envCalcURL, err)
	}

	var pokedex *url.URL
	if raw, ok := lookup(envPokedexURL); ok && raw != "" {
		pokedex, err = parseUpstreamURL(raw)
		if err != nil {
			return config{}, fmt.Errorf("%w: %s が不正: %v", errInvalidConfig, envPokedexURL, err)
		}
	}

	var record *url.URL
	if raw, ok := lookup(envRecordURL); ok && raw != "" {
		record, err = parseUpstreamURL(raw)
		if err != nil {
			return config{}, fmt.Errorf("%w: %s が不正: %v", errInvalidConfig, envRecordURL, err)
		}
	}

	var assets *url.URL
	if raw, ok := lookup(envAssetsURL); ok && raw != "" {
		assets, err = parseUpstreamURL(raw)
		if err != nil {
			return config{}, fmt.Errorf("%w: %s が不正: %v", errInvalidConfig, envAssetsURL, err)
		}
	}

	var web *url.URL
	if raw, ok := lookup(envWebURL); ok && raw != "" {
		web, err = parseWebURL(raw)
		if err != nil {
			return config{}, fmt.Errorf("%w: %s が不正: %v", errInvalidConfig, envWebURL, err)
		}
	}

	origins, err := parseCORSOrigins(lookup)
	if err != nil {
		return config{}, err
	}

	timeout := defaultUpstreamTimeout
	if raw, ok := lookup(envUpstreamTimeout); ok && raw != "" {
		timeout, err = time.ParseDuration(raw)
		if err != nil {
			return config{}, fmt.Errorf("%w: %s が duration として解析できない: %v", errInvalidConfig, envUpstreamTimeout, err)
		}
		if timeout <= 0 {
			return config{}, fmt.Errorf("%w: %s は正でなければならない(%s)", errInvalidConfig, envUpstreamTimeout, raw)
		}
	}
	// 任意7: 上流の応答待ちタイムアウトが http.Server の書き込みタイムアウト以上だと、
	// 上流がタイムアウトぎりぎりまで粘ったときに WriteTimeout がクライアントへの応答を
	// 先に打ち切ってしまう(本文の転送を打ち切らないという ADR-0202 §5 の方針に反する)。
	if timeout >= writeTimeout {
		return config{}, fmt.Errorf("%w: %s(%s)は http.Server の書き込みタイムアウト(%s)より短くなければならない",
			errInvalidConfig, envUpstreamTimeout, timeout, writeTimeout)
	}

	return config{
		Addr: addr,
		Gateway: httpapi.Config{
			CalcURL:            calc,
			PokedexURL:         pokedex,
			RecordURL:          record,
			AssetsURL:          assets,
			WebURL:             web,
			CORSAllowedOrigins: origins,
			UpstreamTimeout:    timeout,
		},
	}, nil
}

// parseUpstreamURL は上流の基底 URL を解析する。絶対 URL・スキームが http/https・ホストがあることを求める。
func parseUpstreamURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("スキームは http/https でなければならない: %q", raw)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("ホストが無い: %q", raw)
	}
	return u, nil
}

// parseWebURL は GATEWAY_WEB_URL を解析する(絶対 URL・スキームは http/https・ホストあり。加えてクエリを
// 含めない。ADR-0205)。
func parseWebURL(raw string) (*url.URL, error) {
	u, err := parseUpstreamURL(raw)
	if err != nil {
		return nil, err
	}
	if u.RawQuery != "" {
		return nil, fmt.Errorf("クエリを含められない: %q", raw)
	}
	return u, nil
}

// parseCORSOrigins は GATEWAY_CORS_ALLOWED_ORIGINS をカンマ区切りで読む(前後の空白は除く)。
// 各要素は scheme://host[:port] のオリジン(パス・末尾スラッシュなし)。"*" は起動エラー。
func parseCORSOrigins(lookup func(string) (string, bool)) ([]string, error) {
	raw, ok := lookup(envCORSAllowedOrigins)
	if !ok || raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		origin := strings.TrimSpace(p)
		if origin == "" {
			continue
		}
		if origin == "*" {
			return nil, fmt.Errorf("%w: %s に \"*\" は使えない", errInvalidConfig, envCORSAllowedOrigins)
		}
		if err := validateOrigin(origin); err != nil {
			return nil, fmt.Errorf("%w: %s の %q が不正: %v", errInvalidConfig, envCORSAllowedOrigins, origin, err)
		}
		origins = append(origins, origin)
	}
	return origins, nil
}

// validateOrigin は1つのオリジン(scheme://host[:port]。パス・末尾スラッシュ・クエリ・フラグメント無し)を検証する。
func validateOrigin(origin string) error {
	u, err := url.Parse(origin)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("スキームが無い、または http/https でない: %q", origin)
	}
	if u.Host == "" {
		return fmt.Errorf("ホストが無い: %q", origin)
	}
	if u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("オリジンはパス・クエリ・フラグメントを含めない: %q", origin)
	}
	return nil
}

// run は設定を読み、ctx が終わるまで待ち受ける。ctx が終わったらサーバを止めて nil を返す。
func run(ctx context.Context, lookup func(string) (string, bool)) error {
	cfg, err := loadConfig(lookup)
	if err != nil {
		return err
	}
	handler, err := httpapi.NewHandler(cfg.Gateway)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
	serveErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		<-serveErr
		return nil
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.LookupEnv); err != nil {
		slog.Error("gateway を起動できない", "error", err)
		os.Exit(1)
	}
}
