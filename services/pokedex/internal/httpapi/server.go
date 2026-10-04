// Package httpapi は pokedex-svc の HTTP 境界(ADR-0105 §2・§3)。生成物 services/internal/api
// (API レーンの make gen の出力)の ServerInterface / ServerInterfaceWrapper をそのまま使う。
// 新しい生成物は作らない・生成型を手書きしない(ADR-0105 §1)。
package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/internal/httpguard"
	"example.com/pokecalc/services/internal/httpmetrics"
	"example.com/pokecalc/services/pokedex/internal/readtx"
)

// Server は api.ServerInterface を実装する。DB へは readtx.DB(store.Querier と読み取り専用 Tx の開始)経由でだけ触る。
type Server struct {
	q readtx.DB
}

var _ api.ServerInterface = (*Server)(nil)

// 締め切りの既定値(ADR-0129 §2・§3)。DefaultReadinessTimeout ≤ DefaultRequestTimeout < writeTimeout(15秒)。
const (
	DefaultRequestTimeout   = 5 * time.Second
	DefaultReadinessTimeout = 2 * time.Second
	// DefaultMaxInflight は DB を使う操作を同時に処理する数(issue #299・ADR-0801)。接続プール(ADR-0112)より
	// 大きく、超えた要求は DB を待たずに 503 upstream_unavailable + Retry-After。
	DefaultMaxInflight = 64
)

type config struct {
	requestTimeout   time.Duration
	readinessTimeout time.Duration
	maxInflight      int
}

// Option は NewHandler の設定(主にテストで短い締め切りを渡す)。本番は渡さない。
type Option func(*config)

// WithRequestTimeout は DB を使う操作の締め切りを変える。
func WithRequestTimeout(d time.Duration) Option { return func(c *config) { c.requestTimeout = d } }

// WithMaxInflight は DB を使う操作の同時実行の上限を変える。
func WithMaxInflight(n int) Option { return func(c *config) { c.maxInflight = n } }

// WithReadinessTimeout は /readyz の締め切りを変える。
func WithReadinessTimeout(d time.Duration) Option { return func(c *config) { c.readinessTimeout = d } }

// NewServer は q を使う Server を作る。
func NewServer(q readtx.DB) *Server {
	return &Server{q: q}
}

// NewHandler は pokedex-svc の HTTP ハンドラ全体を組み立てる。
// pokedex の9操作(検索8 + 内部 API 1。生成ラッパ経由)、calc の調整4操作を含む操作(直接 404。calc-svc の R1 と対称)、
// GET /healthz(liveness。DB に触れない)、GET /readyz(readiness。DB の最小条件を確かめる。ADR-0129)、
// DB を使うルートへの締め切りのミドルウェア(ADR-0129 §2)、panic の回復(500 internal)、echo の既定エラー
// (ルート無し・メソッド違い)を Error 形式に揃えるエラーハンドラを含む。
// serve は起動時に DB へ接続しない(sql.Open だけ)。DB が無くても起動し、DB を使う操作が 503 を返す。
func NewHandler(q readtx.DB, opts ...Option) http.Handler {
	cfg := config{requestTimeout: DefaultRequestTimeout, readinessTimeout: DefaultReadinessTimeout, maxInflight: DefaultMaxInflight}
	for _, o := range opts {
		o(&cfg)
	}
	e := echo.New()
	e.HTTPErrorHandler = httpErrorHandler
	m := httpmetrics.New()
	e.Use(m.Middleware())
	e.Use(recoverMiddleware)
	e.GET(httpmetrics.Path, m.Handler())

	e.GET("/healthz", healthzHandler)
	e.GET("/readyz", readyzHandler(q, cfg.readinessTimeout))

	// echo v5 の Group にミドルウェアを渡すと "" と "/*" に RouteNotFound が登録される。未登録パスは従来どおり
	// 404(エラーハンドラで Error 形式)になり、metrics の route ラベルは "/*" にまとまる(件数は有限)。
	g := e.Group("", deadlineMiddleware(cfg.requestTimeout),
		httpguard.Middleware(httpguard.Config{MaxInflight: cfg.maxInflight, Code: string(api.UpstreamUnavailable)}))
	registerPokedexRoutes(g, NewServer(q))
	registerCalcNotFoundRoutes(g)
	return e
}

// registerPokedexRoutes は pokedex-svc の担当(検索8操作 + 内部 API)だけを、生成ラッパ
// (api.ServerInterfaceWrapper。公開操作は必須ヘッダ X-Device-Id / X-Session-Id の有無を検証してから
// Server を呼ぶ。内部 API はヘッダを要求しない)経由で登録する。
// echo v5.3.1 のルーターは静的セグメントをパラメータより優先するため、`/api/pokedex/moves/batch` は
// `/api/pokedex/moves/:key`(`key="batch"`)に食われない。`TestGetMovesByIds` が固定しているのは
// 「食われないこと」自体(現在の登録順で)。登録順を入れ替えても同じ結果になることは調査時に
// 使い捨てテストで確認しただけで、恒久テストには含まれない。
func registerPokedexRoutes(e *echo.Group, srv *Server) {
	wrapper := api.ServerInterfaceWrapper{Handler: srv}
	e.GET("/api/pokedex/species", wrapper.SearchSpecies)
	e.GET("/api/pokedex/species/:key", wrapper.GetSpecies)
	e.GET("/api/pokedex/moves", wrapper.SearchMoves)
	e.GET("/api/pokedex/moves/batch", wrapper.GetMovesByIds)
	e.GET("/api/pokedex/moves/:key", wrapper.GetMove)
	e.GET("/api/pokedex/moves/:key/learners", wrapper.ListMoveLearners)
	e.GET("/api/pokedex/items", wrapper.SearchItems)
	e.GET("/api/pokedex/natures", wrapper.ListNatures)
	e.GET("/internal/pokedex/master", wrapper.GetMasterExport)
}

// registerCalcNotFoundRoutes は pokedex-svc の担当外(calc)の3操作を、生成ラッパを経由させずに
// 直接 404 not_found で応答する(calc-svc の registerPokedexNotFoundRoutes と対称。ADR-0105 §1)。
// 生成ラッパを経由させるとヘッダの必須検証が先に走り、「担当外の操作は常に not_found」に反するため。
func registerCalcNotFoundRoutes(e *echo.Group) {
	h := func(c *echo.Context) error { return notFoundForCalc() }
	e.POST("/api/calc", h)
	e.POST("/api/calc/bulk", h)
	e.POST("/api/calc/reverse", h)
}

// healthzHandler は GET /healthz。DB に触れず常に 200(liveness 専用。readiness は /readyz。ADR-0129)。
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

// 調整の4操作(calc-svc の担当。ADR-0250)。api.ServerInterface を満たすためだけに置く。

func (s *Server) AdjustIndices(ctx *echo.Context, params api.AdjustIndicesParams) error {
	return notFoundForCalc()
}

func (s *Server) AdjustMinSpToKo(ctx *echo.Context, params api.AdjustMinSpToKoParams) error {
	return notFoundForCalc()
}

func (s *Server) AdjustMinSpToSurvive(ctx *echo.Context, params api.AdjustMinSpToSurviveParams) error {
	return notFoundForCalc()
}

func (s *Server) AdjustAllocation(ctx *echo.Context, params api.AdjustAllocationParams) error {
	return notFoundForCalc()
}

func (s *Server) AdjustGoals(ctx *echo.Context, params api.AdjustGoalsParams) error {
	return notFoundForCalc()
}

// deadlineMiddleware は DB を使う操作の context に締め切りを掛ける(ADR-0129 §2)。
func deadlineMiddleware(d time.Duration) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			ctx, cancel := context.WithTimeout(c.Request().Context(), d)
			defer cancel()
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

// readyzHandler は GET /readyz(ADR-0129 §1)。最小条件の4クエリを読み取り専用 Tx の中で読み、
// Tx を開けない・失敗・空・既定レギュレーション無しなら 503 master_unavailable。キャッシュしない。
// Tx は必ず閉じる(開いたままの Tx を残さない)。
func readyzHandler(q readtx.DB, timeout time.Duration) echo.HandlerFunc {
	return func(c *echo.Context) error {
		ctx, cancel := context.WithTimeout(c.Request().Context(), timeout)
		defer cancel()
		tx, err := q.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			return unavailable("readyz: BeginTx", err)
		}
		defer func() { _ = tx.Rollback() }()
		if dv, err := tx.ListDataVersions(ctx); err != nil {
			return unavailable("readyz: data_versions", err)
		} else if len(dv) == 0 {
			return unavailable("readyz: data_versions が空", nil)
		}
		if ts, err := tx.ListTypes(ctx); err != nil {
			return unavailable("readyz: types", err)
		} else if len(ts) == 0 {
			return unavailable("readyz: types が空", nil)
		}
		if ns, err := tx.ListNatures(ctx); err != nil {
			return unavailable("readyz: natures", err)
		} else if len(ns) == 0 {
			return unavailable("readyz: natures が空", nil)
		}
		if _, err := tx.GetDefaultRegulation(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return unavailable("readyz: 既定のレギュレーションが無い", err)
			}
			return unavailable("readyz: regulation", err)
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
}
