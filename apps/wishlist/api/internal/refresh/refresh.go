// Package refresh は商品の目安価格の更新(取得 → 参考外の判定 → 保存)と、API 用の読み出し(apps/wishlist/CLAUDE.md §6・§8)。
// 受け入れ条件は docs/phase3-api-spec.md の AC-U*。
//
// 更新の流れ(RefreshItem):
//  1. 対象サイト = 商品のジャンルの site_ids(表示順)から、override で enabled=false のものと、Fetcher が無い(link_only・未実装・appid なし)ものを除く
//  2. 対象サイトを順番に(並列にしない)取得する。検索ワードは query.Build(テンプレート・query_override・サイト別 query)。
//     取得結果は先頭 fetcher.MaxListings 件まで。保存できない値は合わせる(タイトルは item.MaxListingTitleLen 文字に切り詰め、
//     URL が item.MaxListingURLLen 文字を超える出品は捨て、image_url が長すぎる・空なら nil)
//  3. すべて取り終えてから基準価格を決め(estimate.Evaluate。基準に使うのは is_reference かつ fetch_type が api / scrape のサイト。
//     今回取得に成功しなかった〈ModeStale で取らなかった・失敗した〉基準サイトは、保存済みの出品を基準の計算にだけ使う)、成功したサイトごとに SaveSiteResult、失敗したサイトは MarkFailed
//  4. 同じ商品の更新は同時に 1 つだけ(実行中に呼ばれたら、その完了を待って同じ結果を返す)
package refresh

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
	"unicode/utf8"

	"example.com/pokecalc/apps/wishlist/api/internal/estimate"
	"example.com/pokecalc/apps/wishlist/api/internal/fetcher"
	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/official"
	"example.com/pokecalc/apps/wishlist/api/internal/query"
)

const (
	// DefaultMaxAge は目安のキャッシュの有効期限(仕様 §6)。これより古ければ GET で裏の更新を起動する。
	DefaultMaxAge = 24 * time.Hour
	// DefaultRetryInterval は、取得に失敗した・古いサイトを GET の裏の更新で取り直すまでの最短間隔(プロセス内で覚える)。
	DefaultRetryInterval = time.Hour
)

// 裏の更新(api の Estimates・Refresh が起動するもの)を起動しない時間帯。毎日 03:00 JST の CronJob と、別のプロセスから
// 同じサイトへ 5 秒以内に重ならないようにする軽い対策(5 秒の間隔はプロセス内でしか守れない)。[QuietStart, QuietEnd) JST。
const (
	quietStartMinute = 2*60 + 30
	quietEndMinute   = 4 * 60
)

// jst は固定オフセット(scratch イメージに tzdata が無いため LoadLocation を使わない)。
var jst = time.FixedZone("JST", 9*60*60)

// InQuietWindow は t が裏の更新を起動しない時間帯(02:30 以上 04:00 未満 JST)か。
func InQuietWindow(t time.Time) bool {
	t = t.In(jst)
	m := t.Hour()*60 + t.Minute()
	return m >= quietStartMinute && m < quietEndMinute
}

// Mode は更新の範囲。
type Mode int

const (
	// ModeStale は「目安が無い・failed・MaxAge より古い」対象サイトのうち、最後に試してから RetryInterval 以上たったものだけ取る(GET の裏の更新)。
	ModeStale Mode = iota
	// ModeAll はすべての対象サイトを取る(手動の更新・CronJob)。
	ModeAll
	// ModeNightly は ModeAll に加えて、夜間だけ取るサイト(fetcher.NightlyOnlyHosts。駿河屋)も取る(CronJob の RefreshAll だけ)。
	// ModeStale・ModeAll では夜間専用のサイトは取らずに飛ばす(前回値は残す)。
	ModeNightly
)

// Deps は New の依存。
type Deps struct {
	Items    item.Repository
	Prices   item.PriceRepository
	Fetchers *fetcher.Registry
	Now      func() time.Time // nil なら time.Now
	// MaxAge は 0 なら DefaultMaxAge、RetryInterval は 0 なら DefaultRetryInterval。
	MaxAge        time.Duration
	RetryInterval time.Duration
	Logger        *slog.Logger    // nil なら slog.Default()
	BaseContext   context.Context // 裏の更新に使う ctx(リクエストの ctx は使わない)。nil なら context.Background()
	// OnJoin は、実行中の更新に合流した(新しく取得を始めなかった)RefreshItem が待ち始める直前に呼ばれる(テスト用。nil で何もしない)。
	OnJoin func(itemID int64)
	// Official は公式ページの販売状況の取得と判定(フェーズ4-3。本番は refresher だけが *official.Checker を渡す)。
	// nil なら監視しない(api のプロセス)。RefreshAll だけが使い、RefreshItem・Estimates・Refresh は使わない。
	Official OfficialChecker
	// Officials は販売状況の保存先(Official があるときは必須)。
	Officials item.OfficialRepository
}

// OfficialChecker は公式ページを取得して判定する(official.Checker)。エラーは返さず状態で表す。
type OfficialChecker interface {
	Check(ctx context.Context, rawURL string) official.Result
}

// Report は 1 商品の更新結果。
type Report struct {
	Targets int // 対象サイトの数(ModeStale で取らなかったものも含む)
	Fetched int // 取得に成功したサイトの数
	Failed  int // 取得に失敗したサイトの数
}

// AllReport は RefreshAll の結果。
type AllReport struct {
	Items  int // 処理した商品の数
	Failed int // 失敗した商品の数(RefreshItem がエラー、または対象サイトがあって 1 つも取得できなかった)
	// 公式ページの監視(フェーズ4-3)。OfficialChecked は確かめた商品の数(item.WatchTarget が true のもの)、
	// OfficialFailed はそのうち結果が failed だった・保存に失敗した数(blocked は数えない)。Failed には含めない。
	OfficialChecked int
	OfficialFailed  int
}

// View は API に返す目安。
type View struct {
	// Sites は対象サイトのうち目安を保存済みのもの(ジャンルの表示順)。
	Sites []item.Estimate
	// Summary は Sites から estimate.Summarize で作る。
	Summary estimate.Summary
	// Refreshing は裏の更新を起動した、または実行中か。
	Refreshing bool
}

// Service は更新と読み出し。
type Service struct {
	d Deps

	mu        sync.Mutex
	inflight  map[int64]*flight      // 実行中の更新(商品ごとに 1 つ。singleflight)
	lastTried map[[2]int64]time.Time // (item_id, site_id) を最後に試した時刻(プロセス内だけ)
	wg        sync.WaitGroup         // 裏の更新
}

// flight は実行中の 1 商品の更新。完了すると done が閉じ、report・err が確定する。
type flight struct {
	done   chan struct{}
	report Report
	err    error
}

// New は Service を返す。
func New(d Deps) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.MaxAge == 0 {
		d.MaxAge = DefaultMaxAge
	}
	if d.RetryInterval == 0 {
		d.RetryInterval = DefaultRetryInterval
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	if d.BaseContext == nil {
		d.BaseContext = context.Background()
	}
	return &Service{d: d, inflight: map[int64]*flight{}, lastTried: map[[2]int64]time.Time{}}
}

// target は更新の対象サイト(表示順)。
type target struct {
	site        item.Site
	f           fetcher.Fetcher
	query       string
	nightlyOnly bool
}

// targets は商品の対象サイトを返す。商品が無ければ item.ErrNotFound。
func (s *Service) targets(ctx context.Context, itemID int64) (item.Item, []target, [][]string, error) {
	it, err := s.d.Items.GetItem(ctx, itemID)
	if err != nil {
		return item.Item{}, nil, nil, err
	}
	genres, err := s.d.Items.ListGenres(ctx)
	if err != nil {
		return item.Item{}, nil, nil, err
	}
	var genre *item.Genre
	for i := range genres {
		if genres[i].ID == it.GenreID {
			genre = &genres[i]
		}
	}
	if genre == nil {
		return it, nil, nil, nil
	}
	sites, err := s.d.Items.ListSites(ctx)
	if err != nil {
		return item.Item{}, nil, nil, err
	}
	byID := make(map[int64]item.Site, len(sites))
	for _, st := range sites {
		byID[st.ID] = st
	}
	overrides := make(map[int64]item.SiteOverride, len(it.SiteOverrides))
	for _, o := range it.SiteOverrides {
		overrides[o.SiteID] = o
	}
	var out []target
	for _, id := range genre.SiteIDs {
		st, ok := byID[id]
		if !ok {
			continue
		}
		o, hasOverride := overrides[id]
		if hasOverride && !o.Enabled {
			continue
		}
		f, ok := s.d.Fetchers.ForSite(st)
		if !ok {
			continue
		}
		var siteQuery *string
		if hasOverride {
			siteQuery = o.Query
		}
		opt := ""
		if it.OptionText != nil {
			opt = *it.OptionText
		}
		out = append(out, target{site: st, f: f, nightlyOnly: s.d.Fetchers.NightlyOnly(st), query: query.Build(genre.QueryTemplate, it.Name, opt, it.QueryOverride, siteQuery)})
	}
	return it, out, genre.Aliases, nil
}

// needsFetch は t を今回取るべきか。ModeAll は常に取る。ModeStale は「目安が無い・failed・MaxAge より古い」うち、
// 最後に試してから RetryInterval 以上たったものだけ。
func (s *Service) needsFetch(itemID int64, t target, est *item.Estimate, mode Mode, now time.Time) bool {
	if t.nightlyOnly && mode != ModeNightly {
		return false
	}
	if mode == ModeAll || mode == ModeNightly {
		return true
	}
	stale := est == nil || est.Status == item.EstimateFailed || now.Sub(est.FetchedAt) > s.d.MaxAge
	if !stale {
		return false
	}
	s.mu.Lock()
	last, tried := s.lastTried[[2]int64{itemID, t.site.ID}]
	s.mu.Unlock()
	return !tried || now.Sub(last) >= s.d.RetryInterval
}

func (s *Service) markTried(itemID, siteID int64, at time.Time) {
	s.mu.Lock()
	s.lastTried[[2]int64{itemID, siteID}] = at
	s.mu.Unlock()
}

// begin は商品の更新の登録を試みる。実行中なら (その flight, false)、無ければ新しく登録して (flight, true)。
func (s *Service) begin(itemID int64) (*flight, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, ok := s.inflight[itemID]; ok {
		return f, false
	}
	f := &flight{done: make(chan struct{})}
	s.inflight[itemID] = f
	return f, true
}

// lead は begin で leader になった呼び出しが更新を実行して完了を知らせる。
func (s *Service) lead(ctx context.Context, itemID int64, f *flight, mode Mode) {
	f.report, f.err = s.run(ctx, itemID, mode)
	s.mu.Lock()
	delete(s.inflight, itemID)
	s.mu.Unlock()
	close(f.done)
}

// RefreshItem は 1 商品を同期的に更新する。商品が無ければ item.ErrNotFound。取得の失敗はエラーにせず Report に数える。
// 同じ商品の更新が実行中なら、取得を重ねずその完了を待って同じ結果を返す。
func (s *Service) RefreshItem(ctx context.Context, itemID int64, mode Mode) (Report, error) {
	f, leader := s.begin(itemID)
	if leader {
		s.lead(ctx, itemID, f, mode)
		return f.report, f.err
	}
	if s.d.OnJoin != nil {
		s.d.OnJoin(itemID)
	}
	select {
	case <-f.done:
		return f.report, f.err
	case <-ctx.Done():
		return Report{}, ctx.Err()
	}
}

// startBackground は裏で更新を起動する(実行中なら起動しない)。起動しない時間帯(InQuietWindow)は何もせず false を返す。
func (s *Service) startBackground(itemID int64, mode Mode) bool {
	if InQuietWindow(s.d.Now()) {
		return false
	}
	f, leader := s.begin(itemID)
	if !leader {
		return true
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.lead(s.d.BaseContext, itemID, f, mode)
		if f.err != nil && !errors.Is(f.err, context.Canceled) {
			s.d.Logger.Error("background refresh failed", "item_id", itemID, "error", f.err)
		}
	}()
	return true
}

func (s *Service) running(itemID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.inflight[itemID]
	return ok
}

// run は 1 商品の更新の本体(singleflight の外側から呼ばない)。
func (s *Service) run(ctx context.Context, itemID int64, mode Mode) (Report, error) {
	it, targets, aliases, err := s.targets(ctx, itemID)
	if err != nil {
		return Report{}, err
	}
	report := Report{Targets: len(targets)}
	ests, err := s.d.Prices.ListEstimates(ctx, itemID)
	if err != nil {
		return report, err
	}
	estBySite := make(map[int64]*item.Estimate, len(ests))
	for i := range ests {
		estBySite[ests[i].SiteID] = &ests[i]
	}

	type outcome struct {
		t        target
		fetched  bool
		failed   bool
		listings []fetcher.Listing
	}
	outs := make([]outcome, 0, len(targets))
	for _, t := range targets {
		o := outcome{t: t}
		if s.needsFetch(itemID, t, estBySite[t.site.ID], mode, s.d.Now()) {
			s.markTried(itemID, t.site.ID, s.d.Now())
			ls, err := t.f.Fetch(ctx, t.site, t.query)
			switch {
			case err == nil:
				o.fetched, o.listings = true, sanitize(ls)
			case ctx.Err() != nil:
				return report, ctx.Err()
			default:
				o.failed = true
				s.d.Logger.Warn("fetch failed", "item_id", itemID, "site_id", t.site.ID, "error", err)
			}
		}
		outs = append(outs, o)
	}

	// 基準価格は全対象を取り終えてから決める。今回取得に成功しなかった基準サイトは保存済みの出品を計算にだけ使う。
	inputs := make([]estimate.SiteInput, 0, len(outs))
	for _, o := range outs {
		in := estimate.SiteInput{SiteID: o.t.site.ID, Reference: isReference(o.t.site)}
		switch {
		case o.fetched:
			in.Listings = toEstimateListings(o.listings)
		case in.Reference:
			stored, err := s.d.Prices.ListListings(ctx, itemID, &o.t.site.ID)
			if err != nil {
				return report, err
			}
			for _, l := range stored {
				in.Listings = append(in.Listings, estimate.Listing{Title: l.Title, Price: l.Price, InStock: l.InStock})
			}
		}
		inputs = append(inputs, in)
	}
	result := estimate.Evaluate(estimate.Item{Name: it.Name, MinPrice: it.MinPrice, Aliases: aliases}, inputs)

	now := s.d.Now()
	for i, o := range outs {
		switch {
		case o.failed:
			report.Failed++
			if err := s.d.Prices.MarkFailed(ctx, itemID, o.t.site.ID, now); err != nil {
				return report, err
			}
		case o.fetched:
			report.Fetched++
			sr := result.Sites[i]
			saved := make([]item.Listing, 0, len(o.listings))
			for j, l := range o.listings {
				var img *string
				if l.ImageURL != "" {
					img = &l.ImageURL
				}
				reasons := make([]string, 0, len(sr.Reasons[j]))
				for _, r := range sr.Reasons[j] {
					reasons = append(reasons, string(r))
				}
				saved = append(saved, item.Listing{Title: l.Title, Price: l.Price, URL: l.URL, ImageURL: img, InStock: l.InStock, SuspiciousReasons: reasons})
			}
			e := item.Estimate{
				ItemID: itemID, SiteID: o.t.site.ID, Low: sr.Estimate.Low, Mid: sr.Estimate.Mid, Count: sr.Estimate.Count,
				SuspiciousCount: sr.Estimate.SuspiciousCount, InStockCount: sr.Estimate.InStockCount,
				Status: item.EstimateStatus(sr.Estimate.Status), FetchedAt: now,
			}
			if err := s.d.Prices.SaveSiteResult(ctx, e, saved); err != nil {
				return report, err
			}
		}
	}
	return report, nil
}

// isReference は基準価格の算出に使うサイトか(is_reference かつ fetch_type が api / scrape)。
func isReference(st item.Site) bool {
	return st.IsReference && (st.FetchType == item.FetchAPI || st.FetchType == item.FetchScrape)
}

func toEstimateListings(ls []fetcher.Listing) []estimate.Listing {
	out := make([]estimate.Listing, 0, len(ls))
	for _, l := range ls {
		out = append(out, estimate.Listing{Title: l.Title, Price: l.Price, InStock: l.InStock})
	}
	return out
}

// sanitize は取得結果を先頭 MaxListings 件に絞り、保存できる値に合わせる(長すぎる URL の出品は捨てる)。
func sanitize(ls []fetcher.Listing) []fetcher.Listing {
	if len(ls) > fetcher.MaxListings {
		ls = ls[:fetcher.MaxListings]
	}
	out := make([]fetcher.Listing, 0, len(ls))
	for _, l := range ls {
		if utf8.RuneCountInString(l.URL) > item.MaxListingURLLen || l.Price < 0 {
			continue
		}
		if utf8.RuneCountInString(l.Title) > item.MaxListingTitleLen {
			l.Title = string([]rune(l.Title)[:item.MaxListingTitleLen])
		}
		if utf8.RuneCountInString(l.ImageURL) > item.MaxListingURLLen {
			l.ImageURL = ""
		}
		out = append(out, l)
	}
	return out
}

// RefreshAll は全商品を順番に ModeNightly で更新する。1 商品の失敗で止めない(ctx が終わったときだけ止める)。
// フェーズ4-3(TODO implementer):価格をすべて更新したあと、Deps.Official があれば item.WatchTarget が true の商品を順番に
// Official.Check して Officials.SaveOfficialCheck で保存する(docs/phase4-spec.md AC-O22〜O25)。
func (s *Service) RefreshAll(ctx context.Context) (AllReport, error) {
	s.pruneHistory(ctx)
	items, err := s.d.Items.ListItems(ctx, nil)
	if err != nil {
		return AllReport{}, err
	}
	var r AllReport
	for _, it := range items {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		rep, err := s.RefreshItem(ctx, it.ID, ModeNightly)
		r.Items++
		switch {
		case err != nil:
			r.Failed++
			s.d.Logger.Error("refresh item failed", "item_id", it.ID, "error", err)
		case rep.Targets > 0 && rep.Fetched == 0:
			r.Failed++
		}
	}
	return r, nil
}

// view は保存済みの目安から View(Refreshing 以外)を作る。Sites は対象サイトのうち保存済みのもの(表示順)。
func (s *Service) view(ctx context.Context, itemID int64, targets []target) (View, []item.Estimate, error) {
	ests, err := s.d.Prices.ListEstimates(ctx, itemID)
	if err != nil {
		return View{}, nil, err
	}
	bySite := make(map[int64]item.Estimate, len(ests))
	for _, e := range ests {
		bySite[e.SiteID] = e
	}
	v := View{Sites: []item.Estimate{}}
	var in []estimate.SummaryInput
	for _, t := range targets {
		e, ok := bySite[t.site.ID]
		if !ok {
			continue
		}
		v.Sites = append(v.Sites, e)
		in = append(in, estimate.SummaryInput{Low: e.Low, Mid: e.Mid, FetchedAt: e.FetchedAt})
	}
	v.Summary = estimate.Summarize(in)
	return v, ests, nil
}

// Estimates は保存済みの目安を返す。ModeStale で取るべき対象サイトがあれば裏で更新を起動して Refreshing を true にする
// (実行中も true)。対象サイトが無ければ Refreshing は false。商品が無ければ item.ErrNotFound。
func (s *Service) Estimates(ctx context.Context, itemID int64) (View, error) {
	_, targets, _, err := s.targets(ctx, itemID)
	if err != nil {
		return View{}, err
	}
	v, ests, err := s.view(ctx, itemID, targets)
	if err != nil {
		return View{}, err
	}
	if len(targets) == 0 {
		return v, nil
	}
	if s.running(itemID) {
		v.Refreshing = true
		return v, nil
	}
	bySite := make(map[int64]*item.Estimate, len(ests))
	for i := range ests {
		bySite[ests[i].SiteID] = &ests[i]
	}
	now := s.d.Now()
	for _, t := range targets {
		if s.needsFetch(itemID, t, bySite[t.site.ID], ModeStale, now) {
			v.Refreshing = s.startBackground(itemID, ModeStale)
			break
		}
	}
	return v, nil
}

// Refresh は裏で ModeAll の更新を起動し(実行中なら起動しない)、保存済みの目安を返す。
// Refreshing は夜間専用でない対象サイトがあれば true。商品が無ければ item.ErrNotFound。
func (s *Service) Refresh(ctx context.Context, itemID int64) (View, error) {
	_, targets, _, err := s.targets(ctx, itemID)
	if err != nil {
		return View{}, err
	}
	v, _, err := s.view(ctx, itemID, targets)
	if err != nil {
		return View{}, err
	}
	for _, t := range targets {
		if !t.nightlyOnly { // 夜間専用のサイトだけなら、取るものが無いので起動しない
			v.Refreshing = s.startBackground(itemID, ModeAll)
			break
		}
	}
	return v, nil
}

// Listings は保存済みの出品(参考外を含む。PriceRepository.ListListings の順)。商品が無ければ item.ErrNotFound。
func (s *Service) Listings(ctx context.Context, itemID int64, siteID *int64) ([]item.Listing, error) {
	if _, err := s.d.Items.GetItem(ctx, itemID); err != nil {
		return nil, err
	}
	return s.d.Prices.ListListings(ctx, itemID, siteID)
}

// Wait は裏の更新がすべて終わるまで待つ(テストと終了処理用)。
func (s *Service) Wait() { s.wg.Wait() }
