package engine

// P2-3b / ADR-0106: 特性によるタイプの無効(ふゆう)・吸収(ちょすい等)。
//
// 受け入れ条件(ADR-0106 §決定 1・3・4):
//   - AbilityEffect.DefImmuneTypes / DefAbsorbTypes がデータとして効果を表す(特性名をコードに書かない)
//   - 当てはまるとダメージは 16 ロールとも 0、KO はゼロ値(倒せない)
//   - 急所・壁・持ち物・天候・ランク・やけどは結果を変えない(最優先の早期 return)
//   - 別タイプの技には効かない(過剰適用しない)
//   - タイプ由来の無効が先に報告される(ADR-0017 §5 / oracle champions.ts L262)
//   - 変化技は今までどおり(無効の有無に関係なくダメージ 0)

import "testing"

// immuneEffect / absorbEffect は「どの特性か」ではなく「どんな効果か」だけを書く。
func immuneEffect(types ...Type) *AbilityEffect {
	return &AbilityEffect{DefImmuneTypes: types}
}

func absorbEffect(t Type, a AbsorbEffect) *AbilityEffect {
	return &AbilityEffect{DefAbsorbTypes: map[Type]AbsorbEffect{t: a}}
}

// healQuarter / boostOne はゲームに実在する吸収の副次効果(ADR-0106 §決定 2)。
var (
	healQuarter = AbsorbEffect{HealNumerator: 1, HealDenominator: 4}
	boostAtkOne = AbsorbEffect{BoostStat: StatAtk, BoostStages: 1}
)

func assertAllRollsZero(t *testing.T, r DamageResult) {
	t.Helper()
	for i, d := range r.Rolls {
		if d != 0 {
			t.Fatalf("rolls[%d]=%d, want 0(無効・吸収は最低1ダメージの床も適用しない): %v", i, d, r.Rolls)
		}
	}
	if r.KO != (KOChance{}) {
		t.Errorf("KO=%+v, want ゼロ値(倒せない)", r.KO)
	}
}

func TestAbilityImmunityZeroesDamage(t *testing.T) {
	cases := []struct {
		name     string
		effect   *AbilityEffect
		moveType Type
		wantKind NullifyKind
	}{
		{"無効(ふゆう相当): 地面技", immuneEffect(TypeGround), TypeGround, NullifyImmune},
		{"吸収(ちょすい相当): 水技", absorbEffect(TypeWater, healQuarter), TypeWater, NullifyAbsorb},
		{"吸収(そうしょく相当): 草技", absorbEffect(TypeGrass, boostAtkOne), TypeGrass, NullifyAbsorb},
		{"吸収(もらいび相当・副次効果なし): 炎技", absorbEffect(TypeFire, AbsorbEffect{}), TypeFire, NullifyAbsorb},
		{"複数タイプの無効", immuneEffect(TypeGround, TypeElectric), TypeElectric, NullifyImmune},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, tc.moveType)
			in.Defender.Ability = Ability{ID: "a", Effect: tc.effect}
			r, err := calcDamage(in)
			if err != nil {
				t.Fatal(err)
			}
			assertAllRollsZero(t, r)
			if r.Nullified != tc.wantKind {
				t.Errorf("Nullified=%q, want %q", r.Nullified, tc.wantKind)
			}
		})
	}
}

// 無効・吸収が無い/当たらないときは今までどおりのダメージが出る(過剰適用の検出)。
func TestAbilityImmunityDoesNotApplyToOtherTypes(t *testing.T) {
	base := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	want, err := calcDamage(base)
	if err != nil {
		t.Fatal(err)
	}
	if want.Rolls[15] == 0 {
		t.Fatal("統制ケースのダメージが 0(テストの前提が壊れている)")
	}

	cases := []struct {
		name   string
		effect *AbilityEffect
	}{
		{"無効の対象が別タイプ", immuneEffect(TypeGround)},
		{"吸収の対象が別タイプ", absorbEffect(TypeWater, healQuarter)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
			in.Defender.Ability = Ability{ID: "a", Effect: tc.effect}
			got, err := calcDamage(in)
			if err != nil {
				t.Fatal(err)
			}
			if got.Rolls != want.Rolls {
				t.Errorf("rolls=%v, want %v(対象外のタイプに効いている)", got.Rolls, want.Rolls)
			}
			if got.Nullified != NullifyNone {
				t.Errorf("Nullified=%q, want 空", got.Nullified)
			}
		})
	}
}

// 攻撃側が同じ効果を持っていても、防御側の効果としてしか働かない(Def* の名のとおり)。
func TestAbilityImmunityIsDefenderOnly(t *testing.T) {
	want, err := calcDamage(ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal))
	if err != nil {
		t.Fatal(err)
	}
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	in.Attacker.Ability = Ability{ID: "a", Effect: immuneEffect(TypeNormal)}
	got, err := calcDamage(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Rolls != want.Rolls || got.Nullified != NullifyNone {
		t.Errorf("攻撃側の DefImmuneTypes が効いた: rolls=%v Nullified=%q", got.Rolls, got.Nullified)
	}
}

// 無効・吸収は最優先。他の補正をいくら乗せても 0 のまま(ADR-0106 §決定 3)。
func TestAbilityImmunityBeatsEveryOtherModifier(t *testing.T) {
	mutations := []struct {
		name  string
		apply func(*DamageInput)
	}{
		{"急所", func(in *DamageInput) { in.Critical = true }},
		{"壁", func(in *DamageInput) { in.Field.DefenderScreens.AuroraVeil = true }},
		{"天候(はれ)", func(in *DamageInput) { in.Field.Weather = WeatherSun }},
		{"フィールド", func(in *DamageInput) { in.Field.Terrain = TerrainGrassy }},
		{"やけど", func(in *DamageInput) { in.Attacker.Status = StatusBurn }},
		{"攻撃ランク+6", func(in *DamageInput) { in.Attacker.Ranks = Ranks{Atk: 6} }},
		{"攻撃側の持ち物", func(in *DamageInput) {
			in.Attacker.Item = &Item{ID: "i", Effect: &ItemEffect{StatMods: map[StatKey]int{StatAtk: 6144}, DamageMod: 5324}}
		}},
		{"防御側の半減きのみ", func(in *DamageInput) {
			in.Defender.Item = &Item{ID: "b", Effect: &ItemEffect{ResistBerryType: TypeGround}}
		}},
	}
	for _, m := range mutations {
		t.Run(m.name, func(t *testing.T) {
			// 地面技 vs 防御ほのお(相性 ×2)。相性ではなく特性で 0 になることを見る。
			in := ctrlInput([]Type{TypeWater}, []Type{TypeFire}, CategoryPhysical, TypeGround)
			in.Defender.Ability = Ability{ID: "a", Effect: immuneEffect(TypeGround)}
			m.apply(&in)
			r, err := calcDamage(in)
			if err != nil {
				t.Fatal(err)
			}
			assertAllRollsZero(t, r)
			if r.Nullified != NullifyImmune {
				t.Errorf("Nullified=%q, want %q", r.Nullified, NullifyImmune)
			}
			// 相性は今までどおり報告する(無効にしたのは特性であって相性ではない)。
			if r.Effectiveness != 2.0 {
				t.Errorf("Effectiveness=%v, want 2(タイプ相性そのものは変えない)", r.Effectiveness)
			}
		})
	}
}

// 同じ特性の中で、無効・吸収と倍率変更(DefResistType / ReduceSuperEffective)が共存できる。
func TestAbilityImmunityCombinedWithMultiplierEffects(t *testing.T) {
	eff := &AbilityEffect{
		DefAbsorbTypes:       map[Type]AbsorbEffect{TypeWater: healQuarter},
		DefResistType:        map[Type]int{TypeFire: ModifierHalf},
		ReduceSuperEffective: 3072,
	}

	// 吸収するタイプ(水)は 0。
	inWater := ctrlInput([]Type{TypeNormal}, []Type{TypePsychic}, CategoryPhysical, TypeWater)
	inWater.Defender.Ability = Ability{ID: "a", Effect: eff}
	rw, err := calcDamage(inWater)
	if err != nil {
		t.Fatal(err)
	}
	assertAllRollsZero(t, rw)
	if rw.Nullified != NullifyAbsorb {
		t.Errorf("Nullified=%q, want %q", rw.Nullified, NullifyAbsorb)
	}

	// 半減するタイプ(炎)は、吸収のフィールドを足したことで値が変わらない。
	inFire := ctrlInput([]Type{TypeNormal}, []Type{TypePsychic}, CategoryPhysical, TypeFire)
	inFire.Defender.Ability = Ability{ID: "a", Effect: &AbilityEffect{DefResistType: map[Type]int{TypeFire: ModifierHalf}}}
	wantFire, err := calcDamage(inFire)
	if err != nil {
		t.Fatal(err)
	}
	inFire2 := ctrlInput([]Type{TypeNormal}, []Type{TypePsychic}, CategoryPhysical, TypeFire)
	inFire2.Defender.Ability = Ability{ID: "a", Effect: eff}
	gotFire, err := calcDamage(inFire2)
	if err != nil {
		t.Fatal(err)
	}
	if gotFire.Rolls != wantFire.Rolls {
		t.Errorf("既存の倍率効果が変わった: %v, want %v", gotFire.Rolls, wantFire.Rolls)
	}
	if gotFire.Nullified != NullifyNone {
		t.Errorf("Nullified=%q, want 空", gotFire.Nullified)
	}
}

// タイプ由来の無効が先(ADR-0017 §5 / oracle champions.ts は typeEffectiveness===0 で先に return)。
func TestTypeImmunityReportedBeforeAbility(t *testing.T) {
	// ノーマル技 vs ゴースト = タイプ由来の無効。特性でも同じタイプを無効にしている。
	in := ctrlInput([]Type{TypeWater}, []Type{TypeGhost}, CategoryPhysical, TypeNormal)
	in.Defender.Ability = Ability{ID: "a", Effect: immuneEffect(TypeNormal)}
	r, err := calcDamage(in)
	if err != nil {
		t.Fatal(err)
	}
	assertAllRollsZero(t, r)
	if r.Effectiveness != 0 {
		t.Errorf("Effectiveness=%v, want 0", r.Effectiveness)
	}
	if r.Nullified != NullifyNone {
		t.Errorf("Nullified=%q, want 空(タイプ由来の無効として報告する。ADR-0017 §5)", r.Nullified)
	}
}

// 変化技は今までどおり。無効の有無で報告が変わらない。
func TestAbilityImmunityStatusMove(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryStatus, TypeGround)
	in.Move.Power = 0
	in.Defender.Ability = Ability{ID: "a", Effect: immuneEffect(TypeGround)}
	r, err := calcDamage(in)
	if err != nil {
		t.Fatal(err)
	}
	assertAllRollsZero(t, r)
}

// 同じタイプが無効と吸収の両方にあるのは、厳格デコード(services/internal/master)と
// WASM 境界が拒否する不正な定義。engine は入力を検証しない(ADR-0106 §決定 1)ので、
// ここでは「どちらが勝つか」を決め打ちにして静かな揺らぎを防ぐ: 無効が勝つ。
func TestAbilityImmunityWinsOverAbsorbForSameType(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeGround)
	in.Defender.Ability = Ability{ID: "a", Effect: &AbilityEffect{
		DefImmuneTypes: []Type{TypeGround},
		DefAbsorbTypes: map[Type]AbsorbEffect{TypeGround: healQuarter},
	}}
	r, err := calcDamage(in)
	if err != nil {
		t.Fatal(err)
	}
	assertAllRollsZero(t, r)
	if r.Nullified != NullifyImmune {
		t.Errorf("Nullified=%q, want %q(無効が勝つ)", r.Nullified, NullifyImmune)
	}
}

// 表に無いタイプを書いても engine は落ちない(相性表の検証は入力のタイプが対象。ADR-0013 §P1-13.3)。
// 効果定義のタイプの検証は厳格デコード側の責務で、そちらでテストする。
func TestAbilityImmunityUnknownTypeIsInert(t *testing.T) {
	want, err := calcDamage(ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeGround))
	if err != nil {
		t.Fatal(err)
	}
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeGround)
	in.Defender.Ability = Ability{ID: "a", Effect: &AbilityEffect{DefImmuneTypes: []Type{"nosuchtype"}}}
	got, err := calcDamage(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Rolls != want.Rolls || got.Nullified != NullifyNone {
		t.Errorf("表に無いタイプが効いた: rolls=%v Nullified=%q", got.Rolls, got.Nullified)
	}
}
