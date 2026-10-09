package master

// 技の処理の定義(MasterMove.rule)・種族の重さ(MasterSpecies.weightHg)・メガストーンの導出(ADR-0143 §6)の取り込みのテスト。
//
//   - FromExport は rule を共通マスタ経由で engine.Move.Rule に、weightHg を engine.Species.WeightHg に写す。
//   - キーが無い・null(古い pokedex-svc・定義なし)は定義なし / 重さ不明(0)として受け付ける(入れ替えの順序に依存しない)。
//   - 定義の形が不正・機構と対応しないときは ErrInvalidMaster(黙って定義なしにしない)。
//   - engine.Item.MegaStone は、取り込んだ種族の requiredItemId に現れる持ち物(ADR-0175 と同じ定義。契約の変更なし)。
//   - Store は Rule をディープコピーして返す(呼び出し側の書き換えが共有のマスタに漏れない)。
//   - 写した定義で engine が計算し、段階2の機構の印が外れる。

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/api"
)

func intPtr(v int) *int { return &v }

// exportWithRules は baseExport に、定義を持つ架空の技と重さを足したもの。
func exportWithRules(t *testing.T) api.MasterExport {
	t.Helper()
	ex := baseExport(t)
	for i := range ex.Species {
		switch ex.Species[i].Key {
		case "9001-000":
			ex.Species[i].WeightHg = intPtr(4600)
		case "9002-000":
			ex.Species[i].WeightHg = intPtr(999)
		}
	}
	ex.Moves = append(ex.Moves,
		api.MasterMove{Id: "testkick", NameJa: "テストけり", Type: api.PokeTypeNormal, Category: api.Physical, Power: 0,
			Mechanisms: []string{"move_specific", "variable_power"},
			Rule:       effect(map[string]any{"PowerFormula": "target_weight", "MoveSpecificResolved": true})},
		api.MasterMove{Id: "testslam", NameJa: "テストのしかかり", Type: api.PokeTypeNormal, Category: api.Physical, Power: 0,
			Mechanisms: []string{"move_specific", "variable_power"},
			Rule:       effect(map[string]any{"PowerFormula": "weight_ratio", "MoveSpecificResolved": true})},
		api.MasterMove{Id: "testknock", NameJa: "テストはたき", Type: api.PokeTypeNormal, Category: api.Physical, Power: 65,
			Mechanisms: []string{"variable_power"},
			Rule: effect(map[string]any{"PowerBoosts": []any{
				map[string]any{"Condition": "defender_item_removable", "Modifier": 6144}}})},
		api.MasterMove{Id: "testnorule", NameJa: "テストさだめなし", Type: api.PokeTypeNormal, Category: api.Physical, Power: 0,
			Mechanisms: []string{"variable_power"}},
	)
	return ex
}

func TestFromExportMapsRuleAndWeight(t *testing.T) {
	store := newStore(t, exportWithRules(t))
	cases := []struct {
		id   string
		want *engine.MoveRule
	}{
		{"testkick", &engine.MoveRule{PowerFormula: engine.PowerFormulaTargetWeight, MoveSpecificResolved: true}},
		{"testslam", &engine.MoveRule{PowerFormula: engine.PowerFormulaWeightRatio, MoveSpecificResolved: true}},
		{"testknock", &engine.MoveRule{PowerBoosts: []engine.MovePowerBoost{{Condition: engine.MoveConditionDefenderItemRemovable, Modifier: 6144}}}},
		{"testnorule", nil},
		{"testbeam", nil},
	}
	for _, tc := range cases {
		mv, ok := store.Move(tc.id)
		if !ok {
			t.Fatalf("Move(%s) が見つからない", tc.id)
		}
		if !reflect.DeepEqual(mv.Rule, tc.want) {
			t.Errorf("Move(%s).Rule = %+v, want %+v", tc.id, mv.Rule, tc.want)
		}
	}
	for key, want := range map[string]int{"9001-000": 4600, "9002-000": 999, "9002-001": 0} {
		sp, ok := store.Species(key)
		if !ok {
			t.Fatalf("Species(%s) が見つからない", key)
		}
		if sp.WeightHg != want {
			t.Errorf("Species(%s).WeightHg = %d, want %d(省略は 0 = 不明)", key, sp.WeightHg, want)
		}
	}
}

// メガ種族(9002-001)の requiredItemId に現れる持ち物だけがメガストーン。
func TestFromExportDerivesMegaStone(t *testing.T) {
	store := newStore(t, exportWithRules(t))
	for id, want := range map[string]bool{"testguardite": true, "testplainitem": false, "testorb": false} {
		it, ok := store.Item(id)
		if !ok {
			t.Fatalf("Item(%s) が見つからない", id)
		}
		if it.MegaStone != want {
			t.Errorf("Item(%s).MegaStone = %v, want %v", id, it.MegaStone, want)
		}
	}
}

func TestFromExportRejectsInvalidRule(t *testing.T) {
	cases := []struct {
		name string
		rule map[string]any
	}{
		{"語彙に無い式", map[string]any{"PowerFormula": "hp_ratio"}},
		{"未知のキー", map[string]any{"PowerFormulas": "target_weight"}},
		{"機構と対応しない(type_change の中身)", map[string]any{"TypeByWeather": map[string]any{"rain": "water"}}},
		{"空の定義", map[string]any{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ex := exportWithRules(t)
			for i := range ex.Moves {
				if ex.Moves[i].Id == "testkick" {
					ex.Moves[i].Rule = effect(tc.rule)
				}
			}
			if _, err := FromExport(ex); !errors.Is(err, ErrInvalidMaster) {
				t.Fatalf("FromExport = %v, want ErrInvalidMaster", err)
			}
		})
	}
}

// rule・weightHg キーの無い本文(古い pokedex-svc)も DecodeExport・FromExport を通る。
func TestDecodeExportRuleAndWeightCompat(t *testing.T) {
	example := string(readExample(t))
	if strings.Contains(example, `"rule"`) || strings.Contains(example, `"weightHg"`) {
		t.Fatal("例のファイルは rule・weightHg を持たない(省略可のキーの互換を確かめる例)")
	}
	ex, err := DecodeExport(strings.NewReader(example))
	if err != nil {
		t.Fatalf("DecodeExport = %v, want nil", err)
	}
	store := newStore(t, ex)
	if mv, _ := store.Move("testbeam"); mv.Rule != nil {
		t.Errorf("testbeam.Rule = %+v, want nil", mv.Rule)
	}
}

func TestStoreCopiesRule(t *testing.T) {
	store := newStore(t, exportWithRules(t))
	mv, _ := store.Move("testknock")
	mv.Rule.PowerBoosts[0].Modifier = 8192
	mv.Rule.MoveSpecificResolved = true
	again, _ := store.Move("testknock")
	if again.Rule.PowerBoosts[0].Modifier != 6144 || again.Rule.MoveSpecificResolved {
		t.Fatalf("Store の Rule が呼び出し側の書き換えで変わった: %+v", again.Rule)
	}
}

// 写した定義・重さ・メガストーンで engine が計算し、段階2の機構の印が外れる(定義の無い技・メガストーンの持ち物は印が残る)。
func TestMasterRuleDrivesCalc(t *testing.T) {
	store := newStore(t, exportWithRules(t))
	attacker, _ := store.Species("9001-000")
	defender, _ := store.Species("9002-000")
	calc := func(id string, defItem *engine.Item) engine.DamageResult {
		t.Helper()
		mv, ok := store.Move(id)
		if !ok {
			t.Fatalf("技 %s が無い", id)
		}
		res, err := engine.CalcDamage(engine.DamageInput{
			Format: engine.FormatSingle, Move: mv, TypeChart: store.TypeChart(),
			Attacker: engine.Individual{Species: attacker, Level: 50},
			Defender: engine.Individual{Species: defender, Level: 50, Item: defItem},
		})
		if err != nil {
			t.Fatalf("CalcDamage(%s): %v", id, err)
		}
		return res
	}
	for _, id := range []string{"testkick", "testslam"} {
		if r := calc(id, nil); r.Unsupported != nil || r.Rolls[15] == 0 {
			t.Errorf("%s: rolls = %v・Unsupported = %v, want ダメージあり・印なし", id, r.Rolls, r.Unsupported)
		}
	}
	plainItem, _ := store.Item("testplainitem")
	if r := calc("testknock", &plainItem); r.Unsupported != nil {
		t.Errorf("testknock × 通常の持ち物: Unsupported = %v, want nil", r.Unsupported)
	}
	stone, _ := store.Item("testguardite")
	if r := calc("testknock", &stone); r.Unsupported == nil {
		t.Error("testknock × メガストーン: 印が無い(防御側に合うかを engine は知らないので残す)")
	}
	if r := calc("testnorule", nil); r.Unsupported == nil {
		t.Error("testnorule: 定義の無い威力 0 の技に印が無い")
	}
}
