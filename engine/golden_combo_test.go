//go:build golden

package engine

// 連続技のゴールデン(ADR-0179 追記「CB-3」§7。docs/plan/combo-calc.md §5 AC-1〜AC-4・AC-9・AC-14・AC-20):
// testdata/golden/combo.jsonl.gz を @smogon/calc 0.12.0(Champions 世代)の期待値と照合する。
//
// - 各手の 16 段階(多段は 1 発ごと)は、tools/golden が手ごとの開始時のランクを oracle の Pokemon の boosts
//   (自分は攻撃側・相手は防御側)に渡して calculate() したもの。engine の CalcCombo の手の開始時のランク
//   (SelfStartRanks・OpponentStartRanks)がその boosts と一致し、16 段階が一致することを見る。
// - 各手の素早さは oracle の getFinalSpeed(手の開始時の boosts)。
// - 合計と倒せる確率は oracle に機能が無いので、tools/golden が oracle の 16 段階から 16^発数 通りを総当たりで数えた
//   koCount / koDenominator と比べる(Go の畳み込みと JS の総当たりの照合)。
// 既存のゴールデンのファイルは変えない(新しいファイルだけ。乱数も使わない)。

import (
	"encoding/json"
	"math"
	"slices"
	"testing"
)

const goldenComboFile = "combo.jsonl.gz"

// goldenComboLabels は combo.jsonl.gz に最低1件ずつ要るラベル(ADR-0179 追記 CB-3 §7 の表)。
var goldenComboLabels = []string{
	"uc3", "uc3-bulky", "uc2", "flame-charge", "close-combat", "rock-tomb", "multi-hit", "three-steps", "clamp",
}

// goldenComboAnchorID は combo-calc.md §5 の JSON の例(AC-1)と同じ入力のベクタ。oracle でも 132/256 になることを確かめる。
const goldenComboAnchorID = "uc3/mimikyu-adamant/garchomp-jolly"

type goldenComboStep struct {
	Boosts         Ranks     `json:"boosts"`
	DefenderBoosts Ranks     `json:"defenderBoosts"`
	Rolls          *[16]int  `json:"rolls"` // 変化技は null
	HitRolls       [][16]int `json:"hitRolls"`
	SelfSpeed      int       `json:"selfSpeed"`
	DefenderSpeed  int       `json:"defenderSpeed"`
}

type goldenComboCase struct {
	ID     string
	Oracle struct {
		Attacker string   `json:"attacker"`
		Defender string   `json:"defender"`
		Moves    []string `json:"moves"`
	}
	Input    ComboInput
	Expected struct {
		Steps         []goldenComboStep `json:"steps"`
		DefenderHP    int               `json:"defenderHp"`
		Total         [2]int            `json:"total"`
		KOCount       int64             `json:"koCount"`
		KODenominator int64             `json:"koDenominator"`
	}
}

func (c *goldenComboCase) UnmarshalJSON(b []byte) error {
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

func TestGoldenCombo(t *testing.T) {
	meta := readGoldenMetadata(t)
	if o := meta.oracle(t, goldenChampionsOracleID); !slices.Contains(o.Files, goldenComboFile) {
		t.Fatalf("%s は Champions の oracle のファイルとして記録する: %v", goldenComboFile, o.Files)
	}
	chart := mustTypeChart(t)
	seen := map[string]int{}
	anchor := false
	eachGoldenLine(t, meta, goldenComboFile, func(line []byte) {
		var v goldenComboCase
		if err := json.Unmarshal(line, &v); err != nil {
			t.Fatalf("combo: %v", err)
		}
		seen[scenarioLabel(v.ID)]++
		if v.Input.OpponentSpeed != OpponentSpeedFromDefender {
			t.Errorf("%s: ゴールデンは相手の素早さも照合するので from_defender: %q", v.ID, v.Input.OpponentSpeed)
		}
		if len(v.Input.Steps) == 0 || len(v.Input.Steps) > MaxComboSteps || len(v.Expected.Steps) != len(v.Input.Steps) {
			t.Fatalf("%s: 手の数 %d・期待値 %d", v.ID, len(v.Input.Steps), len(v.Expected.Steps))
		}
		in := v.Input
		in.TypeChart = chart
		res, err := CalcCombo(in)
		if err != nil {
			t.Fatalf("%s: %v", v.ID, err)
		}
		for i, want := range v.Expected.Steps {
			got := res.Steps[i]
			if got.SelfStartRanks != want.Boosts || got.OpponentStartRanks != want.DefenderBoosts {
				t.Errorf("%s 手 %d: 開始時のランク %+v / %+v, want oracle の boosts %+v / %+v", v.ID, i, got.SelfStartRanks, got.OpponentStartRanks, want.Boosts, want.DefenderBoosts)
			}
			if got.SelfSpeed != want.SelfSpeed || got.OpponentSpeed != want.DefenderSpeed {
				t.Errorf("%s 手 %d: 素早さ %d / %d, want oracle の getFinalSpeed %d / %d", v.ID, i, got.SelfSpeed, got.OpponentSpeed, want.SelfSpeed, want.DefenderSpeed)
			}
			switch {
			case want.Rolls == nil:
				if got.Damage != nil {
					t.Errorf("%s 手 %d: 変化技なのにダメージ %v", v.ID, i, got.Damage.Rolls)
				}
			case got.Damage == nil:
				t.Errorf("%s 手 %d: ダメージ nil, want %v", v.ID, i, *want.Rolls)
			default:
				if got.Damage.Rolls != *want.Rolls {
					t.Errorf("%s 手 %d: 16 段階 %v, want %v", v.ID, i, got.Damage.Rolls, *want.Rolls)
				}
				if len(want.HitRolls) > 0 && !slices.Equal(got.Damage.HitRolls, want.HitRolls) {
					t.Errorf("%s 手 %d: 1 発ごと %v, want %v", v.ID, i, got.Damage.HitRolls, want.HitRolls)
				}
			}
		}
		tot := res.Total
		if tot.DefenderHP != v.Expected.DefenderHP || tot.MinDamage != v.Expected.Total[0] || tot.MaxDamage != v.Expected.Total[1] {
			t.Errorf("%s: 合計 %d〜%d / HP %d, want %v / %d", v.ID, tot.MinDamage, tot.MaxDamage, tot.DefenderHP, v.Expected.Total, v.Expected.DefenderHP)
		}
		if v.Expected.KODenominator <= 0 || v.Expected.KOCount < 0 || v.Expected.KOCount > v.Expected.KODenominator {
			t.Fatalf("%s: koCount %d / koDenominator %d が不正", v.ID, v.Expected.KOCount, v.Expected.KODenominator)
		}
		want := float64(v.Expected.KOCount) / float64(v.Expected.KODenominator) * 100
		if math.Abs(tot.KO.ChancePercent-want) > 1e-9 || tot.KO.Guaranteed != (v.Expected.KOCount == v.Expected.KODenominator) {
			t.Errorf("%s: 倒せる確率 %+v, want %v%%(%d/%d)", v.ID, tot.KO, want, v.Expected.KOCount, v.Expected.KODenominator)
		}
		if v.ID == goldenComboAnchorID {
			anchor = true
			if v.Expected.KOCount != 132 || v.Expected.KODenominator != 256 || v.Expected.Total != [2]int{169, 201} || v.Expected.DefenderHP != 185 {
				t.Errorf("%s: oracle の値 %d/%d・%v・HP %d が combo-calc.md §5 の例(132/256・169〜201・185)と違う", v.ID,
					v.Expected.KOCount, v.Expected.KODenominator, v.Expected.Total, v.Expected.DefenderHP)
			}
		}
	})
	if !anchor {
		t.Errorf("%s が無い(combo-calc.md §5 の例。AC-1)", goldenComboAnchorID)
	}
	for _, l := range goldenComboLabels {
		if seen[l] == 0 {
			t.Errorf("ラベル %s のベクタが無い", l)
		}
	}
	for l := range seen {
		if !slices.Contains(goldenComboLabels, l) {
			t.Errorf("未知のラベル %s(goldenComboLabels と ADR-0179 追記 CB-3 §7 の表に足す)", l)
		}
	}
}
