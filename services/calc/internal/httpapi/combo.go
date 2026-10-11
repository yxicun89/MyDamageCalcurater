package httpapi

// POST /api/calc/combo(連続技。ADR-0252)。

import (
	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/internal/api"
)

// CalcCombo は POST /api/calc/combo。
func (s *Server) CalcCombo(ctx *echo.Context, params api.CalcComboParams) error {
	return newError(api.NotFound, "未実装")
}
