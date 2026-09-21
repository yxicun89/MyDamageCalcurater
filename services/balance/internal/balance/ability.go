package balance

import "errors"

// TB3 の特性による防御相性の変化(ADR-0017)。効果は正規化されたデータで表し、
// 特性 ID ごとの分岐を持たない(設計書 §6 TB3)。

var (
	// ErrUnknownAbility reports an abilityId that the provider does not know.
	ErrUnknownAbility = errors.New("unknown ability")
	// ErrNilAbilities reports a missing AbilityProvider.
	ErrNilAbilities = errors.New("ability provider is required")
	// ErrInvalidAbilityEffect reports an effect whose kind, attack type or factor is invalid.
	ErrInvalidAbilityEffect = errors.New("invalid ability effect")
)

// DefenseEffect is the API enum value of how an ability decided a defensive value (ADR-0017 §3).
type DefenseEffect string

const (
	DefenseEffectNone       DefenseEffect = "none"
	DefenseEffectImmune     DefenseEffect = "immune"
	DefenseEffectAbsorb     DefenseEffect = "absorb"
	DefenseEffectMultiplier DefenseEffect = "multiplier"
)

// AbilityEffectKind is the read model value of one normalized ability effect (ADR-0017 §2).
type AbilityEffectKind string

const (
	// AbilityEffectImmune makes AttackType x0.
	AbilityEffectImmune AbilityEffectKind = "immune"
	// AbilityEffectAbsorb makes AttackType x0 (recovery and stat side effects are out of scope).
	AbilityEffectAbsorb AbilityEffectKind = "absorb"
	// AbilityEffectTypeMultiplier multiplies AttackType by Factor.
	AbilityEffectTypeMultiplier AbilityEffectKind = "type_multiplier"
	// AbilityEffectSuperEffectiveMultiplier multiplies by Factor when the type matchup is above x1.
	AbilityEffectSuperEffectiveMultiplier AbilityEffectKind = "super_effective_multiplier"
)

// AbilityEffect is one normalized effect. AttackType is empty for
// super_effective_multiplier; Factor is the zero value for immune and absorb.
type AbilityEffect struct {
	Kind       AbilityEffectKind
	AttackType TypeID
	Factor     Effectiveness
}

// Ability is one resolved ability. Effects may be empty (no type-related effect).
type Ability struct {
	AbilityID string
	Effects   []AbilityEffect
}

// AbilityProvider resolves an abilityId to its normalized effects. Unknown IDs return
// an error wrapping ErrUnknownAbility. It is the replacement boundary for the shared
// master snapshot; the domain package does not own it.
type AbilityProvider interface {
	Ability(abilityID string) (Ability, error)
}

// ResolveAbility looks up abilityID. A nil provider returns ErrNilAbilities. An unknown
// ID returns *UnknownAbilityError carrying only the ID (adapter detail dropped). Other
// provider errors are propagated. The returned AbilityID is the requested ID.
func ResolveAbility(provider AbilityProvider, abilityID string) (Ability, error) {
	// TODO(TB3): implement (ADR-0017 §4).
	return Ability{}, nil
}

// UnknownAbilityError は provider に無い abilityId を表す。errors.Is(err, ErrUnknownAbility) が真になる。
type UnknownAbilityError struct {
	AbilityID string
}

func (e *UnknownAbilityError) Error() string {
	// TODO(TB3): implement ("unknown ability: <ID>").
	return ""
}

// Unwrap returns ErrUnknownAbility only.
func (e *UnknownAbilityError) Unwrap() error {
	// TODO(TB3): implement.
	return nil
}

// CalculateDefenseWithAbility is CalculateDefense followed by the ability effects in the
// ADR-0017 §3 order. A nil ability gives exactly the CalculateDefense result. Every effect
// of the ability is validated (ErrInvalidAbilityEffect), whatever the attack type.
func CalculateDefenseWithAbility(chart TypeChartProvider, attack TypeID, defenseTypes []TypeID, ability *Ability) (DefenseResult, error) {
	// TODO(TB3): implement (ADR-0017 §3).
	return DefenseResult{}, nil
}
