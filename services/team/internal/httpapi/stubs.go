package httpapi

// team-svc の担当外の操作(calc-svc・pokedex-svc・record-svc の担当)。生成物 api.ServerInterface(単一
// インターフェース。oapi-codegen の skip-prune。ADR-0209 §10-1)を満たすためだけに実装し、
// ルートには登録しない(NewHandler・registerTeamRoutes 参照)。呼ばれたら 404 not_found になるが、
// これらのメソッド自体は登録されないルートなので実際には呼ばれない(echo の既定 404 を
// httpErrorHandler が同じ not_found に揃える)。念のため直接呼ばれても同じ応答になるようにしておく。

import (
	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/internal/api"
)

func (s *Server) CalcDamage(ctx *echo.Context, params api.CalcDamageParams) error {
	return notFoundForOtherServices()
}

func (s *Server) CalcBulk(ctx *echo.Context, params api.CalcBulkParams) error {
	return notFoundForOtherServices()
}

func (s *Server) CalcReverse(ctx *echo.Context, params api.CalcReverseParams) error {
	return notFoundForOtherServices()
}

func (s *Server) SearchItems(ctx *echo.Context, params api.SearchItemsParams) error {
	return notFoundForOtherServices()
}

func (s *Server) SearchMoves(ctx *echo.Context, params api.SearchMovesParams) error {
	return notFoundForOtherServices()
}

func (s *Server) GetMovesByIds(ctx *echo.Context, params api.GetMovesByIdsParams) error {
	return notFoundForOtherServices()
}

func (s *Server) GetMove(ctx *echo.Context, key string, params api.GetMoveParams) error {
	return notFoundForOtherServices()
}

func (s *Server) ListNatures(ctx *echo.Context, params api.ListNaturesParams) error {
	return notFoundForOtherServices()
}

func (s *Server) SearchSpecies(ctx *echo.Context, params api.SearchSpeciesParams) error {
	return notFoundForOtherServices()
}

func (s *Server) GetSpecies(ctx *echo.Context, key api.SpeciesKey, params api.GetSpeciesParams) error {
	return notFoundForOtherServices()
}

func (s *Server) GetMasterExport(ctx *echo.Context) error {
	return notFoundForOtherServices()
}

func (s *Server) DeleteRecordDeviceData(ctx *echo.Context, params api.DeleteRecordDeviceDataParams) error {
	return notFoundForOtherServices()
}

func (s *Server) ListFrequentOpponents(ctx *echo.Context, params api.ListFrequentOpponentsParams) error {
	return notFoundForOtherServices()
}
