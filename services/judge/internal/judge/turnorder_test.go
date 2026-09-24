package judge

import (
	"strconv"
	"testing"

	"example.com/pokecalc/engine"
)

// JD4(ADR-0704 §2)の先制判定。ゲームの一般ルールをそのまま実装する:
//
//	優先度が違う          → 優先度が高い方が必ず先に動く(素早さもトリックルームも見ない)
//	優先度が同じ          → 素早さで決まる(SpeedComparison.Outspeeds。トリックルーム反映済み)
//	優先度も素早さも同じ  → どちらが先か決まらない(Tie。AttackerMovesFirst は false)
//
// 素早さの比較(CompareSpeed)は JD1〜JD3 のまま意味を変えない。先制判定は「優先度を含めた
// 行動順」という別の問いなので、結果を受け取る別の関数にする(ADR-0704 §2・却下した案)。

// TestCompareTurnOrderPriorityWins: 優先度が違えば高い方が必ず先に動く。素早さがどうであっても、
// トリックルームがかかっていても変わらない(トリックルームは優先度に影響しない)。
func TestCompareTurnOrderPriorityWins(t *testing.T) {
	t.Parallel()

	// 素早さの比較は 3 通りだけ用意すれば足りる(値そのものは先制判定に使われない)。
	var (
		faster = SpeedComparison{AttackerSpeed: 167, DefenderSpeed: 120, Outspeeds: true}
		slower = SpeedComparison{AttackerSpeed: 120, DefenderSpeed: 167}
		tied   = SpeedComparison{AttackerSpeed: 120, DefenderSpeed: 120, SpeedTie: true}
	)

	tests := []struct {
		name                               string
		attackerPriority, defenderPriority int
		speed                              SpeedComparison
		want                               TurnOrder
	}{
		{
			// JD4 の主眼: 素早さで負けていても先制技なら先に動く。
			"遅くても優先度が高ければ先に動く",
			1, 0, slower,
			TurnOrder{AttackerMovesFirst: true},
		},
		{
			"速くても優先度が低ければ後になる",
			0, 1, faster,
			TurnOrder{AttackerMovesFirst: false},
		},
		{
			// 同速でも優先度で決まるなら tie にはならない(speedTie とは別の欄)。
			"同速でも優先度が高ければ先に動き、tie にはならない",
			1, 0, tied,
			TurnOrder{AttackerMovesFirst: true},
		},
		{
			"同速で優先度が低ければ後になり、tie にはならない",
			0, 1, tied,
			TurnOrder{AttackerMovesFirst: false},
		},
		{
			// 優先度 0 を特別扱いしない。負どうしでも「高い方」が勝つ。
			"負の優先度どうしでも高い方が先に動く",
			-1, -6, slower,
			TurnOrder{AttackerMovesFirst: true},
		},
		{
			"負の優先度どうしで低い方は後になる",
			-6, -1, faster,
			TurnOrder{AttackerMovesFirst: false},
		},
		{
			"相手だけが先制技(優先度 0 対 +1)",
			0, 1, slower,
			TurnOrder{AttackerMovesFirst: false},
		},
		{
			"どちらも同じ先制技なら優先度では決まらない(速い方が先)",
			1, 1, faster,
			TurnOrder{AttackerMovesFirst: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := CompareTurnOrder(tt.attackerPriority, tt.defenderPriority, tt.speed)
			if got != tt.want {
				t.Errorf("CompareTurnOrder(%d, %d, %+v) = %+v, want %+v",
					tt.attackerPriority, tt.defenderPriority, tt.speed, got, tt.want)
			}
			if got.AttackerMovesFirst && got.Tie {
				t.Error("attackerMovesFirst と tie が同時に true になっている(ADR-0704 §2)")
			}
		})
	}
}

// TestCompareTurnOrderFallsBackToSpeed: 優先度が同じときだけ素早さで決まる。
// SpeedComparison.Outspeeds は ADR-0702 §3 で既にトリックルームを反映しているので、
// 先制判定はトリックルームを二度解釈しない。
func TestCompareTurnOrderFallsBackToSpeed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		speed SpeedComparison
		want  TurnOrder
	}{
		{
			"速い方が先に動く",
			SpeedComparison{AttackerSpeed: 167, DefenderSpeed: 120, Outspeeds: true},
			TurnOrder{AttackerMovesFirst: true},
		},
		{
			"遅い方は後になる",
			SpeedComparison{AttackerSpeed: 120, DefenderSpeed: 167},
			TurnOrder{AttackerMovesFirst: false},
		},
		{
			// トリックルーム中の CompareSpeed の出力(実数値は遅いが Outspeeds が true)。
			"トリックルーム中は遅い方が先に動く",
			SpeedComparison{AttackerSpeed: 120, DefenderSpeed: 167, Outspeeds: true},
			TurnOrder{AttackerMovesFirst: true},
		},
		{
			"トリックルーム中は速い方が後になる",
			SpeedComparison{AttackerSpeed: 167, DefenderSpeed: 120},
			TurnOrder{AttackerMovesFirst: false},
		},
		{
			// 同速 + 同優先度はどちらが先か決まらない(ADR-0704 §2・ADR-0602 と同じ立場)。
			"同速なら tie で、先に動くとは言わない",
			SpeedComparison{AttackerSpeed: 120, DefenderSpeed: 120, SpeedTie: true},
			TurnOrder{AttackerMovesFirst: false, Tie: true},
		},
	}
	for _, priority := range []int{-6, 0, 1, 5} {
		for _, tt := range tests {
			t.Run(tt.name+"(優先度 "+strconv.Itoa(priority)+" どうし)", func(t *testing.T) {
				t.Parallel()

				// 両者が同じ優先度なら、その値が何であれ素早さの結果がそのまま行動順になる。
				got := CompareTurnOrder(priority, priority, tt.speed)
				if got != tt.want {
					t.Errorf("CompareTurnOrder(%d, %d, %+v) = %+v, want %+v",
						priority, priority, tt.speed, got, tt.want)
				}
				if got.AttackerMovesFirst && got.Tie {
					t.Error("attackerMovesFirst と tie が同時に true になっている")
				}
			})
		}
	}
}

// TestCompareTurnOrderIgnoresSpeedValuesWhenPrioritiesDiffer: 優先度が違うときは
// SpeedComparison の中身を一切読まない。速さの値だけを動かしても結果が変わらないことで確かめる
// (「優先度が高ければ勝つが、素早さも見ている」実装を落とすため)。
func TestCompareTurnOrderIgnoresSpeedValuesWhenPrioritiesDiffer(t *testing.T) {
	t.Parallel()

	speeds := []SpeedComparison{
		{AttackerSpeed: 1, DefenderSpeed: 999},
		{AttackerSpeed: 999, DefenderSpeed: 1, Outspeeds: true},
		{AttackerSpeed: 120, DefenderSpeed: 120, SpeedTie: true},
		{}, // ゼロ値(素早さを求める前でも優先度だけで決まる)
	}
	for _, speed := range speeds {
		got := CompareTurnOrder(1, 0, speed)
		want := TurnOrder{AttackerMovesFirst: true}
		if got != want {
			t.Errorf("CompareTurnOrder(1, 0, %+v) = %+v, want %+v(優先度が違えば素早さを見ない)", speed, got, want)
		}
		got = CompareTurnOrder(0, 1, speed)
		want = TurnOrder{AttackerMovesFirst: false}
		if got != want {
			t.Errorf("CompareTurnOrder(0, 1, %+v) = %+v, want %+v(優先度が違えば素早さを見ない)", speed, got, want)
		}
	}
}

// TestCompareTurnOrderMatchesCompareSpeedUnderTrickRoom: CompareSpeed → CompareTurnOrder を
// つないだときに、トリックルームの扱いが JD2(ADR-0702 §3)のままであることを確かめる。
// 優先度が同じなら向きが反転し、優先度が違えばトリックルームでも反転しない。
func TestCompareTurnOrderMatchesCompareSpeedUnderTrickRoom(t *testing.T) {
	t.Parallel()

	fast := Individual{BaseSpeed: 100, Nature: naturePlusSpe, SP: engine.Stats{Spe: 32}} // 167
	slow := Individual{BaseSpeed: 100, Nature: engine.NatureNeutral}                     // 120

	tests := []struct {
		name                               string
		attacker, defender                 Individual
		trickRoom                          bool
		attackerPriority, defenderPriority int
		want                               TurnOrder
	}{
		{"同優先度・通常: 速い方が先", fast, slow, false, 0, 0, TurnOrder{AttackerMovesFirst: true}},
		{"同優先度・トリックルーム: 速い方が後", fast, slow, true, 0, 0, TurnOrder{AttackerMovesFirst: false}},
		{"同優先度・トリックルーム: 遅い方が先", slow, fast, true, 0, 0, TurnOrder{AttackerMovesFirst: true}},
		// 優先度はトリックルームの影響を受けない(ADR-0704 §2)。
		{"優先度が高い側はトリックルームでも先", slow, fast, false, 1, 0, TurnOrder{AttackerMovesFirst: true}},
		{"優先度が高い側はトリックルーム中でも先", slow, fast, true, 1, 0, TurnOrder{AttackerMovesFirst: true}},
		{"優先度が低い側はトリックルーム中でも後", fast, slow, true, 0, 1, TurnOrder{AttackerMovesFirst: false}},
		{"優先度が低い側は通常でも後", fast, slow, false, 0, 1, TurnOrder{AttackerMovesFirst: false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			speed, err := CompareSpeed(tt.attacker, tt.defender, SpeedField{TrickRoom: tt.trickRoom})
			if err != nil {
				t.Fatalf("CompareSpeed: %v", err)
			}
			if got := CompareTurnOrder(tt.attackerPriority, tt.defenderPriority, speed); got != tt.want {
				t.Errorf("CompareTurnOrder = %+v, want %+v(速さ %+v)", got, tt.want, speed)
			}
		})
	}
}
