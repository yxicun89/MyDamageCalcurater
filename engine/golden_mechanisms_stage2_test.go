//go:build golden

package engine

// 技の機構の段階2(ADR-0143 §7)のゴールデン: testdata/golden/mechanisms-stage2.json を
// @smogon/calc 0.12.0(Champions 世代)の期待値と照合する。技の処理は testdata/golden/effects.json の moveRules
// (data/importer/effects.json の写し。ADR-0118)の定義をベクタの入力(Move.Rule)に載せたもの。

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

const goldenMechanismsStage2File = "mechanisms-stage2.json"

// goldenMechanismStage2Labels は mechanisms-stage2.json に最低1件ずつ要るラベル(ADR-0143 §7 の表)。
var goldenMechanismStage2Labels = []string{
	"status-attacker", "status-defender", "status-none",
	"item-none", "item-held", "item-removable", "item-failed",
	"weather",
	"terrain-attacker", "terrain-defender", "terrain-airborne", "terrain-spread", "priority-terrain",
	"ranks",
	"speed-ratio", "speed-inverse", "speed-paralysis", "speed-ability", "speed-item",
	"effectiveness", "screens-broken", "hit-index", "resolved",
	"weight-target", "weight-ratio", "weight-ability",
}

type goldenMechanismStage2Case struct {
	ID     string
	Oracle struct {
		Move string `json:"move"`
	}
	Input    DamageInput
	Expected struct {
		Rolls         [16]int   `json:"rolls"`
		HitRolls      [][16]int `json:"hitRolls"`
		AttackerStats Stats     `json:"attackerStats"`
		DefenderStats Stats     `json:"defenderStats"`
		KO            KOChance  `json:"ko"`
	}
}

func (c *goldenMechanismStage2Case) UnmarshalJSON(b []byte) error {
	var raw struct {
		ID       string          `json:"id"`
		Oracle   json.RawMessage `json:"oracle"`
		Input    json.RawMessage `json:"input"`
		Expected json.RawMessage `json:"expected"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	c.ID = raw.ID
	if err := json.Unmarshal(raw.Oracle, &c.Oracle); err != nil {
		return err
	}
	if err := decodeStrictGolden(raw.Input, &c.Input); err != nil {
		return err
	}
	return json.Unmarshal(raw.Expected, &c.Expected)
}

// readGoldenMechanismStage2Cases は mechanisms-stage2.json のベクタを読む(特性・持ち物の照合の網羅を数えるテストが使う)。
func readGoldenMechanismStage2Cases(t *testing.T) []goldenMechanismStage2Case {
	t.Helper()
	var cases []goldenMechanismStage2Case
	if err := json.Unmarshal(goldenFile(t, readGoldenMetadata(t), goldenMechanismsStage2File), &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

// goldenMoveRules は testdata/golden/effects.json の moveRules(キーは oracle の技名)。
func goldenMoveRules(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile("../testdata/golden/effects.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		MoveRules map[string]json.RawMessage `json:"moveRules"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.MoveRules) == 0 {
		t.Fatal("testdata/golden/effects.json に moveRules が無い(ADR-0143 §4)")
	}
	return f.MoveRules
}

func TestGoldenMechanismsStage2(t *testing.T) {
	meta := readGoldenMetadata(t)
	if _, ok := meta.Files[goldenMechanismsStage2File]; !ok {
		t.Fatalf("metadata.json の files に %s が無い(ADR-0143 §7。make golden-generate)", goldenMechanismsStage2File)
	}
	if o := meta.oracle(t, goldenChampionsOracleID); !slices.Contains(o.Files, goldenMechanismsStage2File) {
		t.Fatalf("%s は Champions の oracle のファイルとして記録する: %v", goldenMechanismsStage2File, o.Files)
	}
	var cases []goldenMechanismStage2Case
	if err := json.Unmarshal(goldenFile(t, meta, goldenMechanismsStage2File), &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != meta.Files[goldenMechanismsStage2File].Count {
		t.Fatalf("件数 = %d, metadata = %d", len(cases), meta.Files[goldenMechanismsStage2File].Count)
	}

	rules := goldenMoveRules(t)
	chart := mustTypeChart(t)
	seen := map[string]int{}
	movesSeen := map[string]int{}
	for _, v := range cases {
		seen[scenarioLabel(v.ID)]++
		movesSeen[v.Oracle.Move]++
		t.Run(v.ID, func(t *testing.T) {
			if v.Input.Move.Rule == nil {
				t.Fatal("ベクタの技に処理の定義(Move.Rule)が無い")
			}
			// ベクタの定義は写しの moveRules と同じ(生成器が写しから載せる)。
			raw, ok := rules[v.Oracle.Move]
			if !ok {
				t.Fatalf("oracle の技 %q が moveRules に無い", v.Oracle.Move)
			}
			var want MoveRule
			if err := decodeStrictGolden(raw, &want); err != nil {
				t.Fatalf("moveRules[%q]: %v", v.Oracle.Move, err)
			}
			if !reflect.DeepEqual(*v.Input.Move.Rule, want) {
				t.Fatalf("ベクタの Rule = %+v, want moveRules[%q] = %+v", *v.Input.Move.Rule, v.Oracle.Move, want)
			}
			v.Input.TypeChart = chart
			got, err := CalcDamage(v.Input)
			if err != nil {
				t.Fatal(err)
			}
			if got.Unsupported != nil {
				t.Errorf("段階2の技に印が残った: %v", got.Unsupported)
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
	for _, l := range goldenMechanismStage2Labels {
		if seen[l] == 0 {
			t.Errorf("ラベル %q のベクタが無い(ADR-0143 §7)。あるラベル: %s", l, strings.Join(sortedLabelKeys(seen), ", "))
		}
	}
	// moveRules の全技に1件以上のベクタ(定義だけで照合していない技を残さない)。
	for _, name := range sortedLabelKeys(func() map[string]int {
		m := map[string]int{}
		for k := range rules {
			m[k] = 1
		}
		return m
	}()) {
		if movesSeen[name] == 0 {
			// 段階3の語彙の技(ADR-0144)は mechanisms-stage3.json の網羅(TestGoldenMechanismsStage3)が見る。
			if isStage3RuleJSON(t, rules[name]) {
				continue
			}
			t.Errorf("moveRules の技 %q のベクタが無い(ADR-0143 §7 (a))", name)
		}
	}
}

// 重さの比のベクタは、比がちょうど整数(2〜5)になる組を使わない(oracle は kg の浮動小数で割り、実機と境界で違う。ADR-0143)。
func TestGoldenMechanismsStage2AvoidsWeightRatioBoundaries(t *testing.T) {
	meta := readGoldenMetadata(t)
	var cases []goldenMechanismStage2Case
	if err := json.Unmarshal(goldenFile(t, meta, goldenMechanismsStage2File), &cases); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, v := range cases {
		r := v.Input.Move.Rule
		if r == nil || r.PowerFormula != PowerFormulaWeightRatio {
			continue
		}
		n++
		a, d := v.Input.Attacker.Species.WeightHg, v.Input.Defender.Species.WeightHg
		if a <= 0 || d <= 0 {
			t.Errorf("%s: 重さが入力に無い(攻撃側 %d・防御側 %d)", v.ID, a, d)
			continue
		}
		// 特性の重さの補正があるベクタは補正後の値で比べる。
		a, d = goldenEffectiveWeight(v.Input.Attacker), goldenEffectiveWeight(v.Input.Defender)
		for k := 2; k <= 5; k++ {
			if a == d*k {
				t.Errorf("%s: 比がちょうど %d(攻撃側 %d・防御側 %d)。境界を避ける", v.ID, k, a, d)
			}
		}
	}
	if n == 0 {
		t.Error("重さの比のベクタが無い")
	}
}

// goldenEffectiveWeight は特性の重さの補正を掛けた重さ(hg)。テストの独立な計算(max(1, trunc(hg × mod / 4096)))。
func goldenEffectiveWeight(in Individual) int {
	w := in.Species.WeightHg
	if e := in.Ability.Effect; e != nil && e.WeightMod != 0 {
		w = max(1, w*e.WeightMod/4096)
	}
	return w
}
