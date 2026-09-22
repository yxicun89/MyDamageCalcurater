package client

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// requestContext はテストで使う架空の端末 ID・セッション ID(CLAUDE.md の技術規約)。
var requestContext = RequestContext{DeviceID: "test-device", SessionID: "test-session"}

// testTimeout は上流が正常に答えるテストで使うタイムアウト。実時間で待たない値にする。
const testTimeout = 2 * time.Second

// shortTimeout はタイムアウトそのものを確かめるテストで使う値(実時間の待ちを 100ms 程度に抑える)。
const shortTimeout = 50 * time.Millisecond

// newTestServer は架空の上流を1つ立てる(テスト終了時に閉じる)。
func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

// blockingServer は、テストが終わるまで応答を返さない上流。タイムアウトの検査に使う。
func blockingServer(t *testing.T) *httptest.Server {
	t.Helper()
	done := make(chan struct{})
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-done:
		case <-r.Context().Done():
		}
	})
	t.Cleanup(func() { close(done) }) // server.Close より先に走る(Cleanup は後入れ先出し。services/calc/internal/master/source_test.go と同じ前例)
	return server
}

// deadBaseURL は誰も待ち受けていないアドレス(接続エラーの検査に使う)。
const deadBaseURL = "http://127.0.0.1:1"

// TestNewRejectsInvalidConfig: base URL とタイムアウトは起動時に検証する(ADR-0700 §3)。
// 設定ミスを起動時に気づけるようにし、リクエストのたびに失敗させない。
// pokedex・calc の両方のコンストラクタが同じ検証を行う。
func TestNewRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config Config
	}{
		{"base URL が空", Config{BaseURL: "", Timeout: testTimeout}},
		{"scheme が http/https でない", Config{BaseURL: "ftp://pokedex", Timeout: testTimeout}},
		{"scheme が無い(相対 URL)", Config{BaseURL: "pokedex:8080", Timeout: testTimeout}},
		{"ホストが無い", Config{BaseURL: "http://", Timeout: testTimeout}},
		{"クエリを含む", Config{BaseURL: "http://pokedex?debug=1", Timeout: testTimeout}},
		{"タイムアウトが 0", Config{BaseURL: "http://pokedex", Timeout: 0}},
		{"タイムアウトが負", Config{BaseURL: "http://pokedex", Timeout: -time.Second}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := NewPokedex(tt.config); err == nil {
				t.Errorf("NewPokedex(%+v) = nil error, want error", tt.config)
			}
			if _, err := NewCalc(tt.config); err == nil {
				t.Errorf("NewCalc(%+v) = nil error, want error", tt.config)
			}
		})
	}
}

// TestNewAcceptsTrailingSlash: 末尾の / の有無でパスが二重の // にならないこと。
// 環境変数で渡される base URL は書き方が揺れるため、どちらでも同じ URL を組み立てる。
func TestNewAcceptsTrailingSlash(t *testing.T) {
	t.Parallel()

	var paths []string
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(validSpeciesBody())
	})

	for _, baseURL := range []string{server.URL, server.URL + "/"} {
		pokedex, err := NewPokedex(Config{BaseURL: baseURL, Timeout: testTimeout})
		if err != nil {
			t.Fatalf("NewPokedex(%q): %v", baseURL, err)
		}
		if _, err := pokedex.Species(t.Context(), requestContext, "9001-000"); err != nil {
			t.Fatalf("Species(%q): %v", baseURL, err)
		}
	}

	want := "/api/pokedex/species/9001-000"
	for _, got := range paths {
		if got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
	}
}
