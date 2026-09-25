package importer_test

// 技の機構(move_mechanisms)の変換(ADR-0121)。
// 取得元は Showdown の技データ(multihit・damage・ohko・willCrit・override*・ignoreDefensive・
// 関数のプロパティ名)と、天候・フィールドのハンドラが技を名指ししている箇所。技の名前では分類しない。
//
// 架空データ(testdata/fictional)の Showdown スナップショットが持つ形:
//   teststrike  multihit [2,5]・hooks [onTry]                → multi_hit(onTry はダメージに効かない)
//   testrevived hooks [basePowerCallback, onModifyMove]      → move_specific・variable_power
//   testsplash  fieldConditions [testterrain.onBasePower]    → field_specific
//   testglare   変化技(hooks [onHit])                        → 行を作らない
//   testflame / testold / testbanned                         → 行を作らない(通常の技・moves 表に採らない技)

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"example.com/pokecalc/services/pokedex/importer"
)

func moveMechanismsByID(out importer.Output) map[string][]string {
	m := map[string][]string{}
	for _, r := range out.MoveMechanisms {
		m[r.MoveID] = append(m[r.MoveID], r.Mechanism)
	}
	return m
}

func TestConvertMoveMechanismsFromFixture(t *testing.T) {
	out, _ := convertOK(t, loadFixture(t))
	want := map[string][]string{
		"teststrike":  {"multi_hit"},
		"testrevived": {"move_specific", "variable_power"},
		"testsplash":  {"field_specific"},
	}
	if got := moveMechanismsByID(out); !reflect.DeepEqual(got, want) {
		t.Fatalf("move_mechanisms = %v,\nwant %v", got, want)
	}
}

// TestConvertMoveMechanismsAreSorted は行が (技 ID, 機構) の昇順であること(投入の決定性。ADR-0101 §8)。
func TestConvertMoveMechanismsAreSorted(t *testing.T) {
	out, _ := convertOK(t, loadFixture(t))
	rows := out.MoveMechanisms
	if !sort.SliceIsSorted(rows, func(i, j int) bool {
		if rows[i].MoveID != rows[j].MoveID {
			return rows[i].MoveID < rows[j].MoveID
		}
		return rows[i].Mechanism < rows[j].Mechanism
	}) {
		t.Fatalf("move_mechanisms が昇順でない: %+v", rows)
	}
}

// TestClassifyMoveMechanism は取得元の各判定材料 → 機構の対応(ADR-0121 の表)。
// 物理技 teststrike(威力 40)の判定材料を1つずつ差し替えて確かめる。
func TestClassifyMoveMechanism(t *testing.T) {
	cases := []struct {
		name string
		set  func(m *importer.ShowdownMoveMechanism)
		want []string
	}{
		{"判定材料なしは通常の技", func(m *importer.ShowdownMoveMechanism) {}, nil},
		{"multihit 回数", func(m *importer.ShowdownMoveMechanism) { m.Multihit = json.RawMessage(`2`) }, []string{"multi_hit"}},
		{"multihit 範囲", func(m *importer.ShowdownMoveMechanism) { m.Multihit = json.RawMessage(`[2,5]`) }, []string{"multi_hit"}},
		{"damage level", func(m *importer.ShowdownMoveMechanism) { m.Damage = json.RawMessage(`"level"`) }, []string{"fixed_damage"}},
		{"damage 数値", func(m *importer.ShowdownMoveMechanism) { m.Damage = json.RawMessage(`40`) }, []string{"fixed_damage"}},
		{"damageCallback", func(m *importer.ShowdownMoveMechanism) { m.Hooks = []string{"damageCallback"} }, []string{"fixed_damage"}},
		{"ohko true", func(m *importer.ShowdownMoveMechanism) { m.OHKO = json.RawMessage(`true`) }, []string{"ohko"}},
		{"ohko タイプ", func(m *importer.ShowdownMoveMechanism) { m.OHKO = json.RawMessage(`"Ice"`) }, []string{"ohko"}},
		{"willCrit", func(m *importer.ShowdownMoveMechanism) { m.WillCrit = true }, []string{"always_crit"}},
		{"overrideOffensiveStat", func(m *importer.ShowdownMoveMechanism) { m.OverrideOffensiveStat = "def" }, []string{"alt_offense_stat"}},
		{"overrideOffensivePokemon", func(m *importer.ShowdownMoveMechanism) { m.OverrideOffensivePokemon = "target" }, []string{"alt_offense_stat"}},
		{"overrideDefensiveStat", func(m *importer.ShowdownMoveMechanism) { m.OverrideDefensiveStat = "def" }, []string{"alt_defense_stat"}},
		{"ignoreDefensive", func(m *importer.ShowdownMoveMechanism) { m.IgnoreDefensive = true }, []string{"ignore_defense_ranks"}},
		{"basePowerCallback", func(m *importer.ShowdownMoveMechanism) { m.Hooks = []string{"basePowerCallback"} }, []string{"variable_power"}},
		{"onBasePower", func(m *importer.ShowdownMoveMechanism) { m.Hooks = []string{"onBasePower"} }, []string{"variable_power"}},
		{"onModifyType", func(m *importer.ShowdownMoveMechanism) { m.Hooks = []string{"onModifyType"} }, []string{"type_change"}},
		{"onEffectiveness", func(m *importer.ShowdownMoveMechanism) { m.Hooks = []string{"onEffectiveness"} }, []string{"effectiveness_change"}},
		{"onModifyPriority", func(m *importer.ShowdownMoveMechanism) { m.Hooks = []string{"onModifyPriority"} }, []string{"priority_change"}},
		{"onModifyMove", func(m *importer.ShowdownMoveMechanism) { m.Hooks = []string{"onModifyMove"} }, []string{"move_specific"}},
		{"onTryHit", func(m *importer.ShowdownMoveMechanism) { m.Hooks = []string{"onTryHit"} }, []string{"move_specific"}},
		{"onPrepareHit", func(m *importer.ShowdownMoveMechanism) { m.Hooks = []string{"onPrepareHit"} }, []string{"move_specific"}},
		{"ダメージに効かないハンドラだけなら通常の技", func(m *importer.ShowdownMoveMechanism) {
			m.Hooks = []string{"beforeMoveCallback", "beforeTurnCallback", "onAfterHit", "onAfterMove", "onAfterMoveSecondarySelf", "onAfterSubDamage",
				"onDisableMove", "onHit", "onModifyTarget", "onMoveFail", "onTry", "onTryImmunity", "onTryMove", "priorityChargeCallback"}
		}, nil},
		{"フィールドが名指しして威力を変える", func(m *importer.ShowdownMoveMechanism) { m.FieldConditions = []string{"testterrain.onBasePower"} }, []string{"field_specific"}},
		{"天候が名指ししてダメージを変える", func(m *importer.ShowdownMoveMechanism) {
			m.FieldConditions = []string{"testweather.onWeatherModifyDamage"}
		}, []string{"field_specific"}},
		{"フィールドの名指しがダメージに効かないハンドラなら通常の技", func(m *importer.ShowdownMoveMechanism) {
			m.FieldConditions = []string{"testterrain.onTryAddVolatile"}
		}, nil},
		{"複数の機構をすべて持つ", func(m *importer.ShowdownMoveMechanism) {
			m.Multihit = json.RawMessage(`3`)
			m.Hooks = []string{"basePowerCallback"}
		}, []string{"multi_hit", "variable_power"}},
		{"同じ機構は1行にまとめる", func(m *importer.ShowdownMoveMechanism) {
			m.Hooks = []string{"basePowerCallback", "onBasePower"}
		}, []string{"variable_power"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := loadFixture(t)
			sm := showdownMove(t, &in, "teststrike")
			sm.Mechanism = &importer.ShowdownMoveMechanism{}
			tc.set(sm.Mechanism)
			out, _ := convertOK(t, in)
			got := moveMechanismsByID(out)["teststrike"]
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("teststrike の機構 = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestClassifyZeroPowerAttackMove は威力 0 で登録された攻撃技(固定ダメージ・一撃必殺でない)を
// variable_power にすること(威力 0 のまま通常の式に渡すとダメージ 0 になる。issue #271)。
func TestClassifyZeroPowerAttackMove(t *testing.T) {
	cases := []struct {
		name string
		set  func(m *importer.ShowdownMoveMechanism)
		want []string
	}{
		{"判定材料なし", func(m *importer.ShowdownMoveMechanism) {}, []string{"variable_power"}},
		{"技固有の処理つき", func(m *importer.ShowdownMoveMechanism) { m.Hooks = []string{"onPrepareHit"} }, []string{"move_specific", "variable_power"}},
		{"固定ダメージは威力を使わない", func(m *importer.ShowdownMoveMechanism) { m.Damage = json.RawMessage(`"level"`) }, []string{"fixed_damage"}},
		{"技の処理で決まる固定ダメージも威力を使わない", func(m *importer.ShowdownMoveMechanism) { m.Hooks = []string{"damageCallback"} }, []string{"fixed_damage"}},
		{"一撃必殺は威力を使わない", func(m *importer.ShowdownMoveMechanism) { m.OHKO = json.RawMessage(`true`) }, []string{"ohko"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := loadFixture(t)
			calcMove(t, &in, "Test Strike").BasePower = 0
			sm := showdownMove(t, &in, "teststrike")
			sm.BasePower = 0
			sm.Mechanism = &importer.ShowdownMoveMechanism{}
			tc.set(sm.Mechanism)
			out, _ := convertOK(t, in)
			if got := moveMechanismsByID(out)["teststrike"]; !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("teststrike の機構 = %v, want %v", got, tc.want)
			}
		})
	}
}

// 未知のハンドラは黙って「通常の技」にしない: 安全側(move_specific / field_specific)に分類して警告する。
func TestClassifyUnknownHookIsConservativeAndWarned(t *testing.T) {
	in := loadFixture(t)
	sm := showdownMove(t, &in, "teststrike")
	sm.Mechanism = &importer.ShowdownMoveMechanism{Hooks: []string{"onBrandNewEvent"}}
	sp := showdownMove(t, &in, "testflame")
	sp.Mechanism = &importer.ShowdownMoveMechanism{FieldConditions: []string{"testterrain.onBrandNewEvent"}}
	out, rep := convertOK(t, in)
	got := moveMechanismsByID(out)
	if !reflect.DeepEqual(got["teststrike"], []string{"move_specific"}) {
		t.Errorf("未知の技のハンドラ: teststrike = %v, want [move_specific]", got["teststrike"])
	}
	if !reflect.DeepEqual(got["testflame"], []string{"field_specific"}) {
		t.Errorf("未知の状態のハンドラ: testflame = %v, want [field_specific]", got["testflame"])
	}
	for _, id := range []string{"teststrike", "testflame"} {
		if !hasFinding(rep.Warnings, importer.KindMoveMechanismUnknownHook, id) {
			t.Errorf("%s の未知のハンドラが Warnings(%s)に無い", id, importer.KindMoveMechanismUnknownHook)
		}
	}
}

// 変化技・moves 表に採らない技には判定材料があっても行を作らない。
func TestClassifySkipsStatusAndExcludedMoves(t *testing.T) {
	in := loadFixture(t)
	for _, id := range []string{"testglare", "testold", "testbanned"} {
		showdownMove(t, &in, id).Mechanism = &importer.ShowdownMoveMechanism{Multihit: json.RawMessage(`2`), Hooks: []string{"onBasePower"}}
	}
	out, _ := convertOK(t, in)
	got := moveMechanismsByID(out)
	for _, id := range []string{"testglare", "testold", "testbanned"} {
		if len(got[id]) != 0 {
			t.Errorf("%s に機構の行がある: %v", id, got[id])
		}
	}
}

// 取得元の形が想定外なら止める(値を推測して分類しない)。
func TestClassifyRejectsMalformedSignals(t *testing.T) {
	cases := []struct {
		name string
		set  func(m *importer.ShowdownMoveMechanism)
	}{
		{"multihit が文字列", func(m *importer.ShowdownMoveMechanism) { m.Multihit = json.RawMessage(`"x"`) }},
		{"multihit が1回", func(m *importer.ShowdownMoveMechanism) { m.Multihit = json.RawMessage(`1`) }},
		{"multihit の範囲が逆", func(m *importer.ShowdownMoveMechanism) { m.Multihit = json.RawMessage(`[5,2]`) }},
		{"multihit の要素が3つ", func(m *importer.ShowdownMoveMechanism) { m.Multihit = json.RawMessage(`[2,3,5]`) }},
		{"damage が 0", func(m *importer.ShowdownMoveMechanism) { m.Damage = json.RawMessage(`0`) }},
		{"damage が未知の文字列", func(m *importer.ShowdownMoveMechanism) { m.Damage = json.RawMessage(`"half"`) }},
		{"ohko が数値", func(m *importer.ShowdownMoveMechanism) { m.OHKO = json.RawMessage(`5`) }},
		{"ohko が false", func(m *importer.ShowdownMoveMechanism) { m.OHKO = json.RawMessage(`false`) }},
		{"fieldConditions に区切りが無い", func(m *importer.ShowdownMoveMechanism) { m.FieldConditions = []string{"testterrain"} }},
		{"hooks が空文字", func(m *importer.ShowdownMoveMechanism) { m.Hooks = []string{""} }},
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

// 機構の判定材料が無い技(古い取得物を直接 Convert に渡した場合)は止める。
func TestConvertRejectsMoveWithoutMechanism(t *testing.T) {
	in := loadFixture(t)
	showdownMove(t, &in, "teststrike").Mechanism = nil
	if _, _, err := importer.Convert(in); !errors.Is(err, importer.ErrInvalidData) {
		t.Fatalf("err = %v, want ErrInvalidData", err)
	}
}

// 機構の判定材料を持たない古いスナップショットはデコードで拒否する(黙って全技を通常の技にしない)。
func TestDecodeShowdownSnapshotRequiresMechanism(t *testing.T) {
	path := filepath.Join(fixtureRoot, "generated", "showdown", "abad1deaabad1deaabad1deaabad1deaabad1dea", "snapshot.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := importer.DecodeShowdownSnapshot(raw); err != nil {
		t.Fatalf("fixture のデコードに失敗: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	moves := doc["moves"].([]any)
	delete(moves[0].(map[string]any), "mechanism")
	old, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	_, err = importer.DecodeShowdownSnapshot(old)
	if !errors.Is(err, importer.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if !strings.Contains(err.Error(), "mechanism") {
		t.Errorf("エラーに原因(mechanism)が無い: %v", err)
	}
}
