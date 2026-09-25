// Package httpapi は record-svc の HTTP 境界(ADR-0209 §5・§6)。生成物 api.ServerInterface を
// 実装するが、実際にルートへ登録するのは record の2操作(GET listFrequentOpponents・
// DELETE deleteRecordDeviceData)だけで、他サービスの操作(calc・pokedex・internal)は
// api.ServerInterface を満たすためのスタブ(stubs.go)として 404 を返すだけ。
//
// 端末 ID はヘッダ(X-Device-Id)からだけ受け取る(ADR-0209 §2・§6)。ボディ・クエリに
// deviceId/device_id が来たら 400 unknown_field で拒否し、ヘッダの値を上書きさせない。
package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/record/internal/store"
)

// defaultFrequentLimit / minFrequentLimit / maxFrequentLimit は契約の limit の既定・範囲
// (api/openapi.yaml の listFrequentOpponents. schema: minimum 1, maximum 50, default 10)。
const (
	defaultFrequentLimit = 10
	minFrequentLimit     = 1
	maxFrequentLimit     = 50
)

// Server は api.ServerInterface を実装する。record の2操作だけを本実装し、他は stubs.go に置く。
type Server struct {
	store store.Store
}

var _ api.ServerInterface = (*Server)(nil)

// NewServer は Store を使う Server を作る。
func NewServer(st store.Store) *Server {
	return &Server{store: st}
}

// NewHandler は record-svc の HTTP ハンドラ全体を組み立てる(fixture_test.go の申し送りどおり)。
func NewHandler(st store.Store) http.Handler {
	e := echo.New()
	e.HTTPErrorHandler = httpErrorHandler
	e.Use(recoverMiddleware)

	registerRecordRoutes(e, NewServer(st))
	e.GET("/healthz", healthzHandler)
	e.GET("/readyz", readyzHandler(st))
	return e
}

// registerRecordRoutes は record の2操作を、生成ラッパ(api.ServerInterfaceWrapper。必須ヘッダの
// 有無を検証してから Server を呼ぶ)経由で登録する。calc / pokedex / internal はここに含めない
// (stubs.go が担当外として 404 を返す。calc-svc の registerPokedexNotFoundRoutes と同じ考え方)。
func registerRecordRoutes(e *echo.Echo, srv *Server) {
	wrapper := api.ServerInterfaceWrapper{Handler: srv}
	e.GET("/api/record/frequent-opponents", wrapper.ListFrequentOpponents)
	e.DELETE("/api/record/device-data", wrapper.DeleteRecordDeviceData)
}

func healthzHandler(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// pinger は TiDBStore が持つ到達確認(Store インターフェースには含めない。store.go の docstring 参照)。
// fake はこれを実装しないので、テストは FrequentOpponents 経由のフォールバックを通る。
type pinger interface {
	Ping(ctx context.Context) error
}

// readyzHandler は DB に届けば 200、届かなければ 503 store_unavailable(ADR-0209 §5.3)。
func readyzHandler(st store.Store) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if err := checkStoreReady(c.Request().Context(), st); err != nil {
			return err
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
}

// checkStoreReady は st が pinger を実装していればそれを、していなければ読み取り専用の
// FrequentOpponents(空の deviceID。副作用が無い)を1件だけ呼んで到達可否を確かめる。
func checkStoreReady(ctx context.Context, st store.Store) error {
	if p, ok := st.(pinger); ok {
		if err := p.Ping(ctx); err != nil {
			return errFromStore("", err)
		}
		return nil
	}
	if _, err := st.FrequentOpponents(ctx, "", 1); err != nil {
		return errFromStore("", err)
	}
	return nil
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

// touchAndRun は「端末 ID を含む要求を受けたら devices.last_seen_at を更新する」(ADR-0209 §4)を
// 全操作で徹底したうえで run を呼ぶ。TouchDevice が失敗したら run は呼ばない(どちらも同じ
// store_unavailable になるだけなので、二重に呼んで待たせない)。
func touchAndRun(ctx context.Context, st store.Store, deviceID string, run func() error) error {
	if err := st.TouchDevice(ctx, deviceID, time.Now().UTC()); err != nil {
		return errFromStore(deviceID, err)
	}
	return run()
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

	var result api.RecordDeletionResult
	err := touchAndRun(ctx.Request().Context(), s.store, params.XDeviceId, func() error {
		res, err := s.store.PurgeDevice(ctx.Request().Context(), params.XDeviceId, time.Now().UTC())
		if err != nil {
			return errFromStore(params.XDeviceId, err)
		}
		result = deletionResultFrom(res)
		return nil
	})
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, result)
}

// deletionResultFrom は store.PurgeResult を契約の RecordDeletionResult に写す。
func deletionResultFrom(res store.PurgeResult) api.RecordDeletionResult {
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

	var out []api.FrequentOpponent
	err := touchAndRun(ctx.Request().Context(), s.store, params.XDeviceId, func() error {
		rows, err := s.store.FrequentOpponents(ctx.Request().Context(), params.XDeviceId, limit)
		if err != nil {
			return errFromStore(params.XDeviceId, err)
		}
		out = frequentOpponentsFrom(rows)
		return nil
	})
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, out)
}

// frequentOpponentsFrom は store.FrequentOpponent のスライスを契約の型に写す(空なら空配列にする。
// nil を返して JSON が `null` になるのを避ける。契約は「記録が無ければ空配列」)。
func frequentOpponentsFrom(rows []store.FrequentOpponent) []api.FrequentOpponent {
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
