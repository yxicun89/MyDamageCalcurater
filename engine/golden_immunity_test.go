//go:build golden

package engine

// P2-3b / ADR-0106 §決定 8: 無効・吸収の特性を oracle(@smogon/calc 0.12.0 Champions)と照合する。
//
// ここが守るのは「ゴールデンに何が入っているか」で、値そのものの一致は TestGoldenDamage が見る。
//   - fixed.json に無効・吸収のベクタが入っていること(特性ごとに、無効・対照・組合せ)
//   - 無効のベクタの oracle の期待値が 16 ロールとも 0 であること(oracle がそう返すことの固定)
//   - 対照のベクタ(同じ特性・別タイプの技)が 0 でないこと(過剰適用の検出)
//   - fixed.json 以外のゴールデンがバイト単位で変わっていないこと
//     (= 無効・吸収の追加が既存の計算を一切動かしていない)

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

// readGoldenFixed は fixed.json のベクタを読む(golden_test.go の goldenFile を使う)。
func readGoldenFixed(t *testing.T) []goldenCase {
	t.Helper()
	meta := readGoldenMetadata(t)
	var cases []goldenCase
	if err := json.Unmarshal(goldenFile(t, meta, "fixed.json"), &cases); err != nil {
		t.Fatalf("fixed.json: %v", err)
	}
	return cases
}

// goldenImmunityAbilities は testdata/golden/effects.json に足す無効・吸収の特性と、
// その特性が 0 にする攻撃タイプ(@smogon/calc の表示名。ADR-0106 §oracle 1〜3 で確認した実装)。
//
// Dry Skin(炎の威力 ×1.25 を今の効果定義で表せない)と Storm Drain(Champions 世代に無い)は
// 入れない。ADR-0106 §限界 1・2。
var goldenImmunityAbilities = map[string]Type{
	"Levitate":      TypeGround,
	"Water Absorb":  TypeWater,
	"Volt Absorb":   TypeElectric,
	"Flash Fire":    TypeFire,
	"Sap Sipper":    TypeGrass,
	"Motor Drive":   TypeElectric,
	"Lightning Rod": TypeElectric,
	"Earth Eater":   TypeGround,
	"Eelevate":      TypeGround, // issue #270 / ADR-0120
}

// goldenImmunityMinPerAbility は、特性ごとに fixed.json に入っているべきベクタの最低件数。
// 内訳(ADR-0106 §決定 8): 無効 1・対照 1・組合せ(急所/壁/天候/ランク)1。
const goldenImmunityMinPerAbility = 3

// goldenUnchangedFiles は、この変更で1バイトも変わってはいけないゴールデンと、その sha256
// (P2-3b の作業開始時点。testdata/golden/metadata.json の files より)。
//
// 更新履歴: issue #231(ADR-0116)で random.jsonl.gz と legacy-effects.jsonl.gz を意図して
// 再生成した(地形ありのケースからひこうタイプを除外していたのを外し、接地判定を検証するため。
// 乱数列は同じで、種族プールだけが変わる)。値はその再生成後のもの。
//
// 無効・吸収の追加は fixed.json にベクタを足すだけで、Champions の random の特性プールも
// legacy-effects も相性表も触らない(ADR-0002 §決定 2・ADR-0106 §決定 8)。ここが変わったら、
// 生成器の乱数列が動いたか、既存の期待値を書き換えたかのどちらかで、どちらも P2-3b では誤り。
var goldenUnchangedFiles = map[string]string{
	"random.jsonl.gz":          "ac0120d1abc50267a837b41f4a6b102608f153f4609712ececbeda91fb7421b9",
	"attack-species.jsonl.gz":  "521f11af3cb8a57d3c03ffd933424a5ef25868a9d9be18fb708aa3d93d2e0339",
	"defense-species.jsonl.gz": "c28659dab4c9f6be0483582890f960a2ac710daa21979713d01a3db5d19b5507",
	"stats-species.jsonl.gz":   "587d63614467798da38a361c50f0963b95196600e6aa8572688e03386943fa85",
	"legacy-effects.jsonl.gz":  "00c950bbead7ce8e20351bcab731ac7944606419b78d33f924803342172df725",
	"typechart.json":           "962b20126fb1996c13b05de4533fa94f90266cb08cc5e9ab3d093a038efa2228",
}

// goldenFixedMinCount は fixed.json の最低件数。P2-3b 開始時点が 172 で、
// 特性 8 件 × 最低 3 ベクタを足すので 196 を下回らない(絶対ルール6: 件数を減らさない)。
var goldenFixedMinCount = 172 + len(goldenImmunityAbilities)*goldenImmunityMinPerAbility

func TestGoldenUntouchedFilesUnchanged(t *testing.T) {
	meta := readGoldenMetadata(t)
	for _, name := range sortedStrings(goldenUnchangedFiles) {
		f, ok := meta.Files[name]
		if !ok {
			t.Errorf("metadata.json の files に %s が無い", name)
			continue
		}
		if f.SHA256 != goldenUnchangedFiles[name] {
			t.Errorf("%s の sha256 が変わった: %s, want %s"+
				"(P2-3b は fixed.json にベクタを足すだけ。乱数列を動かしたか既存の期待値を書き換えた疑い。ADR-0106 §決定 8)",
				name, f.SHA256, goldenUnchangedFiles[name])
		}
	}
	if f, ok := meta.Files["fixed.json"]; !ok {
		t.Error("metadata.json の files に fixed.json が無い")
	} else if f.Count < goldenFixedMinCount {
		t.Errorf("fixed.json の件数 %d が下限 %d 未満(無効・吸収のベクタが生成されていない)", f.Count, goldenFixedMinCount)
	}
}

// TestGoldenCoversAbilityImmunity は fixed.json が特性ごとに
// 「無効(全ロール0)」「対照(0でない)」を持ち、無効のケースでは oracle も 0 を返していることを見る。
func TestGoldenCoversAbilityImmunity(t *testing.T) {
	cases := readGoldenFixed(t)

	type coverage struct{ nullified, control int }
	got := map[string]*coverage{}
	for name := range goldenImmunityAbilities {
		got[name] = &coverage{}
	}

	for _, c := range cases {
		name := c.Input.Defender.Ability.ID
		blocked, ok := goldenImmunityAbilities[name]
		if !ok {
			continue
		}
		eff := c.Input.Defender.Ability.Effect
		if eff == nil {
			t.Errorf("%s: 防御側の特性 %s に効果定義が無い(testdata/golden/effects.json に足す)", c.ID, name)
			continue
		}
		if !effectNullifies(eff, blocked) {
			t.Errorf("%s: %s の効果定義が %s を無効・吸収にしていない: %+v", c.ID, name, blocked, eff)
			continue
		}

		zero := c.Expected.Rolls == [16]int{}
		if c.Input.Move.Type == blocked {
			if !zero {
				t.Errorf("%s: %s が無効にする %s 技なのに oracle の期待値が 0 でない: %v",
					c.ID, name, blocked, c.Expected.Rolls)
			}
			if c.Expected.KO != (KOChance{}) {
				t.Errorf("%s: 無効なのに oracle の KO がゼロ値でない: %+v", c.ID, c.Expected.KO)
			}
			got[name].nullified++
			continue
		}
		// 対照: 同じ特性で別タイプの技。相性で 0 になるケースは対照に数えない。
		if zero {
			continue
		}
		got[name].control++
	}

	for _, name := range sortedStrings(goldenImmunityAbilities) {
		c := got[name]
		if c.nullified == 0 {
			t.Errorf("%s: 無効になるベクタが fixed.json に1件も無い(tools/golden/generate.mjs に足して再生成する)", name)
		}
		if c.control == 0 {
			t.Errorf("%s: 対照(別タイプの技でダメージが出る)ベクタが fixed.json に1件も無い", name)
		}
		if total := c.nullified + c.control; total < goldenImmunityMinPerAbility {
			t.Errorf("%s: ベクタが %d 件しかない(下限 %d。無効・対照・組合せ)", name, total, goldenImmunityMinPerAbility)
		}
	}
}

// effectNullifies は効果定義がその攻撃タイプを 0 にするか(無効・吸収のどちらでもよい)。
func effectNullifies(e *AbilityEffect, t Type) bool {
	for _, it := range e.DefImmuneTypes {
		if it == t {
			return true
		}
	}
	_, ok := e.DefAbsorbTypes[t]
	return ok
}

func sortedStrings[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestGoldenImmunityIDsAreDescriptive は、足したベクタの ID が何のケースか読めることを軽く守る
// (id が "levitate/..." のように特性名を含む)。
func TestGoldenImmunityIDsAreDescriptive(t *testing.T) {
	cases := readGoldenFixed(t)
	for _, c := range cases {
		name := c.Input.Defender.Ability.ID
		if _, ok := goldenImmunityAbilities[name]; !ok {
			continue
		}
		slug := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
		if !strings.Contains(c.ID, slug) {
			t.Errorf("ベクタ %q の id が特性 %q を含まない(失敗したときに何のケースか分かるようにする)", c.ID, name)
		}
	}
}
