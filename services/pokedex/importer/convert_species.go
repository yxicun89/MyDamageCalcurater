package importer

// 種族・フォーム・メガの変換(ADR-0101 §5)。

import (
	"fmt"
	"sort"
	"strings"

	"example.com/pokecalc/services/internal/master"
)

// speciesConversion は Convert の種族処理の結果。
type speciesConversion struct {
	Rows                 []SpeciesRow
	RegulationKeys       []string
	RegulationAbilityIDs []string
	BaseSpeciesName      map[string]string // showdown_id -> toID(raw Showdown BaseSpecies 名)。learnsets のフォールバック用(learnsets のキーは showdown_id 形式)
	AbilityNameEn        map[string]string // ability id -> 英語名(取り込んだ種族が使うものだけとは限らないが上書きは同名で無害)
}

// rawSpecies は畳み込み判定・key 採番より前の中間表現。
type rawSpecies struct {
	showdownID       string
	nameEn           string
	dexNo            int
	form             int
	type1, type2     string
	stats            [6]int
	isMega           bool
	baseSpeciesName  string
	requiredItemName string
	abilities        []master.SpeciesAbilityRow
	fromCalc         bool
	support          bool // 使用可能なメガの FK を満たすためだけに保持する、レギュレーション外の基本種
}

// slotBySDKey は Showdown の abilities マップのキーを species_abilities.slot に写す。
// "S" は Showdown の特殊枠。実データでは H と同時に存在する種族があるため、隠れ特性の
// slot 3 と区別して slot 4 に写す(ADR-0103 §12)。
var slotBySDKey = map[string]int{"0": 1, "1": 2, "H": 3, "S": 4}

// buildSpeciesAbilities は Showdown の abilities マップ(スロットキー→英語名)を、
// species_abilities の行(スロット番号→ID)と ID→英語名のマップに変換する。
func buildSpeciesAbilities(m map[string]string) ([]master.SpeciesAbilityRow, map[string]string, error) {
	if len(m) == 0 {
		return nil, nil, fmt.Errorf("%w: 特性が無い", ErrInvalidData)
	}
	rows := make([]master.SpeciesAbilityRow, 0, len(m))
	names := map[string]string{}
	for k, name := range m {
		slot, ok := slotBySDKey[k]
		if !ok {
			return nil, nil, fmt.Errorf("%w: 特性スロットキーが不正: %q", ErrInvalidData, k)
		}
		id := toID(name)
		rows = append(rows, master.SpeciesAbilityRow{Slot: slot, AbilityID: id})
		names[id] = name
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Slot < rows[j].Slot })
	return rows, names, nil
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// rawSignature は「見た目だけ違うか」の判定に使う性能の署名(タイプ・種族値・特性)。
func rawSignature(r rawSpecies) string {
	abilities := append([]master.SpeciesAbilityRow(nil), r.abilities...)
	sort.Slice(abilities, func(i, j int) bool { return abilities[i].Slot < abilities[j].Slot })
	var b strings.Builder
	fmt.Fprintf(&b, "%s|%s|%v", r.type1, r.type2, r.stats)
	for _, a := range abilities {
		fmt.Fprintf(&b, "|%d:%s", a.Slot, a.AbilityID)
	}
	return b.String()
}

// buildRawSpecies は calc の1種族(fromCalc=false のときは Showdown だけの候補)を中間表現に変換する。
// fromCalc=true のときだけ calc との値の食い違いを検査し、食い違いがあれば blockers を返す
// (この場合 rawSpecies はゼロ値)。
func buildRawSpecies(c CalcSpecies, sd ShowdownSpecies, sdByName map[string]ShowdownSpecies, typeNameToID map[string]string,
	includedItems map[string]bool, fromCalc bool) (rawSpecies, map[string]string, []Finding, error) {

	var type1, type2 string
	var stats [6]int

	if fromCalc {
		if len(c.Types) == 0 {
			return rawSpecies{}, nil, nil, fmt.Errorf("%w: 種族 %q のタイプが空", ErrInvalidData, c.Name)
		}
		t1, ok := typeNameToID[c.Types[0]]
		if !ok {
			return rawSpecies{}, nil, nil, fmt.Errorf("%w: 種族 %q が除外したタイプを使っている: %q", ErrInvalidData, c.Name, c.Types[0])
		}
		type1 = t1
		if len(c.Types) > 1 {
			t2, ok := typeNameToID[c.Types[1]]
			if !ok {
				return rawSpecies{}, nil, nil, fmt.Errorf("%w: 種族 %q が除外したタイプを使っている: %q", ErrInvalidData, c.Name, c.Types[1])
			}
			type2 = t2
		}
		sdTypeIDs := make([]string, 0, len(sd.Types))
		for _, tn := range sd.Types {
			id, ok := typeNameToID[tn]
			if !ok {
				return rawSpecies{}, nil, nil, fmt.Errorf("%w: 種族 %q(Showdown)が除外したタイプを使っている: %q", ErrInvalidData, sd.Name, tn)
			}
			sdTypeIDs = append(sdTypeIDs, id)
		}
		calcTypeIDs := []string{type1}
		if type2 != "" {
			calcTypeIDs = append(calcTypeIDs, type2)
		}
		var blockers []Finding
		if !equalStringSlices(calcTypeIDs, sdTypeIDs) {
			blockers = append(blockers, Finding{Kind: KindSpeciesMismatch, ID: sd.ID, Detail: "type"})
		}
		if c.BaseStats != sd.BaseStats {
			blockers = append(blockers, Finding{Kind: KindSpeciesMismatch, ID: sd.ID, Detail: "stats"})
		}
		if len(blockers) > 0 {
			return rawSpecies{}, nil, blockers, nil
		}
		stats = [6]int{c.BaseStats.HP, c.BaseStats.Atk, c.BaseStats.Def, c.BaseStats.SpA, c.BaseStats.SpD, c.BaseStats.Spe}
	} else {
		ids := make([]string, 0, len(sd.Types))
		for _, tn := range sd.Types {
			id, ok := typeNameToID[tn]
			if !ok {
				return rawSpecies{}, nil, nil, fmt.Errorf("%w: 種族 %q が除外したタイプを使っている", ErrInvalidData, sd.Name)
			}
			ids = append(ids, id)
		}
		if len(ids) > 0 {
			type1 = ids[0]
		}
		if len(ids) > 1 {
			type2 = ids[1]
		}
		stats = [6]int{sd.BaseStats.HP, sd.BaseStats.Atk, sd.BaseStats.Def, sd.BaseStats.SpA, sd.BaseStats.SpD, sd.BaseStats.Spe}
	}

	base, ok := sdByName[sd.BaseSpecies]
	if !ok {
		return rawSpecies{}, nil, nil, fmt.Errorf("%w: 種族 %q の基本種 %q が Showdown に無い", ErrInvalidData, sd.Name, sd.BaseSpecies)
	}
	form := -1
	if len(base.FormeOrder) == 0 && sd.Name == base.Name && sd.Forme == "" {
		// Showdown はフォームを持たない基本種では formeOrder を省略する。
		// 基本種の form 0 は ADR-0101 §5 の規則から一意なので、省略を許容する。
		form = 0
	}
	for i, n := range base.FormeOrder {
		if n == sd.Name {
			form = i
			break
		}
	}
	if form < 0 {
		return rawSpecies{}, nil, nil, fmt.Errorf("%w: 種族 %q が基本種の formeOrder に無い", ErrInvalidData, sd.Name)
	}

	isMega := strings.HasPrefix(sd.Forme, "Mega")
	if fromCalc && isMega {
		if sd.RequiredItem == "" {
			return rawSpecies{}, nil, nil, fmt.Errorf("%w: メガ %q の requiredItem が無い", ErrInvalidData, sd.Name)
		}
		if !includedItems[toID(sd.RequiredItem)] {
			return rawSpecies{}, nil, nil, fmt.Errorf("%w: メガ %q の requiredItem %q が取り込む持ち物に無い", ErrInvalidData, sd.Name, sd.RequiredItem)
		}
	}

	abilities, abilityNames, err := buildSpeciesAbilities(sd.Abilities)
	if err != nil {
		return rawSpecies{}, nil, nil, err
	}

	r := rawSpecies{
		showdownID: sd.ID, nameEn: sd.Name, dexNo: sd.Num, form: form,
		type1: type1, type2: type2, stats: stats, isMega: isMega,
		baseSpeciesName: sd.BaseSpecies, requiredItemName: sd.RequiredItem,
		abilities: abilities, fromCalc: fromCalc,
	}
	return r, abilityNames, nil, nil
}

func convertSpecies(in Input, typeNameToID map[string]string, includedItems map[string]bool, usedOverride map[string]bool) (speciesConversion, []Finding, []Finding, error) {
	sdByID := map[string]ShowdownSpecies{}
	sdByName := map[string]ShowdownSpecies{}
	for _, s := range in.Showdown.Species {
		sdByID[s.ID] = s
		sdByName[s.Name] = s
	}
	matchKey := map[string]string{}
	for _, s := range in.Showdown.Species {
		matchKey[toID(s.Name)] = s.ID
		if s.BaseForme != "" {
			matchKey[toID(s.Name+"-"+s.BaseForme)] = s.ID
		}
	}

	calcNameSet := map[string]bool{}
	for _, c := range in.Calc.Species {
		calcNameSet[c.Name] = true
	}
	excludeSet := map[string]bool{}
	for _, name := range in.Config.ExcludeCalcSpecies {
		if !calcNameSet[name] {
			return speciesConversion{}, nil, nil, fmt.Errorf("%w: excludeCalcSpecies に calc に無い種族名がある: %q", ErrInvalidData, name)
		}
		excludeSet[name] = true
	}

	var warnings, blockers []Finding
	var raw []rawSpecies
	abilityNameEn := map[string]string{}
	rawByID := map[string]bool{}

	for _, c := range in.Calc.Species {
		if excludeSet[c.Name] {
			warnings = append(warnings, Finding{Kind: KindSpeciesExcluded, ID: toID(c.Name)})
			continue
		}
		sdID, ok := matchKey[toID(c.Name)]
		if !ok {
			return speciesConversion{}, nil, nil, fmt.Errorf("%w: calc の種族 %q に対応する Showdown の種族が無い", ErrInvalidData, c.Name)
		}
		sd := sdByID[sdID]
		if sd.IsNonstandard != nil {
			warnings = append(warnings, Finding{Kind: KindSpeciesExcluded, ID: sd.ID})
			continue
		}

		r, names, blockerFindings, err := buildRawSpecies(c, sd, sdByName, typeNameToID, includedItems, true)
		if err != nil {
			return speciesConversion{}, nil, nil, err
		}
		if len(blockerFindings) > 0 {
			blockers = append(blockers, blockerFindings...)
			continue
		}
		for id, name := range names {
			abilityNameEn[id] = name
		}
		raw = append(raw, r)
		rawByID[r.showdownID] = true
	}

	if len(blockers) > 0 {
		return speciesConversion{}, warnings, blockers, nil
	}

	// Showdown では、使用可能なメガの基本種が Past で calc に無い場合がある。メガの
	// base_species_key 外部キーを満たすため、その基本種をマスタの依存行として保持する。
	// 依存行は下でレギュレーションの使用可能集合から除外する。
	for i := 0; i < len(raw); i++ {
		mega := raw[i]
		if !mega.isMega || rawByID[toID(mega.baseSpeciesName)] {
			continue
		}
		base, ok := sdByName[mega.baseSpeciesName]
		if !ok {
			return speciesConversion{}, nil, nil, fmt.Errorf("%w: メガ %q の基本種 %q が Showdown に無い", ErrInvalidData, mega.nameEn, mega.baseSpeciesName)
		}
		dependency, names, _, err := buildRawSpecies(CalcSpecies{}, base, sdByName, typeNameToID, includedItems, false)
		if err != nil {
			return speciesConversion{}, nil, nil, err
		}
		dependency.support = true
		for id, name := range names {
			abilityNameEn[id] = name
		}
		raw = append(raw, dependency)
		rawByID[dependency.showdownID] = true
		warnings = append(warnings, Finding{Kind: KindSpeciesMegaBaseDependency, ID: dependency.showdownID})
	}

	numsWithRaw := map[int]bool{}
	for _, r := range raw {
		numsWithRaw[r.dexNo] = true
	}
	for _, sd := range in.Showdown.Species {
		if rawByID[sd.ID] || sd.IsNonstandard != nil || !numsWithRaw[sd.Num] {
			continue
		}
		r, names, _, err := buildRawSpecies(CalcSpecies{}, sd, sdByName, typeNameToID, includedItems, false)
		if err != nil {
			continue // 型やフォームが不明な showdown-only 候補は扱えない(P2-2c で照合)
		}
		for id, name := range names {
			abilityNameEn[id] = name
		}
		raw = append(raw, r)
	}

	byNum := map[int][]int{}
	for i, r := range raw {
		byNum[r.dexNo] = append(byNum[r.dexNo], i)
	}
	nums := make([]int, 0, len(byNum))
	for n := range byNum {
		nums = append(nums, n)
	}
	sort.Ints(nums)

	excluded := make([]bool, len(raw))
	// repHasCalcMember[代表の index] = その畳み込み組(代表 + 畳んだもの)に calc 由来の
	// 種族が1件でもあるか。フォルム番号が最小のものが代表になるとは限らず、Showdown だけの
	// 候補が代表になっても、組の中に calc で確認された種族があれば取り込む(ADR-0101 §5
	// 「calc にあるかどうかに依らない」)。
	repHasCalcMember := map[int]bool{}
	for _, num := range nums {
		idxs := append([]int(nil), byNum[num]...)
		sort.Slice(idxs, func(a, b int) bool { return raw[idxs[a]].form < raw[idxs[b]].form })
		seen := map[string]int{}
		for _, i := range idxs {
			sig := rawSignature(raw[i])
			rep, ok := seen[sig]
			if !ok {
				seen[sig] = i
				rep = i
			} else {
				warnings = append(warnings, Finding{Kind: KindFormFolded, ID: raw[i].showdownID, Detail: raw[rep].showdownID})
				excluded[i] = true
			}
			if raw[i].fromCalc {
				repHasCalcMember[rep] = true
			}
		}
	}
	for i, r := range raw {
		if !excluded[i] && !r.fromCalc && !r.support && !repHasCalcMember[i] {
			excluded[i] = true
			warnings = append(warnings, Finding{Kind: KindSpeciesShowdownOnly, ID: r.showdownID})
		}
	}

	keyByNameEn := map[string]string{}
	baseSpeciesName := map[string]string{}
	type finalRow struct {
		key   string
		raw   rawSpecies
		order int
	}
	var finals []finalRow
	for i, r := range raw {
		if excluded[i] {
			continue
		}
		key := fmt.Sprintf("%04d-%03d", r.dexNo, r.form)
		keyByNameEn[r.nameEn] = key
		baseSpeciesName[r.showdownID] = toID(r.baseSpeciesName)
		finals = append(finals, finalRow{key: key, raw: r})
	}

	pokeAPISpecies := newPokeAPILookup(in.PokeAPI.Species)
	pokeAPIForms := newPokeAPILookup(in.PokeAPI.Forms)

	var rows []SpeciesRow
	var regulationKeys []string
	regulationAbilitySet := map[string]bool{}
	for _, f := range finals {
		r := f.raw
		var baseKey, itemID string
		if r.isMega {
			bk, ok := keyByNameEn[r.baseSpeciesName]
			if !ok {
				return speciesConversion{}, nil, nil, fmt.Errorf("%w: メガ %q の基本種 %q が取り込まれていない", ErrInvalidData, r.nameEn, r.baseSpeciesName)
			}
			baseKey = bk
			itemID = toID(r.requiredItemName)
		}

		var names map[string]string
		if r.form == 0 {
			names = pokeAPISpecies[r.showdownID]
		} else {
			names = pokeAPIForms[r.showdownID]
		}
		res := resolveJaName(r.showdownID, in.Overrides.Species, names, in.Config.NameJaLanguages, r.nameEn, usedOverride)
		if res.Source == "fallback_en" {
			warnings = append(warnings, Finding{Kind: KindNameFallback, ID: r.showdownID})
		}

		rows = append(rows, SpeciesRow{
			SpeciesRow: master.SpeciesRow{
				Key: f.key, DexNo: r.dexNo, Form: r.form, ShowdownID: r.showdownID,
				NameJa: res.NameJa, NameEn: r.nameEn, Type1: r.type1, Type2: r.type2,
				BaseHP: r.stats[0], BaseAtk: r.stats[1], BaseDef: r.stats[2], BaseSpA: r.stats[3], BaseSpD: r.stats[4], BaseSpe: r.stats[5],
				IsMega: r.isMega, BaseSpeciesKey: baseKey, RequiredItemID: itemID,
			},
			NameJaSource: res.Source,
			Abilities:    r.abilities,
		})
		if !r.support {
			regulationKeys = append(regulationKeys, f.key)
			for _, ability := range r.abilities {
				regulationAbilitySet[ability.AbilityID] = true
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Key < rows[j].Key })
	sort.Strings(regulationKeys)

	return speciesConversion{
		Rows: rows, RegulationKeys: regulationKeys, RegulationAbilityIDs: sortedKeysRaw(regulationAbilitySet),
		BaseSpeciesName: baseSpeciesName, AbilityNameEn: abilityNameEn,
	}, warnings, nil, nil
}
