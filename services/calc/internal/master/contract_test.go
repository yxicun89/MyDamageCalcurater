package master

// 契約テスト(test-strategy.md L4。ADR-0204)。calc-svc が受け取るマスタ一式(例のファイル・偽の上流の本文)が
// api/openapi.yaml の GET /internal/pokedex/master(getMasterExport)の 200 = MasterExport に準拠することを、
// kin-openapi で確かめる。pokedex-svc(P2-3)が同じ契約で実装されれば、ここで照らした形がそのまま届く。

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/legacy"

	"example.com/pokecalc/services/internal/api"
)

// validateMasterExportResponse は GET /internal/pokedex/master の応答(status・本文)を契約に照らす。
// リクエスト(ヘッダ無し)も契約に照らす: 内部 API は端末ID/セッションID を要らない。
func validateMasterExportResponse(t *testing.T, status int, body []byte) error {
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
	req := httptest.NewRequest(http.MethodGet, masterExportPath, nil)
	route, params, err := router.FindRoute(req)
	if err != nil {
		t.Fatalf("契約に GET %s が無い: %v", masterExportPath, err)
	}
	// 生成物に埋め込まれた仕様では operationId の先頭が大文字になる(oapi-codegen の正規化)ので大小を区別しない。
	if !strings.EqualFold(route.Operation.OperationID, "getMasterExport") {
		t.Fatalf("operationId = %q, want getMasterExport", route.Operation.OperationID)
	}
	in := &openapi3filter.RequestValidationInput{
		Request: req, PathParams: params, Route: route,
		Options: &openapi3filter.Options{IncludeResponseStatus: true, MultiError: true},
	}
	if err := openapi3filter.ValidateRequest(context.Background(), in); err != nil {
		t.Fatalf("ヘッダ無しのリクエストが契約に合わない(内部 API は端末ID/セッションID を要らない): %v", err)
	}
	header := http.Header{}
	header.Set("Content-Type", "application/json")
	return openapi3filter.ValidateResponse(context.Background(), &openapi3filter.ResponseValidationInput{
		RequestValidationInput: in, Status: status, Header: header,
		Body: io.NopCloser(bytes.NewReader(body)), Options: in.Options,
	})
}

// AC-C1: 例のファイル(= 偽の上流 pokedex-svc が返す本文。source_test.go)は MasterExport に準拠する。
func TestExampleExportMatchesContract(t *testing.T) {
	if err := validateMasterExportResponse(t, http.StatusOK, readExample(t)); err != nil {
		t.Fatalf("例のファイルが契約(MasterExport)に合わない: %v", err)
	}
}

// AC-C1: HTTPSource が上流から受け取る本文(偽の上流が返す 200・503)が契約どおりであることを、
// 上流の応答をそのまま捕まえて照らす(偽物が契約からずれないため)。
func TestHTTPSourceFixturesMatchContract(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   []byte
	}{
		{"200 MasterExport", http.StatusOK, readExample(t)},
		{"503 master_unavailable", http.StatusServiceUnavailable, []byte(`{"code":"master_unavailable","message":"マスタが未投入"}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var captured []byte
			_, srv := newFakePokedex(t, func(w http.ResponseWriter, r *http.Request) {
				captured = tt.body
				respondJSON(tt.status, tt.body)(w, r)
			})
			src, err := NewHTTPSource(srv.URL, fetchTimeout)
			if err != nil {
				t.Fatalf("NewHTTPSource = %v", err)
			}
			_, _ = src.Fetch(context.Background())
			if captured == nil {
				t.Fatal("HTTPSource が上流を呼ばなかった")
			}
			if err := validateMasterExportResponse(t, tt.status, captured); err != nil {
				t.Fatalf("上流の応答が契約に合わない: %v", err)
			}
		})
	}
}

// 検証ヘルパーが空振りしないこと: 必須の欠落・列挙外の値・語彙外の code は落ちる。例のファイル
// (type2・effect・baseSpeciesKey・requiredItemId・性格の plus/minus に null を含む)は通る。
func TestMasterExportContractIsNotVacuous(t *testing.T) {
	example := string(readExample(t))
	tests := []struct {
		name   string
		status int
		body   string
		wantOK bool
	}{
		{"例のファイル", http.StatusOK, example, true},
		{"natures の欠落", http.StatusOK, strings.Replace(example, `"natures": [`, `"natureZ": [`, 1), false},
		{"schemaVersion が 2", http.StatusOK, strings.Replace(example, `"schemaVersion": 1`, `"schemaVersion": 2`, 1), false},
		{"dataVersion が空", http.StatusOK, strings.Replace(example, `"dataVersion": "example-1"`, `"dataVersion": ""`, 1), false},
		{"相性の code が 3", http.StatusOK, strings.Replace(example, `"code": 4}`, `"code": 3}`, 1), false},
		{"列挙外のタイプ", http.StatusOK, strings.Replace(example, `"type1": "normal"`, `"type1": "cosmic"`, 1), false},
		{"種族の showdownId の欠落", http.StatusOK, strings.Replace(example, `"showdownId": "testmon", `, ``, 1), false},
		{"効果が配列(object|null 以外)", http.StatusOK, strings.Replace(example, `{"DamageMod": 5324}`, `[5324]`, 1), false},
		{"503 の Error(master_unavailable)", http.StatusServiceUnavailable, `{"code":"master_unavailable","message":"未投入"}`, true},
		{"503 の語彙外の code", http.StatusServiceUnavailable, `{"code":"db_down","message":"未投入"}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.body == example && !tt.wantOK {
				t.Fatal("置換が効いていない(テストの前提が崩れた)")
			}
			err := validateMasterExportResponse(t, tt.status, []byte(tt.body))
			if tt.wantOK && err != nil {
				t.Errorf("契約に合うはずが落ちた: %v", err)
			}
			if !tt.wantOK && err == nil {
				t.Error("契約に合わないはずが通った(検証が空振りしている)")
			}
		})
	}
}
