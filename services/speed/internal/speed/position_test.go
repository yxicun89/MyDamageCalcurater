package speed

import (
	"errors"
	"reflect"
	"testing"

	"example.com/pokecalc/engine"
)

// positionRoster は自分の位置(ADR-0602 §3)を確かめる架空の 3 体。種族値は table_test.go の手計算を流用できる
// ように 100 / 81 / 45 にしてある。provider の順は結果に影響しないので、わざと昇順にしていない。
func positionRoster() Roster {
	return Roster{RegulationID: "example", Pokemon: []Pokemon{
		{PokemonID: "9002-000", NameJa: "テストニバンメ", Types: []string{"water"}, BaseSpeed: 81},
		{PokemonID: "9001-000", NameJa: "テストイチバンメ", Types: []string{"fire", "flying"}, BaseSpeed: 100},
		{PokemonID: "9003-000", NameJa: "テストサンバンメ", Types: []string{"rock", "ground"}, BaseSpeed: 45},
	}}
}

func positionPokemon(t *testing.T, pokemonID string) Pokemon {
	t.Helper()
	for _, p := range positionRoster().Pokemon {
		if p.PokemonID == pokemonID {
			return p
		}
	}
	t.Fatalf("positionRoster has no %s", pokemonID)
	return Pokemon{}
}

// positionRoster の 6 プリセット(ADR-0601 §2)の表を手で計算した段(ADR-0600 §3 の式)。
// 実数値 = 種族値 + 20 + SP(補正なし)、上昇は floor(× 11/10)、ランク +n は floor(v × (2+n) / 2)、
// スカーフはランクの後に floor((v × 6144 + 2047) / 4096)。
//
//	9001 種族値 100: 無振り 120 / 準速 152 / 最速 floor(152×1.1)=167 / スカーフ 167×1.5=250.5 → 250(0.5 ちょうどは切り捨て)
//	                 / +1 floor(167×3/2)=250 / +2 334
//	9002 種族値  81: 無振り 101 / 準速 133 / 最速 floor(133×1.1)=146 / スカーフ 146×1.5=219 / +1 219 / +2 292
//	9003 種族値  45: 無振り  65 / 準速  97 / 最速 floor(97×1.1)=106  / スカーフ 106×1.5=159 / +1 159 / +2 212
//
// 段は降順に 334(1 行) 292(1) 250(2) 219(2) 212(1) 167(1) 159(2) 152(1) 146(1)
// 133(1) 120(1) 106(1) 101(1) 97(1) 65(1) の 15 段・18 行。
const positionRosterRows = 18 // 3 体 × 6 プリセット

// TestMinimalPresetsDerivedFromPresets: 最小の選択の 3 つ(ADR-0602 §5)は、Presets() の同じ ID の
// 定義の SP・性格・ランクをそのまま使う(値を 2 か所に書かない。coding-rules §2)。
func TestMinimalPresetsDerivedFromPresets(t *testing.T) {
	t.Parallel()

	got := MinimalPresets()

	// ID は ADR-0601 §2 の 6 つのうちスカーフを含まない最初の 3 つ、その順(ADR-0602 §5)。
	wantIDs := []PresetID{PresetUninvested, PresetNeutralMax, PresetMax}
	gotIDs := make([]PresetID, len(got))
	for i, p := range got {
		gotIDs[i] = p.ID
	}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("MinimalPresets() ids = %v, want %v", gotIDs, wantIDs)
	}

	// SP・性格・ランクは Presets() の同じ ID の定義と一致すること(期待値を書き写さず、1 か所の定義から導く)。
	byID := make(map[PresetID]Preset)
	for _, p := range Presets() {
		byID[p.ID] = p
	}
	for _, p := range got {
		full, ok := byID[p.ID]
		if !ok {
			t.Errorf("MinimalPresets() has %q, which Presets() does not define", p.ID)
			continue
		}
		if p.SP != full.SP || p.Nature != full.Nature || p.Rank != full.Rank {
			t.Errorf("MinimalPresets()[%q] = {SP:%d Nature:%q Rank:%d}, want Presets() の {SP:%d Nature:%q Rank:%d}",
				p.ID, p.SP, p.Nature, p.Rank, full.SP, full.Nature, full.Rank)
		}
		// スカーフは position の入力が独立に決めるので、定義側は常に「なし」(ADR-0602 §2)。
		if p.Scarf {
			t.Errorf("MinimalPresets()[%q].Scarf = true, want false (scarf is chosen separately)", p.ID)
		}
	}
}

// TestMinimalPresetsReturnsCopy: 呼び出し側が変更しても次の呼び出しの定義は変わらない(Presets と同じ)。
func TestMinimalPresetsReturnsCopy(t *testing.T) {
	t.Parallel()

	first := MinimalPresets()
	if len(first) == 0 {
		t.Fatalf("MinimalPresets() is empty")
	}
	first[0].SP = 99
	first[0].ID = "changed"
	if second := MinimalPresets(); second[0].ID != PresetUninvested || second[0].SP != 0 {
		t.Errorf("MinimalPresets()[0] after mutation = %+v, want the uninvested definition", second[0])
	}
}

// bruteForceRawSpeedRange は ADR-0602 §3 の「理論上の最小・最大」を、Speed の全組み合わせの総当たりで求める。
// 実装とは独立にテスト自身が導出する(定数の書き写しにならないよう、期待値をここで計算する)。
func bruteForceRawSpeedRange(t *testing.T) (minSpeed, maxSpeed int) {
	t.Helper()
	natures := []NatureEffect{NatureMinus, NatureNeutral, NaturePlus}
	first := true
	for base := minBaseSpeed; base <= maxBaseSpeed; base++ {
		for sp := 0; sp <= engine.MaxSPPerStat; sp++ {
			for _, nature := range natures {
				for rank := minRank; rank <= maxRank; rank++ {
					for _, scarf := range []bool{false, true} {
						v, err := Speed(Input{BaseSpeed: base, SP: sp, Nature: nature, Rank: rank, Scarf: scarf})
						if err != nil {
							t.Fatalf("Speed(base=%d sp=%d nature=%s rank=%d scarf=%v) error = %v", base, sp, nature, rank, scarf, err)
						}
						if first || v < minSpeed {
							minSpeed = v
						}
						if first || v > maxSpeed {
							maxSpeed = v
						}
						first = false
					}
				}
			}
		}
	}
	return minSpeed, maxSpeed
}

// TestRawSpeedRangeMatchesBruteForce: 実数値の直接入力の範囲は、engine の式から Speed 自身で導いた
// 最小・最大と完全に一致すること(ADR-0602 §3。ハードコードした定数ではない)。
func TestRawSpeedRangeMatchesBruteForce(t *testing.T) {
	t.Parallel()

	wantMin, wantMax := bruteForceRawSpeedRange(t)
	gotMin, gotMax := RawSpeedRange()
	if gotMin != wantMin || gotMax != wantMax {
		t.Errorf("RawSpeedRange() = (%d, %d), want (%d, %d)(Speed の全組み合わせを総当たりして導いた値)", gotMin, gotMax, wantMin, wantMax)
	}
	if gotMin < 1 {
		t.Errorf("RawSpeedRange() min = %d, want at least 1", gotMin)
	}
}

// TestPositionRawAcceptsRangeBoundaries: 範囲の端は受け付け、その外(最小-1・最大+1)は ErrInvalidRawSpeed。
func TestPositionRawAcceptsRangeBoundaries(t *testing.T) {
	t.Parallel()

	minSpeed, maxSpeed := RawSpeedRange()
	for _, value := range []int{minSpeed, maxSpeed} {
		got, err := Position(positionRoster(), PositionRequest{Mode: PositionModeRaw, Value: value})
		if err != nil {
			t.Errorf("Position(raw value=%d) error = %v, want nil (範囲の端は受け付ける)", value, err)
			continue
		}
		if got.Speed != value {
			t.Errorf("Position(raw value=%d).Speed = %d, want %d", value, got.Speed, value)
		}
	}
	for _, value := range []int{minSpeed - 1, maxSpeed + 1} {
		if _, err := Position(positionRoster(), PositionRequest{Mode: PositionModeRaw, Value: value}); !errors.Is(err, ErrInvalidRawSpeed) {
			t.Errorf("Position(raw value=%d) error = %v, want %v", value, err, ErrInvalidRawSpeed)
		}
	}
}

// TestPositionPresetModeExact: preset の 1 件を、段の構成・faster/slower の数・tie の中身まで完全一致で確かめる。
//
// 9002-000 の最速 = 146。上の段(334/292/250×2/219×2/212/167/159×2/152)の行数は
// 1+1+2+2+1+1+2+1 = 11、同速の段は 9002 の最速の行そのもの 1 行、下の段(133/120/106/101/97/65)は 6 行。
// 11 + 1 + 6 = 18 行(= positionRosterRows)。
// tie には「自分と同じ値の段の行」がそのまま入るので、自分自身の行も含む(ADR-0602 §3)。
func TestPositionPresetModeExact(t *testing.T) {
	t.Parallel()

	mine := positionPokemon(t, "9002-000")
	got, err := Position(positionRoster(), PositionRequest{
		Mode:      PositionModePreset,
		PokemonID: "9002-000",
		Preset:    PresetMax,
		Scarf:     false,
	})
	if err != nil {
		t.Fatalf("Position error = %v", err)
	}
	want := PositionResult{
		Speed:   146,
		Pokemon: &mine,
		Faster:  11,
		Slower:  6,
		Tie:     []TableEntry{{Pokemon: mine, Preset: PresetMax}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Position =\n%+v (tie %+v)\nwant\n%+v (tie %+v)", got, got.Tie, want, want.Tie)
	}
}

// TestPositionModes: 3 つの mode それぞれの実数値と位置。期待値はファイル冒頭の手計算の段から数えた値。
func TestPositionModes(t *testing.T) {
	t.Parallel()

	p9001 := positionPokemon(t, "9001-000")
	p9002 := positionPokemon(t, "9002-000")
	p9003 := positionPokemon(t, "9003-000")

	tests := []struct {
		name        string
		request     PositionRequest
		wantSpeed   int
		wantPokemon *Pokemon
		wantFaster  int
		wantSlower  int
		wantTie     []TableEntry
	}{
		{
			// 無振り 120 にスカーフ: 120×1.5 = 180。SP1 の 6 行には無い値なので同速なし
			// (無振り・準速にもスカーフを乗せられるのが SP2 の追加。ADR-0602 §2)。
			// 180 より上: 334/292/250×2/219×2/212 = 7 行、下は 18-7 = 11 行。
			name:        "preset 無振り + スカーフ(同速なし)",
			request:     PositionRequest{Mode: PositionModePreset, PokemonID: "9001-000", Preset: PresetUninvested, Scarf: true},
			wantSpeed:   180,
			wantPokemon: &p9001,
			wantFaster:  7,
			wantSlower:  11,
			wantTie:     nil,
		},
		{
			// preset の最速 + スカーフは、表の max-scarf の行と同じ入力になる(SP1 の行と同速)。
			// 9003 の最速 106 → スカーフ 159。159 の段は 9003 の max-scarf と max-plus1 の 2 行。
			// 159 より上: 334/292/250×2/219×2/212/167 = 8 行、下は 18-8-2 = 8 行。
			name:        "preset 最速 + スカーフ(表の max-scarf と同速)",
			request:     PositionRequest{Mode: PositionModePreset, PokemonID: "9003-000", Preset: PresetMax, Scarf: true},
			wantSpeed:   159,
			wantPokemon: &p9003,
			wantFaster:  8,
			wantSlower:  8,
			wantTie:     []TableEntry{{Pokemon: p9003, Preset: PresetMaxScarf}, {Pokemon: p9003, Preset: PresetMaxPlus1}},
		},
		{
			// custom: 種族値 45・SP 20・上昇・+1・スカーフなし。
			// 実数値 floor((45+20+20)×1.1) = floor(93.5) = 93、+1 で floor(93×3/2) = floor(139.5) = 139。
			// 139 より上: 334/292/250×2/219×2/212/167/159×2/152/146 = 12 行、下は 18-12 = 6 行。
			name:        "custom(同速なし)",
			request:     PositionRequest{Mode: PositionModeCustom, PokemonID: "9003-000", SP: 20, Nature: NaturePlus, Rank: 1, Scarf: false},
			wantSpeed:   139,
			wantPokemon: &p9003,
			wantFaster:  12,
			wantSlower:  6,
			wantTie:     nil,
		},
		{
			// custom: 種族値 81・SP 0・下降・ランク 0。floor((81+20+0)×0.9) = floor(90.9) = 90。
			// 90 より上: 65 と 90 未満の値以外 = 18 - (65 の 1 行) - 0 = 17 行、下は 65 の 1 行。
			name:        "custom 下降の性格",
			request:     PositionRequest{Mode: PositionModeCustom, PokemonID: "9002-000", SP: 0, Nature: NatureMinus, Rank: 0, Scarf: false},
			wantSpeed:   90,
			wantPokemon: &p9002,
			wantFaster:  17,
			wantSlower:  1,
			wantTie:     nil,
		},
		{
			// raw: 219 は 9002 のスカーフ・+1 の段(2 行)。上は 334/292/250×2 = 4 行、下は 18-4-2 = 12 行。
			// pokemonId を渡していないので pokemon は付かない(ADR-0602 §4)。
			name:        "raw(pokemonId なし・同速あり)",
			request:     PositionRequest{Mode: PositionModeRaw, Value: 219},
			wantSpeed:   219,
			wantPokemon: nil,
			wantFaster:  4,
			wantSlower:  12,
			wantTie:     []TableEntry{{Pokemon: p9002, Preset: PresetMaxScarf}, {Pokemon: p9002, Preset: PresetMaxPlus1}},
		},
		{
			// raw + pokemonId: 種族値 100 の 9001 を渡しても計算には使わず、value 101 がそのまま実数値
			// (101 は 9002 の無振り。9001 の種族値からは出ない値)。pokemon は表示用に解決する。
			// 101 より上: 334/292/250×2/219×2/212/167/159×2/152/146/133/120/106 = 15 行、下は 97/65 の 2 行。
			name:        "raw + pokemonId(種族値は使わない)",
			request:     PositionRequest{Mode: PositionModeRaw, PokemonID: "9001-000", Value: 101},
			wantSpeed:   101,
			wantPokemon: &p9001,
			wantFaster:  15,
			wantSlower:  2,
			wantTie:     []TableEntry{{Pokemon: p9002, Preset: PresetUninvested}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Position(positionRoster(), tt.request)
			if err != nil {
				t.Fatalf("Position error = %v", err)
			}
			if got.Speed != tt.wantSpeed {
				t.Errorf("Speed = %d, want %d", got.Speed, tt.wantSpeed)
			}
			if !reflect.DeepEqual(got.Pokemon, tt.wantPokemon) {
				t.Errorf("Pokemon = %+v, want %+v", got.Pokemon, tt.wantPokemon)
			}
			if got.Faster != tt.wantFaster {
				t.Errorf("Faster = %d, want %d", got.Faster, tt.wantFaster)
			}
			if got.Slower != tt.wantSlower {
				t.Errorf("Slower = %d, want %d", got.Slower, tt.wantSlower)
			}
			if len(got.Tie) != len(tt.wantTie) {
				t.Errorf("Tie = %+v, want %+v", got.Tie, tt.wantTie)
			} else if len(tt.wantTie) > 0 && !reflect.DeepEqual(got.Tie, tt.wantTie) {
				t.Errorf("Tie = %+v, want %+v", got.Tie, tt.wantTie)
			}
			// 位置は常に表の全行(6 プリセット)を分けたもの(ADR-0602 §3)。
			if total := got.Faster + len(got.Tie) + got.Slower; total != positionRosterRows {
				t.Errorf("faster+tie+slower = %d, want %d (6 プリセットの表の全行)", total, positionRosterRows)
			}
		})
	}
}

// TestPositionUnknownPokemon: pokemonId が roster に無ければ ErrUnknownPokemon(API では 422)。
func TestPositionUnknownPokemon(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		request PositionRequest
	}{
		{"preset", PositionRequest{Mode: PositionModePreset, PokemonID: "9999-000", Preset: PresetMax}},
		{"custom", PositionRequest{Mode: PositionModeCustom, PokemonID: "9999-000", SP: 0, Nature: NatureNeutral, Rank: 0}},
		{"raw(表示用の pokemonId も解決できなければエラー)", PositionRequest{Mode: PositionModeRaw, PokemonID: "9999-000", Value: 146}},
		{"preset で pokemonId が空", PositionRequest{Mode: PositionModePreset, PokemonID: "", Preset: PresetMax}},
		{"custom で pokemonId が空", PositionRequest{Mode: PositionModeCustom, PokemonID: "", SP: 0, Nature: NatureNeutral, Rank: 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := Position(positionRoster(), tt.request); !errors.Is(err, ErrUnknownPokemon) {
				t.Errorf("Position error = %v, want %v", err, ErrUnknownPokemon)
			}
		})
	}
}

// TestPositionRawWithoutPokemonIDIsNotUnknown: raw で pokemonId を省略したときは未知の検査をしない。
func TestPositionRawWithoutPokemonIDIsNotUnknown(t *testing.T) {
	t.Parallel()

	got, err := Position(positionRoster(), PositionRequest{Mode: PositionModeRaw, Value: 146})
	if err != nil {
		t.Fatalf("Position error = %v, want nil", err)
	}
	if got.Pokemon != nil {
		t.Errorf("Pokemon = %+v, want nil", got.Pokemon)
	}
}

// TestPositionRejectsInvalidInput: mode・プリセット・SP・性格・ランクの検証(ADR-0600 §3 の sentinel を流用)。
func TestPositionRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		request PositionRequest
		want    error
	}{
		{"mode が空", PositionRequest{PokemonID: "9001-000", Preset: PresetMax}, ErrInvalidMode},
		{"未知の mode", PositionRequest{Mode: "manual", PokemonID: "9001-000", Preset: PresetMax}, ErrInvalidMode},
		{"大文字の mode", PositionRequest{Mode: "PRESET", PokemonID: "9001-000", Preset: PresetMax}, ErrInvalidMode},
		// 最小の選択にスカーフ込みのプリセットは選べない(scarf と二重指定になるため。ADR-0602 §5)。
		{"preset に max-scarf", PositionRequest{Mode: PositionModePreset, PokemonID: "9001-000", Preset: PresetMaxScarf}, ErrNotMinimalPreset},
		{"preset に max-plus1", PositionRequest{Mode: PositionModePreset, PokemonID: "9001-000", Preset: PresetMaxPlus1}, ErrNotMinimalPreset},
		{"preset に max-plus2", PositionRequest{Mode: PositionModePreset, PokemonID: "9001-000", Preset: PresetMaxPlus2}, ErrNotMinimalPreset},
		{"preset が未知", PositionRequest{Mode: PositionModePreset, PokemonID: "9001-000", Preset: "fastest"}, ErrNotMinimalPreset},
		{"preset が空", PositionRequest{Mode: PositionModePreset, PokemonID: "9001-000", Preset: ""}, ErrNotMinimalPreset},
		{"custom の SP が上限超え", PositionRequest{Mode: PositionModeCustom, PokemonID: "9001-000", SP: engine.MaxSPPerStat + 1, Nature: NatureNeutral, Rank: 0}, ErrInvalidSP},
		{"custom の SP が負", PositionRequest{Mode: PositionModeCustom, PokemonID: "9001-000", SP: -1, Nature: NatureNeutral, Rank: 0}, ErrInvalidSP},
		{"custom の性格が未知", PositionRequest{Mode: PositionModeCustom, PokemonID: "9001-000", SP: 0, Nature: "fast", Rank: 0}, ErrInvalidNature},
		{"custom の性格が空", PositionRequest{Mode: PositionModeCustom, PokemonID: "9001-000", SP: 0, Nature: "", Rank: 0}, ErrInvalidNature},
		{"custom のランクが上限超え", PositionRequest{Mode: PositionModeCustom, PokemonID: "9001-000", SP: 0, Nature: NatureNeutral, Rank: maxRank + 1}, ErrInvalidRank},
		{"custom のランクが下限未満", PositionRequest{Mode: PositionModeCustom, PokemonID: "9001-000", SP: 0, Nature: NatureNeutral, Rank: minRank - 1}, ErrInvalidRank},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := Position(positionRoster(), tt.request); !errors.Is(err, tt.want) {
				t.Errorf("Position error = %v, want %v", err, tt.want)
			}
		})
	}
}

// TestPositionWrapsTableError: roster の種族値が不正なら表の組み立てのエラーを包んで返す(ADR-0601 §5 と同じ)。
func TestPositionWrapsTableError(t *testing.T) {
	t.Parallel()

	roster := positionRoster()
	roster.Pokemon = append(roster.Pokemon, Pokemon{PokemonID: "9009-000", NameJa: "テストフセイ", Types: []string{"normal"}, BaseSpeed: 0})
	if _, err := Position(roster, PositionRequest{Mode: PositionModeRaw, Value: 146}); !errors.Is(err, ErrInvalidBaseSpeed) {
		t.Errorf("Position error = %v, want %v", err, ErrInvalidBaseSpeed)
	}
}
