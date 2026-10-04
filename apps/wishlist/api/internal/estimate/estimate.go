// Package estimate は参考外の判定と目安価格の算出(apps/wishlist/CLAUDE.md §6)。
// 純粋な関数だけを置く(I/O・時刻の取得なし)。受け入れ条件は docs/phase3-api-spec.md の AC-E*。
package estimate

import (
	"slices"
	"strings"
	"time"
	"unicode"

	"example.com/pokecalc/apps/wishlist/api/internal/query"
)

// Reason は参考外とした理由(listings.suspicious_reasons・openapi の SuspiciousReason と同じ値)。
type Reason string

const (
	ReasonTitleMismatch Reason = "title_mismatch"
	ReasonTooCheap      Reason = "too_cheap"
	ReasonBelowMin      Reason = "below_min"
)

// CheapRatioPercent は too_cheap の閾値(基準価格の 30%。仕様 §6 の cheap_ratio 0.3)。
// 判定は整数で行う: price*100 < base*CheapRatioPercent なら too_cheap。
const CheapRatioPercent = 30

// Status はサイトの目安の状態(取得に成功したサイトだけ。failed は refresh が付ける)。
type Status string

const (
	StatusOK       Status = "ok"
	StatusNoResult Status = "no_result"
)

// Item は判定に使う商品の値。
type Item struct {
	Name     string
	MinPrice *int // nil なら below_min を判定しない
	// Aliases は商品のジャンルの表記揺れの辞書(フェーズ4-1。nil なら辞書なしで TitleMatches と同じ)。
	Aliases [][]string
}

// Listing は判定・算出に使う出品の値。
type Listing struct {
	Title   string
	Price   int
	InStock bool
}

// SiteInput は 1 サイト分の取得結果。
type SiteInput struct {
	SiteID int64
	// Reference は基準価格の算出に使うサイトか(sites.is_reference かつ fetch_type が api / scrape。呼び出し側が決める)。
	Reference bool
	Listings  []Listing
}

// SiteEstimate はサイトの目安。
type SiteEstimate struct {
	Low             *int // 下位 25 パーセンタイル(nearest-rank)。件数 3 未満なら最小値。0 件なら nil
	Mid             *int // 中央値。件数 3 未満・0 件なら nil
	Count           int  // 参考外を除いた件数
	SuspiciousCount int  // 参考外の件数
	InStockCount    int  // 参考外を除き、在庫ありの件数
	Status          Status
}

// SiteResult は 1 サイト分の結果。Reasons は入力の Listings と同じ順・同じ長さ(参考でない出品は長さ 0)。
type SiteResult struct {
	SiteID   int64
	Reasons  [][]Reason
	Estimate SiteEstimate
}

// Result は Evaluate の結果。Sites は入力と同じ順。
type Result struct {
	BasePrice *int // 基準価格。取れなければ nil(too_cheap を判定しない)
	Sites     []SiteResult
}

// TitleMatches は、name を Unicode の空白で分けた各トークンを query.Normalize したものが、
// すべて Normalize(title) に含まれるかを返す(正規化して空になるトークンは無視する。トークンが無ければ true)。
func TitleMatches(name, title string) bool {
	nt := query.Normalize(title)
	for _, tok := range strings.FieldsFunc(name, unicode.IsSpace) {
		n := query.Normalize(tok)
		if n != "" && !strings.Contains(nt, n) {
			return false
		}
	}
	return true
}

// AliasVariants は name の 1 トークンについて、タイトルに含まれていれば一致とみなす語(正規化済み)を返す
// (docs/phase4-spec.md AC-A4)。トークンと正規化して完全一致する語を持つグループがあればその全語、無ければトークン自身だけ。
// 正規化して空のトークンは空を返す。
func AliasVariants(token string, groups [][]string) []string {
	n := query.Normalize(token)
	if n == "" {
		return nil
	}
	for _, g := range groups {
		var words []string
		hit := false
		for _, w := range g {
			nw := query.Normalize(w)
			if nw == "" {
				continue
			}
			if nw == n {
				hit = true
			}
			if !slices.Contains(words, nw) {
				words = append(words, nw)
			}
		}
		if hit {
			return words
		}
	}
	return []string{n}
}

// TitleMatchesWithAliases は、表記揺れの辞書 groups を使う TitleMatches(docs/phase4-spec.md AC-A5)。
// name の各トークンについて、AliasVariants の語のどれかが正規化タイトルに含まれればよい。groups が空なら TitleMatches と同じ。
func TitleMatchesWithAliases(name, title string, groups [][]string) bool {
	nt := query.Normalize(title)
	for _, tok := range strings.FieldsFunc(name, unicode.IsSpace) {
		vs := AliasVariants(tok, groups)
		if len(vs) == 0 {
			continue
		}
		if !slices.ContainsFunc(vs, func(v string) bool { return strings.Contains(nt, v) }) {
			return false
		}
	}
	return true
}

// Judge は 1 件の参考外の理由を返す。順は title_mismatch → too_cheap → below_min。当てはまらなければ長さ 0。
// base が nil なら too_cheap を判定しない。item.MinPrice が nil なら below_min を判定しない。
func Judge(item Item, base *int, l Listing) []Reason {
	var rs []Reason
	if !TitleMatchesWithAliases(item.Name, l.Title, item.Aliases) {
		rs = append(rs, ReasonTitleMismatch)
	}
	if base != nil && l.Price*100 < *base*CheapRatioPercent {
		rs = append(rs, ReasonTooCheap)
	}
	if item.MinPrice != nil && l.Price < *item.MinPrice {
		rs = append(rs, ReasonBelowMin)
	}
	return rs
}

// Median は中央値。偶数件なら中央 2 つの平均を切り捨てる。空なら nil。入力の並びは問わない(書き換えない)。
func Median(prices []int) *int {
	if len(prices) == 0 {
		return nil
	}
	s := slices.Clone(prices)
	slices.Sort(s)
	n := len(s)
	m := s[n/2]
	if n%2 == 0 {
		m = (s[n/2-1] + s[n/2]) / 2
	}
	return &m
}

// Estimate は参考外を除いた価格(と在庫)から目安を出す。
// low は nearest-rank の 25 パーセンタイル(昇順で ceil(n/4) 番目)、mid は Median。
// 3 件未満なら low は最小値・mid は nil。0 件なら low・mid とも nil で Status は no_result。
func Estimate(valid []Listing, suspiciousCount int) SiteEstimate {
	e := SiteEstimate{Count: len(valid), SuspiciousCount: suspiciousCount, Status: StatusNoResult}
	if len(valid) == 0 {
		return e
	}
	prices := make([]int, 0, len(valid))
	for _, l := range valid {
		prices = append(prices, l.Price)
		if l.InStock {
			e.InStockCount++
		}
	}
	slices.Sort(prices)
	e.Status = StatusOK
	if len(prices) < 3 {
		e.Low = &prices[0]
		return e
	}
	low := prices[(len(prices)+3)/4-1]
	e.Low = &low
	e.Mid = Median(prices)
	return e
}

// Evaluate は 2 段階で判定する。
//  1. 基準価格: Reference のサイトの出品のうち、title_mismatch でも below_min でもないものの価格の Median
//  2. 各出品を Judge(基準価格つき)し、参考外を除いて Estimate する
func Evaluate(item Item, sites []SiteInput) Result {
	var basePrices []int
	for _, s := range sites {
		if !s.Reference {
			continue
		}
		for _, l := range s.Listings {
			if !TitleMatchesWithAliases(item.Name, l.Title, item.Aliases) || (item.MinPrice != nil && l.Price < *item.MinPrice) {
				continue
			}
			basePrices = append(basePrices, l.Price)
		}
	}
	res := Result{BasePrice: Median(basePrices), Sites: make([]SiteResult, 0, len(sites))}
	for _, s := range sites {
		sr := SiteResult{SiteID: s.SiteID}
		if len(s.Listings) > 0 {
			sr.Reasons = make([][]Reason, len(s.Listings))
		}
		var valid []Listing
		suspicious := 0
		for i, l := range s.Listings {
			sr.Reasons[i] = Judge(item, res.BasePrice, l)
			if len(sr.Reasons[i]) > 0 {
				suspicious++
			} else {
				valid = append(valid, l)
			}
		}
		sr.Estimate = Estimate(valid, suspicious)
		res.Sites = append(res.Sites, sr)
	}
	return res
}

// SummaryInput はサマリに使う 1 サイト分の保存済みの目安。
type SummaryInput struct {
	Low       *int
	Mid       *int
	FetchedAt time.Time
}

// Summary は全サイトのサマリ。
type Summary struct {
	Low       *int
	Mid       *int
	FetchedAt *time.Time
}

// Summarize は Low を持つサイト(候補)から、Low の最小を Low にし、そのサイトの Mid(nil ならその Low)を Mid にする。
// Low が同じなら Mid(nil は Low とみなす)の小さい方、それも同じなら先のサイト。
// FetchedAt は候補の FetchedAt のうち最も古いもの。候補が無ければすべて nil。
func Summarize(sites []SummaryInput) Summary {
	var out Summary
	var oldest time.Time
	mid := func(s SummaryInput) int {
		if s.Mid != nil {
			return *s.Mid
		}
		return *s.Low
	}
	var best *SummaryInput
	for i := range sites {
		s := &sites[i]
		if s.Low == nil {
			continue
		}
		if best == nil || *s.Low < *best.Low || (*s.Low == *best.Low && mid(*s) < mid(*best)) {
			best = s
		}
		if out.FetchedAt == nil || s.FetchedAt.Before(oldest) {
			oldest = s.FetchedAt
			out.FetchedAt = &oldest
		}
	}
	if best != nil {
		low, m := *best.Low, mid(*best)
		out.Low, out.Mid = &low, &m
	}
	return out
}
