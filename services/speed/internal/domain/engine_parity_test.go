package domain

import (
	"testing"

	"example.com/pokecalc/engine"
)

// TestSpeedMatchesEngineFinalSpeed は、素早さ画面の値が連続技(POST /api/calc/combo)の行動順と同じ式で出ることを守る
// (I-speed-4 / ADR-0610 §3。式は engine.FinalSpeed〈ADR-0179 §2〉を正とし、speed に新しい式を作らない)。
// こだわりスカーフは engine では「SpeedMods: [{always, 6144}] を持つ持ち物」として渡す(ADR-0179 §2)。
func TestSpeedMatchesEngineFinalSpeed(t *testing.T) {
	scarf := &engine.Item{ID: "test-scarf", Effect: &engine.ItemEffect{
		SpeedMods: []engine.SpeedMod{{Condition: engine.SpeedConditionAlways, Modifier: scarfSpeedModifier}},
	}}
	natures := []NatureEffect{NatureMinus, NatureNeutral, NaturePlus}
	for _, base := range []int{1, 2, 45, 73, 100, 101, 130, 200, 255} {
		for _, sp := range []int{0, 1, 16, 31, 32} {
			for _, n := range natures {
				for rank := minRank; rank <= maxRank; rank++ {
					for mask := 0; mask < 8; mask++ {
						in := Input{BaseSpeed: base, SP: sp, Nature: n, Rank: rank,
							Scarf: mask&1 != 0, Tailwind: mask&2 != 0, Paralysis: mask&4 != 0}
						got, err := Speed(in)
						if err != nil {
							t.Fatalf("Speed(%+v) error = %v", in, err)
						}
						nature, err := toEngineNature(n)
						if err != nil {
							t.Fatal(err)
						}
						ind := engine.Individual{
							Species: engine.Species{BaseStats: engine.Stats{Spe: base}},
							Nature:  nature,
							SP:      engine.Stats{Spe: sp},
							Ranks:   engine.Ranks{Spe: rank},
						}
						if in.Scarf {
							ind.Item = scarf
						}
						if in.Paralysis {
							ind.Status = engine.StatusParalysis
						}
						want, err := engine.FinalSpeed(ind, engine.Field{}, engine.SpeedSide{Tailwind: in.Tailwind})
						if err != nil {
							t.Fatalf("engine.FinalSpeed(%+v) error = %v", in, err)
						}
						if got != want.Speed {
							t.Errorf("Speed(%+v) = %d, engine.FinalSpeed = %d", in, got, want.Speed)
						}
					}
				}
			}
		}
	}
}
