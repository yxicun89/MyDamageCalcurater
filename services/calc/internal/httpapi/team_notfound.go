package httpapi

// team-svc の担当(calc-svc の担当外)の操作。生成物 api.ServerInterface(単一インターフェース。
// oapi-codegen の skip-prune。ADR-0209 §10-1)を満たすためだけに実装し、ルートには登録しない
// (record_notfound.go と同じ扱い)。呼ばれたら 404 not_found。
//
// P5-4 で api/openapi.yaml に team の契約(ADR-0213)が入ったことに伴う機械的な追従で、
// calc-svc の振る舞いは変えない(CLAUDE.md 絶対ルール5: 計算 API は保存に依存しない)。

import (
	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/internal/api"
)

// DeleteTeamDeviceData は DELETE /api/team/device-data(team-svc の担当)。
func (s *Server) DeleteTeamDeviceData(ctx *echo.Context, params api.DeleteTeamDeviceDataParams) error {
	return notFoundForPokedex()
}

// ListTeams は GET /api/team/teams(team-svc の担当)。
func (s *Server) ListTeams(ctx *echo.Context, params api.ListTeamsParams) error {
	return notFoundForPokedex()
}

// CreateTeam は POST /api/team/teams(team-svc の担当)。
func (s *Server) CreateTeam(ctx *echo.Context, params api.CreateTeamParams) error {
	return notFoundForPokedex()
}

// GetTeam は GET /api/team/teams/{teamId}(team-svc の担当)。
func (s *Server) GetTeam(ctx *echo.Context, teamId api.TeamId, params api.GetTeamParams) error {
	return notFoundForPokedex()
}

// UpdateTeam は PUT /api/team/teams/{teamId}(team-svc の担当)。
func (s *Server) UpdateTeam(ctx *echo.Context, teamId api.TeamId, params api.UpdateTeamParams) error {
	return notFoundForPokedex()
}

// DeleteTeam は DELETE /api/team/teams/{teamId}(team-svc の担当)。
func (s *Server) DeleteTeam(ctx *echo.Context, teamId api.TeamId, params api.DeleteTeamParams) error {
	return notFoundForPokedex()
}
