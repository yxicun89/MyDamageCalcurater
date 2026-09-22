package importer_test

// 習得技の解決のテスト(ADR-0101 §5・ADR-0103 §7)。
//
// 既定(ADR-0101 §5 の元の規則): 自分の学習元、無ければ基本種(フォーム・メガの base species)の
// 学習元を1段だけ。進化前(prevo)はたどらない。fixture の既定レギュレーション(inheritFromPrevo:
// false)はこの既定に当たる(M-C と同じ)。
//
// 進化前をたどる継承(ADR-0103 §7 の案b)は、レギュレーションの `inheritFromPrevo` が true の
// ときだけ使う実装として残す(将来のレギュレーションのため)。**M-C(champions mod)では使わない**:
// Showdown 本体のソース(dex-species.js の learnsetParent)を読んだところ、champions mod は
// 進化前をたどる継承を世代を問わず常に無効化していると判明した。§10 の実データ確認では、
// 進化前からの継承を仮定した resolved(s) を PS の TeamValidator.checkCanLearn と標本突き合わせ
// したところ、世代でも絞った案bの実装でもなお標本5件が全件「学習不可」(Go 側は「学習可」)と
// 食い違い、原因はこの mod 固有の無効化だった(世代の問題ではなかった)。
//
// fixture には進化系統が無いので、Showdown だけにある架空の進化前(isNonstandard "Past")を
// テスト内で足す。

import (
	"errors"
	"reflect"
	"sort"
	"testing"

	"example.com/pokecalc/services/pokedex/importer"
)

func pastPtr() *string {
	s := "Past"
	return &s
}

// addPrevoChain は Testleaf ← Testsprout ← Testseed の進化系統を足す(進化前は M-C で使えない = Past)。
//   - testsprout の習得技: teststrike(取り込む)/ testold(取り込まない)
//   - testseed の習得技: testflame
func addPrevoChain(t *testing.T, in *importer.Input) {
	t.Helper()
	in.Showdown.Species = append(in.Showdown.Species,
		importer.ShowdownSpecies{ID: "testsprout", Name: "Testsprout", Num: 9007, BaseSpecies: "Testsprout",
			Types: []string{"Grass"}, BaseStats: importer.BaseStats{HP: 40, Atk: 40, Def: 40, SpA: 40, SpD: 40, Spe: 40},
			Abilities: map[string]string{"0": "Test Leaf"}, Prevo: "Testseed", IsNonstandard: pastPtr()},
		importer.ShowdownSpecies{ID: "testseed", Name: "Testseed", Num: 9008, BaseSpecies: "Testseed",
			Types: []string{"Grass"}, BaseStats: importer.BaseStats{HP: 30, Atk: 30, Def: 30, SpA: 30, SpD: 30, Spe: 30},
			Abilities: map[string]string{"0": "Test Leaf"}, IsNonstandard: pastPtr()},
	)
	in.Showdown.Learnsets["testsprout"] = map[string]int{"teststrike": 9, "testold": 9}
	in.Showdown.Learnsets["testseed"] = map[string]int{"testflame": 9}
	showdownSpecies(t, in, "testleaf").Prevo = "Testsprout"
}

// enableInheritFromPrevo はレギュレーションの inheritFromPrevo を true にする(既定は false。
// M-C では使わないが、将来のレギュレーションのために実装を残してあるので、その経路のテストに使う)。
func enableInheritFromPrevo(t *testing.T, in *importer.Input) {
	t.Helper()
	requireRegulation(t, in)
	in.Regulations.Regulations[0].InheritFromPrevo = true
}

func learnsetsByKey(out importer.Output) map[string][]string {
	got := map[string][]string{}
	for _, r := range out.Learnsets {
		got[r.SpeciesKey] = append(got[r.SpeciesKey], r.MoveID)
	}
	for k := range got {
		sort.Strings(got[k])
	}
	return got
}

// 既定(inheritFromPrevo: false。M-C の実際の規則)では、進化系統があっても進化前からは
// 継がない。フォーム(testleafrain)は基本種(testleaf)の学習元を1段だけ継ぐ(ADR-0101 §5。
// これは prevo とは別の規則で、フラグに関わらず常に効く)。
func TestConvertLearnsetsDoesNotInheritFromPrevoByDefault(t *testing.T) {
	in := loadFixture(t)
	addPrevoChain(t, &in)
	out, _ := convertOK(t, in)
	want := map[string][]string{
		"9001-000": {"testflame", "testglare", "teststrike"},
		"9001-001": {"testflame", "testglare", "teststrike"},
		// testleaf は自分の学習元だけ(testsprout/testseed の技を継がない)。
		"9002-000": {"testglare", "testsplash"},
		// フォーム(testleafrain)は基本種(testleaf)の学習元を1段だけ継ぐ(testleaf 自身の値と同じ)。
		"9002-002": {"testglare", "testsplash"},
		"9003-000": {"testrevived", "teststrike"},
	}
	if got := learnsetsByKey(out); !reflect.DeepEqual(got, want) {
		t.Errorf("learnsets = %v, want %v(既定では進化前から継がない)", got, want)
	}
	for _, sp := range out.Species {
		if sp.ShowdownID == "testsprout" || sp.ShowdownID == "testseed" {
			t.Errorf("進化前の種族 %s を取り込んだ", sp.ShowdownID)
		}
	}
}

// inheritFromPrevo: true(既定では使わないが、将来のレギュレーションのために実装を残す経路)。
// このケースは元々 M-C の既定案だったときのテストをそのまま流用する。
func TestConvertLearnsetsInheritFromPrevoChainWhenRegulationOptsIn(t *testing.T) {
	in := loadFixture(t)
	addPrevoChain(t, &in)
	enableInheritFromPrevo(t, &in)
	out, _ := convertOK(t, in)
	want := map[string][]string{
		"9001-000": {"testflame", "testglare", "teststrike"},
		"9001-001": {"testflame", "testglare", "teststrike"},
		// 自分の習得技 + 進化前(何段でも)の習得技。取り込まない技(testold)は落とす。
		"9002-000": {"testflame", "testglare", "testsplash", "teststrike"},
		// 自分の習得技が無いフォームは基本種の resolved(進化前の分を含む)を継ぐ。
		"9002-002": {"testflame", "testglare", "testsplash", "teststrike"},
		"9003-000": {"testrevived", "teststrike"},
	}
	if got := learnsetsByKey(out); !reflect.DeepEqual(got, want) {
		t.Errorf("learnsets = %v, want %v", got, want)
	}
	for _, sp := range out.Species {
		if sp.ShowdownID == "testsprout" || sp.ShowdownID == "testseed" {
			t.Errorf("進化前の種族 %s を取り込んだ(学習元として使うだけ)", sp.ShowdownID)
		}
	}
}

// レギュレーションの minSourceGen(fixture は 9)未満の世代の学習元しか無い技は、
// PS の TeamValidator に合わせて学習技から除く(ADR-0103 §7。§10 の実データ確認で判明)。
// この絞り込みは inheritFromPrevo の値に関わらず常に効く(自分の学習元にも適用する)。
func TestConvertLearnsetsExcludesSourcesOlderThanMinSourceGen(t *testing.T) {
	t.Run("自分の直接の学習元が世代基準未満なら除く(inheritFromPrevo に関わらず)", func(t *testing.T) {
		in := loadFixture(t)
		in.Showdown.Learnsets["testleaf"] = map[string]int{"testglare": 9, "testsplash": 5}
		out, _ := convertOK(t, in)
		got := learnsetsByKey(out)
		if want := []string{"testglare"}; !reflect.DeepEqual(got["9002-000"], want) {
			t.Errorf("9002-000 の学習技 = %v, want %v(testsplash は世代5の学習元しか無く minSourceGen 9 未満)", got["9002-000"], want)
		}
	})
	t.Run("進化前からの継承(inheritFromPrevo: true の経路)も世代基準で絞る", func(t *testing.T) {
		in := loadFixture(t)
		addPrevoChain(t, &in)
		enableInheritFromPrevo(t, &in)
		// testsprout の teststrike を世代5だけにする(testold は元々取り込まない技)。
		in.Showdown.Learnsets["testsprout"] = map[string]int{"teststrike": 5, "testold": 9}
		out, _ := convertOK(t, in)
		got := learnsetsByKey(out)
		want := []string{"testflame", "testglare", "testsplash"} // teststrike は世代基準未満で継承されない
		if !reflect.DeepEqual(got["9002-000"], want) {
			t.Errorf("9002-000 の学習技 = %v, want %v(teststrike は進化前 testsprout でも世代5の学習元しか無い)", got["9002-000"], want)
		}
	})
}

// inheritFromPrevo: true のときだけ prevo を検証する(実際にたどって使うため)。
// このケースは元々 M-C の既定案だったときのテストをそのまま流用する。
func TestConvertLearnsetsPrevoErrorsWhenRegulationOptsIn(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, in *importer.Input)
	}{
		{"prevo が Showdown に無い種族を指す", func(t *testing.T, in *importer.Input) {
			showdownSpecies(t, in, "testleaf").Prevo = "Testnowhere"
		}},
		{"prevo が循環する", func(t *testing.T, in *importer.Input) {
			addPrevoChain(t, in)
			showdownSpecies(t, in, "testseed").Prevo = "Testleaf"
		}},
		{"畳んだフォームだけにある不正な prevo も拒否する", func(t *testing.T, in *importer.Input) {
			in.Showdown.Learnsets["testleafbloom"] = map[string]int{"testglare": 9}
			showdownSpecies(t, in, "testleafbloom").Prevo = "Testnowhere"
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := loadFixture(t)
			enableInheritFromPrevo(t, &in)
			tt.mutate(t, &in)
			if _, _, err := importer.Convert(in); !errors.Is(err, importer.ErrInvalidData) {
				t.Errorf("err = %v, want ErrInvalidData", err)
			}
		})
	}
}

// 既定(inheritFromPrevo: false)では prevo を見ないので、壊れた prevo(無い種族を指す・循環)が
// あっても止めない(単に無視される)。M-C の実データにも、フォームで自分の学習元が無く prevo だけ
// 持つ種族があったが、いずれも基本種(baseSpecies)へのフォールバックで解決でき prevo には依らなかった
// (ADR-0103 §12)。
func TestConvertLearnsetsIgnoresBrokenPrevoByDefault(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, in *importer.Input)
	}{
		{"prevo が Showdown に無い種族を指す", func(t *testing.T, in *importer.Input) {
			showdownSpecies(t, in, "testleaf").Prevo = "Testnowhere"
		}},
		{"prevo が循環する", func(t *testing.T, in *importer.Input) {
			addPrevoChain(t, in)
			showdownSpecies(t, in, "testseed").Prevo = "Testleaf"
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := loadFixture(t)
			tt.mutate(t, &in)
			if _, _, err := importer.Convert(in); err != nil {
				t.Errorf("err = %v, want nil(inheritFromPrevo: false では prevo を見ないので壊れていても無視する)", err)
			}
		})
	}
}

func TestReconcileReportsInheritedLearnsets(t *testing.T) {
	t.Run("進化前が無ければ 0", func(t *testing.T) {
		_, rec := reconcileOK(t, reconcileInput(t))
		if rec.Learnsets.Inherited != 0 || len(rec.Learnsets.InheritedBySpecies) != 0 {
			t.Errorf("Learnsets = %+v, want 0", rec.Learnsets)
		}
	})
	t.Run("既定(inheritFromPrevo: false)では進化系統があっても 0", func(t *testing.T) {
		in := reconcileInput(t)
		addPrevoChain(t, &in)
		_, rec := reconcileOK(t, in)
		if rec.Learnsets.Inherited != 0 || len(rec.Learnsets.InheritedBySpecies) != 0 {
			t.Errorf("Learnsets = %+v, want 0(M-C は進化前から継がない)", rec.Learnsets)
		}
	})
	t.Run("inheritFromPrevo: true なら継承で増えた (species_key, move_id) の行数を数える", func(t *testing.T) {
		in := reconcileInput(t)
		addPrevoChain(t, &in)
		enableInheritFromPrevo(t, &in)
		_, rec := reconcileOK(t, in)
		// 9002-000 と 9002-002 にそれぞれ teststrike / testflame が増える。
		if rec.Learnsets.Inherited != 4 {
			t.Errorf("Learnsets.Inherited = %d, want 4", rec.Learnsets.Inherited)
		}
		want := map[string]int{"9002-000": 2, "9002-002": 2}
		if !reflect.DeepEqual(rec.Learnsets.InheritedBySpecies, want) {
			t.Errorf("Learnsets.InheritedBySpecies = %v, want %v", rec.Learnsets.InheritedBySpecies, want)
		}
	})
}
