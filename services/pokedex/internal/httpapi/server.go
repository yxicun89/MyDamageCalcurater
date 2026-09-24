// Package httpapi は pokedex-svc の HTTP 境界(ADR-0105 §2・§3)。生成物 services/internal/api
// (API レーンの make gen の出力)の ServerInterface / ServerInterfaceWrapper をそのまま使う。
// 新しい生成物は作らない・生成型を手書きしない(ADR-0105 §1)。
package httpapi

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/pokedex/internal/store"
)

// Server は api.ServerInterface を実装する。DB へは store.Querier 経由でだけ触る。
type Server struct {
	q store.Querier
}

var _ api.ServerInterface = (*Server)(nil)

// NewServer は q を使う Server を作る。
func NewServer(q store.Querier) *Server {
	return &Server{q: q}
}

// NewHandler は pokedex-svc の HTTP ハンドラ全体を組み立てる。
// pokedex の8操作(検索7 + 内部 API 1。生成ラッパ経由)、calc の3操作(直接 404。calc-svc の R1 と対称)、
// GET /healthz(DB に触れない運用エンドポイント)、panic の回復(500 internal)、echo の既定エラー
// (ルート無し・メソッド違い)を Error 形式に揃えるエラーハンドラを含む。
// serve は起動時に DB へ接続しない(sql.Open だけ)。DB が無くても起動し、DB を使う操作が 503 を返す。
func NewHandler(q store.Querier) http.Handler {
	e := echo.New()
	e.Use(recoverMiddleware)
	e.HTTPErrorHandler = httpErrorHandler

	registerPokedexRoutes(e, NewServer(q))
	registerCalcNotFoundRoutes(e)
	e.GET("/healthz", healthzHandler)
	return e
}

// registerPokedexRoutes は pokedex-svc の担当(検索7操作 + 内部 API)だけを、生成ラッパ
// (api.ServerInterfaceWrapper。公開操作は必須ヘッダ X-Device-Id / X-Session-Id の有無を検証してから
// Server を呼ぶ。内部 API はヘッダを要求しない)経由で登録する。
// echo v5.3.1 のルーターは静的セグメントをパラメータより優先するため、`/api/pokedex/moves/batch` は
// `/api/pokedex/moves/:key`(`key="batch"`)に食われない。`TestGetMovesByIds` が固定しているのは
// 「食われないこと」自体(現在の登録順で)。登録順を入れ替えても同じ結果になることは調査時に
// 使い捨てテストで確認しただけで、恒久テストには含まれない。
func registerPokedexRoutes(e *echo.Echo, srv *Server) {
	wrapper := api.ServerInterfaceWrapper{Handler: srv}
	e.GET("/api/pokedex/species", wrapper.SearchSpecies)
	e.GET("/api/pokedex/species/:key", wrapper.GetSpecies)
	e.GET("/api/pokedex/moves", wrapper.SearchMoves)
	e.GET("/api/pokedex/moves/batch", wrapper.GetMovesByIds)
	e.GET("/api/pokedex/moves/:key", wrapper.GetMove)
	e.GET("/api/pokedex/items", wrapper.SearchItems)
	e.GET("/api/pokedex/natures", wrapper.ListNatures)
	e.GET("/internal/pokedex/master", wrapper.GetMasterExport)
}

// registerCalcNotFoundRoutes は pokedex-svc の担当外(calc)の3操作を、生成ラッパを経由させずに
// 直接 404 not_found で応答する(calc-svc の registerPokedexNotFoundRoutes と対称。ADR-0105 §1)。
// 生成ラッパを経由させるとヘッダの必須検証が先に走り、「担当外の操作は常に not_found」に反するため。
func registerCalcNotFoundRoutes(e *echo.Echo) {
	h := func(c *echo.Context) error { return notFoundForCalc() }
	e.POST("/api/calc", h)
	e.POST("/api/calc/bulk", h)
	e.POST("/api/calc/reverse", h)
}

// healthzHandler は GET /healthz。DB に触れず常に 200(liveness/readiness 共通。ADR-0105 §1)。
func healthzHandler(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
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

// --- calc-svc の担当(pokedex-svc の担当外)。生成ラッパ経由で呼ばれることは無いが、
// api.ServerInterface を満たすために実装する(未使用のまま not_found を返す)。

func (s *Server) CalcDamage(ctx *echo.Context, params api.CalcDamageParams) error {
	return notFoundForCalc()
}
func (s *Server) CalcBulk(ctx *echo.Context, params api.CalcBulkParams) error {
	return notFoundForCalc()
}
func (s *Server) CalcReverse(ctx *echo.Context, params api.CalcReverseParams) error {
	return notFoundForCalc()
}
