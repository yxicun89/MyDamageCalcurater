package httpapi

import (
	"errors"
	"net/http"

	"example.com/pokecalc/services/balance/internal/api"
	"example.com/pokecalc/services/balance/internal/balance"
	"github.com/labstack/echo/v5"
)

// threats implements the TB4 threat-check endpoint (ADR-0400).
//
// Validation order (ADR-0400 §4/§6): header (400, via requireRequestContext) → body
// (400/413: members/threats count 1..6, per-entry pokemonId/moveIds/abilityId) →
// pokemon read model absent, or move read model absent while any moveId is named,
// or ability read model absent while any abilityId is named (503) → unknown
// pokemonId → unknown moveId → unknown abilityId (422; each members before threats,
// request order, the first one) → 200. Every other failure (provider failure, nil
// chart, invalid move/ability data, multiplier overflow) answers 500 with a fixed
// message, via balance.AnalyzeThreats and internalError.
func threats(c *echo.Context, deps Dependencies) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxAnalyzeBodyBytes)
	request, err := decodeJSONBody[api.ThreatsRequest](c.Request())
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
	if len(request.Threats) < 1 || len(request.Threats) > balance.MaxMembers {
		return c.JSON(http.StatusBadRequest, api.Error{
			Code:    api.InvalidRequest,
			Message: "threats must contain between one and six entries",
		})
	}

	var hasMoveID, hasAbilityID bool
	for _, side := range [][]api.ThreatsRequestPokemon{request.Members, request.Threats} {
		for _, entry := range side {
			entryHasMoveID, entryHasAbilityID, err := validateThreatsEntry(entry)
			if err != nil {
				return c.JSON(http.StatusBadRequest, api.Error{
					Code:    api.InvalidRequest,
					Message: err.Error(),
				})
			}
			hasMoveID = hasMoveID || entryHasMoveID
			hasAbilityID = hasAbilityID || entryHasAbilityID
		}
	}

	// ADR-0400 §4: pokemon read model absent → move read model absent (with a named
	// moveId) → ability read model absent (with a named abilityId).
	if deps.PokemonTypes == nil {
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

	memberCombatants, err := resolveThreatsPokemon(deps, request.Members)
	if err != nil {
		return threatsResolveError(c, err)
	}
	threatCombatants, err := resolveThreatsPokemon(deps, request.Threats)
	if err != nil {
		return threatsResolveError(c, err)
	}

	if err := resolveThreatsMoves(deps, request.Members, memberCombatants); err != nil {
		return threatsResolveError(c, err)
	}
	if err := resolveThreatsMoves(deps, request.Threats, threatCombatants); err != nil {
		return threatsResolveError(c, err)
	}

	if err := resolveThreatsAbilities(deps, request.Members, memberCombatants); err != nil {
		return threatsResolveError(c, err)
	}
	if err := resolveThreatsAbilities(deps, request.Threats, threatCombatants); err != nil {
		return threatsResolveError(c, err)
	}

	analysis, err := balance.AnalyzeThreats(deps.TypeChart, memberCombatants, threatCombatants)
	if err != nil {
		return internalError(c, err)
	}

	return c.JSON(http.StatusOK, toThreatsResponse(analysis))
}

// validateThreatsEntry checks one member or threat entry (ADR-0400 §2/§6): the
// pokemonId format, the moveIds presence/count/format/duplicates (mirrors coverage's
// per-member checks, ADR-0016 §6.1: a missing or null moveIds decodes to a nil
// slice, so this also rejects the field's omission or an explicit JSON null), and
// the abilityId format when present. hasMoveID/hasAbilityID report whether the
// entry names at least one, for the 503 checks.
func validateThreatsEntry(entry api.ThreatsRequestPokemon) (hasMoveID, hasAbilityID bool, err error) {
	if !pokemonIDPattern.MatchString(entry.PokemonId) {
		return false, false, errors.New("pokemonId must use the NNNN-NNN format")
	}
	if entry.MoveIds == nil {
		return false, false, errors.New("moveIds is required")
	}
	if len(entry.MoveIds) > balance.MaxMovesPerMember {
		return false, false, errors.New("a member must have at most four moves")
	}
	seen := make(map[string]struct{}, len(entry.MoveIds))
	for _, moveID := range entry.MoveIds {
		if len(moveID) > maxMoveIDLength || !moveIDPattern.MatchString(moveID) {
			return false, false, errors.New("moveId must match ^[a-z0-9]+(-[a-z0-9]+)*$ and be at most 40 characters")
		}
		if _, ok := seen[moveID]; ok {
			return false, false, errors.New("moveIds must be distinct within a member")
		}
		seen[moveID] = struct{}{}
	}
	hasMoveID = len(entry.MoveIds) > 0

	if entry.AbilityId != nil {
		if len(*entry.AbilityId) > maxAbilityIDLength || !abilityIDPattern.MatchString(*entry.AbilityId) {
			return hasMoveID, false, errors.New("abilityId must match ^[a-z0-9]+(-[a-z0-9]+)*$ and be at most 40 characters")
		}
		hasAbilityID = true
	}
	return hasMoveID, hasAbilityID, nil
}

// resolveThreatsPokemon resolves one side's pokemonId list to Combatants with
// their types (ADR-0400 §5: the existing pokemon type read model, no new data).
func resolveThreatsPokemon(deps Dependencies, entries []api.ThreatsRequestPokemon) ([]balance.Combatant, error) {
	pokemonIDs := make([]string, len(entries))
	for i, entry := range entries {
		pokemonIDs[i] = entry.PokemonId
	}
	members, err := balance.ResolveMembers(deps.PokemonTypes, pokemonIDs)
	if err != nil {
		return nil, err
	}
	combatants := make([]balance.Combatant, len(members))
	for i, member := range members {
		combatants[i] = balance.Combatant{PokemonID: member.PokemonID, Types: member.Types}
	}
	return combatants, nil
}

// resolveThreatsMoves resolves each entry's moveIds into the matching combatant's
// Moves, in request order. An entry without any moveId skips the move provider
// entirely (ADR-0400 §4: the move read model is not needed then).
func resolveThreatsMoves(deps Dependencies, entries []api.ThreatsRequestPokemon, combatants []balance.Combatant) error {
	for i, entry := range entries {
		if len(entry.MoveIds) == 0 {
			continue
		}
		moves, err := balance.ResolveMoves(deps.Moves, entry.MoveIds)
		if err != nil {
			return err
		}
		combatants[i].Moves = moves
	}
	return nil
}

// resolveThreatsAbilities resolves each entry's optional abilityId into the
// matching combatant's Ability, in request order. An entry without an abilityId
// skips the ability provider entirely (ADR-0400 §4/§6.4: request abilityId: null
// decodes the same as an omitted field, so it is likewise skipped here).
func resolveThreatsAbilities(deps Dependencies, entries []api.ThreatsRequestPokemon, combatants []balance.Combatant) error {
	for i, entry := range entries {
		if entry.AbilityId == nil {
			continue
		}
		ability, err := balance.ResolveAbility(deps.Abilities, *entry.AbilityId)
		if err != nil {
			return err
		}
		combatants[i].Ability = &ability
	}
	return nil
}

// threatsResolveError maps a ResolveMembers/ResolveMoves/ResolveAbility error to its
// 422 response, or to internalError for anything else (ADR-0400 §4/ADR-0014 §5.6:
// the message names only the request ID, never adapter detail).
func threatsResolveError(c *echo.Context, err error) error {
	var unknownPokemon *balance.UnknownPokemonError
	if errors.As(err, &unknownPokemon) {
		return c.JSON(http.StatusUnprocessableEntity, api.Error{
			Code:    api.UnknownPokemon,
			Message: "unknown pokemonId: " + unknownPokemon.PokemonID,
		})
	}
	var unknownMove *balance.UnknownMoveError
	if errors.As(err, &unknownMove) {
		return c.JSON(http.StatusUnprocessableEntity, api.Error{
			Code:    api.UnknownMove,
			Message: "unknown moveId: " + unknownMove.MoveID,
		})
	}
	var unknownAbility *balance.UnknownAbilityError
	if errors.As(err, &unknownAbility) {
		return c.JSON(http.StatusUnprocessableEntity, api.Error{
			Code:    api.UnknownAbility,
			Message: "unknown abilityId: " + unknownAbility.AbilityID,
		})
	}
	return internalError(c, err)
}

func toThreatsResponse(analysis balance.ThreatAnalysis) api.ThreatsResponse {
	threatResults := make([]api.ThreatResult, len(analysis.Threats))
	for i, result := range analysis.Threats {
		threatResults[i] = toThreatResult(result)
	}
	return api.ThreatsResponse{Threats: threatResults}
}

func toThreatResult(result balance.ThreatResult) api.ThreatResult {
	attackTypes := make([]api.TypeId, len(result.AttackTypes))
	for i, t := range result.AttackTypes {
		attackTypes[i] = api.TypeId(t)
	}
	matchups := make([]api.ThreatMatchup, len(result.Matchups))
	for i, matchup := range result.Matchups {
		matchups[i] = api.ThreatMatchup{
			PokemonId:      matchup.PokemonID,
			Incoming:       toMatchupMultiplier(matchup.Incoming),
			Outgoing:       toMatchupMultiplier(matchup.Outgoing),
			Safe:           matchup.Safe,
			SuperEffective: matchup.SuperEffective,
		}
	}
	threatResult := api.ThreatResult{
		PokemonId:             result.PokemonID,
		AttackTypes:           attackTypes,
		Matchups:              matchups,
		SafeMembers:           result.SafeMembers,
		SuperEffectiveMembers: result.SuperEffectiveMembers,
	}
	// The response abilityId is present only when the request threat named one (ADR-0400 §6.4).
	if result.AbilityID != "" {
		id := result.AbilityID
		threatResult.AbilityId = &id
	}
	return threatResult
}

func toMatchupMultiplier(e *balance.Effectiveness) *api.MatchupMultiplier {
	if e == nil {
		return nil
	}
	v := api.MatchupMultiplier(e.String())
	return &v
}
