package domain

// 調整の値域の検査(ADR-0250 §3)の境界値テスト。失敗はすべて invalid_input。

import (
	"testing"

	"example.com/pokecalc/engine"
)

func intp(v int) *int           { return &v }
func floatp(v float64) *float64 { return &v }

func TestCheckAdjustHitsBoundaries(t *testing.T) {
	for _, tt := range []struct {
		hits int
		ok   bool
	}{{0, false}, {1, true}, {engine.MaxAdjustHits, true}, {engine.MaxAdjustHits + 1, false}, {-1, false}} {
		err := CheckAdjustHits("hits", tt.hits)
		if tt.ok != (err == nil) || (!tt.ok && codeOf(t, err) != CodeInvalidInput) {
			t.Errorf("hits=%d: err = %v, want ok=%v", tt.hits, err, tt.ok)
		}
	}
}

// しきい値: 省略は 0(engine の既定)、明示は (0, 100]。
func TestAdjustThresholdBoundaries(t *testing.T) {
	if got, err := AdjustThreshold("thresholdPercent", nil); err != nil || got != 0 {
		t.Errorf("省略 = (%v, %v), want (0, nil)", got, err)
	}
	for _, tt := range []struct {
		p  float64
		ok bool
	}{{0, false}, {0.1, true}, {engine.DefaultAdjustThresholdPercent, true}, {engine.DefaultAdjustThresholdPercent + 0.1, false}, {-1, false}} {
		got, err := AdjustThreshold("thresholdPercent", floatp(tt.p))
		if tt.ok {
			if err != nil || got != tt.p {
				t.Errorf("p=%v: (%v, %v), want (%v, nil)", tt.p, got, err, tt.p)
			}
		} else if codeOf(t, err) != CodeInvalidInput {
			t.Errorf("p=%v: err = %v", tt.p, err)
		}
	}
}

// 上限: 省略した項目は MaxSPPerStat、明示は 0..MaxSPPerStat。
func TestAdjustCeilingBoundaries(t *testing.T) {
	full := engine.Stats{
		HP: engine.MaxSPPerStat, Atk: engine.MaxSPPerStat, Def: engine.MaxSPPerStat,
		SpA: engine.MaxSPPerStat, SpD: engine.MaxSPPerStat, Spe: engine.MaxSPPerStat,
	}
	if got, err := AdjustCeiling(nil); err != nil || got != full {
		t.Errorf("nil = (%+v, %v), want 全項目 %d", got, err, engine.MaxSPPerStat)
	}
	got, err := AdjustCeiling(&Ceiling{HP: intp(0), Spe: intp(engine.MaxSPPerStat)})
	want := full
	want.HP = 0
	if err != nil || got != want {
		t.Errorf("境界の内側 = (%+v, %v), want %+v", got, err, want)
	}
	for _, c := range []*Ceiling{{HP: intp(-1)}, {Spe: intp(engine.MaxSPPerStat + 1)}} {
		if _, err := AdjustCeiling(c); codeOf(t, err) != CodeInvalidInput {
			t.Errorf("%+v: err = %v", c, err)
		}
	}
	// 先頭の項目(hp)から順に見る。
	if _, err := AdjustCeiling(&Ceiling{HP: intp(-1), Atk: intp(-2)}); err == nil || err.Error() != "ceiling.hp は 0..32 でなければならない: -1" {
		t.Errorf("hp が先でない: %v", err)
	}
}

// 補正: 省略は 4096、明示は MinEffectModifier..MaxEffectModifier。
func TestAdjustModifierBoundaries(t *testing.T) {
	if got, err := AdjustModifier("modifier", nil); err != nil || got != engine.Modifier4096 {
		t.Errorf("省略 = (%v, %v), want (%d, nil)", got, err, engine.Modifier4096)
	}
	for _, tt := range []struct {
		v  int
		ok bool
	}{{engine.MinEffectModifier - 1, false}, {engine.MinEffectModifier, true}, {engine.MaxEffectModifier, true}, {engine.MaxEffectModifier + 1, false}} {
		got, err := AdjustModifier("modifier", intp(tt.v))
		if tt.ok {
			if err != nil || got != tt.v {
				t.Errorf("v=%d: (%v, %v)", tt.v, got, err)
			}
		} else if codeOf(t, err) != CodeInvalidInput {
			t.Errorf("v=%d: err = %v", tt.v, err)
		}
	}
}

func TestCheckAdjustGoalCountBoundaries(t *testing.T) {
	for _, tt := range []struct {
		n  int
		ok bool
	}{{0, false}, {1, true}, {engine.MaxSPGoals, true}, {engine.MaxSPGoals + 1, false}} {
		err := CheckAdjustGoalCount(tt.n)
		if tt.ok != (err == nil) || (!tt.ok && codeOf(t, err) != CodeInvalidInput) {
			t.Errorf("n=%d: err = %v, want ok=%v", tt.n, err, tt.ok)
		}
	}
}

// 素早さの目標: 省略は 0、0 以上は通り、負は invalid_input。
func TestAdjustMinSpeedBoundaries(t *testing.T) {
	if got, err := AdjustMinSpeed(nil); err != nil || got != 0 {
		t.Errorf("省略 = (%v, %v)", got, err)
	}
	if got, err := AdjustMinSpeed(intp(0)); err != nil || got != 0 {
		t.Errorf("0 = (%v, %v)", got, err)
	}
	if _, err := AdjustMinSpeed(intp(-1)); codeOf(t, err) != CodeInvalidInput {
		t.Errorf("-1: err = %v", err)
	}
}
