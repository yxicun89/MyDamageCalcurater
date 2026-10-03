//go:build golden

package engine

// ADR-0224: テラスタル(オプションの機能)のゴールデン(tera.json・tera-random.jsonl.gz)の照合範囲を守る。
// 値の一致そのものは TestGoldenDamage が全件で見る。ここが守るのは次のこと:
//   - ADR-0224 §1 の表 T1〜T11 の各行と、18 タイプの総当たりが1件以上ある(生成器の取りこぼしを検出する)
//   - テラスは tera* だけに現れ、tera* はシングルだけ(既存ファイルはバイト不変。ダブルとの組合せは対象外)
//   - ランダム部分が各層に十分な件数を持つ(弱い生成で照合が薄くならない。絶対ルール6)
//   - 印: 攻撃側のテラスの印は付かず、防御側のテラスを持つベクタには防御側のテラスの印が付く(ADR-0224 §3)

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const (
	goldenTeraFile         = "tera.json"
	goldenTeraRandomFile   = "tera-random.jsonl.gz"
	goldenTeraPrefix       = "tera/"
	goldenTeraRandomPrefix = "tera-random/"
	// goldenTeraRandomMin はランダム部分の件数の下限(生成器は 3000 件)。
	goldenTeraRandomMin = 3000
)

// goldenTeraRequiredPrefixes は tera.json に1件以上なければならないラベルの接頭辞(ADR-0224 §1 の表の各行)。
var goldenTeraRequiredPrefixes = []string{
	"tera/t1/", "tera/t2/", "tera/t3/", "tera/t4/",
	"tera/t5/adaptability/tera-is-original", "tera/t5/adaptability/tera-only",
	"tera/t5/adaptability/original-move-other-tera", "tera/t5/adaptability/other-original-move-dual",
	"tera/t6/", "tera/t7/", "tera/t8/", "tera/t9/", "tera/t10/", "tera/t11/",
	"tera/combo/",
}

func TestGoldenTeraCoverage(t *testing.T) {
	meta := readGoldenMetadata(t)
	fixed := readGoldenDamageCases(t, meta, goldenTeraFile)
	random := readGoldenDamageCases(t, meta, goldenTeraRandomFile)

	for _, p := range goldenTeraRequiredPrefixes {
		n := 0
		for _, v := range fixed {
			if strings.HasPrefix(v.ID, p) {
				n++
			}
		}
		if n == 0 {
			t.Errorf("%s に %q のケースが無い(tools/golden/generate.mjs の teraCase を確認)", goldenTeraFile, p)
		}
	}
	// 総当たり: 18 タイプそれぞれが攻撃側・防御側・両側に現れる(相性表の全タイプ)。
	chart := mustTypeChart(t)
	for _, ty := range chart.Types() {
		for _, side := range []string{"atk", "def", "both"} {
			p := "tera/all/" + side + "/" + string(ty) + "/"
			found := false
			for _, v := range fixed {
				if strings.HasPrefix(v.ID, p) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s に %q(総当たり)が無い", goldenTeraFile, p)
			}
		}
	}
	for _, v := range fixed {
		if !strings.HasPrefix(v.ID, goldenTeraPrefix) {
			t.Errorf("%s の ID %q が %q で始まらない", goldenTeraFile, v.ID, goldenTeraPrefix)
		}
		if v.Input.Attacker.TeraType == TypeNone && v.Input.Defender.TeraType == TypeNone {
			t.Errorf("%s %s: テラスが無い(対照は生成器の中だけで使い、ファイルには出さない)", goldenTeraFile, v.ID)
		}
	}
	for _, v := range random {
		if !strings.HasPrefix(v.ID, goldenTeraRandomPrefix) {
			t.Errorf("%s の ID %q が %q で始まらない", goldenTeraRandomFile, v.ID, goldenTeraRandomPrefix)
		}
	}

	if len(random) < goldenTeraRandomMin {
		t.Errorf("%s が %d 件(下限 %d)", goldenTeraRandomFile, len(random), goldenTeraRandomMin)
	}
	has := func(in Individual, ty Type) bool {
		for _, x := range in.Species.Types {
			if x == ty {
				return true
			}
		}
		return false
	}
	adapt := func(v goldenCase) bool { return v.Input.Attacker.Ability.ID == "Adaptability" }
	for _, l := range []struct {
		name string
		min  int
		pred func(goldenCase) bool
	}{
		{"攻撃側テラス=技=元タイプ", 200, func(v goldenCase) bool {
			a := v.Input.Attacker
			return a.TeraType != TypeNone && a.TeraType == v.Input.Move.Type && has(a, a.TeraType)
		}},
		{"攻撃側テラス=技(元タイプでない)", 200, func(v goldenCase) bool {
			a := v.Input.Attacker
			return a.TeraType != TypeNone && a.TeraType == v.Input.Move.Type && !has(a, a.TeraType)
		}},
		{"攻撃側テラス≠技・技=元タイプ", 200, func(v goldenCase) bool {
			a := v.Input.Attacker
			return a.TeraType != TypeNone && a.TeraType != v.Input.Move.Type && has(a, v.Input.Move.Type)
		}},
		{"てきおうりょく × 攻撃側テラス", 300, func(v goldenCase) bool {
			return v.Input.Attacker.TeraType != TypeNone && adapt(v)
		}},
		{"攻撃側テラス × フィールド", 300, func(v goldenCase) bool {
			return v.Input.Attacker.TeraType != TypeNone && v.Input.Field.Terrain != TerrainNone
		}},
		{"防御側テラス × すなあらし/ゆき", 200, func(v goldenCase) bool {
			w := v.Input.Field.Weather
			return v.Input.Defender.TeraType != TypeNone && (w == WeatherSand || w == WeatherSnow)
		}},
		{"防御側テラス × ミスト/サイコ", 200, func(v goldenCase) bool {
			f := v.Input.Field.Terrain
			return v.Input.Defender.TeraType != TypeNone && (f == TerrainMisty || f == TerrainPsychic)
		}},
		{"両側テラス", 500, func(v goldenCase) bool {
			return v.Input.Attacker.TeraType != TypeNone && v.Input.Defender.TeraType != TypeNone
		}},
	} {
		n := 0
		for _, v := range random {
			if l.pred(v) {
				n++
			}
		}
		if n < l.min {
			t.Errorf("%s の層「%s」が %d 件(下限 %d)", goldenTeraRandomFile, l.name, n, l.min)
		}
	}

	// tera* はシングルだけで、技の対象を持たない(ダブルとの組合せは対象外。ユーザー決定 2026-10-03)。
	for name, cases := range map[string][]goldenCase{goldenTeraFile: fixed, goldenTeraRandomFile: random} {
		for _, v := range cases {
			if in := v.Input; in.Format != FormatSingle || in.Move.Target != "" {
				t.Errorf("%s %s: Format=%q Target=%q(シングル・対象なしだけ)", name, v.ID, in.Format, in.Move.Target)
			}
		}
	}
}

// ADR-0224 §3: tera* の全ベクタで、攻撃側のテラスの印は付かず、防御側のテラスを持つベクタには
// 防御側のテラスの印が(その ID で)付く。テラス以外の印(形式)も付かない。
func TestGoldenTeraMarks(t *testing.T) {
	meta := readGoldenMetadata(t)
	chart := mustTypeChart(t)
	for _, name := range []string{goldenTeraFile, goldenTeraRandomFile} {
		bad := 0
		for _, v := range readGoldenDamageCases(t, meta, name) {
			in := v.Input
			in.TypeChart = chart
			res, err := CalcDamage(in)
			if err != nil {
				t.Fatalf("%s %s: %v", name, v.ID, err)
			}
			var defMark *UnsupportedMark
			for i, m := range res.Unsupported {
				switch m.Target {
				case UnsupportedTargetAttackerTeraType, UnsupportedTargetFormat:
					bad++
					if bad <= 5 {
						t.Errorf("%s %s: 付かないはずの印 %v", name, v.ID, m)
					}
				case UnsupportedTargetDefenderTeraType:
					defMark = &res.Unsupported[i]
				}
			}
			wantDef := in.Defender.TeraType != TypeNone
			if (defMark != nil) != wantDef || (defMark != nil && defMark.ID != string(in.Defender.TeraType)) {
				bad++
				if bad <= 5 {
					t.Errorf("%s %s: 防御側のテラス %q に対する印 %v", name, v.ID, in.Defender.TeraType, defMark)
				}
			}
		}
		if bad > 5 {
			t.Errorf("%s: ほか %d 件", name, bad-5)
		}
	}
}

// metadata: tera* は Champions oracle に属し、ランダム部分の seed と除外の理由(ADR-0224)が記録されている。
func TestGoldenTeraMetadata(t *testing.T) {
	data, err := os.ReadFile("../testdata/golden/metadata.json")
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Exclusions []goldenExclusion `json:"exclusions"`
		Oracles    []struct {
			ID             string   `json:"id"`
			Files          []string `json:"files"`
			TeraRandomSeed *uint32  `json:"teraRandomSeed"`
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
		if o.TeraRandomSeed == nil || *o.TeraRandomSeed == 0 {
			t.Error("champions oracle に teraRandomSeed が無い(tera-random の再現に必要)")
		}
		for _, f := range []string{goldenTeraFile, goldenTeraRandomFile} {
			ok := false
			for _, g := range o.Files {
				ok = ok || g == f
			}
			if !ok {
				t.Errorf("champions oracle の files に %s が無い", f)
			}
		}
	}
	if !found {
		t.Fatal("metadata に champions oracle が無い")
	}
	ok := false
	for _, e := range raw.Exclusions {
		if e.Scope == "battle" && strings.Contains(e.Reason, "ADR-0224") {
			ok = true
		}
	}
	if !ok {
		t.Error("exclusions(scope=battle)の理由が ADR-0224 を指していない")
	}
}
