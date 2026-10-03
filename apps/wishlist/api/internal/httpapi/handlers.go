package httpapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/oapi-codegen/nullable"

	"example.com/pokecalc/apps/wishlist/api/internal/api"
	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/netguard"
	"example.com/pokecalc/apps/wishlist/api/internal/refresh"
	"example.com/pokecalc/apps/wishlist/api/internal/storage"
)

// maxFieldBytes は multipart のテキスト項目 1 つの上限。
const maxFieldBytes = 64 << 10

// server は api.StrictServerInterface の実装。エラーは error として返し、handleError が Error スキーマにする。
type server struct {
	items     *item.Service
	images    storage.Storage
	remote    Remote
	estimates Estimator
	log       *slog.Logger
}

var _ api.StrictServerInterface = (*server)(nil)

// ---- 変換 ----

func toAPIItem(it item.Item) api.Item {
	out := api.Item{
		Id: it.ID, GenreId: it.GenreID, Name: it.Name, ImageUrl: "images/" + it.ImagePath, SortOrder: it.SortOrder,
		OptionText: nullableOf(it.OptionText), QueryOverride: nullableOf(it.QueryOverride), SourceUrl: nullableOf(it.SourceURL),
		MinPrice:      nullableOf(it.MinPrice),
		SiteOverrides: make([]api.SiteOverride, 0, len(it.SiteOverrides)),
		CreatedAt:     it.CreatedAt.UTC(), UpdatedAt: it.UpdatedAt.UTC(),
	}
	for _, o := range it.SiteOverrides {
		out.SiteOverrides = append(out.SiteOverrides, api.SiteOverride{SiteId: o.SiteID, Query: nullableOf(o.Query), Enabled: o.Enabled})
	}
	return out
}

// nullableOf は nil を明示的な null にする。
func nullableOf[T any](p *T) nullable.Nullable[T] {
	if p == nil {
		return nullable.NewNullNullable[T]()
	}
	return nullable.NewNullableWithValue(*p)
}

func toAPIGenre(g item.Genre) api.Genre {
	ids := g.SiteIDs
	if ids == nil {
		ids = []int64{}
	}
	return api.Genre{Id: g.ID, Name: g.Name, QueryTemplate: g.QueryTemplate, SortOrder: g.SortOrder, SiteIds: ids}
}

func toAPISite(s item.Site) api.Site {
	return api.Site{Id: s.ID, Name: s.Name, SearchUrlTemplate: s.SearchURLTemplate, FetchType: api.FetchType(s.FetchType), IsReference: s.IsReference}
}

func toOverrides(in []api.SiteOverride) []item.SiteOverride {
	out := make([]item.SiteOverride, 0, len(in))
	for _, o := range in {
		var q *string
		if v, ok := nullableValue(o.Query); ok {
			q = &v
		}
		out = append(out, item.SiteOverride{SiteID: o.SiteId, Query: q, Enabled: o.Enabled})
	}
	return out
}

func nullableValue[T any](n nullable.Nullable[T]) (T, bool) {
	var zero T
	if !n.IsSpecified() || n.IsNull() {
		return zero, false
	}
	return n.MustGet(), true
}

// checkID はパス・クエリの id が 1 以上であることを確かめる(0 以下は形式の不正)。
func checkID(id int64, what string) error {
	if id < 1 {
		return badRequest(what + " must be a positive integer")
	}
	return nil
}

// ---- 商品 ----

func (s *server) ListItems(ctx context.Context, req api.ListItemsRequestObject) (api.ListItemsResponseObject, error) {
	if g := req.Params.GenreId; g != nil {
		if err := checkID(*g, "genre_id"); err != nil {
			return nil, err
		}
	}
	items, err := s.items.ListItems(ctx, req.Params.GenreId)
	if err != nil {
		return nil, err
	}
	out := make([]api.Item, 0, len(items))
	for _, it := range items {
		out = append(out, toAPIItem(it))
	}
	return api.ListItems200JSONResponse{Items: out}, nil
}

func (s *server) GetItem(ctx context.Context, req api.GetItemRequestObject) (api.GetItemResponseObject, error) {
	if err := checkID(req.Id, "id"); err != nil {
		return nil, err
	}
	it, err := s.items.GetItem(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return api.GetItem200JSONResponse(toAPIItem(it)), nil
}

func (s *server) CreateItem(ctx context.Context, req api.CreateItemRequestObject) (api.CreateItemResponseObject, error) {
	var (
		in    item.NewItem
		image io.Reader
	)
	switch {
	case req.JSONBody != nil:
		b := req.JSONBody
		if err := checkID(b.GenreId, "genre_id"); err != nil {
			return nil, err
		}
		if strings.TrimSpace(b.ImageUrl) == "" {
			return nil, badRequest("image_url is required")
		}
		in = item.NewItem{GenreID: b.GenreId, Name: b.Name, OptionText: b.OptionText, QueryOverride: b.QueryOverride, SourceURL: b.SourceUrl}
		if b.MinPrice != nil {
			in.MinPrice = b.MinPrice
		}
		if b.SortOrder != nil {
			in.SortOrder = *b.SortOrder
		}
		// 外部の画像を取りに行く前に入力を検査する(無駄な取得をしない)
		if err := item.ValidateNewItem(in); err != nil {
			return nil, err
		}
		data, err := s.fetchImage(ctx, b.ImageUrl)
		if err != nil {
			return nil, err
		}
		image = bytes.NewReader(data)
	case req.MultipartBody != nil:
		f, err := readItemForm(req.MultipartBody)
		if err != nil {
			return nil, err
		}
		if in, err = f.newItem(); err != nil {
			return nil, err
		}
		image = bytes.NewReader(f.image)
	default:
		return nil, badRequest("expected application/json or multipart/form-data")
	}
	it, err := s.items.CreateItem(ctx, in, image)
	if err != nil {
		return nil, err
	}
	return api.CreateItem201JSONResponse(toAPIItem(it)), nil
}

// fetchImage は image_url を検査して取得する。http/https 以外は取得せず 422。取得の失敗は 502。
func (s *server) fetchImage(ctx context.Context, raw string) ([]byte, error) {
	if _, err := netguard.ValidateURL(raw); err != nil {
		return nil, unprocessable("image_url must be an absolute http(s) URL")
	}
	data, err := s.remote.Image(ctx, raw, storage.MaxImageBytes)
	if err != nil {
		return nil, remoteError(err, "image_url", http.StatusUnprocessableEntity)
	}
	return data, nil
}

// remoteError は外部取得のエラーを API のエラーにする。禁止アドレス・URL 違反は 422、
// 上限超えは tooLargeStatus、それ以外の取得失敗は 502。
func remoteError(err error, what string, tooLargeStatus int) error {
	switch {
	case errors.Is(err, netguard.ErrForbiddenAddress), errors.Is(err, netguard.ErrInvalidURL):
		return unprocessable(what + " points to a forbidden or invalid address")
	case errors.Is(err, netguard.ErrTooLarge):
		if tooLargeStatus == http.StatusUnprocessableEntity {
			return unprocessable(what + " response is too large")
		}
		return badGateway(what + " response is too large")
	case errors.Is(err, context.Canceled):
		return err
	}
	return badGateway("failed to fetch " + what)
}

func (s *server) DraftItemFromURL(ctx context.Context, req api.DraftItemFromURLRequestObject) (api.DraftItemFromURLResponseObject, error) {
	b := req.Body
	if b == nil || strings.TrimSpace(b.Url) == "" {
		return nil, badRequest("url is required")
	}
	if b.GenreId != nil {
		if err := checkID(*b.GenreId, "genre_id"); err != nil {
			return nil, err
		}
	}
	if _, err := netguard.ValidateURL(b.Url); err != nil {
		return nil, unprocessable("url must be an absolute http(s) URL")
	}
	d, err := s.remote.Draft(ctx, b.Url)
	if err != nil {
		return nil, remoteError(err, "url", http.StatusBadGateway)
	}
	out := api.ItemDraft{Name: d.Title, SourceUrl: b.Url, ImageUrl: nullableOf(d.ImageURL), GenreId: nullableOf(b.GenreId)}
	return api.DraftItemFromURL200JSONResponse(out), nil
}

func (s *server) UpdateItem(ctx context.Context, req api.UpdateItemRequestObject) (api.UpdateItemResponseObject, error) {
	if err := checkID(req.Id, "id"); err != nil {
		return nil, err
	}
	b := req.Body
	if b == nil {
		return nil, badRequest("request body is required")
	}
	p := item.ItemPatch{
		GenreID: b.GenreId, Name: b.Name, OptionText: b.OptionText, QueryOverride: b.QueryOverride,
		SourceURL: b.SourceUrl, MinPrice: b.MinPrice, SortOrder: b.SortOrder,
	}
	if b.SiteOverrides != nil {
		o := toOverrides(*b.SiteOverrides)
		p.SiteOverrides = &o
	}
	it, err := s.items.UpdateItem(ctx, req.Id, p)
	if err != nil {
		return nil, err
	}
	return api.UpdateItem200JSONResponse(toAPIItem(it)), nil
}

func (s *server) DeleteItem(ctx context.Context, req api.DeleteItemRequestObject) (api.DeleteItemResponseObject, error) {
	if err := checkID(req.Id, "id"); err != nil {
		return nil, err
	}
	if err := s.items.DeleteItem(ctx, req.Id); err != nil {
		return nil, err
	}
	return api.DeleteItem204Response{}, nil
}

func (s *server) ReplaceItemImage(ctx context.Context, req api.ReplaceItemImageRequestObject) (api.ReplaceItemImageResponseObject, error) {
	if err := checkID(req.Id, "id"); err != nil {
		return nil, err
	}
	f, err := readItemForm(req.Body)
	if err != nil {
		return nil, err
	}
	if !f.hasImage {
		return nil, badRequest("image is required")
	}
	if f.imageTooLarge {
		return nil, storage.ErrTooLarge
	}
	it, err := s.items.ReplaceImage(ctx, req.Id, bytes.NewReader(f.image))
	if err != nil {
		return nil, err
	}
	return api.ReplaceItemImage200JSONResponse(toAPIItem(it)), nil
}

// ---- 目安価格(フェーズ3) ----

func toAPIEstimates(itemID int64, v refresh.View) api.ItemEstimates {
	out := api.ItemEstimates{
		ItemId: itemID, Refreshing: v.Refreshing, Sites: make([]api.SiteEstimate, 0, len(v.Sites)),
		SummaryLow: nullableOf(v.Summary.Low), SummaryMid: nullableOf(v.Summary.Mid),
	}
	if v.Summary.FetchedAt != nil {
		out.SummaryFetchedAt = nullable.NewNullableWithValue(v.Summary.FetchedAt.UTC())
	} else {
		out.SummaryFetchedAt = nullable.NewNullNullable[time.Time]()
	}
	for _, e := range v.Sites {
		out.Sites = append(out.Sites, api.SiteEstimate{
			SiteId: e.SiteID, Low: nullableOf(e.Low), Mid: nullableOf(e.Mid), Count: e.Count, SuspiciousCount: e.SuspiciousCount,
			InStockCount: e.InStockCount, Status: api.EstimateStatus(e.Status), FetchedAt: e.FetchedAt.UTC(),
		})
	}
	return out
}

func toAPIListing(l item.Listing) api.Listing {
	reasons := make([]api.SuspiciousReason, 0, len(l.SuspiciousReasons))
	for _, r := range l.SuspiciousReasons {
		reasons = append(reasons, api.SuspiciousReason(r))
	}
	return api.Listing{
		Id: l.ID, SiteId: l.SiteID, Title: l.Title, Price: l.Price, Url: l.URL, ImageUrl: nullableOf(l.ImageURL),
		InStock: l.InStock, SuspiciousReasons: reasons, FetchedAt: l.FetchedAt.UTC(),
	}
}

func (s *server) GetItemEstimates(ctx context.Context, req api.GetItemEstimatesRequestObject) (api.GetItemEstimatesResponseObject, error) {
	if err := checkID(req.Id, "id"); err != nil {
		return nil, err
	}
	v, err := s.estimates.Estimates(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return api.GetItemEstimates200JSONResponse(toAPIEstimates(req.Id, v)), nil
}

func (s *server) RefreshItemEstimates(ctx context.Context, req api.RefreshItemEstimatesRequestObject) (api.RefreshItemEstimatesResponseObject, error) {
	if err := checkID(req.Id, "id"); err != nil {
		return nil, err
	}
	v, err := s.estimates.Refresh(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return api.RefreshItemEstimates202JSONResponse(toAPIEstimates(req.Id, v)), nil
}

func (s *server) ListItemListings(ctx context.Context, req api.ListItemListingsRequestObject) (api.ListItemListingsResponseObject, error) {
	if sid := req.Params.SiteId; sid != nil {
		if err := checkID(*sid, "site_id"); err != nil {
			return nil, err
		}
	}
	if err := checkID(req.Id, "id"); err != nil {
		return nil, err
	}
	ls, err := s.estimates.Listings(ctx, req.Id, req.Params.SiteId)
	if err != nil {
		return nil, err
	}
	out := make([]api.Listing, 0, len(ls))
	for _, l := range ls {
		out = append(out, toAPIListing(l))
	}
	return api.ListItemListings200JSONResponse{Listings: out}, nil
}

// ---- ジャンル・サイト ----

func (s *server) ListGenres(ctx context.Context, _ api.ListGenresRequestObject) (api.ListGenresResponseObject, error) {
	gs, err := s.items.ListGenres(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]api.Genre, 0, len(gs))
	for _, g := range gs {
		out = append(out, toAPIGenre(g))
	}
	return api.ListGenres200JSONResponse{Genres: out}, nil
}

func (s *server) CreateGenre(ctx context.Context, req api.CreateGenreRequestObject) (api.CreateGenreResponseObject, error) {
	b := req.Body
	if b == nil {
		return nil, badRequest("request body is required")
	}
	in := item.NewGenre{Name: b.Name}
	if b.QueryTemplate != nil {
		in.QueryTemplate = *b.QueryTemplate
	}
	if b.SortOrder != nil {
		in.SortOrder = *b.SortOrder
	}
	if b.SiteIds != nil {
		in.SiteIDs = *b.SiteIds
	}
	g, err := s.items.CreateGenre(ctx, in)
	if err != nil {
		return nil, err
	}
	return api.CreateGenre201JSONResponse(toAPIGenre(g)), nil
}

func (s *server) UpdateGenre(ctx context.Context, req api.UpdateGenreRequestObject) (api.UpdateGenreResponseObject, error) {
	if err := checkID(req.Id, "id"); err != nil {
		return nil, err
	}
	b := req.Body
	if b == nil {
		return nil, badRequest("request body is required")
	}
	g, err := s.items.UpdateGenre(ctx, req.Id, item.GenrePatch{Name: b.Name, QueryTemplate: b.QueryTemplate, SortOrder: b.SortOrder, SiteIDs: b.SiteIds})
	if err != nil {
		return nil, err
	}
	return api.UpdateGenre200JSONResponse(toAPIGenre(g)), nil
}

func (s *server) ListSites(ctx context.Context, _ api.ListSitesRequestObject) (api.ListSitesResponseObject, error) {
	ss, err := s.items.ListSites(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]api.Site, 0, len(ss))
	for _, x := range ss {
		out = append(out, toAPISite(x))
	}
	return api.ListSites200JSONResponse{Sites: out}, nil
}

func (s *server) CreateSite(ctx context.Context, req api.CreateSiteRequestObject) (api.CreateSiteResponseObject, error) {
	b := req.Body
	if b == nil {
		return nil, badRequest("request body is required")
	}
	in := item.NewSite{Name: b.Name, SearchURLTemplate: b.SearchUrlTemplate}
	if b.FetchType != nil {
		in.FetchType = item.FetchType(*b.FetchType)
	}
	if b.IsReference != nil {
		in.IsReference = *b.IsReference
	}
	site, err := s.items.CreateSite(ctx, in)
	if err != nil {
		return nil, err
	}
	return api.CreateSite201JSONResponse(toAPISite(site)), nil
}

func (s *server) UpdateSite(ctx context.Context, req api.UpdateSiteRequestObject) (api.UpdateSiteResponseObject, error) {
	if err := checkID(req.Id, "id"); err != nil {
		return nil, err
	}
	b := req.Body
	if b == nil {
		return nil, badRequest("request body is required")
	}
	p := item.SitePatch{Name: b.Name, SearchURLTemplate: b.SearchUrlTemplate, IsReference: b.IsReference}
	if b.FetchType != nil {
		ft := item.FetchType(*b.FetchType)
		p.FetchType = &ft
	}
	site, err := s.items.UpdateSite(ctx, req.Id, p)
	if err != nil {
		return nil, err
	}
	return api.UpdateSite200JSONResponse(toAPISite(site)), nil
}

// ---- ヘルスチェック・画像 ----

func (s *server) GetHealthz(context.Context, api.GetHealthzRequestObject) (api.GetHealthzResponseObject, error) {
	return api.GetHealthz200JSONResponse{Status: api.HealthStatusOk}, nil
}

func (s *server) GetImage(_ context.Context, req api.GetImageRequestObject) (api.GetImageResponseObject, error) {
	rc, ctype, err := s.images.Open(req.Name)
	if err != nil {
		return nil, err
	}
	cc := ImageCacheControl
	return api.GetImage200ImageResponse{Body: rc, ContentType: ctype, Headers: api.GetImage200ResponseHeaders{CacheControl: &cc}}, nil
}

// ---- multipart ----

// itemForm は multipart の商品フォーム(項目の順序に依らず読む)。
type itemForm struct {
	fields        map[string]string
	image         []byte
	hasImage      bool
	imageTooLarge bool
}

func readItemForm(r *multipart.Reader) (itemForm, error) {
	f := itemForm{fields: map[string]string{}}
	if r == nil {
		return f, badRequest("expected multipart/form-data")
	}
	for {
		part, err := r.NextPart()
		if errors.Is(err, io.EOF) {
			return f, nil
		}
		if err != nil {
			return f, wrapMultipart(err)
		}
		name := part.FormName()
		if name == "image" {
			data, err := io.ReadAll(io.LimitReader(part, storage.MaxImageBytes+1))
			if err != nil {
				return f, wrapMultipart(err)
			}
			f.hasImage = true
			f.imageTooLarge = len(data) > storage.MaxImageBytes
			if !f.imageTooLarge {
				f.image = data
			}
			continue
		}
		data, err := io.ReadAll(io.LimitReader(part, maxFieldBytes+1))
		if err != nil {
			return f, wrapMultipart(err)
		}
		if len(data) > maxFieldBytes {
			return f, badRequest("form field " + name + " is too long")
		}
		f.fields[name] = string(data)
	}
}

// wrapMultipart は multipart の読み込みエラーを API のエラーにする(上限超えは classify が 422 にする)。
func wrapMultipart(err error) error {
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) {
		return err
	}
	return badRequest("malformed multipart body")
}

// newItem はフォームを検査して NewItem にする。必須項目(genre_id・name・image)が無い・数が数でないのは 400。
// 任意の項目は空文字なら「無い」として扱う。値の規則違反(空の名前・負の価格など)は Service が 422 にする。
func (f itemForm) newItem() (item.NewItem, error) {
	var in item.NewItem
	if !f.hasImage {
		return in, badRequest("image is required")
	}
	if f.imageTooLarge {
		return in, storage.ErrTooLarge
	}
	gid, ok := f.fields["genre_id"]
	if !ok {
		return in, badRequest("genre_id is required")
	}
	id, err := strconv.ParseInt(gid, 10, 64)
	if err != nil || id < 1 {
		return in, badRequest("genre_id must be a positive integer")
	}
	in.GenreID = id
	if in.Name, ok = f.fields["name"]; !ok {
		return in, badRequest("name is required")
	}
	in.OptionText = f.optString("option_text")
	in.QueryOverride = f.optString("query_override")
	in.SourceURL = f.optString("source_url")
	if v, ok := f.fields["min_price"]; ok && v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return in, badRequest("min_price must be an integer")
		}
		in.MinPrice = &n
	}
	if v, ok := f.fields["sort_order"]; ok && v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return in, badRequest("sort_order must be an integer")
		}
		in.SortOrder = n
	}
	return in, nil
}

func (f itemForm) optString(k string) *string {
	if v := f.fields[k]; v != "" {
		return &v
	}
	return nil
}
