// Package crawl は価格取得のアクセスの節度の規則(apps/wishlist/CLAUDE.md §6。純粋な値と関数だけ)。
// 取得そのもの(HTTP・ブラウザ)は internal/adapters/fetcher が行い、ここの値を読む。
package crawl
