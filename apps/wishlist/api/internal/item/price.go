package item

import (
	"context"
	"fmt"
	"math"
	"time"
	"unicode/utf8"
)

// EstimateStatus は estimates.status。
type EstimateStatus string

const (
	EstimateOK       EstimateStatus = "ok"
	EstimateFailed   EstimateStatus = "failed"
	EstimateNoResult EstimateStatus = "no_result"
)

// 出品の列幅(listings。文字数は rune)。refresh が保存前に合わせる。
const (
	MaxListingTitleLen = 512
	MaxListingURLLen   = 1024
)

// Listing は保存した出品(listings)。SuspiciousReasons は参考外の理由(estimate.Reason の文字列。参考なら長さ 0)。
type Listing struct {
	ID                int64
	ItemID            int64
	SiteID            int64
	Title             string
	Price             int
	URL               string
	ImageURL          *string
	InStock           bool
	SuspiciousReasons []string
	FetchedAt         time.Time
}

// Estimate はサイトの目安(estimates)。
type Estimate struct {
	ItemID          int64
	SiteID          int64
	Low             *int
	Mid             *int
	Count           int
	SuspiciousCount int
	InStockCount    int
	Status          EstimateStatus
	FetchedAt       time.Time
}

// PriceRepository は目安価格(listings・estimates)の永続化(docs/phase3-api-spec.md AC-P*)。
// 時刻は秒未満を切り捨てて UTC で保存・返却する(MySQL の DATETIME に合わせる)。
type PriceRepository interface {
	// SaveSiteResult は 1 つのトランザクションで、(e.ItemID, e.SiteID) の listings をすべて消して ls を入れ、
	// estimates を e で upsert する。ls の ID・ItemID・SiteID・FetchedAt は無視し、e の値を使う。
	// 商品が無い → ErrNotFound、サイトが無い → ErrSiteNotFound(どちらも何も変えない)。
	SaveSiteResult(ctx context.Context, e Estimate, ls []Listing) error
	// MarkFailed は estimates の status だけを failed にする(low・mid・件数・fetched_at は前回の値を残す。listings も変えない)。
	// 行が無ければ status failed・low/mid null・件数 0・fetched_at = at の行を作る。
	// 商品が無い → ErrNotFound、サイトが無い → ErrSiteNotFound。
	MarkFailed(ctx context.Context, itemID, siteID int64, at time.Time) error
	// ListEstimates は商品の estimates(site_id 昇順)。商品が無ければ空(エラーにしない)。
	ListEstimates(ctx context.Context, itemID int64) ([]Estimate, error)
	// ListListings は商品の listings(price 昇順・同額は id 昇順)。siteID が nil でなければそのサイトだけ。
	// SuspiciousReasons は理由が無ければ長さ 0(nil でもよい)。商品・サイトが無ければ空。
	ListListings(ctx context.Context, itemID int64, siteID *int64) ([]Listing, error)
	// 価格の推移(フェーズ4-2。SaveSiteResult が書く。history.go)。
	PriceHistoryRepository
}

// validateSave は SaveSiteResult の入力が列幅・値の範囲に収まるかを検査する(違反は ErrInvalid)。
// 文字数は rune(MySQL の VARCHAR は文字数)。
func validateSave(e Estimate, ls []Listing) error {
	for _, p := range []*int{e.Low, e.Mid} {
		if p != nil && (*p < 0 || *p > math.MaxInt32) {
			return fmt.Errorf("%w: estimate price is out of range", ErrInvalid)
		}
	}
	if e.Count < 0 || e.SuspiciousCount < 0 || e.InStockCount < 0 || e.Count > math.MaxInt32 || e.SuspiciousCount > math.MaxInt32 || e.InStockCount > math.MaxInt32 {
		return fmt.Errorf("%w: estimate count is out of range", ErrInvalid)
	}
	switch e.Status {
	case EstimateOK, EstimateFailed, EstimateNoResult:
	default:
		return fmt.Errorf("%w: unknown estimate status", ErrInvalid)
	}
	for _, l := range ls {
		switch {
		case l.Price < 0 || l.Price > math.MaxInt32:
			return fmt.Errorf("%w: listing price is out of range", ErrInvalid)
		case utf8.RuneCountInString(l.Title) > MaxListingTitleLen:
			return fmt.Errorf("%w: listing title is too long", ErrInvalid)
		case utf8.RuneCountInString(l.URL) > MaxListingURLLen:
			return fmt.Errorf("%w: listing url is too long", ErrInvalid)
		case l.ImageURL != nil && utf8.RuneCountInString(*l.ImageURL) > MaxListingURLLen:
			return fmt.Errorf("%w: listing image_url is too long", ErrInvalid)
		}
	}
	return nil
}

// storedTime は保存・返却する時刻(MySQL の DATETIME に合わせ、秒未満を切り捨てて UTC にする)。
func storedTime(t time.Time) time.Time { return t.UTC().Truncate(time.Second) }
