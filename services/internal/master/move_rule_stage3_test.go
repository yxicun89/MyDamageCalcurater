package master_test

// 技の機構の段階3(ADR-0144 §4)の共通マスタ。
//
// 受け入れ条件:
//   - DecodeMoveRule は段階3のキー FixedDamageFormula(語彙 engine.AllFixedDamageFormulas)・CategoryByStats(true だけ)と、
//     威力の式の新しい値を受け、EncodeMoveRule との往復で同じ値。語彙外・false は ErrInvalidEffect。
//   - DecodeItemEffect は Grounds(true だけ)を受ける。false は ErrInvalidEffect。
//   - ItemRoles: Grounds は両側の役割(攻撃側はフィールドの補正・防御側はフィールドとサイコフィールドの先制技の判定が変わる)。
//   - Item は ItemRow.FlingPower を engine.Item.FlingPower に写す(0 は「投げられない・不明」のまま。負は ErrInvalidRow)。

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
)

func TestDecodeMoveRuleStage3(t *testing.T) {
	chart := testChart(t)
	cases := []struct {
		raw  string
		want engine.MoveRule
	}{
		{`{"PowerFormula":"attacker_hp_scaled"}`, engine.MoveRule{PowerFormula: engine.PowerFormulaAttackerHPScaled}},
		{`{"PowerFormula":"attacker_hp_low"}`, engine.MoveRule{PowerFormula: engine.PowerFormulaAttackerHPLow}},
		{`{"PowerFormula":"defender_hp_ratio"}`, engine.MoveRule{PowerFormula: engine.PowerFormulaDefenderHPRatio}},
		{`{"PowerFormula":"attacker_item_fling","MoveSpecificResolved":true}`,
			engine.MoveRule{PowerFormula: engine.PowerFormulaAttackerItemFling, MoveSpecificResolved: true}},
		{`{"FixedDamageFormula":"attacker_current_hp"}`, engine.MoveRule{FixedDamageFormula: engine.FixedDamageAttackerCurrentHP}},
		{`{"FixedDamageFormula":"defender_current_hp_half"}`, engine.MoveRule{FixedDamageFormula: engine.FixedDamageDefenderHalfHP}},
		{`{"FixedDamageFormula":"defender_minus_attacker_hp"}`, engine.MoveRule{FixedDamageFormula: engine.FixedDamageDefenderMinusAttackerHP}},
		{`{"CategoryByStats":true,"MoveSpecificResolved":true}`, engine.MoveRule{CategoryByStats: true, MoveSpecificResolved: true}},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := master.DecodeMoveRule([]byte(tc.raw), chart)
			if err != nil {
				t.Fatalf("DecodeMoveRule: %v", err)
			}
			if !reflect.DeepEqual(*got, tc.want) {
				t.Fatalf("got %+v, want %+v", *got, tc.want)
			}
			raw, err := master.EncodeMoveRule(*got)
			if err != nil {
				t.Fatalf("EncodeMoveRule: %v", err)
			}
			again, err := master.DecodeMoveRule(raw, chart)
			if err != nil || !reflect.DeepEqual(*again, tc.want) {
				t.Fatalf("往復: %s → %+v (%v), want %+v", raw, again, err, tc.want)
			}
		})
	}
}

func TestDecodeMoveRuleStage3Rejects(t *testing.T) {
	chart := testChart(t)
	for _, raw := range []string{
		`{"FixedDamageFormula":"level"}`,
		`{"FixedDamageFormula":""}`,
		`{"FixedDamageFormula":"Attacker_Current_HP"}`,
		`{"CategoryByStats":false}`,
		`{"CategoryByStats":"true"}`,
		`{"PowerFormula":"hp_ratio"}`,
	} {
		if _, err := master.DecodeMoveRule([]byte(raw), chart); !errors.Is(err, master.ErrInvalidEffect) {
			t.Errorf("%s: err = %v, want ErrInvalidEffect", raw, err)
		}
	}
}

// 機構との対応は engine の検証(ValidateRule)を通す: fixed_damage の無い技の固定ダメージの式は ErrInvalidRow。
func TestMoveRowStage3RuleNeedsMechanism(t *testing.T) {
	chart := testChart(t)
	row := master.MoveRow{ID: "testhalver", NameJa: "テストはんぶん", Type: "normal", Category: "physical",
		Mechanisms: []string{"variable_power"}, Rule: []byte(`{"FixedDamageFormula":"defender_current_hp_half"}`)}
	if _, err := master.Move(row, chart); !errors.Is(err, master.ErrInvalidRow) {
		t.Fatalf("機構と対応しない定義: err = %v, want ErrInvalidRow", err)
	}
	row.Mechanisms = []string{"fixed_damage"}
	m, err := master.Move(row, chart)
	if err != nil {
		t.Fatalf("Move: %v", err)
	}
	if m.Rule == nil || m.Rule.FixedDamageFormula != engine.FixedDamageDefenderHalfHP {
		t.Fatalf("Rule = %+v, want FixedDamageFormula defender_current_hp_half", m.Rule)
	}
}

func TestDecodeItemEffectGrounds(t *testing.T) {
	chart := testChart(t)
	got, err := master.DecodeItemEffect([]byte(`{"Grounds":true,"UnsupportedDefender":true}`), chart)
	if err != nil {
		t.Fatalf("DecodeItemEffect: %v", err)
	}
	if !got.Grounds || !got.UnsupportedDefender {
		t.Fatalf("got %+v, want Grounds・UnsupportedDefender", *got)
	}
	raw, err := master.EncodeItemEffect(*got)
	if err != nil {
		t.Fatalf("EncodeItemEffect: %v", err)
	}
	if again, err := master.DecodeItemEffect(raw, chart); err != nil || !reflect.DeepEqual(again, got) {
		t.Fatalf("往復: %s → %+v (%v)", raw, again, err)
	}
	if _, err := master.DecodeItemEffect([]byte(`{"Grounds":false}`), chart); !errors.Is(err, master.ErrInvalidEffect) {
		t.Errorf("Grounds false: err = %v, want ErrInvalidEffect", err)
	}
}

func TestItemRolesGrounds(t *testing.T) {
	got := master.ItemRoles(&engine.ItemEffect{Grounds: true}, false)
	if want := []master.ItemRole{master.ItemRoleAttacker, master.ItemRoleDefender}; !slices.Equal(got, want) {
		t.Errorf("ItemRoles(Grounds) = %v, want %v(両側で接地が変わる)", got, want)
	}
}

func TestItemFlingPower(t *testing.T) {
	chart := testChart(t)
	item, err := master.Item(master.ItemRow{ID: "testball", NameJa: "テストのたま", FlingPower: 130}, chart)
	if err != nil {
		t.Fatalf("Item: %v", err)
	}
	if item.FlingPower != 130 {
		t.Errorf("FlingPower = %d, want 130", item.FlingPower)
	}
	none, err := master.Item(master.ItemRow{ID: "testplain", NameJa: "テストただのもの"}, chart)
	if err != nil || none.FlingPower != 0 {
		t.Errorf("FlingPower なし: %+v (%v), want 0", none, err)
	}
	if _, err := master.Item(master.ItemRow{ID: "testbad", NameJa: "テストわるい", FlingPower: -1}, chart); !errors.Is(err, master.ErrInvalidRow) {
		t.Errorf("FlingPower -1: err = %v, want ErrInvalidRow", err)
	}
}
