package importer

// Convert は取得元スナップショット(Input)を ADR-0100 の行(Output)に変換する
// (ADR-0101 §4〜§9・§12)。

import (
	"fmt"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
)

// TypeRow は types テーブルの行。
type TypeRow struct {
	master.TypeRow
	NameJaSource string
}

// NamedRow は abilities / items テーブルの行。
type NamedRow struct {
	ID           string
	NameJa       string
	NameJaSource string
	NameEn       string
}

// MoveRow は moves テーブルの行。Accuracy 0 は必中(NULL)。
type MoveRow struct {
	ID           string
	NameJa       string
	NameJaSource string
	NameEn       string
	Type         string
	Category     string
	Power        int
	Accuracy     int
	PP           int
	Priority     int
}

// SpeciesRow は species + species_abilities の行。
type SpeciesRow struct {
	master.SpeciesRow
	NameJaSource string
	Abilities    []master.SpeciesAbilityRow
}

// EffectRow は item_effects / ability_effects の行(Effect は master.Encode*Effect の正準形)。
type EffectRow struct {
	ID     string
	Effect []byte
}

// LearnsetRow は learnsets の行。
type LearnsetRow struct {
	SpeciesKey string
	MoveID     string
}

// RegulationRow は regulations の行。
type RegulationRow struct {
	ID        string
	NameJa    string
	IsDefault bool
	StartsOn  string
	EndsOn    string
}

// RegulationMemberRow は regulation_species / _moves / _items / _abilities の行。
type RegulationMemberRow struct {
	RegulationID string
	MemberID     string
}

// Output は投入する行の集合(ADR-0100)。スライスは ID / key 順(決定的)。
type Output struct {
	Types               []TypeRow
	TypeChart           []master.TypeChartRow
	Abilities           []NamedRow
	Items               []NamedRow
	Moves               []MoveRow
	MoveEffects         []EffectRow
	Species             []SpeciesRow
	Natures             []NatureRow
	ItemEffects         []EffectRow
	AbilityEffects      []EffectRow
	Learnsets           []LearnsetRow
	Regulations         []RegulationRow
	RegulationSpecies   []RegulationMemberRow
	RegulationMoves     []RegulationMemberRow
	RegulationItems     []RegulationMemberRow
	RegulationAbilities []RegulationMemberRow
}

// Convert は Input を検証・変換する。ErrBlocked のときは Report.Blockers に列挙し、
// Output はゼロ値で返す(部分的な結果を投入に回さない)。
func Convert(in Input) (Output, Report, error) {
	var warnings []Finding
	usedOverrideSpecies := map[string]bool{}
	usedOverrideMoves := map[string]bool{}
	usedOverrideItems := map[string]bool{}
	usedOverrideAbilities := map[string]bool{}
	usedOverrideTypes := map[string]bool{}
	usedOverrideNatures := map[string]bool{}

	typesConv, typeWarnings, err := convertTypes(in, usedOverrideTypes)
	if err != nil {
		return Output{}, Report{}, err
	}
	warnings = append(warnings, typeWarnings...)

	chart, err := master.TypeChart(typesConv.MasterRows, typesConv.ChartRows)
	if err != nil {
		return Output{}, Report{}, fmt.Errorf("%w: タイプ相性表を組み立てられない: %v", ErrInvalidData, err)
	}

	includedItems, itemNameEn, itemWarnings := itemPresence(in.Calc.Items, in.Showdown.Items)
	warnings = append(warnings, itemWarnings...)

	moveConv, moveWarnings, moveBlockers, err := convertMoves(in, typesConv.NameToID)
	if err != nil {
		return Output{}, Report{}, err
	}
	warnings = append(warnings, moveWarnings...)

	moveEffectRows, moveEffectWarnings, moveEffectBlockers, err := buildMoveEffects(in.Showdown.Moves, moveConv.Included)
	if err != nil {
		return Output{}, Report{}, err
	}
	warnings = append(warnings, moveEffectWarnings...)

	speciesConv, speciesWarnings, speciesBlockers, err := convertSpecies(in, typesConv.NameToID, includedItems, usedOverrideSpecies)
	if err != nil {
		return Output{}, Report{}, err
	}
	warnings = append(warnings, speciesWarnings...)

	natureRows, natureWarnings, natureBlockers, err := convertNatures(in, usedOverrideNatures)
	if err != nil {
		return Output{}, Report{}, err
	}
	warnings = append(warnings, natureWarnings...)

	if len(moveBlockers)+len(speciesBlockers)+len(natureBlockers)+len(moveEffectBlockers) > 0 {
		blockers := append(append(append(append([]Finding{}, moveBlockers...), speciesBlockers...), natureBlockers...), moveEffectBlockers...)
		sortFindings(warnings)
		sortFindings(blockers)
		return Output{}, Report{Warnings: warnings, Blockers: blockers}, ErrBlocked
	}

	abilityIDSet := map[string]bool{}
	for _, sp := range speciesConv.Rows {
		for _, a := range sp.Abilities {
			abilityIDSet[a.AbilityID] = true
		}
	}
	abilityIDs := sortedKeysRaw(abilityIDSet)
	itemIDs := sortedKeysRaw(includedItems)

	// 取り込んだ特性が calc の一覧に無ければ警告(ADR-0101 §5。calc の一覧は特性の正ではない
	// ので取り込みは止めない)。
	calcAbilitySet := map[string]bool{}
	for _, name := range in.Calc.Abilities {
		calcAbilitySet[toID(name)] = true
	}
	for _, id := range abilityIDs {
		if !calcAbilitySet[id] {
			warnings = append(warnings, Finding{Kind: KindAbilityShowdownOnly, ID: id})
		}
	}

	pokeAPIMoves := newPokeAPILookup(in.PokeAPI.Moves)
	for i := range moveConv.Rows {
		r := &moveConv.Rows[i]
		res := resolveJaName(r.ID, in.Overrides.Moves, pokeAPIMoves[r.ID], in.Config.NameJaLanguages, r.NameEn, usedOverrideMoves)
		r.NameJa, r.NameJaSource = res.NameJa, res.Source
		if res.Source == "fallback_en" {
			warnings = append(warnings, Finding{Kind: KindNameFallback, ID: r.ID})
		}
	}

	itemRows, itemNameWarnings := buildNamedRows(itemIDs, itemNameEn, in.PokeAPI.Items, in.Overrides.Items, in.Config.NameJaLanguages, usedOverrideItems)
	warnings = append(warnings, itemNameWarnings...)

	abilityRows, abilityNameWarnings := buildNamedRows(abilityIDs, speciesConv.AbilityNameEn, in.PokeAPI.Abilities, in.Overrides.Abilities, in.Config.NameJaLanguages, usedOverrideAbilities)
	warnings = append(warnings, abilityNameWarnings...)

	itemEffectRows, itemEffectWarnings, err := buildItemEffects(in.Effects.Items, includedItems, chart)
	if err != nil {
		return Output{}, Report{}, err
	}
	warnings = append(warnings, itemEffectWarnings...)

	abilityEffectRows, abilityEffectWarnings, err := buildAbilityEffects(in.Effects.Abilities, abilityIDSet, chart)
	if err != nil {
		return Output{}, Report{}, err
	}
	warnings = append(warnings, abilityEffectWarnings...)

	learnsetRules, err := regulationLearnsetRules(in.Regulations)
	if err != nil {
		return Output{}, Report{}, err
	}
	learnsetRows, _, err := buildLearnsets(in.Showdown.Species, in.Showdown.Learnsets, speciesConv.Rows, moveConv.Included, learnsetRules)
	if err != nil {
		return Output{}, Report{}, err
	}

	moveIDs := make([]string, 0, len(moveConv.Rows))
	for _, m := range moveConv.Rows {
		moveIDs = append(moveIDs, m.ID)
	}

	regRows, regSpecies, regMoves, regItems, regAbilities, err := buildRegulations(
		in.Regulations, in.Showdown.Mod, speciesConv.RegulationKeys, moveIDs, itemIDs, speciesConv.RegulationAbilityIDs,
	)
	if err != nil {
		return Output{}, Report{}, err
	}

	warnings = append(warnings, checkOverrideUnused(in.Overrides.Species, usedOverrideSpecies)...)
	warnings = append(warnings, checkOverrideUnused(in.Overrides.Moves, usedOverrideMoves)...)
	warnings = append(warnings, checkOverrideUnused(in.Overrides.Items, usedOverrideItems)...)
	warnings = append(warnings, checkOverrideUnused(in.Overrides.Abilities, usedOverrideAbilities)...)
	warnings = append(warnings, checkOverrideUnused(in.Overrides.Types, usedOverrideTypes)...)
	warnings = append(warnings, checkOverrideUnused(in.Overrides.Natures, usedOverrideNatures)...)

	sortFindings(warnings)

	out := Output{
		Types:               typesConv.Rows,
		TypeChart:           typesConv.ChartRows,
		Abilities:           abilityRows,
		Items:               itemRows,
		Moves:               moveConv.Rows,
		MoveEffects:         moveEffectRows,
		Species:             speciesConv.Rows,
		Natures:             natureRows,
		ItemEffects:         itemEffectRows,
		AbilityEffects:      abilityEffectRows,
		Learnsets:           learnsetRows,
		Regulations:         regRows,
		RegulationSpecies:   regSpecies,
		RegulationMoves:     regMoves,
		RegulationItems:     regItems,
		RegulationAbilities: regAbilities,
	}

	if err := validateOutputMapsToEngine(out, chart); err != nil {
		return Output{}, Report{}, err
	}

	return out, Report{Warnings: warnings}, nil
}

// validateOutputMapsToEngine は投入前の最終防衛線として、全行が services/internal/master の
// 写像(→ engine の型)を実際に通ることを実行時に確かめる(ADR-0101 §1)。ここで失敗するのは
// Convert の他の検証の抜け・不整合を意味する。
func validateOutputMapsToEngine(out Output, chart engine.TypeChart) error {
	itemEffects := map[string][]byte{}
	for _, e := range out.ItemEffects {
		itemEffects[e.ID] = e.Effect
	}
	abilityEffects := map[string][]byte{}
	for _, e := range out.AbilityEffects {
		abilityEffects[e.ID] = e.Effect
	}
	moveEffects := map[string][]byte{}
	for _, e := range out.MoveEffects {
		moveEffects[e.ID] = e.Effect
	}

	for _, r := range out.Species {
		if _, err := master.Species(r.SpeciesRow, r.Abilities, chart); err != nil {
			return fmt.Errorf("%w: 種族 %s を engine の型に写像できない: %v", ErrInvalidData, r.Key, err)
		}
	}
	for _, r := range out.Moves {
		row := master.MoveRow{ID: r.ID, NameJa: r.NameJa, Type: r.Type, Category: r.Category, Power: r.Power, Priority: r.Priority, Effect: moveEffects[r.ID]}
		if _, err := master.Move(row, chart); err != nil {
			return fmt.Errorf("%w: 技 %s を engine の型に写像できない: %v", ErrInvalidData, r.ID, err)
		}
	}
	for _, r := range out.Items {
		row := master.ItemRow{ID: r.ID, NameJa: r.NameJa, Effect: itemEffects[r.ID]}
		if _, err := master.Item(row, chart); err != nil {
			return fmt.Errorf("%w: 持ち物 %s を engine の型に写像できない: %v", ErrInvalidData, r.ID, err)
		}
	}
	for _, r := range out.Abilities {
		row := master.AbilityRow{ID: r.ID, NameJa: r.NameJa, Effect: abilityEffects[r.ID]}
		if _, err := master.Ability(row, chart); err != nil {
			return fmt.Errorf("%w: 特性 %s を engine の型に写像できない: %v", ErrInvalidData, r.ID, err)
		}
	}
	return nil
}
