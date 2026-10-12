package httpapi

// お気に入り(手動ピン留め)の3操作(ADR-0227。P5-3c)の HTTP 境界。内容の検証・正規化は domain
// (domain.NormalizeFavorite。SP・レベルの上限もそこ。ADR-0254 で G6 を解消)、手順は app.Service。
// ここは本文の厳格な読み込み・ID の形式・生成型への写し・HTTP のエラー整形だけを持つ。
// label・snapshot の中身は利用者の自由入力を含むのでログに出さない(ADR-0209 §3)。

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/record/internal/app"
	"example.com/pokecalc/services/record/internal/domain"
)

var favoriteIDFormat = regexp.MustCompile(`^[1-9][0-9]{0,18}$`)

// contractEnums は契約の列挙(生成型の Valid())と省略時の値を domain に渡す口(ADR-0254)。
var contractEnums = domain.Enums{
	Status:      func(v string) bool { return api.StatusCondition(v).Valid() },
	Type:        func(v string) bool { return api.PokeType(v).Valid() },
	Format:      func(v string) bool { return api.Format(v).Valid() },
	Weather:     func(v string) bool { return api.Weather(v).Valid() },
	Terrain:     func(v string) bool { return api.Terrain(v).Valid() },
	StatusNone:  string(api.StatusConditionNone),
	WeatherNone: string(api.WeatherNone),
	TerrainNone: string(api.TerrainNone),
}

// individualFrom は保存済みの個体を契約の Individual に写す。個体として読めなければエラー。
func individualFrom(in domain.IndividualSnapshot) (api.Individual, error) {
	if !domain.IsSpeciesKey(in.SpeciesKey) || in.NatureID == "" {
		return api.Individual{}, errors.New("snapshot が個体として読めない")
	}
	level := in.Level
	status := api.StatusCondition(in.Status)
	ind := api.Individual{
		SpeciesKey: api.SpeciesKey(in.SpeciesKey),
		Level:      &level,
		NatureId:   in.NatureID,
		Sp:         api.StatBlock{Hp: in.Sp.Hp, Atk: in.Sp.Atk, Def: in.Sp.Def, Spa: in.Sp.Spa, Spd: in.Sp.Spd, Spe: in.Sp.Spe},
		Ranks:      &api.RankBlock{Atk: &in.Ranks.Atk, Def: &in.Ranks.Def, Spa: &in.Ranks.Spa, Spd: &in.Ranks.Spd, Spe: &in.Ranks.Spe},
		Status:     &status,
	}
	if in.AbilityID != "" {
		ind.AbilityId = &in.AbilityID
	}
	if in.ItemID != "" {
		ind.ItemId = &in.ItemID
	}
	if in.TeraType != "" {
		tera := api.PokeType(in.TeraType)
		ind.TeraType = &tera
	}
	return ind, nil
}

// calcFrom は保存済みの calc を契約の CalcRequest に写す。読めなければエラー。
func calcFrom(c domain.CalcSnapshot) (*api.CalcRequest, error) {
	if !api.Format(c.Format).Valid() || c.MoveID == "" ||
		!api.Weather(c.Field.Weather).Valid() || !api.Terrain(c.Field.Terrain).Valid() {
		return nil, errors.New("snapshot の calc が読めない")
	}
	attacker, err := individualFrom(c.Attacker)
	if err != nil {
		return nil, err
	}
	defender, err := individualFrom(c.Defender)
	if err != nil {
		return nil, err
	}
	weather, terrain := api.Weather(c.Field.Weather), api.Terrain(c.Field.Terrain)
	critical := c.Options.Critical
	return &api.CalcRequest{
		Format:   api.Format(c.Format),
		Attacker: attacker,
		Defender: defender,
		MoveId:   c.MoveID,
		Field: &api.FieldState{
			Weather: &weather, Terrain: &terrain,
			AttackerScreens: screensFrom(c.Field.AttackerScreens),
			DefenderScreens: screensFrom(c.Field.DefenderScreens),
		},
		Options:     &api.CalcOptions{Critical: &critical},
		BattleState: battleStateFrom(c.BattleState),
	}, nil
}

// battleStateFrom は保存済みの battleState を契約の CalcBattleState に写す(無ければ nil = キーを省く)。
func battleStateFrom(b *domain.BattleStateSnapshot) *api.CalcBattleState {
	if b == nil {
		return nil
	}
	return &api.CalcBattleState{AttackerCurrentHp: b.AttackerCurrentHP, DefenderCurrentHp: b.DefenderCurrentHP, Hits: b.Hits}
}

func screensFrom(s domain.ScreensSnapshot) *api.Screens {
	return &api.Screens{Reflect: &s.Reflect, LightScreen: &s.LightScreen, AuroraVeil: &s.AuroraVeil}
}

// favoriteFrom は store の1行を契約の Favorite に写す。snapshot が読めなければエラー(呼び出し側が 500 にする)。
func favoriteFrom(f app.Favorite) (api.Favorite, error) {
	var snap domain.FavoriteSnapshot
	if err := json.Unmarshal(f.Snapshot, &snap); err != nil {
		return api.Favorite{}, err
	}
	ind, err := individualFrom(snap.Individual)
	if err != nil {
		return api.Favorite{}, err
	}
	var calc *api.CalcRequest
	if snap.Calc != nil {
		if calc, err = calcFrom(*snap.Calc); err != nil {
			return api.Favorite{}, err
		}
	}
	return api.Favorite{
		Id:         strconv.FormatInt(f.ID, 10),
		Label:      snap.Label,
		Individual: ind,
		Calc:       calc,
		CreatedAt:  f.CreatedAt,
		UpdatedAt:  f.UpdatedAt,
	}, nil
}

// ListFavorites は GET /api/record/favorites。
func (s *Server) ListFavorites(ctx *echo.Context, params api.ListFavoritesParams) error {
	if err := checkHeaders(params.XDeviceId, params.XSessionId); err != nil {
		return err
	}
	if err := checkNoDeviceIDInQuery(ctx.QueryParams()); err != nil {
		return err
	}
	if err := decodeNoBody(ctx.Request().Body); err != nil {
		return err
	}
	rows, err := s.svc.ListFavorites(ctx.Request().Context(), params.XDeviceId)
	if err != nil {
		return errFromStore(params.XDeviceId, err)
	}
	out := make([]api.Favorite, 0)
	for _, r := range rows {
		fav, err := favoriteFrom(r)
		if err != nil {
			// 中身(label 等)はログに出さない。
			return errFromStore(params.XDeviceId, errors.New("お気に入りの snapshot が読めない"))
		}
		out = append(out, fav)
	}
	return ctx.JSON(http.StatusOK, out)
}

// CreateFavorite は POST /api/record/favorites。新規は 201、同じ内容が既存なら 200(updatedAt を進める)。
func (s *Server) CreateFavorite(ctx *echo.Context, params api.CreateFavoriteParams) error {
	if err := checkHeaders(params.XDeviceId, params.XSessionId); err != nil {
		return err
	}
	if err := checkNoDeviceIDInQuery(ctx.QueryParams()); err != nil {
		return err
	}
	var req domain.FavoriteInput
	if err := decodeStrict(limitedBody(ctx.Request().Body), &req); err != nil {
		return err
	}
	saved, outcome, err := s.svc.CreateFavorite(ctx.Request().Context(), params.XDeviceId, req)
	if err != nil {
		if herr, ok := errFromDomain(err); ok {
			return herr
		}
		if errors.Is(err, app.ErrFavoriteLimitReached) {
			return newError(api.InvalidInput, "1端末が持てるお気に入りの上限(%d件)に達している", app.MaxFavoritesPerDevice)
		}
		return errFromStore(params.XDeviceId, err)
	}
	status := http.StatusCreated
	if outcome == app.FavoriteExisted {
		status = http.StatusOK
	}
	fav, err := favoriteFrom(saved)
	if err != nil {
		return errFromStore(params.XDeviceId, errors.New("お気に入りの snapshot が読めない"))
	}
	return ctx.JSON(status, fav)
}

// DeleteFavorite は DELETE /api/record/favorites/{favoriteId}。形式違い・int64 超過は store を呼ばずに 404。
func (s *Server) DeleteFavorite(ctx *echo.Context, favoriteId api.FavoriteId, params api.DeleteFavoriteParams) error {
	if err := checkHeaders(params.XDeviceId, params.XSessionId); err != nil {
		return err
	}
	if err := checkNoDeviceIDInQuery(ctx.QueryParams()); err != nil {
		return err
	}
	if err := decodeNoBody(ctx.Request().Body); err != nil {
		return err
	}
	id, ok := parseFavoriteID(favoriteId)
	if !ok {
		return newError(api.NotFound, "お気に入りが無い")
	}
	if err := s.svc.DeleteFavorite(ctx.Request().Context(), params.XDeviceId, id); err != nil {
		if errors.Is(err, app.ErrNotFound) {
			return newError(api.NotFound, "お気に入りが無い")
		}
		return errFromStore(params.XDeviceId, err)
	}
	return ctx.NoContent(http.StatusNoContent)
}

func parseFavoriteID(s string) (int64, bool) {
	if !favoriteIDFormat.MatchString(s) {
		return 0, false
	}
	id, err := strconv.ParseInt(s, 10, 64)
	return id, err == nil
}
