package httpapi

// P8-1(ADR-0807)の gateway テスト: `/images/*` はローカルのディレクトリ(Config.ImagesDir。環境変数
// GATEWAY_IMAGES_DIR)から manifest.json と WebP だけを配信する。画像が無い状態(ImagesDir 未設定・
// ファイル無し)でも gateway は壊れず、404 の JSON を返す(クライアントはエンブレムにフォールバックする)。
// 実装前なので、このファイルはコンパイルできず失敗する(Config.ImagesDir が無い)。

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	imgManifestBody = `{"version":1,"images":{"0445-000":{"thumb":"thumb/0445-000.0123abcd.webp","detail":"detail/0445-000.4567ef01.webp"}}}`
	imgThumbPath    = "/images/thumb/0445-000.0123abcd.webp"
	imgThumbBody    = "RIFF....WEBPfake-thumb" // 中身は検査しない(拡張子で型を決める)
)

// newImagesDir は架空の画像ディレクトリを作る(manifest・thumb・detail・拡張子違い・ドットファイル)。
func newImagesDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"manifest.json":                   imgManifestBody,
		"thumb/0445-000.0123abcd.webp":    imgThumbBody,
		"detail/0445-000.4567ef01.webp":   "RIFF....WEBPfake-detail",
		"notes.txt":                       "secret notes",
		".hidden":                         "SECRET=1",
		"thumb/0445-000.0123abcd.webp.gz": "x",
	}
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func withImagesDir(dir string) func(*Config) { return func(c *Config) { c.ImagesDir = dir } }

// AC-I1: GET / HEAD で manifest と WebP を返す。X-Device-Id 等は要らない(<img> はヘッダを送れない)。
// どの上流(assets を含む)にも届かない。型・キャッシュ: ハッシュ付き WebP は長期 immutable、manifest は no-cache。
func TestImagesServeManifestAndWebP(t *testing.T) {
	env := newWebTestEnv(t, withImagesDir(newImagesDir(t)))

	rec := serve(t, env.handler, http.MethodGet, "/images/manifest.json", http.Header{}, nil)
	if rec.Code != http.StatusOK || rec.Body.String() != imgManifestBody {
		t.Fatalf("manifest: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("manifest の Content-Type = %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
		t.Errorf("manifest の Cache-Control = %q, want no-cache を含む(差し替えがすぐ反映される)", cc)
	}

	rec = serve(t, env.handler, http.MethodGet, imgThumbPath, http.Header{}, nil)
	if rec.Code != http.StatusOK || rec.Body.String() != imgThumbBody {
		t.Fatalf("thumb: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/webp" {
		t.Errorf("thumb の Content-Type = %q, want image/webp", ct)
	}
	cc := rec.Header().Get("Cache-Control")
	if !strings.Contains(cc, "max-age=31536000") || !strings.Contains(cc, "immutable") {
		t.Errorf("thumb の Cache-Control = %q, want max-age=31536000 と immutable", cc)
	}

	rec = serve(t, env.handler, http.MethodHead, imgThumbPath, http.Header{}, nil)
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Errorf("HEAD: status=%d bodyLen=%d, want 200 と空本文", rec.Code, rec.Body.Len())
	}
	env.assertNoUpstreamReached(t)
}

// AC-I2: 画像が無い状態でも壊れない。ImagesDir 未設定・ファイル無し・空ディレクトリは 404 not_found の JSON
// (Web の index.html を 200 で返さない。クライアントが HTML を manifest として読まないため)。上流には届かない。
func TestImagesAbsentIsJSON404(t *testing.T) {
	empty := t.TempDir()
	tests := []struct {
		name string
		cfg  func(*Config)
		path string
	}{
		{"ImagesDir 未設定の manifest", nil, "/images/manifest.json"},
		{"ImagesDir 未設定の画像", nil, imgThumbPath},
		{"空ディレクトリの manifest", withImagesDir(empty), "/images/manifest.json"},
		{"存在しないディレクトリ", withImagesDir(filepath.Join(empty, "nope")), "/images/manifest.json"},
		{"manifest に無いキー", withImagesDir(newImagesDir(t)), "/images/thumb/9999-999.00000000.webp"},
		{"/images そのもの", withImagesDir(newImagesDir(t)), "/images"},
		{"/images/ のディレクトリ一覧は出さない", withImagesDir(newImagesDir(t)), "/images/"},
		{"/images/thumb/ の一覧も出さない", withImagesDir(newImagesDir(t)), "/images/thumb/"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var mutate []func(*Config)
			if tc.cfg != nil {
				mutate = append(mutate, tc.cfg)
			}
			env := newWebTestEnv(t, mutate...)
			rec := serve(t, env.handler, http.MethodGet, tc.path, http.Header{}, nil)
			assertGatewayError(t, rec, http.StatusNotFound, "not_found")
			env.assertNoUpstreamReached(t)
		})
	}
}

// AC-I3: 配信してよいのは manifest.json と .webp だけ。他の拡張子・ドットファイル・書き込み系メソッド・
// パストラバーサル・ディレクトリ外を指すシンボリックリンクは 404(中身を漏らさない)。
func TestImagesRejectsUnsafeRequests(t *testing.T) {
	dir := newImagesDir(t)
	outside := filepath.Join(t.TempDir(), "secret.webp")
	if err := os.WriteFile(outside, []byte("outside-secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "thumb", "link.0123abcd.webp")); err != nil {
		t.Skipf("シンボリックリンクを作れない: %v", err)
	}
	env := newWebTestEnv(t, withImagesDir(dir))

	gets := []string{
		"/images/notes.txt",
		"/images/.hidden",
		"/images/thumb/0445-000.0123abcd.webp.gz",
		"/images/../notes.txt",
		"/images/thumb/../../etc/passwd",
		"/images/%2e%2e/notes.txt",
		"/images/thumb/..%2f..%2fnotes.txt",
		"/images/thumb/%2e%2e%2fnotes.txt",
		"/images/thumb/link.0123abcd.webp",
		"/images//manifest.json",
		"/images/thumb//0445-000.0123abcd.webp",
	}
	for _, p := range gets {
		rec := serve(t, env.handler, http.MethodGet, p, http.Header{}, nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s: status=%d, want 404; body=%q", p, rec.Code, rec.Body.String())
		}
		for _, leak := range []string{"secret", "SECRET", "root:"} {
			if strings.Contains(rec.Body.String(), leak) {
				t.Errorf("GET %s: 本文に %q が漏れた", p, leak)
			}
		}
	}
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		rec := serve(t, env.handler, m, "/images/manifest.json", http.Header{}, []byte("x"))
		assertGatewayError(t, rec, http.StatusNotFound, "not_found")
	}
	env.assertNoUpstreamReached(t)
}

// AC-I4: CORS は assets と同じ。許可オリジンの画像取得に ACAO(credentials なし)が付き、許可外には付かない。
func TestImagesCORS(t *testing.T) {
	env := newTestEnv(t, withImagesDir(newImagesDir(t)))
	h := http.Header{"Origin": []string{allowedOrigin}}
	rec := serve(t, env.handler, http.MethodGet, "/images/manifest.json", h, nil)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != allowedOrigin {
		t.Errorf("ACAO = %q, want %q", got, allowedOrigin)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("ACAC = %q, want 空", got)
	}
	h = http.Header{"Origin": []string{disallowedOrigin}}
	rec = serve(t, env.handler, http.MethodGet, "/images/manifest.json", h, nil)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("許可外オリジンに ACAO = %q が付いた", got)
	}
}

// AC-I5: `images` は予約セグメント。ImagesDir 未設定でも /images/* は Web の SPA フォールバックに流れない。
// /imagesx のような別名は予約語ではない(従来どおり Web へ)。
func TestImagesIsReservedSegment(t *testing.T) {
	env := newWebTestEnv(t)
	rec := serve(t, env.handler, http.MethodGet, "/images/anything.webp", http.Header{}, nil)
	assertGatewayError(t, rec, http.StatusNotFound, "not_found")
	if n := len(env.web.requests()); n != 0 {
		t.Fatalf("Web に %d 回届いた(/images/* は Web に流さない)", n)
	}
	rec = serve(t, env.handler, http.MethodGet, "/imagesx/a", http.Header{}, nil)
	if rec.Code != http.StatusOK || len(env.web.requests()) != 1 {
		t.Errorf("/imagesx/a は Web へ: status=%d webReqs=%d", rec.Code, len(env.web.requests()))
	}
}

// AC-I6: 既存の /assets/* の転送(ADR-0202)は変えない。/images の追加で /assets の経路を壊さない。
func TestAssetsRouteUnchangedByImages(t *testing.T) {
	env := newTestEnv(t, withImagesDir(newImagesDir(t)))
	rec := serve(t, env.handler, http.MethodGet, "/assets/0445-000.webp", http.Header{}, nil)
	env.assertOnlyUpstreamReached(t, env.assets, rec)
}
