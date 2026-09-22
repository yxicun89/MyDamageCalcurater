package httpapi_test

// pokedex-svc の HTTP 境界のテストの共通部分(ADR-0105 §2・§3)。DB の代わりに storetest の偽の Querier を使う。
// 応答は api/openapi.yaml(生成物に埋め込まれた仕様)に kin-openapi で照らす(test-strategy.md L4)。

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/legacy"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/pokedex/internal/httpapi"
	"example.com/pokecalc/services/pokedex/internal/store"
)

// 端末ID・セッションID(公開 API の必須ヘッダ。形式の検証は gateway の仕事で、pokedex-svc は有無だけを見る。ADR-0202)。
const (
	testDeviceID  = "00000000-0000-4000-8000-000000000001"
	testSessionID = "00000000-0000-4000-8000-000000000002"
)

// do は handler に1リクエストを送る。withHeaders が真なら端末ID・セッションIDを付ける。
func do(t *testing.T, h http.Handler, method, target string, withHeaders bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	if withHeaders {
		req.Header.Set("X-Device-Id", testDeviceID)
		req.Header.Set("X-Session-Id", testSessionID)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func newHandler(t *testing.T, q store.Querier) http.Handler {
	t.Helper()
	h := httpapi.NewHandler(q)
	if h == nil {
		t.Fatal("NewHandler が nil を返した")
	}
	return h
}

// validateAgainstContract は応答(status・本文)を契約に照らす。リクエストも契約に照らす
// (公開 API はヘッダ付き、内部 API はヘッダ無しで送る)。
func validateAgainstContract(t *testing.T, method, target string, withHeaders bool, rec *httptest.ResponseRecorder) {
	t.Helper()
	doc, err := api.GetSwagger()
	if err != nil {
		t.Fatalf("契約を読めない: %v", err)
	}
	doc.Servers = nil
	router, err := legacy.NewRouter(doc)
	if err != nil {
		t.Fatalf("契約のルータを作れない: %v", err)
	}
	req := httptest.NewRequest(method, target, nil)
	if withHeaders {
		req.Header.Set("X-Device-Id", testDeviceID)
		req.Header.Set("X-Session-Id", testSessionID)
	}
	route, params, err := router.FindRoute(req)
	if err != nil {
		t.Fatalf("契約に %s %s が無い: %v", method, target, err)
	}
	in := &openapi3filter.RequestValidationInput{
		Request: req, PathParams: params, Route: route,
		Options: &openapi3filter.Options{IncludeResponseStatus: true, MultiError: true},
	}
	if err := openapi3filter.ValidateRequest(context.Background(), in); err != nil {
		t.Fatalf("リクエストが契約に合わない: %v", err)
	}
	if ct := rec.Header().Get("Content-Type"); ct == "" {
		t.Fatalf("Content-Type が無い")
	}
	err = openapi3filter.ValidateResponse(context.Background(), &openapi3filter.ResponseValidationInput{
		RequestValidationInput: in, Status: rec.Code, Header: rec.Header(),
		Body: io.NopCloser(bytes.NewReader(rec.Body.Bytes())), Options: in.Options,
	})
	if err != nil {
		t.Fatalf("応答(%d)が契約に合わない: %v\nbody=%s", rec.Code, err, rec.Body.String())
	}
}

// decodeError は Error 形式の本文を読む(未知のフィールドを拒否)。
func decodeError(t *testing.T, rec *httptest.ResponseRecorder) api.Error {
	t.Helper()
	var e api.Error
	dec := json.NewDecoder(bytes.NewReader(rec.Body.Bytes()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&e); err != nil {
		t.Fatalf("Error 形式でない: %v\nbody=%s", err, rec.Body.String())
	}
	if e.Message == "" {
		t.Errorf("message が空")
	}
	return e
}

// assertError は status と code を確かめる。
func assertError(t *testing.T, rec *httptest.ResponseRecorder, status int, code api.ErrorCode) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d\nbody=%s", rec.Code, status, rec.Body.String())
	}
	if e := decodeError(t, rec); e.Code != code {
		t.Fatalf("code = %q, want %q(message=%q)", e.Code, code, e.Message)
	}
}
