//go:build golden

package engine

// P2-1b: ゴールデンの oracle を @smogon/calc 0.12.0 の Champions 世代へ切り替えたことの受け入れテスト
// (ADR-0002 §決定 5 の追記「P2-1b」)。
//
//   - 主のゴールデンは Champions 世代(Generations.get(0))。SP は直接渡す。種族集合は Champions 集合(内部フォーム除外)。
//   - Champions の mechanics に無い効果(こだわりハチマキ・こだわりメガネ・とつげきチョッキ・しんかのきせき・はがねつかい)
//     を使うケースは削除せず、同じ 0.12.0 の gen9 世代で照合する legacy-effects.jsonl.gz に分ける(ADR-0002 §決定 7)。
//   - 両方とも全件一致を要求する(TestGoldenDamage)。known_diffs には何も足さない。
//
// 件数は再生成で変わるため固定値ではなく「下限と整合性(ファイル間・manifest 間で矛盾しない)」で守る。

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
)

// goldenKnownLegacyEffects は P2-1b 第1段階で、Champions 世代に存在しないと確認した効果。
// 生成器は effects.json の名前を Champions 世代の items/abilities と突き合わせて legacyEffects を導出する。
// ここはその導出の下限(この5件が漏れたら導出が壊れている)。持ち物の一覧をロジックに書くのではなく、
// 調査結果(ADR-0002 §4 と P2-1b 追記)をテストの期待値として固定している。
var goldenKnownLegacyEffects = struct{ items, abilities []string }{
	items:     []string{"Choice Band", "Choice Specs", "Assault Vest", "Eviolite"},
	abilities: []string{"Steelworker"},
}

// goldenChampionsFiles は Champions oracle が生成するファイル。
var goldenChampionsFiles = []string{"fixed.json", "random.jsonl.gz", "attack-species.jsonl.gz", "defense-species.jsonl.gz", "stats-species.jsonl.gz", typeChartFixture}

// goldenChampionsDamageFiles は Champions oracle のダメージのベクタ(持ち物・特性を含みうる)。
var goldenChampionsDamageFiles = []string{"fixed.json", "random.jsonl.gz", "attack-species.jsonl.gz", "defense-species.jsonl.gz"}

// goldenLegacyRandomPrefix は legacy-effects のうちランダム生成部分の ID の接頭辞。
// それ以外(固定シナリオ由来)の ID は元の fixed.json と同じ "<ラベル>/<攻撃側>/<技>" 形式を保つ。
const goldenLegacyRandomPrefix = "legacy-random/"

// goldenLegacyRandomMin は legacy-effects のランダム部分の最低件数。
// 旧 random(10,000件)ではこだわり系・チョッキ・はがねつかいが天候・ランク・急所・壁と組み合わさって検証されていた。
// それらを Champions の random から外す代わりに、組合せの検証をここで残す(削除でなく分離。絶対ルール6)。
//
// critic レビュー(P2-1b 再修正): 効果ごとに層を分けて生成するようになったため(generate.mjs の
// legacyRandomLayers)、総数は 1000 から引き上げる。内訳の下限は goldenLegacyEffectMinWorking で守る。
const goldenLegacyRandomMin = 3000

// goldenLegacyEffectMinWorking は legacy-random の各効果が「実際にダメージ計算へ効く」件数の下限。
//
// critic レビュー(P2-1b 再修正): 生成器が持ち物・特性を入れただけで、技の分類・タイプが合わず
// engine の補正(modifiers.go の offensiveStatMod/defensiveStatMod/OffBoostType)が実際には
// 一度も乗らないケースが大半だった(旧実装の Champions random 10,000件中、実際に効いていたのは
// Choice Band 623 / Choice Specs 611 / Assault Vest 1181 / Steelworker 151 件)。
// これを下回ると絶対ルール6(テストを弱めない)に反するため、下限として固定する。
var goldenLegacyEffectMinWorking = map[string]int{
	"Choice Band":  620,
	"Choice Specs": 620,
	"Assault Vest": 1180,
	"Steelworker":  150,
}

// goldenLegacyEffectApplies は、ケースの入力から、その legacy 効果が実際にダメージ計算へ効くかを判定する
// (engine の modifiers.go に対応させたもの。名前を追加するときはここも見直す):
//   - Choice Band / Choice Specs(攻撃側の StatMods.atk / .spa): 技の分類(物理/特殊)が一致する必要がある
//     (offensiveStatMod は atkKey = 技が物理なら atk、特殊なら spa の実数値だけを補正する)
//   - Assault Vest(防御側の StatMods.spd): 技が特殊でないと特防(spd)が参照されない
//     (defensiveStatMod は defKey = 技が物理なら def、特殊なら spd の実数値だけを補正する)
//   - Steelworker(攻撃側 OffBoostType): 技タイプが鋼のときだけ攻撃実数値へ乗る
//   - Eviolite(防御側の StatMods.def と .spd の両方): 物理・特殊どちらでも効く
func goldenLegacyEffectApplies(v goldenCase, name string) bool {
	mv := v.Input.Move
	switch name {
	case "Choice Band":
		return v.Input.Attacker.Item != nil && v.Input.Attacker.Item.ID == name && mv.Category == CategoryPhysical
	case "Choice Specs":
		return v.Input.Attacker.Item != nil && v.Input.Attacker.Item.ID == name && mv.Category == CategorySpecial
	case "Assault Vest":
		return v.Input.Defender.Item != nil && v.Input.Defender.Item.ID == name && mv.Category == CategorySpecial
	case "Eviolite":
		return v.Input.Defender.Item != nil && v.Input.Defender.Item.ID == name
	case "Steelworker":
		return v.Input.Attacker.Ability.ID == name && mv.Type == TypeSteel
	}
	return false
}

// goldenFixedScenarioMinCounts は P2-1b 以前の fixed.json(203件)にあったシナリオのラベルと件数。
// 切り替え後は fixed.json(Champions)と legacy-effects(gen9)の和でこの件数以上を要求する。
// 種族の差し替え(Champions に居ない種族 → 同じ役割の種族)で ID の種族名部分は変わってよいが、シナリオは消さない。
var goldenFixedScenarioMinCounts = map[string]int{
	"adaptability": 6, "assaultvest": 6, "aurora-veil": 6, "baseline": 6, "burn": 6, "charcoal": 6,
	"chilanberry": 6, "choiceband": 6, "choicespecs": 6, "critical": 6, "critical-ranks-screens": 6,
	"electric": 6, "expertbelt": 6, "filter": 6, "filter-life-orb": 6, "grassy": 6, "lifeorb": 6,
	"light-screen": 6, "misty": 6, "muscleband": 6, "occaberry": 6, "psychic": 6, "rain": 6,
	"reflect": 6, "sand": 6, "sand-vest": 6, "snow": 6, "steelworker": 6, "sun": 6,
	"sun-charcoal-thick-fat": 6, "thick-fat": 6, "water-bubble": 6, "wiseglasses": 6,
	"eviolite-snow": 1, "ko-four-hits": 1, "misty-dragon": 1, "water-bubble-attack": 1, "water-bubble-defense": 1,
}

func scenarioLabel(id string) string {
	if i := strings.IndexByte(id, '/'); i >= 0 {
		return id[:i]
	}
	return id
}

// readGoldenDamageCases は fixed 形式(JSON 配列)と gzip JSONL の両方を読み、manifest の sha256 と件数を照合する。
func readGoldenDamageCases(t *testing.T, meta goldenMetadata, name string) []goldenCase {
	t.Helper()
	var out []goldenCase
	if strings.HasSuffix(name, ".jsonl.gz") {
		eachGoldenLine(t, meta, name, func(line []byte) {
			var v goldenCase
			if err := json.Unmarshal(line, &v); err != nil {
				t.Fatal(err)
			}
			out = append(out, v)
		})
		return out
	}
	if err := json.Unmarshal(goldenFile(t, meta, name), &out); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if len(out) != meta.Files[name].Count {
		t.Fatalf("%s count=%d want=%d", name, len(out), meta.Files[name].Count)
	}
	return out
}

// countGoldenFixedInLegacy は legacy-effects のうち、固定シナリオ由来(ランダム以外)の件数。
func countGoldenFixedInLegacy(t *testing.T, meta goldenMetadata) int {
	t.Helper()
	if _, ok := meta.Files[goldenLegacyEffectsFile]; !ok {
		t.Fatalf("%s が manifest に無い(Champions に無い効果のシナリオを gen9 で照合するファイル。P2-1b)", goldenLegacyEffectsFile)
	}
	n := 0
	for _, v := range readGoldenDamageCases(t, meta, goldenLegacyEffectsFile) {
		if !strings.HasPrefix(v.ID, goldenLegacyRandomPrefix) {
			n++
		}
	}
	return n
}

// caseEffects はケースが使う持ち物・特性の名前(effects.json のキー)。
func caseEffects(v goldenCase) (items, abilities []string) {
	for _, p := range []Individual{v.Input.Attacker, v.Input.Defender} {
		if p.Item != nil && p.Item.ID != "" {
			items = append(items, p.Item.ID)
		}
		if p.Ability.ID != "" {
			abilities = append(abilities, p.Ability.ID)
		}
	}
	return items, abilities
}

func toSet(xs []string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// normalizeSpeciesID は @smogon/calc の species.id と同じ正規化(小文字英数のみ)。
func normalizeSpeciesID(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// AC1/AC2: oracle の版・世代・SP の渡し方・ファイルの帰属が metadata に記録されていること。
func TestGoldenOracleMetadata(t *testing.T) {
	meta := readGoldenMetadata(t) // schemaVersion 2 / 0.12.0 / champions / speciesCount の範囲

	champions := meta.oracle(t, goldenChampionsOracleID)
	if champions.Source != "@smogon/calc" || champions.Version != goldenOracleVersion || champions.Generation != "champions" || champions.GenerationNum != goldenChampionsGenNum {
		t.Errorf("champions oracle = %+v, want @smogon/calc %s / champions / Generations.get(%d)", champions, goldenOracleVersion, goldenChampionsGenNum)
	}
	if champions.SPInput != goldenChampionsSPDirect {
		t.Errorf("champions oracle の spInput=%q want %q(Champions 世代は SP 0〜32 を evs にそのまま渡す)", champions.SPInput, goldenChampionsSPDirect)
	}
	if champions.LegacyEffects != nil {
		t.Errorf("champions oracle は legacyEffects を持たない")
	}

	legacy := meta.oracle(t, goldenLegacyOracleID)
	if legacy.Source != "@smogon/calc" || legacy.Version != goldenOracleVersion || legacy.Generation != "gen9" || legacy.GenerationNum != goldenGen9GenNum {
		t.Errorf("legacy oracle = %+v, want @smogon/calc %s(champions と同じ pin)/ gen9 / Generations.get(%d)", legacy, goldenOracleVersion, goldenGen9GenNum)
	}
	if legacy.SPInput != goldenSPToEVFormula {
		t.Errorf("legacy oracle の spInput=%q want %q(gen9 は努力値に換算する)", legacy.SPInput, goldenSPToEVFormula)
	}
	if legacy.Seed == nil {
		t.Errorf("legacy oracle に seed が無い(ランダム部分の再現に必要)")
	}
	if strings.TrimSpace(legacy.Reason) == "" {
		t.Errorf("legacy oracle に reason が無い(なぜ gen9 で照合するか。ADR-0002 §決定 7)")
	}

	if got, want := strings.Join(champions.Files, ","), strings.Join(goldenChampionsFiles, ","); !sameStringSet(champions.Files, goldenChampionsFiles) {
		t.Errorf("champions oracle の files=%s want %s", got, want)
	}
	if !sameStringSet(legacy.Files, []string{goldenLegacyEffectsFile}) {
		t.Errorf("legacy oracle の files=%v want [%s]", legacy.Files, goldenLegacyEffectsFile)
	}

	// manifest の全ファイルがちょうど1つの oracle に属し、oracle のファイルは manifest にある。
	owner := map[string]string{}
	for _, o := range meta.Oracles {
		for _, f := range o.Files {
			if prev, dup := owner[f]; dup {
				t.Errorf("%s が2つの oracle(%s と %s)に属している", f, prev, o.ID)
			}
			owner[f] = o.ID
			if _, ok := meta.Files[f]; !ok {
				t.Errorf("oracle %s の %s が manifest(files)に無い", o.ID, f)
			}
		}
	}
	for f := range meta.Files {
		if _, ok := owner[f]; !ok {
			t.Errorf("manifest の %s がどの oracle にも属していない(どの世代で生成したか不明)", f)
		}
	}

	scope := strings.ToLower(meta.SpeciesScope)
	if !strings.Contains(scope, "champions") || strings.Contains(scope, "gen9 library") {
		t.Errorf("speciesScope=%q は Champions 集合を指すこと", meta.SpeciesScope)
	}
}

// AC4: @smogon/calc は 0.12.0 に完全固定(^ や ~ を許さない)で、lockfile も同じ版を指す。
func TestGoldenOraclePinnedVersion(t *testing.T) {
	var pkg struct {
		Dependencies map[string]string `json:"dependencies"`
	}
	raw, err := os.ReadFile("../tools/golden/package.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &pkg); err != nil {
		t.Fatal(err)
	}
	if got := pkg.Dependencies["@smogon/calc"]; got != goldenOracleVersion {
		t.Errorf("tools/golden/package.json の @smogon/calc=%q want %q(完全固定。範囲指定は不可)", got, goldenOracleVersion)
	}
	var lock struct {
		Packages map[string]struct {
			Version string `json:"version"`
		} `json:"packages"`
	}
	raw, err = os.ReadFile("../tools/golden/package-lock.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &lock); err != nil {
		t.Fatal(err)
	}
	if got := lock.Packages["node_modules/@smogon/calc"].Version; got != goldenOracleVersion {
		t.Errorf("package-lock.json の @smogon/calc=%q want %q", got, goldenOracleVersion)
	}
}

// AC3: Champions に無い効果は legacy-effects(gen9)に分かれ、Champions のベクタには現れない。
func TestGoldenLegacyEffectsPartition(t *testing.T) {
	meta := readGoldenMetadata(t)
	legacy := meta.oracle(t, goldenLegacyOracleID)
	if legacy.LegacyEffects == nil {
		t.Fatal("legacy oracle に legacyEffects が無い")
	}
	legacyItems, legacyAbilities := toSet(legacy.LegacyEffects.Items), toSet(legacy.LegacyEffects.Abilities)
	for _, n := range goldenKnownLegacyEffects.items {
		if !legacyItems[n] {
			t.Errorf("legacyEffects.items に %q が無い(Champions 世代に無いと確認済み。導出が壊れている)", n)
		}
	}
	for _, n := range goldenKnownLegacyEffects.abilities {
		if !legacyAbilities[n] {
			t.Errorf("legacyEffects.abilities に %q が無い(Champions 世代に無いと確認済み。導出が壊れている)", n)
		}
	}

	// legacyEffects は effects.json(テスト専用アダプタ)の名前の部分集合。
	var effects struct {
		Items     map[string]json.RawMessage `json:"items"`
		Abilities map[string]json.RawMessage `json:"abilities"`
	}
	raw, err := os.ReadFile("../testdata/golden/effects.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &effects); err != nil {
		t.Fatal(err)
	}
	for n := range legacyItems {
		if _, ok := effects.Items[n]; !ok {
			t.Errorf("legacyEffects.items の %q が effects.json に無い", n)
		}
	}
	for n := range legacyAbilities {
		if _, ok := effects.Abilities[n]; !ok {
			t.Errorf("legacyEffects.abilities の %q が effects.json に無い", n)
		}
	}

	// legacy-effects の全ケースが、Champions に無い効果を少なくとも1つ実際に使う。
	cases := readGoldenDamageCases(t, meta, goldenLegacyEffectsFile)
	if len(cases) == 0 {
		t.Fatalf("%s が空", goldenLegacyEffectsFile)
	}
	usedItems, usedAbilities := map[string]bool{}, map[string]bool{}
	workingCounts := map[string]int{}
	randomCount, notWorking := 0, 0
	for _, v := range cases {
		items, abilities := caseEffects(v)
		uses := false
		for _, n := range items {
			if legacyItems[n] {
				uses, usedItems[n] = true, true
			}
		}
		for _, n := range abilities {
			if legacyAbilities[n] {
				uses, usedAbilities[n] = true, true
			}
		}
		if !uses {
			t.Errorf("%s: Champions に無い効果を使っていない(items=%v abilities=%v)。Champions で照合できるケースは主のファイルへ", v.ID, items, abilities)
		}
		if !strings.HasPrefix(v.ID, goldenLegacyRandomPrefix) {
			// 固定シナリオ由来(choiceband/choicespecs/assaultvest/steelworker/sand-vest ×
			// 元の6組、eviolite-snow)は P2-1b 以前の fixed.json をそのまま引き継いだもの
			// (ADR-0002 追記 P2-1b「元の種族のまま」)。技の分類・タイプは組み替えていないため、
			// ここでは「使っている(uses)」までを見る。ランダム部分だけ「実際に効く」まで見る。
			continue
		}
		randomCount++
		applied := false
		for _, n := range items {
			if legacyItems[n] && goldenLegacyEffectApplies(v, n) {
				applied, workingCounts[n] = true, workingCounts[n]+1
			}
		}
		for _, n := range abilities {
			if legacyAbilities[n] && goldenLegacyEffectApplies(v, n) {
				applied, workingCounts[n] = true, workingCounts[n]+1
			}
		}
		if !applied {
			notWorking++
			if notWorking <= 5 {
				t.Errorf("%s: legacy 効果を持つが実際には効いていない(技の分類/タイプが合わない): items=%v abilities=%v move=%+v",
					v.ID, items, abilities, v.Input.Move)
			}
		}
	}
	if notWorking > 5 {
		t.Errorf("legacy-random: ほか %d 件が実際には効いていない", notWorking-5)
	}
	for n := range legacyItems {
		if !usedItems[n] {
			t.Errorf("legacyEffects.items の %q を使うケースが legacy-effects に無い(計算式が検証されていない)", n)
		}
	}
	for n := range legacyAbilities {
		if !usedAbilities[n] {
			t.Errorf("legacyEffects.abilities の %q を使うケースが legacy-effects に無い(計算式が検証されていない)", n)
		}
	}
	if randomCount < goldenLegacyRandomMin {
		t.Errorf("legacy-effects のランダム部分=%d 件 want >= %d(旧 random が持っていた組合せの検証を残す)", randomCount, goldenLegacyRandomMin)
	}
	for name, min := range goldenLegacyEffectMinWorking {
		if workingCounts[name] < min {
			t.Errorf("legacy-random: %q が実際に効いた件数=%d want >= %d(旧 random の組合せ検証を下回らないこと。絶対ルール6)", name, workingCounts[name], min)
		}
	}

	// Champions のベクタは Champions に無い効果を使わない(使えば 0.12.0 Champions の mechanics が無視して期待値が変わる)。
	for _, name := range goldenChampionsDamageFiles {
		bad := 0
		for _, v := range readGoldenDamageCases(t, meta, name) {
			items, abilities := caseEffects(v)
			for _, n := range items {
				if legacyItems[n] {
					bad++
					if bad <= 5 {
						t.Errorf("%s %s: Champions に無い持ち物 %q を使っている", name, v.ID, n)
					}
				}
			}
			for _, n := range abilities {
				if legacyAbilities[n] {
					bad++
					if bad <= 5 {
						t.Errorf("%s %s: Champions に無い特性 %q を使っている", name, v.ID, n)
					}
				}
			}
		}
		if bad > 5 {
			t.Errorf("%s: ほか %d 件", name, bad-5)
		}
	}
}

// AC3(削除しない): P2-1b 以前の固定シナリオは、fixed.json と legacy-effects の和で1つも消えていない。
func TestGoldenFixedScenariosPreserved(t *testing.T) {
	meta := readGoldenMetadata(t)
	got := map[string]int{}
	for _, v := range readGoldenDamageCases(t, meta, "fixed.json") {
		got[scenarioLabel(v.ID)]++
	}
	for _, v := range readGoldenDamageCases(t, meta, goldenLegacyEffectsFile) {
		if !strings.HasPrefix(v.ID, goldenLegacyRandomPrefix) {
			got[scenarioLabel(v.ID)]++
		}
	}
	for label, want := range goldenFixedScenarioMinCounts {
		if got[label] < want {
			t.Errorf("固定シナリオ %q = %d 件 want >= %d(Champions に無い効果・種族でも削除せず legacy-effects か差し替えで残す)", label, got[label], want)
		}
	}
}

// AC5: 種族集合はファイル間で一致し、Champions の集合(内部フォーム除外)である。
func TestGoldenSpeciesSetIntegrity(t *testing.T) {
	meta := readGoldenMetadata(t)

	// 実数値ベクタの種族 = 種族集合の正(1種族 6ステータス × SP4点 × 性格3 = 72 件)。
	stats := map[string]int{}
	eachGoldenLine(t, meta, "stats-species.jsonl.gz", func(line []byte) {
		var v struct {
			Individual json.RawMessage `json:"individual"`
		}
		if err := json.Unmarshal(line, &v); err != nil {
			t.Fatal(err)
		}
		var ind Individual
		if err := decodeStrictGolden(v.Individual, &ind); err != nil {
			t.Fatal(err)
		}
		stats[ind.Species.Key]++
	})
	if len(stats) != meta.SpeciesCount {
		t.Fatalf("stats-species の種族数=%d, speciesCount=%d", len(stats), meta.SpeciesCount)
	}
	set := map[string]bool{}
	for k, n := range stats {
		set[k] = true
		if n != 72 {
			t.Errorf("stats-species の %s = %d 件 want 72", k, n)
		}
	}

	attackers, defenders := map[string]bool{}, map[string]bool{}
	for _, v := range readGoldenDamageCases(t, meta, "attack-species.jsonl.gz") {
		attackers[v.Input.Attacker.Species.Key] = true
	}
	for _, v := range readGoldenDamageCases(t, meta, "defense-species.jsonl.gz") {
		defenders[v.Input.Defender.Species.Key] = true
	}
	if !sameStringSet(sortedKeys(attackers), sortedKeys(set)) {
		t.Errorf("attack-species の攻撃側の種族集合(%d)が stats-species(%d)と違う", len(attackers), len(set))
	}
	if !sameStringSet(sortedKeys(defenders), sortedKeys(set)) {
		t.Errorf("defense-species の防御側の種族集合(%d)が stats-species(%d)と違う", len(defenders), len(set))
	}

	// 固定・ランダム・アンカーの種族も Champions 集合の中(Champions に居ない種族は同じ役割の種族へ差し替える)。
	for _, name := range goldenChampionsDamageFiles {
		bad := 0
		for _, v := range readGoldenDamageCases(t, meta, name) {
			for _, k := range []string{v.Input.Attacker.Species.Key, v.Input.Defender.Species.Key} {
				if !set[k] {
					bad++
					if bad <= 5 {
						t.Errorf("%s %s: 種族 %q が Champions の種族集合に無い", name, v.ID, k)
					}
				}
			}
		}
		if bad > 5 {
			t.Errorf("%s: ほか %d 件", name, bad-5)
		}
	}

	// 内部フォームの除外規則が metadata に記録され、実際に集合から外れている。
	excluded := map[string]bool{}
	for _, e := range meta.Exclusions {
		if e.Scope != "species" {
			continue
		}
		if len(e.Names) > 0 && strings.TrimSpace(e.Reason) == "" {
			t.Errorf("種族の除外 %v に理由が無い", e.Names)
		}
		for _, n := range e.Names {
			excluded[n] = true
			if set[normalizeSpeciesID(n)] {
				t.Errorf("除外したはずの %q が種族集合に入っている", n)
			}
		}
	}
	if !excluded["Aegislash-Both"] {
		t.Errorf("exclusions(scope=species)に内部フォーム Aegislash-Both が無い(calc の Champions 種族のうち選択できない内部フォーム。ADR-0002 P2-1b)")
	}
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sa := toSet(a)
	if len(sa) != len(a) {
		return false
	}
	for _, x := range b {
		if !sa[x] {
			return false
		}
	}
	return true
}
