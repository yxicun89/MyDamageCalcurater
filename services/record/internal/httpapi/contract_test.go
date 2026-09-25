package httpapi

// 契約テスト(test-strategy.md L4)。record-svc の応答を api/openapi.yaml に照らして検証する。
// calc-svc / pokedex-svc の contract_test.go と同じ形で、仕様は生成物に埋め込まれたもの
// (api.GetSwagger)を使う。AC-C2(契約とサービスの一致)。

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"

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
		contractDoc.Servers = nil
		contractRouter, contractErr = legacy.NewRouter(contractDoc)
	})
	if contractErr != nil {
		t.Fatalf("契約(api/openapi.yaml)を読めない: %v", contractErr)
	}
	return contractDoc, contractRouter
}

// assertMatchesContract は1往復を契約に照らす。checkRequest が false のときはリクエストの
// スキーマ適合は見ない(意図的に契約の範囲外の値を送る行〈例: limit=0〉は、リクエスト自体が
// スキーマの minimum/maximum に反することが目的で、それでもサーバーが 400 invalid_input を
// 契約どおりの Error 形式で返すこと〈レスポンス側〉を確かめたいため。kin-openapi の
// ValidateRequest はパラメータの範囲もリクエスト検証に含めるので、意図的に範囲外の値を送る
// ケースまでリクエスト検証すると、どんな実装でも必ず失敗する)。
func assertMatchesContract(t *testing.T, method, path string, hdr http.Header, reqBody []byte, rec *httptest.ResponseRecorder, checkRequest bool) {
	t.Helper()
	_, router := loadContract(t)

	req := httptest.NewRequest(method, path, bytes.NewReader(reqBody))
	for k, vs := range hdr {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	route, params, err := router.FindRoute(req)
	if err != nil {
		t.Fatalf("契約に %s %s のルートが無い: %v", method, path, err)
	}
	in := &openapi3filter.RequestValidationInput{
		Request: req, PathParams: params, Route: route,
		Options: &openapi3filter.Options{IncludeResponseStatus: true, MultiError: true},
	}
	reqErr := openapi3filter.ValidateRequest(context.Background(), in)
	switch {
	case checkRequest && reqErr != nil:
		t.Errorf("リクエストが契約に合わない(%s %s): %v", method, path, reqErr)
	case !checkRequest && reqErr == nil:
		// critic 指摘 R-6: checkRequest=false は「リクエストが意図的に契約の範囲外」という前提に
		// 依存しているので、その前提が崩れていないこと(=本当にスキーマ違反であること)も確かめる。
		// 将来 schema の minimum/maximum を緩めてこの前提が崩れたら、ここで気づけるようにする。
		t.Errorf("checkRequest=false のケース(%s %s)が実はスキーマ違反ではない。"+
			"schema の範囲制約が緩んでいないか確認すること", method, path)
	}
	respBody := rec.Body.Bytes()
	respIn := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: in,
		Status:                 rec.Code,
		Header:                 rec.Header(),
		Body:                   io.NopCloser(bytes.NewReader(respBody)),
		Options:                &openapi3filter.Options{IncludeResponseStatus: true, MultiError: true},
	}
	if err := openapi3filter.ValidateResponse(context.Background(), respIn); err != nil {
		t.Errorf("レスポンスが契約に合わない(%s %s → %d): %v; body=%s", method, path, rec.Code, err, respBody)
	}
}

// 成功・エラーのそれぞれが契約どおりであること。
func TestRecordResponsesMatchContract(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(*fakeStore)
		method       string
		path         string
		body         []byte
		checkRequest bool // false: リクエスト自体が意図的に契約の範囲外(assertMatchesContract 参照)
	}{
		{"集計 200", func(f *fakeStore) { f.seed(deviceA, 0, 3, 0) }, http.MethodGet, pathFrequent, nil, true},
		{"集計 200(空)", func(*fakeStore) {}, http.MethodGet, pathFrequent, nil, true},
		{"集計 400(limit 範囲外)", func(*fakeStore) {}, http.MethodGet, pathFrequent + "?limit=0", nil, false},
		{"集計 503", func(f *fakeStore) { f.unavailable = true }, http.MethodGet, pathFrequent, nil, true},
		{"削除 200 completed", func(f *fakeStore) { f.seed(deviceA, 2, 1, 1) }, http.MethodDelete, pathDeviceData, nil, true},
		{"削除 200 partial", func(f *fakeStore) { f.purgeLimit = 1; f.seed(deviceA, 2, 1, 1) }, http.MethodDelete, pathDeviceData, nil, true},
		{"削除 503", func(f *fakeStore) { f.unavailable = true }, http.MethodDelete, pathDeviceData, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newFakeStore()
			tt.setup(st)
			rec := serve(t, NewHandler(st), tt.method, tt.path, headers(deviceA), tt.body)
			assertMatchesContract(t, tt.method, tt.path, headers(deviceA), tt.body, rec, tt.checkRequest)
		})
	}
}

// AC-C2 / ADR-0209 §5.3・§10: 契約そのものに record の要素が揃っていること。
// (openapi.yaml を直接読むのではなく、make gen で埋め込まれた仕様を見る。)
func TestContractHasRecordOperations(t *testing.T) {
	doc, _ := loadContract(t)

	t.Run("store_unavailable が ErrorCode にある", func(t *testing.T) {
		ref := doc.Components.Schemas["ErrorCode"]
		if ref == nil || ref.Value == nil {
			t.Fatal("ErrorCode が契約に無い")
		}
		found := false
		for _, v := range ref.Value.Enum {
			if s, ok := v.(string); ok && s == string(api.StoreUnavailable) {
				found = true
			}
		}
		if !found {
			t.Errorf("ErrorCode の enum に store_unavailable が無い: %v", ref.Value.Enum)
		}
	})

	t.Run("record タグがある", func(t *testing.T) {
		for _, tag := range doc.Tags {
			if tag.Name == "record" {
				return
			}
		}
		t.Error("top-level の tags に record が無い(ADR-0209 §5.3 移植チェックリスト)")
	})

	ops := []struct {
		path, method, operationID string
		wantStatuses              []string
	}{
		{"/api/record/device-data", http.MethodDelete, "deleteRecordDeviceData", []string{"200", "400", "500", "503"}},
		{"/api/record/frequent-opponents", http.MethodGet, "listFrequentOpponents", []string{"200", "400", "503"}},
	}
	for _, o := range ops {
		t.Run(o.operationID, func(t *testing.T) {
			item := doc.Paths.Find(o.path)
			if item == nil {
				t.Fatalf("契約に %s が無い", o.path)
			}
			op := item.GetOperation(o.method)
			if op == nil {
				t.Fatalf("契約に %s %s が無い", o.method, o.path)
			}
			// api.GetSwagger() が埋め込む仕様は oapi-codegen が生成した Go のメソッド名
			// (PascalCase)に正規化された operationId を持つ(openapi.yaml 自体は lowerCamelCase。
			// calc-svc の calc/internal/master/contract_test.go の TestGetMasterExportRoute と
			// 同じ理由で EqualFold にする)。
			if !strings.EqualFold(op.OperationID, o.operationID) {
				t.Errorf("operationId = %q, want %q(大文字小文字を無視して比較)", op.OperationID, o.operationID)
			}
			if len(op.Tags) != 1 || op.Tags[0] != "record" {
				t.Errorf("tags = %v, want [record]", op.Tags)
			}
			for _, s := range o.wantStatuses {
				if op.Responses.Status(mustAtoi(t, s)) == nil {
					t.Errorf("%s の %s のレスポンスが無い", o.operationID, s)
				}
			}
			// 端末 ID / セッション ID をヘッダで必須に取ること(ADR-0209 §2)。
			var hasDevice, hasSession bool
			for _, p := range op.Parameters {
				if p.Value == nil {
					continue
				}
				if p.Value.In == "header" && p.Value.Name == "X-Device-Id" && p.Value.Required {
					hasDevice = true
				}
				if p.Value.In == "header" && p.Value.Name == "X-Session-Id" && p.Value.Required {
					hasSession = true
				}
				// 端末 ID をクエリ・ボディ・パスで受け取らない(§6-4)。
				if p.Value.In != "header" && isDeviceIDName(p.Value.Name) {
					t.Errorf("%s が %s で %q を受け取っている(ADR-0209 §6-4)", o.operationID, p.Value.In, p.Value.Name)
				}
			}
			if !hasDevice || !hasSession {
				t.Errorf("%s に必須ヘッダが足りない(X-Device-Id=%v X-Session-Id=%v)", o.operationID, hasDevice, hasSession)
			}
			// 503 の description に store_unavailable を明記していること(ADR-0209 §5.3)。
			if r := op.Responses.Status(http.StatusServiceUnavailable); r != nil && r.Value != nil && r.Value.Description != nil {
				if !strings.Contains(*r.Value.Description, "store_unavailable") {
					t.Errorf("%s の 503 の description に store_unavailable が書かれていない: %q", o.operationID, *r.Value.Description)
				}
			}
		})
	}
}

func isDeviceIDName(name string) bool {
	switch name {
	case "deviceId", "device_id", "deviceID", "DeviceId":
		return true
	}
	return false
}

func mustAtoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("ステータスの指定が数値でない: %q", s)
	}
	return n
}

// 契約そのものが OpenAPI 3.0 として妥当であること(record を足した差分で壊していないこと)。
func TestContractDocumentIsValid(t *testing.T) {
	doc, _ := loadContract(t)
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("api/openapi.yaml が OpenAPI として不正: %v", err)
	}
}
