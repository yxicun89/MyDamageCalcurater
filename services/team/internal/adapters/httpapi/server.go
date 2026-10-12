// Package httpapi は team-svc の HTTP 境界(ADR-0209 §5・§6、ADR-0213)。生成物 api.ServerInterface を
// 実装するが、実際にルートへ登録するのは team の6操作(生成ラッパ経由)だけで、他サービスの操作
// (calc・pokedex・record・internal)は api.ServerInterface を満たすためのスタブ(stubs.go)として
// 404 を返すだけ。
//
// 端末 ID はヘッダ(X-Device-Id)からだけ受け取る(ADR-0209 §2・§6)。ボディ・クエリに
// deviceId/device_id が来たら 400 unknown_field で拒否し、ヘッダの値を上書きさせない。
//
// team-svc はマスタ(pokedex)を引かない(CLAUDE.md 絶対ルール4)。入力の検証(「マスタを引かなくても
// 判断できること」だけ。ADR-0213 §3)は domain、操作の手順(TouchDevice → 本体)は app にあり、ここは
// 契約の HTTP の形(ヘッダ・クエリの端末 ID の拒否・厳格デコード)と、契約の型・エラーへの写しだけを持つ
// (ADR-0811・ADR-0255)。
package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/internal/httpmetrics"
	"example.com/pokecalc/services/team/internal/app"
	"example.com/pokecalc/services/team/internal/domain"
)

// Server は api.ServerInterface を実装する。team の6操作だけを本実装し、他は stubs.go に置く。
type Server struct {
	svc *app.Service
}

var _ api.ServerInterface = (*Server)(nil)

// NewServer は Store を使う Server を作る(手順は app.Service。時計は time.Now)。
func NewServer(st app.Store) *Server {
	return &Server{svc: app.New(st, time.Now)}
}

// NewHandler は team-svc の HTTP ハンドラ全体を組み立てる。
func NewHandler(st app.Store) http.Handler {
	e := echo.New()
	e.HTTPErrorHandler = httpErrorHandler
	m := httpmetrics.New()
	e.Use(m.Middleware())
	e.Use(recoverMiddleware)
	e.GET(httpmetrics.Path, m.Handler())

	srv := NewServer(st)
	registerTeamRoutes(e, srv)
	e.GET("/healthz", healthzHandler)
	e.GET("/readyz", readyzHandler(srv.svc))
	return e
}

// registerTeamRoutes は team の6操作を、生成ラッパ(api.ServerInterfaceWrapper。必須ヘッダの
// 有無を検証してから Server を呼ぶ)経由で登録する。calc / pokedex / record / internal はここに
// 含めない(stubs.go が担当外として 404 を返す)。
func registerTeamRoutes(e *echo.Echo, srv *Server) {
	wrapper := api.ServerInterfaceWrapper{Handler: srv}
	e.GET("/api/team/teams", wrapper.ListTeams)
	e.POST("/api/team/teams", wrapper.CreateTeam)
	e.GET("/api/team/teams/:teamId", wrapper.GetTeam)
	e.PUT("/api/team/teams/:teamId", wrapper.UpdateTeam)
	e.DELETE("/api/team/teams/:teamId", wrapper.DeleteTeam)
	e.DELETE("/api/team/device-data", wrapper.DeleteTeamDeviceData)
}

func healthzHandler(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// readyzHandler は DB に届けば 200、届かなければ 503 store_unavailable(ADR-0209 §5.3)。
// 確かめ方(Ping か、無ければ読み取り専用の ListTeams)は app.Service.Ready。
func readyzHandler(svc *app.Service) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if err := svc.Ready(c.Request().Context()); err != nil {
			return errFromStore("", err)
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
}

// recoverMiddleware は panic を回復し、500 internal の httpError にする。
func recoverMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = newError(api.Internal, "%s", messageInternal)
			}
		}()
		return next(c)
	}
}

// httpErrorHandler は echo に渡ったエラー(Server が返した httpError・生成ラッパのヘッダ/クエリ検証・
// echo の既定 404/405 を含む)をすべて Error 形式に揃える。
func httpErrorHandler(c *echo.Context, err error) {
	if r, uerr := echo.UnwrapResponse(c.Response()); uerr == nil && r.Committed {
		return
	}
	status, body := errorBodyFor(err)
	_ = c.JSON(status, body)
}

// errFromApp は app が返した失敗を httpError に写す。検証の失敗(domain.Error)は code のまま 400、
// 持っていない構築(ErrNotFound)は 404 not_found、構築の上限(ErrTeamLimitReached)は 400 invalid_input、
// それ以外は errFromStore(503 store_unavailable / 500 internal)。
func errFromApp(deviceID string, err error) error {
	var de *domain.Error
	switch {
	case errors.As(err, &de):
		return errFromDomain(de)
	case errors.Is(err, app.ErrNotFound):
		return newError(api.NotFound, "この端末の構築に無い")
	case errors.Is(err, app.ErrTeamLimitReached):
		return newError(api.InvalidInput, "1端末が持てる構築の上限(%d件)に達している", app.MaxTeamsPerDevice)
	default:
		return errFromStore(deviceID, err)
	}
}

// ListTeams は GET /api/team/teams。
func (s *Server) ListTeams(ctx *echo.Context, params api.ListTeamsParams) error {
	if err := checkHeaders(params.XDeviceId, params.XSessionId); err != nil {
		return err
	}
	if err := checkNoDeviceIDInQuery(ctx.QueryParams()); err != nil {
		return err
	}
	if err := decodeNoBody(ctx.Request().Body); err != nil {
		return err
	}

	rows, err := s.svc.ListTeams(ctx.Request().Context(), params.XDeviceId)
	if err != nil {
		return errFromApp(params.XDeviceId, err)
	}
	return ctx.JSON(http.StatusOK, teamsFrom(rows))
}

// CreateTeam は POST /api/team/teams。
func (s *Server) CreateTeam(ctx *echo.Context, params api.CreateTeamParams) error {
	if err := checkHeaders(params.XDeviceId, params.XSessionId); err != nil {
		return err
	}
	var req api.TeamInput
	if err := decodeStrict(limitedBody(ctx.Request().Body), &req); err != nil {
		return err
	}
	if err := checkNoDeviceIDInQuery(ctx.QueryParams()); err != nil {
		return err
	}

	created, err := s.svc.CreateTeam(ctx.Request().Context(), params.XDeviceId, teamInputFrom(req))
	if err != nil {
		return errFromApp(params.XDeviceId, err)
	}
	return ctx.JSON(http.StatusCreated, teamFrom(created))
}

// GetTeam は GET /api/team/teams/{teamId}。
func (s *Server) GetTeam(ctx *echo.Context, teamId api.TeamId, params api.GetTeamParams) error {
	if err := checkHeaders(params.XDeviceId, params.XSessionId); err != nil {
		return err
	}
	if err := checkNoDeviceIDInQuery(ctx.QueryParams()); err != nil {
		return err
	}
	if err := decodeNoBody(ctx.Request().Body); err != nil {
		return err
	}

	t, err := s.svc.GetTeam(ctx.Request().Context(), params.XDeviceId, teamId)
	if err != nil {
		return errFromApp(params.XDeviceId, err)
	}
	return ctx.JSON(http.StatusOK, teamFrom(t))
}

// UpdateTeam は PUT /api/team/teams/{teamId}(全体置換。部分更新はしない)。
func (s *Server) UpdateTeam(ctx *echo.Context, teamId api.TeamId, params api.UpdateTeamParams) error {
	if err := checkHeaders(params.XDeviceId, params.XSessionId); err != nil {
		return err
	}
	var req api.TeamInput
	if err := decodeStrict(limitedBody(ctx.Request().Body), &req); err != nil {
		return err
	}
	if err := checkNoDeviceIDInQuery(ctx.QueryParams()); err != nil {
		return err
	}

	updated, err := s.svc.UpdateTeam(ctx.Request().Context(), params.XDeviceId, teamId, teamInputFrom(req))
	if err != nil {
		return errFromApp(params.XDeviceId, err)
	}
	return ctx.JSON(http.StatusOK, teamFrom(updated))
}

// DeleteTeam は DELETE /api/team/teams/{teamId}。
func (s *Server) DeleteTeam(ctx *echo.Context, teamId api.TeamId, params api.DeleteTeamParams) error {
	if err := checkHeaders(params.XDeviceId, params.XSessionId); err != nil {
		return err
	}
	if err := checkNoDeviceIDInQuery(ctx.QueryParams()); err != nil {
		return err
	}
	if err := decodeNoBody(ctx.Request().Body); err != nil {
		return err
	}

	if err := s.svc.DeleteTeam(ctx.Request().Context(), params.XDeviceId, teamId); err != nil {
		return errFromApp(params.XDeviceId, err)
	}
	return ctx.NoContent(http.StatusNoContent)
}

// DeleteTeamDeviceData は DELETE /api/team/device-data(ADR-0209 §5)。
func (s *Server) DeleteTeamDeviceData(ctx *echo.Context, params api.DeleteTeamDeviceDataParams) error {
	if err := checkHeaders(params.XDeviceId, params.XSessionId); err != nil {
		return err
	}
	if err := checkNoDeviceIDInQuery(ctx.QueryParams()); err != nil {
		return err
	}
	if err := decodeNoBody(ctx.Request().Body); err != nil {
		return err
	}

	res, err := s.svc.PurgeDevice(ctx.Request().Context(), params.XDeviceId)
	if err != nil {
		return errFromApp(params.XDeviceId, err)
	}
	return ctx.JSON(http.StatusOK, deletionResultFrom(res))
}

// --- 契約の型との相互変換 ----------------------------------------------------

// teamInputFrom は契約の TeamInput を domain の受け口に写す(値はそのまま。検証は domain)。
func teamInputFrom(in api.TeamInput) domain.TeamInput {
	out := domain.TeamInput{Name: in.Name}
	if in.Members != nil {
		ms := make([]domain.MemberInput, 0, len(*in.Members))
		for _, m := range *in.Members {
			var tera *string
			if m.TeraType != nil {
				v := string(*m.TeraType)
				tera = &v
			}
			ms = append(ms, domain.MemberInput{
				SpeciesKey: string(m.SpeciesKey),
				Nickname:   m.Nickname,
				MoveIDs:    m.MoveIds,
				ItemID:     m.ItemId,
				AbilityID:  m.AbilityId,
				NatureID:   m.NatureId,
				SP: domain.StatBlock{
					HP: m.Sp.Hp, Atk: m.Sp.Atk, Def: m.Sp.Def, Spa: m.Sp.Spa, Spd: m.Sp.Spd, Spe: m.Sp.Spe,
				},
				TeraType: tera,
			})
		}
		out.Members = &ms
	}
	return out
}

// teamsFrom は app.Team のスライスを契約の型に写す(空なら空配列にする。nil を返して JSON が
// `null` になるのを避ける。契約は「記録が無ければ空配列」)。
func teamsFrom(rows []app.Team) []api.Team {
	out := make([]api.Team, 0, len(rows))
	for _, t := range rows {
		out = append(out, teamFrom(t))
	}
	return out
}

func teamFrom(t app.Team) api.Team {
	return api.Team{
		Id:        t.ID,
		Name:      t.Name,
		Members:   membersFrom(t.Members),
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

func membersFrom(ms []app.Member) []api.TeamMember {
	out := make([]api.TeamMember, 0, len(ms))
	for _, m := range ms {
		var moveIDs *[]string
		if len(m.MoveIDs) > 0 {
			ids := append([]string(nil), m.MoveIDs...)
			moveIDs = &ids
		}
		var tera *api.PokeType
		if m.TeraType != nil {
			pt := api.PokeType(*m.TeraType)
			tera = &pt
		}
		out = append(out, api.TeamMember{
			SpeciesKey: api.SpeciesKey(m.SpeciesKey),
			Nickname:   m.Nickname,
			MoveIds:    moveIDs,
			ItemId:     m.ItemID,
			AbilityId:  m.AbilityID,
			NatureId:   m.NatureID,
			Sp: api.StatBlock{
				Hp: m.SP.HP, Atk: m.SP.Atk, Def: m.SP.Def, Spa: m.SP.Spa, Spd: m.SP.Spd, Spe: m.SP.Spe,
			},
			TeraType: tera,
		})
	}
	return out
}

// deletionResultFrom は app.PurgeResult を契約の TeamDeletionResult に写す。
func deletionResultFrom(res app.PurgeResult) api.TeamDeletionResult {
	status := api.Completed
	if res.Remaining {
		status = api.Partial
	}
	return api.TeamDeletionResult{
		Status:   status,
		PurgedAt: res.PurgedAt,
		Deleted: struct {
			TeamMembers int `json:"teamMembers"`
			Teams       int `json:"teams"`
		}{
			TeamMembers: res.Deleted.TeamMembers,
			Teams:       res.Deleted.Teams,
		},
	}
}
