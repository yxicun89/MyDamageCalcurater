// Package migrations は wishlist の MySQL スキーマ(golang-migrate 形式の SQL)を埋め込む。
// 実行版がコードと一致するよう、SQL はバイナリに含める。
package migrations

import "embed"

// FS は このディレクトリの *.sql。
//
//go:embed *.sql
var FS embed.FS
