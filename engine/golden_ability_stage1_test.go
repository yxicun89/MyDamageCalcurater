//go:build golden

package engine

// ADR-0176: 特性の段階1の効果の項目を持つ定義(testdata/golden/effects.json)は、生成器が oracle と
// 「効く」(effects/<id>/.../apply)・「効かない対照」(effects/<id>/.../control)の組で照合していること。
// Breakable を持つ定義は、防御側の特性を無視する攻撃側には効かないこと(effects/<id>/.../breakable)も照合していること。
// 値そのものの一致は TestGoldenDamage が fixed.json 全件で見る。ここが守るのは「照合の対象に入っているか」
// (TestGoldenCoversEveryChampionsEffect と同じ考え方。段階1の項目は既存の一覧に無いのでここで足す)。

import (
	"encoding/json"
	"strings"
	"testing"
)

// goldenStage1Fields は、生成器が効く/対照の組を作る段階1の項目。
var goldenStage1Fields = []string{
	"TypeConvert", "PowerMods", "AuraType", "StatMods", "SeparateStatMods", "CritDamageMod",
	"PreventsCritical", "IgnoresOpponentRanks", "IgnoresDefenderAbility",
}

func TestGoldenCoversStage1AbilityEffects(t *testing.T) {
	effects := readGoldenEffects(t)
	var ids []string
	for _, c := range readGoldenFixed(t) {
		ids = append(ids, c.ID)
	}
	has := func(prefix, suffix string) bool {
		for _, id := range ids {
			if strings.HasPrefix(id, prefix) && strings.HasSuffix(id, suffix) {
				return true
			}
		}
		return false
	}

	checked := 0
	for _, name := range sortedStrings(effects.Abilities) {
		def := effects.Abilities[name]
		prefix := "effects/" + goldenEffectSlug(name) + "/"
		usesStage1 := false
		for _, f := range goldenStage1Fields {
			if _, ok := def[f]; ok {
				usesStage1 = true
			}
		}
		if usesStage1 {
			checked++
			if !has(prefix, "/apply") {
				t.Errorf("%s: 効くケース(%s…/apply)が fixed.json に無い", name, prefix)
			}
			if !has(prefix, "/control") {
				t.Errorf("%s: 効かない対照(%s…/control)が fixed.json に無い", name, prefix)
			}
		}
		if raw, ok := def["Breakable"]; ok {
			var b bool
			if err := json.Unmarshal(raw, &b); err != nil || !b {
				t.Errorf("%s: Breakable は true だけ: %s", name, raw)
			}
			// 無効・吸収の特性は既存の命名(<特性名のハイフン区切り>/breakable。ADR-0106 の immunityCases)に合わせる。
			immunitySlug := strings.ToLower(strings.ReplaceAll(name, " ", "-")) + "/"
			if !has(prefix, "/breakable") && !has(immunitySlug, "/breakable") {
				t.Errorf("%s: 防御側の特性を無視する攻撃側に効かないケース(%s…/breakable)が fixed.json に無い", name, prefix)
			}
			if isUnsupportedOnly(def) {
				t.Errorf("%s: 段階1では未対応の印の定義に Breakable を付けない(ADR-0176)", name)
			}
		}
	}
	if checked == 0 {
		t.Fatal("段階1の項目を持つ定義が0件(effects.json の読み方か、データの移行を確認する)")
	}
}

// 段階1で計算に入れた特性は、未対応の印を持たない(印を外す条件。ADR-0176 §5)。
func TestGoldenStage1AbilitiesAreNotMarkedUnsupported(t *testing.T) {
	effects := readGoldenEffects(t)
	for _, name := range sortedStrings(effects.Abilities) {
		def := effects.Abilities[name]
		stage1 := false
		for _, f := range goldenStage1Fields {
			if _, ok := def[f]; ok {
				stage1 = true
			}
		}
		if !stage1 {
			continue
		}
		for k := range goldenUnsupportedFields {
			if _, ok := def[k]; ok {
				t.Errorf("%s: 段階1の効果と未対応の印 %s を同時に持つ", name, k)
			}
		}
	}
}
