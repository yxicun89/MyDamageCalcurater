package httpapi

// record-svc の担当外の操作(calc-svc・pokedex-svc・team-svc の担当)。生成物 api.ServerInterface(単一
// インターフェース。oapi-codegen の skip-prune。ADR-0209 §10-1)を満たすためだけに実装し、
// ルートには登録しない(NewHandler・registerRecordRoutes 参照)。呼ばれたら 404 not_found になるが、
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

// --- team-svc の担当(P5-4 で契約に入った。ADR-0213)---------------------------

func (s *Server) DeleteTeamDeviceData(ctx *echo.Context, params api.DeleteTeamDeviceDataParams) error {
	return notFoundForOtherServices()
}

func (s *Server) ListTeams(ctx *echo.Context, params api.ListTeamsParams) error {
	return notFoundForOtherServices()
}

func (s *Server) CreateTeam(ctx *echo.Context, params api.CreateTeamParams) error {
	return notFoundForOtherServices()
}

func (s *Server) GetTeam(ctx *echo.Context, teamId api.TeamId, params api.GetTeamParams) error {
	return notFoundForOtherServices()
}

func (s *Server) UpdateTeam(ctx *echo.Context, teamId api.TeamId, params api.UpdateTeamParams) error {
	return notFoundForOtherServices()
}

func (s *Server) DeleteTeam(ctx *echo.Context, teamId api.TeamId, params api.DeleteTeamParams) error {
	return notFoundForOtherServices()
}
