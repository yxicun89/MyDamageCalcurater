package httpapi

// 生成型 ⇄ app の写しの検査(ADR-0811・ADR-0252)。

import (
	"reflect"
	"testing"

	"example.com/pokecalc/services/calc/internal/domain"
	"example.com/pokecalc/services/internal/api"
)

// domain.Code は契約の ErrorCode と同じ文字列(httpErrorFrom が文字列のまま写すため)。
func TestDomainCodesAreContractErrorCodes(t *testing.T) {
	for _, c := range domain.AllCodes() {
		if !api.ErrorCode(c).Valid() {
			t.Errorf("domain.Code %q が契約の ErrorCode に無い", c)
		}
	}
}

// 計算イベントにはリクエストのままを載せる(ADR-0212 §7)。生成型 → app → 生成型で省略の有無も含めて変わらない。
func TestCalcRequestRoundTripsIntoEvent(t *testing.T) {
	i := func(v int) *int { return &v }
	b := func(v bool) *bool { return &v }
	s := func(v string) *string { return &v }
	status := api.StatusCondition("burn")
	tera := api.PokeType("fire")
	weather := api.Weather("rain")
	terrain := api.Terrain("not-a-terrain") // 列挙に無い値も、そのまま写る(検証は app・domain)
	full := api.CalcRequest{
		Format: api.Format("single"),
		Attacker: api.Individual{
			SpeciesKey: "0001-000", NatureId: "adamant", Level: i(50), AbilityId: s("a"), ItemId: s("b"),
			Sp:     api.StatBlock{Hp: 1, Atk: 2, Def: 3, Spa: 4, Spd: 5, Spe: 6},
			Ranks:  &api.RankBlock{Atk: i(1), Spe: i(-1)},
			Status: &status, TeraType: &tera,
		},
		Defender: api.Individual{SpeciesKey: "0002-000", NatureId: "bold", Ranks: &api.RankBlock{}},
		MoveId:   "m",
		Field: &api.FieldState{
			Weather: &weather, Terrain: &terrain,
			AttackerScreens: &api.Screens{Reflect: b(true)}, DefenderScreens: &api.Screens{},
		},
		Options:     &api.CalcOptions{Critical: b(false)},
		BattleState: &api.CalcBattleState{Hits: i(2)},
	}
	for _, req := range []api.CalcRequest{full, {Format: "double"}, {Options: &api.CalcOptions{}, Field: &api.FieldState{}}} {
		r := calcRequestFrom(req)
		got := api.CalcRequest{
			Format: api.Format(r.Format.Value), Attacker: individualToAPI(r.Attacker), Defender: individualToAPI(r.Defender),
			MoveId: r.MoveID, Field: fieldToAPI(r.Field), Options: optionsToAPI(r.Options), BattleState: battleStateToAPI(r.BattleState),
		}
		if !reflect.DeepEqual(got, req) {
			t.Errorf("往復で変わった:\n got  %+v\n want %+v", got, req)
		}
	}
}
