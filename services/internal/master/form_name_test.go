package master_test

import (
	"testing"

	"example.com/pokecalc/services/internal/master"
)

func TestFormNameJa(t *testing.T) {
	tests := []struct {
		name, base, form, want string
	}{
		{"基本種名 + 姿の名前", "テストモン", "テストのすがた", "テストモン（テストのすがた）"},
		{"前後の空白は落とす", " テストモン ", " テストのすがた ", "テストモン（テストのすがた）"},
		{"基本種名が無い: 作らない", "", "テストのすがた", ""},
		{"姿の名前が無い: 作らない", "テストモン", "", ""},
		{"姿の名前が空白だけ: 作らない", "テストモン", "  ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := master.FormNameJa(tt.base, tt.form); got != tt.want {
				t.Errorf("FormNameJa(%q, %q) = %q, want %q", tt.base, tt.form, got, tt.want)
			}
		})
	}
}
