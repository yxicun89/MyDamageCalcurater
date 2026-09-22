package httpapi

import (
	"errors"
	"net/http"

	"example.com/pokecalc/services/balance/internal/api"
	"example.com/pokecalc/services/balance/internal/balance"
	"github.com/labstack/echo/v5"
)

// RecommendTeamTypes is the TB5 recommendation endpoint (ADR-0401).
func (h handler) RecommendTeamTypes(c *echo.Context, _ api.RecommendTeamTypesParams) error {
	return recommendations(c, h.deps)
}

// recommendations implements the TB5 recommendation endpoint (ADR-0401 §2〜6), reusing the
// TB4 request/resolve helpers (validateThreatsEntry, resolveThreats*, threatsResolveError):
// a recommendations member has the same shape as a threats entry.
//
// Validation order (ADR-0401 §6, the same flavor as TB4 §4/§6): header (400, via
// requireRequestContext) → body (400/413: members count 1..6, per-member pokemonId/
// moveIds/abilityId, limit range 1..20) → pokemon read model or catalog absent, move
// read model absent while any moveId is named, or ability read model absent while any
// abilityId is named (503) → unknown pokemonId → unknown moveId → unknown abilityId
// (422; request order, the first one) → 200. Every other failure (provider failure, nil
// chart, invalid move/ability data) answers 500 with a fixed message.
func recommendations(c *echo.Context, deps Dependencies) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxAnalyzeBodyBytes)
	request, err := decodeJSONBody[api.RecommendationsRequest](c.Request())
	if err != nil {
		if errors.Is(err, errRequestTooLarge) {
			return c.JSON(http.StatusRequestEntityTooLarge, api.Error{
				Code:    api.RequestTooLarge,
				Message: err.Error(),
			})
		}
		return c.JSON(http.StatusBadRequest, api.Error{
			Code:    api.InvalidRequest,
			Message: err.Error(),
		})
	}

	if len(request.Members) < 1 || len(request.Members) > balance.MaxMembers {
		return c.JSON(http.StatusBadRequest, api.Error{
			Code:    api.InvalidRequest,
			Message: "members must contain between one and six entries",
		})
	}

	entries := make([]api.ThreatsRequestPokemon, len(request.Members))
	var hasMoveID, hasAbilityID bool
	for i, member := range request.Members {
		entry := api.ThreatsRequestPokemon{
			PokemonId: member.PokemonId,
			MoveIds:   member.MoveIds,
			AbilityId: member.AbilityId,
		}
		entryHasMoveID, entryHasAbilityID, err := validateThreatsEntry(entry)
		if err != nil {
			return c.JSON(http.StatusBadRequest, api.Error{
				Code:    api.InvalidRequest,
				Message: err.Error(),
			})
		}
		entries[i] = entry
		hasMoveID = hasMoveID || entryHasMoveID
		hasAbilityID = hasAbilityID || entryHasAbilityID
	}

	limit := balance.DefaultRecommendationLimit
	if request.Limit != nil {
		limit = *request.Limit
		if limit < balance.MinRecommendationLimit || limit > balance.MaxRecommendationLimit {
			return c.JSON(http.StatusBadRequest, api.Error{
				Code:    api.InvalidRequest,
				Message: "limit must be between 1 and 20",
			})
		}
	}

	// ADR-0401 §6: pokemon read model or catalog absent → move read model absent (with a
	// named moveId) → ability read model absent (with a named abilityId).
	if deps.PokemonTypes == nil || deps.PokemonCatalog == nil {
		return c.JSON(http.StatusServiceUnavailable, api.Error{
			Code:    api.MasterUnavailable,
			Message: "the pokemon type read model is not configured",
		})
	}
	if hasMoveID && deps.Moves == nil {
		return c.JSON(http.StatusServiceUnavailable, api.Error{
			Code:    api.MasterUnavailable,
			Message: "the move read model is not configured",
		})
	}
	if hasAbilityID && deps.Abilities == nil {
		return c.JSON(http.StatusServiceUnavailable, api.Error{
			Code:    api.MasterUnavailable,
			Message: "the ability read model is not configured",
		})
	}

	members, err := resolveThreatsPokemon(deps, entries)
	if err != nil {
		return threatsResolveError(c, err)
	}
	if err := resolveThreatsMoves(deps, entries, members); err != nil {
		return threatsResolveError(c, err)
	}
	if err := resolveThreatsAbilities(deps, entries, members); err != nil {
		return threatsResolveError(c, err)
	}

	catalog, err := deps.PokemonCatalog.AllPokemon()
	if err != nil {
		return internalError(c, err)
	}

	recommendation, err := balance.RecommendTypes(deps.TypeChart, members, catalog, deps.Abilities, limit)
	if err != nil {
		return internalError(c, err)
	}

	return c.JSON(http.StatusOK, toRecommendationsResponse(recommendation))
}

func toRecommendationsResponse(r balance.Recommendation) api.RecommendationsResponse {
	return api.RecommendationsResponse{
		DefenseHoles:   toRecommendationTypes(r.DefenseHoles),
		OffenseHoles:   toRecommendationTypes(r.OffenseHoles),
		Candidates:     toTypeCandidates(r.Candidates),
		AbilityOptions: toAbilityOptions(r.AbilityOptions),
	}
}

func toRecommendationTypes(types []balance.TypeID) []api.TypeId {
	result := make([]api.TypeId, len(types))
	for i, t := range types {
		result[i] = api.TypeId(t)
	}
	return result
}

func toTypeCandidates(candidates []balance.TypeCandidate) []api.TypeCandidate {
	result := make([]api.TypeCandidate, len(candidates))
	for i, candidate := range candidates {
		result[i] = api.TypeCandidate{
			Types:          toRecommendationTypes(candidate.Types),
			DefenseCovered: toRecommendationTypes(candidate.DefenseCovered),
			OffenseCovered: toRecommendationTypes(candidate.OffenseCovered),
			Weaknesses:     candidate.Weaknesses,
			Pokemon:        toCandidatePokemon(candidate.Pokemon),
		}
	}
	return result
}

func toCandidatePokemon(pokemon []balance.RecommendedPokemon) []api.CandidatePokemon {
	result := make([]api.CandidatePokemon, len(pokemon))
	for i, p := range pokemon {
		result[i] = api.CandidatePokemon{
			PokemonId:  p.PokemonID,
			Types:      toRecommendationTypes(p.Types),
			ExactMatch: p.ExactMatch,
		}
		if p.NameJa != "" {
			name := p.NameJa
			result[i].NameJa = &name
		}
	}
	return result
}

func toAbilityOptions(options []balance.AbilityOption) []api.AbilityOption {
	result := make([]api.AbilityOption, len(options))
	for i, option := range options {
		result[i] = api.AbilityOption{
			AttackType: api.TypeId(option.AttackType),
			Pokemon:    toAbilityOptionPokemon(option.Pokemon),
		}
	}
	return result
}

func toAbilityOptionPokemon(pokemon []balance.AbilityPokemon) []api.AbilityOptionPokemon {
	result := make([]api.AbilityOptionPokemon, len(pokemon))
	for i, p := range pokemon {
		result[i] = api.AbilityOptionPokemon{
			PokemonId:  p.PokemonID,
			AbilityId:  p.AbilityID,
			Multiplier: api.DefenseMultiplier(p.Multiplier.String()),
		}
		if p.NameJa != "" {
			name := p.NameJa
			result[i].NameJa = &name
		}
	}
	return result
}
