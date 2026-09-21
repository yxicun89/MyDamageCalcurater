package httpapi

import (
	"net/http"

	"example.com/pokecalc/services/balance/internal/api"
	"github.com/labstack/echo/v5"
)

// RecommendTeamTypes is the TB5 recommendation endpoint (ADR-0401).
//
// spec-writer が置いたコンパイル用のスタブ。implementer が recommendations_test.go に従って実装する
// (New の OperationMiddlewares への requireRequestContext の登録を含む)。
func (h handler) RecommendTeamTypes(c *echo.Context, _ api.RecommendTeamTypesParams) error {
	// TODO(implementer): ADR-0401 §6 の実装。
	return c.NoContent(http.StatusNotImplemented)
}
