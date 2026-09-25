package httpapi

// record-svc の担当(pokedex-svc の担当外)の操作。生成物 api.ServerInterface を満たすためだけに
// 実装し、ルートには登録しない(server.go の calc 用スタブと同じ扱い)。呼ばれたら 404 not_found。

import (
	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/internal/api"
)

// DeleteRecordDeviceData は DELETE /api/record/device-data(record-svc の担当)。
func (s *Server) DeleteRecordDeviceData(ctx *echo.Context, params api.DeleteRecordDeviceDataParams) error {
	return notFoundForCalc()
}

// ListFrequentOpponents は GET /api/record/frequent-opponents(record-svc の担当)。
func (s *Server) ListFrequentOpponents(ctx *echo.Context, params api.ListFrequentOpponentsParams) error {
	return notFoundForCalc()
}
