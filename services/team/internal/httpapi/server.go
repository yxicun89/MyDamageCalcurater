// Package httpapi は team-svc の HTTP 境界(ADR-0209 §5・§6、ADR-0213)。生成物 api.ServerInterface を
// 実装するが、実際にルートへ登録するのは team の6操作(生成ラッパ経由)だけで、他サービスの操作
// (calc・pokedex・record・internal)は api.ServerInterface を満たすためのスタブ(stubs.go)として
// 404 を返すだけ。
//
// 端末 ID はヘッダ(X-Device-Id)からだけ受け取る(ADR-0209 §2・§6)。ボディ・クエリに
// deviceId/device_id が来たら 400 unknown_field で拒否し、ヘッダの値を上書きさせない。
//
// team-svc はマスタ(pokedex)を引かない(CLAUDE.md 絶対ルール4)。検証するのは「マスタを引かなくても
// 判断できること」だけ(構築名・メンバー数・技の数と重複・ニックネームの長さ・SP の範囲と合計。
// ADR-0213 §3)。
package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/team/internal/store"
)

// 構築名・ニックネームの文字数、メンバー・技の件数、SP の範囲と合計(契約の maxItems/maxLength と
// 同じ値。requirements.md §2・ADR-0213 §2・§3)。
const (
	minTeamNameRunes = 1
	maxTeamNameRunes = 50
	maxNicknameRunes = 24
	maxMembers       = store.MaxMembersPerTeam
	maxMoveIDs       = store.MaxMovesPerMember
	maxSPPerStat     = 32
	maxSPTotal       = 66

	// maxSpeciesKeyRunes・maxIDRunes・maxTeraTypeRunes は
	// db/migrations/000004_create_team_members.up.sql の列幅とちょうど対応する上限
	// (critic 指摘 重要2)。ここで弾かないと、超過した入力が INSERT の "Data too long" で失敗し、
	// store の全 SQL エラーを ErrUnavailable に包む wrapUnavailable(tidb.go)を経由して
	// 503 store_unavailable(可用性シグナルの誤発火)になってしまう。マスタに実在するかは見ない
	// (ADR-0213 §3)ので、ここは長さだけを見る。
	maxSpeciesKeyRunes = 16 // species_key VARCHAR(16)
	maxIDRunes         = 64 // item_id / ability_id / nature_id VARCHAR(64)。moveIds の各要素にも適用する
	maxTeraTypeRunes   = 16 // tera_type VARCHAR(16)
)

// Server は api.ServerInterface を実装する。team の6操作だけを本実装し、他は stubs.go に置く。
type Server struct {
	store store.Store
}

var _ api.ServerInterface = (*Server)(nil)

// NewServer は Store を使う Server を作る。
func NewServer(st store.Store) *Server {
	return &Server{store: st}
}

// NewHandler は team-svc の HTTP ハンドラ全体を組み立てる。
func NewHandler(st store.Store) http.Handler {
	e := echo.New()
	e.HTTPErrorHandler = httpErrorHandler
	e.Use(recoverMiddleware)

	registerTeamRoutes(e, NewServer(st))
	e.GET("/healthz", healthzHandler)
	e.GET("/readyz", readyzHandler(st))
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

// pinger は TiDBStore が持つ到達確認(Store インターフェースには含めない。store.go の docstring 参照)。
// fake はこれを実装しないので、テストは ListTeams 経由のフォールバックを通る。
type pinger interface {
	Ping(ctx context.Context) error
}

// readyzHandler は DB に届けば 200、届かなければ 503 store_unavailable(ADR-0209 §5.3)。
func readyzHandler(st store.Store) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if err := checkStoreReady(c.Request().Context(), st); err != nil {
			return err
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
}

// checkStoreReady は st が pinger を実装していればそれを、していなければ読み取り専用の
// ListTeams(空の deviceID。副作用が無い)を1件だけ呼んで到達可否を確かめる。
func checkStoreReady(ctx context.Context, st store.Store) error {
	if p, ok := st.(pinger); ok {
		if err := p.Ping(ctx); err != nil {
			return errFromStore("", err)
		}
		return nil
	}
	if _, err := st.ListTeams(ctx, ""); err != nil {
		return errFromStore("", err)
	}
	return nil
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

// touchAndRun は「端末 ID を含む要求を受けたら devices.last_seen_at を更新する」(ADR-0209 §4)を
// 全操作で徹底したうえで run を呼ぶ。TouchDevice が失敗したら run は呼ばない(どちらも同じ
// store_unavailable になるだけなので、二重に呼んで待たせない)。
func touchAndRun(ctx context.Context, st store.Store, deviceID string, run func() error) error {
	if err := st.TouchDevice(ctx, deviceID, time.Now().UTC()); err != nil {
		return errFromStore(deviceID, err)
	}
	return run()
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

	var out []api.Team
	err := touchAndRun(ctx.Request().Context(), s.store, params.XDeviceId, func() error {
		rows, err := s.store.ListTeams(ctx.Request().Context(), params.XDeviceId)
		if err != nil {
			return errFromStore(params.XDeviceId, err)
		}
		out = teamsFrom(rows)
		return nil
	})
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, out)
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
	name, members, err := validateTeamInput(req)
	if err != nil {
		return err
	}

	var result api.Team
	err = touchAndRun(ctx.Request().Context(), s.store, params.XDeviceId, func() error {
		created, err := s.store.CreateTeam(ctx.Request().Context(), params.XDeviceId,
			store.Team{Name: name, Members: members}, time.Now().UTC())
		if err != nil {
			if errors.Is(err, store.ErrTeamLimitReached) {
				return newError(api.InvalidInput, "1端末が持てる構築の上限(%d件)に達している", store.MaxTeamsPerDevice)
			}
			return errFromStore(params.XDeviceId, err)
		}
		result = teamFrom(created)
		return nil
	})
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusCreated, result)
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

	var result api.Team
	err := touchAndRun(ctx.Request().Context(), s.store, params.XDeviceId, func() error {
		t, err := s.store.GetTeam(ctx.Request().Context(), params.XDeviceId, teamId)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return newError(api.NotFound, "この端末の構築に無い")
			}
			return errFromStore(params.XDeviceId, err)
		}
		result = teamFrom(t)
		return nil
	})
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, result)
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
	name, members, err := validateTeamInput(req)
	if err != nil {
		return err
	}

	var result api.Team
	err = touchAndRun(ctx.Request().Context(), s.store, params.XDeviceId, func() error {
		updated, err := s.store.UpdateTeam(ctx.Request().Context(), params.XDeviceId, teamId,
			store.Team{Name: name, Members: members}, time.Now().UTC())
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return newError(api.NotFound, "この端末の構築に無い")
			}
			return errFromStore(params.XDeviceId, err)
		}
		result = teamFrom(updated)
		return nil
	})
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, result)
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

	err := touchAndRun(ctx.Request().Context(), s.store, params.XDeviceId, func() error {
		if err := s.store.DeleteTeam(ctx.Request().Context(), params.XDeviceId, teamId); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return newError(api.NotFound, "この端末の構築に無い")
			}
			return errFromStore(params.XDeviceId, err)
		}
		return nil
	})
	if err != nil {
		return err
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

	var result api.TeamDeletionResult
	err := touchAndRun(ctx.Request().Context(), s.store, params.XDeviceId, func() error {
		res, err := s.store.PurgeDevice(ctx.Request().Context(), params.XDeviceId, time.Now().UTC())
		if err != nil {
			return errFromStore(params.XDeviceId, err)
		}
		result = deletionResultFrom(res)
		return nil
	})
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, result)
}

// --- 契約の型との相互変換・入力検証 ------------------------------------------

// teamsFrom は store.Team のスライスを契約の型に写す(空なら空配列にする。nil を返して JSON が
// `null` になるのを避ける。契約は「記録が無ければ空配列」)。
func teamsFrom(rows []store.Team) []api.Team {
	out := make([]api.Team, 0, len(rows))
	for _, t := range rows {
		out = append(out, teamFrom(t))
	}
	return out
}

func teamFrom(t store.Team) api.Team {
	return api.Team{
		Id:        t.ID,
		Name:      t.Name,
		Members:   membersFrom(t.Members),
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

func membersFrom(ms []store.Member) []api.TeamMember {
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

// deletionResultFrom は store.PurgeResult を契約の TeamDeletionResult に写す。
func deletionResultFrom(res store.PurgeResult) api.TeamDeletionResult {
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

// validateTeamInput は「マスタを引かなくても判断できる」検証だけを行う(ADR-0213 §3)。
// 通れば store.Member のスライスと正規化した名前(前後の空白を除いたもの)を返す。
func validateTeamInput(in api.TeamInput) (name string, members []store.Member, err error) {
	name = strings.TrimSpace(in.Name)
	if n := utf8.RuneCountInString(name); n < minTeamNameRunes || n > maxTeamNameRunes {
		return "", nil, newError(api.InvalidInput, "構築名は%d〜%d文字であること", minTeamNameRunes, maxTeamNameRunes)
	}

	var raw []api.TeamMember
	if in.Members != nil {
		raw = *in.Members
	}
	if len(raw) > maxMembers {
		return "", nil, newError(api.InvalidInput, "メンバーは最大%d体まで(%d体)", maxMembers, len(raw))
	}

	members = make([]store.Member, 0, len(raw))
	for i, m := range raw {
		sm, err := validateMember(i, m)
		if err != nil {
			return "", nil, err
		}
		members = append(members, sm)
	}
	return name, members, nil
}

func validateMember(index int, m api.TeamMember) (store.Member, error) {
	speciesKey := string(m.SpeciesKey)
	if strings.TrimSpace(speciesKey) == "" {
		return store.Member{}, newError(api.InvalidInput, "メンバー%d: speciesKey が空", index+1)
	}
	if n := utf8.RuneCountInString(speciesKey); n > maxSpeciesKeyRunes {
		return store.Member{}, newError(api.InvalidInput, "メンバー%d: speciesKey は%d文字まで(%d文字)", index+1, maxSpeciesKeyRunes, n)
	}
	natureID := m.NatureId
	if strings.TrimSpace(natureID) == "" {
		return store.Member{}, newError(api.InvalidInput, "メンバー%d: natureId が空", index+1)
	}
	if n := utf8.RuneCountInString(natureID); n > maxIDRunes {
		return store.Member{}, newError(api.InvalidInput, "メンバー%d: natureId は%d文字まで(%d文字)", index+1, maxIDRunes, n)
	}

	var moveIDs []string
	if m.MoveIds != nil {
		moveIDs = *m.MoveIds
	}
	if len(moveIDs) > maxMoveIDs {
		return store.Member{}, newError(api.InvalidInput, "メンバー%d: 技は最大%d個まで(%d個)", index+1, maxMoveIDs, len(moveIDs))
	}
	seen := make(map[string]bool, len(moveIDs))
	for _, id := range moveIDs {
		if seen[id] {
			return store.Member{}, newError(api.InvalidInput, "メンバー%d: 技が重複している(%q)", index+1, id)
		}
		seen[id] = true
		if n := utf8.RuneCountInString(id); n > maxIDRunes {
			return store.Member{}, newError(api.InvalidInput, "メンバー%d: moveIds の要素は%d文字まで(%q)", index+1, maxIDRunes, id)
		}
	}

	var nickname *string
	if m.Nickname != nil && *m.Nickname != "" {
		if n := utf8.RuneCountInString(*m.Nickname); n > maxNicknameRunes {
			return store.Member{}, newError(api.InvalidInput, "メンバー%d: ニックネームは%d文字まで", index+1, maxNicknameRunes)
		}
		nickname = m.Nickname
	}

	if m.ItemId != nil {
		if n := utf8.RuneCountInString(*m.ItemId); n > maxIDRunes {
			return store.Member{}, newError(api.InvalidInput, "メンバー%d: itemId は%d文字まで(%d文字)", index+1, maxIDRunes, n)
		}
	}
	if m.AbilityId != nil {
		if n := utf8.RuneCountInString(*m.AbilityId); n > maxIDRunes {
			return store.Member{}, newError(api.InvalidInput, "メンバー%d: abilityId は%d文字まで(%d文字)", index+1, maxIDRunes, n)
		}
	}

	if err := validateSP(index, m.Sp); err != nil {
		return store.Member{}, err
	}

	var tera *string
	if m.TeraType != nil {
		v := string(*m.TeraType)
		if n := utf8.RuneCountInString(v); n > maxTeraTypeRunes {
			return store.Member{}, newError(api.InvalidInput, "メンバー%d: teraType は%d文字まで(%d文字)", index+1, maxTeraTypeRunes, n)
		}
		tera = &v
	}

	return store.Member{
		SpeciesKey: speciesKey,
		Nickname:   nickname,
		MoveIDs:    moveIDs,
		ItemID:     m.ItemId,
		AbilityID:  m.AbilityId,
		NatureID:   natureID,
		SP: store.StatBlock{
			HP: m.Sp.Hp, Atk: m.Sp.Atk, Def: m.Sp.Def, Spa: m.Sp.Spa, Spd: m.Sp.Spd, Spe: m.Sp.Spe,
		},
		TeraType: tera,
	}, nil
}

// validateSP は Individual.sp と同じ規則(各 0..32、合計 <= 66。requirements.md・ADR-0213 §3)。
func validateSP(index int, sp api.StatBlock) error {
	stats := []struct {
		name string
		val  int
	}{
		{"hp", sp.Hp}, {"atk", sp.Atk}, {"def", sp.Def}, {"spa", sp.Spa}, {"spd", sp.Spd}, {"spe", sp.Spe},
	}
	total := 0
	for _, s := range stats {
		if s.val < 0 || s.val > maxSPPerStat {
			return newError(api.InvalidInput, "メンバー%d: sp.%s は0〜%dの範囲であること(%d)", index+1, s.name, maxSPPerStat, s.val)
		}
		total += s.val
	}
	if total > maxSPTotal {
		return newError(api.InvalidInput, "メンバー%d: sp の合計は%d以下であること(%d)", index+1, maxSPTotal, total)
	}
	return nil
}
