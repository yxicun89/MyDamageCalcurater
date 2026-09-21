package httpapi

// X-Device-Id / X-Session-Id の検証の受け入れテスト(ADR-0020 §4。AC-G4)。

import (
	"net/http"
	"strings"
	"testing"
)

// apiRoute は検証を課す /api/* のルートの代表。
type apiRoute struct {
	method string
	path   string
	body   []byte
}

var apiRoutes = []apiRoute{
	{http.MethodPost, "/api/calc", []byte(`{}`)},
	{http.MethodPost, "/api/calc/bulk", []byte(`{}`)},
	{http.MethodPost, "/api/calc/reverse", []byte(`{}`)},
	{http.MethodGet, "/api/pokedex/species?q=a", nil},
	{http.MethodGet, "/api/pokedex/species/9001-000", nil},
}

func headersWith(mutate func(http.Header)) http.Header {
	h := validHeaders()
	mutate(h)
	return h
}

// AC-G4: 欠落・空は 400 missing_header、UUID でない値・同名ヘッダの重複は 400 invalid_header。
// どちらも上流には届かない。欠落と不正が同時にあるときは missing_header を優先する。
func TestHeaderValidationRejects(t *testing.T) {
	tests := []struct {
		name     string
		header   http.Header
		wantCode string
	}{
		{"X-Device-Id なし", headersWith(func(h http.Header) { h.Del("X-Device-Id") }), "missing_header"},
		{"X-Session-Id なし", headersWith(func(h http.Header) { h.Del("X-Session-Id") }), "missing_header"},
		{"両方なし", headersWith(func(h http.Header) { h.Del("X-Device-Id"); h.Del("X-Session-Id") }), "missing_header"},
		{"X-Device-Id が空", headersWith(func(h http.Header) { h.Set("X-Device-Id", "") }), "missing_header"},
		{"X-Session-Id が空", headersWith(func(h http.Header) { h.Set("X-Session-Id", "") }), "missing_header"},
		{"欠落と不正が同時なら missing_header", headersWith(func(h http.Header) {
			h.Del("X-Device-Id")
			h.Set("X-Session-Id", "not-a-uuid")
		}), "missing_header"},

		{"UUID でない文字列", headersWith(func(h http.Header) { h.Set("X-Device-Id", "not-a-uuid") }), "invalid_header"},
		{"1桁足りない", headersWith(func(h http.Header) { h.Set("X-Device-Id", "00000000-0000-4000-8000-00000000000") }), "invalid_header"},
		{"1桁多い", headersWith(func(h http.Header) { h.Set("X-Device-Id", "00000000-0000-4000-8000-0000000000001") }), "invalid_header"},
		{"16進でない文字", headersWith(func(h http.Header) { h.Set("X-Device-Id", "0000000g-0000-4000-8000-000000000001") }), "invalid_header"},
		{"ハイフンの位置が違う", headersWith(func(h http.Header) { h.Set("X-Device-Id", "000000000-000-4000-8000-000000000001") }), "invalid_header"},
		// 以下は google/uuid の uuid.Parse が受け付ける別表記。正準形 8-4-4-4-12 だけを通す。
		{"ハイフンなし32桁", headersWith(func(h http.Header) { h.Set("X-Device-Id", "00000000000040008000000000000001") }), "invalid_header"},
		{"波括弧つき", headersWith(func(h http.Header) { h.Set("X-Device-Id", "{00000000-0000-4000-8000-000000000001}") }), "invalid_header"},
		{"urn:uuid: つき", headersWith(func(h http.Header) { h.Set("X-Session-Id", "urn:uuid:00000000-0000-4000-8000-000000000002") }), "invalid_header"},
		{"X-Session-Id が UUID でない", headersWith(func(h http.Header) { h.Set("X-Session-Id", "session-1") }), "invalid_header"},

		{"X-Device-Id の重複(同じ値)", headersWith(func(h http.Header) { h.Add("X-Device-Id", testDeviceID) }), "invalid_header"},
		{"X-Session-Id の重複(別の値)", headersWith(func(h http.Header) {
			h.Add("X-Session-Id", "00000000-0000-4000-8000-000000000003")
		}), "invalid_header"},
	}
	for _, route := range apiRoutes {
		for _, tt := range tests {
			t.Run(route.method+" "+route.path+"/"+tt.name, func(t *testing.T) {
				env := newTestEnv(t)
				rec := serve(t, env.handler, route.method, route.path, tt.header, route.body)
				assertGatewayError(t, rec, http.StatusBadRequest, tt.wantCode)
				env.assertNoUpstreamReached(t)
			})
		}
	}
}

// AC-G4: 正準形の UUID は大文字小文字・版を問わず通り、値はそのまま(書き換えずに)上流へ届く。
func TestHeaderValidationAccepts(t *testing.T) {
	tests := []struct {
		name     string
		deviceID string
	}{
		{"小文字(版4)", testDeviceID},
		{"大文字", "ABCDEF01-2345-4678-9ABC-DEF012345678"},
		{"大文字小文字の混在", "abcDEF01-2345-4678-9abc-DEF012345678"},
		{"版を問わない(nil UUID)", "00000000-0000-0000-0000-000000000000"},
		{"版を問わない(版7)", "01890a5d-ac96-774b-bcce-b302099a8057"},
	}
	for _, route := range apiRoutes {
		for _, tt := range tests {
			t.Run(route.method+" "+route.path+"/"+tt.name, func(t *testing.T) {
				env := newTestEnv(t)
				header := headersWith(func(h http.Header) { h.Set("X-Device-Id", tt.deviceID) })
				rec := serve(t, env.handler, route.method, route.path, header, route.body)
				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200(上流の応答); body=%s", rec.Code, rec.Body.String())
				}
				upstream := env.calc
				if strings.HasPrefix(route.path, "/api/pokedex/") {
					upstream = env.pokedex
				}
				reqs := upstream.requests()
				if len(reqs) != 1 {
					t.Fatalf("上流 %s に届いた回数 = %d, want 1", upstream.name, len(reqs))
				}
				if got := reqs[0].Header.Get("X-Device-Id"); got != tt.deviceID {
					t.Errorf("上流に届いた X-Device-Id = %q, want %q(書き換えない)", got, tt.deviceID)
				}
				if got := reqs[0].Header.Get("X-Session-Id"); got != testSessionID {
					t.Errorf("上流に届いた X-Session-Id = %q, want %q", got, testSessionID)
				}
			})
		}
	}
}

// AC-G4: /assets/*・/healthz・CORS プリフライト(OPTIONS + Access-Control-Request-Method)には
// ヘッダの検証を課さない(<img> はヘッダを送れない。プリフライトはブラウザが独自ヘッダを付けない)。
func TestHeaderValidationExemptions(t *testing.T) {
	t.Run("/assets はヘッダ無しで上流に届く", func(t *testing.T) {
		env := newTestEnv(t)
		rec := serve(t, env.handler, http.MethodGet, "/assets/0445-000.webp", http.Header{}, nil)
		if rec.Code != http.StatusOK || len(env.assets.requests()) != 1 {
			t.Fatalf("status = %d, assets に届いた回数 = %d; want 200 / 1", rec.Code, len(env.assets.requests()))
		}
	})
	t.Run("/assets は UUID でないヘッダがあっても検証しない", func(t *testing.T) {
		env := newTestEnv(t)
		header := http.Header{"X-Device-Id": []string{"not-a-uuid"}}
		rec := serve(t, env.handler, http.MethodGet, "/assets/0445-000.webp", header, nil)
		if rec.Code != http.StatusOK || len(env.assets.requests()) != 1 {
			t.Fatalf("status = %d, assets に届いた回数 = %d; want 200 / 1", rec.Code, len(env.assets.requests()))
		}
	})
	t.Run("/healthz はヘッダ無しで 200", func(t *testing.T) {
		env := newTestEnv(t)
		rec := serve(t, env.handler, http.MethodGet, "/healthz", http.Header{}, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
	})
	for _, route := range apiRoutes {
		t.Run("プリフライト "+route.path+" はヘッダ無しで 204", func(t *testing.T) {
			env := newTestEnv(t)
			header := http.Header{
				"Origin":                         []string{allowedOrigin},
				"Access-Control-Request-Method":  []string{route.method},
				"Access-Control-Request-Headers": []string{"content-type,x-device-id,x-session-id"},
			}
			rec := serve(t, env.handler, http.MethodOptions, route.path, header, nil)
			if rec.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
			}
			env.assertNoUpstreamReached(t)
		})
	}
}
