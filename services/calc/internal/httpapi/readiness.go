package httpapi

// マスタの準備状態(ADR-0204)。spec-writer のスタブ: 振る舞いは readiness_test.go が固定する。
// implementer が実装する(このファイルの中身は置き換えてよい。公開する名前と意味は ADR-0204 のとおり)。

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/calc/internal/master"
)

// StoreFunc は、マスタを読み込み済みならその Store を、まだなら nil を返す(並行に呼ばれても安全であること)。
// 型付きの nil(例 (*master.MemoryStore)(nil))を master.Store として返さないこと(nil 判定が効かなくなる)。
type StoreFunc func() master.Store

// NewDeferredHandler は、マスタを後から(バックグラウンドの取得で)用意する calc-svc の HTTP ハンドラを作る(ADR-0204 §3)。
// current が nil を返す間、calc の3操作は 503 master_unavailable、GET /readyz は 503 master_unavailable、
// GET /healthz は 200 を返す。current が Store を返すようになったら NewHandler と同じに振る舞う。
//
// TODO(ADR-0204): spec-writer のスタブ。implementer が実装する。
func NewDeferredHandler(current StoreFunc) http.Handler {
	return http.NotFoundHandler()
}

// GetMasterExport は GET /internal/pokedex/master(pokedex-svc の内部 API。ADR-0204)。calc-svc の担当外なので
// ルートに登録しない(生成物 api.ServerInterface を満たすためだけのメソッド)。呼ばれたら 404 not_found。
func (s *Server) GetMasterExport(ctx *echo.Context) error {
	return notFoundForPokedex()
}
