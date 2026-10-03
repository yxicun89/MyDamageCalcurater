// Package httpapi は wishlist API の HTTP 層。api/openapi.yaml から生成した StrictServerInterface を実装し、
// Echo を組み立てる(認証・エラー形式の統一・画像配信)。
package httpapi

import (
	"context"
	"log/slog"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/apps/wishlist/api/internal/api"
	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/ogp"
	"example.com/pokecalc/apps/wishlist/api/internal/refresh"
	"example.com/pokecalc/apps/wishlist/api/internal/storage"
)

// ImageCacheControl は /images/{name} に付ける Cache-Control(名前は保存のたびに変わり内容は不変)。
const ImageCacheControl = "public, max-age=31536000, immutable"

// Remote は外部 URL の取得(本番は ogp.Fetcher。テストでは差し替える)。
type Remote interface {
	// Draft は商品ページの OGP から下書きを作る。
	Draft(ctx context.Context, rawURL string) (ogp.Draft, error)
	// Image は画像を最大 max バイト取る。
	Image(ctx context.Context, rawURL string, max int64) ([]byte, error)
}

// Estimator は目安価格の読み出しと更新の起動(本番は *refresh.Service。docs/phase3-api-spec.md AC-H14・AC-H18〜H20)。
// 商品が無ければ item.ErrNotFound を返すこと。
type Estimator interface {
	// Estimates は GET /api/items/{id}/estimates(古ければ裏で更新を起動する)。
	Estimates(ctx context.Context, itemID int64) (refresh.View, error)
	// Refresh は POST /api/items/{id}/estimates/refresh(裏で全対象を取り直す。実行中なら起動しない)。
	Refresh(ctx context.Context, itemID int64) (refresh.View, error)
	// Listings は GET /api/items/{id}/listings。
	Listings(ctx context.Context, itemID int64, siteID *int64) ([]item.Listing, error)
}

var _ Estimator = (*refresh.Service)(nil)

// Deps は NewServer の依存。
type Deps struct {
	Items  *item.Service
	Images storage.Storage
	Remote Remote
	// Estimates は目安価格(必須。nil なら NewServer が panic する)。
	Estimates Estimator
	// Token は /api/* に要求する Bearer トークン(空は不可。比較は定数時間)。
	Token  string
	Logger *slog.Logger // nil なら slog.Default()
}

// NewServer は API の Echo を組み立てる。
//   - /api/* は Authorization: Bearer <Token> 必須(無い・違う → 401)。/healthz と /images/{name} は認証なし
//   - エラーはすべて Error スキーマ({"code","message"})。Echo の既定のエラー応答をそのまま出さない
//     (存在しないパス → 404 not_found、形式不正 → 400 bad_request、規則違反 → 422 unprocessable、
//     外部取得の失敗 → 502 bad_gateway、内部エラー → 500 internal。500 の message に内部の詳細を出さない)
func NewServer(d Deps) *echo.Echo {
	if d.Token == "" {
		panic("httpapi: Token is empty")
	}
	if d.Estimates == nil {
		panic("httpapi: Estimates is nil")
	}
	log := d.Logger
	if log == nil {
		log = slog.Default()
	}
	s := &server{items: d.Items, images: d.Images, remote: d.Remote, estimates: d.Estimates, log: log}

	e := echo.New()
	e.HTTPErrorHandler = s.handleError
	e.Use(s.recoverPanic, securityHeaders, limitBody, bearerAuth(d.Token))
	api.RegisterHandlers(e, api.NewStrictHandler(s, nil))
	return e
}
