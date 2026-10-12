// Package httpapi は calc-svc の HTTP 境界(ADR-0200)。生成物 api.ServerInterface を実装する。
//
// 1リクエストの流れ(engine/wasmapi と同じ順。同じ失敗は同じ code にする):
//
//	ヘッダ・厳格デコード(unknown_field / invalid_json)・件数上限(ここ)
//	→ 列挙の検証・ID 解決・入力の検証・engine 呼び出し・イベント発行(計算・一括・逆算・連続技は app.Service。ADR-0811)
//	→ 生成型への写し(ここ)
//
// 調整(adjust.go)の手順はまだここにある(S3 の後半の PR で app へ移す)。
//
// 独自のダメージ式・独自の丸めを持たない(CLAUDE.md 絶対ルール2・3)。表示%は engine の
// 0.1% 単位の整数(tenths)を 10 で割るだけ。計算はイベント保存に依存しない(絶対ルール5)。
package httpapi

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/calc/internal/adapters/master"
	"example.com/pokecalc/services/calc/internal/app"
	"example.com/pokecalc/services/calc/internal/domain"
	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/internal/httpguard"
	"example.com/pokecalc/services/internal/httpmetrics"
	"example.com/pokecalc/services/internal/reqlog"
	"example.com/pokecalc/services/internal/version"
)

// messageInternal は回復した panic・想定外の失敗に付ける固定文。
// Go のランタイム情報をクライアントへ出さない(ADR-0200 AC-7)。
const messageInternal = domain.MessageInternal

// maxRequestBodyBytes はリクエスト本文の上限(critic 指摘 R7)。1MiB を超える本文は
// 読み込みを打ち切り invalid_json にする(無制限に読み込んでメモリを使い切らないため)。
const maxRequestBodyBytes = 1 << 20 // 1MiB

// Server は api.ServerInterface を実装する。計算・一括・逆算の手順は app.Service に委ね、ここは
// 生成型 ⇄ app の型の変換と HTTP のエラー整形だけを持つ(ADR-0811)。マスタは Store 経由でだけ引く。
type Server struct {
	store master.Store
	app   *app.Service
}

var _ api.ServerInterface = (*Server)(nil)

// NewServer は Store と(任意の)EventPublisher を使う Server を作る。publisher が nil なら
// 発行しない(呼び出し側で毎回 nil チェックしなくてよいように)。
func NewServer(store master.Store, publisher EventPublisher) *Server {
	var events app.EventPublisher
	if publisher != nil {
		events = eventBridge{publisher: publisher}
	}
	return &Server{store: store, app: app.New(store, events, httpguard.Expired)}
}

// NewHandler は calc-svc の HTTP ハンドラ全体を組み立てる。
// calc の3操作と調整の5操作(生成ラッパ経由)、pokedex の7操作(直接 404。R1)、GET /healthz
// (openapi に載せない運用エンドポイント)、panic の回復(500 internal)、echo の既定エラー
// (ルート無し・メソッド違い)を Error 形式({"code","message"})に揃えるエラーハンドラを含む。
func NewHandler(store master.Store, publisher EventPublisher, opts ...Option) http.Handler {
	e := echo.New()
	e.HTTPErrorHandler = httpErrorHandler
	e.Logger = slog.Default() // main が JSON ハンドラを既定にした後に呼ばれる(issue #246)
	m := httpmetrics.New()
	e.Use(reqlog.Middleware(slog.Default())) // 最も外側: ID の確定とアクセスログ(issue #246)
	e.Use(m.Middleware())
	e.Use(recoverMiddleware)
	e.GET(httpmetrics.Path, m.Handler())

	registerCalcRoutes(e, NewServer(store, publisher), guardMiddleware(opts))
	registerPokedexNotFoundRoutes(e)
	e.GET("/healthz", healthzHandler)
	e.GET("/readyz", readyzHandler)
	return e
}

// readyzHandler は起動時に読み込み済み(NewHandler)の /readyz。常に 200(ADR-0204 §3)。
func readyzHandler(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// registerCalcRoutes は calc の3操作と調整の5操作だけを、生成ラッパ(api.ServerInterfaceWrapper。
// 必須ヘッダ X-Device-Id / X-Session-Id の有無を検証してから Server を呼ぶ)経由で登録する。
// pokedex はここに含めない(registerPokedexNotFoundRoutes 参照。critic 指摘 R1)。
func registerCalcRoutes(e *echo.Echo, srv *Server, guard echo.MiddlewareFunc) {
	wrapper := api.ServerInterfaceWrapper{Handler: srv}
	e.POST("/api/calc", wrapper.CalcDamage, guard)
	e.POST("/api/calc/bulk", wrapper.CalcBulk, guard)
	e.POST("/api/calc/reverse", wrapper.CalcReverse, guard)
	// 調整の5操作(ADR-0250 §8・ADR-0177 §9)。ほかの計算と同じ guard(締め切り・同時実行の上限)を通す。
	e.POST("/api/calc/adjust/indices", wrapper.AdjustIndices, guard)
	e.POST("/api/calc/adjust/min-sp-to-ko", wrapper.AdjustMinSpToKo, guard)
	e.POST("/api/calc/adjust/min-sp-to-survive", wrapper.AdjustMinSpToSurvive, guard)
	e.POST("/api/calc/adjust/allocation", wrapper.AdjustAllocation, guard)
	// 複数の目標(ADR-0177 §9)。
	e.POST("/api/calc/adjust/goals", wrapper.AdjustGoals, guard)
	// 連続技(ADR-0253)。ほかの計算と同じ guard を通す。
	e.POST("/api/calc/combo", wrapper.CalcCombo, guard)
}

// registerPokedexNotFoundRoutes は calc-svc の担当外(pokedex)の7操作を、生成ラッパを
// 経由させずに直接 404 not_found で応答する(critic 指摘 R1)。生成ラッパはヘッダの必須検証に
// 加えて q/limit/format などのクエリパラメータも解析するため、そこを経由させると
// ヘッダ欠落やクエリの型不一致(例 limit=abc)が missing_header / invalid_json 等に化けてしまい、
// 「担当外の操作は常に not_found」という契約に反する。
// echo v5.3.1 のルーターは静的セグメントをパラメータより優先するため、`/api/pokedex/moves/batch` は
// `/api/pokedex/moves/:key`(`key="batch"`)に食われない。恒久テストが固定しているのは「食われないこと」
// 自体(pokedex-svc 側の `TestGetMovesByIds`)で、登録順を入れ替えても同じ結果になることは調査時に
// 使い捨てテストで確認しただけ(恒久テストには含まれない)。calc-svc はどちらに食われても同じ 404 not_found
// を返すため、この順序非依存はここでは区別して検証できない。
func registerPokedexNotFoundRoutes(e *echo.Echo) {
	h := func(c *echo.Context) error { return notFoundForPokedex() }
	e.GET("/api/pokedex/species", h)
	e.GET("/api/pokedex/species/:key", h)
	e.GET("/api/pokedex/moves", h)
	e.GET("/api/pokedex/moves/batch", h)
	e.GET("/api/pokedex/moves/:key", h)
	e.GET("/api/pokedex/items", h)
	e.GET("/api/pokedex/natures", h)
}

func healthzHandler(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok", "version": version.Version})
}

// limitedBody はリクエスト本文を maxRequestBodyBytes に制限した Reader にする(critic 指摘 R7)。
// 上限を超えて読むと Read がエラーを返し、decodeStrict がそれを invalid_json に写す。
func limitedBody(ctx *echo.Context) io.Reader {
	return http.MaxBytesReader(ctx.Response(), ctx.Request().Body, maxRequestBodyBytes)
}

// recoverMiddleware は panic を回復し、500 internal の httpError にする(スタック等を出さない)。
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

// httpErrorHandler は echo に渡ったエラー(Server が返した httpError・生成ラッパのヘッダ検証・
// echo の既定 404/405 を含む)をすべて Error 形式に揃える。
func httpErrorHandler(c *echo.Context, err error) {
	if r, uerr := echo.UnwrapResponse(c.Response()); uerr == nil && r.Committed {
		return
	}
	status, body := errorBodyFor(err)
	var he *httpError
	if errors.As(err, &he) && he.retryAfter {
		c.Response().Header().Set("Retry-After", "1")
	}
	_ = c.JSON(status, body)
}

// duplicateHeaderMessage は生成ラッパ(ServerInterfaceWrapper)が同名ヘッダを複数個
// 受け取ったときに返す echo.HTTPError.Message の文言の断片(oapi-codegen が生成する固定の英文
// "Expected one value for X-Device-Id, got 2" 等)。ヘッダの「欠落」「空」(bind 失敗の
// "is empty, can't bind its value" を含む)とは別の失敗で、missing_header にはしない。
// gateway が同じ失敗を invalid_header にするため(ADR-0202 §9)、calc-svc も揃える。
const duplicateHeaderMessage = "Expected one value for"

func errorBodyFor(err error) (int, api.Error) {
	var he *httpError
	if errors.As(err, &he) {
		return he.status, api.Error{Code: he.code, Message: he.message}
	}
	// echo v5 の既定エラー(ErrNotFound / ErrMethodNotAllowed)は非公開型なので、
	// ステータスは HTTPStatusCoder から読む。
	var sc echo.HTTPStatusCoder
	if errors.As(err, &sc) {
		switch sc.StatusCode() {
		case http.StatusNotFound, http.StatusMethodNotAllowed:
			// ルートが無い・メソッドが違う(ADR-0200: メソッド違いに新しい code を足さず not_found にする)。
			return http.StatusNotFound, api.Error{Code: api.NotFound, Message: "ルートが無い"}
		case http.StatusBadRequest:
			// calc の3操作と調整の5操作だけが生成ラッパを経由する(pokedex は直接 not_found。R1)。
			// そのラッパが返す 400 はヘッダの検証由来。「欠落」「空」(bind 失敗も含む)は
			// missing_header、それ以外(同名ヘッダの重複指定)は invalid_header にする
			// (ADR-0202 §9: gateway と語彙を揃える。missing_header はヘッダ欠落・空に限定する)。
			var ee *echo.HTTPError
			if errors.As(err, &ee) && strings.Contains(ee.Message, duplicateHeaderMessage) {
				slog.Warn("calc-svc: ヘッダが重複している", "message", ee.Message)
				return http.StatusBadRequest, api.Error{Code: api.InvalidHeader, Message: "リクエストヘッダの指定が不正"}
			}
			return http.StatusBadRequest, api.Error{Code: api.MissingHeader, Message: "X-Device-Id / X-Session-Id が無い"}
		}
	}
	// 想定外の失敗は固定文だけをクライアントへ返し、詳細はログにだけ残す(critic 指摘 O4)。
	slog.Error("calc-svc: 想定外のエラー", "error", err)
	return http.StatusInternalServerError, api.Error{Code: api.Internal, Message: messageInternal}
}

// checkHeaders は X-Device-Id / X-Session-Id の欠落・空を missing_header にする。
// UUID 形式の検証はしない(gateway の仕事。ADR-0200 AC-6)。
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

// CalcDamage は POST /api/calc。手順(検証 → 解決 → 計算 → イベント発行)は app.Service.Calc が持つ。
func (s *Server) CalcDamage(ctx *echo.Context, params api.CalcDamageParams) error {
	if err := checkHeaders(params.XDeviceId, params.XSessionId); err != nil {
		return err
	}
	var req api.CalcRequest
	if err := decodeStrict(limitedBody(ctx), &req, "attacker", "defender"); err != nil {
		return err
	}
	res, err := s.app.Calc(ctx.Request().Context(), app.Meta{DeviceID: params.XDeviceId, SessionID: params.XSessionId}, calcRequestFrom(req))
	if err != nil {
		return httpErrorFrom(err)
	}
	return ctx.JSON(http.StatusOK, calcResultFrom(res))
}

// CalcBulk は POST /api/calc/bulk。件数上限(ADR-0208)は列挙の検証より前にここで見る。
func (s *Server) CalcBulk(ctx *echo.Context, params api.CalcBulkParams) error {
	if err := checkHeaders(params.XDeviceId, params.XSessionId); err != nil {
		return err
	}
	var req api.BulkCalcRequest
	if err := decodeStrict(limitedBody(ctx), &req, "attacker"); err != nil {
		return err
	}
	if err := checkBulkLimits(req); err != nil {
		return err
	}
	out, err := s.app.Bulk(ctx.Request().Context(), app.Meta{DeviceID: params.XDeviceId, SessionID: params.XSessionId}, bulkRequestFrom(req))
	if err != nil {
		return httpErrorFrom(err)
	}
	return ctx.JSON(http.StatusOK, bulkResultFrom(out))
}

// CalcReverse は POST /api/calc/reverse。件数上限(ADR-0208)は列挙の検証より前にここで見る。
func (s *Server) CalcReverse(ctx *echo.Context, params api.CalcReverseParams) error {
	if err := checkHeaders(params.XDeviceId, params.XSessionId); err != nil {
		return err
	}
	var req api.ReverseRequest
	if err := decodeStrict(limitedBody(ctx), &req, "known"); err != nil {
		return err
	}
	if err := checkReverseLimits(req); err != nil {
		return err
	}
	out, err := s.app.Reverse(ctx.Request().Context(), app.Meta{DeviceID: params.XDeviceId, SessionID: params.XSessionId}, reverseRequestFrom(req))
	if err != nil {
		return httpErrorFrom(err)
	}
	return ctx.JSON(http.StatusOK, reverseResultFrom(out))
}

// SearchItems は pokedex の操作。calc-svc の担当外なので 404 not_found。
func (s *Server) SearchItems(ctx *echo.Context, params api.SearchItemsParams) error {
	return notFoundForPokedex()
}

// SearchMoves は pokedex の操作。calc-svc の担当外なので 404 not_found。
func (s *Server) SearchMoves(ctx *echo.Context, params api.SearchMovesParams) error {
	return notFoundForPokedex()
}

// ListNatures は pokedex の操作。calc-svc の担当外なので 404 not_found。
func (s *Server) ListNatures(ctx *echo.Context, params api.ListNaturesParams) error {
	return notFoundForPokedex()
}

// SearchSpecies は pokedex の操作。calc-svc の担当外なので 404 not_found。
func (s *Server) SearchSpecies(ctx *echo.Context, params api.SearchSpeciesParams) error {
	return notFoundForPokedex()
}

// GetSpecies は pokedex の操作。calc-svc の担当外なので 404 not_found。
func (s *Server) GetSpecies(ctx *echo.Context, key api.SpeciesKey, params api.GetSpeciesParams) error {
	return notFoundForPokedex()
}

// GetMove は pokedex の操作。calc-svc の担当外なので 404 not_found。
func (s *Server) GetMove(ctx *echo.Context, key string, params api.GetMoveParams) error {
	return notFoundForPokedex()
}

// GetMovesByIds は pokedex の操作。calc-svc の担当外なので 404 not_found。
func (s *Server) GetMovesByIds(ctx *echo.Context, params api.GetMovesByIdsParams) error {
	return notFoundForPokedex()
}

// ListMoveLearners は pokedex の操作(ADR-0251)。calc-svc の担当外なので 404 not_found。
func (s *Server) ListMoveLearners(ctx *echo.Context, key string, params api.ListMoveLearnersParams) error {
	return notFoundForPokedex()
}
