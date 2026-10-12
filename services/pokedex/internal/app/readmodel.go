package app

// `pokedex export`(balance・speed 向けの read model 6ファイル。ADR-0100 §8・ADR-0105 §5・ADR-0128)の手順。
// ファイルへの書き出しは adapters/readmodel が行う(ここはバイト列を作るまで)。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"example.com/pokecalc/services/pokedex/internal/domain"
)

// typeChartSource・typeChartGeneration・typeChartNote は type-chart.json の出典欄
// (balance の type-chart.schema.json。ADR-0013・ADR-0128)。generation 0 は golden と同じ Champions の番号。
const (
	typeChartSource     = "pokedex"
	typeChartGeneration = 0
	typeChartNote       = "Effectiveness x2 as an integer: 0=immune, 1=not very effective, 2=neutral, 4=super effective. A pair missing from the DB is neutral (2). Exported from the pokedex type_chart table (ADR-0013, ADR-0128)."
)

// neutralCode は DB に行が無い攻撃×防御の組の相性コード(importer は等倍の組を保存しない)。
const neutralCode = 2

// exportSchemaVersion は各ファイルの schemaVersion(いまは 1 だけ)。
const exportSchemaVersion = 1

// ErrNoDefaultRegulation は既定のレギュレーションが無い(regulations に is_default=1 の行が無い)。
var ErrNoDefaultRegulation = errors.New("readmodel: 既定のレギュレーションが無い")

// ReadModelFiles は export の出力(6ファイルの中身)。
type ReadModelFiles struct {
	PokemonTypes []byte
	Moves        []byte
	Abilities    []byte
	SpeedPokemon []byte
	TypeChart    []byte // type-chart.json(balance の type-chart.schema.json の形)
	Metadata     []byte // metadata.json({"schemaVersion":1,"dataVersion":...})
}

// TruncatedAbility は balance の上限を超えて落とした特性(pokemonId・abilityId)。
type TruncatedAbility struct {
	PokemonID string
	AbilityID string
}

// ReadModelReport は ExportReadModel の副次的な結果。
type ReadModelReport struct {
	TruncatedAbilities []TruncatedAbility
	// IgnoredEffectFields は、このバイナリが知らないため読み捨てた特性の効果フィールド(ADR-0145)。
	// 名前の昇順。Count はそのフィールドを持っていた特性の数。
	IgnoredEffectFields []IgnoredEffectField
}

// IgnoredEffectField は読み捨てた効果フィールド名と、それを持っていた特性の数。
type IgnoredEffectField struct {
	Field string
	Count int
}

// --- on-disk の形(ADR-0105 §5) ---------------------------------------------------

type pokemonTypesFile struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Pokemon       []pokemonTypesEntry `json:"pokemon"`
}

type pokemonTypesEntry struct {
	PokemonID  string   `json:"pokemonId"`
	NameJa     string   `json:"nameJa"`
	Types      []string `json:"types"`
	AbilityIDs []string `json:"abilityIds"`
}

type movesFile struct {
	SchemaVersion int         `json:"schemaVersion"`
	Moves         []moveEntry `json:"moves"`
}

type moveEntry struct {
	MoveID   string `json:"moveId"`
	Type     string `json:"type"`
	Category string `json:"category"`
}

type abilitiesFile struct {
	SchemaVersion int            `json:"schemaVersion"`
	Abilities     []abilityEntry `json:"abilities"`
}

type abilityEntry struct {
	AbilityID string                     `json:"abilityId"`
	Effects   []domain.AbilityTypeEffect `json:"effects"`
}

type typeChartFile struct {
	SchemaVersion int                       `json:"schemaVersion"`
	Source        string                    `json:"source"`
	Version       string                    `json:"version"`
	Generation    int                       `json:"generation"`
	Note          string                    `json:"note"`
	ExcludedTypes []string                  `json:"excludedTypes"`
	Types         []string                  `json:"types"`
	Effectiveness map[string]map[string]int `json:"effectiveness"`
}

type metadataFile struct {
	SchemaVersion int    `json:"schemaVersion"`
	DataVersion   string `json:"dataVersion"`
}

type speedFile struct {
	SchemaVersion int          `json:"schemaVersion"`
	RegulationID  string       `json:"regulationId"`
	Pokemon       []speedEntry `json:"pokemon"`
}

type speedEntry struct {
	PokemonID string   `json:"pokemonId"`
	NameJa    string   `json:"nameJa"`
	Types     []string `json:"types"`
	BaseSpeed int      `json:"baseSpeed"`
}

// ExportReadModel は DB を読み、balance・speed 向けの6ファイルを組み立てる(ADR-0105 §5)。
// 対象は既定のレギュレーションの使用可能集合。失敗時はゼロ値を返す(部分的な出力をしない)。
func ExportReadModel(ctx context.Context, b SnapshotBeginner) (ReadModelFiles, ReadModelReport, error) {
	tx, err := b.BeginSnapshot(ctx)
	if err != nil {
		return ReadModelFiles{}, ReadModelReport{}, err
	}
	defer func() { _ = tx.Rollback() }()
	files, rep, err := exportFrom(ctx, tx)
	if err != nil {
		return ReadModelFiles{}, ReadModelReport{}, err
	}
	if err := tx.Commit(); err != nil {
		return ReadModelFiles{}, ReadModelReport{}, err
	}
	return files, rep, nil
}

func exportFrom(ctx context.Context, q Reader) (ReadModelFiles, ReadModelReport, error) {
	reg, err := q.GetDefaultRegulation(ctx)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ReadModelFiles{}, ReadModelReport{}, ErrNoDefaultRegulation
		}
		return ReadModelFiles{}, ReadModelReport{}, err
	}

	versions, err := q.ListDataVersions(ctx)
	if err != nil {
		return ReadModelFiles{}, ReadModelReport{}, err
	}
	if len(versions) == 0 {
		return ReadModelFiles{}, ReadModelReport{}, fmt.Errorf("%w: data_versions が空(版の無い read model を出さない)", domain.ErrInvalidExport)
	}
	dataVersion := domain.DataVersionString(versions)

	speciesKeys, err := q.ListRegulationSpeciesKeys(ctx, reg.ID)
	if err != nil {
		return ReadModelFiles{}, ReadModelReport{}, err
	}
	if len(speciesKeys) == 0 {
		return ReadModelFiles{}, ReadModelReport{}, fmt.Errorf("%w: 既定のレギュレーションの使用可能な種族が0件", domain.ErrInvalidExport)
	}
	moveIDs, err := q.ListRegulationMoveIDs(ctx, reg.ID)
	if err != nil {
		return ReadModelFiles{}, ReadModelReport{}, err
	}
	if len(moveIDs) == 0 {
		return ReadModelFiles{}, ReadModelReport{}, fmt.Errorf("%w: 既定のレギュレーションの使用可能な技が0件", domain.ErrInvalidExport)
	}
	abilityIDs, err := q.ListRegulationAbilityIDs(ctx, reg.ID)
	if err != nil {
		return ReadModelFiles{}, ReadModelReport{}, err
	}
	if len(abilityIDs) == 0 {
		return ReadModelFiles{}, ReadModelReport{}, fmt.Errorf("%w: 既定のレギュレーションの使用可能な特性が0件", domain.ErrInvalidExport)
	}

	allTypes, err := q.ListTypes(ctx)
	if err != nil {
		return ReadModelFiles{}, ReadModelReport{}, err
	}
	allTypeChart, err := q.ListTypeChart(ctx)
	if err != nil {
		return ReadModelFiles{}, ReadModelReport{}, err
	}
	chart, err := typeChartOf(allTypes, allTypeChart)
	if err != nil {
		return ReadModelFiles{}, ReadModelReport{}, fmt.Errorf("%w: タイプ相性表を組み立てられない: %v", domain.ErrInvalidExport, err)
	}

	allSpecies, err := q.ListSpecies(ctx)
	if err != nil {
		return ReadModelFiles{}, ReadModelReport{}, err
	}
	allSpeciesAbilities, err := q.ListAllSpeciesAbilities(ctx)
	if err != nil {
		return ReadModelFiles{}, ReadModelReport{}, err
	}
	allMoves, err := q.ListMoves(ctx)
	if err != nil {
		return ReadModelFiles{}, ReadModelReport{}, err
	}
	allAbilityEffects, err := q.ListAbilityEffects(ctx)
	if err != nil {
		return ReadModelFiles{}, ReadModelReport{}, err
	}

	speciesSet := toSet(speciesKeys)
	moveSet := toSet(moveIDs)
	abilitySet := toSet(abilityIDs)

	abilitiesBySpecies := map[string][]SpeciesAbility{}
	for _, sa := range allSpeciesAbilities {
		abilitiesBySpecies[sa.SpeciesKey] = append(abilitiesBySpecies[sa.SpeciesKey], sa)
	}
	for key := range abilitiesBySpecies {
		sort.Slice(abilitiesBySpecies[key], func(i, j int) bool {
			return abilitiesBySpecies[key][i].Slot < abilitiesBySpecies[key][j].Slot
		})
	}

	var species []Species
	for _, s := range allSpecies {
		if speciesSet[s.Key] {
			species = append(species, s)
		}
	}
	sort.Slice(species, func(i, j int) bool { return species[i].Key < species[j].Key })

	var report ReadModelReport
	pokemonEntries := make([]pokemonTypesEntry, 0, len(species))
	speedEntries := make([]speedEntry, 0, len(species))
	for _, s := range species {
		types := speciesTypes(s)
		var ids []string
		for _, sa := range abilitiesBySpecies[s.Key] {
			if len(ids) >= domain.MaxCatalogAbilityCount {
				report.TruncatedAbilities = append(report.TruncatedAbilities, TruncatedAbility{PokemonID: s.Key, AbilityID: sa.AbilityID})
				continue
			}
			ids = append(ids, sa.AbilityID)
		}
		if ids == nil {
			ids = []string{}
		}
		pokemonEntries = append(pokemonEntries, pokemonTypesEntry{PokemonID: s.Key, NameJa: s.NameJa, Types: types, AbilityIDs: ids})
		speedEntries = append(speedEntries, speedEntry{PokemonID: s.Key, NameJa: s.NameJa, Types: types, BaseSpeed: s.BaseStats.Spe})
	}
	sort.Slice(report.TruncatedAbilities, func(i, j int) bool {
		if report.TruncatedAbilities[i].PokemonID != report.TruncatedAbilities[j].PokemonID {
			return report.TruncatedAbilities[i].PokemonID < report.TruncatedAbilities[j].PokemonID
		}
		return report.TruncatedAbilities[i].AbilityID < report.TruncatedAbilities[j].AbilityID
	})

	moves := make([]moveEntry, 0, len(allMoves))
	for _, m := range allMoves {
		if moveSet[m.ID] {
			moves = append(moves, moveEntry{MoveID: m.ID, Type: m.Type, Category: m.Category})
		}
	}
	sort.Slice(moves, func(i, j int) bool { return moves[i].MoveID < moves[j].MoveID })

	effectByAbility := map[string][]byte{}
	for _, e := range allAbilityEffects {
		effectByAbility[e.AbilityID] = e.Effect
	}
	abilityIDsSorted := sortedStrings(abilitySet)
	abilityEntries := make([]abilityEntry, 0, len(abilityIDsSorted))
	ignoredCount := map[string]int{}
	for _, id := range abilityIDsSorted {
		effects, err := domain.NormalizedAbilityEffects(effectByAbility[id], chart, ignoredCount)
		if err != nil {
			return ReadModelFiles{}, ReadModelReport{}, err
		}
		abilityEntries = append(abilityEntries, abilityEntry{AbilityID: id, Effects: effects})
	}
	pokemonJSON, err := marshalLine(pokemonTypesFile{SchemaVersion: exportSchemaVersion, Pokemon: pokemonEntries})
	if err != nil {
		return ReadModelFiles{}, ReadModelReport{}, err
	}
	movesJSON, err := marshalLine(movesFile{SchemaVersion: exportSchemaVersion, Moves: moves})
	if err != nil {
		return ReadModelFiles{}, ReadModelReport{}, err
	}
	abilitiesJSON, err := marshalLine(abilitiesFile{SchemaVersion: exportSchemaVersion, Abilities: abilityEntries})
	if err != nil {
		return ReadModelFiles{}, ReadModelReport{}, err
	}
	speedJSON, err := marshalLine(speedFile{SchemaVersion: exportSchemaVersion, RegulationID: reg.ID, Pokemon: speedEntries})
	if err != nil {
		return ReadModelFiles{}, ReadModelReport{}, err
	}
	typeChartJSON, err := marshalLine(typeChartExport(allTypes, allTypeChart, dataVersion))
	if err != nil {
		return ReadModelFiles{}, ReadModelReport{}, err
	}
	for _, name := range sortedStrings(ignoredCount) {
		report.IgnoredEffectFields = append(report.IgnoredEffectFields, IgnoredEffectField{Field: name, Count: ignoredCount[name]})
	}
	metadataJSON, err := marshalLine(metadataFile{SchemaVersion: exportSchemaVersion, DataVersion: dataVersion})
	if err != nil {
		return ReadModelFiles{}, ReadModelReport{}, err
	}

	return ReadModelFiles{
		PokemonTypes: pokemonJSON, Moves: movesJSON, Abilities: abilitiesJSON, SpeedPokemon: speedJSON,
		TypeChart: typeChartJSON, Metadata: metadataJSON,
	}, report, nil
}

// typeChartExport は DB の types・type_chart から type-chart.json の中身を作る。types は ID 昇順、
// effectiveness は types × types の全組(行が無い組は等倍)。使用可能集合では絞らない。タイプ数は検査しない(ADR-0128 §2)。
func typeChartExport(types []Type, chart []TypeChartEntry, dataVersion string) typeChartFile {
	ids := make([]string, 0, len(types))
	for _, t := range types {
		ids = append(ids, t.ID)
	}
	sort.Strings(ids)
	codes := make(map[[2]string]int, len(chart))
	for _, c := range chart {
		codes[[2]string{c.AttackType, c.DefenseType}] = c.Code
	}
	eff := make(map[string]map[string]int, len(ids))
	for _, a := range ids {
		row := make(map[string]int, len(ids))
		for _, d := range ids {
			code, ok := codes[[2]string{a, d}]
			if !ok {
				code = neutralCode
			}
			row[d] = code
		}
		eff[a] = row
	}
	return typeChartFile{
		SchemaVersion: exportSchemaVersion, Source: typeChartSource, Version: dataVersion,
		Generation: typeChartGeneration, Note: typeChartNote,
		ExcludedTypes: []string{}, Types: ids, Effectiveness: eff,
	}
}

// speciesTypes は [type1] か [type1, type2] を返す。
func speciesTypes(s Species) []string {
	if s.Type2 != nil && *s.Type2 != "" {
		return []string{s.Type1, *s.Type2}
	}
	return []string{s.Type1}
}

func toSet(ids []string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func sortedStrings[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// marshalLine は v を空白なしの正準形(json.Marshal)にし、末尾に改行を1つ付ける。
func marshalLine(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
