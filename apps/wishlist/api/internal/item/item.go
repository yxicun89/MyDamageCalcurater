// Package item は商品・ジャンル・サイトの CRUD(apps/wishlist/CLAUDE.md §7・§8)。
// Repository が永続化(MySQL 実装とテスト用のメモリ実装)、Service が画像の保存・削除と入力の検査を受け持つ。
package item

import (
	"context"
	"errors"
	"time"

	"github.com/oapi-codegen/nullable"
)

var (
	// ErrNotFound はパスで指定した商品・ジャンル・サイトが無いこと(API では 404)。
	ErrNotFound = errors.New("item: not found")
	// ErrGenreNotFound は本文の genre_id が存在しないジャンルを指すこと(422)。
	ErrGenreNotFound = errors.New("item: genre does not exist")
	// ErrSiteNotFound は本文の site_ids・site_overrides が存在しないサイトを指すこと(422)。
	ErrSiteNotFound = errors.New("item: site does not exist")
	// ErrDuplicateName はジャンル名・サイト名の重複(422)。
	ErrDuplicateName = errors.New("item: duplicate name")
	// ErrInvalid は値が規則に合わないこと(422。空の名前・長すぎる値・負の min_price・不正な fetch_type・
	// 不正な検索 URL テンプレート・site_ids/site_overrides 内の site_id の重複など)。
	ErrInvalid = errors.New("item: invalid value")
)

// DefaultQueryTemplate はジャンル作成時にテンプレートを省略したときの値(DB の既定値と同じ)。
const DefaultQueryTemplate = "{name} {option}"

// FetchType は価格の取得方式(仕様 §6)。
type FetchType string

const (
	FetchAPI      FetchType = "api"
	FetchScrape   FetchType = "scrape"
	FetchHeadless FetchType = "headless"
	FetchLinkOnly FetchType = "link_only"
)

// Genre はジャンル。SiteIDs は表示するサイト(表示順。genre_sites.sort_order 昇順)。
type Genre struct {
	ID            int64
	Name          string
	QueryTemplate string
	SortOrder     int
	SiteIDs       []int64
}

// NewGenre はジャンルの作成値。QueryTemplate が空なら Service が DefaultQueryTemplate にする。
type NewGenre struct {
	Name          string
	QueryTemplate string
	SortOrder     int
	SiteIDs       []int64
}

// GenrePatch は部分更新。nil の項目は変えない。SiteIDs を渡すと全件置き換え(空スライスなら全部外す)。
type GenrePatch struct {
	Name          *string
	QueryTemplate *string
	SortOrder     *int
	SiteIDs       *[]int64
}

// Site はサイト。
type Site struct {
	ID                int64
	Name              string
	SearchURLTemplate string
	FetchType         FetchType
	IsReference       bool
}

// NewSite はサイトの作成値。FetchType が空なら Service が FetchLinkOnly にする。
type NewSite struct {
	Name              string
	SearchURLTemplate string
	FetchType         FetchType
	IsReference       bool
}

// SitePatch は部分更新。nil の項目は変えない。
type SitePatch struct {
	Name              *string
	SearchURLTemplate *string
	FetchType         *FetchType
	IsReference       *bool
}

// SiteOverride は商品ごとのサイト別設定(item_site_overrides)。
type SiteOverride struct {
	SiteID  int64
	Query   *string
	Enabled bool
}

// Item は商品。ImagePath は storage のファイル名。SiteOverrides は site_id 昇順。
type Item struct {
	ID            int64
	GenreID       int64
	Name          string
	OptionText    *string
	QueryOverride *string
	ImagePath     string
	SourceURL     *string
	MinPrice      *int
	SortOrder     int
	SiteOverrides []SiteOverride
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// NewItem は商品の作成値。
type NewItem struct {
	GenreID       int64
	Name          string
	OptionText    *string
	QueryOverride *string
	ImagePath     string
	SourceURL     *string
	MinPrice      *int
	SortOrder     int
}

// ItemPatch は部分更新(docs/design.md W-08)。
// ポインタの項目は nil なら変えない。nullable の項目は「未指定なら変えない・null なら消す・値なら設定」。
// SiteOverrides を渡すと全件置き換え(空スライスなら全部消す)。
type ItemPatch struct {
	GenreID       *int64
	Name          *string
	OptionText    nullable.Nullable[string]
	QueryOverride nullable.Nullable[string]
	SourceURL     nullable.Nullable[string]
	MinPrice      nullable.Nullable[int]
	SortOrder     *int
	SiteOverrides *[]SiteOverride
	ImagePath     *string
}

// Repository は永続化。1 回の呼び出しは不可分(途中で失敗したら何も変えない)。
type Repository interface {
	// ListItems は商品の一覧(sort_order 昇順、同順は id 降順)。genreID が nil でなければそのジャンルだけ。
	ListItems(ctx context.Context, genreID *int64) ([]Item, error)
	GetItem(ctx context.Context, id int64) (Item, error)
	CreateItem(ctx context.Context, in NewItem) (Item, error)
	UpdateItem(ctx context.Context, id int64, p ItemPatch) (Item, error)
	// DeleteItem は商品を消し、消した商品の ImagePath を返す。
	DeleteItem(ctx context.Context, id int64) (imagePath string, err error)

	// ListGenres はジャンルの一覧(sort_order 昇順、同順は id 昇順)。
	ListGenres(ctx context.Context) ([]Genre, error)
	CreateGenre(ctx context.Context, in NewGenre) (Genre, error)
	UpdateGenre(ctx context.Context, id int64, p GenrePatch) (Genre, error)

	// ListSites はサイトの一覧(id 昇順)。
	ListSites(ctx context.Context) ([]Site, error)
	CreateSite(ctx context.Context, in NewSite) (Site, error)
	UpdateSite(ctx context.Context, id int64, p SitePatch) (Site, error)
}
