package balance

import (
	"errors"
	"fmt"
)

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
	if provider == nil {
		return Ability{}, ErrNilAbilities
	}
	ability, err := provider.Ability(abilityID)
	if errors.Is(err, ErrUnknownAbility) {
		// adapter の詳細(将来のファイルパス等)を落とし、ID だけを持つエラーにする(ADR-0016 §4 と同じ流儀)。
		return Ability{}, &UnknownAbilityError{AbilityID: abilityID}
	}
	if err != nil {
		return Ability{}, err
	}
	// response の abilityId は request の値のまま返す契約なので、provider が ID を正規化しても request の ID を使う。
	ability.AbilityID = abilityID
	return ability, nil
}

// UnknownAbilityError は provider に無い abilityId を表す。errors.Is(err, ErrUnknownAbility) が真になる。
type UnknownAbilityError struct {
	AbilityID string
}

func (e *UnknownAbilityError) Error() string {
	return fmt.Sprintf("%s: %s", ErrUnknownAbility, e.AbilityID)
}

// Unwrap returns ErrUnknownAbility only.
func (e *UnknownAbilityError) Unwrap() error { return ErrUnknownAbility }

// validateAbilityEffect checks one normalized effect (ADR-0017 §2): the kind must be known,
// its attackType (when applicable) must be one of the 18 types, and its factor (when
// applicable) must have a positive denominator.
func validateAbilityEffect(e AbilityEffect) error {
	switch e.Kind {
	case AbilityEffectImmune, AbilityEffectAbsorb:
		if !e.AttackType.Valid() {
			return fmt.Errorf("%w: %s effect has invalid attackType %q", ErrInvalidAbilityEffect, e.Kind, e.AttackType)
		}
	case AbilityEffectTypeMultiplier:
		if !e.AttackType.Valid() {
			return fmt.Errorf("%w: %s effect has invalid attackType %q", ErrInvalidAbilityEffect, e.Kind, e.AttackType)
		}
		if e.Factor.Den <= 0 || e.Factor.Num < 0 {
			return fmt.Errorf("%w: %s effect has invalid factor %+v", ErrInvalidAbilityEffect, e.Kind, e.Factor)
		}
	case AbilityEffectSuperEffectiveMultiplier:
		if e.Factor.Den <= 0 || e.Factor.Num < 0 {
			return fmt.Errorf("%w: %s effect has invalid factor %+v", ErrInvalidAbilityEffect, e.Kind, e.Factor)
		}
	default:
		return fmt.Errorf("%w: unknown kind %q", ErrInvalidAbilityEffect, e.Kind)
	}
	return nil
}

// CalculateDefenseWithAbility is CalculateDefense followed by the ability effects in the
// ADR-0017 §3 order. A nil ability gives exactly the CalculateDefense result. Every effect
// of the ability is validated (ErrInvalidAbilityEffect), whatever the attack type.
//
// Order: the type matchup is computed first; a type-derived immunity (x0) is final
// (source=type) and the ability is not applied to it, though its effects are still
// validated. Otherwise immune/absorb effects matching the attack type force x0
// (source=ability). type_multiplier applies to its own attack type; super_effective_multiplier
// applies only when the type matchup (before any ability effect) is above x1. Effects are
// applied in order. If the final value equals the type-only value (including a coefficient
// of 1, or effects that cancel out), the result keeps source=type and effect=none.
func CalculateDefenseWithAbility(chart TypeChartProvider, attack TypeID, defenseTypes []TypeID, ability *Ability) (DefenseResult, error) {
	base, err := CalculateDefense(chart, attack, defenseTypes)
	if err != nil {
		return DefenseResult{}, err
	}
	if ability == nil {
		return base, nil
	}

	// Every effect of the ability is validated, whatever the attack type (ADR-0017 spec §5).
	for _, effect := range ability.Effects {
		if err := validateAbilityEffect(effect); err != nil {
			return DefenseResult{}, err
		}
	}

	// A type-derived immunity is final; the ability cannot change it (ADR-0017 §3・§5.5).
	if base.Effectiveness.IsZero() {
		return base, nil
	}

	current := base.Effectiveness
	superEffective := base.Effectiveness.Cmp(Effectiveness{Num: 1, Den: 1}) > 0
	var lastZeroEffect DefenseEffect
	for _, effect := range ability.Effects {
		switch effect.Kind {
		case AbilityEffectImmune:
			if effect.AttackType == attack {
				current = Effectiveness{Num: 0, Den: 1}
				lastZeroEffect = DefenseEffectImmune
			}
		case AbilityEffectAbsorb:
			if effect.AttackType == attack {
				current = Effectiveness{Num: 0, Den: 1}
				lastZeroEffect = DefenseEffectAbsorb
			}
		case AbilityEffectTypeMultiplier:
			if effect.AttackType == attack {
				current = current.Mul(effect.Factor)
			}
		case AbilityEffectSuperEffectiveMultiplier:
			if superEffective {
				current = current.Mul(effect.Factor)
			}
		}
	}

	result := base
	result.Effectiveness = current
	if current == base.Effectiveness {
		result.Source = EffectSourceType
		result.Effect = DefenseEffectNone
		return result, nil
	}
	result.Source = EffectSourceAbility
	if current.IsZero() {
		result.Effect = lastZeroEffect
	} else {
		result.Effect = DefenseEffectMultiplier
	}
	return result, nil
}
