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
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"syscall"
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
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	maxRedirects := opts.MaxRedirects
	if maxRedirects <= 0 {
		maxRedirects = DefaultMaxRedirects
	}
	allow := opts.AllowAddr
	if allow == nil {
		allow = IsPublic
	}
	dialer := &net.Dialer{
		Timeout: timeout,
		// 接続の直前(名前解決後のアドレス)ごとに呼ばれる。リダイレクト先・再接続でも必ず検査される。
		Control: func(_, address string, _ syscall.RawConn) error {
			ap, err := netip.ParseAddrPort(address)
			if err != nil {
				return fmt.Errorf("%w: %v", ErrForbiddenAddress, err)
			}
			if !allow(ap.Addr()) {
				return fmt.Errorf("%w: %s", ErrForbiddenAddress, ap.Addr())
			}
			return nil
		},
	}
	tr := &http.Transport{
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		MaxIdleConns:          4,
		IdleConnTimeout:       30 * time.Second,
	}
	return &http.Client{
		Transport: tr,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("%w: リダイレクト先 %q", ErrInvalidURL, req.URL.Scheme)
			}
			if len(via) > maxRedirects {
				return ErrTooManyRedirects
			}
			return nil
		},
	}
}

// ValidateURL は raw が http/https の絶対 URL(ホストあり)かを検査して返す。違反は ErrInvalidURL を包む。
func ValidateURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("%w: スキームは http/https のみ", ErrInvalidURL)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("%w: ホストが無い", ErrInvalidURL)
	}
	return u, nil
}

// IsPublic は addr がインターネット上の公開ユニキャストアドレスかを返す。
// 拒否: 未指定(0.0.0.0・::)・ループバック・プライベート(10/8・172.16/12・192.168/16・fc00::/7)・
// リンクローカル(169.254/16・fe80::/10)・CGNAT(100.64/10。Tailscale もここ)・マルチキャスト・ブロードキャスト、
// および IPv4 射影 IPv6(::ffff:a.b.c.d)は中の IPv4 で判定する。
func IsPublic(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() {
		return false
	}
	for _, p := range blockedPrefixes {
		if p.Contains(addr) {
			return false
		}
	}
	return true
}

// ReadLimited は r から最大 max バイト読む。max を超えるデータがあれば ErrTooLarge。ちょうど max は可。
func ReadLimited(r io.Reader, max int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, ErrTooLarge
	}
	return b, nil
}

// blockedPrefixes は IsGlobalUnicast・IsPrivate では弾けない、公開されていない範囲。
var blockedPrefixes = []netip.Prefix{
	v4Prefix(0, 0, 0, 0, 8),     // "このネットワーク"
	v4Prefix(100, 64, 0, 0, 10), // CGNAT(Tailscale もここ)
	v4Prefix(192, 0, 0, 0, 24),  // IETF プロトコル割り当て
	v4Prefix(198, 18, 0, 0, 15), // ベンチマーク用
	v4Prefix(240, 0, 0, 0, 4),   // 予約済み
}

func v4Prefix(a, b, c, d byte, bits int) netip.Prefix {
	return netip.PrefixFrom(netip.AddrFrom4([4]byte{a, b, c, d}), bits)
}
