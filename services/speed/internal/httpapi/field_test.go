package httpapi

import (
	"net/http"
	"reflect"
	"strings"
	"testing"

	"example.com/pokecalc/services/speed/internal/api"
)

// ADR-0607: 追い風・まひ・トリックルームの HTTP 契約。
//
// 例の read model(testdata/pokemon.example.json)に表の追い風を掛けた値(ADR-0607 §2 の式。max-scarf は
// スカーフと連結して ×3、それ以外は ×2)。追い風なしの値は table_test.go の手計算。
//
//	9001 種族値 100: 240 / 304 / 334 / 167×3=501 / 250×2=500 / 668
//	9002 種族値  81: 202 / 266 / 292 / 146×3=438 / 219×2=438 / 584
//	9003 種族値  45: 130 / 194 / 212 / 106×3=318 / 159×2=318 / 424
//	9004 種族値 130: 300 / 364 / 400 / 200×3=600 / 300×2=600 / 800
//	9005 種族値  81: 9002 と同じ
//	9006 種族値  60: 160 / 224 / 246 / 123×3=369 / 184×2=368 / 492
//	9007 種族値  30: 100 / 164 / 180 / 90×3=270  / 135×2=270 / 360
//	9008 種族値 110: 260 / 324 / 356 / 178×3=534 / 267×2=534 / 712
var exampleTailwindSpeeds = map[string][]int{
	"9001-000": {240, 304, 334, 501, 500, 668},
	"9002-000": {202, 266, 292, 438, 438, 584},
	"9003-000": {130, 194, 212, 318, 318, 424},
	"9004-000": {300, 364, 400, 600, 600, 800},
	"9005-000": {202, 266, 292, 438, 438, 584},
	"9006-000": {160, 224, 246, 369, 368, 492},
	"9007-000": {100, 164, 180, 270, 270, 360},
	"9008-000": {260, 324, 356, 534, 534, 712},
}

// speedsByPokemon は表の応答を pokemonId → ADR-0601 §2 の順の実数値 6 つに組み替える。
func speedsByPokemon(t *testing.T, body api.TableResponse) map[string][]int {
	t.Helper()
	index := make(map[api.PresetId]int, len(allPresetIDs))
	for i, id := range allPresetIDs {
		index[id] = i
	}
	got := make(map[string][]int)
	for _, tier := range body.Tiers {
		for _, e := range tier.Entries {
			if _, ok := got[e.PokemonId]; !ok {
				got[e.PokemonId] = make([]int, len(allPresetIDs))
			}
			got[e.PokemonId][index[e.Preset]] = tier.Speed
		}
	}
	return got
}

func getTable(t *testing.T, query string) api.TableResponse {
	t.Helper()
	path := tablePath
	if query != "" {
		path += "?" + query
	}
	recorder := serve(Dependencies{Pokemon: exampleProvider(t)}, newRequest(path, validHeaders))
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET %s: status = %d, want %d; body=%s", path, recorder.Code, http.StatusOK, recorder.Body.String())
	}
	return decodeTable(t, recorder.Body.String())
}

// TestTableTailwindQuery: tailwind=true は全行の実数値に追い風を掛ける(ADR-0607 §2)。
// 追い風なしで同速だった 9001・9006 のスカーフと +1 は、連結の丸めで別の段に分かれる。
func TestTableTailwindQuery(t *testing.T) {
	t.Parallel()

	body := getTable(t, "tailwind=true")
	if got := speedsByPokemon(t, body); !reflect.DeepEqual(got, exampleTailwindSpeeds) {
		t.Errorf("tailwind table speeds =\n%v\nwant\n%v", got, exampleTailwindSpeeds)
	}
	for i := 1; i < len(body.Tiers); i++ {
		if body.Tiers[i].Speed >= body.Tiers[i-1].Speed {
			t.Errorf("tier #%d speed %d is not below the previous %d (still descending with tailwind)", i, body.Tiers[i].Speed, body.Tiers[i-1].Speed)
		}
	}
	for _, split := range []int{501, 500, 369, 368} {
		tier, ok := findTier(body.Tiers, split)
		if !ok {
			t.Errorf("tier %d is missing", split)
			continue
		}
		if len(tier.Entries) != 1 {
			t.Errorf("tier %d has %d entries, want 1 (scarf and +1 split under tailwind)", split, len(tier.Entries))
		}
	}
	if !reflect.DeepEqual(body.Presets, allPresetIDs) {
		t.Errorf("presets = %v, want %v", body.Presets, allPresetIDs)
	}
}

// TestTableTailwindWithPresets: tailwind は presets の絞り込みと組み合わせられる。
func TestTableTailwindWithPresets(t *testing.T) {
	t.Parallel()

	body := getTable(t, "presets=max-scarf&tailwind=true")
	if want := []api.PresetId{api.PresetIdMaxScarf}; !reflect.DeepEqual(body.Presets, want) {
		t.Errorf("presets = %v, want %v", body.Presets, want)
	}
	gotSpeeds := make([]int, len(body.Tiers))
	for i, tier := range body.Tiers {
		gotSpeeds[i] = tier.Speed
	}
	// max-scarf の列だけを降順に: 9004 600 / 9008 534 / 9001 501 / 9002・9005 438 / 9006 369 / 9003 318 / 9007 270
	if want := []int{600, 534, 501, 438, 369, 318, 270}; !reflect.DeepEqual(gotSpeeds, want) {
		t.Errorf("tier speeds = %v, want %v", gotSpeeds, want)
	}
}

// TestTableTrickRoomQuery: trickRoom=true は実数値を変えず、段の順だけを逆(昇順)にする。段の中の並びは
// そのまま(ADR-0607 §4)。tailwind と組み合わせても同じ。
func TestTableTrickRoomQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		normal    string
		trickRoom string
	}{
		{"追い風なし", "", "trickRoom=true"},
		{"追い風あり", "tailwind=true", "tailwind=true&trickRoom=true"},
		{"絞り込みあり", "presets=max,max-scarf", "presets=max,max-scarf&trickRoom=true"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			normal := getTable(t, tt.normal)
			reversed := getTable(t, tt.trickRoom)

			want := make([]api.SpeedTier, len(normal.Tiers))
			for i, tier := range normal.Tiers {
				want[len(normal.Tiers)-1-i] = tier
			}
			if !reflect.DeepEqual(reversed.Tiers, want) {
				t.Errorf("trickRoom tiers =\n%+v\nwant the normal tiers reversed\n%+v", reversed.Tiers, want)
			}
			if !reflect.DeepEqual(reversed.Presets, normal.Presets) || reversed.RegulationId != normal.RegulationId {
				t.Errorf("trickRoom presets/regulationId = %v/%q, want %v/%q", reversed.Presets, reversed.RegulationId, normal.Presets, normal.RegulationId)
			}
		})
	}

	// 具体例: 先頭は最も遅い 50(9007 無振り)、同速の 219 の段の中は 9002 → 9005、スカーフ → +1 のまま。
	body := getTable(t, "trickRoom=true")
	if first := body.Tiers[0]; first.Speed != 50 || first.Entries[0].PokemonId != "9007-000" {
		t.Errorf("first tier = %+v, want 50 with 9007-000", first)
	}
	if last := body.Tiers[len(body.Tiers)-1]; last.Speed != 400 {
		t.Errorf("last tier speed = %d, want 400", last.Speed)
	}
	tie, ok := findTier(body.Tiers, 219)
	if !ok {
		t.Fatalf("tier 219 is missing")
	}
	gotOrder := make([]string, len(tie.Entries))
	for i, e := range tie.Entries {
		gotOrder[i] = e.PokemonId + "/" + string(e.Preset)
	}
	if want := []string{"9002-000/max-scarf", "9002-000/max-plus1", "9005-000/max-scarf", "9005-000/max-plus1"}; !reflect.DeepEqual(gotOrder, want) {
		t.Errorf("tier 219 order = %v, want %v (inner order is not reversed)", gotOrder, want)
	}
}

// TestTableFieldQueryOffIsUnchanged: tailwind=false・trickRoom=false は省略と同じ応答(後方互換)。
func TestTableFieldQueryOffIsUnchanged(t *testing.T) {
	t.Parallel()

	omitted := serve(Dependencies{Pokemon: exampleProvider(t)}, newRequest(tablePath, validHeaders))
	for _, query := range []string{"tailwind=false", "trickRoom=false", "tailwind=false&trickRoom=false"} {
		recorder := serve(Dependencies{Pokemon: exampleProvider(t)}, newRequest(tablePath+"?"+query, validHeaders))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want %d; body=%s", query, recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if recorder.Body.String() != omitted.Body.String() {
			t.Errorf("%s: body differs from the omitted query\n got %s\nwant %s", query, recorder.Body.String(), omitted.Body.String())
		}
	}
}

// TestTableRejectsInvalidFieldQuery: 真偽値でない・空・重複は 400 invalid_request。read model の有無より先
// (ADR-0601 §5 の判定順のまま)。
func TestTableRejectsInvalidFieldQuery(t *testing.T) {
	t.Parallel()

	for _, query := range []string{
		"tailwind=yes",
		"tailwind=",
		"tailwind=true&tailwind=false",
		"trickRoom=on",
		"trickRoom=",
		"trickRoom=true&trickRoom=true",
	} {
		t.Run(query, func(t *testing.T) {
			t.Parallel()
			for _, deps := range []Dependencies{{Pokemon: exampleProvider(t)}, {}} {
				recorder := serve(deps, newRequest(tablePath+"?"+query, validHeaders))
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

// TestTableChecksHeadersBeforeFieldQuery: ヘッダーとクエリの両方が不正なら、ヘッダーの 400(ADR-0606 §2)。
func TestTableChecksHeadersBeforeFieldQuery(t *testing.T) {
	t.Parallel()

	recorder := serve(Dependencies{Pokemon: exampleProvider(t)}, newRequest(tablePath+"?tailwind=yes", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	if body := decodeError(t, recorder); body.Code != api.MissingHeader {
		t.Errorf("code = %q, want %q", body.Code, api.MissingHeader)
	}
}

func postPosition(t *testing.T, body string) api.PositionResponse {
	t.Helper()
	recorder := serve(Dependencies{Pokemon: exampleProvider(t)}, newPositionRequest(validHeaders, body))
	if recorder.Code != http.StatusOK {
		t.Fatalf("POST %s: status = %d, want %d; body=%s", body, recorder.Code, http.StatusOK, recorder.Body.String())
	}
	return decodePosition(t, recorder.Body.String())
}

func entry9006(preset api.PresetId) api.SpeedTableEntry {
	return exampleEntry("9006-000", "テストコオリクジラン", []string{"ice", "water"}, 60, preset)
}

// TestPositionFieldEffects: 自分の tailwind・paralysis は自分の実数値に、tableTailwind は表の全行に掛かる
// (ADR-0607 §1)。位置の期待値は position_test.go の段の累積(追い風なし)と exampleTailwindSpeeds(追い風あり)から。
func TestPositionFieldEffects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want api.PositionResponse
	}{
		{
			// 9004 最速 200 → 追い風 400 = 9004 の最速+2 と同速
			name: "preset + 自分の追い風",
			body: `{"mode":"preset","pokemonId":"9004-000","preset":"max","scarf":false,"tailwind":true}`,
			want: api.PositionResponse{Speed: 400, Pokemon: &pokemon9004, Faster: 0, Slower: 47, Tie: []api.SpeedTableEntry{entry9004(api.PresetIdMaxPlus2)}},
		},
		{
			// 200 → まひ 100。100 より速い行は累積 101:42 まで、遅い行は 97/90/82/80/65/50 の 6 行
			name: "preset + まひ",
			body: `{"mode":"preset","pokemonId":"9004-000","preset":"max","scarf":false,"paralysis":true}`,
			want: api.PositionResponse{Speed: 100, Pokemon: &pokemon9004, Faster: 42, Slower: 6, Tie: []api.SpeedTableEntry{}},
		},
		{
			// 200 → スカーフ 300 → まひ 150 = 9004 の無振りと同速。速い行は累積 152:28
			name: "custom + スカーフ + まひ",
			body: `{"mode":"custom","pokemonId":"9004-000","sp":32,"nature":"plus","rank":0,"scarf":true,"paralysis":true}`,
			want: api.PositionResponse{Speed: 150, Pokemon: &pokemon9004, Faster: 28, Slower: 19, Tie: []api.SpeedTableEntry{entry9004(api.PresetIdUninvested)}},
		},
		{
			// 自分は 202 のまま、表に追い風: 202 = 9002・9005 の無振り(101 × 2)と同速。
			// 遅いのは 130/194(9003)・160(9006)・100/164/180(9007)の 6 行
			name: "raw + 表の追い風",
			body: `{"mode":"raw","value":202,"tableTailwind":true}`,
			want: api.PositionResponse{Speed: 202, Faster: 40, Slower: 6, Tie: []api.SpeedTableEntry{entry9002(api.PresetIdUninvested), entry9005(api.PresetIdUninvested)}},
		},
		{
			// 9006 最速 123 → 追い風 + スカーフ 369、表にも追い風: 369 = 9006 の最速スカーフ(連結)と同速、
			// 9006 の +1(368)とは同速にならない
			name: "preset + 自分と表の両方に追い風 + スカーフ",
			body: `{"mode":"preset","pokemonId":"9006-000","preset":"max","scarf":true,"tailwind":true,"tableTailwind":true}`,
			want: api.PositionResponse{
				Speed:   369,
				Pokemon: &api.SpeedPokemon{PokemonId: "9006-000", NameJa: "テストコオリクジラン", Types: []string{"ice", "water"}, BaseSpeed: 60},
				// 369 より速い: 9001 の 501/500/668、9002・9005 の 438/438/584、9003 の 424、9004 の 400/600/600/800、
				// 9006 の 492、9008 の 534/534/712 = 3 + 6 + 1 + 4 + 1 + 3 = 18
				Faster: 18, Slower: 29, Tie: []api.SpeedTableEntry{entry9006(api.PresetIdMaxScarf)},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := postPosition(t, tt.body); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("POST %s =\n%+v\nwant\n%+v", tt.body, got, tt.want)
			}
		})
	}
}

// TestPositionFieldEffectsOffIsUnchanged: tailwind・paralysis・tableTailwind の false と null は省略と同じ応答
// (後方互換。A JSON null for an optional field is treated the same as omitting it)。
func TestPositionFieldEffectsOffIsUnchanged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		omitted string
		same    []string
	}{
		{
			omitted: `{"mode":"preset","pokemonId":"9004-000","preset":"max","scarf":true}`,
			same: []string{
				`{"mode":"preset","pokemonId":"9004-000","preset":"max","scarf":true,"tailwind":false,"paralysis":false,"tableTailwind":false}`,
				`{"mode":"preset","pokemonId":"9004-000","preset":"max","scarf":true,"tailwind":null,"paralysis":null,"tableTailwind":null}`,
			},
		},
		{
			omitted: `{"mode":"custom","pokemonId":"9004-000","sp":32,"nature":"plus","rank":1,"scarf":false}`,
			same: []string{
				`{"mode":"custom","pokemonId":"9004-000","sp":32,"nature":"plus","rank":1,"scarf":false,"tailwind":false,"paralysis":false,"tableTailwind":false}`,
			},
		},
		{
			omitted: `{"mode":"raw","value":219}`,
			same: []string{
				`{"mode":"raw","value":219,"tableTailwind":false}`,
				`{"mode":"raw","value":219,"tableTailwind":null}`,
				// raw でも null は省略と同じなので、tailwind・paralysis の null は拒否しない
				`{"mode":"raw","value":219,"tailwind":null,"paralysis":null}`,
			},
		},
	}
	for _, tt := range tests {
		want := postPosition(t, tt.omitted)
		for _, body := range tt.same {
			if got := postPosition(t, body); !reflect.DeepEqual(got, want) {
				t.Errorf("POST %s =\n%+v\nwant the same as %s\n%+v", body, got, tt.omitted, want)
			}
		}
	}
}

// TestPositionRejectsInvalidFieldEffects: raw に自分の tailwind・paralysis(false も含む)、真偽値でない値は
// 400 invalid_request。read model の有無より先(ADR-0602 §4 の判定順のまま)。
func TestPositionRejectsInvalidFieldEffects(t *testing.T) {
	t.Parallel()

	for _, body := range []string{
		`{"mode":"raw","value":200,"tailwind":true}`,
		`{"mode":"raw","value":200,"tailwind":false}`,
		`{"mode":"raw","value":200,"paralysis":true}`,
		`{"mode":"raw","value":200,"paralysis":false}`,
		`{"mode":"preset","pokemonId":"9004-000","preset":"max","scarf":false,"tailwind":"true"}`,
		`{"mode":"preset","pokemonId":"9004-000","preset":"max","scarf":false,"paralysis":1}`,
		`{"mode":"custom","pokemonId":"9004-000","sp":32,"nature":"plus","rank":0,"scarf":false,"tableTailwind":"yes"}`,
		`{"mode":"raw","value":200,"tableTailwind":0}`,
		// トリックルームは位置の入力ではない(ADR-0607 §4)。未知の欄として拒否される
		`{"mode":"raw","value":200,"trickRoom":true}`,
	} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			for _, deps := range []Dependencies{{Pokemon: exampleProvider(t)}, {}} {
				recorder := serve(deps, newPositionRequest(validHeaders, body))
				if recorder.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
				}
				if got := decodeError(t, recorder); got.Code != api.InvalidRequest {
					t.Errorf("code = %q, want %q", got.Code, api.InvalidRequest)
				}
				if strings.Contains(recorder.Body.String(), "master") {
					t.Errorf("body = %s, want the body check before the read model check", recorder.Body.String())
				}
			}
		})
	}
}
