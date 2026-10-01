package dataversion_test

import (
	"strings"
	"testing"

	"example.com/pokecalc/services/pokedex/internal/dataversion"
	"example.com/pokecalc/services/pokedex/internal/store"
)

func TestString(t *testing.T) {
	sumA := "0123456789abcdef" + strings.Repeat("0", 48)
	sumB := "fedcba9876543210" + strings.Repeat("f", 48)
	tests := []struct {
		name string
		in   []store.DataVersion
		want string
	}{
		{name: "行が無ければ空", in: nil, want: ""},
		{name: "1行", in: []store.DataVersion{{Source: "calc", Version: "0.12.0", Checksum: sumA}}, want: "calc=0.12.0@01234567"},
		{
			name: "source 昇順に連結し、入力の順に左右されない",
			in: []store.DataVersion{
				{Source: "showdown", Version: "abc", Checksum: sumB},
				{Source: "calc", Version: "0.12.0", Checksum: sumA},
			},
			want: "calc=0.12.0@01234567,showdown=abc@fedcba98",
		},
		{name: "8桁未満の checksum はそのまま", in: []store.DataVersion{{Source: "local", Version: "local", Checksum: "abc"}}, want: "local=local@abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dataversion.String(tt.in); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStringDoesNotMutateInput(t *testing.T) {
	in := []store.DataVersion{{Source: "b", Version: "1", Checksum: "11111111"}, {Source: "a", Version: "2", Checksum: "22222222"}}
	_ = dataversion.String(in)
	if in[0].Source != "b" || in[1].Source != "a" {
		t.Errorf("入力の並びを書き換えた: %+v", in)
	}
}
