package crawl

import (
	"net/url"
	"strings"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/domain/item"
)

const (
	// MaxListings は 1 サイトあたりに取得する上位件数(仕様 §6)。
	MaxListings = 20
	// MinInterval は同じサイトへのリクエストの最低間隔(仕様 §6)。
	MinInterval = 5 * time.Second
)

// HostMinIntervals はホストごとの最小間隔(既定の MinInterval より長いもの)。robots.txt の Crawl-delay に合わせる。
// 駿河屋は Crawl-delay: 30(docs/sites.md。ユーザー決定 2026-10-04)。キーは小文字・ポートなしのホスト名。
var HostMinIntervals = map[string]time.Duration{
	"www.suruga-ya.jp": 30 * time.Second,
}

// NightlyOnlyHosts は夜間の CronJob だけで取るホスト(api の裏の更新・手動の更新では取らない)。
var NightlyOnlyHosts = map[string]bool{
	"www.suruga-ya.jp": true,
	MercariHost:        true,
}

// MercariHost は headless で取るメルカリのホスト。
const MercariHost = "jp.mercari.com"

// SiteHost は検索 URL テンプレートのホスト名(小文字・ポートなし)。読めなければ空。
func SiteHost(site item.Site) string {
	u, err := url.Parse(site.SearchURLTemplate)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// NightlyOnly は site が夜間の CronJob だけで取るサイトか(NightlyOnlyHosts。検索 URL テンプレートのホストで決める)。
func NightlyOnly(site item.Site) bool { return NightlyOnlyHosts[SiteHost(site)] }
