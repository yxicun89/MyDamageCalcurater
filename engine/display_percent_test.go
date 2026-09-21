package engine

// P1-11「表示%の分離」の受け入れ条件をテストで固定する。ADR-0010 §3 が定義の正。
//
// 2種類の%を混ぜないことがこのタスクの目的:
//   - 観測%  (Observation.Percent / PercentTenths): 逆算の入力。精度付きの値(P1-12 で区間モデル。§R2)
//   - 表示%  (DisplayPercentTenths*)       : アプリが画面に出す値。0.1% 単位の整数(§3.2)
//
// 期待値は実装の写しではなく、手計算できる小さな例で固定する。
// 例: damage=1, maxHP=16 → 1000/16 = 62.5(0.1%単位)。切り捨て 62 = 6.2%、四捨五入 63 = 6.3%。

import "testing"

// ---------------------------------------------------------------------------
// AC-1: 表示%は 0.1% 単位の整数で、最小側は切り捨てる(ADR-0010 §3.2)
// ---------------------------------------------------------------------------

func TestDisplayPercentTenthsFloor(t *testing.T) {
	tests := []struct {
		name   string
		damage int
		maxHP  int
		want   int // 0.1% 単位
	}{
		// 手計算: 1000*damage/maxHP を切り捨てる
		{"ちょうど .5 は切り捨てる(62.5 → 62 = 6.2%)", 1, 16, 62},
		{"ちょうど .5 は切り捨てる(187.5 → 187 = 18.7%)", 3, 16, 187},
		{"割り切れる(1000/200 = 5 = 0.5%)", 1, 200, 5},
		{"ちょうど 50.0%", 100, 200, 500},
		{"ちょうど 100.0%(damage = HP)", 155, 155, 1000},
		{"HP155 の 114 ダメージ(735.48… → 73.5%)", 114, 155, 735},
		{"HP3 の 2 ダメージ(666.66… → 66.6%)", 2, 3, 666},
		{"HP3 の 1 ダメージ(333.33… → 33.3%)", 1, 3, 333},
		{"HP9 の 7 ダメージ(777.77… → 77.7%)", 7, 9, 777},
		{"HP714 の 1 ダメージ(1.40… → 0.1%)", 1, 714, 1},
		{"HP714 の 5 ダメージ(7.00… → 0.7%)", 5, 714, 7},
		{"0 ダメージは 0", 0, 175, 0},
		{"damage > maxHP は 100% を超えたまま返す(137.0%)", 137, 100, 1370},
		{"maxHP が 0 なら 0(ゼロ除算しない)", 10, 0, 0},
		{"maxHP が負でも 0", 10, -5, 0},
		{"damage が負なら 0(切り捨ての向きを壊さない)", -5, 100, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DisplayPercentTenthsFloor(tt.damage, tt.maxHP); got != tt.want {
				t.Errorf("DisplayPercentTenthsFloor(%d, %d) = %d, want %d(0.1%%単位)",
					tt.damage, tt.maxHP, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC-2: 最大側は 0.1% 単位で四捨五入する(ADR-0010 §3.2)
// ---------------------------------------------------------------------------

func TestDisplayPercentTenthsRound(t *testing.T) {
	tests := []struct {
		name   string
		damage int
		maxHP  int
		want   int // 0.1% 単位
	}{
		// 手計算: 1000*damage/maxHP を四捨五入(round-half-up)
		{"ちょうど .5 は切り上げる(62.5 → 63 = 6.3%)", 1, 16, 63},
		{"ちょうど .5 は切り上げる(187.5 → 188 = 18.8%)", 3, 16, 188},
		{"割り切れる(1000/200 = 5 = 0.5%)", 1, 200, 5},
		{"ちょうど 50.0%", 100, 200, 500},
		{"ちょうど 100.0%(damage = HP)", 155, 155, 1000},
		{"HP155 の 114 ダメージ(735.48… → 73.5%)", 114, 155, 735},
		{"HP3 の 2 ダメージ(666.66… → 66.7%)", 2, 3, 667},
		{"HP3 の 1 ダメージ(333.33… → 33.3%)", 1, 3, 333},
		{"HP9 の 7 ダメージ(777.77… → 77.8%)", 7, 9, 778},
		{"HP714 の 1 ダメージ(1.40… → 0.1%)", 1, 714, 1},
		{"HP714 の 5 ダメージ(7.00… → 0.7%)", 5, 714, 7},
		{"0 ダメージは 0", 0, 175, 0},
		{"damage > maxHP は 100% を超えたまま返す(137.0%)", 137, 100, 1370},
		{"maxHP が 0 なら 0(ゼロ除算しない)", 10, 0, 0},
		{"maxHP が負でも 0", 10, -5, 0},
		{"damage が負なら 0", -5, 100, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DisplayPercentTenthsRound(tt.damage, tt.maxHP); got != tt.want {
				t.Errorf("DisplayPercentTenthsRound(%d, %d) = %d, want %d(0.1%%単位)",
					tt.damage, tt.maxHP, got, tt.want)
			}
		})
	}
}

// TestDisplayPercentFloorNeverExceedsRound は、切り捨てと四捨五入の関係
// floor <= round <= floor+1 が定義域全体で成り立つことを確かめる(ADR-0010 §3.2)。
// 「最低これくらい入る」を保守的に示すため、最小側が最大側を上回ってはならない。
func TestDisplayPercentFloorNeverExceedsRound(t *testing.T) {
	for maxHP := 1; maxHP <= 400; maxHP++ {
		for damage := 0; damage <= maxHP+20; damage++ {
			f := DisplayPercentTenthsFloor(damage, maxHP)
			r := DisplayPercentTenthsRound(damage, maxHP)
			if f > r || r > f+1 {
				t.Fatalf("damage=%d maxHP=%d: floor=%d round=%d(floor <= round <= floor+1 が崩れた)",
					damage, maxHP, f, r)
			}
			// 0.1% 単位の整数であり、100*damage/maxHP の 10 倍の近傍にいること。
			if want := damage * 1000 / maxHP; f != want {
				t.Fatalf("damage=%d maxHP=%d: floor=%d want %d", damage, maxHP, f, want)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// AC-3: 結果からダメージ幅の表示%を取る正規の入口(ADR-0010 §3.2)
// ---------------------------------------------------------------------------

func TestDamageResultDisplayPercentRangeTenths(t *testing.T) {
	rolls := func(min, max int) [16]int {
		var r [16]int
		for i := range r {
			r[i] = min + (max-min)*i/15
		}
		r[0], r[15] = min, max
		return r
	}

	tests := []struct {
		name            string
		result          DamageResult
		wantMin         int
		wantMax         int
		wantMinFromRule string
	}{
		{
			name:    "最小側は切り捨て・最大側は四捨五入(HP16、1〜3ダメージ)",
			result:  DamageResult{Rolls: rolls(1, 3), DefenderHP: 16},
			wantMin: 62,  // 62.5 → 切り捨て 6.2%
			wantMax: 188, // 187.5 → 四捨五入 18.8%
		},
		{
			name:    "乱数幅の両端だけを見る(HP155、108〜114ダメージ)",
			result:  DamageResult{Rolls: rolls(108, 114), DefenderHP: 155},
			wantMin: 696, // 696.77… → 69.6%
			wantMax: 735, // 735.48… → 73.5%
		},
		{
			name:    "最小と最大が同じダメージでも、丸めが違うので 0.1 ずれることがある",
			result:  DamageResult{Rolls: rolls(1, 1), DefenderHP: 16},
			wantMin: 62, // 6.2%
			wantMax: 63, // 6.3%(ADR-0010 §3.2 の既知の帰結。特別扱いしない)
		},
		{
			name:    "確定1発(ダメージが HP を超える)は 100% を超えたまま返す",
			result:  DamageResult{Rolls: rolls(137, 162), DefenderHP: 100},
			wantMin: 1370, // 137.0%
			wantMax: 1620, // 162.0%
		},
		{
			name:    "無効(全ロール 0)は 0.0%",
			result:  DamageResult{Rolls: rolls(0, 0), DefenderHP: 155},
			wantMin: 0,
			wantMax: 0,
		},
		{
			name:    "DefenderHP が 0 でも panic しない",
			result:  DamageResult{Rolls: rolls(10, 12), DefenderHP: 0},
			wantMin: 0,
			wantMax: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMin, gotMax := tt.result.DisplayPercentRangeTenths()
			if gotMin != tt.wantMin || gotMax != tt.wantMax {
				t.Errorf("DisplayPercentRangeTenths() = (%d, %d), want (%d, %d)",
					gotMin, gotMax, tt.wantMin, tt.wantMax)
			}
			// 幅の両端は rolls[0] / rolls[15] からのみ決まる(中間ロールを見ない)。
			if want := DisplayPercentTenthsFloor(tt.result.MinDamage(), tt.result.DefenderHP); gotMin != want {
				t.Errorf("最小側が DisplayPercentTenthsFloor(rolls[0]) と違う: %d != %d", gotMin, want)
			}
			if want := DisplayPercentTenthsRound(tt.result.MaxDamage(), tt.result.DefenderHP); gotMax != want {
				t.Errorf("最大側が DisplayPercentTenthsRound(rolls[15]) と違う: %d != %d", gotMax, want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC-4: 表示%と観測%は別物。表示%は整数%を経由しない(ADR-0010 §3)
// ---------------------------------------------------------------------------

// TestDisplayPercentIsNotDerivedFromObservedPercent は、
// 表示%が「整数%に丸めてから小数にした値」ではないことを、両者が食い違う例で示す。
// P1-12 で ObservedPercent(整数%への round-half-up)は削除し、観測%は区間モデル
// (Observation.Matches。ADR-0010 §R2)になった。ここでは「その整数%の観測と両立する」ことを見る。
// requirements.md §2「HP 比率から直接求め、整数%に丸めてから表示しない」。
func TestDisplayPercentIsNotDerivedFromObservedPercent(t *testing.T) {
	tests := []struct {
		name         string
		damage       int
		maxHP        int
		wantObserved int // 整数%(四捨五入で読んだ観測。区間モデルと両立すること)
		wantFloor    int // 0.1% 単位の切り捨て
	}{
		// 73.548…% : 整数%は 74、表示%(最小側)は 73.5。74.0 になってはいけない。
		{"HP155 の 114 ダメージ", 114, 155, 74, 735},
		// 49.714…% : 整数%は 50、表示%は 49.7。
		{"HP175 の 87 ダメージ", 87, 175, 50, 497},
		// 48.309…% : 整数%は 48、表示%は 48.3。
		{"HP207 の 100 ダメージ", 100, 207, 48, 483},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if o := (Observation{Percent: tt.wantObserved}); !o.Matches(tt.damage, tt.maxHP) {
				t.Errorf("%+v.Matches(%d, %d) = false, want true", o, tt.damage, tt.maxHP)
			}
			got := DisplayPercentTenthsFloor(tt.damage, tt.maxHP)
			if got != tt.wantFloor {
				t.Errorf("DisplayPercentTenthsFloor(%d, %d) = %d, want %d",
					tt.damage, tt.maxHP, got, tt.wantFloor)
			}
			if got == tt.wantObserved*10 {
				t.Errorf("表示%%が整数%%の 10 倍になっている(整数%%に丸めてから小数にしている): %d", got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC-5: 乱数n発の確率は、どのケースでも表示できる値になる(ADR-0010 §3.4)
// ---------------------------------------------------------------------------

func TestKOChanceDisplayChancePercentTenths(t *testing.T) {
	tests := []struct {
		name string
		ko   KOChance
		want int // 0.1% 単位
	}{
		{"倒せない(Hits=0)は 0", KOChance{Hits: 0}, 0},
		{"確定n発は 100.0%(生値が 0 でも 0% と出さない)",
			KOChance{Hits: 2, Guaranteed: true}, 1000},
		{"確定1発も 100.0%", KOChance{Hits: 1, Guaranteed: true}, 1000},
		{"乱数 37.5% は 37.5%", KOChance{Hits: 3, ChancePercent: 37.5}, 375},
		{"乱数 6.25% は 0.1% 単位で四捨五入(62.5 → 63 = 6.3%)",
			KOChance{Hits: 3, ChancePercent: 6.25}, 63},
		{"乱数 82.51953125% → 82.5%", KOChance{Hits: 2, ChancePercent: 82.51953125}, 825},
		{"乱数 2.734375% → 2.7%", KOChance{Hits: 2, ChancePercent: 2.734375}, 27},
		{"乱数 68.75% → 68.8%(687.5 を切り上げ)", KOChance{Hits: 2, ChancePercent: 68.75}, 688},
		{"限りなく 0 に近い確率も 0.0% にしない(下限 0.1%)",
			KOChance{Hits: 4, ChancePercent: 0.0001}, 1},
		{"限りなく 100 に近い確率も 100.0% にしない(上限 99.9%)",
			KOChance{Hits: 2, ChancePercent: 99.9999}, 999},
		{"99.94% は四捨五入して 99.9%", KOChance{Hits: 2, ChancePercent: 99.94}, 999},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ko.DisplayChancePercentTenths(); got != tt.want {
				t.Errorf("DisplayChancePercentTenths() = %d, want %d(0.1%%単位)", got, tt.want)
			}
		})
	}
}

// TestKOChanceRawFieldsAreUnchanged は、表示用メソッドを足しても生値の意味
// (ADR-0006: 確定のとき ChancePercent は 0)が変わっていないことを固定する。
// tools/golden/generate.mjs の期待値がこの意味に依存している(make test-golden)。
func TestKOChanceRawFieldsAreUnchanged(t *testing.T) {
	var rolls [16]int
	for i := range rolls {
		rolls[i] = 100 // 全ロール同じ = 最小ロールでも倒せる
	}
	ko := ComputeKO(rolls, 150)
	if !ko.Guaranteed || ko.Hits != 2 {
		t.Fatalf("確定2発のはず: %+v", ko)
	}
	if ko.ChancePercent != 0 {
		t.Errorf("確定のとき ChancePercent は 0 のまま(ADR-0006)。got %v", ko.ChancePercent)
	}
	if got := ko.DisplayChancePercentTenths(); got != 1000 {
		t.Errorf("表示用は 100.0%% であるべき: got %d", got)
	}

	noKO := ComputeKO([16]int{}, 150)
	if noKO.Hits != 0 || noKO.ChancePercent != 0 {
		t.Errorf("倒せないとき Hits=0・ChancePercent=0 のまま: %+v", noKO)
	}
	if got := noKO.DisplayChancePercentTenths(); got != 0 {
		t.Errorf("倒せないときの表示用は 0: got %d", got)
	}
}

// TestComputeKODisplayIsConsistentWithGuaranteed は、ComputeKO が返す全ケースで
// 表示値が「確定 = 100.0% / 倒せない = 0.0% / 乱数 = その間」に収まることを確かめる。
func TestComputeKODisplayIsConsistentWithGuaranteed(t *testing.T) {
	for minRoll := 0; minRoll <= 40; minRoll += 4 {
		for spread := 0; spread <= 12; spread += 3 {
			var rolls [16]int
			for i := range rolls {
				rolls[i] = minRoll + spread*i/15
			}
			for _, hp := range []int{1, 7, 50, 155, 300} {
				ko := ComputeKO(rolls, hp)
				got := ko.DisplayChancePercentTenths()
				switch {
				case ko.Hits == 0:
					if got != 0 {
						t.Fatalf("倒せない(%v, hp=%d)のに %d", rolls, hp, got)
					}
				case ko.Guaranteed:
					if got != 1000 {
						t.Fatalf("確定(%v, hp=%d)なのに %d", rolls, hp, got)
					}
				default:
					if got < 1 || got > 999 {
						t.Fatalf("乱数(%v, hp=%d, raw=%v)の表示値が [1,999] の外: %d",
							rolls, hp, ko.ChancePercent, got)
					}
				}
			}
		}
	}
}
