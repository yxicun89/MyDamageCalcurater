package httpapi

// record-svc の担当(calc-svc の担当外)の操作。生成物 api.ServerInterface(単一インターフェース。
// oapi-codegen の skip-prune。ADR-0209 §10-1)を満たすためだけに実装し、ルートには登録しない
// (readiness.go の GetMasterExport と同じ扱い)。呼ばれたら 404 not_found。
//
// P5-3 で api/openapi.yaml に record の契約が入ったことに伴う機械的な追従で、
// calc-svc の振る舞いは変えない(CLAUDE.md 絶対ルール5: 計算 API は保存に依存しない)。

import (
	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/internal/api"
)

// DeleteRecordDeviceData は DELETE /api/record/device-data(record-svc の担当)。
func (s *Server) DeleteRecordDeviceData(ctx *echo.Context, params api.DeleteRecordDeviceDataParams) error {
	return notFoundForPokedex()
}

// ListFrequentOpponents は GET /api/record/frequent-opponents(record-svc の担当)。
func (s *Server) ListFrequentOpponents(ctx *echo.Context, params api.ListFrequentOpponentsParams) error {
	return notFoundForPokedex()
}
