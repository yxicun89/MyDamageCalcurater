package httpapi

// GET /internal/pokedex/master(ADR-0105 §2・ADR-0204)の HTTP 境界。calc-svc が起動時に取るマスタ一式。
// ヘッダを要求しない。読み出しと組み立ては app.Service.MasterExport が行い、ここは契約の型への写しだけ。

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/pokedex/internal/app"
	"example.com/pokecalc/services/pokedex/internal/domain"
)

// GetMasterExport は GET /internal/pokedex/master。
func (s *Server) GetMasterExport(ctx *echo.Context) error {
	ex, err := s.app.MasterExport(ctx.Request().Context())
	if err != nil {
		return fail(err)
	}
	return ctx.JSON(http.StatusOK, masterExport(ex))
}

// masterExport は app のマスタ一式を契約の MasterExport にする。
func masterExport(ex app.MasterExport) api.MasterExport {
	masterSpecies := make([]api.MasterSpecies, 0, len(ex.Species))
	for _, ms := range ex.Species {
		sp := ms.Species
		abilityOut := make([]api.MasterSpeciesAbility, 0, len(ms.Abilities))
		for _, a := range ms.Abilities {
			abilityOut = append(abilityOut, api.MasterSpeciesAbility{Slot: a.Slot, AbilityId: a.AbilityID})
		}
		masterSpecies = append(masterSpecies, api.MasterSpecies{
			Key: sp.Key, DexNo: sp.DexNo, Form: sp.Form, ShowdownId: sp.ShowdownID, NameJa: sp.NameJa,
			Type1: api.PokeType(sp.Type1), Type2: pokeTypePtr(sp.Type2),
			BaseStats: statBlock(sp.BaseStats),
			IsMega:    sp.IsMega, BaseSpeciesKey: sp.BaseSpeciesKey, RequiredItemId: sp.RequiredItemID,
			Abilities: abilityOut, WeightHg: sp.WeightHg,
		})
	}

	masterTypes := make([]api.MasterType, 0, len(ex.Types))
	for _, t := range ex.Types {
		masterTypes = append(masterTypes, api.MasterType{Id: api.PokeType(t.ID), NameJa: t.NameJa, SortOrder: t.SortOrder})
	}
	masterTypeChart := make([]api.MasterTypeChartEntry, 0, len(ex.TypeChart))
	for _, c := range ex.TypeChart {
		masterTypeChart = append(masterTypeChart, api.MasterTypeChartEntry{
			AttackType: api.PokeType(c.AttackType), DefenseType: api.PokeType(c.DefenseType), Code: api.MasterTypeChartEntryCode(c.Code),
		})
	}
	masterMoves := make([]api.MasterMove, 0, len(ex.Moves))
	for _, mm := range ex.Moves {
		m := mm.Move
		masterMoves = append(masterMoves, api.MasterMove{
			Id: m.ID, NameJa: m.NameJa, Type: api.PokeType(m.Type), Category: api.MoveCategory(m.Category),
			Power: m.Power, Priority: m.Priority, Effect: masterEffect(mm.Effect), Mechanisms: mm.Mechanisms,
			MechanismParams: masterMechanismParams(mm.MechanismParams), Target: m.Target,
			Flags: mm.Flags, Rule: masterEffect(mm.Rule),
		})
	}
	masterItems := make([]api.MasterItem, 0, len(ex.Items))
	for _, it := range ex.Items {
		masterItems = append(masterItems, api.MasterItem{Id: it.Item.ID, NameJa: it.Item.NameJa, Effect: masterEffect(it.Effect), FlingPower: it.Item.FlingPower})
	}
	masterAbilities := make([]api.MasterAbility, 0, len(ex.Abilities))
	for _, a := range ex.Abilities {
		masterAbilities = append(masterAbilities, api.MasterAbility{Id: a.Ability.ID, NameJa: a.Ability.NameJa, Effect: masterEffect(a.Effect)})
	}
	masterNatures := make([]api.MasterNature, 0, len(ex.Natures))
	for _, n := range ex.Natures {
		masterNatures = append(masterNatures, api.MasterNature{Id: n.ID, NameJa: n.NameJa, Plus: statKeyPtr(n.Plus), Minus: statKeyPtr(n.Minus)})
	}

	return api.MasterExport{
		SchemaVersion: api.MasterExportSchemaVersionN1,
		DataVersion:   ex.DataVersion,
		Types:         masterTypes,
		TypeChart:     masterTypeChart,
		Species:       masterSpecies,
		Moves:         masterMoves,
		Items:         masterItems,
		Abilities:     masterAbilities,
		Natures:       masterNatures,
	}
}

// masterEffect は効果定義(数値の字面を保った map)を契約の MasterEffect にする。nil は nil(ADR-0105 §2・ADR-0204 §2)。
func masterEffect(e *domain.Effect) *api.MasterEffect {
	if e == nil {
		return nil
	}
	me := api.MasterEffect(*e)
	return &me
}

// masterMechanismParams は技の機構の中身を契約の形にする(ADR-0142 §8)。nil は nil。
func masterMechanismParams(p *app.MechanismParams) *api.MasterMoveMechanismParams {
	if p == nil {
		return nil
	}
	out := &api.MasterMoveMechanismParams{
		OffenseStat:    p.OffenseStat,
		OffensePokemon: p.OffensePokemon,
		DefenseStat:    p.DefenseStat,
	}
	if p.MultiHit != nil {
		out.MultiHit = &api.MasterMoveMultiHit{Min: p.MultiHit.Min, Max: p.MultiHit.Max}
	}
	if p.FixedDamage != nil {
		out.FixedDamage = &api.MasterMoveFixedDamage{Level: p.FixedDamage.Level, Value: p.FixedDamage.Value}
	}
	if p.Ohko != nil {
		out.Ohko = &api.MasterMoveOHKO{ImmuneType: p.Ohko.ImmuneType}
	}
	return out
}

func statBlock(b app.StatBlock) api.StatBlock {
	return api.StatBlock{Hp: b.Hp, Atk: b.Atk, Def: b.Def, Spa: b.Spa, Spd: b.Spd, Spe: b.Spe}
}

func pokeTypePtr(v *string) *api.PokeType {
	if v == nil {
		return nil
	}
	t := api.PokeType(*v)
	return &t
}

func statKeyPtr(v *string) *api.StatKey {
	if v == nil {
		return nil
	}
	k := api.StatKey(*v)
	return &k
}
