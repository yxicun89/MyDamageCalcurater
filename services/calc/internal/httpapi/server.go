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

	"example.com/pokecalc/services/calc/internal/master"
	"example.com/pokecalc/services/internal/api"
)

// errNotImplemented は P3-1 の implementer が置き換えるまでのスタブ用。
var errNotImplemented = errors.New("未実装(P3-1)")

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
	// TODO(P3-1): implementer が組み立てる。
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, errNotImplemented.Error(), http.StatusNotImplemented)
	})
}

// CalcDamage は POST /api/calc。
func (s *Server) CalcDamage(ctx echo.Context, params api.CalcDamageParams) error {
	return errNotImplemented
}

// CalcBulk は POST /api/calc/bulk。
func (s *Server) CalcBulk(ctx echo.Context, params api.CalcBulkParams) error {
	return errNotImplemented
}

// CalcReverse は POST /api/calc/reverse。
func (s *Server) CalcReverse(ctx echo.Context, params api.CalcReverseParams) error {
	return errNotImplemented
}

// SearchItems は pokedex の操作。calc-svc の担当外なので 404 not_found。
func (s *Server) SearchItems(ctx echo.Context, params api.SearchItemsParams) error {
	return errNotImplemented
}

// SearchMoves は pokedex の操作。calc-svc の担当外なので 404 not_found。
func (s *Server) SearchMoves(ctx echo.Context, params api.SearchMovesParams) error {
	return errNotImplemented
}

// ListNatures は pokedex の操作。calc-svc の担当外なので 404 not_found。
func (s *Server) ListNatures(ctx echo.Context, params api.ListNaturesParams) error {
	return errNotImplemented
}

// SearchSpecies は pokedex の操作。calc-svc の担当外なので 404 not_found。
func (s *Server) SearchSpecies(ctx echo.Context, params api.SearchSpeciesParams) error {
	return errNotImplemented
}

// GetSpecies は pokedex の操作。calc-svc の担当外なので 404 not_found。
func (s *Server) GetSpecies(ctx echo.Context, key api.SpeciesKey, params api.GetSpeciesParams) error {
	return errNotImplemented
}
