package importer

// P2-1c の裁定の反映の確認(ADR-0103 §5)。

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// idsHash は ID を昇順に並べ "\n" で連結した UTF-8 バイト列の sha256(16進小文字)。
func idsHash(ids []string) string {
	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, "\n")))
	return hex.EncodeToString(sum[:])
}

// computeVerdictChecks は calcOnlyExcluded / showdownOnlyIncluded / statusTypeMismatch の
// 実データ側の ID 集合を計算し、config.json の裁定と照合する。included は取り込む技の ID 集合
// (Convert の技の規則の結果。種族が止まっていても計算できる)。
func computeVerdictChecks(rc *ReconcileConfig, in Input, included map[string]bool) (checks []VerdictCheck, warnings, blockers []Finding) {
	calcByID := map[string]CalcMove{}
	for _, m := range in.Calc.Moves {
		if m.Name == sentinelMoveName {
			continue
		}
		calcByID[toID(m.Name)] = m
	}
	sdByID := map[string]ShowdownMove{}
	for _, m := range in.Showdown.Moves {
		sdByID[m.ID] = m
	}

	calcOnlyExcluded := map[string]bool{}
	for id := range calcByID {
		if !included[id] {
			calcOnlyExcluded[id] = true
		}
	}
	showdownOnlyIncluded := map[string]bool{}
	for id := range included {
		if _, ok := calcByID[id]; !ok {
			showdownOnlyIncluded[id] = true
		}
	}
	statusTypeMismatch := map[string]bool{}
	for id, cm := range calcByID {
		if isFragment(cm) || !included[id] {
			continue
		}
		if resolveCalcCategory(cm.Category) != "status" {
			continue
		}
		sm, ok := sdByID[id]
		if !ok {
			continue
		}
		if !strings.EqualFold(cm.Type, sm.Type) {
			statusTypeMismatch[id] = true
		}
	}

	order := []struct {
		key     string
		actual  map[string]bool
		expect  VerdictCount
		blocker bool
	}{
		{"calcOnlyExcluded", calcOnlyExcluded, rc.Verdicts.Moves.CalcOnlyExcluded, true},
		{"showdownOnlyIncluded", showdownOnlyIncluded, rc.Verdicts.Moves.ShowdownOnlyIncluded, true},
		{"statusTypeMismatch", statusTypeMismatch, rc.Verdicts.Moves.StatusTypeMismatch, false},
	}

	for _, o := range order {
		ids := sortedKeysRaw(o.actual)
		hash := idsHash(ids)
		ok := len(ids) == o.expect.Count && hash == o.expect.IDsSHA256
		checks = append(checks, VerdictCheck{
			Key: o.key, ExpectedCount: o.expect.Count, ActualCount: len(ids),
			ExpectedSHA256: o.expect.IDsSHA256, ActualSHA256: hash, IDs: ids, OK: ok,
		})
		if !ok {
			f := Finding{Kind: KindVerdictMismatch, ID: o.key}
			if o.blocker {
				blockers = append(blockers, f)
			} else {
				warnings = append(warnings, f)
			}
		}
	}

	for _, key := range sortedKeysRaw(rc.Verdicts.Basis) {
		if rc.Verdicts.Basis[key] != in.Config.Sources[key] {
			warnings = append(warnings, Finding{Kind: KindVerdictBasisChanged, ID: key})
		}
	}

	return checks, warnings, blockers
}
