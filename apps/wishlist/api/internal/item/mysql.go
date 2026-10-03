package item

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"

	"github.com/go-sql-driver/mysql"

	"example.com/pokecalc/apps/wishlist/api/internal/store"
)

// MySQL のエラー番号。
const (
	mysqlDuplicateEntry  = 1062 // UNIQUE 違反
	mysqlForeignKeyNoRef = 1452 // 外部キー違反(参照先が無い)
)

// MySQLRepository は MySQL(DB `wishlist`)の Repository。クエリは internal/store(sqlc 生成)を使う。
// 複数の文にまたがる更新(site_ids・site_overrides の置き換え)はトランザクションで不可分にする。
// 外部キー違反(1452)は ErrGenreNotFound / ErrSiteNotFound、UNIQUE 違反(1062)は ErrDuplicateName に変換する。
type MySQLRepository struct {
	db *sql.DB
	q  *store.Queries
}

var _ Repository = (*MySQLRepository)(nil)

// NewMySQLRepository は db(parseTime=true の DSN で開いたもの)を使う Repository を返す。
func NewMySQLRepository(db *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: db, q: store.New(db)}
}

// mysqlErr は err が MySQL のエラー番号 num かを返す。
func mysqlErr(err error, num uint16) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == num
}

// inTx は fn をトランザクションで実行する(fn がエラーなら巻き戻す)。
func (r *MySQLRepository) inTx(ctx context.Context, fn func(q *store.Queries) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(r.q.WithTx(tx)); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func nullStr(p *string) sql.NullString {
	if p == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *p, Valid: true}
}

func strPtr(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	s := n.String
	return &s
}

func toInt32(v int) (int32, error) {
	if v < math.MinInt32 || v > math.MaxInt32 {
		return 0, fmt.Errorf("%w: number is out of range", ErrInvalid)
	}
	return int32(v), nil
}

func nullInt32(p *int) (sql.NullInt32, error) {
	if p == nil {
		return sql.NullInt32{}, nil
	}
	v, err := toInt32(*p)
	if err != nil {
		return sql.NullInt32{}, err
	}
	return sql.NullInt32{Int32: v, Valid: true}, nil
}

func intPtr(n sql.NullInt32) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int32)
	return &v
}

func toItem(row store.Item, overrides []store.ItemSiteOverride) Item {
	it := Item{
		ID: row.ID, GenreID: row.GenreID, Name: row.Name, OptionText: strPtr(row.OptionText), QueryOverride: strPtr(row.QueryOverride),
		ImagePath: row.ImagePath, SourceURL: strPtr(row.SourceUrl), MinPrice: intPtr(row.MinPrice), SortOrder: int(row.SortOrder),
		SiteOverrides: make([]SiteOverride, 0, len(overrides)), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	for _, o := range overrides {
		it.SiteOverrides = append(it.SiteOverrides, SiteOverride{SiteID: o.SiteID, Query: strPtr(o.Query), Enabled: o.Enabled})
	}
	return it
}

func toSite(row store.Site) Site {
	return Site{ID: row.ID, Name: row.Name, SearchURLTemplate: row.SearchUrlTemplate, FetchType: FetchType(row.FetchType), IsReference: row.IsReference}
}

func (r *MySQLRepository) ListItems(ctx context.Context, genreID *int64) ([]Item, error) {
	var rows []store.Item
	var err error
	if genreID == nil {
		rows, err = r.q.ListItems(ctx)
	} else {
		rows, err = r.q.ListItemsByGenre(ctx, *genreID)
	}
	if err != nil {
		return nil, err
	}
	all, err := r.q.ListItemSiteOverrides(ctx)
	if err != nil {
		return nil, err
	}
	byItem := map[int64][]store.ItemSiteOverride{}
	for _, o := range all {
		byItem[o.ItemID] = append(byItem[o.ItemID], o)
	}
	out := make([]Item, 0, len(rows))
	for _, row := range rows {
		out = append(out, toItem(row, byItem[row.ID]))
	}
	return out, nil
}

func (r *MySQLRepository) getItem(ctx context.Context, q *store.Queries, id int64) (Item, error) {
	row, err := q.GetItem(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	if err != nil {
		return Item{}, err
	}
	os, err := q.ListItemSiteOverridesByItem(ctx, id)
	if err != nil {
		return Item{}, err
	}
	return toItem(row, os), nil
}

func (r *MySQLRepository) GetItem(ctx context.Context, id int64) (Item, error) {
	return r.getItem(ctx, r.q, id)
}

func (r *MySQLRepository) CreateItem(ctx context.Context, in NewItem) (Item, error) {
	sortOrder, err := toInt32(in.SortOrder)
	if err != nil {
		return Item{}, err
	}
	minPrice, err := nullInt32(in.MinPrice)
	if err != nil {
		return Item{}, err
	}
	id, err := r.q.CreateItem(ctx, store.CreateItemParams{
		GenreID: in.GenreID, Name: in.Name, OptionText: nullStr(in.OptionText), QueryOverride: nullStr(in.QueryOverride),
		ImagePath: in.ImagePath, SourceUrl: nullStr(in.SourceURL), MinPrice: minPrice, SortOrder: sortOrder,
	})
	if mysqlErr(err, mysqlForeignKeyNoRef) {
		return Item{}, ErrGenreNotFound
	}
	if err != nil {
		return Item{}, err
	}
	return r.GetItem(ctx, id)
}

func (r *MySQLRepository) UpdateItem(ctx context.Context, id int64, p ItemPatch) (Item, error) {
	if p.SiteOverrides != nil {
		if err := checkUniqueOverrides(*p.SiteOverrides); err != nil {
			return Item{}, err
		}
	}
	var out Item
	err := r.inTx(ctx, func(q *store.Queries) error {
		cur, err := q.GetItemForUpdate(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		arg := store.UpdateItemParams{
			ID: id, GenreID: cur.GenreID, Name: cur.Name, OptionText: cur.OptionText, QueryOverride: cur.QueryOverride,
			ImagePath: cur.ImagePath, SourceUrl: cur.SourceUrl, MinPrice: cur.MinPrice, SortOrder: cur.SortOrder,
		}
		if p.GenreID != nil {
			arg.GenreID = *p.GenreID
		}
		if p.Name != nil {
			arg.Name = *p.Name
		}
		if p.SortOrder != nil {
			if arg.SortOrder, err = toInt32(*p.SortOrder); err != nil {
				return err
			}
		}
		if p.ImagePath != nil {
			arg.ImagePath = *p.ImagePath
		}
		arg.OptionText = patchString(arg.OptionText, p.OptionText)
		arg.QueryOverride = patchString(arg.QueryOverride, p.QueryOverride)
		arg.SourceUrl = patchString(arg.SourceUrl, p.SourceURL)
		if p.MinPrice.IsSpecified() {
			if p.MinPrice.IsNull() {
				arg.MinPrice = sql.NullInt32{}
			} else {
				v := p.MinPrice.MustGet()
				if arg.MinPrice, err = nullInt32(&v); err != nil {
					return err
				}
			}
		}
		if err := q.UpdateItem(ctx, arg); err != nil {
			if mysqlErr(err, mysqlForeignKeyNoRef) {
				return ErrGenreNotFound
			}
			return err
		}
		if p.SiteOverrides != nil {
			if err := q.DeleteItemSiteOverrides(ctx, id); err != nil {
				return err
			}
			for _, o := range *p.SiteOverrides {
				err := q.InsertItemSiteOverride(ctx, store.InsertItemSiteOverrideParams{ItemID: id, SiteID: o.SiteID, Query: nullStr(o.Query), Enabled: o.Enabled})
				if mysqlErr(err, mysqlForeignKeyNoRef) {
					return ErrSiteNotFound
				}
				if err != nil {
					return err
				}
			}
		}
		if err := q.TouchItem(ctx, id); err != nil {
			return err
		}
		out, err = r.getItem(ctx, q, id)
		return err
	})
	if err != nil {
		return Item{}, err
	}
	return out, nil
}

func patchString(cur sql.NullString, n interface {
	IsSpecified() bool
	IsNull() bool
	MustGet() string
}) sql.NullString {
	if !n.IsSpecified() {
		return cur
	}
	if n.IsNull() {
		return sql.NullString{}
	}
	return sql.NullString{String: n.MustGet(), Valid: true}
}

func (r *MySQLRepository) DeleteItem(ctx context.Context, id int64) (string, error) {
	var path string
	err := r.inTx(ctx, func(q *store.Queries) error {
		row, err := q.GetItemForUpdate(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		path = row.ImagePath
		_, err = q.DeleteItem(ctx, id)
		return err
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

func (r *MySQLRepository) ListGenres(ctx context.Context) ([]Genre, error) {
	rows, err := r.q.ListGenres(ctx)
	if err != nil {
		return nil, err
	}
	links, err := r.q.ListGenreSites(ctx)
	if err != nil {
		return nil, err
	}
	byGenre := map[int64][]int64{}
	for _, l := range links { // genre_id, sort_order, site_id 順
		byGenre[l.GenreID] = append(byGenre[l.GenreID], l.SiteID)
	}
	out := make([]Genre, 0, len(rows))
	for _, g := range rows {
		ids := byGenre[g.ID]
		if ids == nil {
			ids = []int64{}
		}
		out = append(out, Genre{ID: g.ID, Name: g.Name, QueryTemplate: g.QueryTemplate, SortOrder: int(g.SortOrder), SiteIDs: ids})
	}
	return out, nil
}

// getGenre はジャンルとその site_ids を読む。forUpdate なら行をロックする(更新の直前に読むとき)。
func (r *MySQLRepository) getGenre(ctx context.Context, q *store.Queries, id int64, forUpdate bool) (Genre, error) {
	var g store.Genre
	var err error
	if forUpdate {
		g, err = q.GetGenreForUpdate(ctx, id)
	} else {
		g, err = q.GetGenre(ctx, id)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return Genre{}, ErrNotFound
	}
	if err != nil {
		return Genre{}, err
	}
	links, err := q.ListGenreSitesByGenre(ctx, id)
	if err != nil {
		return Genre{}, err
	}
	ids := make([]int64, 0, len(links))
	for _, l := range links {
		ids = append(ids, l.SiteID)
	}
	return Genre{ID: g.ID, Name: g.Name, QueryTemplate: g.QueryTemplate, SortOrder: int(g.SortOrder), SiteIDs: ids}, nil
}

// replaceGenreSites は genre_sites を ids(指定順)で置き換える。
func replaceGenreSites(ctx context.Context, q *store.Queries, genreID int64, ids []int64) error {
	if err := q.DeleteGenreSites(ctx, genreID); err != nil {
		return err
	}
	for i, sid := range ids {
		err := q.InsertGenreSite(ctx, store.InsertGenreSiteParams{GenreID: genreID, SiteID: sid, SortOrder: int32(i)})
		if mysqlErr(err, mysqlForeignKeyNoRef) {
			return ErrSiteNotFound
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *MySQLRepository) CreateGenre(ctx context.Context, in NewGenre) (Genre, error) {
	if err := checkUniqueSiteIDs(in.SiteIDs); err != nil {
		return Genre{}, err
	}
	sortOrder, err := toInt32(in.SortOrder)
	if err != nil {
		return Genre{}, err
	}
	var out Genre
	err = r.inTx(ctx, func(q *store.Queries) error {
		id, err := q.CreateGenre(ctx, store.CreateGenreParams{Name: in.Name, QueryTemplate: in.QueryTemplate, SortOrder: sortOrder})
		if mysqlErr(err, mysqlDuplicateEntry) {
			return ErrDuplicateName
		}
		if err != nil {
			return err
		}
		if err := replaceGenreSites(ctx, q, id, in.SiteIDs); err != nil {
			return err
		}
		out, err = r.getGenre(ctx, q, id, false)
		return err
	})
	if err != nil {
		return Genre{}, err
	}
	return out, nil
}

func (r *MySQLRepository) UpdateGenre(ctx context.Context, id int64, p GenrePatch) (Genre, error) {
	if p.SiteIDs != nil {
		if err := checkUniqueSiteIDs(*p.SiteIDs); err != nil {
			return Genre{}, err
		}
	}
	var out Genre
	err := r.inTx(ctx, func(q *store.Queries) error {
		cur, err := r.getGenre(ctx, q, id, true)
		if err != nil {
			return err
		}
		arg := store.UpdateGenreParams{ID: id, Name: cur.Name, QueryTemplate: cur.QueryTemplate}
		arg.SortOrder = int32(cur.SortOrder) // 取得した値なので範囲内
		if p.Name != nil {
			arg.Name = *p.Name
		}
		if p.QueryTemplate != nil {
			arg.QueryTemplate = *p.QueryTemplate
		}
		if p.SortOrder != nil {
			if arg.SortOrder, err = toInt32(*p.SortOrder); err != nil {
				return err
			}
		}
		if _, err := q.UpdateGenre(ctx, arg); err != nil {
			if mysqlErr(err, mysqlDuplicateEntry) {
				return ErrDuplicateName
			}
			return err
		}
		if p.SiteIDs != nil {
			if err := replaceGenreSites(ctx, q, id, *p.SiteIDs); err != nil {
				return err
			}
		}
		out, err = r.getGenre(ctx, q, id, false)
		return err
	})
	if err != nil {
		return Genre{}, err
	}
	return out, nil
}

func (r *MySQLRepository) ListSites(ctx context.Context) ([]Site, error) {
	rows, err := r.q.ListSites(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Site, 0, len(rows))
	for _, s := range rows {
		out = append(out, toSite(s))
	}
	return out, nil
}

func (r *MySQLRepository) CreateSite(ctx context.Context, in NewSite) (Site, error) {
	id, err := r.q.CreateSite(ctx, store.CreateSiteParams{
		Name: in.Name, SearchUrlTemplate: in.SearchURLTemplate, FetchType: store.SitesFetchType(in.FetchType), IsReference: in.IsReference,
	})
	if mysqlErr(err, mysqlDuplicateEntry) {
		return Site{}, ErrDuplicateName
	}
	if err != nil {
		return Site{}, err
	}
	row, err := r.q.GetSite(ctx, id)
	if err != nil {
		return Site{}, err
	}
	return toSite(row), nil
}

func (r *MySQLRepository) UpdateSite(ctx context.Context, id int64, p SitePatch) (Site, error) {
	var out Site
	err := r.inTx(ctx, func(q *store.Queries) error {
		cur, err := q.GetSite(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		arg := store.UpdateSiteParams{ID: id, Name: cur.Name, SearchUrlTemplate: cur.SearchUrlTemplate, FetchType: cur.FetchType, IsReference: cur.IsReference}
		if p.Name != nil {
			arg.Name = *p.Name
		}
		if p.SearchURLTemplate != nil {
			arg.SearchUrlTemplate = *p.SearchURLTemplate
		}
		if p.FetchType != nil {
			arg.FetchType = store.SitesFetchType(*p.FetchType)
		}
		if p.IsReference != nil {
			arg.IsReference = *p.IsReference
		}
		if _, err := q.UpdateSite(ctx, arg); err != nil {
			if mysqlErr(err, mysqlDuplicateEntry) {
				return ErrDuplicateName
			}
			return err
		}
		row, err := q.GetSite(ctx, id)
		if err != nil {
			return err
		}
		out = toSite(row)
		return nil
	})
	if err != nil {
		return Site{}, err
	}
	return out, nil
}
