package httpapi

// 契約テスト(test-strategy.md L4。P3-3 の gateway 経由の契約テストの先取り。ADR-0202 AC-G8)。
// gateway 自身が作るエラー応答と、上流が calc-svc の実物(calctest。架空マスタ)のときの応答を
// api/openapi.yaml に照らして kin-openapi で検証する。仕様は生成物に埋め込まれたもの
// (api.GetSwagger。make gen で openapi.yaml から作られる)を使う。検証ヘルパーは
// services/calc/internal/httpapi/contract_test.go と同じやり方(internal 規則で共有できないため写し)。

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"

	"example.com/pokecalc/services/calc/calctest"
	"example.com/pokecalc/services/internal/api"
)

var (
	contractOnce   sync.Once
	contractDoc    *openapi3.T
	contractRouter routers.Router
	contractErr    error
)

func loadContract(t *testing.T) (*openapi3.T, routers.Router) {
	t.Helper()
	contractOnce.Do(func() {
		contractDoc, contractErr = api.GetSwagger()
		if contractErr != nil {
			return
		}
		// servers の "/" はホスト名の照合を持ち込むだけなので外す(パスだけで引く)。
		contractDoc.Servers = nil
		contractRouter, contractErr = legacy.NewRouter(contractDoc)
	})
	if contractErr != nil {
		t.Fatalf("契約(api/openapi.yaml)を読めない: %v", contractErr)
	}
	return contractDoc, contractRouter
}

// assertContract は rec が契約どおりであることを確かめる。契約に無いルートは失敗にする
// (呼び出し側は契約にあるルートだけを渡す。契約外の not_found は assertErrorSchema で見る)。
func assertContract(t *testing.T, method, path string, header http.Header, reqBody []byte,
	rec *httptest.ResponseRecorder, checkRequest bool) {
	t.Helper()
	_, router := loadContract(t)
	req := httptest.NewRequest(method, path, bytes.NewReader(reqBody))
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	route, params, err := router.FindRoute(req)
	if err != nil {
		t.Fatalf("契約に %s %s が無い: %v", method, path, err)
	}
	in := &openapi3filter.RequestValidationInput{
		Request: req, PathParams: params, Route: route,
		Options: &openapi3filter.Options{IncludeResponseStatus: true, MultiError: true},
	}
	if checkRequest {
		if err := openapi3filter.ValidateRequest(context.Background(), in); err != nil {
			t.Errorf("リクエストが契約に合わない: %v", err)
		}
	}
	out := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: in,
		Status:                 rec.Code,
		Header:                 rec.Header(),
		Body:                   io.NopCloser(bytes.NewReader(rec.Body.Bytes())),
		Options:                in.Options,
	}
	if err := openapi3filter.ValidateResponse(context.Background(), out); err != nil {
		t.Errorf("レスポンスが契約に合わない: %v\nstatus=%d body=%s", err, rec.Code, rec.Body.String())
	}
}

// assertErrorSchema は本文が契約の Error スキーマ(code は ErrorCode の enum)に合うことを確かめる。
// 契約に無いルート(未知のパス・/assets)で gateway が返す not_found にも使う。
func assertErrorSchema(t *testing.T, body []byte) {
	t.Helper()
	doc, _ := loadContract(t)
	ref, ok := doc.Components.Schemas["Error"]
	if !ok || ref.Value == nil {
		t.Fatal("契約に Error スキーマが無い")
	}
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("本文が JSON でない: %v; body=%s", err, body)
	}
	if err := ref.Value.VisitJSON(v); err != nil {
		t.Errorf("本文が Error スキーマに合わない: %v; body=%s", err, body)
	}
}

// ErrorCode に invalid_header が追加され、missing_header と別の語彙であること(契約差分)。
func TestContractHasInvalidHeader(t *testing.T) {
	if !api.ErrorCode("invalid_header").Valid() {
		t.Fatal(`api.ErrorCode("invalid_header").Valid() = false(openapi.yaml の ErrorCode に追加して make gen)`)
	}
	// ヘルパーが空振りしないこと: 語彙外の code は落ちる。
	doc, _ := loadContract(t)
	var bad any
	_ = json.Unmarshal([]byte(`{"code":"invalid_uuid","message":"語彙外"}`), &bad)
	if err := doc.Components.Schemas["Error"].Value.VisitJSON(bad); err == nil {
		t.Error("語彙外の code が Error スキーマを通った(検証が空振りしている)")
	}
}

// AC-G8: gateway 自身が作るエラー応答は契約どおり(契約にあるルートは操作のレスポンス定義で、
// 無いルートは Error スキーマで検証する)。
func TestGatewayErrorsMatchContract(t *testing.T) {
	calcBody := []byte(`{}`)
	missingDevice := func() http.Header { h := validHeaders(); h.Del("X-Device-Id"); return h }
	invalidSession := func() http.Header { h := validHeaders(); h.Set("X-Session-Id", "not-a-uuid"); return h }
	duplicateDevice := func() http.Header { h := validHeaders(); h.Add("X-Device-Id", testDeviceID); return h }

	tests := []struct {
		name       string
		mutate     func(*Config)
		method     string
		path       string
		header     http.Header
		wantStatus int
		wantCode   string
	}{
		{"calc: missing_header", nil, http.MethodPost, "/api/calc", missingDevice(), 400, "missing_header"},
		{"bulk: invalid_header(UUID でない)", nil, http.MethodPost, "/api/calc/bulk", invalidSession(), 400, "invalid_header"},
		{"reverse: invalid_header(重複)", nil, http.MethodPost, "/api/calc/reverse", duplicateDevice(), 400, "invalid_header"},
		{"pokedex: missing_header", nil, http.MethodGet, "/api/pokedex/natures", missingDevice(), 400, "missing_header"},
		{"pokedex 未設定: upstream_unavailable", func(c *Config) { c.PokedexURL = nil },
			http.MethodGet, "/api/pokedex/species?q=a", validHeaders(), 503, "upstream_unavailable"},
		{"pokedex 詳細 未設定: upstream_unavailable", func(c *Config) { c.PokedexURL = nil },
			http.MethodGet, "/api/pokedex/species/9001-000", validHeaders(), 503, "upstream_unavailable"},
		{"calc 接続拒否: upstream_unavailable", func(c *Config) { c.CalcURL = closedServerURL(t) },
			http.MethodPost, "/api/calc/reverse", validHeaders(), 503, "upstream_unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mutate []func(*Config)
			if tt.mutate != nil {
				mutate = append(mutate, tt.mutate)
			}
			env := newTestEnv(t, mutate...)
			var body []byte
			if tt.method == http.MethodPost {
				body = calcBody
			}
			rec := serve(t, env.handler, tt.method, tt.path, tt.header, body)
			assertGatewayError(t, rec, tt.wantStatus, tt.wantCode)
			assertContract(t, tt.method, tt.path, tt.header, body, rec, false)
		})
	}

	// 契約に無いルートの not_found(未知のパス・/api/balance・assets 未設定)も Error スキーマに合う。
	for _, path := range []string{"/api/balance", "/nothing", "/assets/0445-000.webp"} {
		t.Run("not_found "+path, func(t *testing.T) {
			env := newTestEnv(t, func(c *Config) { c.AssetsURL = nil })
			rec := serve(t, env.handler, http.MethodGet, path, validHeaders(), nil)
			assertGatewayError(t, rec, http.StatusNotFound, "not_found")
		})
	}
}

// newRealCalcEnv は calc の上流を calc-svc の実物(架空マスタ)にした gateway を作る。
func newRealCalcEnv(t *testing.T) http.Handler {
	t.Helper()
	calcHandler, err := calctest.NewExampleHandler()
	if err != nil {
		t.Fatalf("calc-svc の実物を起動できない: %v", err)
	}
	calcSrv := httptest.NewServer(calcHandler)
	t.Cleanup(calcSrv.Close)
	h, err := NewHandler(Config{CalcURL: mustParseURL(t, calcSrv.URL), UpstreamTimeout: defaultTestTimeout})
	if err != nil {
		t.Fatalf("NewHandler = %v", err)
	}
	return h
}

// realCalcIndividual は例のマスタの個体(攻撃側はテストモン A32 S32、防御側はテストガード H32)。
func realCalcIndividual(speciesKey, natureID string, sp map[string]int) map[string]any {
	return map[string]any{"speciesKey": speciesKey, "natureId": natureID, "sp": sp}
}

func spOf(hp, atk, def, spa, spd, spe int) map[string]int {
	return map[string]int{"hp": hp, "atk": atk, "def": def, "spa": spa, "spd": spd, "spe": spe}
}

// AC-G8 / AC-S1(ADR-0203 §1): 上流が calc-svc の実物のとき、gateway 経由の calc・bulk・reverse の成功と、
// 代表的な 400(gateway の missing_header / invalid_header、calc-svc の unknown_field / unknown_species / unknown_move)、
// pokedex 未設定の 503 が契約どおり(成功はリクエストも契約に照らす)。gateway は上流の応答を書き換えない。
func TestRealCalcThroughGatewayMatchesContract(t *testing.T) {
	h := newRealCalcEnv(t)
	attacker := realCalcIndividual(calctest.SpeciesAttacker, calctest.NatureAtkUp, spOf(0, 32, 0, 0, 0, 32))
	defender := realCalcIndividual(calctest.SpeciesDefender, calctest.NatureNeutral, spOf(32, 0, 0, 0, 0, 0))
	unknownDefender := realCalcIndividual("9999-000", calctest.NatureNeutral, spOf(0, 0, 0, 0, 0, 0))

	// 3操作それぞれの正しい本文(成功の行と、ヘッダだけを崩す行で共有する)。
	calcBody := map[string]any{
		"format": "single", "attacker": attacker, "defender": defender, "moveId": calctest.MovePhysical,
	}
	bulkBody := map[string]any{
		"format": "single", "attacker": attacker, "defenderSpeciesKey": calctest.SpeciesDefender, "moveId": calctest.MovePhysical,
	}
	reverseBody := map[string]any{
		"format": "single", "side": "defender", "known": attacker, "unknownSpeciesKey": calctest.SpeciesDefender,
		"moveId": calctest.MovePhysical, "observations": []map[string]any{{"percent": 40}},
	}
	// with は本文に1つのフィールドを足した(または上書きした)写しを返す。
	with := func(body map[string]any, key string, value any) map[string]any {
		out := make(map[string]any, len(body)+1)
		for k, v := range body {
			out[k] = v
		}
		out[key] = value
		return out
	}
	missingSession := func() http.Header { h := validHeaders(); h.Del("X-Session-Id"); return h }
	invalidDevice := func() http.Header { h := validHeaders(); h.Set("X-Device-Id", "not-a-uuid"); return h }

	tests := []struct {
		name       string
		method     string
		path       string
		header     func() http.Header // nil なら validHeaders
		body       map[string]any     // GET は nil
		wantStatus int
		wantCode   string // エラーのときだけ
	}{
		{"calc の成功", http.MethodPost, "/api/calc", nil, calcBody, http.StatusOK, ""},
		{"bulk の成功", http.MethodPost, "/api/calc/bulk", nil, bulkBody, http.StatusOK, ""},
		{"reverse の成功", http.MethodPost, "/api/calc/reverse", nil, reverseBody, http.StatusOK, ""},

		{"calc: missing_header", http.MethodPost, "/api/calc", missingSession, calcBody, http.StatusBadRequest, "missing_header"},
		{"bulk: missing_header", http.MethodPost, "/api/calc/bulk", missingSession, bulkBody, http.StatusBadRequest, "missing_header"},
		{"reverse: missing_header", http.MethodPost, "/api/calc/reverse", missingSession, reverseBody, http.StatusBadRequest, "missing_header"},
		{"calc: invalid_header", http.MethodPost, "/api/calc", invalidDevice, calcBody, http.StatusBadRequest, "invalid_header"},
		{"bulk: invalid_header", http.MethodPost, "/api/calc/bulk", invalidDevice, bulkBody, http.StatusBadRequest, "invalid_header"},
		{"reverse: invalid_header", http.MethodPost, "/api/calc/reverse", invalidDevice, reverseBody, http.StatusBadRequest, "invalid_header"},

		{"calc: calc-svc の unknown_field はそのまま", http.MethodPost, "/api/calc", nil,
			with(calcBody, "notInContract", 1), http.StatusBadRequest, "unknown_field"},
		{"bulk: calc-svc の unknown_field はそのまま", http.MethodPost, "/api/calc/bulk", nil,
			with(bulkBody, "notInContract", 1), http.StatusBadRequest, "unknown_field"},
		{"reverse: calc-svc の unknown_field はそのまま", http.MethodPost, "/api/calc/reverse", nil,
			with(reverseBody, "notInContract", 1), http.StatusBadRequest, "unknown_field"},
		{"calc: calc-svc の unknown_species はそのまま", http.MethodPost, "/api/calc", nil,
			with(calcBody, "defender", unknownDefender), http.StatusBadRequest, "unknown_species"},
		{"bulk: calc-svc の unknown_species はそのまま", http.MethodPost, "/api/calc/bulk", nil,
			with(bulkBody, "defenderSpeciesKey", "9999-000"), http.StatusBadRequest, "unknown_species"},
		{"reverse: calc-svc の unknown_species はそのまま", http.MethodPost, "/api/calc/reverse", nil,
			with(reverseBody, "unknownSpeciesKey", "9999-000"), http.StatusBadRequest, "unknown_species"},
		{"calc-svc の 400(unknown_move)はそのまま", http.MethodPost, "/api/calc", nil,
			with(calcBody, "moveId", "test-no-such-move"), http.StatusBadRequest, "unknown_move"},

		// pokedex-svc(P2-3)が入るまで k3d の gateway は GATEWAY_POKEDEX_URL 未設定(ADR-0203 §3)。
		{"pokedex 未設定: natures は upstream_unavailable", http.MethodGet, "/api/pokedex/natures", nil, nil,
			http.StatusServiceUnavailable, "upstream_unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := validHeaders()
			if tt.header != nil {
				header = tt.header()
			}
			var body []byte
			if tt.body != nil {
				body = mustJSON(t, tt.body)
			}
			rec := serve(t, h, tt.method, tt.path, header, body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantCode != "" {
				if got := decodeErrorBody(t, rec); got.Code != tt.wantCode {
					t.Errorf("code = %q, want %q", got.Code, tt.wantCode)
				}
			}
			assertContract(t, tt.method, tt.path, header, body, rec, tt.wantStatus == http.StatusOK)
		})
	}

	// 同名ヘッダの重複は gateway が invalid_header で止める(calc-svc の判定と同じ語彙。ADR-0202)。
	t.Run("重複ヘッダは gateway で invalid_header", func(t *testing.T) {
		header := validHeaders()
		header.Add("X-Session-Id", testSessionID)
		body := mustJSON(t, calcBody)
		rec := serve(t, h, http.MethodPost, "/api/calc", header, body)
		assertGatewayError(t, rec, http.StatusBadRequest, "invalid_header")
		assertContract(t, http.MethodPost, "/api/calc", header, body, rec, false)
	})
}
