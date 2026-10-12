package app

// GET /internal/pokedex/master(ADR-0105 §2・ADR-0204)の手順。calc-svc が起動時に取るマスタ一式。
// DB の全行を使用可能集合で絞らずに返す。

import (
	"context"
	"sort"

	"example.com/pokecalc/services/pokedex/internal/domain"
)

// MasterExport は内部 API のマスタ一式(契約の MasterExport への写しは adapters/httpapi が行う)。
type MasterExport struct {
	DataVersion string
	Types       []Type
	TypeChart   []TypeChartEntry
	Species     []MasterSpecies
	Moves       []MasterMove
	Items       []MasterItem
	Abilities   []MasterAbility
	Natures     []Nature
}

// MasterSpecies は種族と特性の枠(slot 昇順)。
type MasterSpecies struct {
	Species   Species
	Abilities []SpeciesAbility
}

// MasterMove は技(Target は DB の値のまま)と効果・機構・処理の定義・フラグ。
type MasterMove struct {
	Move   Move
	Effect *domain.Effect
	// Mechanisms は常に非 nil(通常の技は空。ADR-0121)。
	Mechanisms      []string
	MechanismParams *MechanismParams
	// Flags は move_flags が空(まだ取り込んでいない)なら nil(全技でキーを省く。ADR-0178 §3)。
	Flags *[]string
	// Rule は処理の定義の行が無い技では nil(ADR-0143 §6)。
	Rule *domain.Effect
}

// MasterItem は持ち物と効果。
type MasterItem struct {
	Item   Item
	Effect *domain.Effect
}

// MasterAbility は特性と効果。
type MasterAbility struct {
	Ability Ability
	Effect  *domain.Effect
}

// MasterExport は全 SELECT を1つの読み取り専用トランザクション(一貫したスナップショット)の中で行う
// (ADR-0127)。importer の全置換が間に commit されても、新旧が混在した組を返さない。
// 失敗はすべて 503 master_unavailable(理由 "GetMasterExport")。
func (s *Service) MasterExport(ctx context.Context) (MasterExport, error) {
	ex, err := s.buildMasterExport(ctx)
	if err != nil {
		return MasterExport{}, domain.Unavailable("GetMasterExport", err)
	}
	return ex, nil
}

func (s *Service) buildMasterExport(ctx context.Context) (MasterExport, error) {
	tx, err := s.store.BeginSnapshot(ctx)
	if err != nil {
		return MasterExport{}, err
	}
	defer func() { _ = tx.Rollback() }()
	ex, err := buildMasterExportFrom(ctx, tx)
	if err != nil {
		return MasterExport{}, err
	}
	if err := tx.Commit(); err != nil {
		return MasterExport{}, err
	}
	return ex, nil
}

func buildMasterExportFrom(ctx context.Context, q Reader) (MasterExport, error) {
	versions, err := q.ListDataVersions(ctx)
	if err != nil {
		return MasterExport{}, err
	}
	if len(versions) == 0 {
		return MasterExport{}, domain.ErrEmpty("data_versions")
	}
	types, err := q.ListTypes(ctx)
	if err != nil {
		return MasterExport{}, err
	}
	if len(types) == 0 {
		return MasterExport{}, domain.ErrEmpty("types")
	}
	typeChart, err := q.ListTypeChart(ctx)
	if err != nil {
		return MasterExport{}, err
	}
	species, err := q.ListSpecies(ctx)
	if err != nil {
		return MasterExport{}, err
	}
	speciesAbilities, err := q.ListAllSpeciesAbilities(ctx)
	if err != nil {
		return MasterExport{}, err
	}
	moves, err := q.ListMoves(ctx)
	if err != nil {
		return MasterExport{}, err
	}
	moveEffects, err := q.ListMoveEffects(ctx)
	if err != nil {
		return MasterExport{}, err
	}
	moveMechanisms, err := q.ListMoveMechanisms(ctx)
	if err != nil {
		return MasterExport{}, err
	}
	moveMechanismParams, err := q.ListMoveMechanismParams(ctx)
	if err != nil {
		return MasterExport{}, err
	}
	moveRules, err := q.ListMoveRules(ctx)
	if err != nil {
		return MasterExport{}, err
	}
	moveFlags, err := q.ListMoveFlags(ctx)
	if err != nil {
		return MasterExport{}, err
	}
	items, err := q.ListItems(ctx)
	if err != nil {
		return MasterExport{}, err
	}
	itemEffects, err := q.ListItemEffects(ctx)
	if err != nil {
		return MasterExport{}, err
	}
	abilities, err := q.ListAbilities(ctx)
	if err != nil {
		return MasterExport{}, err
	}
	abilityEffects, err := q.ListAbilityEffects(ctx)
	if err != nil {
		return MasterExport{}, err
	}
	natures, err := q.ListNatures(ctx)
	if err != nil {
		return MasterExport{}, err
	}
	if len(natures) == 0 {
		return MasterExport{}, domain.ErrEmpty("natures")
	}

	abilitiesBySpecies := map[string][]SpeciesAbility{}
	for _, sa := range speciesAbilities {
		abilitiesBySpecies[sa.SpeciesKey] = append(abilitiesBySpecies[sa.SpeciesKey], sa)
	}
	for key := range abilitiesBySpecies {
		sort.Slice(abilitiesBySpecies[key], func(i, j int) bool { return abilitiesBySpecies[key][i].Slot < abilitiesBySpecies[key][j].Slot })
	}
	moveEffectByID := map[string]MoveEffect{}
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
	mechanismParamsByMoveID := make(map[string]*MechanismParams, len(moveMechanismParams))
	for _, p := range moveMechanismParams {
		mechanismParamsByMoveID[p.MoveID] = mechanismParams(p)
	}
	moveRuleByID := make(map[string]MoveRule, len(moveRules))
	for _, r := range moveRules {
		moveRuleByID[r.MoveID] = r
	}
	// flagsByMoveID: 契約(MasterMove.flags)の「昇順」もここでソートする(ADR-0178)。
	// move_flags が空ならまだ取り込んでいない(不明)ので、全技で flags キーを省く(ADR-0178 §3)。
	flagsKnown := len(moveFlags) > 0
	flagsByMoveID := map[string][]string{}
	for _, f := range moveFlags {
		flagsByMoveID[f.MoveID] = append(flagsByMoveID[f.MoveID], f.Flag)
	}
	for id := range flagsByMoveID {
		sort.Strings(flagsByMoveID[id])
	}
	itemEffectByID := map[string]ItemEffect{}
	for _, e := range itemEffects {
		itemEffectByID[e.ItemID] = e
	}
	abilityEffectByID := map[string]AbilityEffect{}
	for _, e := range abilityEffects {
		abilityEffectByID[e.AbilityID] = e
	}

	masterSpecies := make([]MasterSpecies, 0, len(species))
	for _, sp := range species {
		masterSpecies = append(masterSpecies, MasterSpecies{Species: sp, Abilities: abilitiesBySpecies[sp.Key]})
	}

	masterMoves := make([]MasterMove, 0, len(moves))
	for _, m := range moves {
		effect, err := domain.EffectVerbatim(moveEffectByID[m.ID].Effect)
		if err != nil {
			return MasterExport{}, err
		}
		mechanisms := mechanismsByMoveID[m.ID]
		if mechanisms == nil {
			mechanisms = []string{} // 通常の技は空配列(null にしない。ADR-0121)
		}
		// 技の処理の定義(ADR-0143 §6)。行が無い技はキーを省く(定義なし)。
		var rule *domain.Effect
		if r, ok := moveRuleByID[m.ID]; ok {
			if rule, err = domain.EffectVerbatim(r.Rule); err != nil {
				return MasterExport{}, err
			}
		}
		var flags *[]string
		if flagsKnown {
			f := flagsByMoveID[m.ID]
			if f == nil {
				f = []string{} // フラグの無い技は空配列(既知のフラグなし)
			}
			flags = &f
		}
		masterMoves = append(masterMoves, MasterMove{
			Move: m, Effect: effect, Mechanisms: mechanisms, MechanismParams: mechanismParamsByMoveID[m.ID],
			Flags: flags, Rule: rule,
		})
	}
	masterItems := make([]MasterItem, 0, len(items))
	for _, it := range items {
		effect, err := domain.EffectVerbatim(itemEffectByID[it.ID].Effect)
		if err != nil {
			return MasterExport{}, err
		}
		masterItems = append(masterItems, MasterItem{Item: it, Effect: effect})
	}
	masterAbilities := make([]MasterAbility, 0, len(abilities))
	for _, a := range abilities {
		effect, err := domain.EffectVerbatim(abilityEffectByID[a.ID].Effect)
		if err != nil {
			return MasterExport{}, err
		}
		masterAbilities = append(masterAbilities, MasterAbility{Ability: a, Effect: effect})
	}

	return MasterExport{
		DataVersion: domain.DataVersionString(versions),
		Types:       types,
		TypeChart:   typeChart,
		Species:     masterSpecies,
		Moves:       masterMoves,
		Items:       masterItems,
		Abilities:   masterAbilities,
		Natures:     natures,
	}, nil
}
