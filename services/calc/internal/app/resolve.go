package app

// Master を使った ID の解決(unknown_*)。

import (
	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/calc/internal/domain"
)

// ResolveIndividual は個体を解決する。列挙(status・teraType)を先に検証し、
// その後で ID(speciesKey・natureId・abilityId・itemId)を Master で解決する。
func (s *Service) ResolveIndividual(label string, in Individual) (engine.Individual, error) {
	status, err := domain.ParseStatus(label, in.Status)
	if err != nil {
		return engine.Individual{}, err
	}
	tera, err := domain.ParseTeraType(label, in.TeraType)
	if err != nil {
		return engine.Individual{}, err
	}
	species, ok := s.master.Species(in.SpeciesKey)
	if !ok {
		return engine.Individual{}, domain.Errorf(domain.CodeUnknownSpecies, "%s.speciesKey が見つからない: %q", label, in.SpeciesKey)
	}
	nature, ok := s.master.Nature(in.NatureID)
	if !ok {
		return engine.Individual{}, domain.Errorf(domain.CodeUnknownNature, "%s.natureId が見つからない: %q", label, in.NatureID)
	}
	var ability engine.Ability
	if in.AbilityID != nil {
		ability, ok = s.master.Ability(*in.AbilityID)
		if !ok {
			return engine.Individual{}, domain.Errorf(domain.CodeUnknownAbility, "%s.abilityId が見つからない: %q", label, *in.AbilityID)
		}
	}
	var item *engine.Item
	if in.ItemID != nil {
		it, ok := s.master.Item(*in.ItemID)
		if !ok {
			return engine.Individual{}, domain.Errorf(domain.CodeUnknownItem, "%s.itemId が見つからない: %q", label, *in.ItemID)
		}
		item = &it
	}
	return engine.Individual{
		Species: species, Level: derefInt(in.Level), Nature: nature, Ability: ability, Item: item,
		SP: in.SP, Ranks: in.Ranks.engine(), TeraType: tera, Status: status,
	}, nil
}

// resolveSpecies は種族キーを解決する(bulk の defenderSpeciesKey / reverse の unknownSpeciesKey)。
func (s *Service) resolveSpecies(label, key string) (engine.Species, error) {
	sp, ok := s.master.Species(key)
	if !ok {
		return engine.Species{}, domain.Errorf(domain.CodeUnknownSpecies, "%s が見つからない: %q", label, key)
	}
	return sp, nil
}

// ResolveMove は技 ID を解決する。
func (s *Service) ResolveMove(id string) (engine.Move, error) {
	mv, ok := s.master.Move(id)
	if !ok {
		return engine.Move{}, domain.Errorf(domain.CodeUnknownMove, "moveId が見つからない: %q", id)
	}
	return mv, nil
}

// resolveAbilityCandidates は特性の候補を解決する(issue 272。ADR-0126・ADR-0214)。
// overrideID が指定されていればその1件だけを返す(種族が持つかどうかは engine.CalcBulk/CalcReverse の
// abilityCandidates が検証し、ErrInvalidAbilityCandidates を invalid_input に写す。domain.FromEngine 参照)。
// 省略時は species.Abilities(スロット順)から先頭 engine.MaxAbilityCandidates 件を候補にする
// (4件目がある種族は Showdown の特殊枠 "S" を落とす。ADR-0105 §5 と同じ判断)。
// species.Abilities の各 ID がマスタに無いことは起きない(buildSpecies が起動時に検証済み)。
// label は overrideID 解決失敗時のエラーメッセージにそのまま使うフィールド名(呼び出し側が
// "defenderOverride.abilityId"/"unknownAbilityId" 等、末尾に ".abilityId" を含む完全な形で渡すこと)。
func (s *Service) resolveAbilityCandidates(label string, species engine.Species, overrideID *string) ([]engine.Ability, error) {
	if overrideID != nil {
		a, ok := s.master.Ability(*overrideID)
		if !ok {
			return nil, domain.Errorf(domain.CodeUnknownAbility, "%s が見つからない: %q", label, *overrideID)
		}
		return []engine.Ability{a}, nil
	}
	ids := species.Abilities
	if len(ids) > engine.MaxAbilityCandidates {
		ids = ids[:engine.MaxAbilityCandidates]
	}
	out := make([]engine.Ability, 0, len(ids))
	for _, id := range ids {
		a, ok := s.master.Ability(id)
		if !ok {
			return nil, domain.Errorf(domain.CodeInternal, "%s: マスタの不整合(種族の特性 %q が特性一覧に無い)", label, id)
		}
		out = append(out, a)
	}
	return out, nil
}

// resolveItems は持ち物 ID の配列(null は持ち物なし)を解決する。省略(nil)は nil のまま返す
// (engine 側が「持ち物なしの1通り」に既定するため。ADR-0200)。
func (s *Service) resolveItems(label string, ids *[]*string) ([]*engine.Item, error) {
	if ids == nil {
		return nil, nil
	}
	out := make([]*engine.Item, 0, len(*ids))
	for i, id := range *ids {
		if id == nil {
			out = append(out, nil)
			continue
		}
		it, ok := s.master.Item(*id)
		if !ok {
			return nil, domain.Errorf(domain.CodeUnknownItem, "%s[%d] が見つからない: %q", label, i, *id)
		}
		out = append(out, &it)
	}
	return out, nil
}

// natureID は性格補正の構造値から性格 ID を引く(引けなければ nil)。
func (s *Service) natureID(n engine.Nature) *string {
	if id, ok := s.master.NatureID(n); ok {
		return &id
	}
	return nil
}
