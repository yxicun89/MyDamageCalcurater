// Package domain は wishlist の純粋な核を置く層(ヘキサゴナル構成。apps/wishlist/docs/arch-spec.md)。
// 商品・ジャンル・サイトの型と規則、検索ワード・ディープリンク・目安価格の算出、アクセスの節度、商品ページの URL と題名の規則を持つ。
// 配下のパッケージ(domain/**)は domain だけを import する。DB・HTTP・ファイル・ブラウザ・外部のライブラリには触れない。
package domain
