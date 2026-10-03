// Package netguard は外部 URL(OGP の商品ページ・登録時の image_url)を取りに行く HTTP クライアント。
// SSRF 対策(docs/design.md W-05):
//   - http/https だけ(リダイレクト先も同じ)
//   - 接続先の IP を**ダイヤル時に**検査する(DNS の解決結果を使う。名前解決後に差し替える DNS リバインディングを防ぐ)
//   - プロキシの環境変数を使わない(プロキシ経由だと接続先の検査が効かないため)
//   - タイムアウトとリダイレクト回数の上限
//
// 読み込みサイズの上限は呼び出し側が ReadLimited で掛ける(HTML 2 MiB・画像 10 MiB)。
package netguard

import (
	"errors"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"time"
)

const (
	// DefaultTimeout は 1 回の取得全体(リダイレクトを含む)の上限。
	DefaultTimeout = 10 * time.Second
	// DefaultMaxRedirects はリダイレクトを追う回数の上限。超えたら失敗。
	DefaultMaxRedirects = 5
	// MaxHTMLBytes は OGP 用に読む HTML の上限。
	MaxHTMLBytes = 2 << 20
)

var (
	// ErrInvalidURL は URL が解釈できない・http/https 以外・ホストが無いこと。
	ErrInvalidURL = errors.New("netguard: invalid or unsupported url")
	// ErrForbiddenAddress は接続先がプライベート・ループバック・リンクローカル等の許可されないアドレスであること。
	ErrForbiddenAddress = errors.New("netguard: forbidden destination address")
	// ErrTooLarge は ReadLimited の上限を超えたこと。
	ErrTooLarge = errors.New("netguard: response too large")
	// ErrTooManyRedirects はリダイレクトが上限を超えたこと。
	ErrTooManyRedirects = errors.New("netguard: too many redirects")
)

// Options は NewClient の設定。ゼロ値なら既定値を使う。
type Options struct {
	Timeout      time.Duration // 0 なら DefaultTimeout
	MaxRedirects int           // 0 なら DefaultMaxRedirects
	// AllowAddr は接続してよいアドレスかを返す。nil なら IsPublic。テストで httptest(127.0.0.1)を許すために差し替える。
	AllowAddr func(netip.Addr) bool
}

// NewClient は SSRF 対策をした http.Client を返す。拒否したときのエラーは errors.Is で
// ErrInvalidURL・ErrForbiddenAddress・ErrTooManyRedirects と判定できること。
func NewClient(opts Options) *http.Client {
	panic("TODO: netguard.NewClient")
}

// ValidateURL は raw が http/https の絶対 URL(ホストあり)かを検査して返す。違反は ErrInvalidURL を包む。
func ValidateURL(raw string) (*url.URL, error) {
	panic("TODO: netguard.ValidateURL")
}

// IsPublic は addr がインターネット上の公開ユニキャストアドレスかを返す。
// 拒否: 未指定(0.0.0.0・::)・ループバック・プライベート(10/8・172.16/12・192.168/16・fc00::/7)・
// リンクローカル(169.254/16・fe80::/10)・CGNAT(100.64/10。Tailscale もここ)・マルチキャスト・ブロードキャスト、
// および IPv4 射影 IPv6(::ffff:a.b.c.d)は中の IPv4 で判定する。
func IsPublic(addr netip.Addr) bool {
	panic("TODO: netguard.IsPublic")
}

// ReadLimited は r から最大 max バイト読む。max を超えるデータがあれば ErrTooLarge。ちょうど max は可。
func ReadLimited(r io.Reader, max int64) ([]byte, error) {
	panic("TODO: netguard.ReadLimited")
}
