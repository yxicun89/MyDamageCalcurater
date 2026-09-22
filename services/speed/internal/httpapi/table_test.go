package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"example.com/pokecalc/services/speed/internal/api"
	"example.com/pokecalc/services/speed/internal/master"
	"example.com/pokecalc/services/speed/internal/speed"
)

const (
	tablePath          = "/api/speed/v1/table"
	examplePokemonPath = "../../testdata/pokemon.example.json"
)

// allPresetIDs は ADR-0601 §2 の表の順(API の PresetId の enum の順と同じ)。
var allPresetIDs = []api.PresetId{api.PresetIdUninvested, api.PresetIdNeutralMax, api.PresetIdMax, api.PresetIdMaxScarf, api.PresetIdMaxPlus1, api.PresetIdMaxPlus2}

// exampleProvider は testdata/pokemon.example.json(架空の 8 体)の read model。
func exampleProvider(t *testing.T) speed.PokemonProvider {
	t.Helper()
	provider, err := master.LoadPokemonFile(examplePokemonPath)
	if err != nil {
		t.Fatalf("load %s: %v", examplePokemonPath, err)
	}
	return provider
}

func decodeTable(t *testing.T, raw string) api.TableResponse {
	t.Helper()
	var body api.TableResponse
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		t.Fatalf("body is not a TableResponse: %v; body=%s", err, raw)
	}
	return body
}

// TestPresetEnumMatchesCore: API の enum とコアのプリセットの定義が同じ ID・同じ順(ADR-0601 §2。同期の検査)。
func TestPresetEnumMatchesCore(t *testing.T) {
	t.Parallel()

	presets := speed.Presets()
	got := make([]api.PresetId, len(presets))
	for i, p := range presets {
		got[i] = api.PresetId(p.ID)
	}
	if !reflect.DeepEqual(got, allPresetIDs) {
		t.Errorf("core presets = %v, want the API enum %v", got, allPresetIDs)
	}
	for _, id := range allPresetIDs {
		if !id.Valid() {
			t.Errorf("api.PresetId(%q).Valid() = false", id)
		}
	}
}

func TestTableRequiresRequestContext(t *testing.T) {
	t.Parallel()

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
			// ヘッダーの検査は read model の有無より先(provider が無くても 503 ではなく 400)。
			for _, deps := range []Dependencies{{Pokemon: fakeProvider{roster: unorderedRoster()}}, {}} {
				recorder := serve(deps, newRequest(tablePath, tt.headers))
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

// TestTableChecksHeadersBeforeQuery: ヘッダーとクエリの両方が不正なら、ヘッダーの 400 を返す(ADR-0601 §5 の順)。
// どちらも invalid_request なので、ヘッダーの検査の文言(X-Device-Id を含む)で区別する。
func TestTableChecksHeadersBeforeQuery(t *testing.T) {
	t.Parallel()

	recorder := serve(Dependencies{Pokemon: fakeProvider{roster: unorderedRoster()}}, newRequest(tablePath+"?presets=unknown", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	body := decodeError(t, recorder)
	if body.Code != api.InvalidRequest {
		t.Errorf("code = %q, want %q", body.Code, api.InvalidRequest)
	}
	if !strings.Contains(body.Message, "X-Device-Id") {
		t.Errorf("message = %q, want the header check's message (headers are checked before the query)", body.Message)
	}
}

func TestTableRejectsInvalidPresetsQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		query string
	}{
		{"未知の ID", "presets=max-plus3"},
		{"既知の中に未知", "presets=max,fastest"},
		{"大文字", "presets=MAX"},
		{"重複", "presets=max,max"},
		{"離れた重複", "presets=max-scarf,uninvested,max-scarf"},
		{"空(presets=)", "presets="},
		{"空の要素", "presets=max,,max-scarf"},
		{"末尾のカンマ", "presets=max,"},
		// explode: false なので、同じキーの繰り返しは不正(生成コードの 400 を invalid_request にそろえる)。
		{"キーの繰り返し", "presets=max&presets=max-scarf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// クエリの検査は read model の有無より先(provider が無くても 503 ではなく 400)。
			for _, deps := range []Dependencies{{Pokemon: fakeProvider{roster: unorderedRoster()}}, {}} {
				recorder := serve(deps, newRequest(tablePath+"?"+tt.query, validHeaders))
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

func TestTableWithoutReadModel(t *testing.T) {
	t.Parallel()

	for _, path := range []string{tablePath, tablePath + "?presets=max-scarf"} {
		recorder := serve(Dependencies{}, newRequest(path, validHeaders))
		if recorder.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s: status = %d, want %d; body=%s", path, recorder.Code, http.StatusServiceUnavailable, recorder.Body.String())
		}
		if body := decodeError(t, recorder); body.Code != api.MasterUnavailable {
			t.Errorf("%s: code = %q, want %q", path, body.Code, api.MasterUnavailable)
		}
	}
}

func TestTableHidesInternalErrors(t *testing.T) {
	t.Parallel()

	secret := "open /secret/path/pokemon.json: permission denied"
	invalidRoster := speed.Roster{RegulationID: "example", Pokemon: []speed.Pokemon{
		{PokemonID: "9001-000", NameJa: "テストsecretフセイ", Types: []string{"normal"}, BaseSpeed: 0},
	}}
	tests := []struct {
		name     string
		provider speed.PokemonProvider
	}{
		{"provider のエラー", fakeProvider{err: errors.New(secret)}},
		// read model は検証済みなので通常起きないが、計算のエラーも 500 の固定文言(ADR-0601 §5)。
		{"計算のエラー", fakeProvider{roster: invalidRoster}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := serve(Dependencies{Pokemon: tt.provider}, newRequest(tablePath, validHeaders))
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

// 例の read model(testdata/pokemon.example.json)の全 6 行を手計算した値(ADR-0600 §3 の式)。
//
//	9001 種族値 100: 120 / 152 / 167 / 250 / 250 / 334
//	9002 種族値  81: 101 / 133 / 146 / 219 / 219 / 292
//	9003 種族値  45:  65 /  97 / floor(97×1.1)=106 / 159 / 159 / 212
//	9004 種族値 130: 150 / 182 / floor(182×1.1)=200 / 300 / 300 / 400
//	9005 種族値  81: 9002 と同じ
//	9006 種族値  60:  80 / 112 / floor(112×1.1)=123 / 123×1.5=184.5 → 184(0.5 ちょうどは切り捨て) / floor(184.5)=184 / 246
//	9007 種族値  30:  50 /  82 / floor(82×1.1)=90 / 135 / 135 / 180
//	9008 種族値 110: 130 / 162 / floor(162×1.1)=178 / 267 / 267 / 356
//
// 9002 と 9005 は全行が同じ値。各ポケモンの最速スカーフと最速+1 は同じ値(×1.5)なので、1 体あたりの値は 5 つ。
// ポケモンをまたぐ重複はほかに無いので、段は 7 × 5 = 35、行は 8 × 6 = 48。
func TestTableAllPresetsFromExample(t *testing.T) {
	t.Parallel()

	recorder := serve(Dependencies{Pokemon: exampleProvider(t)}, newRequest(tablePath, validHeaders))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	body := decodeTable(t, recorder.Body.String())

	if body.RegulationId != "example" {
		t.Errorf("regulationId = %q, want example", body.RegulationId)
	}
	// presets 省略時は 6 行すべて(ADR-0601 §2 の順)。
	if !reflect.DeepEqual(body.Presets, allPresetIDs) {
		t.Errorf("presets = %v, want %v", body.Presets, allPresetIDs)
	}
	if len(body.Tiers) != 35 {
		t.Errorf("tiers = %d, want 35", len(body.Tiers))
	}
	entries := 0
	for i, tier := range body.Tiers {
		if i > 0 && tier.Speed >= body.Tiers[i-1].Speed {
			t.Errorf("tier #%d speed %d is not below the previous %d", i, tier.Speed, body.Tiers[i-1].Speed)
		}
		entries += len(tier.Entries)
	}
	if entries != 48 {
		t.Errorf("entries = %d, want 48", entries)
	}
	if len(body.Tiers) > 0 {
		if first := body.Tiers[0]; first.Speed != 400 || len(first.Entries) != 1 || first.Entries[0].PokemonId != "9004-000" || first.Entries[0].Preset != api.PresetIdMaxPlus2 {
			t.Errorf("first tier = %+v, want 400 with 9004-000 max-plus2", first)
		}
		if last := body.Tiers[len(body.Tiers)-1]; last.Speed != 50 || len(last.Entries) != 1 || last.Entries[0].PokemonId != "9007-000" || last.Entries[0].Preset != api.PresetIdUninvested {
			t.Errorf("last tier = %+v, want 50 with 9007-000 uninvested", last)
		}
	}

	// 同速の段: 種族値 81 の 9002 と 9005 は同じ段で、pokemonId の昇順 → プリセットの順。
	fixture9002 := api.SpeedTableEntry{PokemonId: "9002-000", NameJa: "テストミズハネウオ", Types: []string{"water"}, BaseSpeed: 81}
	fixture9005 := api.SpeedTableEntry{PokemonId: "9005-000", NameJa: "テストクサモグリン", Types: []string{"grass", "poison"}, BaseSpeed: 81}
	withPreset := func(e api.SpeedTableEntry, preset api.PresetId) api.SpeedTableEntry {
		e.Preset = preset
		return e
	}
	wantTies := []api.SpeedTier{
		{Speed: 292, Entries: []api.SpeedTableEntry{withPreset(fixture9002, api.PresetIdMaxPlus2), withPreset(fixture9005, api.PresetIdMaxPlus2)}},
		{Speed: 219, Entries: []api.SpeedTableEntry{
			withPreset(fixture9002, api.PresetIdMaxScarf), withPreset(fixture9002, api.PresetIdMaxPlus1),
			withPreset(fixture9005, api.PresetIdMaxScarf), withPreset(fixture9005, api.PresetIdMaxPlus1),
		}},
		{Speed: 146, Entries: []api.SpeedTableEntry{withPreset(fixture9002, api.PresetIdMax), withPreset(fixture9005, api.PresetIdMax)}},
		{Speed: 133, Entries: []api.SpeedTableEntry{withPreset(fixture9002, api.PresetIdNeutralMax), withPreset(fixture9005, api.PresetIdNeutralMax)}},
		{Speed: 101, Entries: []api.SpeedTableEntry{withPreset(fixture9002, api.PresetIdUninvested), withPreset(fixture9005, api.PresetIdUninvested)}},
		// 同じポケモンの同速(スカーフと +1)も 1 つの段。0.5 ちょうどの切り捨てで 184 になる例。
		{Speed: 184, Entries: []api.SpeedTableEntry{
			{PokemonId: "9006-000", NameJa: "テストコオリクジラン", Types: []string{"ice", "water"}, BaseSpeed: 60, Preset: api.PresetIdMaxScarf},
			{PokemonId: "9006-000", NameJa: "テストコオリクジラン", Types: []string{"ice", "water"}, BaseSpeed: 60, Preset: api.PresetIdMaxPlus1},
		}},
	}
	for _, want := range wantTies {
		got, ok := findTier(body.Tiers, want.Speed)
		if !ok {
			t.Errorf("tier %d is missing", want.Speed)
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("tier %d = %+v, want %+v", want.Speed, got, want)
		}
	}
}

func findTier(tiers []api.SpeedTier, speedValue int) (api.SpeedTier, bool) {
	for _, tier := range tiers {
		if tier.Speed == speedValue {
			return tier, true
		}
	}
	return api.SpeedTier{}, false
}

// TestTablePresetsSubset: 指定したプリセットの行だけを返し、レスポンスの presets は ADR-0601 §2 の順。
// 値は TestTableAllPresetsFromExample の手計算。
func TestTablePresetsSubset(t *testing.T) {
	t.Parallel()

	type row struct {
		pokemonID string
		preset    api.PresetId
	}
	tests := []struct {
		name        string
		query       string
		wantPresets []api.PresetId
		wantTiers   map[int][]row // 段の値 → 行(段は降順に並ぶこと)
		wantOrder   []int
	}{
		{
			name:        "スカーフだけ",
			query:       "presets=max-scarf",
			wantPresets: []api.PresetId{api.PresetIdMaxScarf},
			wantOrder:   []int{300, 267, 250, 219, 184, 159, 135},
			wantTiers: map[int][]row{
				300: {{"9004-000", api.PresetIdMaxScarf}},
				267: {{"9008-000", api.PresetIdMaxScarf}},
				250: {{"9001-000", api.PresetIdMaxScarf}},
				219: {{"9002-000", api.PresetIdMaxScarf}, {"9005-000", api.PresetIdMaxScarf}},
				184: {{"9006-000", api.PresetIdMaxScarf}},
				159: {{"9003-000", api.PresetIdMaxScarf}},
				135: {{"9007-000", api.PresetIdMaxScarf}},
			},
		},
		{
			// 指定の順(+1 → 無振り)は結果に影響しない。
			name:        "逆順の指定",
			query:       "presets=max-plus1,uninvested",
			wantPresets: []api.PresetId{api.PresetIdUninvested, api.PresetIdMaxPlus1},
			wantOrder:   []int{300, 267, 250, 219, 184, 159, 150, 135, 130, 120, 101, 80, 65, 50},
			wantTiers: map[int][]row{
				300: {{"9004-000", api.PresetIdMaxPlus1}},
				267: {{"9008-000", api.PresetIdMaxPlus1}},
				250: {{"9001-000", api.PresetIdMaxPlus1}},
				219: {{"9002-000", api.PresetIdMaxPlus1}, {"9005-000", api.PresetIdMaxPlus1}},
				184: {{"9006-000", api.PresetIdMaxPlus1}},
				159: {{"9003-000", api.PresetIdMaxPlus1}},
				150: {{"9004-000", api.PresetIdUninvested}},
				135: {{"9007-000", api.PresetIdMaxPlus1}},
				130: {{"9008-000", api.PresetIdUninvested}},
				120: {{"9001-000", api.PresetIdUninvested}},
				101: {{"9002-000", api.PresetIdUninvested}, {"9005-000", api.PresetIdUninvested}},
				80:  {{"9006-000", api.PresetIdUninvested}},
				65:  {{"9003-000", api.PresetIdUninvested}},
				50:  {{"9007-000", api.PresetIdUninvested}},
			},
		},
		{
			name:        "6 つすべてを順不同で指定",
			query:       "presets=max-plus2,max,uninvested,max-scarf,neutral-max,max-plus1",
			wantPresets: allPresetIDs,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := serve(Dependencies{Pokemon: exampleProvider(t)}, newRequest(tablePath+"?"+tt.query, validHeaders))
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
			}
			body := decodeTable(t, recorder.Body.String())
			if !reflect.DeepEqual(body.Presets, tt.wantPresets) {
				t.Errorf("presets = %v, want %v", body.Presets, tt.wantPresets)
			}
			allowed := map[api.PresetId]bool{}
			for _, id := range tt.wantPresets {
				allowed[id] = true
			}
			for _, tier := range body.Tiers {
				for _, e := range tier.Entries {
					if !allowed[e.Preset] {
						t.Errorf("tier %d has an unrequested preset %q (%s)", tier.Speed, e.Preset, e.PokemonId)
					}
				}
			}
			if tt.wantTiers == nil {
				if len(body.Tiers) != 35 {
					t.Errorf("tiers = %d, want 35 (same as omitting presets)", len(body.Tiers))
				}
				return
			}
			gotOrder := make([]int, len(body.Tiers))
			for i, tier := range body.Tiers {
				gotOrder[i] = tier.Speed
				var gotRows []row
				for _, e := range tier.Entries {
					gotRows = append(gotRows, row{e.PokemonId, e.Preset})
				}
				if want := tt.wantTiers[tier.Speed]; !reflect.DeepEqual(gotRows, want) {
					t.Errorf("tier %d rows = %v, want %v", tier.Speed, gotRows, want)
				}
			}
			if !reflect.DeepEqual(gotOrder, tt.wantOrder) {
				t.Errorf("tier order = %v, want %v", gotOrder, tt.wantOrder)
			}
		})
	}
}

// TestTableIgnoresProviderOrder: provider が順不同でも、段の中は pokemonId の昇順(unorderedRoster の 9002 と 9010 は種族値 81)。
//
//	種族値 81 の最速 146(TestTableAllPresetsFromExample と同じ)
func TestTableIgnoresProviderOrder(t *testing.T) {
	t.Parallel()

	recorder := serve(Dependencies{Pokemon: fakeProvider{roster: unorderedRoster()}}, newRequest(tablePath+"?presets=max", validHeaders))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	body := decodeTable(t, recorder.Body.String())
	tier, ok := findTier(body.Tiers, 146)
	if !ok {
		t.Fatalf("tier 146 is missing; tiers=%+v", body.Tiers)
	}
	var ids []string
	for _, e := range tier.Entries {
		ids = append(ids, e.PokemonId)
	}
	if want := []string{"9002-000", "9010-000"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("tier 146 pokemonIds = %v, want %v", ids, want)
	}
}
