package httpapi

// `Expect: 100-continue` を付けたクライアントに、上流のステータスがそのまま返ること(issue #209)。
// 上流が `100 Continue` を返してから 4xx/5xx を返すとき、Echo の Response が 100 で commit されて
// 続く WriteHeader(4xx) を捨て、既定の 200 が出ていた。

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// syncBuffer は複数の goroutine から書かれるログの受け皿。
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestExpectContinueKeepsUpstreamStatus(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		upstream func(*testEnv) *fakeUpstream
		status   int
	}{
		{"calc が 400", http.MethodPost, "/api/calc", func(e *testEnv) *fakeUpstream { return e.calc }, http.StatusBadRequest},
		{"calc が 200", http.MethodPost, "/api/calc", func(e *testEnv) *fakeUpstream { return e.calc }, http.StatusOK},
		{"pokedex が 404", http.MethodGet, "/api/pokedex/species/9001-000", func(e *testEnv) *fakeUpstream { return e.pokedex }, http.StatusNotFound},
		{"pokedex が 503", http.MethodGet, "/api/pokedex/natures", func(e *testEnv) *fakeUpstream { return e.pokedex }, http.StatusServiceUnavailable},
		{"web が 404", http.MethodGet, "/some/page", func(e *testEnv) *fakeUpstream { return e.web }, http.StatusNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logs := &syncBuffer{}
			env := newWebTestEnv(t, func(c *Config) {
				c.logger = slog.New(slog.NewJSONHandler(logs, nil))
			})
			up := tc.upstream(env)
			const body = `{"from":"upstream"}`
			// fakeUpstream は本文を読んでから応答するので、Go の http.Server は Expect に 100 Continue を返す。
			up.respond(upstreamResponse{status: tc.status, contentType: "application/json", body: body})

			gw := httptest.NewServer(env.handler)
			t.Cleanup(gw.Close)

			req, err := http.NewRequest(tc.method, gw.URL+tc.path, strings.NewReader(`{"payload":true}`))
			if err != nil {
				t.Fatalf("NewRequest = %v", err)
			}
			for k, vs := range validHeaders() {
				req.Header[k] = vs
			}
			req.Header.Set("Expect", "100-continue")
			resp, err := gw.Client().Do(req)
			if err != nil {
				t.Fatalf("Do = %v", err)
			}
			defer resp.Body.Close()
			got, _ := io.ReadAll(resp.Body)

			if resp.StatusCode != tc.status {
				t.Errorf("status = %d, want %d(上流のステータス); body=%s", resp.StatusCode, tc.status, got)
			}
			if string(got) != body {
				t.Errorf("body = %q, want %q", got, body)
			}
			reqs := up.requests()
			if len(reqs) != 1 {
				t.Fatalf("上流に届いた回数 = %d, want 1", len(reqs))
			}
			if e := reqs[0].Header.Get("Expect"); e != "" {
				t.Errorf("上流に Expect = %q が届いた(gateway→上流では 100-continue を使わない)", e)
			}
			if strings.Contains(logs.String(), "response already written") {
				t.Errorf("echo が二重の WriteHeader を記録した: %s", logs.String())
			}
		})
	}
}
