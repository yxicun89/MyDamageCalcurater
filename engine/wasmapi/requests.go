package wasmapi

// 3つの境界関数のリクエスト DTO と、engine 公開 API の呼び出し(ADR-0011 §3)。
// 各 run は「変換 → 入力検証 → engine 呼び出し → 結果の写し」だけを行う。

import (
	"fmt"

	"example.com/pokecalc/engine"
)

// validateIndividual は engine.Individual.Validate の失敗を invalid_input にする。
func validateIndividual(label string, in engine.Individual) error {
	if err := in.Validate(); err != nil {
		return fail(CodeInvalidInput, "%s の入力が不正: %v", label, err)
	}
	return nil
}

// validateSpecies は種族だけの妥当性(種族値・タイプ数)を Individual.Validate で確かめる。
// SP・性格・ランクは既定値なので、種族由来の失敗だけが出る。
func validateSpecies(label string, sp engine.Species) error {
	return validateIndividual(label, engine.Individual{Species: sp})
}

// --- calc -------------------------------------------------------------------

type calcRequest struct {
	Format   string        `json:"format"`
	Attacker individualDTO `json:"attacker"`
	Defender individualDTO `json:"defender"`
	Move     moveDTO       `json:"move"`
	Field    fieldDTO      `json:"field"`
	Critical bool          `json:"critical"`
	// TypeChart は必須。省略は type_chart_missing(ADR-0011 §13)。
	TypeChart typeChartDTO `json:"typeChart"`
}

func (r *calcRequest) run() (calcResultDTO, error) {
	format, err := parseFormat(r.Format)
	if err != nil {
		return calcResultDTO{}, err
	}
	attacker, err := r.Attacker.toEngine("attacker")
	if err != nil {
		return calcResultDTO{}, err
	}
	defender, err := r.Defender.toEngine("defender")
	if err != nil {
		return calcResultDTO{}, err
	}
	move, err := r.Move.toEngine("move")
	if err != nil {
		return calcResultDTO{}, err
	}
	field, err := r.Field.toEngine()
	if err != nil {
		return calcResultDTO{}, err
	}
	chart, err := r.TypeChart.toEngine("typeChart")
	if err != nil {
		return calcResultDTO{}, err
	}
	if err := validateIndividual("攻撃側", attacker); err != nil {
		return calcResultDTO{}, err
	}
	if err := validateIndividual("防御側", defender); err != nil {
		return calcResultDTO{}, err
	}

	res, err := engine.CalcDamage(engine.DamageInput{
		Format: format, Attacker: attacker, Defender: defender, Move: move, Field: field, Critical: r.Critical,
		TypeChart: chart,
	})
	if err != nil {
		return calcResultDTO{}, err
	}
	return calcResultFrom(res), nil
}

// --- calcBulk ---------------------------------------------------------------

type presetDTO struct {
	Key     string    `json:"key"`
	Label   string    `json:"label"`
	SP      statsDTO  `json:"sp"`
	Nature  natureDTO `json:"nature"`
	Applies string    `json:"applies"`
}

type bulkRequest struct {
	Format          string        `json:"format"`
	Attacker        individualDTO `json:"attacker"`
	DefenderSpecies speciesDTO    `json:"defenderSpecies"`
	Move            moveDTO       `json:"move"`
	Field           fieldDTO      `json:"field"`
	Critical        bool          `json:"critical"`
	Presets         []presetDTO   `json:"presets"`
	PresetKeys      []string      `json:"presetKeys"`
	ItemVariants    []*itemDTO    `json:"itemVariants"`
	// TypeChart は必須。省略は type_chart_missing(ADR-0011 §13)。
	TypeChart typeChartDTO `json:"typeChart"`
}

type bulkDefenderDTO struct {
	SP     statsDTO  `json:"sp"`
	Nature natureDTO `json:"nature"`
	Stats  statsDTO  `json:"stats"`
}

type bulkRowDTO struct {
	Preset      string          `json:"preset"`
	PresetLabel string          `json:"presetLabel"`
	ItemID      string          `json:"itemId"`
	Defender    bulkDefenderDTO `json:"defender"`
	Result      calcResultDTO   `json:"result"`
}

type bulkResultDTO struct {
	DefenderSpeciesKey string       `json:"defenderSpeciesKey"`
	Rows               []bulkRowDTO `json:"rows"`
}

func (r *bulkRequest) run() (bulkResultDTO, error) {
	format, err := parseFormat(r.Format)
	if err != nil {
		return bulkResultDTO{}, err
	}
	attacker, err := r.Attacker.toEngine("attacker")
	if err != nil {
		return bulkResultDTO{}, err
	}
	species, err := r.DefenderSpecies.toEngine("defenderSpecies")
	if err != nil {
		return bulkResultDTO{}, err
	}
	move, err := r.Move.toEngine("move")
	if err != nil {
		return bulkResultDTO{}, err
	}
	field, err := r.Field.toEngine()
	if err != nil {
		return bulkResultDTO{}, err
	}
	chart, err := r.TypeChart.toEngine("typeChart")
	if err != nil {
		return bulkResultDTO{}, err
	}
	var presets []engine.DefenderPreset
	if r.Presets != nil {
		presets = make([]engine.DefenderPreset, 0, len(r.Presets))
	}
	for i, p := range r.Presets {
		path := fmt.Sprintf("presets[%d]", i)
		nature, err := p.Nature.toEngine(path + ".nature")
		if err != nil {
			return bulkResultDTO{}, err
		}
		applies, err := parseCategory(path+".applies", p.Applies, true)
		if err != nil {
			return bulkResultDTO{}, err
		}
		presets = append(presets, engine.DefenderPreset{
			Key: engine.PresetKey(p.Key), Label: p.Label, SP: p.SP.toEngine(), Nature: nature, Applies: applies,
		})
	}
	var keys []engine.PresetKey
	if r.PresetKeys != nil {
		keys = make([]engine.PresetKey, 0, len(r.PresetKeys))
	}
	for _, k := range r.PresetKeys {
		keys = append(keys, engine.PresetKey(k))
	}
	variants, err := itemsToEngine("itemVariants", r.ItemVariants)
	if err != nil {
		return bulkResultDTO{}, err
	}
	if err := validateIndividual("攻撃側", attacker); err != nil {
		return bulkResultDTO{}, err
	}
	if err := validateSpecies("防御側の種族", species); err != nil {
		return bulkResultDTO{}, err
	}

	res, err := engine.CalcBulk(engine.BulkInput{
		Format: format, Attacker: attacker, DefenderSpecies: species, Move: move, Field: field,
		Critical: r.Critical, Presets: presets, PresetKeys: keys, ItemVariants: variants, TypeChart: chart,
	})
	if err != nil {
		return bulkResultDTO{}, err
	}

	rows := make([]bulkRowDTO, 0, len(res.Rows))
	for _, row := range res.Rows {
		rows = append(rows, bulkRowDTO{
			Preset:      string(row.Preset),
			PresetLabel: row.PresetLabel,
			ItemID:      row.ItemID,
			Defender: bulkDefenderDTO{
				SP:     statsFrom(row.Defender.SP),
				Nature: natureFrom(row.Defender.Nature),
				Stats:  statsFrom(engine.RealStats(row.Defender)),
			},
			Result: calcResultFrom(row.Result),
		})
	}
	return bulkResultDTO{DefenderSpeciesKey: res.DefenderSpeciesKey, Rows: rows}, nil
}

// --- calcReverse ------------------------------------------------------------

type observationDTO struct {
	Percent int    `json:"percent"`
	Damage  int    `json:"damage"`
	Note    string `json:"note"`
}

type reverseRequest struct {
	Format         string           `json:"format"`
	Side           string           `json:"side"`
	Known          individualDTO    `json:"known"`
	UnknownSpecies speciesDTO       `json:"unknownSpecies"`
	Move           moveDTO          `json:"move"`
	Field          fieldDTO         `json:"field"`
	Critical       bool             `json:"critical"`
	ItemCandidates []*itemDTO       `json:"itemCandidates"`
	Observations   []observationDTO `json:"observations"`
	MaxCandidates  int              `json:"maxCandidates"`
	// TypeChart は必須。省略は type_chart_missing(ADR-0011 §13)。
	TypeChart typeChartDTO `json:"typeChart"`
}

type archetypeDTO struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Side        string `json:"side"`
	Stat        string `json:"stat"`
	HPBucket    string `json:"hpBucket"`
	StatBucket  string `json:"statBucket"`
	NatureClass string `json:"natureClass"`
}

type reverseCandidateDTO struct {
	Archetype   archetypeDTO `json:"archetype"`
	ItemID      string       `json:"itemId"`
	SP          statsDTO     `json:"sp"`
	Nature      natureDTO    `json:"nature"`
	MatchScore  float64      `json:"matchScore"`
	Exact       bool         `json:"exact"`
	MinPercent  int          `json:"minPercent"`
	MaxPercent  int          `json:"maxPercent"`
	Points      int          `json:"points"`
	ExactPoints int          `json:"exactPoints"`
}

type reverseResultDTO struct {
	Side       string                `json:"side"`
	Stat       string                `json:"stat"`
	ExactCount int                   `json:"exactCount"`
	Candidates []reverseCandidateDTO `json:"candidates"`
}

func (r *reverseRequest) run() (reverseResultDTO, error) {
	format, err := parseFormat(r.Format)
	if err != nil {
		return reverseResultDTO{}, err
	}
	known, err := r.Known.toEngine("known")
	if err != nil {
		return reverseResultDTO{}, err
	}
	species, err := r.UnknownSpecies.toEngine("unknownSpecies")
	if err != nil {
		return reverseResultDTO{}, err
	}
	move, err := r.Move.toEngine("move")
	if err != nil {
		return reverseResultDTO{}, err
	}
	field, err := r.Field.toEngine()
	if err != nil {
		return reverseResultDTO{}, err
	}
	chart, err := r.TypeChart.toEngine("typeChart")
	if err != nil {
		return reverseResultDTO{}, err
	}
	items, err := itemsToEngine("itemCandidates", r.ItemCandidates)
	if err != nil {
		return reverseResultDTO{}, err
	}
	var obs []engine.Observation
	if r.Observations != nil {
		obs = make([]engine.Observation, 0, len(r.Observations))
	}
	for _, o := range r.Observations {
		obs = append(obs, engine.Observation{Percent: o.Percent, Damage: o.Damage, Note: o.Note})
	}
	if err := validateIndividual("既知の側", known); err != nil {
		return reverseResultDTO{}, err
	}
	if err := validateSpecies("推定側の種族", species); err != nil {
		return reverseResultDTO{}, err
	}

	res, err := engine.CalcReverse(engine.ReverseInput{
		Format: format, Side: engine.ReverseSide(r.Side), Known: known, UnknownSpecies: species,
		Move: move, Field: field, Critical: r.Critical, ItemCandidates: items,
		Observations: obs, MaxCandidates: r.MaxCandidates, TypeChart: chart,
	})
	if err != nil {
		return reverseResultDTO{}, err
	}

	cands := make([]reverseCandidateDTO, 0, len(res.Candidates))
	for _, c := range res.Candidates {
		cands = append(cands, reverseCandidateDTO{
			Archetype: archetypeDTO{
				Key: string(c.Archetype.Key), Label: c.Archetype.Label,
				Side: string(c.Archetype.Side), Stat: string(c.Archetype.Stat),
				HPBucket: string(c.Archetype.HPBucket), StatBucket: string(c.Archetype.StatBucket),
				NatureClass: string(c.Archetype.NatureClass),
			},
			ItemID:      c.ItemID,
			SP:          statsFrom(c.SP),
			Nature:      natureFrom(c.Nature),
			MatchScore:  c.MatchScore,
			Exact:       c.Exact,
			MinPercent:  c.MinPercent,
			MaxPercent:  c.MaxPercent,
			Points:      c.Points,
			ExactPoints: c.ExactPoints,
		})
	}
	return reverseResultDTO{
		Side: string(res.Side), Stat: string(res.Stat), ExactCount: res.ExactCount, Candidates: cands,
	}, nil
}
