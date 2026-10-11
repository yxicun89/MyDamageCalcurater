// Package fromurl は POST /api/items/from-url の手順(URL から商品の下書きを作る)。
// URL の検査 → 共有拡張が読んだ題名があればそれを使う / 取りに行かずに作れる下書き / ページの OGP を取得 →
// 名前の整形 → ジャンルの推定、の順に進む。HTTP の入出力の形とエラーの応答は adapters/httpapi が受け持つ。
package fromurl

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"example.com/pokecalc/apps/wishlist/api/internal/domain/item"
	"example.com/pokecalc/apps/wishlist/api/internal/domain/page"
)

// PageReader は商品ページから下書きを読む(ポート。実装は adapters/ogp の *Fetcher)。
type PageReader interface {
	Draft(ctx context.Context, rawURL string) (page.Draft, error)
}

// GenreLister はジャンルの一覧を返す(ポート。実装は app/items の *Service)。
type GenreLister interface {
	ListGenres(ctx context.Context) ([]item.Genre, error)
}

// Input は下書きを作る入力。GenreID・Title・ImageURL は無ければ nil。
// Title・ImageURL は共有拡張がページから読んだ値(あればページを取りに行かない)。
type Input struct {
	URL      string
	GenreID  *int64
	Title    *string
	ImageURL *string
}

// Output は作った下書き。ImageURL・GenreID は無ければ nil。GenreGuessed は GenreID を推定したか。
type Output struct {
	Name         string
	SourceURL    string
	ImageURL     *string
	GenreID      *int64
	GenreGuessed bool
}

// FetchError は PageReader の失敗(入力の URL の違反とは区別する)。Unwrap で元のエラーが取れる。
type FetchError struct{ Err error }

func (e *FetchError) Error() string { return fmt.Sprintf("fromurl: fetch page: %v", e.Err) }
func (e *FetchError) Unwrap() error { return e.Err }

// Service は from-url の手順。
type Service struct {
	pages  PageReader
	genres GenreLister
}

// New は Service を返す。
func New(pages PageReader, genres GenreLister) *Service {
	return &Service{pages: pages, genres: genres}
}

// Draft は in から下書きを作る。入力の URL の違反は page.ErrInvalidURL を包んで返し、
// ページの取得の失敗は *FetchError で返す。
func (s *Service) Draft(ctx context.Context, in Input) (Output, error) {
	u, err := page.ValidateURL(in.URL)
	if err != nil {
		return Output{}, err
	}
	host := u.Hostname()
	var name string
	var image *string
	switch static, isStatic := page.StaticDraft(u); {
	case in.Title != nil && strings.TrimSpace(*in.Title) != "":
		// 共有拡張がページから読んだ値。取りに行かない。
		name = page.CleanTitle(host, *in.Title)
		if in.ImageURL != nil {
			if _, err := page.ValidateURL(*in.ImageURL); err == nil {
				image = in.ImageURL
			}
		}
	case isStatic:
		name, image = static.Title, static.ImageURL
	default:
		d, err := s.pages.Draft(ctx, in.URL)
		if err != nil {
			return Output{}, &FetchError{Err: err}
		}
		name, image = page.CleanTitle(host, d.Title), d.ImageURL
	}
	out := Output{Name: name, SourceURL: in.URL, ImageURL: image, GenreID: in.GenreID}
	if in.GenreID == nil {
		genres, err := s.genres.ListGenres(ctx)
		if err != nil {
			return Output{}, err
		}
		if id, ok := item.GuessGenre(genres, name, host); ok {
			out.GenreID, out.GenreGuessed = &id, true
		}
	}
	return out, nil
}

// IsInvalidURL は err が入力の URL の違反か(取得の失敗は含めない)。
func IsInvalidURL(err error) bool {
	var fe *FetchError
	return !errors.As(err, &fe) && errors.Is(err, page.ErrInvalidURL)
}
