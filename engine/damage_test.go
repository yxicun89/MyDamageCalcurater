package engine

import "testing"

// mkIndiv は種族値・タイプ・SP・性格を指定して個体を作る(HP種族値は 100 固定)。
func mkIndiv(types []Type, base Stats) Individual {
	base.HP = 100
	return Individual{
		Species: Species{Types: types, BaseStats: base},
		Nature:  NatureNeutral,
	}
}

// 統制ケース: real Atk=200(base180), real Def=100(base80), 威力100 → base=90。
func ctrlInput(atkTypes, defTypes []Type, cat MoveCategory, moveType Type) DamageInput {
	atk := mkIndiv(atkTypes, Stats{Atk: 180, SpA: 180})
	def := mkIndiv(defTypes, Stats{Def: 80, SpD: 80})
	return DamageInput{
		Format:   FormatSingle,
		Attacker: atk,
		Defender: def,
		Move:     Move{ID: "m", Type: moveType, Category: cat, Power: 100},
	}
}

func TestCalcDamageBaseNeutralNoSTAB(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	r, err := CalcDamage(in)
	if err != nil {
		t.Fatal(err)
	}
	if r.Rolls[0] != 76 || r.Rolls[15] != 90 {
		t.Errorf("rolls[0]=%d rolls[15]=%d want 76 90", r.Rolls[0], r.Rolls[15])
	}
	if r.Effectiveness != 1.0 || r.STAB {
		t.Errorf("eff=%v stab=%v want 1.0 false", r.Effectiveness, r.STAB)
	}
	// 単調非減少
	for i := 1; i < 16; i++ {
		if r.Rolls[i] < r.Rolls[i-1] {
			t.Errorf("rolls not monotonic at %d: %v", i, r.Rolls)
		}
	}
}

func TestCalcDamageSTAB(t *testing.T) {
	in := ctrlInput([]Type{TypeNormal}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	r, _ := CalcDamage(in)
	if !r.STAB {
		t.Fatal("STAB should be true")
	}
	if r.Rolls[0] != 114 || r.Rolls[15] != 135 { // pokeRound(76*6144), pokeRound(90*6144)
		t.Errorf("rolls[0]=%d rolls[15]=%d want 114 135", r.Rolls[0], r.Rolls[15])
	}
}

func TestCalcDamageSuperEffective(t *testing.T) {
	in := ctrlInput([]Type{TypeNormal}, []Type{TypeRock}, CategoryPhysical, TypeWater)
	r, _ := CalcDamage(in)
	if r.Effectiveness != 2.0 {
		t.Fatalf("eff=%v want 2.0", r.Effectiveness)
	}
	if r.Rolls[0] != 152 || r.Rolls[15] != 180 { // 76*2, 90*2
		t.Errorf("rolls[0]=%d rolls[15]=%d want 152 180", r.Rolls[0], r.Rolls[15])
	}
}

func TestCalcDamageSTABSuperEffective(t *testing.T) {
	// STAB(6144、丸めない)× 抜群(×2)を最後に一度だけ floor する。
	// 攻撃みず/技みず/防御いわ。base=90、combined 係数 = 6144*4/(4096*2) = 3。
	in := ctrlInput([]Type{TypeWater}, []Type{TypeRock}, CategoryPhysical, TypeWater)
	r, _ := CalcDamage(in)
	if !r.STAB || r.Effectiveness != 2.0 {
		t.Fatalf("stab=%v eff=%v want true 2.0", r.STAB, r.Effectiveness)
	}
	// d_rand*3。i=0:76→228、i=1:77→231(旧実装は STAB を pokeRound して 230 になる)、i=15:90→270
	if r.Rolls[0] != 228 || r.Rolls[1] != 231 || r.Rolls[15] != 270 {
		t.Errorf("rolls[0]=%d rolls[1]=%d rolls[15]=%d want 228 231 270", r.Rolls[0], r.Rolls[1], r.Rolls[15])
	}
}

func TestCalcDamageImmune(t *testing.T) {
	in := ctrlInput([]Type{TypeNormal}, []Type{TypeGhost}, CategoryPhysical, TypeNormal)
	r, _ := CalcDamage(in)
	if r.Effectiveness != 0.0 {
		t.Fatalf("eff=%v want 0", r.Effectiveness)
	}
	for _, d := range r.Rolls {
		if d != 0 {
			t.Fatalf("immune should be 0, got %v", r.Rolls)
		}
	}
}

func TestCalcDamageCritical(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	in.Critical = true
	r, _ := CalcDamage(in)
	// base 90 → floor(90*1.5)=135
	if r.Rolls[15] != 135 || r.Rolls[0] != 114 { // floor(135*.85)=114
		t.Errorf("crit rolls[0]=%d rolls[15]=%d want 114 135", r.Rolls[0], r.Rolls[15])
	}
}

func TestCalcDamageBurnPhysical(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	in.Attacker.Status = StatusBurn
	r, _ := CalcDamage(in)
	if r.Rolls[15] != 45 || r.Rolls[0] != 38 { // pokeRound(90*2048), pokeRound(76*2048)
		t.Errorf("burn rolls[0]=%d rolls[15]=%d want 38 45", r.Rolls[0], r.Rolls[15])
	}
	// 特殊技はやけどの影響を受けない
	inS := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategorySpecial, TypeNormal)
	inS.Attacker.Status = StatusBurn
	rs, _ := CalcDamage(inS)
	if rs.Rolls[15] != 90 {
		t.Errorf("special burn should be unaffected, rolls[15]=%d want 90", rs.Rolls[15])
	}
	// こんじょう(guts)はやけど半減を無効化
	inG := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	inG.Attacker.Status = StatusBurn
	inG.Attacker.Ability = Ability{ID: "guts", Effect: &AbilityEffect{IgnoresBurn: true}}
	rg, _ := CalcDamage(inG)
	if rg.Rolls[15] != 90 {
		t.Errorf("guts should ignore burn, rolls[15]=%d want 90", rg.Rolls[15])
	}
}

func TestCalcDamageStatusMoveZero(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryStatus, TypeNormal)
	r, _ := CalcDamage(in)
	if r.Rolls[15] != 0 {
		t.Errorf("status move should be 0, got %d", r.Rolls[15])
	}
}

func TestCalcDamageMinOne(t *testing.T) {
	// 0.25倍かつ極小ダメージ → 相性≠0 なので最低1ダメージ
	atk := mkIndiv([]Type{TypeNormal}, Stats{Atk: 1})             // real Atk 21
	def := mkIndiv([]Type{TypeFire, TypeDragon}, Stats{Def: 200}) // real Def 220、grass 0.25
	in := DamageInput{
		Format:   FormatSingle,
		Attacker: atk,
		Defender: def,
		Move:     Move{Type: TypeGrass, Category: CategoryPhysical, Power: 10},
	}
	r, _ := CalcDamage(in)
	if r.Effectiveness != 0.25 {
		t.Fatalf("eff=%v want 0.25", r.Effectiveness)
	}
	for _, d := range r.Rolls {
		if d < 1 {
			t.Fatalf("min 1 damage violated: %v", r.Rolls)
		}
	}
}

func TestCritIgnoresUnfavorableStages(t *testing.T) {
	// 防御側の防御ランク+2 は急所で無視される → 通常より急所ダメージが大きい
	base := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	base.Defender.Ranks = Ranks{Def: 2}
	normal, _ := CalcDamage(base)
	crit := base
	crit.Critical = true
	critR, _ := CalcDamage(crit)
	if critR.Rolls[15] <= normal.Rolls[15] {
		t.Errorf("crit should ignore defender +Def: crit=%d normal=%d", critR.Rolls[15], normal.Rolls[15])
	}
}
