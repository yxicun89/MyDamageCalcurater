package refresh_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/fetcher"
	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/official"
	"example.com/pokecalc/apps/wishlist/api/internal/refresh"
)

// フェーズ4-3 公式ページの監視を夜間の更新に組み込む(docs/phase4-spec.md AC-O22〜O25)。

// fakeChecker は URL ごとの結果を返し、呼び出しを記録する。未設定の URL は unknown。
type fakeChecker struct {
	mu      sync.Mutex
	results map[string]official.Result
	calls   []string
	log     *[]string // 価格の取得との順序を確かめる共有のログ(nil 可)
	onCheck func()
}

func (f *fakeChecker) Check(ctx context.Context, rawURL string) official.Result {
	f.mu.Lock()
	f.calls = append(f.calls, rawURL)
	if f.log != nil {
		*f.log = append(*f.log, "official "+rawURL)
	}
	r, ok := f.results[rawURL]
	cb := f.onCheck
	f.mu.Unlock()
	if cb != nil {
		cb()
	}
	if !ok {
		return official.Result{State: item.OfficialUnknown, Evidence: []string{}}
	}
	return r
}

func (f *fakeChecker) called() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

// failingOfficials は保存に失敗する OfficialRepository(1 つ目の商品だけ)。
type failingOfficials struct {
	item.OfficialRepository
	failID int64
}

func (f failingOfficials) SaveOfficialCheck(ctx context.Context, itemID int64, c item.OfficialCheck) (item.OfficialStatus, error) {
	if itemID == f.failID {
		return item.OfficialStatus{}, errors.New("保存に失敗")
	}
	return f.OfficialRepository.SaveOfficialCheck(ctx, itemID, c)
}

type officialEnv struct {
	*env
	checker *fakeChecker
	svc     *refresh.Service
	watched item.Item // ON・https
	off     item.Item // OFF・https
	noURL   item.Item // ON・source_url なし(Service を通さずに作った古いデータ)
	ftp     item.Item // ON・ftp
	second  item.Item // ON・https(2 つ目)
}

func newOfficialEnv(t *testing.T) *officialEnv {
	t.Helper()
	e := newEnv(t)
	ctx := context.Background()
	mk := func(name string, watch bool, src *string) item.Item {
		it, err := e.repo.CreateItem(ctx, item.NewItem{GenreID: e.genre.ID, Name: name, ImagePath: img, SourceURL: src, WatchOfficial: watch})
		if err != nil {
			t.Fatal(err)
		}
		return it
	}
	o := &officialEnv{env: e, checker: &fakeChecker{results: map[string]official.Result{}}}
	o.watched = mk("グリス", true, ptr("https://tamashii.example/item/1/"))
	o.off = mk("ビルド", false, ptr("https://tamashii.example/item/2/"))
	o.noURL = mk("クローズ", true, nil)
	o.ftp = mk("エボル", true, ptr("ftp://tamashii.example/item/3/"))
	o.second = mk("ローグ", true, ptr("https://other.example/item/4/"))
	o.svc = refresh.New(refresh.Deps{
		Items: e.repo, Prices: e.repo, Fetchers: e.reg, Now: e.now.Now, BaseContext: context.Background(),
		Official: o.checker, Officials: e.repo,
	})
	t.Cleanup(o.svc.Wait)
	return o
}

func (o *officialEnv) status(t *testing.T, id int64) *item.OfficialStatus {
	t.Helper()
	it, err := o.repo.GetItem(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return it.Official
}

// AC-O22: RefreshAll は、WatchTarget が true の商品(ON・http(s))だけを ListItems の順に確かめ、結果を Now の時刻で保存する。
// 対象でない商品(OFF・source_url なし・http(s) でない)は取らない・状態を作らない。
func TestRefreshAll_ChecksWatchedItems(t *testing.T) {
	o := newOfficialEnv(t)
	o.checker.results["https://tamashii.example/item/1/"] = official.Result{State: item.OfficialPreorder, Evidence: []string{"予約受付中"}}
	rep, err := o.svc.RefreshAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	list, err := o.repo.ListItems(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, it := range list {
		if it.ID == o.watched.ID || it.ID == o.second.ID {
			want = append(want, *it.SourceURL)
		}
	}
	if got := o.checker.called(); !slices.Equal(got, want) {
		t.Errorf("確かめた URL = %q, want %q(ListItems の順・対象だけ)", got, want)
	}
	if rep.OfficialChecked != 2 || rep.OfficialFailed != 0 {
		t.Errorf("AllReport = %+v, want OfficialChecked 2・OfficialFailed 0", rep)
	}
	s := o.status(t, o.watched.ID)
	if s == nil || s.Status != item.OfficialPreorder || !slices.Equal(s.Evidence, []string{"予約受付中"}) || !s.CheckedAt.Equal(t0) || !s.LastAttemptAt.Equal(t0) {
		t.Errorf("保存した状態 = %+v, want preorder・[予約受付中]・%v", s, t0)
	}
	if s := o.status(t, o.second.ID); s == nil || s.Status != item.OfficialUnknown {
		t.Errorf("2 つ目 = %+v, want unknown", s)
	}
	for _, id := range []int64{o.off.ID, o.noURL.ID, o.ftp.ID, o.it.ID} {
		if s := o.status(t, id); s != nil {
			t.Errorf("対象でない商品 %d に状態がある: %+v", id, s)
		}
	}
}

// AC-O23: failed は OfficialFailed に数え、判定済みの状態を上書きしない(MergeOfficial)。blocked は失敗に数えない。
// 保存の失敗はログに出して次の商品へ進み、OfficialFailed に数える。どれも AllReport.Failed には数えない。
func TestRefreshAll_OfficialFailures(t *testing.T) {
	o := newOfficialEnv(t)
	u1, u2 := *o.watched.SourceURL, *o.second.SourceURL
	o.checker.results[u1] = official.Result{State: item.OfficialAvailable, Evidence: []string{"販売中"}}
	o.checker.results[u2] = official.Result{State: item.OfficialBlocked, Evidence: []string{}}
	if _, err := o.svc.RefreshAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	o.now.Set(t0.Add(24 * time.Hour))
	o.checker.results[u1] = official.Result{State: item.OfficialFailed, Evidence: []string{}}
	rep, err := o.svc.RefreshAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.OfficialChecked != 2 || rep.OfficialFailed != 1 {
		t.Errorf("AllReport = %+v, want OfficialChecked 2・OfficialFailed 1(blocked は数えない)", rep)
	}
	failedBefore := rep.Failed
	s := o.status(t, o.watched.ID)
	if s == nil || s.Status != item.OfficialAvailable || s.LastResult != item.OfficialFailed || !s.CheckedAt.Equal(t0) || !s.LastAttemptAt.Equal(t0.Add(24*time.Hour)) {
		t.Errorf("failed のあと = %+v, want status available のまま・last_result failed", s)
	}
	if s := o.status(t, o.second.ID); s == nil || s.Status != item.OfficialBlocked {
		t.Errorf("blocked = %+v", s)
	}

	// 保存の失敗
	f := newOfficialEnv(t)
	f.svc = refresh.New(refresh.Deps{
		Items: f.repo, Prices: f.repo, Fetchers: f.reg, Now: f.now.Now, BaseContext: context.Background(),
		Official: f.checker, Officials: failingOfficials{OfficialRepository: f.repo, failID: f.watched.ID},
	})
	t.Cleanup(f.svc.Wait)
	rep, err = f.svc.RefreshAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.OfficialChecked != 2 || rep.OfficialFailed != 1 || rep.Failed != failedBefore {
		t.Errorf("保存の失敗: AllReport = %+v, want OfficialChecked 2・OfficialFailed 1・Failed %d(価格の失敗だけ)", rep, failedBefore)
	}
	if s := f.status(t, f.second.ID); s == nil {
		t.Error("保存に失敗した商品の次の商品を確かめていない")
	}
}

// AC-O24: 公式ページは夜間の RefreshAll だけで取る。RefreshItem(全モード)・Estimates・Refresh では取らない。
// 価格の更新をすべて終えてから確かめる。
func TestRefreshAll_OfficialOnlyNightlyAndAfterPrices(t *testing.T) {
	o := newOfficialEnv(t)
	var log []string
	var mu sync.Mutex
	o.checker.log = &log
	o.f.set(o.shop.ID, []fetcher.Listing{l("グリス", 5000)}, nil)
	ctx := context.Background()
	for _, m := range []refresh.Mode{refresh.ModeStale, refresh.ModeAll, refresh.ModeNightly} {
		if _, err := o.svc.RefreshItem(ctx, o.watched.ID, m); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := o.svc.Estimates(ctx, o.watched.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := o.svc.Refresh(ctx, o.watched.ID); err != nil {
		t.Fatal(err)
	}
	o.svc.Wait()
	if got := o.checker.called(); len(got) != 0 {
		t.Fatalf("RefreshItem・Estimates・Refresh で公式ページを取った: %q", got)
	}

	o.f.takeCalls()
	// Check ごとに、その直前までの価格の取得回数を記録する(takeCalls は記録を空にするので上書きしない)
	var perCheck []int
	o.checker.onCheck = func() {
		mu.Lock()
		defer mu.Unlock()
		perCheck = append(perCheck, len(o.f.takeCalls()))
	}
	if _, err := o.svc.RefreshAll(ctx); err != nil {
		t.Fatal(err)
	}
	if len(o.checker.called()) != 2 {
		t.Fatalf("RefreshAll で確かめた数 = %d, want 2", len(o.checker.called()))
	}
	mu.Lock()
	defer mu.Unlock()
	// 最初の Check の時点で、全商品の価格の取得が終わっている(Check のあとに価格の取得が無い)
	if rest := o.f.takeCalls(); len(rest) != 0 {
		t.Errorf("公式ページの確認のあとに価格を取った: %v", rest)
	}
	if len(perCheck) == 0 || perCheck[0] == 0 {
		t.Errorf("公式ページの確認より前に価格を取っていない: %v", perCheck)
	}
	for i, n := range perCheck[1:] {
		if n != 0 {
			t.Errorf("%d 回目の確認の前に価格を取った(確認の間に価格の取得が挟まった): %v", i+2, perCheck)
		}
	}
}

// AC-O25: Official が nil なら監視しない(従来どおり)。ctx が終わったら止めて ctx の err を返す。
func TestRefreshAll_OfficialNilAndCancel(t *testing.T) {
	o := newOfficialEnv(t)
	plain := refresh.New(refresh.Deps{Items: o.repo, Prices: o.repo, Fetchers: o.reg, Now: o.now.Now, BaseContext: context.Background()})
	t.Cleanup(plain.Wait)
	rep, err := plain.RefreshAll(context.Background())
	if err != nil || rep.OfficialChecked != 0 {
		t.Errorf("Official なし: %+v %v", rep, err)
	}
	if s := o.status(t, o.watched.ID); s != nil {
		t.Errorf("Official なしで状態ができた: %+v", s)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	o.checker.onCheck = cancel // 1 つ目を確かめたら止める
	if _, err := o.svc.RefreshAll(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if n := len(o.checker.called()); n != 1 {
		t.Errorf("止めたあとも確かめた(%d 回)", n)
	}
}
