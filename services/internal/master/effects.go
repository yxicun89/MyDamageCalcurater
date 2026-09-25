package master

// 効果定義(item_effects.effect / ability_effects.effect)の JSON デコード/エンコード(ADR-0100 §6)。
//
// デコードは厳格にする: 未知のフィールド・大文字小文字違い・オブジェクト以外・後続データ・
// 空・負の値・小数(4096基準の整数のみ)を拒否する。encoding/json の Unmarshal は
// フィールド名の大文字小文字を無視して一致させてしまうため、標準の struct デコードは使わず
// map[string]json.RawMessage を経由してキーを厳密比較する。
//
// エンコードは正準形(ゼロ値省略・struct 定義順・map キー昇順)にし、
// Decode(Encode(e)) == e であることをテストで保証する(情報が落ちない)。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"

	"example.com/pokecalc/engine"
)

// integerLiteral は 4096 基準の整数として認める JSON トークンの形。小数・指数表記・
// 引用符付き文字列を弾く。
var integerLiteral = regexp.MustCompile(`^-?[0-9]+$`)

// statModKeys は ItemEffect.StatMods のキーとして許すもの(HP は含まない)。
var statModKeys = map[string]engine.StatKey{
	"atk": engine.StatAtk,
	"def": engine.StatDef,
	"spa": engine.StatSpA,
	"spd": engine.StatSpD,
	"spe": engine.StatSpe,
}

// itemEffectFields / abilityEffectFields は既知のフィールド名(大文字小文字を区別)。
var (
	itemEffectFields = map[string]bool{
		"StatMods": true, "DamageMod": true, "PowerMod": true, "PowerCategory": true,
		"OnlySuperEffective": true, "BoostType": true, "BoostTypeMod": true, "ResistBerryType": true,
	}
	abilityEffectFields = map[string]bool{
		"StabMod": true, "OffBoostType": true, "OffBoostTypeMod": true,
		"DefResistType": true, "DefImmuneTypes": true, "DefAbsorbTypes": true,
		"ReduceSuperEffective": true, "IgnoresBurn": true, "Airborne": true,
	}
	// absorbEffectFields は DefAbsorbTypes の値(1タイプぶんの副次効果)の既知のフィールド名。
	absorbEffectFields = map[string]bool{
		"HealNumerator": true, "HealDenominator": true, "BoostStat": true, "BoostStages": true,
	}
)

// decodeEffectObject は raw を JSON オブジェクトとして厳格に読む: 単一の JSON 値であること
// (後続データを許さない)、null でないこと、空でないこと(補正が1つも無い定義は不正)。
func decodeEffectObject(raw []byte) (map[string]json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	var fields map[string]json.RawMessage
	if err := dec.Decode(&fields); err != nil {
		return nil, fmt.Errorf("%w: JSON オブジェクトとして読めない: %v", ErrInvalidEffect, err)
	}
	if dec.More() {
		return nil, fmt.Errorf("%w: 後続のデータがある", ErrInvalidEffect)
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("%w: 効果が空(補正が1つも無い)", ErrInvalidEffect)
	}
	return fields, nil
}

func rejectUnknownFields(fields map[string]json.RawMessage, known map[string]bool) error {
	for k := range fields {
		if !known[k] {
			return fmt.Errorf("%w: 未知のフィールド %q", ErrInvalidEffect, k)
		}
	}
	return nil
}

// decodePositiveInt は 4096 基準の正の整数(1以上)だけを認める。
func decodePositiveInt(raw json.RawMessage) (int, error) {
	s := string(raw)
	if !integerLiteral.MatchString(s) {
		return 0, fmt.Errorf("%w: 4096基準の整数でない: %s", ErrInvalidEffect, s)
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidEffect, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%w: 正の整数でない: %d", ErrInvalidEffect, n)
	}
	return n, nil
}

// decodeTrueLiteral は真偽値のうち true だけを認める(false は「補正なし」と区別が付かない)。
func decodeTrueLiteral(raw json.RawMessage) (bool, error) {
	if string(raw) != "true" {
		return false, fmt.Errorf("%w: true 以外の真偽値: %s", ErrInvalidEffect, raw)
	}
	return true, nil
}

// decodeStrictString は raw が JSON 文字列であることを要求する(数値・オブジェクト等を拒否)。
func decodeStrictString(raw json.RawMessage) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", fmt.Errorf("%w: 文字列でない: %v", ErrInvalidEffect, err)
	}
	return s, nil
}

// decodeKnownType は表(chart.Has)にあるタイプ ID だけを認める。
func decodeKnownType(raw json.RawMessage, chart engine.TypeChart) (engine.Type, error) {
	s, err := decodeStrictString(raw)
	if err != nil {
		return "", err
	}
	if s == "" || !chart.Has(engine.Type(s)) {
		return "", fmt.Errorf("%w: タイプが表に無い: %q", ErrInvalidEffect, s)
	}
	return engine.Type(s), nil
}

// decodePowerCategory は PowerMod の対象分類として physical/special だけを認める
// (status は変化技であり威力を持たないため対象にならない)。
func decodePowerCategory(raw json.RawMessage) (engine.MoveCategory, error) {
	s, err := decodeStrictString(raw)
	if err != nil {
		return "", err
	}
	switch s {
	case "physical":
		return engine.CategoryPhysical, nil
	case "special":
		return engine.CategorySpecial, nil
	default:
		return "", fmt.Errorf("%w: PowerCategory が不正: %q", ErrInvalidEffect, s)
	}
}

// decodeStatMods は ItemEffect.StatMods を検証つきで読む(atk/def/spa/spd/spe のみ、正の整数、空不可)。
func decodeStatMods(raw json.RawMessage) (map[engine.StatKey]int, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("%w: StatMods がオブジェクトでない: %v", ErrInvalidEffect, err)
	}
	if len(obj) == 0 {
		return nil, fmt.Errorf("%w: StatMods が空", ErrInvalidEffect)
	}
	out := make(map[engine.StatKey]int, len(obj))
	for k, v := range obj {
		key, ok := statModKeys[k]
		if !ok {
			return nil, fmt.Errorf("%w: StatMods のキーが不正: %q", ErrInvalidEffect, k)
		}
		n, err := decodePositiveInt(v)
		if err != nil {
			return nil, err
		}
		out[key] = n
	}
	return out, nil
}

// decodeDefResistType は AbilityEffect.DefResistType を検証つきで読む(表にあるタイプのみ、
// 正の整数、空不可)。
func decodeDefResistType(raw json.RawMessage, chart engine.TypeChart) (map[engine.Type]int, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("%w: DefResistType がオブジェクトでない: %v", ErrInvalidEffect, err)
	}
	if len(obj) == 0 {
		return nil, fmt.Errorf("%w: DefResistType が空", ErrInvalidEffect)
	}
	out := make(map[engine.Type]int, len(obj))
	for k, v := range obj {
		if !chart.Has(engine.Type(k)) {
			return nil, fmt.Errorf("%w: DefResistType のタイプが表に無い: %q", ErrInvalidEffect, k)
		}
		n, err := decodePositiveInt(v)
		if err != nil {
			return nil, err
		}
		out[engine.Type(k)] = n
	}
	return out, nil
}

// decodeDefImmuneTypes は AbilityEffect.DefImmuneTypes を検証つきで読む(表にあるタイプの配列、
// 空不可、重複不可。ADR-0106 §決定5)。
func decodeDefImmuneTypes(raw json.RawMessage, chart engine.TypeChart) ([]engine.Type, error) {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, fmt.Errorf("%w: DefImmuneTypes が配列でない: %v", ErrInvalidEffect, err)
	}
	if len(arr) == 0 {
		return nil, fmt.Errorf("%w: DefImmuneTypes が空", ErrInvalidEffect)
	}
	seen := make(map[engine.Type]bool, len(arr))
	out := make([]engine.Type, 0, len(arr))
	for _, v := range arr {
		t, err := decodeKnownType(v, chart)
		if err != nil {
			return nil, err
		}
		if seen[t] {
			return nil, fmt.Errorf("%w: DefImmuneTypes にタイプが重複している: %q", ErrInvalidEffect, t)
		}
		seen[t] = true
		out = append(out, t)
	}
	return out, nil
}

// decodeAbsorbEffect は DefAbsorbTypes の1タイプぶんの副次効果を検証つきで読む。
// {} は「吸収するが副次効果は持たない」として正しい値(ADR-0106 §決定2)。
func decodeAbsorbEffect(raw json.RawMessage) (engine.AbsorbEffect, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return engine.AbsorbEffect{}, fmt.Errorf("%w: 吸収の値がオブジェクトでない: %v", ErrInvalidEffect, err)
	}
	if err := rejectUnknownFields(fields, absorbEffectFields); err != nil {
		return engine.AbsorbEffect{}, err
	}
	var out engine.AbsorbEffect
	num, hasNum := fields["HealNumerator"]
	den, hasDen := fields["HealDenominator"]
	if hasNum != hasDen {
		return engine.AbsorbEffect{}, fmt.Errorf("%w: HealNumerator と HealDenominator は組で指定する", ErrInvalidEffect)
	}
	if hasNum {
		n, err := decodePositiveInt(num)
		if err != nil {
			return engine.AbsorbEffect{}, err
		}
		d, err := decodePositiveInt(den)
		if err != nil {
			return engine.AbsorbEffect{}, err
		}
		if d > 16 {
			return engine.AbsorbEffect{}, fmt.Errorf("%w: HealDenominator は1..16の範囲でない: %d", ErrInvalidEffect, d)
		}
		if n > d {
			return engine.AbsorbEffect{}, fmt.Errorf("%w: 回復の割合が1を超える: %d/%d", ErrInvalidEffect, n, d)
		}
		out.HealNumerator, out.HealDenominator = n, d
	}
	stat, hasStat := fields["BoostStat"]
	stages, hasStages := fields["BoostStages"]
	if hasStat != hasStages {
		return engine.AbsorbEffect{}, fmt.Errorf("%w: BoostStat と BoostStages は組で指定する", ErrInvalidEffect)
	}
	if hasStat {
		s, err := decodeStrictString(stat)
		if err != nil {
			return engine.AbsorbEffect{}, err
		}
		key, ok := statModKeys[s]
		if !ok {
			return engine.AbsorbEffect{}, fmt.Errorf("%w: BoostStat が不正: %q", ErrInvalidEffect, s)
		}
		n, err := decodePositiveInt(stages)
		if err != nil {
			return engine.AbsorbEffect{}, err
		}
		if n > 6 {
			return engine.AbsorbEffect{}, fmt.Errorf("%w: BoostStages は1..6の範囲でない: %d", ErrInvalidEffect, n)
		}
		out.BoostStat, out.BoostStages = key, n
	}
	return out, nil
}

// decodeDefAbsorbTypes は AbilityEffect.DefAbsorbTypes を検証つきで読む(表にあるタイプのみ、空不可)。
func decodeDefAbsorbTypes(raw json.RawMessage, chart engine.TypeChart) (map[engine.Type]engine.AbsorbEffect, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("%w: DefAbsorbTypes がオブジェクトでない: %v", ErrInvalidEffect, err)
	}
	if len(obj) == 0 {
		return nil, fmt.Errorf("%w: DefAbsorbTypes が空", ErrInvalidEffect)
	}
	out := make(map[engine.Type]engine.AbsorbEffect, len(obj))
	for k, v := range obj {
		if !chart.Has(engine.Type(k)) {
			return nil, fmt.Errorf("%w: DefAbsorbTypes のタイプが表に無い: %q", ErrInvalidEffect, k)
		}
		abs, err := decodeAbsorbEffect(v)
		if err != nil {
			return nil, err
		}
		out[engine.Type(k)] = abs
	}
	return out, nil
}

// DecodeItemEffect は item_effects.effect の JSON を engine.ItemEffect に厳格デコードする。
func DecodeItemEffect(raw []byte, chart engine.TypeChart) (*engine.ItemEffect, error) {
	if chart.IsZero() {
		return nil, fmt.Errorf("%w: タイプ相性表が未設定", ErrInvalidEffect)
	}
	fields, err := decodeEffectObject(raw)
	if err != nil {
		return nil, err
	}
	if err := rejectUnknownFields(fields, itemEffectFields); err != nil {
		return nil, err
	}

	var e engine.ItemEffect
	if v, ok := fields["StatMods"]; ok {
		m, err := decodeStatMods(v)
		if err != nil {
			return nil, err
		}
		e.StatMods = m
	}
	if v, ok := fields["DamageMod"]; ok {
		n, err := decodePositiveInt(v)
		if err != nil {
			return nil, err
		}
		e.DamageMod = n
	}
	if v, ok := fields["PowerMod"]; ok {
		n, err := decodePositiveInt(v)
		if err != nil {
			return nil, err
		}
		e.PowerMod = n
	}
	if v, hasCategory := fields["PowerCategory"]; hasCategory {
		cat, err := decodePowerCategory(v)
		if err != nil {
			return nil, err
		}
		if e.PowerMod == 0 {
			return nil, fmt.Errorf("%w: PowerCategory だけで PowerMod が無い", ErrInvalidEffect)
		}
		e.PowerCategory = cat
	}
	if v, ok := fields["OnlySuperEffective"]; ok {
		b, err := decodeTrueLiteral(v)
		if err != nil {
			return nil, err
		}
		if e.DamageMod == 0 {
			return nil, fmt.Errorf("%w: OnlySuperEffective だけで DamageMod が無い", ErrInvalidEffect)
		}
		e.OnlySuperEffective = b
	}
	boostType, hasBoostType := fields["BoostType"]
	boostTypeMod, hasBoostTypeMod := fields["BoostTypeMod"]
	if hasBoostType != hasBoostTypeMod {
		return nil, fmt.Errorf("%w: BoostType と BoostTypeMod は組で指定する", ErrInvalidEffect)
	}
	if hasBoostType {
		t, err := decodeKnownType(boostType, chart)
		if err != nil {
			return nil, err
		}
		n, err := decodePositiveInt(boostTypeMod)
		if err != nil {
			return nil, err
		}
		e.BoostType, e.BoostTypeMod = t, n
	}
	if v, ok := fields["ResistBerryType"]; ok {
		t, err := decodeKnownType(v, chart)
		if err != nil {
			return nil, err
		}
		e.ResistBerryType = t
	}
	return &e, nil
}

// DecodeAbilityEffect は ability_effects.effect の JSON を engine.AbilityEffect に厳格デコードする。
func DecodeAbilityEffect(raw []byte, chart engine.TypeChart) (*engine.AbilityEffect, error) {
	if chart.IsZero() {
		return nil, fmt.Errorf("%w: タイプ相性表が未設定", ErrInvalidEffect)
	}
	fields, err := decodeEffectObject(raw)
	if err != nil {
		return nil, err
	}
	if err := rejectUnknownFields(fields, abilityEffectFields); err != nil {
		return nil, err
	}

	var e engine.AbilityEffect
	if v, ok := fields["StabMod"]; ok {
		n, err := decodePositiveInt(v)
		if err != nil {
			return nil, err
		}
		e.StabMod = n
	}
	offType, hasOffType := fields["OffBoostType"]
	offTypeMod, hasOffTypeMod := fields["OffBoostTypeMod"]
	if hasOffType != hasOffTypeMod {
		return nil, fmt.Errorf("%w: OffBoostType と OffBoostTypeMod は組で指定する", ErrInvalidEffect)
	}
	if hasOffType {
		t, err := decodeKnownType(offType, chart)
		if err != nil {
			return nil, err
		}
		n, err := decodePositiveInt(offTypeMod)
		if err != nil {
			return nil, err
		}
		e.OffBoostType, e.OffBoostTypeMod = t, n
	}
	if v, ok := fields["DefResistType"]; ok {
		m, err := decodeDefResistType(v, chart)
		if err != nil {
			return nil, err
		}
		e.DefResistType = m
	}
	if v, ok := fields["DefImmuneTypes"]; ok {
		m, err := decodeDefImmuneTypes(v, chart)
		if err != nil {
			return nil, err
		}
		e.DefImmuneTypes = m
	}
	if v, ok := fields["DefAbsorbTypes"]; ok {
		m, err := decodeDefAbsorbTypes(v, chart)
		if err != nil {
			return nil, err
		}
		e.DefAbsorbTypes = m
	}
	for _, t := range e.DefImmuneTypes {
		if _, dup := e.DefAbsorbTypes[t]; dup {
			return nil, fmt.Errorf("%w: タイプ %q が DefImmuneTypes と DefAbsorbTypes の両方にある", ErrInvalidEffect, t)
		}
	}
	if v, ok := fields["ReduceSuperEffective"]; ok {
		n, err := decodePositiveInt(v)
		if err != nil {
			return nil, err
		}
		e.ReduceSuperEffective = n
	}
	if v, ok := fields["IgnoresBurn"]; ok {
		b, err := decodeTrueLiteral(v)
		if err != nil {
			return nil, err
		}
		e.IgnoresBurn = b
	}
	if v, ok := fields["Airborne"]; ok {
		b, err := decodeTrueLiteral(v)
		if err != nil {
			return nil, err
		}
		e.Airborne = b
	}
	return &e, nil
}

// quoteJSON は文字列を JSON 文字列リテラルにする(エスケープは encoding/json に任せる)。
func quoteJSON(s string) []byte {
	b, _ := json.Marshal(s)
	return b
}

// encodeStatMods はキー昇順の JSON オブジェクトを作る(正準形)。
func encodeStatMods(m map[engine.StatKey]int) []byte {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		fmt.Fprintf(&buf, "%q:%d", k, m[engine.StatKey(k)])
	}
	buf.WriteByte('}')
	return buf.Bytes()
}

// encodeDefResistType はキー昇順の JSON オブジェクトを作る(正準形)。
func encodeDefResistType(m map[engine.Type]int) []byte {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		fmt.Fprintf(&buf, "%q:%d", k, m[engine.Type(k)])
	}
	buf.WriteByte('}')
	return buf.Bytes()
}

// encodeDefImmuneTypes はタイプ ID 昇順の JSON 配列を作る(正準形)。
func encodeDefImmuneTypes(types []engine.Type) []byte {
	keys := make([]string, 0, len(types))
	for _, t := range types {
		keys = append(keys, string(t))
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	buf.WriteByte('[')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.Write(quoteJSON(k))
	}
	buf.WriteByte(']')
	return buf.Bytes()
}

// encodeAbsorbEffect は struct 定義順(ゼロ値省略)で1タイプぶんの吸収効果を組み立てる。
func encodeAbsorbEffect(a engine.AbsorbEffect) []byte {
	w := newEffectWriter()
	if a.HealNumerator != 0 {
		w.field("HealNumerator", []byte(strconv.Itoa(a.HealNumerator)))
	}
	if a.HealDenominator != 0 {
		w.field("HealDenominator", []byte(strconv.Itoa(a.HealDenominator)))
	}
	if a.BoostStat != "" {
		w.field("BoostStat", quoteJSON(string(a.BoostStat)))
	}
	if a.BoostStages != 0 {
		w.field("BoostStages", []byte(strconv.Itoa(a.BoostStages)))
	}
	w.buf.WriteByte('}')
	return w.buf.Bytes()
}

// encodeDefAbsorbTypes はタイプ ID 昇順の JSON オブジェクトを作る(正準形)。
func encodeDefAbsorbTypes(m map[engine.Type]engine.AbsorbEffect) []byte {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		fmt.Fprintf(&buf, "%q:", k)
		buf.Write(encodeAbsorbEffect(m[engine.Type(k)]))
	}
	buf.WriteByte('}')
	return buf.Bytes()
}

// effectWriter は正準形(ゼロ値省略・struct 定義順)で JSON オブジェクトを組み立てる。
type effectWriter struct {
	buf   bytes.Buffer
	first bool
}

func newEffectWriter() *effectWriter {
	w := &effectWriter{first: true}
	w.buf.WriteByte('{')
	return w
}

func (w *effectWriter) field(key string, value []byte) {
	if !w.first {
		w.buf.WriteByte(',')
	}
	w.first = false
	w.buf.WriteByte('"')
	w.buf.WriteString(key)
	w.buf.WriteString(`":`)
	w.buf.Write(value)
}

// bytes は組み立てた JSON を返す。何もフィールドが書かれていなければ ErrInvalidEffect
// (空の効果定義=補正なしは不正。DB でも意味を持たない行になるため)。
func (w *effectWriter) bytes() ([]byte, error) {
	if w.first {
		return nil, fmt.Errorf("%w: 効果が空(補正が1つも無い)", ErrInvalidEffect)
	}
	w.buf.WriteByte('}')
	return w.buf.Bytes(), nil
}

// EncodeItemEffect は engine.ItemEffect を正準形の JSON にする
// (ゼロ値のフィールドを省く・struct 定義順・map のキーは昇順)。
func EncodeItemEffect(e engine.ItemEffect) ([]byte, error) {
	w := newEffectWriter()
	if len(e.StatMods) > 0 {
		w.field("StatMods", encodeStatMods(e.StatMods))
	}
	if e.DamageMod != 0 {
		w.field("DamageMod", []byte(strconv.Itoa(e.DamageMod)))
	}
	if e.PowerMod != 0 {
		w.field("PowerMod", []byte(strconv.Itoa(e.PowerMod)))
	}
	if e.PowerCategory != "" {
		w.field("PowerCategory", quoteJSON(string(e.PowerCategory)))
	}
	if e.OnlySuperEffective {
		w.field("OnlySuperEffective", []byte("true"))
	}
	if e.BoostType != "" {
		w.field("BoostType", quoteJSON(string(e.BoostType)))
	}
	if e.BoostTypeMod != 0 {
		w.field("BoostTypeMod", []byte(strconv.Itoa(e.BoostTypeMod)))
	}
	if e.ResistBerryType != "" {
		w.field("ResistBerryType", quoteJSON(string(e.ResistBerryType)))
	}
	return w.bytes()
}

// EncodeAbilityEffect は engine.AbilityEffect を正準形の JSON にする。
func EncodeAbilityEffect(e engine.AbilityEffect) ([]byte, error) {
	w := newEffectWriter()
	if e.StabMod != 0 {
		w.field("StabMod", []byte(strconv.Itoa(e.StabMod)))
	}
	if e.OffBoostType != "" {
		w.field("OffBoostType", quoteJSON(string(e.OffBoostType)))
	}
	if e.OffBoostTypeMod != 0 {
		w.field("OffBoostTypeMod", []byte(strconv.Itoa(e.OffBoostTypeMod)))
	}
	if len(e.DefResistType) > 0 {
		w.field("DefResistType", encodeDefResistType(e.DefResistType))
	}
	if len(e.DefImmuneTypes) > 0 {
		w.field("DefImmuneTypes", encodeDefImmuneTypes(e.DefImmuneTypes))
	}
	if len(e.DefAbsorbTypes) > 0 {
		w.field("DefAbsorbTypes", encodeDefAbsorbTypes(e.DefAbsorbTypes))
	}
	if e.ReduceSuperEffective != 0 {
		w.field("ReduceSuperEffective", []byte(strconv.Itoa(e.ReduceSuperEffective)))
	}
	if e.IgnoresBurn {
		w.field("IgnoresBurn", []byte("true"))
	}
	if e.Airborne {
		w.field("Airborne", []byte("true"))
	}
	return w.bytes()
}
