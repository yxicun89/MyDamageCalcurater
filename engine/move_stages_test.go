package engine

import "testing"

// 確率 100% のランク変化(ADR-0179 §1。連続技 CB-2)。
// 確率 100% 未満は常に返さない(docs/plan/combo-calc.md §7 Q4 のユーザー決定 (b))。
// 技の ID は書かない(効果データの形だけで決まる)。

func stagesEffect(chance int, target RankTarget, stages map[StatKey]int) *MoveEffect {
	return &MoveEffect{Chance: chance, Target: target, Stages: stages}
}

// S1
func TestGuaranteedSelfStages(t *testing.T) {
	tests := []struct {
		name string
		move Move
		want Ranks
	}{
		{"効果なし", Move{Category: CategoryPhysical, Power: 80}, Ranks{}},
		{"攻撃技・自分の素早さ +1(ニトロチャージ型)",
			Move{Category: CategoryPhysical, Power: 50, Effect: stagesEffect(100, RankTargetSelf, map[StatKey]int{StatSpe: 1})},
			Ranks{Spe: 1}},
		{"変化技・自分の攻撃 +1 と素早さ +1(りゅうのまい型。CB-1 で取り込む形)",
			Move{Category: CategoryStatus, Effect: stagesEffect(100, RankTargetSelf, map[StatKey]int{StatAtk: 1, StatSpe: 1})},
			Ranks{Atk: 1, Spe: 1}},
		{"変化技・攻撃 +2(つるぎのまい型)",
			Move{Category: CategoryStatus, Effect: stagesEffect(100, RankTargetSelf, map[StatKey]int{StatAtk: 2})},
			Ranks{Atk: 2}},
		{"攻撃技・自分の防御と特防 -1(インファイト型。下降もそのまま)",
			Move{Category: CategoryPhysical, Power: 120, Effect: stagesEffect(100, RankTargetSelf, map[StatKey]int{StatDef: -1, StatSpD: -1})},
			Ranks{Def: -1, SpD: -1}},
		{"攻撃技・自分の特攻 -2(りゅうせいぐん型)",
			Move{Category: CategorySpecial, Power: 130, Effect: stagesEffect(100, RankTargetSelf, map[StatKey]int{StatSpA: -2})},
			Ranks{SpA: -2}},
		{"上げて下げる(からをやぶる型。5 能力すべて)",
			Move{Category: CategoryStatus, Effect: stagesEffect(100, RankTargetSelf, map[StatKey]int{
				StatAtk: 2, StatSpA: 2, StatSpe: 2, StatDef: -1, StatSpD: -1,
			})},
			Ranks{Atk: 2, Def: -1, SpA: 2, SpD: -1, Spe: 2}},
		{"確率 100% 未満は返さない(メタルクロー型 10%)",
			Move{Category: CategoryPhysical, Power: 50, Effect: stagesEffect(10, RankTargetSelf, map[StatKey]int{StatAtk: 1})},
			Ranks{}},
		{"確率 99% も返さない(境界)",
			Move{Category: CategoryPhysical, Power: 50, Effect: stagesEffect(99, RankTargetSelf, map[StatKey]int{StatSpe: 1})},
			Ranks{}},
		{"相手が対象は自分に返さない",
			Move{Category: CategoryPhysical, Power: 60, Effect: stagesEffect(100, RankTargetOpponent, map[StatKey]int{StatSpe: -1})},
			Ranks{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GuaranteedSelfStages(tt.move); got != tt.want {
				t.Errorf("GuaranteedSelfStages = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// S2
func TestGuaranteedTargetStages(t *testing.T) {
	tests := []struct {
		name string
		move Move
		want Ranks
	}{
		{"効果なし", Move{Category: CategoryPhysical, Power: 80}, Ranks{}},
		{"攻撃技・相手の素早さ -1(がんせきふうじ型)",
			Move{Category: CategoryPhysical, Power: 60, Effect: stagesEffect(100, RankTargetOpponent, map[StatKey]int{StatSpe: -1})},
			Ranks{Spe: -1}},
		{"変化技・相手の攻撃 -1(なきごえ型。CB-1 で取り込む形)",
			Move{Category: CategoryStatus, Effect: stagesEffect(100, RankTargetOpponent, map[StatKey]int{StatAtk: -1})},
			Ranks{Atk: -1}},
		{"変化技・相手の防御 -2",
			Move{Category: CategoryStatus, Effect: stagesEffect(100, RankTargetOpponent, map[StatKey]int{StatDef: -2})},
			Ranks{Def: -2}},
		{"確率 100% 未満は返さない(特防 -1 が 20%)",
			Move{Category: CategorySpecial, Power: 90, Effect: stagesEffect(20, RankTargetOpponent, map[StatKey]int{StatSpD: -1})},
			Ranks{}},
		{"自分が対象は相手に返さない",
			Move{Category: CategoryPhysical, Power: 50, Effect: stagesEffect(100, RankTargetSelf, map[StatKey]int{StatSpe: 1})},
			Ranks{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GuaranteedTargetStages(tt.move); got != tt.want {
				t.Errorf("GuaranteedTargetStages = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// S3: ADR-0177 §4 の GuaranteedSelfSpeedStage は GuaranteedSelfStages の素早さと常に同じ(結果を変えない)。
func TestGuaranteedSelfSpeedStageIsSelfStagesSpe(t *testing.T) {
	moves := []Move{
		{Category: CategoryPhysical, Power: 50},
		{Category: CategoryPhysical, Power: 50, Effect: stagesEffect(100, RankTargetSelf, map[StatKey]int{StatSpe: 1})},
		{Category: CategoryStatus, Effect: stagesEffect(100, RankTargetSelf, map[StatKey]int{StatSpe: 2})},
		{Category: CategoryPhysical, Power: 100, Effect: stagesEffect(100, RankTargetSelf, map[StatKey]int{StatSpe: -1})},
		{Category: CategoryStatus, Effect: stagesEffect(100, RankTargetSelf, map[StatKey]int{StatAtk: 1, StatSpe: 1})},
		{Category: CategoryPhysical, Power: 50, Effect: stagesEffect(99, RankTargetSelf, map[StatKey]int{StatSpe: 1})},
		{Category: CategorySpecial, Power: 55, Effect: stagesEffect(100, RankTargetOpponent, map[StatKey]int{StatSpe: -1})},
		{Category: CategoryPhysical, Power: 120, Effect: stagesEffect(100, RankTargetSelf, map[StatKey]int{StatDef: -1, StatSpD: -1})},
	}
	for i, m := range moves {
		if a, b := GuaranteedSelfSpeedStage(m), GuaranteedSelfStages(m).Spe; a != b {
			t.Errorf("#%d: GuaranteedSelfSpeedStage = %d, GuaranteedSelfStages.Spe = %d", i, a, b)
		}
	}
}

// S4: ランクは -6..+6 に収める(連続技 AC-10 の土台)。
func TestRanksAddClamped(t *testing.T) {
	tests := []struct {
		name string
		r, d Ranks
		want Ranks
	}{
		{"ゼロ同士", Ranks{}, Ranks{}, Ranks{}},
		{"単純な加算", Ranks{Atk: 1}, Ranks{Atk: 1, Spe: 1}, Ranks{Atk: 2, Spe: 1}},
		{"+6 にさらに +2 は +6(つるぎのまい)", Ranks{Atk: 6}, Ranks{Atk: 2}, Ranks{Atk: 6}},
		{"+5 に +2 は +6", Ranks{Atk: 5}, Ranks{Atk: 2}, Ranks{Atk: 6}},
		{"-6 にさらに -1 は -6", Ranks{Spe: -6}, Ranks{Spe: -1}, Ranks{Spe: -6}},
		{"-5 に -2 は -6", Ranks{SpA: -5}, Ranks{SpA: -2}, Ranks{SpA: -6}},
		{"下げてから上げる", Ranks{Def: -1, SpD: -1}, Ranks{Def: 2, SpD: 1}, Ranks{Def: 1, SpD: 0}},
		{"全能力が独立に収まる", Ranks{Atk: 6, Def: -6, SpA: 3, SpD: -3, Spe: 0},
			Ranks{Atk: 1, Def: -1, SpA: 4, SpD: -4, Spe: 6},
			Ranks{Atk: 6, Def: -6, SpA: 6, SpD: -6, Spe: 6}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.AddClamped(tt.d); got != tt.want {
				t.Errorf("%+v.AddClamped(%+v) = %+v, want %+v", tt.r, tt.d, got, tt.want)
			}
		})
	}
}
