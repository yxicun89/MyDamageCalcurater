package httpapi

// お気に入り(手動ピン留め)の3操作(ADR-0227。P5-3c)。record-svc は pokedex-svc に依存しないので、
// ID がマスタに実在するかは見ない(形式・範囲だけ検証する)。保存する内容は既定値を補った正規化済みの JSON
// (キーの順序が固定。同じ内容なら同じバイト列)で、store はそのバイト列のハッシュで重複を判定する。
// label・snapshot の中身は利用者の自由入力を含むのでログに出さない(ADR-0209 §3)。

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/record/internal/store"
)

const (
	maxFavoriteLabelRunes = 30
	favoriteLevel         = 50
	maxSPPerStat          = 32
	maxSPTotal            = 66
	maxRank               = 6
)

var (
	speciesKeyPattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{3}$`)
	favoriteIDFormat  = regexp.MustCompile(`^[1-9][0-9]{0,18}$`)
)

// favoriteRequest・individualRequest は本文の受け口。必須項目の欠落を見分けるためポインタで受ける。
// 契約に無いキーは DisallowUnknownFields(decodeStrict)で unknown_field になる。
type favoriteRequest struct {
	Label      *string            `json:"label"`
	Individual *individualRequest `json:"individual"`
}

type individualRequest struct {
	SpeciesKey *string       `json:"speciesKey"`
	Level      *int          `json:"level"`
	NatureID   *string       `json:"natureId"`
	AbilityID  *string       `json:"abilityId"`
	ItemID     *string       `json:"itemId"`
	Sp         *spRequest    `json:"sp"`
	Ranks      *rankSnapshot `json:"ranks"`
	TeraType   *string       `json:"teraType"`
	Status     *string       `json:"status"`
}

type spRequest struct {
	Hp  *int `json:"hp"`
	Atk *int `json:"atk"`
	Def *int `json:"def"`
	Spa *int `json:"spa"`
	Spd *int `json:"spd"`
	Spe *int `json:"spe"`
}

// rankSnapshot はランク。保存では6キー(HP を除く5キー)すべてを持つ(省略は0)。
type rankSnapshot struct {
	Atk *int `json:"atk"`
	Def *int `json:"def"`
	Spa *int `json:"spa"`
	Spd *int `json:"spd"`
	Spe *int `json:"spe"`
}

// favoriteSnapshot・individualSnapshot は保存する正規化済みの形。フィールドの宣言順がそのまま
// JSON のキー順になる(同じ内容なら同じバイト列)。
type favoriteSnapshot struct {
	Label      *string            `json:"label"`
	Individual individualSnapshot `json:"individual"`
}

type individualSnapshot struct {
	SpeciesKey string     `json:"speciesKey"`
	Level      int        `json:"level"`
	NatureID   string     `json:"natureId"`
	AbilityID  string     `json:"abilityId,omitempty"`
	ItemID     string     `json:"itemId,omitempty"`
	Sp         spSnapshot `json:"sp"`
	Ranks      rankValues `json:"ranks"`
	TeraType   string     `json:"teraType,omitempty"`
	Status     string     `json:"status"`
}

type spSnapshot struct {
	Hp  int `json:"hp"`
	Atk int `json:"atk"`
	Def int `json:"def"`
	Spa int `json:"spa"`
	Spd int `json:"spd"`
	Spe int `json:"spe"`
}

type rankValues struct {
	Atk int `json:"atk"`
	Def int `json:"def"`
	Spa int `json:"spa"`
	Spd int `json:"spd"`
	Spe int `json:"spe"`
}

// normalizeFavorite は本文を検証し、既定値を補った正規化済みの形にする。
func normalizeFavorite(req favoriteRequest) (favoriteSnapshot, error) {
	var out favoriteSnapshot
	if req.Label != nil && *req.Label != "" {
		if n := utf8.RuneCountInString(*req.Label); n > maxFavoriteLabelRunes {
			return out, newError(api.InvalidInput, "label は%d文字まで(%d文字)", maxFavoriteLabelRunes, n)
		}
		out.Label = req.Label
	}
	in := req.Individual
	if in == nil {
		return out, newError(api.InvalidInput, "individual が無い")
	}

	// 未知の列挙値は invalid_enum(範囲の検査より先に見る)。
	status := string(api.StatusConditionNone)
	if in.Status != nil {
		if !api.StatusCondition(*in.Status).Valid() {
			return out, newError(api.InvalidEnum, "status が未知の値")
		}
		status = *in.Status
	}
	tera := ""
	if in.TeraType != nil {
		if !api.PokeType(*in.TeraType).Valid() {
			return out, newError(api.InvalidEnum, "teraType が未知の値")
		}
		tera = *in.TeraType
	}

	if in.SpeciesKey == nil || !speciesKeyPattern.MatchString(*in.SpeciesKey) {
		return out, newError(api.InvalidInput, "speciesKey は {図鑑番号4桁}-{フォルム3桁} の形式でなければならない")
	}
	if in.NatureID == nil || *in.NatureID == "" {
		return out, newError(api.InvalidInput, "natureId が無い")
	}
	level := favoriteLevel
	if in.Level != nil {
		level = *in.Level
	}
	if level != favoriteLevel {
		return out, newError(api.InvalidInput, "level は%dでなければならない(%d)", favoriteLevel, level)
	}
	sp, err := normalizeSP(in.Sp)
	if err != nil {
		return out, err
	}
	ranks, err := normalizeRanks(in.Ranks)
	if err != nil {
		return out, err
	}

	out.Individual = individualSnapshot{
		SpeciesKey: *in.SpeciesKey, Level: level, NatureID: *in.NatureID,
		AbilityID: deref(in.AbilityID), ItemID: deref(in.ItemID),
		Sp: sp, Ranks: ranks, TeraType: tera, Status: status,
	}
	return out, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func normalizeSP(in *spRequest) (spSnapshot, error) {
	if in == nil || in.Hp == nil || in.Atk == nil || in.Def == nil || in.Spa == nil || in.Spd == nil || in.Spe == nil {
		return spSnapshot{}, newError(api.InvalidInput, "sp は6つの能力ポイントすべてが必要")
	}
	sp := spSnapshot{Hp: *in.Hp, Atk: *in.Atk, Def: *in.Def, Spa: *in.Spa, Spd: *in.Spd, Spe: *in.Spe}
	total := 0
	for _, v := range []int{sp.Hp, sp.Atk, sp.Def, sp.Spa, sp.Spd, sp.Spe} {
		if v < 0 || v > maxSPPerStat {
			return spSnapshot{}, newError(api.InvalidInput, "sp は各 0〜%d でなければならない", maxSPPerStat)
		}
		total += v
	}
	if total > maxSPTotal {
		return spSnapshot{}, newError(api.InvalidInput, "sp の合計は%d以下でなければならない(%d)", maxSPTotal, total)
	}
	return sp, nil
}

func normalizeRanks(in *rankSnapshot) (rankValues, error) {
	var r rankValues
	if in == nil {
		return r, nil
	}
	for _, f := range []struct {
		src *int
		dst *int
	}{{in.Atk, &r.Atk}, {in.Def, &r.Def}, {in.Spa, &r.Spa}, {in.Spd, &r.Spd}, {in.Spe, &r.Spe}} {
		if f.src == nil {
			continue
		}
		if *f.src < -maxRank || *f.src > maxRank {
			return r, newError(api.InvalidInput, "ランクは -%d〜+%d でなければならない", maxRank, maxRank)
		}
		*f.dst = *f.src
	}
	return r, nil
}

// favoriteFrom は store の1行を契約の Favorite に写す。snapshot が読めなければエラー(呼び出し側が 500 にする)。
func favoriteFrom(f store.Favorite) (api.Favorite, error) {
	var snap favoriteSnapshot
	if err := json.Unmarshal(f.Snapshot, &snap); err != nil {
		return api.Favorite{}, err
	}
	in := snap.Individual
	if !speciesKeyPattern.MatchString(in.SpeciesKey) || in.NatureID == "" {
		return api.Favorite{}, errors.New("snapshot が個体として読めない")
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
	return api.Favorite{
		Id:         strconv.FormatInt(f.ID, 10),
		Label:      snap.Label,
		Individual: ind,
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
	out := make([]api.Favorite, 0)
	err := touchAndRun(ctx.Request().Context(), s.store, params.XDeviceId, func() error {
		rows, err := s.store.ListFavorites(ctx.Request().Context(), params.XDeviceId)
		if err != nil {
			return errFromStore(params.XDeviceId, err)
		}
		for _, r := range rows {
			fav, err := favoriteFrom(r)
			if err != nil {
				// 中身(label 等)はログに出さない。
				return errFromStore(params.XDeviceId, errors.New("お気に入りの snapshot が読めない"))
			}
			out = append(out, fav)
		}
		return nil
	})
	if err != nil {
		return err
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
	var req favoriteRequest
	if err := decodeStrict(limitedBody(ctx.Request().Body), &req); err != nil {
		return err
	}
	snap, err := normalizeFavorite(req)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(snap)
	if err != nil {
		return errFromStore(params.XDeviceId, err)
	}

	var result api.Favorite
	status := http.StatusCreated
	err = touchAndRun(ctx.Request().Context(), s.store, params.XDeviceId, func() error {
		saved, outcome, err := s.store.CreateFavorite(ctx.Request().Context(), params.XDeviceId,
			store.Favorite{SpeciesKey: snap.Individual.SpeciesKey, Snapshot: raw}, time.Now().UTC())
		if err != nil {
			if errors.Is(err, store.ErrFavoriteLimitReached) {
				return newError(api.InvalidInput, "1端末が持てるお気に入りの上限(%d件)に達している", store.MaxFavoritesPerDevice)
			}
			return errFromStore(params.XDeviceId, err)
		}
		if outcome == store.FavoriteExisted {
			status = http.StatusOK
		}
		fav, err := favoriteFrom(saved)
		if err != nil {
			return errFromStore(params.XDeviceId, errors.New("お気に入りの snapshot が読めない"))
		}
		result = fav
		return nil
	})
	if err != nil {
		return err
	}
	return ctx.JSON(status, result)
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
	err := touchAndRun(ctx.Request().Context(), s.store, params.XDeviceId, func() error {
		if err := s.store.DeleteFavorite(ctx.Request().Context(), params.XDeviceId, id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return newError(api.NotFound, "お気に入りが無い")
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

func parseFavoriteID(s string) (int64, bool) {
	if !favoriteIDFormat.MatchString(s) {
		return 0, false
	}
	id, err := strconv.ParseInt(s, 10, 64)
	return id, err == nil
}
