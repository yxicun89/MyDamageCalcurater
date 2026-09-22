// Package readmodel は `pokedex export`(balance・speed 向けの read model。ADR-0100 §8・ADR-0105 §5)。
// HTTP に依存しない。DB は store.Querier(sqlc の生成インターフェース)経由でだけ読む。
package readmodel

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
	"example.com/pokecalc/services/pokedex/internal/store"
)

// File* は WriteDir が書くファイル名(balance・speed の設定 BALANCE_*_PATH・SPEED_POKEMON_PATH が指す名前。ADR-0105 §5)。
const (
	FilePokemonTypes = "pokemon-types.json"
	FileMoves        = "moves.json"
	FileAbilities    = "abilities.json"
	FileSpeedPokemon = "speed-pokemon.json"
)

// exportSchemaVersion は各ファイルの schemaVersion(いまは 1 だけ)。
const exportSchemaVersion = 1

// maxCatalogAbilityCount は balance の read model が受け付ける abilityIds の上限
// (services/balance/schema/pokemon-types.schema.json の maxItems・loader の maxCatalogAbilityCount と同じ。
// 4件ある種族は slot 4(Showdown の特殊枠)を落とす。ADR-0105 §5・限界2)。
const maxCatalogAbilityCount = 4

// ErrNoDefaultRegulation は既定のレギュレーションが無い(regulations に is_default=1 の行が無い)。
var ErrNoDefaultRegulation = errors.New("readmodel: 既定のレギュレーションが無い")

// ErrInvalidExport は出力として成立しない(使用可能集合が空・効果定義が検証を通らない・
// 係数が 1〜16 の比にならない)。
var ErrInvalidExport = errors.New("readmodel: read model として出力できない")

// Files は export の出力(4ファイル)。
type Files struct {
	PokemonTypes []byte
	Moves        []byte
	Abilities    []byte
	SpeedPokemon []byte
}

// TruncatedAbility は balance の上限を超えて落とした特性(pokemonId・abilityId)。
type TruncatedAbility struct {
	PokemonID string
	AbilityID string
}

// Report は Export の副次的な結果。
type Report struct {
	TruncatedAbilities []TruncatedAbility
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
	AbilityID string        `json:"abilityId"`
	Effects   []effectEntry `json:"effects"`
}

// effectEntry は特性の防御側のタイプ相性に関わる効果(ADR-0017 §2。ADR-0105 §5・ADR-0106 §決定7)。
// Numerator/Denominator は immune/absorb の行では意味を持たないので省く
// (abilities.schema.json の factor は minimum 1 で、immune/absorb では存在自体が不正になる)。
type effectEntry struct {
	Kind        string `json:"kind"`
	AttackType  string `json:"attackType,omitempty"`
	Numerator   int    `json:"numerator,omitempty"`
	Denominator int    `json:"denominator,omitempty"`
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

// Export は DB を読み、balance・speed 向けの4ファイルを組み立てる(ADR-0105 §5)。
// 対象は既定のレギュレーションの使用可能集合。失敗時は Files のゼロ値を返す(部分的な出力をしない)。
func Export(ctx context.Context, q store.Querier) (Files, Report, error) {
	reg, err := q.GetDefaultRegulation(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Files{}, Report{}, ErrNoDefaultRegulation
		}
		return Files{}, Report{}, err
	}

	speciesKeys, err := q.ListRegulationSpeciesKeys(ctx, reg.ID)
	if err != nil {
		return Files{}, Report{}, err
	}
	if len(speciesKeys) == 0 {
		return Files{}, Report{}, fmt.Errorf("%w: 既定のレギュレーションの使用可能な種族が0件", ErrInvalidExport)
	}
	moveIDs, err := q.ListRegulationMoveIDs(ctx, reg.ID)
	if err != nil {
		return Files{}, Report{}, err
	}
	if len(moveIDs) == 0 {
		return Files{}, Report{}, fmt.Errorf("%w: 既定のレギュレーションの使用可能な技が0件", ErrInvalidExport)
	}
	abilityIDs, err := q.ListRegulationAbilityIDs(ctx, reg.ID)
	if err != nil {
		return Files{}, Report{}, err
	}
	if len(abilityIDs) == 0 {
		return Files{}, Report{}, fmt.Errorf("%w: 既定のレギュレーションの使用可能な特性が0件", ErrInvalidExport)
	}

	allTypes, err := q.ListTypes(ctx)
	if err != nil {
		return Files{}, Report{}, err
	}
	allTypeChart, err := q.ListTypeChart(ctx)
	if err != nil {
		return Files{}, Report{}, err
	}
	chart, err := typeChartFrom(allTypes, allTypeChart)
	if err != nil {
		return Files{}, Report{}, fmt.Errorf("%w: タイプ相性表を組み立てられない: %v", ErrInvalidExport, err)
	}

	allSpecies, err := q.ListSpecies(ctx)
	if err != nil {
		return Files{}, Report{}, err
	}
	allSpeciesAbilities, err := q.ListAllSpeciesAbilities(ctx)
	if err != nil {
		return Files{}, Report{}, err
	}
	allMoves, err := q.ListMoves(ctx)
	if err != nil {
		return Files{}, Report{}, err
	}
	allAbilityEffects, err := q.ListAbilityEffects(ctx)
	if err != nil {
		return Files{}, Report{}, err
	}

	speciesSet := toSet(speciesKeys)
	moveSet := toSet(moveIDs)
	abilitySet := toSet(abilityIDs)

	abilitiesBySpecies := map[string][]store.SpeciesAbility{}
	for _, sa := range allSpeciesAbilities {
		abilitiesBySpecies[sa.SpeciesKey] = append(abilitiesBySpecies[sa.SpeciesKey], sa)
	}
	for key := range abilitiesBySpecies {
		sort.Slice(abilitiesBySpecies[key], func(i, j int) bool {
			return abilitiesBySpecies[key][i].Slot < abilitiesBySpecies[key][j].Slot
		})
	}

	var species []store.Species
	for _, s := range allSpecies {
		if speciesSet[s.Key] {
			species = append(species, s)
		}
	}
	sort.Slice(species, func(i, j int) bool { return species[i].Key < species[j].Key })

	var report Report
	pokemonEntries := make([]pokemonTypesEntry, 0, len(species))
	speedEntries := make([]speedEntry, 0, len(species))
	for _, s := range species {
		types := speciesTypes(s)
		var ids []string
		for _, sa := range abilitiesBySpecies[s.Key] {
			if len(ids) >= maxCatalogAbilityCount {
				report.TruncatedAbilities = append(report.TruncatedAbilities, TruncatedAbility{PokemonID: s.Key, AbilityID: sa.AbilityID})
				continue
			}
			ids = append(ids, sa.AbilityID)
		}
		if ids == nil {
			ids = []string{}
		}
		pokemonEntries = append(pokemonEntries, pokemonTypesEntry{PokemonID: s.Key, NameJa: s.NameJa, Types: types, AbilityIDs: ids})
		speedEntries = append(speedEntries, speedEntry{PokemonID: s.Key, NameJa: s.NameJa, Types: types, BaseSpeed: int(s.BaseSpe)})
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
	for _, id := range abilityIDsSorted {
		effects, err := normalizedAbilityEffects(effectByAbility[id], chart)
		if err != nil {
			return Files{}, Report{}, err
		}
		abilityEntries = append(abilityEntries, abilityEntry{AbilityID: id, Effects: effects})
	}
	pokemonJSON, err := marshalLine(pokemonTypesFile{SchemaVersion: exportSchemaVersion, Pokemon: pokemonEntries})
	if err != nil {
		return Files{}, Report{}, err
	}
	movesJSON, err := marshalLine(movesFile{SchemaVersion: exportSchemaVersion, Moves: moves})
	if err != nil {
		return Files{}, Report{}, err
	}
	abilitiesJSON, err := marshalLine(abilitiesFile{SchemaVersion: exportSchemaVersion, Abilities: abilityEntries})
	if err != nil {
		return Files{}, Report{}, err
	}
	speedJSON, err := marshalLine(speedFile{SchemaVersion: exportSchemaVersion, RegulationID: reg.ID, Pokemon: speedEntries})
	if err != nil {
		return Files{}, Report{}, err
	}

	return Files{PokemonTypes: pokemonJSON, Moves: movesJSON, Abilities: abilitiesJSON, SpeedPokemon: speedJSON}, report, nil
}

// speciesTypes は [type1] か [type1, type2] を返す。
func speciesTypes(s store.Species) []string {
	if s.Type2.Valid && s.Type2.String != "" {
		return []string{s.Type1, s.Type2.String}
	}
	return []string{s.Type1}
}

// typeChartFrom は sqlc の行から engine.TypeChart を組み立てる(services/internal/master 経由。ADR-0100 §6)。
func typeChartFrom(types []store.Type, chart []store.TypeChart) (engine.TypeChart, error) {
	typeRows := make([]master.TypeRow, 0, len(types))
	for _, t := range types {
		typeRows = append(typeRows, master.TypeRow{ID: t.ID, SortOrder: int(t.SortOrder), NameJa: t.NameJa})
	}
	chartRows := make([]master.TypeChartRow, 0, len(chart))
	for _, c := range chart {
		chartRows = append(chartRows, master.TypeChartRow{AttackType: c.AttackType, DefenseType: c.DefenseType, Code: int(c.Code)})
	}
	return master.TypeChart(typeRows, chartRows)
}

// normalizedAbilityEffects は ability_effects の JSON を防御側のタイプ相性の効果だけに正規化する
// (ADR-0017 §2・ADR-0105 §5)。効果の行が無い・防御側の効果を持たない特性は空配列(null にしない)。
func normalizedAbilityEffects(raw []byte, chart engine.TypeChart) ([]effectEntry, error) {
	out := []effectEntry{}
	if raw == nil {
		return out, nil
	}
	effect, err := master.DecodeAbilityEffect(raw, chart)
	if err != nil {
		return nil, fmt.Errorf("%w: 特性の効果定義を読めない: %v", ErrInvalidExport, err)
	}
	for _, t := range sortedImmuneTypes(effect.DefImmuneTypes) {
		out = append(out, effectEntry{Kind: "immune", AttackType: string(t)})
	}
	for _, t := range sortedTypeKeys(effect.DefAbsorbTypes) {
		out = append(out, effectEntry{Kind: "absorb", AttackType: string(t)})
	}
	for _, t := range sortedTypeKeys(effect.DefResistType) {
		num, den, ok := reduceRatio(effect.DefResistType[t])
		if !ok {
			return nil, fmt.Errorf("%w: DefResistType[%s] の係数が1〜16の比にならない: %d/4096", ErrInvalidExport, t, effect.DefResistType[t])
		}
		out = append(out, effectEntry{Kind: "type_multiplier", AttackType: string(t), Numerator: num, Denominator: den})
	}
	if effect.ReduceSuperEffective != 0 {
		num, den, ok := reduceRatio(effect.ReduceSuperEffective)
		if !ok {
			return nil, fmt.Errorf("%w: ReduceSuperEffective の係数が1〜16の比にならない: %d/4096", ErrInvalidExport, effect.ReduceSuperEffective)
		}
		out = append(out, effectEntry{Kind: "super_effective_multiplier", Numerator: num, Denominator: den})
	}
	return out, nil
}

// reduceRatio は m/4096 を約分し、分子・分母が 1〜16 に収まるかを返す(近似しない。ADR-0105 §5)。
func reduceRatio(m int) (numerator, denominator int, ok bool) {
	if m <= 0 {
		return 0, 0, false
	}
	g := gcd(m, 4096)
	n, d := m/g, 4096/g
	if n < 1 || n > 16 || d < 1 || d > 16 {
		return 0, 0, false
	}
	return n, d, true
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func sortedTypeKeys[V any](m map[engine.Type]V) []engine.Type {
	out := make([]engine.Type, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// sortedImmuneTypes はタイプ ID 昇順にした DefImmuneTypes のコピーを返す(入力を書き換えない)。
func sortedImmuneTypes(types []engine.Type) []engine.Type {
	out := make([]engine.Type, len(types))
	copy(out, types)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func toSet(ids []string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func sortedStrings(m map[string]bool) []string {
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

// WriteDir は4ファイルを dir に書く。無ければ作る。各ファイルは同じディレクトリの一時ファイルに
// 書いてから rename する(読む側が書きかけを読まない。ADR-0105 §5)。dir がファイルならエラー。
func (f Files) WriteDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	files := []struct {
		name string
		data []byte
	}{
		{FilePokemonTypes, f.PokemonTypes},
		{FileMoves, f.Moves},
		{FileAbilities, f.Abilities},
		{FileSpeedPokemon, f.SpeedPokemon},
	}
	for _, file := range files {
		if err := writeFileAtomic(dir, file.name, file.data); err != nil {
			return err
		}
	}
	return nil
}

func writeFileAtomic(dir, name string, data []byte) (err error) {
	tmp, err := os.CreateTemp(dir, "."+name+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmpPath, filepath.Join(dir, name)); err != nil {
		return err
	}
	return nil
}
