package master

// 技の機構の中身(MasterMove.mechanismParams。ADR-0142 §8)の取り込みのテスト。
//
//   - FromExport は mechanismParams を共通マスタ経由で engine.Move.Params に写す。
//   - null と、キーの無い本文(古い pokedex-svc)は「中身なし」として受け付ける(入れ替えの順序に依存しない)。
//   - 中身と機構の対応が合わない・値が語彙に無いときは ErrInvalidMaster(黙って中身なしにしない)。
//   - 写した中身で engine が計算し、段階1の機構の印が外れる。

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/api"
)

func intParams(min, max int) *api.MasterMoveMechanismParams {
	return &api.MasterMoveMechanismParams{MultiHit: &api.MasterMoveMultiHit{Min: min, Max: max}}
}

// exportWithParams は baseExport に、中身を持つ架空の技を足したもの。
func exportWithParams(t *testing.T) api.MasterExport {
	t.Helper()
	ex := baseExport(t)
	ex.Moves = append(ex.Moves,
		api.MasterMove{Id: "testmulti", NameJa: "テストれんだ", Type: api.PokeTypeNormal, Category: api.Physical, Power: 25,
			Mechanisms: []string{"multi_hit"}, MechanismParams: intParams(2, 5)},
		api.MasterMove{Id: "testfixed", NameJa: "テストこてい", Type: api.PokeTypeNormal, Category: api.Physical, Power: 0,
			Mechanisms:      []string{"fixed_damage"},
			MechanismParams: &api.MasterMoveMechanismParams{FixedDamage: &api.MasterMoveFixedDamage{Level: true}}},
		api.MasterMove{Id: "testpress", NameJa: "テストおしつけ", Type: api.PokeTypeNormal, Category: api.Physical, Power: 80,
			Mechanisms:      []string{"alt_offense_stat"},
			MechanismParams: &api.MasterMoveMechanismParams{OffenseStat: strPtr("def")}},
		api.MasterMove{Id: "testshock", NameJa: "テストしょうげき", Type: api.PokeTypeNormal, Category: api.Special, Power: 80,
			Mechanisms:      []string{"alt_defense_stat"},
			MechanismParams: &api.MasterMoveMechanismParams{DefenseStat: strPtr("def")}},
		api.MasterMove{Id: "testcut", NameJa: "テストいちげき", Type: api.PokeTypeNormal, Category: api.Physical, Power: 0,
			Mechanisms:      []string{"ohko"},
			MechanismParams: &api.MasterMoveMechanismParams{Ohko: &api.MasterMoveOHKO{ImmuneType: strPtr("grass")}}},
		api.MasterMove{Id: "testnone", NameJa: "テストなかみなし", Type: api.PokeTypeNormal, Category: api.Physical, Power: 25,
			Mechanisms: []string{"multi_hit"}, MechanismParams: nil},
	)
	return ex
}

func TestFromExportMapsMechanismParams(t *testing.T) {
	store := newStore(t, exportWithParams(t))
	cases := []struct {
		id   string
		want engine.MechanismParams
	}{
		{"testmulti", engine.MechanismParams{MultiHit: &engine.MultiHit{Min: 2, Max: 5}}},
		{"testfixed", engine.MechanismParams{FixedDamage: &engine.FixedDamage{Level: true}}},
		{"testpress", engine.MechanismParams{OffenseStat: engine.StatDef}},
		{"testshock", engine.MechanismParams{DefenseStat: engine.StatDef}},
		{"testcut", engine.MechanismParams{OHKO: &engine.OHKO{ImmuneType: engine.TypeGrass}}},
		{"testnone", engine.MechanismParams{}},
		{"testbeam", engine.MechanismParams{}},
	}
	for _, tc := range cases {
		mv, ok := store.Move(tc.id)
		if !ok {
			t.Fatalf("Move(%s) が見つからない", tc.id)
		}
		if !reflect.DeepEqual(mv.Params, tc.want) {
			t.Errorf("Move(%s).Params = %+v, want %+v", tc.id, mv.Params, tc.want)
		}
	}
}

func TestFromExportRejectsInvalidMechanismParams(t *testing.T) {
	cases := []struct {
		name string
		edit func(m *api.MasterMove)
	}{
		{"機構の無い中身", func(m *api.MasterMove) { m.Mechanisms = []string{} }},
		{"多段の範囲が逆", func(m *api.MasterMove) { m.MechanismParams = intParams(5, 2) }},
		{"攻撃に使う能力値が語彙に無い", func(m *api.MasterMove) {
			m.Mechanisms = []string{"alt_offense_stat"}
			m.MechanismParams = &api.MasterMoveMechanismParams{OffenseStat: strPtr("luck")}
		}},
		{"攻撃に使うポケモンが語彙に無い", func(m *api.MasterMove) {
			m.Mechanisms = []string{"alt_offense_stat"}
			m.MechanismParams = &api.MasterMoveMechanismParams{OffensePokemon: strPtr("target")}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ex := exportWithParams(t)
			for i := range ex.Moves {
				if ex.Moves[i].Id == "testmulti" {
					tc.edit(&ex.Moves[i])
				}
			}
			if _, err := FromExport(ex); !errors.Is(err, ErrInvalidMaster) {
				t.Fatalf("FromExport = %v, want ErrInvalidMaster", err)
			}
		})
	}
}

// mechanismParams キーの無い本文(古い pokedex-svc)も DecodeExport・FromExport を通り、中身なしになる。
func TestDecodeExportMechanismParamsCompat(t *testing.T) {
	example := string(readExample(t))
	if strings.Contains(example, `"mechanismParams"`) {
		body := strings.NewReplacer(`, "mechanismParams": null`, ``, `"mechanismParams": null, `, ``).Replace(example)
		if strings.Contains(body, `"mechanismParams"`) {
			t.Fatal("例のファイルの mechanismParams は null だけのはず(例データは機構の中身を持たない)")
		}
		example = body
	}
	ex, err := DecodeExport(strings.NewReader(example))
	if err != nil {
		t.Fatalf("DecodeExport = %v, want nil(mechanismParams が無くても取り込める)", err)
	}
	store := newStore(t, ex)
	if mv, _ := store.Move("testbeam"); !reflect.DeepEqual(mv.Params, engine.MechanismParams{}) {
		t.Errorf("testbeam.Params = %+v, want 空", mv.Params)
	}
}

// 写した中身で engine が計算し、段階1の機構の印が外れる(中身の無い技は印が残る)。
func TestMasterMechanismParamsDriveCalc(t *testing.T) {
	store := newStore(t, exportWithParams(t))
	attacker, _ := store.Species("9001-000")
	defender, _ := store.Species("9002-000")
	calc := func(id string) engine.DamageResult {
		t.Helper()
		mv, ok := store.Move(id)
		if !ok {
			t.Fatalf("技 %s が無い", id)
		}
		res, err := engine.CalcDamage(engine.DamageInput{
			Format: engine.FormatSingle, Move: mv, TypeChart: store.TypeChart(),
			Attacker: engine.Individual{Species: attacker, Level: 50}, Defender: engine.Individual{Species: defender, Level: 50},
		})
		if err != nil {
			t.Fatalf("CalcDamage(%s): %v", id, err)
		}
		return res
	}
	if res := calc("testmulti"); len(res.HitRolls) != 3 || res.Unsupported != nil {
		t.Errorf("testmulti: len(HitRolls)=%d Unsupported=%v, want 3 回・印なし", len(res.HitRolls), res.Unsupported)
	}
	if res := calc("testfixed"); res.Rolls[0] != 50 || res.Unsupported != nil {
		t.Errorf("testfixed: Rolls=%v Unsupported=%v, want 50・印なし", res.Rolls, res.Unsupported)
	}
	if res := calc("testnone"); res.Unsupported == nil {
		t.Error("testnone: 中身の無い多段に印が無い(取り込み前の DB は従来どおり印を付ける)")
	}
}
