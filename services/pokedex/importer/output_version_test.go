package importer_test

// 変換結果の内容ハッシュ(issue #379・ADR-0122)。取得元の版が同じでも、importer の変換ロジックや
// スキーマの変更で変換結果が変われば投入し、変わらなければスキップするための版。

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"example.com/pokecalc/services/pokedex/importer"
)

func convertFixture(t *testing.T) importer.Output {
	t.Helper()
	in, _, err := importer.LoadInput(fixtureRoot)
	if err != nil {
		t.Fatalf("LoadInput: %v", err)
	}
	out, _, err := importer.Convert(in)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	return out
}

func mustOutputVersion(t *testing.T, out importer.Output) importer.SourceVersion {
	t.Helper()
	v, err := importer.OutputVersion(out)
	if err != nil {
		t.Fatalf("OutputVersion: %v", err)
	}
	return v
}

// 同じ入力を何度変換しても同じハッシュになる(map の反復順などで揺れない)。
func TestOutputVersionIsDeterministic(t *testing.T) {
	first := mustOutputVersion(t, convertFixture(t))
	for i := 0; i < 20; i++ {
		if got := mustOutputVersion(t, convertFixture(t)); got != first {
			t.Fatalf("%d 回目の変換でハッシュが変わった: %+v != %+v", i+2, got, first)
		}
	}
	if first.Source != importer.OutputSource {
		t.Errorf("Source = %q, want %q", first.Source, importer.OutputSource)
	}
	if ok, err := importer.NeedsImport(nil, []importer.SourceVersion{first}); err != nil || !ok {
		t.Errorf("出力の版が data_versions の形式に合わない: (%v, %v)", ok, err)
	}
}

// 行の並び順は DB の中身を変えないので、ハッシュも変えない。
func TestOutputVersionIgnoresRowOrder(t *testing.T) {
	out := convertFixture(t)
	want := mustOutputVersion(t, out)
	reversed := out
	reversed.Species = slices.Clone(out.Species)
	slices.Reverse(reversed.Species)
	reversed.Moves = slices.Clone(out.Moves)
	slices.Reverse(reversed.Moves)
	if len(out.Species) < 2 || len(out.Moves) < 2 {
		t.Fatal("架空データの種族・技が2件未満で並び順を確かめられない")
	}
	if got := mustOutputVersion(t, reversed); got != want {
		t.Errorf("行の並びを逆にしただけでハッシュが変わった")
	}
}

// Output のどの表の中身が変わってもハッシュが変わる(新しい表を足したときの取りこぼし防止)。
func TestOutputVersionCoversEveryTable(t *testing.T) {
	base := mustOutputVersion(t, importer.Output{})
	typ := reflect.TypeOf(importer.Output{})
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		t.Run(f.Name, func(t *testing.T) {
			if f.Type.Kind() != reflect.Slice {
				t.Fatalf("Output.%s が slice ではない(ハッシュの対象に入るか確かめる)", f.Name)
			}
			var out importer.Output
			v := reflect.ValueOf(&out).Elem().Field(i)
			v.Set(reflect.MakeSlice(f.Type, 1, 1))
			if got := mustOutputVersion(t, out); got == base {
				t.Errorf("Output.%s に行を足してもハッシュが変わらない", f.Name)
			}
		})
	}
	// 同じ行でも表が違えば別のハッシュ(表の境界があいまいにならない)。
	a := mustOutputVersion(t, importer.Output{Abilities: []importer.NamedRow{{ID: "x"}}})
	b := mustOutputVersion(t, importer.Output{Items: []importer.NamedRow{{ID: "x"}}})
	if a == b {
		t.Error("特性と持ち物に同じ行を入れたときのハッシュが同じ")
	}
}

// 取得元の版が同じでも、変換結果が変われば投入する。変わらなければスキップする。force は常に投入。
func TestRunStoreReimportsWhenOutputChanges(t *testing.T) {
	pinned := []importer.SourceVersion{sv("calc", "v1", "a"), sv("showdown", "c1", "b")}
	oldOut := importer.Output{Moves: []importer.MoveRow{{ID: "tackle", Power: 40}}}
	newOut := importer.Output{Moves: []importer.MoveRow{{ID: "tackle", Power: 40}}, MoveMechanisms: []importer.MoveMechanismRow{{MoveID: "tackle", Mechanism: "multi-hit"}}}
	appliedOld := append(slices.Clone(pinned), mustOutputVersion(t, oldOut))

	tests := []struct {
		name        string
		out         importer.Output
		force       bool
		wantApplied bool
	}{
		{"変換結果が同じならスキップ", oldOut, false, false},
		{"変換結果が変われば投入", newOut, false, true},
		{"force なら変換結果が同じでも投入", oldOut, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &fakeStore{applied: appliedOld}
			got, err := importer.RunStore(context.Background(), s, tt.out, pinned, storeNow, tt.force)
			if err != nil {
				t.Fatalf("RunStore: %v", err)
			}
			if got != tt.wantApplied {
				t.Errorf("RunStore = %v, want %v", got, tt.wantApplied)
			}
			if !tt.wantApplied {
				return
			}
			// Apply に渡る版には、取得元の版と、今回の変換結果の版が入る(次回の比較に使う)。
			want := append(slices.Clone(pinned), mustOutputVersion(t, tt.out))
			if needs, _ := importer.NeedsImport(s.applyCalls[0], want); needs {
				t.Errorf("Apply に渡った版 %+v, want %+v", s.applyCalls[0], want)
			}
		})
	}
}

// 取得元の設定に出力の版と同じ名前があれば、形式の誤りとして投入しない。
func TestRunStoreRejectsSourceNamedLikeOutput(t *testing.T) {
	s := &fakeStore{}
	bad := []importer.SourceVersion{sv(importer.OutputSource, "v1", "a")}
	if _, err := importer.RunStore(context.Background(), s, importer.Output{}, bad, storeNow, true); err == nil {
		t.Fatal("取得元 importer-output と出力の版が重複しても通った")
	}
	if len(s.applyCalls) != 0 {
		t.Error("不正な版で Apply を呼んだ")
	}
}
