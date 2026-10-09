package master

// 技の処理の定義(effects.json の moveRules・move_rules.rule。ADR-0143 §4)の JSON デコード/エンコード。
//
// effects と同じ流儀で厳格にする: 未知のキー・大文字小文字違い・空・false・語彙に無い値・相性表に無いタイプを拒否する。
// 値の意味の検証(語彙・値域・一覧の形)は engine.Move.ValidateRule に任せる(機構との対応は Move が実際の技で確かめる)。
// エンコードは正準形(ゼロ値省略・struct 定義順・map キー昇順)で、Decode(Encode(r)) == r。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"

	"example.com/pokecalc/engine"
)

var (
	moveRuleFields = map[string]bool{
		"PowerFormula": true, "PowerBoosts": true, "IgnoresBurn": true, "TerrainPowerMods": true,
		"TypeByWeather": true, "TypeByTerrain": true, "ExtraEffectivenessType": true, "SuperEffectiveAgainst": true,
		"PriorityBoost": true, "BreaksScreens": true, "FailsWithoutDefenderItem": true, "SpreadInTerrain": true,
		"MoveSpecificResolved": true,
	}
	movePowerBoostFields  = map[string]bool{"Condition": true, "Statuses": true, "Weathers": true, "Terrains": true, "BaseMultiplier": true, "Modifier": true}
	terrainPowerModFields = map[string]bool{"Terrain": true, "Modifier": true}
	priorityBoostFields   = map[string]bool{"Terrain": true, "Delta": true}
)

// DecodeMoveRule は move_rules.rule(effects.json の moveRules の値)を検証つきで engine.MoveRule にする。
// 失敗は ErrInvalidEffect。
func DecodeMoveRule(raw []byte, chart engine.TypeChart) (*engine.MoveRule, error) {
	if chart.IsZero() {
		return nil, fmt.Errorf("%w: タイプ相性表が未設定", ErrInvalidEffect)
	}
	fields, err := decodeEffectObject(raw)
	if err != nil {
		return nil, err
	}
	if err := rejectUnknownFields(fields, moveRuleFields); err != nil {
		return nil, err
	}
	var r engine.MoveRule
	if v, ok := fields["PowerFormula"]; ok {
		s, err := decodeNonEmptyString(v, "PowerFormula")
		if err != nil {
			return nil, err
		}
		r.PowerFormula = engine.PowerFormula(s)
	}
	if v, ok := fields["PowerBoosts"]; ok {
		if r.PowerBoosts, err = decodeMovePowerBoosts(v); err != nil {
			return nil, err
		}
	}
	for name, dst := range map[string]*bool{
		"IgnoresBurn": &r.IgnoresBurn, "BreaksScreens": &r.BreaksScreens,
		"FailsWithoutDefenderItem": &r.FailsWithoutDefenderItem, "MoveSpecificResolved": &r.MoveSpecificResolved,
	} {
		v, ok := fields[name]
		if !ok {
			continue
		}
		if *dst, err = decodeTrueLiteral(v); err != nil {
			return nil, err
		}
	}
	if v, ok := fields["TerrainPowerMods"]; ok {
		if r.TerrainPowerMods, err = decodeTerrainPowerMods(v); err != nil {
			return nil, err
		}
	}
	if v, ok := fields["TypeByWeather"]; ok {
		m, err := decodeStringTypeMap(v, "TypeByWeather", chart)
		if err != nil {
			return nil, err
		}
		r.TypeByWeather = make(map[engine.Weather]engine.Type, len(m))
		for k, t := range m {
			r.TypeByWeather[engine.Weather(k)] = t
		}
	}
	if v, ok := fields["TypeByTerrain"]; ok {
		m, err := decodeStringTypeMap(v, "TypeByTerrain", chart)
		if err != nil {
			return nil, err
		}
		r.TypeByTerrain = make(map[engine.Terrain]engine.Type, len(m))
		for k, t := range m {
			r.TypeByTerrain[engine.Terrain(k)] = t
		}
	}
	if v, ok := fields["ExtraEffectivenessType"]; ok {
		if r.ExtraEffectivenessType, err = decodeKnownType(v, chart); err != nil {
			return nil, err
		}
	}
	if v, ok := fields["SuperEffectiveAgainst"]; ok {
		if r.SuperEffectiveAgainst, err = decodeTypeList(v, "SuperEffectiveAgainst", chart); err != nil {
			return nil, err
		}
	}
	if v, ok := fields["PriorityBoost"]; ok {
		if r.PriorityBoost, err = decodePriorityBoost(v); err != nil {
			return nil, err
		}
	}
	if v, ok := fields["SpreadInTerrain"]; ok {
		s, err := decodeNonEmptyString(v, "SpreadInTerrain")
		if err != nil {
			return nil, err
		}
		r.SpreadInTerrain = engine.Terrain(s)
	}
	// 語彙・値域・一覧の形は engine の検証に任せる。機構との対応は実際の技で確かめる(Move)ので、ここでは全機構を持つ技で通す。
	probe := engine.Move{
		ID: "rule", Category: engine.CategoryPhysical, Mechanisms: engine.AllMoveMechanisms(),
		Params: engine.MechanismParams{MultiHit: &engine.MultiHit{Min: 2, Max: 2}}, Rule: &r,
	}
	if err := probe.ValidateRule(chart); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidEffect, err)
	}
	return &r, nil
}

func decodeNonEmptyString(raw json.RawMessage, name string) (string, error) {
	s, err := decodeStrictString(raw)
	if err != nil {
		return "", err
	}
	if s == "" {
		return "", fmt.Errorf("%w: %s が空", ErrInvalidEffect, name)
	}
	return s, nil
}

// decodeNonEmptyArray は空でない JSON 配列の要素を返す。
func decodeNonEmptyArray(raw json.RawMessage, name string) ([]json.RawMessage, error) {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, fmt.Errorf("%w: %s が配列でない: %v", ErrInvalidEffect, name, err)
	}
	if len(arr) == 0 {
		return nil, fmt.Errorf("%w: %s が空", ErrInvalidEffect, name)
	}
	return arr, nil
}

// decodeStringList は空でない文字列の配列を読む。
func decodeStringList(raw json.RawMessage, name string) ([]string, error) {
	arr, err := decodeNonEmptyArray(raw, name)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(arr))
	for _, el := range arr {
		s, err := decodeNonEmptyString(el, name)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func decodeTypeList(raw json.RawMessage, name string, chart engine.TypeChart) ([]engine.Type, error) {
	arr, err := decodeNonEmptyArray(raw, name)
	if err != nil {
		return nil, err
	}
	out := make([]engine.Type, 0, len(arr))
	for _, el := range arr {
		t, err := decodeKnownType(el, chart)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

// decodeStringTypeMap は {"<語彙の値>": "<タイプ>"} の空でないオブジェクトを読む(キーの語彙は engine の検証が見る)。
func decodeStringTypeMap(raw json.RawMessage, name string, chart engine.TypeChart) (map[string]engine.Type, error) {
	obj, err := decodeNonEmptyObject(raw, name)
	if err != nil {
		return nil, err
	}
	out := make(map[string]engine.Type, len(obj))
	for k, v := range obj {
		if out[k], err = decodeKnownType(v, chart); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func decodeMovePowerBoosts(raw json.RawMessage) ([]engine.MovePowerBoost, error) {
	arr, err := decodeNonEmptyArray(raw, "PowerBoosts")
	if err != nil {
		return nil, err
	}
	out := make([]engine.MovePowerBoost, 0, len(arr))
	for _, el := range arr {
		fields, err := decodeEffectSubObject(el, "PowerBoosts の要素", movePowerBoostFields)
		if err != nil {
			return nil, err
		}
		condRaw, ok := fields["Condition"]
		if !ok {
			return nil, fmt.Errorf("%w: PowerBoosts の要素に Condition が要る", ErrInvalidEffect)
		}
		cond, err := decodeNonEmptyString(condRaw, "Condition")
		if err != nil {
			return nil, err
		}
		b := engine.MovePowerBoost{Condition: engine.MoveCondition(cond)}
		if v, ok := fields["Statuses"]; ok {
			ss, err := decodeStringList(v, "Statuses")
			if err != nil {
				return nil, err
			}
			for _, s := range ss {
				b.Statuses = append(b.Statuses, engine.Status(s))
			}
		}
		if v, ok := fields["Weathers"]; ok {
			ss, err := decodeStringList(v, "Weathers")
			if err != nil {
				return nil, err
			}
			for _, s := range ss {
				b.Weathers = append(b.Weathers, engine.Weather(s))
			}
		}
		if v, ok := fields["Terrains"]; ok {
			ss, err := decodeStringList(v, "Terrains")
			if err != nil {
				return nil, err
			}
			for _, s := range ss {
				b.Terrains = append(b.Terrains, engine.Terrain(s))
			}
		}
		if v, ok := fields["BaseMultiplier"]; ok {
			if b.BaseMultiplier, err = decodePositiveInt(v); err != nil {
				return nil, err
			}
		}
		if v, ok := fields["Modifier"]; ok {
			if b.Modifier, err = decodePositiveInt(v); err != nil {
				return nil, err
			}
		}
		out = append(out, b)
	}
	return out, nil
}

func decodeTerrainPowerMods(raw json.RawMessage) ([]engine.TerrainPowerMod, error) {
	arr, err := decodeNonEmptyArray(raw, "TerrainPowerMods")
	if err != nil {
		return nil, err
	}
	out := make([]engine.TerrainPowerMod, 0, len(arr))
	for _, el := range arr {
		fields, err := decodeEffectSubObject(el, "TerrainPowerMods の要素", terrainPowerModFields)
		if err != nil {
			return nil, err
		}
		tRaw, okT := fields["Terrain"]
		mRaw, okM := fields["Modifier"]
		if !okT || !okM {
			return nil, fmt.Errorf("%w: TerrainPowerMods の要素は Terrain と Modifier が要る", ErrInvalidEffect)
		}
		t, err := decodeNonEmptyString(tRaw, "Terrain")
		if err != nil {
			return nil, err
		}
		m, err := decodePositiveInt(mRaw)
		if err != nil {
			return nil, err
		}
		out = append(out, engine.TerrainPowerMod{Terrain: engine.Terrain(t), Modifier: m})
	}
	return out, nil
}

func decodePriorityBoost(raw json.RawMessage) (*engine.PriorityBoost, error) {
	fields, err := decodeEffectSubObject(raw, "PriorityBoost", priorityBoostFields)
	if err != nil {
		return nil, err
	}
	tRaw, okT := fields["Terrain"]
	dRaw, okD := fields["Delta"]
	if !okT || !okD {
		return nil, fmt.Errorf("%w: PriorityBoost は Terrain と Delta が要る", ErrInvalidEffect)
	}
	t, err := decodeNonEmptyString(tRaw, "Terrain")
	if err != nil {
		return nil, err
	}
	if !integerLiteral.MatchString(string(dRaw)) {
		return nil, fmt.Errorf("%w: Delta が整数でない: %s", ErrInvalidEffect, dRaw)
	}
	d, err := strconv.Atoi(string(dRaw))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidEffect, err)
	}
	return &engine.PriorityBoost{Terrain: engine.Terrain(t), Delta: d}, nil
}

// EncodeMoveRule は engine.MoveRule を正準形の JSON にする(ゼロ値省略・struct 定義順・map のキーは昇順)。
func EncodeMoveRule(r engine.MoveRule) ([]byte, error) {
	w := newEffectWriter()
	if r.PowerFormula != "" {
		w.field("PowerFormula", quoteJSON(string(r.PowerFormula)))
	}
	if len(r.PowerBoosts) > 0 {
		var buf bytes.Buffer
		buf.WriteByte('[')
		for i, b := range r.PowerBoosts {
			if i > 0 {
				buf.WriteByte(',')
			}
			bw := newEffectWriter()
			bw.field("Condition", quoteJSON(string(b.Condition)))
			if len(b.Statuses) > 0 {
				bw.field("Statuses", encodeStringList(b.Statuses))
			}
			if len(b.Weathers) > 0 {
				bw.field("Weathers", encodeStringList(b.Weathers))
			}
			if len(b.Terrains) > 0 {
				bw.field("Terrains", encodeStringList(b.Terrains))
			}
			if b.BaseMultiplier != 0 {
				bw.field("BaseMultiplier", []byte(strconv.Itoa(b.BaseMultiplier)))
			}
			if b.Modifier != 0 {
				bw.field("Modifier", []byte(strconv.Itoa(b.Modifier)))
			}
			bw.buf.WriteByte('}')
			buf.Write(bw.buf.Bytes())
		}
		buf.WriteByte(']')
		w.field("PowerBoosts", buf.Bytes())
	}
	if r.IgnoresBurn {
		w.field("IgnoresBurn", []byte("true"))
	}
	if len(r.TerrainPowerMods) > 0 {
		var buf bytes.Buffer
		buf.WriteByte('[')
		for i, m := range r.TerrainPowerMods {
			if i > 0 {
				buf.WriteByte(',')
			}
			fmt.Fprintf(&buf, `{"Terrain":%s,"Modifier":%d}`, quoteJSON(string(m.Terrain)), m.Modifier)
		}
		buf.WriteByte(']')
		w.field("TerrainPowerMods", buf.Bytes())
	}
	if len(r.TypeByWeather) > 0 {
		w.field("TypeByWeather", encodeKeyedTypes(r.TypeByWeather))
	}
	if len(r.TypeByTerrain) > 0 {
		w.field("TypeByTerrain", encodeKeyedTypes(r.TypeByTerrain))
	}
	if r.ExtraEffectivenessType != "" {
		w.field("ExtraEffectivenessType", quoteJSON(string(r.ExtraEffectivenessType)))
	}
	if len(r.SuperEffectiveAgainst) > 0 {
		w.field("SuperEffectiveAgainst", encodeStringList(r.SuperEffectiveAgainst))
	}
	if pb := r.PriorityBoost; pb != nil {
		w.field("PriorityBoost", fmt.Appendf(nil, `{"Terrain":%s,"Delta":%d}`, quoteJSON(string(pb.Terrain)), pb.Delta))
	}
	if r.BreaksScreens {
		w.field("BreaksScreens", []byte("true"))
	}
	if r.FailsWithoutDefenderItem {
		w.field("FailsWithoutDefenderItem", []byte("true"))
	}
	if r.SpreadInTerrain != "" {
		w.field("SpreadInTerrain", quoteJSON(string(r.SpreadInTerrain)))
	}
	if r.MoveSpecificResolved {
		w.field("MoveSpecificResolved", []byte("true"))
	}
	return w.bytes()
}

// encodeStringList は文字列型の配列を、与えられた順の JSON 配列にする。
func encodeStringList[T ~string](vs []T) []byte {
	var buf bytes.Buffer
	buf.WriteByte('[')
	for i, v := range vs {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.Write(quoteJSON(string(v)))
	}
	buf.WriteByte(']')
	return buf.Bytes()
}

// encodeKeyedTypes は {"<キー>":"<タイプ>"} をキーの昇順で書く。
func encodeKeyedTypes[K ~string](m map[K]engine.Type) []byte {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range slices.Sorted(maps.Keys(m)) {
		if i > 0 {
			buf.WriteByte(',')
		}
		fmt.Fprintf(&buf, "%s:%s", quoteJSON(string(k)), quoteJSON(string(m[k])))
	}
	buf.WriteByte('}')
	return buf.Bytes()
}
