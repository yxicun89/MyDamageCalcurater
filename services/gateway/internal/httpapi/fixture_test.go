package httpapi

// gateway のテストの共通部品: 上流の偽物(fakeUpstream。httptest.Server で届いたリクエストを記録し、
// 決めたレスポンスを返す)、テスト用の Config とハンドラ、エラー本文の検査。

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// 正しい形の UUID(版 4 の架空の値)。
const (
	testDeviceID  = "00000000-0000-4000-8000-000000000001"
	testSessionID = "00000000-0000-4000-8000-000000000002"
)

// 許可オリジン(架空のホスト名。.test は予約済み TLD)。
const (
	allowedOrigin      = "https://app.example.test"
	allowedOriginLocal = "http://localhost:5173"
	disallowedOrigin   = "https://evil.example.test"
)

// defaultTestTimeout は通常のテストの上流タイムアウト(遅い上流のテストだけ短くする)。
const defaultTestTimeout = 2 * time.Second

// recordedRequest は上流に届いたリクエストの記録。
type recordedRequest struct {
	Method   string
	Path     string
	RawQuery string
	Host     string // 上流が受け取った Host(推奨3: クライアントの Host ではなく上流のホストになる)
	Header   http.Header
	Body     []byte
}

// upstreamResponse は fakeUpstream が返すレスポンス。
type upstreamResponse struct {
	status      int
	contentType string
	body        string
	header      http.Header // 追加のレスポンスヘッダ(Cache-Control など)
}

// fakeUpstream は上流サービスの偽物。届いたリクエストを記録し、resp を返す。
type fakeUpstream struct {
	name string
	srv  *httptest.Server

	mu   sync.Mutex
	reqs []recordedRequest
	resp upstreamResponse
}

func newFakeUpstream(t *testing.T, name string) *fakeUpstream {
	t.Helper()
	f := &fakeUpstream{
		name: name,
		resp: upstreamResponse{status: http.StatusOK, contentType: "application/json", body: `{"from":"` + name + `"}`},
	}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeUpstream) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.reqs = append(f.reqs, recordedRequest{
		Method: r.Method, Path: r.URL.Path, RawQuery: r.URL.RawQuery, Host: r.Host, Header: r.Header.Clone(), Body: body,
	})
	resp := f.resp
	f.mu.Unlock()

	for k, vs := range resp.header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	if resp.contentType != "" {
		w.Header().Set("Content-Type", resp.contentType)
	}
	w.WriteHeader(resp.status)
	if r.Method != http.MethodHead {
		_, _ = io.WriteString(w, resp.body)
	}
}

func (f *fakeUpstream) respond(resp upstreamResponse) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resp = resp
}

func (f *fakeUpstream) requests() []recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recordedRequest(nil), f.reqs...)
}

func (f *fakeUpstream) url(t *testing.T) *url.URL {
	t.Helper()
	return mustParseURL(t, f.srv.URL)
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("URL を解析できない %q: %v", raw, err)
	}
	return u
}

// testEnv は5つの上流の偽物と、それらに向けた gateway のハンドラ。web・record の偽物は常に起動するが、
// Config の WebURL / RecordURL に入るのは newWebTestEnv / newRecordTestEnv のときだけ
// (newTestEnv では未設定 = 従来どおり。ADR-0209 §10 の record_routing_test.go が前提にする)。
type testEnv struct {
	calc, pokedex, assets, web, record *fakeUpstream
	handler                            http.Handler
}

// newTestEnv は既定の Config(calc・pokedex・assets の上流・許可オリジン2つ・タイムアウト 2s。WebURL・
// RecordURL は未設定)で gateway を作る。mutate で Config を書き換えられる(未設定の上流・短いタイムアウト・
// CORS 無しなど)。
func newTestEnv(t *testing.T, mutate ...func(*Config)) *testEnv {
	t.Helper()
	return buildTestEnv(t, false, false, mutate...)
}

// newWebTestEnv は newTestEnv に加えて WebURL を web の偽物に向けた gateway を作る(ADR-0205)。
// mutate は WebURL を設定した後に適用する(WebURL を閉じたサーバや遅いサーバに差し替えられる)。
func newWebTestEnv(t *testing.T, mutate ...func(*Config)) *testEnv {
	t.Helper()
	return buildTestEnv(t, true, false, mutate...)
}

// newRecordTestEnv は newTestEnv に加えて RecordURL を record の偽物に向けた gateway を作る
// (ADR-0209 §10)。record_routing_test.go の既定(RecordURL 未設定)を壊さないよう、newTestEnv とは
// 別のコンストラクタにしてある。
func newRecordTestEnv(t *testing.T, mutate ...func(*Config)) *testEnv {
	t.Helper()
	return buildTestEnv(t, false, true, mutate...)
}

func buildTestEnv(t *testing.T, withWeb, withRecord bool, mutate ...func(*Config)) *testEnv {
	t.Helper()
	env := &testEnv{
		calc:    newFakeUpstream(t, "calc"),
		pokedex: newFakeUpstream(t, "pokedex"),
		assets:  newFakeUpstream(t, "assets"),
		web:     newFakeUpstream(t, "web"),
		record:  newFakeUpstream(t, "record"),
	}
	cfg := Config{
		CalcURL:            env.calc.url(t),
		PokedexURL:         env.pokedex.url(t),
		AssetsURL:          env.assets.url(t),
		CORSAllowedOrigins: []string{allowedOrigin, allowedOriginLocal},
		UpstreamTimeout:    defaultTestTimeout,
	}
	if withWeb {
		cfg.WebURL = env.web.url(t)
	}
	if withRecord {
		cfg.RecordURL = env.record.url(t)
	}
	for _, m := range mutate {
		m(&cfg)
	}
	h, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("NewHandler = %v", err)
	}
	env.handler = h
	return env
}

// upstreams は5つの上流を名前つきで返す(「どこにも届かない」の検査用。web・record は未設定でも含める)。
func (e *testEnv) upstreams() []*fakeUpstream {
	return []*fakeUpstream{e.calc, e.pokedex, e.assets, e.web, e.record}
}

// assertNoUpstreamReached はどの上流にもリクエストが届いていないことを確かめる。
func (e *testEnv) assertNoUpstreamReached(t *testing.T) {
	t.Helper()
	for _, u := range e.upstreams() {
		if got := u.requests(); len(got) != 0 {
			t.Errorf("上流 %s にリクエストが届いた(届いてはいけない): %+v", u.name, got)
		}
	}
}

// validHeaders は必須ヘッダ(X-Device-Id / X-Session-Id)と Content-Type をそろえたヘッダ。
func validHeaders() http.Header {
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	h.Set("X-Device-Id", testDeviceID)
	h.Set("X-Session-Id", testSessionID)
	return h
}

// serve は gateway のハンドラを直接呼ぶ。
func serve(t *testing.T, h http.Handler, method, path string, header http.Header, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// errorBody は契約の Error 本文。
type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// decodeErrorBody は Error 本文を厳格に読む(未知のフィールドを拒否。code と message は必須)。
func decodeErrorBody(t *testing.T, rec *httptest.ResponseRecorder) errorBody {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(rec.Body.Bytes()))
	dec.DisallowUnknownFields()
	var got errorBody
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("Error 本文として読めない: %v; body=%s", err, rec.Body.String())
	}
	if got.Code == "" || got.Message == "" {
		t.Fatalf("Error 本文の code / message が空: %s", rec.Body.String())
	}
	return got
}

// internalLeakMarkers は gateway が作るエラーの message に出してはいけない Go の内部情報の断片。
var internalLeakMarkers = []string{"dial", "tcp", "127.0.0.1", "connection refused", "context deadline", "goroutine", "panic", "http://"}

// assertGatewayError は gateway 自身が作ったエラー応答を確かめる: ステータス・code・JSON の
// Content-Type・Error スキーマへの適合・message に内部情報が無いこと。
func assertGatewayError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Errorf("status = %d, want %d; body=%s", rec.Code, wantStatus, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	got := decodeErrorBody(t, rec)
	if got.Code != wantCode {
		t.Errorf("code = %q, want %q; message=%q", got.Code, wantCode, got.Message)
	}
	for _, marker := range internalLeakMarkers {
		if strings.Contains(strings.ToLower(got.Message), marker) {
			t.Errorf("message に内部情報 %q が含まれる: %q", marker, got.Message)
		}
	}
	assertErrorSchema(t, rec.Body.Bytes())
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("JSON 化に失敗: %v", err)
	}
	return b
}
