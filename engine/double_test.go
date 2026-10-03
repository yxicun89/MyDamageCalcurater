package engine

// issue #232 案B のダブル分・#288(ユーザー決定 2026-10-03)・ADR-0222: 形式 Format=double を計算に反映する
// (防御側の壁 2732/4096・全体技 3072/4096)。テラスタルはポケモンチャンピオンズに無いので計算に使わない
// (ADR-0222 §1。teraType の扱いは PR #497 / ADR-0160 の未対応の印に従う)。
// 規則は @smogon/calc 0.12.0 の Champions 世代が反映するものに合わせる
// (照合は testdata/golden/doubles.json・doubles-random.jsonl.gz。ここは手計算で固定する単体の規則)。
//
// 統制ケース(ctrlInput): 実数値 攻撃/特攻 200・防御/特防 100・威力 100 → base 90。
// rolls[15] = base、rolls[0] = floor(base*85/100)。
//
// 印の期待値は「技の印」だけを見る(moveTargetMarks)。PR #497 が足す形式・テラスの印(target format 等)は
// ここでは比べない(#497 のマージ後、ダブルを計算に反映したので format=double の印は外す。ADR-0222 §5)。

import (
	"errors"
	"reflect"
	"testing"
)

// moveTargetMarks は結果の印のうち技の印(target=move)だけを返す。
func moveTargetMarks(ms []UnsupportedMark) []UnsupportedMark {
	var out []UnsupportedMark
	for _, m := range ms {
		if m.Target == UnsupportedTargetMove {
			out = append(out, m)
		}
	}
	return out
}

// --- 壁(oracle calculateFinalModsChampions。ダブルは 2732/4096) ---------------------------------

func TestDoubleScreens(t *testing.T) {
	tests := []struct {
		name          string
		format        Format
		cat           MoveCategory
		screens       Screens
		critical      bool
		want0, want15 int
	}{
		// pokeRound(76, 2732) = 51(207632/4096 = 50.69)、pokeRound(90, 2732) = 60。
		{"ダブル: リフレクター(物理)", FormatDouble, CategoryPhysical, Screens{Reflect: true}, false, 51, 60},
		{"ダブル: ひかりのかべ(特殊)", FormatDouble, CategorySpecial, Screens{LightScreen: true}, false, 51, 60},
		{"ダブル: オーロラベール(物理)", FormatDouble, CategoryPhysical, Screens{AuroraVeil: true}, false, 51, 60},
		{"ダブル: リフレクター+オーロラベールでも1回だけ", FormatDouble, CategoryPhysical, Screens{Reflect: true, AuroraVeil: true}, false, 51, 60},
		{"ダブル: ひかりのかべは物理に効かない", FormatDouble, CategoryPhysical, Screens{LightScreen: true}, false, 76, 90},
		{"ダブル: 急所は壁を無視", FormatDouble, CategoryPhysical, Screens{Reflect: true}, true, 114, 135},
		{"シングル: リフレクターは 0.5 のまま", FormatSingle, CategoryPhysical, Screens{Reflect: true}, false, 38, 45},
		{"Format 未指定(ゼロ値)はシングル扱い", "", CategoryPhysical, Screens{Reflect: true}, false, 38, 45},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, tt.cat, TypeNormal)
			in.Format = tt.format
			in.Field.DefenderScreens = tt.screens
			in.Critical = tt.critical
			r, err := calcDamage(in)
			if err != nil {
				t.Fatal(err)
			}
			if r.Rolls[0] != tt.want0 || r.Rolls[15] != tt.want15 {
				t.Errorf("rolls[0]=%d rolls[15]=%d, want %d %d", r.Rolls[0], r.Rolls[15], tt.want0, tt.want15)
			}
		})
	}
}

// 攻撃側の壁はダブルでもダメージに効かない(防御側の壁だけを見る)。
func TestDoubleAttackerScreensIgnored(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	in.Format = FormatDouble
	in.Field.AttackerScreens = Screens{Reflect: true, LightScreen: true, AuroraVeil: true}
	r, err := calcDamage(in)
	if err != nil {
		t.Fatal(err)
	}
	if r.Rolls[15] != 90 {
		t.Errorf("rolls[15]=%d want 90", r.Rolls[15])
	}
}

// --- 全体技(oracle calculateBaseDamageChampions。base に 3072/4096、天候・急所より前) --------------

func TestDoubleSpread(t *testing.T) {
	tests := []struct {
		name          string
		format        Format
		target        MoveTarget
		atkTypes      []Type
		moveType      Type
		weather       Weather
		critical      bool
		screens       Screens
		want0, want15 int
	}{
		// pokeRound(90, 3072) = 67(276480/4096 = 67.5、ちょうど半分は切り捨て)。rolls[0] = floor(67*85/100) = 56。
		{"ダブル・全体技", FormatDouble, MoveTargetSpread, []Type{TypeWater}, TypeNormal, WeatherNone, false, Screens{}, 56, 67},
		{"ダブル・単体技は等倍", FormatDouble, MoveTargetSingle, []Type{TypeWater}, TypeNormal, WeatherNone, false, Screens{}, 76, 90},
		{"シングル・全体技は等倍", FormatSingle, MoveTargetSpread, []Type{TypeWater}, TypeNormal, WeatherNone, false, Screens{}, 76, 90},
		{"形式未指定・全体技は等倍", "", MoveTargetSpread, []Type{TypeWater}, TypeNormal, WeatherNone, false, Screens{}, 76, 90},
		// 順序: 全体 → 天候。67 → pokeRound(67, 6144) = 100(100.5 は切り捨て)。逆順なら 135 → 101。
		{"ダブル・全体技 × あめ(全体が先)", FormatDouble, MoveTargetSpread, []Type{TypeNormal}, TypeWater, WeatherRain, false, Screens{}, 85, 100},
		// 順序: 全体 → 急所。67 → floor(67*1.5) = 100。逆順なら 135 → 101。
		{"ダブル・全体技 × 急所(全体が先)", FormatDouble, MoveTargetSpread, []Type{TypeWater}, TypeNormal, WeatherNone, true, Screens{}, 85, 100},
		// 67 → 壁 pokeRound(67, 2732) = 45(44.69)。rolls[0]: 56 → pokeRound(56, 2732) = 37(37.35)。
		{"ダブル・全体技 × リフレクター", FormatDouble, MoveTargetSpread, []Type{TypeWater}, TypeNormal, WeatherNone, false, Screens{Reflect: true}, 37, 45},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := ctrlInput(tt.atkTypes, []Type{TypePsychic}, CategoryPhysical, tt.moveType)
			in.Format = tt.format
			in.Move.Target = tt.target
			in.Field.Weather = tt.weather
			in.Field.DefenderScreens = tt.screens
			in.Critical = tt.critical
			r, err := calcDamage(in)
			if err != nil {
				t.Fatal(err)
			}
			if r.Rolls[0] != tt.want0 || r.Rolls[15] != tt.want15 {
				t.Errorf("rolls[0]=%d rolls[15]=%d, want %d %d(rolls=%v)", r.Rolls[0], r.Rolls[15], tt.want0, tt.want15, r.Rolls)
			}
			if ms := moveTargetMarks(r.Unsupported); len(ms) != 0 {
				t.Errorf("技の対象が分かっているのに技の印が付いた: %+v", ms)
			}
		})
	}
}

// 全体技でも相性が無効ならダメージは 0(最低1ダメージの規則は相性≠0 のときだけ)。
func TestDoubleSpreadImmune(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypeGhost}, CategoryPhysical, TypeNormal)
	in.Format = FormatDouble
	in.Move.Target = MoveTargetSpread
	r, err := calcDamage(in)
	if err != nil {
		t.Fatal(err)
	}
	if r.Rolls != ([16]int{}) || r.Effectiveness != 0 {
		t.Errorf("rolls=%v eff=%v, want 全 0 / 0", r.Rolls, r.Effectiveness)
	}
}

// 全体技 ×0.75 で base が小さくても、各ロールは最低 1(相性≠0)。
func TestDoubleSpreadMinimumOne(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	in.Format = FormatDouble
	in.Move.Target = MoveTargetSpread
	in.Move.Power = 1
	in.Attacker.Ranks.Atk = -6
	in.Defender.Ranks.Def = 6
	r, err := calcDamage(in)
	if err != nil {
		t.Fatal(err)
	}
	for i, d := range r.Rolls {
		if d < 1 {
			t.Fatalf("rolls[%d]=%d(最低 1 のはず): %v", i, d, r.Rolls)
		}
	}
}

// テラスタイプはダブルの計算にも使わない(ポケモンチャンピオンズにテラスタルは無い。ADR-0222 §1)。
func TestDoubleIgnoresTeraType(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	in.Format = FormatDouble
	in.Move.Target = MoveTargetSpread
	base, err := calcDamage(in)
	if err != nil {
		t.Fatal(err)
	}
	in.Attacker.TeraType = TypeNormal
	in.Defender.TeraType = TypeGhost
	tera, err := calcDamage(in)
	if err != nil {
		t.Fatal(err)
	}
	if tera.Rolls != base.Rolls || tera.Effectiveness != base.Effectiveness || tera.STAB != base.STAB {
		t.Errorf("テラスで数値が変わった: %v vs %v", tera.Rolls, base.Rolls)
	}
}

// --- 技の対象が不明(マスタに未収録)のダブル: 未対応の印 ------------------------------------
//
// マスタが技の対象を持つまで(issue #288・データレーン)、calc-svc からの技は Target が空(不明)になる。
// ダブルで対象が不明な攻撃技は、全体の補正を掛けずに(単体技として)計算し、技の印 move_target_unknown を付ける
// (黙って「全体技でない」数値を正しいように見せない。ADR-0123 の印の方式)。壁の 2732 は対象に依らず掛ける。
func TestDoubleUnknownMoveTargetMark(t *testing.T) {
	tests := []struct {
		name      string
		format    Format
		target    MoveTarget
		cat       MoveCategory
		mechanism []MoveMechanism
		want      []UnsupportedMark
	}{
		{"ダブル・対象不明の攻撃技は印", FormatDouble, "", CategoryPhysical, nil,
			[]UnsupportedMark{moveMark("m", UnsupportedMoveTargetUnknown)}},
		{"ダブル・対象が単体なら印なし", FormatDouble, MoveTargetSingle, CategoryPhysical, nil, nil},
		{"ダブル・対象が全体なら印なし", FormatDouble, MoveTargetSpread, CategoryPhysical, nil, nil},
		{"シングル・対象不明は印なし(対象を使わない)", FormatSingle, "", CategoryPhysical, nil, nil},
		{"形式未指定・対象不明は印なし", "", "", CategoryPhysical, nil, nil},
		{"ダブル・変化技は印なし", FormatDouble, "", CategoryStatus, nil, nil},
		// 技の印の並び: 機構(昇順)→ zero_power → move_target_unknown。
		{"ダブル・機構の印の後ろに付く", FormatDouble, "", CategoryPhysical, []MoveMechanism{MechanismMultiHit},
			[]UnsupportedMark{moveMark("m", UnsupportedReason(MechanismMultiHit)), moveMark("m", UnsupportedMoveTargetUnknown)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, tt.cat, TypeNormal)
			in.Format = tt.format
			in.Move.Target = tt.target
			in.Move.Mechanisms = tt.mechanism
			r, err := calcDamage(in)
			if err != nil {
				t.Fatal(err)
			}
			if got := moveTargetMarks(r.Unsupported); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("技の印=%+v want %+v", got, tt.want)
			}
		})
	}
	// 印は数値を変えない: 対象不明のダブルは「単体技のダブル」と同じロール。
	unknown := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	unknown.Format = FormatDouble
	unknown.Field.DefenderScreens = Screens{Reflect: true}
	single := unknown
	single.Move.Target = MoveTargetSingle
	ru, err := calcDamage(unknown)
	if err != nil {
		t.Fatal(err)
	}
	rs, err := calcDamage(single)
	if err != nil {
		t.Fatal(err)
	}
	if ru.Rolls != rs.Rolls || ru.Rolls[15] != 60 {
		t.Errorf("対象不明 rolls=%v / 単体 rolls=%v(壁 2732 は対象に依らず掛かり、全体の補正は掛けない)", ru.Rolls, rs.Rolls)
	}
}

// --- 入力の検証 ------------------------------------------------------------------------------

func TestCalcDamageRejectsUnknownMoveTarget(t *testing.T) {
	for _, f := range []Format{"", FormatSingle, FormatDouble} {
		for _, bad := range []MoveTarget{"allAdjacent", "Spread", "aoe"} {
			in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
			in.Format, in.Move.Target = f, bad
			if _, err := calcDamage(in); !errors.Is(err, ErrUnknownMoveTarget) {
				t.Errorf("Format=%q Target=%q: err=%v want errors.Is ErrUnknownMoveTarget", f, bad, err)
			}
		}
		for _, target := range []MoveTarget{"", MoveTargetSingle, MoveTargetSpread} {
			in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
			in.Format, in.Move.Target = f, target
			if _, err := calcDamage(in); err != nil {
				t.Errorf("Format=%q Target=%q は受け付けること: %v", f, target, err)
			}
		}
	}
	if MoveTargetSingle != "single" || MoveTargetSpread != "spread" || UnsupportedMoveTargetUnknown != "move_target_unknown" {
		t.Errorf("値の綴りが契約(ADR-0222)と違う: %q %q %q", MoveTargetSingle, MoveTargetSpread, UnsupportedMoveTargetUnknown)
	}
}

// engine に直接届いた未知の形式(HTTP・WASM は enum で拒否する)は拒否せず、ダブルの補正を掛けない
// (シングルと同じ数値)。印は PR #497 / ADR-0160 の「安全側の印」(target format)に任せる。
func TestUnknownFormatIsNotDouble(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	in.Move.Target = MoveTargetSpread
	in.Field.DefenderScreens = Screens{Reflect: true}
	in.Format = FormatSingle
	want, err := calcDamage(in)
	if err != nil {
		t.Fatal(err)
	}
	in.Format = "triple"
	got, err := calcDamage(in)
	if err != nil {
		t.Fatalf("未知の形式で失敗した: %v", err)
	}
	if got.Rolls != want.Rolls || got.Rolls[15] != 45 {
		t.Errorf("未知の形式 rolls=%v / シングル %v", got.Rolls, want.Rolls)
	}
}

// --- 逆算: 形式と技の対象を CalcDamage へ素通しする ----------------------------------------------

func TestCalcReversePassesFormatAndMoveTarget(t *testing.T) {
	truth := revDefender(Stats{HP: 32, Def: 19}, NatureNeutral, nil)
	// A32・A上昇の既知側(ダメージが大きく、観測1件で SP を絞れる)。全体技 × リフレクターは
	// 0.75 × 2732/4096 ≒ 0.5 でシングルの壁と区別しにくいので、壁は張らない。
	known := revStrongAttacker()
	move := revMove(CategoryPhysical)
	move.Target = MoveTargetSpread
	field := Field{}
	res, err := calcDamage(DamageInput{Format: FormatDouble, Attacker: known, Defender: truth, Move: move, Field: field})
	if err != nil {
		t.Fatal(err)
	}
	in := ReverseInput{Format: FormatDouble, Side: SideDefender, Known: known, UnknownSpecies: revDefenderSpecies(),
		Move: move, Field: field, Observations: []Observation{{Damage: res.Rolls[7]}}}
	got, err := calcReverse(in)
	if err != nil {
		t.Fatalf("CalcReverse: %v", err)
	}
	assertMatchesOracle(t, in, got)
	_, c := findCand(got.Candidates, NatureClassNeutral, "")
	if c == nil || !c.Exact || !containsSP(c.Ranges, 19) {
		t.Fatalf("真値(無補正・B19)が完全一致の候補に無い: %+v", c)
	}
	// 形式・技の対象を落とすと同じ観測の答えが変わる(素通しされていなければここで一致してしまう)。
	for name, edit := range map[string]func(*ReverseInput){
		"Format=single": func(in *ReverseInput) { in.Format = FormatSingle },
		"技の対象=単体":       func(in *ReverseInput) { in.Move.Target = MoveTargetSingle },
	} {
		alt := in
		edit(&alt)
		ar, err := calcReverse(alt)
		if err != nil {
			t.Fatalf("%s: CalcReverse: %v", name, err)
		}
		_, ac := findCand(ar.Candidates, NatureClassNeutral, "")
		if ac != nil && reflect.DeepEqual(ac.Ranges, c.Ranges) && ac.Exact == c.Exact {
			t.Errorf("%s: 逆算の答えが変わらない(%+v)", name, ac.Ranges)
		}
	}
}

func containsSP(rs []SPRange, x int) bool {
	for _, r := range rs {
		if r.Min <= x && x <= r.Max {
			return true
		}
	}
	return false
}

// --- 一括計算: format=double は CalcDamage へ素通しされ、壁・全体技が全行に効く ------------------
// TestCalcBulkFormatDouble の注記(等価変異)を、single/double で結果が変わる入力で閉じる。
func TestCalcBulkFormatDoubleAppliesScreensAndSpread(t *testing.T) {
	for _, tt := range []struct {
		name string
		edit func(*BulkInput)
	}{
		{"壁(単体技)", func(in *BulkInput) {
			in.Move.Target = MoveTargetSingle
			in.Field.DefenderScreens = Screens{Reflect: true}
		}},
		{"全体技", func(in *BulkInput) { in.Move.Target = MoveTargetSpread }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			single := bulkInput(CategoryPhysical, TypeWater)
			tt.edit(&single)
			double := single
			double.Format = FormatDouble
			sres := checkedBulk(t, single)
			dres := checkedBulk(t, double)
			if !reflect.DeepEqual(rowKeys(sres.Rows), rowKeys(dres.Rows)) {
				t.Fatalf("double の行構成が single と異なる: %v vs %v", rowKeys(sres.Rows), rowKeys(dres.Rows))
			}
			for i := range dres.Rows {
				if reflect.DeepEqual(sres.Rows[i].Result.Rolls, dres.Rows[i].Result.Rolls) {
					t.Errorf("rows[%d](%s): format=double でも single と同じロール %v(Format が CalcDamage に届いていない)", i, dres.Rows[i].Preset, dres.Rows[i].Result.Rolls)
				}
			}
		})
	}
}
