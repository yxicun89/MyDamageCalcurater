package engine

import (
	"reflect"
	"testing"
)

// issue #232 / ADR-0160: engine はテラスタイプ(Individual.TeraType)と対戦形式(Format=double)を
// 受け取るが計算に反映しない。拒否せず(iOS の構築メンバーは teraType を送るため)、結果に
// 「未対応」の印(ADR-0123 の UnsupportedMark)を付ける。数値は変えない(ゴールデン全件一致を保つ)。
//
// 印の形(ADR-0160 §2):
//
//	攻撃側のテラス: {Target: "attacker_tera_type", Reason: "unsupported_effect", ID: <テラスタイプの ID>}
//	防御側のテラス: {Target: "defender_tera_type", Reason: "unsupported_effect", ID: <テラスタイプの ID>}
//	対戦形式:       {Target: "format",             Reason: "unsupported_effect", ID: <Format の値>}
//
// 並びは既存の印(技 → 攻撃側の持ち物 → 攻撃側の特性 → 防御側の持ち物 → 防御側の特性)の後に
// 攻撃側のテラス → 防御側のテラス → 対戦形式。
//
// target の値は unsupported.go の定数(UnsupportedTargetAttackerTeraType 等)で参照する。

const (
	targetAttackerTera = UnsupportedTargetAttackerTeraType
	targetDefenderTera = UnsupportedTargetDefenderTeraType
	targetFormat       = UnsupportedTargetFormat
)

func teraMark(target UnsupportedTarget, tera Type) UnsupportedMark {
	return UnsupportedMark{Target: target, Reason: UnsupportedEffect, ID: string(tera)}
}

func formatMark(f Format) UnsupportedMark {
	return UnsupportedMark{Target: targetFormat, Reason: UnsupportedEffect, ID: string(f)}
}

// AC-1〜AC-4: テラス指定・ダブル(と engine に直接届いた未知の形式)に印を付け、数値は変えない。
func TestUnsupportedTeraAndFormatMarks(t *testing.T) {
	marked := &Item{ID: "atk-item", Effect: &ItemEffect{UnsupportedAttacker: true}}
	cases := []struct {
		name string
		edit func(*DamageInput)
		want []UnsupportedMark
	}{
		{"テラスなし・シングルは印なし", func(*DamageInput) {}, nil},
		{"Format が空(既定=シングル)は印なし", func(in *DamageInput) { in.Format = "" }, nil},
		{"攻撃側のテラス", func(in *DamageInput) { in.Attacker.TeraType = TypeFire },
			[]UnsupportedMark{teraMark(targetAttackerTera, TypeFire)}},
		{"攻撃側のテラスが技と同じタイプ", func(in *DamageInput) { in.Attacker.TeraType = TypeNormal },
			[]UnsupportedMark{teraMark(targetAttackerTera, TypeNormal)}},
		{"攻撃側のテラスが元のタイプと同じでも付ける(指定が無視されたことを示す)", func(in *DamageInput) {
			in.Attacker.TeraType = TypeWater
		}, []UnsupportedMark{teraMark(targetAttackerTera, TypeWater)}},
		{"防御側のテラス", func(in *DamageInput) { in.Defender.TeraType = TypeGhost },
			[]UnsupportedMark{teraMark(targetDefenderTera, TypeGhost)}},
		{"ダブル", func(in *DamageInput) { in.Format = FormatDouble },
			[]UnsupportedMark{formatMark(FormatDouble)}},
		{"engine に直接届いた未知の形式は安全側で印(ADR-0123 の未知の機構と同じ)", func(in *DamageInput) {
			in.Format = "triple"
		}, []UnsupportedMark{formatMark("triple")}},
		{"両側のテラス + ダブル", func(in *DamageInput) {
			in.Attacker.TeraType, in.Defender.TeraType, in.Format = TypeFire, TypeWater, FormatDouble
		}, []UnsupportedMark{
			teraMark(targetAttackerTera, TypeFire),
			teraMark(targetDefenderTera, TypeWater),
			formatMark(FormatDouble),
		}},
		{"既存の印の後に テラス(攻撃側 → 防御側) → 形式 の順", func(in *DamageInput) {
			in.Move.Mechanisms = []MoveMechanism{MechanismMultiHit}
			in.Attacker.Item = marked
			in.Attacker.TeraType, in.Defender.TeraType, in.Format = TypeFire, TypeWater, FormatDouble
		}, []UnsupportedMark{
			moveMark("m", UnsupportedReason(MechanismMultiHit)),
			{Target: UnsupportedTargetAttackerItem, Reason: UnsupportedEffect, ID: "atk-item"},
			teraMark(targetAttackerTera, TypeFire),
			teraMark(targetDefenderTera, TypeWater),
			formatMark(FormatDouble),
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plain := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
			want, err := calcDamage(plain)
			if err != nil {
				t.Fatal(err)
			}
			in := plain
			c.edit(&in)
			got, err := calcDamage(in)
			if err != nil {
				t.Fatalf("テラス・形式の指定で失敗した(拒否しない。ADR-0160 §1): %v", err)
			}
			if !reflect.DeepEqual(got.Unsupported, c.want) {
				t.Errorf("Unsupported = %v, want %v", got.Unsupported, c.want)
			}
			// 数値は印で変えない(テラス・ダブルの補正は未実装のまま。ゴールデン不変)。
			// 機構・持ち物の印だけの定義も数値を変えない(ADR-0123)ので、全ケースで素の入力と同じ。
			if got.Rolls != want.Rolls || got.KO != want.KO || got.Effectiveness != want.Effectiveness ||
				got.STAB != want.STAB || got.DefenderHP != want.DefenderHP || got.Nullified != want.Nullified {
				t.Errorf("印で数値が変わった\n got %+v\nwant %+v", got, want)
			}
		})
	}
}

// AC-1: 変化技でも付ける(持ち物・特性の印と同じ扱い。技の印だけが変化技で外れる。ADR-0123 §2)。
func TestUnsupportedTeraAndFormatMarksOnStatusMove(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryStatus, TypeNormal)
	in.Move.Power = 0
	in.Attacker.TeraType = TypeFire
	in.Format = FormatDouble
	got, err := calcDamage(in)
	if err != nil {
		t.Fatal(err)
	}
	want := []UnsupportedMark{teraMark(targetAttackerTera, TypeFire), formatMark(FormatDouble)}
	if !reflect.DeepEqual(got.Unsupported, want) || got.MaxDamage() != 0 {
		t.Errorf("変化技: Unsupported=%v max=%d, want %v 0", got.Unsupported, got.MaxDamage(), want)
	}
}

// AC-1: 表に無いテラスタイプは従来どおり ErrUnknownType(印にしない。検証が先)。
func TestUnsupportedTeraUnknownTypeStillRejected(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	in.Attacker.TeraType = "cosmic"
	if _, err := calcDamage(in); err == nil {
		t.Fatal("表に無いテラスタイプを受け付けた(ADR-0013 §P1-13.3 の検証が先)")
	}
}

// AC-5: 一括計算の全行に同じ印が付く(Format・攻撃側のテラスが CalcDamage へ素通しされていることも
// これで確かめられる。TestCalcBulkFormatDouble の注記の「等価変異」を殺す)。行構成・数値は single と同じ。
func TestUnsupportedTeraAndFormatPropagateToBulk(t *testing.T) {
	single := bulkInput(CategoryPhysical, TypeWater)
	sres, err := calcBulk(single)
	if err != nil {
		t.Fatal(err)
	}
	in := bulkInput(CategoryPhysical, TypeWater)
	in.Format = FormatDouble
	in.Attacker.TeraType = TypeFire
	res, err := calcBulk(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != len(sres.Rows) || len(res.Rows) == 0 {
		t.Fatalf("行数 = %d, want %d(single と同じ)", len(res.Rows), len(sres.Rows))
	}
	want := []UnsupportedMark{teraMark(targetAttackerTera, TypeFire), formatMark(FormatDouble)}
	for i, row := range res.Rows {
		if !reflect.DeepEqual(row.Result.Unsupported, want) {
			t.Errorf("rows[%d](%s).Unsupported = %v, want %v", i, row.Preset, row.Result.Unsupported, want)
		}
		if row.Result.Rolls != sres.Rows[i].Result.Rolls || row.Result.KO != sres.Rows[i].Result.KO {
			t.Errorf("rows[%d] の数値が single と違う: %v vs %v", i, row.Result.Rolls, sres.Rows[i].Result.Rolls)
		}
	}
}

// AC-5: 一括計算の特性のまとめ(ADR-0126)は、全特性で同じ印なのでテラス・形式で行が割れない。
func TestUnsupportedTeraAndFormatDoNotSplitAbilityGroups(t *testing.T) {
	abilities := []Ability{{ID: "a1"}, {ID: "a2"}}
	base := bulkInput(CategoryPhysical, TypeWater)
	base.DefenderAbilities = abilities
	plain, err := calcBulk(base)
	if err != nil {
		t.Fatal(err)
	}
	marked := base
	marked.Format = FormatDouble
	marked.Attacker.TeraType = TypeFire
	got, err := calcBulk(marked)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Rows) != len(plain.Rows) {
		t.Fatalf("行数 = %d, want %d(テラス・形式で特性のまとめが割れた)", len(got.Rows), len(plain.Rows))
	}
}

// AC-6: 逆算の全候補に既知側のテラス・形式の印が付く(相手側=探索側はテラスを持たない)。
func TestUnsupportedTeraAndFormatPropagateToReverse(t *testing.T) {
	base := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	cases := []struct {
		name  string
		side  ReverseSide
		known Individual
		other Species
		want  []UnsupportedMark
	}{
		{"相手が防御側: 既知の攻撃側のテラス", SideDefender, func() Individual {
			k := base.Attacker
			k.TeraType = TypeFire
			return k
		}(), base.Defender.Species, []UnsupportedMark{teraMark(targetAttackerTera, TypeFire), formatMark(FormatDouble)}},
		{"相手が攻撃側: 既知の防御側のテラス", SideAttacker, func() Individual {
			k := base.Defender
			k.TeraType = TypeGhost
			return k
		}(), base.Attacker.Species, []UnsupportedMark{teraMark(targetDefenderTera, TypeGhost), formatMark(FormatDouble)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rev, err := calcReverse(ReverseInput{
				Format: FormatDouble, Side: c.side, Known: c.known, UnknownSpecies: c.other,
				Move: base.Move, Observations: []Observation{{Percent: 40}},
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(rev.Candidates) == 0 {
				t.Fatal("候補が無い")
			}
			for _, cand := range rev.Candidates {
				if !reflect.DeepEqual(cand.Unsupported, c.want) {
					t.Errorf("候補 (%s, %q) の印 = %v, want %v", cand.NatureClass, cand.ItemID, cand.Unsupported, c.want)
				}
			}
		})
	}
}
