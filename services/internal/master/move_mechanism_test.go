package master_test

import (
	"errors"
	"reflect"
	"sort"
	"testing"

	"example.com/pokecalc/services/internal/master"
)

// 技の機構(move_mechanisms。ADR-0121)の検証。行が無い = 通常の技。

func TestAllMoveMechanismsIsSortedAndUnique(t *testing.T) {
	all := master.AllMoveMechanisms()
	if len(all) == 0 {
		t.Fatal("機構の一覧が空")
	}
	seen := map[master.MoveMechanism]bool{}
	for _, m := range all {
		if seen[m] {
			t.Fatalf("機構 %q が重複", m)
		}
		seen[m] = true
	}
	if !sort.SliceIsSorted(all, func(i, j int) bool { return all[i] < all[j] }) {
		t.Fatalf("機構の一覧が昇順でない: %v", all)
	}
	// 呼び出し側が書き換えても次の呼び出しに影響しない。
	all[0] = "broken"
	if master.AllMoveMechanisms()[0] == "broken" {
		t.Fatal("AllMoveMechanisms が内部のスライスをそのまま返している")
	}
}

func TestMoveMechanismsOf(t *testing.T) {
	cases := []struct {
		name     string
		row      master.MoveRow
		want     []master.MoveMechanism
		wantFail bool
	}{
		{name: "行が無い攻撃技は通常の技(nil)", row: master.MoveRow{Category: "physical"}, want: nil},
		{name: "空スライスも通常の技", row: master.MoveRow{Category: "special", Mechanisms: []string{}}, want: nil},
		{
			name: "複数の機構は昇順に並べ直す",
			row:  master.MoveRow{Category: "physical", Mechanisms: []string{"variable_power", "multi_hit"}},
			want: []master.MoveMechanism{master.MechanismMultiHit, master.MechanismVariablePower},
		},
		{name: "未知の機構は不正", row: master.MoveRow{Category: "physical", Mechanisms: []string{"teleport"}}, wantFail: true},
		{name: "大文字違いは不正", row: master.MoveRow{Category: "physical", Mechanisms: []string{"Multi_Hit"}}, wantFail: true},
		{name: "重複は不正", row: master.MoveRow{Category: "physical", Mechanisms: []string{"ohko", "ohko"}}, wantFail: true},
		{name: "変化技に機構は不正", row: master.MoveRow{Category: "status", Mechanisms: []string{"move_specific"}}, wantFail: true},
		{name: "変化技で行が無いのは正常", row: master.MoveRow{Category: "status"}, want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := master.MoveMechanismsOf(tc.row)
			if tc.wantFail {
				if !errors.Is(err, master.ErrInvalidRow) {
					t.Fatalf("err = %v, want ErrInvalidRow", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// master.Move は機構を検証する(不正な行を calc に渡さない)。engine の型にはまだ載せない(D16)。
func TestMoveRejectsInvalidMechanisms(t *testing.T) {
	c := testChart(t)
	row := master.MoveRow{ID: "testhit", NameJa: "テストヒット", Type: "normal", Category: "physical", Power: 25, Mechanisms: []string{"unknown"}}
	if _, err := master.Move(row, c); !errors.Is(err, master.ErrInvalidRow) {
		t.Fatalf("err = %v, want ErrInvalidRow", err)
	}
	row.Mechanisms = []string{"multi_hit"}
	if _, err := master.Move(row, c); err != nil {
		t.Fatalf("正しい機構で失敗: %v", err)
	}
}
