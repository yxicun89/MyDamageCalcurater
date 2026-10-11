// Package adapters は外側(DB・HTTP・ファイル・ブラウザ・外部サイト)を置く層(ヘキサゴナル構成。apps/wishlist/docs/arch-spec.md)。
// app が宣言したポートを実装し、HTTP の入り口を受け持つ。配下のパッケージ(adapters/**)は domain・app・adapters と、
// 生成物の internal/api・migrations を import してよい。
package adapters
