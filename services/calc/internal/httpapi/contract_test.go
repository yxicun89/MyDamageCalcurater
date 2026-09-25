package httpapi

// 契約テスト(test-strategy.md L4 の先取り。P3-3 で gateway 経由のものを足す)。
// レスポンス(と成功ケースのリクエスト)を api/openapi.yaml に照らして kin-openapi で検証する。
// 仕様は生成物に埋め込まれたもの(api.GetSwagger。make gen で openapi.yaml から作られる)を使う。

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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
		// servers の "/" はホスト名の照合を持ち込むだけなので外す(パスだけで引く)。
		contractDoc.Servers = nil
		contractRouter, contractErr = legacy.NewRouter(contractDoc)
	})
	if contractErr != nil {
		t.Fatalf("契約(api/openapi.yaml)を読めない: %v", contractErr)
	}
	return contractDoc, contractRouter
}

// contractRequest は検証用のリクエストを作り、契約上のルートを引く。契約に無いルートは ok=false。
func contractRequest(t *testing.T, method, path string, header http.Header, body []byte) (*openapi3filter.RequestValidationInput, bool) {
	t.Helper()
	_, router := loadContract(t)
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	route, params, err := router.FindRoute(req)
	if err != nil {
		return nil, false
	}
	return &openapi3filter.RequestValidationInput{
		Request: req, PathParams: params, Route: route,
		Options: &openapi3filter.Options{IncludeResponseStatus: true, MultiError: true},
	}, true
}

// validateAgainstContract はレスポンスを契約に照らす。契約に無いルートは errNoRoute。
func validateAgainstContract(t *testing.T, method, path string, header http.Header, reqBody []byte,
	status int, respHeader http.Header, respBody []byte, checkRequest bool) error {
	t.Helper()
	in, ok := contractRequest(t, method, path, header, reqBody)
	if !ok {
		return errNoRoute
	}
	if checkRequest {
		if err := openapi3filter.ValidateRequest(context.Background(), in); err != nil {
			return &contractError{what: "リクエスト", err: err}
		}
	}
	out := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: in,
		Status:                 status,
		Header:                 respHeader,
		Body:                   io.NopCloser(bytes.NewReader(respBody)),
		Options:                in.Options,
	}
	if err := openapi3filter.ValidateResponse(context.Background(), out); err != nil {
		return &contractError{what: "レスポンス", err: err}
	}
	return nil
}

type contractError struct {
	what string
	err  error
}

func (e *contractError) Error() string { return e.what + "が契約に合わない: " + e.err.Error() }

type noRouteError struct{}

func (noRouteError) Error() string { return "契約に無いルート" }

var errNoRoute error = noRouteError{}

// assertContract は rec が契約どおりであることを確かめる。契約に無いルート(/healthz・未知のルート)は
// 呼び出し側が個別に見るので、ここでは失敗にしない。
func assertContract(t *testing.T, method, path string, header http.Header, reqBody []byte,
	rec *httptest.ResponseRecorder, checkRequest bool) {
	t.Helper()
	err := validateAgainstContract(t, method, path, header, reqBody, rec.Code, rec.Header(), rec.Body.Bytes(), checkRequest)
	if err != nil && err != errNoRoute {
		t.Errorf("%v\nstatus=%d body=%s", err, rec.Code, rec.Body.String())
	}
}

// 契約そのものが OpenAPI 3.0 として妥当であること(例・既定値の型の誤りなどを早く見つける)。
func TestContractDocumentIsValid(t *testing.T) {
	doc, _ := loadContract(t)
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("api/openapi.yaml が OpenAPI として不正: %v", err)
	}
}

// 検証ヘルパーが空振りしないこと: 契約どおりの本文は通り、必須の欠けた本文・語彙外の code は落ちる。
// null を許す性格補正(NatureModifier の nullable + allOf)が kin-openapi で通ることもここで固定する。
func TestContractHelperIsNotVacuous(t *testing.T) {
	const calcOK = `{"rolls":[1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,2],"minDamage":1,"maxDamage":2,` +
		`"minPercent":0.5,"maxPercent":1.1,"defenderHP":180,"effectiveness":1,"stab":false,"category":"physical",` +
		`"ko":{"hits":90,"guaranteed":false,"chancePercent":12.5,"displayChancePercent":12.5},"unsupported":[]}`
	bulkOK := `{"defenderSpeciesKey":"9002-000","rows":[{"preset":"none","presetLabel":"無振り","itemId":null,` +
		`"defender":{"sp":{"hp":0,"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"nature":{"plus":null,"minus":null},"natureId":null,` +
		`"stats":{"hp":170,"atk":80,"def":110,"spa":90,"spd":105,"spe":70}},"result":` + calcOK + `}]}`
	reverseOK := `{"side":"defender","stat":"def","assumedHpSp":32,"exactCount":1,"candidates":[` +
		`{"natureClass":"plus","nature":{"plus":"def","minus":"atk"},"natureId":"test-def-up","itemId":null,` +
		`"ranges":[{"min":3,"max":5}],"spCount":3,"exact":true,"mismatch":0,"support":7,"minPercent":40.2,"maxPercent":47.8,"unsupported":[]}]}`

	tests := []struct {
		name    string
		path    string
		status  int
		body    string
		wantErr bool
	}{
		{"calc の妥当な結果", "/api/calc", 200, calcOK, false},
		{"calc の category 欠落", "/api/calc", 200, strings.Replace(calcOK, `"category":"physical",`, "", 1), true},
		{"calc の rolls が 15 個", "/api/calc", 200, strings.Replace(calcOK, `[1,1,`, `[1,`, 1), true},
		{"bulk の妥当な結果(null の性格補正)", "/api/calc/bulk", 200, bulkOK, false},
		{"bulk の defender 欠落", "/api/calc/bulk", 200, strings.Replace(bulkOK,
			`"defender":{"sp":{"hp":0,"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"nature":{"plus":null,"minus":null},"natureId":null,`+
				`"stats":{"hp":170,"atk":80,"def":110,"spa":90,"spd":105,"spe":70}},`, "", 1), true},
		{"bulk の性格補正に未知のキー", "/api/calc/bulk", 200, strings.Replace(bulkOK, `"plus":null`, `"plus":"luck"`, 1), true},
		{"reverse の妥当な結果", "/api/calc/reverse", 200, reverseOK, false},
		{"reverse の ranges が空", "/api/calc/reverse", 200, strings.Replace(reverseOK, `[{"min":3,"max":5}]`, `[]`, 1), true},
		{"エラーの妥当な本文", "/api/calc", 400, `{"code":"invalid_json","message":"壊れている"}`, false},
		{"エラーの語彙外の code", "/api/calc", 400, `{"code":"invalid_request","message":"旧語彙"}`, true},
		{"503 のエラー", "/api/calc/bulk", 503, `{"code":"master_unavailable","message":"マスタを参照できない"}`, false},
		{"500 のエラー", "/api/calc/reverse", 500, `{"code":"internal","message":"内部エラー"}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			respHeader := http.Header{"Content-Type": []string{"application/json"}}
			err := validateAgainstContract(t, http.MethodPost, tt.path, validHeaders(), []byte(`{}`),
				tt.status, respHeader, []byte(tt.body), false)
			if err == errNoRoute {
				t.Fatalf("契約に %s が無い", tt.path)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("検証エラー = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
