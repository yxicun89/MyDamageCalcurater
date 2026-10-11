package memory

import (
	"cmp"
	"context"
	"slices"
	"sync"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/app/items"
	"example.com/pokecalc/apps/wishlist/api/internal/domain/item"
)

// MemoryRepository はメモリ上の Repository。HTTP 層のテストと、MySQL 実装と同じ契約テスト(itemtest)に使う。
// MySQL 実装と同じ規則(外部キー・UNIQUE・並び・不可分性)を守ること。
type MemoryRepository struct {
	mu     sync.Mutex
	nextID int64
	genres map[int64]item.Genre
	sites  map[int64]item.Site
	items  map[int64]item.Item
	// 目安価格(フェーズ3。memory_price.go)。
	estimates map[[2]int64]item.Estimate // (item_id, site_id)
	listings  []item.Listing
	history   map[historyKey]item.PricePoint // 価格の推移(フェーズ4-2。memory_history.go)
	officials map[int64]item.OfficialStatus  // 公式の販売状況(フェーズ4-3。memory_official.go)
	nowFunc   func() time.Time
}

var _ items.Repository = (*MemoryRepository)(nil)

// NewMemoryRepository は空の MemoryRepository を返す(seed は入れない)。
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		genres:    map[int64]item.Genre{},
		sites:     map[int64]item.Site{},
		items:     map[int64]item.Item{},
		estimates: map[[2]int64]item.Estimate{},
		history:   map[historyKey]item.PricePoint{},
		officials: map[int64]item.OfficialStatus{},
		nowFunc:   time.Now,
	}
}

func (m *MemoryRepository) id() int64 {
	m.nextID++
	return m.nextID
}

// now は MySQL の DATETIME に合わせて秒までに丸める。
func (m *MemoryRepository) now() time.Time { return m.nowFunc().UTC().Truncate(time.Second) }

func (m *MemoryRepository) checkSites(ids []int64) error {
	for _, id := range ids {
		if _, ok := m.sites[id]; !ok {
			return item.ErrSiteNotFound
		}
	}
	return nil
}

func (m *MemoryRepository) genreNameTaken(name string, except int64) bool {
	for id, g := range m.genres {
		if id != except && g.Name == name {
			return true
		}
	}
	return false
}

func (m *MemoryRepository) siteNameTaken(name string, except int64) bool {
	for id, s := range m.sites {
		if id != except && s.Name == name {
			return true
		}
	}
	return false
}

func cloneItem(it item.Item) item.Item {
	it.SiteOverrides = item.CloneOverrides(it.SiteOverrides)
	it.OptionText = item.CloneP(it.OptionText)
	it.QueryOverride = item.CloneP(it.QueryOverride)
	it.SourceURL = item.CloneP(it.SourceURL)
	it.MinPrice = item.CloneP(it.MinPrice)
	if it.Official != nil {
		o := cloneOfficial(*it.Official)
		it.Official = &o
	}
	return it
}

func cloneGenre(g item.Genre) item.Genre {
	g.SiteIDs = slices.Clone(g.SiteIDs)
	if g.SiteIDs == nil {
		g.SiteIDs = []int64{}
	}
	g.Aliases = item.CloneAliases(g.Aliases)
	g.MatchHosts, g.MatchKeywords = cloneStrings(g.MatchHosts), cloneStrings(g.MatchKeywords)
	return g
}

// cloneStrings は v の複製(nil は長さ 0 の配列)。
func cloneStrings(v []string) []string {
	if v == nil {
		return []string{}
	}
	return slices.Clone(v)
}

func (m *MemoryRepository) ListItems(_ context.Context, genreID *int64) ([]item.Item, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []item.Item{}
	for _, it := range m.items {
		if genreID == nil || it.GenreID == *genreID {
			c := cloneItem(m.withOfficial(it))
			sum := item.SummarizeItem(c, m.genres[c.GenreID].SiteIDs, m.itemEstimates(c.ID))
			c.Summary = &sum
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b item.Item) int {
		return cmp.Or(cmp.Compare(a.SortOrder, b.SortOrder), cmp.Compare(b.ID, a.ID))
	})
	return out, nil
}

func (m *MemoryRepository) GetItem(_ context.Context, id int64) (item.Item, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	it, ok := m.items[id]
	if !ok {
		return item.Item{}, item.ErrNotFound
	}
	return cloneItem(m.withOfficial(it)), nil
}

func (m *MemoryRepository) CreateItem(_ context.Context, in item.NewItem) (item.Item, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.genres[in.GenreID]; !ok {
		return item.Item{}, item.ErrGenreNotFound
	}
	now := m.now()
	it := item.Item{
		ID: m.id(), GenreID: in.GenreID, Name: in.Name, OptionText: in.OptionText, QueryOverride: in.QueryOverride,
		ImagePath: in.ImagePath, SourceURL: in.SourceURL, MinPrice: in.MinPrice, SortOrder: in.SortOrder,
		SiteOverrides: []item.SiteOverride{}, CreatedAt: now, UpdatedAt: now, WatchOfficial: in.WatchOfficial,
	}
	it = cloneItem(it)
	m.items[it.ID] = it
	return cloneItem(it), nil
}

func (m *MemoryRepository) UpdateItem(_ context.Context, id int64, p item.ItemPatch) (item.Item, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	it, ok := m.items[id]
	if !ok {
		return item.Item{}, item.ErrNotFound
	}
	it = cloneItem(it)
	if p.GenreID != nil {
		if _, ok := m.genres[*p.GenreID]; !ok {
			return item.Item{}, item.ErrGenreNotFound
		}
		it.GenreID = *p.GenreID
	}
	if p.SiteOverrides != nil {
		if err := item.CheckUniqueOverrides(*p.SiteOverrides); err != nil {
			return item.Item{}, err
		}
		for _, o := range *p.SiteOverrides {
			if _, ok := m.sites[o.SiteID]; !ok {
				return item.Item{}, item.ErrSiteNotFound
			}
		}
		it.SiteOverrides = item.CloneOverrides(*p.SiteOverrides)
	}
	if p.Name != nil {
		it.Name = *p.Name
	}
	if p.SortOrder != nil {
		it.SortOrder = *p.SortOrder
	}
	if p.ImagePath != nil {
		it.ImagePath = *p.ImagePath
	}
	it.OptionText = item.ApplyNullable(it.OptionText, p.OptionText)
	it.QueryOverride = item.ApplyNullable(it.QueryOverride, p.QueryOverride)
	oldSource := it.SourceURL
	it.SourceURL = item.ApplyNullable(it.SourceURL, p.SourceURL)
	if !sameStrP(oldSource, it.SourceURL) {
		delete(m.officials, id) // 別のページの状態を見せない
	}
	it.MinPrice = item.ApplyNullable(it.MinPrice, p.MinPrice)
	if p.WatchOfficial != nil {
		it.WatchOfficial = *p.WatchOfficial
	}
	it.UpdatedAt = m.now()
	m.items[id] = it
	return cloneItem(m.withOfficial(it)), nil
}

func (m *MemoryRepository) DeleteItem(_ context.Context, id int64) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	it, ok := m.items[id]
	if !ok {
		return "", item.ErrNotFound
	}
	delete(m.items, id)
	delete(m.officials, id)
	m.deletePriceData(id)
	return it.ImagePath, nil
}

func (m *MemoryRepository) ListGenres(_ context.Context) ([]item.Genre, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]item.Genre, 0, len(m.genres))
	for _, g := range m.genres {
		out = append(out, cloneGenre(g))
	}
	slices.SortFunc(out, func(a, b item.Genre) int {
		return cmp.Or(cmp.Compare(a.SortOrder, b.SortOrder), cmp.Compare(a.ID, b.ID))
	})
	return out, nil
}

func (m *MemoryRepository) CreateGenre(_ context.Context, in item.NewGenre) (item.Genre, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := item.CheckUniqueSiteIDs(in.SiteIDs); err != nil {
		return item.Genre{}, err
	}
	if err := m.checkSites(in.SiteIDs); err != nil {
		return item.Genre{}, err
	}
	if m.genreNameTaken(in.Name, 0) {
		return item.Genre{}, item.ErrDuplicateName
	}
	if err := item.CheckAliasDuplicates(in.Aliases); err != nil {
		return item.Genre{}, err
	}
	if in.Color == "" {
		in.Color = item.DefaultGenreColor
	}
	if in.Icon == "" {
		in.Icon = item.DefaultGenreIcon
	}
	g := cloneGenre(item.Genre{ID: m.id(), Name: in.Name, QueryTemplate: in.QueryTemplate, SortOrder: in.SortOrder, SiteIDs: in.SiteIDs, Aliases: in.Aliases,
		Color: in.Color, Icon: in.Icon, MatchHosts: in.MatchHosts, MatchKeywords: in.MatchKeywords})
	m.genres[g.ID] = g
	return cloneGenre(g), nil
}

func (m *MemoryRepository) UpdateGenre(_ context.Context, id int64, p item.GenrePatch) (item.Genre, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.genres[id]
	if !ok {
		return item.Genre{}, item.ErrNotFound
	}
	g = cloneGenre(g)
	if p.SiteIDs != nil {
		if err := item.CheckUniqueSiteIDs(*p.SiteIDs); err != nil {
			return item.Genre{}, err
		}
		if err := m.checkSites(*p.SiteIDs); err != nil {
			return item.Genre{}, err
		}
		g.SiteIDs = slices.Clone(*p.SiteIDs)
	}
	if p.Aliases != nil {
		if err := item.CheckAliasDuplicates(*p.Aliases); err != nil {
			return item.Genre{}, err
		}
		g.Aliases = item.CloneAliases(*p.Aliases)
	}
	if p.Name != nil {
		if m.genreNameTaken(*p.Name, id) {
			return item.Genre{}, item.ErrDuplicateName
		}
		g.Name = *p.Name
	}
	if p.QueryTemplate != nil {
		g.QueryTemplate = *p.QueryTemplate
	}
	if p.SortOrder != nil {
		g.SortOrder = *p.SortOrder
	}
	if p.Color != nil {
		g.Color = *p.Color
	}
	if p.Icon != nil {
		g.Icon = *p.Icon
	}
	if p.MatchHosts != nil {
		g.MatchHosts = *p.MatchHosts
	}
	if p.MatchKeywords != nil {
		g.MatchKeywords = *p.MatchKeywords
	}
	g = cloneGenre(g)
	m.genres[id] = g
	return cloneGenre(g), nil
}

func (m *MemoryRepository) ListSites(_ context.Context) ([]item.Site, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]item.Site, 0, len(m.sites))
	for _, s := range m.sites {
		out = append(out, s)
	}
	slices.SortFunc(out, func(a, b item.Site) int { return cmp.Compare(a.ID, b.ID) })
	return out, nil
}

func (m *MemoryRepository) CreateSite(_ context.Context, in item.NewSite) (item.Site, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.siteNameTaken(in.Name, 0) {
		return item.Site{}, item.ErrDuplicateName
	}
	s := item.Site{ID: m.id(), Name: in.Name, SearchURLTemplate: in.SearchURLTemplate, FetchType: in.FetchType, IsReference: in.IsReference}
	m.sites[s.ID] = s
	return s, nil
}

func (m *MemoryRepository) UpdateSite(_ context.Context, id int64, p item.SitePatch) (item.Site, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sites[id]
	if !ok {
		return item.Site{}, item.ErrNotFound
	}
	if p.Name != nil {
		if m.siteNameTaken(*p.Name, id) {
			return item.Site{}, item.ErrDuplicateName
		}
		s.Name = *p.Name
	}
	if p.SearchURLTemplate != nil {
		s.SearchURLTemplate = *p.SearchURLTemplate
	}
	if p.FetchType != nil {
		s.FetchType = *p.FetchType
	}
	if p.IsReference != nil {
		s.IsReference = *p.IsReference
	}
	m.sites[id] = s
	return s, nil
}

func sameStrP(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
