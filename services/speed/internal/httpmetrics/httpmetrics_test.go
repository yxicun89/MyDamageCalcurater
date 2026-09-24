package httpmetrics_test

// httpmetrics の受け入れテスト(ADR-0406 §1〜3。P7-1)。
//
// 公開 API として固定するもの(implementer はこの形で実装する):
//
//	httpmetrics.Path             // "/metrics"
//	httpmetrics.New() *Metrics   // 呼ぶたびに独立したレジストリを持つ(グローバルの既定レジストリに登録しない)
//	(*Metrics).Middleware() echo.MiddlewareFunc
//	(*Metrics).Handler() echo.HandlerFunc   // Prometheus text format を返す
//
// services/internal/httpmetrics/httpmetrics_test.go の複製(import パスだけが違う)。同じ内容のテストが
// services/internal・balance・speed・judge の4箇所にある
// (ADR-0406 §2: 共有 Go module を作らず、1ファイルを複製する)。ここを直したら4箇所とも直すこと。

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/speed/internal/httpmetrics"
)

const (
	requestsTotal   = "http_requests_total"
	durationSeconds = "http_request_duration_seconds"
)

// errTeapot はテスト用のエラーハンドラが 418 に写す番兵。
var errTeapot = errors.New("teapot")

// newInstrumented は httpmetrics を組み込んだ echo を作る。/items/:id(GET・POST)・/created(201)・
// /conflict(HTTPError 409)・/teapot(独自エラーハンドラで 418)・/panic(内側の回復で 500)を持つ。
// errorHandlerCalls はエラーハンドラが呼ばれた回数(二重に書かないことの確認用)。
func newInstrumented(t *testing.T) (e *echo.Echo, errorHandlerCalls *int) {
	t.Helper()
	m := httpmetrics.New()
	if m == nil {
		t.Fatal("httpmetrics.New() が nil を返した")
	}
	calls := 0
	e = echo.New()
	e.HTTPErrorHandler = func(c *echo.Context, err error) {
		calls++
		if errors.Is(err, errTeapot) {
			_ = c.JSON(http.StatusTeapot, map[string]string{"code": "teapot"})
			return
		}
		echo.DefaultHTTPErrorHandler(false)(c, err)
	}
	e.Use(m.Middleware())
	// 既存サービスの recoverMiddleware 相当(panic を 500 の error にする)。httpmetrics より内側に置く。
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = echo.ErrInternalServerError
				}
			}()
			return next(c)
		}
	})
	e.GET(httpmetrics.Path, m.Handler())
	e.GET("/items/:id", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") })
	e.POST("/items/:id", func(c *echo.Context) error { return c.String(http.StatusOK, "posted") })
	e.GET("/created", func(c *echo.Context) error { return c.NoContent(http.StatusCreated) })
	e.GET("/conflict", func(c *echo.Context) error { return echo.NewHTTPError(http.StatusConflict, "conflict") })
	e.GET("/teapot", func(c *echo.Context) error { return errTeapot })
	e.GET("/panic", func(c *echo.Context) error { panic("意図的な panic(テスト)") })
	return e, &calls
}

func do(t *testing.T, h http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

// scrape は GET /metrics を呼び、200 であることを確かめて本文を返す。
func scrape(t *testing.T, h http.Handler) string {
	t.Helper()
	rec := do(t, h, http.MethodGet, httpmetrics.Path)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: status = %d, want 200; body=%s", httpmetrics.Path, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// sample は text format の1行(コメント以外)。
type sample struct {
	name   string
	labels map[string]string
	value  float64
}

var (
	sampleLine = regexp.MustCompile(`^([a-zA-Z_:][a-zA-Z0-9_:]*)(?:\{(.*)\})?\s+(\S+)`)
	labelPair  = regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)="((?:[^"\\]|\\.)*)"`)
)

func parseSamples(t *testing.T, body string) []sample {
	t.Helper()
	var out []sample
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := sampleLine.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("text format として読めない行: %q", line)
		}
		v, err := strconv.ParseFloat(m[3], 64)
		if err != nil {
			t.Fatalf("値を読めない行: %q: %v", line, err)
		}
		labels := map[string]string{}
		for _, p := range labelPair.FindAllStringSubmatch(m[2], -1) {
			labels[p[1]] = p[2]
		}
		out = append(out, sample{name: m[1], labels: labels, value: v})
	}
	return out
}

// exactValue はラベル集合が want と完全一致するサンプルの値を返す。
func exactValue(samples []sample, name string, want map[string]string) (float64, bool) {
	for _, s := range samples {
		if s.name != name || len(s.labels) != len(want) {
			continue
		}
		match := true
		for k, v := range want {
			if s.labels[k] != v {
				match = false
				break
			}
		}
		if match {
			return s.value, true
		}
	}
	return 0, false
}

// sumOf は name のサンプルのうち、subset のラベルをすべて持つものの合計。
func sumOf(samples []sample, name string, subset map[string]string) float64 {
	total := 0.0
	for _, s := range samples {
		if s.name != name {
			continue
		}
		match := true
		for k, v := range subset {
			if s.labels[k] != v {
				match = false
				break
			}
		}
		if match {
			total += s.value
		}
	}
	return total
}

// assertNoLabelContains はどのサンプルのどのラベル値にも raw が含まれないことを確かめる(カーディナリティ対策。ADR-0406 §1)。
func assertNoLabelContains(t *testing.T, samples []sample, raw string) {
	t.Helper()
	for _, s := range samples {
		for k, v := range s.labels {
			if strings.Contains(v, raw) {
				t.Errorf("%s{%s=%q}: 実際に来たパスの一部 %q がラベルに入っている(ルーティングパターンを使うこと)", s.name, k, v, raw)
			}
		}
	}
}

// AC-1: GET /metrics は Prometheus text format(# HELP・# TYPE 行)を返す。
func TestHandlerServesPrometheusTextFormat(t *testing.T) {
	e, _ := newInstrumented(t)
	do(t, e, http.MethodGet, "/items/abc")

	rec := do(t, e, http.MethodGet, httpmetrics.Path)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain で始まる(Prometheus text format)", ct)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"# HELP " + requestsTotal + " ",
		"# TYPE " + requestsTotal + " counter",
		"# HELP " + durationSeconds + " ",
		"# TYPE " + durationSeconds + " histogram",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("本文に %q が無い; body=\n%s", want, body)
		}
	}
	parseSamples(t, body) // 全行が text format として読めること
}

// AC-2: http_requests_total{method,path,status} を数え、path はルーティングパターン(生のパスではない)。
func TestCountsRequestsByRoutePattern(t *testing.T) {
	e, _ := newInstrumented(t)
	for _, target := range []string{"/items/raw-id-0001", "/items/raw-id-0002"} {
		if rec := do(t, e, http.MethodGet, target); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
			t.Fatalf("GET %s: status = %d body = %q, want 200 \"ok\"(計測が応答を変えてはいけない)", target, rec.Code, rec.Body.String())
		}
	}
	do(t, e, http.MethodPost, "/items/raw-id-0003")

	samples := parseSamples(t, scrape(t, e))
	if got, ok := exactValue(samples, requestsTotal, map[string]string{"method": "GET", "path": "/items/:id", "status": "200"}); !ok || got != 2 {
		t.Errorf(`%s{method="GET",path="/items/:id",status="200"} = %v (存在=%v), want 2`, requestsTotal, got, ok)
	}
	if got, ok := exactValue(samples, requestsTotal, map[string]string{"method": "POST", "path": "/items/:id", "status": "200"}); !ok || got != 1 {
		t.Errorf(`%s{method="POST",path="/items/:id",status="200"} = %v (存在=%v), want 1`, requestsTotal, got, ok)
	}
	for _, raw := range []string{"raw-id-0001", "raw-id-0002", "raw-id-0003"} {
		assertNoLabelContains(t, samples, raw)
	}
}

// AC-3: http_request_duration_seconds{method,path} はヒストグラムで、既定のバケット(prometheus.DefBuckets)を使う。
// status ラベルは持たない(ADR-0406 §1 の指標定義どおり)。
func TestRecordsDurationHistogram(t *testing.T) {
	e, _ := newInstrumented(t)
	do(t, e, http.MethodGet, "/items/a")
	do(t, e, http.MethodGet, "/items/b")

	samples := parseSamples(t, scrape(t, e))
	labels := map[string]string{"method": "GET", "path": "/items/:id"}
	if got, ok := exactValue(samples, durationSeconds+"_count", labels); !ok || got != 2 {
		t.Errorf("%s_count%v = %v (存在=%v), want 2(status ラベルを付けないこと)", durationSeconds, labels, got, ok)
	}
	if _, ok := exactValue(samples, durationSeconds+"_sum", labels); !ok {
		t.Errorf("%s_sum%v が無い", durationSeconds, labels)
	}
	// prometheus/client_golang の DefBuckets(.005〜10)と +Inf。
	for _, le := range []string{"0.005", "0.01", "0.025", "0.05", "0.1", "0.25", "0.5", "1", "2.5", "5", "10", "+Inf"} {
		want := map[string]string{"method": "GET", "path": "/items/:id", "le": le}
		if _, ok := exactValue(samples, durationSeconds+"_bucket", want); !ok {
			t.Errorf("%s_bucket{le=%q} が無い(既定のバケットを使うこと)", durationSeconds, le)
		}
	}
	if got, _ := exactValue(samples, durationSeconds+"_bucket", map[string]string{"method": "GET", "path": "/items/:id", "le": "+Inf"}); got != 2 {
		t.Errorf("%s_bucket{le=\"+Inf\"} = %v, want 2", durationSeconds, got)
	}
}

// AC-4: status ラベルはクライアントに実際に返した最終ステータス。handler が error を返した場合
// (echo の HTTPErrorHandler が後から書く場合)も、書かれたステータスで数える。計測が応答を変えない。
func TestStatusLabelReflectsFinalResponse(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		target     string
		wantStatus int
		wantPath   string // ルーティングパターン。"" なら path は検査しない(404/405 のパターンは echo 任せ)
		rawSegment string // ラベルに入ってはいけない生のパスの断片("" なら検査しない)
	}{
		{"handler が 201 を書く", http.MethodGet, "/created", http.StatusCreated, "/created", ""},
		{"handler が HTTPError(409) を返す", http.MethodGet, "/conflict", http.StatusConflict, "/conflict", ""},
		{"handler の error を独自エラーハンドラが 418 に写す", http.MethodGet, "/teapot", http.StatusTeapot, "/teapot", ""},
		{"内側で回復した panic は 500", http.MethodGet, "/panic", http.StatusInternalServerError, "/panic", ""},
		{"ルートが無い(404)", http.MethodGet, "/no-such-route/raw-segment-404", http.StatusNotFound, "", "raw-segment-404"},
		{"メソッド違い(405)", http.MethodDelete, "/items/raw-segment-405", http.StatusMethodNotAllowed, "", "raw-segment-405"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, calls := newInstrumented(t)
			rec := do(t, e, tt.method, tt.target)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d(計測が応答を変えてはいけない); body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantStatus == http.StatusTeapot {
				if got := strings.TrimSpace(rec.Body.String()); got != `{"code":"teapot"}` {
					t.Errorf("body = %q, want {\"code\":\"teapot\"}(エラーハンドラの応答を1回だけ書く)", got)
				}
			}
			if tt.wantStatus >= 400 && *calls != 1 {
				t.Errorf("HTTPErrorHandler の呼び出し回数 = %d, want 1(二重に呼ばない)", *calls)
			}

			samples := parseSamples(t, scrape(t, e))
			subset := map[string]string{"method": tt.method, "status": strconv.Itoa(tt.wantStatus)}
			if tt.wantPath != "" {
				subset["path"] = tt.wantPath
			}
			if got := sumOf(samples, requestsTotal, subset); got != 1 {
				t.Errorf("%s%v の合計 = %v, want 1", requestsTotal, subset, got)
			}
			if tt.rawSegment != "" {
				assertNoLabelContains(t, samples, tt.rawSegment)
			}
		})
	}
}

// AC-5: /metrics 自体は計測しない(自己参照でカウンタ・ヒストグラムを汚さない。ADR-0406 §3)。
func TestMetricsEndpointIsNotCounted(t *testing.T) {
	e, _ := newInstrumented(t)
	do(t, e, http.MethodGet, "/items/a")

	before := parseSamples(t, scrape(t, e))
	for range 3 {
		scrape(t, e)
	}
	after := parseSamples(t, scrape(t, e))

	if b, a := sumOf(before, requestsTotal, nil), sumOf(after, requestsTotal, nil); b != 1 || a != 1 {
		t.Errorf("%s の合計: /metrics の前 = %v, 後 = %v, want どちらも 1(/metrics を数えない)", requestsTotal, b, a)
	}
	if b, a := sumOf(before, durationSeconds+"_count", nil), sumOf(after, durationSeconds+"_count", nil); b != 1 || a != 1 {
		t.Errorf("%s_count の合計: /metrics の前 = %v, 後 = %v, want どちらも 1", durationSeconds, b, a)
	}
	for _, s := range after {
		if s.labels["path"] == httpmetrics.Path {
			t.Errorf("%s に path=%q のサンプルがある(自己参照)", s.name, httpmetrics.Path)
		}
	}
}

// AC-6: New() は呼ぶたびに独立したレジストリを持つ。同じプロセスで何度 NewHandler を作っても
// (テストはハンドラを何度も作る)重複登録で panic せず、カウントも混ざらない。
func TestInstancesAreIndependent(t *testing.T) {
	e1, _ := newInstrumented(t)
	e2, _ := newInstrumented(t)
	do(t, e1, http.MethodGet, "/items/a")

	if got := sumOf(parseSamples(t, scrape(t, e1)), requestsTotal, nil); got != 1 {
		t.Errorf("e1 の %s の合計 = %v, want 1", requestsTotal, got)
	}
	if got := sumOf(parseSamples(t, scrape(t, e2)), requestsTotal, nil); got != 0 {
		t.Errorf("e2 の %s の合計 = %v, want 0(別インスタンスのカウントが混ざっている)", requestsTotal, got)
	}
}

// AC-7: net/http はどんな文字列でも HTTP メソッドとして受け取ってしまうため、標準の9メソッド
// (GET/HEAD/POST/PUT/PATCH/DELETE/OPTIONS/CONNECT/TRACE)以外は method="OTHER" に正規化する
// (path と同じ、カーディナリティ対策。ADR-0406 §1)。
func TestUnknownMethodsNormalizeToOther(t *testing.T) {
	e, _ := newInstrumented(t)
	for _, method := range []string{"XMETHOD1", "XMETHOD2", "XMETHOD3"} {
		do(t, e, method, "/items/a")
	}

	samples := parseSamples(t, scrape(t, e))
	if got := sumOf(samples, requestsTotal, map[string]string{"method": "OTHER"}); got != 3 {
		t.Errorf(`%s{method="OTHER"} の合計 = %v, want 3`, requestsTotal, got)
	}
	assertNoLabelContains(t, samples, "XMETHOD")
}
