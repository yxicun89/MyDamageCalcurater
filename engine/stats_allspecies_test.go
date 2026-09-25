//go:build allspecies

package engine

import (
	"slices"
	"testing"
)

// allTypes は網羅テスト用の全18タイプ。
var allTypes = []Type{
	TypeNormal, TypeFire, TypeWater, TypeElectric, TypeGrass, TypeIce,
	TypeFighting, TypePoison, TypeGround, TypeFlying, TypePsychic, TypeBug,
	TypeRock, TypeGhost, TypeDragon, TypeDark, TypeSteel, TypeFairy,
}

// TestAllSpeciesDamageProperties は test-strategy L3 のダメージ性質を、
// タイプ全組合せ×代表的な種族値グリッドで確認する(外部実装照合は P1-6)。
func TestAllSpeciesDamageProperties(t *testing.T) {
	chart := mustTypeChart(t) // 相性はマスタ(fixture)から引く。ADR-0013
	baseVals := []int{40, 100, 180}
	newIn := func(atkType, moveType Type, defTypes []Type, atkBase, defBase, atkSP, defSP int) DamageInput {
		return DamageInput{
			Format: FormatSingle,
			Attacker: Individual{
				Species: Species{Types: []Type{atkType}, BaseStats: Stats{HP: 100, Atk: atkBase, SpA: atkBase}},
				Nature:  NatureNeutral, SP: Stats{Atk: atkSP, SpA: atkSP},
			},
			Defender: Individual{
				Species: Species{Types: defTypes, BaseStats: Stats{HP: 100, Def: defBase, SpD: defBase}},
				Nature:  NatureNeutral, SP: Stats{Def: defSP, SpD: defSP},
			},
			Move: Move{Type: moveType, Category: CategoryPhysical, Power: 80},
		}
	}
	for _, moveType := range allTypes {
		for _, dt1 := range allTypes {
			for _, atkBase := range baseVals {
				for _, defBase := range baseVals {
					defTypes := []Type{dt1}
					in := newIn(TypeNormal, moveType, defTypes, atkBase, defBase, 0, 0)
					r, err := calcDamage(in)
					if err != nil {
						t.Fatalf("panic-free expected: %v", err)
					}
					eff, err := chart.Effectiveness(moveType, defTypes)
					if err != nil {
						t.Fatalf("Effectiveness(%s, %v): %v", moveType, defTypes, err)
					}
					mult := eff.Multiplier()
					for i := 0; i < 16; i++ {
						if r.Rolls[i] < 0 {
							t.Fatalf("negative damage: %v", r.Rolls)
						}
						if i > 0 && r.Rolls[i] < r.Rolls[i-1] {
							t.Fatalf("not monotonic: %v", r.Rolls)
						}
						if mult == 0 && r.Rolls[i] != 0 {
							t.Fatalf("immune must be 0: %v", r.Rolls)
						}
					}
					if r.MinDamage() > r.MaxDamage() {
						t.Fatalf("min>max: %v", r.Rolls)
					}
					if mult == 0 {
						continue
					}
					// 攻撃側 SP 増でダメージ非減少
					more := newIn(TypeNormal, moveType, defTypes, atkBase, defBase, 32, 0)
					rMore, _ := calcDamage(more)
					if rMore.MaxDamage() < r.MaxDamage() {
						t.Fatalf("atk SP up decreased damage: %d < %d", rMore.MaxDamage(), r.MaxDamage())
					}
					// 防御側 SP 増でダメージ非増加
					tougher := newIn(TypeNormal, moveType, defTypes, atkBase, defBase, 0, 32)
					rTough, _ := calcDamage(tougher)
					if rTough.MaxDamage() > r.MaxDamage() {
						t.Fatalf("def SP up increased damage: %d > %d", rTough.MaxDamage(), r.MaxDamage())
					}
					// タイプ一致を付けると非減少(一致を外すと非増加)
					stabbed := newIn(moveType, moveType, defTypes, atkBase, defBase, 0, 0)
					rStab, _ := calcDamage(stabbed)
					if rStab.MaxDamage() < r.MaxDamage() {
						t.Fatalf("STAB decreased damage: %d < %d", rStab.MaxDamage(), r.MaxDamage())
					}
				}
			}
		}
	}
}

// standardStat は @smogon/calc(gen9)が使う標準式を独立に実装したもの。
// Lv50・個体値31固定。EV は SP から max(0,8×SP−4) で換算する。
// engine の実数値がこの標準式と全 base・全 SP で一致することを確認し、
// チャンピオンズ式の展開が正しいことを担保する(最終照合はゴールデン P1-6)。
func standardEVFromSP(sp int) int {
	ev := 8*sp - 4
	if ev < 0 {
		return 0
	}
	return ev
}

func standardHP(base, sp int) int {
	ev := standardEVFromSP(sp)
	return (2*base+FixedIV+ev/4)*DefaultLevel/100 + DefaultLevel + 10
}

func standardOther(base, sp int, mult10 int) int {
	ev := standardEVFromSP(sp)
	inner := (2*base+FixedIV+ev/4)*DefaultLevel/100 + 5
	// 性格補正は整数 ×(mult10/10)。mult10 ∈ {9,10,11}
	return inner * mult10 / 10
}

func TestAllSpeciesRealStatFormula(t *testing.T) {
	natures := []struct {
		mult10 int
		n      Nature
	}{
		{11, Nature{Plus: StatAtk, Minus: StatSpA}}, // +Atk
		{10, NatureNeutral},                         // 無補正
		{9, Nature{Plus: StatSpA, Minus: StatAtk}},  // -Atk
	}
	// base 1..255 を「全ポケモン」の代理として全 SP 0..32 で網羅。
	for base := 1; base <= 255; base++ {
		for sp := 0; sp <= MaxSPPerStat; sp++ {
			// HP は性格補正なし
			gotHP := RealStats(Individual{
				Species: Species{Types: []Type{TypeNormal}, BaseStats: Stats{HP: base}},
				SP:      Stats{HP: sp},
			}).HP
			if wantHP := standardHP(base, sp); gotHP != wantHP {
				t.Fatalf("HP base=%d sp=%d got=%d want=%d", base, sp, gotHP, wantHP)
			}
			// Atk を代表に性格補正3種を検証
			for _, nc := range natures {
				got := RealStats(Individual{
					Species: Species{Types: []Type{TypeNormal}, BaseStats: Stats{Atk: base}},
					Nature:  nc.n,
					SP:      Stats{Atk: sp},
				}).Atk
				if want := standardOther(base, sp, nc.mult10); got != want {
					t.Fatalf("Atk base=%d sp=%d mult10=%d got=%d want=%d", base, sp, nc.mult10, got, want)
				}
			}
		}
	}
}

// TestAllSpeciesTerrainGroundedProperty は issue #231 / ADR-0116 の性質を全タイプ組合せで確認する:
// フィールドの威力補正は「攻撃側が接地(ひこうでない・Airborne の特性でない)」のときだけ、
// ミストのドラゴン半減は「防御側が接地」のときだけ掛かり、掛からないときは地形なしと同じ結果になる。
// 掛かるときは「地形なしで威力を補正後の値にした結果」と一致する(補正は威力段階。ADR-0004)。
func TestAllSpeciesTerrainGroundedProperty(t *testing.T) {
	terrains := []Terrain{TerrainElectric, TerrainGrassy, TerrainPsychic, TerrainMisty}
	const power = 80
	boostedPower := pokeRound(power, 5325)
	halvedPower := pokeRound(power, ModifierHalf)
	newIn := func(atkTypes, defTypes []Type, atkAir, defAir bool, moveType Type, terr Terrain, pw int) DamageInput {
		in := DamageInput{
			Format: FormatSingle,
			Attacker: Individual{
				Species: Species{Types: atkTypes, BaseStats: Stats{HP: 100, Atk: 100, SpA: 100}},
				Nature:  NatureNeutral,
			},
			Defender: Individual{
				Species: Species{Types: defTypes, BaseStats: Stats{HP: 100, Def: 100, SpD: 100}},
				Nature:  NatureNeutral,
			},
			Move:  Move{Type: moveType, Category: CategorySpecial, Power: pw},
			Field: Field{Terrain: terr},
		}
		if atkAir {
			in.Attacker.Ability = Ability{Effect: &AbilityEffect{Airborne: true}}
		}
		if defAir {
			in.Defender.Ability = Ability{Effect: &AbilityEffect{Airborne: true}}
		}
		return in
	}
	// 各タイプの単タイプと、ひこう複合(浮いている側の代表)。
	typeSets := make([][]Type, 0, len(allTypes)*2)
	for _, ty := range allTypes {
		typeSets = append(typeSets, []Type{ty})
		if ty != TypeFlying {
			typeSets = append(typeSets, []Type{ty, TypeFlying})
		}
	}
	for _, terr := range terrains {
		for _, moveType := range allTypes {
			for _, atkTypes := range typeSets {
				for _, defTypes := range typeSets {
					for _, air := range [][2]bool{{false, false}, {true, false}, {false, true}} {
						in := newIn(atkTypes, defTypes, air[0], air[1], moveType, terr, power)
						got, err := calcDamage(in)
						if err != nil {
							t.Fatalf("%v: %v", in, err)
						}
						atkGrounded := !air[0] && !slices.Contains(atkTypes, TypeFlying)
						defGrounded := !air[1] && !slices.Contains(defTypes, TypeFlying)
						wantPower := power
						switch {
						case terr == TerrainElectric && moveType == TypeElectric && atkGrounded,
							terr == TerrainGrassy && moveType == TypeGrass && atkGrounded,
							terr == TerrainPsychic && moveType == TypePsychic && atkGrounded:
							wantPower = boostedPower
						case terr == TerrainMisty && moveType == TypeDragon && defGrounded:
							wantPower = halvedPower
						}
						want, _ := calcDamage(newIn(atkTypes, defTypes, air[0], air[1], moveType, TerrainNone, wantPower))
						if got.Rolls != want.Rolls {
							t.Fatalf("terrain=%s move=%s atk=%v(air=%v) def=%v(air=%v): rolls=%v want(威力%d・地形なし)=%v",
								terr, moveType, atkTypes, air[0], defTypes, air[1], got.Rolls, wantPower, want.Rolls)
						}
					}
				}
			}
		}
	}
}
