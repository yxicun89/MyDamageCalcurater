package item_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/oapi-codegen/nullable"

	"example.com/pokecalc/apps/wishlist/api/internal/deeplink"
	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/storage"
	"example.com/pokecalc/apps/wishlist/api/internal/testimg"
)

type env struct {
	svc    *item.Service
	repo   *item.MemoryRepository
	images *storage.FileStorage
	dir    string
	genre  item.Genre
}

func newEnv(t *testing.T) env {
	t.Helper()
	dir := t.TempDir()
	images, err := storage.NewFileStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	repo := item.NewMemoryRepository()
	svc := item.NewService(repo, images)
	g, err := svc.CreateGenre(context.Background(), item.NewGenre{Name: "サービステスト"})
	if err != nil {
		t.Fatal(err)
	}
	return env{svc: svc, repo: repo, images: images, dir: dir, genre: g}
}

func files(t *testing.T, dir string) []string {
	t.Helper()
	es, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range es {
		out = append(out, e.Name())
	}
	return out
}

func strp(s string) *string { return &s }
func intp(i int) *int       { return &i }

// AC-V1: 商品の作成で画像を保存し、ImagePath は保存した名前。
func TestService_CreateItem(t *testing.T) {
	e := newEnv(t)
	it, err := e.svc.CreateItem(context.Background(), item.NewItem{GenreID: e.genre.ID, Name: "グリス", ImagePath: "無視される.png"}, bytes.NewReader(testimg.PNG()))
	if err != nil {
		t.Fatal(err)
	}
	if !storage.ValidName(it.ImagePath) {
		t.Errorf("ImagePath = %q", it.ImagePath)
	}
	if fs := files(t, e.dir); len(fs) != 1 || fs[0] != it.ImagePath {
		t.Errorf("保存先 = %v, want [%s]", fs, it.ImagePath)
	}
}

// AC-V2: 商品の作成に失敗したら画像を残さない。画像が不正なら商品を作らない。
func TestService_CreateItem_Rollback(t *testing.T) {
	ctx := context.Background()
	t.Run("存在しないジャンル", func(t *testing.T) {
		e := newEnv(t)
		_, err := e.svc.CreateItem(ctx, item.NewItem{GenreID: 99999999, Name: "x"}, bytes.NewReader(testimg.PNG()))
		if !errors.Is(err, item.ErrGenreNotFound) {
			t.Errorf("err = %v", err)
		}
		if fs := files(t, e.dir); len(fs) != 0 {
			t.Errorf("画像が残っている: %v", fs)
		}
	})
	t.Run("対応外の画像", func(t *testing.T) {
		e := newEnv(t)
		_, err := e.svc.CreateItem(ctx, item.NewItem{GenreID: e.genre.ID, Name: "x"}, bytes.NewReader(testimg.SVG()))
		if !errors.Is(err, storage.ErrUnsupportedImage) {
			t.Errorf("err = %v", err)
		}
		if list, _ := e.svc.ListItems(ctx, nil); len(list) != 0 {
			t.Errorf("商品ができた: %+v", list)
		}
	})
	t.Run("検査で弾く(画像を保存しない)", func(t *testing.T) {
		e := newEnv(t)
		_, err := e.svc.CreateItem(ctx, item.NewItem{GenreID: e.genre.ID, Name: "  "}, bytes.NewReader(testimg.PNG()))
		if !errors.Is(err, item.ErrInvalid) {
			t.Errorf("err = %v", err)
		}
		if fs := files(t, e.dir); len(fs) != 0 {
			t.Errorf("画像が残っている: %v", fs)
		}
	})
}

// AC-V3: 入力の検査(ErrInvalid)。
func TestService_Validation(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	it, err := e.svc.CreateItem(ctx, item.NewItem{GenreID: e.genre.ID, Name: "x"}, bytes.NewReader(testimg.PNG()))
	if err != nil {
		t.Fatal(err)
	}
	long := func(n int) string { return strings.Repeat("あ", n) }
	cases := []struct {
		name string
		call func() error
	}{
		{"商品名が空", func() error {
			_, err := e.svc.CreateItem(ctx, item.NewItem{GenreID: e.genre.ID, Name: ""}, bytes.NewReader(testimg.PNG()))
			return err
		}},
		{"商品名が 256 文字", func() error {
			_, err := e.svc.CreateItem(ctx, item.NewItem{GenreID: e.genre.ID, Name: long(256)}, bytes.NewReader(testimg.PNG()))
			return err
		}},
		{"min_price が負", func() error {
			_, err := e.svc.CreateItem(ctx, item.NewItem{GenreID: e.genre.ID, Name: "x", MinPrice: intp(-1)}, bytes.NewReader(testimg.PNG()))
			return err
		}},
		{"更新で商品名を空に", func() error {
			_, err := e.svc.UpdateItem(ctx, it.ID, item.ItemPatch{Name: strp("")})
			return err
		}},
		{"site_overrides の query が 256 文字", func() error {
			_, err := e.svc.UpdateItem(ctx, it.ID, item.ItemPatch{SiteOverrides: &[]item.SiteOverride{{SiteID: 1, Query: strp(long(256)), Enabled: true}}})
			return err
		}},
		{"ジャンル名が 65 文字", func() error {
			_, err := e.svc.CreateGenre(ctx, item.NewGenre{Name: long(65)})
			return err
		}},
		{"ジャンル名が空白だけ", func() error {
			_, err := e.svc.CreateGenre(ctx, item.NewGenre{Name: " "})
			return err
		}},
		{"source_url が javascript:", func() error {
			_, err := e.svc.CreateItem(ctx, item.NewItem{GenreID: e.genre.ID, Name: "x", SourceURL: strp("javascript:alert(1)")}, bytes.NewReader(testimg.PNG()))
			return err
		}},
		{"source_url が ftp", func() error {
			_, err := e.svc.CreateItem(ctx, item.NewItem{GenreID: e.genre.ID, Name: "x", SourceURL: strp("ftp://example.com/x")}, bytes.NewReader(testimg.PNG()))
			return err
		}},
		{"更新で source_url を javascript: に", func() error {
			_, err := e.svc.UpdateItem(ctx, it.ID, item.ItemPatch{SourceURL: nullable.NewNullableWithValue("javascript:alert(1)")})
			return err
		}},
		{"サイトの検索 URL が不正", func() error {
			_, err := e.svc.CreateSite(ctx, item.NewSite{Name: "s", SearchURLTemplate: "https://example.com/s?q="})
			return err
		}},
		{"サイトの fetch_type が不正", func() error {
			_, err := e.svc.CreateSite(ctx, item.NewSite{Name: "s", SearchURLTemplate: "https://example.com/s?q={q}", FetchType: "rss"})
			return err
		}},
		{"更新で検索 URL を不正に", func() error {
			s, err := e.svc.CreateSite(ctx, item.NewSite{Name: "s2", SearchURLTemplate: "https://example.com/s?q={q}"})
			if err != nil {
				return err
			}
			_, err = e.svc.UpdateSite(ctx, s.ID, item.SitePatch{SearchURLTemplate: strp("javascript:{q}")})
			return err
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.call(); !errors.Is(err, item.ErrInvalid) {
				t.Errorf("err = %v, want ErrInvalid", err)
			}
		})
	}
	// 検索 URL の不正は deeplink のエラーも包む
	_, err = e.svc.CreateSite(ctx, item.NewSite{Name: "s3", SearchURLTemplate: "ftp://x/{q}"})
	if !errors.Is(err, deeplink.ErrInvalidTemplate) {
		t.Errorf("err = %v, want deeplink.ErrInvalidTemplate も包む", err)
	}
}

// AC-V4: 既定値(テンプレート省略 → "{name} {option}"、fetch_type 省略 → link_only)。
func TestService_Defaults(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	if e.genre.QueryTemplate != item.DefaultQueryTemplate {
		t.Errorf("QueryTemplate = %q", e.genre.QueryTemplate)
	}
	s, err := e.svc.CreateSite(ctx, item.NewSite{Name: "既定", SearchURLTemplate: "https://example.com/s?q={q}"})
	if err != nil {
		t.Fatal(err)
	}
	if s.FetchType != item.FetchLinkOnly || s.IsReference {
		t.Errorf("site = %+v", s)
	}
}

// AC-V5: 画像の差し替えは古い画像を消す。商品が無ければ新しい画像を残さない。
func TestService_ReplaceImage(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	it, err := e.svc.CreateItem(ctx, item.NewItem{GenreID: e.genre.ID, Name: "x"}, bytes.NewReader(testimg.PNG()))
	if err != nil {
		t.Fatal(err)
	}
	got, err := e.svc.ReplaceImage(ctx, it.ID, bytes.NewReader(testimg.JPEG()))
	if err != nil {
		t.Fatal(err)
	}
	if got.ImagePath == it.ImagePath || !strings.HasSuffix(got.ImagePath, ".jpg") {
		t.Errorf("ImagePath = %q(前 %q)", got.ImagePath, it.ImagePath)
	}
	if fs := files(t, e.dir); len(fs) != 1 || fs[0] != got.ImagePath {
		t.Errorf("保存先 = %v, want [%s]", fs, got.ImagePath)
	}
	if _, err := e.svc.ReplaceImage(ctx, 99999999, bytes.NewReader(testimg.PNG())); !errors.Is(err, item.ErrNotFound) {
		t.Errorf("存在しない商品 err = %v", err)
	}
	if fs := files(t, e.dir); len(fs) != 1 {
		t.Errorf("存在しない商品への差し替えで画像が残った: %v", fs)
	}
	if _, err := e.svc.ReplaceImage(ctx, it.ID, bytes.NewReader(testimg.SVG())); !errors.Is(err, storage.ErrUnsupportedImage) {
		t.Errorf("SVG err = %v", err)
	}
	if again, _ := e.svc.GetItem(ctx, it.ID); again.ImagePath != got.ImagePath {
		t.Errorf("失敗した差し替えで ImagePath が変わった: %q", again.ImagePath)
	}
}

// AC-V6: 商品の削除で画像も消す。
func TestService_DeleteItem(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	it, err := e.svc.CreateItem(ctx, item.NewItem{GenreID: e.genre.ID, Name: "x"}, bytes.NewReader(testimg.PNG()))
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.DeleteItem(ctx, it.ID); err != nil {
		t.Fatal(err)
	}
	if fs := files(t, e.dir); len(fs) != 0 {
		t.Errorf("画像が残っている: %v", fs)
	}
	if _, err := e.svc.GetItem(ctx, it.ID); !errors.Is(err, item.ErrNotFound) {
		t.Errorf("err = %v", err)
	}
	if err := e.svc.DeleteItem(ctx, it.ID); !errors.Is(err, item.ErrNotFound) {
		t.Errorf("2 回目 err = %v", err)
	}
}
