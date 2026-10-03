// Command api は wishlist の API サーバーと DB マイグレーション。
//
//	api serve        HTTP サーバー(PORT・WISHLIST_DATABASE_DSN・WISHLIST_API_TOKEN・WISHLIST_IMAGE_DIR)
//	api migrate up   migrations を適用(WISHLIST_DATABASE_DSN。DSN に multiStatements=true を足す。docs/design.md W-09)
package main

import (
	"errors"
	"io"
	"os"
)

// DefaultPort は PORT が無いときの待ち受けポート。
const DefaultPort = "8080"

var errMissingEnv = errors.New("missing required environment variable")

// serveConfig は serve の設定。
type serveConfig struct {
	Port     string
	DSN      string // serveDSN を通した後の DSN
	Token    string
	ImageDir string
}

// migrateConfig は migrate の設定。
type migrateConfig struct {
	DSN string // migrateDSN を通した後の DSN
}

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

// run はサブコマンドを実行し、終了コードを返す。使い方の誤り(サブコマンド無し・不明)は 2、設定や実行の失敗は 1。
func run(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	panic("TODO: run")
}

// loadServeConfig は serve の設定を環境変数から読む。WISHLIST_DATABASE_DSN・WISHLIST_API_TOKEN・WISHLIST_IMAGE_DIR は必須
// (空白だけも無いとみなす。errMissingEnv を包み、どの変数かをメッセージに含める)。PORT は省略時 DefaultPort、
// 数字でない・1〜65535 の外ならエラー。
func loadServeConfig(getenv func(string) string) (serveConfig, error) {
	panic("TODO: loadServeConfig")
}

// loadMigrateConfig は migrate の設定を読む。WISHLIST_DATABASE_DSN だけが必須。
func loadMigrateConfig(getenv func(string) string) (migrateConfig, error) {
	panic("TODO: loadMigrateConfig")
}

// serveDSN は API サーバー用の DSN にする: parseTime=true を必ず付け、multiStatements は付けない(付いていれば外す)。
// 解釈できない DSN はエラー。
func serveDSN(dsn string) (string, error) {
	panic("TODO: serveDSN")
}

// migrateDSN は migrate 用の DSN にする: multiStatements=true を足す(他のパラメータは保つ)。解釈できない DSN はエラー。
func migrateDSN(dsn string) (string, error) {
	panic("TODO: migrateDSN")
}
