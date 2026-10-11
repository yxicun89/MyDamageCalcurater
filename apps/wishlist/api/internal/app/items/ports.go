package items

import (
	"context"
	"io"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/domain/item"
)

// ImageStore は商品画像の保存先(ポート)。実装は adapters/storage(PVC 上のファイル)。
type ImageStore interface {
	// Save は画像を保存し、保存した名前を返す。
	Save(ctx context.Context, r io.Reader) (string, error)
	// Delete は保存済みの画像を消す。
	Delete(name string) error
}

// Repository は永続化。1 回の呼び出しは不可分(途中で失敗したら何も変えない)。
type Repository interface {
	// ListItems は商品の一覧(sort_order 昇順、同順は id 降順)。genreID が nil でなければそのジャンルだけ。
	ListItems(ctx context.Context, genreID *int64) ([]item.Item, error)
	GetItem(ctx context.Context, id int64) (item.Item, error)
	CreateItem(ctx context.Context, in item.NewItem) (item.Item, error)
	UpdateItem(ctx context.Context, id int64, p item.ItemPatch) (item.Item, error)
	// DeleteItem は商品を消し、消した商品の ImagePath を返す。
	DeleteItem(ctx context.Context, id int64) (imagePath string, err error)

	// ListGenres はジャンルの一覧(sort_order 昇順、同順は id 昇順)。
	ListGenres(ctx context.Context) ([]item.Genre, error)
	CreateGenre(ctx context.Context, in item.NewGenre) (item.Genre, error)
	UpdateGenre(ctx context.Context, id int64, p item.GenrePatch) (item.Genre, error)

	// ListSites はサイトの一覧(id 昇順)。
	ListSites(ctx context.Context) ([]item.Site, error)
	CreateSite(ctx context.Context, in item.NewSite) (item.Site, error)
	UpdateSite(ctx context.Context, id int64, p item.SitePatch) (item.Site, error)
}

// PriceRepository は目安価格(listings・estimates)の永続化(docs/phase3-api-spec.md AC-P*)。
// 時刻は秒未満を切り捨てて UTC で保存・返却する(MySQL の DATETIME に合わせる)。
type PriceRepository interface {
	// SaveSiteResult は 1 つのトランザクションで、(e.ItemID, e.SiteID) の listings をすべて消して ls を入れ、
	// estimates を e で upsert する。ls の ID・ItemID・SiteID・FetchedAt は無視し、e の値を使う。
	// 商品が無い → ErrNotFound、サイトが無い → ErrSiteNotFound(どちらも何も変えない)。
	SaveSiteResult(ctx context.Context, e item.Estimate, ls []item.Listing) error
	// MarkFailed は estimates の status だけを failed にする(low・mid・件数・fetched_at は前回の値を残す。listings も変えない)。
	// 行が無ければ status failed・low/mid null・件数 0・fetched_at = at の行を作る。
	// 商品が無い → ErrNotFound、サイトが無い → ErrSiteNotFound。
	MarkFailed(ctx context.Context, itemID, siteID int64, at time.Time) error
	// ListEstimates は商品の estimates(site_id 昇順)。商品が無ければ空(エラーにしない)。
	ListEstimates(ctx context.Context, itemID int64) ([]item.Estimate, error)
	// ListListings は商品の listings(price 昇順・同額は id 昇順)。siteID が nil でなければそのサイトだけ。
	// SuspiciousReasons は理由が無ければ長さ 0(nil でもよい)。商品・サイトが無ければ空。
	ListListings(ctx context.Context, itemID int64, siteID *int64) ([]item.Listing, error)
	// 価格の推移(フェーズ4-2。SaveSiteResult が書く。history.go)。
	PriceHistoryRepository
}

// OfficialRepository は販売状況の永続化。
type OfficialRepository interface {
	// SaveOfficialCheck は商品の状態を MergeOfficial(前回, c) で置き換え、保存した状態を返す(1 トランザクション)。
	// 商品が無い → ErrNotFound。c.State が 8 つのどれでもない・Evidence が MaxOfficialEvidence 個を超える・
	// 1 語が MaxOfficialEvidenceLen 文字を超える・空の語 → ErrInvalid(どれも何も変えない)。
	// 保存した状態は Repository の ListItems・GetItem の Item.Official に出る。
	SaveOfficialCheck(ctx context.Context, itemID int64, c item.OfficialCheck) (item.OfficialStatus, error)
}

// PriceHistoryRepository は価格の推移の永続化。PriceRepository に含める。
//
// 書き込みは SaveSiteResult が行う: e.Status が ok かつ e.Low があるときだけ、同じトランザクションで
// (e.ItemID, e.SiteID, HistoryDay(e.FetchedAt)) の行を low・mid・count・recorded_at(= 保存する fetched_at)で upsert する
// (同じ日は最後の値で上書き)。no_result・MarkFailed・入力の検査で弾いた保存は行を作らない・変えない。
type PriceHistoryRepository interface {
	// ListPriceHistory は商品の推移のうち Day >= since の行を返す(site_id 昇順・同じサイトは day 昇順)。
	// since は HistoryDay の形(00:00 UTC)で渡す。商品が無ければ空(エラーにしない)。
	ListPriceHistory(ctx context.Context, itemID int64, since time.Time) ([]item.PricePoint, error)
	// PrunePriceHistory は Day < before の行を全商品から消し、消した行数を返す。
	PrunePriceHistory(ctx context.Context, before time.Time) (int64, error)
}
