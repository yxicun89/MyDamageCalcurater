package item

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/oapi-codegen/nullable"

	"example.com/pokecalc/apps/wishlist/api/internal/deeplink"
	"example.com/pokecalc/apps/wishlist/api/internal/query"
	"example.com/pokecalc/apps/wishlist/api/internal/storage"
)

// 文字数の上限(DB の列幅。文字数は rune で数える)。
const (
	MaxGenreNameLen     = 64
	MaxSiteNameLen      = 64
	MaxQueryTemplateLen = 255
	MaxSearchURLLen     = 1024
	MaxItemNameLen      = 255
	MaxOptionTextLen    = 255
	MaxQueryOverrideLen = 255
	MaxSourceURLLen     = 1024
	MaxSiteQueryLen     = 255
	// MaxAliasLen は表記揺れの辞書の 1 語の上限(genre_aliases.alias。フェーズ4-1)。
	MaxAliasLen = 64
	// MinAliasNormalizedLen は 1 語の正規化後の最小文字数。短い語は部分一致でほとんどのタイトルに当たるため(HG・MG・RG の 2 文字は通す)。
	MinAliasNormalizedLen = 2
)

// Service は入力の検査(ErrInvalid)、既定値の補完、画像の保存・削除を受け持ち、永続化は Repository に任せる。
// HTTP 層は Service だけを使う。
type Service struct {
	repo   Repository
	images storage.Storage
}

// NewService は Service を返す。
func NewService(repo Repository, images storage.Storage) *Service {
	return &Service{repo: repo, images: images}
}

// ListItems は Repository.ListItems と同じ。
func (s *Service) ListItems(ctx context.Context, genreID *int64) ([]Item, error) {
	return s.repo.ListItems(ctx, genreID)
}

// GetItem は Repository.GetItem と同じ。
func (s *Service) GetItem(ctx context.Context, id int64) (Item, error) {
	return s.repo.GetItem(ctx, id)
}

// CreateItem は image を保存してから商品を作る(in.ImagePath は無視して保存した名前にする)。
// 商品の作成に失敗したら保存した画像を消す。画像のエラーは storage のエラー(ErrUnsupportedImage・ErrTooLarge)を包む。
func (s *Service) CreateItem(ctx context.Context, in NewItem, image io.Reader) (Item, error) {
	if err := ValidateNewItem(in); err != nil {
		return Item{}, err
	}
	name, err := s.images.Save(ctx, image)
	if err != nil {
		return Item{}, err
	}
	in.ImagePath = name
	it, err := s.repo.CreateItem(ctx, in)
	if err != nil {
		s.discard(name)
		return Item{}, err
	}
	return it, nil
}

// UpdateItem は検査してから Repository.UpdateItem を呼ぶ(p.ImagePath は無視する。画像は ReplaceImage で変える)。
func (s *Service) UpdateItem(ctx context.Context, id int64, p ItemPatch) (Item, error) {
	p.ImagePath = nil
	if err := validateItemPatch(p); err != nil {
		return Item{}, err
	}
	return s.repo.UpdateItem(ctx, id, p)
}

// ReplaceImage は新しい画像を保存して商品を更新し、成功したら古い画像を消す。
// 商品が無い・更新に失敗したら新しい画像を消す(古い画像は残す)。
func (s *Service) ReplaceImage(ctx context.Context, id int64, image io.Reader) (Item, error) {
	old, err := s.repo.GetItem(ctx, id)
	if err != nil {
		return Item{}, err
	}
	name, err := s.images.Save(ctx, image)
	if err != nil {
		return Item{}, err
	}
	it, err := s.repo.UpdateItem(ctx, id, ItemPatch{ImagePath: &name})
	if err != nil {
		s.discard(name)
		return Item{}, err
	}
	s.discard(old.ImagePath)
	return it, nil
}

// DeleteItem は商品を消し、その画像も消す。
func (s *Service) DeleteItem(ctx context.Context, id int64) error {
	path, err := s.repo.DeleteItem(ctx, id)
	if err != nil {
		return err
	}
	s.discard(path)
	return nil
}

// ListGenres は Repository.ListGenres と同じ。
func (s *Service) ListGenres(ctx context.Context) ([]Genre, error) {
	return s.repo.ListGenres(ctx)
}

// CreateGenre は検査し、QueryTemplate が空なら DefaultQueryTemplate にして作る。
func (s *Service) CreateGenre(ctx context.Context, in NewGenre) (Genre, error) {
	if err := checkText("name", in.Name, MaxGenreNameLen, true); err != nil {
		return Genre{}, err
	}
	if strings.TrimSpace(in.QueryTemplate) == "" {
		in.QueryTemplate = DefaultQueryTemplate
	}
	if err := checkText("query_template", in.QueryTemplate, MaxQueryTemplateLen, true); err != nil {
		return Genre{}, err
	}
	if err := checkInt32("sort_order", in.SortOrder); err != nil {
		return Genre{}, err
	}
	aliases, err := checkAliases(in.Aliases)
	if err != nil {
		return Genre{}, err
	}
	in.Aliases = aliases
	return s.repo.CreateGenre(ctx, in)
}

// UpdateGenre は検査してから更新する。
func (s *Service) UpdateGenre(ctx context.Context, id int64, p GenrePatch) (Genre, error) {
	if err := checkOptText("name", p.Name, MaxGenreNameLen, true); err != nil {
		return Genre{}, err
	}
	if err := checkOptText("query_template", p.QueryTemplate, MaxQueryTemplateLen, true); err != nil {
		return Genre{}, err
	}
	if p.SortOrder != nil {
		if err := checkInt32("sort_order", *p.SortOrder); err != nil {
			return Genre{}, err
		}
	}
	if p.Aliases != nil {
		aliases, err := checkAliases(*p.Aliases)
		if err != nil {
			return Genre{}, err
		}
		p.Aliases = &aliases
	}
	return s.repo.UpdateGenre(ctx, id, p)
}

// ListSites は Repository.ListSites と同じ。
func (s *Service) ListSites(ctx context.Context) ([]Site, error) {
	return s.repo.ListSites(ctx)
}

// CreateSite は検査し(検索 URL テンプレートは deeplink.ValidateTemplate)、FetchType が空なら FetchLinkOnly にして作る。
func (s *Service) CreateSite(ctx context.Context, in NewSite) (Site, error) {
	if err := checkText("name", in.Name, MaxSiteNameLen, true); err != nil {
		return Site{}, err
	}
	if err := checkSearchURL(in.SearchURLTemplate); err != nil {
		return Site{}, err
	}
	if in.FetchType == "" {
		in.FetchType = FetchLinkOnly
	}
	if err := checkFetchType(in.FetchType); err != nil {
		return Site{}, err
	}
	return s.repo.CreateSite(ctx, in)
}

// UpdateSite は検査してから更新する。
func (s *Service) UpdateSite(ctx context.Context, id int64, p SitePatch) (Site, error) {
	if err := checkOptText("name", p.Name, MaxSiteNameLen, true); err != nil {
		return Site{}, err
	}
	if p.SearchURLTemplate != nil {
		if err := checkSearchURL(*p.SearchURLTemplate); err != nil {
			return Site{}, err
		}
	}
	if p.FetchType != nil {
		if err := checkFetchType(*p.FetchType); err != nil {
			return Site{}, err
		}
	}
	return s.repo.UpdateSite(ctx, id, p)
}

// discard は保存済みの画像を消す(後始末。失敗しても呼び出しの結果は変えない)。
func (s *Service) discard(name string) {
	_ = s.images.Delete(name)
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// checkAliases は表記揺れの辞書を検査し、各語の前後の空白(全角空白・タブを含む)を除いたコピーを返す。
func checkAliases(groups [][]string) ([][]string, error) {
	out := make([][]string, len(groups))
	for i, g := range groups {
		if len(g) < 2 {
			return nil, invalid("aliases[%d] must have at least 2 words", i)
		}
		out[i] = make([]string, len(g))
		for j, w := range g {
			w = strings.TrimFunc(w, unicode.IsSpace)
			if w == "" {
				return nil, invalid("aliases[%d][%d] must not be empty", i, j)
			}
			if utf8.RuneCountInString(w) > MaxAliasLen {
				return nil, invalid("aliases[%d][%d] must be at most %d characters", i, j, MaxAliasLen)
			}
			if utf8.RuneCountInString(query.Normalize(w)) < MinAliasNormalizedLen {
				return nil, invalid("aliases[%d][%d] must have at least %d characters after normalization", i, j, MinAliasNormalizedLen)
			}
			if strings.ContainsAny(w, ",，、") {
				return nil, invalid("aliases[%d][%d] must not contain a comma", i, j)
			}
			out[i][j] = w
		}
	}
	if err := checkAliasDuplicates(out); err != nil {
		return nil, err
	}
	return out, nil
}

// checkText は文字数(rune)の上限と、required なら空・空白だけでないことを検査する。
func checkText(field, v string, max int, required bool) error {
	if required && strings.TrimSpace(v) == "" {
		return invalid("%s must not be empty", field)
	}
	if utf8.RuneCountInString(v) > max {
		return invalid("%s must be at most %d characters", field, max)
	}
	return nil
}

func checkOptText(field string, v *string, max int, required bool) error {
	if v == nil {
		return nil
	}
	return checkText(field, *v, max, required)
}

func checkInt32(field string, v int) error {
	if v < math.MinInt32 || v > math.MaxInt32 {
		return invalid("%s is out of range", field)
	}
	return nil
}

func checkMinPrice(v int) error {
	if v < 0 || v > math.MaxInt32 {
		return invalid("min_price must be between 0 and %d", math.MaxInt32)
	}
	return nil
}

func checkSourceURL(v string) error {
	if err := checkText("source_url", v, MaxSourceURLLen, false); err != nil {
		return err
	}
	if v == "" {
		return nil
	}
	if u, err := url.Parse(v); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return invalid("source_url must be an http(s) URL")
	}
	return nil
}

func checkSearchURL(v string) error {
	if err := checkText("search_url_template", v, MaxSearchURLLen, true); err != nil {
		return err
	}
	if err := deeplink.ValidateTemplate(v); err != nil {
		return fmt.Errorf("%w: search_url_template: %w", ErrInvalid, err)
	}
	return nil
}

func checkFetchType(t FetchType) error {
	switch t {
	case FetchAPI, FetchScrape, FetchHeadless, FetchLinkOnly:
		return nil
	}
	return invalid("fetch_type is invalid: %q", t)
}

// ValidateNewItem は商品の作成値を検査する(ErrInvalid)。外部取得など高コストな処理の前に呼べるよう公開している。
func ValidateNewItem(in NewItem) error {
	if err := checkText("name", in.Name, MaxItemNameLen, true); err != nil {
		return err
	}
	if err := checkOptText("option_text", in.OptionText, MaxOptionTextLen, false); err != nil {
		return err
	}
	if err := checkOptText("query_override", in.QueryOverride, MaxQueryOverrideLen, false); err != nil {
		return err
	}
	if in.SourceURL != nil {
		if err := checkSourceURL(*in.SourceURL); err != nil {
			return err
		}
	}
	if in.MinPrice != nil {
		if err := checkMinPrice(*in.MinPrice); err != nil {
			return err
		}
	}
	return checkInt32("sort_order", in.SortOrder)
}

// nullableValue は nullable が値を持つとき(未指定でも null でもないとき)にその値を返す。
func nullableValue[T any](n nullable.Nullable[T]) (T, bool) {
	var zero T
	if !n.IsSpecified() || n.IsNull() {
		return zero, false
	}
	return n.MustGet(), true
}

func validateItemPatch(p ItemPatch) error {
	if err := checkOptText("name", p.Name, MaxItemNameLen, true); err != nil {
		return err
	}
	if v, ok := nullableValue(p.OptionText); ok {
		if err := checkText("option_text", v, MaxOptionTextLen, false); err != nil {
			return err
		}
	}
	if v, ok := nullableValue(p.QueryOverride); ok {
		if err := checkText("query_override", v, MaxQueryOverrideLen, false); err != nil {
			return err
		}
	}
	if v, ok := nullableValue(p.SourceURL); ok {
		if err := checkSourceURL(v); err != nil {
			return err
		}
	}
	if v, ok := nullableValue(p.MinPrice); ok {
		if err := checkMinPrice(v); err != nil {
			return err
		}
	}
	if p.SortOrder != nil {
		if err := checkInt32("sort_order", *p.SortOrder); err != nil {
			return err
		}
	}
	if p.SiteOverrides != nil {
		for _, o := range *p.SiteOverrides {
			if o.Query != nil {
				if err := checkText("site_overrides.query", *o.Query, MaxSiteQueryLen, false); err != nil {
					return err
				}
			}
		}
		if err := checkUniqueOverrides(*p.SiteOverrides); err != nil {
			return err
		}
	}
	return nil
}
