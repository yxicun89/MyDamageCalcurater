package master_test

// 技の対象(moves.target。ADR-0136・issue #288)の検証。
//
// 値の語彙は Showdown の技データの `target`(@smogon/calc 0.12.0 の data/interface.d.ts の MoveTarget と
// 同じ 15 種)を取得元の文字列のまま持つ。下の期待値は実装の写しではなく、取得元の型定義から独立に書いたもの
// (docs/coding-rules.md §2 の「独立した検証」)。値の一覧の正は master.AllMoveTargets で、
// migration の CHECK(chk_moves_target)との一致は services/pokedex/db の layout テストで確かめる。
//
// engine のダメージ計算はまだ対象を読まない(ダブル補正は判定レーン。ADR-0136 §4)。
// ここでは「DB の行の値が正しいか」だけを検証し、engine.Move の写像は変えない。

import (
	"errors"
	"reflect"
	"sort"
	"testing"

	"example.com/pokecalc/services/internal/master"
)

// showdownMoveTargets は Showdown(sim/dex-moves.ts)と @smogon/calc 0.12.0(data/interface.d.ts)の
// MoveTarget の値(昇順)。
var showdownMoveTargets = []string{
	"adjacentAlly", "adjacentAllyOrSelf", "adjacentFoe", "all", "allAdjacent", "allAdjacentFoes",
	"allies", "allySide", "allyTeam", "any", "foeSide", "normal", "randomNormal", "scripted", "self",
}

func TestAllMoveTargetsMatchesShowdownVocabulary(t *testing.T) {
	all := master.AllMoveTargets()
	got := make([]string, 0, len(all))
	for _, v := range all {
		got = append(got, string(v))
	}
	if !reflect.DeepEqual(got, showdownMoveTargets) {
		t.Fatalf("AllMoveTargets = %v,\nwant %v", got, showdownMoveTargets)
	}
	if !sort.SliceIsSorted(all, func(i, j int) bool { return all[i] < all[j] }) {
		t.Fatalf("対象の一覧が昇順でない: %v", all)
	}
	// 呼び出し側が書き換えても次の呼び出しに影響しない。
	all[0] = "broken"
	if master.AllMoveTargets()[0] == "broken" {
		t.Fatal("AllMoveTargets が内部のスライスをそのまま返している")
	}
}

func TestIsMoveTarget(t *testing.T) {
	for _, v := range showdownMoveTargets {
		if !master.IsMoveTarget(v) {
			t.Errorf("IsMoveTarget(%q) = false, want true", v)
		}
	}
	// 大文字小文字・正規化した綴り・空は既知の値ではない(取得元の文字列のまま持つ。ADR-0136 §2)。
	for _, v := range []string{"", "Normal", "ALLADJACENTFOES", "all_adjacent_foes", "teleport"} {
		if master.IsMoveTarget(v) {
			t.Errorf("IsMoveTarget(%q) = true, want false", v)
		}
	}
}

// TestMoveTargetIsSpread は「相手の場の複数に当たる技」の判定。@smogon/calc 0.12.0 の
// mechanics/gen789.js が全体技の補正(ダブルで ×3072/4096)をかける条件
// `['allAdjacent', 'allAdjacentFoes'].includes(move.target)` と同じ集合であること。
func TestMoveTargetIsSpread(t *testing.T) {
	spread := map[string]bool{"allAdjacent": true, "allAdjacentFoes": true}
	for _, v := range showdownMoveTargets {
		if got := master.MoveTarget(v).IsSpread(); got != spread[v] {
			t.Errorf("MoveTarget(%q).IsSpread() = %v, want %v", v, got, spread[v])
		}
	}
}

func TestMoveTargetOf(t *testing.T) {
	cases := []struct {
		name     string
		target   string
		want     master.MoveTarget
		wantFail bool
	}{
		// 空 = 対象が分からない(migration 000010 の直後で importer がまだ入れていない行、
		// または内部 API がまだ target を運ばない calc-svc の経路。ADR-0136 §3)。不正にはしない。
		{name: "空は不明(エラーにしない)", target: "", want: ""},
		{name: "単体", target: "normal", want: "normal"},
		{name: "相手全体", target: "allAdjacentFoes", want: "allAdjacentFoes"},
		{name: "自分以外の全体", target: "allAdjacent", want: "allAdjacent"},
		{name: "自分", target: "self", want: "self"},
		{name: "未知の値は不正", target: "teleport", wantFail: true},
		{name: "大文字違いは不正", target: "AllAdjacentFoes", wantFail: true},
		{name: "正規化した綴りは不正", target: "all_adjacent_foes", wantFail: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := master.MoveTargetOf(master.MoveRow{Category: "special", Target: tc.target})
			if tc.wantFail {
				if !errors.Is(err, master.ErrInvalidRow) {
					t.Fatalf("err = %v, want ErrInvalidRow", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("MoveTargetOf: %v", err)
			}
			if got != tc.want {
				t.Fatalf("MoveTargetOf = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestMoveValidatesTargetWithoutChangingEngineMove は master.Move が対象を検証する(未知の値は ErrInvalidRow)が、
// engine.Move の中身は対象の有無で変わらないこと(ダメージ計算の挙動を変えない。ゴールデン不変。ADR-0136 §4)。
func TestMoveValidatesTargetWithoutChangingEngineMove(t *testing.T) {
	c := testChart(t)
	base := master.MoveRow{ID: "testflame", NameJa: "テストフレイム", Type: "fire", Category: "special", Power: 90, Priority: 0}
	want, err := master.Move(base, c)
	if err != nil {
		t.Fatalf("Move(対象なし): %v", err)
	}
	for _, target := range []string{"allAdjacentFoes", "normal", "any"} {
		row := base
		row.Target = target
		got, err := master.Move(row, c)
		if err != nil {
			t.Fatalf("Move(target=%q): %v", target, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("target=%q で engine.Move が変わった: %+v, want %+v", target, got, want)
		}
	}
	row := base
	row.Target = "teleport"
	if _, err := master.Move(row, c); !errors.Is(err, master.ErrInvalidRow) {
		t.Fatalf("未知の対象: err = %v, want ErrInvalidRow", err)
	}
}
