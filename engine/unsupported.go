package engine

// 未対応の印(issue #271-b / #270 案 B。ADR-0123)。
//
// engine は技を「威力・分類・タイプ・優先度」と、持ち物・特性の効果定義から通常の式で計算する。
// それで正しく計算できない入力(多段・威力が変わる・固定ダメージの技、効果スキーマで表せない
// 持ち物・特性)は、数値を従来どおり通常の式で返したうえで、結果に「未対応」の印を付ける。
// 黙って正しい結果のように見せないため。数値そのものは印で変えない(ゴールデンの全件一致を保つ)。

import (
	"slices"
)

// MoveMechanism は技の機構1つ(ADR-0121)。「威力・分類・タイプから通常の式で計算すると誤る」理由の分類。
// 値の一覧の正はここ。マスタ(services/internal/master)と migration の CHECK はこれと一致させる。
type MoveMechanism string

const (
	// MechanismAltDefenseStat は防御側の参照する能力値が分類と違う(特殊技で防御を参照する 等)。
	MechanismAltDefenseStat MoveMechanism = "alt_defense_stat"
	// MechanismAltOffenseStat は攻撃側の参照する能力値・参照するポケモンが違う(防御で攻撃する・相手の攻撃を使う 等)。
	MechanismAltOffenseStat MoveMechanism = "alt_offense_stat"
	// MechanismAlwaysCrit は必ず急所に当たる。
	MechanismAlwaysCrit MoveMechanism = "always_crit"
	// MechanismEffectivenessChange はタイプ相性の求め方が違う(2タイプの相性を使う・特定タイプへの相性を変える 等)。
	MechanismEffectivenessChange MoveMechanism = "effectiveness_change"
	// MechanismFieldSpecific は天候・フィールドがこの技を名指しして処理を変える(フィールドで威力が変わる 等)。
	MechanismFieldSpecific MoveMechanism = "field_specific"
	// MechanismFixedDamage はダメージを計算式でなく技の処理で決める(レベルと同じ・残り HP の半分・受けたダメージの倍返し 等)。
	MechanismFixedDamage MoveMechanism = "fixed_damage"
	// MechanismIgnoreDefenseRanks は防御側の防御・特防のランク変化を無視する。
	MechanismIgnoreDefenseRanks MoveMechanism = "ignore_defense_ranks"
	// MechanismMoveSpecific は技固有の処理があり、ダメージに効くかをデータから機械的に決められない
	// (技の性質を書き換える処理・当たる前の処理・使う前の処理)。安全側に倒して「通常の技ではない」とする。
	MechanismMoveSpecific MoveMechanism = "move_specific"
	// MechanismMultiHit は複数回当たる。
	MechanismMultiHit MoveMechanism = "multi_hit"
	// MechanismOHKO は一撃必殺。
	MechanismOHKO MoveMechanism = "ohko"
	// MechanismPriorityChange は条件で優先度が変わる(サイコフィールドの先制技の扱いに効く)。
	MechanismPriorityChange MoveMechanism = "priority_change"
	// MechanismTypeChange は条件で技のタイプが変わる。
	MechanismTypeChange MoveMechanism = "type_change"
	// MechanismVariablePower は条件で威力が変わる(重さ・HP・ランク・状態・相手の持ち物 等)。
	// 威力 0 で登録された攻撃技(固定ダメージ・一撃必殺でないもの)もここに入る。
	MechanismVariablePower MoveMechanism = "variable_power"
)

// allMoveMechanisms は機構の一覧(値の昇順)。
var allMoveMechanisms = []MoveMechanism{
	MechanismAltDefenseStat,
	MechanismAltOffenseStat,
	MechanismAlwaysCrit,
	MechanismEffectivenessChange,
	MechanismFieldSpecific,
	MechanismFixedDamage,
	MechanismIgnoreDefenseRanks,
	MechanismMoveSpecific,
	MechanismMultiHit,
	MechanismOHKO,
	MechanismPriorityChange,
	MechanismTypeChange,
	MechanismVariablePower,
}

// AllMoveMechanisms は機構の一覧(値の昇順)のコピーを返す。
func AllMoveMechanisms() []MoveMechanism {
	return slices.Clone(allMoveMechanisms)
}

// Known は m が既知の機構かどうか(大文字小文字を区別する)。
func (m MoveMechanism) Known() bool {
	return slices.Contains(allMoveMechanisms, m)
}

// UnsupportedTarget は印の対象。
type UnsupportedTarget string

const (
	UnsupportedTargetMove            UnsupportedTarget = "move"
	UnsupportedTargetAttackerItem    UnsupportedTarget = "attacker_item"
	UnsupportedTargetAttackerAbility UnsupportedTarget = "attacker_ability"
	UnsupportedTargetDefenderItem    UnsupportedTarget = "defender_item"
	UnsupportedTargetDefenderAbility UnsupportedTarget = "defender_ability"
)

// UnsupportedReason は印の理由のコード。技の印は機構の値(MoveMechanism)か UnsupportedZeroPower、
// 持ち物・特性の印は UnsupportedEffect。
type UnsupportedReason string

const (
	// UnsupportedZeroPower は威力 0 の攻撃技(威力が技の処理で決まる。ダメージ 0 は正しい結果ではない)。
	UnsupportedZeroPower UnsupportedReason = "zero_power"
	// UnsupportedEffect は持ち物・特性のダメージへの効果を計算に入れていない(効果スキーマで表せない。ADR-0120)。
	UnsupportedEffect UnsupportedReason = "unsupported_effect"
)

// UnsupportedMark は「この結果は正しくない可能性がある」印1つ。
type UnsupportedMark struct {
	Target UnsupportedTarget
	Reason UnsupportedReason
	ID     string // 技・持ち物・特性の ID
}

// unsupportedMarks は入力に付く印を 技 → 攻撃側の持ち物 → 攻撃側の特性 → 防御側の持ち物 → 防御側の特性
// の順に返す(技の印は理由の昇順。機構の印の後に zero_power)。印が無ければ nil。
// 変化技は印を付けない(ダメージを持たないので 0 が正しい)。
func unsupportedMarks(in DamageInput) []UnsupportedMark {
	var marks []UnsupportedMark
	if in.Move.Category != CategoryStatus {
		marks = append(marks, moveMarks(in)...)
	}
	if it := in.Attacker.Item; it != nil && it.Effect != nil && it.Effect.UnsupportedAttacker {
		marks = append(marks, UnsupportedMark{Target: UnsupportedTargetAttackerItem, Reason: UnsupportedEffect, ID: it.ID})
	}
	if ae := in.Attacker.Ability.Effect; ae != nil && ae.UnsupportedAttacker {
		marks = append(marks, UnsupportedMark{Target: UnsupportedTargetAttackerAbility, Reason: UnsupportedEffect, ID: in.Attacker.Ability.ID})
	}
	if it := in.Defender.Item; it != nil && it.Effect != nil && it.Effect.UnsupportedDefender {
		marks = append(marks, UnsupportedMark{Target: UnsupportedTargetDefenderItem, Reason: UnsupportedEffect, ID: it.ID})
	}
	if ae := in.Defender.Ability.Effect; ae != nil && ae.UnsupportedDefender {
		marks = append(marks, UnsupportedMark{Target: UnsupportedTargetDefenderAbility, Reason: UnsupportedEffect, ID: in.Defender.Ability.ID})
	}
	return marks
}

// moveMarks は攻撃技の印を返す。条件によっては通常の式で正しくなる機構は、この入力で誤るときだけ印を付ける。
// 未知の機構は安全側で印を付ける。
func moveMarks(in DamageInput) []UnsupportedMark {
	reasons := make([]UnsupportedReason, 0, len(in.Move.Mechanisms))
	for _, m := range in.Move.Mechanisms {
		if mechanismHandled(in, m) {
			continue
		}
		reasons = append(reasons, UnsupportedReason(m))
	}
	slices.Sort(reasons)
	reasons = slices.Compact(reasons)
	if in.Move.Power <= 0 {
		reasons = append(reasons, UnsupportedZeroPower)
	}
	if len(reasons) == 0 {
		return nil
	}
	marks := make([]UnsupportedMark, 0, len(reasons))
	for _, r := range reasons {
		marks = append(marks, UnsupportedMark{Target: UnsupportedTargetMove, Reason: r, ID: in.Move.ID})
	}
	return marks
}

// mechanismHandled は、機構 m を持つ技がこの入力では通常の式と同じ結果になるかを返す。
//   - 必ず急所: 入力が急所ありなら同じ。
//   - 防御側のランク無視: 使う側(物理は防御・特殊は特防)のランクが 0 なら同じ。
//   - 条件で優先度が変わる: engine が優先度を使うのはサイコフィールドの判定だけ(ADR-0123)。
//     防御側が浮いていて判定に関係しないときも印を付ける(安全側の過検出)。
//   - 天候・フィールドが名指しする技: engine が持つ場の状態は天候とフィールドだけで、どちらも無ければ
//     名指しの処理は起きない。
func mechanismHandled(in DamageInput, m MoveMechanism) bool {
	switch m {
	case MechanismAlwaysCrit:
		return in.Critical
	case MechanismIgnoreDefenseRanks:
		defKey := StatDef
		if in.Move.Category == CategorySpecial {
			defKey = StatSpD
		}
		return in.Defender.Ranks.Get(defKey) == 0
	case MechanismPriorityChange:
		return in.Field.Terrain != TerrainPsychic
	case MechanismFieldSpecific:
		noWeather := in.Field.Weather == WeatherNone || in.Field.Weather == ""
		noTerrain := in.Field.Terrain == TerrainNone || in.Field.Terrain == ""
		return noWeather && noTerrain
	}
	return false
}
