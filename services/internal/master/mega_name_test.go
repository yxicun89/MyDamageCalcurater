package master_test

// メガ種族の日本語名の生成規則(ADR-0324・issue #607 で ADR-0140 が範囲を明確化)。
// 生成するのはフォーム名が "Mega" または "Mega-<識別子>" のときだけ。Mega が先頭に来ないフォーム(M-Mega・F-Mega)や
// メガでないフォーム(地方の姿など)は、規則を上流で検証できないので生成しない("" を返し、呼び出し側は英語名のまま)。

import (
	"testing"

	"example.com/pokecalc/services/internal/master"
)

func TestMegaNameJa(t *testing.T) {
	tests := []struct {
		base, forme string
		want        string
	}{
		{"テストモン", "Mega", "メガテストモン"},
		{"テストモン", "Mega-X", "メガテストモンX"},
		{"テストモン", "Mega-Y", "メガテストモンY"},
		{"テストモン", "Mega-Z", "メガテストモンZ"},
		{"", "Mega", ""},
		{"  ", "Mega-X", ""},
		// 以下は issue #607 で明確化: 規則に合わないフォームは生成しない。
		{"テストモン", "M-Mega", ""},
		{"テストモン", "F-Mega", ""},
		{"テストモン", "Alola", ""},
		{"テストモン", "", ""},
	}
	for _, tt := range tests {
		if got := master.MegaNameJa(tt.base, tt.forme); got != tt.want {
			t.Errorf("MegaNameJa(%q, %q) = %q, want %q", tt.base, tt.forme, got, tt.want)
		}
	}
}
