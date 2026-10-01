package httpapi

// team-svc の担当(pokedex-svc の担当外)の操作。生成物 api.ServerInterface を満たすためだけに
// 実装し、ルートには登録しない(record_notfound.go と同じ扱い)。呼ばれたら 404 not_found。

import (
	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/internal/api"
)

// DeleteTeamDeviceData は DELETE /api/team/device-data(team-svc の担当)。
func (s *Server) DeleteTeamDeviceData(ctx *echo.Context, params api.DeleteTeamDeviceDataParams) error {
	return notFoundForCalc()
}

// ListTeams は GET /api/team/teams(team-svc の担当)。
func (s *Server) ListTeams(ctx *echo.Context, params api.ListTeamsParams) error {
	return notFoundForCalc()
}

// CreateTeam は POST /api/team/teams(team-svc の担当)。
func (s *Server) CreateTeam(ctx *echo.Context, params api.CreateTeamParams) error {
	return notFoundForCalc()
}

// GetTeam は GET /api/team/teams/{teamId}(team-svc の担当)。
func (s *Server) GetTeam(ctx *echo.Context, teamId api.TeamId, params api.GetTeamParams) error {
	return notFoundForCalc()
}

// UpdateTeam は PUT /api/team/teams/{teamId}(team-svc の担当)。
func (s *Server) UpdateTeam(ctx *echo.Context, teamId api.TeamId, params api.UpdateTeamParams) error {
	return notFoundForCalc()
}

// DeleteTeam は DELETE /api/team/teams/{teamId}(team-svc の担当)。
func (s *Server) DeleteTeam(ctx *echo.Context, teamId api.TeamId, params api.DeleteTeamParams) error {
	return notFoundForCalc()
}
