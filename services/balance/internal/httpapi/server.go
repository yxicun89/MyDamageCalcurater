// Package httpapi contains the balance service HTTP adapter.
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"

	"example.com/pokecalc/services/balance/internal/api"
	"example.com/pokecalc/services/balance/internal/balance"
	"github.com/labstack/echo/v4"
)

const (
	deviceIDHeader      = "X-Device-Id"
	sessionIDHeader     = "X-Session-Id"
	maxAnalyzeBodyBytes = 16 * 1024
)

var pokemonIDPattern = regexp.MustCompile(`^\d{4}-\d{3}$`)

var errRequestTooLarge = errors.New("request body exceeds 16 KiB")

// Dependencies are the replaceable master-data boundaries of the HTTP adapter.
// PokemonTypes may be nil: the service still starts, health stays 200, and
// analyze answers 503 master_unavailable (ADR-0014 §2).
type Dependencies struct {
	TypeChart    balance.TypeChartProvider
	PokemonTypes balance.PokemonTypeProvider
}

// New returns the HTTP handler.
// TB1: deps are accepted but not yet used; the implementer wires them into analyze.
func New(deps Dependencies) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = writeHTTPError
	api.RegisterHandlersWithOptions(e, handler{}, api.RegisterHandlersOptions{
		OperationMiddlewares: map[string][]echo.MiddlewareFunc{
			"analyzeTeamBalance": {requireRequestContext},
		},
	})
	return e
}

type handler struct{}

var _ api.ServerInterface = handler{}

func (handler) Health(c echo.Context) error {
	return health(c)
}

func (handler) PublicHealth(c echo.Context) error {
	return health(c)
}

func (handler) AnalyzeTeamBalance(c echo.Context, _ api.AnalyzeTeamBalanceParams) error {
	return analyze(c)
}

func health(c echo.Context) error {
	return c.JSON(http.StatusOK, api.Health{Status: api.Ok})
}

func analyze(c echo.Context) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxAnalyzeBodyBytes)
	request, err := decodeAnalyzeRequest(c.Request())
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
	}
	return c.NoContent(http.StatusNotImplemented) // TB1: not implemented
}

func requireRequestContext(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
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

func writeHTTPError(err error, c echo.Context) {
	if c.Response().Committed {
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
	c.Echo().DefaultHTTPErrorHandler(err, c)
}

func decodeAnalyzeRequest(request *http.Request) (api.AnalyzeRequest, error) {
	var input api.AnalyzeRequest
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return api.AnalyzeRequest{}, errRequestTooLarge
		}
		return api.AnalyzeRequest{}, errors.New("request body must be valid JSON")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return api.AnalyzeRequest{}, errRequestTooLarge
		}
		return api.AnalyzeRequest{}, errors.New("request body must contain one JSON object")
	}
	return input, nil
}
