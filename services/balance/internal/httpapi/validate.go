package httpapi

import (
	"errors"
	"net/http"

	"example.com/pokecalc/services/balance/internal/api"
	"example.com/pokecalc/services/balance/internal/balance"
	"github.com/labstack/echo/v5"
)

// This file collects the request-body validation and provider-resolution helpers shared by
// analyze, coverage, threats and recommendations. ADR-0014 (TB1) §5.4/§5.6, ADR-0016 (TB2)
// §4/§6, ADR-0400 (TB4) §4/§6 and ADR-0401 (TB5) §6/§7 independently settled on the same
// per-entry checks (pokemonId format, moveIds presence/count/format/duplicates, abilityId
// format), the same validation order, and the same 422 mapping for a ResolveMembers/
// ResolveMoves/ResolveAbility failure; consolidating them here keeps that agreement
// mechanical instead of four copies that could drift.

// badRequest answers 400 invalid_request with message.
func badRequest(c *echo.Context, message string) error {
	return c.JSON(http.StatusBadRequest, api.Error{Code: api.InvalidRequest, Message: message})
}

// validateCount answers 400 invalid_request when n is outside 1..MaxMembers, naming noun
// ("members" or "threats") in the message.
func validateCount(c *echo.Context, n int, noun string) error {
	if n < 1 || n > balance.MaxMembers {
		return badRequest(c, noun+" must contain between one and six entries")
	}
	return nil
}

// validatePokemonID checks the NNNN-NNN format every endpoint's pokemonId must match.
func validatePokemonID(id string) error {
	if !pokemonIDPattern.MatchString(id) {
		return errors.New("pokemonId must use the NNNN-NNN format")
	}
	return nil
}

// validateAbilityIDFormat checks the AbilityId format (ADR-0017 §2): analyze's member.abilityId
// and the threats/recommendations entry.abilityId both use it.
func validateAbilityIDFormat(id string) error {
	if len(id) > maxAbilityIDLength || !abilityIDPattern.MatchString(id) {
		return errors.New("abilityId must match ^[a-z0-9]+(-[a-z0-9]+)*$ and be at most 40 characters")
	}
	return nil
}

// validateMoveIDs checks one member's moveIds (ADR-0016 §6.1): nil (an omitted field or an
// explicit JSON null, which decode the same way) is rejected, the count is capped at
// MaxMovesPerMember, each ID's format is checked, and IDs must be distinct within the
// member. hasMoveID reports whether moveIDs is non-empty, used by the 503 checks that only
// apply when a moveId was actually named.
func validateMoveIDs(moveIDs []string) (hasMoveID bool, err error) {
	if moveIDs == nil {
		return false, errors.New("moveIds is required")
	}
	if len(moveIDs) > balance.MaxMovesPerMember {
		return false, errors.New("a member must have at most four moves")
	}
	seen := make(map[string]struct{}, len(moveIDs))
	for _, moveID := range moveIDs {
		if len(moveID) > maxMoveIDLength || !moveIDPattern.MatchString(moveID) {
			return false, errors.New("moveId must match ^[a-z0-9]+(-[a-z0-9]+)*$ and be at most 40 characters")
		}
		if _, ok := seen[moveID]; ok {
			return false, errors.New("moveIds must be distinct within a member")
		}
		seen[moveID] = struct{}{}
	}
	return len(moveIDs) > 0, nil
}

// resolveError maps a ResolveMembers/ResolveMoves/ResolveAbility error to its 422 response
// (the message names only the request ID, never adapter detail), or to internalError for
// anything else.
func resolveError(c *echo.Context, err error) error {
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
