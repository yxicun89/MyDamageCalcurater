package importer_test

// 技の処理の定義(effects.json の moveRules)と種族の重さ(weightkg)の取り込み(ADR-0143 §4)。
//
// 受け入れ条件:
//   - DecodeEffectsFile は moveRules の節を受け付ける(他の未知の節は従来どおり拒否)。
//   - Convert は moves 表に採る攻撃技の定義を Output.MoveRules(master.EncodeMoveRule の正準形・技 ID の昇順)に出す。
//     moves 表に無い技のキーは警告 effect-unused。変化技・機構と対応しない・形の不正は ErrInvalidData。
//   - 種族の重さは Showdown・calc とも必須(キーが無い取得物は ErrInvalidInput)。小数1桁までの10進を浮動小数を経ずに hg の
//     整数にする(90.5 → 905)。小数2桁以上・0 以下は ErrInvalidData。両方にある種族で値が違えば Blocker species-mismatch(Detail weightkg)。
//   - 本番の data/importer/effects.json の moveRules は 46 技(ADR-0143 の段階2 36 技 + ADR-0144 の段階3 10 技)で、
//     両 ADR の測定の語彙ごとの件数どおり。
//
// fixture(testdata/fictional)の重さ: testmon 90.5kg・testmonmega 120kg・testleaf 6.9kg・testbug 0.1kg(calc も同じ値)。

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"example.com/pokecalc/services/internal/master"
	"example.com/pokecalc/services/pokedex/importer"
)

func TestDecodeEffectsFileAcceptsMoveRules(t *testing.T) {
	raw := []byte(`{"schemaVersion":1,"items":{},"abilities":{},"moveRules":{"teststrike":{"MoveSpecificResolved":true}}}`)
	f, err := importer.DecodeEffectsFile(raw)
	if err != nil {
		t.Fatalf("DecodeEffectsFile: %v", err)
	}
	if string(f.MoveRules["teststrike"]) != `{"MoveSpecificResolved":true}` {
		t.Fatalf("MoveRules = %v", f.MoveRules)
	}
	if _, err := importer.DecodeEffectsFile([]byte(`{"schemaVersion":1,"moverules":{}}`)); !errors.Is(err, importer.ErrInvalidInput) {
		t.Fatalf("節の名前の大文字小文字違い: err = %v, want ErrInvalidInput", err)
	}
}

// strikeWithPowerHook は teststrike(多段 [2,5] の物理技)に威力を変えるハンドラを足し、variable_power を持たせる。
func strikeWithPowerHook(t *testing.T, in *importer.Input) {
	t.Helper()
	sm := showdownMove(t, in, "teststrike")
	m := *sm.Mechanism
	m.Hooks = append(append([]string(nil), m.Hooks...), "basePowerCallback")
	sm.Mechanism = &m
}

func TestConvertMoveRules(t *testing.T) {
	in := loadFixture(t)
	strikeWithPowerHook(t, &in)
	in.Effects.MoveRules = map[string]json.RawMessage{
		"teststrike":  json.RawMessage(`{"MoveSpecificResolved":false,"PowerFormula":"hit_index"}`),
		"testnowhere": json.RawMessage(`{"PowerFormula":"speed_ratio"}`),
	}
	// false は書かない規則なので、まず不正として止まる。
	if _, _, err := importer.Convert(in); !errors.Is(err, importer.ErrInvalidData) {
		t.Fatalf("false を書いた定義: err = %v, want ErrInvalidData", err)
	}

	in.Effects.MoveRules["teststrike"] = json.RawMessage(`{ "PowerFormula" : "hit_index" }`)
	out, rep := convertOK(t, in)
	dec, err := master.DecodeMoveRule([]byte(`{"PowerFormula":"hit_index"}`), engineChart(t, out))
	if err != nil {
		t.Fatalf("DecodeMoveRule: %v", err)
	}
	want, err := master.EncodeMoveRule(*dec)
	if err != nil {
		t.Fatalf("EncodeMoveRule: %v", err)
	}
	if len(out.MoveRules) != 1 || out.MoveRules[0].ID != "teststrike" || !bytes.Equal(out.MoveRules[0].Effect, want) {
		t.Fatalf("MoveRules = %+v, want [{teststrike %s}](正準形)", out.MoveRules, want)
	}
	if !hasFinding(rep.Warnings, importer.KindEffectUnused, "testnowhere") {
		t.Errorf("moves 表に無い技の定義が effect-unused にならない: %+v", rep.Warnings)
	}
}

func TestConvertMoveRulesRejects(t *testing.T) {
	cases := []struct {
		name string
		id   string
		rule string
		hook bool
	}{
		{"変化技の定義", "testglare", `{"MoveSpecificResolved":true}`, false},
		{"機構と対応しない(teststrike は variable_power を持たない)", "teststrike", `{"PowerFormula":"speed_ratio"}`, false},
		{"未知のキー", "teststrike", `{"PowerFormulas":"speed_ratio"}`, true},
		{"語彙に無い値", "teststrike", `{"PowerFormula":"hp_ratio"}`, true},
		{"空の定義", "teststrike", `{}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := loadFixture(t)
			if tc.hook {
				strikeWithPowerHook(t, &in)
			}
			in.Effects.MoveRules = map[string]json.RawMessage{tc.id: json.RawMessage(tc.rule)}
			if _, _, err := importer.Convert(in); !errors.Is(err, importer.ErrInvalidData) {
				t.Fatalf("err = %v, want ErrInvalidData", err)
			}
		})
	}
}

// --- 種族の重さ ---------------------------------------------------------------

func speciesWeightByShowdownID(out importer.Output) map[string]int {
	m := map[string]int{}
	for _, s := range out.Species {
		m[s.ShowdownID] = s.WeightHg
	}
	return m
}

func TestConvertSpeciesWeight(t *testing.T) {
	in := loadFixture(t)
	unexcludeCalcSpecies(t, &in, "Testbug") // testbug は設定で除外している fixture の種族。重さを確かめるため取り込む
	out, _ := convertOK(t, in)
	got := speciesWeightByShowdownID(out)
	for id, want := range map[string]int{"testmon": 905, "testmonmega": 1200, "testleaf": 69, "testbug": 1} {
		if got[id] != want {
			t.Errorf("%s の重さ = %d hg, want %d(kg の小数1桁を浮動小数を経ずに整数へ)", id, got[id], want)
		}
	}
}

// editSnapshotWeight は取得物の JSON を書き換えてデコードし直す(フィールドの Go の型に依存しない)。
// idKey(Showdown は id、calc は name)が id の種族の weightkg を raw に置き換え、raw が "" ならキーを消す。
func editSnapshotWeight(t *testing.T, snapshot any, idKey, id, raw string) []byte {
	t.Helper()
	b, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("取得物を JSON にできない: %v", err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, v := range doc["species"].([]any) {
		sp := v.(map[string]any)
		if sp[idKey] != id {
			continue
		}
		found = true
		if raw == "" {
			delete(sp, "weightkg")
		} else {
			sp["weightkg"] = json.Number(raw)
		}
	}
	if !found {
		t.Fatalf("種族 %q が取得物に無い", id)
	}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestConvertSpeciesWeightRejects(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		wantErr   error
	}{
		{"小数2桁", "90.55", importer.ErrInvalidData},
		{"0", "0", importer.ErrInvalidData},
		{"負", "-1", importer.ErrInvalidData},
		{"指数表記の 0.01", "1e-2", importer.ErrInvalidData},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := loadFixture(t)
			sd, err := importer.DecodeShowdownSnapshot(editSnapshotWeight(t, in.Showdown, "id", "testmon", tc.raw))
			if err != nil {
				// デコードで拒否してもよい(取得物の形の誤り)。
				if !errors.Is(err, importer.ErrInvalidInput) && !errors.Is(err, importer.ErrInvalidData) {
					t.Fatalf("DecodeShowdownSnapshot: %v", err)
				}
				return
			}
			in.Showdown = sd
			if _, _, err := importer.Convert(in); !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
	t.Run("Showdown の weightkg が無い古い取得物はデコードで拒否", func(t *testing.T) {
		in := loadFixture(t)
		_, err := importer.DecodeShowdownSnapshot(editSnapshotWeight(t, in.Showdown, "id", "testmon", ""))
		if !errors.Is(err, importer.ErrInvalidInput) || !strings.Contains(err.Error(), "weightkg") {
			t.Fatalf("err = %v, want ErrInvalidInput(weightkg と取り直しを案内)", err)
		}
	})
	t.Run("calc の weightkg が無い古い取得物はデコードで拒否", func(t *testing.T) {
		in := loadFixture(t)
		_, err := importer.DecodeCalcSnapshot(editSnapshotWeight(t, in.Calc, "name", "Testmon", ""))
		if !errors.Is(err, importer.ErrInvalidInput) || !strings.Contains(err.Error(), "weightkg") {
			t.Fatalf("err = %v, want ErrInvalidInput(weightkg と取り直しを案内)", err)
		}
	})
}

func TestConvertBlocksOnSpeciesWeightMismatch(t *testing.T) {
	in := loadFixture(t)
	calc, err := importer.DecodeCalcSnapshot(editSnapshotWeight(t, in.Calc, "name", "Testmon", "90.4"))
	if err != nil {
		t.Fatalf("DecodeCalcSnapshot: %v", err)
	}
	in.Calc = calc
	_, rep, err := importer.Convert(in)
	if !errors.Is(err, importer.ErrBlocked) {
		t.Fatalf("err = %v, want ErrBlocked", err)
	}
	found := false
	for _, f := range rep.Blockers {
		if f.Kind == importer.KindSpeciesMismatch && f.ID == "testmon" && f.Detail == "weightkg" {
			found = true
		}
	}
	if !found {
		t.Errorf("Blockers に %s/testmon(weightkg)が無い: %+v", importer.KindSpeciesMismatch, rep.Blockers)
	}
}

// --- 本番の定義(data/importer/effects.json)の件数を ADR-0143 §測定に固定する --------------------------

func TestProductionMoveRulesMatchADR(t *testing.T) {
	prod, err := importer.DecodeEffectsFile(readRepoFile(t, "data", "importer", "effects.json"))
	if err != nil {
		t.Fatalf("data/importer/effects.json: %v", err)
	}
	// 段階2の 36 技(ADR-0143 §測定)に、段階3の 10 技(ADR-0144 §測定: 残り HP 8・なげつける 1・分類の切り替え 1)を足した数。
	if len(prod.MoveRules) != 46 {
		t.Fatalf("moveRules = %d 技, want 46(ADR-0143 の 36 + ADR-0144 の 10)", len(prod.MoveRules))
	}
	type boost struct {
		Condition string
	}
	formulas, conditions := map[string]int{}, map[string]int{}
	flags := map[string]int{}
	for id, raw := range prod.MoveRules {
		var r map[string]json.RawMessage
		if err := json.Unmarshal(raw, &r); err != nil {
			t.Fatalf("moveRules[%s]: %v", id, err)
		}
		for k, v := range r {
			switch k {
			case "PowerFormula":
				var s string
				_ = json.Unmarshal(v, &s)
				formulas[s]++
			case "PowerBoosts":
				var bs []boost
				_ = json.Unmarshal(v, &bs)
				for _, b := range bs {
					conditions[b.Condition]++
				}
			default:
				flags[k]++
			}
		}
	}
	wantFormulas := map[string]int{"attacker_positive_boosts": 2, "speed_ratio": 1, "inverse_speed_ratio": 1,
		"target_weight": 2, "weight_ratio": 2, "hit_index": 1,
		// 段階3(ADR-0144): ふんか・しおふき型 2・きしかいせい・じたばた型 2・ハードプレス型 1・なげつける型 1。
		"attacker_hp_scaled": 2, "attacker_hp_low": 2, "defender_hp_ratio": 1, "attacker_item_fling": 1}
	wantConditions := map[string]int{"attacker_status": 1, "defender_status": 4, "attacker_no_item": 1,
		"defender_item_removable": 1, "weather": 3, "terrain_attacker_grounded": 3, "terrain_defender_grounded": 1}
	wantFlags := map[string]int{"IgnoresBurn": 1, "TerrainPowerMods": 2, "TypeByWeather": 1, "TypeByTerrain": 1,
		"ExtraEffectivenessType": 1, "SuperEffectiveAgainst": 1, "PriorityBoost": 1, "BreaksScreens": 2,
		"FailsWithoutDefenderItem": 1, "SpreadInTerrain": 1,
		// 段階3(ADR-0144): いのちがけ・いかりのまえば・がむしゃら型 3・シェルアームズ型 1。MoveSpecificResolved は
		// なげつける型・シェルアームズ型の 2 を足す。
		"FixedDamageFormula": 3, "CategoryByStats": 1, "MoveSpecificResolved": 17}
	if !reflect.DeepEqual(formulas, wantFormulas) {
		t.Errorf("PowerFormula の件数 = %v, want %v", formulas, wantFormulas)
	}
	if !reflect.DeepEqual(conditions, wantConditions) {
		t.Errorf("PowerBoosts の条件の件数 = %v, want %v", conditions, wantConditions)
	}
	if !reflect.DeepEqual(flags, wantFlags) {
		t.Errorf("その他の項目の件数 = %v, want %v", flags, wantFlags)
	}

	// 持ち物による接地(ADR-0144 §1)。防御側の地面技の相性(浮いている側に当たる)は表せないので印は残す。
	{
		var got, w any
		if err := json.Unmarshal(prod.Items["ironball"], &got); err != nil {
			t.Fatalf("items[ironball]: %v", err)
		}
		_ = json.Unmarshal([]byte(`{"Grounds":true,"UnsupportedDefender":true}`), &w)
		if !reflect.DeepEqual(got, w) {
			t.Errorf("items[ironball] = %s, want {\"Grounds\":true,\"UnsupportedDefender\":true}", prod.Items["ironball"])
		}
	}

	// 重さの補正の特性(oracle の defenderAbilityIgnored にあるので Breakable)。
	for id, want := range map[string]string{
		"heavymetal": `{"WeightMod":8192,"Breakable":true}`,
		"lightmetal": `{"WeightMod":2048,"Breakable":true}`,
	} {
		var got, w any
		if err := json.Unmarshal(prod.Abilities[id], &got); err != nil {
			t.Fatalf("abilities[%s]: %v", id, err)
		}
		_ = json.Unmarshal([]byte(want), &w)
		if !reflect.DeepEqual(got, w) {
			t.Errorf("abilities[%s] = %s, want %s", id, prod.Abilities[id], want)
		}
	}
}

// 重さだけ違う姿は、性能が同じでも畳まない(重さで威力が決まる技の計算が変わるため。ADR-0143)。
func TestConvertDoesNotFoldFormsThatDifferOnlyInWeight(t *testing.T) {
	in := loadFixture(t)
	base := *calcSpecies(t, &in, "Testleaf")
	base.Name = "Testleaf-Bloom"
	in.Calc.Species = append(in.Calc.Species, base)
	var err error
	if in.Calc, err = importer.DecodeCalcSnapshot(editSnapshotWeight(t, in.Calc, "name", "Testleaf-Bloom", "7")); err != nil {
		t.Fatal(err)
	}
	if in.Showdown, err = importer.DecodeShowdownSnapshot(editSnapshotWeight(t, in.Showdown, "id", "testleafbloom", "7")); err != nil {
		t.Fatal(err)
	}
	out, rep := convertOK(t, in)
	if got := speciesWeightByShowdownID(out)["testleafbloom"]; got != 70 {
		t.Errorf("重さだけ違う testleafbloom が別の行にならない(重さ %d hg)", got)
	}
	if hasFinding(rep.Warnings, importer.KindFormFolded, "testleafbloom") {
		t.Errorf("重さだけ違う姿が畳まれた")
	}
}
