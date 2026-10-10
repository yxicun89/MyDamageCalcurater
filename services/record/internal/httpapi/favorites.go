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

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/record/internal/store"
)

// SP・レベルの上限は engine の定数(calc-svc が使う正)を参照し、ここで二重に定義しない。
// ランクの上限だけは engine に定数が無い(engine は ±6 をリテラルで持つ)ので、ここに置く(ADR-0228 §3)。
const (
	maxFavoriteLabelRunes = 30
	maxRank               = 6
	// maxFavoriteSnapshotBytes は正規化後の snapshot の上限(ADR-0228 §3。正当な入力は 2KB に満たない)。
	maxFavoriteSnapshotBytes = 4096
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
	Calc       *calcRequest       `json:"calc"`
}

// calcRequest は契約の CalcRequest の受け口(ADR-0228)。欠落を見分けるためポインタで受ける。
type calcRequest struct {
	Format   *string            `json:"format"`
	Attacker *individualRequest `json:"attacker"`
	Defender *individualRequest `json:"defender"`
	MoveID   *string            `json:"moveId"`
	Field    *fieldRequest      `json:"field"`
	Options  *optionsRequest    `json:"options"`
	// BattleState は対戦の状態(残り HP・多段の回数。ADR-0144)。省略・null・{} は省略のまま保存する。
	BattleState *battleStateRequest `json:"battleState"`
}

// battleStateRequest は契約の CalcBattleState の受け口。値域は契約の minimum / maximum と同じ(最大 HP との照合はマスタが要るのでしない。
// 最大を超える残り HP は計算するとき calc-svc が invalid_input にする)。
type battleStateRequest struct {
	AttackerCurrentHP *int `json:"attackerCurrentHp"`
	DefenderCurrentHP *int `json:"defenderCurrentHp"`
	Hits              *int `json:"hits"`
}

type fieldRequest struct {
	Weather         *string         `json:"weather"`
	Terrain         *string         `json:"terrain"`
	AttackerScreens *screensRequest `json:"attackerScreens"`
	DefenderScreens *screensRequest `json:"defenderScreens"`
}

type screensRequest struct {
	Reflect     *bool `json:"reflect"`
	LightScreen *bool `json:"lightScreen"`
	AuroraVeil  *bool `json:"auroraVeil"`
}

type optionsRequest struct {
	Critical *bool `json:"critical"`
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
	// Calc は計算の入力全体(ADR-0228)。無いときはキーごと出さない(既存行のバイト列・ハッシュを変えない)。
	Calc *calcSnapshot `json:"calc,omitempty"`
}

// calcSnapshot は calc の正規化済みの形。フィールドの宣言順が契約のプロパティ順(= JSON のキー順)。
type calcSnapshot struct {
	Format   string             `json:"format"`
	Attacker individualSnapshot `json:"attacker"`
	Defender individualSnapshot `json:"defender"`
	MoveID   string             `json:"moveId"`
	Field    fieldSnapshot      `json:"field"`
	Options  optionsSnapshot    `json:"options"`
	// BattleState は対戦の状態(ADR-0144)。無いときはキーごと出さない(既存行のバイト列・ハッシュを変えない)。
	BattleState *battleStateSnapshot `json:"battleState,omitempty"`
}

// battleStateSnapshot は battleState の正規化済みの形(指定したキーだけを持つ。フィールドの宣言順が JSON のキー順)。
type battleStateSnapshot struct {
	AttackerCurrentHP *int `json:"attackerCurrentHp,omitempty"`
	DefenderCurrentHP *int `json:"defenderCurrentHp,omitempty"`
	Hits              *int `json:"hits,omitempty"`
}

type fieldSnapshot struct {
	Weather         string          `json:"weather"`
	Terrain         string          `json:"terrain"`
	AttackerScreens screensSnapshot `json:"attackerScreens"`
	DefenderScreens screensSnapshot `json:"defenderScreens"`
}

type screensSnapshot struct {
	Reflect     bool `json:"reflect"`
	LightScreen bool `json:"lightScreen"`
	AuroraVeil  bool `json:"auroraVeil"`
}

type optionsSnapshot struct {
	Critical bool `json:"critical"`
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
	ind, err := normalizeIndividual(req.Individual)
	if err != nil {
		return out, err
	}
	out.Individual = ind
	if req.Calc != nil {
		calc, err := normalizeCalc(req.Calc)
		if err != nil {
			return out, err
		}
		out.Calc = &calc
	}
	return out, nil
}

// normalizeIndividual は individual と calc の attacker / defender に共通の検証・正規化(ADR-0228 §3)。
func normalizeIndividual(in *individualRequest) (individualSnapshot, error) {
	var out individualSnapshot
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
	level := engine.DefaultLevel
	if in.Level != nil {
		level = *in.Level
	}
	if level != engine.DefaultLevel {
		return out, newError(api.InvalidInput, "level は%dでなければならない(%d)", engine.DefaultLevel, level)
	}
	sp, err := normalizeSP(in.Sp)
	if err != nil {
		return out, err
	}
	ranks, err := normalizeRanks(in.Ranks)
	if err != nil {
		return out, err
	}

	out = individualSnapshot{
		SpeciesKey: *in.SpeciesKey, Level: level, NatureID: *in.NatureID,
		AbilityID: deref(in.AbilityID), ItemID: deref(in.ItemID),
		Sp: sp, Ranks: ranks, TeraType: tera, Status: status,
	}
	return out, nil
}

// normalizeCalc は calc を検証し、既定値を補った形にする。マスタとは照合しない(ADR-0228 §3)。
func normalizeCalc(in *calcRequest) (calcSnapshot, error) {
	var out calcSnapshot
	if in.Format == nil || !api.Format(*in.Format).Valid() {
		return out, newError(api.InvalidEnum, "calc.format が無いか未知の値")
	}
	weather, terrain := string(api.WeatherNone), string(api.TerrainNone)
	var fieldIn fieldRequest
	if in.Field != nil {
		fieldIn = *in.Field
	}
	if fieldIn.Weather != nil {
		if !api.Weather(*fieldIn.Weather).Valid() {
			return out, newError(api.InvalidEnum, "calc.field.weather が未知の値")
		}
		weather = *fieldIn.Weather
	}
	if fieldIn.Terrain != nil {
		if !api.Terrain(*fieldIn.Terrain).Valid() {
			return out, newError(api.InvalidEnum, "calc.field.terrain が未知の値")
		}
		terrain = *fieldIn.Terrain
	}
	if in.Attacker == nil || in.Defender == nil {
		return out, newError(api.InvalidInput, "calc.attacker と calc.defender が必要")
	}
	if in.MoveID == nil || *in.MoveID == "" {
		return out, newError(api.InvalidInput, "calc.moveId が無い")
	}
	attacker, err := normalizeIndividual(in.Attacker)
	if err != nil {
		return out, err
	}
	defender, err := normalizeIndividual(in.Defender)
	if err != nil {
		return out, err
	}
	out = calcSnapshot{
		Format: *in.Format, Attacker: attacker, Defender: defender, MoveID: *in.MoveID,
		Field: fieldSnapshot{
			Weather: weather, Terrain: terrain,
			AttackerScreens: normalizeScreens(fieldIn.AttackerScreens),
			DefenderScreens: normalizeScreens(fieldIn.DefenderScreens),
		},
	}
	if in.Options != nil && in.Options.Critical != nil {
		out.Options.Critical = *in.Options.Critical
	}
	if out.BattleState, err = normalizeBattleState(in.BattleState); err != nil {
		return out, err
	}
	return out, nil
}

// 対戦の状態の上限(契約の CalcBattleState.hits の maximum)。
const maxBattleStateHits = 10

// normalizeBattleState は battleState を契約(CalcBattleState)と同じ値域で検証する。残り HP は 1 以上(0 は満タンの意味にしない)、
// 回数は 1..10。何も指定が無い({})は省略と同じ(nil)。
func normalizeBattleState(in *battleStateRequest) (*battleStateSnapshot, error) {
	if in == nil {
		return nil, nil
	}
	for _, f := range []struct {
		name string
		v    *int
		max  int
	}{
		{"calc.battleState.attackerCurrentHp", in.AttackerCurrentHP, 0},
		{"calc.battleState.defenderCurrentHp", in.DefenderCurrentHP, 0},
		{"calc.battleState.hits", in.Hits, maxBattleStateHits},
	} {
		if f.v == nil {
			continue
		}
		if *f.v < 1 || (f.max > 0 && *f.v > f.max) {
			if f.max > 0 {
				return nil, newError(api.InvalidInput, "%s は 1〜%d でなければならない", f.name, f.max)
			}
			return nil, newError(api.InvalidInput, "%s は 1 以上でなければならない", f.name)
		}
	}
	if in.AttackerCurrentHP == nil && in.DefenderCurrentHP == nil && in.Hits == nil {
		return nil, nil
	}
	return &battleStateSnapshot{AttackerCurrentHP: in.AttackerCurrentHP, DefenderCurrentHP: in.DefenderCurrentHP, Hits: in.Hits}, nil
}

func normalizeScreens(in *screensRequest) screensSnapshot {
	if in == nil {
		return screensSnapshot{}
	}
	return screensSnapshot{
		Reflect: derefBool(in.Reflect), LightScreen: derefBool(in.LightScreen), AuroraVeil: derefBool(in.AuroraVeil),
	}
}

func derefBool(b *bool) bool { return b != nil && *b }

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
		if v < 0 || v > engine.MaxSPPerStat {
			return spSnapshot{}, newError(api.InvalidInput, "sp は各 0〜%d でなければならない", engine.MaxSPPerStat)
		}
		total += v
	}
	if total > engine.MaxSPTotal {
		return spSnapshot{}, newError(api.InvalidInput, "sp の合計は%d以下でなければならない(%d)", engine.MaxSPTotal, total)
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

// individualFrom は保存済みの個体を契約の Individual に写す。個体として読めなければエラー。
func individualFrom(in individualSnapshot) (api.Individual, error) {
	if !speciesKeyPattern.MatchString(in.SpeciesKey) || in.NatureID == "" {
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
func calcFrom(c calcSnapshot) (*api.CalcRequest, error) {
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
func battleStateFrom(b *battleStateSnapshot) *api.CalcBattleState {
	if b == nil {
		return nil
	}
	return &api.CalcBattleState{AttackerCurrentHp: b.AttackerCurrentHP, DefenderCurrentHp: b.DefenderCurrentHP, Hits: b.Hits}
}

func screensFrom(s screensSnapshot) *api.Screens {
	return &api.Screens{Reflect: &s.Reflect, LightScreen: &s.LightScreen, AuroraVeil: &s.AuroraVeil}
}

// favoriteFrom は store の1行を契約の Favorite に写す。snapshot が読めなければエラー(呼び出し側が 500 にする)。
func favoriteFrom(f store.Favorite) (api.Favorite, error) {
	var snap favoriteSnapshot
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
	if len(raw) > maxFavoriteSnapshotBytes {
		return newError(api.InvalidInput, "お気に入りの内容が大きすぎる(%dバイト以下)", maxFavoriteSnapshotBytes)
	}

	var result api.Favorite
	status := http.StatusCreated
	err = touchAndRun(ctx.Request().Context(), s.store, params.XDeviceId, func() error {
		saved, outcome, err := s.store.CreateFavorite(ctx.Request().Context(), params.XDeviceId,
			store.Favorite{SpeciesKey: snap.Individual.SpeciesKey, Snapshot: raw}, time.Now().UTC().Truncate(time.Microsecond))
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
