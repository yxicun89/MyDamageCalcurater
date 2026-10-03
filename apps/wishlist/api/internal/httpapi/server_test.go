package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"example.com/pokecalc/apps/wishlist/api/internal/httpapi"
	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/netguard"
	"example.com/pokecalc/apps/wishlist/api/internal/ogp"
	"example.com/pokecalc/apps/wishlist/api/internal/storage"
	"example.com/pokecalc/apps/wishlist/api/internal/testimg"
)

const token = "unit-test-placeholder"

// fakeRemote は外部取得の代わり。呼ばれた URL を記録する。
type fakeRemote struct {
	mu       sync.Mutex
	draft    ogp.Draft
	draftErr error
	image    []byte
	imageErr error
	calls    []string
}

func (f *fakeRemote) Draft(ctx context.Context, rawURL string) (ogp.Draft, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "draft "+rawURL)
	return f.draft, f.draftErr
}

func (f *fakeRemote) Image(ctx context.Context, rawURL string, max int64) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "image "+rawURL)
	if f.imageErr != nil {
		return nil, f.imageErr
	}
	if int64(len(f.image)) > max {
		return nil, netguard.ErrTooLarge
	}
	return f.image, nil
}

type env struct {
	h      http.Handler
	svc    *item.Service
	remote *fakeRemote
	dir    string
	genre  item.Genre
	site1  item.Site
	site2  item.Site
}

func newEnv(t *testing.T) *env {
	t.Helper()
	dir := t.TempDir()
	images, err := storage.NewFileStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	svc := item.NewService(item.NewMemoryRepository(), images)
	ctx := context.Background()
	s1, err := svc.CreateSite(ctx, item.NewSite{Name: "サイト1", SearchURLTemplate: "https://one.example.com/s?q={q}"})
	if err != nil {
		t.Fatal(err)
	}
	s2, err := svc.CreateSite(ctx, item.NewSite{Name: "サイト2", SearchURLTemplate: "https://two.example.com/s?q={q}"})
	if err != nil {
		t.Fatal(err)
	}
	g, err := svc.CreateGenre(ctx, item.NewGenre{Name: "S.H.Figuarts", QueryTemplate: "S.H.Figuarts {name}", SiteIDs: []int64{s1.ID, s2.ID}})
	if err != nil {
		t.Fatal(err)
	}
	remote := &fakeRemote{}
	e := httpapi.NewServer(httpapi.Deps{Items: svc, Images: images, Remote: remote, Token: token})
	return &env{h: e, svc: svc, remote: remote, dir: dir, genre: g, site1: s1, site2: s2}
}

type req struct {
	method, path string
	body         io.Reader
	ctype        string
	auth         string // "" ならトークンを付ける。"-" なら付けない。それ以外はそのまま Authorization に入れる
}

func (e *env) do(t *testing.T, r req) *httptest.ResponseRecorder {
	t.Helper()
	hr := httptest.NewRequest(r.method, r.path, r.body)
	if r.ctype != "" {
		hr.Header.Set("Content-Type", r.ctype)
	}
	switch r.auth {
	case "":
		hr.Header.Set("Authorization", "Bearer "+token)
	case "-":
	default:
		hr.Header.Set("Authorization", r.auth)
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, hr)
	return w
}

func (e *env) json(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	if body != nil {
		switch b := body.(type) {
		case string:
			rd = strings.NewReader(b)
		default:
			bs, err := json.Marshal(b)
			if err != nil {
				t.Fatal(err)
			}
			rd = bytes.NewReader(bs)
		}
	}
	return e.do(t, req{method: method, path: path, body: rd, ctype: "application/json"})
}

func multipartBody(t *testing.T, fields map[string]string, image []byte) (io.Reader, string) {
	t.Helper()
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if image != nil {
		fw, err := mw.CreateFormFile("image", "photo.bin")
		if err != nil {
			t.Fatal(err)
		}
		fw.Write(image)
	}
	mw.Close()
	return &b, mw.FormDataContentType()
}

// expectError は Error スキーマ({"code","message"} だけ・message は空でない)と code を確かめる。
func expectError(t *testing.T, w *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if w.Code != status {
		t.Errorf("status = %d, want %d(body %s)", w.Code, status, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q", ct)
	}
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("Error スキーマでない: %s", w.Body.String())
	}
	if m["code"] != code {
		t.Errorf("code = %v, want %s(body %s)", m["code"], code, w.Body.String())
	}
	if msg, _ := m["message"].(string); msg == "" {
		t.Errorf("message が空: %s", w.Body.String())
	}
	for k := range m {
		if k != "code" && k != "message" {
			t.Errorf("Error に余計な項目 %q: %s", k, w.Body.String())
		}
	}
}

func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode: %v(body %s)", err, w.Body.String())
	}
	return v
}

func countFiles(t *testing.T, dir string) int {
	t.Helper()
	es, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return len(es)
}

// createItem は multipart で PNG の商品を作り、応答の JSON を返す。
func (e *env) createItem(t *testing.T, fields map[string]string) map[string]any {
	t.Helper()
	if fields == nil {
		fields = map[string]string{}
	}
	if _, ok := fields["genre_id"]; !ok {
		fields["genre_id"] = fmt.Sprint(e.genre.ID)
	}
	if _, ok := fields["name"]; !ok {
		fields["name"] = "グリス"
	}
	body, ct := multipartBody(t, fields, testimg.PNG())
	w := e.do(t, req{method: http.MethodPost, path: "/api/items", body: body, ctype: ct})
	if w.Code != http.StatusCreated {
		t.Fatalf("POST /api/items = %d %s", w.Code, w.Body.String())
	}
	return decode[map[string]any](t, w)
}

func idOf(m map[string]any) int64 { return int64(m["id"].(float64)) }

// ---- 認証 ----

// AC-H1: /api/* はトークン必須。無い・違う・形式違いは 401(Error スキーマ)。
func TestAuth(t *testing.T) {
	e := newEnv(t)
	paths := []struct{ method, path string }{
		{http.MethodGet, "/api/items"},
		{http.MethodPost, "/api/items"},
		{http.MethodGet, "/api/items/1"},
		{http.MethodDelete, "/api/items/1"},
		{http.MethodGet, "/api/genres"},
		{http.MethodGet, "/api/sites"},
		{http.MethodPost, "/api/items/from-url"},
		{http.MethodGet, "/api/items/1/estimates"},
	}
	auths := map[string]string{
		"無し":        "-",
		"違う":        "Bearer wrong-token",
		"前方一致":      "Bearer " + token[:len(token)-1],
		"後ろに余計":     "Bearer " + token + "x",
		"Basic":     "Basic " + token,
		"Bearer 無し": token,
		"空":         "Bearer ",
	}
	for _, p := range paths {
		for name, a := range auths {
			t.Run(p.method+" "+p.path+" "+name, func(t *testing.T) {
				w := e.do(t, req{method: p.method, path: p.path, auth: a, ctype: "application/json", body: strings.NewReader("{}")})
				expectError(t, w, http.StatusUnauthorized, "unauthorized")
			})
		}
	}
	if len(e.remote.calls) != 0 {
		t.Errorf("認証前に外部取得した: %v", e.remote.calls)
	}
}

// AC-H2: /healthz と /images/{name} は認証なし。
func TestNoAuthEndpoints(t *testing.T) {
	e := newEnv(t)
	w := e.do(t, req{method: http.MethodGet, path: "/healthz", auth: "-"})
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"status":"ok"}` {
		t.Errorf("/healthz = %d %s", w.Code, w.Body.String())
	}
	it := e.createItem(t, nil)
	w = e.do(t, req{method: http.MethodGet, path: "/" + it["image_url"].(string), auth: "-"})
	if w.Code != http.StatusOK {
		t.Errorf("画像(認証なし) = %d %s", w.Code, w.Body.String())
	}
}

// ---- エラー形式 ----

// AC-H3: Echo の既定エラーを出さない(存在しないパス・不正な id・不正な JSON)。
func TestErrorShape(t *testing.T) {
	e := newEnv(t)
	expectError(t, e.do(t, req{method: http.MethodGet, path: "/api/nope"}), http.StatusNotFound, "not_found")
	expectError(t, e.do(t, req{method: http.MethodGet, path: "/nope", auth: "-"}), http.StatusNotFound, "not_found")
	expectError(t, e.do(t, req{method: http.MethodGet, path: "/api/items/abc"}), http.StatusBadRequest, "bad_request")
	expectError(t, e.do(t, req{method: http.MethodGet, path: "/api/items?genre_id=abc"}), http.StatusBadRequest, "bad_request")
	expectError(t, e.json(t, http.MethodPost, "/api/genres", "{not json"), http.StatusBadRequest, "bad_request")
	expectError(t, e.json(t, http.MethodPatch, fmt.Sprintf("/api/genres/%d", e.genre.ID), `{"name": 1}`), http.StatusBadRequest, "bad_request")
}

// ---- 商品の登録 ----

// AC-H4: multipart で登録 → 201、Item の形(image_url は images/<name>、site_overrides は [])。
func TestCreateItem_Multipart(t *testing.T) {
	e := newEnv(t)
	body, ct := multipartBody(t, map[string]string{
		"genre_id": fmt.Sprint(e.genre.ID), "name": "グリス", "option_text": "ブリザード",
		"source_url": "https://shop.example.com/1", "min_price": "1000", "sort_order": "3",
	}, testimg.PNG())
	w := e.do(t, req{method: http.MethodPost, path: "/api/items", body: body, ctype: ct})
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d %s", w.Code, w.Body.String())
	}
	m := decode[map[string]any](t, w)
	if m["name"] != "グリス" || m["option_text"] != "ブリザード" || m["source_url"] != "https://shop.example.com/1" ||
		m["min_price"] != float64(1000) || m["sort_order"] != float64(3) || m["genre_id"] != float64(e.genre.ID) {
		t.Errorf("Item = %v", m)
	}
	img, _ := m["image_url"].(string)
	if !strings.HasPrefix(img, "images/") || !storage.ValidName(strings.TrimPrefix(img, "images/")) {
		t.Errorf("image_url = %q, want images/<UUID v4>.png", img)
	}
	if so, ok := m["site_overrides"].([]any); !ok || len(so) != 0 {
		t.Errorf("site_overrides = %#v, want []", m["site_overrides"])
	}
	for _, k := range []string{"id", "created_at", "updated_at"} {
		if m[k] == nil {
			t.Errorf("%s が無い", k)
		}
	}
	if m["query_override"] != nil {
		t.Errorf("query_override = %v, want null か省略", m["query_override"])
	}
	// 一覧と GET でも同じものが見える
	got := decode[map[string]any](t, e.do(t, req{method: http.MethodGet, path: fmt.Sprintf("/api/items/%d", idOf(m))}))
	if got["image_url"] != img || got["name"] != "グリス" {
		t.Errorf("GET = %v", got)
	}
}

// AC-H5: multipart の不正。
func TestCreateItem_MultipartErrors(t *testing.T) {
	cases := []struct {
		name   string
		fields map[string]string
		image  []byte
		status int
		code   string
	}{
		{"画像なし", map[string]string{"genre_id": "GENRE", "name": "x"}, nil, 400, "bad_request"},
		{"name なし", map[string]string{"genre_id": "GENRE"}, testimg.PNG(), 400, "bad_request"},
		{"genre_id が数でない", map[string]string{"genre_id": "abc", "name": "x"}, testimg.PNG(), 400, "bad_request"},
		{"存在しないジャンル", map[string]string{"genre_id": "99999999", "name": "x"}, testimg.PNG(), 422, "unprocessable"},
		{"空の名前", map[string]string{"genre_id": "GENRE", "name": ""}, testimg.PNG(), 422, "unprocessable"},
		{"負の min_price", map[string]string{"genre_id": "GENRE", "name": "x", "min_price": "-1"}, testimg.PNG(), 422, "unprocessable"},
		{"source_url が javascript:", map[string]string{"genre_id": "GENRE", "name": "x", "source_url": "javascript:alert(1)"}, testimg.PNG(), 422, "unprocessable"},
		{"source_url が ftp", map[string]string{"genre_id": "GENRE", "name": "x", "source_url": "ftp://example.com/x"}, testimg.PNG(), 422, "unprocessable"},
		{"SVG", map[string]string{"genre_id": "GENRE", "name": "x"}, testimg.SVG(), 422, "unprocessable"},
		{"10 MiB 超", map[string]string{"genre_id": "GENRE", "name": "x"}, testimg.PaddedPNG(storage.MaxImageBytes + 1), 422, "unprocessable"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			if c.fields["genre_id"] == "GENRE" {
				c.fields["genre_id"] = fmt.Sprint(e.genre.ID)
			}
			body, ct := multipartBody(t, c.fields, c.image)
			expectError(t, e.do(t, req{method: http.MethodPost, path: "/api/items", body: body, ctype: ct}), c.status, c.code)
			if n := countFiles(t, e.dir); n != 0 {
				t.Errorf("失敗したのに画像が %d 個残っている", n)
			}
		})
	}
}

// AC-H6: JSON(image_url)で登録。サーバーが取得して保存する。
func TestCreateItem_JSON(t *testing.T) {
	e := newEnv(t)
	e.remote.image = testimg.JPEG()
	w := e.json(t, http.MethodPost, "/api/items", map[string]any{
		"genre_id": e.genre.ID, "name": "エアリアル", "image_url": "https://cdn.example.com/a.jpg", "query_override": "HG エアリアル",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d %s", w.Code, w.Body.String())
	}
	m := decode[map[string]any](t, w)
	img, _ := m["image_url"].(string)
	if !strings.HasPrefix(img, "images/") || !strings.HasSuffix(img, ".jpg") {
		t.Errorf("image_url = %q", img)
	}
	if m["query_override"] != "HG エアリアル" {
		t.Errorf("query_override = %v", m["query_override"])
	}
	if len(e.remote.calls) != 1 || e.remote.calls[0] != "image https://cdn.example.com/a.jpg" {
		t.Errorf("取得 = %v", e.remote.calls)
	}
	// 保存した画像を返す
	w = e.do(t, req{method: http.MethodGet, path: "/" + img, auth: "-"})
	if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), testimg.JPEG()) {
		t.Errorf("画像 = %d(%d バイト)", w.Code, w.Body.Len())
	}
}

// AC-H7: JSON 登録の失敗。
func TestCreateItem_JSONErrors(t *testing.T) {
	cases := []struct {
		name      string
		imageURL  string
		image     []byte
		imageErr  error
		status    int
		code      string
		wantFetch bool
	}{
		{"ftp は取得しない", "ftp://example.com/a.png", testimg.PNG(), nil, 422, "unprocessable", false},
		{"相対 URL は取得しない", "/a.png", testimg.PNG(), nil, 422, "unprocessable", false},
		{"禁止アドレス", "https://internal.example/a.png", nil, fmt.Errorf("dial: %w", netguard.ErrForbiddenAddress), 422, "unprocessable", true},
		{"取得先が 404", "https://cdn.example.com/a.png", nil, fmt.Errorf("x: %w", ogp.ErrUpstreamStatus), 502, "bad_gateway", true},
		{"取得の失敗(その他)", "https://cdn.example.com/a.png", nil, errors.New("connection reset"), 502, "bad_gateway", true},
		{"大きすぎる", "https://cdn.example.com/a.png", testimg.PaddedPNG(storage.MaxImageBytes + 1), nil, 422, "unprocessable", true},
		{"画像でない", "https://cdn.example.com/a.png", testimg.SVG(), nil, 422, "unprocessable", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			e.remote.image, e.remote.imageErr = c.image, c.imageErr
			w := e.json(t, http.MethodPost, "/api/items", map[string]any{"genre_id": e.genre.ID, "name": "x", "image_url": c.imageURL})
			expectError(t, w, c.status, c.code)
			if fetched := len(e.remote.calls) > 0; fetched != c.wantFetch {
				t.Errorf("取得した = %v, want %v(%v)", fetched, c.wantFetch, e.remote.calls)
			}
			if n := countFiles(t, e.dir); n != 0 {
				t.Errorf("画像が %d 個残っている", n)
			}
		})
	}
	// 入力が不正なら外部の画像を取りに行かない
	t.Run("name が空なら取得しない", func(t *testing.T) {
		e := newEnv(t)
		e.remote.image = testimg.PNG()
		expectError(t, e.json(t, http.MethodPost, "/api/items", map[string]any{"genre_id": e.genre.ID, "name": "", "image_url": "https://cdn.example.com/a.png"}), 422, "unprocessable")
		if len(e.remote.calls) != 0 {
			t.Errorf("取得した: %v", e.remote.calls)
		}
	})
	t.Run("source_url が不正なら取得しない", func(t *testing.T) {
		e := newEnv(t)
		e.remote.image = testimg.PNG()
		expectError(t, e.json(t, http.MethodPost, "/api/items", map[string]any{"genre_id": e.genre.ID, "name": "x", "source_url": "javascript:alert(1)", "image_url": "https://cdn.example.com/a.png"}), 422, "unprocessable")
		if len(e.remote.calls) != 0 {
			t.Errorf("取得した: %v", e.remote.calls)
		}
	})
	t.Run("image_url なし", func(t *testing.T) {
		e := newEnv(t)
		expectError(t, e.json(t, http.MethodPost, "/api/items", map[string]any{"genre_id": e.genre.ID, "name": "x"}), 400, "bad_request")
	})
	t.Run("存在しないジャンル", func(t *testing.T) {
		e := newEnv(t)
		e.remote.image = testimg.PNG()
		expectError(t, e.json(t, http.MethodPost, "/api/items", map[string]any{"genre_id": 99999999, "name": "x", "image_url": "https://cdn.example.com/a.png"}), 422, "unprocessable")
		if n := countFiles(t, e.dir); n != 0 {
			t.Errorf("画像が %d 個残っている", n)
		}
	})
}

// ---- 下書き ----

// AC-H8: from-url は OGP の下書きを返し、保存しない。
func TestDraftFromURL(t *testing.T) {
	e := newEnv(t)
	img := "https://cdn.example.com/og.jpg"
	e.remote.draft = ogp.Draft{Title: "架空フィギュア", ImageURL: &img}
	w := e.json(t, http.MethodPost, "/api/items/from-url", map[string]any{"url": "https://shop.example.com/p/1", "genre_id": e.genre.ID})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d %s", w.Code, w.Body.String())
	}
	m := decode[map[string]any](t, w)
	if m["name"] != "架空フィギュア" || m["image_url"] != img || m["source_url"] != "https://shop.example.com/p/1" || m["genre_id"] != float64(e.genre.ID) {
		t.Errorf("draft = %v", m)
	}
	list := decode[map[string][]any](t, e.do(t, req{method: http.MethodGet, path: "/api/items"}))
	if len(list["items"]) != 0 {
		t.Errorf("下書きで商品が保存された: %v", list)
	}

	// genre_id 省略 → null、画像なし → null
	e.remote.draft = ogp.Draft{Title: "画像なし"}
	w = e.json(t, http.MethodPost, "/api/items/from-url", map[string]any{"url": "https://shop.example.com/p/2"})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d %s", w.Code, w.Body.String())
	}
	m = decode[map[string]any](t, w)
	if m["genre_id"] != nil || m["image_url"] != nil || m["name"] != "画像なし" {
		t.Errorf("draft = %v", m)
	}
}

// AC-H9: from-url の失敗。
func TestDraftFromURL_Errors(t *testing.T) {
	cases := []struct {
		name      string
		body      any
		draftErr  error
		status    int
		code      string
		wantFetch bool
	}{
		{"ftp", map[string]any{"url": "ftp://example.com/x"}, nil, 422, "unprocessable", false},
		{"file", map[string]any{"url": "file:///etc/passwd"}, nil, 422, "unprocessable", false},
		{"url なし", map[string]any{}, nil, 400, "bad_request", false},
		{"禁止アドレス", map[string]any{"url": "https://internal.example/"}, fmt.Errorf("x: %w", netguard.ErrForbiddenAddress), 422, "unprocessable", true},
		{"取得先が 500", map[string]any{"url": "https://shop.example.com/"}, fmt.Errorf("x: %w", ogp.ErrUpstreamStatus), 502, "bad_gateway", true},
		{"HTML が大きすぎる", map[string]any{"url": "https://shop.example.com/"}, fmt.Errorf("x: %w", netguard.ErrTooLarge), 502, "bad_gateway", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			e.remote.draftErr = c.draftErr
			expectError(t, e.json(t, http.MethodPost, "/api/items/from-url", c.body), c.status, c.code)
			if fetched := len(e.remote.calls) > 0; fetched != c.wantFetch {
				t.Errorf("取得した = %v, want %v", fetched, c.wantFetch)
			}
		})
	}
}

// ---- 一覧・取得・更新・削除 ----

// AC-H10: 一覧の並びとジャンルでの絞り込み。
func TestListItems(t *testing.T) {
	e := newEnv(t)
	other, err := e.svc.CreateGenre(context.Background(), item.NewGenre{Name: "ガンプラ", QueryTemplate: "{name}"})
	if err != nil {
		t.Fatal(err)
	}
	a := e.createItem(t, map[string]string{"name": "a", "sort_order": "2"})
	b := e.createItem(t, map[string]string{"name": "b", "sort_order": "1"})
	c := e.createItem(t, map[string]string{"name": "c", "sort_order": "1", "genre_id": fmt.Sprint(other.ID)})
	list := decode[struct {
		Items []map[string]any `json:"items"`
	}](t, e.do(t, req{method: http.MethodGet, path: "/api/items"}))
	var got []int64
	for _, x := range list.Items {
		got = append(got, idOf(x))
	}
	if want := []int64{idOf(c), idOf(b), idOf(a)}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("並び = %v, want %v", got, want)
	}
	list = decode[struct {
		Items []map[string]any `json:"items"`
	}](t, e.do(t, req{method: http.MethodGet, path: fmt.Sprintf("/api/items?genre_id=%d", other.ID)}))
	if len(list.Items) != 1 || idOf(list.Items[0]) != idOf(c) {
		t.Errorf("絞り込み = %v", list.Items)
	}
	// 空でも items は配列
	w := e.do(t, req{method: http.MethodGet, path: "/api/items?genre_id=99999999"})
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"items":[]}` {
		t.Errorf("空の一覧 = %d %s", w.Code, w.Body.String())
	}
}

// AC-H11: 存在しない商品は 404。
func TestItemNotFound(t *testing.T) {
	e := newEnv(t)
	body, ct := multipartBody(t, nil, testimg.PNG())
	for _, r := range []req{
		{method: http.MethodGet, path: "/api/items/99999999"},
		{method: http.MethodPatch, path: "/api/items/99999999", body: strings.NewReader(`{"name":"x"}`), ctype: "application/json"},
		{method: http.MethodDelete, path: "/api/items/99999999"},
		{method: http.MethodPut, path: "/api/items/99999999/image", body: body, ctype: ct},
		{method: http.MethodGet, path: "/api/items/99999999/estimates"},
		{method: http.MethodPost, path: "/api/items/99999999/estimates/refresh"},
		{method: http.MethodGet, path: "/api/items/99999999/listings"},
	} {
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			expectError(t, e.do(t, r), http.StatusNotFound, "not_found")
		})
	}
	if n := countFiles(t, e.dir); n != 0 {
		t.Errorf("存在しない商品への画像差し替えで画像が残った(%d)", n)
	}
}

// AC-H12: PATCH の nullable(null で消す・省略は変えない)と site_overrides の全件置き換え。
func TestUpdateItem(t *testing.T) {
	e := newEnv(t)
	it := e.createItem(t, map[string]string{"option_text": "o", "min_price": "500", "source_url": "https://s.example.com/"})
	path := fmt.Sprintf("/api/items/%d", idOf(it))

	w := e.json(t, http.MethodPatch, path, `{"option_text": null, "name": "グリス改"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d %s", w.Code, w.Body.String())
	}
	m := decode[map[string]any](t, w)
	if m["option_text"] != nil || m["name"] != "グリス改" || m["min_price"] != float64(500) || m["source_url"] != "https://s.example.com/" {
		t.Errorf("null で消す・省略は残す: %v", m)
	}
	if m["image_url"] != it["image_url"] {
		t.Errorf("image_url が変わった: %v → %v", it["image_url"], m["image_url"])
	}

	w = e.json(t, http.MethodPatch, path, map[string]any{"site_overrides": []map[string]any{
		{"site_id": e.site2.ID, "query": "サイト2の語", "enabled": true},
		{"site_id": e.site1.ID, "query": nil, "enabled": false},
	}})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d %s", w.Code, w.Body.String())
	}
	so := decode[map[string]any](t, w)["site_overrides"].([]any)
	if len(so) != 2 || so[0].(map[string]any)["site_id"] != float64(e.site1.ID) || so[1].(map[string]any)["query"] != "サイト2の語" {
		t.Errorf("site_overrides = %v", so)
	}
	w = e.json(t, http.MethodPatch, path, map[string]any{"site_overrides": []any{}})
	if so := decode[map[string]any](t, w)["site_overrides"].([]any); len(so) != 0 {
		t.Errorf("空で置き換え後 = %v", so)
	}

	expectError(t, e.json(t, http.MethodPatch, path, map[string]any{"genre_id": 99999999}), 422, "unprocessable")
	expectError(t, e.json(t, http.MethodPatch, path, map[string]any{"site_overrides": []map[string]any{{"site_id": 99999999, "enabled": true}}}), 422, "unprocessable")
	expectError(t, e.json(t, http.MethodPatch, path, map[string]any{"name": ""}), 422, "unprocessable")
	expectError(t, e.json(t, http.MethodPatch, path, map[string]any{"min_price": -1}), 422, "unprocessable")
	expectError(t, e.json(t, http.MethodPatch, path, map[string]any{"source_url": "ftp://example.com/x"}), 422, "unprocessable")
	expectError(t, e.json(t, http.MethodPatch, path, map[string]any{"source_url": "javascript:alert(1)"}), 422, "unprocessable")
}

// AC-H13: 画像の差し替え(古い画像は 404 になる)と削除(204、画像も消える)。
func TestReplaceImageAndDelete(t *testing.T) {
	e := newEnv(t)
	it := e.createItem(t, nil)
	oldImg := it["image_url"].(string)
	body, ct := multipartBody(t, nil, testimg.GIF())
	w := e.do(t, req{method: http.MethodPut, path: fmt.Sprintf("/api/items/%d/image", idOf(it)), body: body, ctype: ct})
	if w.Code != http.StatusOK {
		t.Fatalf("PUT image = %d %s", w.Code, w.Body.String())
	}
	newImg := decode[map[string]any](t, w)["image_url"].(string)
	if newImg == oldImg || !strings.HasSuffix(newImg, ".gif") {
		t.Errorf("image_url = %q(前 %q)", newImg, oldImg)
	}
	expectError(t, e.do(t, req{method: http.MethodGet, path: "/" + oldImg, auth: "-"}), 404, "not_found")

	body, ct = multipartBody(t, nil, testimg.SVG())
	expectError(t, e.do(t, req{method: http.MethodPut, path: fmt.Sprintf("/api/items/%d/image", idOf(it)), body: body, ctype: ct}), 422, "unprocessable")

	w = e.do(t, req{method: http.MethodDelete, path: fmt.Sprintf("/api/items/%d", idOf(it))})
	if w.Code != http.StatusNoContent {
		t.Errorf("DELETE = %d %s", w.Code, w.Body.String())
	}
	expectError(t, e.do(t, req{method: http.MethodGet, path: fmt.Sprintf("/api/items/%d", idOf(it))}), 404, "not_found")
	expectError(t, e.do(t, req{method: http.MethodGet, path: "/" + newImg, auth: "-"}), 404, "not_found")
	if n := countFiles(t, e.dir); n != 0 {
		t.Errorf("画像が %d 個残っている", n)
	}
}

// ---- フェーズ3の契約(W-06) ----

// AC-H14: estimates は空の sites と refreshing:false、refresh は 501、listings は空配列。
func TestEstimatesPhase1(t *testing.T) {
	e := newEnv(t)
	it := e.createItem(t, nil)
	id := idOf(it)
	w := e.do(t, req{method: http.MethodGet, path: fmt.Sprintf("/api/items/%d/estimates", id)})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d %s", w.Code, w.Body.String())
	}
	m := decode[map[string]any](t, w)
	if m["item_id"] != float64(id) || m["refreshing"] != false {
		t.Errorf("estimates = %v", m)
	}
	if s, ok := m["sites"].([]any); !ok || len(s) != 0 {
		t.Errorf("sites = %#v, want []", m["sites"])
	}
	expectError(t, e.do(t, req{method: http.MethodPost, path: fmt.Sprintf("/api/items/%d/estimates/refresh", id)}), http.StatusNotImplemented, "not_implemented")
	w = e.do(t, req{method: http.MethodGet, path: fmt.Sprintf("/api/items/%d/listings?site_id=%d", id, e.site1.ID)})
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"listings":[]}` {
		t.Errorf("listings = %d %s", w.Code, w.Body.String())
	}
}

// ---- 画像配信 ----

// AC-H15: /images/{name} は正しい Content-Type と長期・immutable の Cache-Control で返す。形式外・無いものは 404。
func TestImages(t *testing.T) {
	e := newEnv(t)
	cases := []struct {
		name  string
		data  []byte
		ctype string
	}{
		{"png", testimg.PNG(), "image/png"},
		{"jpeg", testimg.JPEG(), "image/jpeg"},
		{"gif", testimg.GIF(), "image/gif"},
		{"webp", testimg.WebP(), "image/webp"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body, ct := multipartBody(t, map[string]string{"genre_id": fmt.Sprint(e.genre.ID), "name": c.name}, c.data)
			w := e.do(t, req{method: http.MethodPost, path: "/api/items", body: body, ctype: ct})
			if w.Code != http.StatusCreated {
				t.Fatalf("POST = %d %s", w.Code, w.Body.String())
			}
			img := decode[map[string]any](t, w)["image_url"].(string)
			w = e.do(t, req{method: http.MethodGet, path: "/" + img, auth: "-"})
			if w.Code != http.StatusOK {
				t.Fatalf("GET %s = %d", img, w.Code)
			}
			if got := w.Header().Get("Content-Type"); got != c.ctype {
				t.Errorf("Content-Type = %q, want %q", got, c.ctype)
			}
			if got := w.Header().Get("Cache-Control"); got != httpapi.ImageCacheControl {
				t.Errorf("Cache-Control = %q, want %q", got, httpapi.ImageCacheControl)
			}
			if w.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Errorf("X-Content-Type-Options = %q, want nosniff", w.Header().Get("X-Content-Type-Options"))
			}
			if !bytes.Equal(w.Body.Bytes(), c.data) {
				t.Error("内容が一致しない")
			}
		})
	}
	for _, p := range []string{
		"/images/0f8fad5b-d9cb-469f-a165-70867728950e.png", // 形式は正しいが無い
		"/images/x.png",
		"/images/..%2F..%2Fetc%2Fpasswd",
		"/images/0f8fad5b-d9cb-469f-a165-70867728950e.svg",
	} {
		t.Run(p, func(t *testing.T) {
			expectError(t, e.do(t, req{method: http.MethodGet, path: p, auth: "-"}), 404, "not_found")
		})
	}
}

// ---- 設定 ----

// AC-H16: ジャンルの一覧(site_ids を含む)・作成・更新。
func TestGenres(t *testing.T) {
	e := newEnv(t)
	w := e.do(t, req{method: http.MethodGet, path: "/api/genres"})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	gs := decode[map[string][]map[string]any](t, w)["genres"]
	if len(gs) != 1 || gs[0]["name"] != "S.H.Figuarts" || fmt.Sprint(gs[0]["site_ids"]) != fmt.Sprint([]any{float64(e.site1.ID), float64(e.site2.ID)}) {
		t.Errorf("genres = %v", gs)
	}

	w = e.json(t, http.MethodPost, "/api/genres", map[string]any{"name": "デュエマ"})
	if w.Code != http.StatusCreated {
		t.Fatalf("POST = %d %s", w.Code, w.Body.String())
	}
	g := decode[map[string]any](t, w)
	if g["query_template"] != "{name} {option}" || g["sort_order"] != float64(0) {
		t.Errorf("既定値 = %v", g)
	}
	if s, ok := g["site_ids"].([]any); !ok || len(s) != 0 {
		t.Errorf("site_ids = %#v, want []", g["site_ids"])
	}
	expectError(t, e.json(t, http.MethodPost, "/api/genres", map[string]any{"name": "デュエマ"}), 422, "unprocessable")
	expectError(t, e.json(t, http.MethodPost, "/api/genres", map[string]any{"name": "x", "site_ids": []int64{99999999}}), 422, "unprocessable")

	path := fmt.Sprintf("/api/genres/%d", idOf(g))
	w = e.json(t, http.MethodPatch, path, map[string]any{"site_ids": []int64{e.site2.ID, e.site1.ID}})
	if w.Code != http.StatusOK {
		t.Fatalf("PATCH = %d %s", w.Code, w.Body.String())
	}
	g = decode[map[string]any](t, w)
	if fmt.Sprint(g["site_ids"]) != fmt.Sprint([]any{float64(e.site2.ID), float64(e.site1.ID)}) || g["name"] != "デュエマ" {
		t.Errorf("PATCH 後 = %v", g)
	}
	expectError(t, e.json(t, http.MethodPatch, "/api/genres/99999999", map[string]any{"name": "z"}), 404, "not_found")
	expectError(t, e.json(t, http.MethodPatch, path, map[string]any{"name": "S.H.Figuarts"}), 422, "unprocessable")
}

// AC-H17: サイトの一覧・作成・更新(検索 URL テンプレートの検査)。
func TestSites(t *testing.T) {
	e := newEnv(t)
	w := e.json(t, http.MethodPost, "/api/sites", map[string]any{"name": "架空ショップ", "search_url_template": "https://shop.example.com/search?q={q}"})
	if w.Code != http.StatusCreated {
		t.Fatalf("POST = %d %s", w.Code, w.Body.String())
	}
	s := decode[map[string]any](t, w)
	if s["fetch_type"] != "link_only" || s["is_reference"] != false {
		t.Errorf("既定値 = %v", s)
	}
	expectError(t, e.json(t, http.MethodPost, "/api/sites", map[string]any{"name": "y", "search_url_template": "https://shop.example.com/search?q="}), 422, "unprocessable")
	expectError(t, e.json(t, http.MethodPost, "/api/sites", map[string]any{"name": "架空ショップ", "search_url_template": "https://a.example.com/{q}"}), 422, "unprocessable")
	expectError(t, e.json(t, http.MethodPost, "/api/sites", map[string]any{"name": "z", "search_url_template": "https://a.example.com/{q}", "fetch_type": "rss"}), 422, "unprocessable")

	path := fmt.Sprintf("/api/sites/%d", idOf(s))
	w = e.json(t, http.MethodPatch, path, map[string]any{"is_reference": true})
	if w.Code != http.StatusOK {
		t.Fatalf("PATCH = %d %s", w.Code, w.Body.String())
	}
	if got := decode[map[string]any](t, w); got["is_reference"] != true || got["name"] != "架空ショップ" {
		t.Errorf("PATCH 後 = %v", got)
	}
	expectError(t, e.json(t, http.MethodPatch, path, map[string]any{"search_url_template": "javascript:{q}"}), 422, "unprocessable")
	expectError(t, e.json(t, http.MethodPatch, "/api/sites/99999999", map[string]any{"name": "q"}), 404, "not_found")

	list := decode[map[string][]map[string]any](t, e.do(t, req{method: http.MethodGet, path: "/api/sites"}))["sites"]
	if len(list) != 3 || idOf(list[0]) != e.site1.ID || idOf(list[2]) != idOf(s) {
		t.Errorf("sites = %v", list)
	}
}
