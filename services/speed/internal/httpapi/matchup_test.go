package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/speed/internal/api"
)

// I-speed-4 / ADR-0610 の HTTP 層の受け入れ条件。例の read model(testdata/pokemon.example.json)を使う。

const (
	matchupPath = "/api/speed/v1/matchup"
	reachPath   = "/api/speed/v1/reach"
)

func newPostRequest(path string, headers map[string]string, body string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	return request
}

func decodeStrict[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()
	var body T
	decoder := json.NewDecoder(strings.NewReader(recorder.Body.String()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		t.Fatalf("body does not match the contract: %v; body=%s", err, recorder.Body.String())
	}
	return body
}

func TestMatchupAndReachRequireRequestContext(t *testing.T) {
	t.Parallel()
	for _, path := range []string{matchupPath, reachPath} {
		for _, tt := range headerRejectCases {
			t.Run(path+" "+tt.name, func(t *testing.T) {
				t.Parallel()
				body := `{"self":{"pokemonId":"9001-000"},"opponent":{"pokemonId":"9004-000"}}`
				if path == reachPath {
					body = `{"mode":"raw","value":200}`
				}
				recorder := serve(Dependencies{Pokemon: exampleProvider(t)}, newPostRequest(path, tt.headers, body))
				if recorder.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
				}
				if got := decodeError(t, recorder).Code; got != tt.want {
					t.Errorf("code = %s, want %s", got, tt.want)
				}
			})
		}
	}
}

func TestMatchupDefaultAssumptions(t *testing.T) {
	t.Parallel()
	// 自分 100 族 vs 相手 130 族(無振り 150・準速 182・最速 200・最速スカーフ 300)。
	recorder := serve(Dependencies{Pokemon: exampleProvider(t)}, newPostRequest(matchupPath, validHeaders,
		`{"self":{"pokemonId":"9001-000"},"opponent":{"pokemonId":"9004-000"}}`))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	body := decodeStrict[api.MatchupResponse](t, recorder)
	if body.Self.PokemonId != "9001-000" || body.Opponent.PokemonId != "9004-000" || body.TrickRoom {
		t.Errorf("self/opponent/trickRoom = %s/%s/%v", body.Self.PokemonId, body.Opponent.PokemonId, body.TrickRoom)
	}
	if body.SelfSpeed != nil {
		t.Errorf("selfSpeed = %d, want absent without self.current", *body.SelfSpeed)
	}
	wantPresets := []api.PresetId{api.PresetIdUninvested, api.PresetIdNeutralMax, api.PresetIdMax, api.PresetIdMaxScarf}
	wantSpeeds := []int{150, 182, 200, 300}
	if len(body.Lines) != len(wantPresets) {
		t.Fatalf("len(lines) = %d, want %d", len(body.Lines), len(wantPresets))
	}
	for i, line := range body.Lines {
		if line.Assumption.Preset == nil || *line.Assumption.Preset != wantPresets[i] || line.Assumption.Custom != nil {
			t.Errorf("line %d assumption = %+v, want preset %s", i, line.Assumption, wantPresets[i])
		}
		if line.OpponentSpeed != wantSpeeds[i] {
			t.Errorf("line %d opponentSpeed = %d, want %d", i, line.OpponentSpeed, wantSpeeds[i])
		}
		if line.Current != nil {
			t.Errorf("line %d current = %s, want absent", i, *line.Current)
		}
	}
	// 無振り 150 に対して: 下降はなし、補正なし SP 31〜(同速 30)、上昇 SP 18〜(同速 17)。
	want := []api.NatureLine{
		{Nature: api.Minus},
		{Nature: api.Neutral, MovesFirst: &api.SpRange{Min: 31, Max: 32}, Tie: &api.SpRange{Min: 30, Max: 30}},
		{Nature: api.Plus, MovesFirst: &api.SpRange{Min: 18, Max: 32}, Tie: &api.SpRange{Min: 17, Max: 17}},
	}
	if !reflect.DeepEqual(body.Lines[0].Natures, want) {
		t.Errorf("natures vs uninvested = %+v, want %+v", body.Lines[0].Natures, want)
	}
}

func TestMatchupCurrentUnderTrickRoom(t *testing.T) {
	t.Parallel()
	// 自分 最速 167。トリックルームでは 150 の相手が先、300 の相手より自分が先。
	recorder := serve(Dependencies{Pokemon: exampleProvider(t)}, newPostRequest(matchupPath, validHeaders,
		`{"self":{"pokemonId":"9001-000","current":{"sp":32,"nature":"plus"}},`+
			`"opponent":{"pokemonId":"9004-000","assumptions":[{"preset":"uninvested"},{"preset":"max-scarf"}]},"trickRoom":true}`))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	body := decodeStrict[api.MatchupResponse](t, recorder)
	if body.SelfSpeed == nil || *body.SelfSpeed != 167 || !body.TrickRoom {
		t.Fatalf("selfSpeed/trickRoom = %v/%v, want 167/true", body.SelfSpeed, body.TrickRoom)
	}
	want := []api.TurnOrder{api.OpponentFirst, api.SelfFirst}
	for i, line := range body.Lines {
		if line.Current == nil || *line.Current != want[i] {
			t.Errorf("line %d current = %v, want %s", i, line.Current, want[i])
		}
	}
}

func TestMatchupRejectsInvalidBody(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
	}{
		{"not an object", `[]`},
		{"missing opponent", `{"self":{"pokemonId":"9001-000"}}`},
		{"unknown field", `{"self":{"pokemonId":"9001-000"},"opponent":{"pokemonId":"9004-000"},"format":"single"}`},
		{"assumption with both", `{"self":{"pokemonId":"9001-000"},"opponent":{"pokemonId":"9004-000","assumptions":[{"preset":"max","custom":{"sp":0,"nature":"neutral"}}]}}`},
		{"assumption with neither", `{"self":{"pokemonId":"9001-000"},"opponent":{"pokemonId":"9004-000","assumptions":[{}]}}`},
		{"duplicate preset", `{"self":{"pokemonId":"9001-000"},"opponent":{"pokemonId":"9004-000","assumptions":[{"preset":"max"},{"preset":"max"}]}}`},
		{"empty assumptions", `{"self":{"pokemonId":"9001-000"},"opponent":{"pokemonId":"9004-000","assumptions":[]}}`},
		{"unknown preset", `{"self":{"pokemonId":"9001-000"},"opponent":{"pokemonId":"9004-000","assumptions":[{"preset":"fastest"}]}}`},
		{"rank 7", `{"self":{"pokemonId":"9001-000","rank":7},"opponent":{"pokemonId":"9004-000"}}`},
		{"current sp 33", `{"self":{"pokemonId":"9001-000","current":{"sp":33,"nature":"plus"}},"opponent":{"pokemonId":"9004-000"}}`},
		{"bad pokemonId format", `{"self":{"pokemonId":"9001"},"opponent":{"pokemonId":"9004-000"}}`},
		{"trickRoom not boolean", `{"self":{"pokemonId":"9001-000"},"opponent":{"pokemonId":"9004-000"},"trickRoom":"yes"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := serve(Dependencies{Pokemon: exampleProvider(t)}, newPostRequest(matchupPath, validHeaders, tt.body))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
			}
			if got := decodeError(t, recorder).Code; got != api.InvalidRequest {
				t.Errorf("code = %s, want invalid_request", got)
			}
		})
	}
}

func TestMatchupUnknownPokemonAndNoReadModel(t *testing.T) {
	t.Parallel()
	unknown := serve(Dependencies{Pokemon: exampleProvider(t)}, newPostRequest(matchupPath, validHeaders,
		`{"self":{"pokemonId":"9001-000"},"opponent":{"pokemonId":"9999-000"}}`))
	if unknown.Code != http.StatusUnprocessableEntity || decodeError(t, unknown).Code != api.UnknownPokemon {
		t.Errorf("unknown opponent: status = %d body=%s, want 422 unknown_pokemon", unknown.Code, unknown.Body.String())
	}
	none := serve(Dependencies{}, newPostRequest(matchupPath, validHeaders,
		`{"self":{"pokemonId":"9001-000"},"opponent":{"pokemonId":"9004-000"}}`))
	if none.Code != http.StatusServiceUnavailable || decodeError(t, none).Code != api.MasterUnavailable {
		t.Errorf("no read model: status = %d body=%s, want 503 master_unavailable", none.Code, none.Body.String())
	}
}

func TestReachPreset(t *testing.T) {
	t.Parallel()
	// 自分 100 族 最速 167。最速の行: 99 族以下は遅い・100 族が同速・101 族以上が速い。
	// 例の 8 体の最速: 遅い 5 体(81・81・60・45・30)、同速 1 体(自分)、速い 2 体(110・130)。
	recorder := serve(Dependencies{Pokemon: exampleProvider(t)}, newPostRequest(reachPath, validHeaders,
		`{"mode":"preset","pokemonId":"9001-000","preset":"max","scarf":false}`))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	body := decodeStrict[api.ReachResponse](t, recorder)
	if body.Speed != 167 || body.Pokemon == nil || body.Pokemon.PokemonId != "9001-000" {
		t.Fatalf("speed/pokemon = %d/%v, want 167/9001-000", body.Speed, body.Pokemon)
	}
	order := []api.PresetId{api.PresetIdUninvested, api.PresetIdNeutralMax, api.PresetIdMax, api.PresetIdMaxScarf, api.PresetIdMaxPlus1, api.PresetIdMaxPlus2}
	if len(body.Lines) != len(order) {
		t.Fatalf("len(lines) = %d, want 6", len(body.Lines))
	}
	for i, l := range body.Lines {
		if l.Preset != order[i] {
			t.Fatalf("line %d preset = %s, want %s", i, l.Preset, order[i])
		}
	}
	got := body.Lines[2]
	if !reflect.DeepEqual(got.Slower, &api.BaseSpeedRange{Min: 1, Max: 99}) ||
		!reflect.DeepEqual(got.Tie, &api.BaseSpeedRange{Min: 100, Max: 100}) ||
		!reflect.DeepEqual(got.Faster, &api.BaseSpeedRange{Min: 101, Max: 255}) {
		t.Errorf("max line ranges = %v / %v / %v", got.Slower, got.Tie, got.Faster)
	}
	if got.SlowerCount != 5 || got.TieCount != 1 || got.FasterCount != 2 {
		t.Errorf("counts = %d/%d/%d, want 5/1/2", got.SlowerCount, got.TieCount, got.FasterCount)
	}
	if got.FastestSlower == nil || got.FastestSlower.PokemonId != "9002-000" ||
		got.SlowestFaster == nil || got.SlowestFaster.PokemonId != "9008-000" {
		t.Errorf("boundary = %v / %v, want 9002-000 / 9008-000", got.FastestSlower, got.SlowestFaster)
	}
}

func TestReachValidatesLikePosition(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		want int
		code api.ErrorCode
	}{
		{"raw with tailwind", `{"mode":"raw","value":200,"tailwind":true}`, http.StatusBadRequest, api.InvalidRequest},
		{"raw out of range", `{"mode":"raw","value":1}`, http.StatusBadRequest, api.InvalidRequest},
		{"preset missing scarf", `{"mode":"preset","pokemonId":"9001-000","preset":"max"}`, http.StatusBadRequest, api.InvalidRequest},
		{"unknown pokemon", `{"mode":"preset","pokemonId":"9999-000","preset":"max","scarf":false}`, http.StatusUnprocessableEntity, api.UnknownPokemon},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := serve(Dependencies{Pokemon: exampleProvider(t)}, newPostRequest(reachPath, validHeaders, tt.body))
			if recorder.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tt.want, recorder.Body.String())
			}
			if got := decodeError(t, recorder).Code; got != tt.code {
				t.Errorf("code = %s, want %s", got, tt.code)
			}
		})
	}
}

// TestTurnOrderEnumMatchesEngine は契約の TurnOrder が engine の語彙(連続技と同じ)と一致することを守る。
func TestTurnOrderEnumMatchesEngine(t *testing.T) {
	t.Parallel()
	got := []string{string(api.SelfFirst), string(api.OpponentFirst), string(api.SpeedTie)}
	want := []string{string(engine.TurnOrderSelfFirst), string(engine.TurnOrderOpponentFirst), string(engine.TurnOrderSpeedTie)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TurnOrder enum = %v, want %v (engine.TurnOrder*)", got, want)
	}
}
