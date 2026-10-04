// Package judge is judge's own pure core (no I/O): the one calculation JD1 does itself is
// comparing battle speed, reusing engine.RealStats/EffectiveStat for the real stat and rank
// steps and adding only the choice scarf modifier, which engine doesn't have (ADR-0701 §2,
// same posture as services/speed/internal/speed and ADR-0600 §3).
package judge

import (
	"errors"

	"example.com/pokecalc/engine"
)

// 種族値・ランクの範囲(ADR-0701 §2。services/speed/internal/speed と同じ範囲)。
const (
	minBaseSpeed = 1
	maxBaseSpeed = 255
	minRank      = -6
	maxRank      = 6
)

// scarfSpeedModifier はこだわりスカーフの素早さ補正(×1.5)を engine.Modifier4096 基準で表した値
// (ADR-0701 §2。services/speed/internal/speed/speed.go と同一の出典・式)。
const scarfSpeedModifier = 6144

// tailwindSpeedModifier は追い風の素早さ補正(×2)を engine.Modifier4096 基準で表した値
// (ADR-0702 §2。出典は @smogon/calc 0.12.0 の dist/mechanics/util.js の getFinalSpeed)。
const tailwindSpeedModifier = 8192

// DefaultChoiceScarfItemID は pokedex-svc の持ち物 ID の命名規則(Showdown ID。小文字英数のみ)
// に沿った既定値(ADR-0701 §3)。cmd/api が環境変数で上書きできる。
const DefaultChoiceScarfItemID = "choicescarf"

// paralysisSpeedPercent はまひの素早さ補正(50%)。出典は @smogon/calc 0.12.0 の getFinalSpeed
// (`floor(speed * 50 / 100)`。ADR-0712)。
const paralysisSpeedPercent = 50

// 入力検証の sentinel エラー。検証はコアが持つドメインの不変条件(coding-rules §3)。
var (
	// ErrInvalidBaseSpeed は素早さ種族値が 1〜255 の外であることを表す。
	ErrInvalidBaseSpeed = errors.New("base speed must be between 1 and 255")
	// ErrInvalidSP は SP が 0〜engine.MaxSPPerStat の外、または合計が engine.MaxSPTotal 超であることを表す。
	ErrInvalidSP = errors.New("SP is out of range")
	// ErrInvalidRank はランクが -6〜+6 の外であることを表す。
	ErrInvalidRank = errors.New("rank must be between -6 and +6")
)

// Individual は素早さ計算に使う個体(ADR-0701 §2)。状態異常は麻痺だけを Paralysis で持つ
// (ADR-0712。他の状態異常は素早さに効かない)。テラスタルは持たない。
type Individual struct {
	BaseSpeed int
	Nature    engine.Nature
	SP        engine.Stats
	Ranks     engine.Ranks
	Scarf     bool
	// Tailwind は追い風がこの個体の側にかかっているか(ADR-0702 §4)。Scarf と同じ「その個体の
	// 素早さに乗る補正」なので、CompareSpeed の引数ではなく Individual に置く。
	Tailwind bool
	// Paralysis は状態異常がまひか。連結・五捨五超入のあとに floor(x × 50 / 100) を掛ける
	// (@smogon/calc 0.12.0 の getFinalSpeed と同じ。4096 基準の連結には含めない。ADR-0712)。
	Paralysis bool

	// Ability・Item・Env は特性・持ち物の素早さ効果とその評価に使う環境(issue 235 第2段・ADR-0714)。
	// ゼロ値(効果なし・環境なし)なら第1段と同じ値になる。judge は ID を見ず、マスタの効果データだけで評価する。
	Ability AbilitySpeedEffect
	Item    ItemSpeedEffect
	Env     SpeedEnvironment
}

// AbilitySpeedEffect は特性の素早さ効果(マスタの SpeedMods・IgnoresParalysisSpeedDrop。ADR-0139)。
type AbilitySpeedEffect struct {
	// Mods は条件つきの倍率(4096 基準)。配列の順が優先順で、成立した最初の要素だけを掛ける。
	Mods []engine.SpeedMod
	// IgnoresParalysisSpeedDrop はまひの素早さ半減を受けない特性か(無条件のフラグ)。
	IgnoresParalysisSpeedDrop bool
}

// ItemSpeedEffect は持ち物の素早さ効果(マスタの SpeedMods。ADR-0139)。
type ItemSpeedEffect struct {
	Mods []engine.SpeedMod
}

// SpeedEnvironment は素早さ効果の条件を評価する環境(ADR-0714 §2)。天候・場は両者に共通で、
// HasStatus はその個体の状態異常(none 以外)。judge は「持ち物を失った」入力を持たない。
type SpeedEnvironment struct {
	Weather   string
	Terrain   string
	HasStatus bool
}

// Holds は条件 c が env で成立するかを返す。item_lost と語彙に無い条件は成立を判定できないので false
// (判定できないことの扱いは ResolveSpeedMod が区別する)。
func (e SpeedEnvironment) Holds(c engine.SpeedCondition) bool {
	switch c {
	case engine.SpeedConditionAlways:
		return true
	case engine.SpeedConditionWeatherSun:
		return e.Weather == "sun"
	case engine.SpeedConditionWeatherRain:
		return e.Weather == "rain"
	case engine.SpeedConditionWeatherSand:
		return e.Weather == "sand"
	case engine.SpeedConditionWeatherSnow:
		return e.Weather == "snow"
	case engine.SpeedConditionTerrainElectric:
		return e.Terrain == "electric"
	case engine.SpeedConditionHasStatus:
		return e.HasStatus
	default:
		return false
	}
}

// evaluable は条件の成立を env から判定できるか。item_lost と語彙に無い条件は判定できない。
func evaluable(c engine.SpeedCondition) bool {
	return c.Known() && c != engine.SpeedConditionItemLost
}

// ResolveSpeedMod は mods を前から評価し、成立した最初の要素の倍率を返す(ADR-0139 §2)。
// 成立を判定できない条件に当たったらそこで止め、undetermined を返して何も掛けない(前の要素が
// 実は成立していたかもしれないので、後ろを掛けると誤る。ADR-0714 §2)。どれも不成立なら全て false。
// applied と undetermined は同時に true にならない。
func ResolveSpeedMod(mods []engine.SpeedMod, env SpeedEnvironment) (modifier int, applied, undetermined bool) {
	for _, m := range mods {
		if !evaluable(m.Condition) {
			return 0, false, true
		}
		if env.Holds(m.Condition) {
			return m.Modifier, true, false
		}
	}
	return 0, false, false
}

// SpeedField は CompareSpeed の第3引数で、比較そのものに効く場の効果を表す(ADR-0702 §4)。
// 追い風は片側の値を変えるだけなので Individual.Tailwind に置き、ここには持たせない。
type SpeedField struct {
	// TrickRoom はトリックルームがかかっているか。実数値は変えず、outspeeds の向きだけを
	// 反転する(ADR-0702 §3)。
	TrickRoom bool
}

// SpeedComparison は attacker/defender の戦闘中の素早さの比較結果(ADR-0700 §6-1)。
type SpeedComparison struct {
	AttackerSpeed int
	DefenderSpeed int
	Outspeeds     bool
	SpeedTie      bool
}

// Speed は 実数値(engine.RealStats)→ ランク補正(engine.EffectiveStat)→ 素早さ補正(追い風・
// こだわりスカーフ)の順で戦闘中の素早さを求める(ADR-0701 §2・ADR-0702 §2)。効いている補正は
// 4096 基準で 1 つに連結してから 1 回だけ五捨五超入する(各補正ごとに丸めない)。judge は式を
// 複製せず、engine に無い素早さ補正だけを自分で持つ。入力が範囲外なら sentinel エラーを返す。
func Speed(in Individual) (int, error) {
	if in.BaseSpeed < minBaseSpeed || in.BaseSpeed > maxBaseSpeed {
		return 0, ErrInvalidBaseSpeed
	}
	if err := validateSP(in.SP); err != nil {
		return 0, err
	}
	if err := validateRanks(in.Ranks); err != nil {
		return 0, err
	}

	individual := engine.Individual{
		Species: engine.Species{BaseStats: engine.Stats{Spe: in.BaseSpeed}},
		Nature:  in.Nature,
		SP:      in.SP,
		Ranks:   in.Ranks,
	}
	v := engine.EffectiveStat(individual, engine.StatSpe)

	// 配列に積む順は追い風 → 特性 → 持ち物(ADR-0702 §2・ADR-0714 §3 の出典どおり)。
	var mods []int
	if in.Tailwind {
		mods = append(mods, tailwindSpeedModifier)
	}
	if m, ok := abilitySpeedModifier(in); ok {
		mods = append(mods, m)
	}
	if m, ok := itemSpeedModifier(in); ok {
		mods = append(mods, m)
	}
	if len(mods) > 0 {
		v = applySpeedModifiers(v, mods)
	}
	if paralysisHalves(in) {
		v = v * paralysisSpeedPercent / 100
	}
	return v, nil
}

// abilitySpeedModifier は連結に入れる特性の倍率。条件が成立したときだけ ok を返す(判定できない条件は掛けない)。
func abilitySpeedModifier(in Individual) (modifier int, ok bool) {
	modifier, applied, _ := ResolveSpeedMod(in.Ability.Mods, in.Env)
	return modifier, applied
}

// itemSpeedModifier は連結に入れる持ち物の倍率。こだわりスカーフは持ち物の効果データより優先し、
// そのとき効果データは掛けない(持ち物は 1 つ。@smogon/calc も Choice Scarf を先に見る)。
func itemSpeedModifier(in Individual) (modifier int, ok bool) {
	if in.Scarf {
		return scarfSpeedModifier, true
	}
	modifier, applied, _ := ResolveSpeedMod(in.Item.Mods, in.Env)
	return modifier, applied
}

// paralysisHalves はまひの半減を掛けるか。まひの半減を受けない特性なら掛けない(ADR-0714 §3)。
func paralysisHalves(in Individual) bool {
	return in.Paralysis && !in.Ability.IgnoresParalysisSpeedDrop
}

// chainSpeedModifiers merges speed modifiers (each expressed in engine.Modifier4096 units) into
// a single modifier, replicating @smogon/calc 0.12.0's chainMods: each step rounds up
// (ADR-0702 §2. `M = (M*mod + engine.Modifier4096/2) / engine.Modifier4096`, starting from the
// identity modifier engine.Modifier4096). mods must be in the order @smogon/calc pushes them
// (tailwind before item).
func chainSpeedModifiers(mods []int) int {
	m := engine.Modifier4096
	for _, mod := range mods {
		m = (m*mod + engine.Modifier4096/2) / engine.Modifier4096
	}
	return m
}

// applySpeedModifiers はランク適用後の実数値に、連結済みの素早さ補正を五捨五超入で 1 回だけ掛ける
// (floor((v × M + engine.Modifier4096/2 − 1) / engine.Modifier4096)。ADR-0701 §2・ADR-0702 §2。
// 丸めの向きが chainSpeedModifiers の 1 ステップと違うのは @smogon/calc の原典どおり)。
func applySpeedModifiers(v int, mods []int) int {
	m := chainSpeedModifiers(mods)
	return (v*m + engine.Modifier4096/2 - 1) / engine.Modifier4096
}

// validateSP は SP の各ステータスが 0..MaxSPPerStat、合計が MaxSPTotal 以下であることを確かめる
// (CLAUDE.md のドメイン規約)。
func validateSP(sp engine.Stats) error {
	for _, k := range engine.AllStatKeys() {
		v := sp.Get(k)
		if v < 0 || v > engine.MaxSPPerStat {
			return ErrInvalidSP
		}
	}
	if sp.Sum() > engine.MaxSPTotal {
		return ErrInvalidSP
	}
	return nil
}

// validateRanks はランクが -6..+6 の範囲であることを確かめる。
func validateRanks(ranks engine.Ranks) error {
	for _, v := range []int{ranks.Atk, ranks.Def, ranks.SpA, ranks.SpD, ranks.Spe} {
		if v < minRank || v > maxRank {
			return ErrInvalidRank
		}
	}
	return nil
}

// CompareSpeed は attacker と defender の戦闘中の素早さを求めて比較する。outspeeds は
// トリックルームが無ければ厳密な >、あれば < に反転する(ADR-0702 §3)。speedTie は常に ==
// (ADR-0700 §6-1。同速を真偽値 1 つに丸めない。トリックルームでも反転しない)。どちらかが
// 範囲外ならその sentinel エラーを返す。
func CompareSpeed(attacker, defender Individual, field SpeedField) (SpeedComparison, error) {
	attackerSpeed, err := Speed(attacker)
	if err != nil {
		return SpeedComparison{}, err
	}
	defenderSpeed, err := Speed(defender)
	if err != nil {
		return SpeedComparison{}, err
	}
	outspeeds := attackerSpeed > defenderSpeed
	if field.TrickRoom {
		outspeeds = attackerSpeed < defenderSpeed
	}
	return SpeedComparison{
		AttackerSpeed: attackerSpeed,
		DefenderSpeed: defenderSpeed,
		Outspeeds:     outspeeds,
		SpeedTie:      attackerSpeed == defenderSpeed,
	}, nil
}

// IsChoiceScarf は itemID が既知のこだわりスカーフ ID と一致するかを返す(ADR-0701 §3)。
// scarfItemID が空なら DefaultChoiceScarfItemID を使う。judge は持ち物の一覧を持たず、
// 設定 1 つの ID との一致だけで判定する。
func IsChoiceScarf(itemID, scarfItemID string) bool {
	if itemID == "" {
		return false
	}
	if scarfItemID == "" {
		scarfItemID = DefaultChoiceScarfItemID
	}
	return itemID == scarfItemID
}

// 素早さの計算に効かせた補正の名前(契約の SpeedFactor。ADR-0710)。
const (
	SpeedFactorRank        = "rank"
	SpeedFactorTailwind    = "tailwind"
	SpeedFactorAbility     = "ability"
	SpeedFactorItem        = "item"
	SpeedFactorChoiceScarf = "choiceScarf"
	SpeedFactorParalysis   = "paralysis"
)

// AppliedSpeedFactors は Speed が実際に効かせる補正を、rank → tailwind → ability → choiceScarf / item → paralysis の順で返す
// (ADR-0710・ADR-0712・ADR-0714 §4)。効かせる補正が無ければ空のスライス(nil にはしない)。
func AppliedSpeedFactors(in Individual) []string {
	factors := []string{}
	if in.Ranks.Spe != 0 {
		factors = append(factors, SpeedFactorRank)
	}
	if in.Tailwind {
		factors = append(factors, SpeedFactorTailwind)
	}
	if _, ok := abilitySpeedModifier(in); ok {
		factors = append(factors, SpeedFactorAbility)
	}
	if in.Scarf {
		factors = append(factors, SpeedFactorChoiceScarf)
	} else if _, ok := itemSpeedModifier(in); ok {
		factors = append(factors, SpeedFactorItem)
	}
	if paralysisHalves(in) {
		factors = append(factors, SpeedFactorParalysis)
	}
	return factors
}

// 素早さに影響しうるのに反映していない入力の名前(契約の SpeedIgnoredInput。ADR-0710)。
const (
	IgnoredSpeedAbility = "abilityId"
	IgnoredSpeedItem    = "itemId"
	IgnoredSpeedWeather = "fieldWeather"
)

// IgnoredSpeedInputs は、指定されたが素早さに反映していない入力を abilityId → itemId → fieldWeather の順で返す
// (ADR-0710)。特性・持ち物の効果データが引けないときの第1段の規則で、「影響するか」ではなく
// 「指定されたか」で判定する(素早さに効かない特性でも入る)。データが引けるときは IgnoredSpeedInputsFor を使う。
//   - ability: abilityID が空でない
//   - item: itemID が空でなく、こだわりスカーフではない(isScarf が false)
//   - weather: 天候があり(none 以外)、かつ abilityID も指定されている(天候依存の素早さ特性があり得るため)
func IgnoredSpeedInputs(abilityID, itemID string, isScarf, hasWeather bool) []string {
	return IgnoredSpeedInputsFor(SpeedInputResolution{
		AbilitySpecified: abilityID != "",
		ItemSpecified:    itemID != "" && !isScarf,
		HasWeather:       hasWeather,
	})
}

// SpeedInputResolution は、指定された特性・持ち物の素早さへの効き方を効果データで確定できたか(ADR-0714 §4)。
// Resolved が true なのは、データに載っていて、効果の形が正しく、条件を評価できたとき
// (効果なし・条件不成立・反映済みのどれでも)。ItemSpecified はスカーフ以外の持ち物が指定されたとき。
type SpeedInputResolution struct {
	AbilitySpecified bool
	AbilityResolved  bool
	ItemSpecified    bool
	ItemResolved     bool
	// HasWeather は天候があるか(none 以外)。
	HasWeather bool
}

// IgnoredSpeedInputsFor は、指定されたが効き方を確定できなかった入力を abilityId → itemId → fieldWeather の順で返す
// (ADR-0714 §4)。fieldWeather は天候があり、かつ abilityId が入るときだけ(天候依存の特性を評価できなかったため)。
func IgnoredSpeedInputsFor(r SpeedInputResolution) []string {
	ignored := []string{}
	abilityIgnored := r.AbilitySpecified && !r.AbilityResolved
	if abilityIgnored {
		ignored = append(ignored, IgnoredSpeedAbility)
	}
	if r.ItemSpecified && !r.ItemResolved {
		ignored = append(ignored, IgnoredSpeedItem)
	}
	if r.HasWeather && abilityIgnored {
		ignored = append(ignored, IgnoredSpeedWeather)
	}
	return ignored
}
