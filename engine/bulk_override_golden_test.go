//go:build golden

package engine

// issue #274/#272 の残り / ADR-0216 §検証: 一括計算の防御側ランクの上書き(BulkInput.DefenderOverride)が、
// 外部実装(@smogon/calc 0.12.0 Champions)の期待値どおりに効くことを、既存のゴールデン
// (fixed.json・random.jsonl.gz の「防御側にランクがあるケース」)で照合する。
//
// 各ケースの防御側(SP・性格・持ち物・特性)を1件のプリセット + 持ち物バリアント + 特性の候補として CalcBulk に渡し、
// ランク・状態異常は DefenderOverride で当てる。ゴールデンのファイルは1バイトも変えない(新しいベクタは作らない)。
// 上書きを外すと一致しなくなるケースがあることも確かめ、上書きが実際に使われていることを保証する。

import (
	"math"
	"testing"
)

func TestGoldenBulkDefenderOverrideMatchesOracle(t *testing.T) {
	chart := mustTypeChart(t)
	meta := readGoldenMetadata(t)
	totalControlDiffers := 0
	for _, name := range []string{"fixed.json", "random.jsonl.gz"} {
		checked, controlDiffers := 0, 0
		for _, c := range readGoldenDamageCases(t, meta, name) {
			def := c.Input.Defender
			// 一括計算の防御側は Level50・テラスなしで組み立てる(ADR-0009)。ランクが無いケースは上書きの照合にならない。
			if def.Ranks == (Ranks{}) || (def.Level != 0 && def.Level != DefaultLevel) || (def.TeraType != "" && def.TeraType != TypeNone) {
				continue
			}
			if def.Ability.ID == "" && def.Ability.Effect != nil {
				continue // ID の無い効果つき特性は候補として渡せない
			}
			build := func(ov DefenderOverride) BulkInput {
				in := BulkInput{
					Format:           c.Input.Format,
					Attacker:         c.Input.Attacker,
					DefenderSpecies:  def.Species,
					Move:             c.Input.Move,
					Field:            c.Input.Field,
					Critical:         c.Input.Critical,
					TypeChart:        chart,
					Presets:          []DefenderPreset{{Key: "golden", SP: def.SP, Nature: def.Nature}},
					ItemVariants:     []*Item{def.Item},
					DefenderOverride: ov,
				}
				if def.Ability.ID != "" {
					in.DefenderAbilities = []Ability{def.Ability}
				}
				return in
			}
			res, err := CalcBulk(build(DefenderOverride{Ranks: def.Ranks, Status: def.Status}))
			if err != nil {
				t.Errorf("%s %s: CalcBulk: %v", name, c.ID, err)
				continue
			}
			if len(res.Rows) != 1 {
				t.Errorf("%s %s: 行数=%d want 1", name, c.ID, len(res.Rows))
				continue
			}
			row := res.Rows[0]
			ko := c.Expected.KO
			if row.Result.Rolls != c.Expected.Rolls || row.Result.KO.Hits != ko.Hits || row.Result.KO.Guaranteed != ko.Guaranteed || math.Abs(row.Result.KO.ChancePercent-ko.ChancePercent) > 1e-9 {
				t.Errorf("%s %s: 上書きつき一括計算の行が oracle と違う rolls=%v want %v KO=%+v want %+v",
					name, c.ID, row.Result.Rolls, c.Expected.Rolls, row.Result.KO, ko)
			}
			checked++

			control, err := CalcBulk(build(DefenderOverride{}))
			if err != nil {
				t.Errorf("%s %s: 対照の CalcBulk: %v", name, c.ID, err)
				continue
			}
			if control.Rows[0].Result.Rolls != c.Expected.Rolls {
				controlDiffers++
			}
		}
		// fixed.json は「急所・ランク・壁」の6件、random.jsonl.gz は大半が防御側ランクを持つ。
		if checked < 6 {
			t.Errorf("%s: 照合したケースが %d 件しかない(防御側ランクのケースを拾えていない)", name, checked)
		}
		totalControlDiffers += controlDiffers
		t.Logf("%s: 照合 %d 件・上書きなしで不一致 %d 件", name, checked, controlDiffers)
	}
	// fixed.json の6件は急所で防御側の正のランクが無視されるため対照と一致しうる。合計で見る。
	if totalControlDiffers == 0 {
		t.Errorf("上書きを外しても全件一致した(上書きが計算に使われていない疑い)")
	}
}
