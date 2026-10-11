package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/go-sql-driver/mysql"

	"example.com/pokecalc/apps/wishlist/api/internal/adapters/store/sqlcgen"
	"example.com/pokecalc/apps/wishlist/api/internal/app/items"
	"example.com/pokecalc/apps/wishlist/api/internal/domain/item"
	"example.com/pokecalc/apps/wishlist/api/internal/domain/query"
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
	q  *sqlcgen.Queries
}

var _ items.Repository = (*MySQLRepository)(nil)

// NewMySQLRepository は db(parseTime=true の DSN で開いたもの)を使う Repository を返す。
func NewMySQLRepository(db *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: db, q: sqlcgen.New(db)}
}

// mysqlErr は err が MySQL のエラー番号 num かを返す。
func mysqlErr(err error, num uint16) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == num
}

// inTx は fn をトランザクションで実行する(fn がエラーなら巻き戻す)。
func (r *MySQLRepository) inTx(ctx context.Context, fn func(q *sqlcgen.Queries) error) error {
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
		return 0, fmt.Errorf("%w: number is out of range", item.ErrInvalid)
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

func toItem(row sqlcgen.Item, overrides []sqlcgen.ItemSiteOverride, official *item.OfficialStatus) item.Item {
	it := item.Item{
		ID: row.ID, GenreID: row.GenreID, Name: row.Name, OptionText: strPtr(row.OptionText), QueryOverride: strPtr(row.QueryOverride),
		ImagePath: row.ImagePath, SourceURL: strPtr(row.SourceUrl), MinPrice: intPtr(row.MinPrice), SortOrder: int(row.SortOrder),
		SiteOverrides: make([]item.SiteOverride, 0, len(overrides)), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		WatchOfficial: row.WatchOfficial, Official: official,
	}
	for _, o := range overrides {
		it.SiteOverrides = append(it.SiteOverrides, item.SiteOverride{SiteID: o.SiteID, Query: strPtr(o.Query), Enabled: o.Enabled})
	}
	return it
}

func toSite(row sqlcgen.Site) item.Site {
	return item.Site{ID: row.ID, Name: row.Name, SearchURLTemplate: row.SearchUrlTemplate, FetchType: item.FetchType(row.FetchType), IsReference: row.IsReference}
}

func (r *MySQLRepository) ListItems(ctx context.Context, genreID *int64) ([]item.Item, error) {
	var rows []sqlcgen.Item
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
	byItem := map[int64][]sqlcgen.ItemSiteOverride{}
	for _, o := range all {
		byItem[o.ItemID] = append(byItem[o.ItemID], o)
	}
	officials, err := officialByItem(ctx, r.q)
	if err != nil {
		return nil, err
	}
	links, err := r.q.ListGenreSites(ctx) // genre_id, sort_order, site_id 順
	if err != nil {
		return nil, err
	}
	siteIDs := map[int64][]int64{}
	for _, l := range links {
		siteIDs[l.GenreID] = append(siteIDs[l.GenreID], l.SiteID)
	}
	estRows, err := r.q.ListEstimates(ctx)
	if err != nil {
		return nil, err
	}
	ests := map[int64][]item.Estimate{}
	for _, e := range estRows {
		ests[e.ItemID] = append(ests[e.ItemID], toEstimate(e))
	}
	out := make([]item.Item, 0, len(rows))
	for _, row := range rows {
		var official *item.OfficialStatus
		if o, ok := officials[row.ID]; ok {
			official = &o
		}
		it := toItem(row, byItem[row.ID], official)
		sum := item.SummarizeItem(it, siteIDs[row.GenreID], ests[row.ID])
		it.Summary = &sum
		out = append(out, it)
	}
	return out, nil
}

func (r *MySQLRepository) getItem(ctx context.Context, q *sqlcgen.Queries, id int64) (item.Item, error) {
	row, err := q.GetItem(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return item.Item{}, item.ErrNotFound
	}
	if err != nil {
		return item.Item{}, err
	}
	os, err := q.ListItemSiteOverridesByItem(ctx, id)
	if err != nil {
		return item.Item{}, err
	}
	var official *item.OfficialStatus
	orow, err := q.GetOfficialStatus(ctx, id)
	switch {
	case err == nil:
		o, err := toOfficial(orow)
		if err != nil {
			return item.Item{}, err
		}
		official = &o
	case !errors.Is(err, sql.ErrNoRows):
		return item.Item{}, err
	}
	return toItem(row, os, official), nil
}

func (r *MySQLRepository) GetItem(ctx context.Context, id int64) (item.Item, error) {
	return r.getItem(ctx, r.q, id)
}

func (r *MySQLRepository) CreateItem(ctx context.Context, in item.NewItem) (item.Item, error) {
	sortOrder, err := toInt32(in.SortOrder)
	if err != nil {
		return item.Item{}, err
	}
	minPrice, err := nullInt32(in.MinPrice)
	if err != nil {
		return item.Item{}, err
	}
	id, err := r.q.CreateItem(ctx, sqlcgen.CreateItemParams{
		GenreID: in.GenreID, Name: in.Name, OptionText: nullStr(in.OptionText), QueryOverride: nullStr(in.QueryOverride),
		ImagePath: in.ImagePath, SourceUrl: nullStr(in.SourceURL), MinPrice: minPrice, SortOrder: sortOrder, WatchOfficial: in.WatchOfficial,
	})
	if mysqlErr(err, mysqlForeignKeyNoRef) {
		return item.Item{}, item.ErrGenreNotFound
	}
	if err != nil {
		return item.Item{}, err
	}
	return r.GetItem(ctx, id)
}

func (r *MySQLRepository) UpdateItem(ctx context.Context, id int64, p item.ItemPatch) (item.Item, error) {
	if p.SiteOverrides != nil {
		if err := item.CheckUniqueOverrides(*p.SiteOverrides); err != nil {
			return item.Item{}, err
		}
	}
	var out item.Item
	err := r.inTx(ctx, func(q *sqlcgen.Queries) error {
		cur, err := q.GetItemForUpdate(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return item.ErrNotFound
		}
		if err != nil {
			return err
		}
		arg := sqlcgen.UpdateItemParams{
			ID: id, GenreID: cur.GenreID, Name: cur.Name, OptionText: cur.OptionText, QueryOverride: cur.QueryOverride,
			ImagePath: cur.ImagePath, SourceUrl: cur.SourceUrl, MinPrice: cur.MinPrice, SortOrder: cur.SortOrder,
			WatchOfficial: cur.WatchOfficial,
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
		if p.WatchOfficial != nil {
			arg.WatchOfficial = *p.WatchOfficial
		}
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
		if arg.SourceUrl != cur.SourceUrl {
			// 別のページの状態を見せない
			if err := q.DeleteOfficialStatus(ctx, id); err != nil {
				return err
			}
		}
		if err := q.UpdateItem(ctx, arg); err != nil {
			if mysqlErr(err, mysqlForeignKeyNoRef) {
				return item.ErrGenreNotFound
			}
			return err
		}
		if p.SiteOverrides != nil {
			if err := q.DeleteItemSiteOverrides(ctx, id); err != nil {
				return err
			}
			for _, o := range *p.SiteOverrides {
				err := q.InsertItemSiteOverride(ctx, sqlcgen.InsertItemSiteOverrideParams{ItemID: id, SiteID: o.SiteID, Query: nullStr(o.Query), Enabled: o.Enabled})
				if mysqlErr(err, mysqlForeignKeyNoRef) {
					return item.ErrSiteNotFound
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
		return item.Item{}, err
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
	err := r.inTx(ctx, func(q *sqlcgen.Queries) error {
		row, err := q.GetItemForUpdate(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return item.ErrNotFound
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

func (r *MySQLRepository) ListGenres(ctx context.Context) ([]item.Genre, error) {
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
	aliasRows, err := r.q.ListGenreAliases(ctx)
	if err != nil {
		return nil, err
	}
	aliasesByGenre := map[int64][]sqlcgen.GenreAlias{}
	for _, a := range aliasRows { // genre_id, group_no, id 順
		aliasesByGenre[a.GenreID] = append(aliasesByGenre[a.GenreID], a)
	}
	out := make([]item.Genre, 0, len(rows))
	for _, g := range rows {
		ids := byGenre[g.ID]
		if ids == nil {
			ids = []int64{}
		}
		out = append(out, withStyle(item.Genre{ID: g.ID, Name: g.Name, QueryTemplate: g.QueryTemplate, SortOrder: int(g.SortOrder), SiteIDs: ids, Aliases: toAliasGroups(aliasesByGenre[g.ID])}, g))
	}
	return out, nil
}

// withStyle は g に色・絵柄・推定の手がかり(genres の列)を載せる。JSON が読めなければ長さ 0 にする(列は NOT NULL の配列)。
func withStyle(g item.Genre, row sqlcgen.Genre) item.Genre {
	g.Color, g.Icon = row.Color, item.GenreIcon(row.Icon)
	g.MatchHosts, g.MatchKeywords = unmarshalStrings(row.MatchHosts), unmarshalStrings(row.MatchKeywords)
	return g
}

// marshalStrings は文字列の配列を JSON にする(nil は [])。
func marshalStrings(v []string) json.RawMessage {
	if v == nil {
		v = []string{}
	}
	b, _ := json.Marshal(v) // []string は必ず符号化できる
	return b
}

func unmarshalStrings(raw json.RawMessage) []string {
	out := []string{}
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return []string{}
	}
	return out
}

// getGenre はジャンルとその site_ids を読む。forUpdate なら行をロックする(更新の直前に読むとき)。
func (r *MySQLRepository) getGenre(ctx context.Context, q *sqlcgen.Queries, id int64, forUpdate bool) (item.Genre, error) {
	var g sqlcgen.Genre
	var err error
	if forUpdate {
		g, err = q.GetGenreForUpdate(ctx, id)
	} else {
		g, err = q.GetGenre(ctx, id)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return item.Genre{}, item.ErrNotFound
	}
	if err != nil {
		return item.Genre{}, err
	}
	links, err := q.ListGenreSitesByGenre(ctx, id)
	if err != nil {
		return item.Genre{}, err
	}
	ids := make([]int64, 0, len(links))
	for _, l := range links {
		ids = append(ids, l.SiteID)
	}
	aliasRows, err := q.ListGenreAliasesByGenre(ctx, id)
	if err != nil {
		return item.Genre{}, err
	}
	return withStyle(item.Genre{ID: g.ID, Name: g.Name, QueryTemplate: g.QueryTemplate, SortOrder: int(g.SortOrder), SiteIDs: ids, Aliases: toAliasGroups(aliasRows)}, g), nil
}

// toAliasGroups は group_no, id 順の行を、保存した順のグループにまとめる(無ければ長さ 0)。
func toAliasGroups(rows []sqlcgen.GenreAlias) [][]string {
	out := [][]string{}
	for i, a := range rows {
		if i == 0 || a.GroupNo != rows[i-1].GroupNo {
			out = append(out, []string{})
		}
		out[len(out)-1] = append(out[len(out)-1], a.Alias)
	}
	return out
}

// replaceGenreAliases は genre_aliases を groups で置き換える。正規化後の重複は ErrInvalid。
func replaceGenreAliases(ctx context.Context, q *sqlcgen.Queries, genreID int64, groups [][]string) error {
	if err := item.CheckAliasDuplicates(groups); err != nil {
		return err
	}
	if err := q.DeleteGenreAliases(ctx, genreID); err != nil {
		return err
	}
	for gi, g := range groups {
		for _, w := range g {
			err := q.InsertGenreAlias(ctx, sqlcgen.InsertGenreAliasParams{GenreID: genreID, GroupNo: int32(gi), Alias: w, Normalized: query.Normalize(w)})
			if mysqlErr(err, mysqlDuplicateEntry) {
				return fmt.Errorf("%w: alias %q is duplicated after normalization", item.ErrInvalid, w)
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// replaceGenreSites は genre_sites を ids(指定順)で置き換える。
func replaceGenreSites(ctx context.Context, q *sqlcgen.Queries, genreID int64, ids []int64) error {
	if err := q.DeleteGenreSites(ctx, genreID); err != nil {
		return err
	}
	for i, sid := range ids {
		err := q.InsertGenreSite(ctx, sqlcgen.InsertGenreSiteParams{GenreID: genreID, SiteID: sid, SortOrder: int32(i)})
		if mysqlErr(err, mysqlForeignKeyNoRef) {
			return item.ErrSiteNotFound
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *MySQLRepository) CreateGenre(ctx context.Context, in item.NewGenre) (item.Genre, error) {
	if err := item.CheckUniqueSiteIDs(in.SiteIDs); err != nil {
		return item.Genre{}, err
	}
	sortOrder, err := toInt32(in.SortOrder)
	if err != nil {
		return item.Genre{}, err
	}
	// Service を通さない呼び出しでも、省略した色・絵柄は既定値にする(DB の既定値と同じ)。
	if in.Color == "" {
		in.Color = item.DefaultGenreColor
	}
	if in.Icon == "" {
		in.Icon = item.DefaultGenreIcon
	}
	var out item.Genre
	err = r.inTx(ctx, func(q *sqlcgen.Queries) error {
		id, err := q.CreateGenre(ctx, sqlcgen.CreateGenreParams{
			Name: in.Name, QueryTemplate: in.QueryTemplate, SortOrder: sortOrder,
			Color: in.Color, Icon: sqlcgen.GenresIcon(in.Icon), MatchHosts: marshalStrings(in.MatchHosts), MatchKeywords: marshalStrings(in.MatchKeywords),
		})
		if mysqlErr(err, mysqlDuplicateEntry) {
			return item.ErrDuplicateName
		}
		if err != nil {
			return err
		}
		if err := replaceGenreSites(ctx, q, id, in.SiteIDs); err != nil {
			return err
		}
		if err := replaceGenreAliases(ctx, q, id, in.Aliases); err != nil {
			return err
		}
		out, err = r.getGenre(ctx, q, id, false)
		return err
	})
	if err != nil {
		return item.Genre{}, err
	}
	return out, nil
}

func (r *MySQLRepository) UpdateGenre(ctx context.Context, id int64, p item.GenrePatch) (item.Genre, error) {
	if p.SiteIDs != nil {
		if err := item.CheckUniqueSiteIDs(*p.SiteIDs); err != nil {
			return item.Genre{}, err
		}
	}
	var out item.Genre
	err := r.inTx(ctx, func(q *sqlcgen.Queries) error {
		cur, err := r.getGenre(ctx, q, id, true)
		if err != nil {
			return err
		}
		arg := sqlcgen.UpdateGenreParams{
			ID: id, Name: cur.Name, QueryTemplate: cur.QueryTemplate,
			Color: cur.Color, Icon: sqlcgen.GenresIcon(cur.Icon), MatchHosts: marshalStrings(cur.MatchHosts), MatchKeywords: marshalStrings(cur.MatchKeywords),
		}
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
		if p.Color != nil {
			arg.Color = *p.Color
		}
		if p.Icon != nil {
			arg.Icon = sqlcgen.GenresIcon(*p.Icon)
		}
		if p.MatchHosts != nil {
			arg.MatchHosts = marshalStrings(*p.MatchHosts)
		}
		if p.MatchKeywords != nil {
			arg.MatchKeywords = marshalStrings(*p.MatchKeywords)
		}
		if _, err := q.UpdateGenre(ctx, arg); err != nil {
			if mysqlErr(err, mysqlDuplicateEntry) {
				return item.ErrDuplicateName
			}
			return err
		}
		if p.SiteIDs != nil {
			if err := replaceGenreSites(ctx, q, id, *p.SiteIDs); err != nil {
				return err
			}
		}
		if p.Aliases != nil {
			if err := replaceGenreAliases(ctx, q, id, *p.Aliases); err != nil {
				return err
			}
		}
		out, err = r.getGenre(ctx, q, id, false)
		return err
	})
	if err != nil {
		return item.Genre{}, err
	}
	return out, nil
}

func (r *MySQLRepository) ListSites(ctx context.Context) ([]item.Site, error) {
	rows, err := r.q.ListSites(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]item.Site, 0, len(rows))
	for _, s := range rows {
		out = append(out, toSite(s))
	}
	return out, nil
}

func (r *MySQLRepository) CreateSite(ctx context.Context, in item.NewSite) (item.Site, error) {
	id, err := r.q.CreateSite(ctx, sqlcgen.CreateSiteParams{
		Name: in.Name, SearchUrlTemplate: in.SearchURLTemplate, FetchType: sqlcgen.SitesFetchType(in.FetchType), IsReference: in.IsReference,
	})
	if mysqlErr(err, mysqlDuplicateEntry) {
		return item.Site{}, item.ErrDuplicateName
	}
	if err != nil {
		return item.Site{}, err
	}
	row, err := r.q.GetSite(ctx, id)
	if err != nil {
		return item.Site{}, err
	}
	return toSite(row), nil
}

func (r *MySQLRepository) UpdateSite(ctx context.Context, id int64, p item.SitePatch) (item.Site, error) {
	var out item.Site
	err := r.inTx(ctx, func(q *sqlcgen.Queries) error {
		cur, err := q.GetSite(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return item.ErrNotFound
		}
		if err != nil {
			return err
		}
		arg := sqlcgen.UpdateSiteParams{ID: id, Name: cur.Name, SearchUrlTemplate: cur.SearchUrlTemplate, FetchType: cur.FetchType, IsReference: cur.IsReference}
		if p.Name != nil {
			arg.Name = *p.Name
		}
		if p.SearchURLTemplate != nil {
			arg.SearchUrlTemplate = *p.SearchURLTemplate
		}
		if p.FetchType != nil {
			arg.FetchType = sqlcgen.SitesFetchType(*p.FetchType)
		}
		if p.IsReference != nil {
			arg.IsReference = *p.IsReference
		}
		if _, err := q.UpdateSite(ctx, arg); err != nil {
			if mysqlErr(err, mysqlDuplicateEntry) {
				return item.ErrDuplicateName
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
		return item.Site{}, err
	}
	return out, nil
}
