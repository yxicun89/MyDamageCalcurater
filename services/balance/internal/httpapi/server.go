// Package httpapi contains the balance service HTTP adapter.
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"example.com/pokecalc/services/balance/internal/api"
	"example.com/pokecalc/services/balance/internal/balance"
	"github.com/labstack/echo/v5"
)

const (
	deviceIDHeader      = "X-Device-Id"
	sessionIDHeader     = "X-Session-Id"
	maxAnalyzeBodyBytes = 16 * 1024
)

var pokemonIDPattern = regexp.MustCompile(`^\d{4}-\d{3}$`)

// moveIDPattern mirrors the MoveId schema (ADR-0016 §2). The 40-character limit is
// checked separately: the pattern itself has no length bound.
var moveIDPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

const maxMoveIDLength = 40

// abilityIDPattern mirrors the AbilityId schema (ADR-0017 §2). The 40-character limit is
// checked separately: the pattern itself has no length bound.
var abilityIDPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

const maxAbilityIDLength = 40

var errRequestTooLarge = errors.New("request body exceeds 16 KiB")

// Dependencies are the replaceable master-data boundaries of the HTTP adapter.
// PokemonTypes may be nil: the service still starts, health stays 200, and
// analyze answers 503 master_unavailable (ADR-0014 §2).
// Moves may be nil: coverage answers 503 master_unavailable (ADR-0016 §4);
// analyze does not use it.
// Abilities may be nil: analyze answers 503 master_unavailable only when a member
// names an abilityId (ADR-0017 §4); coverage does not use it.
type Dependencies struct {
	TypeChart    balance.TypeChartProvider
	PokemonTypes balance.PokemonTypeProvider
	Moves        balance.MoveProvider
	Abilities    balance.AbilityProvider
}

// New returns the HTTP handler.
func New(deps Dependencies) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = writeHTTPError
	api.RegisterHandlersWithOptions(e, handler{deps: deps}, api.RegisterHandlersOptions{
		OperationMiddlewares: map[string][]echo.MiddlewareFunc{
			"analyzeTeamBalance":  {requireRequestContext},
			"analyzeTeamCoverage": {requireRequestContext},
		},
	})
	return e
}

type handler struct {
	deps Dependencies
}

var _ api.ServerInterface = handler{}

func (handler) Health(c *echo.Context) error {
	return health(c)
}

func (handler) PublicHealth(c *echo.Context) error {
	return health(c)
}

func (h handler) AnalyzeTeamBalance(c *echo.Context, _ api.AnalyzeTeamBalanceParams) error {
	return analyze(c, h.deps)
}

// AnalyzeTeamCoverage is the TB2 offensive coverage endpoint (ADR-0016).
func (h handler) AnalyzeTeamCoverage(c *echo.Context, _ api.AnalyzeTeamCoverageParams) error {
	return coverage(c, h.deps)
}

func health(c *echo.Context) error {
	return c.JSON(http.StatusOK, api.Health{Status: api.Ok})
}

func analyze(c *echo.Context, deps Dependencies) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxAnalyzeBodyBytes)
	request, err := decodeJSONBody[api.AnalyzeRequest](c.Request())
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
	if len(request.Members) < 1 || len(request.Members) > 6 {
		return c.JSON(http.StatusBadRequest, api.Error{
			Code:    api.InvalidRequest,
			Message: "members must contain between one and six entries",
		})
	}
	hasAbilityID := false
	for _, member := range request.Members {
		if !pokemonIDPattern.MatchString(member.PokemonId) {
			return c.JSON(http.StatusBadRequest, api.Error{
				Code:    api.InvalidRequest,
				Message: "pokemonId must use the NNNN-NNN format",
			})
		}
		if member.AbilityId != nil {
			if len(*member.AbilityId) > maxAbilityIDLength || !abilityIDPattern.MatchString(*member.AbilityId) {
				return c.JSON(http.StatusBadRequest, api.Error{
					Code:    api.InvalidRequest,
					Message: "abilityId must match ^[a-z0-9]+(-[a-z0-9]+)*$ and be at most 40 characters",
				})
			}
			hasAbilityID = true
		}
	}

	// ADR-0017 §4: header (400) → body (400/413) → provider absent (503) → pokemonId (422) → abilityId (422) → 200.
	if deps.PokemonTypes == nil {
		return c.JSON(http.StatusServiceUnavailable, api.Error{
			Code:    api.MasterUnavailable,
			Message: "the pokemon type read model is not configured",
		})
	}
	if hasAbilityID && deps.Abilities == nil {
		return c.JSON(http.StatusServiceUnavailable, api.Error{
			Code:    api.MasterUnavailable,
			Message: "the ability read model is not configured",
		})
	}

	pokemonIDs := make([]string, len(request.Members))
	for i, member := range request.Members {
		pokemonIDs[i] = member.PokemonId
	}
	members, err := balance.ResolveMembers(deps.PokemonTypes, pokemonIDs)
	if err != nil {
		var unknown *balance.UnknownPokemonError
		if errors.As(err, &unknown) {
			return c.JSON(http.StatusUnprocessableEntity, api.Error{
				Code:    api.UnknownPokemon,
				Message: "unknown pokemonId: " + unknown.PokemonID,
			})
		}
		return internalError(c, err)
	}

	for i, member := range request.Members {
		if member.AbilityId == nil {
			continue
		}
		ability, err := balance.ResolveAbility(deps.Abilities, *member.AbilityId)
		if err != nil {
			var unknown *balance.UnknownAbilityError
			if errors.As(err, &unknown) {
				return c.JSON(http.StatusUnprocessableEntity, api.Error{
					Code:    api.UnknownAbility,
					Message: "unknown abilityId: " + unknown.AbilityID,
				})
			}
			return internalError(c, err)
		}
		members[i].Ability = &ability
	}

	analysis, err := balance.AnalyzeDefense(deps.TypeChart, members)
	if err != nil {
		return internalError(c, err)
	}

	return c.JSON(http.StatusOK, toAnalyzeResponse(analysis))
}

// internalError answers 500 internal_error with a fixed message: ADR-0014 §5.5
// requires that unexpected internal failures (a missing/broken type chart, or a
// provider failure other than an unknown pokemonId) never leak internal detail
// to the client. The error is still logged for operators.
func internalError(c *echo.Context, err error) error {
	slog.Error("balance internal error", "path", c.Path(), "error", err)
	return c.JSON(http.StatusInternalServerError, api.Error{
		Code:    api.InternalError,
		Message: "internal error",
	})
}

func toAnalyzeResponse(analysis balance.DefenseAnalysis) api.AnalyzeResponse {
	members := make([]api.MemberDefense, len(analysis.Members))
	for i, member := range analysis.Members {
		members[i] = toMemberDefense(member)
	}
	summary := make([]api.TeamSummaryEntry, len(analysis.TeamSummary))
	for i, entry := range analysis.TeamSummary {
		summary[i] = api.TeamSummaryEntry{
			AttackType: api.TypeId(entry.AttackType),
			Weak:       entry.Weak,
			QuadWeak:   entry.QuadWeak,
			Resist:     entry.Resist,
			Immune:     entry.Immune,
			Neutral:    entry.Neutral,
		}
	}
	return api.AnalyzeResponse{Members: members, TeamSummary: summary}
}

func toMemberDefense(member balance.MemberDefense) api.MemberDefense {
	types := make([]api.TypeId, len(member.Types))
	for i, t := range member.Types {
		types[i] = api.TypeId(t)
	}
	defense := make([]api.DefenseEntry, len(member.Defense))
	for i, entry := range member.Defense {
		defense[i] = api.DefenseEntry{
			AttackType: api.TypeId(entry.AttackType),
			Multiplier: api.DefenseMultiplier(entry.Result.Effectiveness.String()),
			Category:   api.DefenseCategory(entry.Category),
			Source:     toEffectSource(entry.Result.Source),
			Effect:     toDefenseEffect(entry.Result.Effect),
		}
	}
	result := api.MemberDefense{PokemonId: member.PokemonID, Types: types, Defense: defense}
	// The response abilityId is present only when the request member named one (ADR-0017 §5.2).
	if member.AbilityID != "" {
		id := member.AbilityID
		result.AbilityId = &id
	}
	return result
}

func toEffectSource(source balance.EffectSource) api.EffectSource {
	if source == balance.EffectSourceAbility {
		return api.Ability
	}
	return api.Type
}

func toDefenseEffect(effect balance.DefenseEffect) api.DefenseEffect {
	switch effect {
	case balance.DefenseEffectImmune:
		return api.EffectImmune
	case balance.DefenseEffectAbsorb:
		return api.EffectAbsorb
	case balance.DefenseEffectMultiplier:
		return api.EffectMultiplier
	default:
		return api.EffectNone
	}
}

func requireRequestContext(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if strings.TrimSpace(c.Request().Header.Get(deviceIDHeader)) == "" ||
			strings.TrimSpace(c.Request().Header.Get(sessionIDHeader)) == "" {
			return c.JSON(http.StatusBadRequest, api.Error{
				Code:    api.MissingRequestContext,
				Message: "X-Device-Id and X-Session-Id are required",
			})
		}
		return next(c)
	}
}

func writeHTTPError(c *echo.Context, err error) {
	if response, _ := echo.UnwrapResponse(c.Response()); response != nil && response.Committed {
		return
	}
	var httpError *echo.HTTPError
	if errors.As(err, &httpError) && httpError.Code == http.StatusBadRequest {
		_ = c.JSON(http.StatusBadRequest, api.Error{
			Code:    api.InvalidRequest,
			Message: "request parameters are invalid",
		})
		return
	}
	echo.DefaultHTTPErrorHandler(false)(c, err)
}

// decodeJSONBody decodes exactly one JSON object into T, rejecting unknown fields,
// trailing content, and (via the http.MaxBytesReader already installed on the
// request body) bodies over the 16 KiB limit. Shared by analyze and coverage.
func decodeJSONBody[T any](request *http.Request) (T, error) {
	var input T
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return input, errRequestTooLarge
		}
		return input, errors.New("request body must be valid JSON")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return input, errRequestTooLarge
		}
		return input, errors.New("request body must contain one JSON object")
	}
	return input, nil
}

// coverage implements the TB2 offensive coverage endpoint (ADR-0016).
//
// Validation order (ADR-0016 §4): header (400, via requireRequestContext) → body
// (400/413) → either read model absent (503) → unknown pokemonId (422) → unknown
// moveId (422) → 200. Other internal failures answer 500 with a fixed message.
func coverage(c *echo.Context, deps Dependencies) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxAnalyzeBodyBytes)
	request, err := decodeJSONBody[api.CoverageRequest](c.Request())
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
	if len(request.Members) < 1 || len(request.Members) > 6 {
		return c.JSON(http.StatusBadRequest, api.Error{
			Code:    api.InvalidRequest,
			Message: "members must contain between one and six entries",
		})
	}
	for _, member := range request.Members {
		if !pokemonIDPattern.MatchString(member.PokemonId) {
			return c.JSON(http.StatusBadRequest, api.Error{
				Code:    api.InvalidRequest,
				Message: "pokemonId must use the NNNN-NNN format",
			})
		}
		// ADR-0016 §6.1: a missing or null moveIds decodes to a nil slice (an
		// explicit [] decodes to a non-nil, zero-length slice), so this also
		// rejects the field's omission or an explicit JSON null.
		if member.MoveIds == nil {
			return c.JSON(http.StatusBadRequest, api.Error{
				Code:    api.InvalidRequest,
				Message: "moveIds is required",
			})
		}
		if len(member.MoveIds) > balance.MaxMovesPerMember {
			return c.JSON(http.StatusBadRequest, api.Error{
				Code:    api.InvalidRequest,
				Message: "a member must have at most four moves",
			})
		}
		seen := make(map[string]struct{}, len(member.MoveIds))
		for _, moveID := range member.MoveIds {
			if len(moveID) > maxMoveIDLength || !moveIDPattern.MatchString(moveID) {
				return c.JSON(http.StatusBadRequest, api.Error{
					Code:    api.InvalidRequest,
					Message: "moveId must match ^[a-z0-9]+(-[a-z0-9]+)*$ and be at most 40 characters",
				})
			}
			if _, ok := seen[moveID]; ok {
				return c.JSON(http.StatusBadRequest, api.Error{
					Code:    api.InvalidRequest,
					Message: "moveIds must be distinct within a member",
				})
			}
			seen[moveID] = struct{}{}
		}
	}

	if deps.PokemonTypes == nil || deps.Moves == nil {
		return c.JSON(http.StatusServiceUnavailable, api.Error{
			Code:    api.MasterUnavailable,
			Message: "the pokemon type or move read model is not configured",
		})
	}

	pokemonIDs := make([]string, len(request.Members))
	for i, member := range request.Members {
		pokemonIDs[i] = member.PokemonId
	}
	if _, err := balance.ResolveMembers(deps.PokemonTypes, pokemonIDs); err != nil {
		var unknown *balance.UnknownPokemonError
		if errors.As(err, &unknown) {
			return c.JSON(http.StatusUnprocessableEntity, api.Error{
				Code:    api.UnknownPokemon,
				Message: "unknown pokemonId: " + unknown.PokemonID,
			})
		}
		return internalError(c, err)
	}

	members := make([]balance.CoverageMember, len(request.Members))
	for i, member := range request.Members {
		moves, err := balance.ResolveMoves(deps.Moves, member.MoveIds)
		if err != nil {
			var unknown *balance.UnknownMoveError
			if errors.As(err, &unknown) {
				return c.JSON(http.StatusUnprocessableEntity, api.Error{
					Code:    api.UnknownMove,
					Message: "unknown moveId: " + unknown.MoveID,
				})
			}
			return internalError(c, err)
		}
		members[i] = balance.CoverageMember{PokemonID: member.PokemonId, Moves: moves}
	}

	analysis, err := balance.AnalyzeCoverage(deps.TypeChart, members)
	if err != nil {
		return internalError(c, err)
	}

	return c.JSON(http.StatusOK, toCoverageResponse(analysis))
}

func toCoverageResponse(analysis balance.CoverageAnalysis) api.CoverageResponse {
	members := make([]api.MemberCoverage, len(analysis.Members))
	for i, member := range analysis.Members {
		members[i] = toMemberCoverage(member)
	}
	team := make([]api.TeamCoverageEntry, len(analysis.TeamCoverage))
	for i, entry := range analysis.TeamCoverage {
		team[i] = api.TeamCoverageEntry{
			DefenseType:           api.TypeId(entry.DefenseType),
			BestMultiplier:        toCoverageMultiplier(entry.BestMultiplier),
			EffectiveMembers:      entry.EffectiveMembers,
			SuperEffectiveMembers: entry.SuperEffectiveMembers,
		}
	}
	return api.CoverageResponse{Members: members, TeamCoverage: team}
}

func toMemberCoverage(member balance.MemberCoverage) api.MemberCoverage {
	moveIDs := make([]api.MoveId, len(member.MoveIDs))
	copy(moveIDs, member.MoveIDs)
	attackTypes := make([]api.TypeId, len(member.AttackTypes))
	for i, t := range member.AttackTypes {
		attackTypes[i] = api.TypeId(t)
	}
	coverage := make([]api.DefenseCoverageEntry, len(member.Coverage))
	for i, entry := range member.Coverage {
		coverage[i] = api.DefenseCoverageEntry{
			DefenseType:    api.TypeId(entry.DefenseType),
			BestMultiplier: toCoverageMultiplier(entry.BestMultiplier),
			Effective:      entry.Effective,
			SuperEffective: entry.SuperEffective,
		}
	}
	return api.MemberCoverage{
		PokemonId:   member.PokemonID,
		MoveIds:     moveIDs,
		AttackTypes: attackTypes,
		Coverage:    coverage,
	}
}

func toCoverageMultiplier(m *balance.Multiplier) *api.CoverageMultiplier {
	if m == nil {
		return nil
	}
	v := api.CoverageMultiplier(m.String())
	return &v
}
