package importer

// Reconcile は Convert に照合(calc と Showdown の値の全項目比較・P2-1c の裁定の確認・
// 効果定義の網羅性・日本語名・畳んだフォームの習得技の差・Showdown だけの種族)を足す
// (ADR-0103)。純粋。ネットワークにもファイルにも触らない(WriteReconciliation を除く)。

import (
	"errors"
	"fmt"
)

// ValueDiff は calc と Showdown の値の食い違い1件(ADR-0103 §3)。
type ValueDiff struct {
	Entity   string `json:"entity"` // "move" | "species"
	ID       string `json:"id"`
	Field    string `json:"field"`
	Calc     string `json:"calc"`
	Showdown string `json:"showdown"`
	Severity string `json:"severity"` // "blocker" | "warning" | "info"
	Imported bool   `json:"imported"`
}

// SetSummary は1カテゴリの件数の要約(ADR-0103 §4)。
type SetSummary struct {
	Calc             int `json:"calc"`
	Showdown         int `json:"showdown"`
	ShowdownStandard int `json:"showdownStandard"`
	Both             int `json:"both"`
	CalcOnly         int `json:"calcOnly"`
	ShowdownOnly     int `json:"showdownOnly"`
	Imported         int `json:"imported"`
}

// TypeChartSummary は相性表の件数(#269)。Rows は等倍を省いた行数で、0 や急な減少は
// 取得元の表の形の変化を疑う手がかりになる。
type TypeChartSummary struct {
	Types int `json:"types"`
	Rows  int `json:"rows"`
}

// Summary は件数の要約一式(ADR-0103 §4)。
type Summary struct {
	Moves         SetSummary          `json:"moves"`
	Species       SetSummary          `json:"species"`
	Items         SetSummary          `json:"items"`
	Abilities     SetSummary          `json:"abilities"`
	TypeChart     TypeChartSummary    `json:"typeChart"`
	WarningCounts map[FindingKind]int `json:"warningCounts"`
	BlockerCounts map[FindingKind]int `json:"blockerCounts"`
}

// VerdictCount / MoveVerdicts / Verdicts / ReconcileConfig は snapshot.go(Config の一部)。

// VerdictCheck は P2-1c の裁定1区分の照合結果(ADR-0103 §5)。
type VerdictCheck struct {
	Key            string   `json:"key"`
	ExpectedCount  int      `json:"expectedCount"`
	ActualCount    int      `json:"actualCount"`
	ExpectedSHA256 string   `json:"expectedSha256"`
	ActualSHA256   string   `json:"actualSha256"`
	IDs            []string `json:"ids"`
	OK             bool     `json:"ok"`
}

// CoverageStats は効果定義の網羅性の件数(ADR-0103 §6)。
type CoverageStats struct {
	Imported        int      `json:"imported"`
	WithDamageHooks int      `json:"withDamageHooks"`
	Defined         int      `json:"defined"`
	Missing         int      `json:"missing"`
	NoHook          int      `json:"noHook"`
	MissingIDs      []string `json:"missingIds"`
	NoHookIDs       []string `json:"noHookIds"`
}

// EffectCoverage は持ち物・特性の効果定義の網羅性(ADR-0103 §6)。
type EffectCoverage struct {
	Items     CoverageStats `json:"items"`
	Abilities CoverageStats `json:"abilities"`
}

// NameStats は1カテゴリの日本語名の解決状況(ADR-0103 §8)。
type NameStats struct {
	Override    int            `json:"override"`
	PokeAPI     int            `json:"pokeApi"`
	FallbackEn  int            `json:"fallbackEn"`
	ByLanguage  map[string]int `json:"byLanguage"`
	FallbackIDs []string       `json:"fallbackIds"`
}

// ShowdownOnlySpecies は Showdown だけにある種族1件(ADR-0103 §8)。
type ShowdownOnlySpecies struct {
	ID             string `json:"id"`
	Num            int    `json:"num"`
	Reason         string `json:"reason"` // folded | different-performance | num-not-in-calc | unresolvable
	Representative string `json:"representative,omitempty"`
}

// FoldedLearnsetDiff は畳んだフォームと代表の習得技の差(取り込む技に絞る。ADR-0103 §8)。
type FoldedLearnsetDiff struct {
	ID                   string   `json:"id"`
	Representative       string   `json:"representative"`
	OnlyInFolded         []string `json:"onlyInFolded,omitempty"`
	OnlyInRepresentative []string `json:"onlyInRepresentative,omitempty"`
}

// LearnsetStats は習得技の継承(進化前)の件数(ADR-0103 §7)。
type LearnsetStats struct {
	Inherited          int            `json:"inherited"`
	InheritedBySpecies map[string]int `json:"inheritedBySpecies,omitempty"`
}

// Reconciliation は Reconcile の結果一式(ADR-0103 §2・§11)。
type Reconciliation struct {
	SchemaVersion       int                   `json:"schemaVersion"`
	Sources             map[string]string     `json:"sources"`
	Partial             bool                  `json:"partial"`
	Summary             Summary               `json:"summary"`
	MoveDiffs           []ValueDiff           `json:"moveDiffs"`
	SpeciesDiffs        []ValueDiff           `json:"speciesDiffs"`
	VerdictChecks       []VerdictCheck        `json:"verdictChecks"`
	EffectCoverage      EffectCoverage        `json:"effectCoverage"`
	Names               map[string]NameStats  `json:"names"`
	ShowdownOnlySpecies []ShowdownOnlySpecies `json:"showdownOnlySpecies"`
	FoldedLearnsets     []FoldedLearnsetDiff  `json:"foldedLearnsets"`
	Learnsets           LearnsetStats         `json:"learnsets"`
	Report              Report                `json:"report"`
}

const reconciliationSchemaVersion = 1

// Reconcile は Convert を呼び、その Output・Report に照合結果を足す(ADR-0103 §1)。
func Reconcile(in Input) (Output, Reconciliation, error) {
	if in.Config.Reconcile == nil {
		return Output{}, Reconciliation{}, fmt.Errorf("%w: config.reconcile が無い(実データの取り込みでは照合の設定が必須)", ErrInvalidInput)
	}
	rc := in.Config.Reconcile

	out, convReport, convErr := Convert(in)
	if convErr != nil && !errors.Is(convErr, ErrBlocked) {
		return Output{}, Reconciliation{}, convErr
	}
	convertBlocked := errors.Is(convErr, ErrBlocked)

	// 照合も Convert と同じく、Showdown の技の別の版を正規の1件にまとめた入力で行う(ADR-0115 追記)。
	// まとめる前の入力のままだと、版の行が正規の id の値として比べられ、誤った差分・verdict になりうる。
	in.Showdown.Moves, _ = foldShowdownMoveVariants(in.Showdown.Moves)

	// 技の判定は種族と独立にできる(ADR-0103 §5)。Convert が種族で止まっても照合する。
	typesConv, _, err := convertTypes(in, map[string]bool{})
	if err != nil {
		return Output{}, Reconciliation{}, err
	}
	moveConv, _, _, err := convertMoves(in, typesConv.NameToID)
	if err != nil {
		return Output{}, Reconciliation{}, err
	}

	moveDiffs := computeMoveValueDiffs(in)
	speciesDiffs := computeSpeciesValueDiffs(in)

	verdictChecks, verdictWarnings, verdictBlockers := computeVerdictChecks(rc, in, moveConv.Included)

	partial := convertBlocked || len(verdictBlockers) > 0

	summary := computeSummary(in, out, partial)
	summary.TypeChart = TypeChartSummary{Types: len(typesConv.Rows), Rows: len(typesConv.ChartRows)}

	warnings := append(append([]Finding{}, convReport.Warnings...), verdictWarnings...)
	blockers := append(append([]Finding{}, convReport.Blockers...), verdictBlockers...)

	var coverage EffectCoverage
	var names map[string]NameStats
	var showdownOnly []ShowdownOnlySpecies
	var folded []FoldedLearnsetDiff
	var learnsetStats LearnsetStats

	if !partial {
		var covWarnings []Finding
		coverage, covWarnings = computeEffectCoverage(rc, in, out)
		warnings = append(warnings, covWarnings...)

		names = computeNames(in, out)

		showdownOnly = computeShowdownOnlySpecies(in, out, convReport)

		// Convert が通っている(partial=false)ので、レギュレーションの食い違いは無い(Convert が
		// 先に ErrInvalidData で止める)。
		rules, _ := regulationLearnsetRules(in.Regulations)

		var foldWarnings []Finding
		folded, foldWarnings = computeFoldedLearnsets(in, out, convReport, moveConv.Included, rules)
		warnings = append(warnings, foldWarnings...)

		learnsetStats = computeLearnsetStats(in, out, rules)
	}

	sortFindings(warnings)
	sortFindings(blockers)
	addFindingCounts(&summary, warnings, blockers)

	rec := Reconciliation{
		SchemaVersion:       reconciliationSchemaVersion,
		Sources:             copyStringMap(in.Config.Sources),
		Partial:             partial,
		Summary:             summary,
		MoveDiffs:           moveDiffs,
		SpeciesDiffs:        speciesDiffs,
		VerdictChecks:       verdictChecks,
		EffectCoverage:      coverage,
		Names:               names,
		ShowdownOnlySpecies: showdownOnly,
		FoldedLearnsets:     folded,
		Learnsets:           learnsetStats,
		Report:              Report{Warnings: warnings, Blockers: blockers},
	}

	if partial {
		return Output{}, rec, fmt.Errorf("%w", ErrBlocked)
	}
	return out, rec, nil
}

func copyStringMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
