//go:build golden

package engine

// 技の機構の段階3(ADR-0144 §7)のゴールデン: testdata/golden/mechanisms-stage3.json を
// @smogon/calc 0.12.0(Champions 世代)の期待値と照合する。対戦の状態(残り HP・多段の回数)は oracle の
// Pokemon の curHP・Move の hits に渡したものを DamageInput.State に載せる。技の処理は testdata/golden/effects.json の
// moveRules(写し)の定義をベクタの入力(Move.Rule)に載せたもの。既存のゴールデンのファイルは変えない。

import (
	"encoding/json"
	"math"
	"slices"
	"strings"
	"testing"
)

const goldenMechanismsStage3File = "mechanisms-stage3.json"

// goldenMechanismStage3Labels は mechanisms-stage3.json に最低1件ずつ要るラベル(ADR-0144 §7 の表)。
var goldenMechanismStage3Labels = []string{
	"hp-attacker-scaled", "hp-attacker-low", "hp-defender-ratio", "hp-fixed", "hp-ko",
	"hits", "fling", "fling-none", "category-physical", "category-special", "grounded-item",
}

// goldenStage3RuleOptionalLabels は技の処理の定義を持たない(通常の技・多段の技・持ち物の効果だけの)ベクタのラベル。
var goldenStage3RuleOptionalLabels = []string{"hp-ko", "hits", "grounded-item"}

// isStage3Rule は段階3の語彙(ADR-0144 §1)を使う定義か。段階2のゴールデンの網羅はこの技を数えない
// (mechanisms-stage3.json が網羅を見る)。
func isStage3Rule(r MoveRule) bool {
	switch r.PowerFormula {
	case PowerFormulaAttackerHPScaled, PowerFormulaAttackerHPLow, PowerFormulaDefenderHPRatio, PowerFormulaAttackerItemFling:
		return true
	}
	return r.FixedDamageFormula != "" || r.CategoryByStats
}

// isStage3RuleJSON は写しの moveRules の1行が段階3の語彙を使うか。
func isStage3RuleJSON(t *testing.T, raw json.RawMessage) bool {
	t.Helper()
	var r MoveRule
	if err := decodeStrictGolden(raw, &r); err != nil {
		t.Fatalf("moveRules: %v", err)
	}
	return isStage3Rule(r)
}

// oracleLacksFixedDamage は oracle(0.12.0)が計算しない固定ダメージの式(いかりのまえば型・がむしゃら型は威力 0 として
// ダメージ 0 を返す)。この式の技はゴールデンの網羅から外し、engine の単体テスト(move_rule_stage3_test.go)が
// ゲームの規則(Showdown の damageCallback)どおりかを見る(ADR-0144 §7)。
func oracleLacksFixedDamage(r MoveRule) bool {
	return r.FixedDamageFormula == FixedDamageDefenderHalfHP || r.FixedDamageFormula == FixedDamageDefenderMinusAttackerHP
}

type goldenMechanismStage3Case struct {
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

func (c *goldenMechanismStage3Case) UnmarshalJSON(b []byte) error {
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

func readGoldenMechanismStage3Cases(t *testing.T) []goldenMechanismStage3Case {
	t.Helper()
	meta := readGoldenMetadata(t)
	if _, ok := meta.Files[goldenMechanismsStage3File]; !ok {
		t.Fatalf("metadata.json の files に %s が無い(ADR-0144 §7。make golden-generate)", goldenMechanismsStage3File)
	}
	var cases []goldenMechanismStage3Case
	if err := json.Unmarshal(goldenFile(t, meta, goldenMechanismsStage3File), &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != meta.Files[goldenMechanismsStage3File].Count {
		t.Fatalf("件数 = %d, metadata = %d", len(cases), meta.Files[goldenMechanismsStage3File].Count)
	}
	return cases
}

func TestGoldenMechanismsStage3(t *testing.T) {
	meta := readGoldenMetadata(t)
	if o := meta.oracle(t, goldenChampionsOracleID); !slices.Contains(o.Files, goldenMechanismsStage3File) {
		t.Fatalf("%s は Champions の oracle のファイルとして記録する: %v", goldenMechanismsStage3File, o.Files)
	}
	cases := readGoldenMechanismStage3Cases(t)
	rules := goldenMoveRules(t)
	chart := mustTypeChart(t)
	seen := map[string]int{}
	movesSeen := map[string]int{}
	for _, v := range cases {
		label := scenarioLabel(v.ID)
		seen[label]++
		movesSeen[v.Oracle.Move]++
		t.Run(v.ID, func(t *testing.T) {
			if v.Input.Move.Rule == nil {
				if !slices.Contains(goldenStage3RuleOptionalLabels, label) {
					t.Fatalf("ラベル %q のベクタの技に処理の定義(Move.Rule)が無い", label)
				}
			} else {
				raw, ok := rules[v.Oracle.Move]
				if !ok {
					t.Fatalf("oracle の技 %q が moveRules に無い", v.Oracle.Move)
				}
				var want MoveRule
				if err := decodeStrictGolden(raw, &want); err != nil {
					t.Fatalf("moveRules[%q]: %v", v.Oracle.Move, err)
				}
				if !goldenRulesEqual(*v.Input.Move.Rule, want) {
					t.Fatalf("ベクタの Rule = %+v, want moveRules[%q] = %+v", *v.Input.Move.Rule, v.Oracle.Move, want)
				}
			}
			// 状態の入力はベクタの中身で決まる(ラベルと入力の対応)。
			switch label {
			case "hp-ko":
				if v.Input.State.DefenderCurrentHP == 0 {
					t.Fatal("hp-ko のベクタに防御側の残り HP が無い")
				}
			case "hits":
				if v.Input.State.Hits == 0 {
					t.Fatal("hits のベクタに回数が無い")
				}
			case "hp-attacker-scaled", "hp-attacker-low", "hp-fixed":
				if v.Input.State.AttackerCurrentHP == 0 {
					t.Fatal("攻撃側の残り HP が無い")
				}
			case "hp-defender-ratio":
				if v.Input.State.DefenderCurrentHP == 0 {
					t.Fatal("防御側の残り HP が無い")
				}
			}
			v.Input.TypeChart = chart
			got, err := CalcDamage(v.Input)
			if err != nil {
				t.Fatal(err)
			}
			if got.Unsupported != nil {
				t.Errorf("段階3の技に印が残った: %v", got.Unsupported)
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
	for _, l := range goldenMechanismStage3Labels {
		if seen[l] == 0 {
			t.Errorf("ラベル %q のベクタが無い(ADR-0144 §7)。あるラベル: %s", l, strings.Join(sortedLabelKeys(seen), ", "))
		}
	}
	// 段階3の語彙の技は、oracle が計算しない固定ダメージの式を除いて全技に1件以上のベクタ。
	for name, raw := range rules {
		var r MoveRule
		if err := decodeStrictGolden(raw, &r); err != nil {
			t.Fatalf("moveRules[%q]: %v", name, err)
		}
		if !isStage3Rule(r) || oracleLacksFixedDamage(r) {
			continue
		}
		if movesSeen[name] == 0 {
			t.Errorf("moveRules の段階3の技 %q のベクタが無い(ADR-0144 §7 (a))", name)
		}
	}
}

// goldenRulesEqual は2つの定義が同じか(map・スライスの nil と空を区別しない比較は要らない: どちらも写しから厳格に読む)。
func goldenRulesEqual(a, b MoveRule) bool {
	ja, err1 := json.Marshal(a)
	jb, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && string(ja) == string(jb)
}

// 状態の入力の値域: 残り HP は 1..最大(oracle は curHP 0 を「満タン」と読むので 0 を渡さない。省略は満タン)、
// 回数は範囲の多段技の最小..最大。ベクタの生成器がこの値域の外を作らないこと。
func TestGoldenMechanismsStage3StateRanges(t *testing.T) {
	n := 0
	for _, v := range readGoldenMechanismStage3Cases(t) {
		s := v.Input.State
		a, d := RealStats(v.Input.Attacker).HP, RealStats(v.Input.Defender).HP
		if s.AttackerCurrentHP < 0 || s.AttackerCurrentHP > a || s.DefenderCurrentHP < 0 || s.DefenderCurrentHP > d {
			t.Errorf("%s: 残り HP が値域外(攻撃側 %d/%d・防御側 %d/%d)", v.ID, s.AttackerCurrentHP, a, s.DefenderCurrentHP, d)
		}
		if s.Hits != 0 {
			n++
			mh := v.Input.Move.Params.MultiHit
			if mh == nil || mh.Min == mh.Max || s.Hits < mh.Min || s.Hits > mh.Max {
				t.Errorf("%s: 回数 %d が範囲の多段の値域外(%+v)", v.ID, s.Hits, mh)
			}
		}
	}
	if n == 0 {
		t.Error("回数を指定したベクタが無い")
	}
}
