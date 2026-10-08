//go:build golden

package engine

// 技の機構の段階1(ADR-0142 §9)のゴールデン: testdata/golden/mechanisms.json を
// @smogon/calc 0.12.0(Champions 世代)の期待値と照合する。多段技は1発ごとの16段階(hitRolls)も照合する。

import (
	"encoding/json"
	"math"
	"slices"
	"strings"
	"testing"
)

const goldenMechanismsFile = "mechanisms.json"

// goldenMechanismLabels は mechanisms.json に最低1件ずつ要るラベル(ADR-0142 §9 の表)。
var goldenMechanismLabels = []string{
	"multi-hit-fixed", "multi-hit-range", "multi-hit-skill-link", "multi-hit-ten",
	"always-crit", "always-crit-shell-armor", "ignore-defense-ranks",
	"alt-offense-def", "alt-offense-target", "alt-defense-def",
	"fixed-damage-level", "fixed-damage-immune",
}

// stage1Mechanisms は段階1で engine が計算する機構。ベクタはこれ以外の機構を持たない(印が残る技は照合しない)。
var stage1Mechanisms = []MoveMechanism{
	MechanismMultiHit, MechanismFixedDamage, MechanismOHKO, MechanismAlwaysCrit,
	MechanismIgnoreDefenseRanks, MechanismAltOffenseStat, MechanismAltDefenseStat,
}

type goldenMechanismCase struct {
	ID       string
	Input    DamageInput
	Expected struct {
		Rolls         [16]int   `json:"rolls"`
		HitRolls      [][16]int `json:"hitRolls"`
		AttackerStats Stats     `json:"attackerStats"`
		DefenderStats Stats     `json:"defenderStats"`
		KO            KOChance  `json:"ko"`
	}
}

func (c *goldenMechanismCase) UnmarshalJSON(b []byte) error {
	var raw struct {
		ID       string          `json:"id"`
		Input    json.RawMessage `json:"input"`
		Expected json.RawMessage `json:"expected"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	c.ID = raw.ID
	if err := decodeStrictGolden(raw.Input, &c.Input); err != nil {
		return err
	}
	return json.Unmarshal(raw.Expected, &c.Expected)
}

func TestGoldenMechanismsStage1(t *testing.T) {
	meta := readGoldenMetadata(t)
	if _, ok := meta.Files[goldenMechanismsFile]; !ok {
		t.Fatalf("metadata.json の files に %s が無い(ADR-0142 §9。make golden-generate)", goldenMechanismsFile)
	}
	if o := meta.oracle(t, goldenChampionsOracleID); !slices.Contains(o.Files, goldenMechanismsFile) {
		t.Fatalf("%s は Champions の oracle のファイルとして記録する: %v", goldenMechanismsFile, o.Files)
	}
	var cases []goldenMechanismCase
	if err := json.Unmarshal(goldenFile(t, meta, goldenMechanismsFile), &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != meta.Files[goldenMechanismsFile].Count {
		t.Fatalf("件数 = %d, metadata = %d", len(cases), meta.Files[goldenMechanismsFile].Count)
	}

	seen := map[string]int{}
	for _, v := range cases {
		seen[scenarioLabel(v.ID)]++
		t.Run(v.ID, func(t *testing.T) {
			if len(v.Input.Move.Mechanisms) == 0 {
				t.Fatal("ベクタの技に機構が無い(段階1の機構を持つ技だけを入れる)")
			}
			for _, m := range v.Input.Move.Mechanisms {
				if !slices.Contains(stage1Mechanisms, m) {
					t.Fatalf("段階1外の機構 %q を持つ技は照合しない", m)
				}
			}
			got, err := CalcDamage(v.Input)
			if err != nil {
				t.Fatal(err)
			}
			if got.Unsupported != nil {
				t.Errorf("段階1の技に印が残った: %v", got.Unsupported)
			}
			if got.Rolls != v.Expected.Rolls {
				t.Errorf("rolls = %v, want %v", got.Rolls, v.Expected.Rolls)
			}
			wantHits := v.Expected.HitRolls
			if len(wantHits) == 0 {
				wantHits = nil
			}
			if len(got.HitRolls) != len(wantHits) {
				t.Fatalf("len(hitRolls) = %d, want %d", len(got.HitRolls), len(wantHits))
			}
			for h := range wantHits {
				if got.HitRolls[h] != wantHits[h] {
					t.Errorf("hitRolls[%d] = %v, want %v", h, got.HitRolls[h], wantHits[h])
				}
			}
			ko := v.Expected.KO
			if got.KO.Hits != ko.Hits || got.KO.Guaranteed != ko.Guaranteed || math.Abs(got.KO.ChancePercent-ko.ChancePercent) > 1e-9 {
				t.Errorf("KO = %+v, want %+v", got.KO, ko)
			}
			if a, d := RealStats(v.Input.Attacker), RealStats(v.Input.Defender); a != v.Expected.AttackerStats || d != v.Expected.DefenderStats {
				t.Errorf("stats = %+v / %+v, want %+v / %+v", a, d, v.Expected.AttackerStats, v.Expected.DefenderStats)
			}
		})
	}
	for _, l := range goldenMechanismLabels {
		if seen[l] == 0 {
			t.Errorf("ラベル %q のベクタが無い(ADR-0142 §9)。あるラベル: %s", l, strings.Join(sortedLabelKeys(seen), ", "))
		}
	}
}

func sortedLabelKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
