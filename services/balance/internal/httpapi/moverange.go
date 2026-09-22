package httpapi

import (
	"net/http"

	"example.com/pokecalc/services/balance/internal/api"
	"github.com/labstack/echo/v5"
)

// AnalyzeMoveRange is the TB6 move-range endpoint (ADR-0404).
func (h handler) AnalyzeMoveRange(c *echo.Context, _ api.AnalyzeMoveRangeParams) error {
	return moveRange(c, h.deps)
}

// moveRange is the spec-writer stub: it answers a zero-value 200 so the package compiles while
// the TB6 contract tests fail. The implementer replaces it with the ADR-0404 §2 behaviour
// (validation order: header 400 → body 400/413 → move read model absent 503 → unknown_move 422
// → pokemon catalog absent 503 → 200; everything else 500 with the fixed message) and registers
// requireRequestContext for "analyzeMoveRange" in New.
func moveRange(c *echo.Context, _ Dependencies) error {
	return c.JSON(http.StatusOK, api.MoveRangeResponse{})
}
