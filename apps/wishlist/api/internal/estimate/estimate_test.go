package estimate_test

import (
	"slices"
	"testing"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/estimate"
)

func ip(i int) *int { return &i }

func eqIntPtr(a, b *int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func fmtPtr(p *int) any {
	if p == nil {
		return "nil"
	}
	return *p
}

// AC-E1: タイトル照合。name を空白で分けた各トークンを Normalize し、正規化したタイトルにすべて含まれるか。
func TestTitleMatches(t *testing.T) {
	cases := []struct {
		name, item, title string
		want              bool
	}{
		{"1 トークン", "グリス", "S.H.Figuarts 仮面ライダーグリス 開封品", true},
		{"複数トークン・順不同", "ボルシャック 銀トレジャー", "【銀トレジャー】ボルシャック・ドラゴン", true},
		{"1 つ欠ける", "ボルシャック 銀トレジャー", "ボルシャック・ドラゴン 通常版", false},
		{"全く含まない", "ボルシャック 銀トレジャー", "まとめ売り カード 100 枚", false},
		{"大文字小文字・全角英数(NFKC)", "HG ガンダムエアリアル", "ＨＧ　ガンダム・エアリアル 1/144", true},
		{"記号の表記揺れ(S.H.Figuarts と SHFiguarts)", "S.H.Figuarts グリス", "SHFiguarts グリス", true},
		{"カタカナ表記は辞書が無いので一致しない(フェーズ2)", "S.H.Figuarts グリス", "S.H.フィギュアーツ グリス", false},
		{"全角空白・タブでも分ける", "ボルシャック　銀トレジャー\t", "銀トレジャー ボルシャック", true},
		{"正規化で空になるトークンは無視", "グリス -", "仮面ライダーグリス", true},
		{"トークンが無ければ true", "  ", "なんでも", true},
		{"タイトルが空", "グリス", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := estimate.TitleMatches(c.item, c.title); got != c.want {
				t.Errorf("TitleMatches(%q, %q) = %v, want %v", c.item, c.title, got, c.want)
			}
		})
	}
}

// AC-E2: 参考外の判定(理由の順は title_mismatch → too_cheap → below_min)。
func TestJudge(t *testing.T) {
	const name = "ボルシャック 銀トレジャー"
	ok := "ボルシャック 銀トレジャー 美品"
	cases := []struct {
		name string
		min  *int
		base *int
		l    estimate.Listing
		want []estimate.Reason
	}{
		{"参考にする", nil, ip(4900), estimate.Listing{Title: ok, Price: 4500}, nil},
		{"title_mismatch", nil, nil, estimate.Listing{Title: "まとめ売り", Price: 4500}, []estimate.Reason{estimate.ReasonTitleMismatch}},
		{"too_cheap(1469 < 4900×0.3=1470)", nil, ip(4900), estimate.Listing{Title: ok, Price: 1469}, []estimate.Reason{estimate.ReasonTooCheap}},
		{"too_cheap の境界(1470 は参考)", nil, ip(4900), estimate.Listing{Title: ok, Price: 1470}, nil},
		{"too_cheap の境界(base 1000・300 は参考)", nil, ip(1000), estimate.Listing{Title: ok, Price: 300}, nil},
		{"too_cheap(base 1000・299)", nil, ip(1000), estimate.Listing{Title: ok, Price: 299}, []estimate.Reason{estimate.ReasonTooCheap}},
		{"基準価格が無ければ too_cheap を判定しない", nil, nil, estimate.Listing{Title: ok, Price: 1}, nil},
		{"below_min", ip(3000), nil, estimate.Listing{Title: ok, Price: 2999}, []estimate.Reason{estimate.ReasonBelowMin}},
		{"below_min の境界(min ちょうどは参考)", ip(3000), nil, estimate.Listing{Title: ok, Price: 3000}, nil},
		{"min_price 未設定なら below_min を判定しない", nil, nil, estimate.Listing{Title: ok, Price: 0}, nil},
		{"3 つすべて(順序)", ip(3000), ip(4900), estimate.Listing{Title: "まとめ売り", Price: 300}, []estimate.Reason{estimate.ReasonTitleMismatch, estimate.ReasonTooCheap, estimate.ReasonBelowMin}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := estimate.Judge(estimate.Item{Name: name, MinPrice: c.min}, c.base, c.l)
			if len(got) != len(c.want) || (len(c.want) > 0 && !slices.Equal(got, c.want)) {
				t.Errorf("Judge = %v, want %v", got, c.want)
			}
		})
	}
}

// AC-E3: 中央値(偶数件は中央 2 つの平均を切り捨て)。入力を書き換えない。
func TestMedian(t *testing.T) {
	cases := []struct {
		name string
		in   []int
		want *int
	}{
		{"空", nil, nil},
		{"1 件", []int{5}, ip(5)},
		{"仕様 §6 の例(5000・4800 → 4900)", []int{5000, 4800}, ip(4900)},
		{"偶数件は切り捨て(1・2 → 1)", []int{2, 1}, ip(1)},
		{"奇数件", []int{4, 1, 2}, ip(2)},
		{"偶数件・未整列", []int{3, 10, 1, 2}, ip(2)},
		{"同額", []int{7, 7, 7, 7}, ip(7)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := slices.Clone(c.in)
			got := estimate.Median(in)
			if !eqIntPtr(got, c.want) {
				t.Errorf("Median(%v) = %v, want %v", c.in, fmtPtr(got), fmtPtr(c.want))
			}
			if !slices.Equal(in, c.in) {
				t.Errorf("入力が書き換えられた: %v → %v", c.in, in)
			}
		})
	}
}

func listings(prices ...int) []estimate.Listing {
	out := make([]estimate.Listing, 0, len(prices))
	for _, p := range prices {
		out = append(out, estimate.Listing{Title: "x", Price: p, InStock: true})
	}
	return out
}

// AC-E4: サイトの目安。low は nearest-rank の 25 パーセンタイル(昇順で ceil(n/4) 番目)、mid は中央値。
// 3 件未満なら low は最小値・mid は nil。0 件なら no_result。
func TestEstimate(t *testing.T) {
	cases := []struct {
		name     string
		in       []estimate.Listing
		low, mid *int
		status   estimate.Status
	}{
		{"0 件", nil, nil, nil, estimate.StatusNoResult},
		{"1 件", listings(500), ip(500), nil, estimate.StatusOK},
		{"2 件(最小値だけ)", listings(500, 300), ip(300), nil, estimate.StatusOK},
		{"3 件(ceil(3/4)=1 番目)", listings(300, 100, 200), ip(100), ip(200), estimate.StatusOK},
		{"4 件", listings(400, 300, 200, 100), ip(100), ip(250), estimate.StatusOK},
		{"5 件(ceil(5/4)=2 番目)", listings(500, 100, 400, 200, 300), ip(200), ip(300), estimate.StatusOK},
		{"8 件", listings(800, 700, 600, 500, 400, 300, 200, 100), ip(200), ip(450), estimate.StatusOK},
		{"9 件(ceil(9/4)=3 番目)", listings(900, 800, 700, 600, 500, 400, 300, 200, 100), ip(300), ip(500), estimate.StatusOK},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := estimate.Estimate(c.in, 2)
			if !eqIntPtr(got.Low, c.low) || !eqIntPtr(got.Mid, c.mid) || got.Status != c.status {
				t.Errorf("low/mid/status = %v/%v/%q, want %v/%v/%q", fmtPtr(got.Low), fmtPtr(got.Mid), got.Status, fmtPtr(c.low), fmtPtr(c.mid), c.status)
			}
			if got.Count != len(c.in) || got.SuspiciousCount != 2 {
				t.Errorf("count/suspicious = %d/%d, want %d/2", got.Count, got.SuspiciousCount, len(c.in))
			}
		})
	}
}

// AC-E5: 在庫ありの件数(参考外を除いた分のうち InStock)。
func TestEstimate_InStockCount(t *testing.T) {
	in := []estimate.Listing{{Title: "a", Price: 100, InStock: true}, {Title: "b", Price: 200, InStock: false}, {Title: "c", Price: 300, InStock: true}}
	if got := estimate.Estimate(in, 0); got.InStockCount != 2 || got.Count != 3 {
		t.Errorf("in_stock/count = %d/%d, want 2/3", got.InStockCount, got.Count)
	}
}

func findSite(t *testing.T, r estimate.Result, id int64) estimate.SiteResult {
	t.Helper()
	for _, s := range r.Sites {
		if s.SiteID == id {
			return s
		}
	}
	t.Fatalf("site %d の結果が無い: %+v", id, r)
	return estimate.SiteResult{}
}

// AC-E6: 仕様 §6 の例。カードラッシュ ¥5,000 / ドラゴンスター ¥4,800 / メルカリ ¥300(タイトルに商品名なし)
// → 基準価格 ¥4,900。メルカリの ¥300 は title_mismatch と too_cheap で参考外。
func TestEvaluate_SpecExample(t *testing.T) {
	const cardrush, dragonstar, mercari = 11, 12, 13
	item := estimate.Item{Name: "ボルシャック 銀トレジャー"}
	r := estimate.Evaluate(item, []estimate.SiteInput{
		{SiteID: cardrush, Reference: true, Listings: []estimate.Listing{{Title: "ボルシャック・ドラゴン 銀トレジャー", Price: 5000, InStock: true}}},
		{SiteID: dragonstar, Reference: true, Listings: []estimate.Listing{{Title: "【銀トレジャー】ボルシャック", Price: 4800, InStock: true}}},
		{SiteID: mercari, Reference: false, Listings: []estimate.Listing{{Title: "まとめ売り カード", Price: 300, InStock: true}}},
	})
	if !eqIntPtr(r.BasePrice, ip(4900)) {
		t.Errorf("BasePrice = %v, want 4900", fmtPtr(r.BasePrice))
	}
	if len(r.Sites) != 3 || r.Sites[0].SiteID != cardrush || r.Sites[1].SiteID != dragonstar || r.Sites[2].SiteID != mercari {
		t.Fatalf("Sites の順が入力と違う: %+v", r.Sites)
	}
	m := findSite(t, r, mercari)
	if len(m.Reasons) != 1 || !slices.Equal(m.Reasons[0], []estimate.Reason{estimate.ReasonTitleMismatch, estimate.ReasonTooCheap}) {
		t.Errorf("メルカリの理由 = %v, want [[title_mismatch too_cheap]]", m.Reasons)
	}
	if m.Estimate.Status != estimate.StatusNoResult || m.Estimate.Count != 0 || m.Estimate.SuspiciousCount != 1 || m.Estimate.Low != nil {
		t.Errorf("メルカリの目安 = %+v, want no_result・count 0・suspicious 1", m.Estimate)
	}
	c := findSite(t, r, cardrush)
	if len(c.Reasons) != 1 || len(c.Reasons[0]) != 0 {
		t.Errorf("カードラッシュの理由 = %v, want [[]]", c.Reasons)
	}
	if c.Estimate.Status != estimate.StatusOK || !eqIntPtr(c.Estimate.Low, ip(5000)) || c.Estimate.Mid != nil || c.Estimate.Count != 1 {
		t.Errorf("カードラッシュの目安 = %+v", c.Estimate)
	}
}

// AC-E7: 基準価格は Reference のサイトの、title_mismatch でも below_min でもない出品の中央値。
// 基準を決めた後に、基準サイト自身の出品も too_cheap で判定する。基準が無ければ too_cheap を判定しない。
func TestEvaluate_BasePrice(t *testing.T) {
	name := "グリス"
	ok := func(p int) estimate.Listing {
		return estimate.Listing{Title: "仮面ライダーグリス", Price: p, InStock: true}
	}
	t.Run("title_mismatch・below_min は基準に入れない", func(t *testing.T) {
		r := estimate.Evaluate(estimate.Item{Name: name, MinPrice: ip(1000)}, []estimate.SiteInput{
			{SiteID: 1, Reference: true, Listings: []estimate.Listing{ok(5000), ok(6000), {Title: "別の商品", Price: 100}, ok(900)}},
		})
		if !eqIntPtr(r.BasePrice, ip(5500)) {
			t.Errorf("BasePrice = %v, want 5500", fmtPtr(r.BasePrice))
		}
		s := findSite(t, r, 1)
		if !slices.Equal(s.Reasons[2], []estimate.Reason{estimate.ReasonTitleMismatch, estimate.ReasonTooCheap, estimate.ReasonBelowMin}) {
			t.Errorf("3 件目の理由 = %v", s.Reasons[2])
		}
		if !slices.Equal(s.Reasons[3], []estimate.Reason{estimate.ReasonTooCheap, estimate.ReasonBelowMin}) {
			t.Errorf("4 件目の理由 = %v", s.Reasons[3])
		}
		if s.Estimate.Count != 2 || s.Estimate.SuspiciousCount != 2 {
			t.Errorf("目安 = %+v, want count 2・suspicious 2", s.Estimate)
		}
	})
	t.Run("基準サイト自身の出品も too_cheap になる", func(t *testing.T) {
		r := estimate.Evaluate(estimate.Item{Name: name}, []estimate.SiteInput{
			{SiteID: 1, Reference: true, Listings: []estimate.Listing{ok(5000), ok(5200), ok(1000)}},
		})
		if !eqIntPtr(r.BasePrice, ip(5000)) {
			t.Errorf("BasePrice = %v, want 5000", fmtPtr(r.BasePrice))
		}
		s := findSite(t, r, 1)
		if !slices.Equal(s.Reasons[2], []estimate.Reason{estimate.ReasonTooCheap}) {
			t.Errorf("1000 円の理由 = %v, want [too_cheap]", s.Reasons[2])
		}
		if !eqIntPtr(s.Estimate.Low, ip(5000)) || s.Estimate.Mid != nil {
			t.Errorf("目安 = low %v mid %v, want 5000/nil(参考外を除いて 2 件)", fmtPtr(s.Estimate.Low), fmtPtr(s.Estimate.Mid))
		}
	})
	t.Run("Reference でないサイトは基準に入れない", func(t *testing.T) {
		r := estimate.Evaluate(estimate.Item{Name: name}, []estimate.SiteInput{
			{SiteID: 1, Reference: false, Listings: []estimate.Listing{ok(100)}},
			{SiteID: 2, Reference: true, Listings: []estimate.Listing{ok(5000)}},
		})
		if !eqIntPtr(r.BasePrice, ip(5000)) {
			t.Errorf("BasePrice = %v, want 5000", fmtPtr(r.BasePrice))
		}
		if s := findSite(t, r, 1); !slices.Equal(s.Reasons[0], []estimate.Reason{estimate.ReasonTooCheap}) {
			t.Errorf("理由 = %v, want [too_cheap]", s.Reasons[0])
		}
	})
	t.Run("基準が取れなければ too_cheap を判定しない", func(t *testing.T) {
		r := estimate.Evaluate(estimate.Item{Name: name}, []estimate.SiteInput{
			{SiteID: 1, Reference: false, Listings: []estimate.Listing{ok(1), ok(5000)}},
			{SiteID: 2, Reference: true, Listings: []estimate.Listing{{Title: "別の商品", Price: 5000}}},
		})
		if r.BasePrice != nil {
			t.Errorf("BasePrice = %v, want nil", fmtPtr(r.BasePrice))
		}
		s := findSite(t, r, 1)
		if len(s.Reasons[0]) != 0 || len(s.Reasons[1]) != 0 {
			t.Errorf("理由 = %v, want 参考外なし", s.Reasons)
		}
		if s.Estimate.Count != 2 || !eqIntPtr(s.Estimate.Low, ip(1)) {
			t.Errorf("目安 = %+v", s.Estimate)
		}
	})
	t.Run("出品 0 件のサイトは no_result", func(t *testing.T) {
		r := estimate.Evaluate(estimate.Item{Name: name}, []estimate.SiteInput{{SiteID: 1, Reference: true}})
		s := findSite(t, r, 1)
		if s.Estimate.Status != estimate.StatusNoResult || len(s.Reasons) != 0 || r.BasePrice != nil {
			t.Errorf("結果 = %+v base %v", s, fmtPtr(r.BasePrice))
		}
	})
}

// AC-E8: サマリ。low の最小と、そのサイトの mid(nil なら low)。fetched_at は low を持つサイトのうち最も古いもの。
func TestSummarize(t *testing.T) {
	t0 := time.Date(2026, 10, 1, 3, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	t2 := t0.Add(2 * time.Hour)
	tp := func(t time.Time) *time.Time { return &t }
	cases := []struct {
		name     string
		in       []estimate.SummaryInput
		low, mid *int
		at       *time.Time
	}{
		{"空", nil, nil, nil, nil},
		{"low を持つサイトが無い", []estimate.SummaryInput{{FetchedAt: t0}}, nil, nil, nil},
		{"最小の low とそのサイトの mid・最も古い fetched_at",
			[]estimate.SummaryInput{{Low: ip(3500), Mid: ip(3600), FetchedAt: t1}, {Low: ip(3000), Mid: ip(4500), FetchedAt: t2}},
			ip(3000), ip(4500), tp(t1)},
		{"選んだサイトの mid が nil なら low",
			[]estimate.SummaryInput{{Low: ip(3000), FetchedAt: t1}, {Low: ip(3500), Mid: ip(3600), FetchedAt: t2}},
			ip(3000), ip(3000), tp(t1)},
		{"low が同じなら mid の小さい方(nil は low とみなす)",
			[]estimate.SummaryInput{{Low: ip(3000), Mid: ip(4000), FetchedAt: t2}, {Low: ip(3000), FetchedAt: t2}},
			ip(3000), ip(3000), tp(t2)},
		{"low を持たないサイトの fetched_at は使わない",
			[]estimate.SummaryInput{{FetchedAt: t0}, {Low: ip(3000), Mid: ip(4000), FetchedAt: t2}},
			ip(3000), ip(4000), tp(t2)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := estimate.Summarize(c.in)
			if !eqIntPtr(got.Low, c.low) || !eqIntPtr(got.Mid, c.mid) {
				t.Errorf("low/mid = %v/%v, want %v/%v", fmtPtr(got.Low), fmtPtr(got.Mid), fmtPtr(c.low), fmtPtr(c.mid))
			}
			switch {
			case c.at == nil && got.FetchedAt != nil:
				t.Errorf("FetchedAt = %v, want nil", *got.FetchedAt)
			case c.at != nil && (got.FetchedAt == nil || !got.FetchedAt.Equal(*c.at)):
				t.Errorf("FetchedAt = %v, want %v", got.FetchedAt, *c.at)
			}
		})
	}
}
