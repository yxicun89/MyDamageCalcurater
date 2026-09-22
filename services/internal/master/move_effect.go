package master

// 技の追加効果(move_effects.effect)の JSON デコード/エンコード(ADR-0107 決定4)。
//
// effects.go(item_effects / ability_effects)と同じ作法を再利用する: 未知のフィールド拒否・
// 大文字小文字の厳密比較・オブジェクト以外や後続データの拒否・空の拒否・正準エンコード
// (ゼロ値省略・struct 定義順・map キー昇順)・Decode(Encode(e)) == e。
//
// Stages の変化量は負・0 も入力として現れうる(0 は不正として拒否)ため、
// 常に正の整数だけを認める既存の decodePositiveInt は使えず、decodeStageInt を新設する。

import (
	"encoding/json"
	"fmt"
	"strconv"

	"example.com/pokecalc/engine"
)

// moveEffectFields は move_effects.effect の既知のフィールド名(大文字小文字を区別)。
var moveEffectFields = map[string]bool{"Chance": true, "Target": true, "Stages": true}

// decodeStageInt は技の追加効果のランク変化量(-6..+6、0 は不可)だけを認める。
func decodeStageInt(raw json.RawMessage) (int, error) {
	s := string(raw)
	if !integerLiteral.MatchString(s) {
		return 0, fmt.Errorf("%w: 変化量が整数でない: %s", ErrInvalidEffect, s)
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidEffect, err)
	}
	if n == 0 {
		return 0, fmt.Errorf("%w: 変化量が 0(意味を持たない)", ErrInvalidEffect)
	}
	if n < -6 || n > 6 {
		return 0, fmt.Errorf("%w: 変化量が -6..+6 の範囲外: %d", ErrInvalidEffect, n)
	}
	return n, nil
}

// decodeChance は発動確率(1..100)だけを認める。
func decodeChance(raw json.RawMessage) (int, error) {
	s := string(raw)
	if !integerLiteral.MatchString(s) {
		return 0, fmt.Errorf("%w: Chance が整数でない: %s", ErrInvalidEffect, s)
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidEffect, err)
	}
	if n < 1 || n > 100 {
		return 0, fmt.Errorf("%w: Chance が1..100の範囲外: %d", ErrInvalidEffect, n)
	}
	return n, nil
}

// decodeRankTarget は self/target だけを認める。
func decodeRankTarget(raw json.RawMessage) (engine.RankTarget, error) {
	s, err := decodeStrictString(raw)
	if err != nil {
		return "", err
	}
	switch engine.RankTarget(s) {
	case engine.RankTargetSelf, engine.RankTargetOpponent:
		return engine.RankTarget(s), nil
	default:
		return "", fmt.Errorf("%w: Target が不正: %q", ErrInvalidEffect, s)
	}
}

// decodeMoveEffectStages は Stages を検証つきで読む(atk/def/spa/spd/spe のみ、空不可)。
// キーは effects.go の statModKeys を再利用する(HP・accuracy・evasion は engine の Ranks に
// 持ち場が無いため、統一して未知のキーとして拒否される)。
func decodeMoveEffectStages(raw json.RawMessage) (map[engine.StatKey]int, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("%w: Stages がオブジェクトでない: %v", ErrInvalidEffect, err)
	}
	if len(obj) == 0 {
		return nil, fmt.Errorf("%w: Stages が空", ErrInvalidEffect)
	}
	out := make(map[engine.StatKey]int, len(obj))
	for k, v := range obj {
		key, ok := statModKeys[k]
		if !ok {
			return nil, fmt.Errorf("%w: Stages のキーが不正: %q", ErrInvalidEffect, k)
		}
		n, err := decodeStageInt(v)
		if err != nil {
			return nil, err
		}
		out[key] = n
	}
	return out, nil
}

// DecodeMoveEffect は move_effects.effect の JSON を engine.MoveEffect に厳格デコードする。
func DecodeMoveEffect(raw []byte) (*engine.MoveEffect, error) {
	fields, err := decodeEffectObject(raw)
	if err != nil {
		return nil, err
	}
	if err := rejectUnknownFields(fields, moveEffectFields); err != nil {
		return nil, err
	}
	chanceRaw, ok := fields["Chance"]
	if !ok {
		return nil, fmt.Errorf("%w: Chance が無い", ErrInvalidEffect)
	}
	targetRaw, ok := fields["Target"]
	if !ok {
		return nil, fmt.Errorf("%w: Target が無い", ErrInvalidEffect)
	}
	stagesRaw, ok := fields["Stages"]
	if !ok {
		return nil, fmt.Errorf("%w: Stages が無い", ErrInvalidEffect)
	}
	chance, err := decodeChance(chanceRaw)
	if err != nil {
		return nil, err
	}
	target, err := decodeRankTarget(targetRaw)
	if err != nil {
		return nil, err
	}
	stages, err := decodeMoveEffectStages(stagesRaw)
	if err != nil {
		return nil, err
	}
	e := engine.MoveEffect{Chance: chance, Target: target, Stages: stages}
	if err := e.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidEffect, err)
	}
	return &e, nil
}

// EncodeMoveEffect は engine.MoveEffect を正準形の JSON にする
// (struct 定義順 Chance → Target → Stages・Stages はキー昇順)。
func EncodeMoveEffect(e engine.MoveEffect) ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidEffect, err)
	}
	w := newEffectWriter()
	w.field("Chance", []byte(strconv.Itoa(e.Chance)))
	w.field("Target", quoteJSON(string(e.Target)))
	w.field("Stages", encodeStatMods(e.Stages))
	return w.bytes()
}
