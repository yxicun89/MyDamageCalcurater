//go:build golden

package engine

import "testing"

// issue #232 / ADR-0160 AC-7: テラス・ダブルの印は数値を変えないので、ゴールデンは全件そのまま一致する。
// その前提として、ゴールデンのベクタ(oracle の対象範囲。metadata.json の "No double/tera")には
// テラス指定もダブルも無く、印(テラス・形式)が1件も付かないことを固定する。
// ここが崩れたら、oracle がテラス・ダブルを計算した期待値を engine の「未適用」の数値と照合している。
func TestGoldenInputsHaveNoTeraOrDouble(t *testing.T) {
	meta := readGoldenMetadata(t)
	chart := mustTypeChart(t)
	files := append(append([]string(nil), goldenChampionsDamageFiles...), goldenLegacyEffectsFile)
	for _, name := range files {
		bad := 0
		for _, v := range readGoldenDamageCases(t, meta, name) {
			in := v.Input
			if in.Format != FormatSingle || in.Attacker.TeraType != TypeNone || in.Defender.TeraType != TypeNone {
				bad++
				if bad <= 5 {
					t.Errorf("%s %s: format=%q attackerTera=%q defenderTera=%q(ゴールデンはシングル・テラスなしだけ)",
						name, v.ID, in.Format, in.Attacker.TeraType, in.Defender.TeraType)
				}
				continue
			}
			in.TypeChart = chart
			res, err := CalcDamage(in)
			if err != nil {
				t.Fatalf("%s %s: %v", name, v.ID, err)
			}
			for _, m := range res.Unsupported {
				if m.Target == "attacker_tera_type" || m.Target == "defender_tera_type" || m.Target == "format" {
					bad++
					if bad <= 5 {
						t.Errorf("%s %s: テラス・形式の印が付いた: %v", name, v.ID, m)
					}
				}
			}
		}
		if bad > 5 {
			t.Errorf("%s: ほか %d 件", name, bad-5)
		}
	}
}
