package httpapi

// 計算履歴の一覧 GET /api/record/calc-history(ADR-0230)。calc_events の payload(calcevents.Event)から
// 1件の計算の入力と結果の要約だけを取り出して返す。端末 ID・セッション ID・event_id などは返さない。
// 読めない行は飛ばし、ログには event_id だけを出す(payload の中身・個体・数値は出さない。ADR-0209 §3)。

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/internal/calcevents"
	"example.com/pokecalc/services/record/internal/app"
	"example.com/pokecalc/services/record/internal/domain"
)

// 契約の limit(1〜50・既定 20)と cursor(1〜200文字・URL にそのまま入る文字だけ)。
const (
	defaultHistoryLimit = 20
	minHistoryLimit     = 1
	maxHistoryLimit     = 50
	maxCursorLength     = 200
)

var cursorPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// encodeHistoryCursor は位置を不透明な文字列にする(occurred_at の UnixNano と event_id を ":" で区切り base64url)。
func encodeHistoryCursor(c app.CalcHistoryCursor) string {
	raw := strconv.FormatInt(c.OccurredAt.UnixNano(), 10) + ":" + c.EventID
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeHistoryCursor(s string) (*app.CalcHistoryCursor, bool) {
	if len(s) == 0 || len(s) > maxCursorLength || !cursorPattern.MatchString(s) {
		return nil, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, false
	}
	nanos, id, ok := strings.Cut(string(raw), ":")
	if !ok || id == "" {
		return nil, false
	}
	n, err := strconv.ParseInt(nanos, 10, 64)
	if err != nil {
		return nil, false
	}
	return &app.CalcHistoryCursor{OccurredAt: time.Unix(0, n).UTC(), EventID: id}, true
}

// ListCalcHistory は GET /api/record/calc-history。
func (s *Server) ListCalcHistory(ctx *echo.Context, params api.ListCalcHistoryParams) error {
	if err := checkHeaders(params.XDeviceId, params.XSessionId); err != nil {
		return err
	}
	if err := checkNoDeviceIDInQuery(ctx.QueryParams()); err != nil {
		return err
	}
	if err := decodeNoBody(ctx.Request().Body); err != nil {
		return err
	}
	limit := defaultHistoryLimit
	if params.Limit != nil {
		limit = *params.Limit
	}
	if limit < minHistoryLimit || limit > maxHistoryLimit {
		return newError(api.InvalidInput, "limit は %d〜%d の範囲でなければならない(%d)", minHistoryLimit, maxHistoryLimit, limit)
	}
	var before *app.CalcHistoryCursor
	if ctx.QueryParams().Has("cursor") {
		c, ok := decodeHistoryCursor(ctx.QueryParam("cursor"))
		if !ok {
			return newError(api.InvalidInput, "cursor が読めない(nextCursor の値をそのまま渡す)")
		}
		before = c
	}

	rows, next, err := s.svc.ListCalcHistory(ctx.Request().Context(), params.XDeviceId, before, limit)
	if err != nil {
		return errFromStore(params.XDeviceId, err)
	}
	page := api.CalcHistoryPage{Items: make([]api.CalcHistoryEntry, 0, limit)}
	if next != nil {
		c := encodeHistoryCursor(*next)
		page.NextCursor = &c
	}
	for _, r := range rows {
		entry, err := calcHistoryEntryFrom(r)
		if err != nil {
			// payload の中身は出さない。event_id だけ。
			slog.Warn("record-svc: 読めない計算履歴の行を飛ばす", "eventId", r.EventID)
			continue
		}
		page.Items = append(page.Items, entry)
	}
	return ctx.JSON(http.StatusOK, page)
}

// calcHistoryEntryFrom は calc_events の payload から契約の1行を作る。必要な項目だけを詰め替える
// (payload を丸ごと中継しない)。未知のキーは許し、読めなければエラー(呼び出し側が飛ばす)。
func calcHistoryEntryFrom(r app.CalcHistoryRow) (api.CalcHistoryEntry, error) {
	var ev calcevents.Event
	if err := json.Unmarshal(r.Payload, &ev); err != nil {
		return api.CalcHistoryEntry{}, err
	}
	if ev.SchemaVersion != calcevents.SchemaVersion || ev.Operation != calcevents.OperationCalc || ev.Detail == nil {
		return api.CalcHistoryEntry{}, errors.New("計算イベントとして読めない")
	}
	d := ev.Detail
	// calc-svc が保存した入力を、お気に入りと同じ関数(domain)で既定値を補った形にする。
	req := calcRequestFromDetail(d)
	snap, err := domain.NormalizeCalc(&req, contractEnums)
	if err != nil {
		return api.CalcHistoryEntry{}, err
	}
	calc, err := calcFrom(snap)
	if err != nil {
		return api.CalcHistoryEntry{}, err
	}
	return api.CalcHistoryEntry{
		OccurredAt: r.OccurredAt,
		Calc:       *calc,
		Result:     api.CalcHistoryResult{MinPercent: d.MinPercent, MaxPercent: d.MaxPercent},
	}, nil
}

// calcRequestFromDetail は detail を domain.NormalizeCalc の受け口(domain.CalcInput)に写す。型の違い(ポインタ・
// 文字列)を JSON 経由で吸収するのではなく、必要な項目を明示的に詰める。
func calcRequestFromDetail(d *calcevents.CalcDetail) domain.CalcInput {
	req := domain.CalcInput{
		Format:   &d.Format,
		Attacker: individualRequestFrom(d.Attacker),
		Defender: individualRequestFrom(d.Defender),
		MoveID:   &d.MoveID,
	}
	if f := d.Field; f != nil {
		fr := &domain.FieldInput{
			AttackerScreens: screensRequestFrom(f.AttackerScreens),
			DefenderScreens: screensRequestFrom(f.DefenderScreens),
		}
		if f.Weather != nil {
			w := string(*f.Weather)
			fr.Weather = &w
		}
		if f.Terrain != nil {
			t := string(*f.Terrain)
			fr.Terrain = &t
		}
		req.Field = fr
	}
	if d.Options != nil {
		req.Options = &domain.OptionsInput{Critical: d.Options.Critical}
	}
	if b := d.BattleState; b != nil {
		req.BattleState = &domain.BattleStateInput{AttackerCurrentHP: b.AttackerCurrentHp, DefenderCurrentHP: b.DefenderCurrentHp, Hits: b.Hits}
	}
	return req
}

func screensRequestFrom(s *api.Screens) *domain.ScreensInput {
	if s == nil {
		return nil
	}
	return &domain.ScreensInput{Reflect: s.Reflect, LightScreen: s.LightScreen, AuroraVeil: s.AuroraVeil}
}

func individualRequestFrom(in api.Individual) *domain.IndividualInput {
	key := string(in.SpeciesKey)
	nature := in.NatureId
	out := &domain.IndividualInput{
		SpeciesKey: &key,
		Level:      in.Level,
		NatureID:   &nature,
		AbilityID:  in.AbilityId,
		ItemID:     in.ItemId,
		Sp:         &domain.SPInput{Hp: &in.Sp.Hp, Atk: &in.Sp.Atk, Def: &in.Sp.Def, Spa: &in.Sp.Spa, Spd: &in.Sp.Spd, Spe: &in.Sp.Spe},
	}
	if r := in.Ranks; r != nil {
		out.Ranks = &domain.RankInput{Atk: r.Atk, Def: r.Def, Spa: r.Spa, Spd: r.Spd, Spe: r.Spe}
	}
	if in.TeraType != nil {
		t := string(*in.TeraType)
		out.TeraType = &t
	}
	if in.Status != nil {
		st := string(*in.Status)
		out.Status = &st
	}
	return out
}
