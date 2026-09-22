package client

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

// validSpeciesBody は pokedex-svc の GET /api/pokedex/species/{key} の 200 の本文
// (ルートの api/openapi.yaml の SpeciesDetail)を模した架空データ。実マスタは使わない
// (CLAUDE.md のドメイン規約)。図鑑番号 9001 は架空の番号。
func validSpeciesBody() []byte {
	return []byte(`{
      "key": "9001-000",
      "dexNo": 9001,
      "form": 0,
      "nameJa": "テストカソウドリ",
      "types": ["fire", "flying"],
      "baseStats": {"hp": 78, "atk": 84, "def": 78, "spa": 109, "spd": 85, "spe": 100},
      "abilities": [{"id": "test-ability", "nameJa": "テストとくせい"}],
      "learnset": ["test-move"]
    }`)
}

func newPokedex(t *testing.T, baseURL string, timeout time.Duration) *Pokedex {
	t.Helper()
	pokedex, err := NewPokedex(Config{BaseURL: baseURL, Timeout: timeout})
	if err != nil {
		t.Fatalf("NewPokedex: %v", err)
	}
	return pokedex
}

// TestSpeciesDecodesUpstreamResponse: 200 の本文から judge が使う欄(種族値・タイプ)を取り出し、
// 呼び出し元の X-Device-Id・X-Session-Id をそのまま上流へ転送する(ADR-0700 §2)。
func TestSpeciesDecodesUpstreamResponse(t *testing.T) {
	t.Parallel()

	var gotMethod, gotPath, gotDevice, gotSession string
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotDevice = r.Header.Get("X-Device-Id")
		gotSession = r.Header.Get("X-Session-Id")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(validSpeciesBody())
	})

	got, err := newPokedex(t, server.URL, testTimeout).Species(t.Context(), requestContext, "9001-000")
	if err != nil {
		t.Fatalf("Species: %v", err)
	}

	want := Species{
		Key:       "9001-000",
		NameJa:    "テストカソウドリ",
		Types:     []string{"fire", "flying"},
		BaseStats: StatBlock{HP: 78, Atk: 84, Def: 78, SpA: 109, SpD: 85, Spe: 100},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Species = %+v, want %+v", got, want)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/api/pokedex/species/9001-000" {
		t.Errorf("path = %q, want /api/pokedex/species/9001-000", gotPath)
	}
	// judge は自分の端末 ID を作らず、呼び出し元のものを中継する(CLAUDE.md の技術規約)。
	if gotDevice != requestContext.DeviceID || gotSession != requestContext.SessionID {
		t.Errorf("headers = (%q, %q), want (%q, %q)", gotDevice, gotSession, requestContext.DeviceID, requestContext.SessionID)
	}
}

// TestSpeciesRequiresRequestContext: 端末 ID・セッション ID が無いまま上流を呼ばない。
// 上流に届いてから 400 で跳ね返されるより、judge の中で止める方が速く、原因も分かりやすい。
func TestSpeciesRequiresRequestContext(t *testing.T) {
	t.Parallel()

	tests := map[string]RequestContext{
		"両方空":            {},
		"DeviceID が空":    {SessionID: "test-session"},
		"SessionID が空":   {DeviceID: "test-device"},
		"DeviceID が空白だけ": {DeviceID: "  ", SessionID: "test-session"},
	}
	for name, rc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			called := false
			server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(validSpeciesBody())
			})

			_, err := newPokedex(t, server.URL, testTimeout).Species(t.Context(), rc, "9001-000")
			if !errors.Is(err, ErrInvalidRequest) {
				t.Errorf("err = %v, want ErrInvalidRequest", err)
			}
			if called {
				t.Error("上流を呼んでいる。端末 ID が無い要求は judge の中で止める")
			}
		})
	}
}

// TestSpeciesNormalizesUpstreamStatus: 上流のステータスを ADR-0700 §3 の番兵エラーに畳む。
func TestSpeciesNormalizesUpstreamStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"500", http.StatusInternalServerError, `{"code":"internal_error","message":"boom"}`, ErrUpstreamUnavailable},
		{"502", http.StatusBadGateway, "", ErrUpstreamUnavailable},
		{"503", http.StatusServiceUnavailable, `{"code":"master_unavailable","message":"db"}`, ErrUpstreamUnavailable},
		{"404", http.StatusNotFound, `{"code":"not_found","message":"no such species"}`, ErrNotFound},
		{"400", http.StatusBadRequest, `{"code":"invalid_input","message":"bad key"}`, ErrInvalidRequest},
		// 契約に無い 2xx(本文が無い)は「届いたが契約に合わない」side。
		{"204", http.StatusNoContent, "", ErrUpstreamInvalidResponse},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			})

			_, err := newPokedex(t, server.URL, testTimeout).Species(t.Context(), requestContext, "9001-000")
			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

// TestSpeciesRejectsInvalidBody: 200 でも本文が契約に合わなければ ErrUpstreamInvalidResponse。
// 欠けた欄を 0 やゼロ値で埋めて計算を続けない(0 の種族値は判定を静かに間違える)。
func TestSpeciesRejectsInvalidBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{"壊れた JSON", `{"key": "9001-000"`},
		{"JSON ですらない", `<html>502 Bad Gateway</html>`},
		{"配列", `[]`},
		{"baseStats が無い", `{"key":"9001-000","dexNo":9001,"form":0,"nameJa":"テストカソウドリ","types":["fire"]}`},
		{
			"baseStats の spe が無い",
			`{"key":"9001-000","dexNo":9001,"form":0,"nameJa":"テストカソウドリ","types":["fire"],
			  "baseStats":{"hp":78,"atk":84,"def":78,"spa":109,"spd":85}}`,
		},
		{
			"types が空",
			`{"key":"9001-000","dexNo":9001,"form":0,"nameJa":"テストカソウドリ","types":[],
			  "baseStats":{"hp":78,"atk":84,"def":78,"spa":109,"spd":85,"spe":100}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			})

			_, err := newPokedex(t, server.URL, testTimeout).Species(t.Context(), requestContext, "9001-000")
			if !errors.Is(err, ErrUpstreamInvalidResponse) {
				t.Errorf("err = %v, want ErrUpstreamInvalidResponse", err)
			}
		})
	}
}

// TestSpeciesRejectsOversizedBody: 本文には上限(1 MiB。ADR-0700 §2)がある。
// 上流の異常時に無制限に読み込んでメモリを使い切らないため。
func TestSpeciesRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	const overLimit = 1<<20 + 1
	server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"nameJa": "` + strings.Repeat("x", overLimit) + `"}`))
	})

	_, err := newPokedex(t, server.URL, testTimeout).Species(t.Context(), requestContext, "9001-000")
	if !errors.Is(err, ErrUpstreamInvalidResponse) {
		t.Errorf("err = %v, want ErrUpstreamInvalidResponse", err)
	}
}

// TestSpeciesOnConnectionError: 誰も待ち受けていない上流は ErrUpstreamUnavailable で、
// 文面に上流の URL(deadBaseURL)を含まない(ADR-0700 §3。受け入れ条件5)。
func TestSpeciesOnConnectionError(t *testing.T) {
	t.Parallel()

	_, err := newPokedex(t, deadBaseURL, testTimeout).Species(t.Context(), requestContext, "9001-000")
	if !errors.Is(err, ErrUpstreamUnavailable) {
		t.Fatalf("err = %v, want ErrUpstreamUnavailable", err)
	}
	if strings.Contains(err.Error(), deadBaseURL) {
		t.Errorf("エラーが上流の URL を漏らしている: %s", err.Error())
	}
}

// TestSpeciesTimesOut: 答えない上流を設定のタイムアウトで打ち切る(ADR-0700 §2)。
// 1 回のクライアント要求の中で呼ぶので、起動時取得のような無限リトライはしない。
func TestSpeciesTimesOut(t *testing.T) {
	t.Parallel()

	server := blockingServer(t)

	start := time.Now()
	_, err := newPokedex(t, server.URL, shortTimeout).Species(t.Context(), requestContext, "9001-000")
	elapsed := time.Since(start)

	if !errors.Is(err, ErrUpstreamUnavailable) {
		t.Errorf("err = %v, want ErrUpstreamUnavailable", err)
	}
	if elapsed > time.Second {
		t.Errorf("%v 待った。設定のタイムアウト %v で打ち切ること", elapsed, shortTimeout)
	}
	if strings.Contains(err.Error(), server.URL) {
		t.Errorf("エラーが上流の URL を漏らしている: %s", err.Error())
	}
}

// TestSpeciesHonorsCallerContext: 呼び出し元の context が終わったら、設定のタイムアウトを待たずに戻り、
// context.Canceled を errors.Is で辿れつつ ErrUpstreamUnavailable の番兵も付く(ADR-0700 §3)。
// (judge の API ハンドラが client の接続断で打ち切れるようにするため)。
func TestSpeciesHonorsCallerContext(t *testing.T) {
	t.Parallel()

	server := blockingServer(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	start := time.Now()
	_, err := newPokedex(t, server.URL, time.Minute).Species(ctx, requestContext, "9001-000")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("err = nil, want an error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want it to wrap context.Canceled", err)
	}
	if !errors.Is(err, ErrUpstreamUnavailable) {
		t.Errorf("err = %v, want it to also wrap ErrUpstreamUnavailable", err)
	}
	if elapsed > time.Second {
		t.Errorf("%v 待った。呼び出し元の context が終わったらすぐ戻ること", elapsed)
	}
}

// TestSpeciesErrorDoesNotLeakUpstreamDetail: エラーの文面に上流の URL・本文・ステータス行を入れない
// (ADR-0700 §3。ADR-0600 §5 の「500 は固定文言」と同じ立場)。原因はログに残す。
func TestSpeciesErrorDoesNotLeakUpstreamDetail(t *testing.T) {
	t.Parallel()

	const upstreamDetail = "dsn dbhost01 svcaccount internal-only-detail"
	server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"code":"internal_error","message":"` + upstreamDetail + `"}`))
	})

	_, err := newPokedex(t, server.URL, testTimeout).Species(t.Context(), requestContext, "9001-000")
	if !errors.Is(err, ErrUpstreamUnavailable) {
		t.Fatalf("err = %v, want ErrUpstreamUnavailable", err)
	}
	message := err.Error()
	if strings.Contains(message, upstreamDetail) {
		t.Errorf("エラーが上流の本文を漏らしている: %s", message)
	}
	if strings.Contains(message, server.URL) {
		t.Errorf("エラーが上流の URL を漏らしている: %s", message)
	}
}
