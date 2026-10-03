package item

import (
	"context"
	"io"

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
	panic("TODO: Service.ListItems")
}

// GetItem は Repository.GetItem と同じ。
func (s *Service) GetItem(ctx context.Context, id int64) (Item, error) {
	panic("TODO: Service.GetItem")
}

// CreateItem は image を保存してから商品を作る(in.ImagePath は無視して保存した名前にする)。
// 商品の作成に失敗したら保存した画像を消す。画像のエラーは storage のエラー(ErrUnsupportedImage・ErrTooLarge)を包む。
func (s *Service) CreateItem(ctx context.Context, in NewItem, image io.Reader) (Item, error) {
	panic("TODO: Service.CreateItem")
}

// UpdateItem は検査してから Repository.UpdateItem を呼ぶ(p.ImagePath は無視する。画像は ReplaceImage で変える)。
func (s *Service) UpdateItem(ctx context.Context, id int64, p ItemPatch) (Item, error) {
	panic("TODO: Service.UpdateItem")
}

// ReplaceImage は新しい画像を保存して商品を更新し、成功したら古い画像を消す。
// 商品が無い・更新に失敗したら新しい画像を消す(古い画像は残す)。
func (s *Service) ReplaceImage(ctx context.Context, id int64, image io.Reader) (Item, error) {
	panic("TODO: Service.ReplaceImage")
}

// DeleteItem は商品を消し、その画像も消す。
func (s *Service) DeleteItem(ctx context.Context, id int64) error {
	panic("TODO: Service.DeleteItem")
}

// ListGenres は Repository.ListGenres と同じ。
func (s *Service) ListGenres(ctx context.Context) ([]Genre, error) {
	panic("TODO: Service.ListGenres")
}

// CreateGenre は検査し、QueryTemplate が空なら DefaultQueryTemplate にして作る。
func (s *Service) CreateGenre(ctx context.Context, in NewGenre) (Genre, error) {
	panic("TODO: Service.CreateGenre")
}

// UpdateGenre は検査してから更新する。
func (s *Service) UpdateGenre(ctx context.Context, id int64, p GenrePatch) (Genre, error) {
	panic("TODO: Service.UpdateGenre")
}

// ListSites は Repository.ListSites と同じ。
func (s *Service) ListSites(ctx context.Context) ([]Site, error) {
	panic("TODO: Service.ListSites")
}

// CreateSite は検査し(検索 URL テンプレートは deeplink.ValidateTemplate)、FetchType が空なら FetchLinkOnly にして作る。
func (s *Service) CreateSite(ctx context.Context, in NewSite) (Site, error) {
	panic("TODO: Service.CreateSite")
}

// UpdateSite は検査してから更新する。
func (s *Service) UpdateSite(ctx context.Context, id int64, p SitePatch) (Site, error) {
	panic("TODO: Service.UpdateSite")
}
