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
	// 件数の上限は DTO の変換より前に見る(issue #110。ADR-0208 §3・ADR-0108 決定3)。
	// engine.CalcBulk も同じ上限を検査するが(直接呼び出し用の防御)、ここで先に見ておくと
	// 「複数の違反が重なったとき HTTP と同じ invalid_input が先に出る」という parity が
	// DTO 変換の失敗(未知の enum 等)より優先される形になる。
	if len(r.Presets) > engine.MaxBulkPresets {
		return bulkResultDTO{}, fail(CodeInvalidInput, "presets は %d 件以下でなければならない: %d 件", engine.MaxBulkPresets, len(r.Presets))
	}
	if len(r.PresetKeys) > engine.MaxBulkPresets {
		return bulkResultDTO{}, fail(CodeInvalidInput, "presetKeys は %d 件以下でなければならない: %d 件", engine.MaxBulkPresets, len(r.PresetKeys))
	}
	if len(r.ItemVariants) > engine.MaxBulkItemVariants {
		return bulkResultDTO{}, fail(CodeInvalidInput, "itemVariants は %d 件以下でなければならない: %d 件", engine.MaxBulkItemVariants, len(r.ItemVariants))
	}

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
	Percent       int    `json:"percent"`
	PercentTenths int    `json:"percentTenths"`
	Damage        int    `json:"damage"`
	Note          string `json:"note"`
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

// spRangeDTO は逆算候補の SP 範囲(両端を含む。ADR-0010 §R3)。
type spRangeDTO struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// reverseCandidateDTO は候補1件(P1-12。ADR-0010 §R3・§R8)。
type reverseCandidateDTO struct {
	NatureClass string       `json:"natureClass"`
	Nature      natureDTO    `json:"nature"`
	ItemID      string       `json:"itemId"`
	Ranges      []spRangeDTO `json:"ranges"`
	SPCount     int          `json:"spCount"`
	Exact       bool         `json:"exact"`
	Mismatch    int          `json:"mismatch"`
	Support     int          `json:"support"`
	MinPercent  tenthPercent `json:"minPercent"`
	MaxPercent  tenthPercent `json:"maxPercent"`
}

type reverseResultDTO struct {
	Side        string                `json:"side"`
	Stat        string                `json:"stat"`
	AssumedHPSP int                   `json:"assumedHpSp"`
	ExactCount  int                   `json:"exactCount"`
	Candidates  []reverseCandidateDTO `json:"candidates"`
}

func (r *reverseRequest) run() (reverseResultDTO, error) {
	// 件数・範囲の上限は DTO の変換より前に見る(issue #110。ADR-0208 §3・ADR-0108 決定3)。
	// bulkRequest.run と同じ理由(parity が DTO 変換の失敗より優先される)。
	if len(r.ItemCandidates) > engine.MaxReverseItemCandidates {
		return reverseResultDTO{}, fail(CodeInvalidInput, "itemCandidates は %d 件以下でなければならない: %d 件", engine.MaxReverseItemCandidates, len(r.ItemCandidates))
	}
	if len(r.Observations) > engine.MaxReverseObservations {
		return reverseResultDTO{}, fail(CodeInvalidInput, "observations は %d 件以下でなければならない: %d 件", engine.MaxReverseObservations, len(r.Observations))
	}
	if r.MaxCandidates < 0 || r.MaxCandidates > engine.MaxReverseMaxCandidates {
		return reverseResultDTO{}, fail(CodeInvalidInput, "maxCandidates は 0..%d でなければならない: %d", engine.MaxReverseMaxCandidates, r.MaxCandidates)
	}

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
		obs = append(obs, engine.Observation{
			Percent: o.Percent, PercentTenths: o.PercentTenths, Damage: o.Damage, Note: o.Note,
		})
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
		ranges := make([]spRangeDTO, 0, len(c.Ranges))
		for _, r := range c.Ranges {
			ranges = append(ranges, spRangeDTO{Min: r.Min, Max: r.Max})
		}
		cands = append(cands, reverseCandidateDTO{
			NatureClass: string(c.NatureClass),
			Nature:      natureFrom(c.Nature),
			ItemID:      c.ItemID,
			Ranges:      ranges,
			SPCount:     c.SPCount,
			Exact:       c.Exact,
			Mismatch:    c.Mismatch,
			Support:     c.Support,
			MinPercent:  tenthPercent(c.MinPercentTenths),
			MaxPercent:  tenthPercent(c.MaxPercentTenths),
		})
	}
	return reverseResultDTO{
		Side: string(res.Side), Stat: string(res.Stat), AssumedHPSP: res.AssumedHPSP,
		ExactCount: res.ExactCount, Candidates: cands,
	}, nil
}
