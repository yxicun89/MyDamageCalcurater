//go:build golden

package engine

// ADR-0178: 技のフラグに依存する特性(段階2)。
//
// 受け入れ条件(ゴールデン):
//   - AC-G1 対象の 14 特性(下の goldenStage2Abilities)は effects.json で段階2の項目を持つ定義になっていて、未対応の印を持たない
//     (印を外す条件。tools/golden/unsupported-effects.json からも外れている = 生成器の網羅の検査が通る)。
//   - AC-G2 段階2の項目を持つ定義は、生成器が oracle と「効く」(…/apply)・「効かない対照」(…/control)の組で照合していて、
//     Breakable なら防御側の特性を無視する攻撃側に効かないこと(…/breakable)も照合している。
//   - AC-G3 それらのベクタは oracle の技のフラグを engine に渡している(Move.FlagsKnown が真)。
// 値そのものの一致は TestGoldenDamage が fixed.json 全件で見る。

import (
	"strings"
	"testing"
)

// goldenStage2Fields は、生成器が効く/対照の組を作る段階2の項目(PowerMods は move_flag の条件のときだけ段階2)。
var goldenStage2Fields = []string{
	"PostAuraPowerMods", "FlagTypeConvert", "DefImmuneFlags", "DefFinalModsByFlag", "DefFinalModsByType", "NoContact",
}

// goldenStage2Abilities は段階2で計算に入れる特性(effects.json の名前。ADR-0178 §5 の表)。
var goldenStage2Abilities = []string{
	"Aura Guard", "Bulletproof", "Fluffy", "Iron Fist", "Liquid Voice", "Long Reach", "Mega Launcher",
	"Punk Rock", "Reckless", "Sharpness", "Sheer Force", "Soundproof", "Strong Jaw", "Tough Claws",
}

func usesStage2(def map[string][]byte) bool {
	for _, f := range goldenStage2Fields {
		if _, ok := def[f]; ok {
			return true
		}
	}
	if raw, ok := def["PowerMods"]; ok && strings.Contains(string(raw), `"move_flag"`) {
		return true
	}
	return false
}

func rawDef[T ~[]byte](def map[string]T) map[string][]byte {
	out := make(map[string][]byte, len(def))
	for k, v := range def {
		out[k] = []byte(v)
	}
	return out
}

func TestGoldenStage2AbilitiesDefined(t *testing.T) {
	effects := readGoldenEffects(t)
	for _, name := range goldenStage2Abilities {
		def, ok := effects.Abilities[name]
		if !ok {
			t.Errorf("%s: effects.json に定義が無い", name)
			continue
		}
		if !usesStage2(rawDef(def)) {
			t.Errorf("%s: 段階2の項目(%v か PowerMods の move_flag)を持たない: %v", name, goldenStage2Fields, sortedStrings(def))
		}
		for k := range goldenUnsupportedFields {
			if _, ok := def[k]; ok {
				t.Errorf("%s: 段階2の効果と未対応の印 %s を同時に持つ(計算に入れたら印を外す。ADR-0178 §5)", name, k)
			}
		}
	}
}

func TestGoldenCoversStage2AbilityEffects(t *testing.T) {
	effects := readGoldenEffects(t)
	fixed := readGoldenFixed(t)

	checked := 0
	for _, name := range sortedStrings(effects.Abilities) {
		def := effects.Abilities[name]
		if !usesStage2(rawDef(def)) {
			continue
		}
		checked++
		prefix := "effects/" + goldenEffectSlug(name) + "/"
		var apply, control, breakable int
		for _, c := range fixed {
			if !strings.HasPrefix(c.ID, prefix) {
				continue
			}
			switch {
			case strings.HasSuffix(c.ID, "/apply"):
				apply++
			case strings.HasSuffix(c.ID, "/control"):
				control++
			case strings.HasSuffix(c.ID, "/breakable"):
				breakable++
			default:
				continue
			}
			if !c.Input.Move.FlagsKnown {
				t.Errorf("%s: 技のフラグを渡していない(FlagsKnown が偽。生成器の moveFlags)", c.ID)
			}
		}
		if apply == 0 {
			t.Errorf("%s: 効くケース(%s…/apply)が fixed.json に無い", name, prefix)
		}
		if control == 0 {
			t.Errorf("%s: 効かない対照(%s…/control)が fixed.json に無い", name, prefix)
		}
		if _, ok := def["Breakable"]; ok && breakable == 0 {
			t.Errorf("%s: 防御側の特性を無視する攻撃側に効かないケース(%s…/breakable)が fixed.json に無い", name, prefix)
		}
	}
	if checked < len(goldenStage2Abilities) {
		t.Fatalf("段階2の項目を持つ定義が %d 件(want >= %d。effects.json の移行を確認する)", checked, len(goldenStage2Abilities))
	}
}
