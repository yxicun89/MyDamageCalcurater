package item

import "context"

// MemoryRepository はメモリ上の Repository。HTTP 層のテストと、MySQL 実装と同じ契約テスト(itemtest)に使う。
// MySQL 実装と同じ規則(外部キー・UNIQUE・並び・不可分性)を守ること。
type MemoryRepository struct{}

var _ Repository = (*MemoryRepository)(nil)

// NewMemoryRepository は空の MemoryRepository を返す(seed は入れない)。
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{}
}

func (m *MemoryRepository) ListItems(ctx context.Context, genreID *int64) ([]Item, error) {
	panic("TODO: MemoryRepository.ListItems")
}

func (m *MemoryRepository) GetItem(ctx context.Context, id int64) (Item, error) {
	panic("TODO: MemoryRepository.GetItem")
}

func (m *MemoryRepository) CreateItem(ctx context.Context, in NewItem) (Item, error) {
	panic("TODO: MemoryRepository.CreateItem")
}

func (m *MemoryRepository) UpdateItem(ctx context.Context, id int64, p ItemPatch) (Item, error) {
	panic("TODO: MemoryRepository.UpdateItem")
}

func (m *MemoryRepository) DeleteItem(ctx context.Context, id int64) (string, error) {
	panic("TODO: MemoryRepository.DeleteItem")
}

func (m *MemoryRepository) ListGenres(ctx context.Context) ([]Genre, error) {
	panic("TODO: MemoryRepository.ListGenres")
}

func (m *MemoryRepository) CreateGenre(ctx context.Context, in NewGenre) (Genre, error) {
	panic("TODO: MemoryRepository.CreateGenre")
}

func (m *MemoryRepository) UpdateGenre(ctx context.Context, id int64, p GenrePatch) (Genre, error) {
	panic("TODO: MemoryRepository.UpdateGenre")
}

func (m *MemoryRepository) ListSites(ctx context.Context) ([]Site, error) {
	panic("TODO: MemoryRepository.ListSites")
}

func (m *MemoryRepository) CreateSite(ctx context.Context, in NewSite) (Site, error) {
	panic("TODO: MemoryRepository.CreateSite")
}

func (m *MemoryRepository) UpdateSite(ctx context.Context, id int64, p SitePatch) (Site, error) {
	panic("TODO: MemoryRepository.UpdateSite")
}
