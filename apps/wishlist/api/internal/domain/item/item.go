// Package item は商品・ジャンル・サイトの型と、そのまわりの純粋な規則(apps/wishlist/CLAUDE.md §7・§8)。
// 永続化のポート(Repository など)と入力の検査・画像の保存を受け持つ Service は internal/app/items にある。
package item

import (
	"errors"
	"fmt"
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

// Invalid は ErrInvalid を包んだエラーを返す(メッセージは format から作る。入力の検査で使う)。
func Invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// DefaultQueryTemplate はジャンル作成時にテンプレートを省略したときの値(DB の既定値と同じ)。
const DefaultQueryTemplate = "{name} {option}"

// ジャンルの色と絵柄(フェーズ5 5-1b。docs/phase5-spec.md)。
const (
	// DefaultGenreColor はジャンル作成時に色を省略したときの値(強調色の青。DB の既定値と同じ)。
	DefaultGenreColor = "#1d5fd1"
	// DefaultGenreIcon はジャンル作成時に絵柄を省略したときの値(DB の既定値と同じ)。
	DefaultGenreIcon = IconBox
)

// GenreIcon はジャンルの絵柄(genres.icon)。
type GenreIcon string

const (
	IconCard   GenreIcon = "card"
	IconFigure GenreIcon = "figure"
	IconRobot  GenreIcon = "robot"
	IconGoods  GenreIcon = "goods"
	IconStar   GenreIcon = "star"
	IconBox    GenreIcon = "box"
)

// FetchType は価格の取得方式(仕様 §6)。
type FetchType string

const (
	FetchAPI      FetchType = "api"
	FetchScrape   FetchType = "scrape"
	FetchHeadless FetchType = "headless"
	FetchLinkOnly FetchType = "link_only"
)

// Genre はジャンル。SiteIDs は表示するサイト(表示順。genre_sites.sort_order 昇順)。
// Aliases は表記揺れの辞書(フェーズ4-1。docs/phase4-spec.md AC-A*)。グループの順・グループ内の語の順は保存した順。無ければ長さ 0。
//
// Color は `#rrggbb`(小文字)、Icon は絵柄。MatchHosts・MatchKeywords は URL からジャンルを推定する手がかり
// (フェーズ5 5-1b・5-1c。保存した順。無ければ長さ 0)。
type Genre struct {
	ID            int64
	Name          string
	QueryTemplate string
	SortOrder     int
	SiteIDs       []int64
	Aliases       [][]string
	Color         string
	Icon          GenreIcon
	MatchHosts    []string
	MatchKeywords []string
}

// NewGenre はジャンルの作成値。QueryTemplate が空なら Service が DefaultQueryTemplate にする。
// Color が空なら DefaultGenreColor、Icon が空なら DefaultGenreIcon(Service が補う)。Color は大文字も受け、小文字に揃える。
type NewGenre struct {
	Name          string
	QueryTemplate string
	SortOrder     int
	SiteIDs       []int64
	Aliases       [][]string
	Color         string
	Icon          GenreIcon
	MatchHosts    []string
	MatchKeywords []string
}

// GenrePatch は部分更新。nil の項目は変えない。SiteIDs・Aliases を渡すと全件置き換え(空スライスなら全部外す)。
// Color・Icon は nil なら変えない。MatchHosts・MatchKeywords は渡すと全件置き換え。
type GenrePatch struct {
	Name          *string
	QueryTemplate *string
	SortOrder     *int
	SiteIDs       *[]int64
	Aliases       *[][]string
	Color         *string
	Icon          *GenreIcon
	MatchHosts    *[]string
	MatchKeywords *[]string
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
	// WatchOfficial は公式ページ(SourceURL)の販売状況を夜間に監視するか(フェーズ4-3)。
	WatchOfficial bool
	// Official は保存した販売状況(official_status)。まだ無ければ nil。ListItems・GetItem・CreateItem・UpdateItem が埋める。
	Official *OfficialStatus
	// Summary は目安のまとめ(フェーズ5 5-1d。summary.go)。ListItems は必ず埋める(商品数に依らない回数のクエリで)。
	// それ以外(GetItem・CreateItem・UpdateItem)は nil でよい。
	Summary *Summary
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
	// WatchOfficial が true なら SourceURL が必要(Service が検査する。無ければ ErrInvalid)。
	WatchOfficial bool
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
	// WatchOfficial は nil なら変えない。更新後に WatchOfficial が true で SourceURL が無くなるなら ErrInvalid(Service が検査する)。
	// SourceURL を今と違う値・null にすると、Repository は保存済みの販売状況(Official)を消す。
	WatchOfficial *bool
}
