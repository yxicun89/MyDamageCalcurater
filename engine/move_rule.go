package engine

// 技の処理の定義(MoveRule。ADR-0143)。
//
// 既存の入力(両側の個体・状態異常・持ち物・特性・ランク・場)と種族の重さだけで決まる技の処理(威力の式・条件つきの威力・
// タイプ・相性・優先度・壁)を、閉じた語彙で表す。技 → 語彙の対応はデータ(effects.json の moveRules)で持ち、
// engine は技の ID を知らない。表(素早さ比・重さの段)と式の定数はゲームの規則なのでここに持つ。

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

// ErrInvalidMoveRule は技の処理の定義が不正(空・語彙外・機構と対応しない・値域外・変化技の定義)。
var ErrInvalidMoveRule = errors.New("技の処理の定義が不正")

// NullifyMoveFailed は技が失敗してダメージが 0 になった理由(防御側が持ち物なしのポルターガイスト型。ADR-0143 §2)。
const NullifyMoveFailed NullifyKind = "move_failed"

// PowerFormula は基本威力を決める式。"" は式なし。
type PowerFormula string

const (
	PowerFormulaPositiveBoosts    PowerFormula = "attacker_positive_boosts" // 威力 × (1 + 攻撃側の正のランクの合計)
	PowerFormulaSpeedRatio        PowerFormula = "speed_ratio"              // エレキボール型の表
	PowerFormulaInverseSpeedRatio PowerFormula = "inverse_speed_ratio"      // ジャイロボール型の式
	PowerFormulaTargetWeight      PowerFormula = "target_weight"            // けたぐり型の表(防御側の重さ)
	PowerFormulaWeightRatio       PowerFormula = "weight_ratio"             // ヘビーボンバー型の表(攻撃側 / 防御側)
	PowerFormulaHitIndex          PowerFormula = "hit_index"                // h 発目の威力 = 威力 × h(多段の中身が要る)
)

var allPowerFormulas = []PowerFormula{
	PowerFormulaPositiveBoosts, PowerFormulaSpeedRatio, PowerFormulaInverseSpeedRatio,
	PowerFormulaTargetWeight, PowerFormulaWeightRatio, PowerFormulaHitIndex,
}

// AllPowerFormulas は語彙のすべてを定義順で返す(呼び出しごとに新しいスライス)。
func AllPowerFormulas() []PowerFormula { return slices.Clone(allPowerFormulas) }

// Known は f が語彙にあるか(大文字小文字を区別する)。
func (f PowerFormula) Known() bool { return slices.Contains(allPowerFormulas, f) }

// MoveCondition は条件つきの威力(MovePowerBoost)の条件。
type MoveCondition string

const (
	MoveConditionAttackerStatus          MoveCondition = "attacker_status" // Statuses のどれか
	MoveConditionDefenderStatus          MoveCondition = "defender_status" // Statuses のどれか
	MoveConditionAttackerNoItem          MoveCondition = "attacker_no_item"
	MoveConditionDefenderItemRemovable   MoveCondition = "defender_item_removable"   // 持ち物あり かつ メガストーンでない
	MoveConditionWeather                 MoveCondition = "weather"                   // Weathers のどれか
	MoveConditionTerrainAttackerGrounded MoveCondition = "terrain_attacker_grounded" // Terrains のどれか かつ 攻撃側が接地
	MoveConditionTerrainDefenderGrounded MoveCondition = "terrain_defender_grounded" // Terrains のどれか かつ 防御側が接地
)

var allMoveConditions = []MoveCondition{
	MoveConditionAttackerStatus, MoveConditionDefenderStatus, MoveConditionAttackerNoItem,
	MoveConditionDefenderItemRemovable, MoveConditionWeather,
	MoveConditionTerrainAttackerGrounded, MoveConditionTerrainDefenderGrounded,
}

// AllMoveConditions は語彙のすべてを定義順で返す(呼び出しごとに新しいスライス)。
func AllMoveConditions() []MoveCondition { return slices.Clone(allMoveConditions) }

// Known は c が語彙にあるか(大文字小文字を区別する)。
func (c MoveCondition) Known() bool { return slices.Contains(allMoveConditions, c) }

// MovePowerBoost は条件つきの威力 1 つ。BaseMultiplier(整数倍。基本威力を直接変える)と
// Modifier(4096 基準。威力の補正の連鎖の最初に入る)のちょうど一方を持つ。
type MovePowerBoost struct {
	Condition      MoveCondition
	Statuses       []Status  // *_status だけ。空不可
	Weathers       []Weather // weather だけ。空不可・none 不可
	Terrains       []Terrain // terrain_* だけ。空不可・none 不可
	BaseMultiplier int       // 威力を整数倍(2 以上)。Modifier と排他
	Modifier       int       // 4096 基準(4096 不可)。威力の補正の連鎖の最初
}

// TerrainPowerMod は防御側が接地しているときのフィールドによる威力の補正(連鎖のフィールドの防御側の位置)。
type TerrainPowerMod struct {
	Terrain  Terrain
	Modifier int
}

// PriorityBoost は攻撃側が接地していて、そのフィールドのとき優先度に Delta を足す(Delta ≠ 0)。
type PriorityBoost struct {
	Terrain Terrain
	Delta   int
}

// MoveRule は技の処理の定義。nil(Move.Rule)は定義なし。
type MoveRule struct {
	PowerFormula             PowerFormula
	PowerBoosts              []MovePowerBoost
	IgnoresBurn              bool // やけどの攻撃半減を受けない(からげんき)
	TerrainPowerMods         []TerrainPowerMod
	TypeByWeather            map[Weather]Type
	TypeByTerrain            map[Terrain]Type // 攻撃側が接地のときだけ
	ExtraEffectivenessType   Type             // 各タイプの相性にこのタイプの相性も掛ける
	SuperEffectiveAgainst    []Type           // このタイプへの相性を 2 にする
	PriorityBoost            *PriorityBoost
	BreaksScreens            bool
	FailsWithoutDefenderItem bool    // 防御側が持ち物なしなら 0(Nullified = move_failed)
	SpreadInTerrain          Terrain // そのフィールドで攻撃側が接地なら全体技(ダブルの補正)
	MoveSpecificResolved     bool    // move_specific のハンドラは上の中身以外にダメージへ効かない(oracle で確かめた)
}

// hasPowerContent は威力の中身(式・条件つきの威力・やけど無視)があるか。
func (r MoveRule) hasPowerContent() bool {
	return r.PowerFormula != "" || len(r.PowerBoosts) > 0 || r.IgnoresBurn
}

// isEmpty は何の中身も無い定義か。
func (r MoveRule) isEmpty() bool {
	return !r.hasPowerContent() && len(r.TerrainPowerMods) == 0 && len(r.TypeByWeather) == 0 && len(r.TypeByTerrain) == 0 &&
		r.ExtraEffectivenessType == TypeNone && len(r.SuperEffectiveAgainst) == 0 && r.PriorityBoost == nil &&
		!r.BreaksScreens && !r.FailsWithoutDefenderItem && r.SpreadInTerrain == "" && !r.MoveSpecificResolved
}

// ValidateRule は定義が語彙・機構・値域に収まることを確かめる(ADR-0143 §1)。定義なしは常に通る。
// タイプが相性表に無ければ ErrUnknownType。
func (m Move) ValidateRule(chart TypeChart) error {
	r := m.Rule
	if r == nil {
		return nil
	}
	bad := func(format string, args ...any) error {
		return fmt.Errorf("%w: 技 %q: %s", ErrInvalidMoveRule, m.ID, fmt.Sprintf(format, args...))
	}
	if m.Category == CategoryStatus {
		return bad("変化技に定義がある")
	}
	if r.isEmpty() {
		return bad("定義が空")
	}
	has := func(mech MoveMechanism) bool { return slices.Contains(m.Mechanisms, mech) }

	if r.hasPowerContent() && !has(MechanismVariablePower) && !has(MechanismMoveSpecific) {
		return bad("威力の中身に対応する機構 variable_power / move_specific が無い")
	}
	if r.PowerFormula != "" {
		if !r.PowerFormula.Known() {
			return bad("未知の威力の式 %q", r.PowerFormula)
		}
		if r.PowerFormula == PowerFormulaHitIndex && m.Params.MultiHit == nil {
			return bad("hit_index には多段の中身(MultiHit)が要る")
		}
	}
	for i, b := range r.PowerBoosts {
		if err := b.validate(); err != nil {
			return bad("PowerBoosts[%d]: %v", i, err)
		}
	}
	if len(r.TerrainPowerMods) > 0 && !has(MechanismFieldSpecific) {
		return bad("TerrainPowerMods に対応する機構 field_specific が無い")
	}
	for i, tm := range r.TerrainPowerMods {
		if !knownTerrain(tm.Terrain) {
			return bad("TerrainPowerMods[%d] のフィールドが不正: %q", i, tm.Terrain)
		}
		if err := validateModifier("Modifier", tm.Modifier, false); err != nil {
			return bad("TerrainPowerMods[%d]: %v", i, err)
		}
	}
	if (len(r.TypeByWeather) > 0 || len(r.TypeByTerrain) > 0) && !has(MechanismTypeChange) {
		return bad("TypeByWeather / TypeByTerrain に対応する機構 type_change が無い")
	}
	for _, w := range slices.Sorted(maps.Keys(r.TypeByWeather)) {
		if !knownWeather(w) {
			return bad("TypeByWeather のキーが不正: %q", w)
		}
		if err := requireRuleType(chart, "TypeByWeather", r.TypeByWeather[w]); err != nil {
			return wrapRuleType(err, m.ID)
		}
	}
	for _, t := range slices.Sorted(maps.Keys(r.TypeByTerrain)) {
		if !knownTerrain(t) {
			return bad("TypeByTerrain のキーが不正: %q", t)
		}
		if err := requireRuleType(chart, "TypeByTerrain", r.TypeByTerrain[t]); err != nil {
			return wrapRuleType(err, m.ID)
		}
	}
	if (r.ExtraEffectivenessType != TypeNone || len(r.SuperEffectiveAgainst) > 0) && !has(MechanismEffectivenessChange) {
		return bad("相性の中身に対応する機構 effectiveness_change が無い")
	}
	if r.ExtraEffectivenessType != TypeNone {
		if err := chart.requireKnown("ExtraEffectivenessType", r.ExtraEffectivenessType); err != nil {
			return err
		}
	}
	for i, t := range r.SuperEffectiveAgainst {
		if t == TypeNone {
			return bad("SuperEffectiveAgainst[%d] が空", i)
		}
		if err := chart.requireKnown("SuperEffectiveAgainst", t); err != nil {
			return err
		}
		if slices.Contains(r.SuperEffectiveAgainst[:i], t) {
			return bad("SuperEffectiveAgainst に %q が重複している", t)
		}
	}
	if pb := r.PriorityBoost; pb != nil {
		if !has(MechanismPriorityChange) {
			return bad("PriorityBoost に対応する機構 priority_change が無い")
		}
		if !knownTerrain(pb.Terrain) {
			return bad("PriorityBoost のフィールドが不正: %q", pb.Terrain)
		}
		if pb.Delta == 0 {
			return bad("PriorityBoost.Delta は 0 以外")
		}
	}
	if r.SpreadInTerrain != "" && !knownTerrain(r.SpreadInTerrain) {
		return bad("SpreadInTerrain が不正: %q", r.SpreadInTerrain)
	}
	if (r.BreaksScreens || r.FailsWithoutDefenderItem || r.SpreadInTerrain != "" || r.MoveSpecificResolved) && !has(MechanismMoveSpecific) {
		return bad("BreaksScreens / FailsWithoutDefenderItem / SpreadInTerrain / MoveSpecificResolved に対応する機構 move_specific が無い")
	}
	return nil
}

// knownTerrain は none 以外の既知のフィールドか。
func knownTerrain(t Terrain) bool {
	switch t {
	case TerrainElectric, TerrainGrassy, TerrainMisty, TerrainPsychic:
		return true
	}
	return false
}

// knownWeather は none 以外の既知の天候か。
func knownWeather(w Weather) bool {
	switch w {
	case WeatherSun, WeatherRain, WeatherSand, WeatherSnow:
		return true
	}
	return false
}

// knownStatus は none 以外の既知の状態異常か。
func knownStatus(s Status) bool {
	switch s {
	case StatusBurn, StatusParalysis, StatusPoison, StatusBadlyPoison, StatusSleep, StatusFreeze:
		return true
	}
	return false
}

// requireRuleType は TypeByWeather / TypeByTerrain の値が空でなく、相性表にあることを確かめる。
func requireRuleType(chart TypeChart, what string, t Type) error {
	if t == TypeNone {
		return fmt.Errorf("%s の値が空", what)
	}
	return chart.requireKnown(what, t)
}

// wrapRuleType は requireRuleType の失敗を、相性表由来のもの(ErrUnknownType 等)はそのまま、空の値は ErrInvalidMoveRule にする。
func wrapRuleType(err error, id string) error {
	if errors.Is(err, ErrUnknownType) || errors.Is(err, ErrTypeChartMissing) {
		return err
	}
	return fmt.Errorf("%w: 技 %q: %v", ErrInvalidMoveRule, id, err)
}

// validate は条件つきの威力 1 つの形と値域を確かめる(エラーは呼び出し側が ErrInvalidMoveRule で包む)。
func (b MovePowerBoost) validate() error {
	var nStatuses, nWeathers, nTerrains int
	switch b.Condition {
	case MoveConditionAttackerStatus, MoveConditionDefenderStatus:
		if len(b.Statuses) == 0 {
			return errors.New("項目 Statuses が空")
		}
		nStatuses = len(b.Statuses)
	case MoveConditionWeather:
		if len(b.Weathers) == 0 {
			return errors.New("項目 Weathers が空")
		}
		nWeathers = len(b.Weathers)
	case MoveConditionTerrainAttackerGrounded, MoveConditionTerrainDefenderGrounded:
		if len(b.Terrains) == 0 {
			return errors.New("項目 Terrains が空")
		}
		nTerrains = len(b.Terrains)
	case MoveConditionAttackerNoItem, MoveConditionDefenderItemRemovable:
	default:
		return fmt.Errorf("未知の条件 %q", b.Condition)
	}
	if nStatuses != len(b.Statuses) || nWeathers != len(b.Weathers) || nTerrains != len(b.Terrains) {
		return fmt.Errorf("条件 %q に使えない一覧がある", b.Condition)
	}
	for _, s := range b.Statuses {
		if !knownStatus(s) {
			return fmt.Errorf("項目 Statuses に不正な値 %q", s)
		}
	}
	for _, w := range b.Weathers {
		if !knownWeather(w) {
			return fmt.Errorf("項目 Weathers に不正な値 %q", w)
		}
	}
	for _, t := range b.Terrains {
		if !knownTerrain(t) {
			return fmt.Errorf("項目 Terrains に不正な値 %q", t)
		}
	}
	if (b.BaseMultiplier != 0) == (b.Modifier != 0) {
		return errors.New("項目 BaseMultiplier と Modifier はちょうど一方を指定する")
	}
	if b.BaseMultiplier != 0 && b.BaseMultiplier < 2 {
		return fmt.Errorf("項目 BaseMultiplier は 2 以上: %d", b.BaseMultiplier)
	}
	if b.Modifier != 0 {
		if b.Modifier == Modifier4096 {
			return fmt.Errorf("項目 Modifier は中立(%d)不可", Modifier4096)
		}
		return validateModifier("Modifier", b.Modifier, false)
	}
	return nil
}

// holds は条件がこの入力で成り立つかを返す。
func (b MovePowerBoost) holds(in DamageInput) bool {
	switch b.Condition {
	case MoveConditionAttackerStatus:
		return slices.Contains(b.Statuses, in.Attacker.Status)
	case MoveConditionDefenderStatus:
		return slices.Contains(b.Statuses, in.Defender.Status)
	case MoveConditionAttackerNoItem:
		return in.Attacker.Item == nil
	case MoveConditionDefenderItemRemovable:
		return in.Defender.Item != nil && !in.Defender.Item.MegaStone
	case MoveConditionWeather:
		return slices.Contains(b.Weathers, in.Field.Weather)
	case MoveConditionTerrainAttackerGrounded:
		return slices.Contains(b.Terrains, in.Field.Terrain) && isGrounded(in.Attacker)
	case MoveConditionTerrainDefenderGrounded:
		return slices.Contains(b.Terrains, in.Field.Terrain) && isGrounded(in.Defender)
	}
	return false
}

// --- 計算 ---------------------------------------------------------------------------------------------

// applyRulePreconditions は計算の最初に決める定義の効果(優先度・タイプ・全体技)を反映したコピーを返す(ADR-0143 §2 の 1)。
// 技のタイプの変更は type_change の機構を持つ技だけ(定義の検証済み)なので、スキン系の特性では変えない。
func applyRulePreconditions(in DamageInput) DamageInput {
	r := in.Move.Rule
	if r == nil {
		return in
	}
	attackerGrounded := isGrounded(in.Attacker)
	if pb := r.PriorityBoost; pb != nil && attackerGrounded && in.Field.Terrain == pb.Terrain {
		in.Move.Priority += pb.Delta
	}
	if t, ok := r.TypeByWeather[in.Field.Weather]; ok {
		in.Move.Type = t
	}
	if t, ok := r.TypeByTerrain[in.Field.Terrain]; ok && attackerGrounded {
		in.Move.Type = t
	}
	if r.SpreadInTerrain != "" && in.Field.Terrain == r.SpreadInTerrain && attackerGrounded {
		in.Move.Target = MoveTargetSpread
	}
	return in
}

// moveEffectiveness は技のタイプの相性を返す。定義の相性の中身(ExtraEffectivenessType・SuperEffectiveAgainst)があれば
// 防御側のタイプごとに反映する(倍率コードの積。分母は 2^防御タイプ数 × 2^(追加のタイプの数))。
func moveEffectiveness(in DamageInput, moveType Type) (Effectiveness, error) {
	r := in.Move.Rule
	if r == nil || (r.ExtraEffectivenessType == TypeNone && len(r.SuperEffectiveAgainst) == 0) {
		return in.TypeChart.Effectiveness(moveType, in.Defender.Species.Types)
	}
	num, den := 1, 1
	for _, def := range in.Defender.Species.Types {
		if def == TypeNone {
			continue
		}
		code, err := in.TypeChart.Code(moveType, def)
		if err != nil {
			return Effectiveness{}, err
		}
		if slices.Contains(r.SuperEffectiveAgainst, def) {
			code = TypeCodeSuperEffective
		}
		if r.ExtraEffectivenessType != TypeNone {
			extra, err := in.TypeChart.Code(r.ExtraEffectivenessType, def)
			if err != nil {
				return Effectiveness{}, err
			}
			num *= code * extra
			den *= 4
			continue
		}
		num *= code
		den *= 2
	}
	return Effectiveness{Num: num, Den: den}, nil
}

// 重さ・素早さの表(ゲームの規則。oracle の calculateBasePowerChampions と同じ)。

// targetWeightPower はけたぐり型の表(防御側の重さ hg → 威力)。
func targetWeightPower(hg int) int {
	switch {
	case hg >= 2000:
		return 120
	case hg >= 1000:
		return 100
	case hg >= 500:
		return 80
	case hg >= 250:
		return 60
	case hg >= 100:
		return 40
	}
	return 20
}

// weightRatioPower はヘビーボンバー型の表(攻撃側 / 防御側の重さ。hg の整数で比べる)。
func weightRatioPower(attacker, defender int) int {
	switch {
	case attacker >= defender*5:
		return 120
	case attacker >= defender*4:
		return 100
	case attacker >= defender*3:
		return 80
	case attacker >= defender*2:
		return 60
	}
	return 40
}

// effectiveWeight は特性の重さの補正を掛けた重さ(hg)。max(1, trunc(hg × WeightMod / 4096))。不明(0)は 0。
func effectiveWeight(ind Individual) int {
	w := ind.Species.WeightHg
	if w <= 0 {
		return 0
	}
	if e := ind.Ability.Effect; e != nil && e.WeightMod != 0 {
		return max(1, w*e.WeightMod/Modifier4096)
	}
	return w
}

// speedModBounds は素早さの補正の連鎖の範囲(oracle の getFinalSpeed の chainMods)。
var speedModBounds = modBounds{lower: 410, upper: 131172}

// maxSpeed は素早さの上限(oracle)。
const maxSpeed = 10000

// speedCondHolds は素早さの補正の条件が成り立つか。item_lost は engine の入力に無いので常に不成立(oracle の既定と同じ)。
func speedCondHolds(c SpeedCondition, ind Individual, f Field) bool {
	switch c {
	case SpeedConditionAlways:
		return true
	case SpeedConditionWeatherSun:
		return f.Weather == WeatherSun
	case SpeedConditionWeatherRain:
		return f.Weather == WeatherRain
	case SpeedConditionWeatherSand:
		return f.Weather == WeatherSand
	case SpeedConditionWeatherSnow:
		return f.Weather == WeatherSnow
	case SpeedConditionTerrainElectric:
		return f.Terrain == TerrainElectric
	case SpeedConditionHasStatus:
		return ind.Status != StatusNone && ind.Status != ""
	}
	return false
}

// firstSpeedMod は成立した最初の素早さの補正を返す。
func firstSpeedMod(mods []SpeedMod, ind Individual, f Field) (int, bool) {
	for _, m := range mods {
		if speedCondHolds(m.Condition, ind, f) {
			return m.Modifier, true
		}
	}
	return 0, false
}

// finalSpeed は戦闘中の素早さ: ランク → [特性・持ち物の SpeedMods(成立した最初の1つずつ)] の chainMods → pokeRound →
// まひ(IgnoresParalysisSpeedDrop が無ければ ×50/100 floor)→ 0..10000(oracle の getFinalSpeed。ADR-0143 §1)。
func finalSpeed(ind Individual, f Field) int {
	speed := applyStatStage(RealStats(ind).Spe, ind.Ranks.Spe)
	var mods []int
	ae := ind.Ability.Effect
	if ae != nil {
		if m, ok := firstSpeedMod(ae.SpeedMods, ind, f); ok {
			mods = append(mods, m)
		}
	}
	if e := itemEffect(ind.Item); e != nil {
		if m, ok := firstSpeedMod(e.SpeedMods, ind, f); ok {
			mods = append(mods, m)
		}
	}
	speed = pokeRound(speed, chainMods(mods, speedModBounds))
	if ind.Status == StatusParalysis && !(ae != nil && ae.IgnoresParalysisSpeedDrop) {
		speed = speed * 50 / 100
	}
	return max(0, min(speed, maxSpeed))
}

// formulaComputable は威力の式をこの入力で計算できるか(重さの式は要る側の重さが分かること)。
func (r *MoveRule) formulaComputable(in DamageInput) bool {
	switch r.PowerFormula {
	case PowerFormulaTargetWeight:
		return in.Defender.Species.WeightHg > 0
	case PowerFormulaWeightRatio:
		return in.Attacker.Species.WeightHg > 0 && in.Defender.Species.WeightHg > 0
	}
	return true
}

// ruleBasePower はこの入力での基本威力を返す。hit は多段の何発目か(0 始まり。hit_index だけが使う)。
// 定義なし・式なし・式を計算できない(重さが不明)ときは技の威力のまま。式の後に、成立した BaseMultiplier を掛ける。
func ruleBasePower(in DamageInput, hit int) int {
	power := in.Move.Power
	r := in.Move.Rule
	if r == nil {
		return power
	}
	if r.PowerFormula != "" && r.formulaComputable(in) {
		switch r.PowerFormula {
		case PowerFormulaPositiveBoosts:
			positives := 0
			for _, k := range rankStatKeys {
				positives += max(0, in.Attacker.Ranks.Get(k))
			}
			power *= 1 + positives
		case PowerFormulaSpeedRatio:
			power = speedRatioPower(finalSpeed(in.Attacker, in.Field), finalSpeed(in.Defender, in.Field))
		case PowerFormulaInverseSpeedRatio:
			power = inverseSpeedPower(finalSpeed(in.Attacker, in.Field), finalSpeed(in.Defender, in.Field))
		case PowerFormulaTargetWeight:
			power = targetWeightPower(effectiveWeight(in.Defender))
		case PowerFormulaWeightRatio:
			power = weightRatioPower(effectiveWeight(in.Attacker), effectiveWeight(in.Defender))
		case PowerFormulaHitIndex:
			power *= hit + 1
		}
	}
	for _, b := range r.PowerBoosts {
		if b.BaseMultiplier != 0 && b.holds(in) {
			power *= b.BaseMultiplier
		}
	}
	return power
}

// speedRatioPower はエレキボール型の表(攻撃側 / 防御側の素早さ。防御側 0 は 40)。
func speedRatioPower(attacker, defender int) int {
	if defender == 0 {
		return 40
	}
	switch r := attacker / defender; {
	case r >= 4:
		return 150
	case r >= 3:
		return 120
	case r >= 2:
		return 80
	case r >= 1:
		return 60
	}
	return 40
}

// inverseSpeedPower はジャイロボール型の式(攻撃側 0 は 1)。
func inverseSpeedPower(attacker, defender int) int {
	if attacker == 0 {
		return 1
	}
	return min(150, 25*defender/attacker+1)
}

// ruleModifiers は威力の補正の連鎖の先頭に入る、成立した Modifier の条件つきの威力を返す。
func ruleModifiers(in DamageInput) []int {
	r := in.Move.Rule
	if r == nil {
		return nil
	}
	var mods []int
	for _, b := range r.PowerBoosts {
		if b.Modifier != 0 && b.holds(in) {
			mods = append(mods, b.Modifier)
		}
	}
	return mods
}

// ruleTerrainPowerMods は防御側が接地しているときの TerrainPowerMods の補正を返す(連鎖のフィールドの防御側の位置)。
func ruleTerrainPowerMods(in DamageInput) []int {
	r := in.Move.Rule
	if r == nil || !isGrounded(in.Defender) {
		return nil
	}
	var mods []int
	for _, tm := range r.TerrainPowerMods {
		if tm.Terrain == in.Field.Terrain {
			mods = append(mods, tm.Modifier)
		}
	}
	return mods
}

// ruleFailed は防御側が持ち物なしで失敗する技(FailsWithoutDefenderItem)か。
func ruleFailed(in DamageInput) bool {
	return in.Move.Rule != nil && in.Move.Rule.FailsWithoutDefenderItem && in.Defender.Item == nil
}

// ruleIgnoresBurn はやけどの攻撃半減を受けない技か。
func ruleIgnoresBurn(in DamageInput) bool { return in.Move.Rule != nil && in.Move.Rule.IgnoresBurn }

// ruleBreaksScreens は壁を壊す技か。
func ruleBreaksScreens(in DamageInput) bool { return in.Move.Rule != nil && in.Move.Rule.BreaksScreens }

// ruleHandlesVariablePower は variable_power の印を外せるか(威力の中身があり、この入力で計算できる)。
func ruleHandlesVariablePower(in DamageInput) bool {
	r := in.Move.Rule
	if r == nil || (r.PowerFormula == "" && len(r.PowerBoosts) == 0) {
		return false
	}
	if !r.formulaComputable(in) {
		return false
	}
	// 防御側がメガストーンを持つと、防御側の種族に合うかを engine は知らないので安全側で印を残す。
	if in.Defender.Item != nil && in.Defender.Item.MegaStone {
		for _, b := range r.PowerBoosts {
			if b.Condition == MoveConditionDefenderItemRemovable {
				return false
			}
		}
	}
	return true
}

// ruleHasComputableFormula は威力 0 の技の威力が、計算できる式で決まるか(zero_power の印を外す条件)。
func ruleHasComputableFormula(in DamageInput) bool {
	r := in.Move.Rule
	return r != nil && r.PowerFormula != "" && r.formulaComputable(in)
}
