package domain

import (
	"errors"
	"reflect"
	"testing"

	"example.com/pokecalc/engine"
)

// I-speed-4 / ADR-0610 の受け入れ条件のテスト。期待値はすべて手計算(式は docs/speed-design.md §4):
//   補正なし = B + 20 + SP、上昇 = floor((B+20+SP) × 11/10)、下降 = floor((B+20+SP) × 9/10)、
//   ランク +1 = floor(v × 3/2)、スカーフ = floor((v × 6144 + 2047) / 4096)、追い風 ×2、まひ = floor(v × 50/100)。

func spr(minSP, maxSP int) *SPRange { return &SPRange{Min: minSP, Max: maxSP} }

func br(minBase, maxBase int) *BaseRange { return &BaseRange{Min: minBase, Max: maxBase} }

func lines(minus, minusTie, neutral, neutralTie, plus, plusTie *SPRange) []NatureLine {
	return []NatureLine{
		{Nature: NatureMinus, MovesFirst: minus, Tie: minusTie},
		{Nature: NatureNeutral, MovesFirst: neutral, Tie: neutralTie},
		{Nature: NaturePlus, MovesFirst: plus, Tie: plusTie},
	}
}

func TestMovesFirstLines(t *testing.T) {
	tests := []struct {
		name string
		in   LineInput
		want []NatureLine
	}{
		{
			// 補正なし: 120+SP > 150 → SP ≧ 31、同速は SP 30。
			// 上昇: SP 17 で floor(137×1.1)=150(同速)、SP 18 で 151。下降: 最大 floor(152×0.9)=136 で届かない。
			name: "base 100 vs 150, no trick room",
			in:   LineInput{SelfBaseSpeed: 100, OpponentSpeed: 150},
			want: lines(nil, nil, spr(31, 32), spr(30, 30), spr(18, 32), spr(17, 17)),
		},
		{
			// トリックルーム: 自分 < 150 が上を取る。下降はどの SP でも 136 以下で常に下を取れる。
			name: "base 100 vs 150, trick room",
			in:   LineInput{SelfBaseSpeed: 100, OpponentSpeed: 150, TrickRoom: true},
			want: lines(spr(0, 32), nil, spr(0, 29), spr(30, 30), spr(0, 16), spr(17, 17)),
		},
		{
			// 無振り・下降でも floor(150×0.9)=135 > 100。全員 SP 0 から上を取れる端。
			name: "always first: base 130 vs 100",
			in:   LineInput{SelfBaseSpeed: 130, OpponentSpeed: 100},
			want: lines(spr(0, 32), nil, spr(0, 32), nil, spr(0, 32), nil),
		},
		{
			// 最速でも floor(82×1.1)=90 < 200。どの性格・SP でも上を取れない端。
			name: "never first: base 30 vs 200",
			in:   LineInput{SelfBaseSpeed: 30, OpponentSpeed: 200},
			want: lines(nil, nil, nil, nil, nil, nil),
		},
		{
			// トリックルームで「どの SP でも下を取れない」端: 下降 SP 0 でも floor(150×0.9)=135 > 100。
			name: "trick room never first: base 130 vs 100",
			in:   LineInput{SelfBaseSpeed: 130, OpponentSpeed: 100, TrickRoom: true},
			want: lines(nil, nil, nil, nil, nil, nil),
		},
		{
			// スカーフ: 実数値 133 → 199、134 → 201(200 になる実数値が無いので同速なし)。
			// 補正なし SP 14 / 上昇 SP 2(floor(122×1.1)=134)/ 下降 SP 29(floor(149×0.9)=134)。
			name: "scarf: base 100 vs 200",
			in:   LineInput{SelfBaseSpeed: 100, Self: Condition{Scarf: true}, OpponentSpeed: 200},
			want: lines(spr(29, 32), nil, spr(14, 32), nil, spr(2, 32), nil),
		},
		{
			// ランク +1: 実数値 120 → 180(同速)、121 → 181。補正なし SP 21 / 同速 SP 20。
			// 上昇: floor(110×1.1)=121 で SP 10。120 になる SP は無い。下降: 最大 118 → 177 で届かない。
			name: "rank +1: base 80 vs 180",
			in:   LineInput{SelfBaseSpeed: 80, Self: Condition{Rank: 1}, OpponentSpeed: 180},
			want: lines(nil, nil, spr(21, 32), spr(20, 20), spr(10, 32), nil),
		},
		{
			// まひ: floor((120+SP)/2)。70 になる SP は 20 と 21 の 2 つ(同速の範囲が 1 点でない例)。
			// 上昇: SP 8 → 140 → 70、SP 9 → 141 → 70、SP 10 → 143 → 71。下降: 最大 136 → 68 で届かない。
			name: "paralysis: base 100 vs 70",
			in:   LineInput{SelfBaseSpeed: 100, Self: Condition{Paralysis: true}, OpponentSpeed: 70},
			want: lines(nil, nil, spr(22, 32), spr(20, 21), spr(10, 32), spr(8, 9)),
		},
		{
			// 追い風 + まひ: ×2 の後に /2 で元の実数値に戻る(まひは連結の後。ADR-0607 §3)。
			// 下降: floor(145×0.9)=130(同速)、floor(146×0.9)=131。上昇は SP 0 で 132。
			name: "tailwind + paralysis: base 100 vs 130",
			in:   LineInput{SelfBaseSpeed: 100, Self: Condition{Tailwind: true, Paralysis: true}, OpponentSpeed: 130},
			want: lines(spr(26, 32), spr(25, 25), spr(11, 32), spr(10, 10), spr(0, 32), nil),
		},
		{
			// 下降補正は 10 刻みで同じ実数値が続く: floor(130×0.9)=floor(131×0.9)=117 → 同速が SP 0〜1。
			name: "minus nature tie spans two SP: base 110 vs 117",
			in:   LineInput{SelfBaseSpeed: 110, OpponentSpeed: 117},
			want: lines(spr(2, 32), spr(0, 1), spr(0, 32), nil, spr(0, 32), nil),
		},
		{
			// 同じ入力のトリックルーム: 下降の最小 117 は 117 未満にならないので下を取れない(同速は残る)。
			name: "minus nature tie spans two SP, trick room",
			in:   LineInput{SelfBaseSpeed: 110, OpponentSpeed: 117, TrickRoom: true},
			want: lines(nil, spr(0, 1), nil, nil, nil, nil),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MovesFirstLines(tt.in)
			if err != nil {
				t.Fatalf("MovesFirstLines(%+v) error = %v", tt.in, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MovesFirstLines(%+v)\n got  %s\n want %s", tt.in, fmtLines(got), fmtLines(tt.want))
			}
		})
	}
}

func fmtLines(ls []NatureLine) string {
	s := ""
	for _, l := range ls {
		s += string(l.Nature) + ":" + fmtRange(l.MovesFirst) + "/tie" + fmtRange(l.Tie) + " "
	}
	return s
}

func fmtRange(r *SPRange) string {
	if r == nil {
		return "nil"
	}
	return "[" + itoa(r.Min) + "," + itoa(r.Max) + "]"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

// TestMovesFirstLinesMatchesBruteForce は、範囲の答えが「Speed で全 SP を計算して engine.CompareSpeed で比べた」結果と
// 一致することを、種族値・相手の素早さ・条件・トリックルームの格子で確かめる(独立の導出。同じ式を 2 つ作らない確認)。
func TestMovesFirstLinesMatchesBruteForce(t *testing.T) {
	bases := []int{1, 45, 100, 130, 255}
	opponents := []int{2, 50, 120, 151, 200, 337, 1000}
	conds := []Condition{{}, {Rank: -6}, {Rank: 2, Scarf: true}, {Tailwind: true}, {Paralysis: true, Scarf: true}}
	natures := []NatureEffect{NatureMinus, NatureNeutral, NaturePlus}
	for _, base := range bases {
		for _, opp := range opponents {
			for _, c := range conds {
				for _, tr := range []bool{false, true} {
					in := LineInput{SelfBaseSpeed: base, Self: c, OpponentSpeed: opp, TrickRoom: tr}
					got, err := MovesFirstLines(in)
					if err != nil {
						t.Fatalf("MovesFirstLines(%+v) error = %v", in, err)
					}
					if len(got) != len(natures) {
						t.Fatalf("MovesFirstLines(%+v) returned %d lines, want %d", in, len(got), len(natures))
					}
					for i, n := range natures {
						var first, tie []int
						for sp := 0; sp <= engine.MaxSPPerStat; sp++ {
							v, err := Speed(Input{BaseSpeed: base, SP: sp, Nature: n, Rank: c.Rank, Scarf: c.Scarf, Tailwind: c.Tailwind, Paralysis: c.Paralysis})
							if err != nil {
								t.Fatal(err)
							}
							switch engine.CompareSpeed(v, opp, tr) {
							case engine.TurnOrderSelfFirst:
								first = append(first, sp)
							case engine.TurnOrderSpeedTie:
								tie = append(tie, sp)
							}
						}
						if got[i].Nature != n {
							t.Fatalf("line %d nature = %s, want %s", i, got[i].Nature, n)
						}
						assertSPSet(t, in, "movesFirst "+string(n), got[i].MovesFirst, first)
						assertSPSet(t, in, "tie "+string(n), got[i].Tie, tie)
					}
				}
			}
		}
	}
}

func assertSPSet(t *testing.T, in LineInput, label string, got *SPRange, want []int) {
	t.Helper()
	if len(want) == 0 {
		if got != nil {
			t.Errorf("%+v %s = %s, want nil", in, label, fmtRange(got))
		}
		return
	}
	w := SPRange{Min: want[0], Max: want[len(want)-1]}
	if w.Max-w.Min+1 != len(want) {
		t.Fatalf("%+v %s: brute force set %v is not contiguous (monotonicity broken)", in, label, want)
	}
	if got == nil || *got != w {
		t.Errorf("%+v %s = %s, want %s", in, label, fmtRange(got), fmtRange(&w))
	}
}

func TestMovesFirstLinesRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		in   LineInput
		want error
	}{
		{"base 0", LineInput{SelfBaseSpeed: 0, OpponentSpeed: 100}, ErrInvalidBaseSpeed},
		{"base 256", LineInput{SelfBaseSpeed: 256, OpponentSpeed: 100}, ErrInvalidBaseSpeed},
		{"rank +7", LineInput{SelfBaseSpeed: 100, Self: Condition{Rank: 7}, OpponentSpeed: 100}, ErrInvalidRank},
		{"rank -7", LineInput{SelfBaseSpeed: 100, Self: Condition{Rank: -7}, OpponentSpeed: 100}, ErrInvalidRank},
		{"opponent speed 0", LineInput{SelfBaseSpeed: 100, OpponentSpeed: 0}, ErrInvalidOpponentSpeed},
		{"opponent speed negative", LineInput{SelfBaseSpeed: 100, OpponentSpeed: -1}, ErrInvalidOpponentSpeed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := MovesFirstLines(tt.in); !errors.Is(err, tt.want) {
				t.Errorf("MovesFirstLines(%+v) error = %v, want %v", tt.in, err, tt.want)
			}
		})
	}
}

// matchupRoster は架空の 3 体(testdata/pokemon.example.json の一部と同じ値)。
func matchupRoster() Roster {
	return Roster{RegulationID: "example", Pokemon: []Pokemon{
		{PokemonID: "9004-000", NameJa: "テストデンキツネビ", Types: []string{"electric"}, BaseSpeed: 130},
		{PokemonID: "9001-000", NameJa: "テストカソウドリ", Types: []string{"fire", "flying"}, BaseSpeed: 100},
		{PokemonID: "9003-000", NameJa: "テストイワゴロンタ", Types: []string{"rock", "ground"}, BaseSpeed: 45},
	}}
}

func TestDefaultOpponentAssumptions(t *testing.T) {
	want := []OpponentAssumption{
		{Preset: PresetUninvested}, {Preset: PresetNeutralMax}, {Preset: PresetMax}, {Preset: PresetMaxScarf},
	}
	if got := DefaultOpponentAssumptions(); !reflect.DeepEqual(got, want) {
		t.Errorf("DefaultOpponentAssumptions() = %+v, want %+v", got, want)
	}
}

func TestMatchupDefaultAssumptions(t *testing.T) {
	// 相手 130 族: 無振り 150・準速 182・最速 floor(182×1.1)=200・最速スカーフ 300。
	res, err := Matchup(matchupRoster(), MatchupRequest{SelfPokemonID: "9001-000", OpponentPokemonID: "9004-000"})
	if err != nil {
		t.Fatalf("Matchup error = %v", err)
	}
	if res.Self.PokemonID != "9001-000" || res.Opponent.PokemonID != "9004-000" {
		t.Errorf("self/opponent = %s/%s, want 9001-000/9004-000", res.Self.PokemonID, res.Opponent.PokemonID)
	}
	gotSpeeds := make([]int, len(res.Lines))
	for i, l := range res.Lines {
		gotSpeeds[i] = l.OpponentSpeed
		if l.Current != nil {
			t.Errorf("line %d Current = %v, want nil without SelfCurrent", i, *l.Current)
		}
	}
	if want := []int{150, 182, 200, 300}; !reflect.DeepEqual(gotSpeeds, want) {
		t.Fatalf("opponent speeds = %v, want %v", gotSpeeds, want)
	}
	if res.SelfSpeed != 0 {
		t.Errorf("SelfSpeed = %d, want 0 without SelfCurrent", res.SelfSpeed)
	}
	// 無振り(150)に対しては TestMovesFirstLines の「base 100 vs 150」と同じ。
	if want := lines(nil, nil, spr(31, 32), spr(30, 30), spr(18, 32), spr(17, 17)); !reflect.DeepEqual(res.Lines[0].Lines, want) {
		t.Errorf("vs uninvested: got %s want %s", fmtLines(res.Lines[0].Lines), fmtLines(want))
	}
	// 準速(182)以上には 100 族はどう振っても届かない(最速 167)。
	if want := lines(nil, nil, nil, nil, nil, nil); !reflect.DeepEqual(res.Lines[1].Lines, want) {
		t.Errorf("vs neutral-max: got %s want %s", fmtLines(res.Lines[1].Lines), fmtLines(want))
	}
}

func TestMatchupCurrentTurnOrder(t *testing.T) {
	order := func(o engine.TurnOrder) *engine.TurnOrder { return &o }
	tests := []struct {
		name      string
		current   Investment
		trickRoom bool
		wantSpeed int
		want      []*engine.TurnOrder // 無振り 150・準速 182
	}{
		{"max 167", Investment{SP: 32, Nature: NaturePlus}, false, 167,
			[]*engine.TurnOrder{order(engine.TurnOrderSelfFirst), order(engine.TurnOrderOpponentFirst)}},
		{"max 167 trick room", Investment{SP: 32, Nature: NaturePlus}, true, 167,
			[]*engine.TurnOrder{order(engine.TurnOrderOpponentFirst), order(engine.TurnOrderSelfFirst)}},
		{"neutral SP30 = 150 tie", Investment{SP: 30, Nature: NatureNeutral}, false, 150,
			[]*engine.TurnOrder{order(engine.TurnOrderSpeedTie), order(engine.TurnOrderOpponentFirst)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cur := tt.current
			res, err := Matchup(matchupRoster(), MatchupRequest{
				SelfPokemonID: "9001-000", SelfCurrent: &cur, OpponentPokemonID: "9004-000",
				Assumptions: []OpponentAssumption{{Preset: PresetUninvested}, {Preset: PresetNeutralMax}},
				TrickRoom:   tt.trickRoom,
			})
			if err != nil {
				t.Fatalf("Matchup error = %v", err)
			}
			if res.SelfSpeed != tt.wantSpeed {
				t.Errorf("SelfSpeed = %d, want %d", res.SelfSpeed, tt.wantSpeed)
			}
			if len(res.Lines) != len(tt.want) {
				t.Fatalf("len(Lines) = %d, want %d", len(res.Lines), len(tt.want))
			}
			for i, w := range tt.want {
				if res.Lines[i].Current == nil || *res.Lines[i].Current != *w {
					t.Errorf("line %d Current = %v, want %v", i, res.Lines[i].Current, *w)
				}
			}
		})
	}
}

func TestMatchupOpponentEffects(t *testing.T) {
	tests := []struct {
		name string
		req  MatchupRequest
		want int
	}{
		{
			// 相手 45 族・無振り 65 に相手側の追い風 → 130。
			name: "opponent tailwind",
			req: MatchupRequest{SelfPokemonID: "9001-000", OpponentPokemonID: "9003-000",
				Assumptions: []OpponentAssumption{{Preset: PresetUninvested}}, OpponentTailwind: true},
			want: 130,
		},
		{
			// 相手 130 族・最速 200 にまひ → 100。
			name: "opponent paralysis",
			req: MatchupRequest{SelfPokemonID: "9001-000", OpponentPokemonID: "9004-000",
				Assumptions: []OpponentAssumption{{Preset: PresetMax}}, OpponentParalysis: true},
			want: 100,
		},
		{
			// 相手 130 族・SP 0・下降・ランク -1: floor(150×0.9)=135 → floor(135×2/3)=90。
			name: "opponent custom",
			req: MatchupRequest{SelfPokemonID: "9001-000", OpponentPokemonID: "9004-000",
				Assumptions: []OpponentAssumption{{Custom: &OpponentCustom{SP: 0, Nature: NatureMinus, Rank: -1}}}},
			want: 90,
		},
		{
			// 相手 130 族・最速+1: floor(200×1.5)=300。
			name: "opponent max-plus1 preset",
			req: MatchupRequest{SelfPokemonID: "9001-000", OpponentPokemonID: "9004-000",
				Assumptions: []OpponentAssumption{{Preset: PresetMaxPlus1}}},
			want: 300,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := Matchup(matchupRoster(), tt.req)
			if err != nil {
				t.Fatalf("Matchup error = %v", err)
			}
			if len(res.Lines) != 1 || res.Lines[0].OpponentSpeed != tt.want {
				t.Fatalf("lines = %+v, want one line with opponent speed %d", res.Lines, tt.want)
			}
		})
	}
}

func TestMatchupRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		req  MatchupRequest
		want error
	}{
		{"unknown self", MatchupRequest{SelfPokemonID: "9999-000", OpponentPokemonID: "9004-000"}, ErrUnknownPokemon},
		{"unknown opponent", MatchupRequest{SelfPokemonID: "9001-000", OpponentPokemonID: "9999-000"}, ErrUnknownPokemon},
		{"unknown preset", MatchupRequest{SelfPokemonID: "9001-000", OpponentPokemonID: "9004-000",
			Assumptions: []OpponentAssumption{{Preset: "fastest"}}}, ErrUnknownPreset},
		{"duplicate preset", MatchupRequest{SelfPokemonID: "9001-000", OpponentPokemonID: "9004-000",
			Assumptions: []OpponentAssumption{{Preset: PresetMax}, {Preset: PresetMax}}}, ErrDuplicatePreset},
		{"assumption with both", MatchupRequest{SelfPokemonID: "9001-000", OpponentPokemonID: "9004-000",
			Assumptions: []OpponentAssumption{{Preset: PresetMax, Custom: &OpponentCustom{Nature: NatureNeutral}}}}, ErrInvalidAssumption},
		{"assumption with neither", MatchupRequest{SelfPokemonID: "9001-000", OpponentPokemonID: "9004-000",
			Assumptions: []OpponentAssumption{{}}}, ErrInvalidAssumption},
		{"custom SP 33", MatchupRequest{SelfPokemonID: "9001-000", OpponentPokemonID: "9004-000",
			Assumptions: []OpponentAssumption{{Custom: &OpponentCustom{SP: 33, Nature: NatureNeutral}}}}, ErrInvalidSP},
		{"self rank 7", MatchupRequest{SelfPokemonID: "9001-000", Self: Condition{Rank: 7}, OpponentPokemonID: "9004-000"}, ErrInvalidRank},
		{"current nature", MatchupRequest{SelfPokemonID: "9001-000", SelfCurrent: &Investment{SP: 0, Nature: "up"}, OpponentPokemonID: "9004-000"}, ErrInvalidNature},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Matchup(matchupRoster(), tt.req); !errors.Is(err, tt.want) {
				t.Errorf("Matchup error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestReachBaseRanges(t *testing.T) {
	tests := []struct {
		name     string
		v        int
		tailwind bool
		want     map[PresetID][3]*BaseRange // slower, tie, faster
	}{
		{
			// v = 170。無振り B+20、準速 B+52、最速 floor((B+52)×1.1)(B=103 で 170)、
			// 最速スカーフ・最速+1 は最速 113 → 169 / 114 → 171(同速なし。B=51 が 113)、最速+2 は 2×最速(B=26 で 85×2=170)。
			name: "v 170",
			v:    170,
			want: map[PresetID][3]*BaseRange{
				PresetUninvested: {br(1, 149), br(150, 150), br(151, 255)},
				PresetNeutralMax: {br(1, 117), br(118, 118), br(119, 255)},
				PresetMax:        {br(1, 102), br(103, 103), br(104, 255)},
				PresetMaxScarf:   {br(1, 51), nil, br(52, 255)},
				PresetMaxPlus1:   {br(1, 51), nil, br(52, 255)},
				PresetMaxPlus2:   {br(1, 25), br(26, 26), br(27, 255)},
			},
		},
		{
			// 全員に抜かれる端: 無振り 1 族でも 21 > 2。
			name: "v 2: everyone faster",
			v:    2,
			want: map[PresetID][3]*BaseRange{
				PresetUninvested: {nil, nil, br(1, 255)},
				PresetNeutralMax: {nil, nil, br(1, 255)},
				PresetMax:        {nil, nil, br(1, 255)},
				PresetMaxScarf:   {nil, nil, br(1, 255)},
				PresetMaxPlus1:   {nil, nil, br(1, 255)},
				PresetMaxPlus2:   {nil, nil, br(1, 255)},
			},
		},
		{
			// 全員を抜く端: 最速+2 の 255 族でも 2×337=674 < 5000。
			name: "v 5000: everyone slower",
			v:    5000,
			want: map[PresetID][3]*BaseRange{
				PresetUninvested: {br(1, 255), nil, nil},
				PresetNeutralMax: {br(1, 255), nil, nil},
				PresetMax:        {br(1, 255), nil, nil},
				PresetMaxScarf:   {br(1, 255), nil, nil},
				PresetMaxPlus1:   {br(1, 255), nil, nil},
				PresetMaxPlus2:   {br(1, 255), nil, nil},
			},
		},
		{
			// 相手側の追い風: 無振り 2×(B+20)。B=65 で 170(同速)。準速 2×(B+52)=170 → B=33。
			name: "v 170 opponent tailwind",
			v:    170, tailwind: true,
			want: map[PresetID][3]*BaseRange{
				PresetUninvested: {br(1, 64), br(65, 65), br(66, 255)},
				PresetNeutralMax: {br(1, 32), br(33, 33), br(34, 255)},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Reach(matchupRoster(), tt.v, tt.tailwind)
			if err != nil {
				t.Fatalf("Reach error = %v", err)
			}
			wantOrder := []PresetID{PresetUninvested, PresetNeutralMax, PresetMax, PresetMaxScarf, PresetMaxPlus1, PresetMaxPlus2}
			if len(got) != len(wantOrder) {
				t.Fatalf("len = %d, want %d", len(got), len(wantOrder))
			}
			for i, line := range got {
				if line.Preset != wantOrder[i] {
					t.Fatalf("line %d preset = %s, want %s", i, line.Preset, wantOrder[i])
				}
				w, ok := tt.want[line.Preset]
				if !ok {
					continue
				}
				gotR := [3]*BaseRange{line.Slower, line.Tie, line.Faster}
				if !reflect.DeepEqual(gotR, w) {
					t.Errorf("%s: got slower=%v tie=%v faster=%v, want %v %v %v",
						line.Preset, line.Slower, line.Tie, line.Faster, w[0], w[1], w[2])
				}
			}
		})
	}
}

// TestReachMatchesBruteForce は族の区切りが、全種族値を Speed で計算して比べた結果と一致することを確かめる。
func TestReachMatchesBruteForce(t *testing.T) {
	for _, v := range []int{1, 21, 100, 152, 167, 200, 301, 505, 674, 675, 2000} {
		for _, tw := range []bool{false, true} {
			got, err := Reach(matchupRoster(), v, tw)
			if err != nil {
				t.Fatalf("Reach(%d, %v) error = %v", v, tw, err)
			}
			for _, line := range got {
				def, _ := presetByID(line.Preset)
				var slower, tie, faster []int
				for b := minBaseSpeed; b <= maxBaseSpeed; b++ {
					s, err := Speed(Input{BaseSpeed: b, SP: def.SP, Nature: def.Nature, Rank: def.Rank, Scarf: def.Scarf, Tailwind: tw})
					if err != nil {
						t.Fatal(err)
					}
					switch {
					case s < v:
						slower = append(slower, b)
					case s == v:
						tie = append(tie, b)
					default:
						faster = append(faster, b)
					}
				}
				check := func(label string, gotR *BaseRange, want []int) {
					if len(want) == 0 {
						if gotR != nil {
							t.Errorf("v=%d tw=%v %s %s = %v, want nil", v, tw, line.Preset, label, *gotR)
						}
						return
					}
					w := BaseRange{Min: want[0], Max: want[len(want)-1]}
					if gotR == nil || *gotR != w {
						t.Errorf("v=%d tw=%v %s %s = %v, want %v", v, tw, line.Preset, label, gotR, w)
					}
				}
				check("slower", line.Slower, slower)
				check("tie", line.Tie, tie)
				check("faster", line.Faster, faster)
			}
		}
	}
}

func TestReachRosterExamples(t *testing.T) {
	roster := Roster{RegulationID: "example", Pokemon: []Pokemon{
		{PokemonID: "9001-000", BaseSpeed: 100}, {PokemonID: "9002-000", BaseSpeed: 81},
		{PokemonID: "9003-000", BaseSpeed: 45}, {PokemonID: "9004-000", BaseSpeed: 130},
		{PokemonID: "9005-000", BaseSpeed: 81}, {PokemonID: "9006-000", BaseSpeed: 60},
		{PokemonID: "9007-000", BaseSpeed: 30}, {PokemonID: "9008-000", BaseSpeed: 110},
	}}
	// v = 167 の最速: 99 族以下が遅い・100 族が同速(floor(152×1.1)=167)・101 族以上が速い。
	got, err := Reach(roster, 167, false)
	if err != nil {
		t.Fatalf("Reach error = %v", err)
	}
	var maxLine ReachLine
	for _, l := range got {
		if l.Preset == PresetMax {
			maxLine = l
		}
	}
	if maxLine.SlowerCount != 5 || maxLine.TieCount != 1 || maxLine.FasterCount != 2 {
		t.Errorf("counts = %d/%d/%d, want 5/1/2", maxLine.SlowerCount, maxLine.TieCount, maxLine.FasterCount)
	}
	// 遅い中で最も速いのは 81 族が 2 体 → pokemonId の昇順の先頭 9002-000。速い中で最も遅いのは 110 族の 9008-000。
	if maxLine.FastestSlower == nil || maxLine.FastestSlower.PokemonID != "9002-000" {
		t.Errorf("FastestSlower = %+v, want 9002-000", maxLine.FastestSlower)
	}
	if maxLine.SlowestFaster == nil || maxLine.SlowestFaster.PokemonID != "9008-000" {
		t.Errorf("SlowestFaster = %+v, want 9008-000", maxLine.SlowestFaster)
	}

	// 全員より速い端: 例は片側だけ。
	got, err = Reach(roster, 5000, false)
	if err != nil {
		t.Fatalf("Reach error = %v", err)
	}
	for _, l := range got {
		if l.SlowerCount != 8 || l.TieCount != 0 || l.FasterCount != 0 || l.SlowestFaster != nil || l.FastestSlower == nil || l.FastestSlower.PokemonID != "9004-000" {
			t.Errorf("v=5000 %s: %+v, want all 8 slower and fastest slower 9004-000", l.Preset, l)
		}
	}
}

func TestReachRejectsInvalidSpeed(t *testing.T) {
	for _, v := range []int{0, -1} {
		if _, err := Reach(matchupRoster(), v, false); !errors.Is(err, ErrInvalidSpeedValue) {
			t.Errorf("Reach(%d) error = %v, want ErrInvalidSpeedValue", v, err)
		}
	}
}
