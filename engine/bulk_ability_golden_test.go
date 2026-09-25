//go:build golden

package engine

// issue #272 / ADR-0126 §検証: 一括計算に渡した防御側の特性が、外部実装(@smogon/calc 0.12.0 Champions)の
// 期待値どおりに効くことを、fixed.json の「防御側に効果つきの特性があるケース」で照合する。
//
// 各ケースの防御側(SP・性格・持ち物・特性)を1件のプリセット + 持ち物バリアント + 特性の候補として CalcBulk に渡し、
// その特性の行が oracle の rolls・KO と一致することを見る。候補には効果の無い特性も並べ、行が分かれても
// 取り違えないことを同時に確かめる。ゴールデンのファイルは1バイトも変えない(新しいベクタは作らない)。

import (
	"math"
	"testing"
)

func TestGoldenBulkDefenderAbilityMatchesOracle(t *testing.T) {
	chart := mustTypeChart(t)
	checked := 0
	for _, c := range readGoldenFixed(t) {
		def := c.Input.Defender
		// 一括計算の防御側は Level50・状態なし・ランク0・テラスなしで組み立てる(ADR-0009)。それ以外のケースは対象外。
		if def.Ability.Effect == nil || def.Ability.ID == "" || def.Level != DefaultLevel || def.Status != StatusNone || def.Ranks != (Ranks{}) || (def.TeraType != "" && def.TeraType != TypeNone) {
			continue
		}
		plain := Ability{ID: def.Ability.ID + "-plain-control"}
		in := BulkInput{
			Format:            c.Input.Format,
			Attacker:          c.Input.Attacker,
			DefenderSpecies:   def.Species,
			Move:              c.Input.Move,
			Field:             c.Input.Field,
			Critical:          c.Input.Critical,
			TypeChart:         chart,
			Presets:           []DefenderPreset{{Key: "golden", SP: def.SP, Nature: def.Nature}},
			ItemVariants:      []*Item{def.Item},
			DefenderAbilities: []Ability{plain, def.Ability},
		}
		res, err := CalcBulk(in)
		if err != nil {
			t.Errorf("%s: CalcBulk: %v", c.ID, err)
			continue
		}
		var row *BulkRow
		for i := range res.Rows {
			for _, id := range res.Rows[i].AbilityIDs {
				if id == def.Ability.ID {
					row = &res.Rows[i]
				}
			}
		}
		if row == nil {
			t.Errorf("%s: 特性 %q の行が無い", c.ID, def.Ability.ID)
			continue
		}
		ko := c.Expected.KO
		if row.Result.Rolls != c.Expected.Rolls || row.Result.KO.Hits != ko.Hits || row.Result.KO.Guaranteed != ko.Guaranteed || math.Abs(row.Result.KO.ChancePercent-ko.ChancePercent) > 1e-9 {
			t.Errorf("%s: 一括計算の行(特性 %s)が oracle と違う rolls=%v want %v KO=%+v want %+v",
				c.ID, def.Ability.ID, row.Result.Rolls, c.Expected.Rolls, row.Result.KO, ko)
		}
		checked++
	}
	// fixed.json には防御側の特性のケースが 60 件以上ある(ADR-0106 の無効・吸収と、軽減の特性)。
	// 絞り込みの条件が壊れて照合が空振りするのを防ぐ。
	if checked < 30 {
		t.Fatalf("照合したケースが %d 件しかない(フィルタが広すぎる / fixed.json の構成が変わった)", checked)
	}
	t.Logf("防御側の特性つきケース %d 件を一括計算で照合した", checked)
}
