package item

import (
	"cmp"
	"context"
	"slices"
	"sync"
	"time"
)

// MemoryRepository はメモリ上の Repository。HTTP 層のテストと、MySQL 実装と同じ契約テスト(itemtest)に使う。
// MySQL 実装と同じ規則(外部キー・UNIQUE・並び・不可分性)を守ること。
type MemoryRepository struct {
	mu      sync.Mutex
	nextID  int64
	genres  map[int64]Genre
	sites   map[int64]Site
	items   map[int64]Item
	nowFunc func() time.Time
}

var _ Repository = (*MemoryRepository)(nil)

// NewMemoryRepository は空の MemoryRepository を返す(seed は入れない)。
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		genres:  map[int64]Genre{},
		sites:   map[int64]Site{},
		items:   map[int64]Item{},
		nowFunc: time.Now,
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
			return ErrSiteNotFound
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

func cloneItem(it Item) Item {
	it.SiteOverrides = cloneOverrides(it.SiteOverrides)
	it.OptionText = cloneP(it.OptionText)
	it.QueryOverride = cloneP(it.QueryOverride)
	it.SourceURL = cloneP(it.SourceURL)
	it.MinPrice = cloneP(it.MinPrice)
	return it
}

func cloneP[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func cloneGenre(g Genre) Genre {
	g.SiteIDs = slices.Clone(g.SiteIDs)
	if g.SiteIDs == nil {
		g.SiteIDs = []int64{}
	}
	return g
}

func (m *MemoryRepository) ListItems(_ context.Context, genreID *int64) ([]Item, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Item{}
	for _, it := range m.items {
		if genreID == nil || it.GenreID == *genreID {
			out = append(out, cloneItem(it))
		}
	}
	slices.SortFunc(out, func(a, b Item) int {
		return cmp.Or(cmp.Compare(a.SortOrder, b.SortOrder), cmp.Compare(b.ID, a.ID))
	})
	return out, nil
}

func (m *MemoryRepository) GetItem(_ context.Context, id int64) (Item, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	it, ok := m.items[id]
	if !ok {
		return Item{}, ErrNotFound
	}
	return cloneItem(it), nil
}

func (m *MemoryRepository) CreateItem(_ context.Context, in NewItem) (Item, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.genres[in.GenreID]; !ok {
		return Item{}, ErrGenreNotFound
	}
	now := m.now()
	it := Item{
		ID: m.id(), GenreID: in.GenreID, Name: in.Name, OptionText: in.OptionText, QueryOverride: in.QueryOverride,
		ImagePath: in.ImagePath, SourceURL: in.SourceURL, MinPrice: in.MinPrice, SortOrder: in.SortOrder,
		SiteOverrides: []SiteOverride{}, CreatedAt: now, UpdatedAt: now,
	}
	it = cloneItem(it)
	m.items[it.ID] = it
	return cloneItem(it), nil
}

func (m *MemoryRepository) UpdateItem(_ context.Context, id int64, p ItemPatch) (Item, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	it, ok := m.items[id]
	if !ok {
		return Item{}, ErrNotFound
	}
	it = cloneItem(it)
	if p.GenreID != nil {
		if _, ok := m.genres[*p.GenreID]; !ok {
			return Item{}, ErrGenreNotFound
		}
		it.GenreID = *p.GenreID
	}
	if p.SiteOverrides != nil {
		if err := checkUniqueOverrides(*p.SiteOverrides); err != nil {
			return Item{}, err
		}
		for _, o := range *p.SiteOverrides {
			if _, ok := m.sites[o.SiteID]; !ok {
				return Item{}, ErrSiteNotFound
			}
		}
		it.SiteOverrides = cloneOverrides(*p.SiteOverrides)
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
	it.OptionText = applyNullable(it.OptionText, p.OptionText)
	it.QueryOverride = applyNullable(it.QueryOverride, p.QueryOverride)
	it.SourceURL = applyNullable(it.SourceURL, p.SourceURL)
	it.MinPrice = applyNullable(it.MinPrice, p.MinPrice)
	it.UpdatedAt = m.now()
	m.items[id] = it
	return cloneItem(it), nil
}

func (m *MemoryRepository) DeleteItem(_ context.Context, id int64) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	it, ok := m.items[id]
	if !ok {
		return "", ErrNotFound
	}
	delete(m.items, id)
	return it.ImagePath, nil
}

func (m *MemoryRepository) ListGenres(_ context.Context) ([]Genre, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Genre, 0, len(m.genres))
	for _, g := range m.genres {
		out = append(out, cloneGenre(g))
	}
	slices.SortFunc(out, func(a, b Genre) int {
		return cmp.Or(cmp.Compare(a.SortOrder, b.SortOrder), cmp.Compare(a.ID, b.ID))
	})
	return out, nil
}

func (m *MemoryRepository) CreateGenre(_ context.Context, in NewGenre) (Genre, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := checkUniqueSiteIDs(in.SiteIDs); err != nil {
		return Genre{}, err
	}
	if err := m.checkSites(in.SiteIDs); err != nil {
		return Genre{}, err
	}
	if m.genreNameTaken(in.Name, 0) {
		return Genre{}, ErrDuplicateName
	}
	g := cloneGenre(Genre{ID: m.id(), Name: in.Name, QueryTemplate: in.QueryTemplate, SortOrder: in.SortOrder, SiteIDs: in.SiteIDs})
	m.genres[g.ID] = g
	return cloneGenre(g), nil
}

func (m *MemoryRepository) UpdateGenre(_ context.Context, id int64, p GenrePatch) (Genre, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.genres[id]
	if !ok {
		return Genre{}, ErrNotFound
	}
	g = cloneGenre(g)
	if p.SiteIDs != nil {
		if err := checkUniqueSiteIDs(*p.SiteIDs); err != nil {
			return Genre{}, err
		}
		if err := m.checkSites(*p.SiteIDs); err != nil {
			return Genre{}, err
		}
		g.SiteIDs = slices.Clone(*p.SiteIDs)
	}
	if p.Name != nil {
		if m.genreNameTaken(*p.Name, id) {
			return Genre{}, ErrDuplicateName
		}
		g.Name = *p.Name
	}
	if p.QueryTemplate != nil {
		g.QueryTemplate = *p.QueryTemplate
	}
	if p.SortOrder != nil {
		g.SortOrder = *p.SortOrder
	}
	g = cloneGenre(g)
	m.genres[id] = g
	return cloneGenre(g), nil
}

func (m *MemoryRepository) ListSites(_ context.Context) ([]Site, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Site, 0, len(m.sites))
	for _, s := range m.sites {
		out = append(out, s)
	}
	slices.SortFunc(out, func(a, b Site) int { return cmp.Compare(a.ID, b.ID) })
	return out, nil
}

func (m *MemoryRepository) CreateSite(_ context.Context, in NewSite) (Site, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.siteNameTaken(in.Name, 0) {
		return Site{}, ErrDuplicateName
	}
	s := Site{ID: m.id(), Name: in.Name, SearchURLTemplate: in.SearchURLTemplate, FetchType: in.FetchType, IsReference: in.IsReference}
	m.sites[s.ID] = s
	return s, nil
}

func (m *MemoryRepository) UpdateSite(_ context.Context, id int64, p SitePatch) (Site, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sites[id]
	if !ok {
		return Site{}, ErrNotFound
	}
	if p.Name != nil {
		if m.siteNameTaken(*p.Name, id) {
			return Site{}, ErrDuplicateName
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
