package httpapi

// ローカル画像の配信(ADR-0807)。Config.ImagesDir の manifest.json と .webp だけを /images/* で返す。
// 素の http.FileServer は使わない(ディレクトリ一覧・ドットファイル・ディレクトリ外へのシンボリックリンクを
// 出さないため)。画像が無い・ImagesDir が未設定のときは JSON の 404(クライアントはエンブレムに戻す)。

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/internal/api"
)

const (
	imagesManifestName = "manifest.json"
	imagesWebPExt      = ".webp"
	// ハッシュ付きの WebP は内容が変わると名前が変わるので、長期 immutable にできる。
	cacheImmutable  = "public, max-age=31536000, immutable"
	cacheRevalidate = "no-cache"
)

// hashedWebP は tools/assets が付ける `{key}.{hash8}.webp` の形。
var hashedWebP = regexp.MustCompile(`\.[0-9a-f]{8}\.webp$`)

// serveImage は /images/* の GET / HEAD に答える。serve から routeImages のときだけ呼ばれる。
func (g *gateway) serveImage(c *echo.Context, origin string, allowed bool) error {
	notFound := func() error { return g.ownError(c, origin, allowed, newError(api.NotFound, "%s", msgNotFound)) }

	rel := strings.TrimPrefix(c.Request().URL.Path, prefixImages)
	f, contentType, cache, ok := g.openImage(rel)
	if !ok {
		return notFound()
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		return notFound()
	}

	h := c.Response().Header()
	if allowed {
		setCORSAllowed(h, origin)
	}
	h.Set("Content-Type", contentType)
	h.Set("Cache-Control", cache)
	h.Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(c.Response(), c.Request(), info.Name(), info.ModTime(), f)
	return nil
}

// openImage は /images/ からの相対パスを、配信してよいファイルとして開く。
// 許可は manifest.json と .webp のみ・ドットで始まるセグメントなし・ImagesDir の外に出ない実体のみ。
func (g *gateway) openImage(rel string) (f *os.File, contentType, cache string, ok bool) {
	root := g.cfg.ImagesDir
	if root == "" || rel == "" {
		return nil, "", "", false
	}
	segs := strings.Split(rel, "/")
	for _, s := range segs {
		if s == "" || strings.HasPrefix(s, ".") || strings.ContainsAny(s, `\`+"\x00") {
			return nil, "", "", false
		}
	}
	switch {
	case rel == imagesManifestName:
		contentType, cache = "application/json; charset=utf-8", cacheRevalidate
	case strings.HasSuffix(rel, imagesWebPExt):
		contentType, cache = "image/webp", cacheRevalidate
		if hashedWebP.MatchString(rel) {
			cache = cacheImmutable
		}
	default:
		return nil, "", "", false
	}

	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, "", "", false
	}
	real, err := filepath.EvalSymlinks(filepath.Join(realRoot, filepath.FromSlash(rel)))
	if err != nil {
		return nil, "", "", false
	}
	if inside, err := filepath.Rel(realRoot, real); err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return nil, "", "", false
	}
	file, err := os.Open(real)
	if err != nil {
		return nil, "", "", false
	}
	return file, contentType, cache, true
}
