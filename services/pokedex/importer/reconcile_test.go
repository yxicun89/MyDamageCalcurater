package importer_test

// 照合と差分報告(Reconcile)のテスト(ADR-0103)。DB もネットワークも使わない。
// 入力は testdata/fictional の架空データ + テスト内での書き換えだけ(ADR-0100 §7 の架空データの規約)。
// fixture の JSON は変えない(P2-2b のテストの前提を保つ)。P2-2c で増えたフィールド
// (Config.Reconcile・hooks・prevo)はテスト内で設定する。

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"example.com/pokecalc/services/pokedex/importer"
)

// verdictHash は ADR-0103 §5 の idsSha256(ID を昇順に "\n" で連結した sha256 の16進小文字)。
func verdictHash(ids ...string) string {
	s := append([]string(nil), ids...)
	sort.Strings(s)
	sum := sha256.Sum256([]byte(strings.Join(s, "\n")))
	return hex.EncodeToString(sum[:])
}

// fixture の技で P2-1c の3区分に当たる ID(ADR-0103 §5 の定義を fixture に当てはめたもの)。
//   - calcOnlyExcluded: calc の技(番兵 (No Move) を除く)のうち取り込まないもの = testold(断片・Past)/ testbanned(Past)
//   - showdownOnlyIncluded: 取り込む技のうち calc に ID が無いもの = testsplash
//     (testrevived は calc に断片として ID があるので含まない)
//   - statusTypeMismatch: 両方にあり取り込む変化技でタイプが違うもの = testglare(calc Normal / Showdown Grass)
var (
	fixtureCalcOnlyExcluded     = []string{"testbanned", "testold"}
	fixtureShowdownOnlyIncluded = []string{"testsplash"}
	fixtureStatusTypeMismatch   = []string{"testglare"}
)

var testEffectHooks = []string{"onBasePower", "onModifyAtk", "onModifySpA", "onModifyDamage", "onSourceModifyDamage"}

// reconcileInput は fixture に照合の設定(裁定の件数・ハッシュが fixture と一致するもの)と
// 効果定義と矛盾しない hooks を足した入力を返す。
func reconcileInput(t *testing.T) importer.Input {
	t.Helper()
	in := loadFixture(t)
	in.Config.Reconcile = &importer.ReconcileConfig{
		EffectHooks: append([]string(nil), testEffectHooks...),
		Verdicts: importer.Verdicts{
			Basis: map[string]string{"calc": "test-calc-1", "showdown": fixtureShowdownVersion},
			Moves: importer.MoveVerdicts{
				CalcOnlyExcluded:     importer.VerdictCount{Count: 2, IDsSHA256: verdictHash(fixtureCalcOnlyExcluded...)},
				ShowdownOnlyIncluded: importer.VerdictCount{Count: 1, IDsSHA256: verdictHash(fixtureShowdownOnlyIncluded...)},
				StatusTypeMismatch:   importer.VerdictCount{Count: 1, IDsSHA256: verdictHash(fixtureStatusTypeMismatch...)},
			},
		},
	}
	// 効果定義(testorb / testberry / testguard)にはダメージに効くハンドラを持たせる(基準状態で
	// effect-missing / effect-no-hook が出ないように)。
	sdItem(t, &in, "testorb").Hooks = []string{"onModifyDamage"}
	sdItem(t, &in, "testberry").Hooks = []string{"onEat", "onSourceModifyDamage"}
	sdAbility(t, &in, "testguard").Hooks = []string{"onSourceModifyDamage"}
	return in
}

func sdItem(t *testing.T, in *importer.Input, id string) *importer.ShowdownItem {
	t.Helper()
	for i := range in.Showdown.Items {
		if in.Showdown.Items[i].ID == id {
			return &in.Showdown.Items[i]
		}
	}
	t.Fatalf("Showdown の持ち物 %q が fixture に無い", id)
	return nil
}

func sdAbility(t *testing.T, in *importer.Input, id string) *importer.ShowdownAbility {
	t.Helper()
	for i := range in.Showdown.Abilities {
		if in.Showdown.Abilities[i].ID == id {
			return &in.Showdown.Abilities[i]
		}
	}
	t.Fatalf("Showdown の特性 %q が fixture に無い", id)
	return nil
}

func reconcileOK(t *testing.T, in importer.Input) (importer.Output, importer.Reconciliation) {
	t.Helper()
	out, rec, err := importer.Reconcile(in)
	if err != nil {
		t.Fatalf("Reconcile: %v\nreport: %+v", err, rec.Report)
	}
	if len(rec.Report.Blockers) != 0 {
		t.Fatalf("エラーなしなのに Blockers がある: %+v", rec.Report.Blockers)
	}
	if rec.Partial {
		t.Fatal("エラーなしなのに Partial = true")
	}
	return out, rec
}

func hasKind(findings []importer.Finding, kind importer.FindingKind) bool {
	for _, f := range findings {
		if f.Kind == kind {
			return true
		}
	}
	return false
}

// sameStrings は nil と空を同じとみなして比べる。
func sameStrings(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

func sameCounts(a, b map[string]int) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

// --- 入口・エラー(ADR-0103 §1・§9) -----------------------------------------------

func TestReconcileRequiresReconcileConfig(t *testing.T) {
	in := loadFixture(t) // Config.Reconcile が無い(P2-2b までの config)
	_, _, err := importer.Reconcile(in)
	if !errors.Is(err, importer.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput(実データの取り込みでは照合の設定を必須にする)", err)
	}
}

func TestReconcilePassesThroughConvertErrors(t *testing.T) {
	in := reconcileInput(t)
	in.Config.ExcludeCalcSpecies = []string{"Testnothing"} // calc に無い除外名 = ErrInvalidData(ADR-0101 §5)
	out, rec, err := importer.Reconcile(in)
	if !errors.Is(err, importer.ErrInvalidData) {
		t.Fatalf("err = %v, want ErrInvalidData", err)
	}
	if !reflect.DeepEqual(out, importer.Output{}) || !reflect.DeepEqual(rec, importer.Reconciliation{}) {
		t.Error("Convert の入力エラーでは Output も Reconciliation もゼロ値で返す")
	}
}

// --- calc と Showdown の値の全項目比較(ADR-0103 §3) --------------------------------

func TestReconcileValueDiffs(t *testing.T) {
	glareType := importer.ValueDiff{Entity: "move", ID: "testglare", Field: "type", Calc: "normal", Showdown: "grass", Severity: "warning", Imported: true}

	tests := []struct {
		name        string
		mutate      func(t *testing.T, in *importer.Input)
		wantMoves   []importer.ValueDiff
		wantSpecies []importer.ValueDiff
		wantBlocked bool
	}{
		{
			name:      "基準: 変化技のタイプの食い違い1件だけ(警告)",
			mutate:    func(t *testing.T, in *importer.Input) {},
			wantMoves: []importer.ValueDiff{glareType},
		},
		{
			name:      "優先度の違いは warning",
			mutate:    func(t *testing.T, in *importer.Input) { calcMove(t, in, "Test Strike").Priority = 0 },
			wantMoves: []importer.ValueDiff{glareType, {Entity: "move", ID: "teststrike", Field: "priority", Calc: "0", Showdown: "1", Severity: "warning", Imported: true}},
		},
		{
			name:        "取り込む攻撃技の威力の違いは blocker",
			mutate:      func(t *testing.T, in *importer.Input) { calcMove(t, in, "Test Flame").BasePower = 95 },
			wantMoves:   []importer.ValueDiff{{Entity: "move", ID: "testflame", Field: "basePower", Calc: "95", Showdown: "90", Severity: "blocker", Imported: true}, glareType},
			wantBlocked: true,
		},
		{
			name:        "取り込む技の分類の違いは blocker(比較は小文字)",
			mutate:      func(t *testing.T, in *importer.Input) { calcMove(t, in, "Test Flame").Category = "Physical" },
			wantMoves:   []importer.ValueDiff{{Entity: "move", ID: "testflame", Field: "category", Calc: "physical", Showdown: "special", Severity: "blocker", Imported: true}, glareType},
			wantBlocked: true,
		},
		{
			name:        "取り込む攻撃技のタイプの違いは blocker(規則3)",
			mutate:      func(t *testing.T, in *importer.Input) { calcMove(t, in, "Test Strike").Type = "Fire" },
			wantMoves:   []importer.ValueDiff{glareType, {Entity: "move", ID: "teststrike", Field: "type", Calc: "fire", Showdown: "normal", Severity: "blocker", Imported: true}},
			wantBlocked: true,
		},
		{
			name:      "取り込まない技(Showdown で Past)の差分も info として並べる",
			mutate:    func(t *testing.T, in *importer.Input) { calcMove(t, in, "Test Banned").BasePower = 85 },
			wantMoves: []importer.ValueDiff{{Entity: "move", ID: "testbanned", Field: "basePower", Calc: "85", Showdown: "80", Severity: "info", Imported: false}, glareType},
		},
		{
			name:        "種族値は項目ごと(blocker)",
			mutate:      func(t *testing.T, in *importer.Input) { calcSpecies(t, in, "Testleaf").BaseStats.Spe = 61 },
			wantMoves:   []importer.ValueDiff{glareType},
			wantSpecies: []importer.ValueDiff{{Entity: "species", ID: "testleaf", Field: "spe", Calc: "61", Showdown: "60", Severity: "blocker", Imported: true}},
			wantBlocked: true,
		},
		{
			name: "種族のタイプはタイプ ID を / で連結して比べる(blocker)",
			mutate: func(t *testing.T, in *importer.Input) {
				calcSpecies(t, in, "Testmon").Types = []string{"Fire", "Water"}
			},
			wantMoves:   []importer.ValueDiff{glareType},
			wantSpecies: []importer.ValueDiff{{Entity: "species", ID: "testmon", Field: "types", Calc: "fire/water", Showdown: "fire", Severity: "blocker", Imported: true}},
			wantBlocked: true,
		},
		{
			name:        "取り込まない種族(HP 種族値 1 で除外)の差分は info",
			mutate:      func(t *testing.T, in *importer.Input) { calcSpecies(t, in, "Testbug").BaseStats.Atk = 91 },
			wantMoves:   []importer.ValueDiff{glareType},
			wantSpecies: []importer.ValueDiff{{Entity: "species", ID: "testbug", Field: "atk", Calc: "91", Showdown: "90", Severity: "info", Imported: false}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := reconcileInput(t)
			tt.mutate(t, &in)
			out, rec, err := importer.Reconcile(in)
			if tt.wantBlocked {
				if !errors.Is(err, importer.ErrBlocked) {
					t.Fatalf("err = %v, want ErrBlocked", err)
				}
				if !rec.Partial {
					t.Error("Convert が止まったときは Partial = true")
				}
				if len(rec.Report.Blockers) == 0 {
					t.Error("止めたのに Report.Blockers が空")
				}
				if !reflect.DeepEqual(out, importer.Output{}) {
					t.Error("止めたときは Output をゼロ値で返す(部分的な結果を投入に回さない)")
				}
			} else if err != nil {
				t.Fatalf("Reconcile: %v", err)
			}
			if !reflect.DeepEqual(rec.MoveDiffs, tt.wantMoves) {
				t.Errorf("MoveDiffs =\n  %+v\nwant\n  %+v", rec.MoveDiffs, tt.wantMoves)
			}
			if len(rec.SpeciesDiffs)+len(tt.wantSpecies) > 0 && !reflect.DeepEqual(rec.SpeciesDiffs, tt.wantSpecies) {
				t.Errorf("SpeciesDiffs =\n  %+v\nwant\n  %+v", rec.SpeciesDiffs, tt.wantSpecies)
			}
		})
	}
}

// --- 件数の要約(ADR-0103 §4) -------------------------------------------------------

func TestReconcileSummary(t *testing.T) {
	_, rec := reconcileOK(t, reconcileInput(t))
	s := rec.Summary

	want := map[string]importer.SetSummary{
		// calc 6(番兵除く。断片 testold / testrevived を含む)、Showdown 7 / null 5、
		// 両方 = testflame, testglare, testrevived, teststrike、calc だけ = testbanned, testold、
		// Showdown だけ = testsplash、取り込む 5。
		"moves": {Calc: 6, Showdown: 7, ShowdownStandard: 5, Both: 4, CalcOnly: 2, ShowdownOnly: 1, Imported: 5},
		// calc 7、Showdown 9 / null 8(testpast が Past)、対応あり 6(Testshield-Both だけ対応なし)、
		// Showdown だけ = testleafbloom, testshieldblade、取り込む 5。
		"species":   {Calc: 7, Showdown: 9, ShowdownStandard: 8, Both: 6, CalcOnly: 1, ShowdownOnly: 2, Imported: 5},
		"items":     {Calc: 3, Showdown: 4, ShowdownStandard: 3, Both: 3, CalcOnly: 0, ShowdownOnly: 0, Imported: 3},
		"abilities": {Calc: 4, Showdown: 4, ShowdownStandard: 4, Both: 4, CalcOnly: 0, ShowdownOnly: 0, Imported: 4},
	}
	got := map[string]importer.SetSummary{"moves": s.Moves, "species": s.Species, "items": s.Items, "abilities": s.Abilities}
	for k := range want {
		if got[k] != want[k] {
			t.Errorf("Summary.%s = %+v, want %+v", k, got[k], want[k])
		}
	}

	wantWarnings := map[importer.FindingKind]int{
		importer.KindMoveExcluded:        3, // nomove(番兵)/ testbanned / testold
		importer.KindMoveShowdownOnly:    2, // testrevived / testsplash
		importer.KindMoveTypeMismatch:    1,
		importer.KindFormFolded:          1,
		importer.KindSpeciesShowdownOnly: 1,
		importer.KindSpeciesExcluded:     2,
		// +1 ずつは性格(natures)の fixture 分(ADR-0105 §4・§7): testneutral の英語名フォールバックと
		// どの性格にも当たらない override(testunknownnature)。TestConvertNaturesNameFindings と同じ。
		importer.KindNameFallback:   4,
		importer.KindOverrideUnused: 2,
		importer.KindEffectUnused:   1,
	}
	for kind, n := range wantWarnings {
		if s.WarningCounts[kind] != n {
			t.Errorf("WarningCounts[%s] = %d, want %d", kind, s.WarningCounts[kind], n)
		}
	}
	total := 0
	for _, n := range s.WarningCounts {
		total += n
	}
	if total != len(rec.Report.Warnings) {
		t.Errorf("WarningCounts の合計 %d が Report.Warnings の件数 %d と合わない", total, len(rec.Report.Warnings))
	}
	if len(s.BlockerCounts) != 0 {
		t.Errorf("BlockerCounts = %v, want 空", s.BlockerCounts)
	}
}

func TestReconcileSpeciesSummaryCountsNonstandardMatchAsBoth(t *testing.T) {
	in := reconcileInput(t)
	in.Calc.Species = append(in.Calc.Species, importer.CalcSpecies{
		Name: "Testlegacy", Types: []string{"Normal"},
		BaseStats: importer.BaseStats{HP: 50, Atk: 50, Def: 50, SpA: 50, SpD: 50, Spe: 50},
	})
	in.Showdown.Species = append(in.Showdown.Species, importer.ShowdownSpecies{
		ID: "testlegacy", Name: "Testlegacy", Num: 9011, BaseSpecies: "Testlegacy",
		Types: []string{"Normal"}, BaseStats: importer.BaseStats{HP: 50, Atk: 50, Def: 50, SpA: 50, SpD: 50, Spe: 50},
		Abilities: map[string]string{"0": "Test Guard"}, FormeOrder: []string{"Testlegacy"}, IsNonstandard: pastPtr(),
	})

	_, rec := reconcileOK(t, in)
	got := rec.Summary.Species
	if got.Calc != 8 || got.Showdown != 10 || got.ShowdownStandard != 8 || got.Both != 7 || got.CalcOnly != 1 || got.ShowdownOnly != 2 || got.Imported != 5 {
		t.Errorf("Summary.Species = %+v; Showdown に対応する非 standard の calc 種族も Both に数える", got)
	}
}

// --- P2-1c の裁定の反映の確認(ADR-0103 §5) ------------------------------------------

func TestReconcileVerdictChecksMatchFixture(t *testing.T) {
	_, rec := reconcileOK(t, reconcileInput(t))
	want := []importer.VerdictCheck{
		{Key: "calcOnlyExcluded", ExpectedCount: 2, ActualCount: 2, ExpectedSHA256: verdictHash(fixtureCalcOnlyExcluded...), ActualSHA256: verdictHash(fixtureCalcOnlyExcluded...), IDs: fixtureCalcOnlyExcluded, OK: true},
		{Key: "showdownOnlyIncluded", ExpectedCount: 1, ActualCount: 1, ExpectedSHA256: verdictHash(fixtureShowdownOnlyIncluded...), ActualSHA256: verdictHash(fixtureShowdownOnlyIncluded...), IDs: fixtureShowdownOnlyIncluded, OK: true},
		{Key: "statusTypeMismatch", ExpectedCount: 1, ActualCount: 1, ExpectedSHA256: verdictHash(fixtureStatusTypeMismatch...), ActualSHA256: verdictHash(fixtureStatusTypeMismatch...), IDs: fixtureStatusTypeMismatch, OK: true},
	}
	if !reflect.DeepEqual(rec.VerdictChecks, want) {
		t.Errorf("VerdictChecks =\n  %+v\nwant\n  %+v", rec.VerdictChecks, want)
	}
	if hasKind(rec.Report.Warnings, importer.KindVerdictMismatch) || hasKind(rec.Report.Warnings, importer.KindVerdictBasisChanged) {
		t.Errorf("一致しているのに裁定の指摘がある: %+v", rec.Report.Warnings)
	}
}

func TestReconcileVerdictMismatch(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(t *testing.T, in *importer.Input)
		wantKind    importer.FindingKind
		wantID      string
		wantBlocked bool
	}{
		{
			name:        "calc だけの技の件数が違う → 止める",
			mutate:      func(t *testing.T, in *importer.Input) { in.Config.Reconcile.Verdicts.Moves.CalcOnlyExcluded.Count = 3 },
			wantKind:    importer.KindVerdictMismatch,
			wantID:      "calcOnlyExcluded",
			wantBlocked: true,
		},
		{
			name: "Showdown だけの技の件数は同じで集合が違う(ハッシュ) → 止める",
			mutate: func(t *testing.T, in *importer.Input) {
				in.Config.Reconcile.Verdicts.Moves.ShowdownOnlyIncluded.IDsSHA256 = verdictHash("testother")
			},
			wantKind:    importer.KindVerdictMismatch,
			wantID:      "showdownOnlyIncluded",
			wantBlocked: true,
		},
		{
			name: "上流のデータが変わって Showdown だけの技が消えた → 止める",
			mutate: func(t *testing.T, in *importer.Input) {
				moves := in.Showdown.Moves[:0]
				for _, m := range in.Showdown.Moves {
					if m.ID != "testsplash" {
						moves = append(moves, m)
					}
				}
				in.Showdown.Moves = moves
			},
			wantKind:    importer.KindVerdictMismatch,
			wantID:      "showdownOnlyIncluded",
			wantBlocked: true,
		},
		{
			name: "変化技のタイプの食い違いの件数が違う → 警告だけ(ダメージに効かない)",
			mutate: func(t *testing.T, in *importer.Input) {
				in.Config.Reconcile.Verdicts.Moves.StatusTypeMismatch = importer.VerdictCount{Count: 0, IDsSHA256: verdictHash()}
			},
			wantKind: importer.KindVerdictMismatch,
			wantID:   "statusTypeMismatch",
		},
		{
			name: "裁定を行った版と取り込む版が違う → 警告(件数が合えば止めない)",
			mutate: func(t *testing.T, in *importer.Input) {
				in.Config.Reconcile.Verdicts.Basis["showdown"] = "0123456789abcdef0123456789abcdef01234567"
			},
			wantKind: importer.KindVerdictBasisChanged,
			wantID:   "showdown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := reconcileInput(t)
			tt.mutate(t, &in)
			out, rec, err := importer.Reconcile(in)
			if tt.wantBlocked {
				if !errors.Is(err, importer.ErrBlocked) {
					t.Fatalf("err = %v, want ErrBlocked", err)
				}
				if !hasFinding(rec.Report.Blockers, tt.wantKind, tt.wantID) {
					t.Errorf("Blockers に %s/%s が無い: %+v", tt.wantKind, tt.wantID, rec.Report.Blockers)
				}
				if !reflect.DeepEqual(out, importer.Output{}) {
					t.Error("裁定の照合で止めたときも Output はゼロ値")
				}
				if len(rec.VerdictChecks) != 3 {
					t.Errorf("止めたときも VerdictChecks を報告する: %+v", rec.VerdictChecks)
				}
				return
			}
			if err != nil {
				t.Fatalf("Reconcile: %v", err)
			}
			if !hasFinding(rec.Report.Warnings, tt.wantKind, tt.wantID) {
				t.Errorf("Warnings に %s/%s が無い: %+v", tt.wantKind, tt.wantID, rec.Report.Warnings)
			}
			if len(out.Moves) == 0 {
				t.Error("警告だけのときは Output を返す")
			}
		})
	}
}

func TestReconcileChecksVerdictsEvenWhenConvertBlocks(t *testing.T) {
	in := reconcileInput(t)
	calcSpecies(t, &in, "Testleaf").BaseStats.Spe = 61 // 種族値の食い違い = Convert の ErrBlocked
	_, rec, err := importer.Reconcile(in)
	if !errors.Is(err, importer.ErrBlocked) {
		t.Fatalf("err = %v, want ErrBlocked", err)
	}
	if len(rec.VerdictChecks) != 3 {
		t.Fatalf("VerdictChecks = %+v, want 3件(技の判定は種族と独立にできる)", rec.VerdictChecks)
	}
	for _, c := range rec.VerdictChecks {
		if !c.OK {
			t.Errorf("%s: OK = false, want true(技の集合は変わっていない)", c.Key)
		}
	}
	if rec.Summary.Moves.Imported != 0 || rec.Summary.Moves.Calc != 6 {
		t.Errorf("Partial の Summary.Moves = %+v, want Calc 6・Imported 0", rec.Summary.Moves)
	}
}

// --- 効果定義の網羅性(ADR-0103 §6) --------------------------------------------------

func TestReconcileEffectCoverage(t *testing.T) {
	in := reconcileInput(t)
	sdItem(t, &in, "testmonite").Hooks = []string{"onBasePower", "onTakeItem"} // ダメージに効くのに定義が無い
	sdAbility(t, &in, "testblaze").Hooks = []string{"onModifyAtk", "onModifySpA"}
	sdAbility(t, &in, "testguard").Hooks = []string{"onStart"}       // 定義はあるがダメージのハンドラが無い
	sdAbility(t, &in, "teststance").Hooks = []string{"onModifyMove"} // effectHooks に無い = 対象外

	_, rec := reconcileOK(t, in)

	type stats struct {
		Imported, WithDamageHooks, Defined, Missing, NoHook int
		MissingIDs, NoHookIDs                               []string
	}
	check := func(label string, got importer.CoverageStats, want stats) {
		t.Helper()
		if got.Imported != want.Imported || got.WithDamageHooks != want.WithDamageHooks || got.Defined != want.Defined ||
			got.Missing != want.Missing || got.NoHook != want.NoHook ||
			!sameStrings(got.MissingIDs, want.MissingIDs) || !sameStrings(got.NoHookIDs, want.NoHookIDs) {
			t.Errorf("EffectCoverage.%s = %+v, want %+v", label, got, want)
		}
	}
	// 持ち物: 取り込む 3(testberry / testmonite / testorb)。testunused は取り込まないので数えない。
	check("Items", rec.EffectCoverage.Items, stats{Imported: 3, WithDamageHooks: 3, Defined: 2, Missing: 1, NoHook: 0, MissingIDs: []string{"testmonite"}})
	// 特性: 取り込む 4(testblaze / testguard / testleaf / teststance)。
	check("Abilities", rec.EffectCoverage.Abilities, stats{Imported: 4, WithDamageHooks: 1, Defined: 1, Missing: 1, NoHook: 1, MissingIDs: []string{"testblaze"}, NoHookIDs: []string{"testguard"}})

	for _, w := range []struct {
		kind importer.FindingKind
		id   string
	}{
		{importer.KindEffectMissing, "testmonite"},
		{importer.KindEffectMissing, "testblaze"},
		{importer.KindEffectNoHook, "testguard"},
	} {
		if !hasFinding(rec.Report.Warnings, w.kind, w.id) {
			t.Errorf("Warnings に %s/%s が無い", w.kind, w.id)
		}
	}
	if hasFinding(rec.Report.Warnings, importer.KindEffectMissing, "teststance") {
		t.Error("effectHooks に無いハンドラだけの特性を effect-missing にした")
	}
	if len(rec.Report.Blockers) != 0 {
		t.Error("効果定義の網羅性では止めない")
	}
}

// --- 日本語名(PokeAPI との照合。ADR-0103 §8) ------------------------------------------

func TestReconcileNameStats(t *testing.T) {
	_, rec := reconcileOK(t, reconcileInput(t))
	// fixture の config は nameJaLanguages = ["ja-Hrkt", "ja"]。
	want := map[string]importer.NameStats{
		// testmon(ja-Hrkt)/ testmonmega(forms, ja-Hrkt)/ testleaf(ja だけ)/ testleafrain(override)/ testshield(欠落)
		"species": {Override: 1, PokeAPI: 3, FallbackEn: 1, ByLanguage: map[string]int{"ja-Hrkt": 2, "ja": 1}, FallbackIDs: []string{"testshield"}},
		// testflame / testrevived(ja-Hrkt)/ testglare(ja)/ teststrike(override)/ testsplash(欠落)
		"moves":     {Override: 1, PokeAPI: 3, FallbackEn: 1, ByLanguage: map[string]int{"ja-Hrkt": 2, "ja": 1}, FallbackIDs: []string{"testsplash"}},
		"items":     {Override: 1, PokeAPI: 2, FallbackEn: 0, ByLanguage: map[string]int{"ja-Hrkt": 2}},
		"abilities": {Override: 0, PokeAPI: 4, FallbackEn: 0, ByLanguage: map[string]int{"ja-Hrkt": 4}},
		"types":     {Override: 0, PokeAPI: 3, FallbackEn: 1, ByLanguage: map[string]int{"ja-Hrkt": 3}, FallbackIDs: []string{"grass"}},
	}
	if len(rec.Names) != len(want) {
		t.Errorf("Names のキー = %v, want %v", keysOf(rec.Names), keysOf(want))
	}
	for k, w := range want {
		g := rec.Names[k]
		if g.Override != w.Override || g.PokeAPI != w.PokeAPI || g.FallbackEn != w.FallbackEn ||
			!sameCounts(g.ByLanguage, w.ByLanguage) || !sameStrings(g.FallbackIDs, w.FallbackIDs) {
			t.Errorf("Names[%s] = %+v, want %+v", k, g, w)
		}
	}
}

// --- Showdown だけの種族(ADR-0103 §8) ---------------------------------------------

func TestReconcileShowdownOnlySpecies(t *testing.T) {
	in := reconcileInput(t)
	in.Showdown.Species = append(in.Showdown.Species,
		// 同じ図鑑番号の種族を calc が持たない(Convert は候補にもしない)
		importer.ShowdownSpecies{ID: "testlone", Name: "Testlone", Num: 9010, BaseSpecies: "Testlone",
			Types: []string{"Water"}, BaseStats: importer.BaseStats{HP: 50, Atk: 50, Def: 50, SpA: 50, SpD: 50, Spe: 50},
			Abilities: map[string]string{"0": "Test Guard"}, FormeOrder: []string{"Testlone"}},
		// 基本種の formeOrder に無い(Convert は黙って飛ばしている)
		importer.ShowdownSpecies{ID: "testleafodd", Name: "Testleaf-Odd", Num: 9002, BaseSpecies: "Testleaf", Forme: "Odd",
			Types: []string{"Grass"}, BaseStats: importer.BaseStats{HP: 70, Atk: 60, Def: 80, SpA: 90, SpD: 80, Spe: 60},
			Abilities: map[string]string{"0": "Test Leaf"}},
	)
	_, rec := reconcileOK(t, in)
	want := []importer.ShowdownOnlySpecies{
		{ID: "testleafbloom", Num: 9002, Reason: "folded", Representative: "testleaf"},
		{ID: "testleafodd", Num: 9002, Reason: "unresolvable"},
		{ID: "testlone", Num: 9010, Reason: "num-not-in-calc"},
		{ID: "testshieldblade", Num: 9003, Reason: "different-performance"},
	}
	if !reflect.DeepEqual(rec.ShowdownOnlySpecies, want) {
		t.Errorf("ShowdownOnlySpecies =\n  %+v\nwant\n  %+v", rec.ShowdownOnlySpecies, want)
	}
}

// --- 畳んだフォームの習得技の差(ADR-0103 §8) ------------------------------------------

func TestReconcileFoldedFormLearnsetDiff(t *testing.T) {
	t.Run("畳んだフォームに自分の習得技が無ければ差は無い", func(t *testing.T) {
		_, rec := reconcileOK(t, reconcileInput(t))
		if len(rec.FoldedLearnsets) != 0 || hasKind(rec.Report.Warnings, importer.KindFormLearnsetDiff) {
			t.Errorf("差が無いのに報告した: %+v", rec.FoldedLearnsets)
		}
	})
	t.Run("差があれば取り込む技に絞って報告し、取り込みは代表のまま", func(t *testing.T) {
		in := reconcileInput(t)
		in.Showdown.Learnsets["testleafbloom"] = map[string]int{"testglare": 9, "testrevived": 9, "testold": 9} // testold は取り込まない技
		out, rec := reconcileOK(t, in)
		want := []importer.FoldedLearnsetDiff{{
			ID: "testleafbloom", Representative: "testleaf",
			OnlyInFolded: []string{"testrevived"}, OnlyInRepresentative: []string{"testsplash"},
		}}
		if !reflect.DeepEqual(rec.FoldedLearnsets, want) {
			t.Errorf("FoldedLearnsets = %+v, want %+v", rec.FoldedLearnsets, want)
		}
		if !hasFinding(rec.Report.Warnings, importer.KindFormLearnsetDiff, "testleafbloom") {
			t.Error("Warnings に form-learnset-diff/testleafbloom が無い")
		}
		for _, r := range out.Learnsets {
			if r.SpeciesKey == "9002-000" && r.MoveID == "testrevived" {
				t.Error("v1 は代表の習得技だけを取り込む(畳んだフォームの技を足さない)")
			}
		}
	})
}

// --- 報告の形(ADR-0103 §2) ---------------------------------------------------------

// 報告は ID・件数・値だけで、英語名・日本語名を含まない(ADR-0002: 第三者データの名前の一覧を残さない)。
func TestReconcileReportHasNoNames(t *testing.T) {
	in := reconcileInput(t)
	in.Showdown.Learnsets["testleafbloom"] = map[string]int{"testrevived": 9} // 報告の節をなるべく埋める
	_, rec := reconcileOK(t, in)
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	summary := importer.FormatSummary(rec)
	if !strings.Contains(string(raw), "testglare") {
		t.Fatal("報告に ID が無い(報告が空になっていないか)")
	}
	// fixture の英語名(大文字始まり・空白入り)と日本語名(「テスト」で始まる)。
	forbidden := []string{"Test ", "Testmon", "Testleaf", "Testshield", "Testbug", "Testpast", "テスト"}
	for _, s := range forbidden {
		if strings.Contains(string(raw), s) {
			t.Errorf("報告の JSON に名前 %q が含まれる", s)
		}
		if strings.Contains(summary, s) {
			t.Errorf("要約に名前 %q が含まれる", s)
		}
	}
}

// 相性表の件数(取り込むタイプ数・行数)が報告と要約に出る(#269: 相性表が黙って空になって
// いないかを人が見られるようにする)。
func TestReconcileSummaryCountsTypeChart(t *testing.T) {
	out, rec := reconcileOK(t, reconcileInput(t))
	want := importer.TypeChartSummary{Types: 4, Rows: 9}
	if rec.Summary.TypeChart != want {
		t.Errorf("Summary.TypeChart = %+v, want %+v", rec.Summary.TypeChart, want)
	}
	if len(out.TypeChart) != want.Rows {
		t.Errorf("Output の相性表 %d 行と要約の行数 %d が食い違う", len(out.TypeChart), want.Rows)
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"typeChart":{"types":4,"rows":9}`) {
		t.Errorf("報告の JSON に相性表の件数が無い: %s", raw)
	}
	if s := importer.FormatSummary(rec); !strings.Contains(s, "typeChart: types=4 rows=9\n") {
		t.Errorf("要約に相性表の件数が無い:\n%s", s)
	}
}

func TestFormatSummaryMentionsVerdicts(t *testing.T) {
	_, rec := reconcileOK(t, reconcileInput(t))
	s := importer.FormatSummary(rec)
	for _, key := range []string{"calcOnlyExcluded", "showdownOnlyIncluded", "statusTypeMismatch"} {
		if !strings.Contains(s, key) {
			t.Errorf("要約に裁定の照合 %s が無い:\n%s", key, s)
		}
	}
	if s != importer.FormatSummary(rec) {
		t.Error("要約が決定的でない")
	}
}

func TestWriteReconciliation(t *testing.T) {
	_, rec := reconcileOK(t, reconcileInput(t))
	dir := t.TempDir()
	now := time.Date(2026, 9, 22, 1, 2, 3, 0, time.UTC)
	if err := importer.WriteReconciliation(dir, rec, now); err != nil {
		t.Fatalf("WriteReconciliation: %v", err)
	}
	stamped, err := os.ReadFile(filepath.Join(dir, "import-20260922T010203Z.json"))
	if err != nil {
		t.Fatal(err)
	}
	latest, err := os.ReadFile(filepath.Join(dir, "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(stamped) != string(latest) {
		t.Error("latest.json は時刻付きの報告と同じバイト列")
	}
	var decoded map[string]any
	if err := json.Unmarshal(latest, &decoded); err != nil {
		t.Fatalf("latest.json が JSON でない: %v", err)
	}
	if decoded["schemaVersion"] != float64(1) {
		t.Errorf("schemaVersion = %v, want 1", decoded["schemaVersion"])
	}
	summary, err := os.ReadFile(filepath.Join(dir, "latest-summary.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(summary) != importer.FormatSummary(rec) {
		t.Error("latest-summary.txt は FormatSummary の出力")
	}
}

func TestReconcileIsDeterministic(t *testing.T) {
	out1, rec1 := reconcileOK(t, reconcileInput(t))
	for i := 0; i < 5; i++ {
		out2, rec2 := reconcileOK(t, reconcileInput(t))
		if !reflect.DeepEqual(out1, out2) || !reflect.DeepEqual(rec1, rec2) {
			t.Fatal("Reconcile の結果が実行ごとに変わる(map の反復順に依存していないか)")
		}
	}
}
