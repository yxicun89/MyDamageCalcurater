package httpapi

// 契約テスト(test-strategy.md L4)。team-svc の応答を api/openapi.yaml に照らして検証する。
// record-svc / calc-svc / pokedex-svc の contract_test.go と同じ形で、仕様は生成物に埋め込まれたもの
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

// assertMatchesContract は1往復を契約に照らす。checkRequest が false のときは「リクエスト自体が
// 意図的に契約の範囲外」という意味で、その前提(本当にスキーマ違反であること)も併せて確かめる
// (record-svc の同名ヘルパと同じ。critic 指摘 R-6 の対応を踏襲)。
func assertMatchesContract(t *testing.T, method, path string, hdr http.Header, reqBody []byte, rec *httptest.ResponseRecorder, checkRequest bool) {
	t.Helper()
	_, router := loadContract(t)

	req := httptest.NewRequest(method, path, bytes.NewReader(reqBody))
	if len(reqBody) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
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
		t.Errorf("checkRequest=false のケース(%s %s)が実はスキーマ違反ではない。"+
			"schema の制約が緩んでいないか確認すること", method, path)
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
func TestTeamResponsesMatchContract(t *testing.T) {
	seededID := fakeTeamID(1) // seed(deviceA, 1, ...) が最初に作る ID
	okBody := mustJSON(teamJSON("契約テスト", memberJSON()))
	longName := mustJSON(teamJSON(strings.Repeat("あ", 51)))

	tests := []struct {
		name         string
		setup        func(*fakeStore)
		method       string
		path         string
		reqBdy       []byte
		checkRequest bool // false: リクエスト自体が意図的に契約の範囲外(assertMatchesContract 参照)
	}{
		{"一覧 200", func(f *fakeStore) { f.seed(deviceA, 2, 2) }, http.MethodGet, pathTeams, nil, true},
		{"一覧 200(空)", func(*fakeStore) {}, http.MethodGet, pathTeams, nil, true},
		{"一覧 503", func(f *fakeStore) { f.unavailable = true }, http.MethodGet, pathTeams, nil, true},
		{"作成 201", func(*fakeStore) {}, http.MethodPost, pathTeams, okBody, true},
		{"作成 400(名前が長すぎる)", func(*fakeStore) {}, http.MethodPost, pathTeams, longName, false},
		{"作成 503", func(f *fakeStore) { f.unavailable = true }, http.MethodPost, pathTeams, okBody, true},
		{"取得 200", func(f *fakeStore) { f.seed(deviceA, 1, 2) }, http.MethodGet, pathTeams + "/" + seededID, nil, true},
		{"取得 404", func(*fakeStore) {}, http.MethodGet, pathTeams + "/" + seededID, nil, true},
		{"更新 200", func(f *fakeStore) { f.seed(deviceA, 1, 2) }, http.MethodPut, pathTeams + "/" + seededID, okBody, true},
		{"更新 404", func(*fakeStore) {}, http.MethodPut, pathTeams + "/" + seededID, okBody, true},
		{"削除 204", func(f *fakeStore) { f.seed(deviceA, 1, 2) }, http.MethodDelete, pathTeams + "/" + seededID, nil, true},
		{"削除 404", func(*fakeStore) {}, http.MethodDelete, pathTeams + "/" + seededID, nil, true},
		{"全削除 200 completed", func(f *fakeStore) { f.seed(deviceA, 1, 2) }, http.MethodDelete, pathDeviceData, nil, true},
		{"全削除 200 partial", func(f *fakeStore) { f.purgeLimit = 1; f.seed(deviceA, 1, 2) }, http.MethodDelete, pathDeviceData, nil, true},
		{"全削除 503", func(f *fakeStore) { f.unavailable = true }, http.MethodDelete, pathDeviceData, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newFakeStore()
			tt.setup(st)
			rec := serve(t, NewHandler(st), tt.method, tt.path, headers(deviceA), tt.reqBdy)
			assertMatchesContract(t, tt.method, tt.path, headers(deviceA), tt.reqBdy, rec, tt.checkRequest)
		})
	}
}

// AC-C2 / ADR-0209 §5.3・§10・ADR-0213: 契約そのものに team の要素が揃っていること。
func TestContractHasTeamOperations(t *testing.T) {
	doc, _ := loadContract(t)

	t.Run("team タグがある", func(t *testing.T) {
		for _, tag := range doc.Tags {
			if tag.Name == "team" {
				return
			}
		}
		t.Error("top-level の tags に team が無い(ADR-0209 §5.3 移植チェックリスト)")
	})

	ops := []struct {
		path, method, operationID string
		wantStatuses              []string
	}{
		{"/api/team/teams", http.MethodGet, "listTeams", []string{"200", "400", "503"}},
		{"/api/team/teams", http.MethodPost, "createTeam", []string{"201", "400", "500", "503"}},
		{"/api/team/teams/{teamId}", http.MethodGet, "getTeam", []string{"200", "400", "404", "503"}},
		{"/api/team/teams/{teamId}", http.MethodPut, "updateTeam", []string{"200", "400", "404", "500", "503"}},
		{"/api/team/teams/{teamId}", http.MethodDelete, "deleteTeam", []string{"204", "400", "404", "503"}},
		{"/api/team/device-data", http.MethodDelete, "deleteTeamDeviceData", []string{"200", "400", "500", "503"}},
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
			// 埋め込まれた仕様の operationId は Go のメソッド名(PascalCase)に正規化される
			// (record-svc の contract_test.go と同じ理由で EqualFold で比べる)。
			if !strings.EqualFold(op.OperationID, o.operationID) {
				t.Errorf("operationId = %q, want %q(大文字小文字を無視して比較)", op.OperationID, o.operationID)
			}
			if len(op.Tags) != 1 || op.Tags[0] != "team" {
				t.Errorf("tags = %v, want [team]", op.Tags)
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

// ADR-0209 §6-2: /api/team/* がパスで受けるリソース ID は teamId だけ(端末 ID をパスに持たない)。
// 増えたらこのテストが落ちるので、そのとき AC-D2 の実テストも一緒に足すこと。
func TestTeamPathParametersAreTeamIDOnly(t *testing.T) {
	doc, _ := loadContract(t)
	for path, item := range doc.Paths.Map() {
		if !strings.HasPrefix(path, "/api/team/") {
			continue
		}
		for method, op := range item.Operations() {
			for _, p := range op.Parameters {
				if p.Value == nil || p.Value.In != "path" {
					continue
				}
				if p.Value.Name != "teamId" {
					t.Errorf("%s %s にパスパラメータ %q がある。AC-D2(他端末のリソース ID は 404 not_found)の"+
						"テストを追加してからこのテストを更新すること", method, path, p.Value.Name)
				}
			}
		}
	}
}

// ADR-0213 §3: 契約の TeamMember は「マスタの ID をそのまま運ぶ」形であること
// (名前・タイプ・種族値などのマスタの中身を team の契約に持ち込まない。CLAUDE.md 絶対ルール4)。
func TestTeamMemberCarriesIDsOnly(t *testing.T) {
	doc, _ := loadContract(t)
	ref := doc.Components.Schemas["TeamMember"]
	if ref == nil || ref.Value == nil {
		t.Fatal("TeamMember が契約に無い")
	}
	forbidden := []string{"nameJa", "types", "baseStats", "power", "category", "learnset"}
	for _, name := range forbidden {
		if _, ok := ref.Value.Properties[name]; ok {
			t.Errorf("TeamMember が %q を持っている(マスタの中身は pokedex-svc の担当。ADR-0213 §3)", name)
		}
	}
	for _, required := range []string{"speciesKey", "natureId", "sp"} {
		if _, ok := ref.Value.Properties[required]; !ok {
			t.Errorf("TeamMember に %q が無い", required)
		}
	}
	if m := ref.Value.Properties["moveIds"]; m == nil || m.Value == nil {
		t.Error("TeamMember に moveIds が無い")
	} else {
		if m.Value.MaxItems == nil || *m.Value.MaxItems != 4 {
			t.Errorf("moveIds の maxItems = %v, want 4(requirements.md §2)", m.Value.MaxItems)
		}
		if !m.Value.UniqueItems {
			t.Error("moveIds の uniqueItems が false(同じ技の重複を契約で禁じる)")
		}
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

// 契約そのものが OpenAPI 3.0 として妥当であること(team を足した差分で壊していないこと)。
func TestContractDocumentIsValid(t *testing.T) {
	doc, _ := loadContract(t)
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("api/openapi.yaml が OpenAPI として不正: %v", err)
	}
}
