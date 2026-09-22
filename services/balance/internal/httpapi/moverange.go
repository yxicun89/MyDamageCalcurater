package httpapi

import (
	"errors"
	"net/http"

	"example.com/pokecalc/services/balance/internal/api"
	"example.com/pokecalc/services/balance/internal/balance"
	"github.com/labstack/echo/v5"
)

// AnalyzeMoveRange is the TB6 move-range endpoint (ADR-0404).
func (h handler) AnalyzeMoveRange(c *echo.Context, _ api.AnalyzeMoveRangeParams) error {
	return moveRange(c, h.deps)
}

// moveRange implements the TB6 move-range endpoint (ADR-0404 §2・§4).
//
// Validation order: header (400, via requireRequestContext) → body (400/413: moveIds count
// 1..4, format, distinct) → move read model absent (503; moveIds is required, so this check is
// always reached) → unknown moveId (422, request order, first one) → pokemon catalog absent
// (503; walledBy always needs it) → 200. A move set that resolves to all-status moves is 400
// invalid_request (ADR-0404 §4.2: this check can sit after the 422/503 checks above). Any other
// internal failure answers 500 with the fixed message.
func moveRange(c *echo.Context, deps Dependencies) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxAnalyzeBodyBytes)
	request, err := decodeJSONBody[api.MoveRangeRequest](c.Request())
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

	if request.MoveIds == nil {
		return badRequest(c, "moveIds is required")
	}
	if len(request.MoveIds) < balance.MinMoveRangeMoves || len(request.MoveIds) > balance.MaxMoveRangeMoves {
		return badRequest(c, "moveIds must contain between one and four moves")
	}
	if _, err := validateMoveIDs(request.MoveIds); err != nil {
		return badRequest(c, err.Error())
	}

	// moveIds is required and non-empty (checked above), so the move read model is always needed.
	if deps.Moves == nil {
		return c.JSON(http.StatusServiceUnavailable, api.Error{
			Code:    api.MasterUnavailable,
			Message: "the move read model is not configured",
		})
	}

	moves, err := balance.ResolveMoves(deps.Moves, request.MoveIds)
	if err != nil {
		return resolveError(c, err)
	}

	// walledBy always needs the catalog (ADR-0404 §2・§4.4); PokemonTypes is not used by TB6.
	if deps.PokemonCatalog == nil {
		return c.JSON(http.StatusServiceUnavailable, api.Error{
			Code:    api.MasterUnavailable,
			Message: "the pokemon catalog read model is not configured",
		})
	}

	catalog, err := deps.PokemonCatalog.AllPokemon()
	if err != nil {
		return internalError(c, err)
	}

	analysis, err := balance.AnalyzeMoveRange(deps.TypeChart, moves, catalog, deps.Abilities)
	if err != nil {
		if errors.Is(err, balance.ErrMoveRangeNoAttackMove) {
			return badRequest(c, "moveIds must contain at least one attack move")
		}
		return internalError(c, err)
	}

	return c.JSON(http.StatusOK, toMoveRangeResponse(analysis))
}

func toMoveRangeResponse(a balance.MoveRangeAnalysis) api.MoveRangeResponse {
	attackTypes := make([]api.TypeId, len(a.AttackTypes))
	for i, t := range a.AttackTypes {
		attackTypes[i] = api.TypeId(t)
	}

	typeChart := make([]api.MoveRangeTypeEntry, len(a.TypeChart))
	for i, entry := range a.TypeChart {
		typeChart[i] = api.MoveRangeTypeEntry{
			DefenseType:    api.TypeId(entry.DefenseType),
			BestMultiplier: api.MoveRangeMultiplier(entry.BestMultiplier.String()),
			Effective:      entry.Effective,
			SuperEffective: entry.SuperEffective,
		}
	}

	walledBy := make([]api.WalledByPokemon, len(a.WalledBy))
	for i, p := range a.WalledBy {
		types := make([]api.TypeId, len(p.Types))
		for j, t := range p.Types {
			types[j] = api.TypeId(t)
		}
		walledBy[i] = api.WalledByPokemon{
			PokemonId:      p.PokemonID,
			Types:          types,
			BestMultiplier: api.DefenseMultiplier(p.BestMultiplier.String()),
		}
		if p.NameJa != "" {
			name := p.NameJa
			walledBy[i].NameJa = &name
		}
	}

	walledByAbility := make([]api.WalledByAbilityPokemon, len(a.WalledByAbility))
	for i, p := range a.WalledByAbility {
		walledByAbility[i] = api.WalledByAbilityPokemon{
			PokemonId:      p.PokemonID,
			AbilityId:      p.AbilityID,
			BestMultiplier: api.DefenseMultiplier(p.BestMultiplier.String()),
		}
		if p.NameJa != "" {
			name := p.NameJa
			walledByAbility[i].NameJa = &name
		}
	}

	return api.MoveRangeResponse{
		AttackTypes:     attackTypes,
		TypeChart:       typeChart,
		WalledBy:        walledBy,
		WalledByAbility: walledByAbility,
	}
}
