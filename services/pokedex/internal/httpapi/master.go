package httpapi

// GET /internal/pokedex/master(ADR-0105 §2・ADR-0204)。calc-svc が起動時に取るマスタ一式。
// ヘッダを要求しない。DB の全行を使用可能集合で絞らずに返す。

import (
	"context"
	"database/sql"
	"net/http"
	"sort"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/pokedex/internal/store"
)

// GetMasterExport は GET /internal/pokedex/master。
func (s *Server) GetMasterExport(ctx *echo.Context) error {
	ex, err := s.buildMasterExport(ctx.Request().Context())
	if err != nil {
		return unavailable("GetMasterExport", err)
	}
	return ctx.JSON(http.StatusOK, ex)
}

func (s *Server) buildMasterExport(ctx context.Context) (api.MasterExport, error) {
	versions, err := s.q.ListDataVersions(ctx)
	if err != nil {
		return api.MasterExport{}, err
	}
	if len(versions) == 0 {
		return api.MasterExport{}, errEmpty("data_versions")
	}
	types, err := s.q.ListTypes(ctx)
	if err != nil {
		return api.MasterExport{}, err
	}
	if len(types) == 0 {
		return api.MasterExport{}, errEmpty("types")
	}
	typeChart, err := s.q.ListTypeChart(ctx)
	if err != nil {
		return api.MasterExport{}, err
	}
	species, err := s.q.ListSpecies(ctx)
	if err != nil {
		return api.MasterExport{}, err
	}
	speciesAbilities, err := s.q.ListAllSpeciesAbilities(ctx)
	if err != nil {
		return api.MasterExport{}, err
	}
	moves, err := s.q.ListMoves(ctx)
	if err != nil {
		return api.MasterExport{}, err
	}
	moveEffects, err := s.q.ListMoveEffects(ctx)
	if err != nil {
		return api.MasterExport{}, err
	}
	moveMechanisms, err := s.q.ListMoveMechanisms(ctx)
	if err != nil {
		return api.MasterExport{}, err
	}
	items, err := s.q.ListItems(ctx)
	if err != nil {
		return api.MasterExport{}, err
	}
	itemEffects, err := s.q.ListItemEffects(ctx)
	if err != nil {
		return api.MasterExport{}, err
	}
	abilities, err := s.q.ListAbilities(ctx)
	if err != nil {
		return api.MasterExport{}, err
	}
	abilityEffects, err := s.q.ListAbilityEffects(ctx)
	if err != nil {
		return api.MasterExport{}, err
	}
	natures, err := s.q.ListNatures(ctx)
	if err != nil {
		return api.MasterExport{}, err
	}
	if len(natures) == 0 {
		return api.MasterExport{}, errEmpty("natures")
	}

	abilitiesBySpecies := map[string][]store.SpeciesAbility{}
	for _, sa := range speciesAbilities {
		abilitiesBySpecies[sa.SpeciesKey] = append(abilitiesBySpecies[sa.SpeciesKey], sa)
	}
	for key := range abilitiesBySpecies {
		sort.Slice(abilitiesBySpecies[key], func(i, j int) bool { return abilitiesBySpecies[key][i].Slot < abilitiesBySpecies[key][j].Slot })
	}
	moveEffectByID := map[string]store.MoveEffect{}
	for _, e := range moveEffects {
		moveEffectByID[e.MoveID] = e
	}
	// mechanismsByMoveID: 契約(MasterMove.mechanisms)の「昇順」保証は SQL の ORDER BY に頼らず
	// ここで明示的にソートする(ADR-0121)。
	mechanismsByMoveID := map[string][]string{}
	for _, mm := range moveMechanisms {
		mechanismsByMoveID[mm.MoveID] = append(mechanismsByMoveID[mm.MoveID], mm.Mechanism)
	}
	for id := range mechanismsByMoveID {
		sort.Strings(mechanismsByMoveID[id])
	}
	itemEffectByID := map[string]store.ItemEffect{}
	for _, e := range itemEffects {
		itemEffectByID[e.ItemID] = e
	}
	abilityEffectByID := map[string]store.AbilityEffect{}
	for _, e := range abilityEffects {
		abilityEffectByID[e.AbilityID] = e
	}

	masterSpecies := make([]api.MasterSpecies, 0, len(species))
	for _, sp := range species {
		abilityRows := abilitiesBySpecies[sp.Key]
		abilityOut := make([]api.MasterSpeciesAbility, 0, len(abilityRows))
		for _, a := range abilityRows {
			abilityOut = append(abilityOut, api.MasterSpeciesAbility{Slot: int(a.Slot), AbilityId: a.AbilityID})
		}
		masterSpecies = append(masterSpecies, api.MasterSpecies{
			Key: sp.Key, DexNo: int(sp.DexNo), Form: int(sp.Form), ShowdownId: sp.ShowdownID, NameJa: sp.NameJa,
			Type1: api.PokeType(sp.Type1), Type2: pokeTypePtr(sp.Type2),
			BaseStats: api.StatBlock{Hp: int(sp.BaseHp), Atk: int(sp.BaseAtk), Def: int(sp.BaseDef), Spa: int(sp.BaseSpa), Spd: int(sp.BaseSpd), Spe: int(sp.BaseSpe)},
			IsMega:    sp.IsMega, BaseSpeciesKey: nullStringPtr(sp.BaseSpeciesKey), RequiredItemId: nullStringPtr(sp.RequiredItemID),
			Abilities: abilityOut,
		})
	}

	masterTypes := make([]api.MasterType, 0, len(types))
	for _, t := range types {
		masterTypes = append(masterTypes, api.MasterType{Id: api.PokeType(t.ID), NameJa: t.NameJa, SortOrder: int(t.SortOrder)})
	}
	masterTypeChart := make([]api.MasterTypeChartEntry, 0, len(typeChart))
	for _, c := range typeChart {
		masterTypeChart = append(masterTypeChart, api.MasterTypeChartEntry{
			AttackType: api.PokeType(c.AttackType), DefenseType: api.PokeType(c.DefenseType), Code: api.MasterTypeChartEntryCode(c.Code),
		})
	}
	masterMoves := make([]api.MasterMove, 0, len(moves))
	for _, m := range moves {
		effect, err := masterEffectFor(moveEffectByID[m.ID].Effect)
		if err != nil {
			return api.MasterExport{}, err
		}
		mechanisms := mechanismsByMoveID[m.ID]
		if mechanisms == nil {
			mechanisms = []string{} // 通常の技は空配列(null にしない。ADR-0121)
		}
		masterMoves = append(masterMoves, api.MasterMove{
			Id: m.ID, NameJa: m.NameJa, Type: api.PokeType(m.Type), Category: api.MoveCategory(m.Category),
			Power: int(m.Power), Priority: int(m.Priority), Effect: effect, Mechanisms: mechanisms,
		})
	}
	masterItems := make([]api.MasterItem, 0, len(items))
	for _, it := range items {
		effect, err := masterEffectFor(itemEffectByID[it.ID].Effect)
		if err != nil {
			return api.MasterExport{}, err
		}
		masterItems = append(masterItems, api.MasterItem{Id: it.ID, NameJa: it.NameJa, Effect: effect})
	}
	masterAbilities := make([]api.MasterAbility, 0, len(abilities))
	for _, a := range abilities {
		effect, err := masterEffectFor(abilityEffectByID[a.ID].Effect)
		if err != nil {
			return api.MasterExport{}, err
		}
		masterAbilities = append(masterAbilities, api.MasterAbility{Id: a.ID, NameJa: a.NameJa, Effect: effect})
	}
	masterNatures := make([]api.MasterNature, 0, len(natures))
	for _, n := range natures {
		masterNatures = append(masterNatures, api.MasterNature{Id: n.ID, NameJa: n.NameJa, Plus: statKeyPtr(n.Plus), Minus: statKeyPtr(n.Minus)})
	}

	return api.MasterExport{
		SchemaVersion: api.MasterExportSchemaVersionN1,
		DataVersion:   dataVersionString(versions),
		Types:         masterTypes,
		TypeChart:     masterTypeChart,
		Species:       masterSpecies,
		Moves:         masterMoves,
		Items:         masterItems,
		Abilities:     masterAbilities,
		Natures:       masterNatures,
	}, nil
}

// dataVersionString は data_versions の source=version を source 昇順に「,」で連結する(ADR-0105 §2)。
func dataVersionString(versions []store.DataVersion) string {
	sorted := append([]store.DataVersion(nil), versions...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Source < sorted[j].Source })
	s := ""
	for i, v := range sorted {
		if i > 0 {
			s += ","
		}
		s += v.Source + "=" + v.Version
	}
	return s
}

// masterEffectFor は item_effects / ability_effects の JSON をそのまま api.MasterEffect にする
// (float64 を経由しない。json.Decoder.UseNumber で数値の字面を保つ。ADR-0105 §2・ADR-0204 §2)。
// 行が無ければ nil(JSON では null。キーは省かない)。JSON が壊れている・オブジェクトでなければ失敗にする
// (503 master_unavailable に写る)。
func masterEffectFor(raw []byte) (*api.MasterEffect, error) {
	if raw == nil {
		return nil, nil
	}
	m, err := decodeEffectVerbatim(raw)
	if err != nil {
		return nil, err
	}
	effect := api.MasterEffect(m)
	return &effect, nil
}

func nullStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func pokeTypePtr(v sql.NullString) *api.PokeType {
	if !v.Valid {
		return nil
	}
	t := api.PokeType(v.String)
	return &t
}

func statKeyPtr(v sql.NullString) *api.StatKey {
	if !v.Valid {
		return nil
	}
	k := api.StatKey(v.String)
	return &k
}

// errEmpty は「未投入」を表すセンチネル(内部の判定用。応答には出さず 503 master_unavailable に写す)。
type errEmpty string

func (e errEmpty) Error() string { return "empty: " + string(e) }
