package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"example.com/pokecalc/services/judge/internal/client"
)

func newRequest(path string, headers map[string]string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	return request
}

func serve(deps Dependencies, request *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	New(deps).ServeHTTP(recorder, request)
	return recorder
}

// unreachableUpstreams は、どこにもつながらない上流(予約ポート 1)を指すクライアントの組。
// ヘルスチェックが上流を呼ばないことを確かめるために使う。
func unreachableUpstreams(t *testing.T) Dependencies {
	t.Helper()
	const deadBaseURL = "http://127.0.0.1:1"

	pokedex, err := client.NewPokedex(client.Config{BaseURL: deadBaseURL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewPokedex: %v", err)
	}
	calc, err := client.NewCalc(client.Config{BaseURL: deadBaseURL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewCalc: %v", err)
	}
	return Dependencies{Pokedex: pokedex, Calc: calc}
}

// TestHealth: ヘルスチェックは上流(pokedex-svc・calc-svc)に依存しない(ADR-0700 §5)。
// 上流のクライアントが 1 つも設定されていない Dependencies{} でも 200 {"status":"ok"} を返し、
// X-Device-Id・X-Session-Id も要らない(kubelet の probe はヘッダーを付けない)。
func TestHealth(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"/healthz", "/api/judge/healthz"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			recorder := serve(Dependencies{}, newRequest(path, nil))
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
			}
			if got := strings.TrimSpace(recorder.Body.String()); got != `{"status":"ok"}` {
				t.Errorf("body = %s, want {\"status\":\"ok\"}", got)
			}
			if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
		})
	}
}

// TestHealthIgnoresUnreachableUpstream: 上流が落ちていてもヘルスは 200 のまま(ADR-0700 §5)。
// 到達できない上流を指すクライアントを渡しても、ヘルスは上流を呼ばずに即座に答える
// (呼んでいれば、クライアントのタイムアウト 1 秒ぶん待たされる)。
func TestHealthIgnoresUnreachableUpstream(t *testing.T) {
	t.Parallel()

	start := time.Now()
	recorder := serve(unreachableUpstreams(t), newRequest("/healthz", nil))
	elapsed := time.Since(start)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if got := strings.TrimSpace(recorder.Body.String()); got != `{"status":"ok"}` {
		t.Errorf("body = %s, want {\"status\":\"ok\"}", got)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("ヘルスに %v かかった。上流を呼んでいる可能性がある(ADR-0700 §5)", elapsed)
	}
}
