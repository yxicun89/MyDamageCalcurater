package importer

// 技の機構(move_mechanisms)の変換(ADR-0121)。
//
// 取得元は Showdown の技データだけ(@smogon/calc は技の特殊な処理を技名の分岐で持ち、データに出ない)。
// 技の名前では分類しない(CLAUDE.md)。判定材料は次の3つ:
//  1. 技のデータのプロパティ(multihit・damage・ohko・willCrit・override*・ignoreDefensive)
//  2. 技のデータが持つ関数(ハンドラ)の名前
//  3. 天候・フィールドのハンドラが技 ID を名指ししている箇所(例: あるフィールドが特定の技の威力を変える)
//
// ハンドラの名前は Showdown のイベント名で、技の名前ではない。ダメージに効くか分からない名前
// (取得元の更新で増えたもの)は、黙って通常の技にせず安全側(move_specific / field_specific)に
// 分類して警告する。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"example.com/pokecalc/services/internal/master"
)

// MoveMechanismRow は move_mechanisms の行(技1つ・機構1つ)。
type MoveMechanismRow struct {
	MoveID    string
	Mechanism string
}

// moveHookMechanisms は技のハンドラ名 → 機構。値が空文字のものは「ダメージの量に効かない」と
// 確かめたハンドラ(使えるか・失敗するか・当たった後の処理・溜め等。ADR-0121 の表)。
var moveHookMechanisms = map[string]master.MoveMechanism{
	"basePowerCallback": master.MechanismVariablePower,
	"onBasePower":       master.MechanismVariablePower,
	"damageCallback":    master.MechanismFixedDamage,
	"onModifyType":      master.MechanismTypeChange,
	"onEffectiveness":   master.MechanismEffectivenessChange,
	"onModifyPriority":  master.MechanismPriorityChange,
	// 技の性質(威力・分類・タイプ・命中 等)の書き換え・当たる前・使う前の処理。ダメージに効くかを
	// 名前だけでは決められないので安全側に倒す。
	"onModifyMove": master.MechanismMoveSpecific,
	"onTryHit":     master.MechanismMoveSpecific,
	"onPrepareHit": master.MechanismMoveSpecific,

	"beforeMoveCallback":       "",
	"beforeTurnCallback":       "",
	"onAfterHit":               "",
	"onAfterMove":              "",
	"onAfterMoveSecondarySelf": "",
	"onAfterSubDamage":         "",
	"onDisableMove":            "",
	"onHit":                    "",
	"onModifyTarget":           "",
	"onMoveFail":               "",
	"onTry":                    "",
	"onTryImmunity":            "",
	"onTryMove":                "",
	"priorityChargeCallback":   "",
}

// fieldHookAffectsDamage は天候・フィールドのハンドラ名 → そのハンドラが技を名指ししているとき
// ダメージに効くか。false は効かないと確かめたもの(例: 名指しが技でなく同名の状態を指す)。
var fieldHookAffectsDamage = map[string]bool{
	"onBasePower":           true,
	"onModifyDamage":        true,
	"onWeatherModifyDamage": true,
	"onModifyAtk":           true,
	"onModifyDef":           true,
	"onModifySpA":           true,
	"onModifySpD":           true,
	"onEffectiveness":       true,
	"onModifyType":          true,
	"onModifyMove":          true,
	"onModifyPriority":      true,
	"onTryHit":              true,
	"onTryAddVolatile":      false,
}

// parseMultihit は Showdown の multihit(null・回数・[最小, 最大])を検証する。複数回当たるなら true。
func parseMultihit(raw json.RawMessage) (bool, error) {
	if isJSONNull(raw) {
		return false, nil
	}
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		if n < 2 {
			return false, fmt.Errorf("multihit の回数が2未満: %d", n)
		}
		return true, nil
	}
	var r []int
	if err := json.Unmarshal(raw, &r); err != nil || len(r) != 2 {
		return false, fmt.Errorf("multihit が回数でも [最小, 最大] でもない: %s", raw)
	}
	if r[0] < 1 || r[1] < 2 || r[0] > r[1] {
		return false, fmt.Errorf("multihit の範囲が不正: %s", raw)
	}
	return true, nil
}

// parseFixedDamage は Showdown の damage(null・正の数値・"level")を検証する。固定ダメージなら true。
func parseFixedDamage(raw json.RawMessage) (bool, error) {
	if isJSONNull(raw) {
		return false, nil
	}
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		if n < 1 {
			return false, fmt.Errorf("damage が正でない: %d", n)
		}
		return true, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil || s != "level" {
		return false, fmt.Errorf("damage が数値でも \"level\" でもない: %s", raw)
	}
	return true, nil
}

// parseOHKO は Showdown の ohko(null・true・タイプ名)を検証する。一撃必殺なら true。
func parseOHKO(raw json.RawMessage) (bool, error) {
	if isJSONNull(raw) {
		return false, nil
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		if !b {
			return false, fmt.Errorf("ohko が false(取得時に null へ正規化しているはず)")
		}
		return true, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil || s == "" {
		return false, fmt.Errorf("ohko が true でもタイプ名でもない: %s", raw)
	}
	return true, nil
}

func isJSONNull(raw json.RawMessage) bool {
	return len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

// classifyMoveMechanisms は1つの攻撃技の機構を決める(昇順・重複なし)。power は moves 表に入れる威力。
func classifyMoveMechanisms(id string, power int, sig ShowdownMoveMechanism) ([]master.MoveMechanism, []Finding, error) {
	set := map[master.MoveMechanism]bool{}
	var warnings []Finding

	multiHit, err := parseMultihit(sig.Multihit)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: 技 %q: %v", ErrInvalidData, id, err)
	}
	fixed, err := parseFixedDamage(sig.Damage)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: 技 %q: %v", ErrInvalidData, id, err)
	}
	ohko, err := parseOHKO(sig.OHKO)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: 技 %q: %v", ErrInvalidData, id, err)
	}
	set[master.MechanismMultiHit] = multiHit
	set[master.MechanismFixedDamage] = fixed
	set[master.MechanismOHKO] = ohko
	set[master.MechanismAlwaysCrit] = sig.WillCrit
	set[master.MechanismAltOffenseStat] = sig.OverrideOffensiveStat != "" || sig.OverrideOffensivePokemon != ""
	set[master.MechanismAltDefenseStat] = sig.OverrideDefensiveStat != ""
	set[master.MechanismIgnoreDefenseRanks] = sig.IgnoreDefensive

	for _, hook := range sig.Hooks {
		if hook == "" {
			return nil, nil, fmt.Errorf("%w: 技 %q のハンドラ名が空", ErrInvalidData, id)
		}
		mech, known := moveHookMechanisms[hook]
		if !known {
			warnings = append(warnings, Finding{Kind: KindMoveMechanismUnknownHook, ID: id, Detail: hook})
			mech = master.MechanismMoveSpecific
		}
		if mech != "" {
			set[mech] = true
		}
	}

	for _, ref := range sig.FieldConditions {
		condition, hook, ok := strings.Cut(ref, ".")
		if !ok || condition == "" || hook == "" {
			return nil, nil, fmt.Errorf("%w: 技 %q の fieldConditions が \"<状態ID>.<ハンドラ名>\" でない: %q", ErrInvalidData, id, ref)
		}
		affects, known := fieldHookAffectsDamage[hook]
		if !known {
			warnings = append(warnings, Finding{Kind: KindMoveMechanismUnknownHook, ID: id, Detail: ref})
			affects = true
		}
		if affects {
			set[master.MechanismFieldSpecific] = true
		}
	}

	// 威力 0 で登録された攻撃技は、威力を使わない機構(固定ダメージ・一撃必殺)でなければ
	// 威力が技の処理で決まる(通常の式に渡すとダメージ 0 になる。issue #271)。
	// 固定ダメージは damage(値)と damageCallback(技の処理)のどちらから来てもよい。
	if power == 0 && !set[master.MechanismFixedDamage] && !set[master.MechanismOHKO] {
		set[master.MechanismVariablePower] = true
	}

	var out []master.MoveMechanism
	for m, on := range set {
		if on {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, warnings, nil
}

// buildMoveMechanisms は moves 表に入れる攻撃技の機構の行を作る(ADR-0121)。変化技は機構を持たない。
func buildMoveMechanisms(moves []ShowdownMove, rows []MoveRow) ([]MoveMechanismRow, []Finding, error) {
	sdByID := make(map[string]ShowdownMove, len(moves))
	for _, m := range moves {
		sdByID[m.ID] = m
	}
	var out []MoveMechanismRow
	var warnings []Finding
	for _, r := range rows {
		if r.Category == "status" {
			continue
		}
		sm, ok := sdByID[r.ID]
		if !ok || sm.Mechanism == nil {
			return nil, nil, fmt.Errorf("%w: 技 %q の機構の判定材料(mechanism)が無い(tools/importer で取り直す)", ErrInvalidData, r.ID)
		}
		mechs, ws, err := classifyMoveMechanisms(r.ID, r.Power, *sm.Mechanism)
		if err != nil {
			return nil, nil, err
		}
		warnings = append(warnings, ws...)
		for _, m := range mechs {
			out = append(out, MoveMechanismRow{MoveID: r.ID, Mechanism: string(m)})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MoveID != out[j].MoveID {
			return out[i].MoveID < out[j].MoveID
		}
		return out[i].Mechanism < out[j].Mechanism
	})
	return out, warnings, nil
}
