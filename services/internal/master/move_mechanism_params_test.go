package master_test

// 技の機構の中身(move_mechanism_params。ADR-0142 §7)の検証と engine.Move.Params への写像、
// 特性の効果 MaxMultiHit・PreventsOHKO(ADR-0142 §3・§4)のデコード。

import (
	"errors"
	"reflect"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
)

func paramsRow(mechs []string, p master.MoveMechanismParamsRow) master.MoveRow {
	row := testMoveRow()
	row.Category = "physical"
	row.Mechanisms = mechs
	row.Params = &p
	return row
}

func TestMoveMapsMechanismParams(t *testing.T) {
	chart := testChart(t)
	cases := []struct {
		name string
		row  master.MoveRow
		want engine.MechanismParams
	}{
		{"中身なし(行が無い)は空", func() master.MoveRow {
			r := testMoveRow()
			r.Mechanisms = []string{"multi_hit"} // 取り込み前の DB: 機構はあるが中身が無い
			return r
		}(), engine.MechanismParams{}},
		{"多段の範囲", paramsRow([]string{"multi_hit"}, master.MoveMechanismParamsRow{MultiHitMin: 2, MultiHitMax: 5}),
			engine.MechanismParams{MultiHit: &engine.MultiHit{Min: 2, Max: 5}}},
		{"固定ダメージ(レベル)", paramsRow([]string{"fixed_damage"}, master.MoveMechanismParamsRow{FixedDamageLevel: true}),
			engine.MechanismParams{FixedDamage: &engine.FixedDamage{Level: true}}},
		{"固定ダメージ(数値)", paramsRow([]string{"fixed_damage"}, master.MoveMechanismParamsRow{FixedDamageValue: 40}),
			engine.MechanismParams{FixedDamage: &engine.FixedDamage{Value: 40}}},
		{"一撃必殺", paramsRow([]string{"ohko"}, master.MoveMechanismParamsRow{OHKO: true}),
			engine.MechanismParams{OHKO: &engine.OHKO{}}},
		{"一撃必殺(効かないタイプ)", paramsRow([]string{"ohko"}, master.MoveMechanismParamsRow{OHKO: true, OHKOImmuneType: "grass"}),
			engine.MechanismParams{OHKO: &engine.OHKO{ImmuneType: "grass"}}},
		{"攻撃に使う能力値", paramsRow([]string{"alt_offense_stat"}, master.MoveMechanismParamsRow{OffenseStat: "def"}),
			engine.MechanismParams{OffenseStat: engine.StatDef}},
		{"攻撃に使うポケモン", paramsRow([]string{"alt_offense_stat"}, master.MoveMechanismParamsRow{OffensePokemon: "defender"}),
			engine.MechanismParams{OffensePokemon: engine.OffensePokemonDefender}},
		{"防御に使う能力値", paramsRow([]string{"alt_defense_stat"}, master.MoveMechanismParamsRow{DefenseStat: "def"}),
			engine.MechanismParams{DefenseStat: engine.StatDef}},
		{"複数の機構の中身", paramsRow([]string{"multi_hit", "variable_power"}, master.MoveMechanismParamsRow{MultiHitMin: 3, MultiHitMax: 3}),
			engine.MechanismParams{MultiHit: &engine.MultiHit{Min: 3, Max: 3}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := master.Move(tc.row, chart)
			if err != nil {
				t.Fatalf("Move: %v", err)
			}
			if !reflect.DeepEqual(got.Params, tc.want) {
				t.Fatalf("Params = %+v, want %+v", got.Params, tc.want)
			}
		})
	}
}

func TestMoveRejectsInvalidMechanismParams(t *testing.T) {
	chart := testChart(t)
	cases := []struct {
		name string
		row  master.MoveRow
	}{
		{"全部空の行", paramsRow([]string{"multi_hit"}, master.MoveMechanismParamsRow{})},
		{"機構の無い多段", paramsRow(nil, master.MoveMechanismParamsRow{MultiHitMin: 2, MultiHitMax: 2})},
		{"多段の片方だけ", paramsRow([]string{"multi_hit"}, master.MoveMechanismParamsRow{MultiHitMax: 5})},
		{"多段の範囲が逆", paramsRow([]string{"multi_hit"}, master.MoveMechanismParamsRow{MultiHitMin: 5, MultiHitMax: 2})},
		{"多段の最大が 1", paramsRow([]string{"multi_hit"}, master.MoveMechanismParamsRow{MultiHitMin: 1, MultiHitMax: 1})},
		{"多段の上限超過", paramsRow([]string{"multi_hit"}, master.MoveMechanismParamsRow{MultiHitMin: 2, MultiHitMax: engine.MaxMultiHits + 1})},
		{"機構の無い固定ダメージ", paramsRow([]string{"multi_hit"}, master.MoveMechanismParamsRow{MultiHitMin: 2, MultiHitMax: 2, FixedDamageLevel: true})},
		{"固定ダメージのレベルと数値の両方", paramsRow([]string{"fixed_damage"}, master.MoveMechanismParamsRow{FixedDamageLevel: true, FixedDamageValue: 40})},
		{"固定ダメージの数値が負", paramsRow([]string{"fixed_damage"}, master.MoveMechanismParamsRow{FixedDamageValue: -1})},
		{"機構の無い一撃必殺", paramsRow([]string{"multi_hit"}, master.MoveMechanismParamsRow{MultiHitMin: 2, MultiHitMax: 2, OHKO: true})},
		{"一撃必殺でないのに効かないタイプ", paramsRow([]string{"ohko"}, master.MoveMechanismParamsRow{OHKOImmuneType: "grass"})},
		{"効かないタイプが相性表に無い", paramsRow([]string{"ohko"}, master.MoveMechanismParamsRow{OHKO: true, OHKOImmuneType: "shadow"})},
		{"機構の無い攻撃の能力値", paramsRow([]string{"alt_defense_stat"}, master.MoveMechanismParamsRow{DefenseStat: "def", OffenseStat: "def"})},
		{"攻撃の能力値が hp", paramsRow([]string{"alt_offense_stat"}, master.MoveMechanismParamsRow{OffenseStat: "hp"})},
		{"攻撃の能力値が未知", paramsRow([]string{"alt_offense_stat"}, master.MoveMechanismParamsRow{OffenseStat: "Def"})},
		{"攻撃に使うポケモンが未知(取得元の target のまま)", paramsRow([]string{"alt_offense_stat"}, master.MoveMechanismParamsRow{OffensePokemon: "target"})},
		{"機構の無い防御の能力値", paramsRow([]string{"alt_offense_stat"}, master.MoveMechanismParamsRow{OffenseStat: "def", DefenseStat: "def"})},
		{"防御の能力値が未知", paramsRow([]string{"alt_defense_stat"}, master.MoveMechanismParamsRow{DefenseStat: "luck"})},
		{"変化技の中身", func() master.MoveRow {
			r := paramsRow(nil, master.MoveMechanismParamsRow{MultiHitMin: 2, MultiHitMax: 2})
			r.Category, r.Power = "status", 0
			return r
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := master.Move(tc.row, chart); !errors.Is(err, master.ErrInvalidRow) {
				t.Fatalf("err = %v, want ErrInvalidRow", err)
			}
		})
	}
}

func TestDecodeAbilityEffectStage1MoveMechanisms(t *testing.T) {
	chart := testChart(t)
	cases := []struct {
		raw  string
		want engine.AbilityEffect
	}{
		{`{"MaxMultiHit":true}`, engine.AbilityEffect{MaxMultiHit: true}},
		{`{"PreventsOHKO":true,"Breakable":true}`, engine.AbilityEffect{PreventsOHKO: true, Breakable: true}},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := master.DecodeAbilityEffect([]byte(tc.raw), chart)
			if err != nil {
				t.Fatalf("DecodeAbilityEffect: %v", err)
			}
			if !reflect.DeepEqual(*got, tc.want) {
				t.Fatalf("got %+v, want %+v", *got, tc.want)
			}
			// 書き出し → 読み直しで同じ(effects の往復。ADR-0118)。
			raw, err := master.EncodeAbilityEffect(*got)
			if err != nil {
				t.Fatalf("EncodeAbilityEffect: %v", err)
			}
			again, err := master.DecodeAbilityEffect(raw, chart)
			if err != nil || !reflect.DeepEqual(*again, tc.want) {
				t.Fatalf("往復: %s → %+v (%v), want %+v", raw, again, err, tc.want)
			}
		})
	}
	for _, raw := range []string{`{"MaxMultiHit":false}`, `{"PreventsOHKO":1}`} {
		if _, err := master.DecodeAbilityEffect([]byte(raw), chart); !errors.Is(err, master.ErrInvalidEffect) {
			t.Errorf("%s: err = %v, want ErrInvalidEffect(true だけを書く)", raw, err)
		}
	}
}
