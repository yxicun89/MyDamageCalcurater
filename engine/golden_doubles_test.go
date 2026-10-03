//go:build golden

package engine

// issue #232 案B のダブル分・#288・ADR-0222: ダブルのゴールデン(doubles.json・doubles-random.jsonl.gz)の照合範囲を守る。
// 値の一致そのものは TestGoldenDamage が全件で見る。ここが守るのは次のこと:
//   - ADR-0222 §2 の各項目(壁・全体技・その組合せ・シングルの対照)が1件以上ある(生成器の取りこぼしを検出する)
//   - 形式 double と技の対象は doubles* だけに現れる(既存のベクタが黙って別の入力に変わっていない)
//   - ランダム部分が各層に十分な件数を持つ(弱い生成で照合が薄くならない。絶対ルール6)
// テラスタルはポケモンチャンピオンズに無いので照合しない(ADR-0222 §1)。ダブルのベクタもテラス無し。

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const (
	goldenDoublesFile         = "doubles.json"
	goldenDoublesRandomFile   = "doubles-random.jsonl.gz"
	goldenDoublesPrefix       = "doubles/"
	goldenDoublesRandomPrefix = "doubles-random/"
	// goldenDoublesRandomMin はランダム部分の件数の下限(生成器は 3000 件)。
	goldenDoublesRandomMin = 3000
)

// goldenDoublesRequiredPrefixes は doubles.json に1件以上なければならないラベルの接頭辞(ADR-0222 §2 の表の各行)。
var goldenDoublesRequiredPrefixes = []string{
	"doubles/single-target/no-screen",
	"doubles/screen/reflect",
	"doubles/screen/light-screen",
	"doubles/screen/aurora-veil",
	"doubles/screen/reflect/critical",
	"doubles/spread/all-adjacent/",
	"doubles/spread/all-adjacent-foes/",
	"doubles/spread/rain",
	"doubles/spread/critical",
	"doubles/spread/reflect",
	"doubles/spread/immune",
	"doubles/single-control/spread/",
}

func TestGoldenDoublesCoverage(t *testing.T) {
	meta := readGoldenMetadata(t)
	fixed := readGoldenDamageCases(t, meta, goldenDoublesFile)
	random := readGoldenDamageCases(t, meta, goldenDoublesRandomFile)

	for _, p := range goldenDoublesRequiredPrefixes {
		n := 0
		for _, v := range fixed {
			if strings.HasPrefix(v.ID, p) {
				n++
			}
		}
		if n == 0 {
			t.Errorf("%s に %q のケースが無い(tools/golden/generate.mjs の doubleCase を確認)", goldenDoublesFile, p)
		}
	}
	for _, v := range fixed {
		if !strings.HasPrefix(v.ID, goldenDoublesPrefix) {
			t.Errorf("%s の ID %q が %q で始まらない", goldenDoublesFile, v.ID, goldenDoublesPrefix)
		}
	}
	for _, v := range random {
		if !strings.HasPrefix(v.ID, goldenDoublesRandomPrefix) {
			t.Errorf("%s の ID %q が %q で始まらない", goldenDoublesRandomFile, v.ID, goldenDoublesRandomPrefix)
		}
	}

	if len(random) < goldenDoublesRandomMin {
		t.Errorf("%s が %d 件(下限 %d)", goldenDoublesRandomFile, len(random), goldenDoublesRandomMin)
	}
	screened := func(v goldenCase) bool {
		s := v.Input.Field.DefenderScreens
		return s.Reflect || s.LightScreen || s.AuroraVeil
	}
	for _, l := range []struct {
		name string
		min  int
		pred func(goldenCase) bool
	}{
		{"double × 全体技", 200, func(v goldenCase) bool {
			return v.Input.Format == FormatDouble && v.Input.Move.Target == MoveTargetSpread
		}},
		{"double × 壁", 200, func(v goldenCase) bool { return v.Input.Format == FormatDouble && screened(v) }},
		{"double × 単体技", 200, func(v goldenCase) bool {
			return v.Input.Format == FormatDouble && v.Input.Move.Target == MoveTargetSingle
		}},
		{"single × 全体技", 200, func(v goldenCase) bool {
			return v.Input.Format == FormatSingle && v.Input.Move.Target == MoveTargetSpread
		}},
		{"double × 全体技 × 壁", 50, func(v goldenCase) bool {
			return v.Input.Format == FormatDouble && v.Input.Move.Target == MoveTargetSpread && screened(v)
		}},
		{"double × 全体技 × 急所", 20, func(v goldenCase) bool {
			return v.Input.Format == FormatDouble && v.Input.Move.Target == MoveTargetSpread && v.Input.Critical
		}},
	} {
		n := 0
		for _, v := range random {
			if l.pred(v) {
				n++
			}
		}
		if n < l.min {
			t.Errorf("%s の層「%s」が %d 件(下限 %d)", goldenDoublesRandomFile, l.name, n, l.min)
		}
	}

	// doubles* のベクタはすべて技の対象を持ち(oracle の move.target から生成。空=不明は使わない)、テラスは持たない。
	for name, cases := range map[string][]goldenCase{goldenDoublesFile: fixed, goldenDoublesRandomFile: random} {
		for _, v := range cases {
			in := v.Input
			if in.Move.Target != MoveTargetSingle && in.Move.Target != MoveTargetSpread {
				t.Errorf("%s %s: Move.Target=%q(single / spread のどちらか)", name, v.ID, in.Move.Target)
			}
			if in.Format != FormatSingle && in.Format != FormatDouble {
				t.Errorf("%s %s: Format=%q", name, v.ID, in.Format)
			}
			if in.Attacker.TeraType != TypeNone || in.Defender.TeraType != TypeNone {
				t.Errorf("%s %s: テラスを持つ(ポケモンチャンピオンズにテラスタルは無い。ADR-0222)", name, v.ID)
			}
		}
	}
}

// 形式 double と技の対象は doubles* 以外のベクタに現れない(既存ファイルのバイト列は ADR-0222 の再生成で不変)。
func TestGoldenDoublesFieldsOnlyInDoublesFiles(t *testing.T) {
	meta := readGoldenMetadata(t)
	for _, name := range []string{"fixed.json", "random.jsonl.gz", "attack-species.jsonl.gz", "defense-species.jsonl.gz", goldenLegacyEffectsFile} {
		bad := 0
		for _, v := range readGoldenDamageCases(t, meta, name) {
			if in := v.Input; in.Format != FormatSingle || in.Move.Target != "" {
				bad++
				if bad <= 5 {
					t.Errorf("%s %s: 形式・技の対象が doubles* 以外に現れた(Format=%q Target=%q)", name, v.ID, in.Format, in.Move.Target)
				}
			}
		}
	}
}

// metadata: doubles* は Champions oracle に属し、ランダム部分の seed と除外の理由(ADR-0222)が記録されている。
func TestGoldenDoublesMetadata(t *testing.T) {
	data, err := os.ReadFile("../testdata/golden/metadata.json")
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Exclusions []goldenExclusion `json:"exclusions"`
		Oracles    []struct {
			ID                string   `json:"id"`
			Files             []string `json:"files"`
			DoublesRandomSeed *uint32  `json:"doublesRandomSeed"`
		} `json:"oracles"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, o := range raw.Oracles {
		if o.ID != goldenChampionsOracleID {
			continue
		}
		found = true
		if o.DoublesRandomSeed == nil || *o.DoublesRandomSeed == 0 {
			t.Errorf("champions oracle に doublesRandomSeed が無い(doubles-random の再現に必要)")
		}
		if !sameStringSet(o.Files, goldenChampionsFiles) {
			t.Errorf("champions oracle の files=%v want %v", o.Files, goldenChampionsFiles)
		}
	}
	if !found {
		t.Fatal("metadata に champions oracle が無い")
	}
	ok := false
	for _, e := range raw.Exclusions {
		if e.Scope == "battle" && strings.Contains(e.Reason, "ADR-0222") {
			ok = true
		}
	}
	if !ok {
		t.Error("exclusions(scope=battle)の理由が ADR-0222 を指していない")
	}
}
