package importer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// FormatSummary は Reconciliation の決定的な人が読める要約を返す(ADR-0103 §2)。
func FormatSummary(r Reconciliation) string {
	var b strings.Builder
	fmt.Fprintf(&b, "schemaVersion: %d\npartial: %t\n", r.SchemaVersion, r.Partial)
	for _, row := range []struct {
		name string
		set  SetSummary
	}{
		{"moves", r.Summary.Moves}, {"species", r.Summary.Species},
		{"items", r.Summary.Items}, {"abilities", r.Summary.Abilities},
	} {
		fmt.Fprintf(&b, "%s: calc=%d showdown=%d standard=%d both=%d calcOnly=%d showdownOnly=%d imported=%d\n",
			row.name, row.set.Calc, row.set.Showdown, row.set.ShowdownStandard, row.set.Both,
			row.set.CalcOnly, row.set.ShowdownOnly, row.set.Imported)
	}
	fmt.Fprintf(&b, "typeChart: types=%d rows=%d\n", r.Summary.TypeChart.Types, r.Summary.TypeChart.Rows)
	fmt.Fprintf(&b, "moveMechanisms: attack=%d withMechanism=%d\n", r.Summary.MoveMechanisms.Attack, r.Summary.MoveMechanisms.WithMechanism)
	mechanisms := make([]string, 0, len(r.Summary.MoveMechanisms.ByMechanism))
	for m := range r.Summary.MoveMechanisms.ByMechanism {
		mechanisms = append(mechanisms, m)
	}
	sort.Strings(mechanisms)
	for _, m := range mechanisms {
		fmt.Fprintf(&b, "moveMechanism %s: %d\n", m, r.Summary.MoveMechanisms.ByMechanism[m])
	}
	writeFindingCounts(&b, "warnings", r.Summary.WarningCounts)
	writeFindingCounts(&b, "blockers", r.Summary.BlockerCounts)
	for _, check := range r.VerdictChecks {
		fmt.Fprintf(&b, "verdict %s: ok=%t expected=%d actual=%d\n", check.Key, check.OK, check.ExpectedCount, check.ActualCount)
	}
	return b.String()
}

func writeFindingCounts(b *strings.Builder, label string, counts map[FindingKind]int) {
	keys := make([]string, 0, len(counts))
	for kind := range counts {
		keys = append(keys, string(kind))
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		fmt.Fprintf(b, "%s: none\n", label)
		return
	}
	for _, key := range keys {
		fmt.Fprintf(b, "%s %s: %d\n", label, key, counts[FindingKind(key)])
	}
}

// WriteReconciliation は機械可読の報告と、その最新版・要約を書く(ADR-0103 §2。Git 管理外)。
func WriteReconciliation(dir string, r Reconciliation, now time.Time) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	name := fmt.Sprintf("import-%s.json", now.UTC().Format("20060102T150405Z"))
	if err := os.WriteFile(filepath.Join(dir, name), raw, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "latest.json"), raw, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "latest-summary.txt"), []byte(FormatSummary(r)), 0o644)
}
