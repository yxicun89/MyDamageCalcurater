package item

import (
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

// ValidateSave は SaveSiteResult の入力が列幅・値の範囲に収まるかを検査する(違反は ErrInvalid)。
// 文字数は rune(MySQL の VARCHAR は文字数)。
func ValidateSave(e Estimate, ls []Listing) error {
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

// StoredTime は保存・返却する時刻(MySQL の DATETIME に合わせ、秒未満を切り捨てて UTC にする)。
func StoredTime(t time.Time) time.Time { return t.UTC().Truncate(time.Second) }

// FetchedListing はサイトの検索で取得した 1 件の出品(仕様 §6。保存前の形。保存したものは Listing)。
type FetchedListing struct {
	Title    string
	Price    int // 円。送料は含めない
	URL      string
	ImageURL string // 無ければ空
	InStock  bool
}
