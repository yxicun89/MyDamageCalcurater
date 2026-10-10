//go:build golden

package engine

// issue #270 / ADR-0120: 効果定義(testdata/golden/effects.json)の持ち物・特性が、1種ずつ
// oracle(@smogon/calc 0.12.0 Champions)と照合されていることを確かめる。
//
// 値そのものの一致は TestGoldenDamage が fixed.json 全件で見る。ここが守るのは「照合の対象に
// 入っているか」で、定義を足したのにベクタを足し忘れると、その効果は黙って未検証のまま残る。
//   - タイプ・相性で効く効果(BoostType / ResistBerryType / OffBoostType / DefResistType /
//     ReduceSuperEffective)は、生成器が効果ごとに「効く」(effects/<id>/.../apply)と
//     「効かない対照」(effects/<id>/.../control)を作る。両方があること。
//     効く/効かないが oracle でも本当にそうなっていること(効果なしとのダメージの差)は生成器が確かめる。
//   - それ以外の効果(実数値・最終ダメージ・無効/吸収など)は既存の固定シナリオで照合している。
//     fixed.json に1件以上あること。
//   - gen9 でだけ照合する legacy 効果(metadata の legacyEffects)は対象外。
//   - 「未対応」の印だけの定義(UnsupportedAttacker / UnsupportedDefender。ADR-0123)は補正を持たないので対象外で、
//     ベクタで使われていないことだけを見る。印の側(攻撃側/防御側)が oracle と合うことは生成器が確かめる。

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// goldenTypedEffectFields は、生成器が効く/対照の組を作る効果のフィールド。
var goldenTypedEffectFields = []string{"BoostType", "ResistBerryType", "OffBoostType", "DefResistType", "ReduceSuperEffective", "WeightMod", "Grounds"}

// goldenEffectPairPrefix は効く/対照の組のベクタの id の接頭辞。持ち物による接地(Grounds。ADR-0144)は
// mechanisms-stage3.json の grounded-item/<id>/…/apply と …/control、それ以外は effects/<id>/…。
func goldenEffectPairPrefix(name string, def map[string]json.RawMessage) string {
	if _, ok := def["Grounds"]; ok {
		return "grounded-item/" + goldenEffectSlug(name) + "/"
	}
	return "effects/" + goldenEffectSlug(name) + "/"
}

type goldenEffectsFile struct {
	Items     map[string]map[string]json.RawMessage `json:"items"`
	Abilities map[string]map[string]json.RawMessage `json:"abilities"`
	// 素早さの効果定義(ADR-0139)。キーの名前だけ使う。
	SpeedItems     map[string]json.RawMessage `json:"speedItems"`
	SpeedAbilities map[string]json.RawMessage `json:"speedAbilities"`
}

func readGoldenEffects(t *testing.T) goldenEffectsFile {
	t.Helper()
	raw, err := os.ReadFile("../testdata/golden/effects.json")
	if err != nil {
		t.Fatalf("testdata/golden/effects.json: %v", err)
	}
	var f goldenEffectsFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("testdata/golden/effects.json: %v", err)
	}
	return f
}

// goldenEffectSlug は生成器の id() と同じ(小文字化して英数字以外を落とす)。
func goldenEffectSlug(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func hasTypedEffect(def map[string]json.RawMessage) bool {
	for _, f := range goldenTypedEffectFields {
		if _, ok := def[f]; ok {
			return true
		}
	}
	return false
}

// goldenUnsupportedFields は「未対応」の印(ADR-0123)。これだけを持つ定義は補正を持たない。
var goldenUnsupportedFields = map[string]bool{"UnsupportedAttacker": true, "UnsupportedDefender": true}

func isUnsupportedOnly(def map[string]json.RawMessage) bool {
	for k := range def {
		if !goldenUnsupportedFields[k] {
			return false
		}
	}
	return len(def) > 0
}

func TestGoldenCoversEveryChampionsEffect(t *testing.T) {
	meta := readGoldenMetadata(t)
	legacy := map[string]bool{}
	for _, o := range meta.Oracles {
		if o.LegacyEffects == nil {
			continue
		}
		for _, n := range o.LegacyEffects.Items {
			legacy[n] = true
		}
		for _, n := range o.LegacyEffects.Abilities {
			legacy[n] = true
		}
	}
	effects := readGoldenEffects(t)
	cases := readGoldenFixed(t)

	used := map[string]int{}
	var ids []string
	for _, c := range cases {
		items, abilities := caseEffects(c)
		for _, n := range append(items, abilities...) {
			used[n]++
		}
		ids = append(ids, c.ID)
	}
	// 技の機構と組み合わさって効く特性(スキルリンク等。ADR-0142)は mechanisms.json で照合する。
	for _, c := range readGoldenMechanismCases(t) {
		items, abilities := caseEffects(goldenCase{Input: c.Input})
		for _, n := range append(items, abilities...) {
			used[n]++
		}
		ids = append(ids, c.ID)
	}
	// 技の機構の段階2(重さの補正の特性。ADR-0143)は mechanisms-stage2.json で照合する。素早さの補正を持つ持ち物
	// (speedItems の名前。防御側の印だけの定義のくろいてっきゅうを攻撃側が持つ)だけは「印だけの定義を使っていない」の
	// 検査に数えず、網羅(1件以上ある)にだけ数える。それ以外は fixed.json・mechanisms.json と同じく数える。
	speedNames := map[string]bool{}
	for _, m := range []map[string]json.RawMessage{effects.SpeedItems, effects.SpeedAbilities} {
		for n := range m {
			speedNames[n] = true
		}
	}
	usedStage2 := map[string]int{}
	for _, c := range readGoldenMechanismStage2Cases(t) {
		items, abilities := caseEffects(goldenCase{Input: c.Input})
		for _, n := range append(items, abilities...) {
			usedStage2[n]++
			if !speedNames[n] {
				used[n]++
			}
		}
		ids = append(ids, c.ID)
	}
	// 技の機構の段階3(持ち物による接地。ADR-0144)は mechanisms-stage3.json で照合する。
	for _, c := range readGoldenMechanismStage3Cases(t) {
		items, abilities := caseEffects(goldenCase{Input: c.Input})
		for _, n := range append(items, abilities...) {
			usedStage2[n]++
			if !speedNames[n] {
				used[n]++
			}
		}
		ids = append(ids, c.ID)
	}
	hasID := func(prefix, suffix string) bool {
		for _, id := range ids {
			if strings.HasPrefix(id, prefix) && strings.HasSuffix(id, suffix) {
				return true
			}
		}
		return false
	}

	checked := 0
	for _, group := range []map[string]map[string]json.RawMessage{effects.Items, effects.Abilities} {
		for _, name := range sortedStrings(group) {
			if legacy[name] {
				continue
			}
			// 未対応の印だけの定義(ADR-0123)は補正を持たないので照合の対象外。ベクタで使っていないこと
			// (印の側は生成器が oracle と照合する)。
			if isUnsupportedOnly(group[name]) {
				if used[name] != 0 {
					t.Errorf("%s: 未対応の印だけの定義が fixed.json で使われている", name)
				}
				continue
			}
			checked++
			if used[name]+usedStage2[name] == 0 {
				t.Errorf("%s: fixed.json・mechanisms.json・mechanisms-stage2.json・mechanisms-stage3.json に1件も無い(tools/golden/generate.mjs で照合されていない)", name)
				continue
			}
			if !hasTypedEffect(group[name]) {
				continue
			}
			prefix := goldenEffectPairPrefix(name, group[name])
			if !hasID(prefix, "/apply") {
				t.Errorf("%s: 効くケース(%s…/apply)が fixed.json・mechanisms-stage3.json に無い", name, prefix)
			}
			if !hasID(prefix, "/control") {
				t.Errorf("%s: 効かない対照(%s…/control)が fixed.json・mechanisms-stage3.json に無い", name, prefix)
			}
		}
	}
	if checked == 0 {
		t.Fatal("照合対象の効果が0件(effects.json か metadata の読み方を確認する)")
	}
}
