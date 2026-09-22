package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"example.com/pokecalc/services/speed/internal/api"
	"example.com/pokecalc/services/speed/internal/speed"
)

const positionPath = "/api/speed/v1/position"

func newPositionRequest(headers map[string]string, body string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, positionPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	return request
}

func decodePosition(t *testing.T, raw string) api.PositionResponse {
	t.Helper()
	var body api.PositionResponse
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		t.Fatalf("body is not a PositionResponse: %v; body=%s", err, raw)
	}
	return body
}

// 例の read model(testdata/pokemon.example.json)の全 48 行(8 体 × 6 プリセット)を手計算した値
// (ADR-0600 §3 の式。table_test.go の手計算と同じ)。
//
//	9001 種族値 100: 120 / 152 / 167 / 250 / 250 / 334
//	9002 種族値  81: 101 / 133 / 146 / 219 / 219 / 292
//	9003 種族値  45:  65 /  97 / 106 / 159 / 159 / 212
//	9004 種族値 130: 150 / 182 / 200 / 300 / 300 / 400
//	9005 種族値  81: 9002 と同じ
//	9006 種族値  60:  80 / 112 / 123 / 184 / 184 / 246
//	9007 種族値  30:  50 /  82 /  90 / 135 / 135 / 180
//	9008 種族値 110: 130 / 162 / 178 / 267 / 267 / 356
//
// 段(降順)と、その段までの累積の行数:
//
//	400:1  356:2  334:3  300:5  292:7  267:9  250:11 246:12 219:16 212:17 200:18 184:20
//	182:21 180:22 178:23 167:24 162:25 159:27 152:28 150:29 146:31 135:33 133:35 130:36
//	123:37 120:38 112:39 106:40 101:42  97:43  90:44  82:45  80:46  65:47  50:48
//
// faster は「その値より上の段までの累積」、slower は 48 − faster − len(tie)。
const examplePositionRows = 48

// exampleEntry は testdata の 1 体 1 行分の期待値。
func exampleEntry(pokemonID, nameJa string, types []string, baseSpeed int, preset api.PresetId) api.SpeedTableEntry {
	return api.SpeedTableEntry{PokemonId: pokemonID, NameJa: nameJa, Types: types, BaseSpeed: baseSpeed, Preset: preset}
}

func entry9002(preset api.PresetId) api.SpeedTableEntry {
	return exampleEntry("9002-000", "テストミズハネウオ", []string{"water"}, 81, preset)
}

func entry9005(preset api.PresetId) api.SpeedTableEntry {
	return exampleEntry("9005-000", "テストクサモグリン", []string{"grass", "poison"}, 81, preset)
}

func entry9004(preset api.PresetId) api.SpeedTableEntry {
	return exampleEntry("9004-000", "テストデンキツネビ", []string{"electric"}, 130, preset)
}

// pokemon9004 は mode=preset / custom の応答に付く自分のポケモン(ADR-0602 §4)。
var pokemon9004 = api.SpeedPokemon{PokemonId: "9004-000", NameJa: "テストデンキツネビ", Types: []string{"electric"}, BaseSpeed: 130}

func TestPositionRequiresRequestContext(t *testing.T) {
	t.Parallel()

	validBody := `{"mode":"raw","value":200}`
	tests := []struct {
		name    string
		headers map[string]string
	}{
		{"両方欠落", nil},
		{"X-Device-Id 欠落", map[string]string{"X-Session-Id": "test-session"}},
		{"X-Session-Id 欠落", map[string]string{"X-Device-Id": "test-device"}},
		{"X-Device-Id 空", map[string]string{"X-Device-Id": "", "X-Session-Id": "test-session"}},
		{"X-Session-Id 空", map[string]string{"X-Device-Id": "test-device", "X-Session-Id": ""}},
		{"X-Device-Id 空白だけ", map[string]string{"X-Device-Id": "  ", "X-Session-Id": "test-session"}},
		{"X-Session-Id 空白だけ", map[string]string{"X-Device-Id": "test-device", "X-Session-Id": "  "}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// ヘッダーの検査は body・read model の有無より先。
			for _, deps := range []Dependencies{{Pokemon: fakeProvider{roster: unorderedRoster()}}, {}} {
				recorder := serve(deps, newPositionRequest(tt.headers, validBody))
				if recorder.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
				}
				if body := decodeError(t, recorder); body.Code != api.InvalidRequest {
					t.Errorf("code = %q, want %q", body.Code, api.InvalidRequest)
				}
			}
		})
	}
}

// TestPositionChecksHeadersBeforeBody: ヘッダーと body の両方が不正なら、ヘッダーの 400 を返す
// (ADR-0602 §4 の判定順)。どちらも invalid_request なので文言で区別する。
func TestPositionChecksHeadersBeforeBody(t *testing.T) {
	t.Parallel()

	recorder := serve(Dependencies{Pokemon: exampleProvider(t)}, newPositionRequest(nil, `{"mode":`))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	body := decodeError(t, recorder)
	if body.Code != api.InvalidRequest {
		t.Errorf("code = %q, want %q", body.Code, api.InvalidRequest)
	}
	if !strings.Contains(body.Message, "X-Device-Id") {
		t.Errorf("message = %q, want the header check's message (headers are checked before the body)", body.Message)
	}
}

// TestPositionRejectsOversizedBody: maxPositionBodyBytes(4 KiB)を超える body は 413
// request_too_large(balance の decodeJSONBody と同じ形。ADR-0602 §4)。ちょうど上限の body は
// 413 にならないことも確かめる(上限を縮める回帰を検出するため。balance の
// TestAnalyzeAcceptsBoundaryInputs と同じ考え方)。
func TestPositionRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	oversized := fmt.Sprintf(`{"mode":"raw","value":200,"pokemonId":"%s"}`, strings.Repeat("9", maxPositionBodyBytes))
	for _, deps := range []Dependencies{{Pokemon: exampleProvider(t)}, {}} {
		recorder := serve(deps, newPositionRequest(validHeaders, oversized))
		if recorder.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusRequestEntityTooLarge, recorder.Body.String())
		}
		if body := decodeError(t, recorder); body.Code != api.RequestTooLarge {
			t.Errorf("code = %q, want %q", body.Code, api.RequestTooLarge)
		}
	}
}

// TestPositionAcceptsBodyAtSizeLimit: ちょうど maxPositionBodyBytes の body は 413 にならない
// (末尾の空白で長さを合わせても、JSON としては1つのオブジェクトのまま)。
func TestPositionAcceptsBodyAtSizeLimit(t *testing.T) {
	t.Parallel()

	base := `{"mode":"raw","value":200}`
	exactLimit := base + strings.Repeat(" ", maxPositionBodyBytes-len(base))
	if len(exactLimit) != maxPositionBodyBytes {
		t.Fatalf("fixture length = %d, want %d", len(exactLimit), maxPositionBodyBytes)
	}

	recorder := serve(Dependencies{Pokemon: exampleProvider(t)}, newPositionRequest(validHeaders, exactLimit))
	if recorder.Code == http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want anything but %d (body is exactly at the limit)", recorder.Code, http.StatusRequestEntityTooLarge)
	}
}

// TestPositionRejectsInvalidBody: JSON の形、mode ごとの必須フィールドの過不足、値の範囲は 400 invalid_request
// (ADR-0602 §2・§4)。read model があってもなくても body の検査が先なので、両方で確かめる。
func TestPositionRejectsInvalidBody(t *testing.T) {
	t.Parallel()

	minRaw, maxRaw := speed.RawSpeedRange()
	tests := []struct {
		name string
		body string
	}{
		{"空の body", ``},
		{"JSON として壊れている", `{"mode":"raw",`},
		{"オブジェクトではない(配列)", `[{"mode":"raw","value":200}]`},
		{"オブジェクトではない(文字列)", `"raw"`},
		{"JSON オブジェクトが 2 つ", `{"mode":"raw","value":200}{"mode":"raw","value":200}`},
		{"未知のフィールド", `{"mode":"raw","value":200,"speed":200}`},
		{"mode が無い", `{"pokemonId":"9001-000","preset":"max","scarf":false}`},
		{"mode が空文字", `{"mode":"","pokemonId":"9001-000","preset":"max","scarf":false}`},
		{"未知の mode", `{"mode":"manual","pokemonId":"9001-000","preset":"max","scarf":false}`},
		{"大文字の mode", `{"mode":"PRESET","pokemonId":"9001-000","preset":"max","scarf":false}`},

		// mode=preset は pokemonId・preset・scarf の 3 つちょうど。
		{"preset: pokemonId が無い", `{"mode":"preset","preset":"max","scarf":false}`},
		{"preset: preset が無い", `{"mode":"preset","pokemonId":"9001-000","scarf":false}`},
		{"preset: scarf が無い", `{"mode":"preset","pokemonId":"9001-000","preset":"max"}`},
		{"preset: sp を含む", `{"mode":"preset","pokemonId":"9001-000","preset":"max","scarf":false,"sp":32}`},
		{"preset: nature を含む", `{"mode":"preset","pokemonId":"9001-000","preset":"max","scarf":false,"nature":"plus"}`},
		{"preset: rank を含む", `{"mode":"preset","pokemonId":"9001-000","preset":"max","scarf":false,"rank":1}`},
		{"preset: value を含む", `{"mode":"preset","pokemonId":"9001-000","preset":"max","scarf":false,"value":200}`},
		// 最小の選択はスカーフを含まない 3 つだけ(scarf と二重指定になるため。ADR-0602 §5)。
		{"preset: max-scarf は選べない", `{"mode":"preset","pokemonId":"9001-000","preset":"max-scarf","scarf":false}`},
		{"preset: max-plus1 は選べない", `{"mode":"preset","pokemonId":"9001-000","preset":"max-plus1","scarf":false}`},
		{"preset: max-plus2 は選べない", `{"mode":"preset","pokemonId":"9001-000","preset":"max-plus2","scarf":false}`},
		{"preset: 未知の preset", `{"mode":"preset","pokemonId":"9001-000","preset":"fastest","scarf":false}`},
		{"preset: preset が空文字", `{"mode":"preset","pokemonId":"9001-000","preset":"","scarf":false}`},
		// pokemonId の形式(NNNN-NNN)。read model に無いだけの場合(422)とは区別する(下の
		// TestPositionUnknownPokemon)。
		{"preset: pokemonId の形式が不正", `{"mode":"preset","pokemonId":"zzz","preset":"max","scarf":false}`},

		// mode=custom は pokemonId・sp・nature・rank・scarf の 5 つちょうど。
		{"custom: rank が無い", `{"mode":"custom","pokemonId":"9001-000","sp":32,"nature":"plus","scarf":false}`},
		{"custom: sp が無い", `{"mode":"custom","pokemonId":"9001-000","nature":"plus","rank":0,"scarf":false}`},
		{"custom: nature が無い", `{"mode":"custom","pokemonId":"9001-000","sp":32,"rank":0,"scarf":false}`},
		{"custom: scarf が無い", `{"mode":"custom","pokemonId":"9001-000","sp":32,"nature":"plus","rank":0}`},
		{"custom: pokemonId が無い", `{"mode":"custom","sp":32,"nature":"plus","rank":0,"scarf":false}`},
		{"custom: preset を含む", `{"mode":"custom","pokemonId":"9001-000","sp":32,"nature":"plus","rank":0,"scarf":false,"preset":"max"}`},
		{"custom: value を含む", `{"mode":"custom","pokemonId":"9001-000","sp":32,"nature":"plus","rank":0,"scarf":false,"value":200}`},
		{"custom: sp が上限超え", `{"mode":"custom","pokemonId":"9001-000","sp":33,"nature":"plus","rank":0,"scarf":false}`},
		{"custom: sp が負", `{"mode":"custom","pokemonId":"9001-000","sp":-1,"nature":"plus","rank":0,"scarf":false}`},
		{"custom: rank が上限超え", `{"mode":"custom","pokemonId":"9001-000","sp":32,"nature":"plus","rank":7,"scarf":false}`},
		{"custom: rank が下限未満", `{"mode":"custom","pokemonId":"9001-000","sp":32,"nature":"plus","rank":-7,"scarf":false}`},
		{"custom: 未知の nature", `{"mode":"custom","pokemonId":"9001-000","sp":32,"nature":"fast","rank":0,"scarf":false}`},
		{"custom: nature が空文字", `{"mode":"custom","pokemonId":"9001-000","sp":32,"nature":"","rank":0,"scarf":false}`},
		{"custom: pokemonId の形式が不正", `{"mode":"custom","pokemonId":"zzz","sp":32,"nature":"plus","rank":0,"scarf":false}`},

		// mode=raw は value(必須)と pokemonId(任意)だけ。
		{"raw: value が無い", `{"mode":"raw"}`},
		{"raw: preset を含む", `{"mode":"raw","value":200,"preset":"max"}`},
		{"raw: scarf を含む", `{"mode":"raw","value":200,"scarf":true}`},
		{"raw: sp を含む", `{"mode":"raw","value":200,"sp":32}`},
		{"raw: nature を含む", `{"mode":"raw","value":200,"nature":"plus"}`},
		{"raw: rank を含む", `{"mode":"raw","value":200,"rank":1}`},
		{"raw: value が 0", `{"mode":"raw","value":0}`},
		{"raw: value が負", `{"mode":"raw","value":-1}`},
		// 範囲の端はテスト側でも書き写さず、コアの導出(ADR-0602 §3)から作る。
		{"raw: value が最小未満", fmt.Sprintf(`{"mode":"raw","value":%d}`, minRaw-1)},
		{"raw: value が最大超え", fmt.Sprintf(`{"mode":"raw","value":%d}`, maxRaw+1)},
		{"raw: pokemonId の形式が不正(名前・タイプの解決だけに使う分でも形式は検査する)", `{"mode":"raw","value":200,"pokemonId":"zzz"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// body の検査は read model の有無より先(provider が無くても 503 ではなく 400)。
			for _, deps := range []Dependencies{{Pokemon: exampleProvider(t)}, {}} {
				recorder := serve(deps, newPositionRequest(validHeaders, tt.body))
				if recorder.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
				}
				if body := decodeError(t, recorder); body.Code != api.InvalidRequest {
					t.Errorf("code = %q, want %q", body.Code, api.InvalidRequest)
				}
			}
		})
	}
}

// TestPositionRawAcceptsRangeBoundaries: 範囲の端はそのまま通る(400 にしない)。
func TestPositionRawAcceptsRangeBoundaries(t *testing.T) {
	t.Parallel()

	minRaw, maxRaw := speed.RawSpeedRange()
	for _, value := range []int{minRaw, maxRaw} {
		recorder := serve(Dependencies{Pokemon: exampleProvider(t)}, newPositionRequest(validHeaders, fmt.Sprintf(`{"mode":"raw","value":%d}`, value)))
		if recorder.Code != http.StatusOK {
			t.Fatalf("value %d: status = %d, want %d; body=%s", value, recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if body := decodePosition(t, recorder.Body.String()); body.Speed != value {
			t.Errorf("value %d: speed = %d, want %d", value, body.Speed, value)
		}
	}
}

func TestPositionWithoutReadModel(t *testing.T) {
	t.Parallel()

	bodies := []string{
		`{"mode":"preset","pokemonId":"9004-000","preset":"max","scarf":false}`,
		`{"mode":"custom","pokemonId":"9004-000","sp":32,"nature":"plus","rank":0,"scarf":false}`,
		`{"mode":"raw","value":200}`,
		`{"mode":"raw","pokemonId":"9004-000","value":200}`,
	}
	for _, body := range bodies {
		recorder := serve(Dependencies{}, newPositionRequest(validHeaders, body))
		if recorder.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s: status = %d, want %d; body=%s", body, recorder.Code, http.StatusServiceUnavailable, recorder.Body.String())
		}
		if got := decodeError(t, recorder); got.Code != api.MasterUnavailable {
			t.Errorf("%s: code = %q, want %q", body, got.Code, api.MasterUnavailable)
		}
	}
}

// TestPositionUnknownPokemon: read model に無い pokemonId は 422 unknown_pokemon(ADR-0602 §3)。
// read model 未設定(503)の検査が先なので、ここでは provider を与える。
func TestPositionUnknownPokemon(t *testing.T) {
	t.Parallel()

	bodies := []string{
		`{"mode":"preset","pokemonId":"9999-000","preset":"max","scarf":false}`,
		`{"mode":"custom","pokemonId":"9999-000","sp":32,"nature":"plus","rank":0,"scarf":false}`,
		// raw でも pokemonId を渡したなら解決できないといけない(名前・タイプを返すため)。
		`{"mode":"raw","pokemonId":"9999-000","value":200}`,
	}
	for _, body := range bodies {
		recorder := serve(Dependencies{Pokemon: exampleProvider(t)}, newPositionRequest(validHeaders, body))
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s: status = %d, want %d; body=%s", body, recorder.Code, http.StatusUnprocessableEntity, recorder.Body.String())
		}
		if got := decodeError(t, recorder); got.Code != api.UnknownPokemon {
			t.Errorf("%s: code = %q, want %q", body, got.Code, api.UnknownPokemon)
		}
	}
}

// TestPositionUnknownPokemonAfterReadModel: pokemonId の検査より read model 未設定(503)が先(ADR-0602 §4)。
func TestPositionUnknownPokemonAfterReadModel(t *testing.T) {
	t.Parallel()

	recorder := serve(Dependencies{}, newPositionRequest(validHeaders, `{"mode":"preset","pokemonId":"9999-000","preset":"max","scarf":false}`))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusServiceUnavailable, recorder.Body.String())
	}
	if got := decodeError(t, recorder); got.Code != api.MasterUnavailable {
		t.Errorf("code = %q, want %q", got.Code, api.MasterUnavailable)
	}
}

// TestPositionPresetMode: 最小の選択(3 プリセット × スカーフ on/off)。9004-000(種族値 130)で全 6 通り。
// 期待値はファイル冒頭の手計算の段から数えた faster/slower と、同じ値の段の行。
func TestPositionPresetMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		wantSpeed  int
		wantFaster int
		wantSlower int
		wantTie    []api.SpeedTableEntry
	}{
		{
			// 無振り 150。同速は自分の行 1 つだけ。上は 152 までの累積 28。
			name:       "無振り",
			body:       `{"mode":"preset","pokemonId":"9004-000","preset":"uninvested","scarf":false}`,
			wantSpeed:  150,
			wantFaster: 28,
			wantSlower: 19,
			wantTie:    []api.SpeedTableEntry{entry9004(api.PresetIdUninvested)},
		},
		{
			// 無振り + スカーフ: 150 × 1.5 = 225。表に 225 の段は無い。上は 246 までの累積 12。
			name:       "無振り + スカーフ",
			body:       `{"mode":"preset","pokemonId":"9004-000","preset":"uninvested","scarf":true}`,
			wantSpeed:  225,
			wantFaster: 12,
			wantSlower: 36,
			wantTie:    nil,
		},
		{
			// 準速 182。上は 184 までの累積 20。
			name:       "準速",
			body:       `{"mode":"preset","pokemonId":"9004-000","preset":"neutral-max","scarf":false}`,
			wantSpeed:  182,
			wantFaster: 20,
			wantSlower: 27,
			wantTie:    []api.SpeedTableEntry{entry9004(api.PresetIdNeutralMax)},
		},
		{
			// 準速 + スカーフ: 182 × 1.5 = 273。表に 273 の段は無い。上は 292 までの累積 7。
			name:       "準速 + スカーフ",
			body:       `{"mode":"preset","pokemonId":"9004-000","preset":"neutral-max","scarf":true}`,
			wantSpeed:  273,
			wantFaster: 7,
			wantSlower: 41,
			wantTie:    nil,
		},
		{
			// 最速 200。上は 212 までの累積 17。
			name:       "最速",
			body:       `{"mode":"preset","pokemonId":"9004-000","preset":"max","scarf":false}`,
			wantSpeed:  200,
			wantFaster: 17,
			wantSlower: 30,
			wantTie:    []api.SpeedTableEntry{entry9004(api.PresetIdMax)},
		},
		{
			// 最速 + スカーフ = 表の max-scarf の行そのもの(300)。+1 も同じ値なので同速は 2 行。
			// 上は 334 までの累積 3。
			name:       "最速 + スカーフ(表の max-scarf と同じ入力)",
			body:       `{"mode":"preset","pokemonId":"9004-000","preset":"max","scarf":true}`,
			wantSpeed:  300,
			wantFaster: 3,
			wantSlower: 43,
			wantTie:    []api.SpeedTableEntry{entry9004(api.PresetIdMaxScarf), entry9004(api.PresetIdMaxPlus1)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := serve(Dependencies{Pokemon: exampleProvider(t)}, newPositionRequest(validHeaders, tt.body))
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
			}
			if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			body := decodePosition(t, recorder.Body.String())
			want := api.PositionResponse{
				Speed:   tt.wantSpeed,
				Pokemon: &pokemon9004,
				Faster:  tt.wantFaster,
				Slower:  tt.wantSlower,
				Tie:     tt.wantTie,
			}
			if want.Tie == nil {
				want.Tie = []api.SpeedTableEntry{}
			}
			if !reflect.DeepEqual(body, want) {
				t.Errorf("body = %+v (tie %+v), want %+v (tie %+v)", body, body.Tie, want, want.Tie)
			}
			assertPositionCoversTable(t, body)
		})
	}
}

// TestPositionTieFromExample: 種族値 81 の 9002-000 と 9005-000 は同速になる(ADR-0601 §3 の同速)。
// 9002 の最速 146 の段は 9002 と 9005 の max の 2 行で、pokemonId の昇順。上は 150 までの累積 29。
func TestPositionTieFromExample(t *testing.T) {
	t.Parallel()

	recorder := serve(Dependencies{Pokemon: exampleProvider(t)}, newPositionRequest(validHeaders,
		`{"mode":"preset","pokemonId":"9002-000","preset":"max","scarf":false}`))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	body := decodePosition(t, recorder.Body.String())
	want := api.PositionResponse{
		Speed:   146,
		Pokemon: &api.SpeedPokemon{PokemonId: "9002-000", NameJa: "テストミズハネウオ", Types: []string{"water"}, BaseSpeed: 81},
		Faster:  29,
		Slower:  17,
		Tie:     []api.SpeedTableEntry{entry9002(api.PresetIdMax), entry9005(api.PresetIdMax)},
	}
	if !reflect.DeepEqual(body, want) {
		t.Errorf("body = %+v (tie %+v), want %+v (tie %+v)", body, body.Tie, want, want.Tie)
	}
	assertPositionCoversTable(t, body)
}

// TestPositionCustomMode: 自由入力。値は ADR-0600 §3 の式から手で導いた値。
func TestPositionCustomMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		wantSpeed  int
		wantFaster int
		wantSlower int
		wantTie    []api.SpeedTableEntry
	}{
		{
			// 9004 準速 182 + スカーフを custom で組む: 182 × 1.5 = 273。preset の「準速 + スカーフ」と同じ。
			name:       "準速相当 + スカーフ",
			body:       `{"mode":"custom","pokemonId":"9004-000","sp":32,"nature":"neutral","rank":0,"scarf":true}`,
			wantSpeed:  273,
			wantFaster: 7,
			wantSlower: 41,
			wantTie:    nil,
		},
		{
			// 種族値 130・SP 10・上昇・ランク 0: floor((130+20+10)×1.1) = floor(176) = 176。
			// 176 の段は無い。上は 178 までの累積 23。
			name:       "SP を途中まで振る",
			body:       `{"mode":"custom","pokemonId":"9004-000","sp":10,"nature":"plus","rank":0,"scarf":false}`,
			wantSpeed:  176,
			wantFaster: 23,
			wantSlower: 25,
			wantTie:    nil,
		},
		{
			// 種族値 130・SP 0・下降・ランク -1: floor(150×0.9) = 135、ランク -1 で floor(135×2/3) = 90。
			// 90 の段は 9007 の最速の 1 行。上は 97 までの累積 43。
			name:       "下降の性格とランク -1(同速あり)",
			body:       `{"mode":"custom","pokemonId":"9004-000","sp":0,"nature":"minus","rank":-1,"scarf":false}`,
			wantSpeed:  90,
			wantFaster: 43,
			wantSlower: 4,
			wantTie: []api.SpeedTableEntry{
				exampleEntry("9007-000", "テストハガネカブトン", []string{"steel", "bug"}, 30, api.PresetIdMax),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := serve(Dependencies{Pokemon: exampleProvider(t)}, newPositionRequest(validHeaders, tt.body))
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
			}
			body := decodePosition(t, recorder.Body.String())
			want := api.PositionResponse{
				Speed:   tt.wantSpeed,
				Pokemon: &pokemon9004,
				Faster:  tt.wantFaster,
				Slower:  tt.wantSlower,
				Tie:     tt.wantTie,
			}
			if want.Tie == nil {
				want.Tie = []api.SpeedTableEntry{}
			}
			if !reflect.DeepEqual(body, want) {
				t.Errorf("body = %+v (tie %+v), want %+v (tie %+v)", body, body.Tie, want, want.Tie)
			}
			assertPositionCoversTable(t, body)
		})
	}
}

// TestPositionRawMode: 実数値の直接入力。pokemonId を渡さなければ pokemon フィールドは付かない(ADR-0602 §4)。
func TestPositionRawMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		body        string
		wantSpeed   int
		wantPokemon *api.SpeedPokemon
		wantFaster  int
		wantSlower  int
		wantTie     []api.SpeedTableEntry
	}{
		{
			// 250 の段は 9001 のスカーフと +1 の 2 行。上は 267 までの累積 9。
			name:        "pokemonId なし(同速あり)",
			body:        `{"mode":"raw","value":250}`,
			wantSpeed:   250,
			wantPokemon: nil,
			wantFaster:  9,
			wantSlower:  37,
			wantTie: []api.SpeedTableEntry{
				exampleEntry("9001-000", "テストカソウドリ", []string{"fire", "flying"}, 100, api.PresetIdMaxScarf),
				exampleEntry("9001-000", "テストカソウドリ", []string{"fire", "flying"}, 100, api.PresetIdMaxPlus1),
			},
		},
		{
			// 201 の段は無い。上は 212 までの累積 17。
			name:        "pokemonId なし(同速なし)",
			body:        `{"mode":"raw","value":201}`,
			wantSpeed:   201,
			wantPokemon: nil,
			wantFaster:  17,
			wantSlower:  31,
			wantTie:     nil,
		},
		{
			// pokemonId は名前・タイプの解決だけに使い、種族値 130 は計算に使わない。
			// value 219 がそのまま実数値で、219 の段は 9002・9005 のスカーフと +1 の 4 行。上は 246 までの累積 12。
			name:        "pokemonId あり(種族値は使わない)",
			body:        `{"mode":"raw","pokemonId":"9004-000","value":219}`,
			wantSpeed:   219,
			wantPokemon: &pokemon9004,
			wantFaster:  12,
			wantSlower:  32,
			wantTie: []api.SpeedTableEntry{
				entry9002(api.PresetIdMaxScarf), entry9002(api.PresetIdMaxPlus1),
				entry9005(api.PresetIdMaxScarf), entry9005(api.PresetIdMaxPlus1),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := serve(Dependencies{Pokemon: exampleProvider(t)}, newPositionRequest(validHeaders, tt.body))
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
			}
			raw := recorder.Body.String()
			// pokemonId を渡さなかったら pokemon フィールドそのものを出さない(null でもない)。
			if tt.wantPokemon == nil && strings.Contains(raw, `"pokemon"`) {
				t.Errorf("body has a pokemon field without a pokemonId: %s", raw)
			}
			// 同速なしは空配列で返す(null にしない。クライアントが長さで判定できるように)。
			if tt.wantTie == nil && !strings.Contains(raw, `"tie":[]`) {
				t.Errorf("body without a tie = %s, want \"tie\":[]", raw)
			}
			body := decodePosition(t, raw)
			want := api.PositionResponse{
				Speed:   tt.wantSpeed,
				Pokemon: tt.wantPokemon,
				Faster:  tt.wantFaster,
				Slower:  tt.wantSlower,
				Tie:     tt.wantTie,
			}
			if want.Tie == nil {
				want.Tie = []api.SpeedTableEntry{}
			}
			if !reflect.DeepEqual(body, want) {
				t.Errorf("body = %+v (tie %+v), want %+v (tie %+v)", body, body.Tie, want, want.Tie)
			}
			assertPositionCoversTable(t, body)
		})
	}
}

// assertPositionCoversTable: 位置は常に 6 プリセットの表の全行を速い / 同速 / 遅いに分けたもの(ADR-0602 §3)。
func assertPositionCoversTable(t *testing.T, body api.PositionResponse) {
	t.Helper()
	if total := body.Faster + len(body.Tie) + body.Slower; total != examplePositionRows {
		t.Errorf("faster+tie+slower = %d, want %d (8 体 × 6 プリセット)", total, examplePositionRows)
	}
}

// TestPositionHidesInternalErrors: provider のエラーも計算のエラーも 500 の固定文言で、内部の情報を返さない
// (ADR-0600 §5・ADR-0602 §4)。
func TestPositionHidesInternalErrors(t *testing.T) {
	t.Parallel()

	secret := "open /secret/path/pokemon.json: permission denied"
	// read model は検証済みなので通常起きないが、表の組み立てが失敗しても 500 の固定文言。
	invalidRoster := speed.Roster{RegulationID: "example", Pokemon: []speed.Pokemon{
		{PokemonID: "9001-000", NameJa: "テストsecretフセイ", Types: []string{"normal"}, BaseSpeed: 0},
	}}
	tests := []struct {
		name     string
		provider speed.PokemonProvider
		body     string
	}{
		{"provider のエラー", fakeProvider{err: errors.New(secret)}, `{"mode":"raw","value":200}`},
		{"計算のエラー", fakeProvider{roster: invalidRoster}, `{"mode":"raw","value":200}`},
		{"計算のエラー(preset)", fakeProvider{roster: invalidRoster}, `{"mode":"preset","pokemonId":"9001-000","preset":"max","scarf":false}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := serve(Dependencies{Pokemon: tt.provider}, newPositionRequest(validHeaders, tt.body))
			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
			}
			body := decodeError(t, recorder)
			if body.Code != api.InternalError {
				t.Errorf("code = %q, want %q", body.Code, api.InternalError)
			}
			if body.Message != "internal error" {
				t.Errorf("message = %q, want the fixed string \"internal error\"", body.Message)
			}
			if strings.Contains(recorder.Body.String(), "secret") {
				t.Errorf("body leaks the internal error: %s", recorder.Body.String())
			}
		})
	}
}

// TestMinimalPresetEnumMatchesCore: API の MinimalPresetId の enum とコアの MinimalPresets() が
// 同じ ID・同じ順(ADR-0602 §5。同期の検査。SP1 の TestPresetEnumMatchesCore と同じ考え方)。
func TestMinimalPresetEnumMatchesCore(t *testing.T) {
	t.Parallel()

	want := []api.MinimalPresetId{api.MinimalPresetIdUninvested, api.MinimalPresetIdNeutralMax, api.MinimalPresetIdMax}
	presets := speed.MinimalPresets()
	got := make([]api.MinimalPresetId, len(presets))
	for i, p := range presets {
		got[i] = api.MinimalPresetId(p.ID)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("core minimal presets = %v, want the API enum %v", got, want)
	}
	for _, id := range want {
		if !id.Valid() {
			t.Errorf("api.MinimalPresetId(%q).Valid() = false", id)
		}
		// 最小の選択の ID は、SP1 の PresetId の enum にもある 6 つのうちの 3 つ(ADR-0602 §5)。
		if !api.PresetId(id).Valid() {
			t.Errorf("api.PresetId(%q).Valid() = false, want the minimal presets to be a subset of PresetId", id)
		}
	}
}

// TestNatureEnumMatchesCore: API の NatureId の enum とコアの NatureEffect が同じ文字列(ADR-0602 §5)。
func TestNatureEnumMatchesCore(t *testing.T) {
	t.Parallel()

	pairs := []struct {
		apiID api.NatureId
		core  speed.NatureEffect
	}{
		{api.Minus, speed.NatureMinus},
		{api.Neutral, speed.NatureNeutral},
		{api.Plus, speed.NaturePlus},
	}
	for _, pair := range pairs {
		if string(pair.apiID) != string(pair.core) {
			t.Errorf("api.NatureId %q != speed.NatureEffect %q", pair.apiID, pair.core)
		}
		if !pair.apiID.Valid() {
			t.Errorf("api.NatureId(%q).Valid() = false", pair.apiID)
		}
	}
}

// TestPositionModeEnumMatchesCore: API の mode の enum とコアの PositionMode が同じ文字列(ADR-0602 §2)。
func TestPositionModeEnumMatchesCore(t *testing.T) {
	t.Parallel()

	pairs := []struct {
		apiID api.PositionRequestMode
		core  speed.PositionMode
	}{
		{api.Preset, speed.PositionModePreset},
		{api.Custom, speed.PositionModeCustom},
		{api.Raw, speed.PositionModeRaw},
	}
	for _, pair := range pairs {
		if string(pair.apiID) != string(pair.core) {
			t.Errorf("api.PositionRequestMode %q != speed.PositionMode %q", pair.apiID, pair.core)
		}
		if !pair.apiID.Valid() {
			t.Errorf("api.PositionRequestMode(%q).Valid() = false", pair.apiID)
		}
	}
}
