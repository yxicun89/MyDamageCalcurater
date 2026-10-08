package importer_test

// 技の機構の中身(move_mechanism_params。ADR-0142 §7)の変換。
// 取得元の値(multihit・damage・ohko・override*)を捨てずに、1技1行の中身にする。
// always_crit・ignore_defense_ranks・damageCallback の固定ダメージ・段階1外の機構は中身を持たない。
// 架空データ(testdata/fictional)のタイプは normal・fire・water・grass。

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"testing"

	"example.com/pokecalc/services/pokedex/importer"
)

func moveMechanismParamsByID(out importer.Output) map[string]importer.MoveMechanismParamsRow {
	m := map[string]importer.MoveMechanismParamsRow{}
	for _, r := range out.MoveMechanismParams {
		m[r.MoveID] = r
	}
	return m
}

// 架空の teststrike は multihit [2,5] を持つ(convert_move_mechanisms_test.go の fixture)。
func TestConvertMoveMechanismParamsFromFixture(t *testing.T) {
	out, _ := convertOK(t, loadFixture(t))
	want := []importer.MoveMechanismParamsRow{{MoveID: "teststrike", MultiHitMin: 2, MultiHitMax: 5}}
	if !reflect.DeepEqual(out.MoveMechanismParams, want) {
		t.Fatalf("move_mechanism_params = %+v,\nwant %+v", out.MoveMechanismParams, want)
	}
}

func TestConvertMoveMechanismParams(t *testing.T) {
	cases := []struct {
		name string
		set  func(m *importer.ShowdownMoveMechanism)
		want *importer.MoveMechanismParamsRow // nil は行を作らない
	}{
		{"判定材料なしは行なし", func(*importer.ShowdownMoveMechanism) {}, nil},
		{"multihit 回数", func(m *importer.ShowdownMoveMechanism) { m.Multihit = json.RawMessage(`2`) },
			&importer.MoveMechanismParamsRow{MultiHitMin: 2, MultiHitMax: 2}},
		{"multihit 範囲", func(m *importer.ShowdownMoveMechanism) { m.Multihit = json.RawMessage(`[2,5]`) },
			&importer.MoveMechanismParamsRow{MultiHitMin: 2, MultiHitMax: 5}},
		{"damage level", func(m *importer.ShowdownMoveMechanism) { m.Damage = json.RawMessage(`"level"`) },
			&importer.MoveMechanismParamsRow{FixedDamageLevel: true}},
		{"damage 数値", func(m *importer.ShowdownMoveMechanism) { m.Damage = json.RawMessage(`40`) },
			&importer.MoveMechanismParamsRow{FixedDamageValue: 40}},
		{"damageCallback は中身なし", func(m *importer.ShowdownMoveMechanism) { m.Hooks = []string{"damageCallback"} }, nil},
		{"ohko true", func(m *importer.ShowdownMoveMechanism) { m.OHKO = json.RawMessage(`true`) },
			&importer.MoveMechanismParamsRow{OHKO: true}},
		{"ohko タイプ名はタイプ ID にする", func(m *importer.ShowdownMoveMechanism) { m.OHKO = json.RawMessage(`"Water"`) },
			&importer.MoveMechanismParamsRow{OHKO: true, OHKOImmuneType: "water"}},
		{"willCrit は中身なし", func(m *importer.ShowdownMoveMechanism) { m.WillCrit = true }, nil},
		{"ignoreDefensive は中身なし", func(m *importer.ShowdownMoveMechanism) { m.IgnoreDefensive = true }, nil},
		{"overrideOffensiveStat", func(m *importer.ShowdownMoveMechanism) { m.OverrideOffensiveStat = "def" },
			&importer.MoveMechanismParamsRow{OffenseStat: "def"}},
		{"overrideOffensivePokemon target は防御側", func(m *importer.ShowdownMoveMechanism) { m.OverrideOffensivePokemon = "target" },
			&importer.MoveMechanismParamsRow{OffensePokemon: "defender"}},
		{"overrideOffensivePokemon source は攻撃側", func(m *importer.ShowdownMoveMechanism) { m.OverrideOffensivePokemon = "source" },
			&importer.MoveMechanismParamsRow{OffensePokemon: "attacker"}},
		{"overrideDefensiveStat", func(m *importer.ShowdownMoveMechanism) { m.OverrideDefensiveStat = "def" },
			&importer.MoveMechanismParamsRow{DefenseStat: "def"}},
		{"段階1外の機構と一緒でも中身は作る", func(m *importer.ShowdownMoveMechanism) {
			m.Multihit = json.RawMessage(`3`)
			m.Hooks = []string{"basePowerCallback"}
		}, &importer.MoveMechanismParamsRow{MultiHitMin: 3, MultiHitMax: 3}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := loadFixture(t)
			sm := showdownMove(t, &in, "teststrike")
			sm.Mechanism = &importer.ShowdownMoveMechanism{}
			tc.set(sm.Mechanism)
			out, _ := convertOK(t, in)
			got, ok := moveMechanismParamsByID(out)["teststrike"]
			if tc.want == nil {
				if ok {
					t.Fatalf("行を作らないはず: %+v", got)
				}
				return
			}
			want := *tc.want
			want.MoveID = "teststrike"
			if !ok || !reflect.DeepEqual(got, want) {
				t.Fatalf("teststrike の中身 = %+v (ok=%v), want %+v", got, ok, want)
			}
		})
	}
}

// 取得元の値が想定外なら止める(推測しない)。
func TestConvertMoveMechanismParamsRejectsMalformed(t *testing.T) {
	cases := []struct {
		name string
		set  func(m *importer.ShowdownMoveMechanism)
	}{
		{"ohko のタイプが相性表に無い", func(m *importer.ShowdownMoveMechanism) { m.OHKO = json.RawMessage(`"Ice"`) }},
		{"overrideOffensiveStat が hp", func(m *importer.ShowdownMoveMechanism) { m.OverrideOffensiveStat = "hp" }},
		{"overrideOffensiveStat が未知", func(m *importer.ShowdownMoveMechanism) { m.OverrideOffensiveStat = "luck" }},
		{"overrideOffensivePokemon が未知", func(m *importer.ShowdownMoveMechanism) { m.OverrideOffensivePokemon = "ally" }},
		{"overrideDefensiveStat が未知", func(m *importer.ShowdownMoveMechanism) { m.OverrideDefensiveStat = "luck" }},
		{"multihit の最大が上限超過", func(m *importer.ShowdownMoveMechanism) { m.Multihit = json.RawMessage(`[2,11]`) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := loadFixture(t)
			sm := showdownMove(t, &in, "teststrike")
			sm.Mechanism = &importer.ShowdownMoveMechanism{}
			tc.set(sm.Mechanism)
			if _, _, err := importer.Convert(in); !errors.Is(err, importer.ErrInvalidData) {
				t.Fatalf("err = %v, want ErrInvalidData", err)
			}
		})
	}
}

// 変化技・moves 表に採らない技には中身の行を作らない。行は技 ID の昇順(投入の決定性。ADR-0101 §8)。
func TestConvertMoveMechanismParamsSkipsAndSorts(t *testing.T) {
	in := loadFixture(t)
	for _, id := range []string{"testglare", "testold", "testbanned"} {
		showdownMove(t, &in, id).Mechanism = &importer.ShowdownMoveMechanism{Multihit: json.RawMessage(`2`)}
	}
	showdownMove(t, &in, "testflame").Mechanism = &importer.ShowdownMoveMechanism{OverrideDefensiveStat: "def"}
	out, _ := convertOK(t, in)
	got := moveMechanismParamsByID(out)
	for _, id := range []string{"testglare", "testold", "testbanned"} {
		if _, ok := got[id]; ok {
			t.Errorf("%s に中身の行がある", id)
		}
	}
	rows := out.MoveMechanismParams
	if len(rows) != 2 || !sort.SliceIsSorted(rows, func(i, j int) bool { return rows[i].MoveID < rows[j].MoveID }) {
		t.Fatalf("行 = %+v, want testflame・teststrike の2行が昇順", rows)
	}
}
