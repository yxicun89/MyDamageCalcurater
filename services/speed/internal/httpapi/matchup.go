package httpapi

import (
	"errors"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/speed/internal/api"
)

// I-speed-4 / ADR-0610 のハンドラのスタブ(spec-writer が置いた。生成された api.ServerInterface を満たしてビルドを通すためだけ)。
// implementer は本体を書き、server.go の OperationMiddlewares に getSpeedMatchup / getSpeedReach を
// {guard, requireRequestContext} で足すこと(ヘッダー検証は 3 つの既存 API と同じ)。

var errMatchupNotImplemented = errors.New("not implemented (I-speed-4 stub)")

// GetSpeedMatchup implements POST /api/speed/v1/matchup (ADR-0610 §2). Stub.
func (h handler) GetSpeedMatchup(c *echo.Context, _ api.GetSpeedMatchupParams) error {
	return internalError(c, errMatchupNotImplemented)
}

// GetSpeedReach implements POST /api/speed/v1/reach (ADR-0610 §2). Stub.
func (h handler) GetSpeedReach(c *echo.Context, _ api.GetSpeedReachParams) error {
	return internalError(c, errMatchupNotImplemented)
}
