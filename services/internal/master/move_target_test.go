package master_test

// 技の対象(moves.target。ADR-0136・issue #288)の検証。
//
// 値の語彙は Showdown の技データの `target`(@smogon/calc 0.12.0 の data/interface.d.ts の MoveTarget と
// 同じ 15 種)を取得元の文字列のまま持つ。下の期待値は実装の写しではなく、取得元の型定義から独立に書いたもの
// (docs/coding-rules.md §2 の「独立した検証」)。値の一覧の正は master.AllMoveTargets で、
// migration の CHECK(chk_moves_target)との一致は services/pokedex/db の layout テストで確かめる。
//
// engine.Move.Target への写像(全体技 → spread、その他の既知の値 → single、空 → 不明)は ADR-0223 で入れた
// (ADR-0136 §4 の「engine.Move には載せない」を置き換える。ダブルの補正の計算は ADR-0222)。

import (
	"errors"
	"reflect"
	"sort"
	"testing"

	"example.com/pokecalc/engine"
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

// TestMoveMapsTargetToEngine は master.Move が技の対象を engine.Move.Target に写すこと(issue 288・ADR-0223。
// ADR-0136 §4 の「engine.Move には載せない」を置き換える)。写像:
//   - 全体技(IsSpread: allAdjacent・allAdjacentFoes)→ engine.MoveTargetSpread
//   - それ以外の既知の 13 種 → engine.MoveTargetSingle(@smogon/calc は全体技の補正を spread の2種にだけかけるため、
//     自分・味方・場の技も「単体として計算する」側に入る)
//   - 空(不明: 取り込み前の行・target を運ばない古い pokedex)→ ""(engine はダブルで move_target_unknown の印を付ける)
//
// 期待値の spread の集合は TestMoveTargetIsSpread と同じく取得元(@smogon/calc の gen789.js)から独立に書く。
// 対象の有無で engine.Move の他のフィールドが変わらないことも確かめる。
func TestMoveMapsTargetToEngine(t *testing.T) {
	c := testChart(t)
	base := master.MoveRow{ID: "testflame", NameJa: "テストフレイム", Type: "fire", Category: "special", Power: 90, Priority: 0}
	unknown, err := master.Move(base, c)
	if err != nil {
		t.Fatalf("Move(対象なし): %v", err)
	}
	if unknown.Target != "" {
		t.Fatalf("対象が空の行: Target = %q, want \"\"(不明)", unknown.Target)
	}
	spread := map[string]bool{"allAdjacent": true, "allAdjacentFoes": true}
	for _, target := range showdownMoveTargets {
		t.Run(target, func(t *testing.T) {
			row := base
			row.Target = target
			got, err := master.Move(row, c)
			if err != nil {
				t.Fatalf("Move(target=%q): %v", target, err)
			}
			want := engine.MoveTargetSingle
			if spread[target] {
				want = engine.MoveTargetSpread
			}
			if got.Target != want {
				t.Errorf("target=%q: engine.Move.Target = %q, want %q", target, got.Target, want)
			}
			// 対象以外は対象なしの写像と同じ(対象が他のフィールドに漏れない)。
			got.Target = ""
			if !reflect.DeepEqual(got, unknown) {
				t.Errorf("target=%q で対象以外のフィールドが変わった: %+v, want %+v", target, got, unknown)
			}
		})
	}
	row := base
	row.Target = "teleport"
	if _, err := master.Move(row, c); !errors.Is(err, master.ErrInvalidRow) {
		t.Fatalf("未知の対象: err = %v, want ErrInvalidRow", err)
	}
}
