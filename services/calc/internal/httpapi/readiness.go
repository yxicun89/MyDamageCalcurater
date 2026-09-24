package httpapi

// マスタの準備状態(ADR-0204)。マスタを後から(バックグラウンドの取得で)用意する calc-svc の
// HTTP ハンドラを組み立てる。1リクエストごとに current() を呼んで、その時点の Store で答える
// (ハンドラを作り直さず、取得が終わった時点で自然に切り替わる)。

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/calc/internal/master"
	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/internal/httpmetrics"
)

// StoreFunc は、マスタを読み込み済みならその Store を、まだなら nil を返す(並行に呼ばれても安全であること)。
// 型付きの nil(例 (*master.MemoryStore)(nil))を master.Store として返さないこと(nil 判定が効かなくなる)。
type StoreFunc func() master.Store

// NewDeferredHandler は、マスタを後から(バックグラウンドの取得で)用意する calc-svc の HTTP ハンドラを作る(ADR-0204 §3)。
// current が nil を返す間、calc の3操作は 503 master_unavailable、GET /readyz は 503 master_unavailable、
// GET /healthz は 200 を返す。current が Store を返すようになったら NewHandler と同じに振る舞う。
func NewDeferredHandler(current StoreFunc) http.Handler {
	e := echo.New()
	e.HTTPErrorHandler = httpErrorHandler
	m := httpmetrics.New()
	e.Use(m.Middleware())
	e.Use(recoverMiddleware)
	e.GET(httpmetrics.Path, m.Handler())

	registerDeferredCalcRoutes(e, current)
	registerPokedexNotFoundRoutes(e)
	e.GET("/healthz", healthzHandler)
	e.GET("/readyz", func(c *echo.Context) error {
		if current() == nil {
			return errMasterUnavailable()
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	return e
}

// registerDeferredCalcRoutes は calc の3操作を、その時点の current() の結果に応じて 503 か
// 生成ラッパ(api.ServerInterfaceWrapper)経由の本来の処理に振り分ける。
func registerDeferredCalcRoutes(e *echo.Echo, current StoreFunc) {
	e.POST("/api/calc", deferredCalcHandler(current, func(w *api.ServerInterfaceWrapper) echo.HandlerFunc { return w.CalcDamage }))
	e.POST("/api/calc/bulk", deferredCalcHandler(current, func(w *api.ServerInterfaceWrapper) echo.HandlerFunc { return w.CalcBulk }))
	e.POST("/api/calc/reverse", deferredCalcHandler(current, func(w *api.ServerInterfaceWrapper) echo.HandlerFunc { return w.CalcReverse }))
}

// deferredCalcHandler は、current() が nil の間は 503 master_unavailable、Store が用意できたら
// pick で選んだ生成ラッパのメソッドへ委ねる echo.HandlerFunc を作る。
func deferredCalcHandler(current StoreFunc, pick func(*api.ServerInterfaceWrapper) echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		store := current()
		if store == nil {
			return errMasterUnavailable()
		}
		wrapper := &api.ServerInterfaceWrapper{Handler: NewServer(store)}
		return pick(wrapper)(c)
	}
}

// errMasterUnavailable は契約どおりの 503 master_unavailable(Error 形式)。
func errMasterUnavailable() error {
	return newError(api.MasterUnavailable, "マスタを取得できていない")
}

// GetMasterExport は GET /internal/pokedex/master(pokedex-svc の内部 API。ADR-0204)。calc-svc の担当外なので
// ルートに登録しない(生成物 api.ServerInterface を満たすためだけのメソッド)。呼ばれたら 404 not_found。
func (s *Server) GetMasterExport(ctx *echo.Context) error {
	return notFoundForPokedex()
}
