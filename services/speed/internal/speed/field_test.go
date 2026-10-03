package speed

import (
	"reflect"
	"testing"
)

// ADR-0607: 追い風・まひ・トリックルーム。
//
// 期待値は ADR-0607 §2・§3 の式から手で計算した値(実装の写しではない)。式の出典は @smogon/calc 0.12.0 の
// dist/mechanics/util.js の getFinalSpeed(Champions 世代 Generations.get(0) も同じ関数を通る)で、
// 下の値は同じ版の getFinalSpeed に種族値を差し替えた個体を通して一致を確かめてある(ADR-0607 §5)。
//
//	ランク適用後の v に、効いている補正を 4096 基準で連結してから 1 回だけ五捨五超入する:
//	  連結 M: 4096 から始めて 1 つずつ M = (M × mod + 2048) / 4096(追い風 8192 → スカーフ 6144 の順)
//	  適用 : floor((v × M + 2047) / 4096)
//	まひはその後に別枠で floor(v × 50 / 100)(4096 基準ではない。ADR-0607 §3)
//
// 種族値 71・SP 0・補正なしの実数値は 91(71 + 20)。奇数なので丸めの向きと順序の違いが値に出る。
func TestSpeedFieldEffects(t *testing.T) {
	t.Parallel()

	base71 := Input{BaseSpeed: 71, SP: 0, Nature: NatureNeutral}
	with := func(in Input, edit func(*Input)) Input {
		edit(&in)
		return in
	}
	tests := []struct {
		name string
		in   Input
		want int
	}{
		// 追い風 ×2(8192)
		{"追い風のみ 91 → 182", with(base71, func(in *Input) { in.Tailwind = true }), 182},
		// 167(最速 100 族)+1 = 250 → 追い風 500
		{"追い風はランクの後", Input{BaseSpeed: 100, SP: 32, Nature: NaturePlus, Rank: 1, Tailwind: true}, 500},

		// 追い風 + スカーフは連結して 1 回だけ丸める(M = 12288)
		// 91 × 3 = 273。スカーフを先に丸めると 136 × 2 = 272 になる(ADR-0702 §2 と同じ数値例)
		{"追い風 + スカーフ 91 → 273(272 ではない)", with(base71, func(in *Input) { in.Tailwind = true; in.Scarf = true }), 273},
		// 93 × 3 = 279。逐次なら floor(139.5 の五捨五超入 = 139) × 2 = 278
		{"追い風 + スカーフ 93 → 279(278 ではない)", Input{BaseSpeed: 73, SP: 0, Nature: NatureNeutral, Tailwind: true, Scarf: true}, 279},
		// 167 × 3 = 501。逐次なら 250 × 2 = 500
		{"追い風 + スカーフ 167 → 501(500 ではない)", Input{BaseSpeed: 100, SP: 32, Nature: NaturePlus, Tailwind: true, Scarf: true}, 501},

		// まひ ×1/2(切り捨て)
		{"まひのみ 91 → 45(切り捨て)", with(base71, func(in *Input) { in.Paralysis = true }), 45},
		{"まひのみ 167 → 83(切り捨て)", Input{BaseSpeed: 100, SP: 32, Nature: NaturePlus, Paralysis: true}, 83},
		{"まひのみ 偶数 120 → 60", Input{BaseSpeed: 100, SP: 0, Nature: NatureNeutral, Paralysis: true}, 60},
		// まひはランクの後: 91 → +2 = 182 → 91。ランク -6: 91 × 2/8 = 22 → 11
		{"まひ + ランク +2", with(base71, func(in *Input) { in.Paralysis = true; in.Rank = 2 }), 91},
		{"まひ + ランク -6", with(base71, func(in *Input) { in.Paralysis = true; in.Rank = -6 }), 11},

		// まひはスカーフの丸めの後: 91 → スカーフ 136 → 68。まひを先にすると 45 → 67(67.5 は切り捨て)
		{"スカーフ + まひ 91 → 68(67 ではない)", with(base71, func(in *Input) { in.Scarf = true; in.Paralysis = true }), 68},
		{"スカーフ + まひ 93 → 69", Input{BaseSpeed: 73, SP: 0, Nature: NatureNeutral, Scarf: true, Paralysis: true}, 69},
		// 追い風 + まひ: 182 → 91
		{"追い風 + まひ 91 → 91", with(base71, func(in *Input) { in.Tailwind = true; in.Paralysis = true }), 91},
		// 追い風 + スカーフ + まひ: 273 → 136。まひを先にすると 45 → 90 → 135
		{"追い風 + スカーフ + まひ 91 → 136(135 ではない)", with(base71, func(in *Input) { in.Tailwind = true; in.Scarf = true; in.Paralysis = true }), 136},

		// 範囲の端
		// 下限: 21 × 0.9 = 18 → ランク -6 で 18 × 2/8 = 4 → まひ 2
		{"下限の組み合わせ + まひ", Input{BaseSpeed: 1, SP: 0, Nature: NatureMinus, Rank: -6, Paralysis: true}, 2},
		// 上限: 337 → +6 で 1348 → 追い風 + スカーフ 1348 × 3 = 4044(上限の打ち切りは入れない。ADR-0607 §5)
		{"上限の組み合わせ + 追い風", Input{BaseSpeed: 255, SP: 32, Nature: NaturePlus, Rank: 6, Scarf: true, Tailwind: true}, 4044},
		{"上限の組み合わせ + 追い風 + まひ", Input{BaseSpeed: 255, SP: 32, Nature: NaturePlus, Rank: 6, Scarf: true, Tailwind: true, Paralysis: true}, 2022},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Speed(tt.in)
			if err != nil {
				t.Fatalf("Speed(%+v) error = %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("Speed(%+v) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

// TestSpeedFieldEffectsOffIsUnchanged: Tailwind・Paralysis が false(ゼロ値)なら従来の値と同じ(後方互換)。
// 既存の TestSpeed の表がそのまま通ることに加え、明示的に false を入れた入力とゼロ値の入力が一致することを確かめる。
func TestSpeedFieldEffectsOffIsUnchanged(t *testing.T) {
	t.Parallel()

	inputs := []Input{
		{BaseSpeed: 71, SP: 0, Nature: NatureNeutral},
		{BaseSpeed: 100, SP: 32, Nature: NaturePlus, Rank: 1, Scarf: true},
		{BaseSpeed: 255, SP: 32, Nature: NaturePlus, Rank: 6, Scarf: true},
	}
	for _, in := range inputs {
		explicit := in
		explicit.Tailwind = false
		explicit.Paralysis = false
		got, err := Speed(explicit)
		if err != nil {
			t.Fatalf("Speed(%+v) error = %v", explicit, err)
		}
		want, err := Speed(in)
		if err != nil {
			t.Fatalf("Speed(%+v) error = %v", in, err)
		}
		if got != want {
			t.Errorf("Speed(%+v) = %d, want %d (same as without field effects)", explicit, got, want)
		}
	}
}

// TestBuildTableTailwind: 表の追い風は全行に ×2 が掛かり、max-scarf の行はスカーフと連結して 1 回だけ丸める
// (ADR-0607 §2)。tableRoster(table_test.go)の 18 行を手計算した完全一致。
//
//	9001 種族値 68 : 176 / 240 / 264 / スカーフ 132×3=396 / +1 198×2=396 / +2 528
//	9002 種族値 100: 240 / 304 / 334 / スカーフ 167×3=501 / +1 250×2=500 / +2 668
//	9003 種族値 81 : 202 / 266 / 292 / スカーフ 146×3=438 / +1 219×2=438 / +2 584
//
// 追い風なしでは 9002 のスカーフと +1 が 250 で同速だが、追い風では 501 と 500 に分かれる
// (各補正ごとに丸めると 500 と 500 で同速のまま。連結の丸めが表の段に出る例)。
func TestBuildTableTailwind(t *testing.T) {
	t.Parallel()

	roster := tableRoster()
	p9001, p9002, p9003 := roster.Pokemon[1], roster.Pokemon[2], roster.Pokemon[0]
	all := []PresetID{"uninvested", "neutral-max", "max", "max-scarf", "max-plus1", "max-plus2"}

	got, err := BuildTable(roster, all, TableField{Tailwind: true})
	if err != nil {
		t.Fatalf("BuildTable error = %v", err)
	}
	want := Table{RegulationID: "example", Presets: all, Tiers: []Tier{
		{Speed: 668, Entries: []TableEntry{entry(p9002, "max-plus2")}},
		{Speed: 584, Entries: []TableEntry{entry(p9003, "max-plus2")}},
		{Speed: 528, Entries: []TableEntry{entry(p9001, "max-plus2")}},
		{Speed: 501, Entries: []TableEntry{entry(p9002, "max-scarf")}},
		{Speed: 500, Entries: []TableEntry{entry(p9002, "max-plus1")}},
		{Speed: 438, Entries: []TableEntry{entry(p9003, "max-scarf"), entry(p9003, "max-plus1")}},
		{Speed: 396, Entries: []TableEntry{entry(p9001, "max-scarf"), entry(p9001, "max-plus1")}},
		{Speed: 334, Entries: []TableEntry{entry(p9002, "max")}},
		{Speed: 304, Entries: []TableEntry{entry(p9002, "neutral-max")}},
		{Speed: 292, Entries: []TableEntry{entry(p9003, "max")}},
		{Speed: 266, Entries: []TableEntry{entry(p9003, "neutral-max")}},
		{Speed: 264, Entries: []TableEntry{entry(p9001, "max")}},
		{Speed: 240, Entries: []TableEntry{entry(p9001, "neutral-max"), entry(p9002, "uninvested")}},
		{Speed: 202, Entries: []TableEntry{entry(p9003, "uninvested")}},
		{Speed: 176, Entries: []TableEntry{entry(p9001, "uninvested")}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("BuildTable(tailwind) =\n%+v\nwant\n%+v", got, want)
	}
}

// TestBuildTableFieldOffIsUnchanged: TableField{} は従来の表と同じ(後方互換)。明示的な false も同じ。
func TestBuildTableFieldOffIsUnchanged(t *testing.T) {
	t.Parallel()

	all := []PresetID{"uninvested", "neutral-max", "max", "max-scarf", "max-plus1", "max-plus2"}
	zero, err := BuildTable(tableRoster(), all, TableField{})
	if err != nil {
		t.Fatalf("BuildTable error = %v", err)
	}
	explicit, err := BuildTable(tableRoster(), all, TableField{Tailwind: false, TrickRoom: false})
	if err != nil {
		t.Fatalf("BuildTable error = %v", err)
	}
	if !reflect.DeepEqual(zero, explicit) {
		t.Errorf("BuildTable(explicit false) = %+v, want %+v", explicit, zero)
	}
	if first := zero.Tiers[0].Speed; first != 334 {
		t.Errorf("first tier speed = %d, want 334 (descending, no tailwind)", first)
	}
}

// reversedTiers は tiers の段の順だけを逆にした複製(段の中の並びは変えない)。
func reversedTiers(tiers []Tier) []Tier {
	out := make([]Tier, len(tiers))
	for i, tier := range tiers {
		out[len(tiers)-1-i] = tier
	}
	return out
}

// TestBuildTableTrickRoom: トリックルームは実数値を変えず、段の順だけを逆(素早さの昇順 = 行動順)にする。
// 同速は 1 つの段のまま、段の中の並び(pokemonId の昇順 → プリセットの順)は反転しない(ADR-0607 §4)。
func TestBuildTableTrickRoom(t *testing.T) {
	t.Parallel()

	all := []PresetID{"uninvested", "neutral-max", "max", "max-scarf", "max-plus1", "max-plus2"}
	for _, tailwind := range []bool{false, true} {
		normal, err := BuildTable(tableRoster(), all, TableField{Tailwind: tailwind})
		if err != nil {
			t.Fatalf("BuildTable(tailwind=%v) error = %v", tailwind, err)
		}
		trickRoom, err := BuildTable(tableRoster(), all, TableField{Tailwind: tailwind, TrickRoom: true})
		if err != nil {
			t.Fatalf("BuildTable(tailwind=%v, trickRoom) error = %v", tailwind, err)
		}

		want := Table{RegulationID: normal.RegulationID, Presets: normal.Presets, Tiers: reversedTiers(normal.Tiers)}
		if !reflect.DeepEqual(trickRoom, want) {
			t.Errorf("BuildTable(tailwind=%v, trickRoom) =\n%+v\nwant the tiers of the normal table reversed\n%+v", tailwind, trickRoom, want)
		}
		for i := 1; i < len(trickRoom.Tiers); i++ {
			if trickRoom.Tiers[i-1].Speed >= trickRoom.Tiers[i].Speed {
				t.Errorf("tailwind=%v: tier %d speed %d is not below tier %d speed %d (want strictly ascending)",
					tailwind, i-1, trickRoom.Tiers[i-1].Speed, i, trickRoom.Tiers[i].Speed)
			}
		}
	}

	// 具体例(追い風なし): 先頭は最も遅い 88、末尾は最も速い 334。同速の 120 の段は
	// 9001 の準速 → 9002 の無振り の順のまま(段の中は反転しない)。
	got, err := BuildTable(tableRoster(), all, TableField{TrickRoom: true})
	if err != nil {
		t.Fatalf("BuildTable(trickRoom) error = %v", err)
	}
	roster := tableRoster()
	p9001, p9002 := roster.Pokemon[1], roster.Pokemon[2]
	if first := got.Tiers[0]; first.Speed != 88 || !reflect.DeepEqual(first.Entries, []TableEntry{entry(p9001, "uninvested")}) {
		t.Errorf("first tier = %+v, want 88 [9001 uninvested]", first)
	}
	if last := got.Tiers[len(got.Tiers)-1]; last.Speed != 334 {
		t.Errorf("last tier speed = %d, want 334", last.Speed)
	}
	if tie := got.Tiers[1]; tie.Speed != 101 {
		t.Errorf("second tier speed = %d, want 101", tie.Speed)
	}
	wantTie := Tier{Speed: 120, Entries: []TableEntry{entry(p9001, "neutral-max"), entry(p9002, "uninvested")}}
	if tie := got.Tiers[2]; !reflect.DeepEqual(tie, wantTie) {
		t.Errorf("third tier = %+v, want %+v (a tie stays one tier, inner order not reversed)", tie, wantTie)
	}
}

// TestPositionFieldEffects: 自分の追い風・まひ(Tailwind・Paralysis)は自分の実数値だけに、表の追い風
// (TableTailwind)は表の全行に掛かる(ADR-0607 §1)。位置は positionRoster(position_test.go)の表で数える。
//
// 追い風なしの表(position_test.go の手計算):
//
//	334(1) 292(1) 250(2) 219(2) 212(1) 167(1) 159(2) 152(1) 146(1) 133(1) 120(1) 106(1) 101(1) 97(1) 65(1)
//
// 追い風ありの表(ADR-0607 §2。max-scarf はスカーフと連結):
//
//	9001 種族値 100: 240 / 304 / 334 / 501 / 500 / 668
//	9002 種族値  81: 202 / 266 / 292 / 438 / 438 / 584
//	9003 種族値  45: 130 / 194 / 212 / 318 / 318 / 424
func TestPositionFieldEffects(t *testing.T) {
	t.Parallel()

	p9001 := positionPokemon(t, "9001-000")
	p9003 := positionPokemon(t, "9003-000")

	tests := []struct {
		name       string
		req        PositionRequest
		wantSpeed  int
		wantFaster int
		wantSlower int
		wantTie    []TableEntry
	}{
		{
			// 167 → 追い風 334 = 9001 の最速+2 と同速
			name:      "自分の追い風",
			req:       PositionRequest{Mode: PositionModePreset, PokemonID: "9001-000", Preset: PresetMax, Tailwind: true},
			wantSpeed: 334, wantFaster: 0, wantSlower: 17, wantTie: []TableEntry{{Pokemon: p9001, Preset: PresetMaxPlus2}},
		},
		{
			// 167 × 3 = 501(連結)。表のどの行より速い
			name:      "自分の追い風 + スカーフ",
			req:       PositionRequest{Mode: PositionModePreset, PokemonID: "9001-000", Preset: PresetMax, Scarf: true, Tailwind: true},
			wantSpeed: 501, wantFaster: 0, wantSlower: positionRosterRows, wantTie: []TableEntry{},
		},
		{
			// 167 → まひ 83。65 だけが遅い
			name:      "自分のまひ(custom)",
			req:       PositionRequest{Mode: PositionModeCustom, PokemonID: "9001-000", SP: 32, Nature: NaturePlus, Rank: 0, Paralysis: true},
			wantSpeed: 83, wantFaster: 17, wantSlower: 1, wantTie: []TableEntry{},
		},
		{
			// 167 → スカーフ 250 → まひ 125(まひを先にすると 83 → 124)
			name:      "自分のスカーフ + まひ",
			req:       PositionRequest{Mode: PositionModePreset, PokemonID: "9001-000", Preset: PresetMax, Scarf: true, Paralysis: true},
			wantSpeed: 125, wantFaster: 13, wantSlower: 5, wantTie: []TableEntry{},
		},
		{
			// 自分は 167 のまま、表だけ追い風。130 だけが遅い
			name:      "表の追い風",
			req:       PositionRequest{Mode: PositionModePreset, PokemonID: "9001-000", Preset: PresetMax, TableTailwind: true},
			wantSpeed: 167, wantFaster: 17, wantSlower: 1, wantTie: []TableEntry{},
		},
		{
			// 両側に追い風: 334 = 追い風の表の 9001 最速(334)と同速。速いのは 501/500/668/438/438/584/424 の 7 行
			name:      "自分と表の両方に追い風",
			req:       PositionRequest{Mode: PositionModePreset, PokemonID: "9001-000", Preset: PresetMax, Tailwind: true, TableTailwind: true},
			wantSpeed: 334, wantFaster: 7, wantSlower: 10, wantTie: []TableEntry{{Pokemon: p9001, Preset: PresetMax}},
		},
		{
			// raw も表の追い風は効く: 212 = 追い風の表の 9003 最速(106 × 2)と同速
			name:      "raw + 表の追い風",
			req:       PositionRequest{Mode: PositionModeRaw, Value: 212, TableTailwind: true},
			wantSpeed: 212, wantFaster: 14, wantSlower: 3, wantTie: []TableEntry{{Pokemon: p9003, Preset: PresetMax}},
		},
		{
			// raw の value はそのまま使う(自分の追い風・まひは掛けない。HTTP 層は 400 で拒否する。ADR-0607 §1)
			name: "raw は自分の追い風・まひを掛けない",
			req:  PositionRequest{Mode: PositionModeRaw, Value: 212, Tailwind: true, Paralysis: true},
			// 追い風なしの表で 212 = 9003 の最速+2。速いのは 334/292/250×2/219×2 の 6 行
			wantSpeed: 212, wantFaster: 6, wantSlower: 11, wantTie: []TableEntry{{Pokemon: p9003, Preset: PresetMaxPlus2}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Position(positionRoster(), tt.req)
			if err != nil {
				t.Fatalf("Position(%+v) error = %v", tt.req, err)
			}
			if got.Speed != tt.wantSpeed || got.Faster != tt.wantFaster || got.Slower != tt.wantSlower {
				t.Errorf("Position(%+v) = speed %d faster %d slower %d, want speed %d faster %d slower %d",
					tt.req, got.Speed, got.Faster, got.Slower, tt.wantSpeed, tt.wantFaster, tt.wantSlower)
			}
			if !reflect.DeepEqual(got.Tie, tt.wantTie) {
				t.Errorf("Position(%+v).Tie = %+v, want %+v", tt.req, got.Tie, tt.wantTie)
			}
			if total := got.Faster + got.Slower + len(got.Tie); total != positionRosterRows {
				t.Errorf("faster + slower + tie = %d, want %d", total, positionRosterRows)
			}
		})
	}
}
