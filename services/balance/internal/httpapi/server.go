// Package httpapi contains the balance service HTTP adapter.
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"reflect"
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
// PokemonCatalog is the list view of the same pokemon read model (ADR-0401 §5); only
// recommendations uses it, and answers 503 master_unavailable when it is nil.
type Dependencies struct {
	TypeChart      balance.TypeChartProvider
	PokemonTypes   balance.PokemonTypeProvider
	Moves          balance.MoveProvider
	Abilities      balance.AbilityProvider
	PokemonCatalog balance.PokemonCatalog
}

// normalizeDependencies clears any provider whose interface value wraps a nil pointer (or
// nil map/slice/chan/func) of a concrete type, so it is treated exactly like an omitted
// (nil interface) dependency: the 503/500 checks above see it as absent instead of the
// handler calling straight into a nil receiver's method and panicking. This can happen when
// a caller assembles Dependencies from a variable that was declared but never assigned,
// e.g. `var m *master.PokemonTypeReadModel` passed as PokemonTypes.
func normalizeDependencies(deps Dependencies) Dependencies {
	if isNilProvider(deps.TypeChart) {
		deps.TypeChart = nil
	}
	if isNilProvider(deps.PokemonTypes) {
		deps.PokemonTypes = nil
	}
	if isNilProvider(deps.Moves) {
		deps.Moves = nil
	}
	if isNilProvider(deps.Abilities) {
		deps.Abilities = nil
	}
	if isNilProvider(deps.PokemonCatalog) {
		deps.PokemonCatalog = nil
	}
	return deps
}

// isNilProvider reports whether v is a nil interface, or a non-nil interface holding a
// "typed nil" (a nil pointer/chan/func value). A nil map or slice is a usable value in Go
// (reading it does not panic; e.g. an empty catalog), so it is not treated as unset. A typed nil never equals plain
// nil through the interface (v == nil is false), so reflection is required to detect it.
func isNilProvider(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Chan, reflect.Func, reflect.Interface, reflect.UnsafePointer:
		return rv.IsNil()
	default:
		return false
	}
}

// New returns the HTTP handler.
func New(deps Dependencies) *echo.Echo {
	deps = normalizeDependencies(deps)
	e := echo.New()
	e.HTTPErrorHandler = writeHTTPError
	api.RegisterHandlersWithOptions(e, handler{deps: deps}, api.RegisterHandlersOptions{
		OperationMiddlewares: map[string][]echo.MiddlewareFunc{
			"analyzeTeamBalance":  {requireRequestContext},
			"analyzeTeamCoverage": {requireRequestContext},
			"analyzeTeamThreats":  {requireRequestContext},
			"recommendTeamTypes":  {requireRequestContext},
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

// AnalyzeTeamThreats is the TB4 threat check endpoint (ADR-0400).
func (h handler) AnalyzeTeamThreats(c *echo.Context, _ api.AnalyzeTeamThreatsParams) error {
	return threats(c, h.deps)
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
	if err := validateCount(c, len(request.Members), "members"); err != nil {
		return err
	}
	hasAbilityID := false
	for _, member := range request.Members {
		if err := validatePokemonID(member.PokemonId); err != nil {
			return badRequest(c, err.Error())
		}
		if member.AbilityId != nil {
			if err := validateAbilityIDFormat(*member.AbilityId); err != nil {
				return badRequest(c, err.Error())
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
		return resolveError(c, err)
	}

	for i, member := range request.Members {
		if member.AbilityId == nil {
			continue
		}
		ability, err := balance.ResolveAbility(deps.Abilities, *member.AbilityId)
		if err != nil {
			return resolveError(c, err)
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
	if err := validateCount(c, len(request.Members), "members"); err != nil {
		return err
	}
	for _, member := range request.Members {
		if err := validatePokemonID(member.PokemonId); err != nil {
			return badRequest(c, err.Error())
		}
		if _, err := validateMoveIDs(member.MoveIds); err != nil {
			return badRequest(c, err.Error())
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
		return resolveError(c, err)
	}

	members := make([]balance.CoverageMember, len(request.Members))
	for i, member := range request.Members {
		moves, err := balance.ResolveMoves(deps.Moves, member.MoveIds)
		if err != nil {
			return resolveError(c, err)
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
