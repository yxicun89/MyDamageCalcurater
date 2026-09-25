// Package httpapi は speed サービスの Echo の HTTP アダプタ(ADR-0600 §5)。
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"sort"

	"example.com/pokecalc/services/speed/internal/api"
	"example.com/pokecalc/services/speed/internal/httpmetrics"
	"example.com/pokecalc/services/speed/internal/speed"
	"github.com/labstack/echo/v5"
)

const (
	deviceIDHeader  = "X-Device-Id"
	sessionIDHeader = "X-Session-Id"

	// maxPositionBodyBytes: PositionRequest の妥当な body はスカラーのフィールドだけで、
	// 最長でも pokemonId(8文字)+ 他のフィールドで 200 バイトに満たない。4 KiB は十分な余裕を
	// 持たせつつ、際限なく大きい body を読み込まないための上限(balance の decodeJSONBody と同じ考え方)。
	maxPositionBodyBytes = 4 * 1024
)

// pokemonIDPattern は PositionRequest.pokemonId の形式(openapi.yaml の PokemonId スキーマと同じ)。
// 生成コードは body の中身を検証しないので、httpapi 側で 1 か所だけ持つ(balance と同じ形)。
var pokemonIDPattern = regexp.MustCompile(`^\d{4}-\d{3}$`)

// Dependencies は HTTP アダプタの差し替え可能な境界。
// Pokemon が nil でも起動はし、/healthz は 200、ポケモンを使う API は 503 master_unavailable(ADR-0600 §4)。
type Dependencies struct {
	Pokemon speed.PokemonProvider
}

// New は HTTP ハンドラを返す。
func New(deps Dependencies) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = writeHTTPError
	m := httpmetrics.New()
	e.Use(m.Middleware())
	e.GET(httpmetrics.Path, m.Handler())
	api.RegisterHandlersWithOptions(e, handler{deps: deps}, api.RegisterHandlersOptions{
		OperationMiddlewares: map[string][]echo.MiddlewareFunc{
			"listPokemon":      {requireRequestContext},
			"getSpeedTable":    {requireRequestContext},
			"getSpeedPosition": {requireRequestContext},
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

func (h handler) ListPokemon(c *echo.Context, _ api.ListPokemonParams) error {
	return listPokemon(c, h.deps)
}

// GetSpeedTable implements GET /api/speed/v1/table (ADR-0601 §5): header
// (400 missing_header/invalid_header) → query (400 invalid_request) → read model absent
// (503) → provider/calc error (500, fixed message) → 200. Header の検査は
// requireRequestContext ミドルウェア(New で登録)がクエリより先に行う(ADR-0606)。
func (h handler) GetSpeedTable(c *echo.Context, params api.GetSpeedTableParams) error {
	return getSpeedTable(c, h.deps, params)
}

// GetSpeedPosition implements POST /api/speed/v1/position (ADR-0602 §4): header
// (400 missing_header/invalid_header, ADR-0606) → body (400 invalid_request) → read model
// absent (503) → unknown pokemonId (422) → 200.
func (h handler) GetSpeedPosition(c *echo.Context, _ api.GetSpeedPositionParams) error {
	return getSpeedPosition(c, h.deps)
}

// getSpeedTable implements the body of GetSpeedTable. deps is passed explicitly, matching
// the listPokemon convention in this file.
func getSpeedTable(c *echo.Context, deps Dependencies, params api.GetSpeedTableParams) error {
	presets, err := speed.NormalizePresets(requestedPresetIDs(params))
	if err != nil {
		return c.JSON(http.StatusBadRequest, api.Error{
			Code:    api.InvalidRequest,
			Message: "presets is invalid",
		})
	}

	if deps.Pokemon == nil {
		return c.JSON(http.StatusServiceUnavailable, api.Error{
			Code:    api.MasterUnavailable,
			Message: "the pokemon read model is not configured",
		})
	}
	roster, err := deps.Pokemon.Roster()
	if err != nil {
		return internalError(c, err)
	}

	table, err := speed.BuildTable(roster, presets)
	if err != nil {
		return internalError(c, err)
	}

	return c.JSON(http.StatusOK, toTableResponse(table))
}

// requestedPresetIDs converts the query parameter to the core's PresetID, defaulting to
// every preset in ADR-0601 §2 order when presets is omitted.
func requestedPresetIDs(params api.GetSpeedTableParams) []speed.PresetID {
	if params.Presets == nil {
		defs := speed.Presets()
		ids := make([]speed.PresetID, len(defs))
		for i, p := range defs {
			ids[i] = p.ID
		}
		return ids
	}
	ids := make([]speed.PresetID, len(*params.Presets))
	for i, id := range *params.Presets {
		ids[i] = speed.PresetID(id)
	}
	return ids
}

// toTableResponse は speed.Table を生成型 api.TableResponse に詰め替える。Types はコアの
// slice を共有しないよう複製する(listPokemon と同じ)。
func toTableResponse(table speed.Table) api.TableResponse {
	presets := make([]api.PresetId, len(table.Presets))
	for i, id := range table.Presets {
		presets[i] = api.PresetId(id)
	}

	tiers := make([]api.SpeedTier, len(table.Tiers))
	for i, tier := range table.Tiers {
		entries := make([]api.SpeedTableEntry, len(tier.Entries))
		for j, e := range tier.Entries {
			types := make([]string, len(e.Pokemon.Types))
			copy(types, e.Pokemon.Types)
			entries[j] = api.SpeedTableEntry{
				PokemonId: e.Pokemon.PokemonID,
				NameJa:    e.Pokemon.NameJa,
				Types:     types,
				BaseSpeed: e.Pokemon.BaseSpeed,
				Preset:    api.PresetId(e.Preset),
			}
		}
		tiers[i] = api.SpeedTier{Speed: tier.Speed, Entries: entries}
	}

	return api.TableResponse{
		RegulationId: table.RegulationID,
		Presets:      presets,
		Tiers:        tiers,
	}
}

func health(c *echo.Context) error {
	return c.JSON(http.StatusOK, api.Health{Status: api.Ok})
}

// listPokemon implements GET /api/speed/v1/pokemon (ADR-0600 §5): read model absent (503) →
// provider error (500, fixed message) → 200 sorted by pokemonId ascending.
func listPokemon(c *echo.Context, deps Dependencies) error {
	if deps.Pokemon == nil {
		return c.JSON(http.StatusServiceUnavailable, api.Error{
			Code:    api.MasterUnavailable,
			Message: "the pokemon read model is not configured",
		})
	}
	roster, err := deps.Pokemon.Roster()
	if err != nil {
		return internalError(c, err)
	}

	pokemon := make([]api.SpeedPokemon, len(roster.Pokemon))
	for i, p := range roster.Pokemon {
		types := make([]string, len(p.Types))
		copy(types, p.Types)
		pokemon[i] = api.SpeedPokemon{
			PokemonId: p.PokemonID,
			NameJa:    p.NameJa,
			Types:     types,
			BaseSpeed: p.BaseSpeed,
		}
	}
	sort.Slice(pokemon, func(i, j int) bool { return pokemon[i].PokemonId < pokemon[j].PokemonId })

	return c.JSON(http.StatusOK, api.PokemonListResponse{
		RegulationId: roster.RegulationID,
		Pokemon:      pokemon,
	})
}

// internalError answers 500 internal_error with a fixed message (ADR-0600 §5): an
// unexpected provider failure never leaks internal detail to the client. The error
// is still logged for operators.
func internalError(c *echo.Context, err error) error {
	slog.Error("speed internal error", "path", c.Path(), "error", err)
	return c.JSON(http.StatusInternalServerError, api.Error{
		Code:    api.InternalError,
		Message: "internal error",
	})
}

func requireRequestContext(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if apiErr := checkAPIHeaders(c.Request().Header); apiErr != nil {
			return c.JSON(http.StatusBadRequest, *apiErr)
		}
		return next(c)
	}
}

// writeHTTPError normalizes any 400 from the generated parameter binding (e.g. a
// duplicate query parameter) to the same Error{code: invalid_request} shape as the
// rest of the API (balance と同じ)。ヘッダーの検証は requireRequestContext が先に行うため、
// ここに落ちてくる 400 はヘッダー以外の理由(クエリの重複など)に限られる。
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
