package engine

// 技の追加効果(命中時のランク変化)の型と検証(ADR-0107)。
//
// ここで確かめるのは2つだけ:
//   - MoveEffect が不正な値を受け付けないこと(決定3の Validate)
//   - 追加効果を持たせてもダメージ計算の結果が1ビットも変わらないこと(決定2。ゴールデン不変の担保)
//
// engine は「発動するかどうか」を判定しない(決定1)。乱数を持たないので、この package に
// 「発動した/しなかった」を決めるテストは無い。呼び出し側(judge)が Individual.Ranks を作り直す。

import (
	"reflect"
	"testing"
)

func TestMoveEffectValidateAcceptsRealisticValues(t *testing.T) {
	cases := []struct {
		name string
		e    MoveEffect
	}{
		{"必ず発動する自分の素早さ+1(ニトロチャージ相当)", MoveEffect{Chance: 100, Target: RankTargetSelf, Stages: map[StatKey]int{StatSpe: 1}}},
		{"必ず発動する自分の特攻-2(りゅうせいぐん相当)", MoveEffect{Chance: 100, Target: RankTargetSelf, Stages: map[StatKey]int{StatSpA: -2}}},
		{"必ず発動する自分の防御・特防-1(インファイト相当)", MoveEffect{Chance: 100, Target: RankTargetSelf, Stages: map[StatKey]int{StatDef: -1, StatSpD: -1}}},
		{"10%で自分の全能力+1(げんしのちから相当)", MoveEffect{Chance: 10, Target: RankTargetSelf, Stages: map[StatKey]int{
			StatAtk: 1, StatDef: 1, StatSpA: 1, StatSpD: 1, StatSpe: 1,
		}}},
		{"20%で相手の防御-1(かみくだく相当)", MoveEffect{Chance: 20, Target: RankTargetOpponent, Stages: map[StatKey]int{StatDef: -1}}},
		{"下限の確率", MoveEffect{Chance: 1, Target: RankTargetSelf, Stages: map[StatKey]int{StatAtk: 1}}},
		{"変化量の下限と上限", MoveEffect{Chance: 100, Target: RankTargetSelf, Stages: map[StatKey]int{StatAtk: 6, StatDef: -6}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.e.Validate(); err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestMoveEffectValidateRejectsInvalidValues(t *testing.T) {
	cases := []struct {
		name string
		e    MoveEffect
	}{
		{"確率0(追加効果なしは nil で表す)", MoveEffect{Chance: 0, Target: RankTargetSelf, Stages: map[StatKey]int{StatSpe: 1}}},
		{"確率が負", MoveEffect{Chance: -1, Target: RankTargetSelf, Stages: map[StatKey]int{StatSpe: 1}}},
		{"確率が101", MoveEffect{Chance: 101, Target: RankTargetSelf, Stages: map[StatKey]int{StatSpe: 1}}},
		{"対象が空", MoveEffect{Chance: 100, Target: "", Stages: map[StatKey]int{StatSpe: 1}}},
		{"対象が未知", MoveEffect{Chance: 100, Target: RankTarget("ally"), Stages: map[StatKey]int{StatSpe: 1}}},
		{"Stages が nil", MoveEffect{Chance: 100, Target: RankTargetSelf, Stages: nil}},
		{"Stages が空", MoveEffect{Chance: 100, Target: RankTargetSelf, Stages: map[StatKey]int{}}},
		{"変化量0(意味を持たない)", MoveEffect{Chance: 100, Target: RankTargetSelf, Stages: map[StatKey]int{StatSpe: 0}}},
		{"変化量が+7", MoveEffect{Chance: 100, Target: RankTargetSelf, Stages: map[StatKey]int{StatSpe: 7}}},
		{"変化量が-7", MoveEffect{Chance: 100, Target: RankTargetSelf, Stages: map[StatKey]int{StatSpe: -7}}},
		{"HP はランクを持たない", MoveEffect{Chance: 100, Target: RankTargetSelf, Stages: map[StatKey]int{StatHP: 1}}},
		{"未知のステータス(accuracy は engine の Ranks に無い)", MoveEffect{Chance: 100, Target: RankTargetSelf, Stages: map[StatKey]int{StatKey("accuracy"): -1}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.e.Validate(); err == nil {
				t.Fatalf("Validate() = nil, want error")
			}
		})
	}
}

// TestRankTargetValuesAreTheJSONIDs は列挙の文字列が効果定義 JSON / export の ID と同じであることを固定する
// (services/internal/master の Decode/EncodeMoveEffect が同じ文字列で往復するため)。
func TestRankTargetValuesAreTheJSONIDs(t *testing.T) {
	if string(RankTargetSelf) != "self" {
		t.Errorf("RankTargetSelf = %q, want \"self\"", RankTargetSelf)
	}
	if string(RankTargetOpponent) != "target" {
		t.Errorf("RankTargetOpponent = %q, want \"target\"", RankTargetOpponent)
	}
}

// --- 決定2: ダメージ計算は Move.Effect を読まない -----------------------------

// TestCalcDamageIgnoresMoveEffect は、追加効果の有無でダメージ計算の結果が完全に一致することを確かめる。
// これが崩れると testdata/golden の期待値が動く(CLAUDE.md 絶対ルール3)。
func TestCalcDamageIgnoresMoveEffect(t *testing.T) {
	effects := []*MoveEffect{
		nil,
		{Chance: 100, Target: RankTargetSelf, Stages: map[StatKey]int{StatSpe: 1}},     // ニトロチャージ相当
		{Chance: 100, Target: RankTargetSelf, Stages: map[StatKey]int{StatSpA: -2}},    // りゅうせいぐん相当
		{Chance: 10, Target: RankTargetSelf, Stages: map[StatKey]int{StatAtk: 1}},      // メタルクロー相当
		{Chance: 20, Target: RankTargetOpponent, Stages: map[StatKey]int{StatDef: -1}}, // かみくだく相当
	}

	// 補正を一通り効かせた入力(追加効果の有無が他の補正と干渉しないことも見る)。
	base := ctrlInput([]Type{TypeFire}, []Type{TypeGrass}, CategoryPhysical, TypeFire)
	base.Field.Weather = WeatherSun
	base.Field.Terrain = TerrainElectric
	base.Attacker.Ranks = Ranks{Atk: 2}
	base.Defender.Ranks = Ranks{Def: 1}

	var want DamageResult
	for i, e := range effects {
		in := base
		in.Move.Effect = e
		got, err := calcDamage(in)
		if err != nil {
			t.Fatalf("effects[%d]: CalcDamage: %v", i, err)
		}
		if i == 0 {
			want = got
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("effects[%d]: 追加効果でダメージ計算の結果が変わった\n got = %+v\nwant = %+v", i, got, want)
		}
	}
}

// TestCalcDamageAcceptsInvalidMoveEffect は、追加効果が不正でも CalcDamage が拒否しないことを固定する。
// 追加効果はダメージ計算の入力ではないので、検証はマスタの写像(services/internal/master)の責務にする
// (ADR-0107 決定2)。ここで弾くと、計算だけしたい呼び出し側がマスタの品質に引きずられる。
func TestCalcDamageAcceptsInvalidMoveEffect(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	want, err := calcDamage(in)
	if err != nil {
		t.Fatalf("CalcDamage: %v", err)
	}
	in.Move.Effect = &MoveEffect{Chance: 999, Target: RankTarget("nowhere"), Stages: map[StatKey]int{StatHP: 99}}
	got, err := calcDamage(in)
	if err != nil {
		t.Fatalf("追加効果が不正でも CalcDamage はエラーにしない想定: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("結果が変わった\n got = %+v\nwant = %+v", got, want)
	}
}
