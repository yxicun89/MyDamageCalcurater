package engine

import (
	"errors"
	"fmt"
	"math"
	"testing"
)

// 調整の指数・HP ラインのテスト(ADR-0800 §2〜§4)。
// 種族値は stats_test.go と同じ攻撃寄りの架空の配分(HP108/Atk130/Def95/SpA80/SpD85/Spe102)。
// 実数値は RealStats の式: HP = 種族値 + 75 + SP、その他 = floor((種族値 + 20 + SP) × 性格補正)。
var adjustBase = Stats{HP: 108, Atk: 130, Def: 95, SpA: 80, SpD: 85, Spe: 102}

var (
	natureAtkUp   = Nature{Plus: StatAtk, Minus: StatSpA} // A↑C↓
	natureAtkDown = Nature{Plus: StatSpA, Minus: StatAtk} // C↑A↓
	natureDefUp   = Nature{Plus: StatDef, Minus: StatSpA} // B↑C↓
	natureSpDUp   = Nature{Plus: StatSpD, Minus: StatSpA} // D↑C↓
	natureSpDDown = Nature{Plus: StatAtk, Minus: StatSpD} // A↑D↓
)

func TestAdjustFirepowerIndex(t *testing.T) {
	// 火力指数 = floor(攻撃実数値 × 威力 × 補正 / 4096)。ランク補正は含めない。
	tests := []struct {
		name     string
		sp       Stats
		nature   Nature
		ranks    Ranks
		category MoveCategory
		power    int
		modifier int
		want     int
	}{
		{"物理 SP0 無補正 等倍", Stats{}, NatureNeutral, Ranks{}, CategoryPhysical, 100, Modifier4096, 15000},                       // A=150
		{"物理 SP0 A↑", Stats{}, natureAtkUp, Ranks{}, CategoryPhysical, 100, Modifier4096, 16500},                             // A=floor(150×1.1)=165
		{"物理 SP0 A↓", Stats{}, natureAtkDown, Ranks{}, CategoryPhysical, 100, Modifier4096, 13500},                           // A=floor(150×0.9)=135
		{"物理 SP32 無補正", Stats{Atk: 32}, NatureNeutral, Ranks{}, CategoryPhysical, 100, Modifier4096, 18200},                  // A=182
		{"物理 SP32 A↑", Stats{Atk: 32}, natureAtkUp, Ranks{}, CategoryPhysical, 100, Modifier4096, 20000},                     // A=floor(182×1.1)=200
		{"物理 SP31 A↑(上限の直前)", Stats{Atk: 31}, natureAtkUp, Ranks{}, CategoryPhysical, 100, Modifier4096, 19900},              // A=floor(181×1.1)=199
		{"物理 SP32 A↓", Stats{Atk: 32}, natureAtkDown, Ranks{}, CategoryPhysical, 100, Modifier4096, 16300},                   // A=floor(182×0.9)=163
		{"補正 ×1.5(6144)", Stats{}, NatureNeutral, Ranks{}, CategoryPhysical, 100, 6144, 22500},                               // 150×100×1.5
		{"端数は最後に floor", Stats{}, natureAtkUp, Ranks{}, CategoryPhysical, 75, 6144, 18562},                                   // 165×75×1.5=18562.5
		{"4096 で割り切れない補正", Stats{Atk: 32}, natureAtkUp, Ranks{}, CategoryPhysical, 100, 5324, 25996},                         // 20000×5324/4096=25996.09
		{"特殊は C を使う", Stats{}, NatureNeutral, Ranks{}, CategorySpecial, 90, Modifier4096, 9000},                              // C=100
		{"特殊 C↓(A↑C↓)", Stats{}, natureAtkUp, Ranks{}, CategorySpecial, 90, Modifier4096, 8100},                              // C=floor(100×0.9)=90
		{"ランク補正は含めない", Stats{}, NatureNeutral, Ranks{Atk: 2}, CategoryPhysical, 100, Modifier4096, 15000},                    // A=150(+2 を無視)
		{"ランク -6 も無視", Stats{}, NatureNeutral, Ranks{Atk: -6}, CategoryPhysical, 100, Modifier4096, 15000},                   // A=150(-6 を無視)
		{"補正が上限 MaxEffectModifier", Stats{}, NatureNeutral, Ranks{}, CategoryPhysical, 100, MaxEffectModifier, 7680000},      // 150×100×(512×4096)/4096=150×100×512
		{"SP 合計ちょうど 66", Stats{HP: 32, Atk: 32, Def: 2}, NatureNeutral, Ranks{}, CategoryPhysical, 100, Modifier4096, 18200}, // 32+32+2=66, A=130+20+32=182
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FirepowerIndex(indiv(adjustBase, tt.sp, tt.nature, tt.ranks), tt.category, tt.power, tt.modifier)
			if err != nil {
				t.Fatalf("err=%v want nil", err)
			}
			if got != tt.want {
				t.Errorf("FirepowerIndex=%d want %d", got, tt.want)
			}
		})
	}
}

func TestAdjustFirepowerIndexRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name     string
		sp       Stats
		category MoveCategory
		power    int
		modifier int
	}{
		{"変化技は指数を持たない", Stats{}, CategoryStatus, 100, Modifier4096},
		{"分類が空", Stats{}, MoveCategory(""), 100, Modifier4096},
		{"威力 0", Stats{}, CategoryPhysical, 0, Modifier4096},
		{"威力が負", Stats{}, CategoryPhysical, -1, Modifier4096},
		{"補正 0", Stats{}, CategoryPhysical, 100, 0},
		{"補正が負", Stats{}, CategoryPhysical, 100, -4096},
		{"補正が上限超過", Stats{}, CategoryPhysical, 100, MaxEffectModifier + 1},
		{"SP が 1 能力の上限超過", Stats{Atk: MaxSPPerStat + 1}, CategoryPhysical, 100, Modifier4096},
		{"SP 合計が上限超過", Stats{HP: 32, Atk: 32, Def: 3}, CategoryPhysical, 100, Modifier4096},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FirepowerIndex(indiv(adjustBase, tt.sp, NatureNeutral, Ranks{}), tt.category, tt.power, tt.modifier)
			if !errors.Is(err, ErrInvalidAdjustInput) {
				t.Errorf("err=%v want ErrInvalidAdjustInput", err)
			}
		})
	}
}

func TestAdjustBulkIndex(t *testing.T) {
	// 耐久指数 = floor(H × B(D) × 4096 / 被ダメージ補正)。ランク補正は含めない。
	tests := []struct {
		name           string
		sp             Stats
		nature         Nature
		ranks          Ranks
		category       MoveCategory
		damageModifier int
		want           int
	}{
		{"物理 SP0 無補正 等倍", Stats{}, NatureNeutral, Ranks{}, CategoryPhysical, Modifier4096, 21045},                        // H=183, B=115
		{"物理 H32 B32 B↑", Stats{HP: 32, Def: 32}, natureDefUp, Ranks{}, CategoryPhysical, Modifier4096, 34615},           // H=215, B=floor(147×1.1)=161
		{"被ダメージ ×0.75 で割る", Stats{HP: 32, Def: 32}, natureDefUp, Ranks{}, CategoryPhysical, 3072, 46153},                 // 34615×4096/3072=46153.3
		{"被ダメージ半減で指数2倍", Stats{HP: 32, Def: 32}, natureDefUp, Ranks{}, CategoryPhysical, ModifierHalf, 69230},            // 34615×2
		{"被ダメージ ×1.5 で割る", Stats{}, NatureNeutral, Ranks{}, CategoryPhysical, 6144, 14030},                               // 21045×4096/6144
		{"4096 で割り切れない補正", Stats{}, NatureNeutral, Ranks{}, CategoryPhysical, 5325, 16187},                               // 21045×4096/5325=16187.85
		{"特殊は D を使う", Stats{}, NatureNeutral, Ranks{}, CategorySpecial, Modifier4096, 19215},                             // H=183, D=105
		{"特殊 D↓", Stats{}, natureSpDDown, Ranks{}, CategorySpecial, Modifier4096, 17202},                                 // D=floor(105×0.9)=94
		{"特殊 H32 D32 D↑", Stats{HP: 32, SpD: 32}, natureSpDUp, Ranks{}, CategorySpecial, Modifier4096, 32250},            // H=215, D=floor((85+20+32)×1.1)=floor(137×1.1)=150
		{"B↑は特殊耐久に効かない", Stats{}, natureDefUp, Ranks{}, CategorySpecial, Modifier4096, 19215},                            // D=105
		{"ランク補正は含めない", Stats{}, NatureNeutral, Ranks{Def: 6}, CategoryPhysical, Modifier4096, 21045},                     // B=115(+6 を無視)
		{"ランク -6 も無視", Stats{}, NatureNeutral, Ranks{Def: -6}, CategoryPhysical, Modifier4096, 21045},                    // B=115(-6 を無視)
		{"被ダメージ補正が下限 MinEffectModifier", Stats{}, NatureNeutral, Ranks{}, CategoryPhysical, MinEffectModifier, 86200320}, // 183×115×4096/1=21045×4096
		{"SP 合計ちょうど 66", Stats{HP: 32, Def: 32, SpD: 2}, NatureNeutral, Ranks{}, CategoryPhysical, Modifier4096, 31605},  // 32+32+2=66, H=215, B=95+20+32=147, 215×147=31605
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BulkIndex(indiv(adjustBase, tt.sp, tt.nature, tt.ranks), tt.category, tt.damageModifier)
			if err != nil {
				t.Fatalf("err=%v want nil", err)
			}
			if got != tt.want {
				t.Errorf("BulkIndex=%d want %d", got, tt.want)
			}
		})
	}
}

func TestAdjustBulkIndexRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name           string
		sp             Stats
		category       MoveCategory
		damageModifier int
	}{
		{"変化技は指数を持たない", Stats{}, CategoryStatus, Modifier4096},
		{"分類が空", Stats{}, MoveCategory(""), Modifier4096},
		{"補正 0", Stats{}, CategoryPhysical, 0},
		{"補正が負", Stats{}, CategoryPhysical, -4096},
		{"補正が上限超過", Stats{}, CategoryPhysical, MaxEffectModifier + 1},
		{"SP が 1 能力の上限超過", Stats{HP: MaxSPPerStat + 1}, CategoryPhysical, Modifier4096},
		{"SP 合計が上限超過", Stats{HP: 32, Def: 32, SpD: 3}, CategoryPhysical, Modifier4096},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BulkIndex(indiv(adjustBase, tt.sp, NatureNeutral, Ranks{}), tt.category, tt.damageModifier)
			if !errors.Is(err, ErrInvalidAdjustInput) {
				t.Errorf("err=%v want ErrInvalidAdjustInput", err)
			}
		})
	}
}

// hpLinePoint はテストの期待値を短く書くための補助。
func hpLinePoint(hp, sp, delta int) *HPLinePoint {
	return &HPLinePoint{HP: hp, SP: sp, SPDelta: delta}
}

func formatHPLinePoint(p *HPLinePoint) string {
	if p == nil {
		return "nil"
	}
	return fmt.Sprintf("{HP:%d SP:%d SPDelta:%d}", p.HP, p.SP, p.SPDelta)
}

func TestAdjustHPLines(t *testing.T) {
	// HP = 種族値 + 75 + SP(SP 0..32)。16n は HP mod 16 == 0、16n-1 は HP mod 16 == 15。
	// 次 = SP が現在より大きい中で最小、前 = SP が現在より小さい中で最大。範囲外は nil。
	tests := []struct {
		name          string
		baseHP        int
		hpSP          int
		want          HPLineReport
		next16n       *HPLinePoint
		prev16n       *HPLinePoint
		next16nMinus1 *HPLinePoint
		prev16nMinus1 *HPLinePoint
	}{
		// 種族値108: HP 183..215。16n = 192(SP9)・208(SP25)、16n-1 = 191(SP8)・207(SP24)。
		{
			name: "SP0 ラインに属さない", baseHP: 108, hpSP: 0,
			want:    HPLineReport{HP: 183, SP: 0, Current: HPLineNone},
			next16n: hpLinePoint(192, 9, 9), prev16n: nil,
			next16nMinus1: hpLinePoint(191, 8, 8), prev16nMinus1: nil,
		},
		{
			name: "16n-1 のライン上(16n の直前)", baseHP: 108, hpSP: 8,
			want:    HPLineReport{HP: 191, SP: 8, Current: HPLine16nMinus1},
			next16n: hpLinePoint(192, 9, 1), prev16n: nil,
			next16nMinus1: hpLinePoint(207, 24, 16), prev16nMinus1: nil,
		},
		{
			name: "16n のライン上", baseHP: 108, hpSP: 9,
			want:    HPLineReport{HP: 192, SP: 9, Current: HPLine16n},
			next16n: hpLinePoint(208, 25, 16), prev16n: nil,
			next16nMinus1: hpLinePoint(207, 24, 15), prev16nMinus1: hpLinePoint(191, 8, -1),
		},
		{
			name: "16n の直後", baseHP: 108, hpSP: 10,
			want:    HPLineReport{HP: 193, SP: 10, Current: HPLineNone},
			next16n: hpLinePoint(208, 25, 15), prev16n: hpLinePoint(192, 9, -1),
			next16nMinus1: hpLinePoint(207, 24, 14), prev16nMinus1: hpLinePoint(191, 8, -2),
		},
		{
			name: "16n-1 の直前", baseHP: 108, hpSP: 23,
			want:    HPLineReport{HP: 206, SP: 23, Current: HPLineNone},
			next16n: hpLinePoint(208, 25, 2), prev16n: hpLinePoint(192, 9, -14),
			next16nMinus1: hpLinePoint(207, 24, 1), prev16nMinus1: hpLinePoint(191, 8, -15),
		},
		{
			name: "SP32 ラインに属さない(次は範囲外)", baseHP: 108, hpSP: 32,
			want:    HPLineReport{HP: 215, SP: 32, Current: HPLineNone},
			next16n: nil, prev16n: hpLinePoint(208, 25, -7),
			next16nMinus1: nil, prev16nMinus1: hpLinePoint(207, 24, -8),
		},
		// 種族値101: HP 176..208。SP0 が 16n(176=16×11)。前のラインは SP が負になるので nil。
		{
			name: "SP0 で 16n のライン上", baseHP: 101, hpSP: 0,
			want:    HPLineReport{HP: 176, SP: 0, Current: HPLine16n},
			next16n: hpLinePoint(192, 16, 16), prev16n: nil,
			next16nMinus1: hpLinePoint(191, 15, 15), prev16nMinus1: nil,
		},
		{
			name: "SP32 で 16n のライン上", baseHP: 101, hpSP: 32,
			want:    HPLineReport{HP: 208, SP: 32, Current: HPLine16n},
			next16n: nil, prev16n: hpLinePoint(192, 16, -16),
			next16nMinus1: nil, prev16nMinus1: hpLinePoint(207, 31, -1),
		},
		// 種族値100: HP 175..207。SP0 と SP32 がどちらも 16n-1。
		{
			name: "SP0 で 16n-1 のライン上", baseHP: 100, hpSP: 0,
			want:    HPLineReport{HP: 175, SP: 0, Current: HPLine16nMinus1},
			next16n: hpLinePoint(176, 1, 1), prev16n: nil,
			next16nMinus1: hpLinePoint(191, 16, 16), prev16nMinus1: nil,
		},
		{
			name: "SP32 で 16n-1 のライン上", baseHP: 100, hpSP: 32,
			want:    HPLineReport{HP: 207, SP: 32, Current: HPLine16nMinus1},
			next16n: nil, prev16n: hpLinePoint(192, 17, -15),
			next16nMinus1: nil, prev16nMinus1: hpLinePoint(191, 16, -16),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HPLines(tt.baseHP, tt.hpSP)
			if err != nil {
				t.Fatalf("err=%v want nil", err)
			}
			if got.HP != tt.want.HP || got.SP != tt.want.SP || got.Current != tt.want.Current {
				t.Errorf("HP=%d SP=%d Current=%q want HP=%d SP=%d Current=%q",
					got.HP, got.SP, got.Current, tt.want.HP, tt.want.SP, tt.want.Current)
			}
			checks := []struct {
				label     string
				got, want *HPLinePoint
			}{
				{"Next16n", got.Next16n, tt.next16n},
				{"Prev16n", got.Prev16n, tt.prev16n},
				{"Next16nMinus1", got.Next16nMinus1, tt.next16nMinus1},
				{"Prev16nMinus1", got.Prev16nMinus1, tt.prev16nMinus1},
			}
			for _, c := range checks {
				if formatHPLinePoint(c.got) != formatHPLinePoint(c.want) {
					t.Errorf("%s=%s want %s", c.label, formatHPLinePoint(c.got), formatHPLinePoint(c.want))
				}
			}
		})
	}
}

func TestAdjustHPLinesConsistentWithRealStats(t *testing.T) {
	// HP は性格補正を受けず、RealStats と同じ式で求まる(実数値の式を二重に持たないことの確認)。
	for sp := 0; sp <= MaxSPPerStat; sp++ {
		got, err := HPLines(adjustBase.HP, sp)
		if err != nil {
			t.Fatalf("SP=%d err=%v", sp, err)
		}
		want := RealStats(indiv(adjustBase, Stats{HP: sp}, natureAtkUp, Ranks{})).HP
		if got.HP != want {
			t.Errorf("SP=%d HP=%d want %d", sp, got.HP, want)
		}
	}
}

func TestAdjustHPLinesRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name   string
		baseHP int
		hpSP   int
	}{
		{"SP が負", 108, -1},
		{"SP が上限超過", 108, MaxSPPerStat + 1},
		{"種族値が下限未満", MinBaseStat - 1, 0},
		{"種族値が上限超過", MaxBaseStat + 1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := HPLines(tt.baseHP, tt.hpSP)
			if !errors.Is(err, ErrInvalidAdjustInput) {
				t.Errorf("err=%v want ErrInvalidAdjustInput", err)
			}
		})
	}
}

func TestAdjustFirepowerIndexPowerBounds(t *testing.T) {
	// 威力が int32 の上限を超えたら拒否する(乗算のオーバーフロー防止)。
	if _, err := FirepowerIndex(indiv(adjustBase, Stats{}, NatureNeutral, Ranks{}), CategoryPhysical, math.MaxInt32+1, Modifier4096); !errors.Is(err, ErrInvalidAdjustInput) {
		t.Errorf("威力 MaxInt32+1: err=%v want ErrInvalidAdjustInput", err)
	}
	// 最大の入力でもオーバーフローせず厳密な値になる。
	// 種族値255・Atk SP32・A↑: A=floor((255+20+32)×1.1)=floor(307×1.1)=337。
	// 期待値 = 337 × MaxInt32 × MaxEffectModifier / 4096(int64 で計算)。
	base := Stats{HP: 100, Atk: 255, Def: 100, SpA: 100, SpD: 100, Spe: 100}
	got, err := FirepowerIndex(indiv(base, Stats{Atk: 32}, natureAtkUp, Ranks{}), CategoryPhysical, math.MaxInt32, MaxEffectModifier)
	if err != nil {
		t.Fatalf("err=%v want nil", err)
	}
	want := int64(337) * math.MaxInt32 * MaxEffectModifier / Modifier4096
	if int64(got) != want {
		t.Errorf("FirepowerIndex=%d want %d", got, want)
	}
}

func TestAdjustHPLinesBaseStatBounds(t *testing.T) {
	// 種族値の下限 1: HP=76+SP。SP0 は 76(76 mod 16 = 12 でライン外)。
	// 16n は 80(SP4)、16n-1 は 79(SP3)。
	low, err := HPLines(MinBaseStat, 0)
	if err != nil {
		t.Fatalf("MinBaseStat err=%v want nil", err)
	}
	if low.HP != 76 || low.Current != HPLineNone ||
		formatHPLinePoint(low.Next16n) != formatHPLinePoint(hpLinePoint(80, 4, 4)) ||
		formatHPLinePoint(low.Next16nMinus1) != formatHPLinePoint(hpLinePoint(79, 3, 3)) {
		t.Errorf("MinBaseStat: %+v", low)
	}
	// 種族値の上限 255: HP=330+SP。SP0 は 330(330 mod 16 = 10 でライン外)。
	// 16n は 336(SP6)、16n-1 は 335(SP5)。
	high, err := HPLines(MaxBaseStat, 0)
	if err != nil {
		t.Fatalf("MaxBaseStat err=%v want nil", err)
	}
	if high.HP != 330 || high.Current != HPLineNone ||
		formatHPLinePoint(high.Next16n) != formatHPLinePoint(hpLinePoint(336, 6, 6)) ||
		formatHPLinePoint(high.Next16nMinus1) != formatHPLinePoint(hpLinePoint(335, 5, 5)) {
		t.Errorf("MaxBaseStat: %+v", high)
	}
}
