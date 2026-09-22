package speed

import (
	"errors"
	"testing"

	"example.com/pokecalc/engine"
)

// 期待値は ADR-0600 §3 の式から手で計算した値(実装の写しではない)。
//
//	実数値 = floor((種族値 + 20 + SP) × 性格)  性格: 上昇 ×11/10、下降 ×9/10(floor)
//	ランク +n: floor(v × (2+n) / 2)、-n: floor(v × 2 / (2+n))
//	スカーフ(ランクの後): floor((v × 6144 + 2047) / 4096)  (×1.5 の端数 0.5 ちょうどは切り捨て)
func TestSpeed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   Input
		want int
	}{
		// 性格・SP の境界(種族値 100)
		{"無振り SP0 補正なし", Input{BaseSpeed: 100, SP: 0, Nature: NatureNeutral}, 120},  // 100+20+0 = 120
		{"SP0 下降", Input{BaseSpeed: 100, SP: 0, Nature: NatureMinus}, 108},          // 120×0.9 = 108
		{"SP0 上昇", Input{BaseSpeed: 100, SP: 0, Nature: NaturePlus}, 132},           // 120×1.1 = 132
		{"準速 SP32 補正なし", Input{BaseSpeed: 100, SP: 32, Nature: NatureNeutral}, 152}, // 100+20+32 = 152
		{"SP32 下降は切り捨て", Input{BaseSpeed: 100, SP: 32, Nature: NatureMinus}, 136},   // 152×0.9 = 136.8 → 136
		{"最速 SP32 上昇は切り捨て", Input{BaseSpeed: 100, SP: 32, Nature: NaturePlus}, 167}, // 152×1.1 = 167.2 → 167
		{"上昇の端数 .9 も切り捨て", Input{BaseSpeed: 139, SP: 0, Nature: NaturePlus}, 174},   // 159×1.1 = 174.9 → 174
		{"種族値の下限 1", Input{BaseSpeed: 1, SP: 0, Nature: NatureNeutral}, 21},         // 1+20 = 21
		{"種族値の上限 255 最速", Input{BaseSpeed: 255, SP: 32, Nature: NaturePlus}, 337},   // 307×1.1 = 337.7 → 337

		// ランク(最速 167 を基準)
		{"ランク +1", Input{BaseSpeed: 100, SP: 32, Nature: NaturePlus, Rank: 1}, 250},  // 167×3/2 = 250.5 → 250
		{"ランク +2", Input{BaseSpeed: 100, SP: 32, Nature: NaturePlus, Rank: 2}, 334},  // 167×4/2 = 334
		{"ランク +6", Input{BaseSpeed: 100, SP: 32, Nature: NaturePlus, Rank: 6}, 668},  // 167×8/2 = 668
		{"ランク -1", Input{BaseSpeed: 100, SP: 32, Nature: NaturePlus, Rank: -1}, 111}, // 167×2/3 = 111.33 → 111
		{"ランク -6", Input{BaseSpeed: 100, SP: 32, Nature: NaturePlus, Rank: -6}, 41},  // 167×2/8 = 41.75 → 41
		{"ランク 0 は変えない", Input{BaseSpeed: 100, SP: 32, Nature: NaturePlus, Rank: 0}, 167},
		{"下限の組み合わせ", Input{BaseSpeed: 1, SP: 0, Nature: NatureMinus, Rank: -6}, 4}, // 21×0.9 = 18.9 → 18、18×2/8 = 4.5 → 4

		// こだわりスカーフ(×6144/4096 の五捨五超入)
		{"スカーフ 偶数 200 → 300", Input{BaseSpeed: 180, SP: 0, Nature: NatureNeutral, Scarf: true}, 300},  // 200×1.5 = 300
		{"スカーフ 奇数 201 → 301", Input{BaseSpeed: 181, SP: 0, Nature: NatureNeutral, Scarf: true}, 301},  // 201×1.5 = 301.5 → 0.5 ちょうどは切り捨て
		{"スカーフ 奇数 167 → 250", Input{BaseSpeed: 100, SP: 32, Nature: NaturePlus, Scarf: true}, 250},    // 167×1.5 = 250.5 → 250
		{"スカーフ 偶数 152 → 228", Input{BaseSpeed: 100, SP: 32, Nature: NatureNeutral, Scarf: true}, 228}, // 152×1.5 = 228
		{"スカーフ 奇数 21 → 31", Input{BaseSpeed: 1, SP: 0, Nature: NatureNeutral, Scarf: true}, 31},       // 21×1.5 = 31.5 → 31

		// ランクの後にスカーフが掛かる
		{"ランク +1 とスカーフ", Input{BaseSpeed: 100, SP: 32, Nature: NaturePlus, Rank: 1, Scarf: true}, 375}, // 250(ランク)×1.5 = 375
		// 201: ランク +2 → 402、スカーフ → 603。逆順(スカーフ 301 → ランク 602)なら 602 になる
		{"ランク +2 の後にスカーフ", Input{BaseSpeed: 181, SP: 0, Nature: NatureNeutral, Rank: 2, Scarf: true}, 603},
		// 200: ランク -1 → 133(133.33)、スカーフ → 199(199.5 は切り捨て)。逆順(300 → 200)なら 200 になる
		{"ランク -1 の後にスカーフ", Input{BaseSpeed: 180, SP: 0, Nature: NatureNeutral, Rank: -1, Scarf: true}, 199},
		// 337 → ランク +6 → 1348 → スカーフ → 2022
		{"上限の組み合わせ", Input{BaseSpeed: 255, SP: 32, Nature: NaturePlus, Rank: 6, Scarf: true}, 2022},
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

func TestSpeedRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	valid := Input{BaseSpeed: 100, SP: 32, Nature: NatureNeutral}
	with := func(edit func(*Input)) Input {
		in := valid
		edit(&in)
		return in
	}
	tests := []struct {
		name    string
		in      Input
		wantErr error
	}{
		{"種族値 0", with(func(in *Input) { in.BaseSpeed = 0 }), ErrInvalidBaseSpeed},
		{"種族値 256", with(func(in *Input) { in.BaseSpeed = 256 }), ErrInvalidBaseSpeed},
		{"種族値 負", with(func(in *Input) { in.BaseSpeed = -1 }), ErrInvalidBaseSpeed},
		{"SP -1", with(func(in *Input) { in.SP = -1 }), ErrInvalidSP},
		{"SP 上限+1", with(func(in *Input) { in.SP = engine.MaxSPPerStat + 1 }), ErrInvalidSP},
		{"ランク -7", with(func(in *Input) { in.Rank = -7 }), ErrInvalidRank},
		{"ランク 7", with(func(in *Input) { in.Rank = 7 }), ErrInvalidRank},
		{"性格 空", with(func(in *Input) { in.Nature = "" }), ErrInvalidNature},
		{"性格 大文字", with(func(in *Input) { in.Nature = "PLUS" }), ErrInvalidNature},
		{"性格 未知", with(func(in *Input) { in.Nature = "up" }), ErrInvalidNature},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Speed(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Speed(%+v) = (%d, %v), want errors.Is(_, %v)", tt.in, got, err, tt.wantErr)
			}
			if got != 0 {
				t.Errorf("Speed(%+v) = %d on error, want 0", tt.in, got)
			}
		})
	}
}

// TestSpeedAcceptsBoundaries は範囲の端が拒否されないことを確かめる(検証の差し間違いの検出)。
func TestSpeedAcceptsBoundaries(t *testing.T) {
	t.Parallel()

	tests := []Input{
		{BaseSpeed: 1, SP: 0, Nature: NatureMinus, Rank: -6},
		{BaseSpeed: 255, SP: engine.MaxSPPerStat, Nature: NaturePlus, Rank: 6, Scarf: true},
	}
	for _, in := range tests {
		if _, err := Speed(in); err != nil {
			t.Errorf("Speed(%+v) error = %v, want nil", in, err)
		}
	}
}
