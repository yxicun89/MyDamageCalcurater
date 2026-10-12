// Package httpapi は record-svc の HTTP 境界(ADR-0209 §5・§6)。生成物 api.ServerInterface を
// 実装するが、実際にルートへ登録するのは record の操作(GET listFrequentOpponents・お気に入りの3操作・
// DELETE deleteRecordDeviceData)だけで、他サービスの操作(calc・pokedex・internal)は
// api.ServerInterface を満たすためのスタブ(stubs.go)として 404 を返すだけ。
//
// 端末 ID はヘッダ(X-Device-Id)からだけ受け取る(ADR-0209 §2・§6)。ボディ・クエリに
// deviceId/device_id が来たら 400 unknown_field で拒否し、ヘッダの値を上書きさせない。
package httpapi

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/internal/httpmetrics"
	"example.com/pokecalc/services/record/internal/app"
)

// defaultFrequentLimit / minFrequentLimit / maxFrequentLimit は契約の limit の既定・範囲
// (api/openapi.yaml の listFrequentOpponents. schema: minimum 1, maximum 50, default 10)。
const (
	defaultFrequentLimit = 10
	minFrequentLimit     = 1
	maxFrequentLimit     = 50
)

// Server は api.ServerInterface を実装する。record の操作だけを本実装し(手順は app.Service)、他は stubs.go に置く。
// ここは生成型 ⇄ app の型の変換と、HTTP のエラー整形だけを持つ(ADR-0811・ADR-0254)。
type Server struct {
	svc *app.Service
	// calcEventsRetention は計算履歴の下限(これより古い行は返さない。ADR-0230 §4)。0 なら下限なし。app.New に渡す。
	calcEventsRetention time.Duration
}

// Option は NewHandler / NewServer の設定。
type Option func(*Server)

// WithCalcEventsRetention は計算履歴に返す行の保持期間を設定する(cmd/record が設定値を渡す。
// httpapi に既定の日数を持たせない。ADR-0211 §7)。
func WithCalcEventsRetention(d time.Duration) Option {
	return func(s *Server) { s.calcEventsRetention = d }
}

var _ api.ServerInterface = (*Server)(nil)

// NewServer は Store を使う Server を作る。
func NewServer(st app.Store, opts ...Option) *Server {
	s := &Server{}
	for _, o := range opts {
		o(s)
	}
	s.svc = app.New(st, contractEnums, time.Now, s.calcEventsRetention)
	return s
}

// NewHandler は record-svc の HTTP ハンドラ全体を組み立てる(fixture_test.go の申し送りどおり)。
func NewHandler(st app.Store, opts ...Option) http.Handler {
	e := echo.New()
	e.HTTPErrorHandler = httpErrorHandler
	m := httpmetrics.New()
	e.Use(m.Middleware())
	e.Use(recoverMiddleware)
	e.GET(httpmetrics.Path, m.Handler())

	srv := NewServer(st, opts...)
	registerRecordRoutes(e, srv)
	e.GET("/healthz", healthzHandler)
	e.GET("/readyz", readyzHandler(srv.svc))
	return e
}

// registerRecordRoutes は record の2操作を、生成ラッパ(api.ServerInterfaceWrapper。必須ヘッダの
// 有無を検証してから Server を呼ぶ)経由で登録する。calc / pokedex / internal はここに含めない
// (stubs.go が担当外として 404 を返す。calc-svc の registerPokedexNotFoundRoutes と同じ考え方)。
func registerRecordRoutes(e *echo.Echo, srv *Server) {
	wrapper := api.ServerInterfaceWrapper{Handler: srv}
	e.GET("/api/record/frequent-opponents", wrapper.ListFrequentOpponents)
	e.GET("/api/record/calc-history", wrapper.ListCalcHistory)
	e.DELETE("/api/record/device-data", wrapper.DeleteRecordDeviceData)
	e.GET("/api/record/favorites", wrapper.ListFavorites)
	e.POST("/api/record/favorites", wrapper.CreateFavorite)
	e.DELETE("/api/record/favorites/:favoriteId", wrapper.DeleteFavorite)
}

func healthzHandler(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// readyzHandler は DB に届けば 200、届かなければ 503 store_unavailable(ADR-0209 §5.3)。
// 到達の確かめ方(Ping か、無ければ FrequentOpponents)は app.Service.Ready。
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

// DeleteRecordDeviceData は DELETE /api/record/device-data(ADR-0209 §5)。
func (s *Server) DeleteRecordDeviceData(ctx *echo.Context, params api.DeleteRecordDeviceDataParams) error {
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
		return errFromStore(params.XDeviceId, err)
	}
	return ctx.JSON(http.StatusOK, deletionResultFrom(res))
}

// deletionResultFrom は app.PurgeResult を契約の RecordDeletionResult に写す。
func deletionResultFrom(res app.PurgeResult) api.RecordDeletionResult {
	status := api.Completed
	if res.Remaining {
		status = api.Partial
	}
	return api.RecordDeletionResult{
		Status:   status,
		PurgedAt: res.PurgedAt,
		Deleted: struct {
			Aggregates int `json:"aggregates"`
			CalcEvents int `json:"calcEvents"`
			Favorites  int `json:"favorites"`
		}{
			Aggregates: res.Deleted.Aggregates,
			CalcEvents: res.Deleted.CalcEvents,
			Favorites:  res.Deleted.Favorites,
		},
	}
}

// ListFrequentOpponents は GET /api/record/frequent-opponents(ADR-0209 §3 #2)。
func (s *Server) ListFrequentOpponents(ctx *echo.Context, params api.ListFrequentOpponentsParams) error {
	if err := checkHeaders(params.XDeviceId, params.XSessionId); err != nil {
		return err
	}
	if err := checkNoDeviceIDInQuery(ctx.QueryParams()); err != nil {
		return err
	}
	// critic 指摘 R-7: AC-D3 は「ボディ・クエリ」の両方が対象。GET でも本文に deviceId を
	// 入れて上書きを試みる要求を 400 unknown_field にする(契約に requestBody は無い)。
	if err := decodeNoBody(ctx.Request().Body); err != nil {
		return err
	}
	limit := defaultFrequentLimit
	if params.Limit != nil {
		limit = *params.Limit
	}
	if limit < minFrequentLimit || limit > maxFrequentLimit {
		return newError(api.InvalidInput, "limit は %d〜%d の範囲でなければならない(%d)", minFrequentLimit, maxFrequentLimit, limit)
	}

	rows, err := s.svc.FrequentOpponents(ctx.Request().Context(), params.XDeviceId, limit)
	if err != nil {
		return errFromStore(params.XDeviceId, err)
	}
	return ctx.JSON(http.StatusOK, frequentOpponentsFrom(rows))
}

// frequentOpponentsFrom は app.FrequentOpponent のスライスを契約の型に写す(空なら空配列にする。
// nil を返して JSON が `null` になるのを避ける。契約は「記録が無ければ空配列」)。
func frequentOpponentsFrom(rows []app.FrequentOpponent) []api.FrequentOpponent {
	out := make([]api.FrequentOpponent, 0, len(rows))
	for _, r := range rows {
		out = append(out, api.FrequentOpponent{
			SpeciesKey:       api.SpeciesKey(r.SpeciesKey),
			Score:            r.Score,
			Count:            r.Count,
			LastCalculatedAt: r.LastCalculatedAt,
		})
	}
	return out
}
