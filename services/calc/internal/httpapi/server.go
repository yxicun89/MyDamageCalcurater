// Package httpapi は calc-svc の HTTP 境界(ADR-0016)。生成物 api.ServerInterface を実装する。
//
// 1リクエストの流れ(engine/wasmapi と同じ順。同じ失敗は同じ code にする):
//
//	厳格デコード(unknown_field / invalid_json)→ 列挙の検証(invalid_enum)
//	→ ID 解決(unknown_*)→ engine の入力検証(invalid_input)→ engine 呼び出し → 生成型への写し
//
// 独自のダメージ式・独自の丸めを持たない(CLAUDE.md 絶対ルール2・3)。表示%は engine の
// 0.1% 単位の整数(tenths)を 10 で割るだけ。計算はイベント保存に依存しない(絶対ルール5)。
package httpapi

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/calc/internal/master"
	"example.com/pokecalc/services/internal/api"
)

// messageInternal は回復した panic・想定外の失敗に付ける固定文。
// Go のランタイム情報をクライアントへ出さない(ADR-0016 AC-7)。
const messageInternal = "内部エラーが発生した"

// Server は api.ServerInterface を実装する。マスタは Store 経由でだけ引く。
type Server struct {
	store master.Store
}

var _ api.ServerInterface = (*Server)(nil)

// NewServer は Store を使う Server を作る。
func NewServer(store master.Store) *Server {
	return &Server{store: store}
}

// NewHandler は calc-svc の HTTP ハンドラ全体を組み立てる。
// 生成ルート(api.RegisterHandlers)、GET /healthz(openapi に載せない運用エンドポイント)、
// panic の回復(500 internal)、echo の既定エラー(ルート無し・メソッド違い・ヘッダ欠落)を
// Error 形式({"code","message"})に揃えるエラーハンドラを含む。
func NewHandler(store master.Store) http.Handler {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(recoverMiddleware)
	e.HTTPErrorHandler = httpErrorHandler

	api.RegisterHandlers(e, NewServer(store))
	e.GET("/healthz", healthzHandler)
	return e
}

func healthzHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// recoverMiddleware は panic を回復し、500 internal の httpError にする(スタック等を出さない)。
func recoverMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = newError(api.Internal, "%s", messageInternal)
			}
		}()
		return next(c)
	}
}

// httpErrorHandler は echo に渡ったエラー(Server が返した httpError・生成ラッパのヘッダ検証・
// echo の既定 404/405 を含む)をすべて Error 形式に揃える。
func httpErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}
	status, body := errorBodyFor(err)
	_ = c.JSON(status, body)
}

func errorBodyFor(err error) (int, api.Error) {
	var he *httpError
	if errors.As(err, &he) {
		return he.status, api.Error{Code: he.code, Message: he.message}
	}
	var ee *echo.HTTPError
	if errors.As(err, &ee) {
		switch ee.Code {
		case http.StatusNotFound, http.StatusMethodNotAllowed:
			// ルートが無い・メソッドが違う(ADR-0016: メソッド違いに新しい code を足さず not_found にする)。
			return http.StatusNotFound, api.Error{Code: api.NotFound, Message: "ルートが無い"}
		default:
			// 生成ラッパ(ServerInterfaceWrapper)が返すのは必須ヘッダ欠落のときだけ(calc-svc の
			// 操作はヘッダ以外のパラメータを持たない)。
			return http.StatusBadRequest, api.Error{Code: api.MissingHeader, Message: "X-Device-Id / X-Session-Id が無い"}
		}
	}
	return http.StatusInternalServerError, api.Error{Code: api.Internal, Message: messageInternal}
}

// checkHeaders は X-Device-Id / X-Session-Id の欠落・空を missing_header にする。
// UUID 形式の検証はしない(gateway の仕事。ADR-0016 AC-6)。
func checkHeaders(deviceID, sessionID string) error {
	if deviceID == "" || sessionID == "" {
		return newError(api.MissingHeader, "X-Device-Id / X-Session-Id が無い")
	}
	return nil
}

// notFoundForPokedex は calc-svc の担当外(pokedex)の操作に返す 404。
func notFoundForPokedex() error {
	return newError(api.NotFound, "このサービスの担当外の操作")
}

// CalcDamage は POST /api/calc。
func (s *Server) CalcDamage(ctx echo.Context, params api.CalcDamageParams) error {
	if err := checkHeaders(params.XDeviceId, params.XSessionId); err != nil {
		return err
	}
	var req api.CalcRequest
	if err := decodeStrict(ctx.Request().Body, &req); err != nil {
		return err
	}
	format, err := parseFormat(req.Format)
	if err != nil {
		return err
	}
	attacker, err := s.resolveIndividual("attacker", req.Attacker)
	if err != nil {
		return err
	}
	defender, err := s.resolveIndividual("defender", req.Defender)
	if err != nil {
		return err
	}
	move, err := s.resolveMove(req.MoveId)
	if err != nil {
		return err
	}
	field, err := parseField(req.Field)
	if err != nil {
		return err
	}
	if err := validateIndividual("攻撃側", attacker); err != nil {
		return err
	}
	if err := validateIndividual("防御側", defender); err != nil {
		return err
	}

	res, err := engine.CalcDamage(engine.DamageInput{
		Format: format, Attacker: attacker, Defender: defender, Move: move, Field: field,
		Critical: criticalFrom(req.Options), TypeChart: s.store.TypeChart(),
	})
	if err != nil {
		return errFromEngine(err)
	}
	return ctx.JSON(http.StatusOK, calcResultFrom(res))
}

// CalcBulk は POST /api/calc/bulk。
func (s *Server) CalcBulk(ctx echo.Context, params api.CalcBulkParams) error {
	if err := checkHeaders(params.XDeviceId, params.XSessionId); err != nil {
		return err
	}
	var req api.BulkCalcRequest
	if err := decodeStrict(ctx.Request().Body, &req); err != nil {
		return err
	}
	format, err := parseFormat(req.Format)
	if err != nil {
		return err
	}
	attacker, err := s.resolveIndividual("attacker", req.Attacker)
	if err != nil {
		return err
	}
	species, err := s.resolveSpecies("defenderSpeciesKey", req.DefenderSpeciesKey)
	if err != nil {
		return err
	}
	move, err := s.resolveMove(req.MoveId)
	if err != nil {
		return err
	}
	field, err := parseField(req.Field)
	if err != nil {
		return err
	}
	variants, err := s.resolveItems("itemVariants", req.ItemVariants)
	if err != nil {
		return err
	}
	if err := validateIndividual("攻撃側", attacker); err != nil {
		return err
	}
	if err := validateIndividual("防御側の種族", engine.Individual{Species: species}); err != nil {
		return err
	}

	res, err := engine.CalcBulk(engine.BulkInput{
		Format: format, Attacker: attacker, DefenderSpecies: species, Move: move, Field: field,
		Critical: criticalFrom(req.Options), PresetKeys: presetKeysFrom(req.Presets), ItemVariants: variants,
		TypeChart: s.store.TypeChart(),
	})
	if err != nil {
		return errFromEngine(err)
	}
	return ctx.JSON(http.StatusOK, s.bulkResultFrom(res))
}

// CalcReverse は POST /api/calc/reverse。
func (s *Server) CalcReverse(ctx echo.Context, params api.CalcReverseParams) error {
	if err := checkHeaders(params.XDeviceId, params.XSessionId); err != nil {
		return err
	}
	var req api.ReverseRequest
	if err := decodeStrict(ctx.Request().Body, &req); err != nil {
		return err
	}
	format, err := parseFormat(req.Format)
	if err != nil {
		return err
	}
	known, err := s.resolveIndividual("known", req.Known)
	if err != nil {
		return err
	}
	species, err := s.resolveSpecies("unknownSpeciesKey", req.UnknownSpeciesKey)
	if err != nil {
		return err
	}
	move, err := s.resolveMove(req.MoveId)
	if err != nil {
		return err
	}
	field, err := parseField(req.Field)
	if err != nil {
		return err
	}
	items, err := s.resolveItems("itemCandidates", req.ItemCandidates)
	if err != nil {
		return err
	}
	observations, err := convertObservations(req.Observations)
	if err != nil {
		return err
	}
	if err := validateIndividual("既知の側", known); err != nil {
		return err
	}
	if err := validateIndividual("推定側の種族", engine.Individual{Species: species}); err != nil {
		return err
	}
	maxCandidates := derefInt(req.MaxCandidates)
	if maxCandidates < 0 {
		return newError(api.InvalidInput, "maxCandidates は 0 以上でなければならない: %d", maxCandidates)
	}

	res, err := engine.CalcReverse(engine.ReverseInput{
		Format: format, Side: engine.ReverseSide(req.Side), Known: known, UnknownSpecies: species,
		Move: move, Field: field, Critical: criticalFrom(req.Options), ItemCandidates: items,
		Observations: observations, MaxCandidates: maxCandidates, TypeChart: s.store.TypeChart(),
	})
	if err != nil {
		return errFromEngine(err)
	}
	return ctx.JSON(http.StatusOK, s.reverseResultFrom(res))
}

// SearchItems は pokedex の操作。calc-svc の担当外なので 404 not_found。
func (s *Server) SearchItems(ctx echo.Context, params api.SearchItemsParams) error {
	return notFoundForPokedex()
}

// SearchMoves は pokedex の操作。calc-svc の担当外なので 404 not_found。
func (s *Server) SearchMoves(ctx echo.Context, params api.SearchMovesParams) error {
	return notFoundForPokedex()
}

// ListNatures は pokedex の操作。calc-svc の担当外なので 404 not_found。
func (s *Server) ListNatures(ctx echo.Context, params api.ListNaturesParams) error {
	return notFoundForPokedex()
}

// SearchSpecies は pokedex の操作。calc-svc の担当外なので 404 not_found。
func (s *Server) SearchSpecies(ctx echo.Context, params api.SearchSpeciesParams) error {
	return notFoundForPokedex()
}

// GetSpecies は pokedex の操作。calc-svc の担当外なので 404 not_found。
func (s *Server) GetSpecies(ctx echo.Context, key api.SpeciesKey, params api.GetSpeciesParams) error {
	return notFoundForPokedex()
}
