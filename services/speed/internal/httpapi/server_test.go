package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"example.com/pokecalc/services/speed/internal/api"
	"example.com/pokecalc/services/speed/internal/speed"
)

const pokemonPath = "/api/speed/v1/pokemon"

// fakeProvider は順不同の Roster を返す(API が pokemonId の昇順に並べることを確かめるため)。
type fakeProvider struct {
	roster speed.Roster
	err    error
}

func (f fakeProvider) Roster() (speed.Roster, error) {
	return f.roster, f.err
}

func unorderedRoster() speed.Roster {
	return speed.Roster{RegulationID: "example", Pokemon: []speed.Pokemon{
		{PokemonID: "9003-000", NameJa: "テストサンバンメ", Types: []string{"rock", "ground"}, BaseSpeed: 45},
		{PokemonID: "9001-000", NameJa: "テストカソウドリ", Types: []string{"fire", "flying"}, BaseSpeed: 100},
		{PokemonID: "9010-000", NameJa: "テストジュウバンメ", Types: []string{"ghost"}, BaseSpeed: 81},
		{PokemonID: "9002-000", NameJa: "テストニバンメ", Types: []string{"water"}, BaseSpeed: 81},
	}}
}

func newRequest(path string, headers map[string]string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	return request
}

// testDeviceID / testSessionID は正準形 8-4-4-4-12 の UUID(ADR-0606 §1。gateway と同じ検証を通る値)。
const (
	testDeviceID  = "11111111-1111-1111-1111-111111111111"
	testSessionID = "22222222-2222-2222-2222-222222222222"
)

var validHeaders = map[string]string{"X-Device-Id": testDeviceID, "X-Session-Id": testSessionID}

// headerCase はヘッダー検証の表の1行(ADR-0606 §2)。3つの API で同じ表を使う。
type headerCase struct {
	name    string
	headers map[string]string
	want    api.ErrorCode
}

// headerRejectCases は 400 になるヘッダーの組み合わせ(ADR-0606 §2):
// 欠落・空は missing_header、UUID でない値は invalid_header、欠落と不正が同時なら missing_header。
// 空白だけの値は gateway と同じく「空ではない・UUID でない」ので invalid_header(ADR-0606 §1 の一字一句同じ判定)。
// 同名ヘッダーの重複は map で表せないので、各ファイルの重複テストで別に確かめる。
var headerRejectCases = []headerCase{
	{"両方欠落", nil, api.MissingHeader},
	{"X-Device-Id 欠落", map[string]string{"X-Session-Id": testSessionID}, api.MissingHeader},
	{"X-Session-Id 欠落", map[string]string{"X-Device-Id": testDeviceID}, api.MissingHeader},
	{"X-Device-Id 空", map[string]string{"X-Device-Id": "", "X-Session-Id": testSessionID}, api.MissingHeader},
	{"X-Session-Id 空", map[string]string{"X-Device-Id": testDeviceID, "X-Session-Id": ""}, api.MissingHeader},
	{"X-Device-Id 空白だけ", map[string]string{"X-Device-Id": "  ", "X-Session-Id": testSessionID}, api.InvalidHeader},
	{"X-Session-Id 空白だけ", map[string]string{"X-Device-Id": testDeviceID, "X-Session-Id": "  "}, api.InvalidHeader},
	{"X-Device-Id 欠落と X-Session-Id 不正が同時", map[string]string{"X-Session-Id": "not-a-uuid"}, api.MissingHeader},
	{"X-Device-Id 空と X-Session-Id 不正が同時", map[string]string{"X-Device-Id": "", "X-Session-Id": "not-a-uuid"}, api.MissingHeader},
	{"X-Device-Id が UUID でない", map[string]string{"X-Device-Id": "not-a-uuid", "X-Session-Id": testSessionID}, api.InvalidHeader},
	{"X-Session-Id が UUID でない(旧フィクスチャの値)", map[string]string{"X-Device-Id": testDeviceID, "X-Session-Id": "test-session"}, api.InvalidHeader},
	{"X-Device-Id がハイフン無し32桁", map[string]string{"X-Device-Id": "11111111111111111111111111111111", "X-Session-Id": testSessionID}, api.InvalidHeader},
	{"X-Device-Id が波括弧つき", map[string]string{"X-Device-Id": "{" + testDeviceID + "}", "X-Session-Id": testSessionID}, api.InvalidHeader},
	{"X-Session-Id が urn:uuid: つき", map[string]string{"X-Device-Id": testDeviceID, "X-Session-Id": "urn:uuid:" + testSessionID}, api.InvalidHeader},
	{"X-Device-Id が1桁足りない", map[string]string{"X-Device-Id": testDeviceID[:35], "X-Session-Id": testSessionID}, api.InvalidHeader},
	{"X-Device-Id に16進でない文字", map[string]string{"X-Device-Id": "1111111g-1111-1111-1111-111111111111", "X-Session-Id": testSessionID}, api.InvalidHeader},
	{"両方が UUID でない", map[string]string{"X-Device-Id": "test-device", "X-Session-Id": "test-session"}, api.InvalidHeader},
}

// headerAcceptIDs は通るべき正準形 UUID(大文字小文字・版を問わない。gateway の TestHeaderValidationAccepts と同じ観点)。
var headerAcceptIDs = []string{
	testDeviceID,
	"ABCDEF01-2345-4678-9ABC-DEF012345678",
	"abcDEF01-2345-4678-9abc-DEF012345678",
	"00000000-0000-0000-0000-000000000000",
	"01890a5d-ac96-774b-bcce-b302099a8057",
}

func serve(deps Dependencies, request *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	New(deps).ServeHTTP(recorder, request)
	return recorder
}

func decodeError(t *testing.T, recorder *httptest.ResponseRecorder) api.Error {
	t.Helper()
	var body api.Error
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not an Error: %v; body=%s", err, recorder.Body.String())
	}
	return body
}

func TestHealth(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"/healthz", "/api/speed/healthz"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			// read model が未設定でもヘルスは 200(ADR-0600 §4)。ヘッダーも不要。
			recorder := serve(Dependencies{}, newRequest(path, nil))
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
			}
			if got := strings.TrimSpace(recorder.Body.String()); got != `{"status":"ok"}` {
				t.Errorf("body = %s, want {\"status\":\"ok\"}", got)
			}
		})
	}
}

func TestListPokemonRequiresRequestContext(t *testing.T) {
	t.Parallel()

	for _, tt := range headerRejectCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// ヘッダーの検査は read model の有無より先(provider があっても 400)。
			recorder := serve(Dependencies{Pokemon: fakeProvider{roster: unorderedRoster()}}, newRequest(pokemonPath, tt.headers))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
			if body := decodeError(t, recorder); body.Code != tt.want {
				t.Errorf("code = %q, want %q", body.Code, tt.want)
			}
		})
	}
}

// TestListPokemonRejectsDuplicateHeaders: 同名ヘッダーの重複は(値が同じでも別でも)400 invalid_header
// (ADR-0606 §2。gateway の headerStatus と同じ)。旧契約では生成コードの 400 を invalid_request にそろえていた。
func TestListPokemonRejectsDuplicateHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(*http.Request)
	}{
		{"X-Device-Id の重複(別の値)", func(r *http.Request) {
			r.Header.Add("X-Device-Id", testDeviceID)
			r.Header.Add("X-Device-Id", "33333333-3333-3333-3333-333333333333")
			r.Header.Set("X-Session-Id", testSessionID)
		}},
		{"X-Session-Id の重複(同じ値)", func(r *http.Request) {
			r.Header.Set("X-Device-Id", testDeviceID)
			r.Header.Add("X-Session-Id", testSessionID)
			r.Header.Add("X-Session-Id", testSessionID)
		}},
		{"X-Device-Id の重複(UUID でない値)", func(r *http.Request) {
			r.Header.Add("X-Device-Id", "first-device")
			r.Header.Add("X-Device-Id", "second-device")
			r.Header.Set("X-Session-Id", testSessionID)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			request := newRequest(pokemonPath, nil)
			tt.setup(request)
			recorder := serve(Dependencies{Pokemon: fakeProvider{roster: unorderedRoster()}}, request)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
			if body := decodeError(t, recorder); body.Code != api.InvalidHeader {
				t.Errorf("code = %q, want %q", body.Code, api.InvalidHeader)
			}
		})
	}
}

// TestListPokemonAcceptsCanonicalUUIDs: 正準形の UUID は大文字小文字・版を問わず通る(ADR-0606 §1)。
func TestListPokemonAcceptsCanonicalUUIDs(t *testing.T) {
	t.Parallel()

	for _, id := range headerAcceptIDs {
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			headers := map[string]string{"X-Device-Id": id, "X-Session-Id": id}
			recorder := serve(Dependencies{Pokemon: fakeProvider{roster: unorderedRoster()}}, newRequest(pokemonPath, headers))
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
			}
		})
	}
}

func TestListPokemonWithoutReadModel(t *testing.T) {
	t.Parallel()

	recorder := serve(Dependencies{}, newRequest(pokemonPath, validHeaders))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusServiceUnavailable, recorder.Body.String())
	}
	if body := decodeError(t, recorder); body.Code != api.MasterUnavailable {
		t.Errorf("code = %q, want %q", body.Code, api.MasterUnavailable)
	}
}

func TestListPokemonReturnsSortedByPokemonID(t *testing.T) {
	t.Parallel()

	recorder := serve(Dependencies{Pokemon: fakeProvider{roster: unorderedRoster()}}, newRequest(pokemonPath, validHeaders))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	var body api.PokemonListResponse
	decoder := json.NewDecoder(strings.NewReader(recorder.Body.String()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		t.Fatalf("body is not a PokemonListResponse: %v; body=%s", err, recorder.Body.String())
	}
	want := api.PokemonListResponse{RegulationId: "example", Pokemon: []api.SpeedPokemon{
		{PokemonId: "9001-000", NameJa: "テストカソウドリ", Types: []string{"fire", "flying"}, BaseSpeed: 100},
		{PokemonId: "9002-000", NameJa: "テストニバンメ", Types: []string{"water"}, BaseSpeed: 81},
		{PokemonId: "9003-000", NameJa: "テストサンバンメ", Types: []string{"rock", "ground"}, BaseSpeed: 45},
		{PokemonId: "9010-000", NameJa: "テストジュウバンメ", Types: []string{"ghost"}, BaseSpeed: 81},
	}}
	if !reflect.DeepEqual(body, want) {
		t.Errorf("body = %+v, want %+v", body, want)
	}
}

func TestListPokemonHidesProviderError(t *testing.T) {
	t.Parallel()

	secret := "open /secret/path/pokemon.json: permission denied"
	recorder := serve(Dependencies{Pokemon: fakeProvider{err: errors.New(secret)}}, newRequest(pokemonPath, validHeaders))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
	body := decodeError(t, recorder)
	if body.Code != api.InternalError {
		t.Errorf("code = %q, want %q", body.Code, api.InternalError)
	}
	// 想定外のエラーは固定文言。内部の文言を返さない(ADR-0600 §5)。
	if body.Message != "internal error" {
		t.Errorf("message = %q, want the fixed string \"internal error\"", body.Message)
	}
	if strings.Contains(recorder.Body.String(), "secret") {
		t.Errorf("body leaks the internal error: %s", recorder.Body.String())
	}
}
