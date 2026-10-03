package item

import (
	"context"
	"database/sql"
)

// MySQLRepository は MySQL(DB `wishlist`)の Repository。クエリは internal/store(sqlc 生成)を使う。
// 複数の文にまたがる更新(site_ids・site_overrides の置き換え)はトランザクションで不可分にする。
// 外部キー違反(1452)は ErrGenreNotFound / ErrSiteNotFound、UNIQUE 違反(1062)は ErrDuplicateName に変換する。
type MySQLRepository struct {
	db *sql.DB
}

var _ Repository = (*MySQLRepository)(nil)

// NewMySQLRepository は db(parseTime=true の DSN で開いたもの)を使う Repository を返す。
func NewMySQLRepository(db *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) ListItems(ctx context.Context, genreID *int64) ([]Item, error) {
	panic("TODO: MySQLRepository.ListItems")
}

func (r *MySQLRepository) GetItem(ctx context.Context, id int64) (Item, error) {
	panic("TODO: MySQLRepository.GetItem")
}

func (r *MySQLRepository) CreateItem(ctx context.Context, in NewItem) (Item, error) {
	panic("TODO: MySQLRepository.CreateItem")
}

func (r *MySQLRepository) UpdateItem(ctx context.Context, id int64, p ItemPatch) (Item, error) {
	panic("TODO: MySQLRepository.UpdateItem")
}

func (r *MySQLRepository) DeleteItem(ctx context.Context, id int64) (string, error) {
	panic("TODO: MySQLRepository.DeleteItem")
}

func (r *MySQLRepository) ListGenres(ctx context.Context) ([]Genre, error) {
	panic("TODO: MySQLRepository.ListGenres")
}

func (r *MySQLRepository) CreateGenre(ctx context.Context, in NewGenre) (Genre, error) {
	panic("TODO: MySQLRepository.CreateGenre")
}

func (r *MySQLRepository) UpdateGenre(ctx context.Context, id int64, p GenrePatch) (Genre, error) {
	panic("TODO: MySQLRepository.UpdateGenre")
}

func (r *MySQLRepository) ListSites(ctx context.Context) ([]Site, error) {
	panic("TODO: MySQLRepository.ListSites")
}

func (r *MySQLRepository) CreateSite(ctx context.Context, in NewSite) (Site, error) {
	panic("TODO: MySQLRepository.CreateSite")
}

func (r *MySQLRepository) UpdateSite(ctx context.Context, id int64, p SitePatch) (Site, error) {
	panic("TODO: MySQLRepository.UpdateSite")
}
